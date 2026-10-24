<script lang="ts">
  // A card back as the game shows it: the picture's crop at the card's shape inside the game's rounded edge. Without an own
  // picture it shows `fallback` (a back made earlier, the global back, or nothing = the frame template's back).
  import { loadImage } from '../lib/accessoryArt';
  import { drawCropped, latestPerFrame } from '../lib/shopLook';

  let { image = '', crop = null, aspect, fallback = '', caption = '' }: {
    image?: string;         // the own picture (source file), cut by crop
    crop?: number[] | null;
    aspect: number;         // width ÷ height of the card back
    fallback?: string;      // a finished back image shown when there is no own picture
    caption?: string;       // what the game uses when there's neither
  } = $props();

  let canvas = $state<HTMLCanvasElement>();
  let pic = $state<HTMLImageElement | null>(null);
  let fb = $state<HTMLImageElement | null>(null);
  $effect(() => {
    if (!image) { pic = null; return; }
    loadImage(image).then((i) => (pic = i)).catch(() => (pic = null));
  });
  $effect(() => {
    if (!fallback) { fb = null; return; }
    loadImage(fallback).then((i) => (fb = i)).catch(() => (fb = null));
  });

  const later = latestPerFrame();
  $effect(() => { later(draw.bind(null, canvas, pic, fb, crop, aspect)); });

  function draw(cv: HTMLCanvasElement | undefined, p: HTMLImageElement | null, f: HTMLImageElement | null, cr: number[] | null, a: number) {
    if (!cv) return;
    const W = 600, H = Math.round(W / a); // drawn at 600 px so the bigger preview stays sharp
    cv.width = W; cv.height = H;
    const g = cv.getContext('2d')!;
    // The game's rounded edge (about 4.5 % of the width, like its card mesh).
    const r = W * 0.045;
    g.beginPath();
    g.roundRect(0, 0, W, H, r);
    g.clip();
    if (p) drawCropped(g, p, cr, a, 0, 0, W, H);
    else if (f) g.drawImage(f, 0, 0, W, H);
    else {
      g.fillStyle = '#262a34';
      g.fillRect(0, 0, W, H);
    }
  }
</script>

<figure class="back">
  <canvas bind:this={canvas} style:aspect-ratio={aspect}></canvas>
  {#if !image && !fallback && caption}<span class="over muted">{caption}</span>{/if}
  <figcaption class="muted small">As the game shows it, inside its rounded edge</figcaption>
</figure>

<style>
  .back { position: relative; margin: 0; display: flex; flex-direction: column; gap: 6px; width: 100%; max-width: 400px; }
  canvas { display: block; width: 100%; border-radius: 12px; box-shadow: 0 6px 18px #0009; }
  .over { position: absolute; left: 0; right: 0; top: 40%; text-align: center; padding: 0 16px; font-size: 13px; }
</style>
