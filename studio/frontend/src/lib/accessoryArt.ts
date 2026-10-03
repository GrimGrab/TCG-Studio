// Accessory art: an editor "net" (the model unfolded, each face upright as seen from outside) with layers on top, mapped into
// the game texture through the model's face → texture targets (internal/uvmap/models.go).

export interface Target { rect: number[]; src: number[]; flipX?: boolean; flipY?: boolean; transpose?: boolean; bleed?: boolean }
export interface Face { id: string; label: string; net: number[]; hidden?: boolean; targets: Target[] }
export interface Base { id: string; label: string; texture: string; targets?: Record<string, Target[]> }
export interface Model { kind: string; mesh: string; textureSize: number; size: number[]; faces: Face[]; palette?: boolean; icon: string; bases?: Base[] }

export type LayerKind = 'image' | 'fill' | 'text';
export interface Layer {
  id: string;
  kind: LayerKind;
  name: string;
  visible: boolean;
  opacity: number;              // 0..1
  blend: GlobalCompositeOperation;
  // image / text placement in net units (centre, size) and rotation in degrees
  x: number; y: number; w: number; h: number; rot: number;
  src?: string;                 // image: library-relative path
  face?: string;                // fill: face id, '' = every face
  color?: string; color2?: string; // fill: solid or top→bottom gradient; text colour
  text?: string; outline?: string; // text: outline colour ('' = none)
  straighten?: { src: string; pts: number[] }; // image made by Straighten: the original photo and its corners (re-editable)
}
export interface Layout {
  version: 2; layers: Layer[]; base: 'vanilla' | 'color'; baseColor: string;
  baseItem?: string;            // pack/box editor: the model base (vanilla item) the art starts from, or 'file'
  baseFile?: string;            // baseItem 'file': project image in the model's own layout (a snapshot of earlier art)
}

export const newLayout = (): Layout => ({ version: 2, layers: [], base: 'vanilla', baseColor: '#3a3a3a' });

/** Net bounds in object units. */
export function netBounds(m: Model) {
  let x0 = Infinity, y0 = Infinity, x1 = -Infinity, y1 = -Infinity;
  for (const f of m.faces) {
    x0 = Math.min(x0, f.net[0]); y0 = Math.min(y0, f.net[1]);
    x1 = Math.max(x1, f.net[0] + f.net[2]); y1 = Math.max(y1, f.net[1] + f.net[3]);
  }
  return { x0, y0, w: x1 - x0, h: y1 - y0 };
}

/** Pixels per object unit for the net canvas: about the texture's own density of the largest face. */
export function netScale(m: Model): number {
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
 * `imageUrl` turns a layer's library path into a URL.
 */
export async function renderNet(canvas: HTMLCanvasElement, m: Model, layout: Layout, vanilla: HTMLImageElement | null,
  imageUrl: (rel: string) => string, S: number) {
  // Where each face is in the base texture (a model base may read other texture areas, e.g. the Rare box in the box atlas).
  const reverse = m.bases?.find((b) => b.id === layout.baseItem)?.targets;
  const b = netBounds(m);
  canvas.width = Math.round(b.w * S);
  canvas.height = Math.round(b.h * S);
  const ctx = canvas.getContext('2d')!;
  ctx.clearRect(0, 0, canvas.width, canvas.height);
  const k = vanilla ? vanilla.width / m.textureSize : 1;

  for (const f of m.faces) {
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
  for (const f of m.faces) { const r = faceRect(m, f, S); ctx.rect(r.x, r.y, r.w, r.h); }
  ctx.clip();
  for (const l of layout.layers) {
    if (!l.visible) continue;
    ctx.save();
    ctx.globalAlpha = l.opacity;
    ctx.globalCompositeOperation = l.blend || 'source-over';
    if (l.kind === 'fill') {
      for (const f of m.faces) {
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
      if (l.kind === 'image' && l.src) {
        try {
          const img = await loadImage(imageUrl(l.src));
          ctx.drawImage(img, (-l.w / 2) * S, (-l.h / 2) * S, l.w * S, l.h * S);
        } catch { /* missing image: skip */ }
      } else if (l.kind === 'text' && l.text) {
        const px = l.h * S;
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
    ctx.restore();
  }
  ctx.restore();
}

/** Maps the net into the texture (vanilla underneath so unmapped areas — insides, edges, the mat's rubber — stay intact). */
export function composeTexture(m: Model, net: HTMLCanvasElement, vanilla: HTMLImageElement | null, S: number): HTMLCanvasElement {
  const size = vanilla?.width || m.textureSize;
  const k = size / m.textureSize;
  const c = document.createElement('canvas');
  c.width = size; c.height = vanilla?.height || size;
  const ctx = c.getContext('2d')!;
  if (vanilla) ctx.drawImage(vanilla, 0, 0);
  else { ctx.fillStyle = '#3a3a3a'; ctx.fillRect(0, 0, c.width, c.height); }
  ctx.imageSmoothingQuality = 'high';
  for (const f of m.faces) {
    const r = faceRect(m, f, S);
    const targets = [...f.targets].sort((a, b) => Number(!!b.bleed) - Number(!!a.bleed)); // bleed first
    for (const t of targets) {
      blit(ctx, net, r.x + t.src[0] * r.w, r.y + t.src[1] * r.h, (t.src[2] - t.src[0]) * r.w, (t.src[3] - t.src[1]) * r.h,
        t.rect[0] * k, t.rect[1] * k, (t.rect[2] - t.rect[0]) * k, (t.rect[3] - t.rect[1]) * k, t.flipX, t.flipY, t.transpose);
    }
  }
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
