// GPU painter for projected models (furniture): every surface reads the layers at its place in its view of the net, blended over its
// own vanilla look (the baked atlas). One shader serves the 3D preview, the net's base picture and the saved atlas, so editing costs
// the same however many surfaces a piece has. Mesh: <base>_paint.bin from Studio (gameextract/furniture_paint.go paintPreview).

import { compileProgram, cropToContent, orbitMatrices, uploadTexture, type Mat } from './meshView';

/** Blend modes the editor offers (AccessoryEditor BLENDS), in shader order. */
export const BLEND_MODES: GlobalCompositeOperation[] = ['source-over', 'multiply', 'screen', 'overlay', 'color', 'soft-light'];

/** At most this many layer textures per draw (groups of normal layers share one). */
export const MAX_LAYER_TEXTURES = 7; // + the base = 8, the texture units WebGL guarantees

/** A layer texture: layers drawn in net space (covering `net`), blended over the base with `blend`. */
export interface LayerTexture { canvas: HTMLCanvasElement; blend: GlobalCompositeOperation }

interface PaintMesh {
  verts: Float32Array;  // 11 floats per vertex: pos xyz, normal xyz (GL space: z negated), atlas uv, net xy, depth
  idx: Uint32Array;
  min: number[]; max: number[];
}

const meshCache = new Map<string, Promise<PaintMesh>>();

export function loadPaintMesh(url: string): Promise<PaintMesh> {
  if (!meshCache.has(url)) {
    meshCache.set(url, fetch(url).then(async (r) => {
      if (!r.ok) throw new Error(`model not found: ${url}`);
      const buf = await r.arrayBuffer();
      const dv = new DataView(buf);
      if (String.fromCharCode(dv.getUint8(0), dv.getUint8(1), dv.getUint8(2), dv.getUint8(3)) !== 'TCGP') throw new Error('not a paint model');
      const nv = dv.getUint32(8, true), ni = dv.getUint32(12, true);
      const verts = new Float32Array(buf.slice(16, 16 + nv * 44));
      const idx = new Uint32Array(buf.slice(16 + nv * 44, 16 + nv * 44 + ni * 4));
      const min = [Infinity, Infinity, Infinity], max = [-Infinity, -Infinity, -Infinity];
      for (let v = 0; v < nv; v++) {
        const o = v * 11;
        verts[o + 2] = -verts[o + 2]; verts[o + 5] = -verts[o + 5]; // Unity → GL (left- to right-handed)
        for (let q = 0; q < 3; q++) { min[q] = Math.min(min[q], verts[o + q]); max[q] = Math.max(max[q], verts[o + q]); }
      }
      return { verts, idx, min, max };
    }));
  }
  return meshCache.get(url)!;
}

const VS = `
precision highp float;
attribute vec3 aPos; attribute vec3 aNrm; attribute vec2 aAtlas; attribute vec2 aNet; attribute float aDepth;
uniform float uMode;      // 0 = 3D view, 1 = into the atlas, 2 = the net (each view seen from its side, nearest wins)
uniform mat4 uMvp; uniform mat4 uRot; uniform vec4 uNet;
varying vec3 vN; varying vec2 vAtlas; varying vec2 vNet;
void main() {
  vN = mat3(uRot) * aNrm; vAtlas = aAtlas; vNet = (aNet - uNet.xy) / uNet.zw;
  if (uMode < 0.5) gl_Position = uMvp * vec4(aPos, 1.0);
  else if (uMode < 1.5) gl_Position = vec4(aAtlas * 2.0 - 1.0, 0.0, 1.0);
  else gl_Position = vec4(vNet.x * 2.0 - 1.0, 1.0 - vNet.y * 2.0, aDepth * 2.0 - 1.0, 1.0);
}`;

// Padding fill: an uncovered pixel takes the nearest covered one within 4 px (8 directions, nearest ring first).
const QUAD_VS = `
attribute vec2 aQuad; varying vec2 vUv;
void main() { vUv = aQuad * 0.5 + 0.5; gl_Position = vec4(aQuad, 0.0, 1.0); }`;
const BLEED_FS = `
precision mediump float;
uniform sampler2D uSrc; uniform vec2 uPx; varying vec2 vUv;
#define TRY(dx, dy) { vec4 s = texture2D(uSrc, vUv + vec2(dx, dy) * uPx * fr); if (s.a > 0.5) { gl_FragColor = s; return; } }
void main() {
  vec4 c = texture2D(uSrc, vUv);
  if (c.a > 0.5) { gl_FragColor = c; return; }
  for (int r = 1; r <= 4; r++) {
    float fr = float(r);
    TRY(1.0, 0.0) TRY(-1.0, 0.0) TRY(0.0, 1.0) TRY(0.0, -1.0) TRY(1.0, 1.0) TRY(-1.0, -1.0) TRY(1.0, -1.0) TRY(-1.0, 1.0)
  }
  gl_FragColor = vec4(0.0);
}`;

function fragmentShader(): string {
  let layers = '';
  for (let i = 0; i < MAX_LAYER_TEXTURES; i++) {
    layers += `  if (uCount > ${i}.5) { vec4 L = texture2D(uL${i}, vNet); c = mix(c, blendMode(c, L.rgb, uBlend[${i}]), L.a); }\n`;
  }
  return `
precision mediump float;
varying vec3 vN; varying vec2 vAtlas; varying vec2 vNet;
uniform sampler2D uBase;
${Array.from({ length: MAX_LAYER_TEXTURES }, (_, i) => `uniform sampler2D uL${i};`).join(' ')}
uniform float uBlend[${MAX_LAYER_TEXTURES}]; uniform float uCount; uniform float uLit; uniform float uAmbient;
// Canvas blend modes (W3C compositing): backdrop b, source s.
float lum(vec3 c) { return dot(c, vec3(0.3, 0.59, 0.11)); }
vec3 clipColor(vec3 c) {
  float l = lum(c), n = min(min(c.r, c.g), c.b), x = max(max(c.r, c.g), c.b);
  if (n < 0.0) c = l + (c - l) * l / (l - n);
  if (x > 1.0) c = l + (c - l) * (1.0 - l) / (x - l);
  return c;
}
vec3 softLight(vec3 b, vec3 s) {
  vec3 d = mix(sqrt(b), ((16.0 * b - 12.0) * b + 4.0) * b, step(b, vec3(0.25)));
  return mix(b - (1.0 - 2.0 * s) * b * (1.0 - b), b + (2.0 * s - 1.0) * (d - b), step(0.5, s));
}
vec3 blendMode(vec3 b, vec3 s, float m) {
  if (m < 0.5) return s;
  if (m < 1.5) return b * s;
  if (m < 2.5) return b + s - b * s;
  if (m < 3.5) return mix(2.0 * b * s, 1.0 - 2.0 * (1.0 - b) * (1.0 - s), step(0.5, b));
  if (m < 4.5) return clipColor(s + (lum(b) - lum(s)));
  return softLight(b, s);
}
void main() {
  vec3 c = texture2D(uBase, vAtlas).rgb;
${layers}  float light = 1.0;
  if (uLit > 0.5) {
    vec3 n = normalize(vN);
    if (!gl_FrontFacing) n = -n;
    light = uAmbient + (1.0 - uAmbient) * max(dot(n, normalize(vec3(0.35, 0.8, 0.55))), 0.0);
  }
  gl_FragColor = vec4(c * light, 1.0);
}`;
}

export class ProjectPainter {
  private gl: WebGLRenderingContext;
  private prog: WebGLProgram;
  private vbuf: WebGLBuffer; private ibuf: WebGLBuffer; private count = 0;
  private bleedProg: WebGLProgram | null = null; private quad: WebGLBuffer | null = null;
  private baseTex: WebGLTexture;
  private layerTex: WebGLTexture[] = [];
  private blends: number[] = [];
  private mesh: PaintMesh | null = null;
  private center = [0, 0, 0];
  private radius = 1;
  /** Net rectangle the layer textures cover (x, y, w, h in net units). */
  private net = [0, 0, 1, 1];
  rx = 0.35; ry = -0.6; zoom = 1; fov = 0.6; ambient = 0.62;
  fixedSize = false;
  private frame = 0;
  private disposed = false;

  constructor(private canvas: HTMLCanvasElement, preserve = false) {
    const gl = canvas.getContext('webgl', { antialias: true, premultipliedAlpha: false, alpha: true, preserveDrawingBuffer: preserve });
    if (!gl) throw new Error('WebGL is not available');
    if (!gl.getExtension('OES_element_index_uint')) throw new Error('WebGL: 32-bit indices are not supported');
    this.gl = gl;
    this.prog = compileProgram(gl, VS, fragmentShader());
    this.vbuf = gl.createBuffer()!; this.ibuf = gl.createBuffer()!;
    this.baseTex = gl.createTexture()!;
    for (let i = 0; i < MAX_LAYER_TEXTURES; i++) this.layerTex.push(gl.createTexture()!);
  }

  async load(url: string) {
    const m = await loadPaintMesh(url);
    if (this.disposed) return;
    const gl = this.gl;
    gl.bindBuffer(gl.ARRAY_BUFFER, this.vbuf);
    gl.bufferData(gl.ARRAY_BUFFER, m.verts, gl.STATIC_DRAW);
    gl.bindBuffer(gl.ELEMENT_ARRAY_BUFFER, this.ibuf);
    gl.bufferData(gl.ELEMENT_ARRAY_BUFFER, m.idx, gl.STATIC_DRAW);
    this.count = m.idx.length;
    this.mesh = m;
    this.center = [0, 1, 2].map((q) => (m.min[q] + m.max[q]) / 2);
    this.radius = Math.max(0.001, Math.hypot(m.max[0] - m.min[0], m.max[1] - m.min[1], m.max[2] - m.min[2]) / 2);
    this.draw();
  }

  /** The surfaces' own look: the baked vanilla atlas, a painted texture or a plain colour. */
  setBase(src: TexImageSource) { uploadTexture(this.gl, this.baseTex, src); this.draw(); }

  /** Layer textures in net space (covering `net`: x, y, w, h), drawn bottom to top. */
  setLayers(textures: LayerTexture[], net: number[]) {
    this.net = net;
    this.blends = textures.slice(0, MAX_LAYER_TEXTURES).map((t) => Math.max(0, BLEND_MODES.indexOf(t.blend)));
    textures.slice(0, MAX_LAYER_TEXTURES).forEach((t, i) => uploadTexture(this.gl, this.layerTex[i], t.canvas, false, false));
    this.draw();
  }

  draw() {
    if (this.disposed || this.frame) return;
    this.frame = requestAnimationFrame(() => { this.frame = 0; this.render(); });
  }

  renderNow() { if (!this.disposed) this.render(); }

  private bind(mode: number, mvp: Mat | null, rot: Mat | null, lit: boolean) {
    const gl = this.gl, p = this.prog;
    gl.useProgram(p);
    const loc = (n: string) => gl.getUniformLocation(p, n);
    const id = new Float32Array([1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1]);
    gl.uniform1f(loc('uMode'), mode);
    gl.uniformMatrix4fv(loc('uMvp'), false, mvp ?? id);
    gl.uniformMatrix4fv(loc('uRot'), false, rot ?? id);
    gl.uniform4f(loc('uNet'), this.net[0], this.net[1], this.net[2], this.net[3]);
    gl.uniform1f(loc('uLit'), lit ? 1 : 0);
    gl.uniform1f(loc('uAmbient'), this.ambient);
    gl.uniform1f(loc('uCount'), this.blends.length);
    gl.uniform1fv(loc('uBlend[0]'), new Float32Array([...this.blends, ...Array(MAX_LAYER_TEXTURES - this.blends.length).fill(0)]));
    gl.activeTexture(gl.TEXTURE0);
    gl.bindTexture(gl.TEXTURE_2D, this.baseTex);
    gl.uniform1i(loc('uBase'), 0);
    for (let i = 0; i < MAX_LAYER_TEXTURES; i++) {
      gl.activeTexture(gl.TEXTURE1 + i);
      gl.bindTexture(gl.TEXTURE_2D, this.layerTex[i]);
      gl.uniform1i(loc(`uL${i}`), 1 + i);
    }
    gl.bindBuffer(gl.ARRAY_BUFFER, this.vbuf);
    gl.bindBuffer(gl.ELEMENT_ARRAY_BUFFER, this.ibuf);
    const attr = (n: string, size: number, off: number) => {
      const a = gl.getAttribLocation(p, n);
      if (a < 0) return;
      gl.enableVertexAttribArray(a);
      gl.vertexAttribPointer(a, size, gl.FLOAT, false, 44, off);
    };
    attr('aPos', 3, 0); attr('aNrm', 3, 12); attr('aAtlas', 2, 24); attr('aNet', 2, 32); attr('aDepth', 1, 40);
    return loc;
  }

  private render() {
    if (!this.mesh) return;
    const gl = this.gl, c = this.canvas;
    let w = c.width, h = c.height;
    if (!this.fixedSize) {
      const dpr = window.devicePixelRatio || 1;
      w = Math.max(1, Math.round(c.clientWidth * dpr)); h = Math.max(1, Math.round(c.clientHeight * dpr));
      if (c.width !== w || c.height !== h) { c.width = w; c.height = h; }
    }
    gl.bindFramebuffer(gl.FRAMEBUFFER, null);
    gl.viewport(0, 0, w, h);
    gl.clearColor(0, 0, 0, 0);
    gl.clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT);
    gl.enable(gl.DEPTH_TEST);
    gl.disable(gl.BLEND);
    // Back faces culled like the game: some models stack two sheets 0.1 mm apart facing opposite ways (warehouse shelf boards), and
    // only the one facing the camera may show. Unity's front faces are clockwise and stay clockwise here (checked by rendering the
    // warehouse shelf from above and below).
    gl.enable(gl.CULL_FACE);
    gl.cullFace(gl.BACK);
    gl.frontFace(gl.CW);
    const { mvp, rot } = orbitMatrices(this, this.center, this.radius, w, h);
    this.bind(0, mvp, rot, true);
    gl.drawElements(gl.TRIANGLES, this.count, gl.UNSIGNED_INT, 0);
  }

  /** A render target (texture + framebuffer, optional depth); bound and cleared. */
  private target(w: number, h: number, depth: boolean) {
    const gl = this.gl;
    const tex = gl.createTexture()!;
    gl.bindTexture(gl.TEXTURE_2D, tex);
    gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, w, h, 0, gl.RGBA, gl.UNSIGNED_BYTE, null);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
    const fb = gl.createFramebuffer()!;
    gl.bindFramebuffer(gl.FRAMEBUFFER, fb);
    gl.framebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, tex, 0);
    let rb: WebGLRenderbuffer | null = null;
    if (depth) {
      rb = gl.createRenderbuffer()!;
      gl.bindRenderbuffer(gl.RENDERBUFFER, rb);
      gl.renderbufferStorage(gl.RENDERBUFFER, gl.DEPTH_COMPONENT16, w, h);
      gl.framebufferRenderbuffer(gl.FRAMEBUFFER, gl.DEPTH_ATTACHMENT, gl.RENDERBUFFER, rb);
    }
    gl.viewport(0, 0, w, h);
    gl.clearColor(0, 0, 0, 0);
    gl.clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT);
    gl.disable(gl.BLEND);
    gl.disable(gl.CULL_FACE);
    if (depth) gl.enable(gl.DEPTH_TEST); else gl.disable(gl.DEPTH_TEST);
    return {
      tex,
      /** Reads the target as a top-down canvas (GL rows are bottom-up). */
      read: (): HTMLCanvasElement => {
        const px = new Uint8Array(w * h * 4);
        gl.readPixels(0, 0, w, h, gl.RGBA, gl.UNSIGNED_BYTE, px);
        const out = document.createElement('canvas');
        out.width = w; out.height = h;
        const ctx = out.getContext('2d')!;
        const img = ctx.createImageData(w, h);
        const row = w * 4;
        for (let y = 0; y < h; y++) img.data.set(px.subarray((h - 1 - y) * row, (h - y) * row), y * row);
        ctx.putImageData(img, 0, 0);
        return out;
      },
      dispose: () => {
        gl.bindFramebuffer(gl.FRAMEBUFFER, null);
        gl.deleteFramebuffer(fb); gl.deleteTexture(tex); if (rb) gl.deleteRenderbuffer(rb);
      },
    };
  }

  /**
   * The finished texture (size²): every surface with its layers at its atlas place, then one pass that fills the padding around the
   * surfaces from their nearest pixels (no base seams when the game filters the texture); unused space keeps the base.
   */
  renderAtlas(size: number, base: TexImageSource): HTMLCanvasElement {
    const gl = this.gl;
    const surfaces = this.target(size, size, false);
    this.bind(1, null, null, false);
    gl.drawElements(gl.TRIANGLES, this.count, gl.UNSIGNED_INT, 0);
    const bled = this.target(size, size, false);
    if (!this.bleedProg) this.bleedProg = compileProgram(gl, QUAD_VS, BLEED_FS);
    gl.useProgram(this.bleedProg);
    if (!this.quad) {
      this.quad = gl.createBuffer()!;
      gl.bindBuffer(gl.ARRAY_BUFFER, this.quad);
      gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([-1, -1, 1, -1, -1, 1, -1, 1, 1, -1, 1, 1]), gl.STATIC_DRAW);
    }
    gl.bindBuffer(gl.ARRAY_BUFFER, this.quad);
    const a = gl.getAttribLocation(this.bleedProg, 'aQuad');
    for (let i = 0; i < 8; i++) gl.disableVertexAttribArray(i);
    gl.enableVertexAttribArray(a);
    gl.vertexAttribPointer(a, 2, gl.FLOAT, false, 8, 0);
    gl.activeTexture(gl.TEXTURE0);
    gl.bindTexture(gl.TEXTURE_2D, surfaces.tex);
    gl.uniform1i(gl.getUniformLocation(this.bleedProg, 'uSrc'), 0);
    gl.uniform2f(gl.getUniformLocation(this.bleedProg, 'uPx'), 1 / size, 1 / size);
    gl.drawArrays(gl.TRIANGLES, 0, 6);
    const out = bled.read();
    bled.dispose(); surfaces.dispose();
    const c = document.createElement('canvas');
    c.width = c.height = size;
    const ctx = c.getContext('2d')!;
    ctx.drawImage(base as CanvasImageSource, 0, 0, size, size);
    ctx.drawImage(out, 0, 0);
    return c;
  }

  /** The net (w×h pixels over `net`): each view as seen from its side — nearest surface wins — with or without the layers. */
  renderNet(w: number, h: number, net: number[], withLayers: boolean): HTMLCanvasElement {
    const keep = { net: this.net, blends: this.blends };
    this.net = net;
    if (!withLayers) this.blends = [];
    const t = this.target(w, h, true);
    try {
      this.bind(2, null, null, false);
      this.gl.drawElements(this.gl.TRIANGLES, this.count, this.gl.UNSIGNED_INT, 0);
      return t.read();
    } finally { t.dispose(); this.net = keep.net; this.blends = keep.blends; }
  }

  /** Net point (x, y in net units) of the surface under a canvas pixel (client coordinates), or null. */
  pick(clientX: number, clientY: number): { x: number; y: number } | null {
    const m = this.mesh;
    if (!m) return null;
    const r = this.canvas.getBoundingClientRect();
    const w = this.canvas.width, h = this.canvas.height;
    const nx = ((clientX - r.left) / r.width) * 2 - 1, ny = 1 - ((clientY - r.top) / r.height) * 2;
    const inv = invert(orbitMatrices(this, this.center, this.radius, w, h).mvp);
    if (!inv) return null;
    const un = (x: number, y: number, z: number) => {
      const v = [0, 1, 2, 3].map((i) => inv[i] * x + inv[4 + i] * y + inv[8 + i] * z + inv[12 + i]);
      return [v[0] / v[3], v[1] / v[3], v[2] / v[3]];
    };
    const o = un(nx, ny, -1), e = un(nx, ny, 1);
    const d = [e[0] - o[0], e[1] - o[1], e[2] - o[2]];
    const V = m.verts, I = m.idx;
    let best = Infinity, hit: { x: number; y: number } | null = null;
    for (let t = 0; t < I.length; t += 3) {
      const a = I[t] * 11, b = I[t + 1] * 11, c = I[t + 2] * 11;
      // Möller–Trumbore
      const e1 = [V[b] - V[a], V[b + 1] - V[a + 1], V[b + 2] - V[a + 2]], e2 = [V[c] - V[a], V[c + 1] - V[a + 1], V[c + 2] - V[a + 2]];
      const p = [d[1] * e2[2] - d[2] * e2[1], d[2] * e2[0] - d[0] * e2[2], d[0] * e2[1] - d[1] * e2[0]];
      const det = e1[0] * p[0] + e1[1] * p[1] + e1[2] * p[2];
      if (Math.abs(det) < 1e-12) continue;
      const s = [o[0] - V[a], o[1] - V[a + 1], o[2] - V[a + 2]];
      const u = (s[0] * p[0] + s[1] * p[1] + s[2] * p[2]) / det;
      if (u < 0 || u > 1) continue;
      const q = [s[1] * e1[2] - s[2] * e1[1], s[2] * e1[0] - s[0] * e1[2], s[0] * e1[1] - s[1] * e1[0]];
      const v = (d[0] * q[0] + d[1] * q[1] + d[2] * q[2]) / det;
      if (v < 0 || u + v > 1) continue;
      const tt = (e2[0] * q[0] + e2[1] * q[1] + e2[2] * q[2]) / det;
      if (tt <= 0 || tt >= best) continue;
      best = tt;
      const w0 = 1 - u - v;
      hit = { x: w0 * V[a + 8] + u * V[b + 8] + v * V[c + 8], y: w0 * V[a + 9] + u * V[b + 9] + v * V[c + 9] };
    }
    return hit;
  }

  dispose() {
    this.disposed = true;
    if (this.frame) cancelAnimationFrame(this.frame);
    this.gl.getExtension('WEBGL_lose_context')?.loseContext();
  }
}

function invert(m: Mat): Mat | null {
  const inv = new Float32Array(16);
  inv[0] = m[5] * m[10] * m[15] - m[5] * m[11] * m[14] - m[9] * m[6] * m[15] + m[9] * m[7] * m[14] + m[13] * m[6] * m[11] - m[13] * m[7] * m[10];
  inv[4] = -m[4] * m[10] * m[15] + m[4] * m[11] * m[14] + m[8] * m[6] * m[15] - m[8] * m[7] * m[14] - m[12] * m[6] * m[11] + m[12] * m[7] * m[10];
  inv[8] = m[4] * m[9] * m[15] - m[4] * m[11] * m[13] - m[8] * m[5] * m[15] + m[8] * m[7] * m[13] + m[12] * m[5] * m[11] - m[12] * m[7] * m[9];
  inv[12] = -m[4] * m[9] * m[14] + m[4] * m[10] * m[13] + m[8] * m[5] * m[14] - m[8] * m[6] * m[13] - m[12] * m[5] * m[10] + m[12] * m[6] * m[9];
  inv[1] = -m[1] * m[10] * m[15] + m[1] * m[11] * m[14] + m[9] * m[2] * m[15] - m[9] * m[3] * m[14] - m[13] * m[2] * m[11] + m[13] * m[3] * m[10];
  inv[5] = m[0] * m[10] * m[15] - m[0] * m[11] * m[14] - m[8] * m[2] * m[15] + m[8] * m[3] * m[14] + m[12] * m[2] * m[11] - m[12] * m[3] * m[10];
  inv[9] = -m[0] * m[9] * m[15] + m[0] * m[11] * m[13] + m[8] * m[1] * m[15] - m[8] * m[3] * m[13] - m[12] * m[1] * m[11] + m[12] * m[3] * m[9];
  inv[13] = m[0] * m[9] * m[14] - m[0] * m[10] * m[13] - m[8] * m[1] * m[14] + m[8] * m[2] * m[13] + m[12] * m[1] * m[10] - m[12] * m[2] * m[9];
  inv[2] = m[1] * m[6] * m[15] - m[1] * m[7] * m[14] - m[5] * m[2] * m[15] + m[5] * m[3] * m[14] + m[13] * m[2] * m[7] - m[13] * m[3] * m[6];
  inv[6] = -m[0] * m[6] * m[15] + m[0] * m[7] * m[14] + m[4] * m[2] * m[15] - m[4] * m[3] * m[14] - m[12] * m[2] * m[7] + m[12] * m[3] * m[6];
  inv[10] = m[0] * m[5] * m[15] - m[0] * m[7] * m[13] - m[4] * m[1] * m[15] + m[4] * m[3] * m[13] + m[12] * m[1] * m[7] - m[12] * m[3] * m[5];
  inv[14] = -m[0] * m[5] * m[14] + m[0] * m[6] * m[13] + m[4] * m[1] * m[14] - m[4] * m[2] * m[13] - m[12] * m[1] * m[6] + m[12] * m[2] * m[5];
  inv[3] = -m[1] * m[6] * m[11] + m[1] * m[7] * m[10] + m[5] * m[2] * m[11] - m[5] * m[3] * m[10] - m[9] * m[2] * m[7] + m[9] * m[3] * m[6];
  inv[7] = m[0] * m[6] * m[11] - m[0] * m[7] * m[10] - m[4] * m[2] * m[11] + m[4] * m[3] * m[10] + m[8] * m[2] * m[7] - m[8] * m[3] * m[6];
  inv[11] = -m[0] * m[5] * m[11] + m[0] * m[7] * m[9] + m[4] * m[1] * m[11] - m[4] * m[3] * m[9] - m[8] * m[1] * m[7] + m[8] * m[3] * m[5];
  inv[15] = m[0] * m[5] * m[10] - m[0] * m[6] * m[9] - m[4] * m[1] * m[10] + m[4] * m[2] * m[9] + m[8] * m[1] * m[6] - m[8] * m[2] * m[5];
  const det = m[0] * inv[0] + m[1] * inv[4] + m[2] * inv[8] + m[3] * inv[12];
  if (Math.abs(det) < 1e-20) return null;
  for (let i = 0; i < 16; i++) inv[i] /= det;
  return inv;
}

/** Shop icon: the painted piece rendered like the vanilla icons (turned by `turn`, slightly from above), cropped to the model. */
export async function renderPaintIcon(url: string, base: TexImageSource, layers: LayerTexture[], net: number[], size: number[], turn = 0): Promise<string> {
  const W = size[0] || 512, H = size[1] || 512, k = 2;
  const c = document.createElement('canvas');
  c.width = W * k; c.height = H * k;
  const p = new ProjectPainter(c, true);
  try {
    p.fixedSize = true; p.ambient = 0.9; p.fov = 1.2; p.zoom = (3.1 * Math.tan(0.6)) / 1.25;
    p.rx = 0.3; p.ry = -0.35 + turn;
    await p.load(url);
    p.setBase(base);
    p.setLayers(layers, net);
    p.renderNow();
    return cropToContent(c, W, H).toDataURL('image/png');
  } finally { p.dispose(); }
}
