<script lang="ts">
  import { onMount } from 'svelte';
  import { App, money, RARITIES, RARITY_COLORS, errText, ask } from '../lib/api';
  import GamifyAccessories from './GamifyAccessories.svelte';

  let { notify }: { notify: (t: string, k?: string) => void } = $props();

  let sets = $state<any[]>([]);
  let included = $state<Record<string, boolean>>({});
  let settings = $state<any>(null);
  let preview = $state<any[] | null>(null);
  let busy = $state(false);
  let reinstall = $state(true);
  let tab = $state<'sets' | 'accessories'>('sets');

  const order = $derived(sets.filter((s) => included[s.id]).map((s) => s.id));

  async function load() {
    const list = await App.ListProjects();
    // Existing tiers first (in tier order), then everything else by release date.
    list.sort((a: any, b: any) => (a.tier || 999) - (b.tier || 999) || (a.releasedAt || '9999').localeCompare(b.releasedAt || '9999'));
    sets = list;
    // Only sets that are in the game count as tiers by default (all of them if none is installed yet).
    const anyInstalled = list.some((s: any) => s.installed);
    included = Object.fromEntries(list.map((s: any) => [s.id, !anyInstalled || !!s.installed]));
    settings = await App.GamifyDefaults();
  }

  function move(i: number, d: number) {
    const j = i + d;
    if (j < 0 || j >= sets.length) return;
    [sets[i], sets[j]] = [sets[j], sets[i]];
    preview = null;
  }

  function sortByDate() {
    sets = [...sets].sort((a, b) => (a.releasedAt || '9999').localeCompare(b.releasedAt || '9999'));
    preview = null;
  }

  async function doPreview() {
    busy = true;
    try { preview = await App.GamifyPreview(order, settings); }
    catch (e) { notify(errText(e), 'error'); }
    busy = false;
  }

  async function apply() {
    const inst = sets.filter((s) => included[s.id] && s.installed).length;
    const tail = !inst ? '' : reinstall ? ` ${inst} installed set(s) will be reinstalled in the game.` : ` ${inst} installed set(s) will NOT be updated in the game until you reinstall them.`;
    if (!(await ask(`Re-price ${order.length} set(s)? Locked cards are skipped.${tail}`))) return;
    busy = true;
    try {
      preview = await App.GamifyApply(order, settings, reinstall);
      const re = (preview ?? []).filter((p: any) => p.installed).length;
      notify(`Gamified ${order.length} set(s)${re && reinstall ? ` and reinstalled ${re} in the game (restart the game to see it)` : re ? ` — ${re} installed set(s) still need reinstalling` : ' (install them to use them in game)'}.`, 'ok');
      await load();
    } catch (e) { notify(errText(e), 'error'); }
    busy = false;
  }

  const tierOf = (id: string) => order.indexOf(id) + 1;

  onMount(load);
</script>

<div class="page">
  <header class="row"><h2 class="grow">Gamify pricing & progression</h2></header>
  <div class="row tabs">
    <button class:on={tab === 'sets'} onclick={() => (tab = 'sets')}>Sets</button>
    <button class:on={tab === 'accessories'} onclick={() => (tab = 'accessories')}>Accessories</button>
  </div>

  {#if tab === 'accessories'}
    <GamifyAccessories {notify} />
  {:else}
  <p class="muted intro">
    Fits imported sets into the game's economy. Sets become <b>tiers</b> in the order below: tier 1 unlocks at shop level 1 like the Basic pack,
    later tiers copy the vanilla progression exactly (Rare → Epic → Legendary → Destiny Basic…Legendary → Ascension, then beyond):
    the same four license levels and prices, the same pack costs, and cards worth more each tier. With more than 9 sets they are spread
    evenly along that same curve (values in between are interpolated), so even 100 sets stay reachable and sensibly priced.
    Sets already installed in the game are reinstalled when you apply (untick the option next to Apply to skip that).
  </p>

  {#if settings}
    <section class="settings">
      <label class="field">Card prices
        <select bind:value={settings.mode} onchange={() => (preview = null)}>
          <option value="hybrid">Hybrid — game price bands, ranked by real price (recommended)</option>
          <option value="game">Game-like — fixed price per rarity</option>
          <option value="real">Real — market USD prices</option>
        </select>
      </label>
      <label class="field">Border & foil values
        <select bind:value={settings.borderCurve} onchange={() => (preview = null)} disabled={settings.mode === 'real'}>
          <option value="game">Game curve — Full Art / foil hits are big (like vanilla)</option>
          <option value="gentle">Gentle — ×1…×5, foil ×2.5</option>
        </select>
      </label>
      <label class="field">Card value per tier (+%)<input type="number" step="5" min="0" value={Math.round(settings.tierStep * 100)} oninput={(e) => { settings.tierStep = (+e.currentTarget.value || 0) / 100; preview = null; }} /></label>
      <label class="field" title="Each tier uses the vanilla product's wholesale pack cost; 100% = exactly vanilla">Pack cost vs vanilla (%)<input type="number" step="5" min="10" value={Math.round(settings.packCostScale * 100)} oninput={(e) => { settings.packCostScale = (+e.currentTarget.value || 100) / 100; preview = null; }} /></label>
      <label class="field" title="Only used with more than 9 sets: the last set unlocks at this shop level and the rest are spread evenly before it">
        Last set unlocks at level (10+ sets)<input type="number" min="10" max="200" bind:value={settings.maxLevel} oninput={() => (preview = null)} disabled={order.length <= 9} /></label>
      <label class="check"><input type="checkbox" bind:checked={settings.progression} onchange={() => (preview = null)} /> Set license levels, license prices & pack costs</label>
    </section>
  {/if}

  <section>
    <div class="row"><h3 class="grow">Tier order</h3>
      <button class="small" onclick={() => { included = Object.fromEntries(sets.map((s) => [s.id, !!s.installed])); preview = null; }}>Installed only</button>
      <button class="small" onclick={() => { included = Object.fromEntries(sets.map((s) => [s.id, true])); preview = null; }}>All sets</button>
      <button class="small" onclick={sortByDate}>Sort by release date</button></div>
    <p class="muted small">Ticked sets become tiers (installed sets are ticked by default). Unticked sets keep their prices.</p>
    {#each sets as s, i (s.id)}
      <div class="tier row" class:off={!included[s.id]}>
        <input type="checkbox" bind:checked={included[s.id]} onchange={() => (preview = null)} />
        <b class="num">{included[s.id] ? tierOf(s.id) : '–'}</b>
        <span class="grow">{s.name} <span class="muted small">{s.releasedAt ?? ''} · {s.cards} cards</span>
          {#if s.installed}<span class="badge ok">Installed</span>{:else}<span class="badge">Not installed</span>{/if}</span>
        <button class="small" onclick={() => move(i, -1)} disabled={i === 0}>↑</button>
        <button class="small" onclick={() => move(i, 1)} disabled={i === sets.length - 1}>↓</button>
      </div>
    {/each}
    {#if sets.length === 0}<p class="muted">No sets yet — import one first.</p>{/if}
  </section>

  <div class="row">
    <button onclick={doPreview} disabled={busy || !order.length}>Preview</button>
    <button class="primary" onclick={apply} disabled={busy || !order.length}>Apply to {order.length} set(s)</button>
    <label class="check"><input type="checkbox" bind:checked={reinstall} /> Reinstall sets that are already installed in the game</label>
  </div>

  {#if preview}
    <section>
      <h3>Result</h3>
      <table>
        <thead>
          <tr><th>Tier</th><th>Set</th><th>Like vanilla</th><th>Pack license (small / big)</th><th>Box license (small / big)</th><th>Pack cost</th>
            {#each RARITIES as r}<th style="color:{RARITY_COLORS[r]}">{r}</th>{/each}<th>Top card</th></tr>
        </thead>
        <tbody>
          {#each preview as p}
            <tr>
              <td>{p.tier}</td>
              <td>{p.name}{p.locked ? ` (${p.locked} locked)` : ''}</td>
              <td class="small">{settings.progression ? p.like : '—'}</td>
              <td>{settings.progression ? `L${p.license.packLevel} ${money(p.license.packPrice)} / L${p.license.packBigLevel} ${money(p.license.packBigPrice)}` : '—'}</td>
              <td>{settings.progression ? `L${p.license.boxLevel} ${money(p.license.boxPrice)} / L${p.license.boxBigLevel} ${money(p.license.boxBigPrice)}` : '—'}</td>
              <td>{settings.progression ? money(p.packCost) : '—'}</td>
              {#each RARITIES as r}
                <td>{#if p.avgAfter[r] !== undefined}<span class="muted small">{money(p.avgBefore[r])} →</span> {money(p.avgAfter[r])}{/if}</td>
              {/each}
              <td class="small">{#if p.top?.[0]}{p.top[0].name}: {money(p.top[0].before)} → <b>{money(p.top[0].after)}</b>{/if}</td>
            </tr>
          {/each}
        </tbody>
      </table>
      <p class="muted small">Averages are base prices (Base border, non-foil). Border and foil variants multiply them.</p>
    </section>
  {/if}
  {/if}
</div>

<style>
  .page { padding: 20px 24px; overflow: auto; height: 100%; display: flex; flex-direction: column; gap: 14px; }
  .intro { max-width: 900px; margin: 0; }
  .tabs { border-bottom: 1px solid var(--line); padding-bottom: 8px; }
  .tabs button.on { border-color: var(--accent); background: #22304d; }
  section { background: var(--panel); border: 1px solid var(--line); border-radius: var(--radius); padding: 12px; display: flex; flex-direction: column; gap: 8px; }
  .settings { display: grid; grid-template-columns: repeat(3, 1fr); gap: 10px; }
  .tier { padding: 6px 8px; border-radius: 6px; background: var(--panel-2); }
  .tier.off { opacity: 0.5; }
  .num { width: 28px; text-align: center; }
  table { border-collapse: collapse; width: 100%; font-size: 13px; }
  th, td { text-align: left; padding: 5px 6px; border-bottom: 1px solid var(--line); white-space: nowrap; }
  .small { font-size: 12px; }
</style>
