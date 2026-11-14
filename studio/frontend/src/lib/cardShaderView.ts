// The live preview of a card shader: the player's GLSL, wrapped the way the game wraps it (App.ShaderWrapping, the
// same text as internal/cardshader's game version except for how the inputs arrive), drawn on a card picture with WebGL 2.
// Compiling here is also the editor's error check: messages carry the code's own line numbers.

export type ShaderParam = { name: string; kind: 'number' | 'vec2' | 'colour' | 'image'; value?: number[]; min?: number; max?: number; image?: string };
export type Wrapping = { head: string; tail: string; bridge?: string };
/** A shader's settings besides its code: what it draws on ('' = the picture, 'seeThrough', 'over') and, for Shadertoy
 * code, how it is mixed with the picture. */
export type ShaderLook = { draws?: string; mix?: string; mixStrength?: number };
/** What a shader is told about the card it draws (the game's iBorder, iFoil, iRarity, iGrade). */
export type CardFacts = { border: number; foil: boolean; rarity: number; grade: number };

/** A parameter list with a card's own values (set.json "shaderParams", by name) in place of the shader's. */
export function withCardValues(params: ShaderParam[], own?: Record<string, number[]>): ShaderParam[] {
  if (!own) return params;
  return params.map((p) => (p.kind !== 'image' && own[p.name] ? { ...p, value: own[p.name] } : p));
}

/** A card's own values with one set or removed (undefined); an empty list becomes undefined, as set.json wants it. */
export function setCardValue(own: Record<string, number[]> | undefined, name: string, value: number[] | undefined): Record<string, number[]> | undefined {
  const next = { ...(own ?? {}) };
  if (value) next[name] = value; else delete next[name];
  return Object.keys(next).length ? next : undefined;
}

/** Code written for shadertoy.com: it has mainImage(...) and no card(...) of its own (the same rule as Studio's compiler). */
export const isShadertoy = (code: string) => /\bvoid\s+mainImage\s*\(/.test(code) && !/\bvec4\s+card\s*\(/.test(code);
/** unknown: the name the code uses that nothing declares (the Shaders page makes a parameter of it). */
export type ShaderMessage = { line: number; text: string; unknown?: string };

/** The kind of parameter a name is, going by how the code uses it: texture(name, …) = a picture, name.rgb = a colour,
 * name.xy = two numbers, anything else = a number. */
export function guessKind(code: string, name: string): ShaderParam['kind'] {
  if (new RegExp(`\\btexture\\s*\\(\\s*${name}\\b`).test(code)) return 'image';
  const swizzles = [...code.matchAll(new RegExp(`\\b${name}\\s*\\.\\s*([a-z]+)\\b`, 'g'))].map((m) => m[1]);
  if (swizzles.some((s) => /^[rgbazw]+$/.test(s) || s.length > 2)) return 'colour';
  if (swizzles.some((s) => /^[xy]+$/.test(s))) return 'vec2';
  return 'number';
}

// What is under the shader's picture in the preview: the card's picture ("over the card" shaders) or a plain card in
// its shape (where the game's own card frame would show through a see-through part of a picture shader).
const UNDER_FRAGMENT = `#version 300 es
precision highp float;
uniform sampler2D _MainTex;
uniform float plain;
in vec4 _uvWorld;
out vec4 colour;
void main() {
  vec4 a = texture(_MainTex, _uvWorld.xy);
  colour = vec4(mix(a.rgb, vec3(0.16, 0.17, 0.21), plain) * a.a, a.a);
}`;

/** Whether the code makes a variable (or function, or function argument) of this name itself: "float band = …". A
 * parameter of the same name then clashes with it, and the compiler's message about that is hard to read. */
export function declares(code: string, name: string): boolean {
  const plain = code.replace(/\/\/.*$/gm, '').replace(/\/\*[\s\S]*?\*\//g, '');
  return new RegExp(`\\b(?:float|int|uint|bool|[iub]?vec[234]|mat[234]|sampler2D|void)\\s+${name}\\b`).test(plain);
}

const VERTEX = `#version 300 es
in vec2 corner;
out vec4 _uvWorld;
void main() {
  _uvWorld = vec4(corner, 0.0, 0.0);
  gl_Position = vec4(corner * 2.0 - 1.0, 0.0, 1.0);
}`;

/** The compiler's messages ("ERROR: 0:12: 'x' : undeclared identifier") as lines of the player's code. */
export function parseMessages(log: string, codeLines: number): ShaderMessage[] {
  const out: ShaderMessage[] = [];
  for (const raw of log.split('\n')) {
    const m = /^(?:ERROR|WARNING):\s*\d+:(\d+):\s*(.*)$/.exec(raw.trim());
    if (!m) continue;
    let line = Number(m[1]);
    let text = m[2].trim();
    // Past the end of the code = in the part that calls card(): the function is missing or has another shape.
    if (line > codeLines) {
      line = Math.max(1, codeLines);
      text = /card/.test(text) ? 'the shader needs a function to run: vec4 card(vec2 uv), or a Shadertoy mainImage(out vec4 fragColor, in vec2 fragCoord)' : text;
    }
    // A name the code uses that nothing declares: most often a parameter that hasn't been added yet.
    const unknown = /^'(\w+)' : undeclared identifier/.exec(text);
    if (unknown) text = `'${unknown[1]}' is not known. If it is a value of yours, it becomes a parameter when you stop typing.`;
    // The compiler's own words for common mistakes, said plainly.
    else if (/l-value required/.test(text)) text = "this line gives a value to something that can't take one (a number, a calculation, or a name that is one of your parameters)";
    else if (/syntax error/.test(text)) text = `${text}: something is missing or extra just before this point, most often a ; at the end of the line above, or a bracket`;
    else if (/cannot convert|wrong operand types|no matching overloaded function/.test(text)) text = `${text}: the kinds don't fit (a float where a vec3 is needed, say, or 1 instead of 1.0)`;
    if (!out.some((x) => x.line === line && x.text === text)) out.push({ line, text, unknown: unknown?.[1] });
  }
  if (!out.length && log.trim()) out.push({ line: 1, text: log.trim() });
  return out;
}

export class CardShaderView {
  private gl: WebGL2RenderingContext;
  private program: WebGLProgram | null = null;
  private art: WebGLTexture;
  private artSize = [512, 716];
  private images = new Map<string, WebGLTexture>(); // by URL
  private white: WebGLTexture;
  private mask: WebGLTexture | null = null;
  private under: WebGLProgram | null = null;
  private seed = Math.random();
  private hoverSince: number | null = null;
  /** What the shader draws on (ShaderLook.draws). */
  draws = '';
  facts: CardFacts = { border: 0, foil: true, rarity: 0, grade: 0 };
  private wrapping: Wrapping = { head: '', tail: '' };
  private params: ShaderParam[] = [];
  private imageUrl: (p: ShaderParam) => string = () => '';
  private raf = 0;
  private started = performance.now();
  private pausedAt: number | null = null;
  /** View tilt in the card plane (what iTilt is in the game: about -0.6..0.6 when looking from the side). */
  tilt: [number, number] = [0, 0];

  constructor(private canvas: HTMLCanvasElement) {
    const gl = canvas.getContext('webgl2', { premultipliedAlpha: true, alpha: true, antialias: false });
    if (!gl) throw new Error('This computer cannot show the preview (WebGL 2 is not available).');
    this.gl = gl;
    gl.pixelStorei(gl.UNPACK_FLIP_Y_WEBGL, true);
    this.white = this.texture();
    gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, 1, 1, 0, gl.RGBA, gl.UNSIGNED_BYTE, new Uint8Array([255, 255, 255, 255]));
    this.art = this.texture();
    gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, 1, 1, 0, gl.RGBA, gl.UNSIGNED_BYTE, new Uint8Array([90, 90, 110, 255]));
    const quad = gl.createBuffer();
    gl.bindBuffer(gl.ARRAY_BUFFER, quad);
    gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([0, 0, 1, 0, 0, 1, 1, 1]), gl.STATIC_DRAW);
  }

  private texture(repeat = false): WebGLTexture {
    const gl = this.gl;
    const t = gl.createTexture()!;
    gl.bindTexture(gl.TEXTURE_2D, t);
    const wrap = repeat ? gl.REPEAT : gl.CLAMP_TO_EDGE;
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, wrap);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, wrap);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR);
    return t;
  }

  private load(url: string, into: WebGLTexture, done?: (img: HTMLImageElement) => void) {
    const img = new Image();
    img.onload = () => {
      const gl = this.gl;
      gl.bindTexture(gl.TEXTURE_2D, into);
      gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, gl.RGBA, gl.UNSIGNED_BYTE, img);
      done?.(img);
      this.draw();
    };
    img.src = url;
  }

  /** The card picture the shader is shown on. */
  setArt(url: string) {
    if (url) this.load(url, this.art, (img) => (this.artSize = [img.naturalWidth, img.naturalHeight]));
  }

  /** The card's mask picture (cardMask); '' = none: white everywhere. */
  setMask(url: string) {
    if (!url) { this.mask = null; this.draw(); return; }
    const t = this.texture();
    this.gl.texImage2D(this.gl.TEXTURE_2D, 0, this.gl.RGBA, 1, 1, 0, this.gl.RGBA, this.gl.UNSIGNED_BYTE, new Uint8Array([255, 255, 255, 255]));
    this.mask = t;
    this.load(url, t);
  }

  /** The pointer went onto or left the card (iHover). */
  hover(on: boolean) {
    this.hoverSince = on ? this.hoverSince ?? performance.now() : null;
  }

  /** The parameters' values and pictures (imageUrl turns a parameter's picture into a URL). */
  setParams(params: ShaderParam[], imageUrl: (p: ShaderParam) => string) {
    this.params = params;
    this.imageUrl = imageUrl;
    for (const p of params) {
      const url = p.kind === 'image' ? imageUrl(p) : '';
      if (url && !this.images.has(url)) {
        const t = this.texture(true);
        this.gl.texImage2D(this.gl.TEXTURE_2D, 0, this.gl.RGBA, 1, 1, 0, this.gl.RGBA, this.gl.UNSIGNED_BYTE, new Uint8Array([255, 255, 255, 255]));
        this.images.set(url, t);
        this.load(url, t);
      }
    }
  }

  /** Compiles the code with the wrapping; returns the compiler's messages (none = it is showing). On errors the last
   * good version stays on screen. */
  setCode(code: string, wrapping: Wrapping): ShaderMessage[] {
    this.wrapping = wrapping;
    const gl = this.gl;
    const compile = (type: number, src: string) => {
      const s = gl.createShader(type)!;
      gl.shaderSource(s, src);
      gl.compileShader(s);
      return s;
    };
    const fs = compile(gl.FRAGMENT_SHADER, wrapping.head + code + '\n' + (isShadertoy(code) ? wrapping.bridge ?? '' : '') + wrapping.tail);
    if (!gl.getShaderParameter(fs, gl.COMPILE_STATUS)) {
      const log = gl.getShaderInfoLog(fs) ?? '';
      gl.deleteShader(fs);
      return parseMessages(log, code.split('\n').length);
    }
    const vs = compile(gl.VERTEX_SHADER, VERTEX);
    const prog = gl.createProgram()!;
    gl.attachShader(prog, vs);
    gl.attachShader(prog, fs);
    gl.bindAttribLocation(prog, 0, 'corner');
    gl.linkProgram(prog);
    gl.deleteShader(vs);
    gl.deleteShader(fs);
    if (!gl.getProgramParameter(prog, gl.LINK_STATUS)) {
      const log = gl.getProgramInfoLog(prog) ?? '';
      gl.deleteProgram(prog);
      return parseMessages(log, code.split('\n').length);
    }
    if (this.program) gl.deleteProgram(this.program);
    this.program = prog;
    this.draw();
    return [];
  }

  /** Seconds the shader's clock shows. */
  get time() {
    return ((this.pausedAt ?? performance.now()) - this.started) / 1000;
  }
  get paused() { return this.pausedAt !== null; }
  pause(on: boolean) {
    if (on && this.pausedAt === null) this.pausedAt = performance.now();
    else if (!on && this.pausedAt !== null) {
      this.started += performance.now() - this.pausedAt;
      this.pausedAt = null;
    }
  }
  restart() {
    this.started = performance.now();
    this.seed = Math.random(); // the card "appears" again: another copy's random number
    if (this.pausedAt !== null) this.pausedAt = this.started;
    this.draw();
  }

  draw() {
    const gl = this.gl;
    const { canvas } = this;
    gl.viewport(0, 0, canvas.width, canvas.height);
    gl.clearColor(0, 0, 0, 0);
    gl.clear(gl.COLOR_BUFFER_BIT);
    const prog = this.program;
    if (!prog) return;
    gl.enableVertexAttribArray(0);
    gl.vertexAttribPointer(0, 2, gl.FLOAT, false, 0, 0);
    gl.disable(gl.BLEND);
    if (this.draws !== 'seeThrough') this.drawUnder(this.draws !== 'over');
    gl.useProgram(prog);
    const u = (name: string) => gl.getUniformLocation(prog, name);
    const [tx, ty] = this.tilt;
    const facing = Math.sqrt(Math.max(0, 1 - tx * tx - ty * ty));
    gl.uniform4f(u('_view'), tx, ty, facing, this.time);
    gl.uniform4f(u('_art'), 0, 0, 1, 1);
    gl.uniform4f(u('_misc'), 1, this.artSize[0], this.artSize[1], 0);
    const hover = this.hoverSince === null || this.paused ? 0 : (performance.now() - this.hoverSince) / 1000;
    gl.uniform4f(u('_card'), this.seed, 0.37, this.time, hover);
    gl.uniform4f(u('_facts'), this.facts.border, this.facts.foil ? 1 : 0, this.facts.rarity, this.facts.grade);
    gl.uniform4f(u('_TextureSampleAdd'), 0, 0, 0, 0);

    const values = new Float32Array(16 * 4);
    const pictures: WebGLTexture[] = [this.white, this.white, this.white, this.white];
    let v = 0, i = 0;
    for (const p of this.params) {
      if (p.kind === 'image') {
        if (i < 4) pictures[i++] = this.images.get(this.imageUrl(p)) ?? this.white;
      } else if (v < 16) {
        values.set((p.value ?? []).slice(0, 4), v++ * 4);
      }
    }
    gl.uniform4fv(u('_P'), values);
    gl.activeTexture(gl.TEXTURE0);
    gl.bindTexture(gl.TEXTURE_2D, this.art);
    gl.uniform1i(u('_MainTex'), 0);
    pictures.forEach((t, n) => {
      gl.activeTexture(gl.TEXTURE1 + n);
      gl.bindTexture(gl.TEXTURE_2D, t);
      gl.uniform1i(u('_Tex' + n), 1 + n);
    });
    gl.activeTexture(gl.TEXTURE5);
    gl.bindTexture(gl.TEXTURE_2D, this.mask ?? this.white);
    gl.uniform1i(u('_Mask'), 5);
    gl.activeTexture(gl.TEXTURE0);

    // Like the game: what the shader returns goes over what is already there (colours already multiplied by alpha).
    gl.enable(gl.BLEND);
    gl.blendFunc(gl.ONE, gl.ONE_MINUS_SRC_ALPHA);
    gl.drawArrays(gl.TRIANGLE_STRIP, 0, 4);
    gl.disable(gl.BLEND);
  }

  /** Draws what lies under the shader's picture: the card's picture, or (plain) a plain card in its shape. */
  private drawUnder(plain: boolean) {
    const gl = this.gl;
    if (!this.under) {
      const make = (type: number, src: string) => {
        const s = gl.createShader(type)!;
        gl.shaderSource(s, src);
        gl.compileShader(s);
        return s;
      };
      const p = gl.createProgram()!;
      gl.attachShader(p, make(gl.VERTEX_SHADER, VERTEX));
      gl.attachShader(p, make(gl.FRAGMENT_SHADER, UNDER_FRAGMENT));
      gl.bindAttribLocation(p, 0, 'corner');
      gl.linkProgram(p);
      this.under = p;
    }
    gl.useProgram(this.under);
    gl.activeTexture(gl.TEXTURE0);
    gl.bindTexture(gl.TEXTURE_2D, this.art);
    gl.uniform1i(gl.getUniformLocation(this.under, '_MainTex'), 0);
    gl.uniform1f(gl.getUniformLocation(this.under, 'plain'), plain ? 1 : 0);
    gl.drawArrays(gl.TRIANGLE_STRIP, 0, 4);
  }

  /** Draws every frame until stop(). */
  start() {
    const loop = () => {
      if (!this.paused) this.draw();
      this.raf = requestAnimationFrame(loop);
    };
    cancelAnimationFrame(this.raf);
    this.raf = requestAnimationFrame(loop);
  }
  stop() { cancelAnimationFrame(this.raf); }

  /** The current frame as a small PNG (data URL) for lists. */
  snapshot(width = 150): string {
    this.draw();
    const out = document.createElement('canvas');
    out.width = width;
    out.height = Math.round((width * this.canvas.height) / this.canvas.width);
    out.getContext('2d')!.drawImage(this.canvas, 0, 0, out.width, out.height);
    return out.toDataURL('image/png');
  }

  dispose() {
    this.stop();
    this.gl.getExtension('WEBGL_lose_context')?.loseContext();
  }
}
