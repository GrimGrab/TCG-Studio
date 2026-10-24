<script lang="ts">
  // One side of the open/closed sign at the door, at the texture's own resolution: the vanilla board with the picture painted inside
  // its black frame, keeping the frame and screws. Same mask as the mod's Runtime/DoorSign.cs. Layout: templates shopsign/door.json.
  import { loadImage } from '../lib/accessoryArt';
  import { drawCropped, latestPerFrame } from '../lib/shopLook';

  let { layoutUrl, side, image = '', crop = null, aspect, drawn = null }: {
    layoutUrl: string;
    side: 'open' | 'closed';
    image?: string;
    crop?: number[] | null;
    aspect: number;
    drawn?: HTMLCanvasElement | null; // a board drawn in the page (door text draft), shown instead of the picture
  } = $props();

  let canvas = $state<HTMLCanvasElement>();
  let layout = $state<{ texture: string; boards: Record<string, number[]>; screws: number[][] } | null>(null);
  let tex = $state<HTMLImageElement | null>(null);
  let pic = $state<HTMLImageElement | null>(null);
  const base = (u: string) => u.replace(/[^/]*$/, '');
  const Frame = 10 / 512; // texture shown around the board (its black frame)

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
  $effect(() => { later(draw.bind(null, canvas, layout, tex, drawn ?? pic, crop)); });

  function draw(cv: HTMLCanvasElement | undefined, lay: typeof layout, t: HTMLImageElement | null, p: HTMLImageElement | HTMLCanvasElement | null,
    cr: number[] | null) {
    if (!cv || !lay || !t) return;
    const [bx, by, bw, bh] = lay.boards[side];
    const tw = t.width, th = t.height;
    const rx = Math.round((bx - Frame) * tw), ry = Math.round((by - Frame) * th);
    const W = Math.round((bw + 2 * Frame) * tw), H = Math.round((bh + 2 * Frame) * th);
    cv.width = W; cv.height = H;
    const g = cv.getContext('2d', { willReadFrequently: true })!; // read back for the mask: keep it on the CPU
    g.drawImage(t, rx, ry, W, H, 0, 0, W, H);
    if (!p) return;
    const ox = Math.round(bx * tw) - rx, oy = Math.round(by * th) - ry, pw = Math.round(bw * tw), ph = Math.round(bh * th);
    const pc = document.createElement('canvas'); pc.width = pw; pc.height = ph;
    const pg = pc.getContext('2d', { willReadFrequently: true })!;
    if (p instanceof HTMLImageElement) drawCropped(pg, p, cr, aspect, 0, 0, pw, ph);
    else pg.drawImage(p, 0, 0, pw, ph); // already board-shaped
    const src = pg.getImageData(0, 0, pw, ph).data;
    const out = g.getImageData(ox, oy, pw, ph);
    const d = out.data;
    for (let y = 0; y < ph; y++) {
      for (let x = 0; x < pw; x++) {
        const i = (y * pw + x) * 4;
        // Keep the dark frame (its rounded inner corners reach into the board) and the screw heads.
        const luma = (0.3 * d[i] + 0.59 * d[i + 1] + 0.11 * d[i + 2]) / 255;
        let keep = Math.min(1, Math.max(0, (luma - 0.1) / 0.15));
        for (const [sx, sy, sr] of lay.screws) {
          const dist = Math.hypot(x + 0.5 - sx * pw, y + 0.5 - sy * ph);
          keep = Math.min(keep, Math.min(1, Math.max(0, dist - sr * tw + 1)));
        }
        const a = (src[i + 3] / 255) * keep;
        for (let c = 0; c < 3; c++) d[i + c] = Math.round(d[i + c] * (1 - a) + src[i + c] * a);
      }
    }
    g.putImageData(out, ox, oy);
  }
</script>

<div class="door">
  <canvas bind:this={canvas}></canvas>
  {#if !layoutUrl}<span class="muted small">Read the game files (Settings → Game) to see the preview.</span>{/if}
</div>

<style>
  .door { line-height: 0; }
  canvas { display: block; width: 100%; max-width: 560px; image-rendering: auto; border-radius: 6px; }
  .muted { line-height: 1.2; }
</style>
