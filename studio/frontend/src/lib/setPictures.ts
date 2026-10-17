// A set's files cut from a picture with the crop box (views/PictureSlot.svelte): its card back and its packs' textures and icons.
// studio.json keeps each slot's picture + crop (project.meta.pictures, app_setpictures.go); saving the set cuts it into the
// file set.json names.
import { App } from './api';
import { projectFile } from './urls';
import type { Picture } from './shopLook';

/** The slot of a pack's art field (packTexture, packIcon, boxTexture, boxIcon). */
export const packSlot = (packId: string, field: string) => `pack/${packId}/${field}`;

/** The set.json field a slot cuts into, read and written through. */
function field(project: any, slot: string): { get: () => string | undefined; set: (v: string | undefined) => void } | null {
  if (slot === 'cardBack') return { get: () => project.set.cardBack, set: (v) => (project.set.cardBack = v) };
  const m = /^pack\/(.+)\/(\w+)$/.exec(slot);
  const pack = m && project.set.packs.find((p: any) => p.id === m[1]);
  return pack ? { get: () => pack[m![2]], set: (v) => (pack[m![2]] = v) } : null;
}

/** The slot's picture for the crop box ({file: ''} = none). */
export function setPicture(project: any, slot: string, bust = 0): Picture {
  const s = project.meta?.pictures?.[slot];
  return s ? { file: s.file, url: projectFile(project.id, s.file, bust), crop: s.crop ?? null } : { file: '', url: '', crop: null };
}

/** Stores a crop box change: a picture (kept until the set is saved, which cuts it) or none, which also clears the set file
 *  (back to the global / vanilla art). */
export function putSetPicture(project: any, slot: string, v: Picture) {
  project.meta.pictures ??= {};
  if (v.file) {
    project.meta.pictures[slot] = { ...project.meta.pictures[slot], file: v.file, crop: v.crop ?? undefined };
    return;
  }
  delete project.meta.pictures[slot];
  field(project, slot)?.set(undefined);
}

/** Chooses a picture for a slot (copied into the set's folder); file '' = cancelled. */
export async function pickSetPicture(project: any, slot: string): Promise<{ file: string; url: string; crop: number[] | null }> {
  const s = await App.PickSetPicture(project.id, slot);
  return { file: s.file, url: s.file ? projectFile(project.id, s.file) : '', crop: s.crop ?? null };
}

/** Brings the page's copy in line with what SaveProject cut (slot → file, '' = the slot was dropped). */
export function applyBakedPictures(project: any, baked: Record<string, string> | undefined) {
  for (const [slot, rel] of Object.entries(baked ?? {})) {
    if (!rel) { delete project.meta.pictures?.[slot]; continue; }
    field(project, slot)?.set(rel);
    if (project.meta.pictures?.[slot]) project.meta.pictures[slot].out = rel;
  }
}
