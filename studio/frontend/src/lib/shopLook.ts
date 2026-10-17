// Shared by the crop boxes (views/PictureSlot.svelte) and the Game look page's previews.

/** A picture cut with the crop box: its file (as the page's backend names it, '' = none), a URL to show it and the crop
 *  ([x, y, w, h] fractions from its top-left, null = the middle). */
export type Picture = { file: string; url: string; crop: number[] | null };

/** The part of an image the game shows, [x, y, w, h] as fractions from its top-left corner: the crop setting (its height
 *  re-derived so the shape is exact) or, without one, the largest `aspect` box in the middle. Same rule as the mod's
 *  ShopPicture.Crop. */
export function cropFor(iw: number, ih: number, crop: number[] | null | undefined, aspect: number): [number, number, number, number] {
  const hFor = (w: number) => (w * iw) / (ih * aspect);
  if (crop && crop.length === 4) return [crop[0], crop[1], crop[2], hFor(crop[2])];
  const w = iw / ih > aspect ? aspect / (iw / ih) : 1, h = hFor(w);
  return [(1 - w) / 2, (1 - h) / 2, w, h];
}

/** Draws the shown part of `img` (see cropFor) stretched into the destination rectangle. */
export function drawCropped(g: CanvasRenderingContext2D, img: HTMLImageElement, crop: number[] | null | undefined, aspect: number,
  dx: number, dy: number, dw: number, dh: number) {
  const [x, y, w, h] = cropFor(img.width, img.height, crop, aspect);
  g.drawImage(img, x * img.width, y * img.height, w * img.width, h * img.height, dx, dy, dw, dh);
}

const families = new Map<string, Promise<string>>();
let familyCount = 0;
/** Loads a font file for canvas drawing (once per URL) and resolves to its CSS family name. */
export function fontFamily(url: string): Promise<string> {
  if (!families.has(url)) {
    const fam = `tcgfont${++familyCount}`;
    families.set(url, new FontFace(fam, `url("${url}")`).load().then((f) => { document.fonts.add(f); return fam; })
      .catch((e) => { families.delete(url); throw e; }));
  }
  return families.get(url)!;
}

/** Text style of the shop name and the door sign words (views/TextStyle.svelte). color '' = the default colour; size 1 = the
 *  default size; outline = fraction of the font size. */
export type TextStyleValue = { color: string; size: number; outline: number; outlineColor: string };

/** Fills text with the current fillStyle, then its outline: `outline` × the font size, centred on the letter edge (half inside,
 *  half outside, like TMP's outline). Same for the shop name preview and the door sign. */
export function fillOutlined(g: CanvasRenderingContext2D, text: string, x: number, y: number, px: number, outline: number,
  outlineColor: string) {
  g.fillText(text, x, y);
  if (outline <= 0) return;
  g.lineJoin = 'round'; g.lineWidth = outline * px; g.strokeStyle = outlineColor;
  g.strokeText(text, x, y);
}

/** A door sign side designed in the page (stored next to its picture by App.SaveShopLook); color '' = white. */
export type SignTextDesign = TextStyleValue & {
  text: string; boardColor: string;
  font: { file: string; name: string } | null;
  background: { file: string; crop: number[] | null } | null; // a picture instead of the board colour
};

const doorStyle = { color: '', size: 1, outline: 0, outlineColor: '#000000' };
/** The vanilla sides' words and colours (T_OpenCloseSign). */
export const doorSignDefaults: Record<string, SignTextDesign> = {
  open: { ...doorStyle, text: "YES, WE'RE\nOPEN", boardColor: '#10a204', font: null, background: null },
  closed: { ...doorStyle, text: "SORRY! WE'RE\nCLOSED", boardColor: '#ce2829', font: null, background: null },
};

/** Draws the design on a W×H board: the background (`bg` = its loaded picture, cropped to `aspect`, or the board colour), then
 *  each line as wide as fits (short words come out big, like the vanilla OPEN), all lines shrunk together when they'd be too
 *  tall. Size scales the lines (still never wider or taller than the board). `family` = loaded font, '' = the default
 *  condensed font. */
export function drawSignText(g: CanvasRenderingContext2D, W: number, H: number, d: SignTextDesign, family: string,
  bg: HTMLImageElement | null, aspect: number) {
  if (bg && d.background) drawCropped(g, bg, d.background.crop, aspect, 0, 0, W, H);
  else { g.fillStyle = d.boardColor; g.fillRect(0, 0, W, H); }
  const lines = d.text.split('\n').map((l) => l.trim()).filter(Boolean);
  if (!lines.length) return;
  const font = (px: number) => `bold ${px}px ${family ? `"${family}"` : 'Impact, "Arial Narrow", sans-serif'}`;
  const lineGap = 1.12, availH = Math.min(H * 0.98, H * 0.8 * d.size);
  const sizes = lines.map((l) => {
    g.font = font(100);
    const fit = 100 / Math.max(1, g.measureText(l).width); // font px per canvas px of line width
    return Math.min(Math.min(fit * W * 0.86, H * 0.8 * 0.62) * d.size, fit * W * 0.98);
  });
  const total = sizes.reduce((s, px) => s + px * lineGap, 0);
  const k = Math.min(1, availH / total);
  g.fillStyle = d.color || '#ffffff'; g.textAlign = 'center'; g.textBaseline = 'middle';
  let y = (H - total * k) / 2;
  lines.forEach((l, i) => {
    const px = sizes[i] * k;
    g.font = font(px);
    fillOutlined(g, l, W / 2, y + (px * lineGap) / 2, px, d.outline, d.outlineColor);
    y += px * lineGap;
  });
}

/** Live previews redraw on every input event (a dragged colour picker or slider fires many per frame): the returned scheduler
 *  runs only the latest call, once on the next animation frame. Read the reactive values when calling it (e.g. with
 *  `draw.bind(null, …)`) so a Svelte $effect still tracks them. */
export function latestPerFrame() {
  let id = 0;
  return (f: () => void) => { cancelAnimationFrame(id); id = requestAnimationFrame(f); };
}
