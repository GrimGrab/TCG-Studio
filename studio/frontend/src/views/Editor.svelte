<script lang="ts">
  // The set editor: every edit (cards, packs, pack art, rotated images, set info) stays in the page's draft until Save, which
  // stores the set; the game reads an installed set's saved files.
  import { onMount } from 'svelte';
  import SaveBar from './SaveBar.svelte';
  import Dropdown from './Dropdown.svelte';
  import { App, errText, ask, sourceName, sourceCode, BrowserOpenURL, originLabel, projectFile } from '../lib/api';
  import CardsTab from './editor/CardsTab.svelte';
  import PacksTab from './editor/PacksTab.svelte';
  import SetTab from './editor/SetTab.svelte';
  import { applyBakedPictures } from '../lib/setPictures';
  import { inPack } from '../lib/packPools';
  import type { Goto, NavRequest, SetTab as TabName } from '../lib/setNav';

  let { id, notify, close }: { id: string; notify: (t: string, k?: string) => void; close: () => void } = $props();

  let project = $state<any>(null);
  let tab = $state<TabName>('cards');
  let issues = $state<any[]>([]);
  let checksOpen = $state(false);
  let saved = $state('');
  let saving = $state(false);
  let imgBust = $state(Date.now());
  let installState = $state<'none' | 'current' | 'stale' | ''>('');
  // Run before saving or switching tabs (the pack art editor applies unapplied edits).
  const hooks = new Set<() => Promise<void>>();
  async function flushAll() { for (const f of [...hooks]) await f(); }
  async function showTab(t: TabName) { if (t !== tab) { await flushAll(); tab = t; } }
  // One link request between tabs (lib/setNav): the tab it names takes it when it shows.
  let nav = $state<NavRequest | null>(null);
  // A link to a tab also leaves the card editor (it covers the Cards tab, so the link would seem to do nothing).
  const goto: Goto = async (t, opts = {}) => { cardEditing = false; nav = { tab: t, ...opts }; await showTab(t); };
  function takeNav(t: TabName) { if (nav?.tab !== t) return null; const r = nav; nav = null; return r; }

  const dirty = $derived(project ? JSON.stringify(project) !== saved : false);
  /** The pack art editor converted a pack's or box's design on opening (same look): that form counts as saved, so opening it
   *  alone is no unsaved change (other unsaved edits stay unsaved). */
  function rebaseArt(packId: string, which: string, layout: any) {
    if (!saved) return;
    const s = JSON.parse(saved);
    ((s.meta ??= {}).packArt ??= {})[packId] ??= {};
    s.meta.packArt[packId][which] = $state.snapshot(layout);
    saved = JSON.stringify(s);
  }
  const errors = $derived(issues.filter((i) => i.level === 'error').length);

  async function load() {
    try {
      project = await App.LoadProject(id);
      imgBust = Date.now();
      saved = JSON.stringify(project);
      issues = await App.ValidateProject(id);
      installState = (await App.ProjectInstallState(id)) as any;
    } catch (e) {
      notify(errText(e), 'error');
      close();
    }
  }

  const restartHint = (running: boolean) => running ? 'restart the game to see it' : "you'll see it next time you start the game";

  /** Saves the set; installed sets are updated in the game at the same time (Go side). */
  async function save() {
    saving = true;
    try {
      await flushAll();
      const r = await App.SaveProject($state.snapshot(project) as any);
      issues = r.issues;
      installState = r.installState as any;
      applyBakedPictures(project, r.pictures); // the files the crop boxes' pictures were cut into
      saved = JSON.stringify(project);
      if (r.synced) notify(`Saved and updated in the game — ${restartHint(r.gameRunning)}.`, 'ok');
      else if (r.syncError) { notify(`Saved, but ${r.syncError}.`, 'error'); checksOpen = errors > 0; }
      else if (installState === 'none') notify(`Saved.${errors ? ` Fix ${errors} error(s) before installing.` : ' Click "Install to game" to play with it.'}`, errors ? 'error' : 'ok');
      else notify('Saved.', 'ok');
    } catch (e) {
      notify(errText(e), 'error');
    }
    saving = false;
  }

  async function install() {
    if (dirty) { await save(); if (installState === 'current') return; }
    try {
      const r = await App.InstallProject(id);
      issues = r.issues;
      installState = r.installState as any;
      notify(`Installed into the game — ${restartHint(r.gameRunning)}.`, 'ok');
    } catch (e) {
      notify(errText(e), 'error');
      checksOpen = true;
    }
  }

  // The full card editor open over the Cards tab: the header then names the card, and ← goes back to the set's cards.
  let cardEditing = $state(false);
  let editingCard = $state<any>(null);
  const inCard = $derived(tab === 'cards' && cardEditing && !!editingCard);

  async function back() {
    if (inCard) { cardEditing = false; return; }
    await leave();
  }

  async function leave() {
    await flushAll();
    if (dirty && !(await ask('Discard unsaved changes?'))) return;
    if (!dirty && installState === 'stale' && !errors && (await ask('The game still has an older version of this set. Update it now?'))) await install();
    close();
  }

  async function refreshPrices() {
    if (dirty && !(await ask('Refreshing prices reloads the set from disk, so your unsaved changes will be lost. Continue?'))) return;
    if (dirty) await save();
    try {
      const msg = await App.RefreshPrices(id);
      await load();
      notify(msg, 'ok');
    } catch (e) {
      notify(errText(e), 'error');
    }
  }

  async function saveToCatalog() {
    try { await App.SaveToCatalog('set', [project.set.id]); notify(`The catalog now has this setup's saved version of "${project.set.name}".`, 'ok'); }
    catch (e) { notify(errText(e), 'error'); }
  }

  // Double-faced cards (transform, modal DFC…) imported before back faces existed: their back picture can be fetched.
  const DFC_LAYOUTS = ['transform', 'modal_dfc', 'reversible_card'];
  let missingBacks = $derived(project?.meta?.source === 'scryfall'
    ? (project.set.cards ?? []).filter((c: any) => !c.backImage && DFC_LAYOUTS.includes(project.meta.cards?.[c.id]?.layout)).length
    : 0);
  let fetchingBacks = $state(false);

  async function fetchBackFaces() {
    if (dirty && !(await ask('Downloading back faces reloads the set from disk, so your unsaved changes will be lost. Continue?'))) return;
    if (dirty) await save();
    fetchingBacks = true;
    try {
      const msg = await App.FetchBackFaces(id);
      await load();
      notify(msg, 'ok');
    } catch (e) {
      notify(errText(e), 'error');
    } finally {
      fetchingBacks = false;
    }
  }

  // Checks: everything worth a look in one list (the game's copy, cards no pack gives, missing back faces, validation).
  const noPackCount = $derived(project?.set.packs.length
    ? project.set.cards.filter((c: any) => !project.set.packs.some((p: any) => inPack(p, c.id))).length : 0);
  const checkCount = $derived(issues.length + (noPackCount ? 1 : 0) + (missingBacks ? 1 : 0) + (installState === 'stale' ? 1 : 0));
  const checksLabel = $derived(errors ? `⛔ ${errors} error${errors === 1 ? '' : 's'}` : checkCount ? `⚠ ${checkCount} to check` : '✓ Ready');

  // The set's cover in the header: the first card's picture, like the Sets page.
  const cover = $derived(project?.set.cards.find((c: any) => c.image)?.image ?? '');

  onMount(load);
</script>

{#if project}
  <div class="editor">
    <SaveBar {dirty} {saving} onsave={save} onrevert={load}
      leaveMessage="Leave this set without saving? Your unsaved changes will be lost."
      revertMessage="Throw away your unsaved changes to this set?"
      saveTitle={installState === 'none' ? 'Save the set (Ctrl+S)' : 'Save the set and update it in the game (Ctrl+S)'}>
      {#snippet title()}
      <button class="back" onclick={back} title={inCard ? `Back to ${project.set.name}'s cards (Esc)` : 'Back to Sets'}>←</button>
      <div class="cover">{#if cover}<img src={projectFile(project.id, cover, imgBust)} alt="" />{:else}<span>{project.set.name.slice(0, 2)}</span>{/if}</div>
      <div class="grow title">
        {#if inCard}
          <h2 class="crumbs" title="{project.set.name} › {editingCard.name || editingCard.id}">
            <button class="crumb" onclick={back} title="Back to the set's cards">{project.set.name}</button>
            <span class="sep">›</span><span class="cardname">{editingCard.name || editingCard.id}</span></h2>
        {:else}
          <h2 title={project.set.name}>{project.set.name}</h2>
        {/if}
        <div class="meta">
          {#if project.meta?.origin?.mod}<span>{originLabel(project.meta.origin.kind, project.meta.origin.mod, project.meta.origin.author)}</span>
          {:else if sourceCode(project.meta)}<span>{sourceName(project.meta.source)} · {sourceCode(project.meta).toUpperCase()}</span>{/if}
          {#if project.set.released}<span>{project.set.released.slice(0, 4)}</span>{/if}
          <span class="code" title="Set id (stable)">{project.id}</span>
          {#if project.meta?.origin?.link}<button class="link" onclick={() => BrowserOpenURL(project.meta.origin.link)} title={project.meta.origin.link}>{project.meta.origin.kind === 'Shared file' ? 'Link' : 'Mod page'} ↗</button>{/if}
        </div>
      </div>
      {/snippet}
      {#snippet extra()}
      <Dropdown bind:open={checksOpen} align="right" width="420px" cls="checks {errors ? 'err' : checkCount ? 'warn' : 'ok'}"
        title={checkCount ? 'Problems and things worth a look' : 'Nothing to fix'}>
        {#snippet label()}{checksLabel}{/snippet}
        {#snippet panel(close)}
          {#if !checkCount}<span class="muted small">No problems: the game can load this set as it is.</span>{/if}
          {#if installState === 'stale'}
            <div class="check"><span>The game reads another setup or workspace, or its mod is older.</span>
              <button class="small" onclick={() => { close(); install(); }}>Fix</button></div>
          {/if}
          {#if noPackCount}
            <div class="check"><span>{noPackCount} card{noPackCount === 1 ? ' is' : 's are'} in no pack, so players can't get {noPackCount === 1 ? 'it' : 'them'} from packs.</span>
              <button class="small" onclick={() => { close(); goto('cards', { noPack: true }); }}>Show</button></div>
          {/if}
          {#if missingBacks > 0}
            <div class="check"><span>{missingBacks} double-faced card{missingBacks === 1 ? ' has' : 's have'} no back picture (MTG games show the other side).</span>
              <button class="small" disabled={fetchingBacks} onclick={fetchBackFaces}>{fetchingBacks ? 'Downloading…' : 'Download'}</button></div>
          {/if}
          {#if issues.length}
            <div class="issue-list">
              {#each issues as i}<div class="notice" class:err={i.level === 'error'}><b>{i.where}</b>: {i.message}</div>{/each}
            </div>
          {/if}
        {/snippet}
      </Dropdown>
      {#if installState === 'current'}
        <span class="chip ok" title="Installed: the game loads the saved set at its next start">● In game</span>
      {:else if installState === 'stale'}
        <button class="chip warn" onclick={install} title="The game reads another setup or workspace, or its mod is older — click to fix">Not read by the game — Fix</button>
      {:else if installState === 'none'}
        <button class="primary" onclick={install} title="This set isn't in the game yet">Install to game</button>
      {/if}
      <Dropdown align="right" cls="dots" title="More" caret={false}>
        {#snippet label()}⋯{/snippet}
        {#snippet panel(close)}
          <div class="ddmenu">
            {#if sourceCode(project.meta)}
              <button onclick={() => { close(); refreshPrices(); }} title="Pull today's {sourceName(project.meta.source)} prices (re-applies this set's Gamify settings)">Refresh prices</button>
            {/if}
            <button onclick={() => { close(); saveToCatalog(); }}
              title="Make this setup's saved version the one other setups get when they add it from the catalog (setups that already have it keep theirs).">Update catalog</button>
            <button onclick={() => { close(); App.OpenProjectFolder(id); }}>Open the set's folder</button>
          </div>
        {/snippet}
      </Dropdown>
      {/snippet}
    </SaveBar>

    <!-- Editing one card: no set tabs (← or the set's name in the header goes back to the set). -->
    {#if !inCard}
    <nav class="tabbar">
      <button class:on={tab === 'cards'} onclick={() => showTab('cards')}>Cards<span class="n">{project.set.cards.length}</span></button>
      <button class:on={tab === 'packs'} onclick={() => showTab('packs')}>Packs<span class="n">{project.set.packs.length}</span></button>
      <button class:on={tab === 'set'} onclick={() => showTab('set')}>Set</button>
    </nav>
    {/if}

    <div class="body">
      {#if tab === 'cards'}
        <CardsTab {project} {notify} {imgBust} {goto} {nav} {takeNav} bind:editing={cardEditing} bind:editingCard />
      {:else if tab === 'packs'}
        <PacksTab {project} {notify} {imgBust} {hooks} {rebaseArt} {goto} {nav} {takeNav} />
      {:else}
        <SetTab {project} {notify} {imgBust} {nav} {takeNav} />
      {/if}
    </div>
  </div>
{:else}
  <p class="muted" style="padding:24px">Loading…</p>
{/if}

<style>
  .editor { display: flex; flex-direction: column; height: 100%; }
  .back { padding: 6px 10px; font-size: 16px; line-height: 1; }
  .cover { width: 34px; height: 47px; flex: none; border-radius: 4px; overflow: hidden; background: var(--panel-2); display: grid; place-items: center;
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.4); }
  .cover img { width: 100%; height: 100%; object-fit: cover; }
  .cover span { font-size: 12px; font-weight: 700; color: var(--muted); text-transform: uppercase; }
  .title { display: flex; flex-direction: column; min-width: 0; gap: 1px; }
  .title h2 { white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .crumbs { display: flex; align-items: baseline; gap: 8px; min-width: 0; }
  .crumb { padding: 0; background: none; border: none; font: inherit; color: var(--muted); cursor: pointer; flex: none; max-width: 45%;
    overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .crumb:hover { color: var(--text); text-decoration: underline; }
  .sep { color: var(--muted); flex: none; }
  .cardname { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .meta { display: flex; flex-wrap: wrap; align-items: center; gap: 2px 12px; color: var(--muted); font-size: 12px; }
  .link { padding: 0; font-size: 12px; }
  .chip { font-size: 12px; padding: 4px 10px; border-radius: 999px; border: 1px solid var(--line); color: var(--muted); white-space: nowrap; }
  .chip.ok { color: var(--ok); border-color: #1f6b4b; }
  .chip.warn { color: var(--warn); border-color: #6b5320; background: transparent; cursor: pointer; }
  :global(.ddbtn.checks) { font-size: 12px; padding: 4px 10px; border-radius: 999px; background: transparent; }
  :global(.ddbtn.checks.ok) { color: var(--ok); border-color: #1f6b4b; }
  :global(.ddbtn.checks.warn) { color: var(--warn); border-color: #6b5320; }
  :global(.ddbtn.checks.err) { color: var(--danger); border-color: #6b2a31; }
  .check { display: flex; align-items: center; gap: 10px; font-size: 13px; }
  .check span { flex: 1; }
  .issue-list { max-height: 320px; overflow: auto; display: flex; flex-direction: column; gap: 4px; }
  .body { flex: 1; min-height: 0; }
</style>
