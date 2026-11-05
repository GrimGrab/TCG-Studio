// A customer body's skeleton (characters.json `skeleton`, gameextract/characters.go): bones in Unity's space, local to their
// parent, at rest = the pose the meshes are bound in. A pose replaces bones' local rotations; skinning moves each vertex by
// its bones' "posed × rest⁻¹" matrices. The meshes' upright space (after the extractor's yUp) is the skeleton's space, so no
// bind poses are needed here. Matrices are column-major Float32Array(16), like WebGL.

export interface Bone { name: string; path: string; parent: number; pos: number[]; rot: number[]; scale: number[] }

/** Local rotations (x, y, z, w) by bone name; bones not in it keep their rest rotation. */
export type Pose = Record<string, number[]>;

export type Quat = [number, number, number, number];

export function trs(p: ArrayLike<number>, q: ArrayLike<number>, s: ArrayLike<number>): Float32Array {
  const [x, y, z, w] = [q[0], q[1], q[2], q[3]];
  return new Float32Array([
    (1 - 2 * (y * y + z * z)) * s[0], 2 * (x * y + z * w) * s[0], 2 * (x * z - y * w) * s[0], 0,
    2 * (x * y - z * w) * s[1], (1 - 2 * (x * x + z * z)) * s[1], 2 * (y * z + x * w) * s[1], 0,
    2 * (x * z + y * w) * s[2], 2 * (y * z - x * w) * s[2], (1 - 2 * (x * x + y * y)) * s[2], 0,
    p[0], p[1], p[2], 1,
  ]);
}

export function mul4(a: Float32Array, b: Float32Array): Float32Array {
  const o = new Float32Array(16);
  for (let c = 0; c < 4; c++)
    for (let r = 0; r < 4; r++)
      o[c * 4 + r] = a[r] * b[c * 4] + a[4 + r] * b[c * 4 + 1] + a[8 + r] * b[c * 4 + 2] + a[12 + r] * b[c * 4 + 3];
  return o;
}

/** Inverse of an affine matrix (rotation × scale + translation). */
export function invAffine(m: Float32Array): Float32Array {
  const [a, b, c, d, e, f, g, h, i] = [m[0], m[4], m[8], m[1], m[5], m[9], m[2], m[6], m[10]]; // rows
  const det = a * (e * i - f * h) - b * (d * i - f * g) + c * (d * h - e * g);
  const k = det ? 1 / det : 0;
  const r = [
    (e * i - f * h) * k, (c * h - b * i) * k, (b * f - c * e) * k,
    (f * g - d * i) * k, (a * i - c * g) * k, (c * d - a * f) * k,
    (d * h - e * g) * k, (b * g - a * h) * k, (a * e - b * d) * k,
  ];
  const t = [m[12], m[13], m[14]];
  return new Float32Array([
    r[0], r[3], r[6], 0, r[1], r[4], r[7], 0, r[2], r[5], r[8], 0,
    -(r[0] * t[0] + r[1] * t[1] + r[2] * t[2]), -(r[3] * t[0] + r[4] * t[1] + r[5] * t[2]), -(r[6] * t[0] + r[7] * t[1] + r[8] * t[2]), 1,
  ]);
}

/** World matrices (relative to the body) of every bone, with the pose's rotations. Parents come first in the list. */
export function worldMatrices(bones: Bone[], pose: Pose = {}): Float32Array[] {
  const out: Float32Array[] = [];
  bones.forEach((b, i) => {
    const local = trs(b.pos, pose[b.name] ?? b.rot, b.scale);
    out[i] = b.parent >= 0 ? mul4(out[b.parent], local) : local;
  });
  return out;
}

/** Per bone: posed × rest⁻¹, turned into the preview's GL space (z negated, like the meshes' positions). */
export function skinMatrices(bones: Bone[], pose: Pose): Float32Array[] {
  const rest = worldMatrices(bones);
  const posed = worldMatrices(bones, pose);
  return posed.map((p, i) => {
    const m = mul4(p, invAffine(rest[i]));
    // F·M·F with F = diag(1, 1, -1, 1): the z row and column change sign (zz stays).
    m[2] = -m[2]; m[6] = -m[6]; m[8] = -m[8]; m[9] = -m[9]; m[14] = -m[14];
    return m;
  });
}

/** Linear blend skinning of positions (and normals, renormalised) by 4 bones per vertex. */
export function skin(pos: Float32Array, nrm: Float32Array, joints: Uint16Array, weights: Float32Array, mats: Float32Array[]): { pos: Float32Array; nrm: Float32Array } {
  const n = pos.length / 3;
  const op = new Float32Array(pos.length), on = new Float32Array(nrm.length);
  for (let v = 0; v < n; v++) {
    const x = pos[v * 3], y = pos[v * 3 + 1], z = pos[v * 3 + 2];
    const nx = nrm[v * 3], ny = nrm[v * 3 + 1], nz = nrm[v * 3 + 2];
    let px = 0, py = 0, pz = 0, qx = 0, qy = 0, qz = 0;
    for (let k = 0; k < 4; k++) {
      const w = weights[v * 4 + k];
      if (!w) continue;
      const m = mats[joints[v * 4 + k]];
      if (!m) continue;
      px += w * (m[0] * x + m[4] * y + m[8] * z + m[12]);
      py += w * (m[1] * x + m[5] * y + m[9] * z + m[13]);
      pz += w * (m[2] * x + m[6] * y + m[10] * z + m[14]);
      qx += w * (m[0] * nx + m[4] * ny + m[8] * nz);
      qy += w * (m[1] * nx + m[5] * ny + m[9] * nz);
      qz += w * (m[2] * nx + m[6] * ny + m[10] * nz);
    }
    op[v * 3] = px; op[v * 3 + 1] = py; op[v * 3 + 2] = pz;
    const l = Math.hypot(qx, qy, qz) || 1;
    on[v * 3] = qx / l; on[v * 3 + 1] = qy / l; on[v * 3 + 2] = qz / l;
  }
  return { pos: op, nrm: on };
}

// ---------------------------------------------------------------- quaternions (Unity's x, y, z, w)

export function qmul(a: ArrayLike<number>, b: ArrayLike<number>): Quat {
  return [
    a[3] * b[0] + a[0] * b[3] + a[1] * b[2] - a[2] * b[1],
    a[3] * b[1] - a[0] * b[2] + a[1] * b[3] + a[2] * b[0],
    a[3] * b[2] + a[0] * b[1] - a[1] * b[0] + a[2] * b[3],
    a[3] * b[3] - a[0] * b[0] - a[1] * b[1] - a[2] * b[2],
  ];
}

/** Unity's Quaternion.Euler (degrees; applied z, then x, then y). */
export function euler(x: number, y: number, z: number): Quat {
  const r = Math.PI / 360;
  const [sx, cx, sy, cy, sz, cz] = [Math.sin(x * r), Math.cos(x * r), Math.sin(y * r), Math.cos(y * r), Math.sin(z * r), Math.cos(z * r)];
  const qx: Quat = [sx, 0, 0, cx], qy: Quat = [0, sy, 0, cy], qz: Quat = [0, 0, sz, cz];
  return qmul(qy, qmul(qx, qz));
}

export function qnorm(q: ArrayLike<number>): Quat {
  const l = Math.hypot(q[0], q[1], q[2], q[3]) || 1;
  return [q[0] / l, q[1] / l, q[2] / l, q[3] / l];
}

export const qconj = (q: ArrayLike<number>): Quat => [-q[0], -q[1], -q[2], q[3]];

export function slerp(a: ArrayLike<number>, b: ArrayLike<number>, t: number): Quat {
  let [bx, by, bz, bw] = [b[0], b[1], b[2], b[3]];
  let d = a[0] * bx + a[1] * by + a[2] * bz + a[3] * bw;
  if (d < 0) { d = -d; bx = -bx; by = -by; bz = -bz; bw = -bw; } // the short way round
  if (d > 0.9995) return qnorm([a[0] + (bx - a[0]) * t, a[1] + (by - a[1]) * t, a[2] + (bz - a[2]) * t, a[3] + (bw - a[3]) * t]);
  const th = Math.acos(d), s = Math.sin(th), ka = Math.sin((1 - t) * th) / s, kb = Math.sin(t * th) / s;
  return [a[0] * ka + bx * kb, a[1] * ka + by * kb, a[2] * ka + bz * kb, a[3] * ka + bw * kb];
}

/** A rotation of `deg` degrees about a unit axis. */
export function axisAngle(axis: ArrayLike<number>, deg: number): Quat {
  const h = (deg * Math.PI) / 360, s = Math.sin(h);
  return [axis[0] * s, axis[1] * s, axis[2] * s, Math.cos(h)];
}

/** Unity's eulerAngles of a rotation (degrees, −180…180; the inverse of `euler`). */
export function toEuler(q: ArrayLike<number>): [number, number, number] {
  const [x, y, z, w] = [q[0], q[1], q[2], q[3]];
  const d = (r: number) => (r * 180) / Math.PI;
  const sx = Math.max(-1, Math.min(1, 2 * (w * x - y * z)));
  return [d(Math.asin(sx)), d(Math.atan2(2 * (x * z + w * y), 1 - 2 * (x * x + y * y))), d(Math.atan2(2 * (x * y + w * z), 1 - 2 * (x * x + z * z)))];
}

/** The rotation part of a matrix (scale removed) as a quaternion. */
export function matQuat(m: Float32Array): Quat {
  const sx = Math.hypot(m[0], m[1], m[2]) || 1, sy = Math.hypot(m[4], m[5], m[6]) || 1, sz = Math.hypot(m[8], m[9], m[10]) || 1;
  const [a, b, c, d, e, f, g, h, i] = [m[0] / sx, m[4] / sy, m[8] / sz, m[1] / sx, m[5] / sy, m[9] / sz, m[2] / sx, m[6] / sy, m[10] / sz]; // rows
  const tr = a + e + i;
  let q: Quat;
  if (tr > 0) { const s = Math.sqrt(tr + 1) * 2; q = [(h - f) / s, (c - g) / s, (d - b) / s, s / 4]; }
  else if (a > e && a > i) { const s = Math.sqrt(1 + a - e - i) * 2; q = [s / 4, (b + d) / s, (c + g) / s, (h - f) / s]; }
  else if (e > i) { const s = Math.sqrt(1 + e - a - i) * 2; q = [(b + d) / s, s / 4, (f + h) / s, (c - g) / s]; }
  else { const s = Math.sqrt(1 + i - a - e) * 2; q = [(c + g) / s, (f + h) / s, s / 4, (d - b) / s]; }
  return qnorm(q);
}

/**
 * A bone's new local rotation after turning it `deg` degrees about an axis in the body's space (Unity), the way a drag in
 * the view turns it: local' = parentWorld⁻¹ · R · parentWorld · local.
 */
export function turnBone(bones: Bone[], pose: Pose, index: number, axis: ArrayLike<number>, deg: number): Quat {
  const b = bones[index];
  const local = pose[b.name] ?? b.rot;
  const parent = b.parent >= 0 ? matQuat(worldMatrices(bones, pose)[b.parent]) : ([0, 0, 0, 1] as Quat);
  return qnorm(qmul(qconj(parent), qmul(axisAngle(axis, deg), qmul(parent, local))));
}

// ---------------------------------------------------------------- animations (setfmt.Animation)

export interface AnimKey { t: number; r: number[] }

/** A bone's rotation at time t: held before the first key and after the last, eased (smooth in and out) between keys. */
export function sampleKeys(keys: AnimKey[], t: number): Quat | null {
  if (!keys.length) return null;
  if (t <= keys[0].t) return qnorm(keys[0].r);
  for (let i = 1; i < keys.length; i++) {
    if (t <= keys[i].t) {
      const a = keys[i - 1], b = keys[i];
      const u = (t - a.t) / Math.max(1e-6, b.t - a.t);
      return slerp(a.r, b.r, u * u * (3 - 2 * u));
    }
  }
  return qnorm(keys[keys.length - 1].r);
}

/** The pose of an animation at time t. */
export function samplePose(bones: Record<string, AnimKey[]>, t: number): Pose {
  const out: Pose = {};
  for (const [name, keys] of Object.entries(bones)) {
    const q = sampleKeys(keys, t);
    if (q) out[name] = q;
  }
  return out;
}

/** Bones each mask moves (by name); `full` = all. Arms = both clavicles and below; upper = spine_01 and everything above. */
/**
 * Whether a bone can be animated: animations name their bones, so a name used more than once can't be. Those are helpers the prefab
 * repeats under each hand (HandPivotGrp, Ctrl, Pos1...), never body parts.
 */
export function animatableBones(bones: Bone[]): (b: Bone) => boolean {
  const count = new Map<string, number>();
  for (const b of bones) count.set(b.name, (count.get(b.name) ?? 0) + 1);
  return (b) => count.get(b.name) === 1;
}

export function maskedBones(bones: Bone[], mask: string): Set<string> {
  const roots = mask === 'arms' ? ['clavicle_l', 'clavicle_r'] : mask === 'upper' ? ['spine_01'] : null;
  if (!roots) return new Set(bones.map((b) => b.name));
  const on = new Set<string>();
  bones.forEach((b) => { if (roots.includes(b.name) || (b.parent >= 0 && on.has(bones[b.parent].name))) on.add(b.name); });
  return on;
}

// ---------------------------------------------------------------- natural posing

const sub = (a: number[], b: number[]) => [a[0] - b[0], a[1] - b[1], a[2] - b[2]];
const len = (a: number[]) => Math.hypot(a[0], a[1], a[2]);
const dot = (a: number[], b: number[]) => a[0] * b[0] + a[1] * b[1] + a[2] * b[2];
const cross = (a: number[], b: number[]) => [a[1] * b[2] - a[2] * b[1], a[2] * b[0] - a[0] * b[2], a[0] * b[1] - a[1] * b[0]];
const unit = (a: number[]) => { const l = len(a) || 1; return [a[0] / l, a[1] / l, a[2] / l]; };
const posOf = (m: Float32Array) => [m[12], m[13], m[14]];

/**
 * Two-bone IK (arm: upperarm → lowerarm → hand; leg: thigh → calf → foot): the new local rotations of the two upper bones
 * that put the end bone's joint at `target` (body space), keeping the elbow / knee bending the way it already does
 * (`hint` = a fallback bend direction when the limb is straight, e.g. the camera's forward). Out-of-reach targets are reached
 * towards. Pure: returns the two bones' rotations.
 */
export function twoBoneIK(bones: Bone[], pose: Pose, upper: number, lower: number, end: number, target: number[], hint: number[]): Pose {
  let W = worldMatrices(bones, pose);
  const A = posOf(W[upper]), Bp = posOf(W[lower]), C = posOf(W[end]);
  const l1 = len(sub(Bp, A)), l2 = len(sub(C, Bp));
  const d = Math.min(Math.max(len(sub(target, A)), Math.abs(l1 - l2) + 1e-4), l1 + l2 - 1e-4);
  // 1. Bend the middle joint to the angle that makes the limb as long as the distance to the target.
  let axis = cross(sub(A, Bp), sub(C, Bp));
  if (len(axis) < 1e-6) axis = cross(sub(A, Bp), hint);
  axis = unit(axis);
  const cur = Math.acos(Math.max(-1, Math.min(1, dot(unit(sub(A, Bp)), unit(sub(C, Bp))))));
  const want = Math.acos(Math.max(-1, Math.min(1, (l1 * l1 + l2 * l2 - d * d) / (2 * l1 * l2))));
  let out: Pose = { ...pose };
  const bendBy = (deg: number) => ({ ...out, [bones[lower].name]: turnBone(bones, out, lower, axis, deg) });
  const angleAt = (p: Pose) => {
    const w = worldMatrices(bones, p);
    const b = posOf(w[lower]);
    return Math.acos(Math.max(-1, Math.min(1, dot(unit(sub(posOf(w[upper]), b)), unit(sub(posOf(w[end]), b))))));
  };
  const deg = ((want - cur) * 180) / Math.PI;
  const plus = bendBy(deg), minus = bendBy(-deg);
  out = Math.abs(angleAt(plus) - want) <= Math.abs(angleAt(minus) - want) ? plus : minus;
  // 2. Swing the upper bone so the end points at the target.
  W = worldMatrices(bones, out);
  const C2 = posOf(W[end]);
  const u = sub(C2, A), v = sub(target, A);
  const sw = cross(u, v);
  if (len(sw) > 1e-9) {
    const ang = (Math.acos(Math.max(-1, Math.min(1, dot(unit(u), unit(v))))) * 180) / Math.PI;
    out = { ...out, [bones[upper].name]: turnBone(bones, out, upper, unit(sw), ang) };
  }
  return { [bones[upper].name]: out[bones[upper].name], [bones[lower].name]: out[bones[lower].name] };
}

/**
 * The turn that swings a bone at `joint`, pointing at `tip`, by what moving the tip by `move` asks for: { axis, deg } in body space, null for none. Short bones
 * count as `minLever` long, so that a mouse drag doesn't spin them.
 */
export function swingBy(joint: number[], tip: number[], move: number[], minLever: number): { axis: number[]; deg: number } | null {
  const from = sub(tip, joint);
  const length = len(from);
  if (length < 1e-6) return null;
  const lever = from.map((v) => (v / length) * Math.max(length, minLever));
  const to = lever.map((v, i) => v + move[i]);
  const axis = cross(lever, to);
  if (len(axis) < 1e-9) return null;
  const cos = Math.max(-1, Math.min(1, dot(unit(lever), unit(to))));
  return { axis: unit(axis), deg: (Math.acos(cos) * 180) / Math.PI };
}

/** The bone chains that move together while posing naturally, and how a picked bone drives them. */
export type Chain =
  | { kind: 'ik'; upper: string; lower: string; end: string }   // drag the end (or the middle): the limb reaches
  | { kind: 'spread'; bones: string[] }                          // a turn shared between these
  | { kind: 'curl'; bones: string[] };                           // the turn repeated down these (fingers)

export function chainFor(name: string): Chain | null {
  const side = /_(l|r)$/.exec(name)?.[1];
  if (side) {
    if (name === `hand_${side}` || name === `lowerarm_${side}`) return { kind: 'ik', upper: `upperarm_${side}`, lower: `lowerarm_${side}`, end: `hand_${side}` };
    if (name === `foot_${side}` || name === `calf_${side}` || name === `ball_${side}`) return { kind: 'ik', upper: `thigh_${side}`, lower: `calf_${side}`, end: `foot_${side}` };
    const f = /^(thumb|index|middle|ring|pinky)_0([123])_/.exec(name);
    if (f) return { kind: 'curl', bones: [1, 2, 3].filter((k) => k >= +f[2]).map((k) => `${f[1]}_0${k}_${side}`) };
  }
  if (/^spine_0[123]$/.test(name)) return { kind: 'spread', bones: ['spine_01', 'spine_02', 'spine_03'] };
  if (name === 'neck_01' || name === 'head') return { kind: 'spread', bones: ['neck_01', 'head'] };
  return null;
}
