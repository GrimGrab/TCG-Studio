<script lang="ts">
  // The set's card versions (set.json "variants"): the default for every card; a card can have its own list (card detail,
  // or several cards at once in the Cards tab). Versions that are off are never made by packs or trade customers and are left
  // out of the binder and Check Price — unless the player already owns one. All on = the field is left out.
  import VersionGrid from './VersionGrid.svelte';
  import { BORDERS } from '../../lib/api';
  import { ALL_VERSIONS, followSet, hasOwnVersions, setVersions, totalVersions, writeSetVersions } from '../../lib/cardVersions';

  let { project }: { project: any } = $props();

  const on = $derived(setVersions(project.set));
  const own = $derived(project.set.cards.filter(hasOwnVersions).length);
  const total = $derived(totalVersions(project.set));
  const foilOn = $derived([...on].filter((k) => k.endsWith('_foil')).length);

  const PRESETS: [string, string, string[]][] = [
    ['all', 'All 12', ALL_VERSIONS],
    ['nofoil', 'No foil', BORDERS],
    ['base', 'Base only', ['Base']],
    ['basefoil', 'Base + foil', ['Base', 'Base_foil']]
  ];
  const same = (a: Set<string>, b: string[]) => a.size === b.length && b.every((k) => a.has(k));
  const preset = $derived(PRESETS.find(([, , keys]) => same(on, keys))?.[0] ?? 'custom');

  function toggle(k: string) {
    const next = new Set(on);
    if (next.has(k)) next.delete(k); else next.add(k);
    writeSetVersions(project.set, next);
  }

  function resetCards() {
    for (const c of project.set.cards) followSet(c);
  }
</script>

<div class="page-cols">
  <div class="block">
    <div class="bhead"><h2>Versions every card comes in</h2><span class="grow"></span>
      <div class="seg" role="group" aria-label="Presets">
        {#each PRESETS as [id, label, keys]}
          <button class:on={preset === id} onclick={() => writeSetVersions(project.set, new Set(keys))}>{label}</button>
        {/each}
        <button class:on={preset === 'custom'} disabled title="Click versions in the grid to make your own mix">Custom</button>
      </div>
    </div>
    <div><div class="gridwrap"><VersionGrid {on} ontoggle={toggle} /></div></div>
    {#if own}
      <div class="bfoot muted small">
        <span class="grow">{own} card{own === 1 ? ' has' : 's have'} their own list and ignore{own === 1 ? 's' : ''} this one.</span>
        <button class="small" onclick={resetCards} title="Those cards follow this list again">Make every card follow this</button>
      </div>
    {/if}
  </div>
  <aside class="page-side">
    <div class="block">
      <div class="facts">
        <div><span>Versions on</span><b>{on.size}<small> / 12</small></b></div>
        <div><span>Foil versions</span><b>{foilOn}<small> / 6</small></b></div>
        <div><span>Binder slots</span><b>{total.toLocaleString()}</b><small>over {project.set.cards.length.toLocaleString()} cards</small></div>
      </div>
    </div>
    <div class="notice info howto"><b>How versions work</b>
      <ul>
        <li>A version is a border × foil pair. Cards follow this list unless you give one its own (card panel → Versions).</li>
        <li>Off versions never come out of packs or from trade customers, and take no binder or Check Price slots.</li>
        <li>Cards a player already owns in an off version stay visible.</li>
        <li>Odds per border and foil are set per pack (Packs → Foil &amp; borders).</li>
      </ul>
    </div>
  </aside>
</div>

<style>
  .gridwrap { max-width: 640px; }
  .facts { display: grid; grid-template-columns: 1fr; }
  .facts > div { display: grid; grid-template-columns: 1fr auto; align-items: baseline; gap: 0 10px; padding: 8px 0; }
  .facts > div + div { border-top: 1px solid var(--line); }
  .facts span { color: var(--muted); font-size: 12px; }
  .facts b { font-size: 20px; font-weight: 600; font-variant-numeric: tabular-nums; }
  .facts b small { font-size: 12px; color: var(--muted); font-weight: 400; }
  .facts > div > small { grid-column: 1 / -1; color: var(--muted); font-size: 11px; text-align: right; }
</style>
