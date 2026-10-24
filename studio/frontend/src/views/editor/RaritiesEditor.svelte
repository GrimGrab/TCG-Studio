<script lang="ts">
  // The set's rarities (set.json "rarities"), lowest first. Each is a rarity of its own in game: its name on the card, binder and
  // Check Price, and its own weight in pack slots. A set without a list has Common, Rare, Epic, Legendary; the first edit writes them.
  import { App, setRarities, rarityColor, rarityId, errText, ask, type RarityDef } from '../../lib/api';

  let { project, notify }: { project: any; notify: (t: string, k?: string) => void } = $props();

  const rows = $derived<RarityDef[]>(setRarities(project.set));
  const counts = $derived.by(() => {
    const m: Record<string, number> = {};
    for (const c of project.set.cards) m[c.rarity] = (m[c.rarity] ?? 0) + 1;
    return m;
  });
  const maxCount = $derived(Math.max(1, ...Object.values(counts)));
  const hasSource =$derived(!project.set.rarities?.length && project.set.cards.some((c: any) => project.meta?.cards?.[c.id]?.srcRarity));
  let busy = $state(false);

  /** The set's own list, written from the rows shown on the first edit. */
  function list(): RarityDef[] {
    if (!project.set.rarities?.length) project.set.rarities = rows.map((r) => ({ ...r }));
    return project.set.rarities;
  }

  async function useSource() {
    busy = true;
    try {
      const p: any = await App.UseSourceRarities(project);
      project.set.rarities = p.set.rarities;
      project.set.cards = p.set.cards;
      project.set.packs = p.set.packs;
      project.meta.rarityOrder = p.meta.rarityOrder;
      notify(`The set now has the source's ${p.set.rarities.length} rarities — save to keep them`, 'ok');
    } catch (e) { notify(errText(e), 'error'); }
    busy = false;
  }

  function add() {
    const l = list();
    let name = 'New rarity', n = 2;
    while (l.some((r) => r.name === name)) name = `New rarity ${n++}`;
    let id = rarityId(name), k = 2;
    while (l.some((r) => r.id === id)) id = `${rarityId(name)}-${k++}`;
    l.push({ id, name });
  }

  function move(i: number, d: number) {
    const l = list();
    [l[i], l[i + d]] = [l[i + d], l[i]];
  }

  async function remove(i: number) {
    const l = list();
    if (l.length === 1) return;
    const r = l[i];
    const to = l[i > 0 ? i - 1 : 1];
    const n = counts[r.id] ?? 0;
    if (n && !(await ask(`Delete “${r.name}”? Its ${n} card(s) and its pack weights move to “${to.name}”.`))) return;
    for (const c of project.set.cards) if (c.rarity === r.id) c.rarity = to.id;
    for (const p of project.set.packs)
      for (const s of p.slots ?? [])
        if (s.weights[r.id] !== undefined) {
          s.weights[to.id] = (s.weights[to.id] ?? 0) + s.weights[r.id];
          delete s.weights[r.id];
          s.weights = { ...s.weights };
        }
    l.splice(i, 1);
  }

  const set = (i: number, field: keyof RarityDef, v: string | undefined) => { (list()[i] as any)[field] = v; };
</script>

<section class="block">
  <div class="bhead"><h2>Rarities</h2><span class="muted small">lowest first</span><span class="grow"></span>
    <button class="small" onclick={add}>+ Rarity</button></div>
  <p class="muted small">Each rarity is its own in game: its name shows on the cards, in the binder and in Check Price, and each
    has its own weight in pack slots (Packs tab). Cards show the game's rarity icon for the rarity's place in the list.</p>
  {#if hasSource}
    <div class="notice info row"><button class="small" onclick={useSource} disabled={busy}>Use the source's rarities</button>
      <span class="muted small">the import's own names (e.g. Secret Rare, SR, a mod's tiers) instead of these four</span></div>
  {/if}
  <div class="rows">
    {#each rows as r, i (r.id)}
      <div class="rr">
        <span class="order">
          <button class="small" onclick={() => move(i, -1)} disabled={i === 0} title="Lower in the list">▲</button>
          <button class="small" onclick={() => move(i, 1)} disabled={i === rows.length - 1} title="Higher in the list">▼</button>
        </span>
        <label class="swatch" style="background:{rarityColor(project.set, r.id)}" title={r.color ? 'Own colour — click to change' : 'Automatic colour — click to choose one'}>
          <input type="color" value={rarityColor(project.set, r.id)} oninput={(e) => set(i, 'color', e.currentTarget.value)} />
        </label>
        <input class="name" value={r.name} oninput={(e) => set(i, 'name', e.currentTarget.value)} title="Id (stable): {r.id}" />
        <span class="bar" title="{counts[r.id] ?? 0} cards"><span style="width:{Math.round(((counts[r.id] ?? 0) / maxCount) * 100)}%; background:{rarityColor(project.set, r.id)}"></span></span>
        <span class="n">{counts[r.id] ?? 0}</span>
        {#if r.color}<button class="small x" title="Use the automatic colour" onclick={() => set(i, 'color', undefined)}>⟲</button>{:else}<span class="x"></span>{/if}
        <button class="small x" onclick={() => remove(i)} disabled={rows.length === 1} title="Delete this rarity">✕</button>
      </div>
    {/each}
  </div>
</section>

<style>
  .rows { display: flex; flex-direction: column; gap: 4px; }
  .rr { display: flex; align-items: center; gap: 8px; padding: 3px 6px; border-radius: 6px; }
  .rr:hover { background: var(--panel-2); }
  .order { display: flex; gap: 2px; }
  .order button { padding: 1px 5px; font-size: 10px; }
  .swatch { position: relative; width: 22px; height: 22px; flex: none; border-radius: 50%; cursor: pointer; border: 2px solid rgba(255, 255, 255, 0.15); }
  .swatch input { position: absolute; inset: 0; opacity: 0; width: 100%; height: 100%; cursor: pointer; }
  .name { flex: 1 1 140px; min-width: 100px; }
  .bar { flex: 0 1 120px; height: 6px; background: var(--bg); border-radius: 3px; overflow: hidden; }
  .bar span { display: block; height: 100%; border-radius: 3px; }
  .n { width: 40px; text-align: right; font-variant-numeric: tabular-nums; color: var(--muted); font-size: 12px; }
  .x { width: 28px; flex: none; padding: 2px 0; }
  .notice.row { gap: 10px; }
</style>
