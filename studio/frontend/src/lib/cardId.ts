// A card's id: what the game's saves, decks and favourites know the card by ("<set>:<card>", "custom:<set>:<card>"). The one
// place that renames one: the set's own references follow; everything outside the set keeps the old id (ChangeCardId.svelte
// tells the player what that means). Rules as setfmt SafeID / the mod's SetLoader.IsSafeId.

/** Why newId can't be this card's id ('' = it can). */
export function cardIdProblem(set: any, oldId: string, newId: string): string {
  if (!newId) return 'The id is empty.';
  if (newId !== newId.trim()) return 'The id starts or ends with a space.';
  if (/[\s:/\\|]/.test(newId)) return "The id can't contain spaces, ':', '/', '\\' or '|'.";
  if (newId === '.' || newId === '..') return "The id can't be '.' or '..'.";
  if (newId !== oldId && (set.cards ?? []).some((c: any) => c.id === newId)) return 'Another card in this set already has this id.';
  return '';
}

/** Renames card oldId to newId in the set (set.json) and its Studio data (studio.json meta): the card, the packs' card
 *  lists, evolves-from and the play effects' card lists of every card, the card's import data. Returns false when nothing
 *  changed (same id or a problem). Part of the set's draft: nothing is written until Save. */
export function renameCard(project: any, oldId: string, newId: string): boolean {
  const set = project.set;
  if (oldId === newId || cardIdProblem(set, oldId, newId)) return false;
  const card = (set.cards ?? []).find((c: any) => c.id === oldId);
  if (!card) return false;
  card.id = newId;
  const swap = (list: string[] | undefined) => list?.map((id) => (id === oldId ? newId : id));
  for (const p of set.packs ?? []) if (p.cards) p.cards = swap(p.cards);
  for (const c of set.cards ?? []) {
    const play = c.play;
    if (!play) continue;
    if (play.evolvesFrom === oldId) play.evolvesFrom = newId;
    const eff = play.effect;
    if (eff?.effectMonsterTypeList) eff.effectMonsterTypeList = swap(eff.effectMonsterTypeList);
    if (eff?.searchData?.specificMonsterTypeList) eff.searchData.specificMonsterTypeList = swap(eff.searchData.specificMonsterTypeList);
  }
  const cards = project.meta?.cards;
  if (cards && oldId in cards) {
    cards[newId] = cards[oldId];
    delete cards[oldId];
  }
  return true;
}
