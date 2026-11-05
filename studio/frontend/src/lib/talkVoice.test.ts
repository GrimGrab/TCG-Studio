import { describe, expect, it } from 'vitest';
import { automaticVoice, LETTER_SAMPLES, PLAIN_VOICE, renderLine, SLOT_SAMPLES } from './talkVoice';

// A fake recording: each letter a steady level of its own, so the output shows which letter was read.
const letters = new Uint8Array(44 + 26 * LETTER_SAMPLES).fill(128);
for (let l = 0; l < 26; l++) letters.fill(130 + l * 4, 44 + l * LETTER_SAMPLES, 44 + (l + 1) * LETTER_SAMPLES);
const level = (l: number) => (130 + l * 4 - 128) / 128;
const voice = PLAIN_VOICE;

describe('talking sounds', () => {
  it('gives every character the same slot, on a strict grid', () => {
    expect(renderLine('ab c', voice, 'neutral', letters).length).toBe(4 * SLOT_SAMPLES);
  });

  it('reads each letter from its own recording, and spaces are silence', () => {
    const d = renderLine('b z', voice, 'neutral', letters);
    expect(d[10]).toBeCloseTo(level(1));
    expect(d[SLOT_SAMPLES + 10]).toBe(0);
    expect(d[2 * SLOT_SAMPLES + 10]).toBeCloseTo(level(25));
  });

  it('never reads past a letter into the next one, even at the highest pitch', () => {
    const d = renderLine('a', { ...voice, pitch: 3 }, 'neutral', letters);
    for (const v of d) expect(v === 0 || Math.abs(v - level(0)) < 1e-6).toBe(true);
  });

  it('speaks digits and other alphabets as letters', () => {
    expect(renderLine('2', voice, 'neutral', letters)[10]).toBeCloseTo(level('t'.charCodeAt(0) - 97));
    expect(renderLine('é', voice, 'neutral', letters)[10]).not.toBe(0);
  });

  it('mood changes the speed', () => {
    expect(renderLine('abc', voice, 'sad', letters).length).toBeGreaterThan(renderLine('abc', voice, 'angry', letters).length);
  });

  // A buzzy fake recording: brightness = how much of the energy is in fast changes, whatever the loudness.
  const buzzy = new Uint8Array(44 + 26 * LETTER_SAMPLES).map((_, i) => 128 + (i % 8 < 4 ? 40 : -40) + (i % 2 ? 20 : -20));
  const energy = (d: ArrayLike<number>) => Array.from(d).reduce((sum, v) => sum + v * v, 0);
  const brightness = (d: Float32Array) => Math.sqrt(energy(d.subarray(1).map((v, i) => v - d[i])) / energy(d));
  const flat = voice;

  it('tone leaves the middle alone and clearly changes both ends, step by step', () => {
    const at = (tone: number) => brightness(renderLine('aaaa', { ...flat, tone }, 'neutral', buzzy));
    expect(at(0)).toBeLessThan(at(0.25) * 0.8);
    expect(at(0.25)).toBeLessThan(at(0.5) * 0.8);
    expect(at(1)).toBeGreaterThan(at(0.5) * 1.2);
  });

  it('keeps its loudness across the tone range', () => {
    const peak = (tone: number) => Math.max(...renderLine('aaaa', { ...flat, tone }, 'neutral', buzzy).map(Math.abs));
    expect(peak(1)).toBeCloseTo(peak(0.5), 3);
    expect(peak(0.1)).toBeCloseTo(peak(0.5), 3);
  });

  it('gives different people different voices, the same every time, in their body\'s range', () => {
    expect(automaticVoice('a', false)).toEqual(automaticVoice('a', false));
    expect(automaticVoice('a', false)).not.toEqual(automaticVoice('b', false));
    expect(automaticVoice('a', false).pitch).toBeLessThan(automaticVoice('a', true).pitch);
  });
});
