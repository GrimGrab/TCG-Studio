<script lang="ts" module>
  // Last report, kept while the page is closed so coming back shows it at once.
  let lastReport: any = null;
</script>

<script lang="ts">
  // Storage: how much disk the workspace, every setup and set take, and the player's actions on the shared card-art library.
  // A set's downloaded card art lives once in <workspace>\library (and <plugin>\Library in the game), shared by every setup
  // with that set; each setup keeps only its own files. Nothing here runs unless its button is pressed.
  import { onMount } from 'svelte';
  import { App, EventsOn, errText, ask } from '../lib/api';

  let { notify }: { notify: (t: string, k?: string) => void } = $props();

  let report = $state<any>(lastReport);
  let running = $state(''); // what's running ("" = idle)
  let alive = true; // false once the page is closed: a task it started then doesn't start a refresh behind the next page
  let adopted = false; // showing a task started before this page was opened (it reports through "storage:done")
  let prog = $state<any>(null);
  let stopping = $state(false);
  let open = $state<Record<string, boolean>>({});
  let pick = $state<Record<string, boolean>>({}); // shrink selection: "<setup>/<set>"

  const size = (n: number) => {
    if (!n) return '0 MB';
    const u = ['B', 'KB', 'MB', 'GB', 'TB'];
    let i = 0;
    while (n >= 1024 && i < u.length - 1) { n /= 1024; i++; }
    return `${n.toFixed(n >= 100 || i === 0 ? 0 : 1)} ${u[i]}`;
  };
  const pct = (p: any) => (p?.total ? Math.min(100, (100 * p.done) / p.total) : 0);

  async function run<T>(label: string, fn: () => Promise<T>): Promise<T | null> {
    running = label;
    stopping = false;
    prog = { message: label, done: 0, total: 0 };
    try {
      return await fn();
    } catch (e) {
      notify(stopping ? 'Stopped. What was done so far is kept.' : errText(e), stopping ? 'ok' : 'error');
      return null;
    } finally {
      running = '';
      prog = null;
    }
  }

  async function measure() {
    const r = await run('Measuring…', () => App.StorageReport());
    if (r) report = lastReport = r;
  }

  function done(r: any, what: string) {
    if (!r) return;
    const parts = [what];
    if (r.freed > 0) parts.push(`${size(r.freed)} freed.`);
    if (r.reinstalled > 0) parts.push(`${r.reinstalled} set${r.reinstalled === 1 ? '' : 's'} updated in the game.`);
    if (r.note) parts.push(r.note);
    notify(parts.join(' '), 'ok');
  }

  async function move() {
    const r = report;
    if (!(await ask(
      `Move card art into the shared library?\n\n` +
      `• ${r.moveFiles} card images move from your setups into one shared folder per set.\n` +
      (r.moveSaves > 0 ? `• Copies of the same art in several setups are stored once: about ${size(r.moveSaves)} freed.\n` : '') +
      `• Card art you changed in a setup stays that setup's own.\n` +
      (r.modHasLibrary ? `• The game keeps card art across setup switches, so switching copies far less.\n` : '') +
      `\nNothing is lost and your sets look the same. You can cancel at any time.`))) return;
    done(await run('Moving card art…', () => App.MoveToLibrary()), 'Card art moved to the shared library.');
    if (alive) await measure();
  }

  // Shrink: sets with PNG card art, per setup.
  const pngSets = (s: any) => s.sets.filter((x: any) => x.pngFiles > 0);
  const picked = $derived(Object.keys(pick).filter((k) => pick[k]));
  const pickedSize = $derived.by(() => {
    let before = 0, after = 0;
    for (const s of report?.setups ?? []) for (const x of s.sets) if (pick[`${s.id}/${x.id}`]) { before += x.png; after += x.shrinkTo; }
    return { before, after };
  });

  // Before/after sample of one card (converted in memory).
  let preview = $state<any>(null); // { key, setName, ...ShrinkPreview }
  let previewing = $state('');
  let zoom = $state(false);

  async function showPreview(s: any, x: any) {
    const key = `${s.id}/${x.id}`;
    previewing = key;
    try {
      preview = { key, setName: x.name, ...(await App.ShrinkPreview(s.id, x.id)) };
    } catch (e) { notify(errText(e), 'error'); }
    previewing = '';
  }

  // Show an example as soon as there is PNG art to shrink (tried once per page visit).
  let autoTried = false;
  $effect(() => {
    if (!report || preview || previewing || autoTried) return;
    autoTried = true;
    for (const s of report.setups) {
      const x = pngSets(s)[0];
      if (x) { showPreview(s, x); return; }
    }
  });

  function pickAll(s: any, on: boolean) {
    for (const x of pngSets(s)) pick[`${s.id}/${x.id}`] = on;
  }

  async function shrink() {
    const { before, after } = pickedSize;
    if (!(await ask(
      `Convert the card art of ${picked.length} set${picked.length === 1 ? '' : 's'} to JPEG?\n\n` +
      `About ${size(before)} → ${size(after)}. Card scans lose a little detail (hard to see in game).\n` +
      `Shared library art is converted once, for every setup that uses it. Art you changed in Studio is left alone.\n\n` +
      `To get PNG art back later, delete the set and import it again.`))) return;
    const bySetup: Record<string, string[]> = {};
    for (const k of picked) {
      const [s, id] = k.split('/');
      (bySetup[s] ??= []).push(id);
    }
    pick = {};
    done(await run('Converting card art…', () => App.ShrinkSets(bySetup)), 'Card art converted to JPEG.');
    if (alive) await measure();
  }

  async function deleteUnused(where: string, ids: string[]) {
    const what = ids.length === 1 ? `the card art of ${ids[0]}` : 'all unused card art';
    const place = where === 'game' ? ' in the game folder' : '';
    if (!(await ask(`Delete ${what}${place}?\n\nNo setup uses it. Importing the set again downloads its art again.`))) return;
    try {
      const freed = await App.DeleteUnusedArt(where, ids);
      notify(`Freed ${size(freed)}.`, 'ok');
    } catch (e) { notify(errText(e), 'error'); }
    if (alive) await measure();
  }

  function cancel() {
    stopping = true;
    App.CancelStorage();
  }

  onMount(() => {
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') zoom = false; };
    window.addEventListener('keydown', onKey);
    const offs = [
      () => window.removeEventListener('keydown', onKey),
      EventsOn('storage:progress', (p: any) => (prog = p)),
      // A task started before the page was opened ended: its result toast came from where it started; refresh here.
      EventsOn('storage:done', () => {
        if (!adopted) return;
        adopted = false;
        running = '';
        prog = null;
        measure();
      }),
    ];
    App.StorageTask().then((t: any) => {
      if (t.running) {
        adopted = true;
        running = t.label + '…';
        prog = t.progress;
      } else {
        measure();
      }
    }).catch(() => measure());
    return () => {
      alive = false;
      offs.forEach((off) => off());
    };
  });
</script>

<div class="page">
  <header class="row">
    <h2 class="grow">Storage</h2>
    <button onclick={measure} disabled={!!running}>Refresh</button>
  </header>
  <p class="muted intro">
    How much space your sets take, and ways to make them smaller. A set's card art is kept once in a shared library and used by
    every setup that has the set; each setup only keeps its own changes. Nothing here runs until you press its button.
  </p>

  {#if running}
    <section class="progress">
      <div class="row"><span class="grow">{prog?.message || running}</span>
        {#if prog?.total}<span class="muted small">{prog.done} / {prog.total}</span>{/if}
        <button class="small" onclick={cancel} disabled={stopping}>{stopping ? 'Stopping…' : 'Cancel'}</button>
      </div>
      <div class="bar"><div class:indet={!prog?.total} style="width:{prog?.total ? pct(prog) : 30}%"></div></div>
    </section>
  {/if}

  {#if report}
    <div class="tiles">
      <div class="tile"><div class="k">Workspace</div><div class="v">{size(report.workspace)}</div>
        <div class="muted small">{size(report.library)} of it is the shared card-art library</div></div>
      {#if report.gameFound}
        <div class="tile"><div class="k">Game folder</div><div class="v">{size(report.gameSets + report.gameLibrary)}</div>
          <div class="muted small">sets {size(report.gameSets)} · shared card art {size(report.gameLibrary)}</div></div>
      {/if}
    </div>
    {#if report.gameFound && !report.modHasLibrary}
      <div class="notice">The mod in the game is older and can't share card art between setups yet: open <b>Setup</b> and click
        Install / Repair. Until then everything keeps working as before.</div>
    {/if}

    <section>
      <h3>Shared card-art library</h3>
      {#if report.moveFiles > 0}
        <p class="small">{report.moveFiles} card images are still kept inside your setups.
          {#if report.moveSaves > 0}Moving them stores copies of the same art once and frees about <b>{size(report.moveSaves)}</b>.{/if}
          {#if report.modHasLibrary}Afterwards switching setups copies far less into the game.{/if}</p>
        <div class="row"><button class="primary" onclick={move} disabled={!!running}>Move card art to the shared library…</button></div>
      {:else}
        <p class="small ok">✓ All card art is in the shared library. New imports go there too, and importing a set another setup already has downloads nothing.</p>
      {/if}
    </section>

    <section>
      <h3>Shrink card art (JPEG)</h3>
      <p class="muted small">Optional. JPEG card art is about 6× smaller than PNG; card scans lose a little detail. New imports can
        use JPEG too (Import sets → Card image format).</p>
      {#if preview}
        <div class="preview">
          <div class="row"><span class="grow small"><b>{preview.card}</b> <span class="muted">· {preview.setName}</span></span>
            <button class="small" onclick={() => (zoom = true)}>Zoom</button></div>
          <div class="pair">
            <figure><button class="imgbtn" onclick={() => (zoom = true)} title="Zoom"><img src={preview.png} alt="PNG" /></button>
              <figcaption>PNG now · <b>{size(preview.pngSize)}</b></figcaption></figure>
            <figure><button class="imgbtn" onclick={() => (zoom = true)} title="Zoom"><img src={preview.jpeg} alt="JPEG" /></button>
              <figcaption>JPEG · <b>{size(preview.jpegSize)}</b>
              <span class="ok">({(preview.pngSize / Math.max(1, preview.jpegSize)).toFixed(1)}× smaller)</span></figcaption></figure>
          </div>
        </div>
        {#if zoom}
          <!-- Both cards as big as the window allows, side by side, no scrolling. Click or Esc closes. -->
          <div class="zoomview" role="presentation" onclick={() => (zoom = false)}>
            <figure><img src={preview.png} alt="PNG" /><figcaption>PNG now · {size(preview.pngSize)}</figcaption></figure>
            <figure><img src={preview.jpeg} alt="JPEG" /><figcaption>JPEG · {size(preview.jpegSize)}
              ({(preview.pngSize / Math.max(1, preview.jpegSize)).toFixed(1)}× smaller)</figcaption></figure>
            <div class="zoomhint">{preview.card} · click anywhere or press Esc to close</div>
          </div>
        {/if}
      {/if}
      {#if report.setups.every((s: any) => !pngSets(s).length)}
        <p class="small ok">✓ No PNG card art left.</p>
      {:else}
        {#each report.setups.filter((s: any) => pngSets(s).length) as s (s.id)}
          <div class="group">
            <div class="row"><b class="grow">{s.name}{#if s.active} <span class="badge ok">In the game</span>{/if}</b>
              <button class="small" onclick={() => pickAll(s, true)}>Select all</button>
              <button class="small" onclick={() => pickAll(s, false)}>None</button></div>
            {#each pngSets(s) as x (x.id)}
              <label class="check pick"><input type="checkbox" bind:checked={pick[`${s.id}/${x.id}`]} />
                <span class="grow">{x.name} <span class="muted small">· {x.pngFiles} images</span></span>
                <span class="num">{size(x.png)} → ≈ {size(x.shrinkTo)}</span>
                <button class="small" class:on={preview?.key === `${s.id}/${x.id}`} disabled={!!previewing}
                  onclick={(e) => { e.preventDefault(); showPreview(s, x); }}>{previewing === `${s.id}/${x.id}` ? '…' : 'Preview'}</button></label>
            {/each}
          </div>
        {/each}
        <div class="row">
          <button class="primary" onclick={shrink} disabled={!!running || !picked.length}>Convert {picked.length || ''} selected to JPEG…</button>
          {#if picked.length}<span class="muted small">{size(pickedSize.before)} → ≈ {size(pickedSize.after)}</span>{/if}
        </div>
      {/if}
    </section>

    <section>
      <h3>Setups and sets</h3>
      <table>
        <thead><tr><th>Setup / set</th><th class="num">Own files</th><th class="num">Shared card art used</th></tr></thead>
        <tbody>
          {#each report.setups as s (s.id)}
            <tr class="setup">
              <td><button class="link" onclick={() => (open[s.id] = !open[s.id])} disabled={!s.sets.length}>
                {s.sets.length ? (open[s.id] ? '▾' : '▸') : '·'} {s.name}</button>
                {#if s.active}<span class="badge ok">In the game</span>{/if}
                <span class="muted small">· {s.sets.length} set{s.sets.length === 1 ? '' : 's'}</span></td>
              <td class="num">{size(s.size)}</td>
              <td class="num muted">{size(s.sets.reduce((n: number, x: any) => n + x.library, 0))}</td>
            </tr>
            {#if open[s.id]}
              {#each s.sets as x (x.id)}
                <tr class="set"><td>{x.name} <span class="muted small">· {x.id}</span></td>
                  <td class="num">{size(x.own)}</td><td class="num muted">{size(x.library)}</td></tr>
              {/each}
            {/if}
          {/each}
        </tbody>
      </table>
      <p class="muted small">Shared card art is stored once however many setups use it.</p>
    </section>

    {#if report.unused.length || report.gameUnused.length}
      <section>
        <h3>Card art no setup uses</h3>
        <p class="muted small">Sets you deleted from every setup. Keeping their art makes importing them again instant.</p>
        {#each [['workspace', report.unused, 'Workspace'], ['game', report.gameUnused, 'Game folder']] as [where, list, title]}
          {#if (list as any[]).length}
            <div class="group">
              <div class="row"><b class="grow">{title}</b>
                <button class="small danger" onclick={() => deleteUnused(where as string, [])} disabled={!!running}>Delete all</button></div>
              {#each list as any[] as u (u.id)}
                <div class="row pick"><span class="grow">{u.id} <span class="muted small">· {u.files} files</span></span>
                  <span class="num">{size(u.size)}</span>
                  <button class="small" onclick={() => deleteUnused(where as string, [u.id])} disabled={!!running}>Delete</button></div>
              {/each}
            </div>
          {/if}
        {/each}
      </section>
    {/if}
  {/if}
</div>

<style>
  .page { padding: 20px 24px; overflow: auto; height: 100%; display: flex; flex-direction: column; gap: 14px; }
  .intro { max-width: 900px; margin: 0; }
  h3 { margin: 0; }
  section { background: var(--panel); border: 1px solid var(--line); border-radius: var(--radius); padding: 12px 14px; display: flex; flex-direction: column; gap: 8px; max-width: 980px; }
  section.progress { border-color: var(--accent-2); }
  .notice { max-width: 980px; padding: 10px 12px; border: 1px solid #6b5320; border-radius: var(--radius); background: var(--panel); }
  .small { font-size: 12px; margin: 0; }
  .ok { color: var(--ok); }
  .bar { height: 8px; background: var(--bg); border-radius: 4px; overflow: hidden; }
  .bar div { height: 100%; background: var(--accent); transition: width 0.2s; }
  .bar div.indet { animation: slide 1.2s ease-in-out infinite; }
  @keyframes slide { from { transform: translateX(-100%); } to { transform: translateX(340%); } }
  .tiles { display: grid; grid-template-columns: repeat(auto-fill, minmax(220px, 1fr)); gap: 8px; max-width: 980px; }
  .tile { background: var(--panel); border: 1px solid var(--line); border-radius: var(--radius); padding: 8px 10px; }
  .k { font-size: 12px; color: var(--muted); }
  .v { font-size: 20px; font-weight: 600; }
  .group { border: 1px solid var(--line); border-radius: var(--radius); padding: 8px 10px; display: flex; flex-direction: column; gap: 4px; }
  .pick { display: flex; gap: 8px; align-items: center; font-size: 13px; }
  .num { text-align: right; white-space: nowrap; }
  .preview { border: 1px solid var(--line); border-radius: var(--radius); padding: 8px 10px; display: flex; flex-direction: column; gap: 6px; }
  .pair { display: flex; gap: 12px; flex-wrap: wrap; }
  .pair figure { margin: 0; display: flex; flex-direction: column; gap: 4px; }
  .pair img { width: 220px; height: auto; border-radius: 6px; background: #000; }
  .imgbtn { padding: 0; border: none; background: none; cursor: zoom-in; display: block; }
  .zoomview { position: fixed; inset: 0; z-index: 200; background: rgba(0, 0, 0, 0.88); display: flex; align-items: center;
    justify-content: center; gap: 2vw; padding: 3vh 2vw 6vh; cursor: zoom-out; }
  .zoomview figure { margin: 0; display: flex; flex-direction: column; align-items: center; gap: 6px; min-width: 0; }
  .zoomview img { height: calc(100vh - 13vh); max-width: 47vw; width: auto; object-fit: contain; border-radius: 10px; background: #000; }
  .zoomview figcaption { color: #ddd; font-size: 14px; }
  .zoomhint { position: absolute; bottom: 1.5vh; left: 0; right: 0; text-align: center; color: #aaa; font-size: 12px; }
  .pair figcaption { font-size: 12px; color: var(--muted); }
  button.on { border-color: var(--accent); }
  table { border-collapse: collapse; width: 100%; font-size: 13px; }
  th { text-align: left; font-weight: 600; color: var(--muted); font-size: 12px; border-bottom: 1px solid var(--line); padding: 4px 6px; }
  td { padding: 4px 6px; border-bottom: 1px solid var(--line); }
  tr.set td:first-child { padding-left: 28px; }
  button.link { background: none; border: none; padding: 0; color: var(--text); font-weight: 600; cursor: pointer; }
  button.link:disabled { cursor: default; opacity: 1; }
</style>
