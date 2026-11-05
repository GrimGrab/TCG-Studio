// Names for the character asset's numbered face shapes (shp_face_01…09, shp_eyes_…, …): the asset only numbers them, so
// each is named from what it measurably does to the mesh — moves the area up/down or forward/back, makes it wider,
// taller or more prominent — and near-duplicates (the same motion, cosine > 0.9) point at the first of them.
import type { CharMesh } from './characterView';

export interface ShapeInfo {
  key: string;
  label: string;
  /** Another shape of the group that does (almost) the same: the editor shows that one instead. */
  same?: string;
}

const DUPLICATE = 0.9;
const MIN_SHARE = 0.15; // a motion must carry this share of the shape's movement to be named

type Motion = { word: [string, string]; value: number };

/** Describes the 01…09 shapes of `prefix` (e.g. "shp_brow") in `m`; shapes the mesh lacks are left out. */
export function describeShapes(m: CharMesh, prefix: string, area: string): ShapeInfo[] {
  const keys = Array.from({ length: 9 }, (_, i) => `${prefix}_0${i + 1}`).filter((k) => m.shapes.has(k));
  const out: ShapeInfo[] = [];
  const used = new Map<string, number>();
  for (let a = 0; a < keys.length; a++) {
    const key = keys[a];
    const same = keys.slice(0, a).find((k) => cosine(m, k, key) > DUPLICATE);
    let label = `${area}: ${name(m, key)}`;
    const n = (used.get(label) ?? 0) + 1;
    used.set(label, n);
    if (n > 1) label += ` (${n})`;
    out.push({ key, label, same });
  }
  return out;
}

function name(m: CharMesh, key: string): string {
  const s = m.shapes.get(key)!;
  const P = m.pos, D = s.d;
  // Centre of the moving part (where the shape acts), then signed motions relative to it.
  let cx = 0, cy = 0, cz = 0, w = 0;
  for (let i = 0; i < s.idx.length; i++) {
    const mag = Math.abs(D[i * 3]) + Math.abs(D[i * 3 + 1]) + Math.abs(D[i * 3 + 2]);
    const v = s.idx[i] * 3;
    cx += P[v] * mag; cy += P[v + 1] * mag; cz += P[v + 2] * mag; w += mag;
  }
  if (!w) return 'no change';
  cx /= w; cy /= w; cz /= w;
  // GL space: x = side, y = up, the face looks along −z (pos and deltas both had z negated on load).
  let up = 0, fwd = 0, wide = 0, tall = 0, deep = 0;
  for (let i = 0; i < s.idx.length; i++) {
    const v = s.idx[i] * 3;
    const dx = D[i * 3], dy = D[i * 3 + 1], dz = -D[i * 3 + 2];
    up += dy;
    fwd += dz;
    wide += dx * Math.sign(P[v] - cx);
    tall += dy * Math.sign(P[v + 1] - cy);
    deep += dz * Math.sign(cz - P[v + 2]);
  }
  const motions: Motion[] = [
    { word: ['up', 'down'], value: up },
    { word: ['forward', 'back'], value: fwd },
    { word: ['wider', 'narrower'], value: wide },
    { word: ['taller', 'shorter'], value: tall },
    { word: ['more prominent', 'flatter'], value: deep },
  ];
  const named = motions
    .map((mo) => ({ ...mo, share: Math.abs(mo.value) / w }))
    .filter((mo) => mo.share >= MIN_SHARE)
    .sort((a, b) => b.share - a.share)
    .slice(0, 2)
    .map((mo) => (mo.value > 0 ? mo.word[0] : mo.word[1]));
  if (!named.length) return 'subtle reshape';
  // Deltas are in 0.1 mm at full strength: the barely visible ones say so.
  let max = 0;
  for (let i = 0; i < s.idx.length; i++) max = Math.max(max, Math.hypot(D[i * 3], D[i * 3 + 1], D[i * 3 + 2]));
  return (max < SLIGHT_MM * 10 ? 'slightly ' : '') + named.join(', ');
}

const SLIGHT_MM = 2.5;

function cosine(m: CharMesh, a: string, b: string): number {
  const A = m.shapes.get(a)!, B = m.shapes.get(b)!;
  const at = new Map<number, number>();
  for (let i = 0; i < A.idx.length; i++) at.set(A.idx[i], i);
  let dot = 0, na = 0, nb = 0;
  for (let i = 0; i < A.d.length; i++) na += A.d[i] * A.d[i];
  for (let i = 0; i < B.idx.length; i++) {
    const x = B.d[i * 3], y = B.d[i * 3 + 1], z = B.d[i * 3 + 2];
    nb += x * x + y * y + z * z;
    const j = at.get(B.idx[i]);
    if (j !== undefined) dot += x * A.d[j * 3] + y * A.d[j * 3 + 1] + z * A.d[j * 3 + 2];
  }
  return na && nb ? dot / Math.sqrt(na * nb) : 0;
}
