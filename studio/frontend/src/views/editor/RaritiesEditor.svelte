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

<div class="page-cols">
  <div class="block">
    <div class="bhead"><h2>Rarity ladder</h2><span class="muted small">most common first · {rows.length} rarities</span><span class="grow"></span>
      <button class="small" onclick={add}>+ Add rarity</button></div>
    {#if hasSource}
      <div class="notice info srcnote">
        <span><b>This set was imported with its own rarities</b><span class="muted small">Use the import's names (e.g. Secret Rare, SR, a mod's tiers) instead of these four.</span></span>
        <button class="small primary" onclick={useSource} disabled={busy}>{busy ? 'Switching…' : "Use the import's rarities"}</button>
      </div>
    {/if}
    <div class="rtable" role="table">
      <div class="rh" role="row"><span>#</span><span>Colour</span><span>Name</span><span class="idcol">Id</span><span>Cards</span><span></span></div>
      {#each rows as r, i (r.id)}
        <div class="rr" role="row">
          <span class="order">
            <span class="rank">{i + 1}</span>
            <span class="arrows">
              <button class="icon" onclick={() => move(i, -1)} disabled={i === 0} title="Move up: more common">▲</button>
              <button class="icon" onclick={() => move(i, 1)} disabled={i === rows.length - 1} title="Move down: rarer">▼</button>
            </span>
          </span>
          <span class="colour">
            <label class="swatch" style="background:{rarityColor(project.set, r.id)}" title={r.color ? 'Own colour — click to change' : 'Automatic colour — click to choose one'}>
              <input type="color" value={rarityColor(project.set, r.id)} oninput={(e) => set(i, 'color', e.currentTarget.value)} />
            </label>
            {#if r.color}<button class="icon" title="Back to the automatic colour" onclick={() => set(i, 'color', undefined)}>⟲</button>{/if}
          </span>
          <input class="name" value={r.name} oninput={(e) => set(i, 'name', e.currentTarget.value)} aria-label="Rarity name" />
          <span class="id idcol" title="Stable id: saves and packs refer to it">{r.id}</span>
          <span class="count">
            <span class="bar"><span style="width:{Math.round(((counts[r.id] ?? 0) / maxCount) * 100)}%; background:{rarityColor(project.set, r.id)}"></span></span>
            <b>{counts[r.id] ?? 0}</b>
          </span>
          <button class="icon del" onclick={() => remove(i)} disabled={rows.length === 1} title="Delete this rarity (its cards move to the one below)">✕</button>
        </div>
      {/each}
    </div>
  </div>
  <aside class="page-side">
    <div class="notice info howto"><b>How rarities work</b>
      <ul>
        <li>Each rarity is its own in the game: its name shows on cards, in the binder and in Check Price.</li>
        <li>The place in the ladder picks the game's rarity icon and its spot in sorting — rarest at the bottom.</li>
        <li>Pack slots give each rarity its own weight (Packs → Contents).</li>
        <li>Deleting a rarity moves its cards and pack weights to the next one down.</li>
      </ul>
    </div>
  </aside>
</div>

<style>
  .srcnote { display: flex; flex-wrap: wrap; align-items: center; gap: 10px 14px; }
  .srcnote > span { flex: 1 1 220px; min-width: 0; display: flex; flex-direction: column; gap: 2px; }
  .rtable { margin: 0 -8px; }
  .rh, .rr { display: grid; grid-template-columns: 56px 56px minmax(0, 1fr) minmax(0, 180px) minmax(90px, 260px) 28px; gap: 10px; align-items: center; }
  /* Narrow panel: no id column, the count without its bar. */
  @container (max-width: 560px) {
    .rh, .rr { grid-template-columns: 52px 52px minmax(0, 1fr) auto 28px; }
    .idcol, .bar { display: none; }
  }
  .rh { padding: 8px 8px 6px; font-size: 11px; text-transform: uppercase; letter-spacing: 0.06em; color: var(--muted); }
  .rr { padding: 6px 8px; border-radius: 8px; }
  .rr:hover { background: var(--panel-2); }
  .order { display: flex; align-items: center; gap: 6px; }
  .rank { width: 18px; text-align: right; color: var(--muted); font-variant-numeric: tabular-nums; }
  .arrows { display: flex; flex-direction: column; }
  .icon { background: none; border: 1px solid transparent; padding: 0 5px; font-size: 10px; line-height: 1.5; color: var(--muted); }
  .icon:hover:not(:disabled) { color: var(--text); border-color: var(--line); }
  .icon:disabled { opacity: 0.25; }
  .rr:not(:hover) .del { opacity: 0.4; }
  .del { font-size: 12px; }
  .colour { display: flex; align-items: center; gap: 4px; }
  .swatch { position: relative; width: 24px; height: 24px; flex: none; border-radius: 50%; cursor: pointer; border: 2px solid rgba(255, 255, 255, 0.18); }
  .swatch input { position: absolute; inset: 0; opacity: 0; width: 100%; height: 100%; cursor: pointer; }
  .name { min-width: 0; font-weight: 600; }
  .id { font-family: Consolas, monospace; font-size: 12px; color: var(--muted); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .count { display: flex; align-items: center; gap: 10px; }
  .count b { width: 44px; text-align: right; font-variant-numeric: tabular-nums; font-weight: 600; }
  .bar { flex: 1; height: 6px; background: var(--bg); border-radius: 3px; overflow: hidden; }
  .bar span { display: block; height: 100%; border-radius: 3px; }
</style>
