// Card borders (the setup's item library "borders" + the ones that come with the mod): a stack of image and shader layers
// drawn over a full-image card. Mirrors setfmt.Border / the mod's BorderDef. Sets give them to their border slots
// (set.json "borders", lib/cardVersions.ts slotOf/setBorder) and cards per version ("versionBorders").
import { accFile } from './urls';
import type { ShaderParam } from './cardShaderView';

export type BorderLayer = {
  kind: 'image' | 'shader';
  /** Image layer: "assets/…" in the shared store, a stock file name (stock borders) or a data: URL not saved yet. */
  file?: string;
  opacity: number;
  /** Shader layer: a card shader id (the setup's or a stock one). */
  shader?: string;
  /** Shader layer: only draw where the image layer under it is. */
  clip?: boolean;
  /** Shader layer: own values for the shader's parameters, by name. */
  params?: Record<string, number[]>;
};

export type Border = {
  id: string;
  name: string;
  icon?: string;
  /** Where the card's picture goes, set by hand on the Borders page (Card picture → In a box): x, y, width, height as
   *  fractions of the card from its top left; none = the whole card. */
  window?: number[];
  /** Back to front, over the card's picture: the layers' transparency decides what of it shows. */
  layers: BorderLayer[];
  /** Comes with the mod (ListBorders, the Borders page's stock list). */
  builtIn?: boolean;
};

/** Ids of the borders that come with the mod start with this (setfmt.BorderStockPrefix). */
export const STOCK_PREFIX = 'tcgcc-border-';
export const isStock = (b: { id: string; builtIn?: boolean }) => !!b.builtIn || b.id.startsWith(STOCK_PREFIX);

/** The size a border picture is best made at (63×88 mm, like the preview's card). Any size of this shape works. */
export const CARD_SIZE: [number, number] = [750, 1048];

/** URL of a layer's picture (or of the border's icon). */
export function borderFile(b: { id: string; builtIn?: boolean }, file: string | undefined): string {
  if (!file) return '';
  if (file.startsWith('data:')) return file;
  return isStock(b) ? `/stockborder/${encodeURIComponent(file)}` : accFile(file);
}

/** A new, empty border. */
export const newBorder = (id: string, name = ''): Border => ({ id, name, layers: [] });

/** The picture a shader layer draws on: the nearest image layer under it (undefined = the card's picture). */
export function under(b: Border, index: number): BorderLayer | undefined {
  for (let i = index - 1; i >= 0; i--) if (b.layers[i].kind === 'image' && b.layers[i].file) return b.layers[i];
  return undefined;
}

export type Box = [number, number, number, number];

/** The smallest box side, as a fraction of the card. */
export const MIN_BOX = 0.05;

/** A box kept inside the card and at least MIN_BOX on each side, rounded to 4 decimals. */
export function clampBox(b: readonly number[]): Box {
  const r4 = (v: number) => Math.round(v * 10000) / 10000;
  const w = Math.min(1, Math.max(MIN_BOX, b[2] || 0)), h = Math.min(1, Math.max(MIN_BOX, b[3] || 0));
  const x = Math.min(1 - w, Math.max(0, b[0] || 0)), y = Math.min(1 - h, Math.max(0, b[1] || 0));
  return [r4(x), r4(y), r4(w), r4(h)];
}

/** The box with one of its numbers (0 left, 1 top, 2 width, 3 height) set to v, capped so the others stay as they are. */
export function setBoxSide(b: Box, k: number, v: number): Box {
  const out: Box = [...b];
  const most = [1 - b[2], 1 - b[3], 1 - b[0], 1 - b[1]][k];
  out[k] = Math.min(most, Math.max(k < 2 ? 0 : MIN_BOX, v || 0));
  return clampBox(out);
}

/** Where the card's picture goes: the border's box, else the whole card. */
export function windowOf(b: Border | null | undefined): Box {
  const w = b?.window;
  return w && w.length === 4 ? clampBox(w) : [0, 0, 1, 1];
}

/** The box a border starts with when "In a box" is switched on: the thin rim's (the picture just inside the frame). */
export const DEFAULT_BOX = (): Box => [...THIN_RIM_WINDOW];

/** The thin rim template's window: as thin as the game's own rim around a full-image picture (3.5 % each side; the stock
 *  borders' window, cmd/stockborders). */
export const THIN_RIM_WINDOW: [number, number, number, number] = [0.0351, 0.0357, 0.9298, 0.9286];

/**
 * A picture to paint a border on: the card's shape at CARD_SIZE (rounded corners, grey) with the window cut out see-through,
 * nothing else on it (no guide lines or notes: the picture can be used as it is). kind 'panel' = a framed card (the
 * border's box, else an art box over a text panel); 'thin' = a thin rim round the whole picture (THIN_RIM_WINDOW).
 */
export function templatePng(b: Border | null, kind: 'panel' | 'thin' = 'panel'): string {
  const [W, H] = CARD_SIZE;
  const c = document.createElement('canvas');
  c.width = W; c.height = H;
  const g = c.getContext('2d')!;
  const [x, y, w, h] = kind === 'thin' ? THIN_RIM_WINDOW : b?.window?.length === 4 ? windowOf(b) : [0.07, 0.07, 0.86, 0.6];
  g.fillStyle = '#9aa0ad';
  g.beginPath(); g.roundRect(0, 0, W, H, 36); g.fill();
  g.clearRect(x * W, y * H, w * W, h * H);
  return c.toDataURL('image/png');
}

/** Loads a picture (for icons); rejects when it can't. */
function load(url: string): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const img = new Image();
    img.onload = () => resolve(img);
    img.onerror = () => reject(new Error('picture not found'));
    img.src = url;
  });
}

/** The border's list picture: its image layers over a plain card (shaders left out), 120 px wide; '' when it has none. */
export async function borderIcon(b: Border): Promise<string> {
  const imgs = b.layers.filter((l) => l.kind === 'image' && l.file);
  if (!imgs.length) return '';
  const W = 120, H = Math.round((W * CARD_SIZE[1]) / CARD_SIZE[0]);
  const c = document.createElement('canvas');
  c.width = W; c.height = H;
  const g = c.getContext('2d')!;
  const [x, y, w, h] = windowOf(b);
  g.fillStyle = '#5b6a8a';
  g.beginPath(); g.roundRect(x * W, y * H, w * W, h * H, 6); g.fill();
  for (const l of imgs) {
    try {
      const img = await load(borderFile(b, l.file));
      g.globalAlpha = l.opacity ?? 1;
      g.drawImage(img, 0, 0, W, H);
    } catch { /* a missing picture leaves a gap in the icon only */ }
  }
  return c.toDataURL('image/png');
}

/** A stock picture as a data: URL (copying a stock border into the setup puts its pictures into the shared store on Save). */
export async function asDataUrl(url: string): Promise<string> {
  const img = await load(url);
  const c = document.createElement('canvas');
  c.width = img.naturalWidth; c.height = img.naturalHeight;
  c.getContext('2d')!.drawImage(img, 0, 0);
  return c.toDataURL('image/png');
}

/** True when some of the alpha values (RGBA bytes, every 4th) let what's under the picture show through. */
export function anySeeThrough(rgba: ArrayLike<number>): boolean {
  for (let i = 3; i < rgba.length; i += 4) if (rgba[i] < 250) return true;
  return false;
}

// Per picture URL: whether it has see-through parts (pictures don't change under one URL).
const seeThrough = new Map<string, Promise<boolean>>();

/** Whether a border picture lets the card's picture show through anywhere (a JPG or an opaque PNG doesn't). A picture that
 *  can't be read counts as yes: there is nothing to warn about yet. */
export function hasSeeThrough(url: string): Promise<boolean> {
  if (!url) return Promise.resolve(true);
  let p = seeThrough.get(url);
  if (!p) {
    p = load(url).then((img) => {
      const W = 300, H = Math.max(1, Math.round((W * img.naturalHeight) / Math.max(1, img.naturalWidth)));
      const c = document.createElement('canvas');
      c.width = W; c.height = H;
      const g = c.getContext('2d', { willReadFrequently: true })!;
      g.drawImage(img, 0, 0, W, H);
      return anySeeThrough(g.getImageData(0, 0, W, H).data);
    }, () => true);
    seeThrough.set(url, p);
  }
  return p;
}

/** The shader of a card's picture itself (versionShaders), as BorderPreview draws it in the picture's window: its code and
 *  settings, the card's own values and mask picture. */
export type PictureShader = { code: string; params?: ShaderParam[]; own?: Record<string, number[]>; mix?: string; mixStrength?: number;
  draws?: string; mask?: string };
