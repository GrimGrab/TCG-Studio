import { describe, expect, it } from 'vitest';
import { borderFor, rimFor, shaderFor, shownImage, cardFit, versionFacts, type Looks } from './cardLooks';

describe('card looks', () => {
  const looks: Looks = {
    shaders: [{ id: 'sh', name: 'Shine', code: '' }],
    borders: [{ id: 'gold', name: 'Gold leaf', layers: [] }],
    frames: [{ template: 'Tetramon', border: 'Base', file: 'Tetramon_Base.png' }, { template: 'Tetramon', border: 'FullArt', file: '' }]
  };

  it('picks what a version is drawn with; the main picture is Base', () => {
    const set: any = { renderMode: 'FullImage', frameTemplate: 'Tetramon', borders: { Base: { look: 'gold' } }, versionShaders: { Gold: 'sh' }, cards: [] };
    const card: any = { image: 'a.png', versionImages: { Gold_foil: 'gf.png' } };
    expect(borderFor(looks, set, card, '')?.name).toBe('Gold leaf');
    expect(borderFor(looks, set, card, 'Silver')).toBeNull();
    expect(shaderFor(looks, set, card, 'Gold')?.name).toBe('Shine');
    expect(shaderFor(looks, set, card, 'Gold', 'sh')?.id).toBe('sh');
    expect(shownImage(card, '')).toBe('a.png');
    expect(shownImage(card, 'Gold_foil')).toBe('gf.png');
    expect(shownImage(card, 'Silver')).toBe('a.png');
    expect(versionFacts('')).toEqual({ border: 0, foil: false, rarity: 0, grade: 0 });
    expect(versionFacts('Gold_foil')).toEqual({ border: 3, foil: true, rarity: 0, grade: 0 });
  });

  it("gives the game's rim of the slot, none on Full Art, nothing for framed sets", () => {
    const set: any = { renderMode: 'FullImage', frameTemplate: 'Tetramon' };
    expect(rimFor(looks, set, '')).toContain('Tetramon_Base.png');
    expect(rimFor(looks, set, 'FullArt_foil')).toBe('');
    expect(rimFor(looks, set, 'Gold')).toBeUndefined(); // not read from the game files
    expect(rimFor(looks, { ...set, renderMode: 'Framed' }, '')).toBeUndefined();
  });

  it('makes every picture card-shaped: scans stretched, anything else cropped', () => {
    for (const odd of [512 / 637, 1, 1.5, 0.5]) expect(cardFit(odd)).toBe('crop'); // 'random', square, landscape, tall
    for (const scan of [0.716, 0.686, 0.717, 0.711]) expect(cardFit(scan)).toBe('stretch'); // MTG, YGO, Pokémon, the game's own
  });
});
