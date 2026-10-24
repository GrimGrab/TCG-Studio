<script lang="ts">
  // The set editor: every edit (cards, packs, pack art, rotated images, set info) stays in the page's draft until Save, which
  // stores the set; the game reads an installed set's saved files.
  import { onMount } from 'svelte';
  import SaveBar from './SaveBar.svelte';
  import { App, errText, ask, sourceName, sourceCode, BrowserOpenURL, originLabel, projectFile } from '../lib/api';
  import CardsTab from './editor/CardsTab.svelte';
  import PacksTab from './editor/PacksTab.svelte';
  import SetTab from './editor/SetTab.svelte';
  import { applyBakedPictures } from '../lib/setPictures';

  let { id, notify, close }: { id: string; notify: (t: string, k?: string) => void; close: () => void } = $props();

  let project = $state<any>(null);
  let tab = $state<'cards' | 'packs' | 'set'>('cards');
  let issues = $state<any[]>([]);
  let showIssues = $state(false);
  let saved = $state('');
  let saving = $state(false);
  let imgBust = $state(Date.now());
  let installState = $state<'none' | 'current' | 'stale' | ''>('');
  // Run before saving or switching tabs (the pack art editor applies unapplied edits).
  const hooks = new Set<() => Promise<void>>();
  async function flushAll() { for (const f of [...hooks]) await f(); }
  async function showTab(t: typeof tab) { if (t !== tab) { await flushAll(); tab = t; } }

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
  const warnings = $derived(issues.filter((i) => i.level === 'warning').length);

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
      else if (r.syncError) { notify(`Saved, but ${r.syncError}.`, 'error'); showIssues = errors > 0; }
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
      showIssues = true;
    }
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

  // The set's cover in the header: the first card's picture, like the Sets page.
  const cover = $derived(project?.set.cards.find((c: any) => c.image)?.image ?? '');
  let menu = $state(false);
  function closeMenu(e: MouseEvent) { if (menu && !(e.target as HTMLElement).closest('.more')) menu = false; }

  onMount(load);
</script>

<svelte:window onmousedown={closeMenu} />

{#if project}
  <div class="editor">
    <SaveBar {dirty} {saving} onsave={save} onrevert={load}
      leaveMessage="Leave this set without saving? Your unsaved changes will be lost."
      revertMessage="Throw away your unsaved changes to this set?"
      saveTitle={installState === 'none' ? 'Save the set (Ctrl+S)' : 'Save the set and update it in the game (Ctrl+S)'}>
      {#snippet title()}
      <button class="back" onclick={leave} title="Back to Sets">←</button>
      <div class="cover">{#if cover}<img src={projectFile(project.id, cover, imgBust)} alt="" />{:else}<span>{project.set.name.slice(0, 2)}</span>{/if}</div>
      <div class="grow title">
        <h2 title={project.set.name}>{project.set.name}</h2>
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
      <button class="issues" class:on={showIssues} onclick={() => (showIssues = !showIssues)} title={issues.length ? 'Show the problems the game would find' : 'Nothing to fix'}>
        {#if errors}<span class="badge err">{errors} error{errors === 1 ? '' : 's'}</span>{/if}
        {#if warnings}<span class="badge warn">{warnings} warning{warnings === 1 ? '' : 's'}</span>{/if}
        {#if !errors && !warnings}<span class="badge ok">✓ Valid</span>{/if}
      </button>
      {#if missingBacks > 0}
        <button onclick={fetchBackFaces} disabled={fetchingBacks}
          title="Download the back-face pictures of this set's double-faced cards, so transformed cards show their other side in MTG games">
          {fetchingBacks ? 'Downloading back faces…' : `Download back faces (${missingBacks})`}</button>
      {/if}
      {#if installState === 'current'}
        <span class="chip ok" title="Installed: the game loads the saved set at its next start">● In game</span>
      {:else if installState === 'stale'}
        <button class="chip warn" onclick={install} title="The game reads another setup or workspace, or its mod is older — click to fix">Not read by the game — Fix</button>
      {:else if installState === 'none'}
        <button class="primary" onclick={install} title="This set isn't in the game yet">Install to game</button>
      {/if}
      <div class="more">
        <button class="dots" onclick={() => (menu = !menu)} aria-label="More" title="More">⋯</button>
        {#if menu}
          <div class="menu">
            {#if sourceCode(project.meta)}
              <button onclick={() => { menu = false; refreshPrices(); }} title="Pull today's {sourceName(project.meta.source)} prices (re-applies this set's Gamify settings)">Refresh prices</button>
            {/if}
            <button onclick={() => { menu = false; App.OpenProjectFolder(id); }}>Open the set's folder</button>
          </div>
        {/if}
      </div>
      {/snippet}
    </SaveBar>

    {#if showIssues && issues.length}
      <div class="issue-list">
        {#each issues as i}<div class="notice" class:err={i.level === 'error'}><b>{i.where}</b>: {i.message}</div>{/each}
      </div>
    {/if}

    <nav class="tabbar">
      <button class:on={tab === 'cards'} onclick={() => showTab('cards')}>Cards<span class="n">{project.set.cards.length}</span></button>
      <button class:on={tab === 'packs'} onclick={() => showTab('packs')}>Packs<span class="n">{project.set.packs.length}</span></button>
      <button class:on={tab === 'set'} onclick={() => showTab('set')}>Set details</button>
    </nav>

    <div class="body">
      {#if tab === 'cards'}
        <CardsTab {project} {notify} {imgBust} />
      {:else if tab === 'packs'}
        <PacksTab {project} {notify} {imgBust} {hooks} {rebaseArt} />
      {:else}
        <SetTab {project} {notify} {imgBust} />
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
  .meta { display: flex; flex-wrap: wrap; align-items: center; gap: 2px 12px; color: var(--muted); font-size: 12px; }
  .link { padding: 0; font-size: 12px; }
  .issues { display: flex; gap: 4px; background: transparent; border-color: transparent; padding: 4px 6px; }
  .issues.on { border-color: var(--line); }
  .chip { font-size: 12px; padding: 4px 10px; border-radius: 999px; border: 1px solid var(--line); color: var(--muted); white-space: nowrap; }
  .chip.ok { color: var(--ok); border-color: #1f6b4b; }
  .chip.warn { color: var(--warn); border-color: #6b5320; background: transparent; cursor: pointer; }
  .menu { top: calc(100% + 4px); min-width: 200px; }
  .issue-list { max-height: 180px; overflow: auto; padding: 10px 20px; display: flex; flex-direction: column; gap: 4px; border-bottom: 1px solid var(--line); }
  .body { flex: 1; min-height: 0; }
</style>
