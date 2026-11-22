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
  /** Where the card's picture goes: x, y, width, height as fractions of the border from its top left; none = the whole card.
   *  Never set by hand: measured from the first picture layer's see-through middle (measureWindow). */
  window?: number[];
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

/** The window as [x, y, w, h] fractions, clamped; the whole card when the border has none. */
export function windowOf(b: Border | null | undefined): [number, number, number, number] {
  const w = b?.window;
  if (!w || w.length !== 4) return [0, 0, 1, 1];
  const c = (v: number) => Math.min(1, Math.max(0, v || 0));
  return [c(w[0]), c(w[1]), Math.max(0.02, c(w[2])), Math.max(0.02, c(w[3]))];
}

/** The thin rim template's window: as thin as the game's own rim around a full-image picture (3.5 % each side; the stock
 *  borders' window, cmd/stockborders). */
export const THIN_RIM_WINDOW: [number, number, number, number] = [0.0351, 0.0357, 0.9298, 0.9286];

/**
 * A picture to paint a border on: the card's shape at CARD_SIZE (rounded corners, grey) with the window cut out see-through,
 * nothing else on it (no guide lines or notes: the picture can be used as it is). kind 'panel' = a framed card (the
 * border's window, else an art box over a text panel); 'thin' = a thin rim round the whole picture (THIN_RIM_WINDOW).
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
  g.fillRect(x * W, y * H, w * W, h * H);
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

/**
 * The border's window: the see-through middle of its first picture layer, as [x, y, w, h] fractions, or undefined when the
 * card picture should cover the whole card (no picture, its middle isn't see-through, or it is see-through to the edges).
 * Measured along three rows and three columns through the middle; the tightest of each side wins, so the rim never covers
 * the picture.
 */
export async function measureWindow(b: Border): Promise<number[] | undefined> {
  const first = b.layers.find((l) => l.kind === 'image' && l.file);
  if (!first) return undefined;
  let img: HTMLImageElement;
  try { img = await load(borderFile(b, first.file)); } catch { return b.window; } // can't read it: keep what it had
  const W = 300, H = Math.max(1, Math.round((W * img.naturalHeight) / img.naturalWidth));
  const c = document.createElement('canvas');
  c.width = W; c.height = H;
  const g = c.getContext('2d', { willReadFrequently: true })!;
  g.drawImage(img, 0, 0, W, H);
  const data = g.getImageData(0, 0, W, H).data;
  const clear = (x: number, y: number) => data[(y * W + x) * 4 + 3] < 128;
  if (!clear(W >> 1, H >> 1)) return undefined;
  let left = 0, right = W - 1, top = 0, bottom = H - 1;
  for (const f of [0.3, 0.5, 0.7]) {
    const y = Math.round(H * f), x = Math.round(W * f);
    if (clear(W >> 1, y)) {
      let l = W >> 1, r = W >> 1;
      while (l > 0 && clear(l - 1, y)) l--;
      while (r < W - 1 && clear(r + 1, y)) r++;
      left = Math.max(left, l); right = Math.min(right, r);
    }
    if (clear(x, H >> 1)) {
      let t = H >> 1, d = H >> 1;
      while (t > 0 && clear(x, t - 1)) t--;
      while (d < H - 1 && clear(x, d + 1)) d++;
      top = Math.max(top, t); bottom = Math.min(bottom, d);
    }
  }
  if (left === 0 && top === 0 && right === W - 1 && bottom === H - 1) return undefined;
  const r4 = (v: number) => Math.round(v * 10000) / 10000;
  return [r4(left / W), r4(top / H), r4((right + 1 - left) / W), r4((bottom + 1 - top) / H)];
}

/** The shader of a card's picture itself (versionShaders), as BorderPreview draws it in the picture's window: its code and
 *  settings, the card's own values and mask picture. */
export type PictureShader = { code: string; params?: ShaderParam[]; own?: Record<string, number[]>; mix?: string; mixStrength?: number;
  draws?: string; mask?: string };
