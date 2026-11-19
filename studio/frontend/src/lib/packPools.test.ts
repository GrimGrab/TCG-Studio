import { describe, expect, it } from 'vitest';
import { addToPack, inPack, ownListPacks, packsOf, removeFromPack, setInPack } from './packPools';

const set = () => ({
  cards: [{ id: 'a' }, { id: 'b' }, { id: 'c' }],
  packs: [{ id: 'p1', name: 'One', cards: [] as string[] }, { id: 'p2', name: 'Two', cards: ['c'] }],
});

describe('pack pools', () => {
  it('an empty list means the whole set', () => {
    const s = set();
    expect(inPack(s.packs[0], 'b')).toBe(true);
    expect(packsOf(s, 'c').map((p) => p.id)).toEqual(['p1', 'p2']);
  });
  it('adding keeps set order and starts a list on a whole-set pack', () => {
    const s = set();
    addToPack(s, s.packs[1], ['a']);
    expect(s.packs[1].cards).toEqual(['a', 'c']);
    addToPack(s, s.packs[0], ['b']);
    expect(s.packs[0].cards).toEqual(['b']);
  });
  it('removing from a whole-set pack lists every other card, and never empties a list', () => {
    const s = set();
    expect(removeFromPack(s, s.packs[0], ['b'])).toBe('');
    expect(s.packs[0].cards).toEqual(['a', 'c']);
    expect(removeFromPack(s, s.packs[1], ['c'])).not.toBe('');
    expect(s.packs[1].cards).toEqual(['c']);
  });
  it('the brush: on keeps a whole-set pack whole, off refuses the last card, own-list packs per card', () => {
    const s = set();
    expect(setInPack(s, s.packs[0], ['a'], true)).toBe('');
    expect(s.packs[0].cards).toEqual([]);
    expect(setInPack(s, s.packs[1], ['b', 'a'], true)).toBe('');
    expect(s.packs[1].cards).toEqual(['a', 'b', 'c']);
    expect(setInPack(s, s.packs[1], ['a', 'b', 'c'], false)).not.toBe('');
    expect(s.packs[1].cards).toEqual(['a', 'b', 'c']);
    expect(ownListPacks(s, 'b').map((p) => p.id)).toEqual(['p2']);
  });
});
