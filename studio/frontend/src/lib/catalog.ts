// Shared by the Catalog page and the "Add from catalog" picker: how catalog entries are grouped.
import { App } from './api';

let games: Promise<Record<string, string>> | null = null;

/** Import source id → its game ("scryfall" → "Magic: The Gathering"). */
export function sourceGames(): Promise<Record<string, string>> {
  games ??= App.ImportSources()
    .then((l: any[]) => Object.fromEntries(l.map((s) => [s.id, s.game])))
    .catch(() => { games = null; return {}; });
  return games;
}

/** The group an entry is listed under: the mod or shared file it came from, else (sets) the game of its import source;
 *  '' = your own. */
export function groupOf(e: any, g: Record<string, string>): string {
  if (e.origin) return e.originKind === SHARED ? `Shared: ${e.origin}` : `From ${e.origin}`;
  if (e.source && e.source !== 'folder') return g[e.source] ?? '';
  return '';
}

/** Entries grouped (named groups A–Z, your own last); key overrides the group of some entries. */
export function grouped(list: any[], g: Record<string, string>, key: (e: any) => string = (e) => groupOf(e, g)): [string, any[]][] {
  const m = new Map<string, any[]>();
  for (const e of list) {
    const k = key(e);
    if (!m.has(k)) m.set(k, []);
    m.get(k)!.push(e);
  }
  return [...m.entries()].sort((a, b) => (a[0] === '') !== (b[0] === '') ? (a[0] === '' ? 1 : -1) : a[0].localeCompare(b[0]));
}

export const OWN_GROUP = 'Your own (image folders, made in Studio)';

/** origin.KindShared: imported from a TCG Studio item file. */
export const SHARED = 'Shared file';

let kinds: Promise<any[]> | null = null;

/** The content registry (sets, accessories, furniture, decorations…): id, one, many, title. */
export function contentKinds(): Promise<any[]> {
  kinds ??= App.ContentKinds().catch(() => { kinds = null; return []; });
  return kinds;
}
