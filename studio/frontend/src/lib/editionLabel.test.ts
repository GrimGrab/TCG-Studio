import { describe, expect, it } from 'vitest';
import { cardLabel, labelCentre, labelOffset, labelOn, setLabel, writeLabel, LABEL_DEFAULT, type LabelLayout } from './editionLabel';

const lay = { cardWidth: 561.5, cardHeight: 785, offsetWidth: 645, offsetHeight: 900, cardX: -3.7, x: 0, y: -117.95, w: 308, h: 25.58 } as LabelLayout;

describe('edition label', () => {
  it('takes each field from the card, else the set, else the default', () => {
    const set: any = { editionLabel: { y: -10 } };
    const card: any = { editionLabel: { size: 2 } };
    expect(setLabel({})).toEqual(LABEL_DEFAULT);
    expect(cardLabel(set, card)).toEqual({ show: true, x: 0, y: -10, size: 2 });
    writeLabel(card, 'size', undefined);
    expect(card.editionLabel).toBeUndefined(); // nothing of its own left: the field goes
  });
  it('turns a dragged spot back into the offsets the mod reads', () => {
    const c = { ...LABEL_DEFAULT, x: 12.3, y: -20 };
    const [fx, fy] = labelCentre(lay, c);
    expect(labelOffset(lay, fx, fy)).toEqual([12.3, -20]);
    expect(labelOffset(lay, 5, -5)).toEqual([50, 65]); // clamped to the ranges offered
    expect(labelOffset(lay, -5, 5)).toEqual([-50, -40]);
  });
  it('draws none on Base, framed sets and hidden labels', () => {
    const set: any = { cards: [] };
    expect(labelOn(set, undefined, 'Gold_foil')?.text).toBe('Gold');
    expect(labelOn(set, undefined, 'Base')).toBeUndefined();
    expect(labelOn({ renderMode: 'Framed' }, undefined, 'Gold')).toBeUndefined();
    expect(labelOn(set, { editionLabel: { show: false } }, 'Gold')).toBeUndefined();
  });
});
