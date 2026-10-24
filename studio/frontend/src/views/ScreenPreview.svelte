<script lang="ts">
  // A game screen drawn over its background picture (custom crop or the vanilla one): the phone's home screen with its app
  // buttons, the main menu with its logo and buttons, the loading screen with its text. Layout and sprites come from the game
  // files (templates shopsign/phone.json, menu.json, loading.json; gameextract/shoplook.go).
  import { loadImage } from '../lib/accessoryArt';
  import { drawCropped, latestPerFrame } from '../lib/shopLook';

  let { layoutUrl, image = '', crop = null, aspect, frame = 'screen', card = { show: 'outline' } }: {
    layoutUrl: string;
    image?: string;
    crop?: number[] | null;
    aspect: number; // of the picture and the screen
    frame?: 'phone' | 'screen';
    // What a card the game draws on the screen shows (the loading screen's spinning card): its outline, a picture cut to
    // the card's shape, or nothing.
    card?: { show: 'outline' | 'picture' | 'none'; image?: string; crop?: number[] | null; aspect?: number; label?: string };
  } = $props();

  type Part = { rect: number[]; image?: string; preserveAspect?: boolean; sliced?: boolean; text?: string; fontSize?: number;
    color: number[]; card?: boolean };
  type Layout = { width: number; height: number; vanilla?: string; vanillaColor: number[]; parts: Part[] };
  let canvas = $state<HTMLCanvasElement>();
  let width = $state(0);
  let layout = $state<Layout | null>(null);
  let sprites = $state<Map<string, HTMLImageElement>>(new Map());
  let back = $state<HTMLImageElement | null>(null);
  const base = (u: string) => u.replace(/[^/]*$/, '');

  $effect(() => {
    const url = layoutUrl;
    if (!url) { layout = null; return; }
    (async () => {
      const lay = await (await fetch(url)).json();
      const files = [...new Set<string>(lay.parts.filter((p: Part) => p.image && !p.sliced).map((p: Part) => p.image))];
      const imgs = await Promise.all(files.map((f) => loadImage(base(url) + f).catch(() => null)));
      sprites = new Map(files.flatMap((f, i) => (imgs[i] ? [[f, imgs[i]!] as [string, HTMLImageElement]] : [])));
      layout = lay;
    })().catch(() => (layout = null));
  });

  let cardPic = $state<HTMLImageElement | null>(null);
  $effect(() => {
    const src = card.show === 'picture' ? card.image ?? '' : '';
    if (!src) { cardPic = null; return; }
    loadImage(src).then((i) => (cardPic = i)).catch(() => (cardPic = null));
  });

  $effect(() => {
    const src = image || (layout?.vanilla ? base(layoutUrl) + layout.vanilla : '');
    if (!src) { back = null; return; }
    loadImage(src).then((i) => (back = i)).catch(() => (back = null));
  });

  const rgb = (c: number[]) => `rgb(${c.slice(0, 3).map((v) => Math.round(v * 255)).join(',')})`;

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
      g.fillStyle = rgb(c);
      g.fillRect(0, 0, cv.width, cv.height);
      g.globalCompositeOperation = 'destination-in'; // back to the sprite's own alpha
      g.drawImage(img, 0, 0);
      tinted.set(key, cv);
    }
    return cv;
  }

  function roundRect(g: CanvasRenderingContext2D, x: number, y: number, w: number, h: number, r: number) {
    g.beginPath();
    g.roundRect(x, y, w, h, Math.min(r, w / 2, h / 2));
  }

  const later = latestPerFrame();
  $effect(() => { later(draw.bind(null, canvas, width, layout, sprites, back, image, crop, { ...card }, cardPic)); });

  function draw(cv: HTMLCanvasElement | undefined, cssW: number, lay: Layout | null, spr: Map<string, HTMLImageElement>,
    bg: HTMLImageElement | null, img: string, cr: number[] | null, cd: typeof card, cp: HTMLImageElement | null) {
    if (!cv || !cssW) return;
    const dpr = window.devicePixelRatio || 1;
    const W = Math.round(cssW * dpr), H = Math.round(W / aspect);
    cv.width = W; cv.height = H;
    const g = cv.getContext('2d')!;
    g.fillStyle = lay && !img && !lay.vanilla ? rgb(lay.vanillaColor) : '#222';
    g.fillRect(0, 0, W, H);
    if (bg) {
      if (img) drawCropped(g, bg, cr, aspect, 0, 0, W, H); // the picture's crop fills the screen like the mod's sprite
      else g.drawImage(bg, 0, 0, W, H); // the vanilla sprite is stretched too
    }
    if (!lay) return;
    const k = W / lay.width;
    for (const p of lay.parts) {
      const [x, y, w, h] = p.rect;
      const cx = W / 2 + x * k, cy = H / 2 - y * k;
      let dw = w * k, dh = h * k;
      g.globalAlpha = p.card ? 1 : p.color[3];
      if (p.card) {
        // A card the game draws here (the loading screen's spinning card): a picture cut to its shape, or its outline.
        if (cd.show === 'none') continue;
        const ch = dh * 0.87, cw = ch * 0.716;
        roundRect(g, cx - cw / 2, cy - ch / 2, cw, ch, cw * 0.06);
        if (cd.show === 'picture' && cp) {
          g.save(); g.clip();
          drawCropped(g, cp, cd.crop, cd.aspect ?? 0.716, cx - cw / 2, cy - ch / 2, cw, ch);
          g.restore();
          continue;
        }
        g.fillStyle = 'rgba(255,255,255,0.12)'; g.fill();
        g.lineWidth = Math.max(1, 2 * dpr); g.strokeStyle = 'rgba(255,255,255,0.6)'; g.stroke();
        if (cd.label) {
          g.font = `bold ${Math.max(10, cw * 0.11)}px "Segoe UI", sans-serif`;
          g.textAlign = 'center'; g.textBaseline = 'middle'; g.fillStyle = 'rgba(255,255,255,0.85)';
          g.fillText(cd.label, cx, cy, cw * 0.9);
        }
      } else if (p.sliced) {
        roundRect(g, cx - dw / 2, cy - dh / 2, dw, dh, Math.min(dw, dh) * 0.18); // a 9-sliced panel
        g.fillStyle = rgb(p.color); g.fill();
      } else if (p.image) {
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
        g.fillStyle = rgb(p.color);
        g.fillText(p.text, cx, cy, dw);
      }
    }
    g.globalAlpha = 1;
  }
</script>

<div class="frame {frame}" bind:clientWidth={width}>
  <canvas bind:this={canvas} style:width="100%" style:aspect-ratio={aspect}></canvas>
  {#if !layoutUrl}<span class="muted small note">Read the game files (Settings → Game) to see the game's buttons and text on top.</span>{/if}
</div>

<style>
  .frame { position: relative; flex: none; line-height: 0; overflow: hidden; background: #111; }
  .phone { width: clamp(240px, 22vw, 340px); border: 8px solid #111; border-radius: 22px; }
  .screen { width: 100%; max-width: 1100px; border: 6px solid #111; border-radius: 8px; }
  canvas { display: block; }
  .note { position: absolute; left: 8px; top: 8px; right: 8px; line-height: 1.2; color: #fff; }
</style>
