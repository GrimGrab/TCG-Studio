<script lang="ts">
  import { onMount } from 'svelte';
  import { App, errText, ask, sourceName, sourceCode, BrowserOpenURL } from '../lib/api';
  import CardsTab from './editor/CardsTab.svelte';
  import PacksTab from './editor/PacksTab.svelte';
  import SetTab from './editor/SetTab.svelte';

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
  const errors = $derived(issues.filter((i) => i.level === 'error').length);
  const warnings = $derived(issues.filter((i) => i.level === 'warning').length);

  async function load() {
    try {
      project = await App.LoadProject(id);
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
      saved = JSON.stringify(project);
      if (r.synced) notify(`Saved and updated in the game — ${restartHint(r.gameRunning)}.`, 'ok');
      else if (r.syncError) { notify(`Saved, but the game still has the old version: ${r.syncError}.`, 'error'); showIssues = errors > 0; }
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

  /** Card image files changed on disk (e.g. rotated): reload them and bring the game up to date like a save would. */
  async function imagesChanged(what: string) {
    imgBust = Date.now();
    try { installState = (await App.ProjectInstallState(id)) as any; } catch {}
    if (dirty) notify(`${what} Save to update the game.`, 'ok');
    else if (installState === 'stale' && !errors) await install();
    else notify(what, 'ok');
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

  function onKey(e: KeyboardEvent) {
    if ((e.ctrlKey || e.metaKey) && e.key === 's') {
      e.preventDefault();
      if (dirty && !saving) save();
    }
  }

  onMount(load);
</script>

<svelte:window onkeydown={onKey} />

{#if project}
  <div class="editor">
    <header class="row">
      <button onclick={leave}>← Sets</button>
      <div class="grow title">
        <h2>{project.set.name}</h2>
        <span class="muted">{project.id}{project.meta?.origin?.mod ? ` · from EPL mod “${project.meta.origin.mod}”${project.meta.origin.author ? ` by ${project.meta.origin.author}` : ''}` : sourceCode(project.meta) ? ` · ${sourceName(project.meta.source)} ${sourceCode(project.meta).toUpperCase()}` : ''}</span>{#if project.meta?.origin?.link}<button class="link small" onclick={() => BrowserOpenURL(project.meta.origin.link)} title={project.meta.origin.link}>Mod page ↗</button>{/if}
      </div>
      <button class="issues" onclick={() => (showIssues = !showIssues)}>
        {#if errors}<span class="badge err">{errors} errors</span>{/if}
        {#if warnings}<span class="badge warn">{warnings} warnings</span>{/if}
        {#if !errors && !warnings}<span class="badge ok">Valid</span>{/if}
      </button>
      {#if sourceCode(project.meta)}
        <button onclick={refreshPrices} title="Pull today's {sourceName(project.meta.source)} prices (re-applies this set's Gamify settings)">Refresh prices</button>
      {/if}
      {#if missingBacks > 0}
        <button onclick={fetchBackFaces} disabled={fetchingBacks}
          title="Download the back-face pictures of this set's double-faced cards, so transformed cards show their other side in MTG games">
          {fetchingBacks ? 'Downloading back faces…' : `Download back faces (${missingBacks})`}</button>
      {/if}
      <button onclick={() => App.OpenProjectFolder(id)}>Folder</button>
      {#if installState === 'current'}
        <span class="chip ok" title="The game has this version of the set">In game ✓</span>
      {:else if installState === 'stale'}
        <button class="chip warn" onclick={install} title="The game has an older version of this set — click to update it">In game: outdated — Update</button>
      {:else if installState === 'none'}
        <span class="chip" title="This set isn't in the game yet">Not in game</span>
      {/if}
      <button class="primary" disabled={!dirty || saving} onclick={save}
        title={installState === 'none' ? 'Save the set' : 'Save the set and update it in the game'}>{saving ? 'Saving…' : dirty ? 'Save •' : 'Saved'}</button>
      {#if installState === 'none'}<button class="primary" onclick={install}>Install to game</button>{/if}
    </header>

    {#if showIssues && issues.length}
      <div class="issue-list">
        {#each issues as i}<div class={i.level}><b>{i.where}</b>: {i.message}</div>{/each}
      </div>
    {/if}

    <nav class="tabs">
      <button class:active={tab === 'cards'} onclick={() => showTab('cards')}>Cards ({project.set.cards.length})</button>
      <button class:active={tab === 'packs'} onclick={() => showTab('packs')}>Packs ({project.set.packs.length})</button>
      <button class:active={tab === 'set'} onclick={() => showTab('set')}>Set</button>
    </nav>

    <div class="body">
      {#if tab === 'cards'}
        <CardsTab {project} {notify} {imgBust} onimages={imagesChanged} />
      {:else if tab === 'packs'}
        <PacksTab {project} {notify} {imgBust} {save} {hooks} />
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
  header { padding: 12px 18px; border-bottom: 1px solid var(--line); background: var(--panel); }
  .title { display: flex; flex-direction: column; }
  .issues { display: flex; gap: 4px; background: transparent; }
  .chip { font-size: 12px; padding: 4px 10px; border-radius: 999px; border: 1px solid var(--line); color: var(--muted); white-space: nowrap; }
  .chip.ok { color: var(--ok, #3fb96b); border-color: var(--ok, #3fb96b); }
  .chip.warn { color: var(--warn, #d9a400); border-color: var(--warn, #d9a400); background: transparent; cursor: pointer; }
  .issue-list { max-height: 160px; overflow: auto; padding: 8px 18px; background: #1a1c22; border-bottom: 1px solid var(--line); font-size: 13px; }
  .issue-list .error { color: var(--danger); }
  .issue-list .warning { color: var(--warn); }
  .tabs { display: flex; gap: 4px; padding: 8px 18px 0; border-bottom: 1px solid var(--line); }
  .tabs button { border-radius: 6px 6px 0 0; border-bottom: none; background: transparent; }
  .tabs button.active { background: var(--panel-2); }
  .body { flex: 1; min-height: 0; }
</style>
