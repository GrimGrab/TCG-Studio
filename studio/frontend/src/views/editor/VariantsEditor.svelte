<script lang="ts">
  // The set's card versions (set.json "variants"): the default for every card; a card can have its own list (card detail,
  // or several cards at once in the Cards tab). Versions that are off are never made by packs or trade customers and are left
  // out of the binder and Check Price — unless the player already owns one. All on = the field is left out.
  import VersionGrid from './VersionGrid.svelte';
  import { ALL_VERSIONS, followSet, hasOwnVersions, setVersions, totalVersions, writeSetVersions } from '../../lib/cardVersions';

  let { project }: { project: any } = $props();

  const on = $derived(setVersions(project.set));
  const own = $derived(project.set.cards.filter(hasOwnVersions).length);
  const total = $derived(totalVersions(project.set));

  function toggle(k: string) {
    const next = new Set(on);
    if (next.has(k)) next.delete(k); else next.add(k);
    writeSetVersions(project.set, next);
  }

  function resetCards() {
    for (const c of project.set.cards) followSet(c);
  }
</script>

<section class="block">
  <div class="bhead"><h2>Card versions</h2><span class="muted small">default for all cards · {on.size} of 12 on</span><span class="grow"></span>
    <button class="small" onclick={() => writeSetVersions(project.set, new Set(ALL_VERSIONS))} disabled={on.size === 12}>All on</button>
    <button class="small" onclick={() => writeSetVersions(project.set, new Set(['Base']))} disabled={on.size === 1 && on.has('Base')}>Base only</button></div>
  <p class="muted small">Which of the game's 12 versions (border × foil) this set's cards come in. A card can have its own list
    (select it in the Cards tab, or several cards to change them together); the others follow this one. Versions that are off never
    come out of packs or trade customers and don't take binder or Check Price slots — cards a player already owns in them stay
    visible.</p>
  <VersionGrid {on} ontoggle={toggle} />
  <div class="foot muted small">
    <span>The binder lists {total} versions over {project.set.cards.length} cards.</span>
    {#if own}<span class="grow"></span><span>{own} card{own === 1 ? ' has' : 's have'} their own versions</span>
      <button class="small" onclick={resetCards} title="Those cards follow this list again">Reset all to this default</button>{/if}
  </div>
</section>

<style>
  .foot { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; margin-top: 8px; }
</style>
