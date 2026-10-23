// Accessory art: an editor "net" (the model unfolded, each face upright as seen from outside) with layers on top, mapped into
// the game texture through the model's face → texture targets (internal/uvmap/models.go).

import { loadObjUV, renderMeshIcon } from './meshView';

import { templateFile } from './urls';

export interface Target { rect: number[]; src: number[]; flipX?: boolean; flipY?: boolean; transpose?: boolean; bleed?: boolean }
/** shown: the part of the face the item shows (x, y, w, h fractions of net; missing = all of it) — the pack's front and back
 *  reach on over texture no mesh shows; art is placed and fitted on the shown part (shownNet). */
export interface Face { id: string; label: string; net: number[]; hidden?: boolean; shown?: number[]; targets: Target[] }
/** A labelled frame of the net (furniture: Front, Left, Top… — each holds many faces). */
export interface View { label: string; net: number[] }
export interface Base { id: string; label: string; texture: string; targets?: Record<string, Target[]> }
export interface PreviewPart { mesh: string; url?: string; texture?: 'main' | 'secondary'; glass?: boolean }
/** projected: no faces — the net is the `views`, and the GPU painter (lib/projectPaint.ts) projects the layers onto every surface by its
 *  place in its view (preview[0] = the paint mesh). density: texture pixels per net unit. */
export interface Model { kind: string; mesh: string; textureSize: number; size: number[]; faces: Face[]; palette?: boolean; icon: string; bases?: Base[]; preview?: PreviewPart[];
  projected?: boolean; views?: View[]; turn?: number; density?: number }

/** URL of a preview mesh: its own, else the accessory templates. */
export const previewUrl = (p: PreviewPart) => p.url ?? templateFile('acc', p.mesh + '.obj');

/** The part of a face the item shows, in net units: where art is placed and fitted (its own net when it shows all of it). */
export function shownNet(f: Face): number[] {
  const [x, y, w, h] = f.net, s = f.shown;
  return s ? [x + s[0] * w, y + s[1] * h, s[2] * w, s[3] * h] : f.net;
}

/** The net rectangle around several faces' shown parts. */
export function shownArea(fs: Face[]): number[] {
  const rs = fs.map(shownNet);
  const x0 = Math.min(...rs.map((r) => r[0])), y0 = Math.min(...rs.map((r) => r[1]));
  return [x0, y0, Math.max(...rs.map((r) => r[0] + r[2])) - x0, Math.max(...rs.map((r) => r[1] + r[3])) - y0];
}

/** Faces the user picks from (fit, wrap, copy to, fill): the model's faces, or a projected model's views as faces. */
export function pickableFaces(m: Model): Face[] {
  return m.projected ? (m.views ?? []).map((v) => ({ id: 'view:' + v.label, label: v.label, net: v.net, targets: [] })) : m.faces;
}

/** The net's areas (layers are clipped to them): the faces, or a projected model's views. */
const netFaces = (m: Model): Face[] => (m.projected ? pickableFaces(m) : m.faces);

export type LayerKind = 'image' | 'fill' | 'text';
export interface Layer {
  id: string;
  kind: LayerKind;
  name: string;
  visible: boolean;
  opacity: number;              // 0..1
  blend: GlobalCompositeOperation;
  // Placement (centre, size) and rotation in degrees: in net units, or for a map layer in texture pixels at the model's
  // textureSize (a map fill is this rectangle).
  x: number; y: number; w: number; h: number; rot: number;
  space?: 'texture';            // on the texture map (UV view) instead of the faces
  mirror?: boolean;             // map layer: mirrored along its own x (with rot in 90° steps: any texture area's orientation)
  crop?: number[];              // map image: the part drawn, u0, v0, u1, v1 as fractions of the picture
  clip?: number[];              // map layer: drawn only inside this texture rect (x0, y0, x1, y1 px) — a piece of a converted face layer
  skew?: number;                // map layer: x shifted by skew × y (after rot and mirror) — a turned face layer on a stretched area
  src?: string;                 // image: library-relative path
  face?: string;                // fill on the faces: face id, '' = every face
  color?: string; color2?: string; // fill: solid or top→bottom gradient; text colour
  text?: string; outline?: string; // text: outline colour ('' = none)
  straighten?: { src: string; pts: number[] }; // image made by Straighten: the original photo and its corners (re-editable)
}
export interface Layout {
  version: 2; layers: Layer[]; base: 'vanilla' | 'color'; baseColor: string;
  net?: number;                 // the net version it was made for (missing = 1): Studio moves older layouts' layers when a net changed (uvmap.MigrateLayout)
  baseItem?: string;            // pack/box editor: the model base (vanilla item) the art starts from, or 'file'
  baseFile?: string;            // baseItem 'file': project image in the model's own layout (a snapshot of earlier art)
  textureFile?: string;         // painted texture (made from an exported template): used at its own resolution, layers go on top
}

/** The current net version (uvmap.NetVersion). */
export const NET_VERSION = 2;

export const newLayout = (): Layout => ({ version: 2, net: NET_VERSION, layers: [], base: 'vanilla', baseColor: '#3a3a3a' });

/** The shop icon of an accessory whose whole texture is given (an imported or painted texture, no layers): the faces are
 *  read straight from that texture, as the editor does with a painted texture. Same output as the editor's export. */
export async function renderAccessoryIcon(m: Model, textureUrl: string, w = 512, h = 512): Promise<string> {
  const tex = await loadImage(textureUrl);
  if (m.icon === 'mesh' || m.icon === 'pack') return meshIcon(m, tex); // the model itself, textured
  const S = netScale(m), net = document.createElement('canvas');
  await renderNet(net, m, newLayout(), tex, () => '', S);
  return composeIcon(m, net, S, w, h).toDataURL('image/png');
}

/** Net bounds in object units. */
export function netBounds(m: Model) {
  let x0 = Infinity, y0 = Infinity, x1 = -Infinity, y1 = -Infinity;
  for (const f of netFaces(m)) {
    x0 = Math.min(x0, f.net[0]); y0 = Math.min(y0, f.net[1]);
    x1 = Math.max(x1, f.net[0] + f.net[2]); y1 = Math.max(y1, f.net[1] + f.net[3]);
  }
  return { x0, y0, w: x1 - x0, h: y1 - y0 };
}

/** Pixels per object unit for the net canvas: about the texture's own density of the largest face. */
export function netScale(m: Model): number {
  if (m.density) return Math.min(480, Math.max(160, Math.round(m.density * 1.15)));
  let best = 0;
  for (const f of m.faces) for (const t of f.targets) {
    if (t.bleed) continue;
    const pxW = (t.rect[2] - t.rect[0]) / ((t.src[2] - t.src[0]) * f.net[2]);
    best = Math.max(best, pxW);
  }
  return Math.min(480, Math.max(160, Math.round(best * 1.15)));
}

const images = new Map<string, Promise<HTMLImageElement>>();
export function loadImage(url: string): Promise<HTMLImageElement> {
  if (!images.has(url)) {
    images.set(url, new Promise((res, rej) => {
      const img = new Image();
      img.onload = () => res(img);
      img.onerror = () => { images.delete(url); rej(new Error('could not load ' + url)); };
      img.src = url;
    }));
  }
  return images.get(url)!;
}

/** Face rectangle in net-canvas pixels. */
export function faceRect(m: Model, f: Face, S: number) {
  const b = netBounds(m);
  return { x: (f.net[0] - b.x0) * S, y: (f.net[1] - b.y0) * S, w: f.net[2] * S, h: f.net[3] * S };
}

/**
 * Draws src sub-rect (sx,sy,sw,sh) of an image/canvas into dest rect, optionally transposed (src x → dest y, src y → dest x)
 * and then mirrored in dest space.
 */
function blit(ctx: CanvasRenderingContext2D, img: CanvasImageSource, sx: number, sy: number, sw: number, sh: number,
  dx: number, dy: number, dw: number, dh: number, flipX = false, flipY = false, transpose = false) {
  if (sw <= 0 || sh <= 0 || dw <= 0 || dh <= 0) return;
  ctx.save();
  ctx.translate(dx + (flipX ? dw : 0), dy + (flipY ? dh : 0));
  ctx.scale(flipX ? -1 : 1, flipY ? -1 : 1);
  if (transpose) {
    ctx.transform(0, 1, 1, 0, 0, 0);
    ctx.drawImage(img, sx, sy, sw, sh, 0, 0, dh, dw);
  } else ctx.drawImage(img, sx, sy, sw, sh, 0, 0, dw, dh);
  ctx.restore();
}

/**
 * Renders the net: the base (vanilla texture projected back onto each face, or a plain colour) and the layers on top.
 * `imageUrl` turns a layer's library path into a URL. `layersOnly`: faces stay transparent under the layers (layers over a
 * painted texture, which composeTexture then keeps at full resolution).
 */
export async function renderNet(canvas: HTMLCanvasElement, m: Model, layout: Layout, vanilla: HTMLImageElement | HTMLCanvasElement | null,
  imageUrl: (rel: string) => string, S: number, layersOnly = false, baseNet?: HTMLCanvasElement) {
  // Where each face is in the base texture (a model base may read other texture areas, e.g. the Rare box in the box atlas).
  const reverse = m.bases?.find((b) => b.id === layout.baseItem)?.targets;
  const b = netBounds(m);
  canvas.width = Math.round(b.w * S);
  canvas.height = Math.round(b.h * S);
  const ctx = canvas.getContext('2d')!;
  ctx.clearRect(0, 0, canvas.width, canvas.height);
  const k = vanilla ? vanilla.width / m.textureSize : 1;

  // A base rendered earlier (renderNet without layers) instead of drawing every face again.
  if (baseNet && !layersOnly) ctx.drawImage(baseNet, 0, 0);
  for (const f of m.faces) {
    if (layersOnly || baseNet) break;
    const r = faceRect(m, f, S);
    if (layout.base === 'color' || !vanilla) {
      ctx.fillStyle = layout.baseColor || '#3a3a3a';
      ctx.fillRect(r.x, r.y, r.w, r.h);
      continue;
    }
    // Reverse mapping: each texture rect → the part of the face it shows (a face split into halves has two). Largest first,
    // so a smaller, more visible target painted over it wins (the deck box's front cover over the tray front underneath).
    const t = (reverse?.[f.id] ?? f.targets).filter((t) => !t.bleed).sort((a, c) => (c.src[2] - c.src[0]) * (c.src[3] - c.src[1]) - (a.src[2] - a.src[0]) * (a.src[3] - a.src[1]));
    for (const tg of t) {
      // Inverse of flip∘transpose is transpose∘flip = (flips swapped)∘transpose.
      const [fx, fy] = tg.transpose ? [tg.flipY, tg.flipX] : [tg.flipX, tg.flipY];
      blit(ctx, vanilla, tg.rect[0] * k, tg.rect[1] * k, (tg.rect[2] - tg.rect[0]) * k, (tg.rect[3] - tg.rect[1]) * k,
        r.x + tg.src[0] * r.w, r.y + tg.src[1] * r.h, (tg.src[2] - tg.src[0]) * r.w, (tg.src[3] - tg.src[1]) * r.h, fx, fy, tg.transpose);
    }
  }

  // Layers are clipped to the faces (the net's empty corners are not part of the model).
  ctx.save();
  ctx.beginPath();
  for (const f of netFaces(m)) { const r = faceRect(m, f, S); ctx.rect(r.x, r.y, r.w, r.h); }
  ctx.clip();
  const imgs = await layerImages(layout, imageUrl);
  for (const l of layout.layers) if (l.visible && !isMapLayer(l)) drawLayer(ctx, m, l, S, imgs);
  ctx.restore();
}

/** A layer on the texture map (UV view) rather than on the faces. */
export const isMapLayer = (l: Layer) => l.space === 'texture';
/** A colour over whole faces (its face, or every face; no size): the older kind, and dice swatches. A colour with a size is a
 *  rectangle placed, moved and sized like an image. */
export const wholeFaceFill = (l: Layer) => l.kind === 'fill' && !isMapLayer(l) && !(l.w > 0 && l.h > 0);

/** The layers' images, loaded (missing ones left out). */
async function layerImages(layout: Layout, imageUrl: (rel: string) => string): Promise<Map<string, HTMLImageElement>> {
  const out = new Map<string, HTMLImageElement>();
  await Promise.all(layout.layers.filter((l) => l.visible && l.kind === 'image' && l.src).map(async (l) => {
    try { out.set(l.src!, await loadImage(imageUrl(l.src!))); } catch { /* missing image: skipped */ }
  }));
  return out;
}

/** Draws one layer (with its opacity and blend, or `blend` instead) in net-canvas pixels (net origin at its top-left, S px per unit). */
function drawLayer(ctx: CanvasRenderingContext2D, m: Model, l: Layer, S: number, imgs: Map<string, HTMLImageElement>, blend?: GlobalCompositeOperation) {
  const b = netBounds(m);
  ctx.save();
  ctx.globalAlpha = l.opacity;
  ctx.globalCompositeOperation = blend ?? (l.blend || 'source-over');
  if (wholeFaceFill(l)) {
    for (const f of l.face?.startsWith('view:') ? pickableFaces(m) : netFaces(m)) {
      if (l.face && l.face !== f.id) continue;
      const r = faceRect(m, f, S);
      if (l.color2) {
        const g = ctx.createLinearGradient(0, r.y, 0, r.y + r.h);
        g.addColorStop(0, l.color || '#000');
        g.addColorStop(1, l.color2);
        ctx.fillStyle = g;
      } else ctx.fillStyle = l.color || '#000';
      ctx.fillRect(r.x, r.y, r.w, r.h);
    }
  } else {
    ctx.translate((l.x - b.x0) * S, (l.y - b.y0) * S);
    ctx.rotate((l.rot * Math.PI) / 180);
    drawShape(ctx, l, S, imgs);
  }
  ctx.restore();
}

/** Draws a layer's image, text or colour rectangle centred on the origin, s canvas pixels per layer unit. */
function drawShape(ctx: CanvasRenderingContext2D, l: Layer, s: number, imgs: Map<string, HTMLImageElement>) {
  const img = l.kind === 'image' && l.src ? imgs.get(l.src) : undefined;
  if (l.kind === 'fill') {
    const w = l.w * s, h = l.h * s;
    if (l.color2) {
      const g = ctx.createLinearGradient(0, -h / 2, 0, h / 2);
      g.addColorStop(0, l.color || '#000');
      g.addColorStop(1, l.color2);
      ctx.fillStyle = g;
    } else ctx.fillStyle = l.color || '#000';
    ctx.fillRect(-w / 2, -h / 2, w, h);
  } else if (img) {
    const [u0, v0, u1, v1] = l.crop ?? [0, 0, 1, 1];
    ctx.drawImage(img, u0 * img.width, v0 * img.height, (u1 - u0) * img.width, (v1 - v0) * img.height, (-l.w / 2) * s, (-l.h / 2) * s, l.w * s, l.h * s);
  } else if (l.kind === 'text' && l.text) {
    const px = l.h * s;
    ctx.font = `bold ${px}px Nunito, "Segoe UI", sans-serif`;
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    if (l.outline) {
      ctx.lineWidth = Math.max(2, px * 0.12);
      ctx.strokeStyle = l.outline;
      ctx.lineJoin = 'round';
      ctx.strokeText(l.text, 0, 0);
    }
    ctx.fillStyle = l.color || '#fff';
    ctx.fillText(l.text, 0, 0);
  }
}

/** Draws a map layer onto the texture canvas (k = canvas pixels per textureSize pixel). A fill is its rectangle. */
function drawMapLayer(ctx: CanvasRenderingContext2D, l: Layer, k: number, imgs: Map<string, HTMLImageElement>) {
  ctx.save();
  ctx.globalAlpha = l.opacity;
  ctx.globalCompositeOperation = l.blend || 'source-over';
  if (l.clip) {
    const [x0, y0, x1, y1] = l.clip;
    ctx.beginPath();
    ctx.rect(x0 * k, y0 * k, (x1 - x0) * k, (y1 - y0) * k);
    ctx.clip();
  }
  ctx.translate(l.x * k, l.y * k);
  ctx.rotate((l.rot * Math.PI) / 180);
  if (l.mirror) ctx.scale(-1, 1);
  if (l.skew) ctx.transform(1, 0, l.skew, 1, 0, 0);
  drawShape(ctx, l, k, imgs);
  ctx.restore();
}

/**
 * Layer textures for the GPU painter (lib/projectPaint.ts): the visible layers drawn in net space (S px per unit, covering the whole
 * net), clipped to the views. Consecutive normal layers share one texture; every other blend mode gets its own (the painter blends it
 * over each surface's own colours). Past the painter's limit the rest are flattened into the last one.
 */
export async function layerTextures(m: Model, layout: Layout, imageUrl: (rel: string) => string, S: number, max: number):
  Promise<{ textures: { canvas: HTMLCanvasElement; blend: GlobalCompositeOperation }[]; net: number[] }> {
  const b = netBounds(m);
  const imgs = await layerImages(layout, imageUrl);
  const groups: { layers: Layer[]; blend: GlobalCompositeOperation }[] = [];
  for (const l of layout.layers) {
    if (!l.visible || isMapLayer(l)) continue;
    const blend = l.blend || 'source-over', last = groups.at(-1);
    if (last && ((blend === 'source-over' && last.blend === 'source-over') || groups.length >= max)) last.layers.push(l);
    else groups.push({ layers: [l], blend });
  }
  const textures = groups.map((g) => {
    const c = document.createElement('canvas');
    c.width = Math.max(1, Math.round(b.w * S)); c.height = Math.max(1, Math.round(b.h * S));
    const ctx = c.getContext('2d')!;
    ctx.beginPath();
    for (const f of netFaces(m)) { const r = faceRect(m, f, S); ctx.rect(r.x, r.y, r.w, r.h); }
    ctx.clip();
    // The group's blend is the painter's; inside the group, layers keep theirs (only when flattened past the limit).
    for (const l of g.layers) drawLayer(ctx, m, l, S, imgs, g.layers.length === 1 ? 'source-over' : undefined);
    return { canvas: c, blend: g.blend };
  });
  return { textures, net: [b.x0, b.y0, b.w, b.h] };
}

/** Maps the net into the texture (vanilla underneath so unmapped areas — insides, edges, the mat's rubber — stay intact). */
export function composeTexture(m: Model, net: HTMLCanvasElement, vanilla: HTMLImageElement | HTMLCanvasElement | null, S: number): HTMLCanvasElement {
  const size = vanilla?.width || m.textureSize;
  const k = size / m.textureSize;
  const c = document.createElement('canvas');
  c.width = size; c.height = vanilla?.height || size;
  const ctx = c.getContext('2d')!;
  if (vanilla) ctx.drawImage(vanilla, 0, 0);
  else { ctx.fillStyle = '#3a3a3a'; ctx.fillRect(0, 0, c.width, c.height); }
  ctx.imageSmoothingQuality = 'high';
  // Each face part goes in through a copy of just its fully covered net pixels: smoothing then never mixes in what lies
  // around the face in the net (empty or the next face), so the area's edge texels are wholly the face's (no faint vanilla
  // seam along it).
  const crop = document.createElement('canvas'), cctx = crop.getContext('2d')!;
  for (const f of m.faces) {
    const r = faceRect(m, f, S);
    const targets = [...f.targets].sort((a, b) => Number(!!b.bleed) - Number(!!a.bleed)); // bleed first
    for (const t of targets) {
      const sx = r.x + t.src[0] * r.w, sy = r.y + t.src[1] * r.h, sw = (t.src[2] - t.src[0]) * r.w, sh = (t.src[3] - t.src[1]) * r.h;
      const x0 = Math.ceil(sx - 1e-6), y0 = Math.ceil(sy - 1e-6), x1 = Math.floor(sx + sw + 1e-6), y1 = Math.floor(sy + sh + 1e-6);
      const dest = [t.rect[0] * k, t.rect[1] * k, (t.rect[2] - t.rect[0]) * k, (t.rect[3] - t.rect[1]) * k] as const;
      if (x1 - x0 < 2 || y1 - y0 < 2) { blit(ctx, net, sx, sy, sw, sh, ...dest, t.flipX, t.flipY, t.transpose); continue; }
      crop.width = x1 - x0; crop.height = y1 - y0;
      cctx.drawImage(net, x0, y0, crop.width, crop.height, 0, 0, crop.width, crop.height);
      blit(ctx, crop, 0, 0, crop.width, crop.height, ...dest, t.flipX, t.flipY, t.transpose);
    }
  }
  return c;
}

/** Most texture px a whole-texture image layer brings (an imported texture, e.g. an EPL mod's 2048² one): its picture's own
 *  width over the texture's width, so the texture keeps that detail. Layers narrower than 90 % of the texture don't count
 *  (a big photo shrunk onto one face must not blow the texture up). 0 = none. */
function fullLayerWidth(m: Model, layout: Layout, imgs: Map<string, HTMLImageElement>): number {
  let w = 0;
  for (const l of layout.layers) {
    const img = l.visible && l.kind === 'image' && isMapLayer(l) && l.src ? imgs.get(l.src) : undefined;
    if (!img || Math.abs(l.w) < m.textureSize * 0.9) continue;
    const crop = l.crop ? l.crop[2] - l.crop[0] : 1;
    w = Math.max(w, (img.naturalWidth * crop * m.textureSize) / Math.abs(l.w));
  }
  return Math.min(4096, Math.round(w));
}

/** The canvas scaled up to `width` (keeping its shape) when that is wider; else the canvas itself. */
function upsized(c: HTMLCanvasElement, width: number): HTMLCanvasElement {
  if (width <= c.width) return c;
  const out = document.createElement('canvas');
  out.width = width; out.height = Math.round((c.height * width) / c.width);
  const ctx = out.getContext('2d')!;
  ctx.imageSmoothingQuality = 'high';
  ctx.drawImage(c, 0, 0, out.width, out.height);
  return out;
}

/** A canvas copy of an image (or a plain one of `size` when there is none). */
function canvasOf(img: HTMLImageElement | HTMLCanvasElement | null, size: number, color = '#3a3a3a'): HTMLCanvasElement {
  const c = document.createElement('canvas');
  c.width = img?.width || size; c.height = img?.height || size;
  const ctx = c.getContext('2d')!;
  if (img) ctx.drawImage(img, 0, 0);
  else { ctx.fillStyle = color; ctx.fillRect(0, 0, c.width, c.height); }
  return c;
}

/**
 * Renders a layout: draws the net view into `net` (faces upright, as the editor shows them) and returns a function that makes
 * the game texture. `vanilla` is the base item's texture, `painted` the layout's painted texture (textureFile), when used.
 * Without map layers the texture is only composed when asked for (the editor's Faces view skips it). With them, the layers
 * are applied in list order — each run of face layers mapped in over what's below it, map layers drawn straight onto the
 * texture — and the net shows the finished texture.
 */
export async function renderTexture(m: Model, layout: Layout, vanilla: HTMLImageElement | HTMLCanvasElement | null,
  painted: HTMLImageElement | HTMLCanvasElement | null, imageUrl: (rel: string) => string, S: number, net: HTMLCanvasElement): Promise<() => HTMLCanvasElement> {
  if (!layout.layers.some((l) => l.visible && isMapLayer(l))) {
    if (!painted) {
      await renderNet(net, m, layout, vanilla, imageUrl, S);
      return () => composeTexture(m, net, vanilla, S);
    }
    // The painted texture projected onto the faces (net view, icons) — always through the faces' own targets, it is in our
    // layout, not the base's — while the texture keeps it at full size with only the layers mapped on top.
    await renderNet(net, m, { ...layout, base: 'vanilla', baseItem: undefined }, painted, imageUrl, S);
    const layers = document.createElement('canvas');
    await renderNet(layers, m, layout, null, imageUrl, S, true);
    return () => composeTexture(m, layers, painted, S);
  }
  const imgs = await layerImages(layout, imageUrl);
  const scratch = document.createElement('canvas');
  // The base: the painted texture, the base item's art moved into our faces (pack bases) or a plain colour, else vanilla.
  let tex: HTMLCanvasElement;
  if (painted) tex = canvasOf(painted, m.textureSize);
  else if (layout.base === 'color' || layout.baseItem) {
    await renderNet(scratch, m, { ...layout, layers: [] }, vanilla, imageUrl, S);
    tex = composeTexture(m, scratch, vanilla, S);
  } else tex = canvasOf(vanilla, m.textureSize);
  tex = upsized(tex, fullLayerWidth(m, layout, imgs));
  const layers = layout.layers.filter((l) => l.visible);
  for (let i = 0; i < layers.length;) {
    const map = isMapLayer(layers[i]);
    let j = i;
    while (j < layers.length && isMapLayer(layers[j]) === map) j++;
    const run = layers.slice(i, j);
    i = j;
    if (map) {
      const ctx = tex.getContext('2d')!;
      for (const l of run) drawMapLayer(ctx, l, tex.width / m.textureSize, imgs);
      continue;
    }
    // Normal layers go over the texture as they are (no resampling); other blends need what's below them in the net.
    const plain = run.every((l) => (l.blend || 'source-over') === 'source-over');
    const runLayout = { ...layout, layers: run, base: 'vanilla' as const, baseItem: undefined };
    await renderNet(scratch, m, runLayout, plain ? null : tex, imageUrl, S, plain);
    tex = composeTexture(m, scratch, tex, S);
  }
  await renderNet(net, m, { ...layout, layers: [], base: 'vanilla', baseItem: undefined }, tex, imageUrl, S);
  return () => tex;
}

// ---------------------------------------------------------------- placing map layers in texture areas

// The mesh's UV islands (bounding boxes in textureSize px), loaded once per model (loadIslands): a face's area takes in the
// islands that belong to it, so art reaches the rounded rims next to it (the pack's strips between front and crimps).
const islandCache = new Map<string, number[][]>();
const islandLoads = new Map<string, Promise<number[][]>>();
const meshParts = (m: Model) => (m.preview ?? []).filter((p) => (p.texture ?? 'main') === 'main' && !p.glass).map(previewUrl);
const meshKey = (m: Model) => meshParts(m).join('|') + '|' + m.textureSize;

/** Loads the UV islands of a model's textured meshes (needed before placing art in its areas; missing meshes = no islands). */
export function loadIslands(m: Model): Promise<number[][]> {
  const key = meshKey(m);
  if (!islandLoads.has(key)) islandLoads.set(key, (async () => {
    const out: number[][] = [];
    for (const url of meshParts(m)) {
      const mesh = await loadObjUV(url).catch(() => null);
      if (!mesh) continue;
      const { uv, pos } = mesh;
      // Triangles sharing a UV edge and facing the same way (one of six directions) are one island, as Go's uvmap Panels:
      // a pack's rims between front and crimps are their own islands, not part of one big wrap.
      const n = uv.length / 6, parent = Array.from({ length: n }, (_, i) => i);
      const find = (x: number): number => { while (parent[x] !== x) x = parent[x] = parent[parent[x]]; return x; };
      const facing: number[] = [];
      for (let t = 0; t < n; t++) {
        const p = (k: number, c: number) => pos[(t * 3 + k) * 3 + c];
        const u = [p(1, 0) - p(0, 0), p(1, 1) - p(0, 1), p(1, 2) - p(0, 2)], w = [p(2, 0) - p(0, 0), p(2, 1) - p(0, 1), p(2, 2) - p(0, 2)];
        const nn = [u[1] * w[2] - u[2] * w[1], u[2] * w[0] - u[0] * w[2], u[0] * w[1] - u[1] * w[0]];
        let ax = 0;
        for (let q = 1; q < 3; q++) if (Math.abs(nn[q]) > Math.abs(nn[ax])) ax = q;
        facing.push(ax * 2 + (nn[ax] < 0 ? 1 : 0));
      }
      const vk = (i: number) => `${Math.round(uv[i * 2] * 1e5)},${Math.round(uv[i * 2 + 1] * 1e5)}`;
      const edges = new Map<string, number[]>();
      for (let t = 0; t < n; t++) for (let k = 0; k < 3; k++) {
        const a = vk(t * 3 + k), b = vk(t * 3 + ((k + 1) % 3)), key = a < b ? a + '|' + b : b + '|' + a;
        const l = edges.get(key);
        if (l) l.push(t); else edges.set(key, [t]);
      }
      for (const l of edges.values()) for (let j = 1; j < l.length; j++) if (facing[l[0]] === facing[l[j]]) parent[find(l[0])] = find(l[j]);
      const boxes = new Map<number, number[]>();
      for (let t = 0; t < n; t++) {
        const b = boxes.get(find(t)) ?? [Infinity, Infinity, -Infinity, -Infinity];
        for (let k = 0; k < 3; k++) {
          const i = t * 3 + k, x = uv[i * 2] * m.textureSize, y = (1 - uv[i * 2 + 1]) * m.textureSize;
          b[0] = Math.min(b[0], x); b[1] = Math.min(b[1], y); b[2] = Math.max(b[2], x); b[3] = Math.max(b[3], y);
        }
        boxes.set(find(t), b);
      }
      out.push(...boxes.values());
    }
    islandCache.set(key, out);
    return out;
  })());
  return islandLoads.get(key)!;
}

/** A face's texture areas: its targets, without the margins drawn around another of its targets (a pack's or box's bleed),
 *  the main one grown over its islands (faceArea). */
function faceAreas(m: Model, f: Face): Target[] {
  const main = mainTarget(f), grown = faceArea(m, f.id);
  return ownTargets(f).map((t) => (t === main && grown ? grown : t));
}

/** A face's targets without the margins drawn around another of its targets. */
function ownTargets(f: Face): Target[] {
  const inside = (a: number[], b: number[]) => a !== b && a[0] <= b[0] && a[1] <= b[1] && a[2] >= b[2] && a[3] >= b[3];
  return f.targets.filter((t) => !f.targets.some((o) => o !== t && inside(t.rect, o.rect)));
}

/** A face's largest target that isn't a bleed margin (a pack's shown front, a box front). */
function mainTarget(f: Face): Target | undefined {
  const size = (t: Target) => (t.rect[2] - t.rect[0]) * (t.rect[3] - t.rect[1]);
  return f.targets.filter((t) => !t.bleed).sort((a, b) => size(b) - size(a))[0] ?? f.targets[0];
}

/** Areas grown from a target (areaAspect needs the target they were measured from). */
const grownFrom = new WeakMap<Target, Target>();

/**
 * The texture area a face's art goes in: its main target, grown to the mesh islands that belong to it — the islands mostly
 * inside it, and islands no face has that run along one of its edges (same span, touching), like a pack's thin rims between
 * the front and the crimps. Without loaded islands (loadIslands) it is the target itself.
 */
export function faceArea(m: Model, faceId: string): Target | undefined {
  const f = m.faces.find((x) => x.id === faceId);
  const t = f && mainTarget(f);
  const r = t && grownRects(m)?.get(faceId);
  if (!t || !r) return t;
  const out = { ...t, rect: r.map(Math.round) };
  grownFrom.set(out, t);
  return out;
}

/** Every face's main target grown over its islands (see faceArea); null without loaded islands. */
function grownRects(m: Model): Map<string, number[]> | null {
  const islands = islandCache.get(meshKey(m));
  if (!islands?.length) return null;
  const mains = new Map<string, number[]>(), rects = new Map<string, number[]>();
  for (const f of m.faces) { const t = mainTarget(f); if (t) { mains.set(f.id, t.rect); rects.set(f.id, [...t.rect]); } }
  const all = m.faces.flatMap((x) => ownTargets(x).map((g) => g.rect));
  const share = (a: number[], b: number[]) => {
    const w = Math.min(a[2], b[2]) - Math.max(a[0], b[0]), h = Math.min(a[3], b[3]) - Math.max(a[1], b[1]);
    return w > 0 && h > 0 ? (w * h) / Math.max(1e-6, (a[2] - a[0]) * (a[3] - a[1])) : 0;
  };
  const add = (r: number[], i: number[]) => { r[0] = Math.min(r[0], i[0]); r[1] = Math.min(r[1], i[1]); r[2] = Math.max(r[2], i[2]); r[3] = Math.max(r[3], i[3]); };
  const placed = new Set<number[]>();
  // Islands mostly inside a face's main target are its own.
  for (const i of islands) for (const [id, r] of rects) if (share(i, mains.get(id)!) >= 0.5) { add(r, i); placed.add(i); }
  // Islands no target covers belong to the face whose edge they run along (span inside it, touching) — the closest span.
  // A few passes: a rim next to a rim (the seam's two thin strips) joins once the first one did.
  const tol = 3;
  for (let pass = 0; pass < 3; pass++) {
    for (const i of islands) {
      if (placed.has(i) || all.some((g) => share(i, g) >= 0.5)) continue;
      let best = '', diff = Infinity;
      for (const [id, q] of rects) {
        const alongX = i[0] >= q[0] - tol && i[2] <= q[2] + tol && (Math.abs(i[3] - q[1]) <= tol || Math.abs(i[1] - q[3]) <= tol);
        const alongY = i[1] >= q[1] - tol && i[3] <= q[3] + tol && (Math.abs(i[2] - q[0]) <= tol || Math.abs(i[0] - q[2]) <= tol);
        const d = alongX ? (q[2] - q[0]) - (i[2] - i[0]) : alongY ? (q[3] - q[1]) - (i[3] - i[1]) : Infinity;
        if (d < diff) { diff = d; best = id; }
      }
      if (best) { add(rects.get(best)!, i); placed.add(i); }
    }
  }
  // Island edges are UV positions: the GPU blends the texels on both sides of one, so a grown edge takes in one more whole
  // texel, unless another face's area is there — as Go's uvmap.inclusive does for the targets.
  for (const [id, r] of rects) {
    const main = mains.get(id)!, others = m.faces.filter((f) => f.id !== id).flatMap((f) => ownTargets(f).map((g) => g.rect));
    const free = (x0: number, y0: number, x1: number, y1: number) => !others.some((g) => g[0] < x1 && g[2] > x0 && g[1] < y1 && g[3] > y0);
    const [x0, y0, x1, y1] = [Math.floor(r[0]), Math.floor(r[1]), Math.ceil(r[2]), Math.ceil(r[3])];
    r[0] = r[0] < main[0] && x0 > 0 && free(x0 - 1, y0, x0, y1) ? x0 - 1 : x0;
    r[1] = r[1] < main[1] && y0 > 0 && free(x0, y0 - 1, x1, y0) ? y0 - 1 : y0;
    r[2] = r[2] > main[2] && x1 < m.textureSize && free(x1, y0, x1 + 1, y1) ? x1 + 1 : x1;
    r[3] = r[3] > main[3] && y1 < m.textureSize && free(x0, y1, x1, y1 + 1) ? y1 + 1 : y1;
  }
  return rects;
}

/** Texture layers placed on an area before areas changed (inclusive edges, the deck box's moved seams) end 1–2 px off its
 *  edges: short of them (an unpainted seam) or past them (over the next face). A layer spanning an area (one edge on the
 *  area's) whose opposite edge is that close is snapped to the area's edge, on every side, and so are clip edges. Only
 *  unturned or quarter-turned, unskewed layers. null when nothing changes. */
export function layersToInclusive(m: Model, layers: Layer[]): Layer[] | null {
  const areas = m.faces.flatMap((f) => [...ownTargets(f).filter((t) => !t.bleed).map((t) => t.rect), faceArea(m, f.id)?.rect ?? []])
    .filter((r) => r.length === 4);
  const off = (edge: number, to: number) => Math.abs(to - edge) > 0.01 && Math.abs(to - edge) <= 2.01;
  let changed = false;
  const out = layers.map((l) => {
    if (!isMapLayer(l) || l.skew) return l;
    const turn = (((Math.round(l.rot) % 360) + 360) % 360);
    if (turn % 90 || Math.abs(l.rot - Math.round(l.rot)) > 1e-6) return l;
    const swap = turn === 90 || turn === 270;
    const [w, h] = swap ? [l.h, l.w] : [l.w, l.h];
    let [x0, y0, x1, y1] = [l.x - w / 2, l.y - h / 2, l.x + w / 2, l.y + h / 2];
    let clip = l.clip ? [...l.clip] : undefined;
    for (const a of areas) {
      const spansY = y0 >= a[1] - 2 && y1 <= a[3] + 2, spansX = x0 >= a[0] - 2 && x1 <= a[2] + 2;
      if (spansY && Math.abs(x0 - a[0]) <= 2 && off(x1, a[2])) x1 = a[2];
      if (spansY && Math.abs(x1 - a[2]) <= 2 && off(a[0], x0)) x0 = a[0];
      if (spansX && Math.abs(y0 - a[1]) <= 2 && off(y1, a[3])) y1 = a[3];
      if (spansX && Math.abs(y1 - a[3]) <= 2 && off(a[1], y0)) y0 = a[1];
      if (clip && Math.abs(clip[0] - a[0]) <= 2 && clip[1] >= a[1] - 2 && clip[3] <= a[3] + 2 && off(clip[2], a[2])) clip[2] = a[2];
      if (clip && Math.abs(clip[1] - a[1]) <= 2 && clip[0] >= a[0] - 2 && clip[2] <= a[2] + 2 && off(clip[3], a[3])) clip[3] = a[3];
      if (clip && Math.abs(clip[2] - a[2]) <= 2 && clip[1] >= a[1] - 2 && clip[3] <= a[3] + 2 && off(a[0], clip[0])) clip[0] = a[0];
      if (clip && Math.abs(clip[3] - a[3]) <= 2 && clip[0] >= a[0] - 2 && clip[2] <= a[2] + 2 && off(a[1], clip[1])) clip[1] = a[1];
    }
    const nw = x1 - x0, nh = y1 - y0;
    if (Math.abs(nw - w) < 1e-6 && Math.abs(nh - h) < 1e-6 && (!clip || clip.every((v, i) => v === l.clip![i]))) return l;
    changed = true;
    const n: Layer = { ...l, x: (x0 + x1) / 2, y: (y0 + y1) / 2, w: swap ? nh : nw, h: swap ? nw : nh };
    if (clip) n.clip = clip;
    return n;
  });
  return changed ? out : null;
}

/** The mesh islands no face area reaches (small parts: a pack's fin edges): rects to colour so nothing keeps vanilla art. */
export function leftoverIslands(m: Model): number[][] {
  const islands = islandCache.get(meshKey(m)) ?? [];
  const areas = m.faces.flatMap((f) => faceAreas(m, f).map((t) => t.rect));
  const inside = (i: number[], a: number[]) => i[0] >= a[0] - 1 && i[1] >= a[1] - 1 && i[2] <= a[2] + 1 && i[3] <= a[3] + 1;
  return islands.filter((i) => !areas.some((a) => inside(i, a)));
}

/** Width ÷ height of a texture area on the model (its face's real shape — the texture may stretch it; a grown area by how
 *  much it grew). */
export function areaAspect(m: Model, t: Target): number {
  const from = grownFrom.get(t) ?? t;
  const f = m.faces.find((x) => x.targets.includes(from));
  const size = (q: Target) => { const w = q.rect[2] - q.rect[0], h = q.rect[3] - q.rect[1]; return q.transpose ? [h, w] : [w, h]; };
  if (!f) { const [w, h] = size(t); return w / h; }
  const [w0, h0] = size(from), [w1, h1] = size(t);
  return ((f.net[2] * (from.src[2] - from.src[0])) / (f.net[3] * (from.src[3] - from.src[1]))) * (w1 / w0) / (h1 / h0);
}

/**
 * Places a map layer on a part of a texture area: `part` = x, y, w, h as fractions of the area seen upright (top-left origin).
 * The layer takes the area's orientation (rot in 90° steps + mirror), so it reads upright on the model.
 */
export function placeRect(t: Target, l: Layer, part: number[] = [0, 0, 1, 1]) {
  const [x0, y0, x1, y1] = t.rect, W = x1 - x0, H = y1 - y0;
  const T = !!t.transpose;
  // The area's upright x and y as texture directions (labelTexture's mapping, inverted).
  const ex = T ? [0, t.flipY ? -1 : 1] : [t.flipX ? -1 : 1, 0];
  const ey = T ? [t.flipX ? -1 : 1, 0] : [0, t.flipY ? -1 : 1];
  const fw = T ? H : W, fh = T ? W : H; // the area's size seen upright
  // Centre of the part in texture pixels.
  const a = part[0] + part[2] / 2, b = part[1] + part[3] / 2;
  const ox = ex[0] < 0 || ey[0] < 0 ? x1 : x0, oy = ex[1] < 0 || ey[1] < 0 ? y1 : y0;
  const cx = ox + (ex[0] * a * fw + ey[0] * b * fh), cy = oy + (ex[1] * a * fw + ey[1] * b * fh);
  // Rotation that turns the layer's y into the area's y; then the layer's x either matches the area's x or is mirrored.
  const rot = ey[1] === 1 ? 0 : ey[0] === -1 ? 90 : ey[1] === -1 ? 180 : -90;
  const r = (rot * Math.PI) / 180, lx = [Math.round(Math.cos(r)), Math.round(Math.sin(r))];
  Object.assign(l, { space: 'texture', x: cx, y: cy, w: part[2] * fw, h: part[3] * fh, rot, mirror: lx[0] !== ex[0] || lx[1] !== ex[1] });
  delete l.clip;
  delete l.skew;
}

/**
 * Puts a picture (aspect = width ÷ height of the part of it used, `crop`) into a texture area: stretched over it, as large as
 * fits (contain) or filling it with the overflow cropped off (cover) — nothing is drawn outside the area.
 */
export function placeIn(m: Model, t: Target, l: Layer, mode: 'stretch' | 'contain' | 'cover', aspect = 1, crop: number[] = [0, 0, 1, 1]) {
  const area = areaAspect(m, t);
  if (mode === 'stretch') { placeRect(t, l); l.crop = crop; return; }
  if (mode === 'cover') {
    // The middle of the picture cut to the area's shape.
    const [u0, v0, u1, v1] = crop;
    if (aspect > area) { const f = area / aspect, m = (u1 - u0) * (1 - f) / 2; l.crop = [u0 + m, v0, u1 - m, v1]; }
    else { const f = aspect / area, m = (v1 - v0) * (1 - f) / 2; l.crop = [u0, v0 + m, u1, v1 - m]; }
    placeRect(t, l);
    return;
  }
  const w = aspect > area ? 1 : aspect / area, h = aspect > area ? area / aspect : 1;
  placeRect(t, l, [(1 - w) / 2, (1 - h) / 2, w, h]);
  l.crop = crop;
}

/** Colour rectangles over every texture area of these faces (all faces when empty), each upright like its face (gradients run down the face). */
export function areaFills(m: Model, faceIds: string[], base: Omit<Layer, 'id' | 'x' | 'y' | 'w' | 'h' | 'rot'>): Layer[] {
  const out: Layer[] = [];
  for (const f of m.faces) {
    if (faceIds.length && !faceIds.includes(f.id)) continue;
    for (const t of faceAreas(m, f)) {
      const l = { ...base, id: layerId(), kind: 'fill', x: 0, y: 0, w: 0, h: 0, rot: 0 } as Layer;
      placeRect(t, l);
      delete l.face;
      out.push(l);
    }
  }
  return out;
}

/**
 * A picture over several faces stacked top to bottom (a pack's crimp, front and crimp): a slice per face, cut by the faces'
 * heights, on every area of the face (the crimp's front and back, the seam's end) — each area shows its part of the face, the
 * picture spanning the part the item shows.
 */
export function placeStacked(m: Model, faceIds: string[], base: Omit<Layer, 'id' | 'x' | 'y' | 'w' | 'h' | 'rot'>): Layer[] {
  const faces = faceIds.map((id) => m.faces.find((f) => f.id === id)).filter((f): f is Face => !!f);
  const total = faces.reduce((s, f) => s + f.net[3], 0);
  let y = 0;
  const out: Layer[] = [];
  for (const f of faces) {
    const v0 = y / total, v1 = (y + f.net[3]) / total;
    y += f.net[3];
    const [sx, sy, sw, sh] = f.shown ?? [0, 0, 1, 1];
    for (const t of faceAreas(m, f)) {
      // The area's part of the face, as fractions of the shown part (the picture covers that); parts past it get none.
      const u0 = (t.src[0] - sx) / sw, u1 = (t.src[2] - sx) / sw, w0 = (t.src[1] - sy) / sh, w1 = (t.src[3] - sy) / sh;
      if (u0 >= 1 || u1 <= 0 || w0 >= 1 || w1 <= 0) continue;
      const l = { ...base, id: layerId(), x: 0, y: 0, w: 0, h: 0, rot: 0 } as Layer;
      placeIn(m, t, l, 'stretch', 1, [Math.max(0, u0), v0 + Math.max(0, w0) * (v1 - v0), Math.min(1, u1), v0 + Math.min(1, w1) * (v1 - v0)]);
      out.push(l);
    }
  }
  return out;
}

/** UV island outlines of a model's textured preview meshes: edges used by one triangle only, in textureSize pixels. */
const outlineCache = new Map<string, Promise<number[][]>>();
export function uvOutlines(m: Model): Promise<number[][]> {
  const urls = meshParts(m), key = meshKey(m);
  if (!outlineCache.has(key)) outlineCache.set(key, (async () => {
    const out: number[][] = [];
    for (const url of urls) {
      const { uv } = await loadObjUV(url);
      const q = (i: number) => `${Math.round(uv[i * 2] * 1e5)},${Math.round(uv[i * 2 + 1] * 1e5)}`;
      const edges = new Map<string, { n: number; a: number; b: number }>();
      for (let t = 0; t < uv.length / 6; t++) for (let k = 0; k < 3; k++) {
        const a = t * 3 + k, b = t * 3 + ((k + 1) % 3);
        const ka = q(a), kb = q(b), key = ka < kb ? ka + '|' + kb : kb + '|' + ka;
        const e = edges.get(key);
        if (e) e.n++; else edges.set(key, { n: 1, a, b });
      }
      const s = m.textureSize;
      for (const e of edges.values()) if (e.n === 1)
        out.push([uv[e.a * 2] * s, (1 - uv[e.a * 2 + 1]) * s, uv[e.b * 2] * s, (1 - uv[e.b * 2 + 1]) * s]);
    }
    return out;
  })().catch((e) => { outlineCache.delete(key); throw e; }));
  return outlineCache.get(key)!;
}

/** Name of one texture area of a face: a face split over several areas says which part (deck box back halves, front cover). */
export function targetLabel(f: Face, t: Target): string {
  return f.label + (t.src[0] > 0 || t.src[2] < 1 ? (t.src[0] === 0 ? ' (left half)' : ' (right half)') : '') + (t.src[1] > 0 ? ' — cover' : '');
}

/**
 * Painting template in the texture's own size (w×h): every area the game reads a face from, filled per face and labelled
 * upright as the face is seen on the model (with an "up" arrow — rotated/mirrored areas show their orientation). Everything
 * else is the dark background (not painted by the faces).
 */
export function renderTemplate(m: Model, w: number, h: number): HTMLCanvasElement {
  const k = w / m.textureSize;
  const c = document.createElement('canvas');
  c.width = w; c.height = h;
  const ctx = c.getContext('2d')!;
  ctx.fillStyle = '#262626';
  ctx.fillRect(0, 0, w, h);
  m.faces.forEach((f, i) => {
    const fill = f.hidden ? '#6a6a6a' : `hsl(${Math.round((i * 137.5) % 360)} 42% 58%)`;
    for (const t of f.targets) {
      if (t.bleed) continue;
      const dx = t.rect[0] * k, dy = t.rect[1] * k, dw = (t.rect[2] - t.rect[0]) * k, dh = (t.rect[3] - t.rect[1]) * k;
      // The area drawn upright on its own canvas, then mapped like the editor maps a face (same flips / transpose).
      const [fw, fh] = t.transpose ? [Math.round(dh), Math.round(dw)] : [Math.round(dw), Math.round(dh)];
      if (fw < 1 || fh < 1) continue;
      const fc = document.createElement('canvas');
      fc.width = fw; fc.height = fh;
      const fx = fc.getContext('2d')!;
      fx.fillStyle = fill;
      fx.fillRect(0, 0, fw, fh);
      const lines = [targetLabel(f, t) + (f.hidden ? ' (hidden)' : ''), `${Math.round(dw)}×${Math.round(dh)} px`];
      // Text fits the area: up to ~1/6 of its height, shrunk to 90% of its width.
      let px = Math.max(6, Math.min(fh / 6, 40 * k));
      fx.font = `bold ${px}px Nunito, "Segoe UI", sans-serif`;
      const widest = Math.max(...lines.map((s) => fx.measureText(s).width));
      if (widest > fw * 0.9) px = Math.max(5, px * (fw * 0.9) / widest);
      const small = fh < px * 3.2; // thin strip (crimps, spine): one line, no arrow
      fx.fillStyle = '#111';
      fx.textAlign = 'center';
      fx.textBaseline = 'middle';
      fx.font = `bold ${px}px Nunito, "Segoe UI", sans-serif`;
      if (small) fx.fillText(lines[0], fw / 2, fh / 2);
      else {
        fx.fillText(lines[0], fw / 2, fh / 2);
        fx.font = `${px * 0.75}px Nunito, "Segoe UI", sans-serif`;
        fx.fillText(lines[1], fw / 2, fh / 2 + px * 1.1);
        // Up arrow near the top edge.
        const a = Math.min(px * 1.2, fh * 0.12), ay = Math.max(a * 0.4, fh * 0.06);
        fx.beginPath();
        fx.moveTo(fw / 2, ay);
        fx.lineTo(fw / 2 + a * 0.7, ay + a);
        fx.lineTo(fw / 2 - a * 0.7, ay + a);
        fx.closePath();
        fx.fill();
        fx.font = `bold ${px * 0.6}px Nunito, "Segoe UI", sans-serif`;
        fx.fillText('UP', fw / 2, ay + a + px * 0.5);
      }
      blit(ctx, fc, 0, 0, fw, fh, dx, dy, dw, dh, t.flipX, t.flipY, t.transpose);
      // Readable note on areas whose art lies sideways / mirrored in the texture (their label above is drawn that way too).
      const note = t.transpose ? 'turned on its side — paint as shown' : t.flipX || t.flipY ? 'mirrored — paint as shown' : '';
      if (note && dh >= 24 * k) {
        let np = Math.max(6, Math.min(14 * k, dh / 8));
        ctx.font = `${np}px Nunito, "Segoe UI", sans-serif`;
        const nw = ctx.measureText(note).width;
        if (nw > dw * 0.92) { np = Math.max(5, np * dw * 0.92 / nw); ctx.font = `${np}px Nunito, "Segoe UI", sans-serif`; }
        ctx.fillStyle = 'rgba(0,0,0,0.55)';
        ctx.fillRect(dx, dy + dh - np * 1.6, dw, np * 1.6);
        ctx.fillStyle = '#fff';
        ctx.textAlign = 'center';
        ctx.textBaseline = 'middle';
        ctx.fillText(note, dx + dw / 2, dy + dh - np * 0.8);
      }
      ctx.strokeStyle = '#ffffff';
      ctx.lineWidth = Math.max(1, 2 * k);
      ctx.strokeRect(dx + ctx.lineWidth / 2, dy + ctx.lineWidth / 2, dw - ctx.lineWidth, dh - ctx.lineWidth);
    }
  });
  return c;
}

/** Shop icon in the vanilla icon's size: a pseudo-3D deck box (front, right side, lid) or a tilted playmat. */
export function composeIcon(m: Model, net: HTMLCanvasElement, S: number, w = 512, h = 512): HTMLCanvasElement {
  const c = document.createElement('canvas');
  c.width = w; c.height = h;
  const ctx = c.getContext('2d')!;
  ctx.imageSmoothingQuality = 'high';
  const face = (id: string) => m.faces.find((f) => f.id === id);
  if (m.icon === 'box') {
    const fr = face('front')!, sd = face('right')!, tp = face('top')!;
    const tab = face('tab'); // battle deck: hang tab standing up from the lid's back edge
    const [bw, bh, bd] = m.size;
    const th = tab ? tab.net[3] : 0;
    // Oblique projection: depth goes up-right. Fit the whole drawing into the icon with a margin.
    const dx = 0.55 * bd, dy = -0.32 * bd;
    const totalW = bw + dx, totalH = bh - dy + th;
    const s = Math.min((w * 0.84) / totalW, (h * 0.86) / totalH);
    const ox = (w - totalW * s) / 2, oy = (h - totalH * s) / 2 - dy * s + th * s;
    const R = (f: typeof fr) => faceRect(m, f, S);
    if (tab) {
      const rb = R(tab);
      ctx.drawImage(net, rb.x, rb.y, rb.w, rb.h, ox + dx * s, oy + dy * s - th * s, bw * s, th * s);
    }
    // Front
    const rf = R(fr);
    ctx.drawImage(net, rf.x, rf.y, rf.w, rf.h, ox, oy, bw * s, bh * s);
    // Right side: x axis along the depth vector, y axis down.
    const rs = R(sd);
    ctx.save();
    ctx.setTransform((dx * s) / rs.w, (dy * s) / rs.w, 0, (bh * s) / rs.h, ox + bw * s, oy);
    ctx.drawImage(net, rs.x, rs.y, rs.w, rs.h, 0, 0, rs.w, rs.h);
    ctx.fillStyle = 'rgba(0,0,0,0.28)';
    ctx.fillRect(0, 0, rs.w, rs.h);
    ctx.restore();
    // Lid: image top = back edge.
    const rt = R(tp);
    ctx.save();
    ctx.setTransform((bw * s) / rt.w, 0, (-dx * s) / rt.h, (-dy * s) / rt.h, ox + dx * s, oy + dy * s);
    ctx.drawImage(net, rt.x, rt.y, rt.w, rt.h, 0, 0, rt.w, rt.h);
    ctx.fillStyle = 'rgba(255,255,255,0.12)';
    ctx.fillRect(0, 0, rt.w, rt.h);
    ctx.restore();
  } else {
    const f = face('front') ?? face('surface') ?? m.faces[0];
    const r = faceRect(m, f, S);
    const aspect = r.w / r.h;
    // Tilted card-like view (like the vanilla icons): skew up to the right. Portrait faces are limited by height.
    let W = w * 0.9, H = W / aspect;
    if (H > h * 0.82) { H = h * 0.82; W = H * aspect; }
    const skewY = -0.12, skewX = 0.12;
    const ox = (w - W - H * skewX) / 2, oy = (h - H + W * skewY) / 2 - W * skewY;
    ctx.save();
    ctx.setTransform(W / r.w, (skewY * W) / r.w, (skewX * H) / r.h, H / r.h, ox, oy);
    ctx.drawImage(net, r.x, r.y, r.w, r.h, 0, 0, r.w, r.h);
    ctx.restore();
  }
  return c;
}

/** Average colour of a canvas/image region (0–255 RGB). */
function avgColor(src: CanvasImageSource, x: number, y: number, w: number, h: number): number[] {
  const c = document.createElement('canvas');
  c.width = 8; c.height = 8;
  const ctx = c.getContext('2d', { willReadFrequently: true })!;
  ctx.drawImage(src, x, y, Math.max(1, w), Math.max(1, h), 0, 0, 8, 8);
  const d = ctx.getImageData(0, 0, 8, 8).data;
  const s = [0, 0, 0];
  for (let i = 0; i < d.length; i += 4) { s[0] += d[i]; s[1] += d[i + 1]; s[2] += d[i + 2]; }
  return s.map((v) => v / 64);
}

/**
 * Palette models (dice box): the vanilla icon with each swatch's colour replaced — every icon pixel is matched to the closest
 * vanilla swatch colour and re-lit with the new colour (keeps the render's shading).
 */
export function recolorIcon(m: Model, net: HTMLCanvasElement, S: number, vanilla: HTMLImageElement, icon: HTMLImageElement): HTMLCanvasElement {
  const k = vanilla.width / m.textureSize;
  const swatches = m.faces.map((f) => {
    const t = f.targets[0].rect;
    const r = faceRect(m, f, S);
    return { old: avgColor(vanilla, t[0] * k + 2, t[1] * k + 2, (t[2] - t[0]) * k - 4, (t[3] - t[1]) * k - 4), now: avgColor(net, r.x + 2, r.y + 2, r.w - 4, r.h - 4) };
  });
  const c = document.createElement('canvas');
  c.width = icon.width; c.height = icon.height;
  const ctx = c.getContext('2d', { willReadFrequently: true })!;
  ctx.drawImage(icon, 0, 0);
  const img = ctx.getImageData(0, 0, c.width, c.height);
  const d = img.data;
  const lum = (r: number, g: number, b: number) => 0.299 * r + 0.587 * g + 0.114 * b + 1;
  for (let i = 0; i < d.length; i += 4) {
    if (d[i + 3] === 0) continue;
    const L = lum(d[i], d[i + 1], d[i + 2]);
    // Closest swatch by colour direction (chroma) and brightness.
    let best = 0, bestD = Infinity;
    swatches.forEach((s, j) => {
      const Lo = lum(s.old[0], s.old[1], s.old[2]);
      let dist = 0;
      for (let q = 0; q < 3; q++) dist += (d[i + q] / L - s.old[q] / Lo) ** 2 * 4000;
      dist += (Math.log(L / Lo)) ** 2 * 60;
      if (dist < bestD) { bestD = dist; best = j; }
    });
    const s = swatches[best];
    const ratio = L / lum(s.old[0], s.old[1], s.old[2]);
    for (let q = 0; q < 3; q++) d[i + q] = Math.max(0, Math.min(255, s.now[q] * ratio));
  }
  ctx.putImageData(img, 0, 0);
  return c;
}

let seq = 0;
/** Face-net models: whole-face colours as colour rectangles that look the same — one the size of each face they cover
 *  (an all-faces colour: one per face, named after it); its gradient still runs down the face. null when there are none. */
export function faceFillsToRects(m: Model, layers: Layer[]): Layer[] | null {
  if (m.projected || !layers.some(wholeFaceFill)) return null;
  return layers.flatMap((l) => {
    if (!wholeFaceFill(l)) return [l];
    const faces = m.faces.filter((f) => !l.face || f.id === l.face);
    return faces.map((f): Layer => ({
      ...l, id: faces.length === 1 ? l.id : layerId(), name: faces.length === 1 ? l.name : `${l.name} – ${f.label}`, face: undefined,
      x: f.net[0] + f.net[2] / 2, y: f.net[1] + f.net[3] / 2, w: f.net[2], h: f.net[3], rot: 0,
    }));
  });
}

export const layerId = () => `l${Date.now().toString(36)}${(seq++).toString(36)}`;

type Picture = CanvasImageSource & { width: number; height: number };

/** Packaging colours of a product photo: the most common colour of its left and right edge strips in an upper and a lower
 *  band (the art sits in the middle and only touches the edges here and there; the bands skip a pack's crimps). Near-white
 *  pixels (background the crop left) are ignored. */
export function edgeColors(img: Picture): { top: string; bottom: string } { return bandColors(img, true); }

/** The most common colour of an image's upper and lower part (all columns): the main colours of a piece of card art. */
export function dominantColors(img: Picture): { top: string; bottom: string } { return bandColors(img, false); }

function bandColors(img: Picture, edges: boolean): { top: string; bottom: string } {
  const w = 64, h = Math.max(16, Math.round((64 * img.height) / img.width));
  const c = document.createElement('canvas');
  c.width = w; c.height = h;
  const ctx = c.getContext('2d', { willReadFrequently: true })!;
  ctx.drawImage(img, 0, 0, w, h);
  const d = ctx.getImageData(0, 0, w, h).data;
  // Edges: skip the outermost, anti-aliased pixels.
  const cols = edges ? [2, 3, 4, 5, w - 6, w - 5, w - 4, w - 3] : Array.from({ length: w }, (_, i) => i);
  const band = (y0: number, y1: number) => {
    const buckets = new Map<number, number[]>(); // 3 bits per channel → summed r, g, b and count
    for (let y = Math.floor(y0 * h); y < Math.ceil(y1 * h); y++)
      for (const x of cols) {
        const i = (y * w + x) * 4, r = d[i], g = d[i + 1], b = d[i + 2];
        if (d[i + 3] < 128 || (r > 235 && g > 235 && b > 235)) continue;
        const k = ((r >> 5) << 6) | ((g >> 5) << 3) | (b >> 5);
        const q = buckets.get(k) ?? [0, 0, 0, 0];
        q[0] += r; q[1] += g; q[2] += b; q[3]++;
        buckets.set(k, q);
      }
    let best: number[] | undefined;
    for (const q of buckets.values()) if (!best || q[3] > best[3]) best = q;
    if (!best) return '#3a3a3a';
    return '#' + best.slice(0, 3).map((v) => Math.round(v / best![3]).toString(16).padStart(2, '0')).join('');
  };
  return edges ? { top: band(0.12, 0.35), bottom: band(0.65, 0.88) } : { top: band(0, 0.5), bottom: band(0.5, 1) };
}

/** Shop icon of a model whose icon is 'pack': the real game mesh rendered with the finished texture (1024², like vanilla). */
export function meshIcon(m: Model, texture: TexImageSource): Promise<string> {
  const parts = (m.preview ?? [{ mesh: m.mesh }]).filter((p) => !p.glass && p.texture !== 'secondary')
    .map((p) => ({ url: previewUrl(p), texture: p.texture }));
  return renderMeshIcon(parts, texture, [1024, 1024], { rx: m.turn ? 0.3 : 0, ry: -0.35 + (m.turn ?? 0), fov: 1.2, ambient: 0.9 });
}
