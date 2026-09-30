// WebGL scene for the figurine editor: several meshes with their own model matrices (the figurine, vanilla toys, a shelf),
// an orbit camera (drag = turn, right/shift-drag = pan, wheel = zoom) and an offscreen render for shop icons. No dependencies.
//
// Space: GL (right-handed, Y up). Unity data (the mod's exports, baked figurines) is converted by mirroring z on load;
// figurine sources (TCG Studio .fig.obj) are already right-handed.

export type Mat = Float32Array;

export interface Geom {
  data: Float32Array; // interleaved position(3) normal(3) uv(2), non-indexed
  count: number;
  min: number[];
  max: number[];
}

const cache = new Map<string, Promise<Geom>>();

/** Loads an OBJ; unity = mirror z (Unity → GL). Faces may be "f a/b/c" or polygons (fanned). */
export function loadGeom(url: string, unity: boolean): Promise<Geom> {
  const key = `${unity ? 'u' : 'r'}|${url}`;
  if (!cache.has(key)) {
    const p = fetch(url).then(async (r) => {
      if (!r.ok) throw new Error(`model not found: ${url}`);
      return parseObj(await r.text(), unity);
    });
    p.catch(() => cache.delete(key));
    cache.set(key, p);
  }
  return cache.get(key)!;
}

export function forgetGeom(url: string) {
  cache.delete(`u|${url}`);
  cache.delete(`r|${url}`);
}

function parseObj(text: string, unity: boolean): Geom {
  const zs = unity ? -1 : 1;
  const v: number[] = [], vt: number[] = [], vn: number[] = [];
  const out: number[] = [];
  const min = [Infinity, Infinity, Infinity], max = [-Infinity, -Infinity, -Infinity];
  const idx = (s: string, n: number) => { const i = parseInt(s, 10); return i < 0 ? n + i : i - 1; };
  const lines = text.split('\n');
  for (const line of lines) {
    const c0 = line.charCodeAt(0);
    if (c0 !== 118 && c0 !== 102) continue; // 'v' / 'f'
    const f = line.trim().split(/\s+/);
    switch (f[0]) {
      case 'v': v.push(+f[1], +f[2], zs * +f[3]); break;
      case 'vt': vt.push(+f[1], +f[2]); break;
      case 'vn': vn.push(+f[1], +f[2], zs * +f[3]); break;
      case 'f': {
        const corners = f.slice(1).map((c) => {
          const [a, b, n] = c.split('/');
          return [idx(a, v.length / 3), b ? idx(b, vt.length / 2) : -1, n ? idx(n, vn.length / 3) : -1];
        });
        for (let k = 1; k + 1 < corners.length; k++) {
          // Mirroring z flips the winding; keep triangles counter-clockwise from outside in GL.
          const tri = unity ? [corners[0], corners[k + 1], corners[k]] : [corners[0], corners[k], corners[k + 1]];
          for (const [a, b, n] of tri) {
            const px = v[a * 3], py = v[a * 3 + 1], pz = v[a * 3 + 2];
            out.push(px, py, pz,
              n >= 0 ? vn[n * 3] : 0, n >= 0 ? vn[n * 3 + 1] : 1, n >= 0 ? vn[n * 3 + 2] : 0,
              b >= 0 ? vt[b * 2] : 0, b >= 0 ? vt[b * 2 + 1] : 0);
            if (px < min[0]) min[0] = px; if (px > max[0]) max[0] = px;
            if (py < min[1]) min[1] = py; if (py > max[1]) max[1] = py;
            if (pz < min[2]) min[2] = pz; if (pz > max[2]) max[2] = pz;
          }
        }
        break;
      }
    }
  }
  return { data: new Float32Array(out), count: out.length / 8, min, max };
}

// ---- matrices (column-major)

export const ident = (): Mat => new Float32Array([1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1]);
export function mul(a: Mat, b: Mat): Mat {
  const o = new Float32Array(16);
  for (let c = 0; c < 4; c++) for (let r = 0; r < 4; r++) {
    let s = 0;
    for (let k = 0; k < 4; k++) s += a[k * 4 + r] * b[c * 4 + k];
    o[c * 4 + r] = s;
  }
  return o;
}
export function translate(x: number, y: number, z: number): Mat { const m = ident(); m[12] = x; m[13] = y; m[14] = z; return m; }
export function scale(x: number, y = x, z = x): Mat { const m = ident(); m[0] = x; m[5] = y; m[10] = z; return m; }
const rad = (d: number) => (d * Math.PI) / 180;
export function rotX(d: number): Mat { const m = ident(), c = Math.cos(rad(d)), s = Math.sin(rad(d)); m[5] = c; m[6] = s; m[9] = -s; m[10] = c; return m; }
export function rotY(d: number): Mat { const m = ident(), c = Math.cos(rad(d)), s = Math.sin(rad(d)); m[0] = c; m[2] = -s; m[8] = s; m[10] = c; return m; }
export function rotZ(d: number): Mat { const m = ident(), c = Math.cos(rad(d)), s = Math.sin(rad(d)); m[0] = c; m[1] = s; m[4] = -s; m[5] = c; return m; }
/** Row-major 4×4 rows (as the mod exports Matrix4x4) → column-major. */
export function fromRows(rows: number[][]): Mat {
  const m = new Float32Array(16);
  for (let r = 0; r < 4; r++) for (let c = 0; c < 4; c++) m[c * 4 + r] = rows[r][c];
  return m;
}
/** Unity ↔ GL: mirror z. */
export const MIRROR_Z = scale(1, 1, -1);
export function point(m: Mat, p: number[]): number[] {
  return [0, 1, 2].map((r) => m[r] * p[0] + m[4 + r] * p[1] + m[8 + r] * p[2] + m[12 + r]);
}
function perspective(fov: number, aspect: number, near: number, far: number): Mat {
  const f = 1 / Math.tan(fov / 2), m = new Float32Array(16);
  m[0] = f / aspect; m[5] = f; m[10] = (far + near) / (near - far); m[11] = -1; m[14] = (2 * far * near) / (near - far);
  return m;
}

// ---- rendering

export interface SceneObj {
  geom: Geom;
  matrix: Mat;
  texture?: string;              // key given to setTexture; missing = flat colour
  color?: [number, number, number, number];
}

export interface Box { min: number[]; max: number[] }

const VS = `
attribute vec3 aPos; attribute vec3 aNrm; attribute vec2 aUv;
uniform mat4 uVp; uniform mat4 uModel; uniform mat3 uNrm;
varying vec3 vN; varying vec2 vUv;
void main() { vN = uNrm * aNrm; vUv = aUv; gl_Position = uVp * uModel * vec4(aPos, 1.0); }`;

const FS = `
precision mediump float;
varying vec3 vN; varying vec2 vUv;
uniform sampler2D uTex; uniform float uTextured; uniform vec4 uColor; uniform float uLit;
void main() {
  vec3 n = normalize(vN);
  if (!gl_FrontFacing) n = -n;
  float light = uLit > 0.5 ? 0.6 + 0.4 * max(dot(n, normalize(vec3(0.35, 0.8, 0.55))), 0.0) : 1.0;
  vec4 base = uTextured > 0.5 ? texture2D(uTex, vUv) : vec4(1.0);
  gl_FragColor = vec4(base.rgb * uColor.rgb * light, uColor.a);
}`;

interface GpuGeom { buf: WebGLBuffer; count: number }

export class FigurineView {
  private gl: WebGLRenderingContext;
  private prog: WebGLProgram;
  private bufs = new Map<Geom, GpuGeom>();
  private textures = new Map<string, WebGLTexture>();
  private objs: SceneObj[] = [];
  private lines: { buf: WebGLBuffer; count: number; color: [number, number, number, number]; top: boolean }[] = [];
  private frame = 0;
  private disposed = false;
  // Orbit camera around target.
  target = [0, 0, 0];
  dist = 1;
  yaw = 30;    // degrees, 0 = looking at the front (from +z)
  pitch = 18;  // degrees, positive = from above
  fov = 0.6;
  onchange?: () => void;
  /**
   * Tool hooks (e.g. drag handles): onpointerdown gets left presses first (canvas CSS pixels) and returns true to take the drag;
   * the camera orbits/pans otherwise. onhover sees the pointer while nothing is dragged (for the cursor).
   */
  onpointerdown?: (e: PointerEvent, x: number, y: number) => boolean;
  onpointerdrag?: (e: PointerEvent, x: number, y: number) => void;
  onpointerup?: (e: PointerEvent) => void;
  onhover?: (x: number, y: number) => void;

  constructor(private canvas: HTMLCanvasElement, opts: { preserve?: boolean; interactive?: boolean } = {}) {
    const gl = canvas.getContext('webgl', { antialias: true, premultipliedAlpha: false, alpha: true, preserveDrawingBuffer: !!opts.preserve });
    if (!gl) throw new Error('WebGL is not available');
    this.gl = gl;
    gl.getExtension('OES_element_index_uint');
    const sh = (type: number, src: string) => {
      const s = gl.createShader(type)!;
      gl.shaderSource(s, src);
      gl.compileShader(s);
      if (!gl.getShaderParameter(s, gl.COMPILE_STATUS)) throw new Error(gl.getShaderInfoLog(s) || 'shader error');
      return s;
    };
    const p = gl.createProgram()!;
    gl.attachShader(p, sh(gl.VERTEX_SHADER, VS));
    gl.attachShader(p, sh(gl.FRAGMENT_SHADER, FS));
    gl.linkProgram(p);
    this.prog = p;
    if (opts.interactive !== false) this.attachControls();
  }

  private attachControls() {
    const c = this.canvas;
    let drag: { x: number; y: number; pan: boolean } | null = null;
    let tool = false;
    const local = (e: PointerEvent) => { const r = c.getBoundingClientRect(); return [e.clientX - r.left, e.clientY - r.top]; };
    c.addEventListener('contextmenu', (e) => e.preventDefault());
    // Middle button pans the view; without this the browser starts auto-scrolling the page.
    c.addEventListener('mousedown', (e) => { if (e.button === 1) e.preventDefault(); });
    c.addEventListener('auxclick', (e) => { if (e.button === 1) e.preventDefault(); });
    c.addEventListener('pointerdown', (e) => {
      if (e.button === 1) e.preventDefault();
      const [x, y] = local(e);
      if (e.button === 0 && this.onpointerdown?.(e, x, y)) {
        tool = true;
        c.setPointerCapture(e.pointerId);
        return;
      }
      drag = { x: e.clientX, y: e.clientY, pan: e.button === 2 || e.button === 1 || e.shiftKey };
      c.setPointerCapture(e.pointerId);
    });
    c.addEventListener('pointerup', (e) => {
      if (tool) { tool = false; this.onpointerup?.(e); }
      drag = null;
      c.releasePointerCapture(e.pointerId);
    });
    c.addEventListener('pointermove', (e) => {
      if (tool) { const [x, y] = local(e); this.onpointerdrag?.(e, x, y); return; }
      if (!drag) { const [x, y] = local(e); this.onhover?.(x, y); return; }
      const dx = e.clientX - drag.x, dy = e.clientY - drag.y;
      drag.x = e.clientX; drag.y = e.clientY;
      if (drag.pan) {
        const k = (this.dist * Math.tan(this.fov / 2) * 2) / Math.max(1, c.clientHeight);
        const [right, up] = this.axes();
        for (let q = 0; q < 3; q++) this.target[q] += (-dx * right[q] + dy * up[q]) * k;
      } else {
        this.yaw -= dx * 0.4;
        this.pitch = Math.max(-89, Math.min(89, this.pitch + dy * 0.4));
      }
      this.draw();
      this.onchange?.();
    });
    c.addEventListener('wheel', (e) => {
      e.preventDefault();
      this.dist *= Math.exp(e.deltaY * 0.001);
      this.draw();
      this.onchange?.();
    }, { passive: false });
  }

  /** Camera right / up / back vectors. */
  private axes(): number[][] {
    const y = rad(this.yaw), p = rad(this.pitch);
    const back = [Math.sin(y) * Math.cos(p), Math.sin(p), Math.cos(y) * Math.cos(p)];
    const right = [Math.cos(y), 0, -Math.sin(y)];
    const up = [back[1] * right[2] - back[2] * right[1], back[2] * right[0] - back[0] * right[2], back[0] * right[1] - back[1] * right[0]];
    return [right, up, back];
  }

  private viewMatrix(): Mat {
    const [r, u, b] = this.axes();
    const eye = [0, 1, 2].map((q) => this.target[q] + b[q] * this.dist);
    const m = ident();
    for (let q = 0; q < 3; q++) { m[q * 4] = r[q]; m[q * 4 + 1] = u[q]; m[q * 4 + 2] = b[q]; }
    m[12] = -(r[0] * eye[0] + r[1] * eye[1] + r[2] * eye[2]);
    m[13] = -(u[0] * eye[0] + u[1] * eye[1] + u[2] * eye[2]);
    m[14] = -(b[0] * eye[0] + b[1] * eye[1] + b[2] * eye[2]);
    return m;
  }

  private viewProj(): Mat {
    const c = this.canvas;
    const near = Math.max(1e-4, this.dist * 0.01), far = this.dist * 50;
    return mul(perspective(this.fov, Math.max(1, c.clientWidth) / Math.max(1, c.clientHeight), near, far), this.viewMatrix());
  }

  /** Screen position (canvas CSS pixels) of a GL point; null when behind the camera. */
  project(p: number[]): number[] | null {
    const m = this.viewProj(), c = this.canvas;
    const x = m[0] * p[0] + m[4] * p[1] + m[8] * p[2] + m[12];
    const y = m[1] * p[0] + m[5] * p[1] + m[9] * p[2] + m[13];
    const w = m[3] * p[0] + m[7] * p[1] + m[11] * p[2] + m[15];
    if (w <= 1e-6) return null;
    return [((x / w + 1) / 2) * c.clientWidth, ((1 - y / w) / 2) * c.clientHeight];
  }

  /** Picking ray through a canvas point (CSS pixels): origin (camera) and unit direction, GL space. */
  ray(x: number, y: number): { o: number[]; d: number[] } {
    const c = this.canvas, [r, u, b] = this.axes();
    const t = Math.tan(this.fov / 2), aspect = Math.max(1, c.clientWidth) / Math.max(1, c.clientHeight);
    const nx = (x / Math.max(1, c.clientWidth)) * 2 - 1, ny = 1 - (y / Math.max(1, c.clientHeight)) * 2;
    const d = [0, 1, 2].map((q) => -b[q] + r[q] * nx * t * aspect + u[q] * ny * t);
    const l = Math.hypot(d[0], d[1], d[2]);
    return { o: [0, 1, 2].map((q) => this.target[q] + b[q] * this.dist), d: d.map((v) => v / l) };
  }

  /** Points the camera at a box so it fills the view (fill = fraction of the view height). */
  frameBox(box: Box, fill = 0.8) {
    this.target = [0, 1, 2].map((q) => (box.min[q] + box.max[q]) / 2);
    const r = Math.max(1e-4, Math.hypot(box.max[0] - box.min[0], box.max[1] - box.min[1], box.max[2] - box.min[2]) / 2);
    this.dist = r / Math.tan(this.fov / 2) / fill;
    this.draw();
  }

  setTexture(key: string, src: TexImageSource) {
    const gl = this.gl;
    let tex = this.textures.get(key);
    if (!tex) { tex = gl.createTexture()!; this.textures.set(key, tex); }
    gl.bindTexture(gl.TEXTURE_2D, tex);
    gl.pixelStorei(gl.UNPACK_FLIP_Y_WEBGL, true); // images are top-down, UVs bottom-up
    gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, gl.RGBA, gl.UNSIGNED_BYTE, src);
    const w = (src as any).width, h = (src as any).height, pot = (n: number) => (n & (n - 1)) === 0;
    if (pot(w) && pot(h)) {
      gl.generateMipmap(gl.TEXTURE_2D);
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR_MIPMAP_LINEAR);
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.REPEAT);
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.REPEAT);
    } else {
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR);
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
    }
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR);
    this.draw();
  }

  setScene(objs: SceneObj[]) {
    const gl = this.gl;
    const used = new Set(objs.map((o) => o.geom));
    for (const [g, b] of this.bufs) if (!used.has(g)) { gl.deleteBuffer(b.buf); this.bufs.delete(g); }
    for (const o of objs) {
      if (this.bufs.has(o.geom)) continue;
      const buf = gl.createBuffer()!;
      gl.bindBuffer(gl.ARRAY_BUFFER, buf);
      gl.bufferData(gl.ARRAY_BUFFER, o.geom.data, gl.STATIC_DRAW);
      this.bufs.set(o.geom, { buf, count: o.geom.count });
    }
    this.objs = objs;
    this.draw();
  }

  /** Wireframe boxes/lines: pairs of points (GL space). top = drawn over everything (handles). */
  setLines(sets: { points: number[]; color: [number, number, number, number]; top?: boolean }[]) {
    const gl = this.gl;
    for (const l of this.lines) gl.deleteBuffer(l.buf);
    this.lines = sets.filter((s) => s.points.length).map((s) => {
      const buf = gl.createBuffer()!;
      gl.bindBuffer(gl.ARRAY_BUFFER, buf);
      const d: number[] = [];
      for (let i = 0; i + 2 < s.points.length; i += 3) d.push(s.points[i], s.points[i + 1], s.points[i + 2], 0, 1, 0, 0, 0);
      gl.bufferData(gl.ARRAY_BUFFER, new Float32Array(d), gl.STATIC_DRAW);
      return { buf, count: d.length / 8, color: s.color, top: !!s.top };
    });
    this.draw();
  }

  draw() {
    if (this.disposed || this.frame) return;
    this.frame = requestAnimationFrame(() => { this.frame = 0; this.renderNow(); });
  }

  renderNow() {
    if (this.disposed) return;
    const gl = this.gl, c = this.canvas;
    const dpr = window.devicePixelRatio || 1;
    const w = Math.max(1, Math.round((c.clientWidth || c.width / dpr) * dpr)), h = Math.max(1, Math.round((c.clientHeight || c.height / dpr) * dpr));
    if (c.clientWidth && (c.width !== w || c.height !== h)) { c.width = w; c.height = h; }
    gl.viewport(0, 0, c.width, c.height);
    gl.clearColor(0, 0, 0, 0);
    gl.clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT);
    gl.enable(gl.DEPTH_TEST);
    gl.disable(gl.CULL_FACE);
    gl.useProgram(this.prog);
    const near = Math.max(1e-4, this.dist * 0.01), far = this.dist * 50;
    const vp = mul(perspective(this.fov, c.width / c.height, near, far), this.viewMatrix());
    const loc = (n: string) => gl.getUniformLocation(this.prog, n);
    gl.uniformMatrix4fv(loc('uVp'), false, vp);
    gl.uniform1i(loc('uTex'), 0);
    const aPos = gl.getAttribLocation(this.prog, 'aPos'), aNrm = gl.getAttribLocation(this.prog, 'aNrm'), aUv = gl.getAttribLocation(this.prog, 'aUv');
    const bind = (buf: WebGLBuffer) => {
      gl.bindBuffer(gl.ARRAY_BUFFER, buf);
      gl.enableVertexAttribArray(aPos); gl.vertexAttribPointer(aPos, 3, gl.FLOAT, false, 32, 0);
      if (aNrm >= 0) { gl.enableVertexAttribArray(aNrm); gl.vertexAttribPointer(aNrm, 3, gl.FLOAT, false, 32, 12); }
      if (aUv >= 0) { gl.enableVertexAttribArray(aUv); gl.vertexAttribPointer(aUv, 2, gl.FLOAT, false, 32, 24); }
    };
    const opaque = this.objs.filter((o) => (o.color?.[3] ?? 1) >= 1), clear = this.objs.filter((o) => (o.color?.[3] ?? 1) < 1);
    for (const o of [...opaque, ...clear]) {
      const g = this.bufs.get(o.geom);
      if (!g) continue;
      bind(g.buf);
      gl.uniformMatrix4fv(loc('uModel'), false, o.matrix);
      gl.uniformMatrix3fv(loc('uNrm'), false, normalMatrix(o.matrix));
      const tex = o.texture ? this.textures.get(o.texture) : undefined;
      gl.activeTexture(gl.TEXTURE0);
      gl.bindTexture(gl.TEXTURE_2D, tex ?? null);
      gl.uniform1f(loc('uTextured'), tex ? 1 : 0);
      gl.uniform1f(loc('uLit'), 1);
      const col = o.color ?? [1, 1, 1, 1];
      gl.uniform4fv(loc('uColor'), col);
      if (col[3] < 1) { gl.enable(gl.BLEND); gl.blendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA); gl.depthMask(false); }
      else { gl.disable(gl.BLEND); gl.depthMask(true); }
      gl.drawArrays(gl.TRIANGLES, 0, g.count);
    }
    gl.depthMask(true);
    gl.disable(gl.BLEND);
    for (const l of [...this.lines.filter((x) => !x.top), ...this.lines.filter((x) => x.top)]) {
      if (l.top) gl.disable(gl.DEPTH_TEST);
      bind(l.buf);
      gl.uniformMatrix4fv(loc('uModel'), false, ident());
      gl.uniformMatrix3fv(loc('uNrm'), false, new Float32Array([1, 0, 0, 0, 1, 0, 0, 0, 1]));
      gl.uniform1f(loc('uTextured'), 0);
      gl.uniform1f(loc('uLit'), 0);
      gl.uniform4fv(loc('uColor'), l.color);
      gl.drawArrays(gl.LINES, 0, l.count);
    }
  }

  dispose() {
    this.disposed = true;
    if (this.frame) cancelAnimationFrame(this.frame);
    this.gl.getExtension('WEBGL_lose_context')?.loseContext();
  }
}

function normalMatrix(m: Mat): Float32Array {
  const a = m[0], b = m[4], c = m[8], d = m[1], e = m[5], f = m[9], g = m[2], h = m[6], i = m[10];
  const det = a * (e * i - f * h) - b * (d * i - f * g) + c * (d * h - e * g) || 1;
  // inverse-transpose, column-major
  return new Float32Array([
    (e * i - f * h) / det, -(b * i - c * h) / det, (b * f - c * e) / det,
    -(d * i - f * g) / det, (a * i - c * g) / det, -(a * f - c * d) / det,
    (d * h - e * g) / det, -(a * h - b * g) / det, (a * e - b * d) / det,
  ]);
}

/** Transforms a box's 8 corners and returns the bounding box. */
export function transformBox(m: Mat, box: Box): Box {
  const min = [Infinity, Infinity, Infinity], max = [-Infinity, -Infinity, -Infinity];
  for (let k = 0; k < 8; k++) {
    const p = point(m, [k & 1 ? box.max[0] : box.min[0], k & 2 ? box.max[1] : box.min[1], k & 4 ? box.max[2] : box.min[2]]);
    for (let q = 0; q < 3; q++) { min[q] = Math.min(min[q], p[q]); max[q] = Math.max(max[q], p[q]); }
  }
  return { min, max };
}

/** Box edges as line pairs. */
export function boxLines(m: Mat, box: Box): number[] {
  const c = (k: number) => point(m, [k & 1 ? box.max[0] : box.min[0], k & 2 ? box.max[1] : box.min[1], k & 4 ? box.max[2] : box.min[2]]);
  const out: number[] = [];
  for (const [a, b] of [[0, 1], [2, 3], [4, 5], [6, 7], [0, 2], [1, 3], [4, 6], [5, 7], [0, 4], [1, 5], [2, 6], [3, 7]]) out.push(...c(a), ...c(b));
  return out;
}
