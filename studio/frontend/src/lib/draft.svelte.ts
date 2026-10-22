// A page's unsaved edits: `value` is the working copy, `dirty` compares it with the stored copy (so undoing an edit makes the
// page clean again). Children with edits that aren't in `value` yet (an editor's unexported paint) add a check with `track`.
const clone = <T>(v: T): T => JSON.parse(JSON.stringify(v));

export class Draft<T> {
  value = $state() as T;
  saved = $state('');
  #extras = $state<Array<() => boolean>>([]);
  dirty = $derived(this.saved !== '' && (JSON.stringify(this.value) !== this.saved || this.#extras.some((f) => f())));

  constructor(v?: T) {
    if (v !== undefined) this.reset(v);
  }

  /** Starts from what's stored (after a load or a save). */
  reset(v: T) {
    this.saved = JSON.stringify(v);
    this.value = clone(v);
  }

  /** Back to the stored copy. */
  revert() {
    this.value = JSON.parse(this.saved);
  }

  /** The stored copy (for comparing a part of the draft). */
  stored(): T {
    return JSON.parse(this.saved);
  }

  /** Adds a dirty check; returns its remover (use in an $effect). */
  track(f: () => boolean): () => void {
    this.#extras.push(f);
    return () => {
      const i = this.#extras.indexOf(f);
      if (i >= 0) this.#extras.splice(i, 1);
    };
  }
}
