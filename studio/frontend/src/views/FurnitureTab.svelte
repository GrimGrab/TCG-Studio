<script lang="ts" module>
  /** Editor state of a furniture piece (kept in the library's studio.json layouts). */
  export interface FurLayout {
    version: 'fur1';
    model: string;       // library-relative source OBJ (right-handed, Y up), '' = base model
    texture: string;     // library-relative texture of the model
    sourceName: string;
    rotX: number; rotY: number; rotZ: number; // degrees, applied Z, X, Y
    height: number;      // metres, 0 = base piece's height
    triangles: number;
    warnings: string[];
    autoIcon: boolean;   // the saved icon was rendered from the model (re-rendered on save)
  }
  export function newFurLayout(): FurLayout {
    return { version: 'fur1', model: '', texture: '', sourceName: '', rotX: 0, rotY: 0, rotZ: 0, height: 0, triangles: 0, warnings: [], autoIcon: false };
  }
</script>

<script lang="ts">
  // Custom furniture: new pieces built on a vanilla piece of the chosen type (its behaviour). Vanilla pieces (icons, merged models,
  // spots) come from the game files (game-templates\furniture). Spots are edited numerically with a live 3D view; the maths matches
  // the mod's Runtime/FurnitureSpots (piece-local Unity space, rotation = Quaternion.Euler(x, y, z)).
  import { maximizable } from '../lib/maximize';
  import { onMount, untrack } from 'svelte';
  import { App, EventsOn, errText, ask } from '../lib/api';
  import { loadImage } from '../lib/accessoryArt';
  import CatalogPicker from './CatalogPicker.svelte';
  import {
    FigurineView, loadGeom, type Geom, type Mat, type Box, type SceneObj, ident, mul, translate, scale, rotX, rotY, rotZ, point, boxLines, MIRROR_Z,
  } from '../lib/figurineView';
  import { type Compartment, type ItemTpl, slotMatrices, grid as slotGrid, meshMatrix } from '../lib/figurineShelf';

  let { notify }: { notify: (t: string, k?: string) => void } = $props();

  let types = $state<any[]>([]);
  let pieces = $state<any[]>([]);        // game templates: vanilla furniture
  let tplStatus = $state<any>(null);
  let view = $state<any>(null);          // AccessoryView (furniture list)
  let selectedId = $state('');
  // Selected in the list for Delete (Ctrl+click adds/removes, Shift+click a range); the open piece is selectedId.
  let picked = $state<string[]>([]);
  let anchor = $state('');
  let f = $state<any>(null);             // working copy
  let layout = $state<FurLayout>(newFurLayout());
  let dirty = $state(false);
  let saving = $state(false);
  let busy = $state(false);
  let bust = $state(Date.now());
  let newType = $state('Shelf');
  let sel = $state(0);                   // selected spot
  let part = $state<'spot' | 'customer' | 'tag' | 'area' | 'point'>('spot');
  let selPt = $state(0);                 // selected position point (seats, stand points…) when part === 'point' // what the 3D handles act on (a part of the selected spot, or the placement area)
  let redraw = $state(0);                // bumped while dragging (spot edits don't change f.spots itself)
  let showBase = $state(true);
  let tool = $state<'move' | 'rotate' | 'size'>('move');
  let gridFollows = $state(true);        // resizing a spot keeps the item spacing (grid follows size)
  let showHelp = $state(false);
  let showPeople = $state(false);        // stand-in customers on the customer points (P)
  let showTags = $state(false);          // price tags drawn as tag-sized labels where the game puts them (L)
  const TAG_W = 0.09, TAG_H = 0.05;      // approximate size of a shelf price tag (m)

  /**
   * A stand-in customer (GL space, facing +z, feet at 0, ~1.65 m): boxes for legs, body, arms and head plus a nose, so you can
   * judge reach and aisle room. The game's own customers are rigged characters; this is only for scale.
   */
  const personGeom: Geom = (() => {
    const out: number[] = [];
    const box = (x0: number, y0: number, z0: number, x1: number, y1: number, z1: number) => {
      const faces: [number[], number[][]][] = [
        [[1, 0, 0], [[x1, y0, z0], [x1, y1, z0], [x1, y1, z1], [x1, y0, z1]]], [[-1, 0, 0], [[x0, y0, z1], [x0, y1, z1], [x0, y1, z0], [x0, y0, z0]]],
        [[0, 1, 0], [[x0, y1, z0], [x0, y1, z1], [x1, y1, z1], [x1, y1, z0]]], [[0, -1, 0], [[x0, y0, z1], [x0, y0, z0], [x1, y0, z0], [x1, y0, z1]]],
        [[0, 0, 1], [[x1, y0, z1], [x1, y1, z1], [x0, y1, z1], [x0, y0, z1]]], [[0, 0, -1], [[x0, y0, z0], [x0, y1, z0], [x1, y1, z0], [x1, y0, z0]]],
      ];
      for (const [n, q] of faces) for (const k of [0, 1, 2, 0, 2, 3]) out.push(...q[k], ...n, 0, 0);
    };
    box(-0.15, 0, -0.08, -0.02, 0.8, 0.08); box(0.02, 0, -0.08, 0.15, 0.8, 0.08);      // legs
    box(-0.19, 0.8, -0.1, 0.19, 1.42, 0.1);                                            // body
    box(-0.27, 0.78, -0.06, -0.19, 1.38, 0.06); box(0.19, 0.78, -0.06, 0.27, 1.38, 0.06); // arms
    box(-0.1, 1.44, -0.1, 0.1, 1.65, 0.1);                                              // head
    box(-0.03, 1.52, 0.1, 0.03, 1.56, 0.14);                                            // nose (facing)
    let min = [Infinity, Infinity, Infinity], max = [-Infinity, -Infinity, -Infinity];
    for (let i = 0; i < out.length; i += 8) for (let q = 0; q < 3; q++) { min[q] = Math.min(min[q], out[i + q]); max[q] = Math.max(max[q], out[i + q]); }
    return { data: new Float32Array(out), count: out.length / 8, min, max };
  })();

  // ---------------------------------------------------------------- position points (seats, cashier, worker, stand points…)
  // Roles per type come from Go (setfmt.PointRoles, same as the mod's FurnitureKinds.PointRoles); vanilla points from the game files.

  const ROLE_COL: Record<string, [number, number, number, number]> = {
    sit: [1, 0.6, 0.2, 1], stand: [0.4, 0.8, 1, 1], standB: [0.65, 0.6, 1, 1], cashier: [1, 0.9, 0.3, 1], queue: [1, 0.5, 0.8, 1],
    placeItems: [0.95, 0.95, 0.95, 1], trade: [0.5, 1, 0.85, 1], worker: [1, 0.7, 0.3, 1], player: [0.3, 1, 0.6, 1], customer: [0.45, 0.95, 0.45, 1],
  };
  const roleInfo = (role: string) => (info?.points ?? []).find((r: any) => r.role === role);
  /** The piece's points: its own, or the base piece's. */
  const pointsOf = (): any[] => f?.points ?? t?.points ?? [];
  /** Index of a point among the points of its role (1-based, for labels). */
  const roleIndex = (i: number) => { const pts = pointsOf(); return pts.slice(0, i + 1).filter((q: any) => q.role === pts[i]?.role).length; };
  const pointLabel = (i: number) => { const q = pointsOf()[i]; return q ? `${roleInfo(q.role)?.label ?? q.role} ${roleIndex(i)}` : ''; };
  function pointsCustom() { if (!f.points) f.points = JSON.parse(JSON.stringify(t?.points ?? [])); }
  function usePointsVanilla() { f.points = undefined; if (part === 'point') part = 'spot'; changed(); redraw++; }
  function addPoint() {
    pointsCustom();
    const src = f.points[selPt];
    if (!src || !roleInfo(src.role)?.resizable) return;
    const c = JSON.parse(JSON.stringify(src));
    c.pos[0] = r3(c.pos[0] + 0.3);
    // New points of a role go right after the others of that role (the game uses them in order).
    let at = f.points.length;
    for (let i = f.points.length - 1; i >= 0; i--) if (f.points[i].role === src.role) { at = i + 1; break; }
    f.points = [...f.points.slice(0, at), c, ...f.points.slice(at)];
    selPt = at; part = 'point'; tool = 'move'; changed(); redraw++;
  }
  function deletePoint() {
    pointsCustom();
    const q = f.points[selPt];
    if (!q) return;
    if (!roleInfo(q.role)?.resizable) { notify(`${roleInfo(q.role)?.label ?? q.role} points can only be moved — the game expects exactly ${f.points.filter((x: any) => x.role === q.role).length}.`, 'warn'); return; }
    if (f.points.filter((x: any) => x.role === q.role).length <= 1) { notify('Keep at least one — the game picks from these.', 'warn'); return; }
    f.points = f.points.filter((_: any, i: number) => i !== selPt);
    selPt = Math.max(0, selPt - 1); changed(); redraw++;
  }

  /** Point marker: a ring (on the floor or at its height) with an arrow for the direction it faces; seats also get a seat square. */
  function pointLines(q: any, selected: boolean): { points: number[]; color: [number, number, number, number]; top?: boolean }[] {
    const g = toGL(q.pos), r = 0.12, pts: number[] = [];
    for (let k = 0; k < 16; k++) {
      const a0 = (k / 16) * Math.PI * 2, a1 = ((k + 1) / 16) * Math.PI * 2;
      pts.push(g[0] + Math.cos(a0) * r, g[1] + 0.01, g[2] + Math.sin(a0) * r, g[0] + Math.cos(a1) * r, g[1] + 0.01, g[2] + Math.sin(a1) * r);
    }
    const R = eulerMat(q.rot), fw = point(R, [0, 0, 1]), dir = [fw[0], 0, -fw[2]], l = Math.hypot(dir[0], dir[2]) || 1;
    const tip = [g[0] + (dir[0] / l) * r * 2, g[1] + 0.01, g[2] + (dir[2] / l) * r * 2];
    pts.push(g[0], g[1] + 0.01, g[2], ...tip);
    if (q.role === 'sit') { const s2 = 0.2; pts.push(g[0] - s2, 0.45, g[2] - s2, g[0] + s2, 0.45, g[2] - s2, g[0] + s2, 0.45, g[2] - s2, g[0] + s2, 0.45, g[2] + s2, g[0] + s2, 0.45, g[2] + s2, g[0] - s2, 0.45, g[2] + s2, g[0] - s2, 0.45, g[2] + s2, g[0] - s2, 0.45, g[2] - s2); }
    return [{ points: pts, color: selected ? [1, 0.85, 0.2, 1] : ROLE_COL[q.role] ?? [1, 1, 1, 1], top: true }];
  }

  function makePointHandles(q: any): Handle[] {
    if (!v3) return [];
    const L = v3.dist * 0.13, out: Handle[] = [], P = (p: number[]) => v3!.project(p), c = toGL(q.pos);
    const axis = (id: string, dir: number[], color: Col, k: number, sign: number) => {
      const tip = add3(c, dir, L), side = Math.abs(dir[1]) > 0.9 ? [1, 0, 0] : [0, 1, 0];
      const draw = [...c, ...tip, ...tip, ...add3(add3(tip, dir, -L * 0.15), side, L * 0.06), ...tip, ...add3(add3(tip, dir, -L * 0.15), side, -L * 0.06)];
      out.push({ id, kind: 'axis', color, draw, origin: c, dir, hit: (x, y) => segDist(x, y, P(c), P(tip)),
        apply: (pt, amt, fine) => { pt.pos[k] = snap(pt.pos[k] + sign * amt, fine); } });
    };
    if (tool === 'rotate') {
      // Turning around the vertical is what matters for people (which way they face).
      const pts: number[] = [], seg = 48;
      for (let i = 0; i < seg; i++) {
        const a0 = (i / seg) * Math.PI * 2, a1 = ((i + 1) / seg) * Math.PI * 2;
        pts.push(c[0] + Math.cos(a0) * L, c[1], c[2] + Math.sin(a0) * L, c[0] + Math.cos(a1) * L, c[1], c[2] + Math.sin(a1) * L);
      }
      out.push({ id: 'pry', kind: 'ring', color: GREEN, draw: pts, origin: c, n: [0, 1, 0],
        apply: (pt, deg, fine) => rotateSpot(pt, 1, -deg, fine),
        hit: (x, y) => { let best = Infinity; for (let i = 0; i + 5 < pts.length; i += 6) best = Math.min(best, segDist(x, y, P(pts.slice(i, i + 3)), P(pts.slice(i + 3, i + 6)))); return best; } });
      return out;
    }
    axis('px', [1, 0, 0], RED, 0, 1);
    axis('py', [0, 1, 0], GREEN, 1, 1);
    axis('pz', [0, 0, 1], BLUE, 2, -1);
    const r = L * 0.14, q4 = [[-r, -r], [r, -r], [r, r], [-r, r]].map(([u, v]) => [c[0] + u, c[1], c[2] + v]), sq: number[] = [];
    for (let i = 0; i < 4; i++) sq.push(...q4[i], ...q4[(i + 1) % 4]);
    out.push({ id: 'pxz', kind: 'plane', color: YELLOW, draw: sq, origin: c, n: [0, 1, 0], hit: (x, y) => pointDist(x, y, P(c)) - 4,
      apply: (pt, d, fine) => { pt.pos = [snap(pt.pos[0] + d[0], fine), pt.pos[1], snap(pt.pos[2] - d[2], fine)]; } });
    return out;
  }

  /** A seated stand-in (GL, facing +z, hips at the seat height). */
  const sitGeom: Geom = (() => {
    const out: number[] = [];
    const box = (x0: number, y0: number, z0: number, x1: number, y1: number, z1: number) => {
      const faces: [number[], number[][]][] = [
        [[1, 0, 0], [[x1, y0, z0], [x1, y1, z0], [x1, y1, z1], [x1, y0, z1]]], [[-1, 0, 0], [[x0, y0, z1], [x0, y1, z1], [x0, y1, z0], [x0, y0, z0]]],
        [[0, 1, 0], [[x0, y1, z0], [x0, y1, z1], [x1, y1, z1], [x1, y1, z0]]], [[0, -1, 0], [[x0, y0, z1], [x0, y0, z0], [x1, y0, z0], [x1, y0, z1]]],
        [[0, 0, 1], [[x1, y0, z1], [x1, y1, z1], [x0, y1, z1], [x0, y0, z1]]], [[0, 0, -1], [[x0, y0, z0], [x0, y1, z0], [x1, y1, z0], [x1, y0, z0]]],
      ];
      for (const [n, q] of faces) for (const k of [0, 1, 2, 0, 2, 3]) out.push(...q[k], ...n, 0, 0);
    };
    box(-0.15, 0.42, -0.05, -0.02, 0.55, 0.4); box(0.02, 0.42, -0.05, 0.15, 0.55, 0.4); // thighs
    box(-0.15, 0, 0.3, -0.02, 0.45, 0.42); box(0.02, 0, 0.3, 0.15, 0.45, 0.42);         // shins
    box(-0.19, 0.55, -0.12, 0.19, 1.15, 0.08);                                          // body
    box(-0.27, 0.62, -0.06, -0.19, 1.12, 0.06); box(0.19, 0.62, -0.06, 0.27, 1.12, 0.06); // arms
    box(-0.1, 1.17, -0.1, 0.1, 1.38, 0.1);                                              // head
    box(-0.03, 1.25, 0.1, 0.03, 1.29, 0.14);                                            // nose
    const min = [Infinity, Infinity, Infinity], max = [-Infinity, -Infinity, -Infinity];
    for (let i = 0; i < out.length; i += 8) for (let q = 0; q < 3; q++) { min[q] = Math.min(min[q], out[i + q]); max[q] = Math.max(max[q], out[i + q]); }
    return { data: new Float32Array(out), count: out.length / 8, min, max };
  })();

  /** Stand-ins on the people points (not on item drop / queue markers), facing the way the point faces. */
  function pointPeople(): SceneObj[] {
    const out: SceneObj[] = [];
    for (const q of pointsOf()) {
      if (q.role === 'placeItems') continue;
      const g = toGL(q.pos), yaw = 180 - q.rot[1];
      out.push({ geom: q.role === 'sit' ? sitGeom : personGeom, matrix: mul(translate(g[0], q.role === 'sit' ? 0 : g[1], g[2]), rotY(yaw)), color: [0.95, 0.75, 0.5, 0.5] });
    }
    return out;
  }

  /** One stand-in per distinct customer point, turned towards the middle of the spots it serves. */
  function peopleObjs(): SceneObj[] {
    const list: any[] = f?.spots ?? (t?.spots ?? []).filter((q: any) => q.kind === spotKind);
    const groups = new Map<string, { c: number[]; targets: number[][] }>();
    for (const sp of list) {
      if (!sp.customer) continue;
      const key = `${Math.round(sp.customer[0] * 200)}|${Math.round(sp.customer[1] * 200)}`;
      const g = groups.get(key) ?? { c: sp.customer as number[], targets: [] as number[][] };
      g.targets.push(sp.pos);
      groups.set(key, g);
    }
    const objs: SceneObj[] = [];
    for (const g of groups.values()) {
      const at = [g.c[0], 0, -g.c[1]];
      const tx = g.targets.reduce((a, q) => a + q[0], 0) / g.targets.length, tz = -g.targets.reduce((a, q) => a + q[2], 0) / g.targets.length;
      const yaw = (Math.atan2(tx - at[0], tz - at[2]) * 180) / Math.PI;
      objs.push({ geom: personGeom, matrix: mul(translate(at[0], 0, at[2]), rotY(yaw)), color: [0.55, 0.85, 0.6, 0.5] });
    }
    return objs;
  }
  // Fill preview: a vanilla shop item drawn in every item spot, placed like the game does (ShelfCompartment.CalculatePositionList).
  interface FillItem { key: string; label: string; group: string; mesh: string; texture: string; tpl: ItemTpl }
  let fillItems = $state<FillItem[]>([]);
  let fillKey = $state('');
  let lastFill = 'BasicCardPack';        // what F switches the fill preview back on with
  let itemPrefab: any = null;

  const accUrl = (rel?: string) => (rel ? `/acc/${rel.split('/').map(encodeURIComponent).join('/')}?v=${bust}` : '');
  const furnUrl = (file?: string) => (file ? `/furntemplates/${encodeURIComponent(file)}` : '');
  const typeInfo = (t: string) => types.find((x) => x.type === t);
  const baseOf = (x: any) => x?.base || typeInfo(x?.type)?.defaultBase || '';
  const tplOf = (x: any) => pieces.find((p) => p.base === baseOf(x));
  let list = $derived<any[]>(view?.furniture ?? []);
  // Display order of the list (the library order itself stays the Gamify ladder order): library order, name, or source
  // (where it came from: converted mods grouped by mod, then things made in Studio).
  const SORT_LABELS: Record<string, string> = { order: 'List order', name: 'Name', source: 'Source' };
  const originOf = (x: any) => view?.origins?.[x.id];
  const sourceKey = (x: any) => (originOf(x) ? '0 ' + (originOf(x).mod || '').toLowerCase() : '1');
  function sortShown(list: any[], by: string): any[] {
    if (by === 'name') return [...list].sort((a, b) => a.name.localeCompare(b.name));
    if (by === 'source') return [...list].sort((a, b) => sourceKey(a).localeCompare(sourceKey(b)) || a.name.localeCompare(b.name));
    return list;
  }
  let sortBy = $state((() => { try { return localStorage.getItem('furniture.sort') || 'order'; } catch { return 'order'; } })());
  let shown = $derived(sortShown(list, sortBy));
  let t = $derived(tplOf(f));
  let info = $derived(typeInfo(f?.type));
  let spotKind = $derived<string>(info?.spots ?? '');
  let bases = $derived(pieces.filter((p) => p.type && p.type === f?.type));
  let baseHeight = $derived(t?.bounds ? t.bounds[1][1] - t.bounds[0][1] : 1);

  // ---------------------------------------------------------------- data

  async function loadTemplates() {
    const raw = await App.FurnitureTemplates();
    pieces = raw ? JSON.parse(raw).pieces ?? [] : [];
    loadFillItems().catch(() => {});
    tplStatus = await App.TemplatesStatus();
  }

  /** Items for the fill preview: packs and boxes (packs.json) and the accessories/figurines (accessories.json) of the game files. */
  async function loadFillItems() {
    const get = async (f: string) => { const r = await fetch(`/acctemplates/${f}`); return r.ok ? r.json() : null; };
    const [packs, acc] = await Promise.all([get('packs.json'), get('accessories.json')]);
    itemPrefab = acc?.itemPrefab ?? null;
    const out: FillItem[] = [];
    const words = (t: string) => t.replace(/([a-z])([A-Z0-9])/g, '$1 $2').replace(/_/g, ' ');
    for (const it of packs?.items ?? []) {
      if (!it.mesh || !it.itemDimension) continue;
      out.push({ key: it.type, label: words(it.type), group: it.kind === 'box' ? 'Card boxes' : 'Card packs', mesh: `/acctemplates/${encodeURIComponent(it.mesh)}`,
        texture: it.texture ? `/templates/${encodeURIComponent(it.texture)}` : '', tpl: { type: it.type, itemDimension: it.itemDimension, isTallItem: it.isTallItem,
          posYOffsetInBox: it.posYOffsetInBox, scaleOffsetInBox: it.scaleOffsetInBox, bounds: it.bounds, mesh: it.mesh, texture: it.texture } });
    }
    for (const it of acc?.items ?? []) {
      if (!it.mesh || !it.itemDimension) continue;
      out.push({ key: it.type, label: it.name || words(it.type), group: it.category === 'Figurine' || it.type.startsWith('Toy_') ? 'Figurines' : 'Accessories',
        mesh: `/acctemplates/${encodeURIComponent(it.mesh)}`, texture: it.texture ? `/acctemplates/${encodeURIComponent(it.texture)}` : '', tpl: it });
    }
    fillItems = out;
  }
  let fillItem = $derived(fillItems.find((x) => x.key === fillKey));
  /** F: fill preview on (the last item used) / off. */
  function toggleFill() {
    if (fillKey) { lastFill = fillKey; fillKey = ''; return; }
    fillKey = fillItems.some((x) => x.key === lastFill) ? lastFill : fillItems[0]?.key ?? '';
  }
  /** Shift+F: next fill item (previous with Ctrl). */
  function cycleFill(dir = 1) {
    if (!fillItems.length) return;
    const i = fillItems.findIndex((x) => x.key === fillKey);
    fillKey = fillItems[(i + dir + fillItems.length) % fillItems.length].key;
    lastFill = fillKey;
  }
  let fillGroups = $derived([...new Set(fillItems.map((x) => x.group))]);

  /** A spot as the game's compartment (Unity piece space), for the placement maths. Vanilla spots keep their 144 slots. */
  function spotComp(s: any, custom: boolean): Compartment {
    const R = eulerMat(s.rot), col = (c: number) => [R[c * 4], R[c * 4 + 1], R[c * 4 + 2]];
    const right = col(0), up = col(1), fwd = col(2);
    const [w, d, h] = s.size ?? [0.49, 0.236, 0];
    const sign = t?.heightGoesUp ? 1 : -1;
    const start = [0, 1, 2].map((q) => s.pos[q] + right[q] * w / 2 - fwd[q] * d / 2 - sign * up[q] * h / 2);
    const g = s.grid ?? [4, 8, 1];
    return { path: '', start, endWidth: start, endDepth: start, endHeight: start, right, up, forward: fwd, width: w, depth: d, height: h,
      sizeX: g[0], sizeY: g[1], sizeZ: g[2], m_CanPutItem: true, m_ApplyScaleOffset: !!t?.applyScaleOffset, m_HeightGoesUp: !!t?.heightGoesUp,
      m_AffectedByTallItem: !!t?.affectedByTallItem, posCount: custom ? 0 : 144 };
  }
  /** How many of the fill item a spot holds. */
  const fillCount = (s: any, custom: boolean) => (fillItem && s.kind === 'items' ? slotGrid(spotComp(s, custom), fillItem.tpl).count : 0);
  let fillGeom: Geom | null = null;
  let loadedFill = '';

  let fromCatalog = $state(false);

  async function addedFromCatalog(ids: string[]) {
    fromCatalog = false;
    await load();
    if (ids.length) notify(`Added ${ids.length} piece${ids.length === 1 ? '' : 's'} from the catalog.`, 'ok');
  }

  async function saveToCatalog() {
    if (!f) return;
    if (dirty) { notify('Save your changes first — the catalog takes the saved version.', 'warn'); return; }
    try { await App.SaveToCatalog('furniture', [f.id]); notify(`"${f.name}" is now the catalog's version.`, 'ok'); }
    catch (e) { notify(errText(e), 'error'); }
  }

  async function load(keep = true) {
    view = await App.Accessories();
    if (!keep || !list.some((x) => x.id === selectedId)) selectedId = list[0]?.id ?? '';
    select(selectedId);
  }

  function select(id: string) {
    selectedId = id;
    picked = id ? [id] : [];
    anchor = id;
    const x = list.find((p) => p.id === id);
    f = x ? JSON.parse(JSON.stringify(x)) : null;
    let l: any = null;
    try { l = x && view.layouts[id] ? JSON.parse(view.layouts[id]) : null; } catch { l = null; }
    layout = l && l.version === 'fur1' ? { ...newFurLayout(), ...l } : newFurLayout();
    sel = 0;
    dirty = false;
    framedFor = '';
    resetHistory();
  }

  async function trySelect(id: string) {
    if (dirty && !(await ask('Discard unsaved changes to this piece?'))) return;
    select(id);
  }

  async function add() {
    if (dirty && !(await ask('Discard unsaved changes to this piece?'))) return;
    try {
      const x = await App.NewFurniture(newType, '', '');
      await load();
      select(x.id);
      notify(`Added ${x.name}.`, 'ok');
    } catch (e) { notify(errText(e), 'error'); }
  }

  async function remove() {
    if (!f || !(await ask(`Delete "${f.name}"? Placed copies disappear from saves the next time they load.`))) return;
    try { view = await App.DeleteFurniture(f.id); dirty = false; await load(false); } catch (e) { notify(errText(e), 'error'); }
  }

  /** List click: plain = open it; Ctrl = add/remove it from the selection; Shift = select the range from the last click. */
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
    await trySelect(id);
  }

  /** Delete key in the list: deletes the selected pieces after one confirmation (not the spot shortcut of the 3D view). */
  async function listKey(e: KeyboardEvent) {
    if (e.key !== 'Delete' || !picked.length) return;
    e.preventDefault();
    e.stopPropagation();
    const names = list.filter((x) => picked.includes(x.id)).map((x) => x.name);
    const what = names.length === 1 ? `"${names[0]}"` : `${names.length} pieces`;
    if (!(await ask(`Delete ${what}? Placed copies disappear from saves the next time they load.`))) return;
    try {
      view = await App.DeleteFurnitureMany(picked);
      dirty = false;
      await load(false);
      notify(`Deleted ${what}.`, 'ok');
    } catch (err) { notify(errText(err), 'error'); }
  }

  // ---------------------------------------------------------------- undo / redo (Ctrl+Z, Ctrl+Y / Ctrl+Shift+Z)
  // Snapshots of the piece and its editor state. `base` is the state after the last edit; an edit pushes it (the state before the
  // edit) unless it continues a burst (typing, <600 ms apart). A whole drag is one step: pushed when it starts.

  let undoStack = $state<string[]>([]);
  let redoStack = $state<string[]>([]);
  let base = '';
  let lastEdit = 0;
  const snapshot = () => JSON.stringify({ f, layout });
  function resetHistory() { undoStack = []; redoStack = []; base = snapshot(); lastEdit = 0; }
  function pushUndo(state: string) {
    if (!state || undoStack.at(-1) === state) return;
    undoStack = [...undoStack.slice(-99), state];
    redoStack = [];
  }
  function restore(state: string) {
    const o = JSON.parse(state);
    f = o.f;
    layout = o.layout;
    base = state;
    lastEdit = 0;
    if (!f?.spots?.[sel]) sel = 0;
    dirty = true;
    redraw++;
  }
  function undo() {
    if (!undoStack.length || dragging) return;
    redoStack = [...redoStack, snapshot()];
    const prev = undoStack[undoStack.length - 1];
    undoStack = undoStack.slice(0, -1);
    restore(prev);
  }
  function redo() {
    if (!redoStack.length || dragging) return;
    undoStack = [...undoStack, snapshot()];
    const next = redoStack[redoStack.length - 1];
    redoStack = redoStack.slice(0, -1);
    restore(next);
  }

  function changed() {
    dirty = true;
    const now = Date.now();
    if (!dragging && now - lastEdit > 600) pushUndo(base);
    if (!dragging) lastEdit = now;
    queueMicrotask(() => { if (!dragging) base = snapshot(); });
  }
  const opt = (v: string): number | undefined => (v === '' || isNaN(+v) ? undefined : +v);

  async function setType(tp: string) {
    if (tp === f.type) return;
    if (f.spots?.length && !(await ask('Changing the type removes this piece\'s custom spots. Continue?'))) return;
    f.type = tp; f.base = ''; f.spots = undefined; sel = 0; framedFor = ''; changed();
  }

  function setBase(b: string) {
    f.base = b === info?.defaultBase ? '' : b;
    framedFor = '';
    changed();
  }

  // ---------------------------------------------------------------- model

  async function importModel() {
    busy = true;
    try {
      const r = await App.ImportFigurineModel();
      if (!r.model) return;
      Object.assign(layout, { model: r.model, texture: r.texture, sourceName: r.name, triangles: r.triangles, warnings: r.warnings ?? [],
        rotX: 0, rotY: 0, rotZ: 0, height: baseHeight, autoIcon: true });
      framedFor = '';
      changed();
      notify(`Imported ${r.name}: ${r.triangles.toLocaleString()} triangles.`, r.warnings?.length ? 'warn' : 'ok');
    } catch (e) { notify(errText(e), 'error'); } finally { busy = false; }
  }

  function removeModel() {
    layout.model = ''; layout.texture = ''; layout.sourceName = '';
    if (layout.autoIcon) { f.icon = ''; layout.autoIcon = false; }
    f.mesh = ''; f.texture = '';
    framedFor = '';
    changed();
  }

  function turn(axis: 'rotX' | 'rotY' | 'rotZ', deg: number) {
    layout[axis] = ((((layout[axis] + deg) % 360) + 540) % 360) - 180;
    changed();
  }

  async function pickIcon() {
    try {
      const rel = await App.PickAccessoryImage();
      if (!rel) return;
      f.icon = rel; layout.autoIcon = false; changed();
    } catch (e) { notify(errText(e), 'error'); }
  }

  // ---------------------------------------------------------------- spots

  const cardSize = [0.065, 0.09]; // Card3d collider (runtime-facts)

  function spotsFromBase() {
    f.spots = JSON.parse(JSON.stringify((t?.spots ?? []).filter((s: any) => s.kind === spotKind)));
    ensureCustomers();
    sel = 0; changed();
  }
  /** Every spot gets a customer point (customers can't take from a spot without one); missing ones go in front of the spot. */
  function ensureCustomers(): number {
    let n = 0;
    for (const sp of f?.spots ?? []) if (!sp.customer || sp.customer.length !== 2) { sp.customer = defaultCustomer(sp); n++; }
    return n;
  }
  function useBaseSpots() { f.spots = undefined; sel = 0; changed(); }
  function addSpot() {
    if (!f.spots) spotsFromBase(); // the vanilla spots become editable, then the new one joins them
    const src = f.spots?.[sel] ?? (t?.spots ?? []).find((s: any) => s.kind === spotKind);
    const s = src ? JSON.parse(JSON.stringify(src)) : spotKind === 'card'
      ? { kind: 'card', pos: [0, 1, 0.1], rot: [15, 180, 0] }
      : { kind: 'items', pos: [0, 1, 0.15], rot: [0, 0, 0], size: [0.49, 0.236, 0], grid: [4, 8, 1] };
    if (src) shiftSpot(s, 0, spotKind === 'card' ? 0.08 : 0.1);
    if (!s.customer) s.customer = defaultCustomer(s);
    f.spots = [...(f.spots ?? []), s];
    sel = f.spots.length - 1; part = 'spot'; tool = 'move'; changed();
  }
  function deleteSpot() {
    if (!f.spots) spotsFromBase();
    if (!f.spots?.length) return;
    f.spots = f.spots.filter((_: any, i: number) => i !== sel);
    sel = Math.max(0, sel - 1); part = 'spot'; changed();
  }
  /** Moves a spot and its customer point / price tag with it. */
  function shiftSpot(s: any, axis: number, d: number) {
    s.pos[axis] = r3(s.pos[axis] + d);
    if (s.priceTag) s.priceTag[axis] = r3(s.priceTag[axis] + d);
    if (s.customer && axis !== 1) s.customer[axis === 0 ? 0 : 1] = r3(s.customer[axis === 0 ? 0 : 1] + d);
  }
  function setPos(axis: number, v: number) {
    const s = f.spots[sel];
    if (isNaN(v)) return;
    shiftSpot(s, axis, v - s.pos[axis]);
    changed();
  }
  const r3 = (v: number) => Math.round(v * 1000) / 1000;
  const capacity = (s: any) => (s.grid ? s.grid[0] * s.grid[1] * s.grid[2] : 1);

  /** Metres per grid unit of the base piece's item spots (vanilla shop shelf: 0.1225 wide, 0.0295 deep); height: a pack ≈ 12 cm. */
  function unitSize(): number[] {
    const b = (t?.spots ?? []).find((q: any) => q.kind === 'items' && q.size && q.grid);
    const u = b ? [b.size[0] / b.grid[0], b.size[1] / b.grid[1], b.size[2] > 0 ? b.size[2] / b.grid[2] : 0.12] : [0.1225, 0.0295, 0.12];
    return u.map((v) => (v > 0 ? v : 0.12));
  }
  /** Grid that keeps the vanilla item spacing for the spot's size (at most 512 units). */
  function fitGrid(sp: any) {
    if (sp.kind !== 'items' || !sp.size) return;
    const u = unitSize();
    let g = [Math.max(1, Math.round(sp.size[0] / u[0])), Math.max(1, Math.round(sp.size[1] / u[1])), sp.size[2] > 0 ? Math.max(1, Math.round(sp.size[2] / u[2])) : 1];
    while (g[0] * g[1] * g[2] > 512) g = g.map((v, i) => (i === g.indexOf(Math.max(...g)) ? Math.max(1, v - 1) : v));
    sp.grid = g;
  }
  /** How many packs a spot holds (the fill preview's maths with a Basic Card Pack). */
  const packsIn = (sp: any) => {
    const pack = fillItems.find((x) => x.key === 'BasicCardPack');
    return pack && sp.kind === 'items' ? slotGrid(spotComp(sp, true), pack.tpl).count : capacity(sp);
  };

  // ---------------------------------------------------------------- 3D

  let canvas = $state<HTMLCanvasElement>();
  let v3: FigurineView | null = null;
  let baseGeom: Geom | null = null, figGeom: Geom | null = null;
  let loadedBase = '', loadedModel = '', loadedTex = '';
  let framedFor = '';

  const toGL = (p: number[]) => [p[0], p[1], -p[2]];
  /** Unity piece-local matrix of a spot (T · Ry · Rx · Rz); the numbers of Unity's rotation matrices are the standard ones. */
  const spotMat = (s: any): Mat => mul(translate(s.pos[0], s.pos[1], s.pos[2]), mul(rotY(s.rot[1]), mul(rotX(s.rot[0]), rotZ(s.rot[2]))));

  function anchorUnity(): number[] {
    const b = t?.bounds;
    return b ? [(b[0][0] + b[1][0]) / 2, b[0][1], (b[0][2] + b[1][2]) / 2] : [0, 0, 0];
  }

  /** Source model → piece space in GL (mirrors figurine.Place in Go). */
  function placement(): { m: Mat; box: Box } | null {
    if (!figGeom) return null;
    const R = mul(rotY(layout.rotY), mul(rotX(layout.rotX), rotZ(layout.rotZ)));
    const d = figGeom.data, lo = [Infinity, Infinity, Infinity], hi = [-Infinity, -Infinity, -Infinity];
    for (let i = 0; i < d.length; i += 8) {
      for (let q = 0; q < 3; q++) {
        const val = R[q] * d[i] + R[4 + q] * d[i + 1] + R[8 + q] * d[i + 2];
        if (val < lo[q]) lo[q] = val;
        if (val > hi[q]) hi[q] = val;
      }
    }
    const h = hi[1] - lo[1];
    if (!(h > 0)) return null;
    const s = (layout.height || baseHeight) / h;
    const c = [(lo[0] + hi[0]) / 2, lo[1], (lo[2] + hi[2]) / 2];
    const a = toGL(anchorUnity());
    const m = mul(translate(a[0], a[1], a[2]), mul(scale(s), mul(translate(-c[0], -c[1], -c[2]), R)));
    return { m, box: { min: [a[0] + (lo[0] - c[0]) * s, a[1], a[2] + (lo[2] - c[2]) * s], max: [a[0] + (hi[0] - c[0]) * s, a[1] + h * s, a[2] + (hi[2] - c[2]) * s] } };
  }

  function tintRGB(): [number, number, number, number] {
    const m = /^#([0-9a-f]{6})$/i.exec(f?.tint ?? '');
    if (!m) return [0.72, 0.74, 0.78, 1];
    const n = parseInt(m[1], 16);
    return [((n >> 16) & 255) / 255 * 0.9 + 0.05, ((n >> 8) & 255) / 255 * 0.9 + 0.05, (n & 255) / 255 * 0.9 + 0.05, 1];
  }

  const CUST_R = 0.1, CUST_POST = 0.3; // customer point marker: floor ring radius / post height (m)
  function spotLines(s: any, selected: boolean, highlight?: [number, number, number, number]) {
    const out: { points: number[]; color: [number, number, number, number]; top?: boolean }[] = [];
    const M = spotMat(s);
    const seg = (a: number[], b: number[], pts: number[]) => pts.push(...toGL(point(M, a)), ...toGL(point(M, b)));
    const col: [number, number, number, number] = !s.customer && f?.spots ? [1, 0.25, 0.25, 1]
      : selected && part === 'spot' ? [1, 0.85, 0.2, 1] : selected ? [1, 0.95, 0.6, 1]
      : s.kind === 'card' ? [0.9, 0.4, 0.9, 1] : [0.3, 0.8, 1, 1];
    const pts: number[] = [];
    if (s.kind === 'items' && s.size) {
      const [w, d, h] = s.size, hw = w / 2, hd = d / 2, hh = h / 2;
      const c = [[-hw, -hh, -hd], [hw, -hh, -hd], [hw, -hh, hd], [-hw, -hh, hd]];
      for (let i = 0; i < 4; i++) seg(c[i], c[(i + 1) % 4], pts);
      if (h > 0) for (let i = 0; i < 4; i++) { const u = [c[i][0], hh, c[i][2]], u2 = [c[(i + 1) % 4][0], hh, c[(i + 1) % 4][2]]; seg(u, u2, pts); seg(c[i], u, pts); }
      if (selected && s.grid) {
        const gp: number[] = [];
        for (let i = 1; i < s.grid[0]; i++) { const x = -hw + (w * i) / s.grid[0]; seg([x, -hh, -hd], [x, -hh, hd], gp); }
        for (let j = 1; j < s.grid[1]; j++) { const z = -hd + (d * j) / s.grid[1]; seg([-hw, -hh, z], [hw, -hh, z], gp); }
        out.push({ points: gp, color: [1, 0.85, 0.2, 0.45] });
      }
      // Front edge marker (customers face this side: −z of the spot is where items start from the aisle).
      seg([-hw * 0.3, -hh, -hd - 0.02], [hw * 0.3, -hh, -hd - 0.02], pts);
    } else {
      const [cw, ch] = cardSize, x = cw / 2, y = ch / 2;
      const c = [[-x, -y, 0], [x, -y, 0], [x, y, 0], [-x, y, 0]];
      for (let i = 0; i < 4; i++) seg(c[i], c[(i + 1) % 4], pts);
      seg([0, 0, 0], [0, 0, 0.04], pts);
    }
    out.push({ points: pts, color: highlight ?? col, top: !!highlight });
    const mark = (p: number[], r: number, color: [number, number, number, number]) => {
      const g = toGL(p);
      out.push({ points: [g[0] - r, g[1], g[2], g[0] + r, g[1], g[2], g[0], g[1], g[2] - r, g[0], g[1], g[2] + r, g[0], g[1] - r, g[2], g[0], g[1] + r, g[2]], color, top: true } as any);
    };
    const yel: [number, number, number, number] = [1, 0.85, 0.2, 1];
    if (s.customer) {
      const g = [s.customer[0], 0.01, -s.customer[1]], r = CUST_R, pts: number[] = [];
      for (let k = 0; k < 20; k++) {
        const a0 = (k / 20) * Math.PI * 2, a1 = ((k + 1) / 20) * Math.PI * 2;
        pts.push(g[0] + Math.cos(a0) * r, g[1], g[2] + Math.sin(a0) * r, g[0] + Math.cos(a1) * r, g[1], g[2] + Math.sin(a1) * r);
      }
      pts.push(g[0], g[1], g[2], g[0], CUST_POST, g[2], g[0] - r * 0.4, g[1], g[2], g[0] + r * 0.4, g[1], g[2], g[0], g[1], g[2] - r * 0.4, g[0], g[1], g[2] + r * 0.4);
      out.push({ points: pts, color: selected && part === 'customer' ? yel : [0.45, 0.95, 0.45, selected ? 1 : 0.75], top: true } as any);
    }
    if (s.priceTag && showTags) {
      // The tag keeps the spot's rotation in game (it's part of the spot), so it faces the same way as the spot.
      const R = eulerMat(s.rot), P = (lx: number, ly: number) => {
        const q = point(R, [lx, ly, 0]);
        return toGL([s.priceTag[0] + q[0], s.priceTag[1] + q[1], s.priceTag[2] + q[2]]);
      };
      const c4 = [P(-TAG_W / 2, -TAG_H / 2), P(TAG_W / 2, -TAG_H / 2), P(TAG_W / 2, TAG_H / 2), P(-TAG_W / 2, TAG_H / 2)];
      const pts: number[] = [];
      for (let k = 0; k < 4; k++) pts.push(...c4[k], ...c4[(k + 1) % 4]);
      pts.push(...c4[0], ...c4[2], ...c4[1], ...c4[3]);
      out.push({ points: pts, color: selected && part === 'tag' ? yel : [1, 1, 1, 0.95], top: true } as any);
    } else if (s.priceTag) mark(s.priceTag, selected ? 0.04 : 0.025, selected && part === 'tag' ? yel : [1, 1, 1, 0.85]);
    return out;
  }

  // ---------------------------------------------------------------- placement area
  // The box the game keeps free of other furniture and walls when placing the piece (and snaps neighbours to). The def stores its
  // centre x/z and width/depth; height and rotation stay the base piece's.

  /** Effective area: the piece's own, or the base piece's (from the game files). */
  function areaOf(): { pos: number[]; size: number[] } | null {
    if (f?.area) return f.area;
    const a = t?.area;
    return a ? { pos: [a.pos[0], a.pos[2]], size: [a.size[0], a.size[2]] } : null;
  }
  const areaY = () => (t?.area ? [t.area.pos[1] - t.area.size[1] / 2, t.area.pos[1] + t.area.size[1] / 2] : [0, 0.5]);
  const ORANGE: [number, number, number, number] = [1, 0.55, 0.15, 0.95];

  function areaLines(): { points: number[]; color: [number, number, number, number] }[] {
    const a = areaOf();
    if (!a) return [];
    const [y0, y1] = areaY(), hw = a.size[0] / 2, hd = a.size[1] / 2, cx = a.pos[0], cz = -a.pos[1];
    const box = boxLines(ident(), { min: [cx - hw, y0, cz - hd], max: [cx + hw, y1, cz + hd] });
    const r = 0.05, marker = [cx - r, 0.01, cz, cx + r, 0.01, cz, cx, 0.01, cz - r, cx, 0.01, cz + r];
    const on = part === 'area';
    return [{ points: [...box, ...marker], color: on ? [1, 0.85, 0.2, 1] : ORANGE }];
  }

  function fitAreaToModel() {
    const p = placement(), a = areaOf();
    if (!p || !a || !t?.area || !t?.bounds) return;
    // Keep the vanilla piece's clearance around its model (aisle room), around the new model instead.
    const extraW = Math.max(0, t.area.size[0] - (t.bounds[1][0] - t.bounds[0][0]));
    const extraD = Math.max(0, t.area.size[2] - (t.bounds[1][2] - t.bounds[0][2]));
    const w = p.box.max[0] - p.box.min[0], d = p.box.max[2] - p.box.min[2];
    f.area = { pos: [r3((p.box.min[0] + p.box.max[0]) / 2), r3(-(p.box.min[2] + p.box.max[2]) / 2)], size: [r3(w + extraW), r3(d + extraD)] };
    part = 'area';
    changed();
  }

  async function rebuild(reframe = false) {
    if (!v3 || !f) return;
    try {
      if (t?.model && t.model !== loadedBase) { baseGeom = await loadGeom(furnUrl(t.model), true); loadedBase = t.model; }
      if (!t?.model) { baseGeom = null; loadedBase = ''; }
      if (layout.model && layout.model !== loadedModel) { figGeom = await loadGeom(accUrl(layout.model).split('?')[0], false); loadedModel = layout.model; }
      if (!layout.model) { figGeom = null; loadedModel = ''; }
      if (layout.texture && layout.texture !== loadedTex) { v3.setTexture('fig', await loadImage(accUrl(layout.texture))); loadedTex = layout.texture; }
    } catch (e) { notify(errText(e), 'error'); return; }
    if (fillItem && fillItem.key !== loadedFill) {
      try {
        fillGeom = await loadGeom(fillItem.mesh, true);
        if (fillItem.texture) v3.setTexture('fill', await loadImage(fillItem.texture));
        loadedFill = fillItem.key;
      } catch (e) { notify(errText(e), 'error'); fillKey = ''; }
    }
    const objs: SceneObj[] = [];
    const p = placement();
    if (showPeople && spotKind) objs.push(...peopleObjs());
    if (showPeople) objs.push(...pointPeople());
    if (fillItem && fillGeom && spotKind === 'items') {
      const mm = meshMatrix(itemPrefab), custom = !!f.spots;
      const spotList: any[] = f.spots ?? (t?.spots ?? []).filter((x: any) => x.kind === 'items');
      for (const sp of spotList) {
        if (sp.kind !== 'items') continue;
        for (const slot of slotMatrices(spotComp(sp, custom), fillItem.tpl))
          objs.push({ geom: fillGeom, matrix: mul(MIRROR_Z, mul(slot, mul(mm, MIRROR_Z))), texture: fillItem.texture ? 'fill' : undefined });
      }
    }
    if (p && figGeom) objs.push({ geom: figGeom, matrix: p.m, texture: layout.texture ? 'fig' : undefined, color: f.tint ? tintRGB() : [1, 1, 1, 1] });
    if (baseGeom && (!figGeom || showBase)) objs.push({ geom: baseGeom, matrix: ident(), color: figGeom ? [0.6, 0.62, 0.7, 0.35] : tintRGB() });
    const lines: { points: number[]; color: [number, number, number, number]; top?: boolean }[] = [];
    const spots: any[] = f.spots ?? (t?.spots ?? []).filter((s: any) => s.kind === spotKind);
    // The spots a selected customer point serves light up (green, on top) with a line from the point to each; a selected spot
    // shows a faint line to its customer point.
    const served = part === 'customer' && f.spots?.[sel] ? new Set([sel, ...sharedCustomer(sel)]) : null;
    const GREEN_HI: [number, number, number, number] = [0.45, 1, 0.45, 1];
    spots.forEach((s, i) => lines.push(...spotLines(s, !!f.spots && i === sel, served?.has(i) ? GREEN_HI : undefined)));
    if (f.spots?.[sel]?.customer) {
      const link: number[] = [];
      for (const i of served ?? [sel]) {
        const sp = f.spots[i];
        if (!sp?.customer) continue;
        link.push(sp.customer[0], CUST_POST, -sp.customer[1], ...toGL(sp.pos));
      }
      lines.push({ points: link, color: served ? GREEN_HI : [0.45, 0.95, 0.45, 0.45], top: !!served });
    }
    lines.push(...areaLines());
    pointsOf().forEach((q, i) => lines.push(...pointLines(q, part === 'point' && i === selPt)));
    handles = part === 'area' ? makeAreaHandles() : part === 'point' ? (pointsOf()[selPt] ? makePointHandles(pointsOf()[selPt]) : []) : f.spots?.[sel] ? makeHandles(f.spots[sel]) : [];
    for (const h of handles) lines.push({ points: h.draw, color: h.id === hot ? [1, 1, 1, 1] : h.color, top: true });
    // Floor grid around the footprint.
    const b = t?.bounds, r = b ? Math.max(b[1][0] - b[0][0], b[1][2] - b[0][2]) : 1.5, fl: number[] = [];
    for (let i = -4; i <= 4; i++) { const q = (i / 4) * r; fl.push(-r, 0, q, r, 0, q, q, 0, -r, q, 0, r); }
    lines.push({ points: fl, color: [0.35, 0.4, 0.5, 0.6] });
    if (p) lines.push({ points: boxLines(ident(), p.box), color: [0.35, 0.6, 1, 0.6] });
    v3.setScene(objs);
    v3.setLines(lines);
    const key = `${f.id}|${baseOf(f)}|${layout.model}`;
    if (reframe || framedFor !== key) {
      framedFor = key;
      const box = b ? { min: toGL([b[0][0], b[0][1], b[1][2]]), max: toGL([b[1][0], b[1][1], b[0][2]]) } : p?.box ?? { min: [-1, 0, -1], max: [1, 2, 1] };
      v3.frameBox(box, 0.8);
    }
  }

  $effect(() => {
    void [f, f?.spots, f?.points, selPt, f?.area, f?.tint, fillKey, fillItems, showPeople, showTags, f?.base, f?.type, sel, part, tool, redraw, layout.model, layout.texture, layout.rotX, layout.rotY, layout.rotZ, layout.height, showBase, pieces];
    untrack(() => rebuild());
  });

  /** Shop icon rendered from the own model (three-quarter front view, like the vanilla icons). */
  async function renderIcon(size: number[]): Promise<string> {
    await rebuild();
    const p = placement();
    if (!p || !figGeom) throw new Error('import a model first');
    const c = document.createElement('canvas');
    c.width = size[0] || 512; c.height = size[1] || 512;
    const v = new FigurineView(c, { preserve: true, interactive: false });
    try {
      if (layout.texture) v.setTexture('fig', await loadImage(accUrl(layout.texture)));
      v.setScene([{ geom: figGeom, matrix: p.m, texture: layout.texture ? 'fig' : undefined, color: f.tint ? tintRGB() : [1, 1, 1, 1] }]);
      v.yaw = 30; v.pitch = 15; v.fov = 0.35;
      v.frameBox(p.box, 0.9);
      v.renderNow();
      return c.toDataURL('image/png');
    } finally { v.dispose(); }
  }

  // ---------------------------------------------------------------- drag handles (3D view)
  //
  // Click a spot's body, its customer point or its price tag to select that part; the handles act on the selected part.
  // Handles live in GL space. Kinds: 'axis' (along a direction), 'plane' (on a plane through the handle, normal n), 'ring' (turn
  // around n). Values are written from a snapshot taken when the drag starts, so snapping never drifts.

  type Col = [number, number, number, number];
  interface Handle {
    id: string; kind: 'axis' | 'plane' | 'ring'; color: Col; draw: number[];
    origin: number[]; dir?: number[]; n?: number[]; apply: (s: any, amount: any, fine: boolean) => void;
    hit: (x: number, y: number) => number; // screen distance in px
  }
  let handles: Handle[] = [];
  let hot = '';
  let dragging: { h: Handle; start: any; mx: number; my: number; hit0: number[] | null; pushed?: boolean; linked?: number[] } | null = null;

  const RED: Col = [0.95, 0.3, 0.3, 1], GREEN: Col = [0.4, 0.9, 0.4, 1], BLUE: Col = [0.35, 0.55, 1, 1], YELLOW: Col = [1, 0.85, 0.2, 1];
  const AXES: [string, number[], Col][] = [['x', [1, 0, 0], RED], ['y', [0, 1, 0], GREEN], ['z', [0, 0, 1], BLUE]];
  const snap = (v: number, fine: boolean, step = 0.01) => (fine ? r3(v) : r3(Math.round(v / step) * step));
  const add3 = (a: number[], b: number[], k = 1) => [a[0] + b[0] * k, a[1] + b[1] * k, a[2] + b[2] * k];
  const sub3 = (a: number[], b: number[]) => [a[0] - b[0], a[1] - b[1], a[2] - b[2]];
  const dot3 = (a: number[], b: number[]) => a[0] * b[0] + a[1] * b[1] + a[2] * b[2];
  const cross3 = (a: number[], b: number[]) => [a[1] * b[2] - a[2] * b[1], a[2] * b[0] - a[0] * b[2], a[0] * b[1] - a[1] * b[0]];
  function segDist(px: number, py: number, a: number[] | null, b: number[] | null): number {
    if (!a || !b) return Infinity;
    const dx = b[0] - a[0], dy = b[1] - a[1], l2 = dx * dx + dy * dy || 1;
    const t = Math.max(0, Math.min(1, ((px - a[0]) * dx + (py - a[1]) * dy) / l2));
    return Math.hypot(px - (a[0] + t * dx), py - (a[1] + t * dy));
  }
  const pointDist = (px: number, py: number, p: number[] | null) => (p ? Math.hypot(px - p[0], py - p[1]) : Infinity);
  function planeHit(x: number, y: number, at: number[], n: number[]): number[] | null {
    const r = v3!.ray(x, y), dn = dot3(r.d, n);
    if (Math.abs(dn) < 1e-5) return null;
    const t = dot3(sub3(at, r.o), n) / dn;
    return t > 0 ? add3(r.o, r.d, t) : null;
  }

  /** Unity Euler angles (degrees) → rotation matrix R = Ry · Rx · Rz (numbers as in Unity). */
  const eulerMat = (e: number[]): Mat => mul(rotY(e[1]), mul(rotX(e[0]), rotZ(e[2])));
  /** Rotation matrix → Unity Euler angles (degrees, each in (-180, 180]); same maths as the Go extractor. */
  function matEuler(m: Mat): number[] {
    const el = (r: number, c: number) => m[c * 4 + r];
    const sx = Math.max(-1, Math.min(1, -el(1, 2)));
    const x = Math.asin(sx);
    let y: number, z: number;
    if (Math.abs(Math.cos(x)) > 1e-6) { y = Math.atan2(el(0, 2), el(2, 2)); z = Math.atan2(el(1, 0), el(1, 1)); }
    else { y = Math.atan2(-el(2, 0), el(0, 0)); z = 0; }
    const deg = (a: number) => { const d = Math.round(((a * 180) / Math.PI) * 10) / 10; return Object.is(d, -0) ? 0 : d; };
    return [deg(x), deg(y), deg(z)];
  }
  /** Rotation of `deg` around a Unity world axis (0 x, 1 y, 2 z) applied on top of the spot's rotation. */
  function rotateSpot(sp: any, k: number, deg: number, fine: boolean) {
    const step = fine ? 1 : 5, d = Math.round(deg / step) * step;
    if (!d) return;
    const R = k === 0 ? rotX(d) : k === 1 ? rotY(d) : rotZ(d);
    sp.rot = matEuler(mul(R, eulerMat(sp.rot)));
  }

  function makeHandles(s: any): Handle[] {
    if (!v3) return [];
    const L = v3.dist * 0.13, out: Handle[] = [];
    const P = (p: number[]) => v3!.project(p);
    const axis = (id: string, from: number[], dir: number[], color: Col, apply: Handle['apply'], len = L) => {
      const tip = add3(from, dir, len), side = Math.abs(dir[1]) > 0.9 ? [1, 0, 0] : [0, 1, 0];
      const draw = [...from, ...tip, ...tip, ...add3(add3(tip, dir, -len * 0.15), side, len * 0.06), ...tip, ...add3(add3(tip, dir, -len * 0.15), side, -len * 0.06)];
      out.push({ id, kind: 'axis', color, draw, origin: from, dir, apply, hit: (x, y) => segDist(x, y, P(from), P(tip)) });
    };
    const square = (id: string, at: number[], r: number, color: Col, apply: Handle['apply']) => {
      const q = [[-r, -r], [r, -r], [r, r], [-r, r]].map(([a, b]) => [at[0] + a, at[1], at[2] + b]), draw: number[] = [];
      for (let i = 0; i < 4; i++) draw.push(...q[i], ...q[(i + 1) % 4]);
      out.push({ id, kind: 'plane', color, draw, origin: at, n: [0, 1, 0], apply, hit: (x, y) => pointDist(x, y, P(at)) - 4 });
    };
    // Moves in GL deltas (d) → Unity (z mirrored).
    if (part === 'customer' && s.customer) {
      const at = [s.customer[0], 0.01, -s.customer[1]];
      const setC = (sp: any, dx: number, dz: number, fine: boolean) => { sp.customer = [snap(sp.customer[0] + dx, fine), snap(sp.customer[1] + dz, fine)]; };
      axis('cx', at, [1, 0, 0], RED, (sp, a, fine) => setC(sp, a, 0, fine), L * 0.7);
      axis('cz', at, [0, 0, 1], BLUE, (sp, a, fine) => setC(sp, 0, -a, fine), L * 0.7);
      square('cxz', at, L * 0.12, YELLOW, (sp, d, fine) => setC(sp, d[0], -d[2], fine));
      return out;
    }
    if (part === 'tag' && s.priceTag) {
      const at = toGL(s.priceTag);
      const setT = (sp: any, d: number[], fine: boolean) => { sp.priceTag = [snap(sp.priceTag[0] + d[0], fine), snap(sp.priceTag[1] + d[1], fine), snap(sp.priceTag[2] - d[2], fine)]; };
      for (const [id, dir, col] of AXES) axis('t' + id, at, dir, col, (sp, a, fine) => setT(sp, dir.map((v) => v * a), fine), L * 0.7);
      square('txz', at, L * 0.1, YELLOW, (sp, d, fine) => setT(sp, d, fine));
      return out;
    }
    const c = toGL(s.pos);
    if (tool === 'move') {
      AXES.forEach(([id, dir, col], k) => axis(id, c, dir, col, (sp, a, fine) => {
        const sign = k === 2 ? -1 : 1, target = snap(sp.pos[k] + sign * a, fine);
        shiftSpot(sp, k, target - sp.pos[k]);
      }));
      square('xz', c, L * 0.14, YELLOW, (sp, d, fine) => {
        shiftSpot(sp, 0, snap(sp.pos[0] + d[0], fine) - sp.pos[0]);
        shiftSpot(sp, 2, snap(sp.pos[2] - d[2], fine) - sp.pos[2]);
      });
    } else if (tool === 'rotate') {
      // Three rings around the world axes (GL). A turn of Δ around GL axis n is a turn of −Δ around Unity axis (nx, ny, −nz).
      AXES.forEach(([id, n, col], k) => {
        const u = k === 1 ? [1, 0, 0] : [0, 1, 0], w = cross3(n, u), pts: number[] = [], seg = 48, R = L * (1 - k * 0.08);
        for (let i = 0; i < seg; i++) {
          const a0 = (i / seg) * Math.PI * 2, a1 = ((i + 1) / seg) * Math.PI * 2;
          pts.push(...add3(add3(c, u, Math.cos(a0) * R), w, Math.sin(a0) * R), ...add3(add3(c, u, Math.cos(a1) * R), w, Math.sin(a1) * R));
        }
        out.push({ id: 'r' + id, kind: 'ring', color: col, draw: pts, origin: c, n,
          apply: (sp, deg, fine) => rotateSpot(sp, k, k === 2 ? deg : -deg, fine),
          hit: (x, y) => {
            let best = Infinity;
            for (let i = 0; i + 5 < pts.length; i += 6) best = Math.min(best, segDist(x, y, P(pts.slice(i, i + 3)), P(pts.slice(i + 3, i + 6))));
            return best;
          } });
      });
    } else if (tool === 'size' && s.kind === 'items' && s.size) {
      const M = spotMat(s), o = toGL(s.pos);
      const dirOf = (local: number[]) => { const d = sub3(toGL(point(M, local)), o), l = Math.hypot(d[0], d[1], d[2]) || 1; return d.map((v) => v / l); };
      const face = (local: number[]) => toGL(point(M, local));
      const resize = (k: number) => (sp: any, amt: number, fine: boolean) => { sp.size[k] = Math.max(0.02, snap(sp.size[k] + 2 * amt, fine)); if (gridFollows) fitGrid(sp); };
      const [w, d, h] = s.size;
      axis('w', face([w / 2, -h / 2, 0]), dirOf([1, 0, 0]), RED, resize(0), L * 0.6);
      axis('d', face([0, -h / 2, d / 2]), dirOf([0, 0, 1]), BLUE, resize(1), L * 0.6);
      axis('h', face([0, h / 2, 0]), dirOf([0, 1, 0]), GREEN, (sp, amt, fine) => { sp.size[2] = Math.max(0, snap(sp.size[2] + amt, fine)); if (gridFollows) fitGrid(sp); }, L * 0.6);
    }
    return out;
  }

  function makeAreaHandles(): Handle[] {
    const a = areaOf();
    if (!v3 || !a) return [];
    const L = v3.dist * 0.13, out: Handle[] = [], P = (p: number[]) => v3!.project(p);
    const c = [a.pos[0], 0.02, -a.pos[1]];
    const axis = (id: string, from: number[], dir: number[], color: Col, apply: Handle['apply'], len = L) => {
      const tip = add3(from, dir, len), draw = [...from, ...tip, ...tip, ...add3(add3(tip, dir, -len * 0.15), [0, 1, 0], len * 0.06), ...tip, ...add3(add3(tip, dir, -len * 0.15), [0, 1, 0], -len * 0.06)];
      out.push({ id, kind: 'axis', color, draw, origin: from, dir, apply, hit: (x, y) => segDist(x, y, P(from), P(tip)) });
    };
    if (tool === 'size') {
      // Width / depth: handles on the right and front edges; like the spot sizers, both sides move (the centre stays put).
      axis('aw', [c[0] + a.size[0] / 2, 0.02, c[2]], [1, 0, 0], RED, (ar, amt, fine) => { ar.size[0] = Math.max(0.05, snap(ar.size[0] + 2 * amt, fine)); }, L * 0.6);
      axis('ad', [c[0], 0.02, c[2] + a.size[1] / 2], [0, 0, 1], BLUE, (ar, amt, fine) => { ar.size[1] = Math.max(0.05, snap(ar.size[1] + 2 * amt, fine)); }, L * 0.6);
      return out;
    }
    axis('ax', c, [1, 0, 0], RED, (ar, amt, fine) => { ar.pos[0] = snap(ar.pos[0] + amt, fine); });
    axis('az', c, [0, 0, 1], BLUE, (ar, amt, fine) => { ar.pos[1] = snap(ar.pos[1] - amt, fine); });
    const r = L * 0.14, q = [[-r, -r], [r, -r], [r, r], [-r, r]].map(([u, v]) => [c[0] + u, c[1], c[2] + v]), sq: number[] = [];
    for (let i = 0; i < 4; i++) sq.push(...q[i], ...q[(i + 1) % 4]);
    out.push({ id: 'axz', kind: 'plane', color: YELLOW, draw: sq, origin: c, n: [0, 1, 0], hit: (x, y) => pointDist(x, y, P(c)) - 4,
      apply: (ar, d, fine) => { ar.pos = [snap(ar.pos[0] + d[0], fine), snap(ar.pos[1] - d[2], fine)]; } });
    return out;
  }

  function pickHandle(x: number, y: number): Handle | null {
    let best: Handle | null = null, bd = 9;
    for (const h of handles) { const d = h.hit(x, y); if (d < bd) { bd = d; best = h; } }
    return best;
  }

  type Pick = { i: number; part: 'spot' | 'customer' | 'tag' | 'area' | 'point' };

  /** Point-in-polygon (screen space). */
  function inPoly(x: number, y: number, pts: (number[] | null)[]): boolean {
    if (pts.some((q) => !q)) return false;
    let inside = false;
    for (let i = 0, j = pts.length - 1; i < pts.length; j = i++) {
      const a = pts[i]!, b = pts[j]!;
      if ((a[1] > y) !== (b[1] > y) && x < ((b[0] - a[0]) * (y - a[1])) / (b[1] - a[1] || 1e-9) + a[0]) inside = !inside;
    }
    return inside;
  }

  /** Screen outlines of a spot: its box's bottom and top faces (item spots) or the card (card spots). */
  function spotOutlines(sp: any): (number[] | null)[][] {
    const M = spotMat(sp), P = (l: number[]) => v3!.project(toGL(point(M, l)));
    if (sp.kind === 'items' && sp.size) {
      const [w, d, h] = sp.size, hw = w / 2, hd = d / 2, hh = h / 2;
      const face = (yy: number) => [[-hw, yy, -hd], [hw, yy, -hd], [hw, yy, hd], [-hw, yy, hd]].map(P);
      // A flat spot gets a little thickness so it can be clicked from the side too.
      return h > 0 ? [face(-hh), face(hh)] : [face(0), [[-hw, 0, -hd], [hw, 0, -hd], [hw, 0.03, -hd], [-hw, 0.03, -hd]].map(P)];
    }
    const [cw, ch] = cardSize;
    return [[[-cw / 2, -ch / 2, 0], [cw / 2, -ch / 2, 0], [cw / 2, ch / 2, 0], [-cw / 2, ch / 2, 0]].map(P)];
  }

  /**
   * Everything under the pointer, in picking order: customer points / price tags right under it (their small + marks), spots whose
   * outline contains it (nearest centre first), the placement area's centre, then the area's floor box.
   */
  function picksAt(x: number, y: number): Pick[] {
    if (!v3) return [];
    const marks: { p: Pick; d: number }[] = [], spots: { p: Pick; d: number }[] = [], out: Pick[] = [];
    const list: any[] = f?.spots ?? (t?.spots ?? []).filter((q: any) => q.kind === spotKind);
    list.forEach((sp, i) => {
      if (sp.customer) {
        const g = [sp.customer[0], 0.01, -sp.customer[1]];
        const ring = Array.from({ length: 16 }, (_, k) => v3!.project([g[0] + Math.cos((k / 16) * Math.PI * 2) * CUST_R, g[1], g[2] + Math.sin((k / 16) * Math.PI * 2) * CUST_R]));
        const dc = pointDist(x, y, v3!.project(g)), dp = segDist(x, y, v3!.project(g), v3!.project([g[0], CUST_POST, g[2]]));
        const d = Math.min(dc, dp);
        if (d < 12 || inPoly(x, y, ring)) marks.push({ p: { i, part: 'customer' }, d });
      }
      if (sp.priceTag) { const d = pointDist(x, y, v3!.project(toGL(sp.priceTag))); if (d < 7) marks.push({ p: { i, part: 'tag' }, d }); }
      const dc = pointDist(x, y, v3!.project(toGL(sp.pos)));
      if (dc < 10 || spotOutlines(sp).some((poly) => inPoly(x, y, poly))) spots.push({ p: { i, part: 'spot' }, d: dc });
    });
    pointsOf().forEach((q: any, i: number) => {
      const g = toGL(q.pos);
      const ring = Array.from({ length: 16 }, (_, k) => v3!.project([g[0] + Math.cos((k / 16) * Math.PI * 2) * 0.12, g[1] + 0.01, g[2] + Math.sin((k / 16) * Math.PI * 2) * 0.12]));
      const d = pointDist(x, y, v3!.project(g));
      if (d < 12 || inPoly(x, y, ring)) marks.push({ p: { i, part: 'point' }, d });
    });
    marks.sort((a, b) => a.d - b.d);
    spots.sort((a, b) => a.d - b.d);
    out.push(...marks.map((m) => m.p), ...spots.map((m) => m.p));
    const a = areaOf();
    if (a) {
      const dc = pointDist(x, y, v3.project([a.pos[0], 0.01, -a.pos[1]]));
      const [hw, hd] = [a.size[0] / 2, a.size[1] / 2], cx = a.pos[0], cz = -a.pos[1];
      const floor = [[cx - hw, 0, cz - hd], [cx + hw, 0, cz - hd], [cx + hw, 0, cz + hd], [cx - hw, 0, cz + hd]].map((q) => v3!.project(q));
      if (dc < 12) out.push({ i: sel, part: 'area' });
      else if (!out.length && inPoly(x, y, floor)) out.push({ i: sel, part: 'area' });
    }
    return out;
  }

  /** What a click selects: the first thing under the pointer, or — clicking again on something already selected — the next one. */
  function pickPart(x: number, y: number, cycle = false): Pick | null {
    const all = picksAt(x, y);
    if (!all.length) return null;
    if (cycle) {
      const cur = all.findIndex((q) => q.part === part && (q.part === 'area' || q.i === (q.part === 'point' ? selPt : sel)));
      if (cur >= 0) return all[(cur + 1) % all.length];
    }
    return all[0];
  }


  /** Other spots whose customer point is at the same place as spot i's (vanilla shelves share one per column). */
  /**
   * Default customer point of a spot: where the base piece's first spot of that kind has its point relative to the spot, turned with
   * the spot's facing (vanilla shop shelf: about 54 cm in front). Every spot needs one — customers stand there to take from it.
   */
  function defaultCustomer(sp: any): number[] {
    const b = (t?.spots ?? []).find((q: any) => q.kind === sp.kind && q.customer);
    const off = b ? [b.customer[0] - b.pos[0], b.customer[1] - b.pos[2]] : [0, 0.54];
    const a = (((sp.rot?.[1] ?? 0) - (b?.rot?.[1] ?? 0)) * Math.PI) / 180;
    return [r3(sp.pos[0] + off[0] * Math.cos(a) + off[1] * Math.sin(a)), r3(sp.pos[2] - off[0] * Math.sin(a) + off[1] * Math.cos(a))];
  }
  function resetCustomer() {
    const sp = f.spots?.[sel];
    if (!sp) return;
    sp.customer = defaultCustomer(sp);
    changed(); redraw++;
  }
  /** Spots next to the selected one (same facing, within 15 cm sideways) use its customer point too — like a vanilla shelf column. */
  /** The distinct customer points of the piece: position + the spots using it. */
  function customerPoints(): { c: number[]; spots: number[] }[] {
    const out: { c: number[]; spots: number[] }[] = [];
    (f?.spots ?? []).forEach((sp: any, i: number) => {
      if (!sp.customer) return;
      const g = out.find((o) => Math.abs(o.c[0] - sp.customer[0]) < 0.005 && Math.abs(o.c[1] - sp.customer[1]) < 0.005);
      if (g) g.spots.push(i); else out.push({ c: [...sp.customer], spots: [i] });
    });
    return out;
  }

  /**
   * Removes the selected customer point: the spots using it move over to the nearest other customer point (every spot keeps one).
   * The last point of the piece can't go.
   */
  function deleteCustomer() {
    const sp = f.spots?.[sel];
    if (!sp?.customer) return;
    const pts = customerPoints();
    const mine = pts.find((o) => o.spots.includes(sel));
    const others = pts.filter((o) => o !== mine);
    if (!mine || !others.length) { notify('This is the only customer point — every spot needs one to be shopped from.', 'warn'); return; }
    const near = others.reduce((a, b) => (Math.hypot(b.c[0] - mine.c[0], b.c[1] - mine.c[1]) < Math.hypot(a.c[0] - mine.c[0], a.c[1] - mine.c[1]) ? b : a));
    for (const i of mine.spots) f.spots[i].customer = [...near.c];
    notify(`Customer point removed — ${mine.spots.length} spot${mine.spots.length === 1 ? '' : 's'} now use${mine.spots.length === 1 ? 's' : ''} the nearest other point.`, 'ok');
    changed(); redraw++;
  }

  /** Gives the selected spot a customer point of its own (split off a shared one): in front of the spot, or beside the shared one. */
  function ownCustomer() {
    const sp = f.spots?.[sel];
    if (!sp) return;
    let c = defaultCustomer(sp);
    const taken = (q: number[]) => customerPoints().some((o) => Math.abs(o.c[0] - q[0]) < 0.005 && Math.abs(o.c[1] - q[1]) < 0.005);
    for (let k = 1; taken(c) && k < 20; k++) c = [r3(c[0] + 0.15 * k), c[1]];
    sp.customer = c;
    part = 'customer'; tool = 'move';
    changed(); redraw++;
  }

  function shareCustomerNearby() {
    const sp = f.spots?.[sel];
    if (!sp?.customer) return;
    let n = 0;
    (f.spots as any[]).forEach((o, j) => {
      if (j === sel || o.kind !== sp.kind) return;
      const dyaw = Math.abs(((((o.rot[1] - sp.rot[1]) % 360) + 540) % 360) - 180);
      if (dyaw < 10 && Math.abs(o.pos[0] - sp.pos[0]) < 0.15 && Math.abs(o.pos[2] - sp.pos[2]) < 0.3) { o.customer = [...sp.customer]; n++; }
    });
    notify(n ? `${n} nearby spot${n === 1 ? '' : 's'} now share this customer point.` : 'No spots right above/below this one to share with.', n ? 'ok' : 'warn');
    changed(); redraw++;
  }

  function sharedCustomer(i: number): number[] {
    const c = f?.spots?.[i]?.customer;
    if (!c) return [];
    return (f.spots as any[]).map((sp, j) => (j !== i && sp.customer && Math.abs(sp.customer[0] - c[0]) < 0.005 && Math.abs(sp.customer[1] - c[1]) < 0.005 ? j : -1)).filter((j) => j >= 0);
  }

  /** What a drag edits: the selected spot, or the placement area (created from the base piece's on first edit). */
  const dragTarget = () => (part === 'area' ? f.area ?? areaOf() : part === 'point' ? pointsOf()[selPt] : f.spots?.[sel]);
  function setDragTarget(v: any) {
    if (part === 'area') f.area = v;
    else if (part === 'point') { pointsCustom(); f.points[selPt] = v; }
    else f.spots[sel] = v;
  }

  function startDrag(h: Handle, x: number, y: number) {
    const target = dragTarget();
    if (!target) return;
    const hit0 = h.kind !== 'axis' ? planeHit(x, y, h.origin, h.n ?? [0, 1, 0]) : null;
    dragging = { h, start: JSON.parse(JSON.stringify(target)), mx: x, my: y, hit0, linked: part === 'customer' ? sharedCustomer(sel) : [] };
  }

  function onDown(e: PointerEvent, x: number, y: number): boolean {
    if (!f) return false;
    const h = pickHandle(x, y);
    if (h) { startDrag(h, x, y); return true; }
    const pk = pickPart(x, y, true);
    if (!pk) return false;
    if (pk.part === 'point') {
      pointsCustom(); // the vanilla points become editable (same points, now yours)
      selPt = pk.i;
      part = 'point';
      if (tool === 'size') tool = 'move';
      handles = makePointHandles(f.points[selPt]);
      const at = toGL(f.points[selPt].pos);
      startDrag({ id: 'drag', kind: 'plane', color: YELLOW, draw: [], origin: at, n: [0, 1, 0], hit: () => Infinity,
        apply: (pt, d, fine) => { pt.pos = [snap(pt.pos[0] + d[0], fine), pt.pos[1], snap(pt.pos[2] - d[2], fine)]; } }, x, y);
      redraw++;
      return true;
    }
    if (pk.part !== 'area' && !f.spots) spotsFromBase(); // clicking a vanilla spot makes the spots editable (same spots, now yours)
    sel = pk.i;
    part = pk.part;
    if (part === 'area') {
      handles = makeAreaHandles();
      const a = areaOf()!, at = [a.pos[0], 0.02, -a.pos[1]];
      startDrag({ id: 'drag', kind: 'plane', color: YELLOW, draw: [], origin: at, n: [0, 1, 0], hit: () => Infinity,
        apply: (ar, d, fine) => { ar.pos = [snap(ar.pos[0] + d[0], fine), snap(ar.pos[1] - d[2], fine)]; } }, x, y);
      redraw++;
      return true;
    }
    handles = makeHandles(f.spots[sel]);
    // Click-and-drag: slide the picked part on its horizontal plane.
    const s = f.spots[sel];
    const at = part === 'customer' ? [s.customer[0], 0.01, -s.customer[1]] : part === 'tag' ? toGL(s.priceTag) : toGL(s.pos);
    startDrag({ id: 'drag', kind: 'plane', color: YELLOW, draw: [], origin: at, n: [0, 1, 0], hit: () => Infinity,
      apply: (sp, d, fine) => {
        if (part === 'customer') sp.customer = [snap(sp.customer[0] + d[0], fine), snap(sp.customer[1] - d[2], fine)];
        else if (part === 'tag') sp.priceTag = [snap(sp.priceTag[0] + d[0], fine), sp.priceTag[1], snap(sp.priceTag[2] - d[2], fine)];
        else { shiftSpot(sp, 0, snap(sp.pos[0] + d[0], fine) - sp.pos[0]); shiftSpot(sp, 2, snap(sp.pos[2] - d[2], fine) - sp.pos[2]); }
      } }, x, y);
    redraw++;
    return true;
  }

  function onDrag(e: PointerEvent, x: number, y: number) {
    if (!dragging || !v3 || !dragTarget()) return;
    const { h, start } = dragging, fine = e.shiftKey;
    const s = JSON.parse(JSON.stringify(start));
    if (h.kind === 'axis') {
      const a = v3.project(h.origin), b = v3.project(add3(h.origin, h.dir!, 1));
      if (!a || !b) return;
      const sx = b[0] - a[0], sy = b[1] - a[1], l2 = sx * sx + sy * sy;
      if (l2 < 1e-6) return;
      h.apply(s, ((x - dragging.mx) * sx + (y - dragging.my) * sy) / l2, fine);
    } else {
      const hit = planeHit(x, y, h.origin, h.n ?? [0, 1, 0]);
      if (!hit || !dragging.hit0) return;
      if (h.kind === 'plane') h.apply(s, sub3(hit, dragging.hit0), fine);
      else {
        const v0 = sub3(dragging.hit0, h.origin), v1 = sub3(hit, h.origin);
        h.apply(s, (Math.atan2(dot3(h.n!, cross3(v0, v1)), dot3(v0, v1)) * 180) / Math.PI, fine);
      }
    }
    if (!dragging.pushed) { pushUndo(base); dragging.pushed = true; } // the whole drag is one undo step
    setDragTarget(s);
    // A shared customer point moves for every spot using it (Alt = only the selected spot's).
    if (part === 'customer' && !e.altKey) for (const j of dragging.linked ?? []) if (f.spots[j]) f.spots[j].customer = [...s.customer];
    changed();
    redraw++;
  }

  function onHover(x: number, y: number) {
    if (!canvas) return;
    const h = pickHandle(x, y), id = h?.id ?? '';
    canvas.style.cursor = h ? 'grab' : pickPart(x, y) ? 'pointer' : '';
    if (id !== hot) { hot = id; redraw++; }
  }

  // ---------------------------------------------------------------- save

  async function save() {
    if (!f) return;
    saving = true;
    try {
      let icon = '';
      if (layout.model) {
        if (!t?.bounds) throw new Error(`No size known for ${baseOf(f)} — the game templates aren't available yet (Settings → Game).`);
        const b = await App.BakeFurnitureModel(f.id, layout.model, layout.texture,
          { rotX: layout.rotX, rotY: layout.rotY, rotZ: layout.rotZ, height: layout.height || baseHeight, anchor: anchorUnity() } as any);
        f.mesh = b.mesh; f.texture = b.texture;
        if (!f.icon || layout.autoIcon) { icon = await renderIcon(t?.iconSize ?? [512, 512]); layout.autoIcon = true; }
      } else {
        // A texture that belonged to a removed model goes with it (older libraries: its fixed file name).
        const hadModel = !!f.mesh;
        f.mesh = '';
        if (hadModel || f.texture === `images/${f.id}_texture.png`) f.texture = '';
      }
      // Every spot needs a customer point (customers can't take from a spot without one): refuse to save instead of guessing.
      const missing = (f.spots ?? []).map((q: any, i: number) => (q.customer?.length === 2 ? 0 : i + 1)).filter((n: number) => n);
      if (missing.length) {
        sel = missing[0] - 1; part = 'spot'; redraw++;
        throw new Error(`Spot${missing.length > 1 ? 's' : ''} ${missing.join(', ')} ${missing.length > 1 ? 'have' : 'has'} no customer point (shown in red). Select the spot and press C to give it one, or share a nearby spot's point.`);
      }
      const clean = JSON.parse(JSON.stringify(f));
      for (const k of ['description', 'icon', 'texture', 'tint', 'mesh', 'base']) if (!clean[k]) delete clean[k];
      view = await App.SaveFurniture(clean, JSON.stringify(layout), icon);
      bust = Date.now();
      dirty = false;
      const id = f.id;
      f = JSON.parse(JSON.stringify(list.find((x) => x.id === id)));
      base = snapshot();
      notify(view.installed ? 'Saved and installed — restart the game to see it.' : 'Saved. Set the game folder in Settings to install it.', 'ok');
    } catch (e) { notify(errText(e), 'error'); }
    saving = false;
  }

  /** Selects a part of the selected spot (B body, C customer point, T price tag); vanilla spots become editable. */
  function selectPart(pt: 'spot' | 'customer' | 'tag') {
    if (!f.spots) spotsFromBase();
    const sp = f.spots?.[sel];
    if (!sp) return;
    if (pt === 'customer' && !sp.customer) sp.customer = defaultCustomer(sp);
    if (pt === 'tag' && !sp.priceTag) { notify('This spot has no price tag of its own position yet — set one in the Price tag fields.', 'warn'); return; }
    part = pt;
    if (pt !== 'spot') tool = 'move';
    redraw++;
  }

  /** Keys 1 / 2 / 3 = Move / Rotate / Size (not while typing). Rotate and Size act on a spot's body, so they select it. */
  function onKey(e: KeyboardEvent) {
    if (!f) return;
    const el = e.target as HTMLElement | null;
    if (el && (el.isContentEditable || ['INPUT', 'SELECT', 'TEXTAREA'].includes(el.tagName))) return;
    if ((e.ctrlKey || e.metaKey) && !e.altKey) {
      const k = e.key.toLowerCase();
      if (k === 'd' && spotKind) { e.preventDefault(); addSpot(); return; }
      if (k === 'z' && !e.shiftKey) { e.preventDefault(); undo(); }
      else if (k === 'y' || (k === 'z' && e.shiftKey)) { e.preventDefault(); redo(); }
      return;
    }
    if (e.altKey) return;
    if (spotKind && part !== 'point' && (e.key === 'n' || e.key === 'N' || e.key === 'Insert')) { e.preventDefault(); addSpot(); return; }
    const partKey = ({ b: 'spot', c: 'customer', t: 'tag' } as const)[e.key.toLowerCase() as 'b' | 'c' | 't'];
    if (spotKind && partKey && !e.shiftKey) { e.preventDefault(); selectPart(partKey); return; }
    if (spotKind === 'items' && (e.key === 'f' || e.key === 'F')) { e.preventDefault(); if (e.shiftKey) cycleFill(1); else toggleFill(); return; }
    if ((spotKind || pointsOf().length) && (e.key === 'p' || e.key === 'P')) { e.preventDefault(); showPeople = !showPeople; return; }
    if (spotKind && (e.key === 'l' || e.key === 'L')) { e.preventDefault(); showTags = !showTags; return; }
    if (e.key === 'h' || e.key === 'H' || e.key === '?') { e.preventDefault(); showHelp = !showHelp; return; }
    if (spotKind && (e.key === 'Delete' || e.key === 'Backspace') && part === 'spot' && (f.spots?.length ?? 0) > 0) { e.preventDefault(); deleteSpot(); return; }
    if (spotKind && (e.key === 'Delete' || e.key === 'Backspace') && part === 'customer') { e.preventDefault(); deleteCustomer(); return; }
    if (part === 'point' && (e.key === 'Delete' || e.key === 'Backspace')) { e.preventDefault(); deletePoint(); return; }
    if (part === 'point' && (e.key === 'n' || e.key === 'N' || e.key === 'Insert')) { e.preventDefault(); addPoint(); return; }
    const next = ({ '1': 'move', '2': 'rotate', '3': 'size' } as const)[e.key as '1' | '2' | '3'];
    if (!next) return;
    if (next === 'size' && spotKind !== 'items') return;
    if (part === 'area') {
      if (next === 'rotate') return;
    } else if (part === 'point') {
      if (next === 'size') return;
    } else if (next !== 'move') {
      if (!f.spots?.length) return;
      part = 'spot';
    }
    tool = next;
    e.preventDefault();
  }

  onMount(() => {
    window.addEventListener('keydown', onKey);
    const offs = [
      EventsOn('templates:progress', (m: string) => (tplStatus = { ...tplStatus, busy: true, message: m })),
      EventsOn('templates:ready', () => loadTemplates().catch((e) => notify(errText(e), 'error'))),
    ];
    (async () => {
      try {
        types = await App.FurnitureTypes();
        await loadTemplates();
        await load(false);
      } catch (e) { notify(errText(e), 'error'); }
    })();
    return () => { offs.forEach((off) => off()); window.removeEventListener('keydown', onKey); };
  });

  // The canvas exists only while a piece is selected: attach the 3D view when it appears.
  $effect(() => {
    if (!canvas) return;
    const c = canvas;
    try { v3 = new FigurineView(c); } catch (e) { notify(errText(e), 'error'); return; }
    v3.onpointerdown = onDown;
    v3.onpointerdrag = onDrag;
    v3.onpointerup = () => { dragging = null; base = snapshot(); lastEdit = 0; };
    v3.onhover = onHover;
    v3.onchange = () => rebuild(); // handles keep their on-screen size while zooming
    const ro = new ResizeObserver(() => v3?.draw());
    ro.observe(c);
    loadedTex = '';
    untrack(() => rebuild(true));
    return () => { ro.disconnect(); v3?.dispose(); v3 = null; };
  });

  let spot = $derived(f?.spots?.[sel]);
</script>

<p class="muted small intro">
  Make new furniture for the furniture shop. Each piece copies a vanilla piece of the <b>type</b> you pick — that decides what it does
  (shelf, card shelf, donation bin…). Then give it your own colour or 3D model, shop price, and — for shelves — your own spots.
  <br />Only want your own furniture in the shop? Turn off <b>Mod settings → Content → ShowVanillaFurniture</b> (cash counter, workbench,
  trash bin and empty box storage stay until you make your own).
</p>
{#if !pieces.length}
  {#if tplStatus?.busy}<p class="muted small">Reading the game's furniture… {tplStatus.message ?? ''}</p>
  {:else}<p class="warn small">The game's furniture isn't available{tplStatus?.error ? `: ${tplStatus.error}` : ' — set the game folder in Settings'}.</p>{/if}
{/if}

<div class="fwrap">
  <div class="list" role="listbox" tabindex="-1" onkeydown={listKey} title="Ctrl+click or Shift+click to select several; Delete removes them">
    <label class="sortby small">Sort
      <select bind:value={sortBy} onchange={() => { try { localStorage.setItem('furniture.sort', sortBy); } catch {} }}>
        {#each Object.entries(SORT_LABELS) as [k, label]}<option value={k}>{label}</option>{/each}
      </select></label>
    {#each shown as x (x.id)}
      <button class:active={x.id === selectedId} class:picked={picked.includes(x.id) && x.id !== selectedId} onclick={(e) => clickItem(e, x.id)}>
        <span class="ico">{#if x.icon}<img src={accUrl(x.icon)} alt="" />{:else if tplOf(x)?.icon}<img src={furnUrl(tplOf(x).icon)} alt="" />{/if}</span>
        <span class="grow">{x.name}<br /><span class="muted small">{typeInfo(x.type)?.one ?? x.type}{originOf(x) ? ` · from ${originOf(x).mod}${originOf(x).author ? ` (${originOf(x).author})` : ''}` : ''}</span></span>
      </button>
    {/each}
    <div class="row newrow">
      <select bind:value={newType}>{#each types as tp}<option value={tp.type}>{tp.one}</option>{/each}</select>
      <button onclick={add}>+ New</button>
    </div>
    <button onclick={() => (fromCatalog = true)} title="Furniture from your other setups and earlier imports — no re-import">Add from catalog…</button>
  </div>

  {#if f}
    <div class="form">
      <section>
        <div class="row">
          <label class="field grow" title="Name shown in the furniture shop and when you look at the piece.">Name<input bind:value={f.name} oninput={changed} /></label>
          <label class="field" title="What the piece does in the game: a shelf holds items, a card shelf holds cards, a donation bin takes cards… It is copied from a vanilla piece of this type.">Type
            <select value={f.type} onchange={(e) => setType(e.currentTarget.value)}>{#each types as tp}<option value={tp.type}>{tp.one}</option>{/each}</select>
          </label>
          <label class="field" title="Vanilla piece this one is built on: its behaviour, size, default spots, price and level.">Start from
            <select value={baseOf(f)} onchange={(e) => setBase(e.currentTarget.value)}>
              {#each bases as b}<option value={b.base}>{b.base}</option>{/each}
              {#if !bases.some((b) => b.base === baseOf(f))}<option value={baseOf(f)}>{baseOf(f)}</option>{/if}
            </select>
          </label>
          <label class="field" style="width:150px" title="Fixed id that saves use to recognise this piece. It never changes, so renaming is safe.">Id<input value={f.id} readonly /></label>
        </div>
        <label class="field" title="Text shown in the furniture shop's purchase window. Empty = the vanilla piece's description.">Description <span class="muted small">(empty = the vanilla piece's)</span>
          <input value={f.description ?? ''} oninput={(e) => { f.description = e.currentTarget.value; changed(); }} />
        </label>
        <div class="row"><div class="grow"></div><button class="small" onclick={saveToCatalog} title="Make this setup's saved version the one other setups get when they add it from the catalog (setups that already have it keep theirs).">Update catalog</button><button class="danger small" onclick={remove}>Delete</button></div>
      </section>

      <section>
        <div class="row">
          <h3 class="grow">Look & spots</h3>
          <span class="thumbs"><span class="small muted">Shop icon:</span>
            {#if f.icon}<img src={accUrl(f.icon)} alt="icon" />{:else if t?.icon}<img src={furnUrl(t.icon)} alt="vanilla icon" />{/if}
          </span>
        </div>
        <div class="editor">
          <div class="stage">
            <div class="scene" use:maximizable>
              <canvas bind:this={canvas} class="view3d"></canvas>
              {#if busy}<p class="overlay muted small">Loading…</p>{/if}
              {#if part === 'area'}
                <div class="tools">
                  <span class="selpart">Placement area</span>
                  <button class:on={tool !== 'size'} onclick={() => (tool = 'move')} title="Move the area: drag the arrows or the yellow square (key 1).">Move <kbd>1</kbd></button>
                  <button class:on={tool === 'size'} onclick={() => (tool = 'size')} title="Resize the area: drag the red (width) or blue (depth) handle — both sides grow or shrink around the centre (key 3).">Size <kbd>3</kbd></button>
                </div>
              {:else if part === 'point' && pointsOf()[selPt]}
                <div class="tools">
                  <span class="selpart" title={roleInfo(pointsOf()[selPt].role)?.tip ?? ''}>{pointLabel(selPt)}</span>
                  <button class:on={tool !== 'rotate'} onclick={() => (tool = 'move')} title="Move the point: arrows or the yellow square (key 1).">Move <kbd>1</kbd></button>
                  <button class:on={tool === 'rotate'} onclick={() => (tool = 'rotate')} title="Turn the point — which way the person faces (key 2).">Rotate <kbd>2</kbd></button>
                  {#if roleInfo(pointsOf()[selPt].role)?.resizable}
                    <button onclick={addPoint} title="Add another point of this kind beside it (N). The game picks one of them.">+ Point <kbd>N</kbd></button>
                    <button onclick={deletePoint} title="Remove this point (Delete). At least one stays.">Delete <kbd>Del</kbd></button>
                  {/if}
                  <button class:on={showPeople} onclick={() => (showPeople = !showPeople)} title="Show stand-in people on the points (P).">People <kbd>P</kbd></button>
                </div>
              {:else if !spotKind && pointsOf().length}
                <div class="tools">
                  <span class="selpart">Click a coloured ring to edit a position</span>
                  <button class:on={showPeople} onclick={() => (showPeople = !showPeople)} title="Show stand-in people on the points (P).">People <kbd>P</kbd></button>
                </div>
              {:else if spotKind}
                <div class="tools">
                  {#if f.spots?.length}<span class="selpart">Spot {sel + 1} of {f.spots.length}: {part === 'spot' ? 'body' : part === 'customer' ? `customer point${sharedCustomer(sel).length ? ` (shared by ${sharedCustomer(sel).length + 1} spots — Alt-drag = only this one)` : ''}` : 'price tag'}</span>
                  {:else if !f.spots}<span class="selpart">Vanilla spots — click one to edit</span>{/if}
                  {#if f.spots?.length}
                    <span class="seg">
                      <button class:on={part === 'spot'} onclick={() => selectPart('spot')} title="Edit the selected spot itself (B).">Body <kbd>B</kbd></button>
                      <button class:on={part === 'customer'} onclick={() => selectPart('customer')} title="Edit where customers stand to take from the selected spot (C).">Customer <kbd>C</kbd></button>
                      <button class:on={part === 'tag'} onclick={() => selectPart('tag')} title="Edit the selected spot's price tag position (T).">Tag <kbd>T</kbd></button>
                    </span>
                  {/if}
                  {#if part === 'customer'}
                    <button onclick={resetCustomer} title="Put this spot's customer point back in front of the spot (where the vanilla piece has it). Every spot needs exactly one: customers stand there to take items.">Reset point</button>
                    <button onclick={shareCustomerNearby} title="Spots directly above/below this one (same facing) use this same customer point, like a vanilla shelf column. Alt-drag a point to give a spot its own again.">Share with nearby spots</button>
                    <button onclick={ownCustomer} disabled={!sharedCustomer(sel).length} title="Give the selected spot a customer point of its own, split off the shared one (you can then move it).">Own point</button>
                    <button onclick={deleteCustomer} disabled={customerPoints().length < 2} title="Remove this customer point: the spots using it move over to the nearest other customer point (Delete key). Every spot keeps one, so the last point can't be removed.">Remove point <kbd>Del</kbd></button>
                  {/if}
                  {#if spotKind === 'items'}
                    <button class:on={showTags} onclick={() => (showTags = !showTags)} title="Show every price tag as a tag-sized label where the game puts it, facing the way it faces in game (L). Off = small white crosses.">Tags <kbd>L</kbd></button>
                    <button class:on={showPeople} onclick={() => (showPeople = !showPeople)} title="Show a life-size stand-in customer (~1.65 m) on every customer point, facing the spots it serves (P). Only for judging reach and aisle room.">Customers <kbd>P</kbd></button>
                    <button class:on={!!fillKey} onclick={toggleFill} title="Fill preview on/off (F): every item spot filled with a shop item, placed like the game does. Shift+F = next item.">Fill <kbd>F</kbd></button>
                    {#if fillKey}
                      <select class="fillsel" bind:value={fillKey} onchange={() => (lastFill = fillKey)} title="Item shown in the fill preview (Shift+F = next)">
                        {#each fillGroups as g}<optgroup label={g}>{#each fillItems.filter((x) => x.group === g) as it}<option value={it.key}>{it.label}</option>{/each}</optgroup>{/each}
                      </select>
                    {/if}
                  {/if}
                  <button onclick={addSpot} title="Add a spot: a copy of the selected one, placed beside it (N or Ctrl+D). On vanilla spots this makes them editable first.">+ Spot <kbd>N</kbd></button>
                  <button onclick={deleteSpot} disabled={part !== 'spot' || (!!f.spots && !f.spots.length)} title="Delete the selected spot (Delete key).">Delete spot{#if part === 'spot'} <kbd>Del</kbd>{/if}</button>
                  <button class:on={tool === 'move' || part !== 'spot'} onclick={() => (tool = 'move')} title="Drag an arrow to move the selected part along that axis, or the yellow square to slide it across its level. You can also click and drag any spot, customer point or price tag directly.">Move <kbd>1</kbd></button>
                  <button class:on={tool === 'rotate' && part === 'spot'} disabled={part !== 'spot'} onclick={() => (tool = 'rotate')} title="Drag a ring to turn the selected spot around that axis (5° steps): green = turn left/right, red = tilt forward/back, blue = roll. Tilt a spot to make hanging or slanted spots.">Rotate <kbd>2</kbd></button>
                  {#if spotKind === 'items'}<button class:on={tool === 'size' && part === 'spot'} disabled={part !== 'spot'} onclick={() => (tool = 'size')} title="Drag the handles to change the selected spot's width (red), depth (blue) and height (green).">Size <kbd>3</kbd></button>{/if}
                </div>
              {/if}
              <button class="helpbtn" onclick={() => (showHelp = !showHelp)} title="Controls and keys (H)">?</button>
              {#if showHelp}
                <div class="helpcard" role="dialog">
                  <div class="row"><b class="grow">Controls</b><button class="tiny" onclick={() => (showHelp = false)}>✕</button></div>
                  <table>
                    <tbody>
                      <tr><th colspan="2">Select</th></tr>
                      <tr><td>Click inside a spot</td><td>select the spot</td></tr>
                      <tr><td>Click a <span class="c-cust">green ring</span></td><td>its customer point (where customers stand)</td></tr>
                      <tr><td>Click a <span class="c-tag">white tag</span></td><td>its price tag</td></tr>
                      <tr><td>Click the <span class="c-area">orange box</span>'s centre</td><td>placement area (space kept free around the piece)</td></tr>
                      <tr><td>Click again</td><td>next thing under the pointer</td></tr>
                      <tr><td><kbd>B</kbd> <kbd>C</kbd> <kbd>T</kbd></td><td>body / customer point / tag of the selected spot</td></tr>
                      <tr><th colspan="2">Edit</th></tr>
                      <tr><td><kbd>1</kbd> <kbd>2</kbd> <kbd>3</kbd></td><td>move / rotate / size</td></tr>
                      <tr><td>Drag a handle</td><td>arrows = along an axis, square = slide, rings = turn</td></tr>
                      <tr><td>Hold <kbd>Shift</kbd></td><td>no snapping (1 cm / 5° steps otherwise)</td></tr>
                      <tr><td>Hold <kbd>Alt</kbd> on a customer point</td><td>move only this spot's (not the shared one)</td></tr>
                      <tr><td><kbd>N</kbd> / <kbd>Del</kbd></td><td>add a spot / delete the selection</td></tr>
                      <tr><td><kbd>Ctrl</kbd>+<kbd>Z</kbd> / <kbd>Ctrl</kbd>+<kbd>Y</kbd></td><td>undo / redo</td></tr>
                      <tr><th colspan="2">Show</th></tr>
                      <tr><td><kbd>F</kbd> / <kbd>Shift</kbd>+<kbd>F</kbd></td><td>fill preview on/off / next item</td></tr>
                      <tr><td><kbd>L</kbd></td><td>price tags as labels</td></tr>
                      <tr><td><kbd>P</kbd></td><td>stand-in customers</td></tr>
                      <tr><th colspan="2">View</th></tr>
                      <tr><td>Drag empty space</td><td>turn</td></tr>
                      <tr><td>Right / middle drag</td><td>pan</td></tr>
                      <tr><td>Wheel</td><td>zoom</td></tr>
                      <tr><td>⛶ / <kbd>Esc</kbd></td><td>full view / back</td></tr>
                    </tbody>
                  </table>
                </div>
              {/if}
              <p class="hint muted small">Click to select · drag the handles to edit · <b>?</b> for all controls</p>
            </div>
          </div>
          <div class="side">
            <div class="props">
              <h4>Colour</h4>
              <div class="row">
                <input type="color" title="Colour multiplied onto the piece's own materials (the vanilla model or your model). Price tags and spots keep their colours." value={f.tint || '#ffffff'} oninput={(e) => { f.tint = e.currentTarget.value; changed(); }} />
                <span class="muted small grow">{f.tint ? `Tint ${f.tint}` : 'No tint (vanilla colours)'}</span>
                {#if f.tint}<button class="tiny" onclick={() => { f.tint = ''; changed(); }}>Clear</button>{/if}
              </div>
            </div>
            <div class="props">
              <h4>Model</h4>
              {#if layout.model}
                <div class="small"><b>{layout.sourceName}</b> <span class="muted">· {layout.triangles.toLocaleString()} triangles</span></div>
                {#each layout.warnings ?? [] as w}<div class="warn small">⚠ {w}</div>{/each}
                <label class="field" title="Height of your model in the game, in centimetres. It stands on the floor, centred on the vanilla piece's footprint.">Height (cm)
                  <input type="number" min="1" step="1" value={Math.round((layout.height || baseHeight) * 100)}
                    onchange={(e) => { const v = +e.currentTarget.value; if (v > 0) { layout.height = v / 100; changed(); } }} />
                </label>
                <button class="small" onclick={() => { layout.height = baseHeight; changed(); }}>Match {baseOf(f)} ({Math.round(baseHeight * 100)} cm)</button>
                <div class="row wrap">
                  <button class="tiny" onclick={() => turn('rotX', 90)}>Tip ↓</button>
                  <button class="tiny" onclick={() => turn('rotX', -90)}>Tip ↑</button>
                  <button class="tiny" onclick={() => turn('rotZ', 90)}>Roll ↺</button>
                  <button class="tiny" onclick={() => turn('rotY', 90)}>Turn 90°</button>
                </div>
                <label class="check small"><input type="checkbox" bind:checked={showBase} /> Show {baseOf(f)} (ghost)</label>
                <div class="row"><button class="small" onclick={importModel} disabled={busy}>Replace…</button><button class="small" onclick={removeModel}>Use vanilla model</button></div>
                <p class="muted small">The front faces you in the default view (−z = the aisle side of vanilla shelves). Spots stay where they are — move them onto your model.</p>
              {:else}
                <p class="muted small">Vanilla model of {baseOf(f)}{f.tint ? ', tinted' : ''}.</p>
                <button class="small" onclick={importModel} disabled={busy}>Import own model… (.glb/.gltf/.obj)</button>
              {/if}
            </div>
            {#if spotKind === 'items'}
              <div class="props" title="Preview only: fills every item spot with a vanilla item, placed the way the game does it, so you can see how many fit and how it looks. Nothing is saved.">
                <h4>Fill preview</h4>
                <select bind:value={fillKey}>
                  <option value="">Empty</option>
                  {#each fillGroups as g}
                    <optgroup label={g}>{#each fillItems.filter((x) => x.group === g) as it}<option value={it.key}>{it.label}</option>{/each}</optgroup>
                  {/each}
                </select>
                {#if fillItem}
                  {@const list = (f.spots ?? (t?.spots ?? []).filter((x: any) => x.kind === 'items')) as any[]}
                  {@const counts = list.map((x) => fillCount(x, !!f.spots))}
                  <p class="muted small">{counts.reduce((a, b) => a + b, 0)} in total · {counts.length ? `${Math.min(...counts)}–${Math.max(...counts)}` : 0} per spot. Bigger items take several grid units.</p>
                {/if}
              </div>
            {/if}
            <div class="props" title="The box the game keeps free of other furniture and walls when you place or move this piece, and uses to snap pieces side by side. It usually includes room in front for customers.">
              <h4>Placement area {#if !f.area}<span class="muted small">(vanilla)</span>{/if}</h4>
              {#if areaOf()}
                {@const a = areaOf()!}
                <div class="grid2">
                  <label class="field" title="Centre of the area, left/right of the piece's centre (m).">Centre x<input type="number" step="0.01" value={a.pos[0]}
                    onchange={(e) => { f.area = { pos: [+e.currentTarget.value, a.pos[1]], size: [...a.size] }; part = 'area'; changed(); }} /></label>
                  <label class="field" title="Centre of the area, front/back (m).">Centre z<input type="number" step="0.01" value={a.pos[1]}
                    onchange={(e) => { f.area = { pos: [a.pos[0], +e.currentTarget.value], size: [...a.size] }; part = 'area'; changed(); }} /></label>
                  <label class="field" title="Width of the area (m). Other furniture can't be placed inside it.">Width<input type="number" step="0.01" min="0.05" value={a.size[0]}
                    onchange={(e) => { f.area = { pos: [...a.pos], size: [Math.max(0.05, +e.currentTarget.value), a.size[1]] }; part = 'area'; changed(); }} /></label>
                  <label class="field" title="Depth of the area (m), front to back — include room for customers in front of shelves.">Depth<input type="number" step="0.01" min="0.05" value={a.size[1]}
                    onchange={(e) => { f.area = { pos: [...a.pos], size: [a.size[0], Math.max(0.05, +e.currentTarget.value)] }; part = 'area'; changed(); }} /></label>
                </div>
                <div class="row wrap">
                  <button class="small" onclick={() => { part = 'area'; redraw++; }} title="Select the area in the 3D view to drag it.">Edit in view</button>
                  {#if layout.model}<button class="small" onclick={fitAreaToModel} title="Size the area to your model, keeping the vanilla piece's room around it.">Fit to model</button>{/if}
                  {#if f.area}<button class="small" onclick={() => { f.area = undefined; changed(); }} title="Use the vanilla piece's placement area again.">Vanilla</button>{/if}
                </div>
              {:else}
                <p class="muted small">Needs the game templates (Settings → Game).</p>
              {/if}
            </div>
            <div class="props">
              <h4>Shop icon</h4>
              <p class="muted small">{f.icon ? (layout.autoIcon ? 'Rendered from your model on save.' : 'Your own image.') : `The vanilla ${baseOf(f)} icon${f.tint ? ', tinted in game' : ''}.`}</p>
              <div class="row"><button class="small" onclick={pickIcon}>Use an image…</button>{#if f.icon}<button class="small" onclick={() => { f.icon = ''; layout.autoIcon = !!layout.model; changed(); }}>Default</button>{/if}</div>
            </div>
          </div>
        </div>

        {#if (info?.points ?? []).length}
          <div class="spots">
            <div class="row">
              <h4 class="grow">Positions <span class="muted small">{f.points ? 'custom' : 'vanilla'}</span></h4>
              {#if f.points}<button class="small" onclick={usePointsVanilla} title="Use the vanilla piece's positions again.">Use vanilla positions</button>{/if}
            </div>
            {#each info.points as r}
              <div class="row wrap small" title={r.tip}>
                <span class="rolechip" style={`border-color: rgb(${(ROLE_COL[r.role] ?? [1, 1, 1]).slice(0, 3).map((v) => Math.round(v * 255)).join(',')})`}>{r.label}</span>
                {#each pointsOf().map((q, i) => [q, i]).filter(([q]) => q.role === r.role) as [q, i]}
                  <button class="small" class:active={part === 'point' && selPt === i} onclick={() => { pointsCustom(); selPt = i as number; part = 'point'; if (tool === 'size') tool = 'move'; redraw++; }}>{roleIndex(i as number)}</button>
                {/each}
                {#if r.resizable}<span class="muted">(add/remove with N / Del when one is selected)</span>{:else}<span class="muted">(move / turn only)</span>{/if}
              </div>
            {/each}
            <p class="muted small">Where people stand, sit or work at this piece. Select one (here or its coloured ring in the view) and drag it; <b>P</b> shows stand-in people.</p>
          </div>
        {/if}
        {#if spotKind}
          <div class="spots">
            <div class="row">
              <h4 class="grow">{spotKind === 'card' ? 'Card spots' : 'Item spots'}
                <span class="muted small">{f.spots ? `${f.spots.length} custom` : `vanilla (${(t?.spots ?? []).filter((s: any) => s.kind === spotKind).length})`}</span></h4>
              {#if f.spots}
                <button class="small" onclick={addSpot} title="Add a copy of the selected spot, a little to the side.">+ Add</button>
                <button class="small" onclick={deleteSpot} disabled={!f.spots.length}>Delete</button>
                <button class="small" onclick={useBaseSpots} title="Drop your spots and use the vanilla piece's own again.">Use vanilla spots</button>
              {:else}
                <button class="small" onclick={spotsFromBase} disabled={!t} title="Copy the vanilla piece's spots so you can move, resize, add or remove them.">Customise spots</button>
              {/if}
            </div>
            {#if f.spots}
              <div class="spotlist">
                {#each f.spots as s, i}
                  <button class:active={i === sel} onclick={() => { sel = i; part = 'spot'; }}>
                    {i + 1}{#if s.kind === 'items'} · {fillItem ? `${fillCount(s, true)} ${fillItem.label}` : `${capacity(s)} units`}{/if}
                  </button>
                {/each}
              </div>
              {#if spot}
                <div class="grid6">
                  {#each ['x', 'y', 'z'] as ax, k}
                    <label class="field" title={['Left/right of the piece\'s centre, in metres.', 'Height above the floor, in metres (for item spots: the middle of the spot box; with height 0 the items stand on it).', 'Front/back, in metres. Vanilla shop shelves face the aisle on the + side.'][k]}>Position {ax} (m)<input type="number" step="0.01" value={spot.pos[k]} onchange={(e) => setPos(k, +e.currentTarget.value)} /></label>
                  {/each}
                  {#each ['x', 'y', 'z'] as ax, k}
                    <label class="field" title={['Tilt forward/back in degrees (vanilla card shelves lean cards 15°).', 'Turn around the vertical in degrees — which way the spot faces (180 = the other side of the piece).', 'Roll sideways in degrees.'][k]}>Rotation {ax} (°)<input type="number" step="5" value={spot.rot[k]} onchange={(e) => { spot.rot[k] = +e.currentTarget.value; changed(); }} /></label>
                  {/each}
                  {#if spot.kind === 'items'}
                    {#each ['Width', 'Depth', 'Height'] as nm, k}
                      <label class="field" title={['Width of the spot in metres. Items are spread evenly across it (see Grid across).', 'Depth of the spot in metres, front to back (see Grid deep).', 'Height of the spot in metres. 0 = items stand in one layer on the spot (vanilla shop shelves); more = stacked layers (see Grid high).'][k]}>{nm} (m)<input type="number" step="0.01" min="0" value={spot.size[k]} onchange={(e) => { spot.size[k] = Math.max(0, +e.currentTarget.value); if (gridFollows) fitGrid(spot); changed(); }} /></label>
                    {/each}
                    {#each ['across', 'deep', 'high'] as nm, k}
                      <label class="field" title={['How many item units fit side by side — this, deep and high set how many items the spot holds. A pack is 1 unit; bigger items (boxes, figurines) take several. Vanilla: 4.', 'How many item units fit front to back. Vanilla: 8.', 'How many layers of item units fit on top of each other. Vanilla shop shelves: 1.'][k]}>Grid {nm}<input type="number" step="1" min="1" value={spot.grid?.[k] ?? [4, 8, 1][k]}
                        onchange={(e) => { spot.grid = spot.grid ?? [4, 8, 1]; spot.grid[k] = Math.max(1, Math.round(+e.currentTarget.value)); changed(); }} /></label>
                    {/each}
                    <div class="gridinfo">
                      <b>Holds {packsIn(spot)} packs</b> <span class="muted small">({capacity(spot)} grid units{fillItem && fillItem.key !== 'BasicCardPack' ? ` · ${fillCount(spot, true)} × ${fillItem.label}` : ''})</span>
                      <label class="check small" title="While on, changing the spot's width, depth or height (fields or Size handles) changes the grid too, keeping the vanilla spacing between items — a bigger spot holds more. Off = the same number of items, spread out."><input type="checkbox" bind:checked={gridFollows} /> Grid follows size</label>
                      <button class="small" onclick={() => { fitGrid(spot); changed(); }} title="Set the grid from the spot's size with the vanilla spacing between items.">Fit grid to size</button>
                    </div>
                  {/if}
                  <label class="field" title="Where customers stand to take from this spot: left/right on the floor, in metres. Every spot has one; clear the field to put it back in front of the spot.">Customer x<input type="number" step="0.05" value={spot.customer?.[0] ?? ''} placeholder="auto"
                    onchange={(e) => { const v = opt(e.currentTarget.value); spot.customer = v === undefined ? defaultCustomer(spot) : [v, (spot.customer ?? defaultCustomer(spot))[1]]; changed(); }} /></label>
                  <label class="field" title="Where customers stand to take from this spot: front/back on the floor, in metres. It should be in the aisle in front of the spot. Clear the field to put it back in front of the spot.">Customer z<input type="number" step="0.05" value={spot.customer?.[1] ?? ''} placeholder="auto"
                    onchange={(e) => { const v = opt(e.currentTarget.value); spot.customer = v === undefined ? defaultCustomer(spot) : [(spot.customer ?? defaultCustomer(spot))[0], v]; changed(); }} /></label>
                  {#each ['x', 'y', 'z'] as ax, k}
                    <label class="field" title="Position of this spot's price tag (the label players click to set a price and customers read), in metres. Empty = where the vanilla spot had it.">Price tag {ax}<input type="number" step="0.01" value={spot.priceTag?.[k] ?? ''} placeholder="auto"
                      onchange={(e) => { const v = opt(e.currentTarget.value); if (v === undefined) spot.priceTag = undefined; else { spot.priceTag = spot.priceTag ?? [...spot.pos]; spot.priceTag[k] = v; } changed(); }} /></label>
                  {/each}
                  {#if spot.kind === 'items'}
                    <label class="check small" title="Allow placing whole delivery boxes here (like warehouse shelves), not only loose items."><input type="checkbox" checked={!!spot.boxes} onchange={(e) => { spot.boxes = e.currentTarget.checked; changed(); }} /> Whole boxes fit here</label>
                  {/if}
                </div>
                <p class="muted small">
                  {#if spot.kind === 'items'}Items fill the box in a grid of item units (a pack is 1×1×1: 4 across × 8 deep × 1 high = 32 packs). Height 0 = one layer standing on that plane, like vanilla shop shelves.{:else}The card sits at this point, facing along the spot's forward direction (vanilla: tilted 15°, facing the aisle).{/if}
                  Moving a spot moves its customer point and price tag with it.
                </p>
              {/if}
            {:else}
              <p class="muted small">Uses {baseOf(f)}'s own spots (shown in the view). "Customise spots" copies them so you can move, resize, add or remove spots.</p>
            {/if}
          </div>
        {/if}
      </section>

      <section>
        <h3>Shop</h3>
        <div class="grid4">
          <label class="field" title="Price in the furniture shop. Empty = the vanilla piece's price. Selling a placed piece gives back half.">Price<input type="number" min="1" placeholder={t ? String(t.price) : 'base'} value={f.price ?? ''} oninput={(e) => { f.price = opt(e.currentTarget.value); changed(); }} /></label>
          <label class="field" title="Shop level needed to buy it. Empty = the vanilla piece's level.">Shop level<input type="number" min="0" placeholder={t ? String(t.level) : 'base'} value={f.level ?? ''} oninput={(e) => { f.level = opt(e.currentTarget.value); changed(); }} /></label>
          <label class="field" title="How much a placed piece makes the shop more attractive to customers.">Deco bonus<input type="number" step="0.05" min="0" placeholder={t ? String(t.decoBonus) : 'base'} value={f.decoBonus ?? ''} oninput={(e) => { f.decoBonus = opt(e.currentTarget.value); changed(); }} /></label>
        </div>
        <p class="muted small">Empty = the "Start from" piece's vanilla values.</p>
      </section>

      <div class="row savebar">
        <div class="grow"></div>
        {#if dirty}<span class="muted small">Unsaved changes</span>{/if}
        <button onclick={undo} disabled={!undoStack.length || saving} title="Undo (Ctrl+Z)">↶ Undo</button>
        <button onclick={redo} disabled={!redoStack.length || saving} title="Redo (Ctrl+Y or Ctrl+Shift+Z)">↷ Redo</button>
        <button onclick={() => select(f.id)} disabled={!dirty || saving} title="Throw away every change since the last save.">Revert</button>
        <button class="primary" onclick={save} disabled={!dirty || saving}>{saving ? 'Saving…' : 'Save & install'}</button>
      </div>
    </div>
  {:else if view}
    <p class="muted" style="padding:20px">No custom furniture yet — pick a type and click “+ New”.</p>
  {/if}
</div>

{#if fromCatalog}<CatalogPicker kind="furniture" onadded={addedFromCatalog} onclose={() => (fromCatalog = false)} />{/if}

<style>
  .fwrap { display: flex; flex: 1; min-height: 0; }
  .list { width: 200px; flex-shrink: 0; border-right: 1px solid var(--line); padding: 10px; display: flex; flex-direction: column; gap: 4px; overflow: auto; }
  .list button { text-align: left; display: flex; gap: 8px; align-items: center; }
  .list button.active, .spotlist button.active { border-color: var(--accent); background: #22304d; }
  .list button.picked { border-color: var(--accent); background: #1c2740; }
  .sortby { display: flex; align-items: center; gap: 6px; margin-bottom: 4px; }
  .sortby select { flex: 1; }
  .newrow select { flex: 1; min-width: 0; }
  .ico { width: 36px; height: 36px; flex-shrink: 0; display: flex; align-items: center; justify-content: center; }
  .ico img { max-width: 100%; max-height: 100%; }
  .form { flex: 1; min-width: 0; overflow: auto; padding: 12px 14px; display: flex; flex-direction: column; gap: 14px; }
  section { background: var(--panel); border: 1px solid var(--line); border-radius: var(--radius); padding: 12px; display: flex; flex-direction: column; gap: 10px; }
  .editor { display: flex; flex-wrap: wrap; gap: 14px; align-items: flex-start; }
  .stage { flex: 1 1 380px; min-width: 0; }
  .scene { position: relative; height: clamp(300px, 55vh, 520px); background: radial-gradient(circle at 50% 40%, #2a3140, var(--bg)); border-radius: 6px; }
  .view3d { width: 100%; height: 100%; display: block; cursor: grab; touch-action: none; }
  .overlay { position: absolute; left: 84px; top: 10px; margin: 0; }
  .hint { position: absolute; left: 12px; right: 12px; bottom: 4px; margin: 0; pointer-events: none; }
  .intro { max-width: 900px; line-height: 1.5; }
  .helpbtn { position: absolute; left: 44px; top: 8px; z-index: 5; padding: 2px 9px; font-weight: bold; }
  .helpcard { position: absolute; left: 8px; top: 40px; z-index: 6; max-height: calc(100% - 60px); overflow: auto; background: rgba(14, 18, 26, 0.94);
    border: 1px solid var(--line); border-radius: 6px; padding: 10px 12px; font-size: 12px; width: min(420px, calc(100% - 16px)); }
  .helpcard table { border-collapse: collapse; width: 100%; }
  .helpcard th { text-align: left; padding: 8px 0 3px; color: var(--accent); font-weight: 600; }
  .helpcard td { padding: 2px 8px 2px 0; vertical-align: top; }
  .helpcard td:first-child { white-space: nowrap; color: #cfd6e4; }
  .helpcard kbd { font-size: 10px; border: 1px solid currentColor; border-radius: 3px; padding: 0 3px; opacity: 0.8; }
  .c-cust { color: #73f273; } .c-tag { color: #fff; } .c-area { color: #ff8c26; }
  .tools { position: absolute; top: 8px; right: 8px; left: 48px; display: flex; flex-wrap: wrap; justify-content: flex-end; gap: 4px; }
  .tools button { padding: 3px 10px; font-size: 12px; }
  .tools button.on { border-color: var(--accent); background: #22304d; }
  .seg { display: inline-flex; gap: 0; }
  .seg button { border-radius: 0; }
  .seg button:first-child { border-radius: 4px 0 0 4px; }
  .seg button:last-child { border-radius: 0 4px 4px 0; }
  .seg button.on { border-color: var(--accent); background: #22304d; }
  .fillsel { max-width: 170px; font-size: 12px; padding: 2px 4px; }
  .tools kbd { font-size: 10px; opacity: 0.6; border: 1px solid currentColor; border-radius: 3px; padding: 0 3px; margin-left: 2px; }
  .selpart { align-self: center; font-size: 12px; color: #ffd84d; background: rgba(0, 0, 0, 0.45); padding: 2px 8px; border-radius: 4px; }
  .side { flex: 1 1 240px; max-width: 340px; display: flex; flex-direction: column; gap: 10px; }
  .props { display: flex; flex-direction: column; gap: 8px; background: var(--bg); border: 1px solid var(--line); border-radius: var(--radius); padding: 10px; }
  .props h4, .spots h4 { margin: 0; font-size: 13px; }
  .spots { display: flex; flex-direction: column; gap: 8px; }
  .spotlist { display: flex; flex-wrap: wrap; gap: 4px; }
  .rolechip { border: 2px solid; border-radius: 10px; padding: 0 8px; min-width: 120px; }
  .spots .row button.small.active { border-color: var(--accent); background: #22304d; }
  .spotlist button { padding: 2px 8px; font-size: 12px; }
  .gridinfo { grid-column: 1 / -1; display: flex; flex-wrap: wrap; align-items: center; gap: 10px; }
  .grid2 { display: grid; grid-template-columns: 1fr 1fr; gap: 6px; }
  .grid2 input { width: 100%; min-width: 0; box-sizing: border-box; }
  .grid6 { display: grid; grid-template-columns: repeat(auto-fill, minmax(118px, 1fr)); gap: 6px; }
  .grid4 { display: grid; grid-template-columns: repeat(auto-fill, minmax(140px, 1fr)); gap: 8px; }
  .grid6 input, .grid4 input { width: 100%; min-width: 0; box-sizing: border-box; }
  section > .row { flex-wrap: wrap; }
  section > .row .field { min-width: 120px; }
  .thumbs { display: flex; gap: 8px; align-items: center; }
  .thumbs img { height: 44px; border-radius: 4px; background: var(--bg); }
  .check { display: flex; align-items: center; gap: 6px; }
  .savebar { position: sticky; bottom: 0; background: var(--bg); padding: 8px 0; }
  .wrap.row, .row.wrap { flex-wrap: wrap; }
  button.tiny { padding: 2px 7px; font-size: 12px; }
  .small { font-size: 12px; }
  .warn { color: #d9a400; }
</style>
