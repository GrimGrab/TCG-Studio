<script lang="ts">
  // A card's own details: rarity, number, the packs that give it, artist and description. Shared by the card panel and the
  // card editor (Card page).
  import { setRarities } from '../../lib/api';
  import { addToPack, inPack, packColour, removeFromPack } from '../../lib/packPools';

  let { project, card, notify }: { project: any; card: any; notify: (t: string, k?: string) => void } = $props();
</script>

<div class="fields">
  <div class="row">
    <label class="field grow">Rarity
      <select bind:value={card.rarity}>{#each setRarities(project.set) as r}<option value={r.id}>{r.name}</option>{/each}</select>
    </label>
    <label class="field" style="flex: 0 0 90px">Number<input bind:value={card.number} /></label>
  </div>
  {#if project.set.packs.length}
    <div class="field">In packs
      <div class="row packs">
        {#each project.set.packs as p, pi (p.id)}
          <label class="check" title={p.cards?.length ? `${p.cards.length} cards on its list` : 'Draws from the whole set: untick to give it a list of every other card'}>
            <input type="checkbox" checked={inPack(p, card.id)} onchange={(e) => {
              if (e.currentTarget.checked) addToPack(project.set, p, [card.id]);
              else {
                const why = removeFromPack(project.set, p, [card.id]);
                if (why) { e.currentTarget.checked = true; notify(why, 'error'); }
              }
            }} /> <span class="pdot" style="background:{packColour(pi)}"></span>{p.name}{p.cards?.length ? '' : ' (whole set)'}</label>
        {/each}
      </div>
      {#if !project.set.packs.some((p: any) => inPack(p, card.id))}<span class="muted small">No pack gives this card.</span>{/if}
    </div>
  {/if}
  <label class="field">Artist<input bind:value={card.artist} /></label>
  <label class="field">Description<textarea rows="4" bind:value={card.description}></textarea></label>
</div>

<style>
  .fields { display: flex; flex-direction: column; gap: 10px; }
  /* Narrow places (the card panel): fields share a row and shrink. */
  .row > .field { flex: 1 1 0; min-width: 0; }
  .field { min-width: 0; }
  .field > input, .field > select, .field > textarea { width: 100%; }
  .packs { flex-wrap: wrap; gap: 4px 14px; font-weight: normal; }
  .pdot { display: inline-block; width: 8px; height: 8px; border-radius: 50%; margin-right: 2px; }
</style>
