// Zoom and pan of a canvas inside a stage element (overflow hidden): the canvas keeps its bitmap and is shown at a CSS size
// (fit × zoom) and offset, so getBoundingClientRect stays exact for pointer math. Wheel zooms around the pointer (Ctrl+wheel
// is left to the page, e.g. to resize a layer), Space+drag or middle-drag pans, Ctrl+0 fits, Ctrl+1 = one canvas px per
// screen px, Ctrl+= / Ctrl+− zoom in and out. An overlay canvas in screen pixels lies on top for guides and handles, so
// they stay sharp and the same size at every zoom (beginOverlay).

export class ZoomPan {
  /** Zoom against the fitted size (1 = fit). */
  zoom = 1;
  private panX = 0;
  private panY = 0;
  private cw = 0;
  private ch = 0;
  private space = false;
  private hover = false;
  private pan: { x: number; y: number; px: number; py: number; id: number } | null = null;
  private ro: ResizeObserver;
  readonly overlay: HTMLCanvasElement;

  constructor(
    private stage: HTMLElement,
    private canvas: () => HTMLCanvasElement | undefined,
    private onChange: () => void,
    private active: () => boolean = () => true,
  ) {
    if (getComputedStyle(stage).position === 'static') stage.style.position = 'relative';
    stage.style.overflow = 'hidden';
    this.overlay = document.createElement('canvas');
    Object.assign(this.overlay.style, { position: 'absolute', inset: '0', width: '100%', height: '100%', pointerEvents: 'none' });
    stage.appendChild(this.overlay);
    stage.addEventListener('wheel', this.onWheel, { passive: false });
    stage.addEventListener('pointerdown', this.onDown, { capture: true });
    stage.addEventListener('pointermove', this.onMove, { capture: true });
    stage.addEventListener('pointerup', this.onUp, { capture: true });
    stage.addEventListener('pointercancel', this.onUp, { capture: true });
    stage.addEventListener('pointerenter', this.onEnter);
    stage.addEventListener('pointerleave', this.onLeave);
    stage.addEventListener('auxclick', this.noAux);
    window.addEventListener('keydown', this.onKey);
    window.addEventListener('keyup', this.onKeyUp);
    window.addEventListener('blur', this.onBlur);
    this.ro = new ResizeObserver(() => this.layout());
    this.ro.observe(stage);
  }

  destroy() {
    this.ro.disconnect();
    this.stage.removeEventListener('wheel', this.onWheel);
    this.stage.removeEventListener('pointerdown', this.onDown, { capture: true });
    this.stage.removeEventListener('pointermove', this.onMove, { capture: true });
    this.stage.removeEventListener('pointerup', this.onUp, { capture: true });
    this.stage.removeEventListener('pointercancel', this.onUp, { capture: true });
    this.stage.removeEventListener('pointerenter', this.onEnter);
    this.stage.removeEventListener('pointerleave', this.onLeave);
    this.stage.removeEventListener('auxclick', this.noAux);
    window.removeEventListener('keydown', this.onKey);
    window.removeEventListener('keyup', this.onKeyUp);
    window.removeEventListener('blur', this.onBlur);
    this.overlay.remove();
  }

  private fitScale() {
    const sw = this.stage.clientWidth, sh = this.stage.clientHeight;
    if (!this.cw || !this.ch || !sw || !sh) return 1;
    return Math.min(sw / this.cw, sh / this.ch);
  }
  /** Screen px per canvas px. */
  scale() { return this.fitScale() * this.zoom; }
  /** Canvas px per screen px (line widths and handle sizes in canvas units). */
  px() { return 1 / this.scale(); }
  private maxZoom() { return Math.max(16, 8 / this.fitScale()); }

  /** Places the canvas; call after its bitmap size may have changed. */
  layout() {
    const c = this.canvas();
    if (!c) return;
    const sizeChanged = c.width !== this.cw || c.height !== this.ch;
    this.cw = c.width; this.ch = c.height;
    const s = this.scale(), w = this.cw * s, h = this.ch * s;
    const sw = this.stage.clientWidth, sh = this.stage.clientHeight;
    if (sizeChanged && this.zoom === 1) { this.panX = 0; this.panY = 0; }
    // Smaller than the stage: centred. Bigger: the stage stays covered.
    this.panX = w <= sw ? (sw - w) / 2 : Math.min(0, Math.max(sw - w, this.panX));
    this.panY = h <= sh ? (sh - h) / 2 : Math.min(0, Math.max(sh - h, this.panY));
    Object.assign(c.style, {
      position: 'absolute', left: `${this.panX}px`, top: `${this.panY}px`, width: `${w}px`, height: `${h}px`,
      maxWidth: 'none', maxHeight: 'none', imageRendering: s >= 2 ? 'pixelated' : 'auto',
    });
    const dpr = window.devicePixelRatio || 1;
    const ow = Math.round(sw * dpr), oh = Math.round(sh * dpr);
    if (this.overlay.width !== ow || this.overlay.height !== oh) { this.overlay.width = ow; this.overlay.height = oh; }
    this.onChange();
  }

  /** Zooms to `z` (against fit), keeping the stage point (sx, sy) where it is (default: the stage centre). */
  zoomTo(z: number, sx = this.stage.clientWidth / 2, sy = this.stage.clientHeight / 2) {
    z = Math.max(1, Math.min(this.maxZoom(), z));
    const s0 = this.scale();
    const qx = (sx - this.panX) / s0, qy = (sy - this.panY) / s0;
    this.zoom = z;
    const s1 = this.scale();
    this.panX = sx - qx * s1; this.panY = sy - qy * s1;
    this.layout();
  }
  fit() { this.zoom = 1; this.layout(); }
  oneToOne() { this.zoomTo(1 / this.fitScale()); }
  zoomIn() { this.zoomTo(this.zoom * 1.25); }
  zoomOut() { this.zoomTo(this.zoom / 1.25); }
  /** Shown size in % of the canvas's own pixels. */
  percent() { return Math.round(this.scale() * 100); }

  /** Clears the overlay and returns it with canvas px as units (null before the first layout). */
  beginOverlay(): { ctx: CanvasRenderingContext2D; px: number } | null {
    const ctx = this.overlay.getContext('2d');
    if (!ctx) return null;
    ctx.setTransform(1, 0, 0, 1, 0, 0);
    ctx.clearRect(0, 0, this.overlay.width, this.overlay.height);
    if (!this.cw) return null;
    const dpr = window.devicePixelRatio || 1, s = this.scale();
    ctx.setTransform(dpr * s, 0, 0, dpr * s, dpr * this.panX, dpr * this.panY);
    return { ctx, px: 1 / s };
  }

  private onWheel = (e: WheelEvent) => {
    e.preventDefault();          // no page scroll or WebView zoom over the stage
    if (e.ctrlKey || e.metaKey) return; // the page's own (resize a layer)
    const r = this.stage.getBoundingClientRect();
    this.zoomTo(this.zoom * (e.deltaY < 0 ? 1.15 : 1 / 1.15), e.clientX - r.left, e.clientY - r.top);
  };
  private onDown = (e: PointerEvent) => {
    if (e.button !== 1 && !(e.button === 0 && this.space)) return;
    e.preventDefault(); e.stopPropagation();
    this.pan = { x: e.clientX, y: e.clientY, px: this.panX, py: this.panY, id: e.pointerId };
    this.stage.setPointerCapture(e.pointerId);
    this.stage.style.cursor = 'grabbing';
  };
  private onMove = (e: PointerEvent) => {
    if (!this.pan) return;
    e.stopPropagation();
    this.panX = this.pan.px + e.clientX - this.pan.x;
    this.panY = this.pan.py + e.clientY - this.pan.y;
    this.layout();
  };
  private onUp = (e: PointerEvent) => {
    if (!this.pan) return;
    e.stopPropagation();
    this.pan = null;
    this.stage.style.cursor = this.space ? 'grab' : '';
  };
  private noAux = (e: MouseEvent) => { if (e.button === 1) e.preventDefault(); };
  private onEnter = () => { this.hover = true; };
  private onLeave = () => { this.hover = false; };
  private live() { return this.active() && this.stage.isConnected && this.stage.offsetParent !== null; }
  private typing(e: KeyboardEvent) {
    const t = e.target as HTMLElement | null;
    return !!t && (t.isContentEditable || ['INPUT', 'SELECT', 'TEXTAREA'].includes(t.tagName) || !!t.closest?.('.backdrop'));
  }
  private onKey = (e: KeyboardEvent) => {
    if (!this.live() || this.typing(e)) return;
    if (e.key === ' ' && this.hover) {
      e.preventDefault();
      if (!this.space) { this.space = true; if (!this.pan) this.stage.style.cursor = 'grab'; }
      return;
    }
    if (!(e.ctrlKey || e.metaKey) || e.altKey) return;
    const act = (f: () => void) => { e.preventDefault(); f(); };
    if (e.key === '0') act(() => this.fit());
    else if (e.key === '1') act(() => this.oneToOne());
    else if (e.key === '=' || e.key === '+') act(() => this.zoomIn());
    else if (e.key === '-' || e.key === '_') act(() => this.zoomOut());
  };
  private onKeyUp = (e: KeyboardEvent) => {
    if (e.key !== ' ' || !this.space) return;
    this.space = false;
    if (!this.pan) this.stage.style.cursor = '';
  };
  private onBlur = () => { this.space = false; if (!this.pan) this.stage.style.cursor = ''; };
}
