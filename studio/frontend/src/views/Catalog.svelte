<script lang="ts">
  // Catalog: every set, accessory and furniture piece in this workspace (whatever setup it's in), which setups use it, and
  // deleting — which removes it everywhere: from the catalog, every setup that has it, the game (active setup) and its
  // shared files. "Add to this setup" copies its game info into the active setup.
  import { onMount } from 'svelte';
  import { App, errText, ask, projectFile } from '../lib/api';
  import { grouped, sourceGames, OWN_GROUP } from '../lib/catalog';

  let { notify }: { notify: (t: string, k?: string) => void } = $props();

  const TABS = [{ kind: 'set', title: 'Sets' }, { kind: 'accessory', title: 'Accessories' }, { kind: 'furniture', title: 'Furniture' }];
  let kind = $state('set');
  let lists = $state<Record<string, any[]>>({});
  let loading = $state(true);
  let busy = $state('');
  let filter = $state('');
  let games = $state<Record<string, string>>({});
  let picked = $state<string[]>([]);
  let anchor = '';

  async function load() {
    loading = true;
    try {
      const [s, a, f] = await Promise.all(TABS.map((t) => App.CatalogAll(t.kind)));
      lists = { set: s, accessory: a, furniture: f };
    } catch (e) { notify(errText(e), 'error'); }
    loading = false;
    picked = picked.filter((k) => (lists[kind] ?? []).some((e) => e.key === k));
  }

  onMount(() => {
    load();
    sourceGames().then((g) => (games = g));
  });

  const shown = $derived((lists[kind] ?? []).filter((e) => {
    const q = filter.trim().toLowerCase();
    return !q || `${e.name} ${e.id} ${e.origin} ${e.author} ${e.sub} ${(e.usedBy ?? []).map((u: any) => u.name).join(' ')}`.toLowerCase().includes(q);
  }));
  const groups = $derived(grouped(shown, games));
  const flat = $derived(groups.flatMap(([, l]) => l));

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

  const noun = (k: string, n: number) => k === 'set' ? (n === 1 ? 'set' : 'sets') : k === 'accessory' ? (n === 1 ? 'accessory' : 'accessories') : (n === 1 ? 'furniture piece' : 'furniture pieces');

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

  const thumb = (e: any) => kind === 'set'
    ? (e.cover ? projectFile(e.id, e.cover) : '')
    : (e.icon ? `/acc/${e.icon.split('/').map(encodeURIComponent).join('/')}` : '');
</script>

<div class="page" role="listbox" tabindex="-1" onkeydown={onKey}>
  <header class="row">
    <h2 class="grow">Catalog</h2>
    <button onclick={load} disabled={loading || !!busy}>Refresh</button>
  </header>
  <p class="muted intro">Everything in this workspace — every set, accessory and furniture piece in any of your setups — and which
    setups use it. Add one to the setup you're in, or delete it: deleting removes it <b>everywhere</b>. Click to select;
    Ctrl/Shift+click for several; <b>Delete</b> removes the selection.</p>

  <div class="row tabs">
    {#each TABS as t}
      <button class:on={kind === t.kind} onclick={() => switchTab(t.kind)}>{t.title} <span class="muted small">({(lists[t.kind] ?? []).length})</span></button>
    {/each}
    <div class="grow"></div>
    <input class="search" placeholder="Search name, mod, author, setup" bind:value={filter} />
  </div>

  {#if picked.length}
    <div class="row selbar">
      <span class="grow">{picked.length} selected</span>
      <button class="small" onclick={() => (picked = [])}>Clear</button>
      <button class="small danger" onclick={remove} disabled={!!busy}>Delete everywhere…</button>
    </div>
  {/if}
  {#if busy}<p class="muted small">{busy}</p>{/if}

  <div class="list">
    {#if loading && !(lists[kind] ?? []).length}<p class="muted">Loading…</p>
    {:else if !shown.length}<p class="muted">{filter ? 'Nothing matches.' : `No ${noun(kind, 2)} yet.`}</p>{/if}
    {#each groups as [g, items] (g)}
      <div class="group">
        <div class="ghead">{g || OWN_GROUP} <span class="muted small">· {items.length}</span></div>
        {#each items as e (e.key)}
          <div class="item" class:sel={picked.includes(e.key)} role="option" aria-selected={picked.includes(e.key)} tabindex="-1"
            onclick={(ev) => click(ev, e)} onkeydown={() => {}}>
            <div class="thumb" class:card={kind === 'set'}>{#if thumb(e)}<img src={thumb(e)} alt="" loading="lazy" onerror={(ev) => ((ev.currentTarget as HTMLImageElement).style.display = 'none')} />{/if}</div>
            <div class="grow info">
              <div class="name">{e.name || e.id}{#if e.author} <span class="muted small">by {e.author}</span>{/if}</div>
              <div class="muted small">{e.detail}{e.key !== e.id ? ' · own art' : ''}</div>
              <div class="used">
                {#if e.usedBy?.length}
                  {#each e.usedBy as u}<span class="chip">{u.name}</span>{/each}
                {:else}<span class="muted small">not in any setup</span>{/if}
              </div>
            </div>
            {#if e.here}<span class="muted small">in this setup</span>
            {:else}<button class="small" disabled={!!busy} onclick={(ev) => { ev.stopPropagation(); addHere(e); }}>Add to this setup</button>{/if}
          </div>
        {/each}
      </div>
    {/each}
  </div>
</div>

<style>
  .page { padding: 20px 24px; overflow: auto; height: 100%; display: flex; flex-direction: column; gap: 10px; outline: none; }
  .intro { max-width: 900px; margin: 0; }
  .tabs { gap: 6px; align-items: center; }
  .tabs button.on { border-color: var(--accent); }
  .search { width: 280px; }
  .selbar { gap: 8px; align-items: center; padding: 6px 10px; border: 1px solid var(--accent); border-radius: var(--radius); background: var(--panel); max-width: 980px; }
  .list { display: flex; flex-direction: column; gap: 12px; max-width: 980px; }
  .group { display: flex; flex-direction: column; gap: 4px; }
  .ghead { font-weight: 600; font-size: 13px; padding: 4px 2px; border-bottom: 1px solid var(--line); }
  .item { display: flex; gap: 12px; align-items: center; padding: 6px 8px; border: 1px solid transparent; border-radius: var(--radius);
    cursor: pointer; user-select: none; }
  .item:hover { background: var(--panel); }
  .item.sel { border-color: var(--accent); background: var(--panel); }
  .thumb { width: 44px; height: 44px; flex: none; display: flex; align-items: center; justify-content: center; background: var(--bg);
    border-radius: 6px; overflow: hidden; }
  .thumb.card { height: 60px; }
  .thumb img { max-width: 100%; max-height: 100%; object-fit: contain; }
  .info { min-width: 0; display: flex; flex-direction: column; gap: 2px; }
  .name { font-weight: 600; }
  .used { display: flex; flex-wrap: wrap; gap: 4px; }
  .chip { font-size: 11px; border: 1px solid var(--line); border-radius: 8px; padding: 0 6px; }
  .small { font-size: 12px; }
</style>
