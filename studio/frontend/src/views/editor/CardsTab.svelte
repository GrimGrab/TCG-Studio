<script lang="ts">
  import { App, projectFile, money, setRarities, rarityName, rarityColor, errText, ask, sourceInfo } from '../../lib/api';
  import { matchSearch } from '../../lib/search';
  import CardDetail from './CardDetail.svelte';
  import BulkEdit from './BulkEdit.svelte';
  import CardViewer from './CardViewer.svelte';

  let { project, notify, imgBust }: {
    project: any; notify: (t: string, k?: string) => void; imgBust: number;
  } = $props();

  let query = $state('');
  let rarity = $state('');
  let color = $state('');
  let sort = $state('set');
  let size = $state(150);
  let selected = $state<string[]>([]);
  let anchor = $state<number>(-1);

  const meta = (id: string) => project.meta?.cards?.[id] ?? {};
  const rarities = $derived(setRarities(project.set));
  const RANK = $derived<Record<string, number>>(Object.fromEntries(rarities.map((r, i) => [r.id, i])));
  // Sets with their own rarities name them on the card already; the source rarity filter/sort only adds to the game's 4.
  const ownRarities = $derived(!!project.set.rarities?.length);

  // The import source decides the colour filter (Magic colours, Pokémon types) and the order of its own rarities.
  let source = $state<any>(null);
  $effect(() => {
    const id = project.meta?.source;
    sourceInfo(id).then((s) => { if (project.meta?.source === id) source = s; });
  });
  const colors = $derived<any[]>(source?.colors ?? []);
  // A set's own order (image-folder imports: its rarity subfolders) beats the source's fixed list.
  const srcOrder = $derived<string[]>(project.meta?.rarityOrder?.length ? project.meta.rarityOrder : source?.rarityOrder ?? []);
  // Unknown source rarities rank in the middle of the ladder.
  const srcRank = (r: string | undefined) => {
    const i = r ? srcOrder.indexOf(r) : -1;
    return i >= 0 ? i : (srcOrder.length - 1) / 2 + 0.25;
  };
  const srcRarities = $derived(
    ownRarities ? [] :
    [...new Set<string>(project.set.cards.map((c: any) => meta(c.id).srcRarity).filter(Boolean))].sort((a, b) => srcRank(a) - srcRank(b))
  );
  const setIndex = $derived(new Map(project.set.cards.map((c: any, i: number) => [c.id, i])));
  const bySet = (a: any, b: any) => (setIndex.get(a.id) as number) - (setIndex.get(b.id) as number);
  // Extra sorts from the source: "cost" low first, "power" high first (cards without one go last), and its colour/type order.
  const sorts = $derived<any[]>(source?.sorts ?? []);
  const num = (v: any) => (v === undefined || v === null || v === '' || isNaN(Number(v)) ? null : Number(v));
  function colorIndex(m: any): number {
    if (m.colors?.length > 1) {
      const multi = colors.findIndex((f) => f.value === 'M');
      if (multi >= 0) return multi;
    }
    const i = colors.findIndex((f) => f.value !== 'M' && colorMatch(m, f.value));
    return i >= 0 ? i : colors.length;
  }

  function colorMatch(m: any, v: string): boolean {
    if (v === 'C') return !m.colors?.length && !(m.typeLine ?? '').match(/^(Trainer|Energy)/);
    if (v === 'M') return m.colors?.length > 1;
    return !!m.colors?.includes(v) || (m.typeLine ?? '').startsWith(v);
  }

  const shown = $derived.by(() => {
    const q = query.trim().toLowerCase();
    let list = project.set.cards.filter((c: any) => {
      const m = meta(c.id);
      if (rarity.startsWith('src:') ? m.srcRarity !== rarity.slice(4) : rarity && c.rarity !== rarity) return false;
      if (color && !colorMatch(m, color)) return false;
      if (!q) return true;
      return matchSearch(q, c.name, c.id, c.number, m.typeLine, c.description);
    });
    if (sort === 'name') list = [...list].sort((a: any, b: any) => a.name.localeCompare(b.name));
    if (sort === 'rarity')
      list = [...list].sort((a: any, b: any) => (RANK[b.rarity] ?? -1) - (RANK[a.rarity] ?? -1) ||
        (ownRarities ? 0 : srcRank(meta(b.id).srcRarity) - srcRank(meta(a.id).srcRarity)) || bySet(a, b));
    if (sort === 'cost' || sort === 'power') {
      const key = sort === 'cost' ? 'cmc' : 'power';
      const dir = sort === 'cost' ? 1 : -1;
      list = [...list].sort((a: any, b: any) => {
        const x = num(meta(a.id)[key]), y = num(meta(b.id)[key]);
        if (x === null || y === null) return (x === null ? 1 : 0) - (y === null ? 1 : 0) || bySet(a, b);
        return dir * (x - y) || bySet(a, b);
      });
    }
    if (sort === 'color') list = [...list].sort((a: any, b: any) => colorIndex(meta(a.id)) - colorIndex(meta(b.id)) || bySet(a, b));
    if (sort === 'price') list = [...list].sort((a: any, b: any) => b.price.base - a.price.base);
    if (sort === 'real') list = [...list].sort((a: any, b: any) => (meta(b.id).usd ?? 0) - (meta(a.id).usd ?? 0));
    return list;
  });

  const selectedCards = $derived(project.set.cards.filter((c: any) => selected.includes(c.id)));

  function click(e: MouseEvent, index: number, id: string) {
    if (e.shiftKey && anchor >= 0) {
      const [a, b] = [Math.min(anchor, index), Math.max(anchor, index)];
      const range = shown.slice(a, b + 1).map((c: any) => c.id);
      selected = e.ctrlKey ? [...new Set([...selected, ...range])] : range;
    } else if (e.ctrlKey || e.metaKey) {
      selected = selected.includes(id) ? selected.filter((s) => s !== id) : [...selected, id];
      anchor = index;
    } else {
      selected = [id];
      anchor = index;
    }
  }

  function selectAll() { selected = shown.map((c: any) => c.id); }

  // Set order = the game's order (binder "Set order", price app); saves key cards by id, so moving them is safe.
  // Drag works in Set order only; dragging a selected card moves the whole selection, filtered or not.
  let dragIds = $state<string[]>([]);
  let dropAt = $state<{ id: string; after: boolean } | null>(null);

  function dragStart(e: DragEvent, id: string) {
    dragIds = selected.includes(id) ? project.set.cards.filter((c: any) => selected.includes(c.id)).map((c: any) => c.id) : [id];
    e.dataTransfer!.effectAllowed = 'move';
    e.dataTransfer!.setData('text/plain', id);
  }

  function dragOver(e: DragEvent, id: string) {
    if (!dragIds.length) return;
    e.preventDefault();
    if (dragIds.includes(id)) { dropAt = null; return; }
    const r = (e.currentTarget as HTMLElement).getBoundingClientRect();
    dropAt = { id, after: e.clientX > r.left + r.width / 2 };
  }

  function drop(e: DragEvent) {
    e.preventDefault();
    if (dropAt) {
      const moving = project.set.cards.filter((c: any) => dragIds.includes(c.id));
      const rest = project.set.cards.filter((c: any) => !dragIds.includes(c.id));
      rest.splice(rest.findIndex((c: any) => c.id === dropAt!.id) + (dropAt.after ? 1 : 0), 0, ...moving);
      project.set.cards = rest;
    }
    dragEnd();
  }

  function dragEnd() { dragIds = []; dropAt = null; }

  // Full-screen viewer over the shown cards (double-click a card, or click the picture in the card panel); browsing selects.
  let viewing = $state<number | null>(null);
  function view(id: string) { const i = shown.findIndex((c: any) => c.id === id); if (i >= 0) viewing = i; }
  function viewAt(i: number) { viewing = i; selected = [shown[i].id]; anchor = i; }

  // Strip leading numbers: "001 Captain Marvel.png" → card 001 "Captain Marvel" (shared with the Import page's image-folder
  // option). Off: the whole file name is the name, so "2099 Spider-Man" stays whole.
  let stripNumbers = $state((() => { try { return localStorage.getItem('import.stripNumbers') === '1'; } catch { return false; } })());
  function rememberStrip() { try { localStorage.setItem('import.stripNumbers', stripNumbers ? '1' : ''); } catch {} }

  async function addCards() {
    try {
      const images: string[] = await App.PickImages(project.id);
      const ids = new Set(project.set.cards.map((c: any) => c.id));
      const added: string[] = [];
      for (const rel of images) {
        const stem = rel.replace(/^images\//, '').replace(/\.[^.]+$/, '');
        const base = stem.toLowerCase().replace(/[\s:/|\\]+/g, '-');
        let id = base;
        for (let n = 2; ids.has(id); n++) id = `${base}-${n}`;
        ids.add(id);
        let name = stem, number = '';
        const m = stripNumbers ? stem.match(/^#?(\d{1,5}[a-zA-Z]?)(?:\s*[-_.)]\s*|\s+)(.+)$/) : null;
        if (m) { number = m[1]; name = m[2]; }
        name = name.replace(/_+/g, ' ').replace(/\s+/g, ' ').trim(); // hyphens stay (Spider-Man)
        project.set.cards.push({
          id, name, ...(number ? { number } : {}), description: '', artist: '', rarity: rarities[0]?.id ?? 'Common', image: rel,
          price: { base: 0.25 }, play: { laneAttack: [1, 1, 1, 1], element: 'Fire' }
        });
        added.push(id);
      }
      if (added.length) { selected = added; notify(`Added ${added.length} card(s)`, 'ok'); }
    } catch (e) {
      notify(errText(e), 'error');
    }
  }

  async function deleteSelected() {
    if (!(await ask(`Delete ${selected.length === 1 ? 'this card' : `${selected.length} cards`} from this set? Players who own them keep the data (it comes back if a card with the same id is re-added).`))) return;
    project.set.cards = project.set.cards.filter((c: any) => !selected.includes(c.id));
    for (const p of project.set.packs) p.cards = (p.cards ?? []).filter((id: string) => !selected.includes(id));
    selected = [];
  }

  const rarityCounts = $derived(rarities.map((r) => [r, project.set.cards.filter((c: any) => c.rarity === r.id).length] as [any, number]));
  const maxRarity = $derived(Math.max(1, ...rarityCounts.map(([, n]) => n)));
  const value = $derived(project.set.cards.reduce((s: number, c: any) => s + (c.price?.base ?? 0), 0));
  const top = $derived([...project.set.cards].sort((a: any, b: any) => b.price.base - a.price.base).slice(0, 5));
  const filtered = $derived(!!(query.trim() || rarity || color));
  function clearFilters() { query = ''; rarity = ''; color = ''; }
  /** Selects a card from the summary (clearing filters that hide it). */
  function pick(id: string) {
    if (!shown.some((c: any) => c.id === id)) clearFilters();
    selected = [id];
    anchor = -1;
  }
  // Card size, remembered between launches.
  try { const s = Number(localStorage.getItem('cards.size')); if (s >= 100 && s <= 260) size = s; } catch {}
  $effect(() => { try { localStorage.setItem('cards.size', String(size)); } catch {} });
</script>

<div class="wrap">
  <div class="left">
    <div class="toolbar">
      <input class="search" type="search" placeholder="Search name, type, text…" bind:value={query} />
      {#if colors.length}
        <select bind:value={color} title="Filter by {source.colorLabel.toLowerCase()}">
          <option value="">All {source.colorLabel.toLowerCase()}s</option>
          {#each colors as f}<option value={f.value}>{f.label}</option>{/each}
        </select>
      {/if}
      {#if srcRarities.length}
        <select value={rarity.startsWith('src:') ? rarity : ''} onchange={(e) => (rarity = e.currentTarget.value)} title="The card's own rarity from its import">
          <option value="">All card rarities</option>
          {#each srcRarities as r}<option value={'src:' + r}>{r}</option>{/each}
        </select>
      {/if}
      <span class="grow"></span>
      <label class="sortby">Sort
        <select bind:value={sort}>
          <option value="set">Set order</option><option value="name">Name</option><option value="rarity">Rarity</option>
          {#if colors.length}<option value="color">{source.colorLabel}</option>{/if}
          {#each sorts as s}<option value={s.value}>{s.label}</option>{/each}
          <option value="price">Game price</option><option value="real">Real price</option>
        </select>
      </label>
      <label class="sortby" title="Card size"><span class="sz small">▪</span><input type="range" min="100" max="260" bind:value={size} /><span class="sz">■</span></label>
    </div>
    <div class="chips">
      <button class="chip" class:on={!rarity} onclick={() => (rarity = '')}>All <span class="n">{project.set.cards.length}</span></button>
      {#each rarityCounts as [r, n]}
        <button class="chip" class:on={rarity === r.id} onclick={() => (rarity = rarity === r.id ? '' : r.id)} disabled={!n && rarity !== r.id}>
          <span class="dot" style="background:{rarityColor(project.set, r.id)}"></span>{r.name} <span class="n">{n}</span></button>
      {/each}
    </div>
    <div class="subbar">
      <span class="muted">{shown.length === project.set.cards.length ? `${shown.length} cards` : `${shown.length} of ${project.set.cards.length} shown`}{selected.length ? ` · ${selected.length} selected` : ''}</span>
      {#if filtered}<button class="small link" onclick={clearFilters}>Clear filters</button>{/if}
      <span class="grow"></span>
      <button class="small" onclick={selectAll} disabled={!shown.length}>Select all shown</button>
      <button class="small" onclick={() => (selected = [])} disabled={!selected.length}>Clear selection</button>
      <label class="check small" title="For files named with a card number first, like “001 Captain Marvel.png”: the number becomes the card's number and is left out of its name. Off: the whole file name is the name.">
        <input type="checkbox" bind:checked={stripNumbers} onchange={rememberStrip} /> Strip leading numbers</label>
      <button class="small primary" onclick={addCards}>+ Add cards from images</button>
    </div>
    <!-- Drops land on the grid (the drop line sits in the gap between cards), at the last spot a card showed. -->
    <div class="grid" style="grid-template-columns: repeat(auto-fill, minmax({size}px, 1fr))" role="list"
      ondragover={(e) => { if (dragIds.length) e.preventDefault(); }} ondrop={drop}
      ondragleave={(e) => { if (!(e.currentTarget as HTMLElement).contains(e.relatedTarget as Node)) dropAt = null; }}>
      {#each shown as c, i (c.id)}
        <button class="card" class:sel={selected.includes(c.id)} onclick={(e) => click(e, i, c.id)} ondblclick={() => view(c.id)}
          draggable={sort === 'set'} ondragstart={(e) => dragStart(e, c.id)} ondragover={(e) => dragOver(e, c.id)} ondragend={dragEnd}
          class:dragging={dragIds.includes(c.id)} class:before={dropAt?.id === c.id && !dropAt?.after} class:after={dropAt?.id === c.id && !!dropAt?.after}
          style="--rc:{rarityColor(project.set, c.rarity)}"
          title={meta(c.id).srcRarity && !ownRarities ? `${rarityName(project.set, c.rarity)} · ${meta(c.id).srcRarity}` : rarityName(project.set, c.rarity)}>
          <div class="img" style="aspect-ratio: 63/88">
            {#if c.image}<img src={projectFile(project.id, c.image, imgBust)} alt={c.name} loading="lazy" draggable="false" />
            {:else}<span class="noimg">No image</span>{/if}
            {#if selected.includes(c.id)}<span class="tick">✓</span>{/if}
            <span class="flags">
              {#if c.backImage}<span title="Double-faced card (double-click to see both sides)">⇄</span>{/if}
              {#if meta(c.id).locked}<span title="Price locked">🔒</span>{/if}
            </span>
            {#if c.number}<span class="num">#{c.number}</span>{/if}
          </div>
          <div class="cap">
            <span class="dot" style="background:var(--rc)"></span>
            <span class="nm">{c.name}</span>
          </div>
          <div class="price"><b>{money(c.price.base)}</b>{#if meta(c.id).usd !== undefined}<span class="real">real {money(meta(c.id).usd)}</span>{/if}</div>
        </button>
      {:else}
        <div class="none">
          {#if project.set.cards.length}
            <p class="muted">No card matches.</p><button onclick={clearFilters}>Clear filters</button>
          {:else}
            <div class="emptyglyph">🂠</div>
            <h1>No cards yet</h1>
            <p class="muted">Add card pictures from your computer — each image becomes a card you can name, price and give a rarity.</p>
            <button class="primary" onclick={addCards}>+ Add cards from images</button>
          {/if}
        </div>
      {/each}
    </div>
  </div>

  <aside>
    {#if selectedCards.length === 1}
      <CardDetail {project} card={selectedCards[0]} {imgBust} {notify} ondelete={deleteSelected} onzoom={() => view(selectedCards[0].id)} />
    {:else if selectedCards.length > 1}
      <BulkEdit {project} cards={selectedCards} {imgBust} {notify} ondelete={deleteSelected} />
    {:else}
      <div class="summary">
        <div class="minis">
          <div class="mini"><span class="label">Cards</span><b>{project.set.cards.length.toLocaleString()}</b></div>
          <div class="mini"><span class="label">Base value</span><b>{money(value)}</b></div>
          <div class="mini"><span class="label">Average</span><b>{money(project.set.cards.length ? value / project.set.cards.length : 0)}</b></div>
        </div>

        <div class="sec">
          <h3>Rarities</h3>
          {#each rarityCounts as [r, n]}
            <button class="rrow" onclick={() => (rarity = rarity === r.id ? '' : r.id)} title="Show only {r.name}">
              <span class="dot" style="background:{rarityColor(project.set, r.id)}"></span>
              <span class="rn">{r.name}</span>
              <span class="bar"><span style="width:{(n / maxRarity) * 100}%; background:{rarityColor(project.set, r.id)}"></span></span>
              <b>{n}</b>
            </button>
          {/each}
        </div>

        {#if top.length && top[0].price.base > 0}
          <div class="sec">
            <h3>Most valuable</h3>
            {#each top as c (c.id)}
              <button class="trow" onclick={() => pick(c.id)}>
                {#if c.image}<img src={projectFile(project.id, c.image, imgBust)} alt="" loading="lazy" />{:else}<span class="ph"></span>{/if}
                <span class="tn">{c.name}</span><b>{money(c.price.base)}</b>
              </button>
            {/each}
          </div>
        {/if}

        <div class="sec tips">
          <h3>Tips</h3>
          <p><b>Click</b> a card to edit it, <b>double-click</b> to view it full screen.</p>
          <p><b>Ctrl/Shift-click</b> or “Select all shown” to edit many at once.</p>
          <p>In <b>Set order</b>, drag cards to move them (a selection moves together). The game's binder and price app show cards in this order.</p>
        </div>
      </div>
    {/if}
  </aside>
</div>

{#if viewing !== null && shown[viewing]}
  <CardViewer {project} cards={shown} index={viewing} {imgBust} onindex={viewAt} onclose={() => (viewing = null)} />
{/if}

<style>
  .wrap { display: flex; height: 100%; }
  .left { flex: 1; min-width: 0; display: flex; flex-direction: column; }
  .toolbar { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; padding: 10px 20px; background: var(--panel); border-bottom: 1px solid var(--line); }
  .toolbar .search { width: 280px; }
  .sortby { display: flex; align-items: center; gap: 6px; font-size: 13px; color: var(--muted); }
  .sortby input[type="range"] { width: 110px; }
  .sz { font-size: 14px; line-height: 1; }
  .sz.small { font-size: 9px; }
  .chips { display: flex; flex-wrap: wrap; gap: 6px; padding: 10px 20px 0; }
  .chip { display: inline-flex; align-items: center; gap: 6px; padding: 3px 10px; border-radius: 999px; font-size: 12px; background: var(--panel); }
  .chip.on { border-color: var(--accent); background: #22304d; }
  .chip .n { color: var(--muted); font-size: 11px; }
  .subbar { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; padding: 8px 20px; font-size: 13px; }
  .grid { flex: 1; overflow: auto; display: grid; gap: 14px; padding: 4px 20px 24px; align-content: start; }
  .card { padding: 6px 6px 8px; display: flex; flex-direction: column; gap: 5px; background: var(--panel); text-align: left; user-select: none;
    border-radius: 10px; border-bottom: 2px solid var(--rc); transition: transform 0.12s, border-color 0.12s, box-shadow 0.12s; }
  .card:hover { transform: translateY(-2px); box-shadow: 0 6px 18px rgba(0, 0, 0, 0.35); }
  .card.sel { border-color: var(--accent); background: #1f2b45; box-shadow: 0 0 0 1px var(--accent); }
  .card.dragging { opacity: 0.4; }
  .card.before { box-shadow: -7px 0 0 -2px var(--accent); }
  .card.after { box-shadow: 7px 0 0 -2px var(--accent); }
  .img { position: relative; background: var(--bg); border-radius: 6px; overflow: hidden; }
  .img img { width: 100%; height: 100%; object-fit: contain; display: block; }
  .noimg { position: absolute; inset: 0; display: grid; place-items: center; color: var(--muted); font-size: 12px; }
  .tick { position: absolute; top: 6px; left: 6px; width: 20px; height: 20px; border-radius: 50%; background: var(--accent); color: #fff;
    display: grid; place-items: center; font-size: 12px; font-weight: 700; box-shadow: 0 2px 6px rgba(0, 0, 0, 0.5); }
  .flags { position: absolute; top: 5px; right: 5px; display: flex; gap: 3px; }
  .flags span { font-size: 11px; background: rgba(10, 12, 16, 0.75); color: #fff; border-radius: 4px; padding: 0 4px; }
  .num { position: absolute; bottom: 5px; right: 5px; font-size: 10px; background: rgba(10, 12, 16, 0.75); color: #ddd; border-radius: 4px; padding: 0 4px;
    font-family: Consolas, monospace; }
  .cap { display: flex; align-items: center; gap: 6px; font-size: 12px; font-weight: 600; padding: 0 2px; }
  .nm { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .dot { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; display: inline-block; }
  .price { display: flex; align-items: baseline; gap: 6px; font-size: 12px; padding: 0 2px; }
  .price b { font-weight: 600; }
  .real { color: var(--muted); font-size: 11px; }
  .none { grid-column: 1 / -1; margin: 10vh auto 0; max-width: 420px; text-align: center; display: flex; flex-direction: column; align-items: center; gap: 10px; }
  .none p { margin: 0; }
  .none h1 { font-size: 20px; }
  .emptyglyph { font-size: 44px; }

  aside { width: 380px; flex-shrink: 0; border-left: 1px solid var(--line); background: var(--panel); overflow: auto; }
  .summary { padding: 16px; display: flex; flex-direction: column; gap: 18px; }
  .minis { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 8px; }
  .mini { background: var(--bg); border: 1px solid var(--line); border-radius: 8px; padding: 8px 10px; display: flex; flex-direction: column; min-width: 0; }
  .sec { display: flex; flex-direction: column; gap: 4px; }
  .sec h3 { margin-bottom: 4px; }
  .rrow, .trow { display: flex; align-items: center; gap: 8px; text-align: left; background: none; border: 1px solid transparent; padding: 4px 6px; border-radius: 6px; }
  .rrow:hover:not(:disabled), .trow:hover:not(:disabled) { background: var(--panel-2); border-color: transparent; }
  .rn { width: 110px; flex: none; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .bar { flex: 1; height: 6px; background: var(--bg); border-radius: 3px; overflow: hidden; }
  .bar span { display: block; height: 100%; border-radius: 3px; }
  .rrow b, .trow b { min-width: 44px; text-align: right; font-variant-numeric: tabular-nums; font-weight: 600; }
  .trow img, .trow .ph { width: 26px; height: 36px; object-fit: cover; border-radius: 3px; background: var(--bg); flex: none; }
  .tn { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .tips p { margin: 0 0 4px; font-size: 12px; color: var(--muted); }
  .tips b { color: var(--text); font-weight: 600; }
</style>
