// What a card version is drawn with, for every card preview (Card look, the card editor, the card panel, the Cards tab's
// tiles): the setup's shaders and borders and the game's rims, loaded together, and which picture, border, rim and shader
// a version shows. What a set or card chose stays in cardVersions.ts; this only reads it.
import { App, projectFile, BORDERS } from './api';
import { templateFile } from './urls';
import { fallbackImage, isFullImage, shownBorder, shownShader, versionImage } from './cardVersions';
import type { Border, PictureShader } from './borders';
import type { CardFacts, ShaderParam } from './cardShaderView';

export type ShaderInfo = { id: string; name: string; code: string; params?: ShaderParam[]; builtIn?: boolean; draws?: string; mix?: string; mixStrength?: number };
export type BorderChoice = Border & { builtIn?: boolean };
/** One of the game's card rims, read from the game files (App.CardFrames). */
export type CardFrame = { template: string; border: string; file: string };
export type Looks = { shaders: ShaderInfo[]; borders: BorderChoice[]; frames: CardFrame[] };

export const noLooks = (): Looks => ({ shaders: [], borders: [], frames: [] });

/** The setup's shaders and borders and the game's rims. A list that can't be read comes back empty (previews then draw less). */
export async function loadLooks(): Promise<Looks> {
  const list = <T>(p: Promise<unknown>): Promise<T[]> => p.then((l) => ((l ?? []) as T[])).catch(() => []);
  const [shaders, borders, frames] = await Promise.all([
    list<ShaderInfo>(App.ListShaders()), list<BorderChoice>(App.ListBorders()), list<CardFrame>(App.CardFrames())
  ]);
  return { shaders, borders, frames };
}

/** Every full-image card's shape, width / height: a real card's 63 × 88 mm (the mod's SetCardUIPatch.CardShape). */
export const CARD_SHAPE = 63 / 88;
/**
 * How the mod makes a picture (width / height) card-shaped on a card without a border: a scan of a real card is close and
 * is stretched the few percent ('stretch'); any other picture is cropped from its middle ('crop'). With a border it is
 * always cropped to the border's opening.
 */
export const cardFit = (aspect: number): 'stretch' | 'crop' => (Math.abs(aspect / CARD_SHAPE - 1) > 0.06 ? 'crop' : 'stretch');

const byId = <T extends { id: string }>(list: T[], id: string): T | undefined => (id ? list.find((x) => x.id === id) : undefined);

/** A shader's or border's name for a choice: '' = `none`, an id not in the list = "missing …". */
export const nameOf = (list: { id: string; name: string }[], id: string, none: string, what: string) =>
  id ? list.find((x) => x.id === id)?.name || `missing ${what}` : none;

/** What the game tells a shader about a version (iBorder, iFoil); '' = the main picture (Base). */
export const versionFacts = (key: string): CardFacts => ({
  border: Math.max(0, BORDERS.indexOf((key || 'Base').replace('_foil', ''))), foil: key.endsWith('_foil'), rarity: 0, grade: 0
});

/** A version's picture as the game picks it: its own, else (foil) its border's non-foil one, else the main picture. '' = the main picture. */
export const shownImage = (card: any, key: string): string =>
  (key ? versionImage(card, key) ?? fallbackImage(card, key) : undefined) ?? card.image;

/** The border a version is drawn with (null = none: the game's rim). Choices of `card`, else the set's; '' = Base. */
export const borderFor = (looks: Looks, set: any, card: any, key: string): BorderChoice | null =>
  byId(looks.borders, shownBorder(set, card ?? {}, key || 'Base')) ?? null;

/** The shader drawn on a version's picture (undefined = none); `tried` = a shader previewed instead. */
export const shaderFor = (looks: Looks, set: any, card: any, key: string, tried = ''): ShaderInfo | undefined =>
  byId(looks.shaders, tried || shownShader(set, card ?? {}, key || 'Base'));

/**
 * The game's rim of a version's slot for the set's frame template, for a version without a border: a URL, '' (the game
 * draws none there: Full Art) or undefined (a framed set, or the rims weren't read from the game files yet).
 */
export function rimFor(looks: Looks, set: any, key: string): string | undefined {
  if (!isFullImage(set)) return undefined;
  const slot = (key || 'Base').replace('_foil', '');
  const f = looks.frames.find((x) => x.template === (set.frameTemplate || 'Tetramon') && x.border === slot);
  return f ? (f.file ? templateFile('frame', f.file) : '') : undefined;
}

/** A shader as BorderPreview draws it on a card's picture, with the card's mask and (ownValues) its own values. */
export function pictureShader(sh: ShaderInfo | undefined, card: any, projectId: string, imgBust: number, ownValues = true): PictureShader | undefined {
  if (!sh) return undefined;
  return {
    code: sh.code, params: sh.params, mix: sh.mix, mixStrength: sh.mixStrength, draws: sh.draws,
    own: ownValues ? card?.shaderParams : undefined, mask: card?.shaderMask ? projectFile(projectId, card.shaderMask, imgBust) : ''
  };
}
