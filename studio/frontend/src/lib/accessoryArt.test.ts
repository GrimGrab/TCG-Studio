// The conversions that rewrite a design when it opens (saved with the next save): a mistake here changes people's designs.
import { describe, expect, it } from 'vitest';
import { faceFillsToRects, layersToInclusive, type Layer, type Model } from './accessoryArt';

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
const edges = (l: Layer) => [l.x - l.w / 2, l.y - l.h / 2, l.x + l.w / 2, l.y + l.h / 2];

describe('layersToInclusive', () => {
  it('extends a layer 1 px short of its area to the edge', () => {
    expect(edges(layersToInclusive(model, [box(100, 100, 299, 299)])![0])).toEqual([100, 100, 300, 300]);
  });
  it('trims a layer 2 px past its area back to the edge (no line over the next face)', () => {
    expect(edges(layersToInclusive(model, [box(98, 100, 302, 300)])![0])).toEqual([100, 100, 300, 300]);
  });
  it('keeps a quarter-turned layer on its area, w and h swapped', () => {
    const l = layersToInclusive(model, [box(100, 100, 299, 300, { rot: 90, w: 200, h: 199 })])![0];
    expect([l.w, l.h, l.x]).toEqual([200, 200, 200]);
  });
  it('leaves layers that fit, are turned at an angle, or sit away from the edges', () => {
    expect(layersToInclusive(model, [box(100, 100, 300, 300)])).toBeNull();
    expect(layersToInclusive(model, [box(100, 100, 299, 299, { rot: 30 })])).toBeNull();
    expect(layersToInclusive(model, [box(150, 150, 250, 250)])).toBeNull();
  });
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
