// URLs of files the Go asset handler serves (app_files.go fileRoutes: keep both in step).
import { EventsOn } from '../../wailsjs/runtime/runtime';
import { Ev } from './events';

const path = (rel: string) => rel.split('/').map(encodeURIComponent).join('/');
const bustQuery = (bust?: number) => (bust ? `?v=${bust}` : '');

/** A file a set refers to ("images/1.png"), from the set's shared folder. */
export function projectFile(projectId: string, rel: string | undefined, bust = 0): string {
  if (!rel) return '';
  return `/proj/${encodeURIComponent(projectId)}/${path(rel)}${bustQuery(bust)}`;
}

/** An accessory, furniture or decoration file ("assets/<hash>.png") from the shared store. */
export function accFile(rel: string | undefined, bust = 0): string {
  return rel ? `/acc/${path(rel)}${bustQuery(bust)}` : '';
}

/** Game template folders: game = pack/box textures, acc = accessory models, furn = furniture, deco = decorations,
 * sign = shop look previews (file may be "sub/file"), frame = the game's card rims. */
type TemplateKind = 'game' | 'acc' | 'furn' | 'deco' | 'sign' | 'frame';
const templatePrefix: Record<TemplateKind, string> = {
  game: '/templates/', acc: '/acctemplates/', furn: '/furntemplates/', deco: '/decotemplates/', sign: '/signtemplates/', frame: '/frametemplates/',
};

// When Studio last finished reading the game files (0 = not during this run). Template URLs carry it, so every cache of
// template files (models, textures, outlines) misses after a new read and the new files show without a restart. Registered
// when this module loads, before any page's own TemplatesReady handler, so those already build the new URLs.
let templatesRead = 0;
if (typeof window !== 'undefined' && (window as any).runtime) EventsOn(Ev.TemplatesReady, () => { templatesRead = Date.now(); }); // not in unit tests
const templateQuery = () => (templatesRead ? `?t=${templatesRead}` : '');

/** A file from the game templates Studio read from the game files. */
export function templateFile(kind: TemplateKind, file: string | undefined): string {
  return file ? templatePrefix[kind] + path(file) + templateQuery() : '';
}

/** The game's portrait of a customer character (empty when Studio hasn't read the game files yet). */
export function customerPortrait(female: boolean, model: number): string {
  return model >= 0 ? `/custtemplates/${female ? 'female' : 'male'}_${model}.png${templateQuery()}` : '';
}

/** A picture or font chosen for the shop look (in <plugin>\ShopSign). */
export function shopSignFile(file: string, bust = 0): string {
  return `/shopsign/${encodeURIComponent(file)}${bustQuery(bust)}`;
}

/** A font of App.ShopSignFonts / App.PickFontFile, for previews (app_fonts.go routes; '' for the built-in font). */
export function fontFile(f: { source: string; id: string }): string {
  if (f.source === 'game') return templateFile('sign', f.id);
  // The id is the font's full Windows path; the route takes only its file name (app_fonts.go winFontFile).
  if (f.source === 'windows') return `/winfonts/${encodeURIComponent(f.id.split(/[\\/]/).pop() ?? '')}`;
  if (f.source === 'file') return `/pickedfont/${encodeURIComponent(f.id)}`;
  return '';
}
