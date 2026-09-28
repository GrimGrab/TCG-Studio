// Tiny WebGL viewer for the accessory meshes the mod exports (templates\accessories\*.obj). No dependencies.
// OBJ from the mod: Unity space (left-handed, Y up), UV origin bottom-left, one "f a/a/a" index per vertex.

export interface MeshPart {
  url: string;                          // .obj
  texture?: 'main' | 'secondary';       // main = the texture being edited; secondary = a fixed image (e.g. binder pages)
  secondaryUrl?: string;
  glass?: boolean;                      // drawn see-through, untextured
}

interface Gpu { buf: WebGLBuffer; count: number; part: MeshPart; tex?: WebGLTexture }

interface Parsed { data: Float32Array; count: number; min: number[]; max: number[] }

const objCache = new Map<string, Promise<Parsed>>();

async function loadObj(url: string): Promise<Parsed> {
  if (!objCache.has(url)) {
    objCache.set(url, fetch(url).then(async (r) => {
      if (!r.ok) throw new Error(`model not found: ${url}`);
      return parseObj(await r.text());
    }));
  }
  return objCache.get(url)!;
}

/** Interleaved position(3) normal(3) uv(2) per vertex, non-indexed. Unity → GL: z is negated (left- to right-handed). */
function parseObj(text: string): Parsed {
  const v: number[][] = [], vt: number[][] = [], vn: number[][] = [];
  const out: number[] = [];
  const min = [Infinity, Infinity, Infinity], max = [-Infinity, -Infinity, -Infinity];
  for (const line of text.split('\n')) {
    const f = line.trim().split(/\s+/);
    switch (f[0]) {
      case 'v': v.push([+f[1], +f[2], -f[3]]); break;
      case 'vt': vt.push([+f[1], +f[2]]); break;
      case 'vn': vn.push([+f[1], +f[2], -f[3]]); break;
      case 'f':
        for (let k = 1; k <= 3; k++) {
          const [a, b, c] = f[k].split('/').map((x) => parseInt(x, 10) - 1);
          const p = v[a] ?? [0, 0, 0], t = vt[b] ?? [0, 0], n = vn[c] ?? [0, 1, 0];
          out.push(p[0], p[1], p[2], n[0], n[1], n[2], t[0], t[1]);
          for (let q = 0; q < 3; q++) { min[q] = Math.min(min[q], p[q]); max[q] = Math.max(max[q], p[q]); }
        }
        break;
    }
  }
  return { data: new Float32Array(out), count: out.length / 8, min, max };
}

const VS = `
attribute vec3 aPos; attribute vec3 aNrm; attribute vec2 aUv;
uniform mat4 uMvp; uniform mat4 uModel;
varying vec3 vN; varying vec2 vUv;
void main() { vN = mat3(uModel) * aNrm; vUv = aUv; gl_Position = uMvp * vec4(aPos, 1.0); }`;

const FS = `
precision mediump float;
varying vec3 vN; varying vec2 vUv;
uniform sampler2D uTex; uniform float uGlass; uniform float uTextured;
void main() {
  vec3 n = normalize(vN);
  if (!gl_FrontFacing) n = -n;
  float light = 0.62 + 0.38 * max(dot(n, normalize(vec3(0.35, 0.8, 0.55))), 0.0);
  vec4 base = uTextured > 0.5 ? texture2D(uTex, vUv) : vec4(0.85, 0.9, 0.95, 1.0);
  if (uGlass > 0.5) gl_FragColor = vec4(base.rgb * light, 0.22);
  else gl_FragColor = vec4(base.rgb * light, 1.0);
}`;

type Mat = Float32Array;
const ident = (): Mat => new Float32Array([1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1]);
function mul(a: Mat, b: Mat): Mat {
  const o = new Float32Array(16);
  for (let c = 0; c < 4; c++) for (let r = 0; r < 4; r++) {
    let s = 0;
    for (let k = 0; k < 4; k++) s += a[k * 4 + r] * b[c * 4 + k];
    o[c * 4 + r] = s;
  }
  return o;
}
function rotX(t: number): Mat { const m = ident(), c = Math.cos(t), s = Math.sin(t); m[5] = c; m[6] = s; m[9] = -s; m[10] = c; return m; }
function rotY(t: number): Mat { const m = ident(), c = Math.cos(t), s = Math.sin(t); m[0] = c; m[2] = -s; m[8] = s; m[10] = c; return m; }
function translate(x: number, y: number, z: number): Mat { const m = ident(); m[12] = x; m[13] = y; m[14] = z; return m; }
function scale(s: number): Mat { const m = ident(); m[0] = m[5] = m[10] = s; return m; }
function perspective(fov: number, aspect: number, near: number, far: number): Mat {
  const f = 1 / Math.tan(fov / 2), m = new Float32Array(16);
  m[0] = f / aspect; m[5] = f; m[10] = (far + near) / (near - far); m[11] = -1; m[14] = (2 * far * near) / (near - far);
  return m;
}

export class MeshView {
  private gl: WebGLRenderingContext;
  private prog: WebGLProgram;
  private parts: Gpu[] = [];
  private mainTex: WebGLTexture;
  private center = [0, 0, 0];
  private radius = 1;
  rx = 0.4; // positive = looking down onto the top
  ry = -0.6;
  zoom = 1;
  private frame = 0;
  private disposed = false;

  constructor(private canvas: HTMLCanvasElement, private pixelated = false) {
    const gl = canvas.getContext('webgl', { antialias: true, premultipliedAlpha: false, alpha: true });
    if (!gl) throw new Error('WebGL is not available');
    this.gl = gl;
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
    this.mainTex = gl.createTexture()!;
  }

  /** Loads the model parts; frames the camera on their combined bounds. */
  async load(parts: MeshPart[]) {
    const gl = this.gl;
    const min = [Infinity, Infinity, Infinity], max = [-Infinity, -Infinity, -Infinity];
    const loaded: Gpu[] = [];
    for (const part of parts) {
      const m = await loadObj(part.url);
      const buf = gl.createBuffer()!;
      gl.bindBuffer(gl.ARRAY_BUFFER, buf);
      gl.bufferData(gl.ARRAY_BUFFER, m.data, gl.STATIC_DRAW);
      const g: Gpu = { buf, count: m.count, part };
      if (part.texture === 'secondary' && part.secondaryUrl) {
        const img = await new Promise<HTMLImageElement>((res, rej) => { const i = new Image(); i.onload = () => res(i); i.onerror = rej; i.src = part.secondaryUrl!; });
        g.tex = gl.createTexture()!;
        this.upload(g.tex, img, false);
      }
      loaded.push(g);
      for (let q = 0; q < 3; q++) { min[q] = Math.min(min[q], m.min[q]); max[q] = Math.max(max[q], m.max[q]); }
    }
    if (this.disposed) return;
    this.parts = loaded;
    this.center = [0, 1, 2].map((q) => (min[q] + max[q]) / 2);
    this.radius = Math.max(0.001, Math.hypot(max[0] - min[0], max[1] - min[1], max[2] - min[2]) / 2);
    this.draw();
  }

  private upload(tex: WebGLTexture, src: TexImageSource, nearest: boolean) {
    const gl = this.gl;
    gl.bindTexture(gl.TEXTURE_2D, tex);
    gl.pixelStorei(gl.UNPACK_FLIP_Y_WEBGL, true); // images are top-down, UVs bottom-up
    gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, gl.RGBA, gl.UNSIGNED_BYTE, src);
    const pot = (n: number) => (n & (n - 1)) === 0;
    const w = (src as any).width, h = (src as any).height;
    if (!nearest && pot(w) && pot(h)) {
      gl.generateMipmap(gl.TEXTURE_2D);
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR_MIPMAP_LINEAR);
    } else gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, nearest ? gl.NEAREST : gl.LINEAR);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, nearest ? gl.NEAREST : gl.LINEAR);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
  }

  /** The texture being edited (the composed game texture). */
  setTexture(src: TexImageSource) {
    this.upload(this.mainTex, src, this.pixelated);
    this.draw();
  }

  draw() {
    if (this.disposed || this.frame) return;
    this.frame = requestAnimationFrame(() => { this.frame = 0; this.render(); });
  }

  private render() {
    const gl = this.gl, c = this.canvas;
    const dpr = window.devicePixelRatio || 1;
    const w = Math.max(1, Math.round(c.clientWidth * dpr)), h = Math.max(1, Math.round(c.clientHeight * dpr));
    if (c.width !== w || c.height !== h) { c.width = w; c.height = h; }
    gl.viewport(0, 0, w, h);
    gl.clearColor(0, 0, 0, 0);
    gl.clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT);
    gl.enable(gl.DEPTH_TEST);
    gl.disable(gl.CULL_FACE);
    gl.useProgram(this.prog);

    const model = mul(rotX(this.rx), mul(rotY(this.ry), mul(scale(1 / this.radius), translate(-this.center[0], -this.center[1], -this.center[2]))));
    const view = translate(0, 0, -3.1 / this.zoom);
    const mvp = mul(perspective(0.6, w / h, 0.05, 50), mul(view, model));
    const loc = (n: string) => gl.getUniformLocation(this.prog, n);
    gl.uniformMatrix4fv(loc('uMvp'), false, mvp);
    gl.uniformMatrix4fv(loc('uModel'), false, mul(rotX(this.rx), rotY(this.ry)));
    gl.uniform1i(loc('uTex'), 0);
    const aPos = gl.getAttribLocation(this.prog, 'aPos'), aNrm = gl.getAttribLocation(this.prog, 'aNrm'), aUv = gl.getAttribLocation(this.prog, 'aUv');

    // Opaque parts first, then glass (blended, no depth writes).
    const ordered = [...this.parts.filter((p) => !p.part.glass), ...this.parts.filter((p) => p.part.glass)];
    for (const g of ordered) {
      gl.bindBuffer(gl.ARRAY_BUFFER, g.buf);
      gl.enableVertexAttribArray(aPos); gl.vertexAttribPointer(aPos, 3, gl.FLOAT, false, 32, 0);
      if (aNrm >= 0) { gl.enableVertexAttribArray(aNrm); gl.vertexAttribPointer(aNrm, 3, gl.FLOAT, false, 32, 12); }
      if (aUv >= 0) { gl.enableVertexAttribArray(aUv); gl.vertexAttribPointer(aUv, 2, gl.FLOAT, false, 32, 24); }
      gl.activeTexture(gl.TEXTURE0);
      gl.bindTexture(gl.TEXTURE_2D, g.tex ?? this.mainTex);
      gl.uniform1f(loc('uGlass'), g.part.glass ? 1 : 0);
      gl.uniform1f(loc('uTextured'), g.part.glass ? 0 : 1);
      if (g.part.glass) { gl.enable(gl.BLEND); gl.blendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA); gl.depthMask(false); }
      else { gl.disable(gl.BLEND); gl.depthMask(true); }
      gl.drawArrays(gl.TRIANGLES, 0, g.count);
    }
    gl.depthMask(true);
  }

  dispose() {
    this.disposed = true;
    if (this.frame) cancelAnimationFrame(this.frame);
    this.gl.getExtension('WEBGL_lose_context')?.loseContext();
  }
}
