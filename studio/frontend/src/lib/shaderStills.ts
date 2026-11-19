// Still frames of card shaders for the Cards grid: one hidden preview (lib/cardShaderView) draws each card in turn and
// keeps a small picture of it. Live previews per card would be far too slow (and browsers cap WebGL contexts); the card
// panel shows the shader live.
import { App } from './api';
import { accFile } from './urls';
import { CardShaderView, withCardValues, type CardFacts, type ShaderLook, type ShaderParam, type Wrapping } from './cardShaderView';

export type StillJob = {
  key: string; // what the picture depends on: the same key = the same picture
  code: string; params: ShaderParam[]; look: ShaderLook;
  art: string; mask: string; own?: Record<string, number[]>; facts: CardFacts;
};

const WIDTH = 240;
const MAX_KEPT = 600;
const stills = new Map<string, string>(); // key → picture (data URL); '' = the code doesn't compile
const wrappings = new Map<string, Promise<Wrapping>>();
let view: CardShaderView | null = null;
let queue: Promise<unknown> = Promise.resolve();

/** A picture made earlier for this key (undefined = not made yet; '' = the shader can't be shown). */
export const stillFor = (key: string): string | undefined => stills.get(key);

/**
 * Makes the picture for a job, one at a time. `wanted` is asked again just before drawing, so jobs the page no longer
 * needs (another version picked, the set closed) are skipped.
 */
export function makeStill(job: StillJob, wanted: () => boolean): Promise<string | undefined> {
  const run = queue.then(async () => {
    if (stills.has(job.key)) return stills.get(job.key);
    if (!wanted()) return undefined;
    const still = await draw(job).catch(() => '');
    if (stills.size >= MAX_KEPT) stills.delete(stills.keys().next().value!);
    stills.set(job.key, still);
    return still;
  });
  queue = run.catch(() => undefined);
  return run;
}

function wrappingFor(params: ShaderParam[], look: ShaderLook): Promise<Wrapping> {
  const shape = JSON.stringify([params.map((p) => [p.name, p.kind]), look.mix ?? '', look.mixStrength ?? 0]);
  let w = wrappings.get(shape);
  if (!w) {
    w = App.ShaderWrapping(params.map((p) => ({ name: p.name, kind: p.kind })) as any, look.mix ?? '', look.mixStrength ?? 0) as Promise<Wrapping>;
    wrappings.set(shape, w);
    w.catch(() => wrappings.delete(shape));
  }
  return w;
}

async function draw(job: StillJob): Promise<string> {
  if (!view) {
    const canvas = document.createElement('canvas');
    canvas.width = WIDTH;
    canvas.height = Math.round(WIDTH * 1.4);
    view = new CardShaderView(canvas);
  }
  const wrapping = await wrappingFor(job.params, job.look);
  view.draws = job.look.draws ?? '';
  view.facts = { ...job.facts };
  if (view.setCode(job.code, wrapping).length) return '';
  view.setParams(JSON.parse(JSON.stringify(withCardValues(job.params, job.own))), (p) => accFile(p.image));
  view.setArt(job.art);
  view.setMask(job.mask);
  // A picture that didn't load would leave the previous card's in the texture: no still then (the tile shows the plain picture).
  if (!(await view.idle())) return '';
  view.freeze(1.5); // a moment into the effect, the same for every card
  return view.snapshot(WIDTH, 'image/webp', 0.85);
}
