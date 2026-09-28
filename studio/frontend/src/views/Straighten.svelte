<script lang="ts">
  // Straighten a photo taken at an angle: drag the four corner handles onto the corners of the flat surface (box front, pack
  // face…) and it is warped into a head-on rectangle. The aspect ratio is recovered from the perspective (or taken from a face).
  import { onMount } from 'svelte';
  import { type Pt, estimateAspect, outputSize, rectify } from '../lib/perspective';
  import { loadImage } from '../lib/accessoryArt';
  import { errText } from '../lib/api';

  let { url, points = null, faces = [], onapply, oncancel }: {
    url: string;                              // source image
    points?: number[] | null;                 // earlier corners [x,y × 4] in source pixels (tl, tr, br, bl)
    faces?: { label: string; aspect: number }[]; // "match face" choices
    onapply: (png: string, pts: number[], aspect: number) => void;
    oncancel: () => void;
  } = $props();

  let img = $state.raw<HTMLImageElement | null>(null);
  let error = $state('');
  let view = $state<HTMLCanvasElement>();
  let preview = $state<HTMLCanvasElement>();
  let q = $state<Pt[]>([]);
  let aspectMode = $state('auto');            // 'auto' | 'face:<i>' | 'custom'
  let customW = $state(3), customH = $state(2);
  let busy = $state(false);
  let drag = -1;
  let hover = $state(-1);
  let loupe: { x: number; y: number } | null = null;

  const autoAspect = $derived(img && q.length === 4 ? estimateAspect(q, img.width, img.height) : 1);
  const aspect = $derived(
    aspectMode === 'auto' ? autoAspect
      : aspectMode === 'custom' ? Math.max(0.01, customW) / Math.max(0.01, customH)
      : faces[+aspectMode.slice(5)]?.aspect ?? autoAspect);

  onMount(async () => {
    try {
      img = await loadImage(url);
      if (points && points.length === 8) q = [0, 2, 4, 6].map((i) => [points![i], points![i + 1]] as Pt);
      else {
        const w = img.width, h = img.height;
        q = [[w * 0.15, h * 0.15], [w * 0.85, h * 0.15], [w * 0.85, h * 0.85], [w * 0.15, h * 0.85]];
      }
      draw();
    } catch (e) { error = errText(e); }
  });

  // Redraw when the canvas gets its size (the first draw can run before the dialog is laid out) or is resized.
  $effect(() => {
    if (!view) return;
    const ro = new ResizeObserver(() => draw());
    ro.observe(view);
    return () => ro.disconnect();
  });

  // Display scale: the image fitted into the canvas element's box.
  function fit() {
    const c = view!, r = c.getBoundingClientRect();
    // Computed from the element's box, not the bitmap: resizing the bitmap clears it, so only draw() does that.
    const w = Math.round(r.width * devicePixelRatio), h = Math.round(r.height * devicePixelRatio);
    const s = Math.min(w / img!.width, h / img!.height);
    return { s, ox: (w - img!.width * s) / 2, oy: (h - img!.height * s) / 2, w, h };
  }

  function draw() {
    if (!view || !img || q.length !== 4) return;
    const { s, ox, oy, w, h } = fit();
    if (view.width !== w || view.height !== h) { view.width = w; view.height = h; }
    const ctx = view.getContext('2d')!;
    ctx.clearRect(0, 0, view.width, view.height);
    ctx.drawImage(img, ox, oy, img.width * s, img.height * s);
    const P = (p: Pt): Pt => [ox + p[0] * s, oy + p[1] * s];
    // Dim everything outside the quad.
    ctx.save();
    ctx.fillStyle = 'rgba(0,0,0,0.45)';
    ctx.beginPath();
    ctx.rect(0, 0, view.width, view.height);
    const pts = q.map(P);
    ctx.moveTo(pts[0][0], pts[0][1]);
    for (const p of [pts[3], pts[2], pts[1]]) ctx.lineTo(p[0], p[1]);
    ctx.closePath();
    ctx.fill('evenodd');
    ctx.restore();
    const lw = 2 * devicePixelRatio;
    ctx.strokeStyle = '#4fa3ff'; ctx.lineWidth = lw;
    ctx.beginPath();
    pts.forEach((p, i) => (i ? ctx.lineTo(p[0], p[1]) : ctx.moveTo(p[0], p[1])));
    ctx.closePath(); ctx.stroke();
    // Thirds grid inside the quad (bilinear, good enough as a guide).
    ctx.strokeStyle = 'rgba(79,163,255,0.45)'; ctx.lineWidth = lw / 2;
    const lerp = (a: Pt, b: Pt, t: number): Pt => [a[0] + (b[0] - a[0]) * t, a[1] + (b[1] - a[1]) * t];
    for (const t of [1 / 3, 2 / 3]) {
      let a = lerp(pts[0], pts[1], t), b = lerp(pts[3], pts[2], t);
      ctx.beginPath(); ctx.moveTo(a[0], a[1]); ctx.lineTo(b[0], b[1]); ctx.stroke();
      a = lerp(pts[0], pts[3], t); b = lerp(pts[1], pts[2], t);
      ctx.beginPath(); ctx.moveTo(a[0], a[1]); ctx.lineTo(b[0], b[1]); ctx.stroke();
    }
    const labels = ['TL', 'TR', 'BR', 'BL'];
    pts.forEach((p, i) => {
      ctx.fillStyle = i === hover || i === drag ? '#ffd24a' : '#4fa3ff';
      ctx.beginPath(); ctx.arc(p[0], p[1], 7 * devicePixelRatio, 0, Math.PI * 2); ctx.fill();
      ctx.fillStyle = '#fff'; ctx.font = `bold ${11 * devicePixelRatio}px sans-serif`;
      ctx.fillText(labels[i], p[0] + 10 * devicePixelRatio, p[1] - 8 * devicePixelRatio);
    });
    // Loupe while dragging: 4× around the handle, in the corner away from it.
    if (drag >= 0 && loupe) {
      const R = 70 * devicePixelRatio, z = 4;
      const cx = loupe.x < view.width / 2 ? view.width - R - 10 : R + 10, cy = R + 10;
      const src = q[drag];
      ctx.save();
      ctx.beginPath(); ctx.arc(cx, cy, R, 0, Math.PI * 2); ctx.clip();
      ctx.fillStyle = '#111'; ctx.fillRect(cx - R, cy - R, 2 * R, 2 * R);
      const k = s * z;
      ctx.drawImage(img, cx - src[0] * k, cy - src[1] * k, img.width * k, img.height * k);
      ctx.strokeStyle = '#ffd24a'; ctx.lineWidth = 1 * devicePixelRatio;
      ctx.beginPath(); ctx.moveTo(cx - R, cy); ctx.lineTo(cx + R, cy); ctx.moveTo(cx, cy - R); ctx.lineTo(cx, cy + R); ctx.stroke();
      ctx.restore();
      ctx.strokeStyle = '#fff'; ctx.lineWidth = 2 * devicePixelRatio;
      ctx.beginPath(); ctx.arc(cx, cy, R, 0, Math.PI * 2); ctx.stroke();
    }
    schedulePreview();
  }

  let pv = 0;
  function schedulePreview() {
    cancelAnimationFrame(pv);
    pv = requestAnimationFrame(() => {
      if (!preview || !img || q.length !== 4) return;
      try {
        const [W, H] = outputSize(q, aspect, 260);
        const r = rectify(img, q, W, H);
        preview.width = W; preview.height = H;
        preview.getContext('2d')!.drawImage(r, 0, 0);
        error = '';
      } catch (e) { error = errText(e); }
    });
  }

  function toImg(e: PointerEvent): Pt {
    const { s, ox, oy } = fit();
    const r = view!.getBoundingClientRect();
    const x = (e.clientX - r.left) * devicePixelRatio, y = (e.clientY - r.top) * devicePixelRatio;
    return [(x - ox) / s, (y - oy) / s];
  }

  function nearest(p: Pt): number {
    const { s } = fit();
    let best = -1, bd = (18 * devicePixelRatio) / s;
    q.forEach((c, i) => { const d = Math.hypot(c[0] - p[0], c[1] - p[1]); if (d < bd) { bd = d; best = i; } });
    return best;
  }

  function down(e: PointerEvent) {
    if (!img) return;
    const p = toImg(e);
    drag = nearest(p);
    if (drag < 0) {
      // Click anywhere: move the closest corner there.
      let bd = Infinity;
      q.forEach((c, i) => { const d = Math.hypot(c[0] - p[0], c[1] - p[1]); if (d < bd) { bd = d; drag = i; } });
      q[drag] = clamp(p);
    }
    view!.setPointerCapture(e.pointerId);
    move(e);
  }
  const clamp = (p: Pt): Pt => [Math.max(0, Math.min(img!.width, p[0])), Math.max(0, Math.min(img!.height, p[1]))];
  function move(e: PointerEvent) {
    if (!img) return;
    const p = toImg(e);
    if (drag >= 0) {
      q[drag] = clamp(p);
      const r = view!.getBoundingClientRect();
      loupe = { x: (e.clientX - r.left) * devicePixelRatio, y: (e.clientY - r.top) * devicePixelRatio };
    } else {
      const h = nearest(p);
      if (h === hover) return;
      hover = h;
    }
    draw();
  }
  function up() { drag = -1; loupe = null; draw(); }

  // Arrow keys nudge the hovered/last corner by one source pixel (Shift: 10).
  let last = 0;
  $effect(() => { if (hover >= 0) last = hover; });
  function key(e: KeyboardEvent) {
    const d = e.shiftKey ? 10 : 1;
    const m: Record<string, Pt> = { ArrowLeft: [-d, 0], ArrowRight: [d, 0], ArrowUp: [0, -d], ArrowDown: [0, d] };
    if (e.key === 'Escape') { oncancel(); return; }
    if (!m[e.key] || !img) return;
    e.preventDefault();
    q[last] = clamp([q[last][0] + m[e.key][0], q[last][1] + m[e.key][1]]);
    draw();
  }

  $effect(() => { aspect; schedulePreview(); });

  async function apply() {
    if (!img) return;
    busy = true;
    try {
      await new Promise((r) => setTimeout(r, 20)); // let "Straightening…" paint
      const [W, H] = outputSize(q, aspect);
      const out = rectify(img, q, W, H);
      onapply(out.toDataURL('image/png'), q.flat(), aspect);
    } catch (e) { error = errText(e); }
    busy = false;
  }
</script>

<svelte:window onkeydown={key} onresize={draw} />

<div class="backdrop" role="presentation">
  <div class="dialog">
    <div class="row head">
      <h3 class="grow">Straighten image</h3>
      <button onclick={oncancel}>Cancel</button>
      <button class="primary" disabled={busy || !!error} onclick={apply}>{busy ? 'Straightening…' : 'Apply'}</button>
    </div>
    <div class="body">
      <canvas class="src" bind:this={view} onpointerdown={down} onpointermove={move} onpointerup={up}></canvas>
      <div class="side">
        <p class="small">Drag the four corners onto the corners of the flat surface you want (e.g. the front of the box). Click
          anywhere to move the nearest corner there; arrow keys nudge the last corner (Shift = 10 px).</p>
        <label class="field">Shape
          <select bind:value={aspectMode}>
            <option value="auto">Automatic from perspective ({autoAspect.toFixed(2)} : 1)</option>
            {#each faces as f, i}<option value={'face:' + i}>Match {f.label} ({f.aspect.toFixed(2)} : 1)</option>{/each}
            <option value="custom">Custom ratio</option>
          </select>
        </label>
        {#if aspectMode === 'custom'}
          <div class="row">
            <label class="field">Width<input type="number" min="0.1" step="0.1" bind:value={customW} /></label>
            <label class="field">Height<input type="number" min="0.1" step="0.1" bind:value={customH} /></label>
          </div>
        {/if}
        <div class="small muted">Result</div>
        <div class="pv"><canvas bind:this={preview}></canvas></div>
        {#if error}<p class="err small">{error}</p>{/if}
        <p class="small muted">Automatic works best on uncropped photos; if the result looks stretched, match it to the face it goes on.</p>
      </div>
    </div>
  </div>
</div>

<style>
  .backdrop { position: fixed; inset: 0; background: rgba(0, 0, 0, 0.6); display: flex; align-items: center; justify-content: center; z-index: 100; }
  .dialog { width: min(1200px, 94vw); height: min(820px, 90vh); background: var(--panel); border: 1px solid var(--line); border-radius: var(--radius);
    padding: 12px; display: flex; flex-direction: column; gap: 10px; }
  .head h3 { margin: 0; }
  .body { flex: 1; min-height: 0; display: flex; gap: 12px; }
  .src { flex: 1; min-width: 0; height: 100%; background: repeating-conic-gradient(#1b1f27 0 25%, #232834 0 50%) 0 0 / 24px 24px; border-radius: 6px;
    cursor: crosshair; touch-action: none; }
  .side { width: 280px; flex-shrink: 0; display: flex; flex-direction: column; gap: 10px; overflow: auto; }
  .pv { background: repeating-conic-gradient(#1b1f27 0 25%, #232834 0 50%) 0 0 / 16px 16px; border-radius: 6px; padding: 8px; display: flex; justify-content: center; }
  .pv canvas { max-width: 100%; max-height: 260px; }
  .small { font-size: 12px; margin: 0; }
  .err { color: var(--danger); }
</style>
