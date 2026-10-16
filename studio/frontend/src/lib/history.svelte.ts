// Undo / redo for an editor whose state can be snapshotted as a string (Ctrl+Z, Ctrl+Y / Ctrl+Shift+Z).
// `base` is the state after the last edit. An edit pushes it (the state before the edit) unless it continues a burst
// (typing, sliders: edits under BURST_MS apart are one step). A drag is one step: the caller pushes when it starts.

const BURST_MS = 600;
const LIMIT = 100;

export interface History {
  readonly canUndo: boolean;
  readonly canRedo: boolean;
  /** The state after the last edit. */
  base: string;
  /** Forget everything; the current state becomes the base. */
  reset(): void;
  /** Push a state (default: base) as an undo step, unless it is the top one already. */
  push(state?: string): void;
  /** Call after an edit: pushes base unless the edit continues a burst. The caller then updates base. */
  edited(): void;
  /** The next edit starts a new step. */
  split(): void;
  /** After a drag: the current state becomes the base; a drag that changed nothing leaves no step. */
  settle(): void;
  undo(): void;
  redo(): void;
}

/** snapshot = the editor's state now; restore puts a snapshot back; busy (optional) blocks undo/redo (e.g. mid-drag). */
export function createHistory(snapshot: () => string, restore: (state: string) => void, busy: () => boolean = () => false): History {
  let undoStack = $state<string[]>([]);
  let redoStack = $state<string[]>([]);
  let lastEdit = 0;
  const h: History = {
    get canUndo() { return undoStack.length > 0; },
    get canRedo() { return redoStack.length > 0; },
    base: '',
    reset() {
      undoStack = [];
      redoStack = [];
      h.base = snapshot();
      lastEdit = 0;
    },
    push(state = h.base) {
      if (!state || undoStack.at(-1) === state) return;
      undoStack = [...undoStack.slice(-(LIMIT - 1)), state];
      redoStack = [];
    },
    edited() {
      const now = Date.now();
      if (now - lastEdit > BURST_MS) h.push();
      lastEdit = now;
    },
    split() { lastEdit = 0; },
    settle() {
      h.base = snapshot();
      lastEdit = 0;
      if (undoStack.at(-1) === h.base) undoStack = undoStack.slice(0, -1);
    },
    undo() {
      if (!undoStack.length || busy()) return;
      redoStack = [...redoStack, snapshot()];
      const prev = undoStack[undoStack.length - 1];
      undoStack = undoStack.slice(0, -1);
      h.base = prev;
      lastEdit = 0;
      restore(prev);
    },
    redo() {
      if (!redoStack.length || busy()) return;
      undoStack = [...undoStack, snapshot()];
      const next = redoStack[redoStack.length - 1];
      redoStack = redoStack.slice(0, -1);
      h.base = next;
      lastEdit = 0;
      restore(next);
    },
  };
  return h;
}
