<script lang="ts">
  // A still card face for lists of many cards (the Cards tab's tiles): the picture over the whole card (or the border's box), the border's
  // image layers on top. No shaders run here (one WebGL view per tile would be too many); BorderPreview is the live one.
  // rim: the game's own rim sheet when the version has no border (BorderPreview's rim: the sheet is the card, the picture
  // inset to 90% on it); '' = the game draws none there (Full Art). Without either it is just the picture.
  import { borderFile, windowOf, type Border } from '../lib/borders';
  import { cardFit } from '../lib/cardLooks';

  // cardShaped: a full-image card (the picture is the card: made card-shaped like in the game); off = framed artwork, shown whole.
  let { src, border = null, rim, alt = '', cardShaped = true }: { src: string; border?: Border | null; rim?: string; alt?: string; cardShaped?: boolean } = $props();

  const win = $derived(windowOf(border));
  const pct = (v: number) => `${(v * 100).toFixed(3)}%`;

  // Every picture is card-shaped, like in the game (lib/cardLooks cardFit): scans stretched, anything else cropped.
  let wide = $state(false);
  const measure = (e: Event) => { const i = e.currentTarget as HTMLImageElement; wide = cardFit(i.naturalWidth / Math.max(1, i.naturalHeight)) === 'crop'; };
</script>

{#if border}
  <div class="face">
    <div class="win" style:left={pct(win[0])} style:top={pct(win[1])} style:width={pct(win[2])} style:height={pct(win[3])}>
      <!-- The picture at the whole card's size, scaled into the box: a box resizes it, never crops it differently. -->
      <div class="fit" style:width={pct(1 / win[2])} style:height={pct(1 / win[3])} style:transform="scale({win[2]}, {win[3]})">
        <img {src} {alt} loading="lazy" draggable="false" />
      </div>
    </div>
    {#each border.layers as layer, i (i)}
      {#if layer.kind === 'image' && layer.file}
        <img class="layer" src={borderFile(border, layer.file)} alt="" loading="lazy" draggable="false" style:opacity={layer.opacity ?? 1} />
      {/if}
    {/each}
  </div>
{:else if rim}
  <div class="face">
    <img class="layer" src={rim} alt="" loading="lazy" draggable="false" />
    <div class="inset"><img class="plain" class:wide {src} {alt} loading="lazy" draggable="false" onload={measure} /></div>
  </div>
{:else}
  <img class="plain" class:wide class:whole={!cardShaped} {src} {alt} loading="lazy" draggable="false" onload={measure} />
{/if}

<style>
  .face { position: absolute; inset: 0; }
  .win { position: absolute; overflow: hidden; }
  .fit { position: absolute; left: 0; top: 0; transform-origin: 0 0; }
  .win img { width: 100%; height: 100%; object-fit: cover; display: block; }
  .layer { position: absolute; inset: 0; width: 100%; height: 100%; object-fit: fill; pointer-events: none; }
  .inset { position: absolute; inset: 5%; }
  .plain { width: 100%; height: 100%; object-fit: fill; display: block; }
  .plain.wide { object-fit: cover; }
  .plain.whole { object-fit: contain; }
</style>
