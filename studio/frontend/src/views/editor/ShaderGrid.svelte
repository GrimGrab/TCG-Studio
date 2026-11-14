<script lang="ts" module>
  /** The choice "use the set's shader" in a card's grid, and "the selected cards differ" in bulk edit. */
  export const FOLLOW = '~follow';
  export const MIXED = '~mixed';
</script>

<script lang="ts">
  // A card shader per version, as a grid like VersionGrid (rows = borders, columns = normal / foil). Used by the set's
  // default (Set tab), a card's own choices (card detail) and several cards at once (bulk edit). A cell's value is a
  // shader id, '' (no shader), FOLLOW (cards: the set's choice) or MIXED (bulk edit, shown only).
  import { BORDERS, BORDER_LABELS } from '../../lib/api';
  import { versionKey } from '../../lib/cardVersions';

  let { value, shaders, onchange, follow, dim }: {
    value: (key: string) => string; shaders: { id: string; name: string; builtIn?: boolean }[]; onchange: (key: string, v: string) => void;
    /** The label of the "leave it to the set / the default" choice (FOLLOW) for a version. */
    follow?: (key: string) => string;
    /** Versions the card or set doesn't have: shown faded. */
    dim?: (key: string) => boolean;
  } = $props();

  const known = $derived(new Set(shaders.map((s) => s.id)));
</script>

<div class="sgrid">
  <span></span><span class="colhead">Normal</span><span class="colhead">Foil</span>
  {#each BORDERS as b}
    <span class="rowhead">{BORDER_LABELS[b]}</span>
    {#each [false, true] as foil}
      {@const k = versionKey(b, foil)}
      {@const v = value(k)}
      <select class:set={v !== '' && v !== FOLLOW} class:dim={dim?.(k)} value={v} aria-label="{BORDER_LABELS[b]} {foil ? 'foil' : 'normal'} shader"
        onchange={(e) => onchange(k, e.currentTarget.value)}>
        {#if v === MIXED}<option value={MIXED} disabled>(different per card)</option>{/if}
        {#if follow}<option value={FOLLOW}>{follow(k)}</option>{/if}
        <option value="">No shader</option>
        {#each shaders.filter((s) => !s.builtIn) as s}<option value={s.id}>{s.name || 'Unnamed'}</option>{/each}
        <optgroup label="Comes with the mod">
          {#each shaders.filter((s) => s.builtIn) as s}<option value={s.id}>{s.name}</option>{/each}
        </optgroup>
        {#if v && v !== FOLLOW && v !== MIXED && !known.has(v)}<option value={v}>Missing shader ({v})</option>{/if}
      </select>
    {/each}
  {/each}
</div>

<style>
  .sgrid { display: grid; grid-template-columns: minmax(90px, auto) 1fr 1fr; gap: 4px 6px; align-items: center; }
  .colhead { font-size: 12px; color: var(--muted); text-align: center; }
  .rowhead { font-size: 13px; }
  select { min-width: 0; font-size: 12px; padding: 3px 4px; color: var(--muted); }
  select.set { color: var(--text); border-color: var(--accent-2); }
  select.dim { opacity: 0.45; }
</style>
