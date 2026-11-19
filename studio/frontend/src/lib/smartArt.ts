// Smart generate: builds a pack's and its box's editor layouts from TCGplayer product photos (App.SmartArtSources), or from
// the set's own card art when there are none. Everything ends up as ordinary face layers (named "Auto: …"): one layer per thing
// on the Faces tab, shown in every texture area of its face, that can be moved, hidden or recoloured; box faces cut from the
// display photo keep their corners (layer.straighten) for Adjust straighten….
import { App, projectFile } from './api';
import { templateFile } from './urls';
import {
  textureIcon, dominantColors, edgeColors, layerId, loadImage, netScale, newLayout, renderTexture, shownArea, shownNet,
  type Layer, type Layout, type Model,
} from './accessoryArt';
import { rectify, type Pt } from './perspective';
import { fontFamily } from './shopLook';

const AUTO = 'Auto: ';

type Picture = CanvasImageSource & { width: number; height: number };
type Part = number[]; // x, y, w, h as fractions of the part of a face the item shows

export interface SmartSources {
  packPhoto: string; packName: string; boxPhoto: string; boxName: string;
  box?: { front: number[]; lid: number[]; confident: boolean } | null;
  packQuad?: number[];          // dialog only: the pack (crimps included) in the pack photo, tl tr br bl; none = whole photo
  cards: { name: string; image: string; window: number[] }[];
  icon: string; notes: string[];
}


interface Job { projectId: string; packId: string; title: string; src: SmartSources; packModel: Model; boxModel: Model }

// ---------------------------------------------------------------- layers (one face layer each: shows in all the face's areas)

const base = (name: string, kind: Layer['kind'], opacity = 1): Layer =>
  ({ id: layerId(), kind, name: AUTO + name, visible: true, opacity, blend: 'source-over', x: 0, y: 0, w: 0, h: 0, rot: 0 });

/** A face of the model (it always has the faces Smart generate uses). */
const faceOf = (m: Model, face: string) => m.faces.find((f) => f.id === face)!;

/** Width ÷ height of the part of a face the item shows: pictures are cut to it. */
const aspectOf = (m: Model, face: string) => { const [, , w, h] = shownNet(faceOf(m, face)); return w / h; };

/** Places a layer on a part of a net rectangle (x, y, w, h). */
function placeAt(l: Layer, [x, y, w, h]: number[], part: Part = [0, 0, 1, 1]): Layer {
  return Object.assign(l, { x: x + (part[0] + part[2] / 2) * w, y: y + (part[1] + part[3] / 2) * h, w: part[2] * w, h: part[3] * h });
}

/** An image stretched over the shown part of a face (or a part of that). */
const image = (m: Model, name: string, src: string, face: string, opacity = 1, straighten?: Layer['straighten'], part?: Part): Layer =>
  placeAt({ ...base(name, 'image', opacity), src, straighten }, shownNet(faceOf(m, face)), part);

/** An image keeping the picture's shape, as large as fits in a part of a face (scale ≤ 1), centred. */
function contain(m: Model, name: string, src: string, pic: Picture, face: string, scale = 1, opacity = 1, part: Part = [0, 0, 1, 1]): Layer {
  const asp = pic.width / Math.max(1, pic.height), ar = aspectOf(m, face) * part[2] / part[3];
  let w = scale, h = (w * ar) / asp;
  if (h > scale) { h = scale; w = (h * asp) / ar; }
  return placeAt({ ...base(name, 'image', opacity), src }, shownNet(faceOf(m, face)),
    [part[0] + part[2] * (1 - w) / 2, part[1] + part[3] * (1 - h) / 2, part[2] * w, part[3] * h]);
}

/** Colour (top → bottom gradient) over a whole face, also the part the item doesn't show. */
const fill = (m: Model, name: string, face: string, color: string, color2 = ''): Layer =>
  placeAt({ ...base(name, 'fill'), color, color2 }, faceOf(m, face).net);

/** A text layer as large as fits in a part of a face (rough width estimate: 0.62 em per letter, bold); maxH as a fraction of
 *  the face's height. */
function text(m: Model, name: string, t: string, face: string, part: Part, maxH: number): Layer {
  const h = Math.min(maxH, (part[2] * aspectOf(m, face)) / Math.max(1, t.length * 0.62));
  return placeAt({ ...base(name, 'text'), text: t, color: '#ffffff', outline: '#000000' }, shownNet(faceOf(m, face)),
    [part[0], part[1] + part[3] / 2 - h / 2, part[2], h]);
}

/** Copies of the layers centred on one face for another face of the same size, in the same order. */
function onOpposite(m: Model, layers: Layer[], from: string, to: string): Layer[] {
  const [x, y, w, h] = faceOf(m, from).net, n = faceOf(m, to).net;
  return layers.filter((l) => l.x >= x && l.x < x + w && l.y >= y && l.y < y + h)
    .map((l) => ({ ...l, id: layerId(), name: `${l.name} (${to})`, x: l.x + n[0] - x, y: l.y + n[1] - y }));
}

// ---------------------------------------------------------------- pictures

function canvas(w: number, h: number) {
  const c = document.createElement('canvas');
  c.width = Math.max(2, Math.round(w)); c.height = Math.max(2, Math.round(h));
  return c;
}

/** The middle of pic cut to aspect (w/h), at most `width` px wide, optionally through a canvas filter. */
function cover(pic: Picture, aspect: number, width = 1024, filter = ''): HTMLCanvasElement {
  let sw = pic.width, sh = sw / aspect;
  if (sh > pic.height) { sh = pic.height; sw = sh * aspect; }
  const out = canvas(Math.min(width, sw), Math.min(width, sw) / aspect);
  const ctx = out.getContext('2d')!;
  ctx.filter = filter || 'none';
  ctx.drawImage(pic, (pic.width - sw) / 2, (pic.height - sh) / 2, sw, sh, 0, 0, out.width, out.height);
  return out;
}

/** The part of pic inside a window of fractions (x0, y0, x1, y1). */
function crop(pic: Picture, win: number[]): HTMLCanvasElement {
  const [x0, y0, x1, y1] = [win[0] * pic.width, win[1] * pic.height, win[2] * pic.width, win[3] * pic.height];
  const out = canvas(x1 - x0, y1 - y0);
  out.getContext('2d')!.drawImage(pic, x0, y0, x1 - x0, y1 - y0, 0, 0, out.width, out.height);
  return out;
}

/** Makes the white background reaching in from the borders transparent (the corners of a lid cut from a photo). */
function clearBackground(c: HTMLCanvasElement) {
  const ctx = c.getContext('2d', { willReadFrequently: true })!;
  const img = ctx.getImageData(0, 0, c.width, c.height), d = img.data, W = c.width, H = c.height;
  const bg = (i: number) => d[i * 4 + 3] < 20 || (d[i * 4] > 232 && d[i * 4 + 1] > 232 && d[i * 4 + 2] > 232);
  const seen = new Uint8Array(W * H), stack: number[] = [];
  for (let x = 0; x < W; x++) stack.push(x, (H - 1) * W + x);
  for (let y = 0; y < H; y++) stack.push(y * W, y * W + W - 1);
  while (stack.length) {
    const i = stack.pop()!;
    if (seen[i] || !bg(i)) continue;
    seen[i] = 1;
    d[i * 4 + 3] = 0;
    const x = i % W;
    if (x > 0) stack.push(i - 1);
    if (x < W - 1) stack.push(i + 1);
    if (i >= W) stack.push(i - W);
    if (i < W * (H - 1)) stack.push(i + W);
  }
  ctx.putImageData(img, 0, 0);
}

/** The largest fully solid (opaque) rectangle of a picture with see-through parts — a display lid's art without the
 *  corners its header and shoulders leave (Lorcana) — cut out as a canvas. The whole picture when that rectangle is small. */
function solidCore(c: HTMLCanvasElement): HTMLCanvasElement {
  const k = Math.max(1, c.width / 160), W = Math.round(c.width / k), H = Math.max(1, Math.round(c.height / k));
  const small = canvas(W, H), sctx = small.getContext('2d', { willReadFrequently: true })!;
  sctx.drawImage(c, 0, 0, W, H);
  const a = sctx.getImageData(0, 0, W, H).data;
  // Largest rectangle of solid cells: per row, the run of solid cells above each column, then the histogram method.
  const run = new Array(W).fill(0);
  let best = { x: 0, y: 0, w: W, h: H, area: 0 };
  for (let y = 0; y < H; y++) {
    for (let x = 0; x < W; x++) run[x] = a[(y * W + x) * 4 + 3] > 230 ? run[x] + 1 : 0;
    const stack: number[] = [];
    for (let x = 0; x <= W; x++) {
      const h = x < W ? run[x] : 0;
      while (stack.length && run[stack[stack.length - 1]] >= h) {
        const top = run[stack.pop()!], left = stack.length ? stack[stack.length - 1] + 1 : 0;
        if (top * (x - left) > best.area) best = { x: left, y: y - top + 1, w: x - left, h: top, area: top * (x - left) };
      }
      stack.push(x);
    }
  }
  if (best.area < W * H * 0.35) return c; // mostly see-through: keep it all rather than a sliver
  const sx = c.width / W, sy = c.height / H;
  return crop(c, [best.x * sx / c.width, best.y * sy / c.height, (best.x + best.w) * sx / c.width, (best.y + best.h) * sy / c.height]);
}

/** Share of a picture's pixels that are (mostly) opaque. */
function opaqueShare(c: HTMLCanvasElement): number {
  const k = Math.max(1, c.width / 128), W = Math.round(c.width / k), H = Math.max(1, Math.round(c.height / k));
  const small = canvas(W, H), ctx = small.getContext('2d', { willReadFrequently: true })!;
  ctx.drawImage(c, 0, 0, W, H);
  const d = ctx.getImageData(0, 0, W, H).data;
  let n = 0;
  for (let i = 3; i < d.length; i += 4) if (d[i] > 200) n++;
  return n / (W * H);
}

/** True when a quad is just the picture's own corners (nothing to straighten). */
function wholePicture(q: number[], pic: Picture): boolean {
  const c = [0, 0, pic.width, 0, pic.width, pic.height, 0, pic.height];
  return q.every((v, i) => Math.abs(v - c[i]) < 1.5);
}

const quad = (q: number[]): Pt[] => [0, 2, 4, 6].map((i) => [q[i], q[i + 1]] as Pt);

/** Relative brightness of a #rrggbb colour (0..1). */
function lum(hex: string): number {
  const n = parseInt(hex.slice(1), 16);
  return (0.299 * ((n >> 16) & 255) + 0.587 * ((n >> 8) & 255) + 0.114 * (n & 255)) / 255;
}

/** A one-colour silhouette of an icon (512 px tall): Scryfall's set icons are black and vanish on dark packaging. */
function silhouette(icon: Picture, color: string): HTMLCanvasElement {
  const h = 512, w = Math.max(1, Math.round((h * icon.width) / Math.max(1, icon.height)));
  const c = canvas(w, h), ctx = c.getContext('2d')!;
  ctx.drawImage(icon, 0, 0, w, h);
  ctx.globalCompositeOperation = 'source-in';
  ctx.fillStyle = color;
  ctx.fillRect(0, 0, w, h);
  return c;
}

function shade(hex: string, f: number): string {
  const n = parseInt(hex.slice(1), 16);
  return '#' + [16, 8, 0].map((s) => Math.max(0, Math.min(255, Math.round(((n >> s) & 255) * f))).toString(16).padStart(2, '0')).join('');
}

type Saver = (part: string, c: HTMLCanvasElement) => Promise<string>;

/** The set icon on a background colour: one-colour SVG icons become white on dark and near-black on light packaging;
 *  colour logos (PNG) stay as they are. null without an icon. */
async function iconMaker(projectId: string, iconRel: string, save: Saver) {
  const icon = iconRel ? await loadImage(projectFile(projectId, iconRel)).catch(() => null) : null;
  const cache: Record<string, { rel: string; pic: Picture }> = {};
  return async (bg: string) => {
    if (!icon) return null;
    if (!iconRel.endsWith('.svg')) return { rel: iconRel, pic: icon as Picture };
    const tone = lum(bg) < 0.55 ? 'light' : 'dark';
    if (!cache[tone]) {
      const pic = silhouette(icon, tone === 'light' ? '#f4f4f4' : '#161616');
      cache[tone] = { rel: await save('icon_' + tone, pic), pic };
    }
    return cache[tone];
  };
}

// ---------------------------------------------------------------- generate

interface SmartResult { pack: Layout; box: Layout; notes: string[] }

/** Thrown when a pack has no product photos and no card images: callers use the simple generator instead. */
class NoSources extends Error {
  constructor() { super('no product photos and no card images for this pack'); }
}

async function smartGenerate(job: Job): Promise<SmartResult> {
  const { projectId, src } = job;
  const url = (rel: string) => projectFile(projectId, rel);
  const save = (part: string, c: HTMLCanvasElement) => App.SaveAutoArt(projectId, job.packId, part, c.toDataURL('image/png'));
  const notes = [...(src.notes ?? [])];

  const iconOn = await iconMaker(projectId, src.icon, save);
  const arts: HTMLCanvasElement[] = [];
  for (const c of src.cards ?? []) {
    try { arts.push(crop(await loadImage(url(c.image)), c.window)); } catch { /* missing image */ }
  }
  let packPhoto: Picture | null = src.packPhoto ? await loadImage(url(src.packPhoto)) : null;
  // The pack as marked in the dialog: straightened out of the photo (a photo at an angle, or with extras around the pack).
  let packRel = src.packPhoto, packStraight: Layer['straighten'];
  if (packPhoto && src.packQuad?.length === 8 && !wholePicture(src.packQuad, packPhoto)) {
    const q = quad(src.packQuad), len = (a: Pt, b: Pt) => Math.hypot(a[0] - b[0], a[1] - b[1]);
    const c = rectify(packPhoto, q, Math.round((len(q[0], q[1]) + len(q[3], q[2])) / 2), Math.round((len(q[0], q[3]) + len(q[1], q[2])) / 2));
    packRel = await save('packfront', c);
    packStraight = { src: src.packPhoto, pts: [...src.packQuad] };
    packPhoto = c;
  }
  if (!packPhoto && !arts.length) throw new NoSources();

  // ---- pack
  const P = job.packModel, pack = newLayout();
  pack.baseItem = 'BasicCardPack';
  const look: Picture = packPhoto ?? arts[0];
  const c = packPhoto ? edgeColors(packPhoto) : dominantColors(arts[0]);
  if (!packPhoto) {
    c.top = shade(c.top, 0.8); c.bottom = shade(c.bottom, 0.55); // card art colours are bright: darker packaging
    pack.layers.push(fill(P, 'crimps', 'crimpTop', c.top), fill(P, 'crimps (bottom)', 'crimpBottom', c.bottom));
  }
  pack.layers.push(fill(P, 'front', 'front', c.top, c.bottom), fill(P, 'back', 'back', c.top, c.bottom),
    fill(P, 'edges', 'edges', c.top, c.bottom), fill(P, 'seam', 'seam', c.top, c.bottom));
  // Back: the art faded over the packaging colours and the set icon (no set name on packs: user choice 2026-10-03).
  const band = packPhoto ? crop(packPhoto, [0.05, 0.3, 0.95, 0.75]) : arts[0];
  const backArt = await save('packback', cover(band, aspectOf(P, 'back'), 768, 'blur(5px) brightness(0.8)'));
  pack.layers.push(image(P, 'back art', backArt, 'back', 0.45));
  // The back flap lies over the middle of the back: the same faded art, its middle strip, so the flap doesn't stand out as solid colour.
  const flap = faceOf(P, 'seam').net[2] / shownNet(faceOf(P, 'back'))[2];
  pack.layers.push({ ...image(P, 'back art (flap)', backArt, 'seam', 0.45), crop: [0.5 - flap / 2, 0, 0.5 + flap / 2, 1] });
  const backIcon = await iconOn(c.top);
  if (backIcon) pack.layers.push(contain(P, 'back icon', backIcon.rel, backIcon.pic, 'back', 0.45, 0.9));
  if (packPhoto) {
    // The photo includes its crimps and has nearly their shape: one layer over crimp, front and crimp, which lie on top of
    // each other in the net; every area of those faces (the crimps' back and ends too) shows its part.
    pack.layers.push(placeAt({ ...base(src.packName || 'pack photo', 'image'), src: packRel, straighten: packStraight },
      shownArea(['crimpTop', 'front', 'crimpBottom'].map((f) => faceOf(P, f)))));
  } else {
    pack.layers.push(image(P, `art – ${src.cards[0]?.name ?? 'card'}`, await save('packfront', cover(arts[0], aspectOf(P, 'front'))), 'front'));
    const frontIcon = await iconOn(dominantColors(arts[0]).bottom);
    if (frontIcon) pack.layers.push(contain(P, 'icon', frontIcon.rel, frontIcon.pic, 'front', 1, 0.95, [0, 0.8, 1, 0.16]));
  }

  // ---- box
  const B = job.boxModel, box = newLayout();
  box.baseItem = 'BasicCardBox';
  const boxPhoto = src.boxPhoto && src.box ? await loadImage(url(src.boxPhoto)) : null;
  if (boxPhoto && src.box) {
    // Like a real closed box (the user's hand-made Modern Horizons 3 box, 2026-10-03): the front panel — set name and logo —
    // fills front & back, the lid art goes on top and on the sides. Both are straightened out of the display photo with
    // their corners kept (Adjust straighten…); the dialog lets the user fix the corners before generating.
    const pan = rectify(boxPhoto, quad(src.box.front), 1024, 1024 / aspectOf(B, 'front'));
    const lid = rectify(boxPhoto, quad(src.box.lid), 1024, 1024 / aspectOf(B, 'top'));
    clearBackground(pan);
    clearBackground(lid);
    const pc = edgeColors(pan), lc = edgeColors(lid);
    box.layers.push(fill(B, 'front', 'front', pc.top, pc.bottom), fill(B, 'top', 'top', lc.top, lc.bottom), fill(B, 'sides', 'right', lc.top, pc.bottom));
    box.layers.push(image(B, 'front panel', await save('boxpanel', pan), 'front', 1, { src: src.boxPhoto, pts: src.box.front }));
    // A lid cut that is mostly background (detection missed it: Welcome to Rathe 1st Ed.) → the pack photo's art on top and
    // sides instead of an empty lid; the dialog lets the user fix the lid corners instead.
    if (opaqueShare(lid) < 0.7 && packPhoto) {
      const art = crop(packPhoto, [0.06, 0.22, 0.94, 0.72]);
      box.layers.push(image(B, 'top art', await save('boxtop', cover(art, aspectOf(B, 'top'))), 'top'));
      box.layers.push(image(B, 'side art', await save('boxside', cover(art, aspectOf(B, 'right'))), 'right'));
      notes.push('The lid in the box photo was hard to find — top and sides use the pack art (Smart generate… lets you mark the lid).');
    } else {
      box.layers.push(image(B, 'lid art', await save('boxtop', lid), 'top', 1, { src: src.boxPhoto, pts: src.box.lid }));
      // Sides: the middle of the lid's solid art (its main character), cut to the side's shape.
      box.layers.push(image(B, 'side art', await save('boxside', cover(solidCore(lid), aspectOf(B, 'right'))), 'right'));
    }
  } else {
    // No box photo: the second-best card art (or the pack photo's art) with the set name, and the pack photo stacked.
    const art = arts[1] ?? arts[0] ?? crop(look, [0.05, 0.3, 0.95, 0.75]);
    const ac = dominantColors(art);
    box.layers.push(fill(B, 'top', 'top', shade(ac.top, 0.7), shade(ac.bottom, 0.5)), fill(B, 'sides', 'right', shade(ac.top, 0.7), shade(ac.bottom, 0.5)));
    box.layers.push(image(B, 'front art', await save('boxfront', cover(art, aspectOf(B, 'front'))), 'front'));
    box.layers.push(image(B, 'top art', await save('boxtop', cover(arts[2] ?? art, aspectOf(B, 'top'), 1024, 'brightness(0.85)')), 'top'));
    const textW = packPhoto ? 0.58 : 0.9;
    box.layers.push(text(B, 'title', job.title.toUpperCase(), 'front', [0.04, 0.06, textW, 0.22], 0.13));
    // "BOOSTER BOX" centred under the title (narrower box = smaller text).
    box.layers.push(text(B, 'booster box', 'BOOSTER BOX', 'front', [0.04 + textW * 0.15, 0.3, textW * 0.7, 0.12], 0.08));
    if (packPhoto) {
      // Three packs stacked at the right of the front (sizes as fractions of the front, seen upright).
      const h = 0.78, w = (h * packPhoto.width) / packPhoto.height / aspectOf(B, 'front');
      for (let i = 0; i < 3; i++) {
        const x = 1 - w * 0.55 - (2 - i) * w * 0.22, y = 0.55 + (2 - i) * 0.012;
        box.layers.push(image(B, `pack ${i + 1}`, packRel, 'front', 1, undefined, [x - w / 2, y - h / 2, w, h]));
      }
    }
    const sideIcon = await iconOn(shade(ac.top, 0.7));
    if (sideIcon) box.layers.push(contain(B, 'side icon', sideIcon.rel, sideIcon.pic, 'right', 0.6, 0.9));
    if (src.cards.length) notes.push(`Box made from card art (${src.cards.slice(0, 3).map((c) => c.name).join(', ')}).`);
  }
  // The opposite sides get the same art: like a real box, and every side of the model is covered.
  box.layers.push(...onOpposite(B, box.layers, 'front', 'back'), ...onOpposite(B, box.layers, 'right', 'left'), ...onOpposite(B, box.layers, 'top', 'bottom'));
  return { pack, box, notes };
}

/** True when a layout has layers Smart generate didn't make or a painted texture (so replacing it would lose hand work). */
export const hasOwnLayers = (l?: Layout) => !!l?.textureFile || !!l?.layers.some((x) => !x.name.startsWith(AUTO));

/** A layout's texture and shop icon as PNG data URLs, without the editor (same steps as AccessoryEditor.exportImages). */
async function renderLayout(projectId: string, m: Model, layout: Layout): Promise<{ texture: string; icon: string }> {
  const b = m.bases?.find((x) => x.id === layout.baseItem) ?? m.bases?.[0];
  if (!b) throw new Error("the game templates aren't available yet (Settings → Game)");
  const vanilla = await loadImage(templateFile('game', b.texture));
  const S = netScale(m), net = document.createElement('canvas');
  const tex = (await renderTexture(m, layout, vanilla, null, (rel) => projectFile(projectId, rel), S, net))();
  const icon = await textureIcon(m, tex, net, S); // the same icon the editor makes
  return { texture: tex.toDataURL('image/png'), icon };
}

// ---------------------------------------------------------------- classic design

/** The classic generator's choices: packaging colour, title, an own front picture (set file, '' = the set icon) with or
 *  without the title on it, a font (set file, '' = Studio's bold); noText = no words on the pack (Smart's fallback). */
export interface ClassicOptions { color: string; title: string; front: string; titleOnFront: boolean; font: string; noText?: boolean }

/** Lighter or darker shade of a colour (f > 1 = towards white). */
function tint(hex: string, f: number): string {
  if (f <= 1) return shade(hex, f);
  const n = parseInt(hex.slice(1), 16), k = f - 1;
  return '#' + [16, 8, 0].map((s) => { const v = (n >> s) & 255; return Math.round(v + (255 - v) * k).toString(16).padStart(2, '0'); }).join('');
}

/** A text layer sized to fit a part of a face, measured in its font (text() estimates the width of Studio's bold). */
function fittedText(m: Model, name: string, t: string, face: string, part: Part, maxH: number, emWidth: number, font: string): Layer {
  const h = Math.min(maxH, (part[2] * aspectOf(m, face)) / Math.max(0.5, emWidth));
  return placeAt({ ...base(name, 'text'), text: t, color: '#ffffff', outline: '#000000', ...(font ? { font } : {}) },
    shownNet(faceOf(m, face)), [part[0], part[1] + part[3] / 2 - h / 2, part[2], h]);
}

/** The classic design — colour on every face, the set icon or your own front picture, the title and "BOOSTER BOX" — as face
 *  layers like Smart generate's, so every face is covered and the 3D editor can change any part. */
async function classicGenerate(job: { projectId: string; packId: string; cards: number; packModel: Model; boxModel: Model },
  o: ClassicOptions): Promise<SmartResult> {
  const { projectId } = job;
  const save: Saver = (part, c) => App.SaveAutoArt(projectId, job.packId, part, c.toDataURL('image/png'));
  // The set's own icon, as Smart generate finds it.
  let iconRel = '';
  for (const rel of ['images/set_icon.svg', 'images/set_logo.png']) {
    if (await loadImage(projectFile(projectId, rel)).then(() => true, () => false)) { iconRel = rel; break; }
  }
  const iconOn = await iconMaker(projectId, iconRel, save);
  // Width of a text in em of the chosen font (Studio's bold when there's none or it can't load).
  const fam = o.font ? await fontFamily(projectFile(projectId, o.font)).catch(() => '') : '';
  const meter = canvas(1, 1).getContext('2d')!;
  meter.font = fam ? `100px "${fam}"` : 'bold 100px Nunito, "Segoe UI", sans-serif';
  const em = (t: string) => meter.measureText(t).width / 100;
  const font = fam ? o.font : '';
  const top = tint(o.color, 1.25), bottom = shade(o.color, 0.55);
  const title = (o.title || 'BOOSTER').toUpperCase();

  // ---- pack
  const P = job.packModel, pack = newLayout();
  pack.baseItem = 'BasicCardPack';
  pack.layers.push(fill(P, 'crimps', 'crimpTop', top), fill(P, 'crimps (bottom)', 'crimpBottom', bottom),
    fill(P, 'front', 'front', top, bottom), fill(P, 'back', 'back', top, bottom),
    fill(P, 'edges', 'edges', top, bottom), fill(P, 'seam', 'seam', top, bottom));
  const frontPic = o.front ? await loadImage(projectFile(projectId, o.front)).catch(() => null) : null;
  if (frontPic) pack.layers.push(image(P, 'front picture', await save('packfront', cover(frontPic, aspectOf(P, 'front'))), 'front'));
  else {
    const ic = await iconOn(o.color);
    if (ic) pack.layers.push(contain(P, 'icon', ic.rel, ic.pic, 'front', 1, 1, [0.14, 0.3, 0.72, 0.42]));
  }
  if (!o.noText && (!frontPic || o.titleOnFront)) {
    pack.layers.push(fittedText(P, 'title', title, 'front', [0.06, 0.05, 0.88, 0.2], 0.12, em(title), font));
    if (!frontPic) {
      const n = `${job.cards} CARDS`;
      pack.layers.push(fittedText(P, 'card count', n, 'front', [0.25, 0.8, 0.5, 0.1], 0.06, em(n), font));
    }
  }
  const backIcon = await iconOn(o.color);
  if (backIcon) pack.layers.push(contain(P, 'back icon', backIcon.rel, backIcon.pic, 'back', 0.45, 0.9));

  // ---- box: front, top and one side made, the opposite faces get copies (every face covered, like Smart generate)
  const B = job.boxModel, box = newLayout();
  box.baseItem = 'BasicCardBox';
  box.layers.push(fill(B, 'front', 'front', top, bottom), fill(B, 'top', 'top', top, bottom), fill(B, 'sides', 'right', top, bottom));
  if (frontPic) box.layers.push(image(B, 'front picture', await save('boxfront', cover(frontPic, aspectOf(B, 'front'))), 'front', 0.9));
  const boxIcon = await iconOn(o.color);
  if (boxIcon && !frontPic) box.layers.push(contain(B, 'icon', boxIcon.rel, boxIcon.pic, 'front', 1, 1, [0.3, 0.45, 0.4, 0.45]));
  box.layers.push(fittedText(B, 'title', title, 'front', [0.05, 0.06, 0.9, 0.22], 0.14, em(title), font));
  box.layers.push(fittedText(B, 'booster box', 'BOOSTER BOX', 'front', [0.2, 0.3, 0.6, 0.12], 0.08, em('BOOSTER BOX'), font));
  box.layers.push(fittedText(B, 'top title', title, 'top', [0.06, 0.25, 0.88, 0.5], 0.4, em(title), font));
  if (boxIcon) box.layers.push(contain(B, 'side icon', boxIcon.rel, boxIcon.pic, 'right', 0.6, 0.9));
  box.layers.push(...onOpposite(B, box.layers, 'front', 'back'), ...onOpposite(B, box.layers, 'right', 'left'), ...onOpposite(B, box.layers, 'top', 'bottom'));
  return { pack, box, notes: [] };
}

const BACKING = 'backing colour';

/** Puts one colour under everything, on the texture itself (a map layer over the whole texture): the face areas leave
 *  hairline seams between them (pack: beside the front, the back seam, where the crimps meet), where the vanilla print
 *  would show as thin lines along the sides. No face is stretched over another. Colour = the design's first fill's. */
function withBacking(m: Model, layout: Layout) {
  if (layout.layers.some((l) => l.name === AUTO + BACKING)) return;
  const f = layout.layers.find((l) => l.kind === 'fill');
  if (!f) return;
  const s = m.textureSize;
  layout.layers.unshift({ ...base(BACKING, 'fill'), space: 'texture', x: s / 2, y: s / 2, w: s, h: s, color: f.color2 || f.color });
}

const loadModels = async (models?: { pack: Model; box: Model }) => models ? [models.pack, models.box]
  : (await Promise.all([App.AccessoryModel('Pack'), App.AccessoryModel('Box')])) as unknown as Model[];

/** Stores a pack's designs in meta.packArt, writes their textures and icons and points the pack at them (the 3D editor then
 *  opens on these layers). The caller saves the project. */
async function applyLayouts(project: any, pack: any, r: SmartResult, packModel: Model, boxModel: Model) {
  withBacking(packModel, r.pack);
  withBacking(boxModel, r.box);
  project.meta ??= {};
  project.meta.packArt ??= {};
  project.meta.packArt[pack.id] = { pack: r.pack, box: r.box };
  const p = await renderLayout(project.id, packModel, r.pack);
  const pf = await App.SavePackArt(project.id, pack.id, 'pack', p.texture, p.icon);
  pack.packTexture = pf.texture; pack.packIcon = pf.icon;
  project.meta.packArt[pack.id].pack.madeTexture = pf.texture; // the 3D editor opens on these layers while the art is theirs
  if (pack.hasBox) {
    const b = await renderLayout(project.id, boxModel, r.box);
    const bf = await App.SavePackArt(project.id, pack.id, 'box', b.texture, b.icon);
    pack.boxTexture = bf.texture; pack.boxIcon = bf.icon;
    project.meta.packArt[pack.id].box.madeTexture = bf.texture;
  }
}

/** The classic generator: builds the classic design as face layers and applies it. The caller saves the project. */
export async function applyClassicArt(project: any, pack: any, o: ClassicOptions, models?: { pack: Model; box: Model }) {
  const [packModel, boxModel] = await loadModels(models);
  const r = await classicGenerate({ projectId: project.id, packId: pack.id, cards: pack.cardsPerPack || 7, packModel, boxModel }, o);
  await applyLayouts(project, pack, r, packModel, boxModel);
}

/** Builds and applies one pack's art from its sources (automatic, or picked and adjusted in the dialog). Nothing to build
 *  from → the classic design in the set's colour without words on the pack. The caller saves the project. Returns the notes. */
export async function applySmartArt(project: any, pack: any, src: SmartSources, models?: { pack: Model; box: Model }): Promise<string[]> {
  const [packModel, boxModel] = await loadModels(models);
  try {
    const r = await smartGenerate({ projectId: project.id, packId: pack.id, title: project.set.name || 'BOOSTER', src, packModel, boxModel });
    await applyLayouts(project, pack, r, packModel, boxModel);
    return r.notes;
  } catch (e) {
    if (!(e instanceof NoSources)) throw e;
    await applyClassicArt(project, pack, { color: await App.DefaultArtColor(project.id), title: project.set.name, front: '', titleOnFront: false,
      font: '', noText: true }, { pack: packModel, box: boxModel });
    return [`${pack.name || pack.id}: no product photos or card images — used a simple colour + set icon design.`];
  }
}

/** Smart generate for every pack of a project, choosing photos automatically (imports): pack n ↔ n-th booster product.
 *  The caller saves the project. Returns the notes. */
export async function smartArtForProject(project: any, status: (s: string) => void = () => {}): Promise<string[]> {
  const [pack, box] = (await Promise.all([App.AccessoryModel('Pack'), App.AccessoryModel('Box')])) as unknown as Model[];
  const notes: string[] = [];
  for (const pk of project.set.packs ?? []) {
    const name = pk.name || pk.id;
    status(`Pack art for ${name}: looking for product photos…`);
    const src = (await App.SmartArtSources(project.id, pk.id, { manual: false } as any)) as unknown as SmartSources;
    status(`Pack art for ${name}: building…`);
    notes.push(...(await applySmartArt(project, pk, src, { pack, box })));
  }
  return [...new Set(notes)];
}

/** A box for a pack that has its own art but none for a box (e.g. an EPL mod that sells only packs): the game's booster
 *  box with the pack's front on it (App.BoxFromPack), its shop icon rendered from the box model like the 3D editor does.
 *  Turns the box on and points the pack at the files; the pack's own art stays. The caller saves the project. */
export async function boxFromPackArt(project: any, pack: any, boxModel?: Model): Promise<void> {
  const m = boxModel ?? ((await App.AccessoryModel('Box')) as unknown as Model);
  const r: any = await App.BoxFromPack(project.id, pack.id);
  const layout = JSON.parse(r.layout) as Layout;
  const b = await renderLayout(project.id, m, layout);
  const bf = await App.SavePackArt(project.id, pack.id, 'box', b.texture, b.icon);
  layout.madeTexture = bf.texture; // the 3D editor opens on this box while the art is its
  pack.hasBox = true;
  pack.boxTexture = bf.texture; pack.boxIcon = bf.icon;
  project.meta ??= {};
  project.meta.packArt ??= {};
  project.meta.packArt[pack.id] ??= {};
  project.meta.packArt[pack.id].box = layout;
  // This box is new, so it's in the current layout even when the pack's art still waits to be converted (an older mod's).
  project.meta.artKinds = [...new Set([...(project.meta.artKinds ?? []), 'Box'])];
}

/** Boxes for every pack of a project that has pack art but no box art (see boxFromPackArt). Returns how many were made. */
export async function boxesFromPacks(project: any): Promise<number> {
  const todo = (project.set?.packs ?? []).filter((p: any) => p.packTexture && !p.boxTexture && p.hasBox);
  if (!todo.length) return 0;
  const m = (await App.AccessoryModel('Box')) as unknown as Model;
  for (const pk of todo) await boxFromPackArt(project, pk, m);
  return todo.length;
}
