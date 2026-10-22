// The unsaved edits of a library page (Accessories, Furniture, Decorations): new entries, changed ones ("parked" when the
// page switches away from them, with their finished editor images), deleted ones and the accessories' order. The page shows
// `apply(stored)` as its list; its Save sends `request()` to App.SaveLibraryDraft in one go. The entry being edited stays in
// the page's own working copy until it is parked.

// problem: why the entry couldn't be finished when the page left it (e.g. a poster without a picture); Save opens it and says so.
export type Parked = { item: any; layout: string; texture?: string; icon?: string; problem?: string };

const clone = <T>(v: T): T => JSON.parse(JSON.stringify(v));

export class LibraryDraft {
  parked = $state<Record<string, Parked>>({});
  added = $state<string[]>([]);   // ids of new entries (their data is in parked)
  deleted = $state<string[]>([]); // ids of stored entries
  order = $state<string[] | null>(null); // every id in its new order (accessories), null = unchanged
  shownDirty = $state(false);     // the entry being edited differs from where it started
  dirty = $derived(this.shownDirty || Object.keys(this.parked).length > 0 || this.deleted.length > 0 || this.order !== null);

  /** The page's list: stored entries with the draft applied (new ones at the end unless the order says otherwise). */
  apply(stored: any[]): any[] {
    const out = stored.filter((x) => !this.deleted.includes(x.id)).map((x) => this.parked[x.id]?.item ?? x);
    for (const id of this.added) if (this.parked[id]) out.push(this.parked[id].item);
    if (this.order) {
      const pos = new Map(this.order.map((id, i) => [id, i]));
      out.sort((a, b) => (pos.get(a.id) ?? 1e9) - (pos.get(b.id) ?? 1e9));
    }
    return out;
  }

  isNew(id: string) { return this.added.includes(id); }
  isEdited(id: string) { return !!this.parked[id] && !this.isNew(id); }
  /** The first parked entry that still has a problem (Save refuses until it is fixed). */
  firstProblem(): [string, string] | null {
    for (const [id, p] of Object.entries(this.parked)) if (p.problem) return [id, p.problem];
    return null;
  }

  /** Keeps a finished entry until Save. */
  park(id: string, p: Parked) { this.parked[id] = clone(p); }

  /** Adds a new entry (from App.New…, not stored yet). */
  add(item: any, layout = '') {
    this.parked[item.id] = { item: clone(item), layout };
    this.added.push(item.id);
  }

  /** Deletes entries: new ones just go, stored ones are deleted on Save. */
  remove(ids: string[]) {
    for (const id of ids) {
      delete this.parked[id];
      if (this.isNew(id)) this.added = this.added.filter((x) => x !== id);
      else if (!this.deleted.includes(id)) this.deleted.push(id);
      if (this.order) this.order = this.order.filter((x) => x !== id);
    }
  }

  /** Moves `id` to where `other` is in the full list (shown = apply(stored) of every kind). */
  swap(shown: any[], id: string, other: string) {
    const ids = shown.map((x) => x.id);
    const i = ids.indexOf(id), j = ids.indexOf(other);
    if (i < 0 || j < 0) return;
    ids.splice(i, 1);
    ids.splice(j, 0, id);
    this.order = ids;
  }

  /** The request for App.SaveLibraryDraft. */
  request() {
    return {
      order: this.order ?? [],
      deleted: [...this.deleted],
      items: Object.values(this.parked).map((p) => ({ item: p.item, layout: p.layout, texture: p.texture ?? '', icon: p.icon ?? '' })),
    };
  }

  clear() {
    this.parked = {}; this.added = []; this.deleted = []; this.order = null; this.shownDirty = false;
  }
}
