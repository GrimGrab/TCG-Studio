// The face editor's 3D preview of a customer character (Customers page). Meshes come from the game templates
// (characters\*.bin, gameextract/characters.go): the body (skin, head, eyes, lashes) with the face sliders as morphs, the
// hair / beard pieces, and the clothes, which follow the sliders as the game's do. Drawn in one shader with a mode per kind of surface,
// an approximation of the game's skin, eye and hair shaders built from the same textures and settings.
import { compileProgram, uploadTexture, mul, type Mat } from './meshView';
import { skin, skinMatrices, type Bone, type Pose } from './skeleton';

// ---------------------------------------------------------------- mesh format (writeCharMesh)

export interface CharMesh {
  n: number;
  pos: Float32Array; // GL space (z negated)
  nrm: Float32Array;
  uv: Float32Array;
  uv2: Float32Array | null;
  subs: Uint32Array[];
  shapes: Map<string, { idx: Uint32Array; d: Int16Array }>;
  joints: Uint16Array | null;  // 4 per vertex: indices into the body's skeleton
  weights: Float32Array | null;
}

const meshCache = new Map<string, Promise<CharMesh>>();

export function loadCharMesh(url: string): Promise<CharMesh> {
  if (!meshCache.has(url)) meshCache.set(url, fetch(url).then(async (r) => {
    if (!r.ok) throw new Error(`character model not found: ${url}`);
    return parseCharMesh(await r.arrayBuffer());
  }));
  return meshCache.get(url)!;
}

function parseCharMesh(buf: ArrayBuffer): CharMesh {
  const dv = new DataView(buf);
  let o = 4;
  const u = () => { const v = dv.getUint32(o, true); o += 4; return v; };
  if (new TextDecoder().decode(new Uint8Array(buf, 0, 4)) !== 'TCGM') throw new Error('not a character model');
  const version = u();
  if (version !== 2 && version !== 3) throw new Error(`character model version ${version} isn't supported`);
  const n = u(), ns = u();
  const counts: number[] = [];
  for (let i = 0; i < ns; i++) counts.push(u());
  const nsh = u(), flags = u();
  const floats = (k: number) => { const a = new Float32Array(buf.slice(o, o + k * 4)); o += k * 4; return a; };
  const pos = floats(3 * n), nrm = floats(3 * n), uv = floats(2 * n);
  const uv2 = flags & 1 ? floats(2 * n) : null;
  let joints: Uint16Array | null = null, weights: Float32Array | null = null;
  if (flags & 2) {
    joints = new Uint16Array(buf.slice(o, o + n * 8)); o += n * 8;
    weights = floats(4 * n);
  }
  for (let i = 0; i < n; i++) { pos[i * 3 + 2] = -pos[i * 3 + 2]; nrm[i * 3 + 2] = -nrm[i * 3 + 2]; } // Unity → GL
  const subs = counts.map((c) => { const a = new Uint32Array(buf.slice(o, o + c * 4)); o += c * 4; return a; });
  const shapes = new Map<string, { idx: Uint32Array; d: Int16Array }>();
  for (let k = 0; k < nsh; k++) {
    const len = u();
    const name = new TextDecoder().decode(new Uint8Array(buf, o, len));
    o += len + ((4 - (len % 4)) % 4);
    const c = u();
    const idx = new Uint32Array(buf.slice(o, o + c * 4)); o += c * 4;
    const d = new Int16Array(buf.slice(o, o + c * 6)); o += c * 6;
    o += (4 - (o % 4)) % 4;
    for (let i = 0; i < c; i++) d[i * 3 + 2] = -d[i * 3 + 2];
    shapes.set(name, { idx, d });
  }
  return { n, pos, nrm, uv, uv2, subs, shapes, joints, weights };
}

/** Positions with the face sliders applied (value × the full shape, as the game's weight = value × 100). */
export function morphed(m: CharMesh, values: Record<string, number>): Float32Array {
  const out = new Float32Array(m.pos);
  for (const [name, v] of Object.entries(values)) {
    const s = m.shapes.get(name);
    if (!s || !v) continue;
    const k = v / 10000;
    for (let i = 0; i < s.idx.length; i++) {
      const b = s.idx[i] * 3;
      out[b] += s.d[i * 3] * k; out[b + 1] += s.d[i * 3 + 1] * k; out[b + 2] += s.d[i * 3 + 2] * k;
    }
  }
  return out;
}

// ---------------------------------------------------------------- drawing

/** What the preview shows: colours as #rrggbb, amounts 0–1. */
export interface Look {
  shapes: Record<string, number>;
  skin: string; skinTint: string; skinTintAmount: number; shine: number;
  hairColor: string; eyeColor: string; lipsColor: string; lipstick: number;
  freckles: number; makeUp: string; makeUpAmount: number;
  brows: string; hairShadow: string; beardShadow: string; // head textures painted under hair / brows / beard
  height: number; width: number;
}

export interface Piece { kind: 'hair' | 'beard'; url: string; texture: string }
export interface Cloth { url: string; color: string }

const VS = `
attribute vec3 aPos; attribute vec3 aNrm; attribute vec2 aUv; attribute vec2 aUv2;
uniform mat4 uMvp; uniform mat4 uRot;
varying vec3 vN; varying vec2 vUv; varying vec2 vUv2;
void main() { vN = mat3(uRot) * aNrm; vUv = aUv; vUv2 = aUv2; gl_Position = uMvp * vec4(aPos, 1.0); }`;

// Modes: 0 body skin, 1 head skin (+ the head's detail maps on uv2), 2 eyes, 3 cut-out strands (hair, lashes), 4 plain colour.
const FS = `
precision mediump float;
varying vec3 vN; varying vec2 vUv; varying vec2 vUv2;
uniform int uMode;
uniform sampler2D t0; uniform sampler2D t1; uniform sampler2D t2; uniform sampler2D t3;
uniform sampler2D t4; uniform sampler2D t5; uniform sampler2D t6; uniform sampler2D t7;
uniform vec3 uSkinTint; uniform float uSkinTintAmt; uniform vec3 uHair; uniform vec3 uEye; uniform vec3 uLips;
uniform float uLipstick; uniform float uFreckles; uniform float uMakeUp; uniform vec3 uColor; uniform float uShine;
float lum(vec3 c) { return dot(c, vec3(0.3, 0.59, 0.11)); }
float hairMask(vec3 c) { return clamp((0.88 - lum(c)) * 3.5, 0.0, 1.0); }
vec3 skin(vec2 uv) {
  vec3 c = texture2D(t0, fract(uv)).rgb;
  // The game multiplies the skin by _Skin_Tint, weakly: red at strength 1 took only ~15% off green and blue in game
  // (screenshot vs T_Skin_01, 2026-10-26), so the tint works at a fifth of the slider; white changes nothing.
  return c * mix(vec3(1.0), uSkinTint, uSkinTintAmt * 0.2);
}
// _Skin_Smoothness_Offset: a soft highlight, tighter and brighter with the slider (fitted by eye to the game).
float sheen(vec3 n) {
  float s = clamp(0.3 + uShine * 0.7, 0.0, 1.0);
  vec3 h = normalize(normalize(vec3(0.3, 0.45, 0.85)) + vec3(0.0, 0.0, 1.0));
  return pow(max(dot(n, h), 0.0), mix(10.0, 48.0, s)) * mix(0.03, 0.3, s);
}
void main() {
  vec3 n = normalize(vN);
  if (!gl_FrontFacing) n = -n;
  float light = 0.5 + 0.5 * max(dot(n, normalize(vec3(0.3, 0.45, 0.85))), 0.0);
  vec3 c;
  if (uMode == 0) c = skin(vUv);
  else if (uMode == 1) {
    c = skin(vUv);
    vec2 d = vUv2;
    c *= mix(vec3(1.0), texture2D(t4, d).rgb, uFreckles);
    c *= mix(vec3(1.0), texture2D(t2, d).rgb, uMakeUp);
    c *= mix(vec3(1.0), texture2D(t7, d).rgb, 0.6);
    c = mix(c, uLips * (0.5 + lum(c)), texture2D(t3, d).r * uLipstick);
    // Hairline, stubble and brow maps: white with the hair drawn darker (a coloured glow around it is not hair).
    c = mix(c, uHair * 0.55, hairMask(texture2D(t5, d).rgb) * 0.8);
    c = mix(c, uHair * 0.55, hairMask(texture2D(t6, d).rgb) * 0.8);
    // The brow maps are drawn 2.3× larger than the head's uv2: scaled around the centre they sit above the eyes (fitted to
    // the mesh; the lips and eye-shadow maps line up unscaled).
    c = mix(c, uHair * 0.45, hairMask(texture2D(t1, (d - 0.5) * 2.3 + 0.5).rgb));
  } else if (uMode == 2) {
    // The game's eye material draws the iris mask scaled by _Iris_Size (2.75) around the eye's centre.
    vec3 sclera = texture2D(t0, vUv).rgb;
    vec2 iuv = (vUv - 0.5) * 2.75 + 0.5;
    float iris = texture2D(t1, iuv).r;
    float r = length(iuv - 0.5);
    vec3 ic = uEye * (0.55 + 1.2 * r);
    c = mix(sclera, ic, iris);
    c = mix(c, vec3(0.02), smoothstep(0.13, 0.11, r));
  } else if (uMode == 3) {
    vec4 s = texture2D(t0, vUv);
    if (s.a < 0.33) discard;
    c = uHair * (0.55 + 0.6 * s.r);
  } else c = uColor;
  vec3 lit = c * light;
  if (uMode <= 1) lit += vec3(sheen(n));
  gl_FragColor = vec4(lit, 1.0);
}`;

interface Gpu {
  pos: WebGLBuffer; nrm: WebGLBuffer; rest: WebGLBuffer; idx: WebGLBuffer[]; mesh: CharMesh;
  morph?: Float32Array; morphKey?: string; // the face sliders applied (kept: posing reuses them)
}

const hex = (h: string): number[] => {
  const m = /^#?([0-9a-f]{6})/i.exec(h ?? '');
  const v = m ? parseInt(m[1], 16) : 0xffffff;
  return [((v >> 16) & 255) / 255, ((v >> 8) & 255) / 255, (v & 255) / 255];
};

const MALE_HEAD_MIDDLE = 1.672; // metres up: measured from the male body's head material

export type CameraFrame = 'face' | 'body' | 'portrait';

export class CharacterView {
  private gl: WebGLRenderingContext;
  private prog: WebGLProgram;
  private body: Gpu | null = null;
  private parts: string[] = [];               // body submesh materials
  private pieces: { gpu: Gpu; piece: Piece }[] = [];
  private cloths: { gpu: Gpu; color: string }[] = [];
  private headLower = 0; // how much lower this body's head is than the male one the cameras are set for
  private tex = new Map<string, WebGLTexture>();
  private look: Look | null = null;
  private shapesKey = '';                     // the slider values last morphed (a copy: the editor changes them in place)
  private bones: Bone[] = [];                 // the body's skeleton (posing)
  private skinMats: Float32Array[] | null = null; // the pose's skinning matrices; null = rest pose
  private bodyPos: Float32Array | null = null; // the body's positions as drawn (GL space), for picking
  private posePending = false;                 // a new pose waits for the next frame (several changes, one skinning)
  rx = 0.05; ry = 0; zoom = 1;
  frame: CameraFrame = 'face';
  fixedSize = false;                          // render at the canvas's own size (off-screen portraits)
  background: number[] | null = null;         // clear colour (RGB 0–1); null = transparent
  private loading: Promise<void>[] = [];      // textures still loading
  private raf = 0;
  private disposed = false;

  constructor(private canvas: HTMLCanvasElement, private texUrl: (name: string) => string, preserve = false) {
    const gl = canvas.getContext('webgl', { antialias: true, alpha: true, premultipliedAlpha: false, preserveDrawingBuffer: preserve });
    if (!gl) throw new Error('WebGL is not available');
    this.gl = gl;
    this.prog = compileProgram(gl, VS, FS);
  }

  private upload(m: CharMesh, values: Record<string, number>): Gpu {
    const gl = this.gl;
    const pos = gl.createBuffer()!, nrm = gl.createBuffer()!;
    const rest = gl.createBuffer()!; // uv(2) uv2(2)
    const data = new Float32Array(m.n * 4);
    for (let i = 0; i < m.n; i++) {
      data[i * 4] = m.uv[i * 2]; data[i * 4 + 1] = m.uv[i * 2 + 1];
      if (m.uv2) { data[i * 4 + 2] = m.uv2[i * 2]; data[i * 4 + 3] = m.uv2[i * 2 + 1]; }
    }
    gl.bindBuffer(gl.ARRAY_BUFFER, rest);
    gl.bufferData(gl.ARRAY_BUFFER, data, gl.STATIC_DRAW);
    const ext = gl.getExtension('OES_element_index_uint');
    if (!ext) throw new Error('WebGL: 32-bit indices not supported');
    const idx = m.subs.map((s) => { const b = gl.createBuffer()!; gl.bindBuffer(gl.ELEMENT_ARRAY_BUFFER, b); gl.bufferData(gl.ELEMENT_ARRAY_BUFFER, s, gl.STATIC_DRAW); return b; });
    const g = { pos, nrm, rest, idx, mesh: m };
    this.shape(g, values);
    return g;
  }

  /** The mesh's positions and normals: face sliders, then the pose (skinned meshes only). */
  private shape(g: Gpu, values: Record<string, number>) {
    const gl = this.gl, m = g.mesh;
    const key = JSON.stringify(values);
    if (g.morphKey !== key || !g.morph) { g.morph = morphed(m, values); g.morphKey = key; }
    let pos = g.morph, nrm = m.nrm;
    if (this.skinMats && m.joints && m.weights) ({ pos, nrm } = skin(pos, m.nrm, m.joints, m.weights, this.skinMats));
    if (g === this.body) this.bodyPos = pos;
    gl.bindBuffer(gl.ARRAY_BUFFER, g.pos);
    gl.bufferData(gl.ARRAY_BUFFER, pos, gl.DYNAMIC_DRAW);
    gl.bindBuffer(gl.ARRAY_BUFFER, g.nrm);
    gl.bufferData(gl.ARRAY_BUFFER, nrm, gl.DYNAMIC_DRAW);
  }

  private reshape() {
    const values = this.look?.shapes ?? {};
    for (const g of [this.body, ...this.pieces.map((p) => p.gpu), ...this.cloths.map((c) => c.gpu)]) if (g) this.shape(g, values);
  }

  /** The body's skeleton (characters.json `skeleton`); needed before setPose. */
  setSkeleton(bones: Bone[]) {
    this.bones = bones ?? [];
  }

  /** Bones' local rotations; null or {} = the rest pose. Clothes (not skinned) are hidden while posed. */
  setPose(pose: Pose | null) {
    const posed = !!pose && Object.keys(pose).length > 0 && this.bones.length > 0;
    this.skinMats = posed ? skinMatrices(this.bones, pose!) : null;
    this.posePending = true; // skinned once, right before the next frame is drawn
    this.draw();
  }

  private flushPose() {
    if (!this.posePending) return;
    this.posePending = false;
    this.reshape();
  }

  private texture(name: string): WebGLTexture | null {
    if (!name) return null;
    const have = this.tex.get(name);
    if (have) return have;
    const gl = this.gl, t = gl.createTexture()!;
    gl.bindTexture(gl.TEXTURE_2D, t);
    gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, 1, 1, 0, gl.RGBA, gl.UNSIGNED_BYTE, new Uint8Array([255, 255, 255, 255]));
    this.tex.set(name, t);
    const img = new Image();
    this.loading.push(new Promise<void>((done) => {
      img.onload = () => { if (!this.disposed) { uploadTexture(gl, t, img); this.draw(); } done(); };
      img.onerror = () => done();
    }));
    img.src = this.texUrl(name);
    return t;
  }

  /** The body: its mesh and its parts' materials (characters.json parts). */
  async setBody(url: string, materials: string[]) {
    const m = await loadCharMesh(url);
    if (this.disposed) return;
    this.body = this.upload(m, this.look?.shapes ?? {});
    this.shape(this.body, this.look?.shapes ?? {}); // again now that it is the body (picking positions)
    this.parts = materials;
    const head = m.subs[materials.indexOf('M_Skin_Head')];
    if (head?.length) {
      let lo = Infinity, hi = -Infinity;
      for (const v of head) { const y = m.pos[v * 3 + 1]; if (y < lo) lo = y; if (y > hi) hi = y; }
      this.headLower = MALE_HEAD_MIDDLE - (lo + hi) / 2;
    }
    this.draw();
  }

  async setPieces(pieces: Piece[]) {
    const loaded = await Promise.all(pieces.map(async (p) => ({ gpu: this.upload(await loadCharMesh(p.url), this.look?.shapes ?? {}), piece: p })));
    if (this.disposed) return;
    this.pieces = loaded;
    this.draw();
  }

  async setClothes(cloths: Cloth[]) {
    const gl = this.gl;
    const loaded = await Promise.all(cloths.map(async (c) => ({ gpu: this.upload(await loadCharMesh(c.url), this.look?.shapes ?? {}), color: c.color })));
    if (this.disposed) return;
    this.cloths = loaded;
    this.draw();
  }

  /** New settings; the morphs are redone only when the sliders changed. */
  setLook(look: Look) {
    const key = JSON.stringify(look.shapes);
    const reshape = key !== this.shapesKey;
    this.shapesKey = key;
    this.look = look;
    if (reshape) this.reshape();
    this.draw();
  }

  draw() {
    if (this.disposed || this.raf) return;
    this.raf = requestAnimationFrame(() => { this.raf = 0; this.render(); });
  }

  private matrices(w: number, h: number): { mvp: Mat; rot: Mat } {
    const L = this.look;
    // Face: the head (1.62 m up); body: the whole character. Height / width scale like the game's ScaleCharacter.
    const sy = L?.height ?? 1, sx = L?.width ?? 1;
    const [cy, radius] = this.frame === 'portrait' ? [(1.665 - this.headLower) * sy, 0.2] : this.frame === 'face' ? [(1.63 - this.headLower) * sy, 0.2] : [0.9 * sy, 1.0];
    const c = Math.cos(this.ry), s = Math.sin(this.ry), cx = Math.cos(this.rx), sxr = Math.sin(this.rx);
    const id = () => new Float32Array([1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1]);
    const scale = id(); scale[0] = sx; scale[5] = sy; scale[10] = sx;
    const rotY = id(); rotY[0] = c; rotY[2] = -s; rotY[8] = s; rotY[10] = c;
    const rotX = id(); rotX[5] = cx; rotX[6] = sxr; rotX[9] = -sxr; rotX[10] = cx;
    const tr = id(); tr[13] = -cy;
    const k = id(); k[0] = k[5] = k[10] = 1 / radius;
    const view = id(); view[14] = -3.1 / this.zoom;
    const fov = 0.6, f = 1 / Math.tan(fov / 2), near = 0.05, far = 50;
    const proj = new Float32Array(16);
    proj[0] = f / (w / h); proj[5] = f; proj[10] = (far + near) / (near - far); proj[11] = -1; proj[14] = (2 * far * near) / (near - far);
    // The character faces +z in GL after the flip? It faced +z in Unity, so −z in GL: turn it around to face the camera.
    const face = id(); face[0] = -1; face[10] = -1;
    const rot = mul(rotX, mul(rotY, face));
    const model = mul(rot, mul(k, mul(tr, scale)));
    return { mvp: mul(proj, mul(view, model)), rot };
  }

  private size(): [number, number] {
    const dpr = window.devicePixelRatio || 1;
    return this.fixedSize ? [this.canvas.width, this.canvas.height]
      : [Math.max(1, Math.round(this.canvas.clientWidth * dpr)), Math.max(1, Math.round(this.canvas.clientHeight * dpr))];
  }

  /** Where a point of the body (Unity space) is on the canvas, in CSS pixels from its top-left, and its depth (−1…1). */
  project(p: ArrayLike<number>): { x: number; y: number; depth: number } {
    const [w, h] = this.size();
    const m = this.matrices(w, h).mvp;
    const x = p[0], y = p[1], z = -p[2];
    const cw = m[3] * x + m[7] * y + m[11] * z + m[15];
    const cx = (m[0] * x + m[4] * y + m[8] * z + m[12]) / cw, cy = (m[1] * x + m[5] * y + m[9] * z + m[13]) / cw;
    const cz = (m[2] * x + m[6] * y + m[10] * z + m[14]) / cw;
    return { x: ((cx + 1) / 2) * this.canvas.clientWidth, y: ((1 - cy) / 2) * this.canvas.clientHeight, depth: cz };
  }

  /** The camera's right, up and forward (into the screen) directions in the body's space (Unity). */
  viewAxes(): { right: number[]; up: number[]; forward: number[] } {
    const [w, h] = this.size();
    const r = this.matrices(w, h).rot; // body (GL) → view; its rows are the view axes in GL space
    const row = (i: number) => [r[i], r[4 + i], -r[8 + i]]; // GL → Unity: z negated
    const f = row(2);
    return { right: row(0), up: row(1), forward: [-f[0], -f[1], -f[2]] };
  }

  /**
   * The skeleton bone under a point of the canvas (CSS pixels): the front-most body vertex within a few pixels, and the bone
   * that moves it most. -1 = none.
   */
  pick(px: number, py: number, radius = 14): number {
    this.flushPose();
    const g = this.body, pos = this.bodyPos;
    if (!g || !pos || !g.mesh.joints || !g.mesh.weights) return -1;
    const [w, h] = this.size();
    const m = this.matrices(w, h).mvp;
    const cw = this.canvas.clientWidth, ch = this.canvas.clientHeight;
    let best = -1, bestDepth = Infinity;
    for (let v = 0; v < g.mesh.n; v++) {
      const x = pos[v * 3], y = pos[v * 3 + 1], z = pos[v * 3 + 2];
      const q = m[3] * x + m[7] * y + m[11] * z + m[15];
      const sx = (((m[0] * x + m[4] * y + m[8] * z + m[12]) / q + 1) / 2) * cw;
      const sy = ((1 - (m[1] * x + m[5] * y + m[9] * z + m[13]) / q) / 2) * ch;
      if (Math.abs(sx - px) > radius || Math.abs(sy - py) > radius || Math.hypot(sx - px, sy - py) > radius) continue;
      const d = (m[2] * x + m[6] * y + m[10] * z + m[14]) / q;
      if (d < bestDepth) { bestDepth = d; best = v; }
    }
    if (best < 0) return -1;
    let joint = -1, most = 0;
    for (let k = 0; k < 4; k++) {
      const wt = g.mesh.weights[best * 4 + k];
      if (wt > most) { most = wt; joint = g.mesh.joints[best * 4 + k]; }
    }
    return joint;
  }

  /** Draws now and once more after every texture it asked for has loaded (off-screen use: read the canvas after). */
  async renderNow() {
    this.render();
    while (this.loading.length) {
      const wait = this.loading;
      this.loading = [];
      await Promise.all(wait);
      this.render();
    }
  }

  private render() {
    this.flushPose();
    const gl = this.gl, cv = this.canvas, L = this.look;
    let w = cv.width, h = cv.height;
    if (!this.fixedSize) {
      const dpr = window.devicePixelRatio || 1;
      w = Math.max(1, Math.round(cv.clientWidth * dpr)); h = Math.max(1, Math.round(cv.clientHeight * dpr));
      if (cv.width !== w || cv.height !== h) { cv.width = w; cv.height = h; }
    }
    gl.viewport(0, 0, w, h);
    const bg = this.background;
    gl.clearColor(bg?.[0] ?? 0, bg?.[1] ?? 0, bg?.[2] ?? 0, bg ? 1 : 0);
    gl.clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT);
    if (!this.body || !L) return;
    gl.enable(gl.DEPTH_TEST);
    gl.disable(gl.CULL_FACE);
    gl.useProgram(this.prog);
    const p = this.prog, loc = (n: string) => gl.getUniformLocation(p, n);
    const { mvp, rot } = this.matrices(w, h);
    gl.uniformMatrix4fv(loc('uMvp'), false, mvp);
    gl.uniformMatrix4fv(loc('uRot'), false, rot);
    for (let i = 0; i < 8; i++) gl.uniform1i(loc('t' + i), i);
    gl.uniform3fv(loc('uSkinTint'), hex(L.skinTint));
    gl.uniform1f(loc('uSkinTintAmt'), L.skinTintAmount);
    gl.uniform1f(loc('uShine'), L.shine);
    gl.uniform3fv(loc('uHair'), hex(L.hairColor));
    gl.uniform3fv(loc('uEye'), hex(L.eyeColor));
    gl.uniform3fv(loc('uLips'), hex(L.lipsColor));
    gl.uniform1f(loc('uLipstick'), L.lipstick);
    gl.uniform1f(loc('uFreckles'), L.freckles);
    gl.uniform1f(loc('uMakeUp'), L.makeUpAmount);
    const bind = (unit: number, name: string) => {
      gl.activeTexture(gl.TEXTURE0 + unit);
      gl.bindTexture(gl.TEXTURE_2D, this.texture(name) ?? this.texture('T_Flat_White'));
    };
    const aPos = gl.getAttribLocation(p, 'aPos'), aNrm = gl.getAttribLocation(p, 'aNrm');
    const aUv = gl.getAttribLocation(p, 'aUv'), aUv2 = gl.getAttribLocation(p, 'aUv2');
    const drawIndexed = (g: Gpu, sub: number) => {
      gl.bindBuffer(gl.ARRAY_BUFFER, g.pos);
      gl.enableVertexAttribArray(aPos); gl.vertexAttribPointer(aPos, 3, gl.FLOAT, false, 0, 0);
      gl.bindBuffer(gl.ARRAY_BUFFER, g.nrm);
      gl.enableVertexAttribArray(aNrm); gl.vertexAttribPointer(aNrm, 3, gl.FLOAT, false, 0, 0);
      gl.bindBuffer(gl.ARRAY_BUFFER, g.rest);
      gl.enableVertexAttribArray(aUv); gl.vertexAttribPointer(aUv, 2, gl.FLOAT, false, 16, 0);
      if (aUv2 >= 0) { gl.enableVertexAttribArray(aUv2); gl.vertexAttribPointer(aUv2, 2, gl.FLOAT, false, 16, 8); }
      gl.bindBuffer(gl.ELEMENT_ARRAY_BUFFER, g.idx[sub]);
      gl.drawElements(gl.TRIANGLES, g.mesh.subs[sub].length, gl.UNSIGNED_INT, 0);
    };
    const mode = (m: number) => gl.uniform1i(loc('uMode'), m);

    // Body parts by material.
    this.parts.forEach((mat, i) => {
      if (i >= this.body!.mesh.subs.length) return;
      if (mat === 'M_Skin_Body' || mat === 'M_Skin_Head') {
        mode(mat === 'M_Skin_Head' ? 1 : 0);
        bind(0, L.skin); bind(1, L.brows); bind(2, L.makeUp); bind(3, 'T_Lips'); bind(4, 'T_Freckles_01');
        bind(5, L.hairShadow); bind(6, L.beardShadow); bind(7, 'T_Eye_Shadow');
      } else if (mat === 'M_Eyes') {
        mode(2); bind(0, 'T_Sclera'); bind(1, 'T_Iris_Mask');
      } else if (mat === 'M_Eyelashes') {
        mode(3); bind(0, 'T_Eyelashes');
      } else return; // eye water, eye occlusion: see-through layers
      drawIndexed(this.body!, i);
    });

    // Clothes over the skin (the game hides the body under them with masks): pulled towards the camera a little.
    if (this.cloths.length && !this.skinMats) {
      gl.enable(gl.POLYGON_OFFSET_FILL);
      gl.polygonOffset(-2, -2);
      mode(4);
      for (const c of this.cloths) {
        gl.uniform3fv(loc('uColor'), hex(c.color));
        for (let s = 0; s < c.gpu.mesh.subs.length; s++) drawIndexed(c.gpu, s);
      }
      gl.disable(gl.POLYGON_OFFSET_FILL);
    }

    // Hair and beard: strands cut out of their texture.
    mode(3);
    for (const { gpu, piece } of this.pieces) {
      bind(0, piece.texture);
      for (let s = 0; s < gpu.mesh.subs.length; s++) drawIndexed(gpu, s);
    }
  }

  dispose() {
    this.disposed = true;
    if (this.raf) cancelAnimationFrame(this.raf);
    this.gl.getExtension('WEBGL_lose_context')?.loseContext();
  }
}

/**
 * A portrait like the game's own (tournament pairing screen: 512² on a transparent background, head and shoulders from the
 * front), rendered from the character as the face editor shows it. PNG data URL.
 */
export async function renderPortrait(texUrl: (name: string) => string, body: { url: string; materials: string[] }, look: Look,
  pieces: Piece[], cloths: Cloth[], size = 512): Promise<string> {
  const k = 2; // drawn at twice the size, then scaled down: smooth edges
  const c = document.createElement('canvas');
  c.width = c.height = size * k;
  const v = new CharacterView(c, texUrl, true);
  try {
    v.fixedSize = true;
    v.frame = 'portrait';
    v.rx = 0; v.ry = 0;
    v.setLook(look);
    await v.setBody(body.url, body.materials);
    await v.setPieces(pieces);
    await v.setClothes(cloths);
    await v.renderNow();
    const out = document.createElement('canvas');
    out.width = out.height = size;
    const g = out.getContext('2d')!;
    g.imageSmoothingQuality = 'high';
    g.drawImage(c, 0, 0, size, size);
    return out.toDataURL('image/png');
  } finally { v.dispose(); }
}
