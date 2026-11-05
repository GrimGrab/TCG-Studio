<script lang="ts">
  // Debug panel for face maps (uvmap/facemap_*.go): the pieces are dragged on the texture view (AccessoryEditor); this lists
  // them, shows the numbers and copies the rows for the table. Edits only this editor's model in memory; nothing is stored,
  // reopening the editor goes back to the table.
  import type { Model, Target, Face } from '../../lib/accessoryArt';
  import { targetLabel, moveTarget, trueScale } from '../../lib/accessoryArt';
  import { ClipboardSetText } from '../../../wailsjs/runtime/runtime';

  let { model, picked = $bindable(null), pattern = $bindable(true), adding = $bindable(''), hold = $bindable(false), rev = 0, faceImage, onchange, notify, canUndo, canRedo, canPaste, onundo, onredo, oncopy, onpaste }: {
    model: Model; picked?: Target | null; pattern?: boolean; adding?: string; hold?: boolean; rev?: number;
    faceImage: (f: Face) => { canvas: HTMLCanvasElement; x: number; y: number; w: number; h: number }; onchange: () => void; notify: (msg: string, kind?: any) => void;
    canUndo: boolean; canRedo: boolean; canPaste: boolean; onundo: () => void; onredo: () => void; oncopy: () => void; onpaste: () => void;
  } = $props();

  let addTo = $state(''); // the face a new piece joins
  let newName = $state(''); // the name of a new face
  /** The next drag on the texture draws the new face's first piece (the face is made then). */
  function newFace() {
    const name = newName.trim();
    if (!name) return;
    if (model.faces.some((f) => f.id === name || f.label === name)) { notify('A face already has that name', 'error'); return; }
    adding = name;
    newName = '';
  }
  const faceOf = (t: Target) => model.faces.find((f) => f.targets.includes(t));
  /** The picked piece out of the layout. */
  function removePiece() {
    const f = picked && faceOf(picked);
    if (!f || !picked) return;
    f.targets.splice(f.targets.indexOf(picked), 1);
    picked = null;
    onchange();
  }
  // ---- which part of its face the picked piece shows: a picture of the face with a box on it

  let shown = $state<HTMLCanvasElement>();
  const round3 = (v: number) => Math.round(v * 1000) / 1000;
  /** A new "shows" area, kept inside the face. */
  function setSrc(u0: number, v0: number, u1: number, v1: number) {
    if (!picked) return;
    const w = Math.min(1, u1 - u0), h = Math.min(1, v1 - v0);
    const x = Math.max(0, Math.min(1 - w, u0)), y = Math.max(0, Math.min(1 - h, v0));
    picked.src = [round3(x), round3(y), round3(x + w), round3(y + h)];
    onchange();
  }
  /** At true scale, around its middle, or against one edge of the face. */
  function place(at: string) {
    const f = picked && faceOf(picked);
    if (!picked || !f) return;
    if (at === 'whole') return setSrc(0, 0, 1, 1);
    const [w, h] = trueScale(f, picked), s = picked.src;
    const cx = (s[0] + s[2]) / 2, cy = (s[1] + s[3]) / 2;
    if (at === 'scale') setSrc(cx - w / 2, cy - h / 2, cx + w / 2, cy + h / 2);
    if (at === 'left') setSrc(0, cy - h / 2, w, cy + h / 2);
    if (at === 'right') setSrc(1 - w, cy - h / 2, 1, cy + h / 2);
    if (at === 'top') setSrc(cx - w / 2, 0, cx + w / 2, h);
    if (at === 'bottom') setSrc(cx - w / 2, 1 - h, cx + w / 2, 1);
  }
  function drawShown() {
    const f = picked && faceOf(picked);
    if (!shown || !f || !picked) return;
    const im = faceImage(f);
    const W = shown.clientWidth || 260;
    shown.width = W;
    shown.height = Math.max(40, Math.min(260, Math.round((W * im.h) / Math.max(1, im.w))));
    const ctx = shown.getContext('2d')!, H = shown.height;
    ctx.drawImage(im.canvas, im.x, im.y, im.w, im.h, 0, 0, W, H);
    const [u0, v0, u1, v1] = picked.src, x = u0 * W, y = v0 * H, w = (u1 - u0) * W, h = (v1 - v0) * H;
    ctx.fillStyle = 'rgba(0,0,0,0.6)';
    ctx.fillRect(0, 0, W, y); ctx.fillRect(0, y + h, W, H - y - h); ctx.fillRect(0, y, x, h); ctx.fillRect(x + w, y, W - x - w, h);
    ctx.strokeStyle = '#ff4fd8'; ctx.lineWidth = 2;
    ctx.strokeRect(x + 1, y + 1, Math.max(0, w - 2), Math.max(0, h - 2));
    ctx.fillStyle = '#ff4fd8';
    for (const [cx, cy] of [[x, y], [x + w, y], [x, y + h], [x + w, y + h]]) ctx.fillRect(cx - 4, cy - 4, 8, 8);
  }
  $effect(() => { void rev; void picked; void picked?.src; drawShown(); });

  let boxDrag: { corner: number[] | null; px: number; py: number; src: number[] } | null = null;
  function boxDown(e: PointerEvent) {
    if (!picked || !shown) return;
    const r = shown.getBoundingClientRect(), s = picked.src;
    const px = (e.clientX - r.left) / r.width, py = (e.clientY - r.top) / r.height;
    const nearX = 9 / r.width, nearY = 9 / r.height;
    const corner = [[0, 1], [2, 1], [0, 3], [2, 3]].find(([i, j]) => Math.abs(px - s[i]) < nearX && Math.abs(py - s[j]) < nearY) ?? null;
    boxDrag = { corner, px, py, src: [...s] };
    hold = true;
    shown.setPointerCapture(e.pointerId);
  }
  function boxMove(e: PointerEvent) {
    if (!boxDrag || !shown) return;
    const r = shown.getBoundingClientRect(), s = [...boxDrag.src];
    const dx = (e.clientX - r.left) / r.width - boxDrag.px, dy = (e.clientY - r.top) / r.height - boxDrag.py;
    if (boxDrag.corner) {
      const [i, j] = boxDrag.corner;
      s[i] = Math.max(0, Math.min(1, s[i] + dx)); s[j] = Math.max(0, Math.min(1, s[j] + dy));
      if (s[2] - s[0] < 0.005 || s[3] - s[1] < 0.005) return;
      setSrc(s[0], s[1], s[2], s[3]);
    } else setSrc(s[0] + dx, s[1] + dy, s[2] + dx, s[3] + dy);
  }
  function boxUp() {
    if (!boxDrag) return;
    boxDrag = null;
    hold = false;
    onchange();
  }

  // svelte-ignore state_referenced_locally
  const original: Face[] = JSON.parse(JSON.stringify(model.faces));
  const SIDES = ['Left', 'Top', 'Right', 'Bottom'], SHOW = ['left', 'top', 'right', 'bottom'];

  function setRect(t: Target, i: number, v: number) {
    if (!Number.isFinite(v)) return;
    const r = [...t.rect];
    r[i] = v;
    moveTarget(t, r);
    onchange();
  }
  function setShow(t: Target, i: number, v: number) {
    if (Number.isFinite(v)) { t.src[i] = v; onchange(); }
  }
  const flipOf = (t: Target) => (t.flipX ? 'x' : '') + (t.flipY ? 'y' : '');
  function setFlip(t: Target, v: string) {
    t.flipX = v.includes('x'); t.flipY = v.includes('y');
    onchange();
  }
  const shapeText = (t: Target) => (t.shape ?? []).map(([x, y]) => `${x},${y}`).join('  ');
  /** Outline typed as "x,y  x,y  …"; ignored until it parses to at least three points, empty = the whole rectangle. */
  function setShape(t: Target, text: string) {
    if (!text.trim()) { delete t.shape; onchange(); return; }
    const pts = text.trim().split(/\s+/).map((p) => p.split(',').map(Number));
    if (pts.length < 3 || pts.some((p) => p.length !== 2 || p.some((v) => !Number.isFinite(v)))) return;
    t.shape = pts;
    onchange();
  }

  /** The picked piece back to the face map's values. */
  function resetPiece() {
    if (!picked) return;
    for (let i = 0; i < model.faces.length; i++) {
      const j = model.faces[i].targets.indexOf(picked);
      if (j >= 0) Object.assign(picked, JSON.parse(JSON.stringify(original[i].targets[j])));
      if (j >= 0 && !original[i].targets[j].shape) delete picked.shape;
    }
    onchange();
  }
  function resetAll() {
    model.faces = JSON.parse(JSON.stringify(original));
    picked = null;
    onchange();
  }

  /** The pieces as rows of the face map (uvmap/facemap_*.go). */
  const rows = $derived(model.faces.flatMap((f: Face) => f.targets.map((t) => {
    const part = t.part ? ` Part: "${t.part}",` : '';
    const show = t.src.join() === '0,0,1,1' ? '' : `, Show: [4]float64{${t.src.join(', ')}}`;
    const flip = (flipOf(t) ? `, Flip: "${flipOf(t)}"` : '') + (t.transpose ? ', Turn: true' : '');
    const shape = t.shape ? `, Shape: [][2]float64{${t.shape.map(([x, y]) => `{${x}, ${y}}`).join(', ')}}` : '';
    return `\t{Face: "${f.id}",${part} Rect: [4]float64{${t.rect.join(', ')}}${show}${flip}${shape}},`;
  })).join('\n'));

  async function copy() {
    try { await ClipboardSetText(rows); notify('Face map rows copied'); } catch { notify("Couldn't reach the clipboard: copy them from the box", 'error'); }
  }
</script>

<div class="tuner">
  <p class="how"><b>Click a box on the texture</b> to pick it. <b>Drag</b> to move, <b>drag a square</b> to resize, <b>arrow keys</b> nudge 1 px (Shift: 10).</p>

  <div class="seg">
    <button class:on={pattern} onclick={() => { pattern = true; onchange(); }}>Test pattern</button>
    <button class:on={!pattern} onclick={() => { pattern = false; onchange(); }}>My art</button>
  </div>

  <div class="list">
    {#each model.faces as f}
      {#each f.targets as t}
        <button class="pick" class:on={picked === t} onclick={() => { picked = t; onchange(); }}>{targetLabel(f, t)}</button>
      {/each}
    {/each}
  </div>

  <div class="actions">
    <button class="small" disabled={!canUndo} title="Ctrl+Z" onclick={onundo}>Undo</button>
    <button class="small" disabled={!canRedo} title="Ctrl+Y" onclick={onredo}>Redo</button>
    <button class="small" disabled={!picked} title="Ctrl+C" onclick={oncopy}>Copy</button>
    <button class="small" disabled={!canPaste} title="Ctrl+V: into the picked piece's face" onclick={onpaste}>Paste</button>
  </div>

  <div class="add">
    <select bind:value={addTo}>
      <option value="">Add a piece to…</option>
      {#each model.faces as f}<option value={f.id}>{f.label}</option>{/each}
    </select>
    <button class="small" class:primary={adding} disabled={!addTo} onclick={() => (adding = adding ? '' : addTo)}>{adding ? 'Now drag on the texture' : 'Draw it'}</button>
  </div>
  <div class="add">
    <input type="text" placeholder="New face name" bind:value={newName} onkeydown={(e) => { if (e.key === 'Enter') newFace(); }} />
    <button class="small" disabled={!newName.trim()} onclick={newFace}>New face</button>
  </div>

  {#if picked}
    {@const t = picked}
    <div class="nums">
      {#each t.rect as v, i}
        <label>{SIDES[i]}<input type="number" step="1" value={v} oninput={(e) => setRect(t, i, e.currentTarget.valueAsNumber)} /></label>
      {/each}
    </div>
    <div class="size muted small">{t.rect[2] - t.rect[0]} × {t.rect[3] - t.rect[1]} px</div>
    <div class="shows">
      <b>Shows this part of the {faceOf(t)?.label} art</b>
      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <canvas bind:this={shown} onpointerdown={boxDown} onpointermove={boxMove} onpointerup={boxUp} onpointercancel={boxUp}></canvas>
      <div class="presets">
        <button class="small" title="The art at the same size as on the face's main piece" onclick={() => place('scale')}>True scale</button>
        <button class="small" onclick={() => place('whole')}>Whole face</button>
        <button class="small" onclick={() => place('top')}>Top edge</button>
        <button class="small" onclick={() => place('bottom')}>Bottom edge</button>
        <button class="small" onclick={() => place('left')}>Left edge</button>
        <button class="small" onclick={() => place('right')}>Right edge</button>
      </div>
    </div>
    <details>
      <summary>More (exact numbers, flip, outline)</summary>
      <div class="nums">
        {#each t.src as v, i}
          <label>{SHOW[i]}<input type="number" step="0.001" min="0" max="1" value={v} oninput={(e) => setShow(t, i, e.currentTarget.valueAsNumber)} /></label>
        {/each}
      </div>
      <label class="flip"><input type="checkbox" style="width:auto" checked={!!t.transpose} onchange={(e) => { t.transpose = e.currentTarget.checked; onchange(); }} /> Turned on its side</label>
      <label class="flip">Flip
        <select value={flipOf(t)} onchange={(e) => setFlip(t, e.currentTarget.value)}>
          <option value="">none</option><option value="x">x</option><option value="y">y</option><option value="xy">xy</option>
        </select>
      </label>
      {#if t.shape}
        <label>Outline (x,y points)<textarea rows="3" value={shapeText(t)} onchange={(e) => setShape(t, e.currentTarget.value)}></textarea></label>
      {/if}
    </details>
    <div class="actions">
      <button class="small" onclick={resetPiece}>Reset this piece</button>
      <button class="small" onclick={removePiece}>Remove it</button>
    </div>
  {/if}

  <div class="actions">
    <button class="small primary" onclick={copy}>Copy rows</button>
    <button class="small" onclick={resetAll}>Reset all</button>
  </div>
  <details>
    <summary>Rows to paste</summary>
    <textarea class="out" rows="6" readonly value={rows}></textarea>
  </details>
</div>

<style>
  .tuner { display: flex; flex-direction: column; gap: 7px; padding: 8px; border: 1px dashed var(--accent); border-radius: 6px; }
  .how { margin: 0; font-size: 12px; }
  .seg { display: flex; }
  .seg button { flex: 1; border-radius: 0; }
  .seg button.on, .pick.on { background: #ff4fd8; color: #111; border-color: #ff4fd8; }
  .list { display: flex; flex-wrap: wrap; gap: 3px; }
  .pick { font-size: 11px; padding: 2px 6px; }
  .nums { display: grid; grid-template-columns: repeat(4, 1fr); gap: 3px; }
  .size { text-align: right; }
  label { font-size: 11px; display: flex; flex-direction: column; }
  .flip { flex-direction: row; align-items: center; gap: 5px; margin: 4px 0; }
  input, select, textarea { font-size: 12px; width: 100%; min-width: 0; box-sizing: border-box; padding: 2px 3px; }
  summary { font-size: 11px; cursor: pointer; }
  .actions, .add { display: flex; gap: 6px; }
  .shows { display: flex; flex-direction: column; gap: 4px; font-size: 11px; }
  .shows canvas { width: 100%; display: block; cursor: move; touch-action: none; background: #222; }
  .presets { display: flex; flex-wrap: wrap; gap: 3px; }
  .out { font-family: monospace; white-space: pre; overflow: auto; margin-top: 4px; }
</style>
