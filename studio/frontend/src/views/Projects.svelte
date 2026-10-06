<script lang="ts">
  import { onMount } from 'svelte';
  import { App, projectFile, errText, ask, sourceName } from '../lib/api';
  import CatalogPicker from './CatalogPicker.svelte';

  let { open, notify }: { open: (id: string) => void; notify: (t: string, k?: string) => void } = $props();

  let projects = $state<any[]>([]);
  let loading = $state(true);
  let creating = $state(false);
  let newId = $state('');
  let newName = $state('');

  // List controls, remembered between launches.
  const saved = (() => { try { return JSON.parse(localStorage.getItem('sets.view') || '{}'); } catch { return {}; } })();
  let query = $state('');
  let show = $state<'all' | 'installed' | 'not'>(saved.show ?? 'all');
  let sort = $state<string>(saved.sort ?? 'installed');
  $effect(() => { try { localStorage.setItem('sets.view', JSON.stringify({ show, sort })); } catch {} });

  const byName = (a: any, b: any) => a.name.localeCompare(b.name, undefined, { sensitivity: 'base' });
  /** Where a set came from, for the Source sort: converted mods by name, then each import source, then hand-made sets. */
  function sourceKey(p: any): string {
    if (p.origin) return '0 ' + p.origin.toLowerCase();
    if (p.source && p.source !== 'manual') return '1 ' + sourceName(p.source).toLowerCase();
    return '2';
  }

  const SORTS: Record<string, [string, (a: any, b: any) => number]> = {
    installed: ['Installed first', (a, b) => Number(b.installed) - Number(a.installed) || byName(a, b)],
    name: ['Name', byName],
    newest: ['Release date (newest)', (a, b) => (b.releasedAt || '').localeCompare(a.releasedAt || '') || byName(a, b)],
    oldest: ['Release date (oldest)', (a, b) => (a.releasedAt || '9999').localeCompare(b.releasedAt || '9999') || byName(a, b)],
    tier: ['Gamify tier', (a, b) => (a.tier || 1e9) - (b.tier || 1e9) || byName(a, b)],
    source: ['Source (where it came from)', (a, b) => sourceKey(a).localeCompare(sourceKey(b)) || byName(a, b)],
    modified: ['Recently edited', (a, b) => String(b.modified).localeCompare(String(a.modified))]
  };

  const installedCount = $derived(projects.filter((p) => p.installed).length);
  const outdated = $derived(projects.filter((p) => p.installState === 'stale'));
  let updating = $state(false);
  const shown = $derived.by(() => {
    const q = query.trim().toLowerCase();
    return projects
      .filter((p) => show === 'all' || (show === 'installed') === !!p.installed)
      .filter((p) => !q || [p.name, p.id, p.code].some((s) => s && s.toLowerCase().includes(q)))
      .sort(SORTS[sort]?.[1] ?? byName);
  });

  /** Reloads the list. Only the first load shows "Loading…": refreshes after install/uninstall update the rows in place,
   *  so the page keeps its scroll position. */
  let fromCatalog = $state(false);

  async function addedFromCatalog(ids: string[]) {
    fromCatalog = false;
    await load();
    if (ids.length) notify(`Added ${ids.length} set${ids.length === 1 ? '' : 's'} from the catalog — Install ${ids.length === 1 ? 'it' : 'them'} to play.`, 'ok');
  }

  async function load(first = false) {
    if (first) loading = true;
    try {
      projects = await App.ListProjects();
    } catch (e) {
      notify(errText(e), 'error');
    }
    loading = false;
  }

  async function create() {
    try {
      const p = await App.CreateProject(newId.trim(), newName.trim() || newId.trim());
      creating = false;
      open(p.id);
    } catch (e) {
      notify(errText(e), 'error');
    }
  }

  const restartHint = (running: boolean) => running ? 'restart the game to see it' : "you'll see it next time you start the game";

  async function install(p: any) {
    try {
      const r = await App.InstallProject(p.id);
      const warns = r.issues.filter((i: any) => i.level === 'warning').length;
      notify(`${p.installed ? 'Updated' : 'Installed'} "${p.name}" in the game${warns ? ` (${warns} warnings)` : ''} — ${restartHint(r.gameRunning)}.`, 'ok');
      load();
    } catch (e) {
      notify(`${p.name}: ${errText(e)}`, 'error');
    }
  }

  /** Installs every set whose game copy is older than the project. */
  async function updateAll() {
    updating = true;
    const failed: string[] = [];
    let done = 0, running = false;
    for (const p of outdated) {
      try { const r = await App.InstallProject(p.id); done++; running = r.gameRunning; }
      catch (e) { failed.push(`${p.name} (${errText(e)})`); }
    }
    updating = false;
    await load();
    if (failed.length) notify(`Updated ${done} set(s); not updated: ${failed.join(', ')}`, 'error');
    else notify(`Updated ${done} set(s) in the game — ${restartHint(running)}.`, 'ok');
  }

  async function uninstall(p: any) {
    if (!(await ask(`Remove "${p.name}" from the game? Your project stays here, and player saves keep their data for when you reinstall.`))) return;
    try {
      await App.UninstallProject(p.id);
      load();
    } catch (e) {
      notify(errText(e), 'error');
    }
  }

  async function remove(p: any) {
    if (!(await ask(`Delete "${p.name}"? This removes it from this setup${p.installed ? ' and the game' : ''}; it stays in the catalog (Add from catalog… brings it back). Player saves keep their data and get it back if you re-import the set.`))) return;
    try {
      await App.DeleteProject(p.id);
      notify(`Deleted "${p.name}"${p.installed ? ' and removed it from the game' : ''}`, 'ok');
    } catch (e) {
      notify(errText(e), 'error');
    }
    load();
  }

  onMount(() => load(true));
</script>

<div class="page">
  <header class="row">
    <h2 class="grow">Sets</h2>
    <button onclick={() => (fromCatalog = true)} title="Sets from your other setups and earlier imports — no re-import">Add from catalog…</button>
    <button onclick={() => (creating = !creating)}>New empty set</button>
  </header>

  {#if creating}
    <div class="create row">
      <label class="field">Set id (stable, no spaces)<input bind:value={newId} placeholder="my-set" /></label>
      <label class="field grow">Name<input bind:value={newName} placeholder="My Set" /></label>
      <button class="primary" disabled={!newId.trim()} onclick={create}>Create</button>
    </div>
  {/if}

  {#if loading}
    <p class="muted">Loading…</p>
  {:else if projects.length === 0}
    <div class="empty">
      <p>No sets yet.</p>
      <p class="muted">Use <b>Import</b> to pull in a whole set (Magic, Pokémon, Yu-Gi-Oh!, One Piece, Star Wars: Unlimited, Lorcana, Flesh and Blood, Union Arena), or create an empty set.</p>
    </div>
  {:else}
    <div class="toolbar row">
      <input class="grow" type="search" bind:value={query} placeholder="Search by name, id or set code…" />
      <div class="seg">
        <button class:on={show === 'all'} onclick={() => (show = 'all')}>All ({projects.length})</button>
        <button class:on={show === 'installed'} onclick={() => (show = 'installed')}>Installed ({installedCount})</button>
        <button class:on={show === 'not'} onclick={() => (show = 'not')}>Not installed ({projects.length - installedCount})</button>
      </div>
      {#if outdated.length}
        <button class="primary" disabled={updating} onclick={updateAll} title={outdated.map((p) => p.name).join(', ')}>
          {updating ? 'Updating…' : `Update all outdated (${outdated.length})`}</button>
      {/if}
      <label class="sortby">Sort
        <select bind:value={sort}>{#each Object.entries(SORTS) as [k, [label]]}<option value={k}>{label}</option>{/each}</select>
      </label>
    </div>
    {#if shown.length === 0}<p class="muted">No sets match.</p>{/if}
    <div class="list">
      {#each shown as p (p.id)}
        <div class="item" class:installed={p.installed}>
          <button class="thumb" onclick={() => open(p.id)} aria-label="Open {p.name}">
            {#if p.cover}<img src={projectFile(p.id, p.cover)} alt="" loading="lazy" />{/if}
          </button>
          <div class="info grow">
            <button class="name" onclick={() => open(p.id)}>{p.name}</button>
            <div class="muted small">
              {p.id} · {p.cards} cards · {p.packs} pack{p.packs === 1 ? '' : 's'}
              {#if p.origin} · from EPL mod “{p.origin}”
              {:else if p.code} · {sourceName(p.source)} {p.code.toUpperCase()}{p.releasedAt ? ` (${p.releasedAt})` : ''}{/if}
            </div>
            <div class="row" style="margin-top:6px">
              {#if p.installState === 'stale'}<span class="badge warn" title="Edited since it was installed — the game has an older version">Outdated in game</span>
              {:else if p.installed}<span class="badge ok">In game ✓</span>{:else}<span class="badge">Not installed</span>{/if}
              {#if p.tier}<span class="badge">Tier {p.tier}</span>{/if}
            </div>
          </div>
          <div class="actions">
            <button class="primary" onclick={() => open(p.id)}>Edit</button>
            {#if p.installState === 'stale'}<button class="primary" onclick={() => install(p)}>Update game</button>
            {:else}<button onclick={() => install(p)}>{p.installed ? 'Reinstall' : 'Install'}</button>{/if}
            {#if p.installed}<button onclick={() => uninstall(p)}>Uninstall</button>{/if}
            <button class="danger" onclick={() => remove(p)}>Delete</button>
          </div>
        </div>
      {/each}
    </div>
  {/if}
</div>

{#if fromCatalog}<CatalogPicker kind="set" onadded={addedFromCatalog} onclose={() => (fromCatalog = false)} />{/if}

<style>
  .page { padding: 20px 24px; overflow: auto; height: 100%; }
  header { margin-bottom: 16px; }
  .create { background: var(--panel); padding: 12px; border-radius: var(--radius); margin-bottom: 16px; align-items: flex-end; }
  .empty { padding: 40px; text-align: center; background: var(--panel); border-radius: var(--radius); }
  .toolbar { margin-bottom: 12px; align-items: center; flex-wrap: wrap; }
  .toolbar input[type="search"] { min-width: 220px; }
  .seg { display: flex; }
  .seg button { border-radius: 0; margin-left: -1px; font-size: 13px; }
  .seg button:first-child { border-radius: 6px 0 0 6px; }
  .seg button:last-child { border-radius: 0 6px 6px 0; }
  .seg button.on { border-color: var(--accent); background: #22304d; position: relative; }
  .sortby { display: flex; align-items: center; gap: 6px; font-size: 13px; }
  .list { display: flex; flex-direction: column; gap: 10px; }
  .item.installed { border-left: 3px solid var(--ok, #3fb96b); }
  .item { display: flex; gap: 14px; align-items: center; background: var(--panel); border: 1px solid var(--line); border-radius: var(--radius); padding: 10px; }
  .thumb { width: 64px; height: 89px; padding: 0; overflow: hidden; background: var(--bg); flex-shrink: 0; }
  .thumb img { width: 100%; height: 100%; object-fit: cover; }
  .name { background: none; border: none; padding: 0; font-size: 16px; font-weight: 600; }
  .small { font-size: 12px; margin-top: 2px; }
  .actions { display: flex; flex-direction: column; gap: 4px; }
</style>
