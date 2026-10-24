<script lang="ts">
  // The customers' credit card, its printed side at the card's shape: the picture fills the vanilla texture, and the chip and
  // embossed number/expiry/name are painted back when kept. Same areas and mask as the mod's Runtime/CreditCard.cs; layout:
  // templates shopsign/card.json.
  import { loadImage } from '../lib/accessoryArt';
  import { drawCropped, latestPerFrame } from '../lib/shopLook';

  let { layoutUrl, image = '', crop = null, aspect, keepChip = true, keepPrint = true }: {
    layoutUrl: string;
    image?: string;
    crop?: number[] | null;
    aspect: number;
    keepChip?: boolean;
    keepPrint?: boolean;
  } = $props();

  type Layout = { texture: string; aspect: number; chip: number[]; print: number[][] };
  let canvas = $state<HTMLCanvasElement>();
  let layout = $state<Layout | null>(null);
  let tex = $state<HTMLImageElement | null>(null);
  let pic = $state<HTMLImageElement | null>(null);
  const base = (u: string) => u.replace(/[^/]*$/, '');

  $effect(() => {
    const url = layoutUrl;
    if (!url) { layout = null; return; }
    (async () => {
      const lay = await (await fetch(url)).json();
      tex = await loadImage(base(url) + lay.texture);
      layout = lay;
    })().catch(() => (layout = null));
  });

  $effect(() => {
    if (!image) { pic = null; return; }
    loadImage(image).then((i) => (pic = i)).catch(() => (pic = null));
  });

  const later = latestPerFrame();
  $effect(() => { later(draw.bind(null, canvas, layout, tex, pic, crop, keepChip, keepPrint)); });

  function draw(cv: HTMLCanvasElement | undefined, lay: Layout | null, t: HTMLImageElement | null, p: HTMLImageElement | null,
    cr: number[] | null, chip: boolean, print: boolean) {
    if (!cv || !lay || !t) return;
    const W = t.width, H = t.height; // texture pixels; CSS stretches it to the card's shape like the mesh
    cv.width = W; cv.height = H;
    const g = cv.getContext('2d', { willReadFrequently: true })!; // read back for the mask: keep it on the CPU
    g.drawImage(t, 0, 0);
    if (!p) return;
    const vanilla = g.getImageData(0, 0, W, H);
    drawCropped(g, p, cr, aspect, 0, 0, W, H);
    const out = g.getImageData(0, 0, W, H);
    const areas = [...(chip ? [lay.chip] : []), ...(print ? lay.print : [])];
    const v = vanilla.data, d = out.data;
    for (const [ax, ay, aw, ah] of areas) {
      for (let y = ay; y < Math.min(H, ay + ah); y++) {
        for (let x = ax; x < Math.min(W, ax + aw); x++) {
          const i = (y * W + x) * 4;
          // The low-colour print (silver letters, their shadow, the chip) stays; the saturated blue card around it goes.
          const chroma = Math.max(v[i], v[i + 1], v[i + 2]) - Math.min(v[i], v[i + 1], v[i + 2]);
          const keep = Math.min(1, Math.max(0, (70 - chroma) / 30));
          for (let c = 0; c < 3; c++) d[i + c] = Math.round(d[i + c] * (1 - keep) + v[i + c] * keep);
        }
      }
    }
    g.putImageData(out, 0, 0);
  }
</script>

<div class="card">
  <canvas bind:this={canvas} style:aspect-ratio={aspect}></canvas>
  {#if !layoutUrl}<span class="muted small">Read the game files (Settings → Game) to see the preview.</span>{/if}
</div>

<style>
  .card { line-height: 0; }
  canvas { display: block; width: 100%; max-width: 560px; border-radius: 16px; box-shadow: 0 4px 14px #0008; }
  .muted { line-height: 1.2; }
</style>
