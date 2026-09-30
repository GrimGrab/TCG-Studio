// Svelte action: adds a "full view" button to a 3D view container. Maximised, the container covers the whole Studio window
// (fixed, on top of everything); the button or Esc brings it back. A same-size placeholder keeps the page layout (and the
// scroll positions of the scrolling parents are restored), so nothing below the view moves. Canvases inside resize through
// their own ResizeObservers.

export function maximizable(node: HTMLElement) {
  if (getComputedStyle(node).position === 'static') node.style.position = 'relative';
  const btn = document.createElement('button');
  btn.type = 'button';
  Object.assign(btn.style, { position: 'absolute', left: '8px', top: '8px', zIndex: '5', padding: '2px 8px', fontSize: '14px', lineHeight: '18px' });
  node.appendChild(btn);

  let on = false;
  let saved: string | null = null;
  let placeholder: HTMLElement | null = null;
  let scrolls: [Element, number, number][] = [];
  const label = () => {
    btn.textContent = on ? '✕' : '⛶';
    btn.title = on ? 'Back to the normal view (Esc)' : 'Full view — fill the whole window (Esc to leave)';
  };
  const scrollParents = () => {
    const out: [Element, number, number][] = [];
    for (let p = node.parentElement; p; p = p.parentElement) if (p.scrollHeight > p.clientHeight || p.scrollWidth > p.clientWidth) out.push([p, p.scrollTop, p.scrollLeft]);
    return out;
  };
  const set = (v: boolean) => {
    if (v === on) return;
    on = v;
    if (on) {
      scrolls = scrollParents();
      const r = node.getBoundingClientRect();
      placeholder = document.createElement('div');
      Object.assign(placeholder.style, { width: `${r.width}px`, height: `${r.height}px`, flex: getComputedStyle(node).flex });
      node.parentElement?.insertBefore(placeholder, node);
      saved = node.getAttribute('style');
      Object.assign(node.style, { position: 'fixed', inset: '0', zIndex: '1000', height: 'auto', width: 'auto', maxHeight: 'none', borderRadius: '0', margin: '0' });
    } else {
      if (saved === null) node.removeAttribute('style');
      else node.setAttribute('style', saved);
      placeholder?.remove();
      placeholder = null;
      for (const [el, top, left] of scrolls) { el.scrollTop = top; el.scrollLeft = left; }
      requestAnimationFrame(() => { for (const [el, top, left] of scrolls) { el.scrollTop = top; el.scrollLeft = left; } });
    }
    label();
  };
  const onClick = (e: MouseEvent) => { e.stopPropagation(); set(!on); };
  const onKey = (e: KeyboardEvent) => { if (on && e.key === 'Escape') { e.preventDefault(); set(false); } };
  btn.addEventListener('click', onClick);
  window.addEventListener('keydown', onKey);
  label();
  return {
    destroy() {
      window.removeEventListener('keydown', onKey);
      placeholder?.remove();
      btn.remove();
    },
  };
}
