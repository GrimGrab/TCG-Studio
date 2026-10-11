<script lang="ts">
  // The phone's home screen as the game draws it: the wallpaper (custom crop or the vanilla one) under the app buttons, labels and
  // top bar. Layout and sprites come from the game files (templates shopsign/phone.json, gameextract/shoplook.go).
  import { loadImage } from '../lib/accessoryArt';
  import { drawCropped, latestPerFrame } from '../lib/shopLook';

  let { layoutUrl, image = '', crop = null, aspect }: {
    layoutUrl: string;
    image?: string;
    crop?: number[] | null;
    aspect: number;
  } = $props();

  type Part = { rect: number[]; image?: string; preserveAspect?: boolean; text?: string; fontSize?: number; color: number[] };
  let canvas = $state<HTMLCanvasElement>();
  let width = $state(0);
  let layout = $state<{ width: number; height: number; vanilla: string; parts: Part[] } | null>(null);
  let sprites = $state<Map<string, HTMLImageElement>>(new Map());
  let back = $state<HTMLImageElement | null>(null);
  const base = (u: string) => u.replace(/[^/]*$/, '');

  $effect(() => {
    const url = layoutUrl;
    if (!url) { layout = null; return; }
    (async () => {
      const lay = await (await fetch(url)).json();
      const files = [...new Set<string>(lay.parts.filter((p: Part) => p.image).map((p: Part) => p.image))];
      const imgs = await Promise.all(files.map((f) => loadImage(base(url) + f).catch(() => null)));
      sprites = new Map(files.flatMap((f, i) => (imgs[i] ? [[f, imgs[i]!] as [string, HTMLImageElement]] : [])));
      layout = lay;
    })().catch(() => (layout = null));
  });

  $effect(() => {
    const src = image || (layout ? base(layoutUrl) + layout.vanilla : '');
    if (!src) { back = null; return; }
    loadImage(src).then((i) => (back = i)).catch(() => (back = null));
  });

  // A sprite multiplied by an Image colour (uGUI tint), cached per sprite + colour.
  const tinted = new Map<string, HTMLCanvasElement>();
  function tint(img: HTMLImageElement, file: string, c: number[]) {
    const key = file + c.join(',');
    let cv = tinted.get(key);
    if (!cv) {
      cv = document.createElement('canvas'); cv.width = img.width; cv.height = img.height;
      const g = cv.getContext('2d')!;
      g.drawImage(img, 0, 0);
      g.globalCompositeOperation = 'multiply';
      g.fillStyle = `rgb(${c.slice(0, 3).map((v) => Math.round(v * 255)).join(',')})`;
      g.fillRect(0, 0, cv.width, cv.height);
      g.globalCompositeOperation = 'destination-in'; // back to the sprite's own alpha
      g.drawImage(img, 0, 0);
      tinted.set(key, cv);
    }
    return cv;
  }

  const later = latestPerFrame();
  $effect(() => { later(draw.bind(null, canvas, width, layout, sprites, back, image, crop)); });

  function draw(cv: HTMLCanvasElement | undefined, cssW: number, lay: typeof layout, spr: Map<string, HTMLImageElement>,
    bg: HTMLImageElement | null, img: string, cr: number[] | null) {
    if (!cv || !cssW) return;
    const dpr = window.devicePixelRatio || 1;
    const W = Math.round(cssW * dpr), H = Math.round(W / aspect);
    cv.width = W; cv.height = H;
    const g = cv.getContext('2d')!;
    g.fillStyle = '#222'; g.fillRect(0, 0, W, H);
    if (bg) {
      if (img) drawCropped(g, bg, cr, aspect, 0, 0, W, H); // stretched to the screen like the mod's sprite
      else g.drawImage(bg, 0, 0, W, H); // the vanilla sprite is stretched too
    }
    if (!lay) return;
    const k = W / lay.width;
    for (const p of lay.parts) {
      const [x, y, w, h] = p.rect;
      const cx = W / 2 + x * k, cy = H / 2 - y * k;
      let dw = w * k, dh = h * k;
      g.globalAlpha = p.color[3];
      if (p.image) {
        const s = spr.get(p.image);
        if (!s) continue;
        if (p.preserveAspect) {
          const f = Math.min(dw / s.width, dh / s.height);
          dw = s.width * f; dh = s.height * f;
        }
        g.drawImage(tint(s, p.image, p.color), cx - dw / 2, cy - dh / 2, dw, dh);
      } else if (p.text) {
        g.font = `bold ${(p.fontSize ?? 1) * k}px "Fredoka One", "Segoe UI", sans-serif`;
        g.textAlign = 'center'; g.textBaseline = 'middle';
        g.fillStyle = `rgb(${p.color.slice(0, 3).map((v) => Math.round(v * 255)).join(',')})`;
        g.fillText(p.text, cx, cy, dw);
      }
    }
    g.globalAlpha = 1;
  }
</script>

<div class="phone" bind:clientWidth={width}>
  <canvas bind:this={canvas} style:width="100%" style:aspect-ratio={aspect}></canvas>
  {#if !layoutUrl}<span class="muted small note">Read the game files (Settings → Game) to see the app buttons.</span>{/if}
</div>

<style>
  .phone { position: relative; width: 240px; flex: none; line-height: 0; border: 8px solid #111; border-radius: 22px; overflow: hidden; background: #111; }
  canvas { display: block; }
  .note { position: absolute; left: 8px; top: 8px; right: 8px; line-height: 1.2; color: #fff; }
</style>
