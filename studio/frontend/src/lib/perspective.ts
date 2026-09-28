// Perspective straightening: four corners of a rectangle photographed at an angle → a flat, head-on image.
// Corners are [tl, tr, br, bl] in source pixels.

export type Pt = [number, number];

/** Homography H (3×3, row-major, h33 = 1) mapping the four `from` points onto the four `to` points. */
export function homography(from: Pt[], to: Pt[]): number[] {
  // Solve the 8×8 system A·h = b.
  const A: number[][] = [], b: number[] = [];
  for (let i = 0; i < 4; i++) {
    const [x, y] = from[i], [u, v] = to[i];
    A.push([x, y, 1, 0, 0, 0, -u * x, -u * y]); b.push(u);
    A.push([0, 0, 0, x, y, 1, -v * x, -v * y]); b.push(v);
  }
  for (let c = 0; c < 8; c++) {
    let p = c;
    for (let r = c + 1; r < 8; r++) if (Math.abs(A[r][c]) > Math.abs(A[p][c])) p = r;
    [A[c], A[p]] = [A[p], A[c]]; [b[c], b[p]] = [b[p], b[c]];
    const d = A[c][c];
    if (Math.abs(d) < 1e-12) throw new Error('corners are degenerate (three in a line?)');
    for (let r = 0; r < 8; r++) {
      if (r === c) continue;
      const f = A[r][c] / d;
      if (!f) continue;
      for (let k = c; k < 8; k++) A[r][k] -= f * A[c][k];
      b[r] -= f * b[c];
    }
  }
  return [...b.map((v, i) => v / A[i][i]), 1];
}

const dist = (a: Pt, b: Pt) => Math.hypot(a[0] - b[0], a[1] - b[1]);

/**
 * Width / height of the real rectangle. Uses the camera model of Zhang & He ("Whiteboard scanning and image enhancement",
 * 2007) with the principal point at the image centre; falls back to average edge lengths when the focal length can't be
 * recovered (nearly parallel edges or noisy corners).
 */
export function estimateAspect(q: Pt[], imgW: number, imgH: number): number {
  const [tl, tr, br, bl] = q;
  const naive = ((dist(tl, tr) + dist(bl, br)) / 2) / Math.max(1e-6, (dist(tl, bl) + dist(tr, br)) / 2);
  const u0 = imgW / 2, v0 = imgH / 2;
  const m1 = [tl[0], tl[1], 1], m2 = [tr[0], tr[1], 1], m3 = [bl[0], bl[1], 1], m4 = [br[0], br[1], 1];
  const cross = (a: number[], b: number[]) => [a[1] * b[2] - a[2] * b[1], a[2] * b[0] - a[0] * b[2], a[0] * b[1] - a[1] * b[0]];
  const dot = (a: number[], b: number[]) => a[0] * b[0] + a[1] * b[1] + a[2] * b[2];
  const k2 = dot(cross(m1, m4), m3) / dot(cross(m2, m4), m3);
  const k3 = dot(cross(m1, m4), m2) / dot(cross(m3, m4), m2);
  if (!isFinite(k2) || !isFinite(k3)) return naive;
  const n2 = m2.map((v, i) => k2 * v - m1[i]);
  const n3 = m3.map((v, i) => k3 * v - m1[i]);
  const den = n2[2] * n3[2];
  let r: number;
  if (Math.abs(den) < 1e-9) {
    // Affine case (no perspective): the edge vectors themselves.
    r = Math.sqrt((n2[0] ** 2 + n2[1] ** 2) / (n3[0] ** 2 + n3[1] ** 2));
  } else {
    const f2 = -((n2[0] * n3[0] - (n2[0] * n3[2] + n2[2] * n3[0]) * u0 + n2[2] * n3[2] * u0 * u0)
      + (n2[1] * n3[1] - (n2[1] * n3[2] + n2[2] * n3[1]) * v0 + n2[2] * n3[2] * v0 * v0)) / den;
    if (!(f2 > 0)) return naive;
    const f = Math.sqrt(f2);
    const len2 = (n: number[]) => ((n[0] - u0 * n[2]) / f) ** 2 + ((n[1] - v0 * n[2]) / f) ** 2 + n[2] ** 2;
    r = Math.sqrt(len2(n2) / len2(n3));
  }
  // Trust the model only within reason of the naive estimate (bad corners make it blow up).
  return isFinite(r) && r > naive / 3 && r < naive * 3 ? r : naive;
}

/** Output size for the rectified image: about the source resolution along the longer edges, capped. */
export function outputSize(q: Pt[], aspect: number, cap = 2048): [number, number] {
  const [tl, tr, br, bl] = q;
  const w = Math.max(dist(tl, tr), dist(bl, br)), h = Math.max(dist(tl, bl), dist(tr, br));
  let W = Math.max(w, h * aspect), H = W / aspect;
  const k = Math.min(1, cap / Math.max(W, H));
  W *= k; H *= k;
  return [Math.max(2, Math.round(W)), Math.max(2, Math.round(H))];
}

/** Warps the quad of `img` into a W×H canvas (inverse mapping, bilinear sampling). */
export function rectify(img: CanvasImageSource & { width: number; height: number }, q: Pt[], W: number, H: number): HTMLCanvasElement {
  const sw = img.width, sh = img.height;
  const src = document.createElement('canvas');
  src.width = sw; src.height = sh;
  const sctx = src.getContext('2d', { willReadFrequently: true })!;
  sctx.drawImage(img, 0, 0);
  const s = sctx.getImageData(0, 0, sw, sh).data;
  const Hm = homography([[0, 0], [W, 0], [W, H], [0, H]], q); // output → source
  const out = document.createElement('canvas');
  out.width = W; out.height = H;
  const octx = out.getContext('2d')!;
  const o = octx.createImageData(W, H);
  const d = o.data;
  for (let y = 0; y < H; y++) {
    const py = y + 0.5;
    for (let x = 0; x < W; x++) {
      const px = x + 0.5;
      const w = Hm[6] * px + Hm[7] * py + 1;
      const u = (Hm[0] * px + Hm[1] * py + Hm[2]) / w - 0.5;
      const v = (Hm[3] * px + Hm[4] * py + Hm[5]) / w - 0.5;
      const i = (y * W + x) * 4;
      if (u < -0.5 || v < -0.5 || u > sw - 0.5 || v > sh - 0.5) continue; // outside: transparent
      const x0 = Math.max(0, Math.min(sw - 1, Math.floor(u))), y0 = Math.max(0, Math.min(sh - 1, Math.floor(v)));
      const x1 = Math.min(sw - 1, x0 + 1), y1 = Math.min(sh - 1, y0 + 1);
      const fx = Math.max(0, Math.min(1, u - x0)), fy = Math.max(0, Math.min(1, v - y0));
      const a = (y0 * sw + x0) * 4, b = (y0 * sw + x1) * 4, c = (y1 * sw + x0) * 4, e = (y1 * sw + x1) * 4;
      for (let k = 0; k < 4; k++) {
        const top = s[a + k] + (s[b + k] - s[a + k]) * fx;
        const bot = s[c + k] + (s[e + k] - s[c + k]) * fx;
        d[i + k] = top + (bot - top) * fy;
      }
    }
  }
  octx.putImageData(o, 0, 0);
  return out;
}
