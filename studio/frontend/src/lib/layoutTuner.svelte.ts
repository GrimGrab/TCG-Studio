// The layout tuner's work on the texture view (views/editor/LayoutTuner.svelte is its panel): picking, dragging and adding the
// pieces of a face map, with its own undo. It edits only the editor's model in memory; the pattern only ever goes to the screen.
import { moveTarget, newFaceNet, trueScale, tunerPattern, type Model, type Target } from './accessoryArt';

export const TUNE_HANDLES = [[-1, -1], [0, -1], [1, -1], [-1, 0], [1, 0], [-1, 1], [0, 1], [1, 1]];

type Point = { x: number; y: number };

/** What the tuner needs from the editor. */
export interface TunerHost {
  model(): Model | null;
  redraw(): void;
  toSpace(e: PointerEvent): Point;
  /** Texture pixels per screen pixel at the current zoom. */
  pixelsPerScreen(): number;
  outlines(): number[][];
  setCursor(c: string): void;
  /** Called when the tuner opens: the texture view with guides on. */
  onOpen(): Promise<void>;
}

export class LayoutTunerState {
  on = $state(false);
  picked = $state<any>(null);
  usePattern = $state(true);
  adding = $state('');
  hold = $state(false);
  rev = $state(0);
  undoList = $state<string[]>([]);
  redoList = $state<string[]>([]);
  clip = $state<{ piece: string; face: string } | null>(null);
  /** The test pattern shown instead of the art; null = the art. */
  pattern: HTMLCanvasElement | null = null;

  private drag: { hx: number; hy: number; sx: number; sy: number; rect: number[] } | null = null;
  private last = '';
  private isNew = false;
  private host: TunerHost;

  constructor(host: TunerHost) { this.host = host; }

  private get model() { return this.host.model()!; }
  private faceOfPicked() { return this.model.faces.find((x) => x.targets.includes(this.picked)); }

  /** The faces as JSON after each change (a whole drag is one step). */
  private record() {
    const now = JSON.stringify(this.model.faces);
    if (now === this.last) return;
    this.undoList = [...this.undoList.slice(-199), this.last];
    this.redoList = [];
    this.last = now;
  }
  /** Goes to a saved state; the picked piece stays picked when it's still there. */
  private restore(json: string) {
    let at = [-1, -1];
    this.model.faces.forEach((f, i) => { const j = f.targets.indexOf(this.picked); if (j >= 0) at = [i, j]; });
    this.model.faces = JSON.parse(json);
    this.last = json;
    this.picked = this.model.faces[at[0]]?.targets[at[1]] ?? null;
    this.retune();
  }
  undo = () => {
    const json = this.undoList.at(-1);
    if (json === undefined) return;
    this.redoList = [...this.redoList, this.last];
    this.undoList = this.undoList.slice(0, -1);
    this.restore(json);
  };
  redo = () => {
    const json = this.redoList.at(-1);
    if (json === undefined) return;
    this.undoList = [...this.undoList, this.last];
    this.redoList = this.redoList.slice(0, -1);
    this.restore(json);
  };
  copy = () => {
    const f = this.faceOfPicked();
    if (f) this.clip = { piece: JSON.stringify(this.picked), face: f.id };
  };
  /** A copy of the copied piece, 8 px off it, in the picked piece's face (none picked: the face it came from). */
  paste = () => {
    if (!this.clip) return;
    const f = this.faceOfPicked() ?? this.model.faces.find((x) => x.id === this.clip!.face);
    if (!f) return;
    const t = JSON.parse(this.clip.piece);
    t.part = `${t.part || f.label} copy`;
    moveTarget(t, [t.rect[0] + 8, t.rect[1] + 8, t.rect[2] + 8, t.rect[3] + 8]);
    f.targets.push(t);
    this.picked = f.targets[f.targets.length - 1];
    this.retune();
  };

  /** The tuner changed something: the pattern, the art under it and the views. */
  retune = () => {
    const m = this.host.model();
    if (this.on && m && !this.drag && !this.hold) this.record();
    this.pattern = this.on && this.usePattern && m ? tunerPattern(m, this.picked) : null;
    this.host.redraw();
  };
  toggle = async () => {
    this.on = !this.on;
    if (this.on) {
      this.last = JSON.stringify(this.model.faces);
      this.undoList = [];
      this.redoList = [];
      await this.host.onOpen();
    }
    this.retune();
  };

  /** The handle of the picked piece under a texture point. */
  private handleAt(p: Point) {
    if (!this.picked) return null;
    const r = this.picked.rect, q = 8 * this.host.pixelsPerScreen();
    return TUNE_HANDLES.find(([hx, hy]) => Math.abs(p.x - ((r[0] + r[2]) / 2 + (hx * (r[2] - r[0])) / 2)) < q && Math.abs(p.y - ((r[1] + r[3]) / 2 + (hy * (r[3] - r[1])) / 2)) < q) ?? null;
  }
  /** The piece under a texture point: the smallest box there (a thin piece inside a big one stays reachable). */
  private pieceAt(p: Point): Target | null {
    const area = (t: Target) => (t.rect[2] - t.rect[0]) * (t.rect[3] - t.rect[1]);
    return this.model.faces.flatMap((f) => f.targets).filter((t) => p.x >= t.rect[0] && p.x < t.rect[2] && p.y >= t.rect[1] && p.y < t.rect[3])
      .sort((a, b) => area(a) - area(b))[0] ?? null;
  }
  /** A dragged edge lands on a blue mesh line within a few screen px (Alt: free). */
  private snap(v: number, axis: 0 | 1, alt: boolean) {
    if (alt) return v;
    let best = v, d = 6 * this.host.pixelsPerScreen();
    for (const s of this.host.outlines()) for (const c of [s[axis], s[axis + 2]]) if (Math.abs(c - v) < d) { d = Math.abs(c - v); best = c; }
    return Math.round(best);
  }

  pointerDown(e: PointerEvent) {
    const p = this.host.toSpace(e);
    if (this.adding) { // a new piece of that face: it starts here, and the drag pulls out its far corner
      let f = this.model.faces.find((x) => x.id === this.adding);
      if (!f) { // a new face, named in the tuner
        this.model.faces.push({ id: this.adding, label: this.adding, net: [0, 0, 1, 1], targets: [] });
        f = this.model.faces[this.model.faces.length - 1];
      }
      this.adding = '';
      const x = this.snap(p.x, 0, e.altKey), y = this.snap(p.y, 1, e.altKey);
      f.targets.push({ rect: [x, y, x + 2, y + 2], src: [0, 0, 1, 1], part: `Piece ${f.targets.length + 1}` });
      this.isNew = true;
      this.picked = f.targets[f.targets.length - 1];
      this.drag = { hx: 1, hy: 1, sx: x + 2, sy: y + 2, rect: [...this.picked.rect] };
      (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
      this.retune();
      return;
    }
    const h = this.handleAt(p);
    if (!h) { // a click inside the picked piece keeps it (it may hold a smaller one); elsewhere picks what's there
      const r = this.picked?.rect;
      if (!(r && p.x >= r[0] && p.x < r[2] && p.y >= r[1] && p.y < r[3])) { this.picked = this.pieceAt(p); this.retune(); }
    }
    if (!this.picked) return;
    this.drag = { hx: h?.[0] ?? 0, hy: h?.[1] ?? 0, sx: p.x, sy: p.y, rect: [...this.picked.rect] };
    (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
  }
  pointerMove(e: PointerEvent) {
    const p = this.host.toSpace(e);
    if (!this.drag) {
      const h = this.handleAt(p);
      this.host.setCursor(h ? (h[0] && h[1] ? (h[0] === h[1] ? 'nwse-resize' : 'nesw-resize') : h[0] ? 'ew-resize' : 'ns-resize') : 'move');
      return;
    }
    const dx = Math.round(p.x - this.drag.sx), dy = Math.round(p.y - this.drag.sy), r = [...this.drag.rect], { hx, hy } = this.drag;
    if (!hx && !hy) { r[0] += dx; r[2] += dx; r[1] += dy; r[3] += dy; }
    if (hx < 0) r[0] = Math.min(this.snap(r[0] + dx, 0, e.altKey), r[2] - 2);
    if (hx > 0) r[2] = Math.max(this.snap(r[2] + dx, 0, e.altKey), r[0] + 2);
    if (hy < 0) r[1] = Math.min(this.snap(r[1] + dy, 1, e.altKey), r[3] - 2);
    if (hy > 0) r[3] = Math.max(this.snap(r[3] + dy, 1, e.altKey), r[1] + 2);
    if (r.join() !== this.picked.rect.join()) { moveTarget(this.picked, r); this.retune(); }
  }
  pointerUp() {
    if (!this.drag) return;
    this.drag = null;
    // A face with one piece and made here (no table face has this net) takes its size from that piece.
    const f = this.faceOfPicked();
    if (f && f.id === f.label && f.targets.length === 1) f.net = newFaceNet(this.model, this.picked);
    // A new piece shows the part of its face that fits at true scale, not the whole face squeezed in.
    if (f && this.isNew) { const [w, h] = trueScale(f, this.picked); this.picked.src = [(1 - w) / 2, (1 - h) / 2, (1 + w) / 2, (1 + h) / 2].map((v) => Math.round(v * 1000) / 1000); }
    this.isNew = false;
    this.retune();
  }

  /** The tuner's keys while it's open; true = handled (or held back from the layer shortcuts). */
  onKey(e: KeyboardEvent, k: string, ctrl: boolean, act: (f: () => void) => void): boolean {
    if (!this.on) return false;
    const nudge = this.picked ? ({ arrowleft: [-1, 0], arrowright: [1, 0], arrowup: [0, -1], arrowdown: [0, 1] } as Record<string, number[]>)[k] : undefined;
    if (nudge) {
      const n = e.shiftKey ? 10 : 1, r = this.picked.rect;
      act(() => { moveTarget(this.picked, [r[0] + nudge[0] * n, r[1] + nudge[1] * n, r[2] + nudge[0] * n, r[3] + nudge[1] * n]); this.retune(); });
    } else if (ctrl && k === 'z' && !e.shiftKey) act(this.undo);
    else if (ctrl && (k === 'y' || (k === 'z' && e.shiftKey))) act(this.redo);
    else if (ctrl && k === 'c' && this.picked) act(this.copy);
    else if (ctrl && k === 'v') act(this.paste);
    else if (e.key === 'Delete' && this.picked) act(() => {
      const f = this.faceOfPicked();
      if (f) { f.targets.splice(f.targets.indexOf(this.picked), 1); this.picked = null; this.retune(); }
    });
    else return ctrl;
    return true;
  }
}
