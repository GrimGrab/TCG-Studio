<script lang="ts">
  import { onMount } from 'svelte';
  import { App, money, RARITIES, RARITY_COLORS, errText, ask, projectFile } from '../lib/api';
  import GamifyAccessories from './GamifyAccessories.svelte';

  let { notify }: { notify: (t: string, k?: string) => void } = $props();

  let sets = $state<any[]>([]);
  let included = $state<Record<string, boolean>>({});
  let settings = $state<any>(null);
  let preview = $state<any[] | null>(null);
  let busy = $state(false);
  let tab = $state<'sets' | 'accessories'>('sets');
  let howOpen = $state(false); // the intro's "How tiers are worked out"

  // Sets that aren't installed are hidden unless asked for (they can't be played, so tiering them does nothing); hidden
  // sets never become tiers.
  let showUninstalled = $state((() => { try { return localStorage.getItem('gamify.showUninstalled') === '1'; } catch { return false; } })());
  $effect(() => { try { localStorage.setItem('gamify.showUninstalled', showUninstalled ? '1' : ''); } catch {} });
  const visible = $derived(sets.filter((s) => showUninstalled || s.installed));
  const hiddenCount = $derived(sets.length - visible.length);
  const order = $derived(visible.filter((s) => included[s.id]).map((s) => s.id));

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

  /** Swaps a set with the next shown one above or below it (hidden sets keep their place). */
  function move(id: string, d: number) {
    const k = visible.findIndex((s) => s.id === id), other = visible[k + d];
    if (!other) return;
    const i = sets.findIndex((s) => s.id === id), j = sets.findIndex((s) => s.id === other.id);
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
    const tail = !inst ? '' : ` ${inst} installed set(s) change in the game at its next start.`;
    if (!(await ask(`Re-price ${order.length} set(s)? Locked cards are skipped.${tail}`))) return;
    busy = true;
    try {
      preview = await App.GamifyApply(order, settings);
      const re = (preview ?? []).filter((p: any) => p.installed).length;
      notify(`Gamified ${order.length} set(s)${re ? ` — restart the game to see ${re} installed set(s) change` : ' (install them to use them in game)'}.`, 'ok');
      await load();
    } catch (e) { notify(errText(e), 'error'); }
    busy = false;
  }

  const tierOf = (id: string) => order.indexOf(id) + 1;

  onMount(load);
</script>

<div class="page">
  <header class="head">
    <div><h2>Gamify</h2><div class="muted small">Pricing &amp; progression</div></div>
    <nav class="tabbar">
      <button class:on={tab === 'sets'} onclick={() => (tab = 'sets')}>Sets</button>
      <button class:on={tab === 'accessories'} onclick={() => (tab = 'accessories')}>Accessories</button>
    </nav>
  </header>
  <!-- Fixed intro strip under the header (not part of the scrolling body). -->
  <div class="intro">
    {#if tab === 'sets'}
      <p><span class="muted">Each ticked set becomes a <b>tier</b> in the ladder's order: tier 1 unlocks at shop level 1 like the Basic pack,
        later tiers follow the vanilla progression (licenses, pack costs, card values).</span>
        <button class="link" onclick={() => (howOpen = !howOpen)}>{howOpen ? 'Hide details' : 'How tiers are worked out'}</button></p>
      {#if howOpen}<p class="muted">Tiers follow Rare → Epic → Legendary → Destiny Basic…Legendary → Ascension, then beyond. With more than 9 sets
        they are spread evenly along that same curve (values in between are interpolated), so even 100 sets stay reachable and sensibly
        priced. Installed sets change in the game at its next start.</p>{/if}
    {:else}
      <p class="muted">Accessories get their own progression, separate from the set tiers: one ladder per type. License levels are spread from the
        first to the last level; prices follow the vanilla license curve.</p>
    {/if}
  </div>

  <div class="main">
  {#if tab === 'accessories'}
    <GamifyAccessories {notify} />
  {:else}
    <div class="cols">
      <section class="block">
        <div class="bhead"><h2>Tier ladder</h2><span class="count">{order.length} of {visible.length} sets</span><span class="grow"></span>
          <label class="check small" title="Sets that aren't installed can't be played, so they're hidden and left out of the tiers unless you show them.">
            <input type="checkbox" bind:checked={showUninstalled} onchange={() => (preview = null)} /> Show sets that aren't installed{hiddenCount ? ` (${hiddenCount})` : ''}</label>
          <button class="small" onclick={() => { included = Object.fromEntries(sets.map((s) => [s.id, !!s.installed])); preview = null; }}>Installed only</button>
          <button class="small" onclick={() => { included = Object.fromEntries(sets.map((s) => [s.id, true])); preview = null; }}>All sets</button>
          <button class="small" onclick={sortByDate}>Sort by release date</button></div>
        <p class="muted small">Ticked sets become tiers (installed sets are ticked by default). Unticked sets keep their prices.</p>
        <div class="ladder">
          {#each visible as s, i (s.id)}
            <div class="tier" class:off={!included[s.id]}>
              <input type="checkbox" bind:checked={included[s.id]} onchange={() => (preview = null)} aria-label="Make {s.name} a tier" />
              <span class="num">{included[s.id] ? tierOf(s.id) : '–'}</span>
              <span class="cover">{#if s.cover}<img src={projectFile(s.id, s.cover)} alt="" loading="lazy" />{/if}</span>
              <span class="info"><span class="name">{s.name}</span>
                <span class="sub">{[s.releasedAt ? s.releasedAt.slice(0, 4) : '', `${s.cards} cards`].filter(Boolean).join(' · ')}</span></span>
              {#if s.installed}<span class="badge ok">Installed</span>{:else}<span class="badge">Not installed</span>{/if}
              <span class="arrows">
                <button class="small" onclick={() => move(s.id, -1)} disabled={i === 0} aria-label="Up">↑</button>
                <button class="small" onclick={() => move(s.id, 1)} disabled={i === visible.length - 1} aria-label="Down">↓</button>
              </span>
            </div>
          {/each}
          {#if sets.length === 0}<p class="muted">No sets yet — import one first.</p>
          {:else if visible.length === 0}<p class="muted">No set is installed yet — install one on the Sets page, or show the ones that aren't.</p>{/if}
        </div>
      </section>

      <aside class="right">
        {#if settings}
          <section class="block">
            <div class="bhead"><h2>Settings</h2></div>
            <label class="field">Card prices
              <select bind:value={settings.mode} onchange={() => (preview = null)}>
                <option value="game">Game-like — fixed price per rarity (recommended)</option>
                <option value="hybrid">Hybrid — game price bands, ranked by real price</option>
                <option value="real">Real — market USD prices (sets from mods: the mod's own prices)</option>
              </select>
            </label>
            <label class="field">Border &amp; foil values
              <select bind:value={settings.borderCurve} onchange={() => (preview = null)} disabled={settings.mode === 'real'}>
                <option value="game">Game curve — Full Art / foil hits are big (like vanilla)</option>
                <option value="gentle">Gentle — ×1…×5, foil ×2.5</option>
              </select>
            </label>
            <div class="two">
              <label class="field">Card value per tier (+%)<input type="number" step="5" min="0" value={Math.round(settings.tierStep * 100)} oninput={(e) => { settings.tierStep = (+e.currentTarget.value || 0) / 100; preview = null; }} /></label>
              <label class="field" title="Each tier uses the vanilla product's wholesale pack cost; 100% = exactly vanilla">Pack cost vs vanilla (%)<input type="number" step="5" min="10" value={Math.round(settings.packCostScale * 100)} oninput={(e) => { settings.packCostScale = (+e.currentTarget.value || 100) / 100; preview = null; }} /></label>
            </div>
            <label class="field" title="Only used with more than 9 sets: the last set unlocks at this shop level and the rest are spread evenly before it">
              Last set unlocks at level (10+ sets)<input type="number" min="10" max="200" bind:value={settings.maxLevel} oninput={() => (preview = null)} disabled={order.length <= 9} /></label>
            <label class="check"><input type="checkbox" bind:checked={settings.progression} onchange={() => (preview = null)} /> Set license levels, license prices &amp; pack costs</label>
          </section>
        {/if}
        <section class="block action">
          <div><b>{order.length}</b> set{order.length === 1 ? '' : 's'} become tiers</div>
          <div class="row">
            <button class="grow" onclick={doPreview} disabled={busy || !order.length}>Preview</button>
            <button class="primary grow" onclick={apply} disabled={busy || !order.length}>Apply</button>
          </div>
          <span class="muted small">Preview first: nothing changes until Apply.</span>
        </section>
      </aside>
    </div>

    {#if preview}
      <section class="block">
        <div class="bhead"><h2>Result</h2><span class="muted small">Averages are base prices (Base border, non-foil); border and foil variants multiply them.</span></div>
        <div class="tablewrap">
          <table>
            <thead>
              <tr><th>Tier</th><th>Set</th><th>Like vanilla</th><th>Pack license (small / big)</th><th>Box license (small / big)</th><th>Pack cost</th>
                {#each RARITIES as r}<th style="color:{RARITY_COLORS[r]}">{r}</th>{/each}<th>Top card</th></tr>
            </thead>
            <tbody>
              {#each preview as p}
                <tr>
                  <td><span class="num">{p.tier}</span></td>
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
        </div>
        <p class="muted small">Sets with their own rarities are shown by each rarity's place in the list; Game-like and Hybrid prices step up
          the whole price range in list order.</p>
      </section>
    {/if}
  {/if}
  </div>
</div>

<style>
  .page { display: flex; flex-direction: column; height: 100%; overflow: hidden; }
  .head { display: flex; align-items: center; gap: 28px; padding: 14px 24px 0; border-bottom: 1px solid var(--line); }
  .head > div { padding-bottom: 10px; }
  .head h2 { font-size: 20px; }
  .tabbar { align-self: flex-end; padding: 0; border-bottom: none; }
  .main { flex: 1; min-height: 0; overflow: auto; padding: 18px 24px 28px; display: flex; flex-direction: column; gap: 14px; }
  .intro { flex: none; padding: 8px 24px; font-size: 13px; background: var(--panel); border-bottom: 1px solid var(--line); }
  .intro p { margin: 0; max-width: 1000px; }
  .intro p + p { margin-top: 6px; }
  .link { padding: 0 0 0 6px; font-size: 13px; }
  .cols { display: grid; grid-template-columns: minmax(0, 1fr) 340px; gap: 14px; align-items: start; }
  .right { position: sticky; top: 0; display: flex; flex-direction: column; gap: 14px; }
  .two { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); gap: 8px; align-items: end; }
  /* The narrow settings column: fields fill it and shrink with it instead of running past its edge. */
  .right .field { min-width: 0; }
  .right .field input, .right .field select { width: 100%; min-width: 0; }
  .action { gap: 8px; }
  .count { font-size: 11px; color: var(--muted); background: var(--panel-2); border-radius: 8px; padding: 0 6px; }
  .ladder { display: flex; flex-direction: column; gap: 4px; }
  .tier { display: flex; align-items: center; gap: 10px; padding: 6px 10px; border-radius: 8px; background: var(--panel-2); }
  .tier.off { opacity: 0.5; }
  .num { display: inline-grid; place-items: center; min-width: 26px; height: 26px; padding: 0 4px; border-radius: 13px; background: var(--accent-2);
    font-weight: 700; font-size: 12px; }
  .tier.off .num { background: var(--line); }
  .cover { width: 28px; height: 39px; flex: none; border-radius: 3px; background: var(--bg); overflow: hidden; }
  .cover img { width: 28px; height: 39px; object-fit: cover; }
  .info { flex: 1; min-width: 0; display: flex; flex-direction: column; }
  .info .name { font-weight: 600; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .info .sub { font-size: 12px; color: var(--muted); }
  .arrows { display: flex; gap: 4px; }
  .tablewrap { overflow: auto; }
  table { border-collapse: collapse; width: 100%; font-size: 13px; }
  th { color: var(--muted); font-weight: 500; }
  th, td { text-align: left; padding: 6px 8px; border-bottom: 1px solid var(--line); white-space: nowrap; }
  tbody tr:hover { background: var(--panel-2); }
</style>
