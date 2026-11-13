import { describe, expect, it } from 'vitest';
import { anyCardHas, fallbackImage, cardVersions, setVersionImage, totalVersions, versionImage, versionsWithArt, writeCardVersions } from './cardVersions';

describe('card versions', () => {
  it('follows the set until the card gets its own list', () => {
    const set: any = { variants: ['Base', 'Gold_foil'], cards: [{ id: 'a' }, { id: 'b' }] };
    expect([...cardVersions(set, set.cards[0])]).toEqual(['Base', 'Gold_foil']);
    writeCardVersions(set, set.cards[0], new Set(['FullArt']));
    expect(set.cards[0].variants).toEqual(['FullArt']);
    expect(totalVersions(set)).toBe(3);
    // Back to the set's list = follows the set again.
    writeCardVersions(set, set.cards[0], new Set(['Base', 'Gold_foil']));
    expect(set.cards[0].variants).toBeUndefined();
  });

  it('never leaves a card without versions', () => {
    const set: any = { cards: [{ id: 'a', variants: ['EX'] }] };
    writeCardVersions(set, set.cards[0], new Set());
    expect(set.cards[0].variants).toEqual(['EX']);
  });

  it('knows which versions any card has', () => {
    const set: any = { variants: ['Base'], cards: [{ id: 'a' }, { id: 'b', variants: ['Gold_foil'] }] };
    expect(anyCardHas(set, 'Gold', true)).toBe(true);
    expect(anyCardHas(set, 'Silver', null)).toBe(false);
    expect(anyCardHas(set, null, true)).toBe(true);
  });

  it('stores pictures per version, case-insensitively', () => {
    const card: any = {};
    setVersionImage(card, 'gold_foil', 'images/a_gf.png');
    expect(card.versionImages).toEqual({ Gold_foil: 'images/a_gf.png' });
    expect(versionImage(card, 'GOLD_FOIL')).toBe('images/a_gf.png');
    expect([...versionsWithArt(card)]).toEqual(['Gold_foil']);
    expect(fallbackImage(card, 'Gold_foil')).toBeUndefined(); // it has its own
    setVersionImage(card, 'Gold', 'images/a_g.png');
    expect(fallbackImage(card, 'Gold_foil')).toBe('images/a_g.png');
    expect(fallbackImage(card, 'Silver_foil')).toBeUndefined(); // → main image
    expect(fallbackImage(card, 'Gold')).toBeUndefined();
    setVersionImage(card, 'Gold', undefined);
    setVersionImage(card, 'Gold_foil', undefined);
    expect(card.versionImages).toBeUndefined();
  });
});
