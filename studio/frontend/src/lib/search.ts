// Shared search matching for every list's search box: accents and punctuation don't matter ("pokemon" finds "Pokémon",
// "yugioh" finds "Yu-Gi-Oh!"), every word of the query must match somewhere (any order), and a game's name also matches
// its usual nicknames ("mtg", "ygo", "swu").

/** Players' short names for each game, keyed by the game name the import sources use. */
const GAME_ALIASES: Record<string, string[]> = {
  'Magic: The Gathering': ['mtg', 'magic', 'mtga'],
  'Pokémon TCG': ['ptcg', 'pkmn', 'poke'],
  'Pokémon (Japanese)': ['ptcg', 'pkmn', 'poke', 'jp', 'japan'],
  'Yu-Gi-Oh!': ['ygo', 'yugioh'],
  'One Piece Card Game': ['op', 'optcg', 'opcg'],
  'Disney Lorcana': ['lorcana'],
  'Star Wars: Unlimited': ['swu'],
  'Star Wars: Destiny': ['swd'],
  'Flesh and Blood': ['fab', 'fabtcg'],
  'Union Arena': ['ua', 'uatcg'],
  'Digimon Card Game': ['dcg', 'dtcg'],
  'Dragon Ball Super Card Game (Masters)': ['dbs', 'dbscg', 'masters'],
  'Dragon Ball Super: Fusion World': ['dbs', 'dbsfw', 'fw', 'fusionworld'],
  'Dragon Ball Z TCG': ['dbz'],
  'Cardfight!! Vanguard': ['cfv', 'vg'],
  'Weiß Schwarz': ['ws', 'wsjp'],
  'Force of Will': ['fow'],
  'Final Fantasy TCG': ['fftcg', 'ff'],
  'Gundam Card Game': ['gcg'],
  'Shadowverse: Evolve': ['sve', 'sv'],
  'Sorcery: Contested Realm': ['sorc'],
  'Warhammer Age of Sigmar: Champions': ['aos', 'aosc'],
  'World of Warcraft TCG': ['wow', 'wowtcg'],
  'hololive OFFICIAL CARD GAME': ['hocg', 'holo'],
  'Riftbound: League of Legends': ['lol', 'rb'],
  'Battle Spirits Saga': ['bss'],
  'Future Card Buddyfight': ['fcbf', 'bf'],
  'Grand Archive': ['ga', 'gatcg'],
  'Transformers TCG': ['ttcg'],
  'UniVersus': ['uvs', 'ufs'],
  'CookieRun: Braverse': ['crb', 'cookierun'],
  'MetaZoo': ['mz'],
  'WIXOSS': ['wx'],
  'Your own card images': ['own', 'mine', 'custom', 'folder'],
  'Import EPL mod': ['epl'],
};
const ALIASES = new Map(Object.entries(GAME_ALIASES).map(([g, a]) => [fold(g), a.join(' ')]));

/** Lower case without accents (é → e, ß → ss). */
export function fold(s: string): string {
  return s.normalize('NFD').replace(/[̀-ͯ]/g, '').replace(/ß/g, 'ss').toLowerCase();
}

/** Only letters and digits: "Yu-Gi-Oh!" → "yugioh". */
const compact = (s: string) => fold(s).replace(/[^\p{L}\p{N}]+/gu, '');

/**
 * Whether a search query matches an item described by `parts` (name, id, game…; empty parts are skipped). Every word of
 * the query must be found in the parts, ignoring accents, case and punctuation; a part that is a game name also matches
 * that game's nicknames. An empty query matches everything.
 */
export function matchSearch(query: string, ...parts: (string | null | undefined)[]): boolean {
  const words = fold(query).split(/\s+/).filter(Boolean);
  if (!words.length) return true;
  // Each part and each nickname on its own, so a match never runs from the end of one into the next.
  const fields = parts.filter(Boolean).flatMap((p) => {
    const f = fold(p as string);
    return [f, ...(ALIASES.get(f)?.split(' ') ?? [])];
  });
  const flat = fields.map(compact);
  return words.every((w) => {
    const cw = compact(w);
    return fields.some((f) => f.includes(w)) || (cw !== '' && flat.some((f) => f.includes(cw)));
  });
}
