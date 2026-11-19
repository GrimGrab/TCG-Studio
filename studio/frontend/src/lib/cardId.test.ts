import { describe, expect, it } from 'vitest';
import { cardIdProblem, renameCard } from './cardId';

const project = () => ({
  set: {
    cards: [
      { id: 'a', play: { evolvesFrom: '' } },
      { id: 'b', play: { evolvesFrom: 'a', effect: { effectMonsterTypeList: ['a', 'PiggyA'], searchData: { specificMonsterTypeList: ['a', 'b'] } } } },
    ],
    packs: [{ id: 'p', cards: ['a', 'b'] }, { id: 'all' }],
  },
  meta: { cards: { a: { scryfallId: 'x' }, b: {} } },
});

describe('card ids', () => {
  it('follows the rules the game reads ids with', () => {
    const set = project().set;
    expect(cardIdProblem(set, 'a', 'a2')).toBe('');
    expect(cardIdProblem(set, 'a', 'a')).toBe(''); // its own id
    expect(cardIdProblem(set, 'a', 'b')).toMatch(/already/);
    for (const bad of ['', ' a', 'a b', 'a:b', 'a/b', 'a\\b', 'a|b', '.', '..']) expect(cardIdProblem(set, 'a', bad)).not.toBe('');
  });

  it("renames the card and the set's own references to it", () => {
    const p: any = project();
    expect(renameCard(p, 'a', 'new')).toBe(true);
    expect(p.set.cards.map((c: any) => c.id)).toEqual(['new', 'b']);
    expect(p.set.packs[0].cards).toEqual(['new', 'b']);
    expect(p.set.packs[1].cards).toBeUndefined(); // a pack drawing from the whole set stays that way
    expect(p.set.cards[1].play.evolvesFrom).toBe('new');
    expect(p.set.cards[1].play.effect.effectMonsterTypeList).toEqual(['new', 'PiggyA']);
    expect(p.set.cards[1].play.effect.searchData.specificMonsterTypeList).toEqual(['new', 'b']);
    expect(p.meta.cards).toEqual({ new: { scryfallId: 'x' }, b: {} });
  });

  it('changes nothing for a taken or bad id', () => {
    const p: any = project();
    expect(renameCard(p, 'a', 'b')).toBe(false);
    expect(renameCard(p, 'a', 'a b')).toBe(false);
    expect(p).toEqual(project());
  });
});
