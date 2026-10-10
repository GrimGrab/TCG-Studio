<script lang="ts">
  import { App, projectFile, money, setRarities, rarityName, rarityColor, errText, ask, sourceInfo } from '../../lib/api';
  import CardDetail from './CardDetail.svelte';
  import BulkEdit from './BulkEdit.svelte';
  import CardViewer from './CardViewer.svelte';

  let { project, notify, imgBust, onimages }: {
    project: any; notify: (t: string, k?: string) => void; imgBust: number; onimages: (what: string) => Promise<void>;
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
      return c.name.toLowerCase().includes(q) || c.id.includes(q) || (m.typeLine ?? '').toLowerCase().includes(q) || (c.description ?? '').toLowerCase().includes(q);
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
</script>

<div class="wrap">
  <div class="left">
    <div class="filters row">
      <input class="grow" placeholder="Search name, type, text…" bind:value={query} />
      <select bind:value={rarity}>
        <option value="">All rarities</option>
        {#if srcRarities.length}
          <optgroup label="Game rarity">{#each rarities as r}<option value={r.id}>{r.name}</option>{/each}</optgroup>
          <optgroup label="Card rarity">{#each srcRarities as r}<option value={'src:' + r}>{r}</option>{/each}</optgroup>
        {:else}
          {#each rarities as r}<option value={r.id}>{r.name}</option>{/each}
        {/if}
      </select>
      {#if colors.length}
        <select bind:value={color}>
          <option value="">All {source.colorLabel.toLowerCase()}s</option>
          {#each colors as f}<option value={f.value}>{f.label}</option>{/each}
        </select>
      {/if}
      <select bind:value={sort}>
        <option value="set">Set order</option><option value="name">Name</option><option value="rarity">Rarity</option>
        {#if colors.length}<option value="color">{source.colorLabel}</option>{/if}
        {#each sorts as s}<option value={s.value}>{s.label}</option>{/each}
        <option value="price">Game price</option><option value="real">Real price</option>
      </select>
      <input type="range" min="100" max="260" bind:value={size} title="Card size" />
    </div>
    <div class="subbar row">
      <span class="muted grow">{shown.length} shown · {selected.length} selected</span>
      <button class="small" onclick={selectAll}>Select all shown</button>
      <button class="small" onclick={() => (selected = [])} disabled={!selected.length}>Clear</button>
      <button class="small" onclick={addCards}>+ Add cards from images</button>
      <label class="check small" title="For files named with a card number first, like “001 Captain Marvel.png”: the number becomes the card's number and is left out of its name. Off: the whole file name is the name.">
        <input type="checkbox" bind:checked={stripNumbers} onchange={rememberStrip} /> Strip leading numbers</label>
    </div>
    <div class="grid" style="grid-template-columns: repeat(auto-fill, minmax({size}px, 1fr))">
      {#each shown as c, i (c.id)}
        <button class="card" class:sel={selected.includes(c.id)} onclick={(e) => click(e, i, c.id)} ondblclick={() => view(c.id)}
          title={meta(c.id).srcRarity && !ownRarities ? `${rarityName(project.set, c.rarity)} · ${meta(c.id).srcRarity}` : rarityName(project.set, c.rarity)}>
          <div class="img" style="aspect-ratio: 63/88">
            {#if c.image}<img src={projectFile(project.id, c.image, imgBust)} alt={c.name} loading="lazy" draggable="false" />{/if}
            {#if meta(c.id).locked}<span class="lock" title="Price locked">🔒</span>{/if}
            {#if c.backImage}<span class="dfc" title="Double-faced card (double-click to see both sides)">⇄</span>{/if}
          </div>
          <div class="cap">
            <span class="dot" style="background:{rarityColor(project.set, c.rarity)}"></span>
            <span class="nm">{c.name}</span>
          </div>
          <div class="price muted">{money(c.price.base)}{#if meta(c.id).usd !== undefined}<span class="real"> · real {money(meta(c.id).usd)}</span>{/if}</div>
        </button>
      {/each}
    </div>
  </div>

  <aside>
    {#if selectedCards.length === 1}
      <CardDetail {project} card={selectedCards[0]} {imgBust} {notify} {onimages} ondelete={deleteSelected} onzoom={() => view(selectedCards[0].id)} />
    {:else if selectedCards.length > 1}
      <BulkEdit {project} cards={selectedCards} {notify} {onimages} ondelete={deleteSelected} />
    {:else}
      <div class="summary">
        <h3>Set summary</h3>
        {#each rarityCounts as [r, n]}
          <div class="row"><span class="dot" style="background:{rarityColor(project.set, r.id)}"></span><span class="grow">{r.name}</span><b>{n}</b></div>
        {/each}
        <p class="muted">Click a card to edit it, double-click to view it full screen. Ctrl/Shift-click or “Select all shown” to edit many at once.</p>
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
  .filters { padding: 10px 14px 6px; flex-wrap: wrap; }
  .subbar { padding: 0 14px 8px; }
  .grid { flex: 1; overflow: auto; display: grid; gap: 10px; padding: 4px 14px 20px; align-content: start; }
  .card { padding: 6px; display: flex; flex-direction: column; gap: 4px; background: var(--panel); text-align: left; user-select: none; }
  .card.sel { border-color: var(--accent); background: #22304d; }
  .img { position: relative; background: var(--bg); border-radius: 4px; overflow: hidden; }
  .img img { width: 100%; height: 100%; object-fit: contain; display: block; }
  .lock { position: absolute; top: 4px; right: 4px; font-size: 12px; }
  .dfc { position: absolute; top: 4px; left: 4px; font-size: 12px; background: rgba(0, 0, 0, 0.65); color: #fff; border-radius: 4px; padding: 0 4px; }
  .cap { display: flex; align-items: center; gap: 6px; font-size: 12px; }
  .nm { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .dot { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; display: inline-block; }
  .price { font-size: 11px; }
  .real { opacity: 0.7; }
  aside { width: 380px; flex-shrink: 0; border-left: 1px solid var(--line); background: var(--panel); overflow: auto; }
  .summary { padding: 16px; display: flex; flex-direction: column; gap: 8px; }
</style>
