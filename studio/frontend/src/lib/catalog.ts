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

/** The group an entry is listed under: the mod it came from, else (sets) the game of its import source; '' = your own. */
export function groupOf(e: any, g: Record<string, string>): string {
  if (e.origin) return `From ${e.origin}`;
  if (e.source && e.source !== 'folder') return g[e.source] ?? '';
  return '';
}

/** Entries grouped (named groups A–Z, your own last). */
export function grouped(list: any[], g: Record<string, string>): [string, any[]][] {
  const m = new Map<string, any[]>();
  for (const e of list) {
    const k = groupOf(e, g);
    if (!m.has(k)) m.set(k, []);
    m.get(k)!.push(e);
  }
  return [...m.entries()].sort((a, b) => (a[0] === '') !== (b[0] === '') ? (a[0] === '' ? 1 : -1) : a[0].localeCompare(b[0]));
}

export const OWN_GROUP = 'Your own (image folders, made in Studio)';
