<script lang="ts" module>
  // Layer clipboard (Ctrl+C / Ctrl+V), shared by every editor instance: a layer copied on a deck box pastes into a pack.
  let clipboard: { layer: any; kind: string; face: number[]; textureSize: number } | null = null;
</script>

<script lang="ts">
  // Texture editor for accessories, packs/boxes and Game look boxes: layers placed straight on the game texture (the Texture
  // view, with the mesh's UV outlines and the faces' areas as guides) and a 3D preview of the real game mesh. Projected models
  // (furniture paint) keep their Views: layers on views unfolded from the model, projected onto it by the GPU painter.
  // Dice (palette models) only pick one colour per swatch.
  import { maximizable, fullWindow } from '../lib/maximize';
  import { ZoomPan } from '../lib/zoomPan';
  import { splitter } from '../lib/splitter';
  import { createHistory } from '../lib/history.svelte';
  import { accFile, templateFile } from '../lib/urls';
  import { onMount, untrack } from 'svelte';
  import { App, errText, ask } from '../lib/api';
  import {
    type Model, type Layout, type Layer, type Face, newLayout, netBounds, netScale, faceRect, renderNet, renderTexture, composeIcon,
    recolorIcon, loadImage, layerId, renderTemplate, targetLabel, meshIcon, pickableFaces, previewUrl, layerTextures,
    shownArea, shownNet, isMapLayer, uvOutlines, faceArea, placeIn, placeRect, placeStacked, areaFills, loadIslands, wholeFaceFill, faceFillsToRects, layersToInclusive,
  } from '../lib/accessoryArt';
  import { ProjectPainter, MAX_LAYER_TEXTURES, renderPaintIcon, type LayerTexture } from '../lib/projectPaint';
  import { MeshView } from '../lib/meshView';
  import Straighten from './Straighten.svelte';

  let {
    kind, base = '', vanillaUrl, vanillaIconUrl = '', layout = $bindable(), iconSize = [512, 512], notify, onchange,
    imageUrl, pickImage, saveImage, defaultText = 'DECK BOX', givenModel, active = true, onbaseline,
  }: {
    kind: string;
    givenModel?: Model;                 // a model made elsewhere (furniture paint templates); default: the accessory kind's
    active?: boolean;                   // false: mounted but hidden (keys go to the page around it)
    base?: string;                      // vanilla item it starts from (for its second texture in the 3D preview)
    vanillaUrl: string;                 // vanilla texture of the chosen base ('' when not exported)
    vanillaIconUrl?: string;            // vanilla shop icon of the base (palette models recolour it)
    layout: Layout;
    iconSize?: number[];
    notify: (t: string, k?: string) => void;
    onchange?: () => void;              // any edit (pages that compare snapshots need not listen)
    onbaseline?: () => void;            // the layout was converted on opening (same look): pages that compare snapshots take it as
                                        // the saved state, so opening alone is no unsaved change
    imageUrl?: (rel: string) => string; // URL of a layer image (default: the accessory library)
    pickImage?: () => Promise<string>;  // lets the user add an image, returns its path for imageUrl (default: accessory library)
    saveImage?: (name: string, dataUrl: string) => Promise<string>; // stores a made image (straightened), returns its path
    defaultText?: string;               // text of a new text layer
  } = $props();

  let model = $state<Model | null>(null);
  /** Faces offered in the pickers (fit, wrap, copy to, fill): projected models offer their views. */
  let pickFaces = $derived(model ? pickableFaces(model) : []);
  let S = 300;
  let vanilla: HTMLImageElement | null = null;
  let net: HTMLCanvasElement = document.createElement('canvas');
  let view = $state<'net' | 'texture' | '3d'>('net');
  let selected = $state('');
  // Guides on the texture: off / on / only (the texture's base hidden: just the guides and your layers; display only).
  type Guides = 'off' | 'on' | 'only';
  const readPref = (key: string) => { try { return localStorage.getItem(key); } catch { return null; } };
  const writePref = (key: string, v: string) => { try { localStorage.setItem(key, v); } catch { /* per-viewer only */ } };
  let guides = $state<Guides>((['off', 'on', 'only'] as const).find((g) => g === readPref('texEditor.guides')) ?? 'on');
  let showGuides = $derived(guides !== 'off');

  // Display canvases
  let netView = $state<HTMLCanvasElement>();
  let texView = $state<HTMLCanvasElement>();
  let textureCanvas: HTMLCanvasElement | null = null;
  // Guides only: the texture rendered on a plain base (shown on the Texture view; the 3D view and saving keep the real one).
  let onlyCanvas: HTMLCanvasElement | null = null;
  const onlyNet: HTMLCanvasElement = document.createElement('canvas');
  const guidesOnly = () => guides === 'only' && !!model && !model.projected && !model.palette;
  const shownTexture = () => (guidesOnly() && onlyCanvas) || textureCanvas;

  // Zoom and pan of the Texture and Views canvases (overlay = guides and handles in screen pixels).
  let zpTex: ZoomPan | null = null, zpNet: ZoomPan | null = null;
  let zoomPct = $state(100);
  const zpMain = () => (view === 'net' ? zpNet : zpTex);
  function zoomStage(node: HTMLElement, which: 'tex' | 'net') {
    const zp: ZoomPan = new ZoomPan(node, () => (which === 'tex' ? texView : netView), () => {
      if (zp === zpMain()) zoomPct = zp.percent();
      if (which === 'tex') drawTexOverlay(); else drawNetOverlay();
    }, () => active);
    if (which === 'tex') zpTex = zp; else zpNet = zp;
    return { destroy() { zp.destroy(); if (zpTex === zp) zpTex = null; if (zpNet === zp) zpNet = null; } };
  }
  /** Canvas px per screen px of the current view (handle sizes). */
  const viewPx = () => zpMain()?.px() ?? 1;
  // The game texture is made only when something shows or needs it (texture view, 3D preview, export): on the Faces view an
  // edit redraws the net alone (renderTexture). makeTexture = what the last render left to make it.
  let makeTexture: (() => HTMLCanvasElement) | null = null;

  function ensureTexture() {
    if (!makeTexture) return;
    textureCanvas = makeTexture();
    makeTexture = null;
    mesh?.setTexture(textureCanvas);
  }

  const accUrl = (rel: string) => imageUrl ? imageUrl(rel) : accFile(rel);
  let sel = $derived(layout?.layers.find((l) => l.id === selected));

  // ---------------------------------------------------------------- rendering

  // Edits while a render runs (a colour picker or slider sends many) are coalesced into one more render, after the browser
  // got a frame to handle input and paint — so dragging a picker stays smooth on big textures.
  let rendering = false, again = false;
  const nextFrame = () => new Promise<void>((r) => requestAnimationFrame(() => r()));
  async function redraw() {
    if (!model) return;
    if (rendering) { again = true; return; }
    rendering = true;
    try {
      do {
        if (again) await nextFrame();
        again = false;
        if (model.projected) {
          // GPU painter (lib/projectPaint.ts): the views' base picture is rendered once per base; each edit only redraws the layers
          // (net view: over that picture; 3D: as layer textures the painter projects onto every surface). The texture is made on demand.
          const base = (await paintedImage()) ?? projectedBase();
          if (!base) break;
          const w = await workPainter();
          if (paintBase !== base || baseNetS !== S) {
            paintBase = base; baseNetS = S;
            w.setBase(base);
            painter3d?.setBase(base);
            const b = netBounds(model);
            baseNet = w.renderNet(Math.max(1, Math.round(b.w * S)), Math.max(1, Math.round(b.h * S)), [b.x0, b.y0, b.w, b.h], false);
          }
          await renderNet(net, model, layout, null, accUrl, S, false, baseNet);
          drawNetView();
          paintLayers = await layerTextures(model, layout, accUrl, S, MAX_LAYER_TEXTURES);
          painter3d?.setLayers(paintLayers.textures, paintLayers.net);
          textureCanvas = null;
          if (view === 'texture') { textureCanvas = atlasNow(); drawTexView(); }
          continue;
        }
        makeTexture = await renderTexture(model, layout, vanilla, await paintedImage(), accUrl, S, net);
        onlyCanvas = guidesOnly()
          ? (await renderTexture(model, { ...layout, base: 'color', baseColor: '#3a3a3a', baseItem: undefined, textureFile: undefined }, null, null, accUrl, S, onlyNet))()
          : null;
        drawNetView();
        ensureTexture(); // the 3D preview beside either tab shows it
        if (view !== 'net') drawTexView();
      } while (again);
    } catch (e) { notify(errText(e), 'error'); }
    rendering = false;
  }

  function drawNetView() {
    if (!netView || !model) return;
    if (netView.width !== net.width || netView.height !== net.height) { netView.width = net.width; netView.height = net.height; }
    const ctx = netView.getContext('2d')!;
    ctx.clearRect(0, 0, net.width, net.height);
    ctx.drawImage(net, 0, 0);
    if (zpNet) zpNet.layout(); else drawNetOverlay(); // layout redraws the overlay
  }

  /** Frames, labels and the selection over the Views canvas (screen-sized lines at any zoom). */
  function drawNetOverlay() {
    const o = zpNet?.beginOverlay();
    if (!o || !model) return;
    const { ctx, px } = o;
    const lw = 2 * px;
    if (showGuides) {
      ctx.font = `bold ${13 * px}px Nunito, sans-serif`;
      // Projected models: their views are the frames (their faces are many and overlap).
      const b0 = netBounds(model);
      const frames = [
        ...model.faces.map((f) => ({ label: f.label, hidden: f.hidden, r: faceRect(model!, f, S), shown: f.shown })),
        ...(model.views ?? []).map((v) => ({ label: v.label, hidden: false, shown: undefined, r: { x: (v.net[0] - b0.x0) * S, y: (v.net[1] - b0.y0) * S, w: v.net[2] * S, h: v.net[3] * S } })),
      ];
      for (const f of frames) {
        const r = f.r;
        ctx.strokeStyle = 'rgba(255,255,255,0.7)';
        ctx.setLineDash([lw * 4, lw * 3]);
        ctx.lineWidth = lw;
        ctx.strokeRect(r.x, r.y, r.w, r.h);
        if (f.shown) {
          // The face's outer strip (texture past the item's edge): a finer line where the item ends.
          const [sx, sy, sw, sh] = f.shown;
          ctx.strokeStyle = 'rgba(255,214,90,0.8)';
          ctx.setLineDash([lw * 2, lw * 2]);
          ctx.strokeRect(r.x + sx * r.w, r.y + sy * r.h, sw * r.w, sh * r.h);
        }
        ctx.setLineDash([]);
        ctx.fillStyle = 'rgba(0,0,0,0.55)';
        const label = f.label + (f.hidden ? ' (hidden)' : '');
        const tw = ctx.measureText(label).width;
        ctx.fillRect(r.x + lw * 3, r.y + lw * 3, tw + lw * 6, 20 * px);
        ctx.fillStyle = '#fff';
        ctx.fillText(label, r.x + lw * 6, r.y + lw * 3 + 14 * px);
      }
    }
    const b = netBounds(model);
    if (sel && movable(sel)) drawSelection(ctx, sel, S, b.x0, b.y0, lw);
    // A selected face colour fill: its face (or every face) outlined; while dragging it, the face it would move to.
    if (sel && wholeFaceFill(sel)) {
      const dragging = drag?.mode === 'face';
      ctx.lineWidth = 3 * px;
      for (const f of pickFaces) {
        const target = dragging && f.id === fillOver && fillOver !== fillFrom;
        if (!target && !(sel.face ? f.id === sel.face : !f.hidden)) continue;
        const r = { x: (f.net[0] - b.x0) * S, y: (f.net[1] - b.y0) * S, w: f.net[2] * S, h: f.net[3] * S };
        ctx.strokeStyle = target ? '#ffd24a' : '#4fa3ff';
        ctx.setLineDash(target ? [8 * px, 5 * px] : []);
        ctx.strokeRect(r.x, r.y, r.w, r.h);
      }
      ctx.setLineDash([]);
    }
  }

  /** The selected layer's frame and handles (s canvas px per layer unit, x0/y0 = the canvas origin in layer units). */
  function drawSelection(ctx: CanvasRenderingContext2D, l: Layer, s: number, x0: number, y0: number, lw: number) {
    ctx.save();
    ctx.translate((l.x - x0) * s, (l.y - y0) * s);
    ctx.rotate((l.rot * Math.PI) / 180);
    if (l.skew) ctx.transform(1, 0, l.mirror ? -l.skew : l.skew, 1, 0, 0);
    const w = l.w * s, h = l.h * s;
    ctx.strokeStyle = '#4fa3ff';
    ctx.lineWidth = lw;
    ctx.strokeRect(-w / 2, -h / 2, w, h);
    const hs = lw * 7;
    // Corner (bottom-right): proportional scale (Shift = free). Edge midpoints: stretch that side only.
    for (const [hx, hy, corner] of HANDLES) {
      ctx.fillStyle = corner ? '#4fa3ff' : '#ffffff';
      ctx.strokeStyle = '#4fa3ff';
      ctx.fillRect(hx * w - hs / 2, hy * h - hs / 2, hs, hs);
      if (!corner) ctx.strokeRect(hx * w - hs / 2, hy * h - hs / 2, hs, hs);
    }
    ctx.restore();
  }

  // UV island outlines of the mesh (guides on the Texture view; map layers aren't limited by them).
  let outlines: number[][] = [];

  function drawTexView() {
    const src = shownTexture();
    if (!texView || !src || !model) return;
    if (texView.width !== src.width || texView.height !== src.height) { texView.width = src.width; texView.height = src.height; }
    const ctx = texView.getContext('2d')!;
    ctx.clearRect(0, 0, texView.width, texView.height);
    ctx.drawImage(src, 0, 0);
    if (zpTex) zpTex.layout(); else drawTexOverlay(); // layout redraws the overlay
  }

  /** UV outlines, face areas and the selection over the Texture canvas (screen-sized lines at any zoom). */
  function drawTexOverlay() {
    const o = zpTex?.beginOverlay();
    if (!o || !model || !texView) return;
    const { ctx, px } = o;
    const k = texView.width / model.textureSize;
    if (showGuides) {
      ctx.strokeStyle = 'rgba(120,220,255,0.85)';
      ctx.lineWidth = 1.25 * px;
      ctx.beginPath();
      for (const [ax, ay, bx, by] of outlines) { ctx.moveTo(ax * k, ay * k); ctx.lineTo(bx * k, by * k); }
      ctx.stroke();
      ctx.font = `bold ${13 * px}px Nunito, sans-serif`;
      for (const f of model.faces) for (const t of f.targets) {
        if (t.bleed) continue;
        ctx.strokeStyle = f.hidden ? 'rgba(255,255,255,0.4)' : '#ffd24a';
        ctx.lineWidth = 2 * px;
        ctx.strokeRect(t.rect[0] * k, t.rect[1] * k, (t.rect[2] - t.rect[0]) * k, (t.rect[3] - t.rect[1]) * k);
        ctx.fillStyle = 'rgba(0,0,0,0.6)';
        const label = targetLabel(f, t);
        const tw = ctx.measureText(label).width;
        ctx.fillRect(t.rect[0] * k + 4 * px, t.rect[1] * k + 4 * px, tw + 8 * px, 19 * px);
        ctx.fillStyle = '#fff';
        ctx.fillText(label, t.rect[0] * k + 8 * px, t.rect[1] * k + 18 * px);
      }
    }
    if (sel && mapView() && movable(sel)) drawSelection(ctx, sel, k, 0, 0, 2 * px);
  }

  /** Selection changes: only the overlay. */
  const drawView = () => (view === 'texture' ? drawTexOverlay() : drawNetOverlay());

  /** PNG data URLs of the texture and the shop icon for saving. */
  export async function exportImages(): Promise<{ texture: string; icon: string }> {
    await loaded;                                         // model + vanilla art (an export right after mounting)
    await redraw();
    while (rendering) await new Promise((r) => setTimeout(r, 30)); // a redraw already running returns at once
    if (model?.projected) {
      const tex = atlasNow();
      if (!tex || !paintBase) throw new Error('editor not ready');
      const icon = await renderPaintIcon(paintMeshUrl(), paintBase, paintLayers?.textures ?? [], paintLayers?.net ?? [0, 0, 1, 1], iconSize, model.turn ?? 0);
      return { texture: tex.toDataURL('image/png'), icon };
    }
    ensureTexture();
    if (!model || !textureCanvas) throw new Error('editor not ready');
    let icon: HTMLCanvasElement;
    const texture = textureCanvas.toDataURL('image/png');
    // Packs: the game mesh rendered with the finished texture (what the pack looks like in game).
    if (model.icon === 'pack' || model.icon === 'mesh') return { texture, icon: await meshIcon(model, textureCanvas) };
    if (model.icon === 'recolor') {
      if (!vanilla || !vanillaIconUrl) throw new Error("the vanilla icon is needed to recolour it — the game templates aren't available yet (Settings → Game)");
      icon = recolorIcon(model, net, S, vanilla, await loadImage(vanillaIconUrl));
    } else icon = composeIcon(model, net, S, iconSize[0], iconSize[1]);
    return { texture, icon: icon.toDataURL('image/png') };
  }

  /** After a layer action (add, paste, duplicate, delete, copy to face, reorder): always its own undo step. */
  function commit() { hist.split(); changed(); hist.split(); }

  /** After any edit; edits close together (typing, sliders) are one undo step. */
  function changed() {
    hist.edited();
    hist.base = snapshot();
    onchange?.();
    redraw();
  }

  // ---------------------------------------------------------------- undo / redo (Ctrl+Z, Ctrl+Y / Ctrl+Shift+Z)
  // A whole drag on the net is one step, pushed when it starts.

  const snapshot = () => JSON.stringify(layout);
  const hist = createHistory(snapshot, (state) => {
    layout = JSON.parse(state);
    ownLayout = layout;
    if (!layout.layers.some((l) => l.id === selected)) selected = '';
    onchange?.();
    redraw();
  }, () => !!drag);
  const undo = () => hist.undo();

  // The page swaps the layout object itself (Revert reloads the saved one): start over from it — new history, nothing
  // selected — and show it. The editor's own swaps (undo/redo, a new layout) set ownLayout first.
  let ownLayout: Layout | null = null;
  $effect(() => {
    const l = layout;
    if (!l || l === ownLayout || !ownLayout) return;
    ownLayout = l;
    untrack(() => {
      if (!l.layers.some((x) => x.id === selected)) selected = '';
      if (mapModel()) { paintedToLayer(); fillsToRects(); edgesToInclusive(); } // the reloaded design in the editor's form, as on opening
      hist.reset();
      redraw();
    });
  });
  const redo = () => hist.redo();

  // ---------------------------------------------------------------- layers

  function faceById(id: string) { return pickFaces.find((f) => f.id === id); }
  const mainFace = () => (faceById('front') ?? faceById('surface') ?? pickFaces[0])!;

  /** Faces in the main row (same height as the main face): the deck box's four sides, a book's back + spine + front. */
  function wrapFaces() {
    const m = mainFace();
    return pickFaces.filter((f) => !f.hidden && f.net[1] === m.net[1] && f.net[3] === m.net[3]);
  }

  function fitInto(l: Layer, net: number[], mode: 'cover' | 'contain', aspect: number) {
    const [x, y, w, h] = net;
    let lw = w, lh = w / aspect;
    if ((mode === 'cover' && lh < h) || (mode === 'contain' && lh > h)) { lh = h; lw = h * aspect; }
    l.w = lw; l.h = lh; l.x = x + w / 2; l.y = y + h / 2; l.rot = 0;
  }

  function wrapArea(): number[] {
    // One panorama across the main row (deck box: Left–Front–Right–Back; books: Back–Spine–Front); else the main face.
    const row = wrapFaces();
    return row.length < 2 ? shownNet(mainFace()) : shownArea(row);
  }

  async function addImage() {
    try {
      const rel = await (pickImage ? pickImage() : App.PickAccessoryImage());
      if (rel) await addImageFile(rel, '', 'contain', [], onMap());
    } catch (e) { notify(errText(e), 'error'); }
  }

  /** Face-net models: every layer is a map layer (placed in the faces' texture areas). Projected models paint on views. */
  const mapModel = () => !!model && !model.projected;
  /** New layers from the buttons go where you are: texture layers on the Texture tab, face layers on the Faces tab (one
   *  picture over every area of a face, e.g. the deck box's tray front and cover). */
  const onMap = () => mapModel() && view === 'texture';
  /** The main face's texture area (where new art goes). */
  const mainArea = () => faceArea(model!, mainFace().id)!;

  // ---------------------------------------------------------------- painting templates / painted texture

  /** Size of the game texture (the base's own art; the model's nominal size when it isn't available). */
  const textureSize = () => [vanilla?.width || model!.textureSize, vanilla?.height || vanilla?.width || model!.textureSize];

  /** The painted texture, stretched to the game texture's shape when it differs (its own width kept, so no detail is lost). */
  // Projected models (furniture): an off-screen GPU painter for the views' base picture and the texture; the 3D view has its own.
  let work: ProjectPainter | null = null;
  let workLoading: Promise<ProjectPainter> | null = null;
  let painter3d: ProjectPainter | null = null;
  let paintBase: HTMLImageElement | HTMLCanvasElement | null = null, baseNetS = 0;
  let baseNet: HTMLCanvasElement = document.createElement('canvas');
  let paintLayers: { textures: LayerTexture[]; net: number[] } | null = null;
  let colorBase: { color: string; canvas: HTMLCanvasElement } | null = null;
  const paintMeshUrl = () => previewUrl(model!.preview![0]);

  function workPainter(): Promise<ProjectPainter> {
    if (!workLoading) workLoading = (async () => {
      const p = new ProjectPainter(document.createElement('canvas'));
      await p.load(paintMeshUrl());
      work = p;
      return p;
    })();
    return workLoading;
  }

  /** The finished texture of a projected model (GPU render; only when shown or saved). */
  function atlasNow(): HTMLCanvasElement | null {
    if (!work || !paintBase || !model) return null;
    work.setLayers(paintLayers?.textures ?? [], paintLayers?.net ?? [0, 0, 1, 1]);
    return work.renderAtlas(model.textureSize, paintBase);
  }

  /** Projected models keep their base texture whole (faces overlap in the net): the vanilla bake, or the base colour. */
  function projectedBase(): HTMLImageElement | HTMLCanvasElement | null {
    if (!model?.projected) return null;
    if (layout.base !== 'color' && vanilla) return vanilla;
    const color = layout.baseColor || '#3a3a3a';
    if (colorBase?.color === color) return colorBase.canvas;
    const c = document.createElement('canvas');
    c.width = c.height = model.textureSize;
    const ctx = c.getContext('2d')!;
    ctx.fillStyle = color;
    ctx.fillRect(0, 0, c.width, c.height);
    colorBase = { color, canvas: c };
    return c;
  }

  async function paintedImage(): Promise<HTMLImageElement | HTMLCanvasElement | null> {
    if (!layout.textureFile) return null;
    let img: HTMLImageElement;
    try { img = await loadImage(accUrl(layout.textureFile)); }
    catch { notify(`Painted texture ${layout.textureFile} is missing — using the faces only.`, 'error'); return null; }
    const [w, h] = textureSize();
    const ph = Math.round(img.width * h / w);
    if (Math.abs(ph - img.height) <= 1) return img;
    const c = document.createElement('canvas');
    c.width = img.width; c.height = ph;
    c.getContext('2d')!.drawImage(img, 0, 0, c.width, c.height);
    return c;
  }

  /** Saves a blank template of the texture (face areas, labels, up arrows) in the game texture's size, to paint on elsewhere. */
  async function exportTemplate() {
    if (!model) return;
    try {
      const [w, h] = textureSize();
      const name = `${kind}${base ? '_' + base : ''}_template.png`;
      const path = await App.SaveTemplateImage(name, renderTemplate(model, w, h).toDataURL('image/png'));
      if (path) notify(`Template saved to ${path} (${w}×${h}). Paint over it, then use "Import painted texture…".`);
    } catch (e) { notify(errText(e), 'error'); }
  }

  /** Uses a painted image (in the template's layout, any size of the same shape) as the whole texture. */
  async function importPainted() {
    if (!model) return;
    try {
      const rel = await (pickImage ? pickImage() : App.PickAccessoryImage());
      if (!rel) return;
      const img = await loadImage(accUrl(rel));
      const [w, h] = textureSize();
      if (Math.abs(img.width / img.height - w / h) > 0.01
        && !(await ask(`This image is ${img.width}×${img.height}, but the texture is ${w}×${h} — it won't line up with the faces. Stretch it to fit anyway?`))) return;
      if (img.width < w && !(await ask(`This image (${img.width} px wide) is smaller than the game texture (${w} px) and will look blurrier. Use it anyway?`))) return;
      if (mapModel()) {
        const l = textureLayer(rel);
        layout.layers = [l, ...layout.layers];
        selected = l.id;
        commit();
        notify(`Your texture (${img.width}×${img.height}) is now the bottom layer — layers above it go on top.`);
        return;
      }
      layout.textureFile = rel;
      commit();
      notify(`Painted texture in use (${img.width}×${img.height}). Layers you add go on top of it.`);
    } catch (e) { notify(errText(e), 'error'); }
  }

  function removePainted() {
    delete layout.textureFile;
    commit();
  }

  /** Adds an image (path for imageUrl) `fit`ted into the main face's area, or stretched over the areas of `faces` stacked top
   *  to bottom (a pack photo over crimp, front and crimp: a slice per face); `name` labels the layer. */
  export async function addImageFile(rel: string, name = '', fit: 'cover' | 'contain' | 'stretch' = 'contain', faces: string[] = [], toMap = mapModel()) {
    const img = await loadImage(accUrl(rel));
    const l: Layer = { id: layerId(), kind: 'image', name: name || rel.split('/').pop() || 'Image', visible: true, opacity: 1, blend: 'source-over',
      x: 0, y: 0, w: 1, h: 1, rot: 0, src: rel };
    if (toMap) {
      if (faces.length > 1) {
        const parts = placeStacked(model!, faces, l);
        layout.layers.push(...parts);
        selected = parts.at(-1)?.id ?? '';
        commit();
        return;
      }
      placeIn(model!, (faces[0] && faceArea(model!, faces[0])) || mainArea(), l, fit, img.width / img.height);
    } else {
      const fs = faces.map(faceById).filter((f): f is Face => !!f);
      const area = fs.length ? shownArea(fs) : shownNet(mainFace());
      const [x, y, w, h] = area;
      if (fit === 'stretch') { l.w = w; l.h = h; l.x = x + w / 2; l.y = y + h / 2; }
      else fitInto(l, area, fit, img.width / img.height);
    }
    layout.layers.push(l);
    selected = l.id;
    commit();
  }

  function addFill() {
    const l: Layer = { id: layerId(), kind: 'fill', name: 'Colour', visible: true, opacity: 1, blend: 'source-over',
      x: 0, y: 0, w: 0, h: 0, rot: 0, face: '', color: '#2a6df4', color2: '' };
    if (onMap()) { placeRect(mainArea(), l); delete l.face; } // over the main face's area: drag or resize it from there
    else if (mapModel()) {
      // Faces tab: a colour rectangle over the main face, moved and sized like an image (Stretch over a face to cover it).
      const [x, y, w, h] = shownNet(mainFace());
      Object.assign(l, { x: x + w / 2, y: y + h / 2, w, h });
      delete l.face;
    }
    layout.layers.push(l);
    selected = l.id;
    commit();
  }

  /** Puts colour fills under every other layer, first removing earlier fills whose name starts with `replace`. Face-net
   *  models get a colour rectangle over each of the face's texture areas. */
  export function addFills(fills: { face: string; color: string; color2?: string; name: string }[], replace = '') {
    if (replace) layout.layers = layout.layers.filter((l) => !(l.kind === 'fill' && l.name.startsWith(replace)));
    layout.layers.unshift(...fills.flatMap((f): Layer[] => {
      const base: Layer = { id: layerId(), kind: 'fill', name: f.name, visible: true, opacity: 1, blend: 'source-over',
        x: 0, y: 0, w: 0, h: 0, rot: 0, face: f.face, color: f.color, color2: f.color2 ?? '' };
      return mapModel() ? areaFills(model!, f.face ? [f.face] : [], base) : [base];
    }));
    commit();
  }

  function addText() {
    const [fx, fy, fw, fh] = shownNet(mainFace());
    const l: Layer = { id: layerId(), kind: 'text', name: 'Text', visible: true, opacity: 1, blend: 'source-over',
      x: fx + fw / 2, y: fy + fh * 0.15, w: fw * 0.8, h: fh * 0.08, rot: 0,
      text: defaultText, color: '#ffffff', outline: '#000000' };
    if (onMap()) placeRect(mainArea(), l, [0.1, 0.11, 0.8, 0.08]);
    layout.layers.push(l);
    selected = l.id;
    commit();
  }

  function selectLayer(l: Layer) {
    selected = l.id;
    // Face-net models: a layer is edited on its own tab (texture layers on Texture, face layers on Faces).
    if (mapModel() && view !== '3d') view = isMapLayer(l) ? 'texture' : 'net';
    drawView();
  }

  /** Puts the selected map layer into a face's texture area (its picture's own shape for cover/contain). */
  async function placeSelected(faceId: string, mode: 'cover' | 'contain' | 'stretch') {
    const t = model && faceArea(model, faceId);
    if (!sel || !t) return;
    let aspect = 1;
    if (sel.kind === 'image' && sel.src) { try { const img = await loadImage(accUrl(sel.src)); aspect = img.width / img.height; } catch { /* keep */ } }
    placeIn(model!, t, sel, sel.kind === 'image' ? mode : 'stretch', aspect);
    changed();
  }

  function removeLayer(id: string) {
    layout.layers = layout.layers.filter((l) => l.id !== id);
    if (selected === id) selected = '';
    commit();
  }

  /** A copy of a layer (same image file) with a new id, inserted just above `after` (or on top) and selected. */
  function insertCopy(l: Layer, name: string, after?: string): Layer {
    const c: Layer = { ...JSON.parse(JSON.stringify(l)), id: layerId(), name };
    const i = after ? layout.layers.findIndex((x) => x.id === after) : -1;
    const list = [...layout.layers];
    list.splice(i < 0 ? list.length : i + 1, 0, c);
    layout.layers = list;
    selected = c.id;
    return c;
  }

  function duplicateLayer(id: string) {
    const l = layout.layers.find((x) => x.id === id);
    if (!l) return;
    insertCopy(l, l.name + ' copy', id);
    commit();
  }

  /** The face a layer sits on: the visible face whose net holds its centre (else the main face). */
  function faceOf(l: Layer) {
    return pickFaces.find((f) => !f.hidden && l.x >= f.net[0] && l.x <= f.net[0] + f.net[2] && l.y >= f.net[1] && l.y <= f.net[1] + f.net[3]) ?? mainFace();
  }

  /** Places l at the same relative spot of `to` that it had on `from` (nets: x, y, w, h). */
  function mapRect(l: Layer, from: number[], to: number[]) {
    const kx = to[2] / from[2], ky = to[3] / from[3];
    l.x = to[0] + (l.x - from[0]) * kx; l.y = to[1] + (l.y - from[1]) * ky;
    l.w *= kx; l.h *= ky;
  }

  /** Copies the selected layer onto other faces, at the same relative place and size. */
  function copyToFaces(ids: string[]) {
    if (!sel || !model) return;
    const src = sel, from = faceOf(src);
    let last = src.id;
    for (const id of ids) {
      const f = faceById(id);
      if (!f || f.id === from.id) continue;
      const c = insertCopy(src, `${src.name.replace(/ \([^()]*\)$/, '')} (${f.label.toLowerCase()})`, last);
      mapRect(c, shownNet(from), shownNet(f));
      last = c.id;
    }
    commit();
  }

  /** The face opposite the selected layer's face (deck box: front ↔ back, left ↔ right, lid ↔ bottom), when the model shows it. */
  const OPPOSITE: Record<string, string> = { front: 'back', back: 'front', left: 'right', right: 'left', top: 'bottom', bottom: 'top' };
  const oppositeFace = $derived.by(() => {
    if (!sel || !model || wholeFaceFill(sel) || isMapLayer(sel)) return undefined;
    // Views of projected models are "view:Front" etc.
    const id = faceOf(sel).id, isView = id.startsWith('view:');
    const opp = OPPOSITE[(isView ? id.slice(5) : id).toLowerCase()] ?? '';
    const f = faceById(isView && opp ? 'view:' + opp[0].toUpperCase() + opp.slice(1) : opp);
    return f && !f.hidden ? f : undefined;
  });

  function copyLayer() {
    if (!sel || !model) return;
    clipboard = { layer: JSON.parse(JSON.stringify(sel)), kind: model.kind, face: [...shownNet(faceOf(sel))], textureSize: model.textureSize };
    notify(`Copied "${sel.name}" — Ctrl+V pastes it`);
  }

  function pasteLayer() {
    if (!clipboard || !model) return;
    if (isMapLayer(clipboard.layer) && !mapModel()) { notify("A texture layer can't go on furniture views.", 'warn'); return; }
    const c = insertCopy(clipboard.layer as Layer, clipboard.layer.name, selected || undefined);
    // From another model (e.g. a deck box into a pack): same relative place on this model's main face, or of its texture map.
    if (isMapLayer(c)) {
      const k = model.textureSize / clipboard.textureSize;
      c.x *= k; c.y *= k; c.w *= k; c.h *= k;
      if (c.clip) c.clip = c.clip.map((v) => v * k);
    } else if (clipboard.kind !== model.kind && !wholeFaceFill(c)) mapRect(c, clipboard.face, shownNet(mainFace()));
    if (c.kind === 'fill' && c.face && !faceById(c.face)) c.face = '';
    commit();
  }

  function onKey(e: KeyboardEvent) {
    if (!model || !active || straightening || e.defaultPrevented) return;
    const t = e.target as HTMLElement | null;
    if (t && (t.isContentEditable || ['INPUT', 'SELECT', 'TEXTAREA'].includes(t.tagName))) return;
    if (t?.closest?.('.backdrop')) return; // a dialog is open
    const ctrl = e.ctrlKey || e.metaKey, k = e.key.toLowerCase();
    const act = (f: () => void) => { e.preventDefault(); f(); };
    if (ctrl && k === 'z' && !e.shiftKey) act(undo);
    else if (ctrl && (k === 'y' || (k === 'z' && e.shiftKey))) act(redo);
    else if (ctrl && k === 'c' && sel) act(copyLayer);
    else if (ctrl && k === 'v' && clipboard) act(pasteLayer);
    else if (ctrl && k === 'd' && sel) act(() => duplicateLayer(sel!.id));
    else if ((e.key === 'Delete') && sel) act(() => removeLayer(sel!.id));
  }

  // ---------------------------------------------------------------- layer list: drag a row to reorder (top of the list = on top)

  let layersEl = $state<HTMLElement>();
  let rowDrag: { id: string; y: number; on: boolean; pointer: number } | null = null;
  let dropAt = $state(-1);           // insertion place in the shown (top-first) list while dragging; -1 = none
  let noClick = false;               // the click that ends a drag doesn't select

  function rowDown(e: PointerEvent, id: string) {
    if (e.button !== 0 || (e.target as HTMLElement).closest('button, input, select')) return;
    rowDrag = { id, y: e.clientY, on: false, pointer: e.pointerId };
  }
  function rowMove(e: PointerEvent) {
    if (!rowDrag || e.pointerId !== rowDrag.pointer) return;
    if (!rowDrag.on) {
      if (Math.abs(e.clientY - rowDrag.y) < 5) return;
      rowDrag.on = true;
      (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
    }
    const rows = [...(layersEl?.querySelectorAll<HTMLElement>('.layer[data-id]') ?? [])];
    const i = rows.findIndex((r) => { const b = r.getBoundingClientRect(); return e.clientY < b.top + b.height / 2; });
    dropAt = i < 0 ? rows.length : i;
    // Near the top or bottom edge of the scrolling list (or of the workspace dock): scroll it along.
    const scroller = layersEl && layersEl.scrollHeight > layersEl.clientHeight ? layersEl : layersEl?.closest<HTMLElement>('.side');
    if (scroller) {
      const b = scroller.getBoundingClientRect();
      if (e.clientY < b.top + 28) scroller.scrollTop -= 10;
      else if (e.clientY > b.bottom - 28) scroller.scrollTop += 10;
    }
  }
  function rowUp() {
    const d = rowDrag;
    rowDrag = null;
    if (!d?.on) return;
    noClick = true;
    setTimeout(() => (noClick = false));
    const at = dropAt;
    dropAt = -1;
    const shown = [...layout.layers].reverse(), from = shown.findIndex((l) => l.id === d.id);
    if (from < 0 || at < 0) return;
    const to = at > from ? at - 1 : at;
    if (to === from) return;
    const [l] = shown.splice(from, 1);
    shown.splice(to, 0, l);
    layout.layers = shown.reverse();
    commit();
  }

  function moveLayer(id: string, delta: number) {
    const i = layout.layers.findIndex((l) => l.id === id), j = i + delta;
    if (i < 0 || j < 0 || j >= layout.layers.length) return;
    const list = [...layout.layers];
    [list[i], list[j]] = [list[j], list[i]];
    layout.layers = list;
    commit();
  }

  function setSize(dim: 'w' | 'h', pct: number) {
    if (!sel || !(pct > 0)) return;
    const f = shownNet(mainFace());
    if (dim === 'w') sel.w = (pct / 100) * f[2];
    else sel.h = (pct / 100) * f[3];
    changed();
  }

  async function resetProportions() {
    if (!sel?.src) return;
    try { const img = await loadImage(accUrl(sel.src)); sel.h = (sel.w * img.height) / img.width; changed(); } catch { /* keep */ }
  }

  async function fitSelected(target: string, mode: 'cover' | 'contain' | 'stretch') {
    if (!sel || !model) return;
    const face = faceById(target), area = target === '_wrap' ? wrapArea() : face && shownNet(face);
    if (!area) return;
    let aspect = sel.w / sel.h;
    if (sel.kind === 'image' && sel.src) { try { const img = await loadImage(accUrl(sel.src)); aspect = img.width / img.height; } catch { /* keep */ } }
    if (mode === 'stretch') { sel.x = area[0] + area[2] / 2; sel.y = area[1] + area[3] / 2; sel.w = area[2]; sel.h = area[3]; sel.rot = 0; }
    else fitInto(sel, area, mode, aspect);
    changed();
  }

  // ---------------------------------------------------------------- straighten (perspective correction of a photo)

  let straightening = $state<Layer | null>(null);
  const straightenFaces = $derived(pickFaces.filter((f) => !f.hidden).map((f) => ({ label: f.label.toLowerCase(), aspect: shownNet(f)[2] / shownNet(f)[3] })));

  async function applyStraighten(png: string, pts: number[], aspect: number) {
    const l = straightening;
    straightening = null;
    if (!l) return;
    try {
      const orig = l.straighten?.src ?? l.src!;
      const base = (orig.split('/').pop() || 'image').replace(/\.[^.]+$/, '');
      const rel = await (saveImage ? saveImage(`${base}_straight.png`, png) : App.SaveAccessorySourceImage(`${base}_straight.png`, png));
      l.straighten = { src: orig, pts };
      l.src = rel;
      l.h = l.w / aspect; // keep the centre and width, take the new shape
      changed();
    } catch (e) { notify(errText(e), 'error'); }
  }

  function undoStraighten(l: Layer) {
    if (!l.straighten) return;
    l.src = l.straighten.src;
    l.straighten = undefined;
    loadImage(accUrl(l.src)).then((img) => { l.h = l.w * img.height / img.width; changed(); }).catch(() => changed());
  }

  // ---------------------------------------------------------------- palette models (dice): one colour per swatch

  let swatchColors = $state<Record<string, string>>({});

  /** Vanilla swatch colours (shown until the user picks one). */
  function readSwatches() {
    if (!model?.palette || !vanilla) return;
    const k = vanilla.width / model.textureSize;
    const c = document.createElement('canvas');
    c.width = vanilla.width; c.height = vanilla.height;
    const ctx = c.getContext('2d', { willReadFrequently: true })!;
    ctx.drawImage(vanilla, 0, 0);
    const out: Record<string, string> = {};
    for (const f of model.faces) {
      const t = f.targets[0].rect;
      const d = ctx.getImageData(Math.round((t[0] + t[2]) / 2 * k), Math.round((t[1] + t[3]) / 2 * k), 1, 1).data;
      out[f.id] = '#' + [d[0], d[1], d[2]].map((v) => v.toString(16).padStart(2, '0')).join('');
    }
    swatchColors = out;
  }

  const swatchLayer = (id: string) => layout.layers.find((l) => l.kind === 'fill' && l.face === id);
  const swatchValue = (id: string) => swatchLayer(id)?.color ?? swatchColors[id] ?? '#808080';

  function setSwatch(id: string, color: string) {
    let l = swatchLayer(id);
    if (!l) {
      l = { id: layerId(), kind: 'fill', name: faceById(id)?.label ?? id, visible: true, opacity: 1, blend: 'source-over',
        x: 0, y: 0, w: 0, h: 0, rot: 0, face: id, color };
      layout.layers.push(l);
    } else l.color = color;
    changed();
  }

  function resetSwatch(id: string) {
    layout.layers = layout.layers.filter((l) => !(l.kind === 'fill' && l.face === id));
    changed();
  }

  // ---------------------------------------------------------------- pointer: drag to move, handle/wheel to scale

  // Handles in layer-local units of (w, h): the corner scales, edge midpoints stretch one side (the opposite side stays put).
  const HANDLES: [number, number, boolean][] = [[0.5, 0.5, true], [0.5, 0, false], [-0.5, 0, false], [0, 0.5, false], [0, -0.5, false]];
  let drag: { mode: 'move' | 'scale' | 'edge' | 'face'; hx: number; hy: number; sx: number; sy: number; x: number; y: number; w: number; h: number } | null = null;
  // Dragging a face colour fill (Faces tab): the face it started on and the face under the pointer now.
  let fillFrom = '', fillOver = '';
  let netCursor = $state('move');

  /** Map layers are placed on the Texture view, face layers on the Faces view (and the 3D view of projected models). */
  const mapView = () => view === 'texture' && !!model && !model.projected;
  const inView = (l: Layer) => isMapLayer(l) === mapView();
  /** Layers dragged in the current view: its own space's images and text, and map fills (rectangles). */
  const movable = (l: Layer) => inView(l) && !wholeFaceFill(l);

  /** The pointer acts on layers: the Faces view, or the Texture view of a face-net model once its texture is shown. */
  const canPoint = () => !!model && (view !== 'texture' || (mapView() && !!textureCanvas));

  /** The current view's canvas and units: canvas size, canvas px per layer unit, the canvas origin in layer units. */
  function space() {
    if (mapView()) return { canvas: texView!, w: texView!.width, h: texView!.height, s: texView!.width / model!.textureSize, x0: 0, y0: 0 };
    const b = netBounds(model!);
    return { canvas: netView!, w: net.width, h: net.height, s: S, x0: b.x0, y0: b.y0 };
  }

  /** The handle of the selected layer under p (index into HANDLES), or -1. */
  function handleAt(p: { x: number; y: number }): number {
    if (!sel || !movable(sel)) return -1;
    const sp = space(), q = local(sel, p), hs = (2 * viewPx() * 7) / sp.s; // drawSelection's handle size, in layer units
    return HANDLES.findIndex(([hx, hy]) => Math.abs(q.x - hx * sel!.w) < hs * 1.5 && Math.abs(q.y - hy * sel!.h) < hs * 1.5);
  }

  /** The pointer in the current view's layer units. */
  function toSpace(e: PointerEvent | WheelEvent) {
    const sp = space(), r = sp.canvas.getBoundingClientRect();
    // The canvas is shown with object-fit: contain — a tall net (the pack) is letterboxed inside the element's box.
    const k = Math.min(r.width / sp.w, r.height / sp.h);
    const ox = r.left + (r.width - sp.w * k) / 2, oy = r.top + (r.height - sp.h * k) / 2;
    return { x: (e.clientX - ox) / k / sp.s + sp.x0, y: (e.clientY - oy) / k / sp.s + sp.y0 };
  }

  function local(l: Layer, p: { x: number; y: number }) {
    const a = (-l.rot * Math.PI) / 180, dx = p.x - l.x, dy = p.y - l.y;
    return { x: dx * Math.cos(a) - dy * Math.sin(a), y: dx * Math.sin(a) + dy * Math.cos(a) };
  }

  /** The visible face (or view) under a Faces-tab point. */
  function faceAt(p: { x: number; y: number }) {
    return pickFaces.find((f) => !f.hidden && p.x >= f.net[0] && p.x <= f.net[0] + f.net[2] && p.y >= f.net[1] && p.y <= f.net[1] + f.net[3]);
  }

  /** The top visible face colour fill on a face (its own, or one over all faces). */
  function fillOn(faceId: string): Layer | undefined {
    for (let i = layout.layers.length - 1; i >= 0; i--) {
      const l = layout.layers[i];
      if (l.visible && wholeFaceFill(l) && (!l.face || l.face === faceId)) return l;
    }
  }

  function hit(p: { x: number; y: number }): Layer | undefined {
    for (let i = layout.layers.length - 1; i >= 0; i--) {
      const l = layout.layers[i];
      if (!l.visible || !movable(l)) continue;
      const q = local(l, p);
      if (Math.abs(q.x) <= l.w / 2 && Math.abs(q.y) <= l.h / 2) return l;
    }
  }

  function onDown(e: PointerEvent) {
    if (!canPoint()) return;
    const p = toSpace(e), target = e.currentTarget as HTMLElement;
    const hi = handleAt(p);
    if (hi >= 0 && sel) {
      const [hx, hy, corner] = HANDLES[hi];
      drag = { mode: corner ? 'scale' : 'edge', hx, hy, sx: p.x, sy: p.y, x: sel.x, y: sel.y, w: sel.w, h: sel.h };
      hist.push(snapshot());
      target.setPointerCapture(e.pointerId);
      return;
    }
    const l = hit(p);
    // Faces tab, nothing else here: the colour fill on this face — drag it onto another face to move it there.
    const f = !l && !mapView() ? faceAt(p) : undefined, fill = f && fillOn(f.id);
    if (f && fill) {
      selected = fill.id;
      fillFrom = fillOver = f.id;
      drag = { mode: 'face', hx: 0, hy: 0, sx: p.x, sy: p.y, x: 0, y: 0, w: 0, h: 0 };
      hist.push(snapshot());
      target.setPointerCapture(e.pointerId);
      netCursor = 'grabbing';
      drawView();
      return;
    }
    // Clicking empty space keeps a face fill selected (it has no frame to click).
    selected = l?.id ?? (sel && wholeFaceFill(sel) ? selected : '');
    if (l) {
      drag = { mode: 'move', hx: 0, hy: 0, sx: p.x, sy: p.y, x: l.x, y: l.y, w: l.w, h: l.h };
      hist.push(snapshot());
      target.setPointerCapture(e.pointerId);
    }
    drawView();
  }

  /** Snaps a moved layer's centre to face edges/centres (Faces view) or texture area edges/centres (Texture view). */
  function snap(l: Layer) {
    if (!model) return;
    const xs: number[] = [], ys: number[] = [];
    let tol: number;
    if (isMapLayer(l)) {
      const t = model.textureSize;
      tol = 0.006 * t;
      xs.push(0, t / 2, t); ys.push(0, t / 2, t);
      for (const f of model.faces) for (const tg of f.targets) {
        if (tg.bleed) continue;
        xs.push(tg.rect[0], (tg.rect[0] + tg.rect[2]) / 2, tg.rect[2]);
        ys.push(tg.rect[1], (tg.rect[1] + tg.rect[3]) / 2, tg.rect[3]);
      }
    } else {
      tol = 0.03 * Math.max(model.size[0], model.size[1]);
      for (const f of pickFaces) {
        const [fx, fy, fw, fh] = shownNet(f);
        xs.push(fx, fx + fw / 2, fx + fw); ys.push(fy, fy + fh / 2, fy + fh);
      }
    }
    for (const cx of xs) if (Math.abs(l.x - cx) < tol) l.x = cx;
    for (const cy of ys) if (Math.abs(l.y - cy) < tol) l.y = cy;
  }

  function onMove(e: PointerEvent) {
    if (!drag) {
      // Hover: resize cursors over the handles.
      const hi = canPoint() ? handleAt(toSpace(e)) : -1;
      netCursor = hi < 0 ? 'move' : HANDLES[hi][2] ? 'nwse-resize' : HANDLES[hi][0] ? 'ew-resize' : 'ns-resize';
      return;
    }
    if (!sel) return;
    const p = toSpace(e);
    if (drag.mode === 'face') {
      const over = faceAt(p)?.id ?? '';
      if (over !== fillOver) { fillOver = over; drawView(); }
      return;
    }
    if (drag.mode === 'move') {
      sel.x = drag.x + (p.x - drag.sx);
      sel.y = drag.y + (p.y - drag.sy);
      if (!e.altKey) snap(sel); // hold Alt to move freely
    } else if (drag.mode === 'scale') {
      const q0 = local({ ...sel, x: drag.x, y: drag.y } as Layer, { x: drag.sx, y: drag.sy });
      const q1 = local({ ...sel, x: drag.x, y: drag.y } as Layer, p);
      if (e.shiftKey) {
        // Free: width and height follow the corner separately.
        sel.w = drag.w * Math.max(0.02, q1.x / q0.x);
        sel.h = drag.h * Math.max(0.02, q1.y / q0.y);
      } else {
        const f = Math.max(0.05, Math.max(q1.x / q0.x, q1.y / q0.y));
        sel.w = drag.w * f;
        sel.h = drag.h * f;
      }
    } else {
      // Edge: stretch one side, the opposite edge stays where it was.
      const q0 = local({ ...sel, x: drag.x, y: drag.y } as Layer, { x: drag.sx, y: drag.sy });
      const q1 = local({ ...sel, x: drag.x, y: drag.y } as Layer, p);
      const a = (sel.rot * Math.PI) / 180, min = isMapLayer(sel) ? 1 : 0.01;
      if (drag.hx) {
        const w = Math.max(min, drag.w + Math.sign(drag.hx) * (q1.x - q0.x));
        const shift = (Math.sign(drag.hx) * (w - drag.w)) / 2;
        sel.w = w; sel.x = drag.x + Math.cos(a) * shift; sel.y = drag.y + Math.sin(a) * shift;
      } else {
        const h = Math.max(min, drag.h + Math.sign(drag.hy) * (q1.y - q0.y));
        const shift = (Math.sign(drag.hy) * (h - drag.h)) / 2;
        sel.h = h; sel.x = drag.x - Math.sin(a) * shift; sel.y = drag.y + Math.cos(a) * shift;
      }
    }
    onchange?.();
    redraw();
  }

  function onUp() {
    if (drag?.mode === 'face') {
      // Dropped on another face: the fill moves there (an all-faces fill becomes that face's).
      if (sel && fillOver && fillOver !== fillFrom) { sel.face = fillOver; onchange?.(); redraw(); }
      fillFrom = fillOver = '';
      netCursor = 'move';
    }
    if (drag) hist.settle(); // a click without moving: no step
    drag = null;
    drawView();
  }

  /** Ctrl+wheel resizes the selected layer (the wheel alone zooms the view, lib/zoomPan). */
  function onWheel(e: WheelEvent) {
    if (!(e.ctrlKey || e.metaKey) || !sel || !canPoint() || !movable(sel)) return;
    e.preventDefault();
    const f = e.deltaY < 0 ? 1.05 : 1 / 1.05;
    sel.w *= f; sel.h *= f;
    changed();
  }

  // ---------------------------------------------------------------- 3D preview: the real game mesh (templates\accessories\*.obj)

  let canvas3d = $state<HTMLCanvasElement>();
  let mesh: MeshView | null = null;
  let meshError = $state('');
  const tpl = (file: string) => templateFile('acc', file);

  $effect(() => {
    const c = canvas3d;
    if (!c || !model?.projected) return;
    let pp: ProjectPainter;
    try { pp = new ProjectPainter(c); } catch (e) { meshError = errText(e); return; }
    painter3d = pp;
    meshError = '';
    pp.ry += model.turn ?? 0;
    pp.load(paintMeshUrl()).then(() => {
      if (paintBase) pp.setBase(paintBase);
      if (paintLayers) pp.setLayers(paintLayers.textures, paintLayers.net);
    }).catch((e) => (meshError = errText(e)));
    const ro = new ResizeObserver(() => pp.draw());
    ro.observe(c);
    return () => { ro.disconnect(); pp.dispose(); if (painter3d === pp) painter3d = null; };
  });

  $effect(() => {
    const c = canvas3d;
    if (!c || !model || model.projected) return;
    let mv: MeshView;
    try { mv = new MeshView(c, !!model.palette); } catch (e) { meshError = errText(e); return; }
    mesh = mv;
    meshError = '';
    if (model.kind === 'Playmat') { mv.rx = 0.85; mv.ry = -0.25; }
    mv.ry += model.turn ?? 0;
    const parts = ((model as any).preview ?? []).map((p: any) => ({
      url: previewUrl(p), texture: p.texture, glass: p.glass, secondaryUrl: base ? tpl(`${base}_texture2.png`) : undefined,
    }));
    mv.load(parts).then(() => { ensureTexture(); if (textureCanvas) mv.setTexture(textureCanvas); })
      .catch((e) => (meshError = errText(e) + " — the game templates aren't available yet (Settings → Game)"));
    const ro = new ResizeObserver(() => mv.draw()); // e.g. full view on/off
    ro.observe(c);
    return () => { ro.disconnect(); mv.dispose(); if (mesh === mv) mesh = null; };
  });

  // Drag on the model: a layer under the pointer moves with it (painted models; Shift+click puts the selected layer there); anywhere
  // else, or with Alt, the view turns.
  let spin: { x: number; y: number; rx: number; ry: number } | null = null;
  let drag3d: { id: string; dx: number; dy: number } | null = null;
  const cam = () => mesh ?? painter3d;
  function spinDown(e: PointerEvent) {
    const c = cam();
    if (!c) return;
    (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
    if (painter3d && !e.altKey) {
      const p = painter3d.pick(e.clientX, e.clientY);
      if (p && e.shiftKey && sel && sel.kind !== 'fill') {
        hist.push(snapshot()); sel.x = p.x; sel.y = p.y; changed();
        drag3d = { id: sel.id, dx: 0, dy: 0 };
        return;
      }
      const l = p ? hit(p) : undefined;
      if (p && l) {
        selected = l.id;
        hist.push(snapshot());
        drag3d = { id: l.id, dx: l.x - p.x, dy: l.y - p.y };
        return;
      }
    }
    spin = { x: e.clientX, y: e.clientY, rx: c.rx, ry: c.ry };
  }
  function spinMove(e: PointerEvent) {
    if (drag3d && painter3d) {
      const l = layout.layers.find((q) => q.id === drag3d!.id), p = painter3d.pick(e.clientX, e.clientY);
      if (l && p) { l.x = p.x + drag3d.dx; l.y = p.y + drag3d.dy; changed(); }
      return;
    }
    const c = cam();
    if (!spin || !c) return;
    c.ry = spin.ry + (e.clientX - spin.x) * 0.01;
    c.rx = Math.max(-1.55, Math.min(1.55, spin.rx + (e.clientY - spin.y) * 0.01));
    c.draw();
  }
  function spinUp() { spin = null; drag3d = null; }
  function spinWheel(e: WheelEvent) {
    const c = cam();
    if (!c) return;
    e.preventDefault();
    c.zoom = Math.max(0.5, Math.min(4, c.zoom * (e.deltaY < 0 ? 1.1 : 1 / 1.1)));
    c.draw();
  }

  // ---------------------------------------------------------------- lifecycle

  let markLoaded = () => {};
  const loaded = new Promise<void>((r) => (markLoaded = r));

  onMount(async () => {
    try {
      model = givenModel ?? ((await App.AccessoryModelFor(kind, base)) as unknown as Model); // tabletop items: per base
      S = netScale(model);
      // Face-net models: the Faces or the Texture tab, as last used (Faces by default).
      view = model.palette || model.projected ? '3d' : readPref('texEditor.view') === 'texture' ? 'texture' : 'net';
      if (!layout || layout.version !== 2) layout = newLayout();
      ownLayout = layout;
      hist.reset();
      if (!model.palette && !model.projected) {
        uvOutlines(model).then((o) => { outlines = o; drawTexView(); }).catch(() => {});
        await loadIslands(model).catch(() => []); // face areas take in their rims (placement buttons, new layers)
      }
      await loadVanilla();
      if (!model.palette && !model.projected) { paintedToLayer(); fillsToRects(); edgesToInclusive(); }
    } catch (e) { notify(errText(e), 'error'); }
    markLoaded();
  });

  /** Face-net models: a painted texture (Import painted texture…, or an EPL mod's own texture) is an ordinary image layer at
   *  the bottom, covering the whole texture — it shows in every guides mode and can be moved, hidden or covered like any
   *  layer; the texture keeps its resolution (renderTexture sizes it to the layer). */
  function paintedToLayer() {
    if (!mapModel() || !layout.textureFile) return;
    layout.layers = [textureLayer(layout.textureFile), ...layout.layers];
    delete layout.textureFile;
    converted();
  }

  /** After a conversion on opening: the new form is where this editor starts (no undo step, no unsaved change); it is
   *  stored with the next save. */
  function converted() {
    ownLayout = layout;
    hist.reset();
    onbaseline?.();
    redraw();
  }

  /** Face-net models: older whole-face colours become colour rectangles (moved and sized like an image), looking the same. */
  function fillsToRects() {
    const out = mapModel() ? faceFillsToRects(model!, layout.layers) : null;
    if (!out) return;
    layout.layers = out;
    if (!layout.layers.some((l) => l.id === selected)) selected = '';
    converted();
  }

  /** Face-net models: texture layers placed on an area before areas were inclusive reach its edge again (no 1 px seam). */
  function edgesToInclusive() {
    const out = mapModel() ? layersToInclusive(model!, layout.layers) : null;
    if (!out) return;
    layout.layers = out;
    converted();
  }

  /** An image layer covering the whole game texture (texture px, the base texture's shape). */
  function textureLayer(rel: string): Layer {
    const T = model!.textureSize, [w, h] = textureSize(), th = (T * h) / w;
    return { id: layerId(), kind: 'image', name: `Texture (${rel.split('/').pop()})`, visible: true, opacity: 1, blend: 'source-over',
      space: 'texture', x: T / 2, y: th / 2, w: T, h: th, rot: 0, src: rel };
  }

  async function loadVanilla() {
    vanilla = null;
    if (vanillaUrl) { try { vanilla = await loadImage(vanillaUrl); } catch { vanilla = null; } }
    readSwatches();
    redraw();
  }

  // Reload when the base model (vanilla art) changes; redraw when switching views (canvases re-mount).
  let lastVanilla = '';
  $effect(() => {
    const v = vanillaUrl;
    if (v !== lastVanilla && untrack(() => model)) { lastVanilla = v; untrack(loadVanilla); }
  });
  $effect(() => {
    view; showGuides; netView; texView;
    untrack(() => {
      if (model?.projected && view === 'texture' && !textureCanvas) textureCanvas = atlasNow();
      if (view !== 'net') ensureTexture();
      drawNetView(); drawTexView();
      zoomPct = zpMain()?.percent() ?? 100;
    });
  });
  $effect(() => { if (model && !model.projected && !model.palette && view !== '3d') writePref('texEditor.view', view); });
  // Guides only needs its own render (and drops it when off).
  let lastGuides = untrack(() => guides);
  $effect(() => {
    const g = guides;
    writePref('texEditor.guides', g);
    if ((g === 'only') !== (lastGuides === 'only')) untrack(() => { if (g !== 'only') onlyCanvas = null; redraw(); });
    lastGuides = g;
  });

  // ---------------------------------------------------------------- workspace: the editor fills the window, panels docked right

  let editorEl = $state<HTMLElement>();
  let ws = $state(false);
  let wsCtl: ReturnType<typeof fullWindow> | null = null;
  $effect(() => {
    const el = editorEl;
    if (!el) return;
    // The stage changes size: show the whole picture again once the new layout is in place.
    const c = fullWindow(el, (on) => { ws = on; requestAnimationFrame(() => requestAnimationFrame(() => { zpTex?.fit(); zpNet?.fit(); })); });
    wsCtl = c;
    return () => { c.destroy(); if (wsCtl === c) wsCtl = null; ws = false; };
  });
  type Panels = { d3: boolean; layers: boolean; props: boolean };
  let panels = $state<Panels>((() => {
    try { return { d3: true, layers: true, props: true, ...JSON.parse(readPref('texEditor.panels') ?? '{}') }; } catch { return { d3: true, layers: true, props: true }; }
  })());
  function togglePanel(k: keyof Panels) { panels[k] = !panels[k]; writePref('texEditor.panels', JSON.stringify(panels)); }
  let splitEl = $state<HTMLElement>();
  let sceneEl = $state<HTMLElement>();
  const splitOpts = {
    container: () => splitEl, prop: '--split', key: 'texEditor.split', def: 50, unit: '%' as const,
    measure: (e: PointerEvent, r: DOMRect) => ((e.clientX - r.left) / r.width) * 100, min: () => 20, max: () => 80,
  };
  const dockOpts = {
    container: () => editorEl, prop: '--dock', key: 'texEditor.dock', def: 380, unit: 'px' as const,
    measure: (e: PointerEvent, r: DOMRect) => r.right - e.clientX, min: () => 260, max: (r: DOMRect) => r.width * 0.5,
  };
  // The 3D preview opens at a third of the dock's height (whatever the window size); drag to change it while it's open.
  const h3dOpts = {
    container: () => editorEl, prop: '--h3d', def: 30, unit: '%' as const,
    measure: (e: PointerEvent, r: DOMRect) => ((e.clientY - (sceneEl?.getBoundingClientRect().top ?? r.top)) / r.height) * 100,
    min: (r: DOMRect) => (120 / r.height) * 100, max: (r: DOMRect) => ((r.height - 220) / r.height) * 100,
  };

  const BLENDS: [GlobalCompositeOperation, string][] = [
    ['source-over', 'Normal'], ['multiply', 'Multiply (tint)'], ['screen', 'Screen'], ['overlay', 'Overlay'], ['color', 'Colour'], ['soft-light', 'Soft light'],
  ];
</script>

<svelte:window onkeydown={onKey} />

{#snippet viewer()}
  <div class="scene" use:maximizable>
    <canvas class="view3d" bind:this={canvas3d} onpointerdown={spinDown} onpointermove={spinMove} onpointerup={spinUp} onwheel={spinWheel}></canvas>
    {#if meshError}<p class="warn small overlay">{meshError}</p>{/if}
  </div>
  <p class="muted small">{model?.projected ? 'Drag a layer on the model to move it (Shift+click: put the selected layer there); drag elsewhere or with Alt to turn, scroll to zoom.' : "The game's own model with your texture. Drag to turn, scroll to zoom."}</p>
{/snippet}

{#if straightening}
  <Straighten url={accUrl(straightening.straighten?.src ?? straightening.src!)} points={straightening.straighten?.pts ?? null}
    faces={straightenFaces} onapply={applyStraighten} oncancel={() => (straightening = null)} />
{/if}

{#if model && layout && model.palette}
  <div class="editor">
    <div class="stage">
      {@render viewer()}
      <div class="row">
        <canvas class="texview palette" bind:this={texView}></canvas>
        <p class="muted small">The {kind === 'Dice' ? 'dice box' : 'model'} is coloured from this small palette texture — each swatch colours one
          part (dice faces, the glass frame, the box, the dice edges and numbers).</p>
      </div>
    </div>
    <div class="side">
      {#each model.faces as f}
        <div class="layer base">
          <span class="grow name">{f.label}</span>
          <input type="color" value={swatchValue(f.id)} oninput={(e) => setSwatch(f.id, e.currentTarget.value)} />
          <button class="tiny" title="Back to the vanilla colour" disabled={!swatchLayer(f.id)} onclick={() => resetSwatch(f.id)}>↺</button>
        </div>
      {/each}
      <p class="muted small">The shop icon is the vanilla render with these colours swapped in. Parts that share a colour in the "Start from"
        box can't be told apart in its icon — start from a box whose parts differ if the icon matters.</p>
    </div>
  </div>
{:else if model && layout}
  <div class="editor" class:ws class:proj={!!model.projected} class:no3d={!panels.d3} bind:this={editorEl}>
    <div class="stage">
      <div class="row tabs bar">
        {#if model.projected}
          <button class:on={view === 'net'} onclick={() => (view = 'net')}>Views</button>
          <button class:on={view === 'texture'} onclick={() => (view = 'texture')}>Texture (UV map)</button>
          <button class:on={view === '3d'} onclick={() => (view = '3d')}>3D preview</button>
          <div class="grow"></div>
        {:else}
          <span class="seg" title="Faces: one picture per face, drawn into every area of it (e.g. the deck box's tray front and cover). Texture: the game texture itself, for gaps and rims.">
            <button class:on={view === 'net'} onclick={() => (view = 'net')}>Faces</button>
            <button class:on={view === 'texture'} onclick={() => (view = 'texture')}>Texture</button>
          </span>
          <div class="grow"></div>
        {/if}
        {#if view !== '3d'}
          <span class="zoombar" title="Wheel: zoom at the pointer · Space+drag or middle-drag: pan · Ctrl+0: fit · Ctrl+1: actual pixels">
            <button class="tiny" onclick={() => zpMain()?.zoomOut()} title="Zoom out (Ctrl+−)">−</button>
            <span class="small pct">{zoomPct}%</span>
            <button class="tiny" onclick={() => zpMain()?.zoomIn()} title="Zoom in (Ctrl+=)">+</button>
            <button class="tiny" onclick={() => zpMain()?.fit()} title="Show the whole picture (Ctrl+0)">Fit</button>
            <button class="tiny" onclick={() => zpMain()?.oneToOne()} title="One texture pixel per screen pixel (Ctrl+1)">1:1</button>
          </span>
        {/if}
        <label class="small guides" title="Guides only: hides the texture under your layers, so only the outlines, the face areas and your own layers show (the 3D view and the saved texture stay as they are)">Guides
          <select bind:value={guides}>
            <option value="on">On</option>
            <option value="off">Off</option>
            {#if !model.projected}<option value="only">Guides only</option>{/if}
          </select>
        </label>
        <button class="small" class:primary={ws} onclick={() => wsCtl?.toggle()}
          title={ws ? 'Back to the page (Esc)' : 'Fill the whole window: the texture big in the middle, 3D preview, layers and properties docked on the right'}>{ws ? '✕ Close workspace' : '⛶ Workspace'}</button>
      </div>
      {#if !model.projected}
        <!-- The texture you edit and the 3D model side by side: every edit shows on the model right away. -->
        <div class="split" bind:this={splitEl}>
          {#if view === 'net'}
            <div class="texstage" use:zoomStage={'net'}>
              <canvas class="netview" style:cursor={netCursor} bind:this={netView}
                onpointerdown={onDown} onpointermove={onMove} onpointerup={onUp} onwheel={onWheel}></canvas>
            </div>
          {:else}
            <div class="texstage" use:zoomStage={'tex'}>
              <canvas class="texview map" style:cursor={netCursor} bind:this={texView}
                onpointerdown={onDown} onpointermove={onMove} onpointerup={onUp} onwheel={onWheel}></canvas>
            </div>
          {/if}
          <div class="divider v" use:splitter={splitOpts}></div>
          <div class="scene" use:maximizable bind:this={sceneEl}>
            <canvas class="view3d" bind:this={canvas3d} onpointerdown={spinDown} onpointermove={spinMove} onpointerup={spinUp} onwheel={spinWheel}></canvas>
            {#if meshError}<p class="warn small overlay">{meshError}</p>{/if}
          </div>
        </div>
        {#if view === 'net'}
          <p class="muted small">Every face upright as seen from outside: a picture on a face shows in all of its areas on the texture (the
            deck box's front goes on the tray front and the cover). The Fill / Stretch / Copy to buttons put the selected layer on a face;
            an image across {wrapFaces().map((f) => f.label).join('–') || 'the faces'} wraps around. Drag a face's colour onto another face to move it there.
            Drag to move (snaps to face edges, Alt: free),
            corner keeps proportions (Shift: freely). Wheel zooms, Space+drag pans, Ctrl+wheel resizes the selected layer.</p>
        {:else}
          <p class="muted small">Blue lines outline every part of the model, yellow boxes are the faces' areas (guides only). Layers here sit on
            the game texture itself — for gaps and rims the faces don't reach. The corner keeps proportions (Shift: freely), the side handles
            stretch, snapping to the yellow boxes (Alt: free). Wheel zooms, Space+drag pans, Ctrl+wheel resizes the selected layer.</p>
        {/if}
      {:else if view === 'net'}
        <div class="texstage" use:zoomStage={'net'}>
          <canvas class="netview" style:cursor={netCursor} bind:this={netView} onpointerdown={onDown} onpointermove={onMove} onpointerup={onUp} onwheel={onWheel}></canvas>
        </div>
        <p class="muted small">Drag a layer to move it (snaps to face edges/centres; hold Alt for free), drag the blue corner or Ctrl+scroll to resize;
          scroll zooms, Space+drag pans. Every face is shown upright as seen from outside; an image across {wrapFaces().map((f) => f.label).join('–') || 'the faces'} wraps around.</p>
      {:else if view === 'texture'}
        <div class="texstage" use:zoomStage={'tex'}>
          <canvas class="texview" bind:this={texView}></canvas>
        </div>
        <p class="muted small">This is the texture the game gets.</p>
      {:else}
        {@render viewer()}
      {/if}
    </div>

    {#if ws}
      <div class="divider dock" use:splitter={dockOpts}></div>
      {#if !model.projected}
        <button class="sect head3d" onclick={() => togglePanel('d3')}>{panels.d3 ? '▾' : '▸'} 3D preview</button>
        {#if panels.d3}<div class="divider h" use:splitter={h3dOpts}></div>{/if}
      {/if}
    {/if}

    <div class="side">
      {#if ws}<button class="sect" onclick={() => togglePanel('layers')}>{panels.layers ? '▾' : '▸'} Layers</button>{/if}
      {#if !ws || panels.layers}
      <div class="row wrap">
        <button class="small primary" onclick={addImage}>+ Image</button>
        <button class="small" onclick={addFill}>+ Colour</button>
        <button class="small" onclick={addText}>+ Text</button>
        <div class="grow"></div>
        <button class="small" onclick={undo} disabled={!hist.canUndo} title="Undo (Ctrl+Z)">↶</button>
        <button class="small" onclick={redo} disabled={!hist.canRedo} title="Redo (Ctrl+Y)">↷</button>
      </div>
      <div class="layers" bind:this={layersEl}>
        {#each [...layout.layers].reverse() as l, i (l.id)}
          <div class="layer" data-id={l.id} class:active={l.id === selected} class:drop-above={dropAt === i}
            class:drop-below={dropAt === layout.layers.length && i === layout.layers.length - 1}
            role="button" tabindex="0" title="Drag to change the order (top of the list = on top)"
            onclick={() => { if (!noClick) selectLayer(l); }} onkeydown={() => {}}
            onpointerdown={(e) => rowDown(e, l.id)} onpointermove={rowMove} onpointerup={rowUp} onpointercancel={rowUp}>
            <input type="checkbox" bind:checked={l.visible} onchange={changed} onclick={(e) => e.stopPropagation()} title="Visible" />
            <span class="grow name">{l.kind === 'image' ? '🖼' : l.kind === 'text' ? 'T' : '■'} {l.name}</span>
            {#if isMapLayer(l)}<span class="tag" title="On the game texture (Texture tab)">texture</span>{:else if mapModel()}<span class="tag face" title="On a face (Faces tab): shows in every area of the face">face</span>{/if}
            <button class="tiny" onclick={(e) => { e.stopPropagation(); moveLayer(l.id, 1); }} title="Move up">▲</button>
            <button class="tiny" onclick={(e) => { e.stopPropagation(); moveLayer(l.id, -1); }} title="Move down">▼</button>
            <button class="tiny" onclick={(e) => { e.stopPropagation(); duplicateLayer(l.id); }} title="Duplicate (Ctrl+D)">⧉</button>
            <button class="tiny" onclick={(e) => { e.stopPropagation(); removeLayer(l.id); }} title="Delete (Del)">✕</button>
          </div>
        {/each}
        {#if layout.textureFile}
          <div class="layer base" title="Your painted texture, used at its own resolution under the layers">
            <span class="grow name">🎨 Painted: {layout.textureFile.split('/').pop()}</span>
            <button class="tiny" onclick={importPainted} title="Use another painted texture">↻</button>
            <button class="tiny" onclick={removePainted} title="Stop using the painted texture">✕</button>
          </div>
        {:else}
          <div class="layer base">
            <span class="grow name">Base:</span>
            <select bind:value={layout.base} onchange={changed}>
              <option value="vanilla">Vanilla art</option>
              <option value="color">Plain colour</option>
            </select>
            {#if layout.base === 'color'}<input type="color" bind:value={layout.baseColor} oninput={changed} />{/if}
          </div>
        {/if}
      </div>
      <div class="row wrap">
        <button class="small" onclick={exportTemplate}
          title="Saves an image of the game texture at its real size with every face outlined and labelled, to paint your own art on in another program">Export template…</button>
        <button class="small" onclick={importPainted}
          title="Uses a texture you painted on an exported template as the whole texture, at full resolution">Import painted texture…</button>
      </div>
      {/if}

      {#if ws}<button class="sect" onclick={() => togglePanel('props')}>{panels.props ? '▾' : '▸'} Properties{sel ? ` · ${sel.name}` : ''}</button>{/if}
      {#if sel && (!ws || panels.props)}
        <div class="props">
          <label class="field">Name<input bind:value={sel.name} /></label>
          {#if sel.kind === 'text'}
            <label class="field">Text<input bind:value={sel.text} oninput={changed} /></label>
            <div class="row">
              <label class="field">Colour<input type="color" bind:value={sel.color} oninput={changed} /></label>
              <label class="field">Outline<input type="color" value={sel.outline || '#000000'} oninput={(e) => { sel.outline = e.currentTarget.value; changed(); }} /></label>
              <label class="check small"><input type="checkbox" checked={!!sel.outline} onchange={(e) => { sel.outline = e.currentTarget.checked ? '#000000' : ''; changed(); }} /> outline</label>
            </div>
          {/if}
          {#if sel.kind === 'fill'}
            {#if wholeFaceFill(sel)}<label class="field">Face
              <select bind:value={sel.face} onchange={changed}>
                <option value="">All faces</option>
                {#each pickFaces as f}<option value={f.id}>{f.label}</option>{/each}
              </select>
            </label>{/if}
            <div class="row">
              <label class="field">Colour<input type="color" bind:value={sel.color} oninput={changed} /></label>
              <label class="check small"><input type="checkbox" checked={!!sel.color2} onchange={(e) => { sel.color2 = e.currentTarget.checked ? '#000000' : ''; changed(); }} /> gradient</label>
              {#if sel.color2}<label class="field">to<input type="color" bind:value={sel.color2} oninput={changed} /></label>{/if}
            </div>
          {/if}
          <div class="row">
            <label class="field grow">Blend
              <select bind:value={sel.blend} onchange={changed}>{#each BLENDS as [v, t]}<option value={v}>{t}</option>{/each}</select>
            </label>
            <label class="field" style="width:90px">Opacity %<input type="number" min="0" max="100" step="5" value={Math.round(sel.opacity * 100)} oninput={(e) => { sel.opacity = Math.max(0, Math.min(100, +e.currentTarget.value)) / 100; changed(); }} /></label>
          </div>
          {#if sel.kind === 'image' && sel.src}
            <div class="row wrap">
              <button class="tiny" onclick={() => (straightening = sel!)} title="Photo taken at an angle? Pick the corners of the surface and flatten it">
                {sel.straighten ? 'Adjust straighten…' : 'Straighten…'}</button>
              {#if sel.straighten}<button class="tiny" onclick={() => undoStraighten(sel!)}>Use original photo</button>{/if}
            </div>
          {/if}
          {#if isMapLayer(sel)}
            <label class="field">Rotation {Math.round(sel.rot)}°<input type="range" min="-180" max="180" step="1" bind:value={sel.rot} oninput={changed} /></label>
            <div class="row size">
              <label class="field" title="Texture pixels">Width px<input type="number" min="1" step="1" value={Math.round(sel.w)}
                oninput={(e) => { if (+e.currentTarget.value > 0) { sel.w = +e.currentTarget.value; changed(); } }} /></label>
              <label class="field" title="Texture pixels">Height px<input type="number" min="1" step="1" value={Math.round(sel.h)}
                oninput={(e) => { if (+e.currentTarget.value > 0) { sel.h = +e.currentTarget.value; changed(); } }} /></label>
              {#if sel.kind === 'image'}<button class="tiny" onclick={resetProportions} title="Back to the image's own proportions (keeps the width)">Unstretch</button>{/if}
            </div>
            {#if sel.clip}
              <div class="row">
                <span class="small muted grow">Shows only inside its face's area (from a design made on the faces).</span>
                <button class="tiny" onclick={() => { delete sel!.clip; changed(); }} title="Show the whole layer">Unclip</button>
              </div>
            {/if}
            {#if sel.kind === 'image'}
              <div class="small muted">Fill (keeps proportions, crops the overflow):</div>
              <div class="row wrap">{#each model.faces as f}<button class="tiny" onclick={() => placeSelected(f.id, 'cover')}>{f.label}</button>{/each}</div>
              <div class="small muted">Fit inside (whole picture):</div>
              <div class="row wrap">{#each model.faces as f}<button class="tiny" onclick={() => placeSelected(f.id, 'contain')}>{f.label}</button>{/each}</div>
            {/if}
            <div class="small muted">Stretch over (fills the area exactly):</div>
            <div class="row wrap">{#each model.faces as f}<button class="tiny" onclick={() => placeSelected(f.id, 'stretch')}>{f.label}</button>{/each}</div>
            <p class="muted small">Keys: Ctrl+C / Ctrl+V copy and paste a layer, Ctrl+D duplicate, Del delete, Ctrl+Z / Ctrl+Y undo and redo.</p>
          {:else if !wholeFaceFill(sel)}
            <label class="field">Rotation {Math.round(sel.rot)}°<input type="range" min="-180" max="180" step="1" bind:value={sel.rot} oninput={changed} /></label>
            <div class="row size">
              <label class="field" title="% of the {mainFace().label.toLowerCase()} width">Width %<input type="number" min="1" step="1"
                value={Math.round((sel.w / shownNet(mainFace())[2]) * 100)} oninput={(e) => setSize('w', +e.currentTarget.value)} /></label>
              <label class="field" title="% of the {mainFace().label.toLowerCase()} height">Height %<input type="number" min="1" step="1"
                value={Math.round((sel.h / shownNet(mainFace())[3]) * 100)} oninput={(e) => setSize('h', +e.currentTarget.value)} /></label>
              {#if sel.kind === 'image'}<button class="tiny" onclick={resetProportions} title="Back to the image's own proportions (keeps the width)">Unstretch</button>{/if}
            </div>
            <div class="small muted">Fill (keeps proportions, crops the overflow):</div>
            <div class="row wrap">
              {#if wrapFaces().length > 1}<button class="tiny" onclick={() => fitSelected('_wrap', 'cover')} title={wrapFaces().map((f) => f.label).join(' + ') + ' as one panorama'}>Wrap around</button>{/if}
              {#each pickFaces as f}<button class="tiny" onclick={() => fitSelected(f.id, 'cover')}>{f.label}</button>{/each}
            </div>
            <div class="small muted">Stretch to (fills exactly, may distort):</div>
            <div class="row wrap">
              {#if wrapFaces().length > 1}<button class="tiny" onclick={() => fitSelected('_wrap', 'stretch')}>Wrap around</button>{/if}
              {#each pickFaces as f}<button class="tiny" onclick={() => fitSelected(f.id, 'stretch')}>{f.label}</button>{/each}
            </div>
            <div class="row wrap">
              <button class="tiny" onclick={() => fitSelected(mainFace().id, 'contain')}>Fit inside {mainFace().label.toLowerCase()}</button>
            </div>
            <div class="small muted">Copy to (same place on another face; uses the same image file):</div>
            <div class="row wrap">
              {#if oppositeFace}<button class="tiny primary" onclick={() => copyToFaces([oppositeFace!.id])}
                title="The same place on the opposite side">Opposite: {oppositeFace.label}</button>{/if}
              {#each pickFaces.filter((f) => !f.hidden && f.id !== faceOf(sel!).id) as f}<button class="tiny" onclick={() => copyToFaces([f.id])}>{f.label}</button>{/each}
            </div>
            <p class="muted small">Drag the white side handles to stretch one side; the blue corner keeps proportions (hold Shift to stretch freely).
              Keys: Ctrl+C / Ctrl+V copy and paste a layer, Ctrl+D duplicate, Del delete, Ctrl+Z / Ctrl+Y undo and redo.</p>
          {/if}
        </div>
      {:else if !ws || panels.props}
        <p class="muted small">Add an image, colour or text. Tip: a colour layer with <b>Multiply</b> over the vanilla base tints the leather and keeps its grain.</p>
      {/if}
    </div>
  </div>
{:else}
  <p class="muted">Loading model…</p>
{/if}

<style>
  .editor { display: flex; gap: 14px; align-items: flex-start; }
  /* A container: texture and 3D stack when the editor itself is narrow (a smaller window), not only below a window width. */
  .stage { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 8px; container-type: inline-size; }
  .tabs button.on { border-color: var(--accent); background: #22304d; }
  .seg button { padding: 3px 12px; }
  .tag { font-size: 11px; padding: 0 5px; border-radius: 4px; background: #2b3a55; color: #9cc4ff; }
  .tag.face { background: #3a3352; color: #d2b8ff; }
  .texview.palette { image-rendering: pixelated; width: 128px; height: 128px; flex-shrink: 0; }
  .netview, .texview { width: 100%; max-height: 62vh; object-fit: contain; background: repeating-conic-gradient(#1b1f27 0 25%, #232834 0 50%) 0 0 / 24px 24px; border-radius: 6px; }
  .netview { cursor: move; touch-action: none; }
  /* Zoom stage (lib/zoomPan places the canvas inside). */
  .texstage { position: relative; overflow: hidden; height: 62vh; min-height: 320px; background: #12151b; border-radius: 6px; }
  .texstage .netview, .texstage .texview { border-radius: 0; }
  .zoombar { display: inline-flex; align-items: center; gap: 3px; }
  .zoombar .pct { min-width: 44px; text-align: center; font-variant-numeric: tabular-nums; }
  .guides { display: inline-flex; align-items: center; gap: 4px; }
  .guides select { padding: 1px 4px; font-size: 12px; }
  .divider { background: transparent; border-radius: 3px; touch-action: none; }
  .divider:hover, .divider:global(.dragging) { background: var(--accent); opacity: 0.6; }
  .divider.v, .divider.dock { cursor: col-resize; }
  .divider.h { cursor: row-resize; }
  .scene { position: relative; height: 440px; background: radial-gradient(circle at 50% 40%, #2a3140, var(--bg)); border-radius: 6px; }
  /* Out of the layout flow: MeshView sizes the canvas bitmap to its box × devicePixelRatio, and an in-flow canvas would
     then grow its auto-height row on every resize (the model kept zooming in with Windows scaling > 100 %). */
  .view3d { position: absolute; inset: 0; width: 100%; height: 100%; display: block; cursor: grab; touch-action: none; }
  .view3d:active { cursor: grabbing; }
  .overlay { position: absolute; left: 12px; bottom: 4px; margin: 0; }
  .side { width: 300px; flex-shrink: 0; display: flex; flex-direction: column; gap: 10px; }
  .layers { display: flex; flex-direction: column; gap: 3px; max-height: 240px; overflow: auto; }
  .layer { display: flex; align-items: center; gap: 6px; padding: 4px 6px; border: 1px solid var(--line); border-radius: 6px; cursor: pointer; background: var(--panel); }
  .layer.active { border-color: var(--accent); background: #22304d; }
  .layer { touch-action: none; }
  .layer.drop-above { box-shadow: 0 -3px 0 0 var(--accent); }
  .layer.drop-below { box-shadow: 0 3px 0 0 var(--accent); }
  .layer.base { cursor: default; }
  .texview.map { touch-action: none; }
  .split { display: grid; grid-template-columns: minmax(0, var(--split, 50%)) 8px minmax(0, 1fr); gap: 0; align-items: stretch; }
  .split .scene { height: auto; min-height: 320px; }
  @container (max-width: 760px) { .split { grid-template-columns: 1fr; gap: 8px; } .split > .divider { display: none; } .split .scene { height: 360px; } }
  .bar { flex-wrap: wrap; row-gap: 6px; }
  .bar > .grow { min-width: 120px; }

  /* Workspace: the editor fills the window (lib/maximize fullWindow). The stage and split boxes step aside (display: contents)
     so the texture, the 3D view and the side panel become cells of one grid; nothing is re-mounted. */
  .editor.ws { display: grid; gap: 0; padding: 8px 10px 10px; background: var(--bg); align-items: stretch;
    grid-template-columns: minmax(0, 1fr) 8px var(--dock, 380px);
    grid-template-rows: auto auto var(--h3d, 30%) 8px minmax(0, 1fr);
    grid-template-areas: 'bar bar bar' 'main vdiv head3d' 'main vdiv scene' 'main vdiv hdiv' 'main vdiv side'; }
  .editor.ws.no3d { grid-template-rows: auto auto 0 0 minmax(0, 1fr); }
  .editor.ws.proj { grid-template-rows: auto minmax(0, 1fr); grid-template-areas: 'bar bar bar' 'main vdiv side'; }
  .ws .stage, .ws .split { display: contents; }
  .ws .bar { grid-area: bar; padding-bottom: 8px; margin-bottom: 8px; border-bottom: 1px solid var(--line); }
  .ws .texstage { grid-area: main; height: auto; min-height: 0; }
  .ws .split > .divider, .ws .stage > p { display: none; }
  .ws .scene { grid-area: scene; height: auto; min-height: 0; }
  .ws.proj .scene { grid-area: main; }
  .ws.no3d .scene { display: none; }
  .ws > .divider.dock { grid-area: vdiv; margin: 0 2px; }
  .ws > .divider.h { grid-area: hdiv; margin: 2px 0; }
  .ws > .head3d { grid-area: head3d; }
  .ws .side { grid-area: side; width: auto; min-height: 0; overflow: auto; padding-right: 2px; }
  .ws.proj .side { padding-top: 0; }
  .ws .layers { max-height: none; }
  .ws .side > * { flex-shrink: 0; } /* the dock scrolls; the layer list must not shrink to nothing */
  .sect { text-align: left; font-weight: 600; font-size: 13px; padding: 4px 8px; background: var(--panel); border: 1px solid var(--line); border-radius: 6px; }
  .ws > .head3d { margin-bottom: 6px; }
  .layer input[type='color'] { width: 36px; height: 24px; padding: 1px; }
  .name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 13px; }
  .props { display: flex; flex-direction: column; gap: 8px; background: var(--panel); border: 1px solid var(--line); border-radius: var(--radius); padding: 10px; }
  .props input[type='color'] { width: 44px; height: 28px; padding: 2px; }
  .wrap { flex-wrap: wrap; }
  .size { align-items: flex-end; gap: 6px; }
  .size .field { flex: 1; min-width: 0; }
  .size input { width: 100%; min-width: 0; box-sizing: border-box; }
</style>
