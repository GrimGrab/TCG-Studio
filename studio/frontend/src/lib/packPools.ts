// Which cards a pack can give: its own card list (set.json pack "cards"), or the whole set when the list is empty
// (the mod's PackRoller reads it the same way). One place for the Cards tab filter, bulk edit and the card detail.

type Pack = { id: string; name: string; cards?: string[] };
type CardSet = { cards: { id: string }[]; packs: Pack[] };

/** Whether a pack can give the card. */
export function inPack(pack: Pack, id: string): boolean {
  return !pack.cards?.length || pack.cards.includes(id);
}

/** The set's packs that can give the card. */
export function packsOf(set: CardSet, id: string): Pack[] {
  return set.packs.filter((p) => inPack(p, id));
}

/** Puts cards on a pack's list (kept in set order). On a pack that draws from the whole set this starts its own list. */
export function addToPack(set: CardSet, pack: Pack, ids: string[]): void {
  const want = new Set([...(pack.cards ?? []), ...ids]);
  pack.cards = set.cards.map((c) => c.id).filter((id) => want.has(id));
}

/**
 * Takes cards off a pack's list. A pack that draws from the whole set gets a list of every other card. Returns why it
 * can't be done ('' = done): an empty list would mean the whole set again.
 */
export function removeFromPack(set: CardSet, pack: Pack, ids: string[]): string {
  const drop = new Set(ids);
  const from = pack.cards?.length ? pack.cards : set.cards.map((c) => c.id);
  const left = from.filter((id) => !drop.has(id));
  if (!left.length) return `"${pack.name}" needs at least one card: a pack with no card list draws from the whole set.`;
  pack.cards = left;
  return '';
}

/** Puts cards in a pack (on) or takes them out (off). Returns why it can't be done ('' = done). */
export function setInPack(set: CardSet, pack: Pack, ids: string[], on: boolean): string {
  if (on) {
    if (pack.cards?.length) addToPack(set, pack, ids); // a whole-set pack already gives every card
    return '';
  }
  return removeFromPack(set, pack, ids);
}

/** Packs with their own card list that give the card (a whole-set pack gives every card, so it isn't named per card). */
export function ownListPacks(set: CardSet, id: string): Pack[] {
  return set.packs.filter((p) => !!p.cards?.length && p.cards.includes(id));
}

const PACK_COLOURS = ['#4f9dff', '#f2a03d', '#5fd07a', '#e0607e', '#b07cf0', '#40c9c4', '#e6d35a', '#9aa4b5'];

/** A pack's colour on the Cards tab: by its place in the set's pack list. */
export function packColour(index: number): string {
  return PACK_COLOURS[Math.max(0, index) % PACK_COLOURS.length];
}
