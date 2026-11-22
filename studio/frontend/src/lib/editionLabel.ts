// The edition label (1st Edition, Silver, Gold, EX) on full-image cards: set.json "editionLabel" on the set and on a card,
// where it sits on the card and how the previews draw it. The mod (Patches/FullImageGradeLabelPatch) puts it at the game's
// own spot (read from the game files: App.EditionLabelLayout) moved by x / y % of the card and scaled by size.
import { App } from './api';
import { EventsOn } from '../../wailsjs/runtime/runtime';
import { Ev } from './events';
import { templateFile } from './urls';
import { loadAtlas, type SdfAtlas, type TmpFont } from './sdfText';
import { isFullImage } from './cardVersions';

export type LabelFields = { show?: boolean; x?: number; y?: number; size?: number };
/** A label as drawn: every field filled in. */
export type LabelChoice = { show: boolean; x: number; y: number; size: number };

/** What a set that says nothing gets (the mod's defaults; y chosen in game 2026-09-24). */
export const LABEL_DEFAULT: LabelChoice = { show: true, x: 0, y: -24.882, size: 1 };
/** Sideways and up / down ranges (% of the card from the game's spot; y: just above the bottom edge to the top). */
export const LABEL_X: [number, number] = [-50, 50];
export const LABEL_Y: [number, number] = [-40, 65];
export const LABEL_SIZE: [number, number] = [0.3, 4];

/** The set's label: its own fields, else the defaults. */
export const setLabel = (set: any): LabelChoice => ({ ...LABEL_DEFAULT, ...defined(set?.editionLabel) });
/** A card's label: its own fields, else the set's. */
export const cardLabel = (set: any, card: any): LabelChoice => ({ ...setLabel(set), ...defined(card?.editionLabel) });
/** Whether the card changes any field of the set's label. */
export const hasOwnLabel = (card: any) => !!card?.editionLabel && Object.keys(defined(card.editionLabel)).length > 0;

const defined = (f: LabelFields | undefined): LabelFields =>
  Object.fromEntries(Object.entries(f ?? {}).filter(([, v]) => v !== undefined && v !== null)) as LabelFields;

/** Sets one field on a set or card (undefined = leave it to the level above); an empty entry is left out. */
export function writeLabel(target: any, field: keyof LabelFields, value: boolean | number | undefined) {
  const next: Record<string, unknown> = { ...defined(target.editionLabel) };
  if (value === undefined) delete next[field]; else next[field] = value;
  target.editionLabel = Object.keys(next).length ? next : undefined;
}

/** The label a version shows, as the game names it; '' = none (Base and Full Art have no label). */
export function labelText(key: string): string {
  const b = key.replace(/_foil$/i, '').toLowerCase();
  return ({ firstedition: '1st Edition', silver: 'Silver', gold: 'Gold', ex: 'EX' } as Record<string, string>)[b] ?? '';
}

/**
 * The label a preview of this version draws, or undefined when the mod draws none there: framed sets (the game prints its
 * own in its frame), Base and Full Art, a set or card that hides it. card = undefined: the set's.
 */
export function labelOn(set: any, card: any, key: string) {
  const text = labelText(key);
  if (!text || !isFullImage(set)) return undefined;
  const choice = card ? cardLabel(set, card) : setLabel(set);
  return choice.show ? { text, choice } : undefined;
}

/** Where and how the game prints the label (editionlabel.json), with its font's atlas. */
export type LabelLayout = {
  /** The vanilla card as drawn (the previews' whole card), and the unit of x / y (% of offsetWidth × offsetHeight). */
  cardWidth: number; cardHeight: number; offsetWidth: number; offsetHeight: number;
  /** The card's centre from the card front's (x, + = right); the label's spot (x, y) is from the front's. */
  cardX: number;
  x: number; y: number; w: number; h: number;
  fontSize: number; autoSize: boolean; sizeMin: number; sizeMax: number; color: number[]; font: TmpFont; atlas: string;
};
export type LabelGame = { layout: LabelLayout; atlas: SdfAtlas };
// The game's label layout and font: loaded once, again after every read of the game files (a page opened while Studio was
// still reading them gets it when the read is done).
let game: LabelGame | null = null;
let loading = false;
const watchers = new Set<(g: LabelGame) => void>();
function load() {
  if (loading) return;
  loading = true;
  (async () => {
    const layout = (await App.EditionLabelLayout()) as unknown as LabelLayout | null;
    if (!layout?.font?.glyphs?.length || !layout.offsetHeight || layout.cardX === undefined) return; // an older read
    game = { layout, atlas: await loadAtlas(templateFile('frame', layout.atlas)) };
    for (const w of watchers) w(game);
  })().catch(() => {}).finally(() => (loading = false));
}
if (typeof window !== 'undefined' && (window as any).runtime) EventsOn(Ev.TemplatesReady, () => { game = null; if (watchers.size) load(); }); // not in unit tests
/** Calls back with the game's label layout and font now (when loaded) and after every new read; returns the unsubscribe. */
export function watchLabelLayout(cb: (g: LabelGame) => void): () => void {
  watchers.add(cb);
  if (game) cb(game); else load();
  return () => watchers.delete(cb);
}

/** The label's centre on the card as fractions of its width and height from the top left. */
export function labelCentre(lay: LabelLayout, c: LabelChoice): [number, number] {
  return [0.5 + (lay.x - lay.cardX + (c.x / 100) * lay.offsetWidth) / lay.cardWidth, 0.5 - (lay.y + (c.y / 100) * lay.offsetHeight) / lay.cardHeight];
}

/** The x / y (% of the card from the game's spot) that put the label's centre at these card fractions; clamped, 0.1 steps. */
export function labelOffset(lay: LabelLayout, fx: number, fy: number): [number, number] {
  const clamp = (v: number, [lo, hi]: [number, number]) => Math.round(Math.max(lo, Math.min(hi, v)) * 10) / 10;
  return [clamp((((fx - 0.5) * lay.cardWidth - lay.x + lay.cardX) / lay.offsetWidth) * 100, LABEL_X),
    clamp((((0.5 - fy) * lay.cardHeight - lay.y) / lay.offsetHeight) * 100, LABEL_Y)];
}

/** The text's width at font size 1, in the label font's letters (TMP advances). */
export function labelTextWidth(lay: LabelLayout, text: string): number {
  const f = lay.font;
  const glyphs = new Map(f.glyphs.map((g) => [g.u, g]));
  return [...text].reduce((s, ch) => s + (glyphs.get(ch.codePointAt(0)!)?.adv ?? f.glyphs[0].adv), 0) / f.pointSize;
}

/** The label's font size in card units: TMP's auto size in the game's box (biggest that fits, within its min / max). */
export function labelFontSize(lay: LabelLayout, text: string): number {
  if (!lay.autoSize) return lay.fontSize;
  const f = lay.font;
  const adv = labelTextWidth(lay, text);
  const tall = (f.ascent - f.descent) / f.pointSize;
  return Math.max(lay.sizeMin, Math.min(lay.sizeMax, lay.w / Math.max(adv, 1e-6), lay.h / Math.max(tall, 1e-6)));
}
