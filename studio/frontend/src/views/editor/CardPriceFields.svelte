<script lang="ts">
  // A card's price: base, foil multiplier, own border multipliers and exact prices per version, with what each version
  // sells for. Shared by the card panel and the card editor (Price page).
  import { money, BORDERS } from '../../lib/api';
  import { borderLabel } from '../../lib/cardVersions';

  let { project, card }: { project: any; card: any } = $props();

  const meta = $derived(project.meta?.cards?.[card.id]);
  let newOverride = $state('Base_foil');
  const variantKeys = BORDERS.flatMap((b: string) => [b, b + '_foil']);

  function optNumber(v: string): number | undefined {
    const n = parseFloat(v);
    return isNaN(n) ? undefined : n;
  }

  function toggleBorderMults() {
    card.price.borderMultipliers = card.price.borderMultipliers ? undefined : [...project.set.priceDefaults.borderMultipliers];
  }

  function addOverride() {
    card.price.overrides = { ...(card.price.overrides ?? {}), [newOverride]: card.price.base };
  }

  function removeOverride(k: string) {
    const o = { ...card.price.overrides };
    delete o[k];
    card.price.overrides = Object.keys(o).length ? o : undefined;
  }

  function setLocked(v: boolean) {
    project.meta.cards ??= {};
    project.meta.cards[card.id] = { ...(project.meta.cards[card.id] ?? {}), locked: v };
  }

  const preview = $derived(
    BORDERS.map((b: string, i: number) => {
      const bm = (card.price.borderMultipliers ?? project.set.priceDefaults.borderMultipliers)[i];
      const fm = card.price.foilMultiplier ?? project.set.priceDefaults.foilMultiplier;
      const o = card.price.overrides ?? {};
      const min = project.set.priceDefaults.minimum;
      return [b, o[b] ?? Math.max(min, card.price.base * bm), o[b + '_foil'] ?? Math.max(min, card.price.base * bm * fm)] as [string, number, number];
    })
  );
</script>

<div class="fields">
  <div class="shead"><span class="grow"></span>
    <label class="check small lock" class:on={!!meta?.locked} title="Gamify and price refresh skip locked cards"><input type="checkbox" checked={!!meta?.locked} onchange={(e) => setLocked(e.currentTarget.checked)} /> 🔒 Lock</label>
  </div>
  {#if meta && (meta.usd !== undefined || meta.usdFoil !== undefined)}
    <div class="real">Real price <b>{money(meta.usd)}</b> · foil <b>{money(meta.usdFoil)}</b></div>
  {/if}
  <div class="row">
    <label class="field grow">Base price<input type="number" step="0.01" min="0" bind:value={card.price.base} /></label>
    <label class="field grow">Foil ×<input type="number" step="0.1" placeholder={String(project.set.priceDefaults.foilMultiplier)}
      value={card.price.foilMultiplier ?? ''} oninput={(e) => (card.price.foilMultiplier = optNumber(e.currentTarget.value))} /></label>
  </div>
  <table class="pricetable">
    <thead><tr><th>Version</th><th>Normal</th><th>Foil</th></tr></thead>
    <tbody>{#each preview as [b, n, f]}<tr><td>{borderLabel(b)}</td><td>{money(n)}</td><td class="foil">{money(f)}</td></tr>{/each}</tbody>
  </table>
  <label class="check small"><input type="checkbox" checked={!!card.price.borderMultipliers} onchange={toggleBorderMults} /> Own border multipliers</label>
  {#if card.price.borderMultipliers}
    <div class="mults">
      {#each BORDERS as b, i}<label class="field">{borderLabel(b)}<input type="number" step="0.05" bind:value={card.price.borderMultipliers[i]} /></label>{/each}
    </div>
  {/if}
  <div class="overrides">
    {#each Object.entries(card.price.overrides ?? {}) as [k, v]}
      <div class="row ov"><span class="grow small">Exact: {k.replace('_foil', ' foil')}</span>
        <input type="number" step="0.01" style="width:100px" value={v} oninput={(e) => (card.price.overrides[k] = optNumber(e.currentTarget.value) ?? 0)} />
        <button class="small" onclick={() => removeOverride(k)} title="Remove this exact price">✕</button></div>
    {/each}
    <div class="row">
      <select class="grow" bind:value={newOverride}>{#each variantKeys as k}<option value={k}>{k.replace('_foil', ' foil')}</option>{/each}</select>
      <button class="small" onclick={addOverride}>Add exact price</button>
    </div>
  </div>
</div>

<style>
  .fields { display: flex; flex-direction: column; gap: 10px; }
  .shead { display: flex; align-items: center; gap: 8px; }
  .lock { padding: 2px 8px; border: 1px solid var(--line); border-radius: 999px; cursor: pointer; }
  .lock.on { border-color: var(--warn); color: var(--warn); }
  .real { font-size: 12px; color: var(--muted); }
  .real b { color: var(--ok); font-weight: 600; }
  .row > .field { flex: 1 1 0; min-width: 0; }
  .field { min-width: 0; }
  .field > input { width: 100%; }
  .row > select { min-width: 0; }
  .mults { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 6px; }
  .overrides { display: flex; flex-direction: column; gap: 4px; }
  .ov { background: var(--panel); border-radius: 6px; padding: 2px 2px 2px 8px; }
  .pricetable { width: 100%; border-collapse: collapse; font-size: 12px; font-variant-numeric: tabular-nums; }
  .pricetable th { text-align: left; font-weight: 500; color: var(--muted); padding: 2px 6px 4px; }
  .pricetable td { text-align: left; padding: 3px 6px; border-top: 1px solid var(--line); }
  .pricetable td.foil { color: #c9a8ff; }
</style>
