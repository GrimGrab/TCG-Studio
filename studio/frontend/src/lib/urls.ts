// URLs of files the Go asset handler serves (app_files.go fileRoutes: keep both in step).

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
 * sign = shop look previews (file may be "sub/file"). */
export type TemplateKind = 'game' | 'acc' | 'furn' | 'deco' | 'sign';
const templatePrefix: Record<TemplateKind, string> = {
  game: '/templates/', acc: '/acctemplates/', furn: '/furntemplates/', deco: '/decotemplates/', sign: '/signtemplates/',
};

/** A file from the game templates Studio read from the game files. */
export function templateFile(kind: TemplateKind, file: string | undefined): string {
  return file ? templatePrefix[kind] + path(file) : '';
}

/** A picture or font chosen for the shop look (in <plugin>\ShopSign). */
export function shopSignFile(file: string, bust = 0): string {
  return `/shopsign/${encodeURIComponent(file)}${bustQuery(bust)}`;
}
