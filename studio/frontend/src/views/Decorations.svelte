<script lang="ts" module>
  /** Editor state of a decoration (kept in the library's studio.json layouts). */
  export interface DecoLayout {
    version: 'deco1';
    // Poster: the source image and how the board is built (decoart.Poster).
    image?: string;
    width?: number;      // metres
    frame?: number;      // fraction of the shorter side
    color?: string;      // frame colour
    // Object: imported model source (right-handed, Y up) placed like furniture.
    mode?: 'placed' | 'kept'; // kept = converted from a mod, model as the mod placed it (until rotated/resized here)
    model?: string;
    texture?: string;
    sourceName?: string;
    sourceFile?: string; // the picked model file (imported again when textures are picked)
    textures?: { color: string; normal: string; normalDirectX: boolean; ao: string }; // picked by the user, library paths
    rotX?: number; rotY?: number; rotZ?: number;
    height?: number;     // metres
    triangles?: number;
    warnings?: string[];
    autoIcon: boolean;   // the saved icon was rendered here (re-rendered on save)
  }
  export function newDecoLayout(): DecoLayout {
    return { version: 'deco1', width: 0.45, frame: 0.04, color: '#202020', rotX: 0, rotY: 0, rotZ: 0, height: 0.6, autoIcon: false };
  }
</script>

<script lang="ts">
  // Decorations page: wall / floor / ceiling looks and placeable decorations (posters from an image, objects from a 3D model) for the
  // phone's "Buy Decoration" app. Stored in the accessory library (decorations list); the mod builds them on vanilla decorations
  // (Runtime/DecorationInjector). Vanilla names, prices and icons come from the game files (game-templates\decorations).
  import { onMount, untrack } from 'svelte';
  import { App, EventsOn, errText, ask } from '../lib/api';
  import { loadImage } from '../lib/accessoryArt';
  import CatalogPicker from './CatalogPicker.svelte';
  import { FigurineView, loadGeom, quadGeom, type Geom, type Mat, type Box, type SceneObj, mul, translate, scale, rotX, rotY, rotZ } from '../lib/figurineView';

  let { notify }: { notify: (t: string, k?: string) => void } = $props();

  let kinds = $state<any[]>([]);
  let tab = $state('Poster');
  let tpl = $state<any>(null);            // decorations.json of the game templates
  let tplStatus = $state<any>(null);
  let view = $state<any>(null);           // AccessoryView
  let selectedId = $state('');
  let picked = $state<string[]>([]);
  let anchor = $state('');
  let d = $state<any>(null);              // working copy
  let isNew = $state(false);              // a draft from NewDecoration, not stored yet
  let layout = $state<DecoLayout>(newDecoLayout());
  let dirty = $state(false);
  let saving = $state(false);
  let busy = $state('');
  let fromCatalog = $state(false);
  let bust = $state(Date.now());

  const kindInfo = (k: string) => kinds.find((x) => x.kind === k);
  const surface = (k: string) => !!kindInfo(k)?.surface;
  const list = $derived<any[]>(view?.decorations ?? []);
  const shown = $derived(list.filter((x) => x.kind === tab).slice().sort((a, b) => a.name.localeCompare(b.name)));
  const accUrl = (rel?: string) => (rel ? `/acc/${rel.split('/').map(encodeURIComponent).join('/')}?v=${bust}` : '');
  const tplUrl = (f?: string) => (f ? `/decotemplates/${encodeURIComponent(f)}` : '');
  const originOf = (x: any) => view?.origins?.[x.id];
  const vanilla = $derived<any[]>(
    !tpl ? [] : surface(tab) ? tpl.surfaces?.[tab] ?? [] : (tpl.objects ?? []).filter((o: any) => (tab === 'Poster' ? o.tab === 'Poster' : o.tab === 'Other')));
  const problems = $derived(d ? [...(view?.errors ?? []), ...(view?.warnings ?? [])].filter((m: string) => m.includes(`"${d.id}"`)) : []);

  function changed() { dirty = true; }

  async function loadTemplates() {
    const raw = await App.DecorationTemplates();
    tpl = raw ? JSON.parse(raw) : null;
    tplStatus = await App.TemplatesStatus();
  }

  async function load(keep = true) {
    view = await App.Accessories();
    if (isNew && keep) return;
    if (!keep || !shown.some((x) => x.id === selectedId)) selectedId = shown[0]?.id ?? '';
    select(selectedId);
  }

  function select(id: string) {
    selectedId = id;
    picked = id ? [id] : [];
    anchor = id;
    isNew = false;
    const x = list.find((p) => p.id === id);
    d = x ? JSON.parse(JSON.stringify(x)) : null;
    let l: any = null;
    try { l = x && view.layouts[id] ? JSON.parse(view.layouts[id]) : null; } catch { l = null; }
    layout = l && l.version === 'deco1' ? { ...newDecoLayout(), ...l } : newDecoLayout();
    dirty = false;
  }

  async function confirmDiscard() { return !dirty || (await ask('Discard unsaved changes to this decoration?')); }

  async function setTab(k: string) {
    if (k === tab || !(await confirmDiscard())) return;
    tab = k;
    selectedId = '';
    select(shown[0]?.id ?? '');
  }

  async function add() {
    if (!(await confirmDiscard())) return;
    try {
      d = await App.NewDecoration(tab, '');
      layout = newDecoLayout();
      selectedId = d.id;
      picked = [];
      isNew = true;
      dirty = true;
    } catch (e) { notify(errText(e), 'error'); }
  }

  async function clickItem(e: MouseEvent, id: string) {
    if (e.ctrlKey || e.metaKey) {
      picked = picked.includes(id) ? picked.filter((p) => p !== id) : [...picked, id];
      anchor = id;
      return;
    }
    if (e.shiftKey && anchor) {
      const ids = shown.map((x) => x.id);
      const [i, j] = [ids.indexOf(anchor), ids.indexOf(id)].sort((x, y) => x - y);
      if (i >= 0) { picked = ids.slice(i, j + 1); return; }
    }
    if (!(await confirmDiscard())) return;
    select(id);
  }

  async function removeIds(ids: string[]) {
    const names = list.filter((x) => ids.includes(x.id)).map((x) => x.name);
    const what = names.length === 1 ? `"${names[0]}"` : `${names.length} decorations`;
    if (!(await ask(`Delete ${what}? Placed or owned copies disappear from saves the next time they load (bought looks fall back to the default).`))) return;
    try {
      view = await App.DeleteDecorationMany(ids);
      dirty = false;
      await load(false);
      notify(`Deleted ${what}.`, 'ok');
    } catch (err) { notify(errText(err), 'error'); }
  }

  async function remove() {
    if (!d) return;
    if (isNew) { isNew = false; dirty = false; select(shown[0]?.id ?? ''); return; }
    await removeIds([d.id]);
  }

  async function listKey(e: KeyboardEvent) {
    if (e.key !== 'Delete' || !picked.length) return;
    e.preventDefault();
    await removeIds(picked);
  }

  async function addedFromCatalog(ids: string[]) {
    fromCatalog = false;
    await load();
    if (ids.length) notify(`Added ${ids.length} decoration${ids.length === 1 ? '' : 's'} from the catalog.`, 'ok');
  }

  async function saveToCatalog() {
    if (!d || isNew) return;
    if (dirty) { notify('Save your changes first — the catalog takes the saved version.', 'warn'); return; }
    try { await App.SaveToCatalog('decoration', [d.id]); notify(`"${d.name}" is now the catalog's version.`, 'ok'); }
    catch (e) { notify(errText(e), 'error'); }
  }

  // ---------------------------------------------------------------- files

  async function pick(field: 'texture' | 'normalMap' | 'roughnessMap' | 'image') {
    try {
      const rel = await App.PickAccessoryImage();
      if (!rel) return;
      if (field === 'image') layout.image = rel; else d[field] = rel;
      bust = Date.now();
      changed();
    } catch (e) { notify(errText(e), 'error'); }
  }

  async function importModel() {
    busy = 'Importing the model…';
    try {
      const src: any = await App.ImportFigurineModel();
      if (src?.model) {
        layout.model = src.model;
        layout.texture = src.texture;
        layout.sourceName = src.name;
        layout.sourceFile = src.file;
        layout.textures = { color: '', normal: '', normalDirectX: false, ao: '' };
        layout.triangles = src.triangles;
        layout.warnings = src.warnings ?? [];
        layout.mode = 'placed';
        layout.rotX = 0; layout.rotY = 0; layout.rotZ = 0;
        if (!layout.height) layout.height = d.mount === 'Wall' ? 0.6 : 1;
        changed();
      }
    } catch (e) { notify(errText(e), 'error'); }
    busy = '';
  }

  // ---------------------------------------------------------------- textures picked by the user (one picker per slot)

  type TexSlot = 'color' | 'normal' | 'ao';
  const texSlots: { slot: TexSlot; label: string; tip: string }[] = [
    { slot: 'color', label: 'Colour texture', tip: 'The picture laid on the model (base colour / albedo). Replaces the colours the model came with.' },
    { slot: 'normal', label: 'Normal map', tip: 'Surface detail (bumps, scratches). The game draws one texture, so it is baked in as light and shade.' },
    { slot: 'ao', label: 'Ambient occlusion', tip: 'Dark creases and corners, multiplied into the texture.' },
  ];
  const fileName = (rel: string) => rel.split('/').pop() ?? rel;

  async function pickTexture(slot: TexSlot) {
    try {
      const rel = await App.PickAccessoryImage();
      if (!rel) return;
      layout.textures = { ...(layout.textures ?? { color: '', normal: '', normalDirectX: false, ao: '' }), [slot]: rel };
      await applyTextures();
    } catch (e) { notify(errText(e), 'error'); }
  }

  async function clearTexture(slot: TexSlot) {
    if (!layout.textures) return;
    layout.textures = { ...layout.textures, [slot]: '' };
    await applyTextures();
  }

  async function setDirectX(on: boolean) {
    if (!layout.textures) return;
    layout.textures = { ...layout.textures, normalDirectX: on };
    if (layout.textures.normal) await applyTextures();
  }

  /** Imports the model file again with the picked textures (the placement stays). */
  async function applyTextures() {
    if (!layout.sourceFile) { notify('Import the model again first — Studio needs its file to lay textures on it.', 'warn'); return; }
    busy = 'Applying the textures…';
    try {
      const src: any = await App.ImportModelWithTextures(layout.sourceFile, layout.textures as any);
      layout.model = src.model;
      layout.texture = src.texture;
      layout.warnings = src.warnings ?? [];
      layout.triangles = src.triangles;
      if (layout.mode === 'kept') layout.mode = 'placed';
      changed();
    } catch (e) { notify(errText(e), 'error'); }
    busy = '';
  }

  /** Moving a converted model by hand: from now on it is placed like an imported one. */
  function placementEdited() {
    if (layout.mode === 'kept') layout.mode = 'placed';
    changed();
  }

  // ---------------------------------------------------------------- 2D previews and icons (surfaces, posters)

  let surfCanvas = $state<HTMLCanvasElement>();
  let posterCanvas = $state<HTMLCanvasElement>();
  let posterAspect = $state(0); // image height / width

  const hex = (s: string | undefined, fallback: string) => (/^#[0-9a-f]{6}$/i.test(s ?? '') ? s! : fallback);

  async function drawSurface(c: HTMLCanvasElement, tiles: number) {
    const ctx = c.getContext('2d')!;
    ctx.clearRect(0, 0, c.width, c.height);
    if (!d?.texture) return;
    const img = await loadImage(accUrl(d.texture));
    const tw = c.width / tiles, th = c.height / tiles;
    for (let y = 0; y < tiles; y++) for (let x = 0; x < tiles; x++) ctx.drawImage(img, x * tw, y * th, tw, th);
    ctx.globalCompositeOperation = 'multiply';
    ctx.fillStyle = hex(d.color, '#ffffff');
    ctx.fillRect(0, 0, c.width, c.height);
    ctx.globalCompositeOperation = 'source-over';
  }

  /** Poster as built by decoart.BuildPoster: the image in a frame of the frame colour (fitted into the canvas). */
  async function drawPoster(c: HTMLCanvasElement, pad = 0) {
    const ctx = c.getContext('2d')!;
    ctx.clearRect(0, 0, c.width, c.height);
    if (!layout.image) return;
    const img = await loadImage(accUrl(layout.image));
    posterAspect = img.height / img.width;
    const fr = Math.max(0, layout.frame ?? 0) * Math.min(img.width, img.height);
    const bw = img.width + 2 * fr, bh = img.height + 2 * fr;
    const s = Math.min((c.width - 2 * pad) / bw, (c.height - 2 * pad) / bh);
    const x = (c.width - bw * s) / 2, y = (c.height - bh * s) / 2;
    ctx.fillStyle = hex(layout.color, '#202020');
    ctx.fillRect(x, y, bw * s, bh * s);
    ctx.drawImage(img, x + fr * s, y + fr * s, img.width * s, img.height * s);
  }

  $effect(() => {
    void [d?.texture, d?.color, surfCanvas, bust];
    if (surfCanvas && d && surface(d.kind)) untrack(() => drawSurface(surfCanvas!, 3).catch(() => {}));
  });
  $effect(() => {
    void [layout.image, layout.frame, layout.color, posterCanvas, bust];
    if (posterCanvas && d?.kind === 'Poster') untrack(() => drawPoster(posterCanvas!, 8).catch(() => {}));
  });

  /** Board size in cm (frame included), as the mod gets it. */
  const posterSize = $derived.by(() => {
    if (!posterAspect) return null;
    const f = Math.max(0, layout.frame ?? 0), short = Math.min(1, posterAspect);
    const bw = 1 + 2 * f * short, bh = posterAspect + 2 * f * short;
    const w = layout.width ?? 0.45;
    return { w: Math.round(w * 100), h: Math.round((w * bh) / bw * 100) };
  });

  // ---------------------------------------------------------------- 3D preview (objects)

  let canvas3d = $state<HTMLCanvasElement>();
  let v3: FigurineView | null = null;
  let figGeom: Geom | null = null;
  let loadedModel = '';
  let loadedTex = '';
  let baseGeoms: Record<string, Geom> = {};
  let wallGeom: Geom | null = null;
  let framedFor = '';

  /** The model as the mod gets it, in GL space (the Unity result mirrored on z): see decoart.OnWall / figurine.Place. */
  function placement(): { m: Mat; box: Box } | null {
    if (!figGeom) return null;
    const R = mul(rotY(layout.rotY ?? 0), mul(rotX(layout.rotX ?? 0), rotZ(layout.rotZ ?? 0)));
    const g = figGeom.data, lo = [Infinity, Infinity, Infinity], hi = [-Infinity, -Infinity, -Infinity];
    for (let i = 0; i < g.length; i += 8) {
      for (let q = 0; q < 3; q++) {
        const val = R[q] * g[i] + R[4 + q] * g[i + 1] + R[8 + q] * g[i + 2];
        if (val < lo[q]) lo[q] = val;
        if (val > hi[q]) hi[q] = val;
      }
    }
    const h = hi[1] - lo[1];
    if (!(h > 0)) return null;
    const s = (layout.height || 1) / h;
    const c = [(lo[0] + hi[0]) / 2, lo[1], (lo[2] + hi[2]) / 2];
    let m = mul(scale(s), mul(translate(-c[0], -c[1], -c[2]), R));
    const hw = ((hi[0] - lo[0]) * s) / 2, hd = ((hi[2] - lo[2]) * s) / 2, H = h * s;
    if (d?.mount === 'Wall') {
      // Turned to face out (−z in GL), back on the wall (z = 0), centred on the hanging point.
      m = mul(translate(0, -H / 2, -hd), mul(rotY(180), m));
      return { m, box: { min: [-hw, -H / 2, -2 * hd], max: [hw, H / 2, 0] } };
    }
    return { m, box: { min: [-hw, 0, -hd], max: [hw, H, hd] } };
  }

  async function rebuild() {
    if (!canvas3d || !d || d.kind !== 'Object') return;
    if (!v3) v3 = new FigurineView(canvas3d, { preserve: true });
    const kept = layout.mode === 'kept' && !!d.mesh;
    const modelUrl = kept ? accUrl(d.mesh).split('?')[0] : layout.model ? accUrl(layout.model).split('?')[0] : '';
    const texRel = kept ? d.texture : layout.texture;
    try {
      if (modelUrl !== loadedModel) { figGeom = modelUrl ? await loadGeom(modelUrl, kept) : null; loadedModel = modelUrl; }
      if (texRel && texRel !== loadedTex) { v3.setTexture('deco', await loadImage(accUrl(texRel))); loadedTex = texRel; }
      const mount = d.mount === 'Wall' ? 'Wall' : 'Floor';
      const base = tpl?.bases?.[mount];
      if (base?.model && !baseGeoms[mount]) baseGeoms[mount] = await loadGeom(tplUrl(base.model), true);
      if (!wallGeom) wallGeom = quadGeom(3, 2.4);
    } catch (e) { notify(errText(e), 'error'); return; }
    const objs: SceneObj[] = [];
    let box: Box | null = null;
    if (figGeom) {
      const p = kept ? { m: scale(1), box: { min: figGeom.min, max: figGeom.max } } : placement();
      if (p) { objs.push({ geom: figGeom, matrix: p.m, texture: texRel ? 'deco' : undefined }); box = p.box; }
    }
    const mount = d.mount === 'Wall' ? 'Wall' : 'Floor';
    const bg = baseGeoms[mount];
    if (bg) {
      // A vanilla one beside it, for scale.
      const w = box ? box.max[0] - box.min[0] : 0.5;
      const bw = bg.max[0] - bg.min[0];
      objs.push({ geom: bg, matrix: translate(-(w / 2 + bw / 2 + 0.15) - (bg.min[0] + bg.max[0]) / 2, 0, 0), color: [0.75, 0.75, 0.78, 1] });
    }
    if (wallGeom && mount === 'Wall') objs.push({ geom: wallGeom, matrix: translate(0, 0, 0.002), color: [0.25, 0.27, 0.32, 1] });
    v3.setScene(objs);
    const key = `${d.id}|${mount}|${!!figGeom}`;
    if (framedFor !== key) {
      v3.yaw = mount === 'Wall' ? 180 : 30; v3.pitch = mount === 'Wall' ? 5 : 18;
      v3.frameBox(box ?? { min: [-0.5, 0, -0.5], max: [0.5, 1, 0.5] }, 0.6);
      framedFor = key;
    }
    v3.draw();
  }

  $effect(() => {
    void [d?.id, d?.kind, d?.mount, d?.mesh, layout.model, layout.texture, layout.rotX, layout.rotY, layout.rotZ, layout.height, layout.mode, canvas3d, tpl];
    untrack(() => rebuild());
  });
  $effect(() => {
    if (d?.kind !== 'Object' && v3) { v3.dispose(); v3 = null; loadedModel = ''; loadedTex = ''; framedFor = ''; }
  });

  async function renderObjectIcon(size: number[]): Promise<string> {
    await rebuild();
    const p = layout.mode === 'kept' && d.mesh && figGeom ? { m: scale(1), box: { min: figGeom.min, max: figGeom.max } } : placement();
    if (!p || !figGeom) throw new Error('import a model first');
    const c = document.createElement('canvas');
    c.width = size[0] || 512; c.height = size[1] || 512;
    const v = new FigurineView(c, { preserve: true, interactive: false });
    try {
      const tex = layout.mode === 'kept' ? d.texture : layout.texture;
      if (tex) v.setTexture('deco', await loadImage(accUrl(tex)));
      v.setScene([{ geom: figGeom, matrix: p.m, texture: tex ? 'deco' : undefined }]);
      v.yaw = d.mount === 'Wall' ? 200 : 30; v.pitch = 12; v.fov = 0.35;
      v.frameBox(p.box, 0.9);
      v.renderNow();
      return c.toDataURL('image/png');
    } finally { v.dispose(); }
  }

  // ---------------------------------------------------------------- save

  async function save() {
    if (!d) return;
    saving = true;
    try {
      let icon = '';
      const autoIcon = !d.icon || layout.autoIcon;
      if (surface(d.kind)) {
        if (!d.texture) throw new Error('Pick a texture first.');
        if (autoIcon) { const c = document.createElement('canvas'); c.width = c.height = 256; await drawSurface(c, 2); icon = c.toDataURL('image/png'); }
      } else if (d.kind === 'Poster') {
        if (!layout.image) throw new Error('Pick an image first.');
        busy = 'Building the poster…';
        const b: any = await App.BakePoster(layout.image, { width: layout.width ?? 0.45, frame: layout.frame ?? 0, color: hex(layout.color, '#202020'), depth: 0 } as any);
        d.mesh = b.mesh; d.texture = b.texture;
        d.mount = undefined;
        if (autoIcon) { const c = document.createElement('canvas'); c.width = c.height = 512; await drawPoster(c, 24); icon = c.toDataURL('image/png'); }
      } else {
        if (layout.mode !== 'kept' || !d.mesh) {
          if (!layout.model) throw new Error('Import a model first.');
          busy = 'Placing the model…';
          const b: any = await App.BakeDecorationModel(layout.model, layout.texture ?? '', d.mount ?? 'Floor',
            { rotX: layout.rotX ?? 0, rotY: layout.rotY ?? 0, rotZ: layout.rotZ ?? 0, height: layout.height || 1, anchor: [0, 0, 0] } as any);
          d.mesh = b.mesh; d.texture = b.texture;
        }
        if (autoIcon) icon = await renderObjectIcon([512, 512]);
      }
      if (icon) layout.autoIcon = true;
      const clean = JSON.parse(JSON.stringify(d));
      for (const k of Object.keys(clean)) if (clean[k] === '' || clean[k] === null || clean[k] === undefined) delete clean[k];
      view = await App.SaveDecoration(clean, JSON.stringify(layout), icon);
      isNew = false;
      dirty = false;
      bust = Date.now();
      await load();
      notify(`Saved "${d.name}"${view.installed ? ' and installed it in the game' : ''}.`, 'ok');
    } catch (e) { notify(errText(e), 'error'); }
    busy = '';
    saving = false;
  }

  async function iconFromFile() {
    try {
      const rel = await App.PickAccessoryImage();
      if (!rel) return;
      d.icon = rel;
      layout.autoIcon = false;
      bust = Date.now();
      changed();
    } catch (e) { notify(errText(e), 'error'); }
  }

  onMount(() => {
    const offs = [
      EventsOn('templates:progress', (m: string) => (tplStatus = { ...tplStatus, busy: true, message: m })),
      EventsOn('templates:ready', () => loadTemplates().catch((e) => notify(errText(e), 'error'))),
    ];
    (async () => {
      try {
        kinds = await App.DecorationKinds();
        await loadTemplates();
        await load(false);
      } catch (e) { notify(errText(e), 'error'); }
    })();
    return () => { offs.forEach((o) => o()); v3?.dispose(); };
  });
</script>

<div class="page">
  <header class="row">
    <h2 class="grow">Decorations</h2>
    <button onclick={() => App.OpenAccessoriesFolder()} title="Decorations are saved with the accessories (accessories.json).">Open folder</button>
  </header>
  <p class="muted small intro">
    New things for the phone's <b>Buy Decoration</b> app: wall, floor and ceiling looks, posters made from your own pictures and decorations
    from your own 3D models. Bought looks are switched in the game's Decorate screen; posters and decorations are placed like vanilla ones.
    <br />Only want your own in the shop? Turn off <b>Mod settings → Content - Decorations</b> (ShowVanillaPosters, ShowVanillaDecoObjects,
    ShowVanillaSurfaces).
  </p>
  {#if !tpl}
    {#if tplStatus?.busy}<p class="muted small">Reading the game's decorations… {tplStatus.message ?? ''}</p>
    {:else}<p class="warn small">The game's decorations aren't available{tplStatus?.error ? `: ${tplStatus.error}` : ' — set the game folder in Settings'} (you can still make your own).</p>{/if}
  {/if}

  <div class="tabs">
    {#each kinds as k}
      <button class:active={tab === k.kind} onclick={() => setTab(k.kind)}>{k.title} <span class="muted small">{list.filter((x) => x.kind === k.kind).length || ''}</span></button>
    {/each}
  </div>

  <div class="fwrap">
    <div class="list" role="listbox" tabindex="-1" onkeydown={listKey} title="Ctrl+click or Shift+click to select several; Delete removes them">
      {#each shown as x (x.id)}
        <button class:active={x.id === selectedId && !isNew} class:picked={picked.includes(x.id) && x.id !== selectedId} onclick={(e) => clickItem(e, x.id)}>
          <span class="ico">{#if x.icon}<img src={accUrl(x.icon)} alt="" />{/if}</span>
          <span class="grow">{x.name}<br /><span class="muted small">{x.kind === 'Object' ? `${x.mount === 'Wall' ? 'on a wall' : 'on the floor'}` : kindInfo(x.kind)?.one ?? x.kind}{originOf(x) ? ` · from ${originOf(x).mod}` : ''}</span></span>
        </button>
      {/each}
      {#if isNew && d}<button class="active"><span class="ico"></span><span class="grow">{d.name}<br /><span class="muted small">new — not saved</span></span></button>{/if}
      <button onclick={add}>+ New {kindInfo(tab)?.one ?? ''}</button>
      <button onclick={() => (fromCatalog = true)} title="Decorations from your other setups and earlier imports — no re-import">Add from catalog…</button>
    </div>

    {#if d}
      <div class="form">
        <section>
          <div class="row">
            <label class="field grow" title="Name shown in the Buy Decoration app.">Name<input bind:value={d.name} oninput={changed} /></label>
            <label class="field" style="width:120px" title="Price in the Buy Decoration app.">Price<input type="number" min="0" step="10" bind:value={d.price} oninput={changed} /></label>
            <label class="field" style="width:170px" title="Fixed id that saves use to recognise this decoration. It never changes, so renaming is safe.">Id<input value={d.id} readonly /></label>
          </div>
          <div class="row thumbs">
            <span class="muted small">Shop icon</span>
            {#if d.icon}<img src={accUrl(d.icon)} alt="" />{/if}
            <span class="muted small">{layout.autoIcon || !d.icon ? 'made from the decoration when you save' : 'your own picture'}</span>
            <div class="grow"></div>
            <button class="small" onclick={iconFromFile}>Use a picture…</button>
            {#if !layout.autoIcon && d.icon}<button class="small" onclick={() => { layout.autoIcon = true; changed(); }}>Make it from the decoration</button>{/if}
          </div>
          <div class="row"><div class="grow"></div>
            {#if !isNew}<button class="small" onclick={saveToCatalog} title="Make this setup's saved version the one other setups get when they add it from the catalog.">Update catalog</button>{/if}
            <button class="danger small" onclick={remove}>{isNew ? 'Discard' : 'Delete'}</button></div>
        </section>

        {#if surface(d.kind)}
          <section>
            <h3>Look</h3>
            <div class="editor">
              <canvas bind:this={surfCanvas} width="420" height="420" class="preview"></canvas>
              <div class="side">
                <p class="muted small">A picture that repeats across the whole {d.kind.toLowerCase()} (seamless textures look best). The game tiles it the way it tiles its own
                  {d.kind.toLowerCase()} looks.</p>
                <div class="row"><button onclick={() => pick('texture')}>{d.texture ? 'Change texture…' : 'Pick texture…'}</button></div>
                <label class="field" title="Colour multiplied onto the texture (white = the texture as it is).">Colour
                  <span class="row"><input type="color" value={hex(d.color, '#ffffff')} oninput={(e) => { d.color = e.currentTarget.value.toUpperCase() === '#FFFFFF' ? '' : e.currentTarget.value; changed(); }} />
                  {#if d.color}<button class="small" onclick={() => { d.color = ''; changed(); }}>White</button>{/if}</span></label>
                <label class="field" title="How glossy it looks: 0 = matt, 1 = mirror-like. The game's own looks are mostly 0.1–0.2 or use a map.">Smoothness {(d.smoothness ?? 0.2).toFixed(2)}
                  <input type="range" min="0" max="1" step="0.01" value={d.smoothness ?? 0.2} oninput={(e) => { d.smoothness = +e.currentTarget.value; changed(); }} /></label>
                <details>
                  <summary class="small">Bump and shine maps (optional)</summary>
                  <div class="row small"><span class="grow">Normal map {d.normalMap ? '✓' : '—'}</span><button class="small" onclick={() => pick('normalMap')}>Pick…</button>
                    {#if d.normalMap}<button class="small" onclick={() => { d.normalMap = ''; changed(); }}>Remove</button>{/if}</div>
                  <div class="row small"><span class="grow">Metallic/smoothness map {d.roughnessMap ? '✓' : '—'}</span><button class="small" onclick={() => pick('roughnessMap')}>Pick…</button>
                    {#if d.roughnessMap}<button class="small" onclick={() => { d.roughnessMap = ''; changed(); }}>Remove</button>{/if}</div>
                  <p class="muted small">Unity Standard layout: metal in red, smoothness in alpha (scaled by Smoothness).</p>
                </details>
              </div>
            </div>
          </section>
        {:else if d.kind === 'Poster'}
          <section>
            <h3>Poster</h3>
            <div class="editor">
              <canvas bind:this={posterCanvas} width="360" height="420" class="preview"></canvas>
              <div class="side">
                <div class="row"><button onclick={() => pick('image')}>{layout.image ? 'Change picture…' : 'Pick picture…'}</button></div>
                <label class="field" title="Width of the whole board on the wall (frame included). The game's posters are 45 cm wide.">Width (cm)
                  <input type="number" min="5" max="500" step="1" value={Math.round((layout.width ?? 0.45) * 100)} oninput={(e) => { layout.width = Math.max(0.05, +e.currentTarget.value / 100); changed(); }} /></label>
                {#if posterSize}<p class="muted small">Board: {posterSize.w} × {posterSize.h} cm (the game's posters: 45 × 61 cm).</p>{/if}
                <label class="field" title="Frame around the picture, as a share of its shorter side.">Frame {Math.round((layout.frame ?? 0) * 100)} %
                  <input type="range" min="0" max="0.2" step="0.005" value={layout.frame ?? 0} oninput={(e) => { layout.frame = +e.currentTarget.value; changed(); }} /></label>
                <label class="field" title="Frame and edge colour.">Frame colour<input type="color" value={hex(layout.color, '#202020')} oninput={(e) => { layout.color = e.currentTarget.value; changed(); }} /></label>
                <label class="field" title="Shop tab it is listed on.">Shop tab
                  <select value={d.tab ?? 'Poster'} onchange={(e) => { d.tab = e.currentTarget.value === 'Poster' ? '' : e.currentTarget.value; changed(); }}>
                    <option value="Poster">Posters</option><option value="Other">Other</option></select></label>
              </div>
            </div>
          </section>
        {:else}
          <section>
            <h3>Model</h3>
            <div class="editor">
              <div class="scene"><canvas bind:this={canvas3d} class="view3d"></canvas>
                <p class="hint muted small">Drag to turn, right-drag to pan, wheel to zoom. Grey: a vanilla {d.mount === 'Wall' ? 'poster' : 'plant'} for scale.</p></div>
              <div class="side">
                <div class="row"><button onclick={importModel} disabled={!!busy}>{layout.model || d.mesh ? 'Replace model…' : 'Import model…'}</button></div>
                <p class="muted small">{layout.sourceName ? `${layout.sourceName} · ${layout.triangles ?? 0} triangles` : 'GLB, glTF or OBJ (+ MTL), like figurines.'}</p>
                {#each layout.warnings ?? [] as w}<p class="warn small">{w}</p>{/each}
                <label class="field" title="Where it goes in the game: hung on a wall (like posters and arrow signs) or stood on the floor (like statues and plants).">Goes
                  <select value={d.mount ?? 'Floor'} onchange={(e) => { d.mount = e.currentTarget.value; placementEdited(); }}>
                    <option value="Floor">on the floor</option><option value="Wall">on a wall</option></select></label>
                <label class="field" title="Shop tab it is listed on.">Shop tab
                  <select value={d.tab ?? 'Other'} onchange={(e) => { d.tab = e.currentTarget.value === 'Other' ? '' : e.currentTarget.value; changed(); }}>
                    <option value="Other">Other</option><option value="Poster">Posters</option></select></label>
                {#if layout.model || layout.sourceFile}
                  <div class="texbox">
                    <b class="small">Textures</b>
                    {#if !layout.sourceFile}<p class="muted small">Import the model again to pick textures for it.</p>
                    {:else}
                      <p class="muted small">Pick each file yourself — e.g. from a texture download that came separately.</p>
                      {#each texSlots as t}
                        <div class="row small" title={t.tip}>
                          <span class="grow">{t.label}: <span class="muted">{layout.textures?.[t.slot] ? fileName(layout.textures[t.slot]) : 'none'}</span></span>
                          <button class="small" disabled={!!busy} onclick={() => pickTexture(t.slot)}>Pick…</button>
                          {#if layout.textures?.[t.slot]}<button class="small" disabled={!!busy} onclick={() => clearTexture(t.slot)}>Remove</button>{/if}
                        </div>
                        {#if t.slot === 'normal' && layout.textures?.normal}
                          <label class="check small" title="Tick if the normal map is DirectX style (the download usually says so, e.g. …_Normal_DirectX). Bumps look inverted when this is wrong.">
                            <input type="checkbox" checked={layout.textures.normalDirectX} onchange={(e) => setDirectX(e.currentTarget.checked)} /> DirectX normal map</label>
                        {/if}
                      {/each}
                    {/if}
                  </div>
                {/if}
                {#if layout.mode === 'kept' && d.mesh}
                  <p class="muted small">Placed as the mod had it. Change the height or turn it to place it yourself.</p>
                {/if}
                <label class="field" title="Height in the game (metres). The game's statues are about 1.5 m, plants 1.1 m, posters 0.6 m.">Height (m)
                  <input type="number" min="0.02" max="5" step="0.05" value={layout.height ?? 1} oninput={(e) => { layout.height = Math.max(0.02, +e.currentTarget.value); placementEdited(); }} /></label>
                <div class="grid3">
                  {#each [['rotX', 'Tilt X'], ['rotY', 'Turn Y'], ['rotZ', 'Roll Z']] as [k, label]}
                    <label class="field">{label}°<input type="number" step="15" value={(layout as any)[k] ?? 0} oninput={(e) => { (layout as any)[k] = +e.currentTarget.value; placementEdited(); }} /></label>
                  {/each}
                </div>
              </div>
            </div>
          </section>
        {/if}

        {#if problems.length}<section>{#each problems as m}<p class="warn small">{m}</p>{/each}</section>{/if}

        <div class="row savebar">
          {#if busy}<span class="muted small">{busy}</span>{/if}
          <div class="grow"></div>
          <button class="primary" onclick={save} disabled={!dirty || saving}>{saving ? 'Saving…' : 'Save & install'}</button>
        </div>
      </div>
    {:else}
      <div class="form">
        <p class="muted">No custom {kindInfo(tab)?.title.toLowerCase() ?? 'decorations'} yet — click <b>+ New</b>.</p>
        {#if vanilla.length}
          <section>
            <h3>The game's {kindInfo(tab)?.title.toLowerCase()}</h3>
            <div class="vanilla">
              {#each vanilla as v}<span class="vico" title={`${v.name} — ${v.price}`}>{#if v.icon}<img src={tplUrl(v.icon)} alt="" />{/if}</span>{/each}
            </div>
          </section>
        {/if}
      </div>
    {/if}
  </div>
</div>

{#if fromCatalog}<CatalogPicker kind="decoration" sub={tab} onadded={addedFromCatalog} onclose={() => (fromCatalog = false)} />{/if}

<style>
  .page { padding: 16px 20px; height: 100%; display: flex; flex-direction: column; gap: 10px; overflow: auto; }
  .intro { max-width: 900px; line-height: 1.5; margin: 0; }
  .tabs { display: flex; gap: 4px; flex-wrap: wrap; }
  .tabs button.active { border-color: var(--accent); background: #22304d; }
  .fwrap { display: flex; flex: 1; min-height: 0; }
  .list { width: 210px; flex-shrink: 0; border-right: 1px solid var(--line); padding: 10px; display: flex; flex-direction: column; gap: 4px; overflow: auto; }
  .list button { text-align: left; display: flex; gap: 8px; align-items: center; }
  .list button.active { border-color: var(--accent); background: #22304d; }
  .list button.picked { border-color: var(--accent); background: #1c2740; }
  .ico { width: 36px; height: 36px; flex-shrink: 0; display: flex; align-items: center; justify-content: center; }
  .ico img { max-width: 100%; max-height: 100%; }
  .form { flex: 1; min-width: 0; overflow: auto; padding: 12px 14px; display: flex; flex-direction: column; gap: 14px; }
  section { background: var(--panel); border: 1px solid var(--line); border-radius: var(--radius); padding: 12px; display: flex; flex-direction: column; gap: 10px; }
  section h3 { margin: 0; font-size: 14px; }
  section > .row { flex-wrap: wrap; }
  .editor { display: flex; flex-wrap: wrap; gap: 14px; align-items: flex-start; }
  .preview { background: repeating-conic-gradient(#2a2f3a 0% 25%, #232833 0% 50%) 50% / 24px 24px; border-radius: 6px; max-width: 100%; }
  .side { flex: 1 1 240px; max-width: 360px; display: flex; flex-direction: column; gap: 10px; }
  .scene { position: relative; flex: 1 1 380px; height: clamp(300px, 50vh, 480px); background: radial-gradient(circle at 50% 40%, #2a3140, var(--bg)); border-radius: 6px; }
  .view3d { width: 100%; height: 100%; display: block; cursor: grab; touch-action: none; }
  .hint { position: absolute; left: 12px; right: 12px; bottom: 4px; margin: 0; pointer-events: none; }
  .grid3 { display: grid; grid-template-columns: repeat(3, 1fr); gap: 6px; }
  .grid3 input { width: 100%; min-width: 0; box-sizing: border-box; }
  .thumbs { align-items: center; gap: 8px; }
  .thumbs img { height: 44px; border-radius: 4px; background: var(--bg); }
  .vanilla { display: flex; flex-wrap: wrap; gap: 6px; }
  .vico { width: 56px; height: 56px; display: flex; align-items: center; justify-content: center; background: var(--bg); border-radius: 4px; }
  .vico img { max-width: 100%; max-height: 100%; }
  .texbox { display: flex; flex-direction: column; gap: 6px; background: var(--bg); border: 1px solid var(--line); border-radius: var(--radius); padding: 8px; }
  .savebar { position: sticky; bottom: 0; background: var(--bg); padding: 8px 0; }
</style>
