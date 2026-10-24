import { describe, expect, it } from 'vitest';
import { matchSearch } from './search';

describe('matchSearch', () => {
  it('ignores accents, case and punctuation', () => {
    expect(matchSearch('pokemon', 'Pokémon TCG')).toBe(true);
    expect(matchSearch('POKÉMON', 'Pokémon TCG')).toBe(true);
    expect(matchSearch('yugioh', 'Yu-Gi-Oh!')).toBe(true);
    expect(matchSearch('weiss', 'Weiß Schwarz')).toBe(true);
    expect(matchSearch('flabebe', 'Flabébé')).toBe(true);
  });

  it('matches a game by its nicknames', () => {
    expect(matchSearch('mtg', 'Dominaria', 'Magic: The Gathering')).toBe(true);
    expect(matchSearch('ygo', 'Yu-Gi-Oh!')).toBe(true);
    expect(matchSearch('swu', 'Star Wars: Unlimited')).toBe(true);
    expect(matchSearch('mtg', 'Pokémon TCG')).toBe(false);
  });

  it('needs every word, in any order', () => {
    expect(matchSearch('mtg dominaria', 'Dominaria', 'Magic: The Gathering')).toBe(true);
    expect(matchSearch('dominaria mtg', 'Dominaria', 'Magic: The Gathering')).toBe(true);
    expect(matchSearch('mtg zendikar', 'Dominaria', 'Magic: The Gathering')).toBe(false);
  });

  it('never matches across two parts', () => {
    expect(matchSearch('k12', 'Shock', '12')).toBe(false);
    expect(matchSearch('12', 'Shock', '12')).toBe(true);
  });

  it('matches everything on an empty query and skips empty parts', () => {
    expect(matchSearch('  ', 'x')).toBe(true);
    expect(matchSearch('abc', undefined, null, '', 'ABC set')).toBe(true);
  });
});
