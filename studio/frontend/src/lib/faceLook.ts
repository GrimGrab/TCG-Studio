// A regular's own face (customer.face) as the character preview draws it: the game characters' data (templates
// characters.json, gameextract/characters.go) turned into lib/characterView's Look and hair pieces. Shared by the face
// editor and the Customers page (portraits made at Save).
import { renderPortrait, type Cloth, type Look, type Piece } from './characterView';
import type { Bone } from './skeleton';

export type Face = {
  shapes: Record<string, number>; hair: string[]; hairColor: string;
  floats: Record<string, number>; textures: Record<string, string>; colors: Record<string, string>;
};
export type HairItem = { name: string; mesh?: string; texture?: string; shadow?: string };
export type Preset = { name: string; shapes: Record<string, number>; hair: string[]; hairColor?: string; floats: Record<string, number>; textures: Record<string, string>; colors: Record<string, string> };
export type Body = { mesh: string; skeleton?: Bone[]; parts: { material: string }[]; hair: { table: string; items: HairItem[] }[]; presets: Preset[] };
export type Characters = { bodies: Record<string, Body>; textures: Record<string, string> };

const rgb = (h: string | undefined) => (h ?? '#ffffff').slice(0, 7);

/** URL of a character texture by its game name ('' when the templates don't have it). */
export const charTexUrl = (chars: Characters) => (name: string) => (chars.textures[name] ? `/chartemplates/${chars.textures[name]}` : '');

function itemOf(B: Body, f: Face, table: RegExp): HairItem | undefined {
  const slot = B.hair.findIndex((h) => table.test(h.table));
  return slot >= 0 ? B.hair[slot]?.items.find((it) => it.name === f.hair[slot]) : undefined;
}

export function faceLook(B: Body, f: Face): Look {
  return {
    shapes: f.shapes,
    skin: f.textures['_Skin_Texture'] || 'T_Skin_01', skinTint: rgb(f.colors['_Skin_Tint']), skinTintAmount: f.floats['_Skin_Tint_Alpha'] ?? 0, shine: f.floats['_Skin_Smoothness_Offset'] ?? 0,
    hairColor: rgb(f.hairColor), eyeColor: rgb(f.colors['_Eye_Color']), lipsColor: rgb(f.colors['_Lips_Tint']),
    lipstick: f.floats['_Lipstick_Alpha'] ?? 0, freckles: f.floats['_Freckles'] ?? 0,
    makeUp: f.textures['_Make_Up'] || 'T_Flat_White', makeUpAmount: f.floats['_Make_Up_Alpha'] ?? 0,
    brows: itemOf(B, f, /^Eyebrows/)?.shadow || 'T_Flat_White', hairShadow: itemOf(B, f, /^Hair/)?.shadow || 'T_Flat_White',
    beardShadow: itemOf(B, f, /^Beards/)?.shadow || 'T_Flat_White',
    height: f.floats['Height'] ?? 1, width: f.floats['Width'] ?? 1,
  };
}

export function facePieces(B: Body, f: Face): Piece[] {
  const out: Piece[] = [];
  B.hair.forEach((h, slot) => {
    const it = h.items.find((x) => x.name === f.hair[slot]);
    if (it?.mesh) out.push({ kind: /Beard/.test(h.table) ? 'beard' : 'hair', url: `/chartemplates/${it.mesh}`, texture: it.texture || 'T_Hair_Strands_Cutout' });
  });
  return out;
}

/** Their portrait (PNG data URL) like the game's own, from their face and the clothes they wear. */
export function facePortrait(chars: Characters, body: string, f: Face, clothes: Cloth[]): Promise<string> {
  const B = chars.bodies[body];
  return renderPortrait(charTexUrl(chars), { url: `/chartemplates/${B.mesh}`, materials: B.parts.map((p) => p.material) },
    faceLook(B, f), facePieces(B, f), clothes);
}
