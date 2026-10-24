<script lang="ts">
  import { onMount } from 'svelte';
  import { App, projectFile, errText, ask, sourceName } from '../lib/api';
  import { matchSearch } from '../lib/search';
  import CatalogPicker from './CatalogPicker.svelte';

  let { open, notify }: { open: (id: string) => void; notify: (t: string, k?: string) => void } = $props();

  let projects = $state<any[]>([]);
  let loading = $state(true);
  let creating = $state(false);
  let newName = $state('');
  let games = $state<Record<string, string>>({}); // import source id → game ("Magic: The Gathering")

  // List controls, remembered between launches.
  const saved = (() => { try { return JSON.parse(localStorage.getItem('sets.view') || '{}'); } catch { return {}; } })();
  let query = $state('');
  let show = $state<'all' | 'installed' | 'not'>(saved.show ?? 'all');
  let sort = $state<string>(saved.sort ?? 'installed');
  let layout = $state<'grid' | 'list'>(saved.layout ?? 'grid');
  let groupBy = $state<'none' | 'game' | 'source'>(saved.groupBy ?? 'game');
  // Folded groups, per grouping ("game:Disney Lorcana"), remembered with the rest of the view.
  let folded = $state<Record<string, boolean>>(saved.folded ?? {});
  $effect(() => { try { localStorage.setItem('sets.view', JSON.stringify({ show, sort, layout, groupBy, folded })); } catch {} });
  const foldKey = (title: string) => `${groupBy}:${title}`;
  function toggleFold(title: string) {
    const k = foldKey(title);
    if (folded[k]) delete folded[k];
    else folded[k] = true;
  }
  function foldAll(on: boolean) {
    const next = { ...folded };
    for (const sec of sections) {
      if (on) next[foldKey(sec.title)] = true;
      else delete next[foldKey(sec.title)];
    }
    folded = next;
  }

  const byName = (a: any, b: any) => a.name.localeCompare(b.name, undefined, { sensitivity: 'base' });
  /** Where a set came from, for the Source sort: converted mods by name, then each import source, then hand-made sets. */
  function sourceKey(p: any): string {
    if (p.origin) return '0 ' + p.origin.toLowerCase();
    if (p.source && p.source !== 'manual') return '1 ' + sourceName(p.source).toLowerCase();
    return '2';
  }
  /** The game a set belongs to: its import source's game, a converted mod's name, else "Your own sets". */
  function gameOf(p: any): string {
    if (p.origin) return p.origin; // a converted mod / shared file: its name
    if (p.source === 'folder') return 'Your own card images';
    return games[p.source] || p.collection || 'Your own sets';
  }

  const SORTS: Record<string, [string, (a: any, b: any) => number]> = {
    installed: ['Installed first', (a, b) => Number(b.installed) - Number(a.installed) || byName(a, b)],
    name: ['Name', byName],
    game: ['Game', (a, b) => gameOf(a).localeCompare(gameOf(b)) || byName(a, b)],
    newest: ['Release date (newest)', (a, b) => (b.releasedAt || '').localeCompare(a.releasedAt || '') || byName(a, b)],
    oldest: ['Release date (oldest)', (a, b) => (a.releasedAt || '9999').localeCompare(b.releasedAt || '9999') || byName(a, b)],
    tier: ['Gamify tier', (a, b) => (a.tier || 1e9) - (b.tier || 1e9) || byName(a, b)],
    source: ['Source (where it came from)', (a, b) => sourceKey(a).localeCompare(sourceKey(b)) || byName(a, b)],
    modified: ['Recently edited', (a, b) => String(b.modified).localeCompare(String(a.modified))]
  };

  const installedCount = $derived(projects.filter((p) => p.installed).length);
  const outdated = $derived(projects.filter((p) => p.installState === 'stale'));
  const totalCards = $derived(projects.reduce((n, p) => n + (p.cards ?? 0), 0));
  const gameCount = $derived(new Set(projects.map(gameOf)).size);
  let updating = $state(false);
  const shown = $derived.by(() => {
    return projects
      .filter((p) => show === 'all' || (show === 'installed') === !!p.installed)
      .filter((p) => matchSearch(query, p.name, p.id, p.code, gameOf(p), sourceName(p.source), p.origin))
      .sort(SORTS[sort]?.[1] ?? byName);
  });
  /** Where a set came from, as a group heading: the database, the converted mod or shared file, or Studio itself. */
  function sourceLabel(p: any): string {
    if (p.origin) return `${p.originKind === 'Shared file' ? 'Shared file' : 'EPL mod'}: ${p.origin}`;
    if (p.source && p.source !== 'manual') return sourceName(p.source);
    return 'Made in TCG Studio';
  }
  // Group: one section per game / source (A–Z) with a heading, the chosen sort inside each; None = one section.
  const sections = $derived.by(() => {
    if (groupBy === 'none') return [{ title: '', items: shown }];
    const key = groupBy === 'game' ? gameOf : sourceLabel;
    const by = new Map<string, any[]>();
    for (const p of shown) {
      const k = key(p);
      if (!by.has(k)) by.set(k, []);
      by.get(k)!.push(p);
    }
    return [...by.entries()].sort((a, b) => a[0].localeCompare(b[0])).map(([title, items]) => ({ title, items }));
  });

  const year = (d: string) => (d ? d.slice(0, 4) : '');
  const subline = (p: any) => [gameOf(p), p.code ? p.code.toUpperCase() : '', year(p.releasedAt)].filter(Boolean).join(' · ');

  // The ⋯ menu of one tile (Delete and the rarer actions).
  let menuFor = $state('');
  function closeMenu(e: MouseEvent) {
    if (menuFor && !(e.target as HTMLElement).closest('.more')) menuFor = '';
  }

  let fromCatalog = $state(false);

  async function addedFromCatalog(ids: string[]) {
    fromCatalog = false;
    await load();
    if (ids.length) notify(`Added ${ids.length} set${ids.length === 1 ? '' : 's'} from the catalog — Install ${ids.length === 1 ? 'it' : 'them'} to play.`, 'ok');
  }

  /** Reloads the list. Only the first load shows "Loading…": refreshes after install/uninstall update the rows in place,
   *  so the page keeps its scroll position. */
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
      const p = await App.CreateProject(newName.trim());
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
      notify(`${p.installed ? 'The game reads' : 'Installed'} "${p.name}"${p.installed ? ' from here again' : ' in the game'}${warns ? ` (${warns} warnings)` : ''} — ${restartHint(r.gameRunning)}.`, 'ok');
      load();
    } catch (e) {
      notify(`${p.name}: ${errText(e)}`, 'error');
    }
  }

  /** Points the game at this setup again for every set it doesn't read from here (another setup / workspace, older mod). */
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
    if (failed.length) notify(`Fixed ${done} set(s); not fixed: ${failed.join(', ')}`, 'error');
    else notify(`The game reads ${done} set(s) from here again — ${restartHint(running)}.`, 'ok');
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
    menuFor = '';
    if (!(await ask(`Delete "${p.name}"? This removes it from this setup${p.installed ? ' and the game' : ''}; it stays in the catalog (Add from catalog… brings it back). Player saves keep their data and get it back if you re-import the set.`))) return;
    try {
      await App.DeleteProject(p.id);
      notify(`Deleted "${p.name}"${p.installed ? ' and removed it from the game' : ''}`, 'ok');
    } catch (e) {
      notify(errText(e), 'error');
    }
    load();
  }

  onMount(() => {
    load(true);
    App.ImportSources().then((list: any[]) => (games = Object.fromEntries(list.map((s) => [s.id, s.game])))).catch(() => {});
  });
</script>

<svelte:window onmousedown={closeMenu} />

<div class="page">
  <header class="page-head">
    <div class="title">
      <h2>Sets</h2>
      {#if !loading && projects.length}
        <div class="page-summary">
          <span><b>{projects.length}</b> sets</span>
          <span><b>{installedCount}</b> installed</span>
          <span><b>{totalCards.toLocaleString()}</b> cards</span>
          <span><b>{gameCount}</b> {gameCount === 1 ? 'game' : 'games'}</span>
          {#if outdated.length}<span class="warn"><b>{outdated.length}</b> not read by the game</span>{/if}
        </div>
      {/if}
    </div>
    <span class="grow"></span>
    <button onclick={() => (fromCatalog = true)} title="Sets from your other setups and earlier imports — no re-import">Add from catalog…</button>
    <button class="primary" onclick={() => (creating = !creating)}>New empty set</button>
  </header>

  {#if creating}
    <div class="create">
      <label class="field grow">Name of the new set<input bind:value={newName} placeholder="My Set" onkeydown={(e) => e.key === 'Enter' && create()} /></label>
      <button class="primary" onclick={create}>Create</button>
      <button onclick={() => (creating = false)}>Cancel</button>
    </div>
  {/if}

  {#if !loading && projects.length}
    <div class="toolbar">
      <input class="search" type="search" bind:value={query} placeholder="Search by name, game (mtg, ygo, ptcg…), id or set code…" title="Accents don't matter, and every word must match: “mtg dominaria”" />
      <div class="seg">
        <button class:on={show === 'all'} onclick={() => (show = 'all')}>All <span class="n">{projects.length}</span></button>
        <button class:on={show === 'installed'} onclick={() => (show = 'installed')}>Installed <span class="n">{installedCount}</span></button>
        <button class:on={show === 'not'} onclick={() => (show = 'not')}>Not installed <span class="n">{projects.length - installedCount}</span></button>
      </div>
      <span class="grow"></span>
      {#if outdated.length}
        <button class="primary" disabled={updating} onclick={updateAll} title={outdated.map((p) => p.name).join(', ')}>
          {updating ? 'Fixing…' : `Fix all not read by the game (${outdated.length})`}</button>
      {/if}
      <label class="sortby">Group
        <select bind:value={groupBy}><option value="none">None</option><option value="game">Game</option><option value="source">Source</option></select>
      </label>
      {#if groupBy !== 'none' && sections.length > 1}
        <div class="seg" title="Fold or open every group">
          <button onclick={() => foldAll(true)}>Collapse all</button>
          <button onclick={() => foldAll(false)}>Expand all</button>
        </div>
      {/if}
      <label class="sortby">Sort
        <select bind:value={sort}>{#each Object.entries(SORTS) as [k, [label]]}<option value={k}>{label}</option>{/each}</select>
      </label>
      <div class="seg" title="Layout">
        <button class:on={layout === 'grid'} onclick={() => (layout = 'grid')} aria-label="Grid">▦</button>
        <button class:on={layout === 'list'} onclick={() => (layout = 'list')} aria-label="List">☰</button>
      </div>
    </div>
  {/if}

  <div class="main">
    {#if loading}
      <div class="grid">{#each Array(8) as _}<div class="tile skeleton"><div class="cover"></div><div class="body"><div class="bar"></div><div class="bar short"></div></div></div>{/each}</div>
    {:else if projects.length === 0}
      <div class="empty">
        <div class="emptyglyph">🃏</div>
        <h1>No sets yet</h1>
        <p class="muted">Use <b>Import</b> to bring in a whole set from 53 card games, add one from your <b>catalog</b>, or start an empty set and add your own cards.</p>
        <div class="row center">
          <button onclick={() => (fromCatalog = true)}>Add from catalog…</button>
          <button class="primary" onclick={() => (creating = true)}>New empty set</button>
        </div>
      </div>
    {:else if shown.length === 0}
      <p class="muted nomatch">No set matches — try another search or filter.</p>
    {:else}
      {#each sections as sec (sec.title)}
      {#if sec.title}
        <button class="section" class:folded={folded[foldKey(sec.title)]} onclick={() => toggleFold(sec.title)}
          aria-expanded={!folded[foldKey(sec.title)]} title={folded[foldKey(sec.title)] ? 'Show these sets' : 'Hide these sets'}>
          <span class="chev">{folded[foldKey(sec.title)] ? '▸' : '▾'}</span>
          <h3>{sec.title}</h3><span class="muted small">{sec.items.length} set{sec.items.length === 1 ? '' : 's'} ·
            {sec.items.reduce((n, p) => n + (p.cards ?? 0), 0).toLocaleString()} cards</span>
        </button>
      {/if}
      {#if !sec.title || !folded[foldKey(sec.title)]}
      <div class:grid={layout === 'grid'} class:rows={layout === 'list'}>
        {#each sec.items as p (p.id)}
          <div class="tile" class:stale={p.installState === 'stale'}>
            <button class="cover" onclick={() => open(p.id)} aria-label="Edit {p.name}" title="Edit {p.name}">
              {#if p.cover}<img src={projectFile(p.id, p.cover)} alt="" loading="lazy" />{:else}<span class="nocover">{p.name.slice(0, 2)}</span>{/if}
              {#if layout === 'grid'}
                {#if p.installState === 'stale'}<span class="pill warn">Not read by the game</span>
                {:else if p.installed}<span class="pill ok">Installed</span>{/if}
              {/if}
            </button>
            <div class="body">
              <button class="name" onclick={() => open(p.id)} title={p.name}>{p.name}</button>
              <div class="sub" title={subline(p)}>{subline(p)}</div>
              <div class="facts">
                <span>{p.cards} cards</span><span>{p.packs} pack{p.packs === 1 ? '' : 's'}</span>
                {#if p.tier}<span>Tier {p.tier}</span>{/if}
                {#if layout === 'list'}
                  {#if p.installState === 'stale'}<span class="badge warn">Not read by the game</span>
                  {:else if p.installed}<span class="badge ok">Installed</span>{:else}<span class="badge">Not installed</span>{/if}
                {/if}
              </div>
            </div>
            <div class="actions">
              <button class="primary" onclick={() => open(p.id)}>Edit</button>
              {#if p.installState === 'stale'}<button class="primary" onclick={() => install(p)} title="The game reads another setup or workspace, or its mod is older than this TCG Studio">Fix</button>
              {:else if p.installed}<button onclick={() => uninstall(p)}>Uninstall</button>
              {:else}<button onclick={() => install(p)}>Install</button>{/if}
              <div class="more">
                <button class="dots" onclick={() => (menuFor = menuFor === p.id ? '' : p.id)} aria-label="More" title="More">⋯</button>
                {#if menuFor === p.id}
                  <div class="menu">
                    <div class="muted small id">{p.id}</div>
                    <button class="danger" onclick={() => remove(p)}>Delete set…</button>
                  </div>
                {/if}
              </div>
            </div>
          </div>
        {/each}
      </div>
      {/if}
      {/each}
    {/if}
  </div>
</div>

{#if fromCatalog}<CatalogPicker kind="set" onadded={addedFromCatalog} onclose={() => (fromCatalog = false)} />{/if}

<style>
  .page { display: flex; flex-direction: column; height: 100%; overflow: hidden; }
  .page-summary .warn b { color: var(--warn); }
  .create { display: flex; align-items: flex-end; gap: 8px; padding: 12px 24px; background: var(--panel); border-bottom: 1px solid var(--line); }
  .create .field { max-width: 420px; }
  .toolbar { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; padding: 12px 24px; border-bottom: 1px solid var(--line); background: var(--panel); }
  .toolbar .search { width: 300px; }
  .seg .n { font-size: 11px; color: var(--muted); margin-left: 2px; }
  .sortby { display: flex; align-items: center; gap: 6px; font-size: 13px; color: var(--muted); }
  .main { flex: 1; min-height: 0; overflow: auto; padding: 18px 24px 28px; }
  .nomatch { padding: 8px 0; }
  .section { display: flex; align-items: baseline; gap: 10px; width: 100%; margin: 22px 0 10px; padding: 0 0 6px; text-align: left;
    background: none; border: none; border-bottom: 1px solid var(--line); border-radius: 0; }
  .section:hover:not(:disabled) { border-color: var(--line); }
  .section:hover h3 { color: var(--accent); }
  .section:first-child { margin-top: 0; }
  .section.folded { margin-bottom: 0; }
  .section h3 { color: var(--text); font-size: 13px; }
  .chev { width: 12px; color: var(--muted); font-size: 12px; }

  /* Grid: a library of tiles, cover card on top. */
  .grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: 16px; }
  .tile { position: relative; display: flex; flex-direction: column; background: var(--panel); border: 1px solid var(--line); border-radius: 12px;
    overflow: visible; transition: border-color 0.12s, transform 0.12s; }
  .tile:hover { border-color: #46506a; }
  .tile.stale { border-color: #6b5320; }
  .grid .cover { position: relative; display: grid; place-items: center; height: 220px; padding: 14px; border: none; border-radius: 12px 12px 0 0;
    background: radial-gradient(ellipse at 50% 30%, #2b3040, #1a1d25); overflow: hidden; }
  /* Fixed size: a % height inside the button doesn't resolve, so the full-size image would show cropped to its top left. */
  .grid .cover img { width: 137px; height: 192px; object-fit: cover; border-radius: 7px; box-shadow: 0 8px 22px rgba(0, 0, 0, 0.5);
    transition: transform 0.15s; }
  .grid .tile:hover .cover img { transform: translateY(-3px) scale(1.02); }
  .nocover { font-size: 28px; font-weight: 700; color: var(--muted); text-transform: uppercase; }
  .pill { position: absolute; top: 10px; left: 10px; font-size: 11px; font-weight: 600; padding: 2px 8px; border-radius: 10px;
    background: rgba(12, 14, 18, 0.85); border: 1px solid var(--line); }
  .pill.ok { color: var(--ok); border-color: #1f6b4b; }
  .pill.warn { color: var(--warn); border-color: #6b5320; }
  .body { padding: 10px 12px 4px; min-width: 0; display: flex; flex-direction: column; gap: 2px; }
  .name { background: none; border: none; padding: 0; font-size: 15px; font-weight: 600; text-align: left; white-space: nowrap; overflow: hidden;
    text-overflow: ellipsis; }
  .name:hover { color: var(--accent); }
  .sub { font-size: 12px; color: var(--muted); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .facts { display: flex; flex-wrap: wrap; gap: 4px 10px; font-size: 12px; color: var(--muted); margin-top: 4px; }
  .actions { display: flex; gap: 6px; padding: 8px 12px 12px; margin-top: auto; }
  .actions > button { flex: 1; }
  .menu { bottom: calc(100% + 4px); min-width: 180px; }
  .menu .id { padding: 2px 6px 4px; font-family: Consolas, monospace; overflow: hidden; text-overflow: ellipsis; }

  /* List: compact rows for many sets. */
  .rows { display: flex; flex-direction: column; gap: 6px; }
  .rows .tile { flex-direction: row; align-items: center; gap: 12px; padding: 6px 10px; border-radius: 8px; }
  .rows .cover { width: 40px; height: 56px; padding: 0; border: none; background: none; flex: none; }
  .rows .cover img { width: 40px; height: 56px; object-fit: cover; border-radius: 4px; }
  .rows .nocover { font-size: 13px; }
  .rows .body { flex: 1; padding: 0; flex-direction: row; align-items: baseline; gap: 14px; }
  .rows .name { flex: 0 1 320px; }
  .rows .sub { flex: 0 1 300px; }
  .rows .facts { margin-top: 0; flex-wrap: nowrap; }
  .rows .actions { padding: 0; margin: 0; }
  .rows .actions > button { flex: none; }
  .rows .menu { bottom: auto; top: calc(100% + 4px); }

  /* Loading placeholders and the empty page. */
  .skeleton { animation: pulse 1.2s ease-in-out infinite; }
  .skeleton .cover { height: 220px; border-radius: 12px 12px 0 0; background: var(--panel-2); }
  .skeleton .bar { height: 12px; background: var(--panel-2); border-radius: 4px; margin: 6px 0; }
  .skeleton .bar.short { width: 60%; }
  .skeleton .body { padding: 12px; }
  @keyframes pulse { 50% { opacity: 0.55; } }
  .empty { margin: 12vh auto 0; max-width: 460px; text-align: center; display: flex; flex-direction: column; gap: 10px; }
  .emptyglyph { font-size: 44px; }
  .empty h1 { font-size: 22px; }
  .empty p { margin: 0; }
  .center { justify-content: center; margin-top: 6px; }
</style>
