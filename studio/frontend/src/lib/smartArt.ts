// Smart generate: builds a pack's and its box's editor layouts from TCGplayer product photos (App.SmartArtSources), or from
// the set's own card art when there are none. Everything ends up as ordinary layers (named "Auto: …") that can be moved,
// hidden or recoloured; box faces cut from the display photo keep their corners (layer.straighten) for Adjust straighten….
import { App, projectFile } from './api';
import { composeIcon, composeTexture, meshIcon, dominantColors, edgeColors, layerId, loadImage, netScale, newLayout, renderNet, type Layer, type Layout, type Model } from './accessoryArt';
import { rectify, type Pt } from './perspective';

export const AUTO = 'Auto: ';

type Picture = CanvasImageSource & { width: number; height: number };
type Area = number[]; // x, y, w, h in net units

export interface SmartSources {
  packPhoto: string; packName: string; boxPhoto: string; boxName: string;
  box?: { front: number[]; lid: number[]; confident: boolean } | null;
  packQuad?: number[];          // dialog only: the pack (crimps included) in the pack photo, tl tr br bl; none = whole photo
  cards: { name: string; image: string; window: number[] }[];
  icon: string; notes: string[];
}


interface Job { projectId: string; packId: string; title: string; src: SmartSources; packModel: Model; boxModel: Model }

// ---------------------------------------------------------------- layers

function area(m: Model, ids: string[]): Area {
  const fs = m.faces.filter((f) => ids.includes(f.id));
  const x0 = Math.min(...fs.map((f) => f.net[0])), y0 = Math.min(...fs.map((f) => f.net[1]));
  return [x0, y0, Math.max(...fs.map((f) => f.net[0] + f.net[2])) - x0, Math.max(...fs.map((f) => f.net[1] + f.net[3])) - y0];
}

const base = (name: string, kind: Layer['kind'], opacity = 1): Layer =>
  ({ id: layerId(), kind, name: AUTO + name, visible: true, opacity, blend: 'source-over', x: 0, y: 0, w: 0, h: 0, rot: 0 });

function image(name: string, src: string, a: Area, opacity = 1, straighten?: Layer['straighten']): Layer {
  return { ...base(name, 'image', opacity), src, x: a[0] + a[2] / 2, y: a[1] + a[3] / 2, w: a[2], h: a[3], straighten };
}

/** An image layer keeping the picture's shape, as large as fits in a (scale ≤ 1), centred. */
function contain(name: string, src: string, pic: Picture, a: Area, scale = 1, opacity = 1): Layer {
  const asp = pic.width / Math.max(1, pic.height);
  let w = a[2] * scale, h = w / asp;
  if (h > a[3] * scale) { h = a[3] * scale; w = h * asp; }
  return { ...base(name, 'image', opacity), src, x: a[0] + a[2] / 2, y: a[1] + a[3] / 2, w, h };
}

function fill(name: string, face: string, color: string, color2 = ''): Layer {
  return { ...base(name, 'fill'), face, color, color2 };
}

/** A text layer as large as fits in a (rough width estimate: 0.62 em per letter, bold). */
function text(name: string, t: string, a: Area, maxH: number): Layer {
  const h = Math.min(maxH, a[2] / Math.max(1, t.length * 0.62));
  return { ...base(name, 'text'), text: t, color: '#ffffff', outline: '#000000', x: a[0] + a[2] / 2, y: a[1] + a[3] / 2, w: a[2], h };
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

// ---------------------------------------------------------------- generate

export interface SmartResult { pack: Layout; box: Layout; notes: string[] }

/** Thrown when a pack has no product photos and no card images: callers use the simple generator instead. */
export class NoSources extends Error {
  constructor() { super('no product photos and no card images for this pack'); }
}

export async function smartGenerate(job: Job): Promise<SmartResult> {
  const { projectId, src } = job;
  const url = (rel: string) => projectFile(projectId, rel);
  const save = (part: string, c: HTMLCanvasElement) => App.SaveAutoArt(projectId, job.packId, part, c.toDataURL('image/png'));
  const notes = [...(src.notes ?? [])];

  const icon = src.icon ? await loadImage(url(src.icon)).catch(() => null) : null;
  // The icon on a background colour: one-colour SVG icons become white on dark and near-black on light packaging;
  // colour logos (PNG) stay as they are.
  const iconCache: Record<string, { rel: string; pic: Picture }> = {};
  const iconOn = async (bg: string) => {
    if (!icon) return null;
    if (!src.icon.endsWith('.svg')) return { rel: src.icon, pic: icon };
    const tone = lum(bg) < 0.55 ? 'light' : 'dark';
    if (!iconCache[tone]) {
      const pic = silhouette(icon, tone === 'light' ? '#f4f4f4' : '#161616');
      iconCache[tone] = { rel: await save('icon_' + tone, pic), pic };
    }
    return iconCache[tone];
  };
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
  const back = area(P, ['back']), front = area(P, ['front']);
  const look: Picture = packPhoto ?? arts[0];
  const c = packPhoto ? edgeColors(packPhoto) : dominantColors(arts[0]);
  if (!packPhoto) {
    c.top = shade(c.top, 0.8); c.bottom = shade(c.bottom, 0.55); // card art colours are bright: darker packaging
    pack.layers.push(fill('crimps', 'crimpTop', c.top), fill('crimps (bottom)', 'crimpBottom', c.bottom));
  }
  pack.layers.push(fill('back', 'back', c.top, c.bottom), fill('edges', 'edges', c.top, c.bottom), fill('seam', 'seam', c.top, c.bottom));
  // Back: the art faded over the packaging colours and the set icon (no set name on packs: user choice 2026-10-03).
  const band = packPhoto ? crop(packPhoto, [0.05, 0.3, 0.95, 0.75]) : arts[0];
  pack.layers.push(image('back art', await save('packback', cover(band, back[2] / back[3], 768, 'blur(5px) brightness(0.8)')), back, 0.45));
  const backIcon = await iconOn(c.top);
  if (backIcon) pack.layers.push(contain('back icon', backIcon.rel, backIcon.pic, back, 0.45, 0.9));
  if (packPhoto) {
    // The photo includes its crimps and has nearly their shape: stretched over crimps + front.
    pack.layers.push(image(src.packName || 'pack photo', packRel, area(P, ['crimpTop', 'front', 'crimpBottom']), 1, packStraight));
  } else {
    pack.layers.push(image(`art – ${src.cards[0]?.name ?? 'card'}`, await save('packfront', cover(arts[0], front[2] / front[3])), front));
    const frontIcon = await iconOn(dominantColors(arts[0]).bottom);
    if (frontIcon) pack.layers.push(contain('icon', frontIcon.rel, frontIcon.pic, [front[0], front[1] + front[3] * 0.8, front[2], front[3] * 0.16], 1, 0.95));
  }

  // ---- box
  const B = job.boxModel, box = newLayout();
  box.baseItem = 'BasicCardBox';
  const bf = area(B, ['front']), top = area(B, ['top']), side = area(B, ['right']);
  const boxPhoto = src.boxPhoto && src.box ? await loadImage(url(src.boxPhoto)) : null;
  if (boxPhoto && src.box) {
    // Like a real closed box (the user's hand-made Modern Horizons 3 box, 2026-10-03): the front panel — set name and logo —
    // fills front & back, the lid art goes on top and on the sides. Both are straightened out of the display photo with
    // their corners kept (Adjust straighten…); the dialog lets the user fix the corners before generating.
    const pan = rectify(boxPhoto, quad(src.box.front), 1024, (1024 * bf[3]) / bf[2]);
    const lid = rectify(boxPhoto, quad(src.box.lid), 1024, (1024 * top[3]) / top[2]);
    clearBackground(pan);
    clearBackground(lid);
    const pc = edgeColors(pan), lc = edgeColors(lid);
    box.layers.push(fill('front', 'front', pc.top, pc.bottom), fill('top', 'top', lc.top, lc.bottom), fill('sides', 'right', lc.top, pc.bottom));
    box.layers.push(image('front panel', await save('boxpanel', pan), bf, 1, { src: src.boxPhoto, pts: src.box.front }));
    // A lid cut that is mostly background (detection missed it: Welcome to Rathe 1st Ed.) → the pack photo's art on top and
    // sides instead of an empty lid; the dialog lets the user fix the lid corners instead.
    if (opaqueShare(lid) < 0.7 && packPhoto) {
      const art = crop(packPhoto, [0.06, 0.22, 0.94, 0.72]);
      box.layers.push(image('top art', await save('boxtop', cover(art, top[2] / top[3])), top));
      box.layers.push(image('side art', await save('boxside', cover(art, side[2] / side[3])), side));
      notes.push('The lid in the box photo was hard to find — top and sides use the pack art (Smart generate… lets you mark the lid).');
    } else {
      box.layers.push(image('lid art', await save('boxtop', lid), top, 1, { src: src.boxPhoto, pts: src.box.lid }));
      // Sides: the middle of the lid's solid art (its main character), cut to the side's shape.
      box.layers.push(image('side art', await save('boxside', cover(solidCore(lid), side[2] / side[3])), side));
    }
  } else {
    // No box photo: the second-best card art (or the pack photo's art) with the set name, and the pack photo stacked.
    const art = arts[1] ?? arts[0] ?? crop(look, [0.05, 0.3, 0.95, 0.75]);
    const ac = dominantColors(art);
    box.layers.push(fill('top', 'top', shade(ac.top, 0.7), shade(ac.bottom, 0.5)), fill('sides', 'right', shade(ac.top, 0.7), shade(ac.bottom, 0.5)));
    box.layers.push(image('front art', await save('boxfront', cover(art, bf[2] / bf[3])), bf));
    box.layers.push(image('top art', await save('boxtop', cover(arts[2] ?? art, top[2] / top[3], 1024, 'brightness(0.85)')), top));
    const textW = packPhoto ? 0.58 : 0.9;
    box.layers.push(text('title', job.title.toUpperCase(), [bf[0] + bf[2] * 0.04, bf[1] + bf[3] * 0.06, bf[2] * textW, bf[3] * 0.22], bf[3] * 0.13));
    // "BOOSTER BOX" centred under the title (narrower box = smaller text).
    box.layers.push(text('booster box', 'BOOSTER BOX', [bf[0] + bf[2] * (0.04 + textW * 0.15), bf[1] + bf[3] * 0.3, bf[2] * textW * 0.7, bf[3] * 0.12], bf[3] * 0.08));
    if (packPhoto) {
      const h = bf[3] * 0.78, w = (h * packPhoto.width) / packPhoto.height;
      for (let i = 0; i < 3; i++) {
        const x = bf[0] + bf[2] - w * 0.55 - (2 - i) * w * 0.22, y = bf[1] + bf[3] * 0.55 + (2 - i) * bf[3] * 0.012;
        box.layers.push({ ...image(`pack ${i + 1}`, packRel, [x - w / 2, y - h / 2, w, h]) });
      }
    }
    const sideIcon = await iconOn(shade(ac.top, 0.7));
    if (sideIcon) box.layers.push(contain('side icon', sideIcon.rel, sideIcon.pic, side, 0.6, 0.9));
    if (src.cards.length) notes.push(`Box made from card art (${src.cards.slice(0, 3).map((c) => c.name).join(', ')}).`);
  }
  return { pack, box, notes };
}

/** True when a layout has layers Smart generate didn't make or a painted texture (so replacing it would lose hand work). */
export const hasOwnLayers = (l?: Layout) => !!l?.textureFile || !!l?.layers.some((x) => !x.name.startsWith(AUTO));

/** A layout's texture and shop icon as PNG data URLs, without the editor (same steps as AccessoryEditor.exportImages). */
export async function renderLayout(projectId: string, m: Model, layout: Layout): Promise<{ texture: string; icon: string }> {
  const b = m.bases?.find((x) => x.id === layout.baseItem) ?? m.bases?.[0];
  if (!b) throw new Error("the game templates aren't available yet (Settings → Game)");
  const vanilla = await loadImage(`/templates/${encodeURIComponent(b.texture)}`);
  const S = netScale(m), net = document.createElement('canvas');
  await renderNet(net, m, layout, vanilla, (rel) => projectFile(projectId, rel), S);
  const tex = composeTexture(m, net, vanilla, S);
  const icon = m.icon === 'pack' ? await meshIcon(m, tex) : composeIcon(m, net, S, 512, 512).toDataURL('image/png');
  return { texture: tex.toDataURL('image/png'), icon };
}

/** Re-renders a pack's shop icon from its texture file on the game mesh (after the Go generator, whose icon is the vanilla
 *  pack picture with the front pasted in) and points the pack at it. */
export async function packIconFromTexture(projectId: string, pack: any, m: Model) {
  const img = await loadImage(projectFile(projectId, pack.packTexture));
  const c = document.createElement('canvas');
  c.width = img.width; c.height = img.height;
  c.getContext('2d')!.drawImage(img, 0, 0);
  const f = await App.SavePackArt(projectId, pack.id, 'pack', c.toDataURL('image/png'), await meshIcon(m, c));
  pack.packTexture = f.texture; pack.packIcon = f.icon;
}

/** Builds and applies one pack's art from its sources (automatic, or picked and adjusted in the dialog): stores the
 *  layouts in meta.packArt, writes the textures and icons and points the pack at them. Nothing to build from → the simple
 *  generator's design (colour + set icon, no text). The caller saves the project. Returns the notes. */
export async function applySmartArt(project: any, pack: any, src: SmartSources, models?: { pack: Model; box: Model }): Promise<string[]> {
  const [packModel, boxModel] = models ? [models.pack, models.box]
    : (await Promise.all([App.AccessoryModel('Pack'), App.AccessoryModel('Box')])) as unknown as Model[];
  project.meta ??= {};
  project.meta.packArt ??= {};
  let r: SmartResult;
  try {
    r = await smartGenerate({ projectId: project.id, packId: pack.id, title: project.set.name || 'BOOSTER', src, packModel, boxModel });
  } catch (e) {
    if (!(e instanceof NoSources)) throw e;
    const g: any = await App.GeneratePackArt(project.id, pack.id, { color: '', title: '', icon: '', filePrefix: '', frontImage: '', titleOnImage: false, noPackText: true } as any);
    pack.packTexture = g.packTexture; pack.packIcon = g.packIcon;
    pack.boxTexture = g.boxTexture; pack.boxIcon = g.boxIcon;
    await packIconFromTexture(project.id, pack, packModel);
    delete project.meta.packArt[pack.id]; // earlier generated layouts no longer match the art
    return [`${pack.name || pack.id}: no product photos or card images — used a simple colour + set icon design.`];
  }
  project.meta.packArt[pack.id] = { pack: r.pack, box: r.box };
  const p = await renderLayout(project.id, packModel, r.pack);
  const pf = await App.SavePackArt(project.id, pack.id, 'pack', p.texture, p.icon);
  pack.packTexture = pf.texture; pack.packIcon = pf.icon;
  if (pack.hasBox) {
    const b = await renderLayout(project.id, boxModel, r.box);
    const bf = await App.SavePackArt(project.id, pack.id, 'box', b.texture, b.icon);
    pack.boxTexture = bf.texture; pack.boxIcon = bf.icon;
  }
  return r.notes;
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
  pack.hasBox = true;
  pack.boxTexture = bf.texture; pack.boxIcon = bf.icon;
  project.meta ??= {};
  project.meta.packArt ??= {};
  project.meta.packArt[pack.id] ??= {};
  project.meta.packArt[pack.id].box = layout;
}

/** Boxes for every pack of a project that has pack art but no box art (see boxFromPackArt). Returns how many were made. */
export async function boxesFromPacks(project: any): Promise<number> {
  const todo = (project.set?.packs ?? []).filter((p: any) => p.packTexture && !p.boxTexture && p.hasBox);
  if (!todo.length) return 0;
  const m = (await App.AccessoryModel('Box')) as unknown as Model;
  for (const pk of todo) await boxFromPackArt(project, pk, m);
  return todo.length;
}
