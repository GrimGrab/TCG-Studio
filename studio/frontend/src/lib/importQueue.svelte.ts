// Import queue: sets imported one after another (Go runs one import at a time). Lives outside the Import page so the
// queue keeps going while the player looks at other pages; the page shows it and adds to it.
import { App, EventsOn, errText } from './api';
import { Ev } from './events';
import { smartArtForProject } from './smartArt';

export type ImportJob = {
  key: string; // source + code + lang: one job per set
  source: string;
  code: string;
  name: string; // shown in the queue ("MH3", or the folder set's name)
  cards: number;
  opt: any; // importer.Options, complete (lang, useLibrary, setName…)
  ownArt: boolean; // source brings its own pack art (EPL): no smart generate
  open: boolean; // open the set when done (a single import with nothing queued after it)
};

type Notify = (t: string, k?: string) => void;

export const queue = $state({
  jobs: [] as ImportJob[], // waiting, in order
  current: null as string | null, // name of the running import (also set by the EPL import, which doesn't queue)
  key: '', // jobKey of the running queue job
  progress: null as any,
  finished: 0, // bumped after every job, so the Import page reloads its list
});

let listening = false;
export function listen() {
  if (listening) return;
  listening = true;
  EventsOn(Ev.ImportProgress, (p: any) => (queue.progress = p));
}

export const jobKey = (source: string, code: string, lang: string) => `${source}|${code}|${lang}`;
export const isQueued = (key: string) => queue.jobs.some((j) => j.key === key);

let notify: Notify = () => {};
let open: (id: string) => void = () => {};

// Adds jobs to the end of the queue (skips sets already waiting or running) and starts it if idle.
export function enqueue(jobs: ImportJob[], n: Notify, o: (id: string) => void) {
  listen();
  notify = n;
  open = o;
  for (const j of jobs) if (!isQueued(j.key) && j.key !== queue.key) queue.jobs.push(j);
  if (!queue.current) run();
}

export function unqueue(key: string) {
  queue.jobs = queue.jobs.filter((j) => j.key !== key);
}

// Cancel the running import; with all, the waiting ones too.
export function cancel(all: boolean) {
  if (all) queue.jobs = [];
  App.CancelImport();
}

async function run() {
  while (queue.jobs.length) {
    const job = queue.jobs.shift()!;
    queue.key = job.key;
    queue.current = job.name;
    queue.progress = { stage: 'cards', done: 0, total: job.cards, message: 'Starting…' };
    try {
      const id = await App.ImportSet(job.source, job.code, job.opt);
      const done = queue.progress?.message ?? 'Imported';
      // Pack & box art from the set's product photos (or its best card art); the import's simple art stays if this fails.
      if (!job.ownArt) try {
        const p: any = await App.LoadProject(id);
        await smartArtForProject(p, (m) => (queue.progress = { ...queue.progress, stage: 'art', message: m }));
        await App.SaveProject(p);
      } catch (e) { notify(`Pack art for ${job.name}: ` + errText(e) + ' — kept the simple generated art', 'error'); }
      notify(done, 'ok');
      if (job.open && !queue.jobs.length) open(id);
    } catch (e) {
      notify(`${job.name}: ` + errText(e), 'error');
    }
    queue.finished++;
  }
  queue.key = '';
  queue.current = null;
  queue.progress = null;
}
