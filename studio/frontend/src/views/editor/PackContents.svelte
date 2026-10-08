<script lang="ts">
  // A pack's slots as a rarity × slot table: weights in the cells (with their % of the slot), and what they mean on the right —
  // cards of each rarity per pack and how often one particular card comes out. Mirrors the mod's PackRoller: a slot only rolls
  // rarities that have cards in the pack's pool, then any card of the rolled rarity.
  import { setRarities, rarityColor } from '../../lib/api';

  let { project, pack }: { project: any; pack: any } = $props();

  let showAll = $state(false);

  const rarities = $derived(setRarities(project.set));
  /** Cards of each rarity in the pack's pool (its own card list, else the whole set). */
  const pool = $derived.by(() => {
    const only = pack.cards?.length ? new Set<string>(pack.cards) : null;
    const m: Record<string, number> = {};
    for (const c of project.set.cards) if (!only || only.has(c.id)) m[c.rarity] = (m[c.rarity] ?? 0) + 1;
    return m;
  });
  /** Per slot: rarity → share of the slot (0–1), counting only rarities with cards, as the mod rolls. */
  const shares = $derived(pack.slots.map((s: any) => {
    const live = Object.entries(s.weights as Record<string, number>).filter(([r, w]) => w > 0 && pool[r] > 0);
    const sum = live.reduce((t, [, w]) => t + w, 0);
    return Object.fromEntries(live.map(([r, w]) => [r, w / sum])) as Record<string, number>;
  }));
  const perPack = $derived(Object.fromEntries(rarities.map((r) => [r.id,
    pack.slots.reduce((t: number, s: any, i: number) => t + (s.count || 0) * (shares[i][r.id] ?? 0), 0)])) as Record<string, number>);
  const used = (id: string) => pack.slots.some((s: any) => (s.weights[id] ?? 0) > 0);
  const shown = $derived(rarities.filter((r) => showAll || used(r.id)));
  const hidden = $derived(rarities.filter((r) => !used(r.id)));
  const neverCards = $derived(hidden.reduce((t, r) => t + (pool[r.id] ?? 0), 0));
  const slotSum = $derived(pack.slots.reduce((s: number, x: any) => s + (x.count || 0), 0));

  function weight(slot: any, r: string, v: string) {
    const n = parseFloat(v);
    if (isNaN(n) || n <= 0) delete slot.weights[r];
    else slot.weights[r] = n;
    slot.weights = { ...slot.weights };
  }

  const pct = (x: number) => (x >= 0.995 ? '100%' : x >= 0.1 ? `${(x * 100).toFixed(0)}%` : x >= 0.001 ? `${(x * 100).toFixed(1)}%` : `${(x * 100).toPrecision(2)}%`);
  /** "4 cards", "0.35 cards", "1 card": an amount per pack without trailing zeros. */
  function cards(x: number): string {
    const v = x >= 0.1 ? +x.toFixed(2) : +x.toPrecision(2);
    return `${v} card${v === 1 ? '' : 's'}`;
  }

  /** How often one particular card of the rarity comes out. */
  function oneCard(id: string): string {
    const n = pool[id] ?? 0, x = perPack[id] / Math.max(1, n);
    if (!n) return '';
    if (x <= 0) return 'never';
    if (x >= 0.5) return `${+x.toFixed(2)} per pack`;
    const packs = 1 / x;
    return `1 in ${packs >= 100 ? Math.round(packs).toLocaleString() : packs.toFixed(packs >= 10 ? 0 : 1)} packs`;
  }
</script>

<div class="scroll">
  <table>
    <thead>
      <tr>
        <th class="name">Rarity</th>
        <th class="num" title="Cards of this rarity this pack can give">Cards</th>
        {#each pack.slots as slot, si}
          <th class="slot">
            <div class="row head">Slot {si + 1}<button class="x" title="Remove this slot" onclick={() => pack.slots.splice(si, 1)}>✕</button></div>
            <label class="cnt">×<input type="number" min="1" bind:value={slot.count} /> card{slot.count === 1 ? '' : 's'}</label>
          </th>
        {/each}
        <th class="sep"></th>
        <th class="num" title="How many cards of this rarity an average pack holds">In each pack</th>
        <th class="num" title="How often one particular card of this rarity comes out (its rarity's cards share the slots)">Each card</th>
      </tr>
    </thead>
    <tbody>
      {#each shown as r (r.id)}
        <tr class:dim={!pool[r.id]}>
          <td class="name"><span class="dot" style="background:{rarityColor(project.set, r.id)}"></span>{r.name}
            {#if !pool[r.id] && used(r.id)}<span class="warn" title="No card of this rarity is in this pack: the mod skips it and rolls the slot's other rarities">no cards</span>{/if}</td>
          <td class="num muted">{pool[r.id] ?? 0}</td>
          {#each pack.slots as slot, si}
            <td class="cell">
              <input type="number" min="0" step="any" value={slot.weights[r.id] ?? ''} placeholder="–"
                oninput={(e) => weight(slot, r.id, e.currentTarget.value)} />
              <span class="pct">{shares[si][r.id] ? pct(shares[si][r.id]) : ''}</span>
            </td>
          {/each}
          <td class="sep"></td>
          <td class="num">{perPack[r.id] ? cards(perPack[r.id]) : ''}</td>
          <td class="num muted">{oneCard(r.id)}</td>
        </tr>
      {/each}
    </tbody>
    <tfoot>
      <tr>
        <td class="name muted">Total</td><td></td>
        {#each pack.slots as slot, si}
          <td class="cell muted">{Object.keys(shares[si]).length ? '100%' : ''}
            {#if !Object.keys(shares[si]).length}<span class="warn" title="None of this slot's rarities has cards in this pack: the mod picks any card">any card</span>{/if}</td>
        {/each}
        <td class="sep"></td>
        <td class="num"><b>{cards(slotSum)}</b></td><td></td>
      </tr>
    </tfoot>
  </table>
</div>
<div class="row foot">
  <button class="small" onclick={() => pack.slots.push({ count: 1, weights: { [rarities[0]?.id ?? 'Common']: 1 } })}>+ Slot</button>
  {#if hidden.length}
    <button class="small link" onclick={() => (showAll = !showAll)}>{showAll ? '▾ Hide' : '▸ Show'} {hidden.length} rarit{hidden.length === 1 ? 'y' : 'ies'} not in any slot</button>
    {#if neverCards}<span class="warn">{neverCards} card{neverCards === 1 ? '' : 's'} of those can never come out of this pack</span>{/if}
  {/if}
  <span class="grow"></span>
  {#if slotSum !== pack.cardsPerPack}<span class="err">Slot cards add up to {slotSum}, must be {pack.cardsPerPack}</span>{/if}
</div>
<p class="muted small">Weights are relative within a slot (the % beside each is its share). Each slot rolls a rarity, then any card of
  it — so “In each pack” is how many cards of a rarity you get, and “Each card” how often one particular card of it turns up.</p>

<style>
  .scroll { overflow-x: auto; }
  table { border-collapse: collapse; font-size: 13px; }
  th, td { padding: 3px 6px; white-space: nowrap; }
  th { font-weight: 500; text-align: left; vertical-align: bottom; color: var(--muted, #8a93a3); }
  thead th { border-bottom: 1px solid var(--line); }
  tfoot td { border-top: 1px solid var(--line); }
  tbody tr:hover { background: rgba(255, 255, 255, 0.03); }
  .name { min-width: 150px; }
  .num { text-align: right; }
  .sep { border-left: 1px solid var(--line); padding: 0; width: 6px; }
  .slot { min-width: 104px; }
  .head { gap: 4px; align-items: center; }
  .x { padding: 0 5px; font-size: 11px; line-height: 16px; margin-left: auto; }
  .cnt { display: flex; align-items: center; gap: 3px; font-weight: 400; font-size: 12px; }
  .cnt input { width: 44px; padding: 1px 4px; }
  .cell input { width: 58px; padding: 2px 4px; }
  .pct { display: inline-block; width: 40px; font-size: 11px; color: var(--muted, #8a93a3); }
  .dot { display: inline-block; width: 9px; height: 9px; border-radius: 50%; margin-right: 6px; }
  .dim { opacity: 0.55; }
  .warn { color: var(--warn, #e0a030); font-size: 11px; margin-left: 6px; }
  .err { color: var(--danger); font-size: 12px; }
  .foot { gap: 8px; align-items: center; }
  .link { background: none; border: none; color: var(--accent); }
  .small { font-size: 12px; }
</style>
