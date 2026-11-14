// Card versions (6 borders × normal/foil) of a set and of each card, and each version's own picture. set.json: the set's
// "variants" (empty = all 12) is the default for every card; a card's own "variants" replaces it; "versionImages" maps a
// version to a picture (a version without one shows the card's main image). Keys are the game's names ("Gold_foil"),
// compared without case like the mod does.
import { BORDERS, BORDER_LABELS } from './api';

export const versionKey = (border: string, foil: boolean) => (foil ? `${border}_foil` : border);
/** All 12 keys in grid order (Base, Base_foil, FirstEdition, …). */
export const ALL_VERSIONS: string[] = BORDERS.flatMap((b: string) => [versionKey(b, false), versionKey(b, true)]);
const canon = new Map(ALL_VERSIONS.map((k) => [k.toLowerCase(), k]));
/** The spelling the game uses for a key in any case ("gold_FOIL" → "Gold_foil"); undefined for an unknown name. */
export const canonical = (key: string) => canon.get(key.toLowerCase());
export const versionLabel = (key: string) => {
  const k = canonical(key) ?? key;
  const foil = k.endsWith('_foil');
  const b = foil ? k.slice(0, -5) : k;
  return `${BORDER_LABELS[b] ?? b}${foil ? ' foil' : ''}`;
};

const toSet = (list: string[] | undefined) =>
  new Set((list?.length ? list : ALL_VERSIONS).map((v) => canonical(v)).filter((v): v is string => !!v));

/** The set's versions (the default for its cards). */
export const setVersions = (set: any): Set<string> => toSet(set.variants);
/** Whether the card has its own list instead of the set's. */
export const hasOwnVersions = (card: any) => !!card.variants?.length;
/** The versions a card has: its own list, else the set's. */
export const cardVersions = (set: any, card: any): Set<string> => (hasOwnVersions(card) ? toSet(card.variants) : setVersions(set));

/** A list in grid order; undefined for all 12. */
const listOf = (on: Set<string>) => {
  const list = ALL_VERSIONS.filter((k) => on.has(k));
  return list.length === 12 ? undefined : list;
};

/** Writes the set's default versions (at least one stays on; all 12 = the field left out). */
export function writeSetVersions(set: any, on: Set<string>) {
  if (on.size === 0) return;
  set.variants = listOf(on);
}

/**
 * Writes a card's versions. The same list as the set's = the card follows the set again (its field left out), so a later
 * change of the set's default reaches it. At least one version stays on.
 */
export function writeCardVersions(set: any, card: any, on: Set<string>) {
  if (on.size === 0) return;
  const same = on.size === setVersions(set).size && [...on].every((k) => setVersions(set).has(k));
  card.variants = same ? undefined : ALL_VERSIONS.filter((k) => on.has(k));
}

/** The card follows the set's versions again. */
export const followSet = (card: any) => { card.variants = undefined; };

/** The version's own picture (relative path), or undefined (= the main image). */
export function versionImage(card: any, key: string): string | undefined {
  const imgs = card.versionImages;
  if (!imgs) return undefined;
  const want = key.toLowerCase();
  for (const [k, v] of Object.entries(imgs)) if (k.toLowerCase() === want && v) return v as string;
  return undefined;
}

/**
 * The picture the game shows for a version when it has none of its own, like the mod: a foil uses the same border's normal
 * picture (Gold foil → Gold); undefined = the main image.
 */
export function fallbackImage(card: any, key: string): string | undefined {
  const k = canonical(key) ?? key;
  return k.endsWith('_foil') ? versionImage(card, k.slice(0, -5)) : undefined;
}

/** Sets (rel) or removes (undefined) a version's own picture; an empty map is left out. */
export function setVersionImage(card: any, key: string, rel: string | undefined) {
  const k = canonical(key) ?? key;
  const next: Record<string, string> = {};
  for (const [x, v] of Object.entries(card.versionImages ?? {})) if (x.toLowerCase() !== k.toLowerCase() && v) next[x] = v as string;
  if (rel) next[k] = rel;
  card.versionImages = Object.keys(next).length ? next : undefined;
}

// ---- card shaders (set.json "versionShaders" on the set and on a card; ids of the setup's shaders)

const entryOf = (map: Record<string, string> | undefined, key: string): string | undefined => {
  if (!map) return undefined;
  const want = key.toLowerCase();
  for (const [k, v] of Object.entries(map)) if (k.toLowerCase() === want) return v;
  return undefined;
};
const withEntry = (map: Record<string, string> | undefined, key: string, v: string | undefined) => {
  const k = canonical(key) ?? key;
  const next: Record<string, string> = {};
  for (const [x, old] of Object.entries(map ?? {})) if (x.toLowerCase() !== k.toLowerCase()) next[x] = old;
  if (v !== undefined) next[k] = v;
  return Object.keys(next).length ? next : undefined;
};

/**
 * The stock holo a foil version is drawn with when nothing was chosen for it (the mod's [Foil] HoloFoil, on by default):
 * "tcgcc-holo-<border>". Undefined for normal versions.
 */
export const stockShader = (key: string): string | undefined => {
  const k = canonical(key) ?? key;
  return k.endsWith('_foil') ? `tcgcc-holo-${k.slice(0, -5).toLowerCase()}` : undefined;
};
/** The set's choice for a version: a shader id, '' (no shader) or undefined (nothing chosen: foil = the stock holo). */
export const setShader = (set: any, key: string): string | undefined => entryOf(set.versionShaders, key);
/** Sets the set's choice for a version (undefined = nothing chosen). */
export function writeSetShader(set: any, key: string, id: string | undefined) {
  set.versionShaders = withEntry(set.versionShaders, key, id);
}
/** A card's own choice for a version: a shader id, '' (no shader even when the set has one) or undefined (the set's). */
export const cardShader = (card: any, key: string): string | undefined => entryOf(card.versionShaders, key);
/** Sets a card's own choice (undefined = follow the set). */
export function writeCardShader(card: any, key: string, id: string | undefined) {
  card.versionShaders = withEntry(card.versionShaders, key, id);
}
/** The shader chosen for a card's version by the card or the set: an id, '' (none) or undefined (nothing chosen). */
export const chosenShader = (set: any, card: any, key: string): string | undefined => cardShader(card, key) ?? setShader(set, key);
/** The shader the game draws a card's version with ('' = none): the choice, else the stock holo of a foil version. */
export const shownShader = (set: any, card: any, key: string): string =>
  chosenShader(set, card, key) ?? (isFullImage(set) ? stockShader(key) : undefined) ?? '';
/** Full-image sets get the stock holo on foils; framed sets keep the game's own foil unless a shader is chosen. */
export const isFullImage = (set: any): boolean => set.renderMode !== 'Framed';
/** How many cards have shader choices of their own. */
export const cardsWithOwnShaders = (set: any): number => set.cards.filter((c: any) => c.versionShaders && Object.keys(c.versionShaders).length).length;
/** Whether a shader was chosen for any version of the card (by the card or the set; the stock holo of foils doesn't count). */
export const hasShader = (set: any, card: any): boolean => ALL_VERSIONS.some((k) => !!chosenShader(set, card, k));
/** What a version gets when the choice is left to the level above: the label of that option in a picker. */
export function inheritLabel(set: any, key: string, above: string | undefined, nameOf: (id: string) => string): string {
  if (above !== undefined) return above ? `Set: ${nameOf(above)}` : 'Set: no shader';
  return isFullImage(set) && stockShader(key) ? 'Default: stock holo' : 'Default: no shader';
}

/** The version the main image is (it never has a separate picture). */
export const MAIN_VERSION = 'Base';

/** The versions that have a picture of their own (Base = the main image, not counted). */
export const versionsWithArt = (card: any): Set<string> =>
  new Set(ALL_VERSIONS.filter((k) => k !== MAIN_VERSION && !!versionImage(card, k)));

/** How many versions the binder lists for the set: each card's own count. */
export const totalVersions = (set: any) => set.cards.reduce((n: number, c: any) => n + cardVersions(set, c).size, 0);

/** Whether any card of the set has this version (border null = any border, foil null = normal or foil). */
export function anyCardHas(set: any, border: string | null, foil: boolean | null): boolean {
  const lists = [setVersions(set), ...set.cards.filter(hasOwnVersions).map((c: any) => cardVersions(set, c))];
  return lists.some((on) => BORDERS.some((b: string) => (border == null || b === border) &&
    [false, true].some((f) => (foil == null || f === foil) && on.has(versionKey(b, f)))));
}
