// Text in one of the game's TextMeshPro fonts, drawn from its SDF atlas (read from the game files) the way TMP's shader
// does: each pixel takes the highest distance value of the letters covering it, then face and outline are thresholded.
// Used by the shop sign preview (its default font) and the edition label on card previews.

export type TmpGlyph = { u: number; w: number; h: number; bx: number; by: number; adv: number; rect: [number, number, number, number] };
export type TmpFont = { name: string; pointSize: number; ascent: number; descent: number; glyphs: TmpGlyph[]; atlasWidth: number; atlasHeight: number; gradientScale: number };
/** The atlas's alpha (top-down pixel rows). */
export type SdfAtlas = { data: Uint8ClampedArray; w: number; h: number };

export async function loadAtlas(url: string): Promise<SdfAtlas> {
  const img = new Image();
  img.src = url;
  await img.decode();
  const c = document.createElement('canvas');
  c.width = img.width; c.height = img.height;
  const g = c.getContext('2d')!;
  g.drawImage(img, 0, 0);
  return { data: g.getImageData(0, 0, img.width, img.height).data, w: img.width, h: img.height };
}

export type SdfStyle = {
  /** Font size in canvas pixels. */
  px: number;
  /** Face colour, 0..1; or one per height (v = 0 at the text's top, 1 at its bottom: a vertical gradient). */
  color: number[] | ((v: number) => number[]);
  /** TMP outline width (0..1 of the distance range) and colour (0..1, with alpha). */
  outline: number;
  outlineColor: number[];
};

/** Draws text centred on (cx, cy) (the middle of ascender..descender), like TMP's centred alignment. */
export function drawSdfText(g: CanvasRenderingContext2D, font: TmpFont, A: SdfAtlas, text: string, cx: number, cy: number, st: SdfStyle) {
  const glyphs = new Map(font.glyphs.map((gl) => [gl.u, gl]));
  const sc = st.px / font.pointSize; // canvas px per font unit (= atlas px)
  const adv = [...text].reduce((s, ch) => s + (glyphs.get(ch.codePointAt(0)!)?.adv ?? font.glyphs[0].adv), 0) * sc;
  const asc = font.ascent * sc, desc = -font.descent * sc;
  const left = cx - adv / 2, baseline = cy + (asc - desc) / 2;
  const top = baseline - asc, bottom = baseline + desc;
  const pad = font.gradientScale - 1;
  const W = g.canvas.width, H = g.canvas.height;
  const x0 = Math.max(0, Math.floor(left - pad * sc)), y0 = Math.max(0, Math.floor(top - pad * sc));
  const x1 = Math.min(W, Math.ceil(left + adv + pad * sc)), y1 = Math.min(H, Math.ceil(bottom + pad * sc));
  if (x1 <= x0 || y1 <= y0) return;
  type Quad = { l: number; t: number; r: number; b: number; ax: number; ay: number };
  const quads: Quad[] = [];
  let pen = left;
  for (const ch of text) {
    const gl = glyphs.get(ch.codePointAt(0)!) ?? font.glyphs[0];
    const [rx, ry, rw, rh] = gl.rect;
    if (rw > 0 && rh > 0) {
      const l = pen + (gl.bx - pad) * sc, tq = baseline - (gl.by + pad) * sc;
      quads.push({ l, t: tq, r: l + (rw + 2 * pad) * sc, b: tq + (rh + 2 * pad) * sc, ax: rx - pad, ay: A.h - (ry + rh) - pad });
    }
    pen += gl.adv * sc;
  }
  const sample = (x: number, y: number) => { // bilinear alpha of the atlas
    const fx = Math.min(Math.max(x - 0.5, 0), A.w - 1), fy = Math.min(Math.max(y - 0.5, 0), A.h - 1);
    const ix = Math.min(Math.floor(fx), A.w - 2), iy = Math.min(Math.floor(fy), A.h - 2), dx = fx - ix, dy = fy - iy;
    const at = (xx: number, yy: number) => A.data[(yy * A.w + xx) * 4 + 3];
    return ((at(ix, iy) * (1 - dx) + at(ix + 1, iy) * dx) * (1 - dy) + (at(ix, iy + 1) * (1 - dx) + at(ix + 1, iy + 1) * dx) * dy) / 255;
  };
  const out = new ImageData(x1 - x0, y1 - y0);
  const spread = 2 * font.gradientScale * sc; // canvas px per unit of distance value
  const half = st.outline / 2;
  const oa = st.outlineColor[3] ?? 1;
  const colorAt = typeof st.color === 'function' ? st.color : () => st.color as number[];
  for (let y = 0; y < out.height; y++) {
    const py = y + y0 + 0.5;
    const faceRGB = colorAt(Math.min(1, Math.max(0, (py - 0.5 - top) / Math.max(1, bottom - top))));
    for (let x = 0; x < out.width; x++) {
      const pxx = x + x0 + 0.5;
      let a = 0;
      for (const q of quads)
        if (pxx >= q.l && pxx < q.r && py >= q.t && py < q.b) a = Math.max(a, sample(q.ax + (pxx - q.l) / sc, q.ay + (py - q.t) / sc));
      const face = Math.min(1, Math.max(0, (a - 0.5 - half) * spread + 0.5));
      const all = Math.min(1, Math.max(0, (a - 0.5 + half) * spread + 0.5));
      if (all <= 0) continue;
      const i = (y * out.width + x) * 4;
      if (oa >= 1) { // an opaque outline: face over outline, the shape's coverage as alpha (the sign preview's look)
        for (let c = 0; c < 3; c++) out.data[i + c] = Math.round(Math.min(1, st.outlineColor[c] * (1 - face) + faceRGB[c] * face) * 255);
        out.data[i + 3] = Math.round(all * 255);
        continue;
      }
      const alpha = face + (all - face) * oa; // a see-through outline drawn with its own alpha
      for (let c = 0; c < 3; c++) out.data[i + c] = Math.round(Math.min(1, (st.outlineColor[c] * (all - face) * oa + faceRGB[c] * face) / Math.max(alpha, 1e-6)) * 255);
      out.data[i + 3] = Math.round(alpha * 255);
    }
  }
  const tmp = document.createElement('canvas');
  tmp.width = out.width; tmp.height = out.height;
  tmp.getContext('2d')!.putImageData(out, 0, 0);
  g.drawImage(tmp, x0, y0);
}
