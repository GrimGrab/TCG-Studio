// The conversions that rewrite a design when it opens (saved with the next save): a mistake here changes people's designs.
import { describe, expect, it } from 'vitest';
import { type Layer, type Model } from './accessoryArt';
import { faceFillsToRects } from './migrations';

const model: Model = {
  kind: 'Test', mesh: 'Test', textureSize: 1024, size: [1, 1, 1], icon: 'box',
  faces: [
    { id: 'a', label: 'A', net: [0, 0, 1, 2], targets: [{ rect: [100, 100, 300, 300], src: [0, 0, 1, 1] }] },
    { id: 'b', label: 'B', net: [1, 0, 2, 2], targets: [{ rect: [300, 100, 500, 300], src: [0, 0, 1, 1] }] },
  ],
};

/** A texture layer covering x0..x1 × y0..y1 (texture px). */
const box = (x0: number, y0: number, x1: number, y1: number, extra: Partial<Layer> = {}): Layer => ({
  id: 'l', kind: 'image', name: 'L', visible: true, opacity: 1, blend: 'source-over', space: 'texture',
  x: (x0 + x1) / 2, y: (y0 + y1) / 2, w: x1 - x0, h: y1 - y0, rot: 0, ...extra,
});

describe('faceFillsToRects', () => {
  const fill = (face?: string): Layer => ({ id: 'f', kind: 'fill', name: 'Colour', visible: true, opacity: 1, blend: 'multiply',
    x: 0, y: 0, w: 0, h: 0, rot: 0, face, color: '#ff0000', color2: '#000000' });

  it('turns an all-faces colour into one face-sized rectangle per face', () => {
    const out = faceFillsToRects(model, [fill('')])!;
    expect(out.map((l) => [l.name, l.x, l.y, l.w, l.h, l.face])).toEqual([
      ['Colour – A', 0.5, 1, 1, 2, undefined],
      ['Colour – B', 2, 1, 2, 2, undefined],
    ]);
    expect(out.every((l) => l.blend === 'multiply' && l.color2 === '#000000')).toBe(true);
    expect(new Set(out.map((l) => l.id)).size).toBe(2);
  });
  it('keeps the id of a one-face colour and leaves other layers alone', () => {
    const other = box(100, 100, 300, 300);
    const out = faceFillsToRects(model, [fill('b'), other])!;
    expect(out[0]).toMatchObject({ id: 'f', name: 'Colour', x: 2, w: 2 });
    expect(out[1]).toBe(other);
  });
  it('changes nothing without whole-face colours', () => {
    expect(faceFillsToRects(model, [box(100, 100, 300, 300)])).toBeNull();
  });
});
