<script lang="ts">
  // The 12 card versions as a grid (rows = borders, columns = normal / foil). Used by the set's default (Set tab), a card's
  // own versions (card detail) and several cards at once (bulk edit). `mixed` = some of the selected cards have it.
  // `extra` adds content under a cell's button (the card detail's picture per version).
  import type { Snippet } from 'svelte';
  import { BORDERS, BORDER_LABELS } from '../../lib/api';
  import { versionKey } from '../../lib/cardVersions';

  let { on, mixed = new Set<string>(), ontoggle, keepOne = true, extra }: {
    on: Set<string>; mixed?: Set<string>; ontoggle: (key: string) => void; keepOne?: boolean; extra?: Snippet<[string]>;
  } = $props();
</script>

<div class="vgrid">
  <span></span><span class="colhead">Normal</span><span class="colhead">Foil</span>
  {#each BORDERS as b}
    <span class="rowhead">{BORDER_LABELS[b]}</span>
    {#each [false, true] as foil}
      {@const k = versionKey(b, foil)}
      {@const isOn = on.has(k)}
      {@const isMixed = !isOn && mixed.has(k)}
      <div class="cell">
        <button class="v" class:on={isOn} class:mixed={isMixed} class:foil disabled={keepOne && on.size === 1 && isOn}
          onclick={() => ontoggle(k)} aria-pressed={isOn} aria-label="{BORDER_LABELS[b]} {foil ? 'foil' : 'normal'}">
          {isOn ? '✓ On' : isMixed ? '◐ Some' : 'Off'}</button>
        {#if extra}{@render extra(k)}{/if}
      </div>
    {/each}
  {/each}
</div>

<style>
  .vgrid { display: grid; grid-template-columns: minmax(90px, auto) 1fr 1fr; gap: 4px 6px; align-items: center; }
  .colhead { font-size: 12px; color: var(--muted); text-align: center; }
  .rowhead { font-size: 13px; }
  .cell { display: flex; align-items: center; gap: 4px; min-width: 0; }
  .v { flex: 1 1 0; min-width: 0; padding: 4px 0; font-size: 12px; color: var(--muted); background: var(--bg); }
  .v.on { color: var(--text); background: #1c2740; border-color: var(--accent-2); }
  .v.on.foil { background: linear-gradient(120deg, #1c2740, #2e2148 55%, #1d3a3a); }
  .v.mixed { color: var(--text); border-style: dashed; border-color: var(--accent-2); }
</style>
