<script lang="ts" module>
  /** Editor state of a figurine (kept in the library's studio.json layouts). */
  export interface FigLayout {
    version: 'fig1';
    model: string;       // library-relative source OBJ (right-handed, Y up), '' = none imported yet
    texture: string;     // library-relative texture (the import's, or a replacement)
    sourceName: string;  // imported file name
    rotX: number; rotY: number; rotZ: number; // degrees, applied Z, X, Y
    height: number;      // mesh units (× cmPerUnit = cm in game)
    triangles: number;
    warnings: string[];
  }
  export function newFigLayout(): FigLayout {
    return { version: 'fig1', model: '', texture: '', sourceName: '', rotX: 0, rotY: 0, rotZ: 0, height: 0, triangles: 0, warnings: [] };
  }
</script>

<script lang="ts">
  // Figurine editor: import a model (.glb/.gltf/.obj), turn and scale it, and check it against its base toy on a real shelf
  // (the mod's shelf export). Saving bakes it in Go (BakeFigurine) — the same placement maths as placement() below.
  import { onMount, untrack } from 'svelte';
  import { App, errText } from '../lib/api';
  import { loadImage } from '../lib/accessoryArt';
  import {
    FigurineView, loadGeom, type Geom, type Mat, type Box, type SceneObj, ident, mul, translate, scale, rotX, rotY, rotZ,
    MIRROR_Z, transformBox, boxLines,
  } from '../lib/figurineView';
  import { type ShelfTpl, type ItemTpl, slotMatrices, meshMatrix, cmPerUnit, fitCheck, boxOf, type Fit } from '../lib/figurineShelf';

  let {
    base, templates, layout = $bindable(), notify, onchange, onimported,
  }: {
    base: string;              // vanilla toy (slot size, shop tab, prices)
    templates: any;            // mod export (accessories.json), null when missing
    layout: FigLayout;
    notify: (t: string, k?: string) => void;
    onchange: () => void;
    onimported?: (name: string) => void;
  } = $props();

  const accUrl = (rel: string) => `/acc/${rel.split('/').map(encodeURIComponent).join('/')}`;
  const tplUrl = (file: string) => `/acctemplates/${encodeURIComponent(file)}`;

  let canvas = $state<HTMLCanvasElement>();
  let view: FigurineView | null = null;
  let mode = $state<'model' | 'shelf'>('model');
  let shelfName = $state('');
  let compare = $state(true);
  let busy = $state(false);
  let fit = $state<Fit | null>(null);

  let figGeom: Geom | null = null;
  let baseGeom: Geom | null = null;
  let shelfGeom: Geom | null = null;
  let loadedModel = '', loadedTexture = '', loadedBase = '', loadedShelf = '';

  let item = $derived<ItemTpl | undefined>(templates?.items?.find((i: any) => i.type === base));
  let prefab = $derived(templates?.itemPrefab);
  let cmU = $derived(cmPerUnit(prefab));
  let shelves = $derived<ShelfTpl[]>((templates?.shelves ?? []).filter((s: ShelfTpl) => s.mesh && s.compartments.some((c) => c.m_CanPutItem)));
  let shelf = $derived(shelves.find((s) => s.name === shelfName) ?? shelves[0]);
  let baseHeight = $derived(item?.bounds?.size[1] ?? 1.3);

  // ---------------------------------------------------------------- placement (mirrors figurine.Place in Go)

  /** Source → base toy mesh space, in GL space (Unity with z mirrored). Also the placed box (GL). */
  function placement(): { m: Mat; box: Box } | null {
    if (!figGeom) return null;
    const R = mul(rotY(layout.rotY), mul(rotX(layout.rotX), rotZ(layout.rotZ)));
    const d = figGeom.data, lo = [Infinity, Infinity, Infinity], hi = [-Infinity, -Infinity, -Infinity];
    for (let i = 0; i < d.length; i += 8) {
      const x = d[i], y = d[i + 1], z = d[i + 2];
      for (let q = 0; q < 3; q++) {
        const v = R[q] * x + R[4 + q] * y + R[8 + q] * z;
        if (v < lo[q]) lo[q] = v;
        if (v > hi[q]) hi[q] = v;
      }
    }
    const h = hi[1] - lo[1];
    if (!(h > 0)) return null;
    const s = (layout.height || baseHeight) / h;
    const c = [(lo[0] + hi[0]) / 2, lo[1], (lo[2] + hi[2]) / 2];
    const a = anchorGL();
    const m = mul(translate(a[0], a[1], a[2]), mul(scale(s), mul(translate(-c[0], -c[1], -c[2]), R)));
    const box = {
      min: [a[0] + (lo[0] - c[0]) * s, a[1], a[2] + (lo[2] - c[2]) * s],
      max: [a[0] + (hi[0] - c[0]) * s, a[1] + h * s, a[2] + (hi[2] - c[2]) * s],
    };
    return { m, box };
  }

  /** Footprint centre and bottom of the base toy (Unity mesh space). */
  function anchorUnity(): [number, number, number] {
    const b = item?.bounds;
    if (!b) return [0, 0, 0];
    return [b.center[0], b.center[1] - b.size[1] / 2, b.center[2]];
  }
  function anchorGL(): number[] { const a = anchorUnity(); return [a[0], a[1], -a[2]]; }
  const mirrorBox = (b: Box): Box => ({ min: [b.min[0], b.min[1], -b.max[2]], max: [b.max[0], b.max[1], -b.min[2]] });

  /** Placement for BakeFigurine. */
  export function bakeParams() {
    return { rotX: layout.rotX, rotY: layout.rotY, rotZ: layout.rotZ, height: layout.height || baseHeight, anchor: anchorUnity() };
  }

  // ---------------------------------------------------------------- loading

  async function ensureLoaded() {
    if (!view) return;
    if (layout.model && layout.model !== loadedModel) {
      figGeom = await loadGeom(accUrl(layout.model), false);
      loadedModel = layout.model;
    }
    if (!layout.model) { figGeom = null; loadedModel = ''; }
    if (layout.texture && layout.texture !== loadedTexture) {
      view.setTexture('fig', await loadImage(accUrl(layout.texture)));
      loadedTexture = layout.texture;
    }
    if (item?.mesh && base !== loadedBase) {
      baseGeom = await loadGeom(tplUrl(item.mesh), true);
      if (item.texture) view.setTexture('vanilla', await loadImage(tplUrl(item.texture)));
      loadedBase = base;
    }
    if (mode === 'shelf' && shelf?.mesh && shelf.name !== loadedShelf) {
      busy = true;
      try { shelfGeom = await loadGeom(tplUrl(shelf.mesh), true); } finally { busy = false; }
      loadedShelf = shelf.name;
      frameShelf();
    }
  }

  let framedFor = '';
  async function rebuild(reframe = false) {
    if (!view) return;
    try { await ensureLoaded(); } catch (e) { notify(errText(e), 'error'); return; }
    const p = placement();
    const figBoxU = p ? mirrorBox(p.box) : null;
    fit = shelf && item && figBoxU ? fitCheck(shelf, shelf.compartments.find((c) => c.m_CanPutItem)!, item, prefab, figBoxU) : null;
    const objs: SceneObj[] = [];
    const lines: { points: number[]; color: [number, number, number, number] }[] = [];
    if (mode === 'model') {
      if (p && figGeom) objs.push({ geom: figGeom, matrix: p.m, texture: layout.texture ? 'fig' : undefined });
      // The vanilla toy next to it, same floor.
      if (baseGeom && item?.bounds) {
        const bw = item.bounds.size[0], fw = p ? p.box.max[0] - p.box.min[0] : 0;
        const dx = -(bw / 2 + fw / 2) * 1.2 - 0.05;
        objs.push({ geom: baseGeom, matrix: translate(dx, 0, 0), texture: 'vanilla', color: [1, 1, 1, 1] });
        if (p) lines.push({ points: boxLines(ident(), p.box), color: [0.35, 0.6, 1, 1] });
      }
      const a = anchorGL(), r = Math.max(baseHeight, layout.height || 0) * 1.6;
      const floor: number[] = [];
      for (let i = -4; i <= 4; i++) {
        const t = (i / 4) * r;
        floor.push(a[0] - r, a[1], a[2] + t, a[0] + r, a[1], a[2] + t, a[0] + t, a[1], a[2] - r, a[0] + t, a[1], a[2] + r);
      }
      lines.push({ points: floor, color: [0.35, 0.4, 0.5, 1] });
      const key = `model|${layout.model}`;
      if (reframe || framedFor !== key) {
        framedFor = key;
        const boxes = objs.map((o) => transformBox(o.matrix, { min: o.geom.min, max: o.geom.max }));
        if (boxes.length) view.frameBox({ min: [0, 1, 2].map((q) => Math.min(...boxes.map((b) => b.min[q]))), max: [0, 1, 2].map((q) => Math.max(...boxes.map((b) => b.max[q]))) }, 0.75);
      }
    } else if (shelf && item) {
      if (shelfGeom) objs.push({ geom: shelfGeom, matrix: ident(), color: [0.62, 0.66, 0.74, 1] });
      const mm = meshMatrix(prefab);
      let ci = 0;
      for (const c of shelf.compartments) {
        if (!c.m_CanPutItem) continue;
        const vanillaHere = compare && ci % 2 === 1;
        ci++;
        for (const slot of slotMatrices(c, item)) {
          const m = mul(MIRROR_Z, mul(slot, mul(mm, MIRROR_Z)));
          if (vanillaHere) { if (baseGeom) objs.push({ geom: baseGeom, matrix: m, texture: 'vanilla' }); }
          else if (p && figGeom) objs.push({ geom: figGeom, matrix: mul(m, p.m), texture: layout.texture ? 'fig' : undefined });
        }
      }
    }
    view.setScene(objs);
    view.setLines(lines);
  }

  function frameShelf() {
    if (!view || !shelfGeom) return;
    view.yaw = 20; view.pitch = 12;
    view.frameBox({ min: shelfGeom.min, max: shelfGeom.max }, 0.95);
  }

  // Rebuild on any relevant change.
  $effect(() => {
    void [layout.model, layout.texture, layout.rotX, layout.rotY, layout.rotZ, layout.height, base, mode, shelfName, compare, templates];
    untrack(() => rebuild());
  });

  onMount(() => {
    if (!canvas) return;
    try { view = new FigurineView(canvas); } catch (e) { notify(errText(e), 'error'); return; }
    const ro = new ResizeObserver(() => view?.draw());
    ro.observe(canvas);
    rebuild(true);
    return () => { ro.disconnect(); view?.dispose(); view = null; };
  });

  // ---------------------------------------------------------------- actions

  function changed() { onchange(); }

  async function importModel() {
    busy = true;
    try {
      const r = await App.ImportFigurineModel();
      if (!r.model) return;
      layout.model = r.model;
      layout.texture = r.texture;
      layout.sourceName = r.name;
      layout.triangles = r.triangles;
      layout.warnings = r.warnings ?? [];
      layout.rotX = 0; layout.rotY = 0; layout.rotZ = 0;
      layout.height = baseHeight;
      framedFor = '';
      changed();
      onimported?.(r.name);
      notify(`Imported ${r.name}: ${r.triangles.toLocaleString()} triangles.`, r.warnings?.length ? 'warn' : 'ok');
    } catch (e) { notify(errText(e), 'error'); } finally { busy = false; }
  }

  async function replaceTexture() {
    try {
      const rel = await App.PickAccessoryImage();
      if (!rel) return;
      layout.texture = rel;
      changed();
    } catch (e) { notify(errText(e), 'error'); }
  }

  function turn(axis: 'rotX' | 'rotY' | 'rotZ', deg: number) {
    layout[axis] = ((((layout[axis] + deg) % 360) + 540) % 360) - 180;
    changed();
  }

  function setHeightCm(v: number) {
    if (!(v > 0)) return;
    layout.height = v / cmU;
    changed();
  }

  /** Shop icon (PNG data URL): the figurine alone, three-quarter front view like the vanilla icons. */
  export async function exportIcon(size = [512, 512]): Promise<string> {
    await rebuild();
    const p = placement();
    if (!p || !figGeom) throw new Error('import a model first');
    const c = document.createElement('canvas');
    c.width = size[0]; c.height = size[1];
    const v = new FigurineView(c, { preserve: true, interactive: false });
    try {
      if (layout.texture) v.setTexture('fig', await loadImage(accUrl(layout.texture)));
      v.setScene([{ geom: figGeom, matrix: p.m, texture: layout.texture ? 'fig' : undefined }]);
      v.yaw = 30; v.pitch = 15; v.fov = 0.35;
      v.frameBox(p.box, 0.9);
      v.renderNow();
      return c.toDataURL('image/png');
    } finally { v.dispose(); }
  }

  const cm = (m: number) => (m * 100).toFixed(1);
</script>

<div class="fig">
  <div class="stage">
    <div class="row wrap">
      <button class="primary" onclick={importModel} disabled={busy}>{layout.model ? 'Replace model…' : 'Import model…'}</button>
      {#if layout.model}<button onclick={replaceTexture} title="Use another image as the texture (same UV layout as the model's own)">Replace texture…</button>{/if}
      <div class="grow"></div>
      <div class="seg">
        <button class:on={mode === 'model'} onclick={() => (mode = 'model')}>Model</button>
        <button class:on={mode === 'shelf'} onclick={() => (mode = 'shelf')} disabled={!shelves.length} title={shelves.length ? '' : 'Load a save once with the mod installed (shelf export)'}>On a shelf</button>
      </div>
    </div>
    <div class="scene">
      <canvas bind:this={canvas} class="view3d"></canvas>
      {#if !layout.model}
        <div class="empty">
          <p>Import a <b>.glb</b>, <b>.gltf</b> or <b>.obj</b> model with its texture.</p>
          <p class="muted small">Sketchfab: Download → glTF. Blender: File → Export → glTF 2.0. FBX: open it in Blender and export as .glb.</p>
        </div>
      {/if}
      {#if busy}<p class="overlay muted small">Loading…</p>{/if}
      <p class="hint muted small">Drag to turn · right-drag to move · wheel to zoom</p>
    </div>
    {#if mode === 'shelf'}
      <div class="row wrap small">
        <label class="field">Shelf
          <select bind:value={shelfName}>
            {#each shelves as s}<option value={s.name}>{s.name}{s.m_ItemNotForSale ? ' (personal)' : ''}</option>{/each}
          </select>
        </label>
        <label class="check"><input type="checkbox" bind:checked={compare} /> Vanilla {base} on every other compartment</label>
        <button class="small" onclick={frameShelf}>Reset view</button>
      </div>
    {/if}
  </div>

  <div class="side">
    {#if layout.model}
      <div class="props">
        <div class="small"><b>{layout.sourceName}</b> <span class="muted">· {layout.triangles.toLocaleString()} triangles</span></div>
        {#if layout.warnings?.length}
          <div class="warn small">{#each layout.warnings as w}<div>⚠ {w}</div>{/each}</div>
        {/if}
      </div>
      <div class="props">
        <h4>Size</h4>
        <label class="field">Height in game (cm)
          <input type="number" min="0.5" step="0.5" value={((layout.height || baseHeight) * cmU).toFixed(1)}
            onchange={(e) => setHeightCm(+e.currentTarget.value)} />
        </label>
        <input type="range" min={baseHeight * 0.3} max={baseHeight * 3} step={baseHeight / 100} value={layout.height || baseHeight}
          oninput={(e) => { layout.height = +e.currentTarget.value; changed(); }} />
        <button class="small" onclick={() => { layout.height = baseHeight; changed(); }}>Match {base} ({(baseHeight * cmU).toFixed(1)} cm)</button>
      </div>
      <div class="props">
        <h4>Orientation</h4>
        <p class="muted small">The front should face you in the Model view (the vanilla toy shows which way).</p>
        <div class="row wrap">
          <button class="tiny" onclick={() => turn('rotX', 90)} title="Tip forward 90° (fixes Z-up models)">Tip ↓</button>
          <button class="tiny" onclick={() => turn('rotX', -90)}>Tip ↑</button>
          <button class="tiny" onclick={() => turn('rotZ', 90)} title="Roll 90°">Roll ↺</button>
          <button class="tiny" onclick={() => turn('rotZ', -90)}>Roll ↻</button>
          <button class="tiny" onclick={() => turn('rotY', 180)} title="Turn around">Turn 180°</button>
        </div>
        <label class="field">Turn ({layout.rotY}°)
          <input type="range" min="-180" max="180" step="5" value={layout.rotY} oninput={(e) => { layout.rotY = +e.currentTarget.value; changed(); }} />
        </label>
        <button class="small" onclick={() => { layout.rotX = 0; layout.rotY = 0; layout.rotZ = 0; changed(); }}>Reset orientation</button>
      </div>
      {#if fit}
        <div class="props small">
          <h4>Shelf fit {#if shelf}<span class="muted">({shelf.name})</span>{/if}</h4>
          <div>{fit.perCompartment} per compartment, like {base}. Slot {cm(fit.slot.w)} × {cm(fit.slot.d)} cm.</div>
          <div class="muted">Figurine {cm(fit.fig.w)} wide × {cm(fit.fig.d)} deep × {cm(fit.fig.h)} tall · {base} {cm(fit.base.w)} × {cm(fit.base.d)} × {cm(fit.base.h)}</div>
          {#each fit.problems as pr}<div class="warn">⚠ {pr}</div>{/each}
          {#if fit.problems.length}<div class="muted">Make it smaller, or pick a bigger “Start from” toy (its slot size is used).</div>{/if}
        </div>
      {/if}
    {:else if !templates}
      <p class="warn small">Templates not exported yet: start the game and load a save once with the mod installed (vanilla toys and shelves for size).</p>
    {/if}
  </div>
</div>

<style>
  .fig { display: flex; gap: 14px; align-items: flex-start; }
  .stage { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 8px; }
  .scene { position: relative; height: 480px; background: radial-gradient(circle at 50% 40%, #2a3140, var(--bg)); border-radius: 6px; }
  .view3d { width: 100%; height: 100%; display: block; cursor: grab; touch-action: none; }
  .view3d:active { cursor: grabbing; }
  .empty { position: absolute; inset: 0; display: flex; flex-direction: column; align-items: center; justify-content: center; text-align: center; padding: 20px; pointer-events: none; }
  .overlay { position: absolute; left: 12px; top: 8px; margin: 0; }
  .hint { position: absolute; left: 12px; bottom: 4px; margin: 0; }
  .seg button.on { border-color: var(--accent); background: #22304d; }
  .side { width: 300px; flex-shrink: 0; display: flex; flex-direction: column; gap: 10px; }
  .props { display: flex; flex-direction: column; gap: 8px; background: var(--panel); border: 1px solid var(--line); border-radius: var(--radius); padding: 10px; }
  .props h4 { margin: 0; font-size: 13px; }
  .check { display: flex; align-items: center; gap: 6px; }
  .wrap { flex-wrap: wrap; }
  .warn { color: #d9a400; }
  button.tiny { padding: 2px 7px; font-size: 12px; }
  .small { font-size: 12px; }
</style>
