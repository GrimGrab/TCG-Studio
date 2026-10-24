<script lang="ts">
  // Catalog: every set, accessory, furniture piece and decoration in this workspace (whatever setup it's in), which setups use it, and
  // deleting — which removes it everywhere: from the catalog, every setup that has it, the game (active setup) and its
  // shared files. "Add to this setup" copies its game info into the active setup; Export… writes the selection to an item
  // file (Import → "TCG Studio file" reads it). The tabs are the content registry (App.ContentKinds).
  import { onMount } from 'svelte';
  import { accFile, templateFile } from '../lib/urls';
  import { App, errText, ask, projectFile } from '../lib/api';
  import { grouped, groupOf, sourceGames, contentKinds, OWN_GROUP } from '../lib/catalog';
  import { matchSearch } from '../lib/search';

  let { notify }: { notify: (t: string, k?: string) => void } = $props();

  let kinds = $state<any[]>([]);
  let kind = $state('set');
  let lists = $state<Record<string, any[]>>({});
  let loading = $state(true);
  let busy = $state('');
  let filter = $state('');
  let orphans = $state(false); // only what no setup uses (cleanup)
  let games = $state<Record<string, string>>({});
  let picked = $state<string[]>([]);
  let anchor = '';
  // View, remembered: grid of tiles or compact rows; folded groups per kind ("set:Magic: The Gathering").
  const saved = (() => { try { return JSON.parse(localStorage.getItem('catalog.view') || '{}'); } catch { return {}; } })();
  let layout = $state<'grid' | 'list'>(saved.layout ?? 'grid');
  let folded = $state<Record<string, boolean>>(saved.folded ?? {});
  $effect(() => { try { localStorage.setItem('catalog.view', JSON.stringify({ layout, folded })); } catch {} });
  const foldKey = (g: string) => `${kind}:${g || OWN_GROUP}`;
  function toggleFold(g: string) {
    const k = foldKey(g);
    if (folded[k]) delete folded[k];
    else folded[k] = true;
  }
  function foldAll(on: boolean) {
    const next = { ...folded };
    for (const [g] of groups) { if (on) next[foldKey(g)] = true; else delete next[foldKey(g)]; }
    folded = next;
  }
  const total = $derived(Object.values(lists).reduce((n, l) => n + l.length, 0));
  const unused = $derived((lists[kind] ?? []).filter((e) => !e.usedBy?.length).length);

  async function load() {
    loading = true;
    try {
      if (!kinds.length) kinds = await contentKinds();
      const all = await Promise.all(kinds.map((k) => App.CatalogAll(k.id)));
      lists = Object.fromEntries(kinds.map((k, i) => [k.id, all[i]]));
    } catch (e) { notify(errText(e), 'error'); }
    loading = false;
    picked = picked.filter((k) => (lists[kind] ?? []).some((e) => e.key === k));
  }

  // Fallback icons: an accessory or furniture piece without its own shop icon shows the vanilla item it is built on
  // (as its own page does), from the game files.
  let accIcons = $state<Record<string, string>>({});   // vanilla accessory type → icon file
  let accDefault = $state<Record<string, string>>({}); // accessory kind → its first base
  let furnIcons = $state<Record<string, string>>({});  // vanilla furniture base → icon file
  let furnDefault = $state<Record<string, string>>({}); // furniture type → its default base
  async function loadVanillaIcons() {
    try {
      const a = await App.AccessoryTemplates();
      accIcons = Object.fromEntries(((a ? JSON.parse(a).items : []) ?? []).filter((i: any) => i.icon).map((i: any) => [i.type, i.icon]));
      accDefault = Object.fromEntries((await App.AccessoryKinds()).map((k: any) => [k.kind, k.bases?.[0] ?? '']));
    } catch { /* no game templates: no fallback */ }
    try {
      const f = await App.FurnitureTemplates();
      furnIcons = Object.fromEntries(((f ? JSON.parse(f).pieces : []) ?? []).filter((p: any) => p.icon).map((p: any) => [p.base, p.icon]));
      furnDefault = Object.fromEntries((await App.FurnitureTypes()).map((t: any) => [t.type, t.defaultBase ?? '']));
    } catch { /* no game templates: no fallback */ }
  }

  onMount(() => {
    load();
    sourceGames().then((g) => (games = g));
    loadVanillaIcons();
  });

  const shown = $derived((lists[kind] ?? []).filter((e) => {
    if (orphans && e.usedBy?.length) return false;
    return matchSearch(filter, e.name, e.id, e.origin, e.author, e.sub, groupOf(e, games), ...(e.usedBy ?? []).map((u: any) => u.name));
  }));
  const groups = $derived(grouped(shown, games));
  const flat = $derived(groups.filter(([g]) => !folded[foldKey(g)]).flatMap(([, l]) => l)); // what's on screen (Ctrl+A, Shift ranges)

  function switchTab(k: string) { kind = k; picked = []; anchor = ''; }

  /** Click: select; Ctrl: toggle; Shift: range (in the order shown). */
  function click(ev: MouseEvent, e: any) {
    if (ev.ctrlKey || ev.metaKey) {
      picked = picked.includes(e.key) ? picked.filter((k) => k !== e.key) : [...picked, e.key];
      anchor = e.key;
      return;
    }
    if (ev.shiftKey && anchor) {
      const a = flat.findIndex((x) => x.key === anchor), b = flat.findIndex((x) => x.key === e.key);
      if (a >= 0 && b >= 0) { picked = flat.slice(Math.min(a, b), Math.max(a, b) + 1).map((x) => x.key); return; }
    }
    picked = [e.key];
    anchor = e.key;
  }

  function onKey(ev: KeyboardEvent) {
    if (ev.key === 'Delete' && picked.length && !busy) { ev.preventDefault(); remove(); }
    if (ev.key === 'a' && (ev.ctrlKey || ev.metaKey)) { ev.preventDefault(); picked = flat.map((x) => x.key); }
    if (ev.key === 'Escape') picked = [];
  }

  const noun = (k: string, n: number) => { const x = kinds.find((y) => y.id === k); return x ? (n === 1 ? x.one : x.many) : k; };

  /** Writes the selection to an item file (content converted from another mod is left out). */
  async function exportPicked() {
    const items = (lists[kind] ?? []).filter((e) => picked.includes(e.key));
    if (!items.length) return;
    const converted = items.filter((e) => e.converted);
    if (converted.length === items.length) {
      notify(`Content converted from another mod can't be shared: its art belongs to the mod's authors.`, 'error');
      return;
    }
    busy = 'Exporting…';
    try {
      const name = items.length === 1 ? items[0].name : `${items.length} ${noun(kind, items.length)}`;
      const r: any = await App.ExportItems(kind, items.map((e) => e.key), name);
      if (r.file) notify(`Exported ${items.length - (r.skipped?.length ?? 0)} ${noun(kind, items.length)} to ${r.file}` +
        (r.skipped?.length ? ` (left out, converted from another mod: ${r.skipped.join(', ')})` : '') + '.', 'ok');
    } catch (e) { notify(errText(e), 'error'); }
    busy = '';
  }

  async function remove() {
    const items = (lists[kind] ?? []).filter((e) => picked.includes(e.key));
    if (!items.length) return;
    const used = items.filter((e) => e.usedBy?.length);
    const lines = items.slice(0, 12).map((e) => `• ${e.name}${e.usedBy?.length ? ` — used by ${e.usedBy.map((u: any) => u.name).join(', ')}` : ''}`);
    if (items.length > 12) lines.push(`• … and ${items.length - 12} more`);
    if (!(await ask(
      `Delete ${items.length} ${noun(kind, items.length)} everywhere?\n\n${lines.join('\n')}\n\n` +
      (used.length ? `⚠ ${used.length === items.length ? 'They are' : 'Some are'} in use: deleting removes them from those setups too` +
        (kind === 'set' ? ', and from the game if installed in the active setup' : '') + `. ` : '') +
      `Their shared files are deleted when nothing else uses them. Player saves keep their data (re-importing brings it back).\n\nThis can't be undone.`))) return;
    busy = 'Deleting…';
    try {
      const r: any = await App.DeleteFromCatalog(kind, items.map((e) => e.key));
      notify(`Deleted ${items.length} ${noun(kind, items.length)}${r.setups?.length ? ` (from ${r.setups.length} setup${r.setups.length === 1 ? '' : 's'})` : ''}.`, 'ok');
      picked = [];
    } catch (e) { notify(errText(e), 'error'); }
    busy = '';
    await load();
  }

  async function addHere(e: any) {
    busy = 'Adding…';
    try {
      const r: any = await App.AddFromCatalog(kind, [e.key]);
      if (r.added?.length) notify(`Added “${e.name}” to this setup${kind === 'set' ? ' — Install it on the Sets page to play' : ''}.`, 'ok');
      else if (r.skipped?.length) notify(`This setup already has “${e.name}”.`, 'error');
    } catch (err) { notify(errText(err), 'error'); }
    busy = '';
    await load();
  }

  function thumb(e: any): string {
    if (kind === 'set') return e.cover ? projectFile(e.id, e.cover) : '';
    if (e.icon) return accFile(e.icon);
    if (kind === 'accessory') { const i = accIcons[e.base || accDefault[e.sub]]; return i ? templateFile('acc', i) : ''; }
    if (kind === 'furniture') { const i = furnIcons[e.base || furnDefault[e.sub]]; return i ? templateFile('furn', i) : ''; }
    return '';
  }
</script>

<div class="page" role="listbox" tabindex="-1" onkeydown={onKey}>
  <header class="page-head">
    <div class="title">
      <h2>Catalog</h2>
      <div class="page-summary">
        <span><b>{total.toLocaleString()}</b> things in this workspace</span>
        <span>every setup's sets, accessories, furniture and decorations</span>
      </div>
    </div>
    <span class="grow"></span>
    <button onclick={load} disabled={loading || !!busy}>Refresh</button>
  </header>

  <nav class="tabbar">
    {#each kinds as t (t.id)}
      <button class:on={kind === t.id} onclick={() => switchTab(t.id)}>{t.title}<span class="n">{(lists[t.id] ?? []).length}</span></button>
    {/each}
  </nav>

  <div class="toolbar">
    <input class="search" type="search" placeholder="Search name, mod, author or setup…" bind:value={filter} />
    <div class="seg" title="Show everything, or only what no setup uses (to clean up: then Ctrl+A and Delete)">
      <button class:on={!orphans} onclick={() => { orphans = false; picked = []; }}>All</button>
      <button class:on={orphans} onclick={() => { orphans = true; picked = []; }}>Not used by any setup <span class="cnt">{unused}</span></button>
    </div>
    <span class="grow"></span>
    {#if groups.length > 1}
      <div class="seg" title="Fold or open every group">
        <button onclick={() => foldAll(true)}>Collapse all</button>
        <button onclick={() => foldAll(false)}>Expand all</button>
      </div>
    {/if}
    <div class="seg" title="Layout">
      <button class:on={layout === 'grid'} onclick={() => (layout = 'grid')} aria-label="Grid">▦</button>
      <button class:on={layout === 'list'} onclick={() => (layout = 'list')} aria-label="List">☰</button>
    </div>
  </div>
  <div class="hint muted small">Click to select, Ctrl/Shift+click for several, Ctrl+A for all shown. <b>Delete everywhere</b> removes
    things from every setup that has them; <b>Add to this setup</b> copies one into the setup you're in.</div>

  <div class="main">
    {#if busy && !picked.length}<div class="notice info">{busy}</div>{/if}
    {#if loading && !(lists[kind] ?? []).length}
      <div class="grid">{#each Array(8) as _}<div class="tile skeleton"><div class="art"></div><div class="body"><div class="bar"></div><div class="bar short"></div></div></div>{/each}</div>
    {:else if !shown.length}
      <div class="empty">
        <h1>{filter ? 'Nothing matches' : orphans ? 'Nothing to clean up' : `No ${noun(kind, 2)} yet`}</h1>
        <p class="muted">{filter ? 'Try another search.' : orphans ? `Every ${noun(kind, 1)} is used by a setup.` : 'Everything you import or make shows up here.'}</p>
      </div>
    {/if}
    {#each groups as [g, items] (g)}
      <button class="section" class:folded={folded[foldKey(g)]} onclick={() => toggleFold(g)} aria-expanded={!folded[foldKey(g)]}>
        <span class="chev">{folded[foldKey(g)] ? '▸' : '▾'}</span><h3>{g || OWN_GROUP}</h3>
        <span class="muted small">{items.length} {noun(kind, items.length)}</span>
      </button>
      {#if !folded[foldKey(g)]}
        <div class:grid={layout === 'grid'} class:rows={layout === 'list'}>
          {#each items as e (e.key)}
            <div class="tile" class:sel={picked.includes(e.key)} role="option" aria-selected={picked.includes(e.key)} tabindex="-1"
              onclick={(ev) => click(ev, e)} onkeydown={() => {}}>
              <div class="art" class:card={kind === 'set'}>
                {#if thumb(e)}<img src={thumb(e)} alt="" loading="lazy" onerror={(ev) => ((ev.currentTarget as HTMLImageElement).style.display = 'none')} />
                {:else}<span class="noart">{(e.name || e.id).slice(0, 2)}</span>{/if}
                {#if layout === 'grid'}
                  {#if e.here}<span class="pill ok">In this setup</span>{:else if !e.usedBy?.length}<span class="pill warn">Unused</span>{/if}
                {/if}
              </div>
              <div class="body">
                <div class="name" title={e.name || e.id}>{e.name || e.id}</div>
                <div class="sub">{e.detail}{e.key !== e.id ? ' · own art' : ''}{e.author ? ` · by ${e.author}` : ''}</div>
                <div class="used">
                  {#if e.usedBy?.length}
                    {#each e.usedBy as u}<span class="chip" title="Used by setup {u.name}">{u.name}</span>{/each}
                  {:else}<span class="muted small">not in any setup</span>{/if}
                </div>
              </div>
              <div class="actions">
                {#if e.here}{#if layout === 'list'}<span class="badge ok">In this setup</span>{/if}
                {:else}<button class="small" disabled={!!busy} onclick={(ev) => { ev.stopPropagation(); addHere(e); }}>Add to this setup</button>{/if}
              </div>
            </div>
          {/each}
        </div>
      {/if}
    {/each}
  </div>

  <!-- Stays at the bottom of the window while you scroll and select. -->
  {#if picked.length}
    <div class="selbar">
      <span class="grow"><b>{picked.length}</b> selected{#if busy} · <span class="muted">{busy}</span>{/if}</span>
      <button onclick={() => (picked = [])}>Clear</button>
      <button onclick={exportPicked} disabled={!!busy} title="Save the selection as a file to share; others add it with Import → TCG Studio file">Export…</button>
      <button class="danger" onclick={remove} disabled={!!busy}>Delete everywhere…</button>
    </div>
  {/if}
</div>

<style>
  .page { display: flex; flex-direction: column; height: 100%; overflow: hidden; outline: none; }
  .page-head { padding-bottom: 12px; border-bottom: none; }
  .tabbar { padding: 0 24px; }
  .cnt { font-size: 11px; background: var(--panel-2); border-radius: 8px; padding: 0 6px; margin-left: 6px; color: var(--muted); }
  .toolbar { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; padding: 12px 24px 6px; background: var(--panel); }
  .toolbar .search { width: 300px; }
  .hint { padding: 0 24px 10px; background: var(--panel); border-bottom: 1px solid var(--line); }
  .main { flex: 1; min-height: 0; overflow: auto; padding: 16px 24px 24px; }

  .section { display: flex; align-items: baseline; gap: 10px; width: 100%; margin: 20px 0 10px; padding: 0 0 6px; text-align: left; background: none;
    border: none; border-bottom: 1px solid var(--line); border-radius: 0; }
  .section:hover:not(:disabled) { border-color: var(--line); }
  .section:hover h3 { color: var(--accent); }
  .section:first-of-type { margin-top: 0; }
  .section h3 { color: var(--text); font-size: 13px; }
  .chev { width: 12px; color: var(--muted); font-size: 12px; }

  /* Grid: tiles with a big thumbnail on a dark stage (fixed image sizes: a % size inside the box doesn't resolve). */
  .grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(190px, 1fr)); gap: 14px; }
  .tile { position: relative; display: flex; flex-direction: column; background: var(--panel); border: 1px solid var(--line); border-radius: 12px;
    cursor: pointer; user-select: none; transition: border-color 0.12s; }
  .tile:hover { border-color: #46506a; }
  .tile.sel { border-color: var(--accent); box-shadow: 0 0 0 1px var(--accent); }
  .grid .art { position: relative; height: 150px; display: grid; place-items: center; border-radius: 12px 12px 0 0;
    background: radial-gradient(ellipse at 50% 30%, #2b3040, #1a1d25); overflow: hidden; }
  .grid .art img { width: 120px; height: 120px; object-fit: contain; }
  .grid .art.card { height: 190px; }
  .grid .art.card img { width: 116px; height: 162px; object-fit: cover; border-radius: 6px; box-shadow: 0 6px 18px rgba(0, 0, 0, 0.5); }
  .noart { font-size: 22px; font-weight: 700; color: var(--muted); text-transform: uppercase; }
  .rows .noart { font-size: 13px; }
  .pill { position: absolute; top: 8px; left: 8px; font-size: 11px; font-weight: 600; padding: 2px 8px; border-radius: 10px;
    background: rgba(12, 14, 18, 0.85); border: 1px solid var(--line); }
  .pill.ok { color: var(--ok); border-color: #1f6b4b; }
  .pill.warn { color: var(--warn); border-color: #6b5320; }
  .body { padding: 10px 12px 4px; min-width: 0; display: flex; flex-direction: column; gap: 3px; }
  .name { font-weight: 600; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .sub { font-size: 12px; color: var(--muted); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .used { display: flex; flex-wrap: wrap; gap: 4px; margin-top: 2px; }
  .chip { font-size: 11px; border: 1px solid var(--line); border-radius: 8px; padding: 0 6px; background: var(--panel-2); }
  .actions { padding: 8px 12px 12px; margin-top: auto; display: flex; }
  .actions button { flex: 1; }

  /* List: compact rows. */
  .rows { display: flex; flex-direction: column; gap: 4px; }
  .rows .tile { flex-direction: row; align-items: center; gap: 12px; padding: 6px 10px; border-radius: 8px; }
  .rows .art { width: 44px; height: 44px; flex: none; display: grid; place-items: center; background: var(--bg); border-radius: 6px; overflow: hidden; }
  .rows .art.card { width: 40px; height: 56px; }
  .rows .art img { width: 40px; height: 40px; object-fit: contain; }
  .rows .art.card img { width: 40px; height: 56px; object-fit: cover; }
  .rows .body { flex: 1; padding: 0; }
  .rows .actions { padding: 0; margin: 0; }
  .rows .actions button { flex: none; }

  .skeleton { animation: pulse 1.2s ease-in-out infinite; cursor: default; }
  .skeleton .art { height: 150px; border-radius: 12px 12px 0 0; background: var(--panel-2); }
  .skeleton .bar { height: 12px; background: var(--panel-2); border-radius: 4px; margin: 6px 0; }
  .skeleton .bar.short { width: 60%; }
  .skeleton .body { padding: 12px; }
  @keyframes pulse { 50% { opacity: 0.55; } }
  .empty { margin: 12vh auto 0; max-width: 440px; text-align: center; display: flex; flex-direction: column; gap: 8px; }
  .empty h1 { font-size: 20px; }
  .empty p { margin: 0; }
  .selbar { display: flex; gap: 8px; align-items: center; padding: 10px 24px; border-top: 1px solid var(--accent); background: var(--panel-2);
    box-shadow: 0 -6px 18px rgba(0, 0, 0, 0.35); }
</style>
