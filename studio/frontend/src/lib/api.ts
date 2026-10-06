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

/** Native Yes/No dialog titled "TCG Studio"; use instead of window.confirm(). */
export const ask = (message: string): Promise<boolean> => Confirm(message);
