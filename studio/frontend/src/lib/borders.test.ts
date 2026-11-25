import { describe, expect, it } from 'vitest';
import { anySeeThrough, clampBox, setBoxSide, windowOf } from './borders';

describe('card picture box', () => {
  it('stays inside the card and keeps a minimum size', () => {
    expect(clampBox([0.9, 0.5, 0.3, 0.7])).toEqual([0.7, 0.3, 0.3, 0.7]);
    expect(clampBox([-0.2, 0, 0.01, 2])).toEqual([0, 0, 0.05, 1]);
  });
  it('typing one number never moves the others', () => {
    expect(setBoxSide([0.5, 0.1, 0.4, 0.6], 2, 0.7)).toEqual([0.5, 0.1, 0.5, 0.6]); // width capped, left kept
    expect(setBoxSide([0.5, 0.1, 0.4, 0.6], 0, 0.9)).toEqual([0.6, 0.1, 0.4, 0.6]); // left capped, width kept
    expect(setBoxSide([0.5, 0.1, 0.4, 0.6], 3, 0)).toEqual([0.5, 0.1, 0.4, 0.05]); // minimum size
  });
  it('no box = the whole card', () => {
    expect(windowOf({ id: 'a', name: '', layers: [] })).toEqual([0, 0, 1, 1]);
    expect(windowOf({ id: 'a', name: '', layers: [], window: [0.1, 0.1, 0.8, 0.6] })).toEqual([0.1, 0.1, 0.8, 0.6]);
  });
});

const px = (...alphas: number[]) => alphas.flatMap((a) => [200, 150, 50, a]);

describe('anySeeThrough', () => {
  it('finds a see-through or soft pixel', () => {
    expect(anySeeThrough(px(255, 255, 0, 255))).toBe(true);
    expect(anySeeThrough(px(255, 128))).toBe(true);
  });
  it('an opaque picture (a JPG) has none', () => {
    expect(anySeeThrough(px(255, 255, 255))).toBe(false);
    expect(anySeeThrough(px(252))).toBe(false);
  });
});
