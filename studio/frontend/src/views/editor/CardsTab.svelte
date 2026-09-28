<script lang="ts">
  import { App, projectFile, money, RARITIES, RARITY_COLORS, errText, ask } from '../../lib/api';
  import CardDetail from './CardDetail.svelte';
  import BulkEdit from './BulkEdit.svelte';

  let { project, notify, imgBust }: { project: any; notify: (t: string, k?: string) => void; imgBust: number } = $props();

  let query = $state('');
  let rarity = $state('');
  let color = $state('');
  let sort = $state('set');
  let size = $state(150);
  let selected = $state<string[]>([]);
  let anchor = $state<number>(-1);

  const meta = (id: string) => project.meta?.cards?.[id] ?? {};
  const RANK: Record<string, number> = Object.fromEntries(RARITIES.map((r, i) => [r, i]));

  const shown = $derived.by(() => {
    const q = query.trim().toLowerCase();
    let list = project.set.cards.filter((c: any) => {
      if (rarity && c.rarity !== rarity) return false;
      const m = meta(c.id);
      if (color === 'C' && m.colors?.length) return false;
      if (color === 'M' && !(m.colors?.length > 1)) return false;
      if (color && color !== 'C' && color !== 'M' && !m.colors?.includes(color)) return false;
      if (!q) return true;
      return c.name.toLowerCase().includes(q) || c.id.includes(q) || (m.typeLine ?? '').toLowerCase().includes(q) || (c.description ?? '').toLowerCase().includes(q);
    });
    if (sort === 'name') list = [...list].sort((a: any, b: any) => a.name.localeCompare(b.name));
    if (sort === 'rarity') list = [...list].sort((a: any, b: any) => RANK[b.rarity] - RANK[a.rarity] || a.name.localeCompare(b.name));
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

  async function addCards() {
    try {
      const images: string[] = await App.PickImages(project.id);
      const ids = new Set(project.set.cards.map((c: any) => c.id));
      const added: string[] = [];
      for (const rel of images) {
        const stem = rel.replace(/^images\//, '').replace(/\.[^.]+$/, '');
        let id = stem.toLowerCase().replace(/[\s:/|\\]+/g, '-');
        for (let n = 2; ids.has(id); n++) id = `${stem.toLowerCase()}-${n}`;
        ids.add(id);
        project.set.cards.push({
          id, name: stem.replace(/[-_]+/g, ' '), description: '', artist: '', rarity: 'Common', image: rel,
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
    if (!(await ask(`Delete ${selected.length} card(s) from this set? Players who own them keep the data (it comes back if a card with the same id is re-added).`))) return;
    project.set.cards = project.set.cards.filter((c: any) => !selected.includes(c.id));
    for (const p of project.set.packs) p.cards = (p.cards ?? []).filter((id: string) => !selected.includes(id));
    selected = [];
  }

  const rarityCounts = $derived(RARITIES.map((r) => [r, project.set.cards.filter((c: any) => c.rarity === r).length]));
</script>

<div class="wrap">
  <div class="left">
    <div class="filters row">
      <input class="grow" placeholder="Search name, type, text…" bind:value={query} />
      <select bind:value={rarity}>
        <option value="">All rarities</option>
        {#each RARITIES as r}<option value={r}>{r}</option>{/each}
      </select>
      <select bind:value={color}>
        <option value="">All colors</option>
        <option value="W">White</option><option value="U">Blue</option><option value="B">Black</option>
        <option value="R">Red</option><option value="G">Green</option><option value="M">Multicolor</option><option value="C">Colorless</option>
      </select>
      <select bind:value={sort}>
        <option value="set">Set order</option><option value="name">Name</option><option value="rarity">Rarity</option>
        <option value="price">Game price</option><option value="real">Real price</option>
      </select>
      <input type="range" min="100" max="260" bind:value={size} title="Card size" />
    </div>
    <div class="subbar row">
      <span class="muted grow">{shown.length} shown · {selected.length} selected</span>
      <button class="small" onclick={selectAll}>Select all shown</button>
      <button class="small" onclick={() => (selected = [])} disabled={!selected.length}>Clear</button>
      <button class="small" onclick={addCards}>+ Add cards from images</button>
    </div>
    <div class="grid" style="grid-template-columns: repeat(auto-fill, minmax({size}px, 1fr))">
      {#each shown as c, i (c.id)}
        <button class="card" class:sel={selected.includes(c.id)} onclick={(e) => click(e, i, c.id)}>
          <div class="img" style="aspect-ratio: 63/88">
            {#if c.image}<img src={projectFile(project.id, c.image, imgBust)} alt={c.name} loading="lazy" draggable="false" />{/if}
            {#if meta(c.id).locked}<span class="lock" title="Price locked">🔒</span>{/if}
          </div>
          <div class="cap">
            <span class="dot" style="background:{RARITY_COLORS[c.rarity]}"></span>
            <span class="nm">{c.name}</span>
          </div>
          <div class="price muted">{money(c.price.base)}{#if meta(c.id).usd !== undefined}<span class="real"> · real {money(meta(c.id).usd)}</span>{/if}</div>
        </button>
      {/each}
    </div>
  </div>

  <aside>
    {#if selectedCards.length === 1}
      <CardDetail {project} card={selectedCards[0]} {imgBust} {notify} />
    {:else if selectedCards.length > 1}
      <BulkEdit {project} cards={selectedCards} {notify} ondelete={deleteSelected} />
    {:else}
      <div class="summary">
        <h3>Set summary</h3>
        {#each rarityCounts as [r, n]}
          <div class="row"><span class="dot" style="background:{RARITY_COLORS[r as string]}"></span><span class="grow">{r}</span><b>{n}</b></div>
        {/each}
        <p class="muted">Click a card to edit it. Ctrl/Shift-click or “Select all shown” to edit many at once.</p>
      </div>
    {/if}
  </aside>
</div>

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
  .cap { display: flex; align-items: center; gap: 6px; font-size: 12px; }
  .nm { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .dot { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; display: inline-block; }
  .price { font-size: 11px; }
  .real { opacity: 0.7; }
  aside { width: 380px; flex-shrink: 0; border-left: 1px solid var(--line); background: var(--panel); overflow: auto; }
  .summary { padding: 16px; display: flex; flex-direction: column; gap: 8px; }
</style>
