// Where the game stands shop items on a shelf: ShelfCompartment.CalculatePositionList (Assembly-CSharp\ShelfCompartment.cs:440)
// rebuilt from the mod's template export (accessories.json v2: "shelves", "itemPrefab", item placement fields).
// Everything here is in Unity space (shelf root space for positions); the editor mirrors z to draw it.

import { type Mat, type Box, ident, fromRows, transformBox } from './figurineView';

export interface Compartment {
  path: string;
  start: number[]; endWidth: number[]; endDepth: number[]; endHeight: number[];
  right: number[]; up: number[]; forward: number[];
  width: number; depth: number; height: number;
  sizeX: number; sizeY: number; sizeZ: number;
  m_CanPutItem: boolean; m_ApplyScaleOffset: boolean; m_HeightGoesUp: boolean; m_AffectedByTallItem: boolean;
  posCount: number;
}

export interface ShelfTpl { name: string; mesh: string | null; isPrefab: boolean; m_ItemNotForSale: boolean; compartments: Compartment[] }

/** The template fields of a toy item this module uses. */
export interface ItemTpl {
  type: string;
  itemDimension: number[];
  isTallItem: boolean;
  posYOffsetInBox: number;
  scaleOffsetInBox: number;
  bounds: { center: number[]; size: number[] } | null;
  mesh: string | null;
  texture: string | null;
}

/** Mathf.RoundToInt = Math.Round: halves go to the even number. */
export function roundToInt(v: number): number {
  const f = Math.floor(v), d = v - f;
  if (Math.abs(d - 0.5) < 1e-9) return f % 2 === 0 ? f : f + 1;
  return Math.round(v);
}

export interface Grid { maxX: number; maxY: number; maxZ: number; count: number }

export function grid(c: Compartment, item: ItemTpl): Grid {
  let dx = item.itemDimension[0];
  if (dx > c.sizeX) dx = c.sizeX;
  const maxX = roundToInt(c.sizeX / dx);
  const maxY = roundToInt(c.sizeY / item.itemDimension[1]);
  let maxZ = roundToInt(c.sizeZ / item.itemDimension[2]);
  if (item.isTallItem && c.m_AffectedByTallItem) maxZ = roundToInt(c.sizeZ / (item.itemDimension[2] * 2));
  // The game indexes m_PosList (posCount entries) with every position; more would overflow (vanilla bug in ShelfCompartment.CalculatePositionList).
  const count = Math.min(maxX * maxY * maxZ, c.posCount || Infinity);
  return { maxX, maxY, maxZ, count };
}

/** Item root matrices (Unity, shelf root space) for every slot of a compartment, in the game's order. */
export function slotMatrices(c: Compartment, item: ItemTpl): Mat[] {
  const g = grid(c, item);
  const k = 1 + (c.m_ApplyScaleOffset ? item.scaleOffsetInBox : 0);
  const posY = c.m_ApplyScaleOffset ? item.posYOffsetInBox : 0;
  const v2 = c.m_HeightGoesUp ? c.up : c.up.map((x) => -x);
  const out: Mat[] = [];
  for (let i = 0; i < g.maxZ; i++) {
    let num = c.height / g.maxZ / 2;
    if (item.isTallItem && c.m_AffectedByTallItem) num = c.height / g.maxZ;
    const num2 = num * (i * 2 + 1);
    for (let j = 0; j < g.maxY; j++) {
      const num3 = (c.depth / g.maxY / 2) * (j * 2 + 1);
      for (let x = 0; x < g.maxX; x++) {
        const num4 = (c.width / g.maxX / 2) * (x * 2 + 1);
        const p = [0, 1, 2].map((q) => c.start[q] - c.right[q] * num4 + c.forward[q] * num3 + v2[q] * num2 + v2[q] * -posY);
        const m = ident();
        for (let q = 0; q < 3; q++) {
          m[q] = c.right[q] * k; m[4 + q] = c.up[q] * k; m[8 + q] = c.forward[q] * k; m[12 + q] = p[q];
        }
        out.push(m);
        if (out.length >= g.count) return out;
      }
    }
  }
  return out;
}

/** Item root → item mesh (the prefab's mesh child: scale 0.1, turned 180° about Y in game 1.02). */
export function meshMatrix(prefab: any): Mat {
  return prefab?.meshMatrix ? fromRows(prefab.meshMatrix) : new Float32Array([-0.1, 0, 0, 0, 0, 0.1, 0, 0, 0, 0, -0.1, 0, 0, 0, 0, 1]);
}

/** Centimetres per mesh unit (1 unit × prefab scale × 100). */
export function cmPerUnit(prefab: any): number {
  const m = meshMatrix(prefab);
  return Math.hypot(m[0], m[1], m[2]) * 100;
}

export interface Fit {
  slot: { w: number; d: number };        // slot size (m) of the reference compartment
  base: { w: number; d: number; h: number }; // vanilla toy extents in item space (m)
  fig: { w: number; d: number; h: number; top: number };
  clearance: number;                       // free height above the compartment's start plane (m), Infinity if nothing above
  perCompartment: number;
  problems: string[];
}

/** Compares a figurine (mesh-space box, Unity) with its base toy in a compartment. */
export function fitCheck(shelf: ShelfTpl, c: Compartment, item: ItemTpl, prefab: any, figBox: Box): Fit {
  const g = grid(c, item);
  const mm = meshMatrix(prefab);
  const ext = (b: Box) => transformBox(mm, b);
  const baseBox = item.bounds ? boxOf(item.bounds) : figBox;
  const bi = ext(baseBox), fi = ext(figBox);
  const slot = { w: c.width / g.maxX, d: c.depth / g.maxY };
  const base = { w: bi.max[0] - bi.min[0], d: bi.max[2] - bi.min[2], h: bi.max[1] - bi.min[1] };
  const fig = { w: fi.max[0] - fi.min[0], d: fi.max[2] - fi.min[2], h: fi.max[1] - fi.min[1], top: fi.max[1] };
  // Height to the next compartment directly above.
  let clearance = Infinity;
  for (const o of shelf.compartments) {
    if (o === c) continue;
    const d = [0, 1, 2].map((q) => o.start[q] - c.start[q]);
    const up = dot(d, c.up);
    if (up > 0.01 && Math.abs(dot(d, c.right)) < c.width * 0.9 && Math.abs(dot(d, c.forward)) < c.depth * 0.9) clearance = Math.min(clearance, up);
  }
  const problems: string[] = [];
  const cm = (m: number) => `${(m * 100).toFixed(1)} cm`;
  const allowW = Math.max(slot.w, base.w) * 1.05, allowD = Math.max(slot.d, base.d) * 1.05;
  if (fig.w > allowW) problems.push(`too wide for its slot: ${cm(fig.w)} (slot ${cm(slot.w)}, vanilla ${cm(base.w)}) — neighbours will overlap`);
  if (fig.d > allowD) problems.push(`too deep for its slot: ${cm(fig.d)} (slot ${cm(slot.d)}, vanilla ${cm(base.d)}) — the rows will overlap`);
  if (fig.top > clearance) problems.push(`too tall for this shelf: top at ${cm(fig.top)}, the shelf above is at ${cm(clearance)}`);
  return { slot, base, fig, clearance, perCompartment: g.count, problems };
}

export function boxOf(b: { center: number[]; size: number[] }): Box {
  return { min: b.center.map((c, q) => c - b.size[q] / 2), max: b.center.map((c, q) => c + b.size[q] / 2) };
}

function dot(a: number[], b: number[]) { return a[0] * b[0] + a[1] * b[1] + a[2] * b[2]; }

