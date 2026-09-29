<script lang="ts">
  // Face-by-face editor for deck boxes / playmats: the model unfolded into a net, layers placed on it (images can span faces),
  // mapped into the game texture through the model's targets. Views: net, the raw texture (UV map) and a 3D preview of the
  // real game mesh.
  import { onMount, untrack } from 'svelte';
  import { App, errText } from '../lib/api';
  import {
    type Model, type Layout, type Layer, newLayout, netBounds, netScale, faceRect, renderNet, composeTexture, composeIcon,
    recolorIcon, loadImage, layerId,
  } from '../lib/accessoryArt';
  import { MeshView } from '../lib/meshView';
  import Straighten from './Straighten.svelte';

  let {
    kind, base = '', vanillaUrl, vanillaIconUrl = '', layout = $bindable(), iconSize = [512, 512], notify, onchange,
    imageUrl, pickImage, saveImage, defaultText = 'DECK BOX',
  }: {
    kind: string;
    base?: string;                      // vanilla item it starts from (for its second texture in the 3D preview)
    vanillaUrl: string;                 // vanilla texture of the chosen base ('' when not exported)
    vanillaIconUrl?: string;            // vanilla shop icon of the base (palette models recolour it)
    layout: Layout;
    iconSize?: number[];
    notify: (t: string, k?: string) => void;
    onchange: () => void;               // any edit (marks the accessory dirty)
    imageUrl?: (rel: string) => string; // URL of a layer image (default: the accessory library)
    pickImage?: () => Promise<string>;  // lets the user add an image, returns its path for imageUrl (default: accessory library)
    saveImage?: (name: string, dataUrl: string) => Promise<string>; // stores a made image (straightened), returns its path
    defaultText?: string;               // text of a new text layer
  } = $props();

  let model = $state<Model | null>(null);
  let S = 300;
  let vanilla: HTMLImageElement | null = null;
  let net: HTMLCanvasElement = document.createElement('canvas');
  let view = $state<'net' | 'texture' | '3d'>('net');
  let selected = $state('');
  let showGuides = $state(true);

  // Display canvases
  let netView = $state<HTMLCanvasElement>();
  let texView = $state<HTMLCanvasElement>();
  let textureCanvas: HTMLCanvasElement | null = null;

  const accUrl = (rel: string) => imageUrl ? imageUrl(rel) : `/acc/${rel.split('/').map(encodeURIComponent).join('/')}`;
  let sel = $derived(layout?.layers.find((l) => l.id === selected));

  // ---------------------------------------------------------------- rendering

  let rendering = false, again = false;
  async function redraw() {
    if (!model) return;
    if (rendering) { again = true; return; }
    rendering = true;
    try {
      do {
        again = false;
        await renderNet(net, model, layout, vanilla, accUrl, S);
        drawNetView();
        textureCanvas = composeTexture(model, net, vanilla, S);
        drawTexView();
        mesh?.setTexture(textureCanvas);
      } while (again);
    } catch (e) { notify(errText(e), 'error'); }
    rendering = false;
  }

  function drawNetView() {
    if (!netView || !model) return;
    netView.width = net.width; netView.height = net.height;
    const ctx = netView.getContext('2d')!;
    ctx.clearRect(0, 0, net.width, net.height);
    ctx.drawImage(net, 0, 0);
    const lw = Math.max(2, net.width / 500);
    if (showGuides) {
      ctx.font = `bold ${Math.round(net.width / 45)}px Nunito, sans-serif`;
      for (const f of model.faces) {
        const r = faceRect(model, f, S);
        ctx.strokeStyle = 'rgba(255,255,255,0.7)';
        ctx.setLineDash([lw * 4, lw * 3]);
        ctx.lineWidth = lw;
        ctx.strokeRect(r.x, r.y, r.w, r.h);
        ctx.setLineDash([]);
        ctx.fillStyle = 'rgba(0,0,0,0.55)';
        const label = f.label + (f.hidden ? ' (hidden)' : '');
        const tw = ctx.measureText(label).width;
        ctx.fillRect(r.x + lw * 3, r.y + lw * 3, tw + lw * 6, net.width / 32);
        ctx.fillStyle = '#fff';
        ctx.fillText(label, r.x + lw * 6, r.y + lw * 3 + net.width / 45);
      }
    }
    if (sel && sel.kind !== 'fill') {
      const b = netBounds(model);
      ctx.save();
      ctx.translate((sel.x - b.x0) * S, (sel.y - b.y0) * S);
      ctx.rotate((sel.rot * Math.PI) / 180);
      const w = sel.w * S, h = sel.h * S;
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
  }

  function drawTexView() {
    if (!texView || !textureCanvas || !model) return;
    texView.width = textureCanvas.width; texView.height = textureCanvas.height;
    const ctx = texView.getContext('2d')!;
    ctx.drawImage(textureCanvas, 0, 0);
    if (!showGuides) return;
    const k = textureCanvas.width / model.textureSize;
    ctx.font = `bold ${Math.round(18 * k)}px Nunito, sans-serif`;
    for (const f of model.faces) for (const t of f.targets) {
      if (t.bleed) continue;
      ctx.strokeStyle = f.hidden ? 'rgba(255,255,255,0.4)' : '#ffd24a';
      ctx.lineWidth = 2 * k;
      ctx.strokeRect(t.rect[0] * k, t.rect[1] * k, (t.rect[2] - t.rect[0]) * k, (t.rect[3] - t.rect[1]) * k);
      ctx.fillStyle = 'rgba(0,0,0,0.6)';
      const label = f.label + (t.src[0] > 0 || t.src[2] < 1 ? (t.src[0] === 0 ? ' (left half)' : ' (right half)') : '') + (t.src[1] > 0 ? ' — cover' : '');
      const tw = ctx.measureText(label).width;
      ctx.fillRect(t.rect[0] * k + 4, t.rect[1] * k + 4, tw + 8, 24 * k);
      ctx.fillStyle = '#fff';
      ctx.fillText(label, t.rect[0] * k + 8, t.rect[1] * k + 22 * k);
    }
  }

  /** PNG data URLs of the texture and the shop icon for saving. */
  export async function exportImages(): Promise<{ texture: string; icon: string }> {
    await redraw();
    if (!model || !textureCanvas) throw new Error('editor not ready');
    let icon: HTMLCanvasElement;
    const texture = textureCanvas.toDataURL('image/png');
    // Packs: the vanilla pack render with the front warped in (Go side, like the generator).
    if (model.icon === 'pack') return { texture, icon: await App.PackIconFromTexture(texture) };
    if (model.icon === 'recolor') {
      if (!vanilla || !vanillaIconUrl) throw new Error("the vanilla icon is needed to recolour it — the game templates aren't available yet (Settings → Game)");
      icon = recolorIcon(model, net, S, vanilla, await loadImage(vanillaIconUrl));
    } else icon = composeIcon(model, net, S, iconSize[0], iconSize[1]);
    return { texture, icon: icon.toDataURL('image/png') };
  }

  function changed() {
    onchange();
    redraw();
  }

  // ---------------------------------------------------------------- layers

  function faceById(id: string) { return model?.faces.find((f) => f.id === id); }
  const mainFace = () => (faceById('front') ?? faceById('surface') ?? model?.faces[0])!;

  /** Faces in the main row (same height as the main face): the deck box's four sides, a book's back + spine + front. */
  function wrapFaces() {
    const m = mainFace();
    return (model?.faces ?? []).filter((f) => !f.hidden && f.net[1] === m.net[1] && f.net[3] === m.net[3]);
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
    if (row.length < 2) return mainFace().net;
    const x0 = Math.min(...row.map((f) => f.net[0])), x1 = Math.max(...row.map((f) => f.net[0] + f.net[2]));
    return [x0, row[0].net[1], x1 - x0, row[0].net[3]];
  }

  async function addImage() {
    try {
      const rel = await (pickImage ? pickImage() : App.PickAccessoryImage());
      if (!rel) return;
      const img = await loadImage(accUrl(rel));
      const l: Layer = { id: layerId(), kind: 'image', name: rel.split('/').pop() || 'Image', visible: true, opacity: 1, blend: 'source-over',
        x: 0, y: 0, w: 1, h: 1, rot: 0, src: rel };
      fitInto(l, mainFace().net, 'cover', img.width / img.height);
      layout.layers.push(l);
      selected = l.id;
      changed();
    } catch (e) { notify(errText(e), 'error'); }
  }

  function addFill() {
    const l: Layer = { id: layerId(), kind: 'fill', name: 'Colour', visible: true, opacity: 1, blend: 'source-over',
      x: 0, y: 0, w: 0, h: 0, rot: 0, face: '', color: '#2a6df4', color2: '' };
    layout.layers.push(l);
    selected = l.id;
    changed();
  }

  function addText() {
    const f = mainFace();
    const l: Layer = { id: layerId(), kind: 'text', name: 'Text', visible: true, opacity: 1, blend: 'source-over',
      x: f.net[0] + f.net[2] / 2, y: f.net[1] + f.net[3] * 0.15, w: f.net[2] * 0.8, h: f.net[3] * 0.08, rot: 0,
      text: defaultText, color: '#ffffff', outline: '#000000' };
    layout.layers.push(l);
    selected = l.id;
    changed();
  }

  function removeLayer(id: string) {
    layout.layers = layout.layers.filter((l) => l.id !== id);
    if (selected === id) selected = '';
    changed();
  }

  function moveLayer(id: string, delta: number) {
    const i = layout.layers.findIndex((l) => l.id === id), j = i + delta;
    if (i < 0 || j < 0 || j >= layout.layers.length) return;
    const list = [...layout.layers];
    [list[i], list[j]] = [list[j], list[i]];
    layout.layers = list;
    changed();
  }

  function setSize(dim: 'w' | 'h', pct: number) {
    if (!sel || !(pct > 0)) return;
    const f = mainFace();
    if (dim === 'w') sel.w = (pct / 100) * f.net[2];
    else sel.h = (pct / 100) * f.net[3];
    changed();
  }

  async function resetProportions() {
    if (!sel?.src) return;
    try { const img = await loadImage(accUrl(sel.src)); sel.h = (sel.w * img.height) / img.width; changed(); } catch { /* keep */ }
  }

  async function fitSelected(target: string, mode: 'cover' | 'contain' | 'stretch') {
    if (!sel || !model) return;
    const area = target === '_wrap' ? wrapArea() : faceById(target)?.net;
    if (!area) return;
    let aspect = sel.w / sel.h;
    if (sel.kind === 'image' && sel.src) { try { const img = await loadImage(accUrl(sel.src)); aspect = img.width / img.height; } catch { /* keep */ } }
    if (mode === 'stretch') { sel.x = area[0] + area[2] / 2; sel.y = area[1] + area[3] / 2; sel.w = area[2]; sel.h = area[3]; sel.rot = 0; }
    else fitInto(sel, area, mode, aspect);
    changed();
  }

  // ---------------------------------------------------------------- straighten (perspective correction of a photo)

  let straightening = $state<Layer | null>(null);
  const straightenFaces = $derived((model?.faces ?? []).filter((f) => !f.hidden).map((f) => ({ label: f.label.toLowerCase(), aspect: f.net[2] / f.net[3] })));

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
  let drag: { mode: 'move' | 'scale' | 'edge'; hx: number; hy: number; sx: number; sy: number; x: number; y: number; w: number; h: number } | null = null;
  let netCursor = $state('move');

  /** The handle of the selected layer under p (index into HANDLES), or -1. */
  function handleAt(p: { x: number; y: number }): number {
    if (!sel || sel.kind === 'fill') return -1;
    const q = local(sel, p), hs = (Math.max(2, net.width / 500) * 7) / S;
    return HANDLES.findIndex(([hx, hy]) => Math.abs(q.x - hx * sel!.w) < hs * 1.5 && Math.abs(q.y - hy * sel!.h) < hs * 1.5);
  }

  function toNet(e: PointerEvent | WheelEvent) {
    const r = netView!.getBoundingClientRect();
    const b = netBounds(model!);
    // The canvas is shown with object-fit: contain — a tall net (the pack) is letterboxed inside the element's box.
    const k = Math.min(r.width / net.width, r.height / net.height);
    const ox = r.left + (r.width - net.width * k) / 2, oy = r.top + (r.height - net.height * k) / 2;
    return { x: (e.clientX - ox) / k / S + b.x0, y: (e.clientY - oy) / k / S + b.y0 };
  }

  function local(l: Layer, p: { x: number; y: number }) {
    const a = (-l.rot * Math.PI) / 180, dx = p.x - l.x, dy = p.y - l.y;
    return { x: dx * Math.cos(a) - dy * Math.sin(a), y: dx * Math.sin(a) + dy * Math.cos(a) };
  }

  function hit(p: { x: number; y: number }): Layer | undefined {
    for (let i = layout.layers.length - 1; i >= 0; i--) {
      const l = layout.layers[i];
      if (!l.visible || l.kind === 'fill') continue;
      const q = local(l, p);
      if (Math.abs(q.x) <= l.w / 2 && Math.abs(q.y) <= l.h / 2) return l;
    }
  }

  function onDown(e: PointerEvent) {
    if (!model) return;
    const p = toNet(e);
    const hi = handleAt(p);
    if (hi >= 0 && sel) {
      const [hx, hy, corner] = HANDLES[hi];
      drag = { mode: corner ? 'scale' : 'edge', hx, hy, sx: p.x, sy: p.y, x: sel.x, y: sel.y, w: sel.w, h: sel.h };
      netView!.setPointerCapture(e.pointerId);
      return;
    }
    const l = hit(p);
    selected = l?.id ?? (sel?.kind === 'fill' ? selected : '');
    if (l) {
      drag = { mode: 'move', hx: 0, hy: 0, sx: p.x, sy: p.y, x: l.x, y: l.y, w: l.w, h: l.h };
      netView!.setPointerCapture(e.pointerId);
    }
    drawNetView();
  }

  function onMove(e: PointerEvent) {
    if (!drag) {
      // Hover: resize cursors over the handles.
      const hi = model ? handleAt(toNet(e)) : -1;
      netCursor = hi < 0 ? 'move' : HANDLES[hi][2] ? 'nwse-resize' : HANDLES[hi][0] ? 'ew-resize' : 'ns-resize';
      return;
    }
    if (!sel) return;
    const p = toNet(e);
    if (drag.mode === 'move') {
      sel.x = drag.x + (p.x - drag.sx);
      sel.y = drag.y + (p.y - drag.sy);
      // Snap the centre to face centres/edges when close (hold Alt to move freely).
      if (!e.altKey && model) {
        const tol = 0.03 * Math.max(model.size[0], model.size[1]);
        for (const f of model.faces) {
          for (const cx of [f.net[0], f.net[0] + f.net[2] / 2, f.net[0] + f.net[2]]) if (Math.abs(sel.x - cx) < tol) sel.x = cx;
          for (const cy of [f.net[1], f.net[1] + f.net[3] / 2, f.net[1] + f.net[3]]) if (Math.abs(sel.y - cy) < tol) sel.y = cy;
        }
      }
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
      const a = (sel.rot * Math.PI) / 180, min = 0.01;
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
    onchange();
    redraw();
  }

  function onUp() { drag = null; }

  function onWheel(e: WheelEvent) {
    if (!sel || sel.kind === 'fill') return;
    e.preventDefault();
    const f = e.deltaY < 0 ? 1.05 : 1 / 1.05;
    sel.w *= f; sel.h *= f;
    changed();
  }

  // ---------------------------------------------------------------- 3D preview: the real game mesh (templates\accessories\*.obj)

  let canvas3d = $state<HTMLCanvasElement>();
  let mesh: MeshView | null = null;
  let meshError = $state('');
  const tpl = (file: string) => `/acctemplates/${encodeURIComponent(file)}`;

  $effect(() => {
    const c = canvas3d;
    if (!c || !model) return;
    let mv: MeshView;
    try { mv = new MeshView(c, !!model.palette); } catch (e) { meshError = errText(e); return; }
    mesh = mv;
    meshError = '';
    if (model.kind === 'Playmat') { mv.rx = 0.85; mv.ry = -0.25; }
    const parts = ((model as any).preview ?? []).map((p: any) => ({
      url: tpl(p.mesh + '.obj'), texture: p.texture, glass: p.glass, secondaryUrl: base ? tpl(`${base}_texture2.png`) : undefined,
    }));
    mv.load(parts).then(() => { if (textureCanvas) mv.setTexture(textureCanvas); })
      .catch((e) => (meshError = errText(e) + " — the game templates aren't available yet (Settings → Game)"));
    return () => { mv.dispose(); if (mesh === mv) mesh = null; };
  });

  let spin: { x: number; y: number; rx: number; ry: number } | null = null;
  function spinDown(e: PointerEvent) {
    if (!mesh) return;
    spin = { x: e.clientX, y: e.clientY, rx: mesh.rx, ry: mesh.ry };
    (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
  }
  function spinMove(e: PointerEvent) {
    if (!spin || !mesh) return;
    mesh.ry = spin.ry + (e.clientX - spin.x) * 0.01;
    mesh.rx = Math.max(-1.55, Math.min(1.55, spin.rx + (e.clientY - spin.y) * 0.01));
    mesh.draw();
  }
  function spinWheel(e: WheelEvent) {
    if (!mesh) return;
    e.preventDefault();
    mesh.zoom = Math.max(0.5, Math.min(4, mesh.zoom * (e.deltaY < 0 ? 1.1 : 1 / 1.1)));
    mesh.draw();
  }

  // ---------------------------------------------------------------- lifecycle

  onMount(async () => {
    try {
      model = (await App.AccessoryModel(kind)) as unknown as Model;
      S = netScale(model);
      if (model.palette) view = '3d';
      if (!layout || layout.version !== 2) layout = newLayout();
      await loadVanilla();
    } catch (e) { notify(errText(e), 'error'); }
  });

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
    untrack(() => { drawNetView(); drawTexView(); });
  });

  const BLENDS: [GlobalCompositeOperation, string][] = [
    ['source-over', 'Normal'], ['multiply', 'Multiply (tint)'], ['screen', 'Screen'], ['overlay', 'Overlay'], ['color', 'Colour'], ['soft-light', 'Soft light'],
  ];
</script>

{#snippet viewer()}
  <div class="scene">
    <canvas class="view3d" bind:this={canvas3d} onpointerdown={spinDown} onpointermove={spinMove} onpointerup={() => (spin = null)} onwheel={spinWheel}></canvas>
    {#if meshError}<p class="warn small overlay">{meshError}</p>{/if}
  </div>
  <p class="muted small">The game's own model with your texture. Drag to turn, scroll to zoom.</p>
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
  <div class="editor">
    <div class="stage">
      <div class="row tabs">
        <button class:on={view === 'net'} onclick={() => (view = 'net')}>Faces</button>
        <button class:on={view === 'texture'} onclick={() => (view = 'texture')}>Texture (UV map)</button>
        <button class:on={view === '3d'} onclick={() => (view = '3d')}>3D preview</button>
        <div class="grow"></div>
        <label class="check small"><input type="checkbox" bind:checked={showGuides} /> Guides</label>
      </div>

      {#if view === 'net'}
        <canvas class="netview" style:cursor={netCursor} bind:this={netView} onpointerdown={onDown} onpointermove={onMove} onpointerup={onUp} onwheel={onWheel}></canvas>
        <p class="muted small">Drag a layer to move it (snaps to face edges/centres; hold Alt for free), drag the blue corner or scroll to resize.
          Every face is shown upright as seen from outside; an image across {wrapFaces().map((f) => f.label).join('–') || 'the faces'} wraps around.</p>
      {:else if view === 'texture'}
        <canvas class="texview" bind:this={texView}></canvas>
        <p class="muted small">This is the texture the game gets. Outlined areas are painted from the faces; everything else (insides, edges, the
          playmat's rubber underside, a book's page edges) keeps the vanilla art.</p>
      {:else}
        {@render viewer()}
      {/if}
    </div>

    <div class="side">
      <div class="row wrap">
        <button class="small primary" onclick={addImage}>+ Image</button>
        <button class="small" onclick={addFill}>+ Colour</button>
        <button class="small" onclick={addText}>+ Text</button>
      </div>
      <div class="layers">
        {#each [...layout.layers].reverse() as l (l.id)}
          <div class="layer" class:active={l.id === selected} role="button" tabindex="0" onclick={() => { selected = l.id; drawNetView(); }} onkeydown={() => {}}>
            <input type="checkbox" bind:checked={l.visible} onchange={changed} onclick={(e) => e.stopPropagation()} title="Visible" />
            <span class="grow name">{l.kind === 'image' ? '🖼' : l.kind === 'text' ? 'T' : '■'} {l.name}</span>
            <button class="tiny" onclick={(e) => { e.stopPropagation(); moveLayer(l.id, 1); }} title="Move up">▲</button>
            <button class="tiny" onclick={(e) => { e.stopPropagation(); moveLayer(l.id, -1); }} title="Move down">▼</button>
            <button class="tiny" onclick={(e) => { e.stopPropagation(); removeLayer(l.id); }} title="Delete">✕</button>
          </div>
        {/each}
        <div class="layer base">
          <span class="grow name">Base:</span>
          <select bind:value={layout.base} onchange={changed}>
            <option value="vanilla">Vanilla art</option>
            <option value="color">Plain colour</option>
          </select>
          {#if layout.base === 'color'}<input type="color" bind:value={layout.baseColor} oninput={changed} />{/if}
        </div>
      </div>

      {#if sel}
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
            <label class="field">Face
              <select bind:value={sel.face} onchange={changed}>
                <option value="">All faces</option>
                {#each model.faces as f}<option value={f.id}>{f.label}</option>{/each}
              </select>
            </label>
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
          {#if sel.kind !== 'fill'}
            <label class="field">Rotation {Math.round(sel.rot)}°<input type="range" min="-180" max="180" step="1" bind:value={sel.rot} oninput={changed} /></label>
            <div class="row size">
              <label class="field" title="% of the {mainFace().label.toLowerCase()} width">Width %<input type="number" min="1" step="1"
                value={Math.round((sel.w / mainFace().net[2]) * 100)} oninput={(e) => setSize('w', +e.currentTarget.value)} /></label>
              <label class="field" title="% of the {mainFace().label.toLowerCase()} height">Height %<input type="number" min="1" step="1"
                value={Math.round((sel.h / mainFace().net[3]) * 100)} oninput={(e) => setSize('h', +e.currentTarget.value)} /></label>
              {#if sel.kind === 'image'}<button class="tiny" onclick={resetProportions} title="Back to the image's own proportions (keeps the width)">Unstretch</button>{/if}
            </div>
            <div class="small muted">Fill (keeps proportions, crops the overflow):</div>
            <div class="row wrap">
              {#if wrapFaces().length > 1}<button class="tiny" onclick={() => fitSelected('_wrap', 'cover')} title={wrapFaces().map((f) => f.label).join(' + ') + ' as one panorama'}>Wrap around</button>{/if}
              {#each model.faces as f}<button class="tiny" onclick={() => fitSelected(f.id, 'cover')}>{f.label}</button>{/each}
            </div>
            <div class="small muted">Stretch to (fills exactly, may distort):</div>
            <div class="row wrap">
              {#if wrapFaces().length > 1}<button class="tiny" onclick={() => fitSelected('_wrap', 'stretch')}>Wrap around</button>{/if}
              {#each model.faces as f}<button class="tiny" onclick={() => fitSelected(f.id, 'stretch')}>{f.label}</button>{/each}
            </div>
            <div class="row wrap">
              <button class="tiny" onclick={() => fitSelected(mainFace().id, 'contain')}>Fit inside {mainFace().label.toLowerCase()}</button>
            </div>
            <p class="muted small">Drag the white side handles to stretch one side; the blue corner keeps proportions (hold Shift to stretch freely).</p>
          {/if}
        </div>
      {:else}
        <p class="muted small">Add an image, colour or text. Tip: a colour layer with <b>Multiply</b> over the vanilla base tints the leather and keeps its grain.</p>
      {/if}
    </div>
  </div>
{:else}
  <p class="muted">Loading model…</p>
{/if}

<style>
  .editor { display: flex; gap: 14px; align-items: flex-start; }
  .stage { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 8px; }
  .tabs button.on { border-color: var(--accent); background: #22304d; }
  .texview.palette { image-rendering: pixelated; width: 128px; height: 128px; flex-shrink: 0; }
  .netview, .texview { width: 100%; max-height: 62vh; object-fit: contain; background: repeating-conic-gradient(#1b1f27 0 25%, #232834 0 50%) 0 0 / 24px 24px; border-radius: 6px; }
  .netview { cursor: move; touch-action: none; }
  .scene { position: relative; height: 440px; background: radial-gradient(circle at 50% 40%, #2a3140, var(--bg)); border-radius: 6px; }
  .view3d { width: 100%; height: 100%; display: block; cursor: grab; touch-action: none; }
  .view3d:active { cursor: grabbing; }
  .overlay { position: absolute; left: 12px; bottom: 4px; margin: 0; }
  .warn { color: #d9a400; }
  .side { width: 300px; flex-shrink: 0; display: flex; flex-direction: column; gap: 10px; }
  .layers { display: flex; flex-direction: column; gap: 3px; max-height: 240px; overflow: auto; }
  .layer { display: flex; align-items: center; gap: 6px; padding: 4px 6px; border: 1px solid var(--line); border-radius: 6px; cursor: pointer; background: var(--panel); }
  .layer.active { border-color: var(--accent); background: #22304d; }
  .layer.base { cursor: default; }
  .layer input[type='color'] { width: 36px; height: 24px; padding: 1px; }
  .name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 13px; }
  .props { display: flex; flex-direction: column; gap: 8px; background: var(--panel); border: 1px solid var(--line); border-radius: var(--radius); padding: 10px; }
  .props input[type='color'] { width: 44px; height: 28px; padding: 2px; }
  .wrap { flex-wrap: wrap; }
  .size { align-items: flex-end; gap: 6px; }
  .size .field { flex: 1; min-width: 0; }
  .size input { width: 100%; min-width: 0; box-sizing: border-box; }
  button.tiny { padding: 2px 7px; font-size: 12px; }
  .small { font-size: 12px; }
</style>
