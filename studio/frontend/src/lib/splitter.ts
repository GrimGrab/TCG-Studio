// Svelte action for a drag divider: dragging sets a CSS variable on a container (a column width, a row height or a ratio in
// %), clamped and (with a key) remembered per viewer in localStorage. Double-click goes back to the default.

export interface SplitOpts {
  container: () => HTMLElement | null | undefined;
  prop: string;                       // CSS variable, e.g. '--split'
  key?: string;                       // localStorage key (none: starts from def every time)
  def: number;
  unit: '%' | 'px';
  /** The value for a pointer position over the container's rect. */
  measure: (e: PointerEvent, r: DOMRect) => number;
  min: (r: DOMRect) => number;
  max: (r: DOMRect) => number;
}

function stored(key?: string): number | null {
  if (!key) return null;
  try { const v = parseFloat(localStorage.getItem(key) ?? ''); return Number.isFinite(v) ? v : null; } catch { return null; }
}

export function splitter(node: HTMLElement, o: SplitOpts) {
  let opts = o;
  let dragging = false;
  const apply = (v: number) => opts.container()?.style.setProperty(opts.prop, `${v}${opts.unit}`);
  const clamp = (v: number) => {
    const r = opts.container()?.getBoundingClientRect();
    return r && r.width ? Math.max(opts.min(r), Math.min(opts.max(r), v)) : v;
  };
  const save = (v: number) => { if (!opts.key) return; try { localStorage.setItem(opts.key, String(Math.round(v * 10) / 10)); } catch { /* per-viewer only */ } };
  apply(stored(opts.key) ?? opts.def);

  const down = (e: PointerEvent) => {
    if (e.button !== 0) return;
    e.preventDefault();
    dragging = true;
    node.setPointerCapture(e.pointerId);
    node.classList.add('dragging');
  };
  const move = (e: PointerEvent) => {
    if (!dragging) return;
    const r = opts.container()?.getBoundingClientRect();
    if (r) apply(clamp(opts.measure(e, r)));
  };
  const up = (e: PointerEvent) => {
    if (!dragging) return;
    dragging = false;
    node.classList.remove('dragging');
    const r = opts.container()?.getBoundingClientRect();
    if (r) save(clamp(opts.measure(e, r)));
  };
  const reset = () => { apply(opts.def); try { if (opts.key) localStorage.removeItem(opts.key); } catch { /* ignore */ } };
  node.addEventListener('pointerdown', down);
  node.addEventListener('pointermove', move);
  node.addEventListener('pointerup', up);
  node.addEventListener('pointercancel', up);
  node.addEventListener('dblclick', reset);
  if (!node.title) node.title = 'Drag to resize · double-click: back to the default';
  return {
    update(n: SplitOpts) { opts = n; },
    destroy() {
      node.removeEventListener('pointerdown', down);
      node.removeEventListener('pointermove', move);
      node.removeEventListener('pointerup', up);
      node.removeEventListener('pointercancel', up);
      node.removeEventListener('dblclick', reset);
    },
  };
}
