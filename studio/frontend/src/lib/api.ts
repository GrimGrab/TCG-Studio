// Thin helpers over the generated Wails bindings.
export * as App from '../../wailsjs/go/main/App';
export { EventsOn, EventsOff, BrowserOpenURL, OnFileDrop, OnFileDropOff } from '../../wailsjs/runtime/runtime';
import { Confirm, ImportSources } from '../../wailsjs/go/main/App';

// SuperLegend exists in the game's enum but has no rarity icon (shows as Common), so the studio doesn't offer it.
export const RARITIES = ['Common', 'Rare', 'Epic', 'Legendary'];
export const ELEMENTS = ['Fire', 'Earth', 'Water', 'Wind'];
export const BORDERS = ['Base', 'FirstEdition', 'Silver', 'Gold', 'EX', 'FullArt'];
export const FRAME_TEMPLATES = ['Tetramon', 'Destiny', 'Ghost', 'Megabot', 'FantasyRPG', 'CatJob', 'Ascension'];
export const LANES = ['Fire', 'Earth', 'Water', 'Wind'];

/** URL for a file inside a project folder (served by the Go asset handler). */
export function projectFile(projectId: string, rel: string | undefined, bust = 0): string {
  if (!rel) return '';
  return `/proj/${encodeURIComponent(projectId)}/${rel.split('/').map(encodeURIComponent).join('/')}${bust ? `?v=${bust}` : ''}`;
}

export function money(v: number | undefined | null): string {
  if (v === undefined || v === null || isNaN(v)) return '–';
  return v >= 100 ? `$${v.toFixed(0)}` : `$${v.toFixed(2)}`;
}

/** Display name of an import source (project meta.source). */
export function sourceName(source: string | undefined): string {
  return ({ scryfall: 'Scryfall', tcgdex: 'TCGdex', ygoprodeck: 'YGOPRODeck', optcg: 'TCGplayer', swudb: 'SWU-DB', lorcast: 'Lorcast', fab: 'TCGplayer', unionarena: 'TCGplayer', folder: 'Image folder', epl: 'EPL mod' } as Record<string, string>)[source ?? ''] ?? source ?? '';
}

let sourcesPromise: Promise<any[]> | null = null;

/** The import source's info (colour filter, rarity order…) for a project's meta.source; null for hand-made sets. */
export async function sourceInfo(source: string | undefined): Promise<any | null> {
  if (!source) return null;
  sourcesPromise ??= ImportSources().catch(() => { sourcesPromise = null; return []; });
  return (await sourcesPromise).find((s) => s.id === source) ?? null;
}

/** The imported set's code at its source ('' for hand-made sets). */
export function sourceCode(meta: any): string {
  return meta?.scryfallCode || meta?.setCode || '';
}

export function errText(e: unknown): string {
  return typeof e === 'string' ? e : (e as any)?.message ?? String(e);
}

export const RARITY_COLORS: Record<string, string> = {
  Common: '#9aa4b2', Rare: '#4fa3ff', Epic: '#b66dff', Legendary: '#ffb020', SuperLegend: '#ff5e7a'
};

/** One of a set's rarities (set.json "rarities"; mirrors setfmt.Rarity). */
export type RarityDef = { id: string; name: string; color?: string };

/** A set's rarities, lowest first: its own list, else Common…Legendary (SuperLegend too when an old card uses it). */
export function setRarities(set: any): RarityDef[] {
  if (set?.rarities?.length) return set.rarities;
  const list: RarityDef[] = RARITIES.map((r) => ({ id: r, name: r }));
  if (set?.cards?.some((c: any) => c.rarity === 'SuperLegend')) list.push({ id: 'SuperLegend', name: 'SuperLegend' });
  return list;
}

/** Shown name of a rarity id. */
export function rarityName(set: any, id: string): string {
  return setRarities(set).find((r) => r.id === id)?.name ?? id;
}

/**
 * The game rarity the mod uses for a rarity where the game only knows its 4 (rarity icon, fame): by its place
 * in the list — the n-th of up to four is the n-th game rarity, longer lists spread evenly (mirrors setfmt.TierOf).
 */
export function rarityTier(set: any, id: string): string {
  const list = setRarities(set);
  const i = Math.max(0, list.findIndex((r) => r.id === id));
  if (!set?.rarities?.length) return RARITY_COLORS[id] ? id : 'Common';
  return RARITIES[list.length <= 4 ? i : Math.round((i * 3) / (list.length - 1))];
}

/** Dot colour: the rarity's own, else a colour along Common → Legendary by its place in the list. */
export function rarityColor(set: any, id: string): string {
  const list = setRarities(set);
  const r = list.find((x) => x.id === id);
  if (r?.color) return r.color;
  if (!set?.rarities?.length) return RARITY_COLORS[id] ?? RARITY_COLORS.Common;
  const stops = RARITIES.map((x) => RARITY_COLORS[x]);
  const t = list.length <= 1 ? 0 : (Math.max(0, list.findIndex((x) => x.id === id)) / (list.length - 1)) * (stops.length - 1);
  const a = stops[Math.floor(t)], b = stops[Math.min(stops.length - 1, Math.floor(t) + 1)], f = t - Math.floor(t);
  const ch = (i: number) => Math.round(parseInt(a.slice(i, i + 2), 16) * (1 - f) + parseInt(b.slice(i, i + 2), 16) * f).toString(16).padStart(2, '0');
  return `#${ch(1)}${ch(3)}${ch(5)}`;
}

/**
 * Slot weights written in the game's 4 rarities (presets) → weights over the set's own rarities: each one's weight is shared by
 * the rarities at its place in the list (rarityTier), by card count. Sets without a list: unchanged.
 */
export function splitWeights(set: any, w: Record<string, number>): Record<string, number> {
  if (!set?.rarities?.length) return w;
  const counts: Record<string, number> = {};
  for (const c of set.cards ?? []) counts[c.rarity] = (counts[c.rarity] ?? 0) + 1;
  const out: Record<string, number> = {};
  for (const [key, weight] of Object.entries(w)) {
    if (set.rarities.some((r: RarityDef) => r.id === key)) { out[key] = (out[key] ?? 0) + weight; continue; }
    const members = set.rarities.filter((r: RarityDef) => rarityTier(set, r.id) === key && counts[r.id] > 0);
    const total = members.reduce((s: number, r: RarityDef) => s + counts[r.id], 0);
    for (const r of members) out[r.id] = (out[r.id] ?? 0) + Math.round((weight * counts[r.id] / total) * 10000) / 10000;
  }
  return out;
}

/** Id for a new rarity (mirrors setfmt.RarityID). */
export function rarityId(name: string): string {
  return name.toLowerCase().replace(/[^\p{L}\p{N}]+/gu, '-').replace(/^-+|-+$/g, '') || 'rarity';
}

/** Native Yes/No dialog titled "TCG Studio"; use instead of window.confirm(). */
export const ask = (message: string): Promise<boolean> => Confirm(message);
