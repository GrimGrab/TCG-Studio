import { describe, expect, it } from 'vitest';
import { declares, guessKind, parseMessages, setCardValue, withCardValues } from './cardShaderView';

describe('card shader messages', () => {
  it('keeps the code line and names what is not declared', () => {
    const msgs = parseMessages("ERROR: 0:14: 'shards' : undeclared identifier\nERROR: 0:41: 'rgb' : field selection requires structure", 50);
    expect(msgs.map((m) => [m.line, m.unknown])).toEqual([[14, 'shards'], [41, undefined]]);
  });

  it('points a missing card() at the end of the code', () => {
    const msgs = parseMessages("ERROR: 0:9: 'card' : no matching overloaded function found", 3);
    expect(msgs[0].line).toBe(3);
    expect(msgs[0].text).toContain('vec4 card(vec2 uv)');
  });

  it("puts a card's own values in place of the shader's", () => {
    const params = [{ name: 'speed', kind: 'number' as const, value: [1] }, { name: 'pattern', kind: 'image' as const, image: 'assets/a.png' }];
    expect(withCardValues(params, { speed: [3], pattern: [9] })).toEqual([{ ...params[0], value: [3] }, params[1]]);
    expect(withCardValues(params, undefined)).toBe(params);
    expect(setCardValue(undefined, 'speed', [2])).toEqual({ speed: [2] });
    expect(setCardValue({ speed: [2] }, 'speed', undefined)).toBeUndefined();
  });

  it('sees the names the code makes itself', () => {
    const code = ['float glow(vec2 p) { return p.x; }', 'vec4 card(vec2 uv) {', '  float band = sin(uv.x); // float gone = 1.0;', '  return art(uv) * band * strength;', '}'].join('\n');
    expect(['band', 'glow', 'uv', 'p'].map((n) => declares(code, n))).toEqual([true, true, true, true]);
    expect(['strength', 'gone', 'ban'].map((n) => declares(code, n))).toEqual([false, false, false]);
  });

  it('guesses a parameter kind from how the code uses the name', () => {
    const code = 'vec4 card(vec2 uv) { vec4 a = art(uv + shift.xy * bend); return vec4(a.rgb + edge.rgb * edge.a * texture(pattern, uv).r * shine, a.a); }';
    expect(guessKind(code, 'bend')).toBe('number');
    expect(guessKind(code, 'shine')).toBe('number');
    expect(guessKind(code, 'shift')).toBe('vec2');
    expect(guessKind(code, 'edge')).toBe('colour');
    expect(guessKind(code, 'pattern')).toBe('image');
  });
});
