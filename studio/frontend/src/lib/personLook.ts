// What the Customers and Employees pages share about how a person looks.
import type { Characters, Face } from '../views/FaceEditor.svelte';
import { ask, errText } from './api';
import { facePortrait } from './faceLook';
import { accFile } from './urls';
import type { Cloth } from './characterView';

export type OutfitSlot = { slot: number; piece: string; variant?: string; clothing?: string };
export type SavedFace = { id: string; body: string; model: number; face: Face; portrait?: string; noWalkIns?: boolean };
export type Piece = { name: string; display: string; materials: number; mesh?: string; morph?: string; variants: { label: string; defs: { tint: string }[] }[] | null };
export type ClothesTpl = { bodies: Record<string, { slots: { label: string; items: Piece[] }[]; presets: { name: string; apparel: string[]; variants: number[] }[] }> };
export type Look = { id: string; name: string; body: string; piece: string; parts: { tint: string }[] };

/** The look fields of a regular or an employee. */
export type Person = { id: string; name: string; female?: boolean; model: number; outfit?: OutfitSlot[]; face?: Face; faceId?: string; portrait?: string };

export const bodyOf = (c: Person) => (c.female ? 'female' : 'male');
export const outfitOf = (c: Person, slot: number) => c.outfit?.find((o) => o.slot === slot);

/** What they wear in the face editor's whole-body view: their character's clothes with the outfit's changes. */
export function previewClothes(c: Person, clothesTpl: ClothesTpl | null, looks: Look[]): Cloth[] {
  const b = clothesTpl?.bodies[bodyOf(c)];
  if (!b) return [];
  const pre = b.presets.find((p) => p.name === (c.female ? 'Female' : 'Male') + c.model);
  const out: Cloth[] = [];
  b.slots.forEach((s, slot) => {
    const o = outfitOf(c, slot);
    const piece = s.items.find((it) => it.name === (o ? o.piece : pre?.apparel[slot]));
    if (!piece?.morph) return;
    let tint = piece.variants?.[o ? 0 : pre?.variants[slot] ?? 0]?.defs[0]?.tint;
    if (o?.variant) tint = piece.variants?.find((v) => v.label === o.variant)?.defs[0]?.tint ?? tint;
    if (o?.clothing) tint = looks.find((l) => l.id === o.clothing)?.parts[0]?.tint ?? tint;
    out.push({ url: `/clothtemplates/${piece.morph}`, color: tint ?? '#888888' });
  });
  return out;
}

/** What a portrait shows: it's made again on Save when this changed. */
export const personPortraitKey = (c: Person, looks: Look[]) => JSON.stringify({ v: 2, f: c.face, o: c.outfit, m: c.model, b: !!c.female, l: looks.map((l) => [l.id, l.parts[0]?.tint]) });

/** A per-situation map with this situation set to list (removed when empty); undefined when nothing is left. */
export function withKey(map: Record<string, string[]> | undefined, id: string, list: string[]) {
  const next = { ...(map ?? {}) };
  if (list.length) next[id] = list; else delete next[id];
  return Object.keys(next).length ? next : undefined;
}

/** How many situations the maps change. */
export const changed = (...maps: (Record<string, string[]> | undefined)[]) => new Set(maps.flatMap((m) => Object.keys(m ?? {}))).size;

/** What everyone of this setup can say in a situation, with where each line comes from. */
export function setupLines(builtIn: string[], added: string[] | undefined, off: string[] | undefined) {
  return [
    ...builtIn.filter((t) => !(off ?? []).includes(t)).map((text) => ({ text, from: 'built-in' })),
    ...(added ?? []).filter((t) => t.trim()).map((text) => ({ text, from: 'this setup' })),
  ];
}

/** The animations everyone of this setup can play at a moment. */
export function setupAnimsAt(m: string, defaults: Record<string, string[]>, off: Record<string, string[]>, added: Record<string, string[]>) {
  return [
    ...(defaults[m] ?? []).filter((id) => !(off[m] ?? []).includes(id)).map((id) => ({ id, from: 'built-in' })),
    ...(added[m] ?? []).map((id) => ({ id, from: 'this setup' })),
  ];
}

/** What each person's portrait showed when loaded. One that is still their saved face's own was drawn in someone else's clothes: no key, so the next save redraws it. */
export function portraitKeysOf(people: Person[], faces: SavedFace[], looks: Look[]) {
  return Object.fromEntries(people.map((c) => [c.id, c.portrait && faces.find((f) => f.id === c.faceId)?.portrait === c.portrait ? '' : personPortraitKey(c, looks)]));
}

/** Portraits of people with their own face, made again where what they show changed since loading (keys) or since the face editor (fresh). */
export async function refreshPortraits(people: Person[], chars: Characters | null, clothesTpl: ClothesTpl | null, looks: Look[],
  fresh: Record<string, string>, keys: Record<string, string>, onfail: (name: string, err: unknown) => void) {
  for (const c of people) {
    if (!c.face || c.model < 0 || !chars) { c.portrait = undefined; continue; }
    if (c.portrait && (fresh[c.id] ?? keys[c.id]) === personPortraitKey(c, looks)) continue;
    try { c.portrait = await facePortrait(chars, bodyOf(c), c.face, previewClothes(c, clothesTpl, looks)); }
    catch (e) { onfail(c.name, e); }
  }
}

/** A saved portrait or picture for an img: a data URL until saved, then "assets/…"; '' = none. */
export const pictureUrl = (p: string | undefined) => (!p ? '' : p.startsWith('data:') ? p : accFile(p));

/** Asks, then the face goes: true when it should. Those wearing it keep their copy. */
export async function confirmDeleteFace(f: SavedFace, people: Person[]) {
  const users = people.filter((x) => x.faceId === f.id);
  if (!(await ask('Delete this saved face?' + (users.length ? ` ${users.map((u) => u.name).join(', ')} keep${users.length === 1 ? 's' : ''} it as their own copy.` : '')))) return false;
  for (const u of users) u.faceId = undefined;
  return true;
}

/** A saved face was edited: a new portrait (in what wearer has on), and everyone with this face follows. */
export async function shareFaceEdit(f: SavedFace, wearer: Person, people: Person[], chars: Characters, clothesTpl: ClothesTpl | null, looks: Look[],
  notify: (t: string, k?: string) => void, onfresh?: (p: Person) => void) {
  try {
    f.portrait = await facePortrait(chars, f.body, f.face, previewClothes(wearer, clothesTpl, looks));
  } catch (e) { notify(`The portrait couldn't be made (${errText(e)}); it's made again when you save.`, 'warn'); }
  for (const u of people) {
    if (u.faceId !== f.id) continue;
    u.face = JSON.parse(JSON.stringify(f.face));
    u.model = f.model;
    u.portrait = f.portrait;
    onfresh?.(u);
  }
}
