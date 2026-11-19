import { describe, expect, it } from 'vitest';
import { anyCardHas, fallbackImage, cardVersions, setVersionImage, totalVersions, versionImage, versionsWithArt, writeCardVersions, cardsWithOwnShaders, hasShader, inheritLabel, shownShader, writeCardShader, writeSetShader, shaderSource } from './cardVersions';

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

  it('keeps shaders per version on the set and on cards', () => {
    const set: any = { cards: [{ id: 'a' }, { id: 'b' }] };
    writeSetShader(set, 'gold_foil', 'holo');
    expect(set.versionShaders).toEqual({ Gold_foil: 'holo' });
    const [a, b] = set.cards;
    expect(shownShader(set, a, 'Gold_foil')).toBe('holo');
    writeCardShader(a, 'Gold_foil', '');        // none on this card
    writeCardShader(b, 'Base', 'sweep');        // its own
    expect(shownShader(set, a, 'GOLD_FOIL')).toBe('');
    expect(shownShader(set, b, 'Base')).toBe('sweep');
    expect(shownShader(set, b, 'Gold_foil')).toBe('holo');
    expect(cardsWithOwnShaders(set)).toBe(2);
    expect(hasShader(set, b)).toBe(true);
    writeCardShader(a, 'Gold_foil', undefined); // follows the set again
    expect(a.versionShaders).toBeUndefined();
    // Nothing chosen: a foil version gets the stock holo of its border, a normal one nothing.
    writeSetShader(set, 'Gold_foil', undefined);
    expect(set.versionShaders).toBeUndefined();
    expect(shownShader(set, a, 'Gold_foil')).toBe('tcgcc-holo-gold');
    expect(shownShader(set, a, 'FirstEdition_foil')).toBe('tcgcc-holo-firstedition');
    expect(shownShader(set, a, 'Gold')).toBe('');
    expect(hasShader(set, a)).toBe(false);
    writeSetShader(set, 'Gold_foil', '');       // the set says: no shader, not even the stock holo
    expect(set.versionShaders).toEqual({ Gold_foil: '' });
    expect(shownShader(set, a, 'Gold_foil')).toBe('');
    expect(inheritLabel(set, 'Gold_foil', undefined, (id) => id)).toBe('Default: stock holo');
    expect(inheritLabel(set, 'Gold_foil', '', (id) => id)).toBe('Set: no shader');
    expect(inheritLabel(set, 'Gold', undefined, (id) => id)).toBe('Default: no shader');
    // Framed sets keep the game's own foil unless a shader is chosen.
    const framed: any = { renderMode: 'Framed', cards: [{ id: 'a' }] };
    expect(shownShader(framed, framed.cards[0], 'Gold_foil')).toBe('');
    expect(inheritLabel(framed, 'Gold_foil', undefined, (id) => id)).toBe('Default: no shader');
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
  it('says where the shader of a version comes from', () => {
    const set: any = { renderMode: 'FullImage', cards: [] };
    const card: any = { id: 'a' };
    expect(shaderSource(set, card, 'Gold_foil')).toBe('default');
    expect(shaderSource(set, card, 'Gold')).toBe('');
    writeSetShader(set, 'Gold_foil', 'x');
    expect(shaderSource(set, card, 'Gold_foil')).toBe('set');
    writeCardShader(card, 'Gold_foil', 'y');
    expect(shaderSource(set, card, 'Gold_foil')).toBe('card');
    writeCardShader(card, 'Gold_foil', '');
    expect(shaderSource(set, card, 'Gold_foil')).toBe('');
    expect(shaderSource({ renderMode: 'Framed', cards: [] }, {}, 'Gold_foil')).toBe('');
  });
});
