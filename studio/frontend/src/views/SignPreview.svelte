<script lang="ts">
  import { loadImage } from '../lib/accessoryArt';
  import { drawCropped, fillOutlined, fontFamily, latestPerFrame } from '../lib/shopLook';
  import { drawSdfText } from '../lib/sdfText';
  // The shop sign as the game draws it (daylight): banner (custom crop or the vanilla art) plus the shop name in its font, colour,
  // size and outline. Layout, default font (Fredoka One as a TMP SDF atlas) and vanilla art come from the game files
  // (templates shopsign/sign.json). Mirrors the mod's Runtime/ShopSign.cs: TMP auto size in the name's rect, which grows in
  // height with the size setting; outline = fraction of the font size, half inside and half outside the letter edge.
  let { layoutUrl, image = '', crop = null, aspect, showName = true, name, style }: {
    layoutUrl: string;
    image?: string;
    crop?: number[] | null;
    aspect: number;
    showName?: boolean;
    name: string;
    style: { fontUrl: string; color: string; size: number; outline: number; outlineColor: string };
  } = $props();

  let canvas = $state<HTMLCanvasElement>();
  let width = $state(0);
  let layout = $state<any>(null);
  let atlas: { data: Uint8ClampedArray; w: number; h: number; img: HTMLImageElement } | null = null;
  let bg = $state<HTMLImageElement | null>(null);
  let family = $state(''); // loaded FontFace family for style.fontUrl, '' = the default SDF font

  const base = (u: string) => u.replace(/[^/]*$/, '');

  $effect(() => {
    const url = layoutUrl;
    (async () => {
      const lay = await (await fetch(url)).json();
      const img = await loadImage(base(url) + lay.defaultAtlas);
      const c = document.createElement('canvas'); c.width = img.width; c.height = img.height;
      const g = c.getContext('2d')!; g.drawImage(img, 0, 0);
      atlas = { data: g.getImageData(0, 0, img.width, img.height).data, w: img.width, h: img.height, img };
      layout = lay;
    })().catch(() => (layout = null));
  });

  $effect(() => {
    const src = image || (layout?.vanilla ? base(layoutUrl) + layout.vanilla : '');
    if (!src) { bg = null; return; }
    loadImage(src).then((i) => (bg = i)).catch(() => (bg = null));
  });

  $effect(() => {
    const url = style.fontUrl;
    if (!url) { family = ''; return; }
    fontFamily(url).then((f) => (family = f)).catch(() => (family = ''));
  });

  const rgb = (c: number[], k = 1) => `rgb(${c.slice(0, 3).map((v) => Math.round(Math.min(1, v * k) * 255)).join(',')})`;
  const hex = (s: string) => {
    const m = /^#?([0-9a-f]{6})$/i.exec(s?.trim() ?? '');
    if (!m) return null;
    const n = parseInt(m[1], 16);
    return [(n >> 16) / 255, ((n >> 8) & 255) / 255, (n & 255) / 255, 1];
  };

  const later = latestPerFrame();
  $effect(() => { later(draw.bind(null, canvas, width, layout, bg, family, image, crop, showName, name, style.color, style.size, style.outline, style.outlineColor)); });

  function draw(cv: HTMLCanvasElement | undefined, cssW: number, lay: any, back: HTMLImageElement | null, fam: string, img: string,
    cr: number[] | null, show: boolean, text: string, color: string, size: number, outline: number, outlineColor: string) {
    if (!cv || !cssW) return;
    const dpr = window.devicePixelRatio || 1;
    const W = Math.round(cssW * dpr), H = Math.round(W / aspect);
    cv.width = W; cv.height = H;
    const g = cv.getContext('2d')!;
    g.fillStyle = '#333'; g.fillRect(0, 0, W, H);
    if (back) {
      if (img) drawCropped(g, back, cr, aspect, 0, 0, W, H); // the custom image's crop box, stretched to the sign like the mod
      else g.drawImage(back, 0, 0, W, H);
    }
    if (!lay || !show || !text) return;

    const t = lay.text, f = lay.defaultFont;
    const k = W / lay.faceWidth; // canvas px per UI unit
    const cx = W / 2 + t.x * k, cy = H / 2 - t.y * k;
    const boxW = t.w * k, boxH = t.h * k * Math.max(1, size);
    const custom = hex(color);
    const outCol = hex(outlineColor) ?? [0, 0, 0, 1];

    // Measure at 100 px, then auto size like TMP: the biggest size up to sizeMax that fits the rect, not below sizeMin.
    let adv100: number, asc100: number, desc100: number;
    const glyphs = new Map<number, any>((f.glyphs as any[]).map((gl) => [gl.u, gl]));
    if (fam) {
      g.font = `100px "${fam}"`;
      const m = g.measureText(text);
      adv100 = m.width; asc100 = m.fontBoundingBoxAscent; desc100 = m.fontBoundingBoxDescent;
    } else {
      adv100 = [...text].reduce((s, ch) => s + (glyphs.get(ch.codePointAt(0)!)?.adv ?? f.glyphs[0].adv), 0) * 100 / f.pointSize;
      asc100 = f.ascent * 100 / f.pointSize; desc100 = -f.descent * 100 / f.pointSize;
    }
    let px = t.sizeMax * size * k;
    if (t.autoSize) {
      px = Math.min(px, (boxW / adv100) * 100, (boxH / (asc100 + desc100)) * 100);
      px = Math.max(px, t.sizeMin * size * k);
    } else px = t.fontSize * size * k;
    const s = px / 100;
    const left = cx - (adv100 * s) / 2; // centred
    const baseline = cy + ((asc100 - desc100) * s) / 2; // middle of ascender..descender
    const top = baseline - asc100 * s, bottom = baseline + desc100 * s;
    const fill = (ctx: CanvasRenderingContext2D) => {
      if (custom) return rgb(custom);
      if (!t.gradient) return rgb(t.color);
      const gr = ctx.createLinearGradient(0, top, 0, bottom); // TMP vertex gradient (per letter) × the gold
      const [tl, tr, bl, br] = t.gradientColors;
      gr.addColorStop(0, rgb(t.color.map((v: number, i: number) => v * (tl[i] + tr[i]) / 2)));
      gr.addColorStop(1, rgb(t.color.map((v: number, i: number) => v * (bl[i] + br[i]) / 2)));
      return gr;
    };

    if (fam) {
      g.font = `${px}px "${fam}"`;
      g.textBaseline = 'alphabetic';
      g.fillStyle = fill(g);
      fillOutlined(g, text, left, baseline, px, outline, rgb(outCol));
      return;
    }
    if (!atlas) return;
    // The game's font from its SDF atlas (lib/sdfText); TMP's vertex gradient per row × the gold, unless a colour was chosen.
    const grad = custom ? null : t.gradient ? t.gradientColors : null;
    const rowColor = (v: number) => custom ?? (grad
      ? t.color.map((c: number, i: number) => c * ((grad[0][i] + grad[1][i]) / 2 * (1 - v) + (grad[2][i] + grad[3][i]) / 2 * v))
      : t.color);
    // The sign's outline is a fraction of the font size, half inside and half outside the letter edge: in TMP's width.
    drawSdfText(g, f, atlas, text, cx, cy, { px, color: rowColor, outline: (outline * f.pointSize) / (2 * f.gradientScale), outlineColor: outCol });
  }
</script>

<div class="preview" bind:clientWidth={width}>
  <canvas bind:this={canvas} style:width="100%" style:aspect-ratio={aspect}></canvas>
  {#if !layoutUrl}<span class="muted small note">Read the game files (Settings → Game) to see the preview.</span>{/if}
</div>

<style>
  .preview { position: relative; max-width: 1400px; line-height: 0; }
  canvas { display: block; border-radius: 6px; }
  .note { position: absolute; left: 8px; top: 8px; line-height: 1.2; }
</style>
