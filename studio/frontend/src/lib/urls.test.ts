import { describe, expect, it } from 'vitest';
import { fontFile } from './urls';

describe('fontFile', () => {
  // The Go route only serves a bare file name (it refuses paths with "C:\" in them): a Windows font's full path must be
  // cut down to its file name, whichever slash the path uses.
  it('sends only the file name of a Windows font', () => {
    expect(fontFile({ source: 'windows', id: 'C:\\WINDOWS\\Fonts\\arial.ttf' })).toBe('/winfonts/arial.ttf');
    expect(fontFile({ source: 'windows', id: 'C:\\Users\\me\\AppData\\Local\\Microsoft\\Windows\\Fonts\\Almendra-Regular.ttf' }))
      .toBe('/winfonts/Almendra-Regular.ttf');
    expect(fontFile({ source: 'windows', id: 'C:/fonts/My Font.otf' })).toBe('/winfonts/My%20Font.otf');
  });

  it('builds the game and picked-file routes', () => {
    expect(fontFile({ source: 'game', id: 'fonts/ARIAL.ttf' })).toBe('/signtemplates/fonts/ARIAL.ttf');
    expect(fontFile({ source: 'file', id: '0' })).toBe('/pickedfont/0');
    expect(fontFile({ source: '', id: '' })).toBe('');
  });
});
