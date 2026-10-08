<script lang="ts">
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
  let fontCounter = 0;

  const loadImage = (src: string) => new Promise<HTMLImageElement>((ok, fail) => {
    const i = new Image(); i.onload = () => ok(i); i.onerror = fail; i.src = src;
  });
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
    const fam = `tcgsign${++fontCounter}`;
    new FontFace(fam, `url("${url}")`).load().then((f) => { document.fonts.add(f); family = fam; }).catch(() => (family = ''));
  });

  const rgb = (c: number[], k = 1) => `rgb(${c.slice(0, 3).map((v) => Math.round(Math.min(1, v * k) * 255)).join(',')})`;
  const hex = (s: string) => {
    const m = /^#?([0-9a-f]{6})$/i.exec(s?.trim() ?? '');
    if (!m) return null;
    const n = parseInt(m[1], 16);
    return [(n >> 16) / 255, ((n >> 8) & 255) / 255, (n & 255) / 255, 1];
  };

  $effect(() => { draw(canvas, width, layout, bg, family, image, crop, showName, name, style.color, style.size, style.outline, style.outlineColor); });

  function draw(cv: HTMLCanvasElement | undefined, cssW: number, lay: any, back: HTMLImageElement | null, fam: string, img: string,
    cr: number[] | null, show: boolean, text: string, color: string, size: number, outline: number, outlineColor: string) {
    if (!cv || !cssW) return;
    const dpr = window.devicePixelRatio || 1;
    const W = Math.round(cssW * dpr), H = Math.round(W / aspect);
    cv.width = W; cv.height = H;
    const g = cv.getContext('2d')!;
    g.fillStyle = '#333'; g.fillRect(0, 0, W, H);
    if (back) {
      if (img) {
        // The custom image: its crop box (or the largest sign-shaped box in the middle), stretched to the sign like the mod.
        let [x, y, w, h] = cr && cr.length === 4 ? cr : [0, 0, 0, 0];
        if (!(cr && cr.length === 4)) {
          const ia = back.width / back.height;
          if (ia > aspect) { w = aspect / ia; h = 1; x = (1 - w) / 2; y = 0; } else { w = 1; h = ia / aspect; x = 0; y = (1 - h) / 2; }
        } else h = (w * back.width) / (back.height * aspect);
        g.drawImage(back, x * back.width, y * back.height, w * back.width, h * back.height, 0, 0, W, H);
      } else g.drawImage(back, 0, 0, W, H);
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
      g.fillText(text, left, baseline);
      if (outline > 0) {
        g.lineJoin = 'round'; g.lineWidth = outline * px; g.strokeStyle = rgb(outCol);
        g.strokeText(text, left, baseline);
      }
      return;
    }
    if (!atlas) return;
    // The game's font: each pixel takes the highest SDF value of the letters covering it (padded quads overlap), then is
    // thresholded like TMP's shader.
    const A = atlas;
    const pad = f.gradientScale - 1, sc = px / f.pointSize; // canvas px per atlas px
    const x0 = Math.max(0, Math.floor(left - pad * sc)), y0 = Math.max(0, Math.floor(top - pad * sc));
    const x1 = Math.min(W, Math.ceil(left + adv100 * s + pad * sc)), y1 = Math.min(H, Math.ceil(bottom + pad * sc));
    if (x1 <= x0 || y1 <= y0) return;
    type Quad = { l: number; t: number; r: number; b: number; ax: number; ay: number };
    const quads: Quad[] = [];
    let pen = left;
    for (const ch of text) {
      const gl = glyphs.get(ch.codePointAt(0)!) ?? f.glyphs[0];
      const [rx, ry, rw, rh] = gl.rect;
      if (rw > 0 && rh > 0) {
        const l = pen + (gl.bx - pad) * sc, tq = baseline - (gl.by + pad) * sc;
        quads.push({ l, t: tq, r: l + (rw + 2 * pad) * sc, b: tq + (rh + 2 * pad) * sc, ax: rx - pad, ay: A.h - (ry + rh) - pad });
      }
      pen += gl.adv * sc;
    }
    const sample = (x: number, y: number) => { // bilinear alpha of the atlas (top-down pixel coords)
      const fx = Math.min(Math.max(x - 0.5, 0), A.w - 1), fy = Math.min(Math.max(y - 0.5, 0), A.h - 1);
      const ix = Math.min(Math.floor(fx), A.w - 2), iy = Math.min(Math.floor(fy), A.h - 2), dx = fx - ix, dy = fy - iy;
      const at = (xx: number, yy: number) => A.data[(yy * A.w + xx) * 4 + 3];
      return ((at(ix, iy) * (1 - dx) + at(ix + 1, iy) * dx) * (1 - dy) + (at(ix, iy + 1) * (1 - dx) + at(ix + 1, iy + 1) * dx) * dy) / 255;
    };
    const sdf = document.createElement('canvas'); sdf.width = x1 - x0; sdf.height = y1 - y0;
    const sg = sdf.getContext('2d')!;
    const out = new ImageData(sdf.width, sdf.height);
    const spread = 2 * f.gradientScale * sc; // canvas px per unit of SDF value
    const half = (outline * f.pointSize) / (2 * f.gradientScale) / 2; // TMP _OutlineWidth / 2, in SDF value
    const grad = custom ? null : t.gradient ? t.gradientColors : null;
    for (let y = 0; y < sdf.height; y++) {
      const v = Math.min(1, Math.max(0, (y + y0 - top) / Math.max(1, bottom - top)));
      const faceRGB = custom ?? (grad
        ? t.color.map((c: number, i: number) => c * ((grad[0][i] + grad[1][i]) / 2 * (1 - v) + (grad[2][i] + grad[3][i]) / 2 * v))
        : t.color);
      const py = y + y0 + 0.5;
      for (let x = 0; x < sdf.width; x++) {
        const i = (y * sdf.width + x) * 4;
        const pxx = x + x0 + 0.5;
        let a = 0;
        for (const q of quads)
          if (pxx >= q.l && pxx < q.r && py >= q.t && py < q.b) a = Math.max(a, sample(q.ax + (pxx - q.l) / sc, q.ay + (py - q.t) / sc));
        const faceCov = Math.min(1, Math.max(0, (a - 0.5 - half) * spread + 0.5));
        const allCov = Math.min(1, Math.max(0, (a - 0.5 + half) * spread + 0.5));
        if (allCov <= 0) continue;
        for (let c = 0; c < 3; c++) out.data[i + c] = Math.round(Math.min(1, outCol[c] * (1 - faceCov) + faceRGB[c] * faceCov) * 255);
        out.data[i + 3] = Math.round(allCov * 255);
      }
    }
    sg.putImageData(out, 0, 0);
    g.drawImage(sdf, x0, y0);
  }
</script>

<div class="preview" bind:clientWidth={width}>
  <canvas bind:this={canvas} style:width="100%" style:aspect-ratio={aspect}></canvas>
  {#if !layoutUrl}<span class="muted small note">Read the game files (Settings → Game) to see the preview.</span>{/if}
</div>

<style>
  .preview { position: relative; max-width: 820px; line-height: 0; }
  canvas { display: block; border-radius: 6px; }
  .note { position: absolute; left: 8px; top: 8px; line-height: 1.2; }
</style>
