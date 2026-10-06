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
    if (r) { report = lastReport = r; staleUnused = false; }
  }

  function done(r: any, what: string) {
    if (!r) return;
    const parts = [what];
    if (r.freed > 0) parts.push(`${size(r.freed)} freed.`);
    if (r.reinstalled > 0) parts.push(`${r.reinstalled} set${r.reinstalled === 1 ? '' : 's'} updated in the game.`);
    if (r.note) parts.push(r.note);
    notify(parts.join(' '), 'ok');
  }

  // Move everything: every setup's set files into the set's shared folder, accessory & furniture files into the shared
  // store, leftovers deleted. Files that differ from the shared ones wait for the player's choice (below).
  async function moveEverything() {
    const r = report;
    if (!(await ask(
      `Move everything to shared storage?\n\n` +
      `• ${r.moveFiles} files (card art, pack & box art, photos, accessory files) move from your setups into shared folders; ` +
      `copies already shared are stored once and leftovers nothing uses are deleted` +
      (r.moveSaves > 0 ? ` — about ${size(r.moveSaves)} freed.\n` : '.\n') +
      `• Afterwards your setups keep only their game info (names, prices, tiers) and saves.\n` +
      `• Files that differ from the shared ones are NOT touched: you decide for each set afterwards.\n` +
      `\nYour sets and items look the same. You can cancel at any time.`))) return;
    done(await run('Moving everything to shared…', () => App.MoveEverything()), 'Moved to shared storage.');
    if (alive) await measure();
  }

  // Sets whose files differ from the shared ones: the player says whether it's the same art or different art.
  let differPv = $state<Record<string, any>>({});
  let differName = $state<Record<string, string>>({});
  let keeping = $state(''); // set whose "keep both" name field is open
  const dkey = (d: any) => `${d.setup}/${d.set}`;

  async function loadDifferPreview(d: any) {
    try { differPv[dkey(d)] = await App.DifferPreview(d.setup, d.set); } catch (e) { notify(errText(e), 'error'); }
  }

  $effect(() => {
    for (const d of report?.differ ?? []) {
      const k = dkey(d);
      if (!(k in differPv)) { differPv[k] = null; loadDifferPreview(d); }
      if (!(k in differName)) differName[k] = `${d.setName} (${d.setupName})`;
    }
  });

  // A choice changes one setup's set: update the report here instead of measuring everything again. (On failure the page
  // re-measures. After "replace" the old shared files become unused, which only a new measure lists: the page says so.)
  let staleUnused = $state(false);
  function applied(d: any, r: any, choice: string, name = '') {
    if (!r) { if (alive) measure(); return; }
    report.differ = report.differ.filter((x: any) => dkey(x) !== dkey(d));
    const s = report.setups.find((x: any) => x.id === d.setup);
    if (s) {
      s.size -= d.bytes;
      s.cardArt -= d.cardBytes;
      s.setFiles -= d.bytes - d.cardBytes;
      const x = s.sets.find((x: any) => x.id === d.set);
      if (x) {
        x.own -= d.bytes;
        x.shared += d.bytes;
        x.differs = 0;
        if (choice === 'own') x.artFolder = name;
      }
    }
    if (choice === 'shared') report.workspace -= d.bytes;
    if (choice === 'replace') staleUnused = true;
  }

  async function useShared(d: any) {
    if (!(await ask(`Use the shared art for “${d.setName}” in ${d.setupName}?

` +
      `Its ${d.files} differing file${d.files === 1 ? '' : 's'} (${size(d.bytes)}) are deleted from ${d.setupName}; the set then ` +
      `shows the shared art, like your other setups.`))) return;
    const r = await run('Applying your choice…', () => App.ResolveDiffering(d.setup, d.set, 'shared', ''));
    done(r, `${d.setName} in ${d.setupName} now uses the shared art.`);
    applied(d, r, 'shared');
  }

  async function replaceShared(d: any) {
    if (!(await ask(`Make ${d.setupName}'s art the shared art for “${d.setName}”?

` +
      `Its ${d.files} differing file${d.files === 1 ? '' : 's'} replace the shared ones, so EVERY setup that uses the shared art of ` +
      `this set shows ${d.setupName}'s version from now on (setups with their own copies keep theirs). The old shared files then ` +
      `show up under “Not used by any setup”, where you can delete them.`))) return;
    const r = await run('Applying your choice…', () => App.ResolveDiffering(d.setup, d.set, 'replace', ''));
    done(r, `${d.setName}: ${d.setupName}'s art is now the shared art.`);
    applied(d, r, 'replace');
  }

  async function keepOwn(d: any) {
    const name = (differName[dkey(d)] ?? '').trim();
    if (!name) { notify('Give its shared folder a name first.', 'error'); return; }
    const r = await run('Applying your choice…', () => App.ResolveDiffering(d.setup, d.set, 'own', name));
    done(r, `${d.setName} in ${d.setupName} keeps its own art, now shared as “${name}”.`);
    keeping = '';
    applied(d, r, 'own', name);
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

  // Not used by any setup: one list whatever the kind.
  const KIND: Record<string, string> = {
    'set-art': 'Card art of a set no setup has',
    'set-files': 'Unused files in a set\'s shared folder',
    'game-art': 'Card art in the game folder',
    'shared-files': 'Accessory & furniture files',
    'catalog-set': 'Set kept only in the catalog',
    'catalog-item': 'Accessory / furniture kept only in the catalog',
  };
  const unusedTotal = $derived((report?.unused ?? []).reduce((n: number, u: any) => n + u.size, 0));

  async function deleteUnused(u: any) {
    const what = u.inCatalog
      ? `Delete “${u.name}” from the catalog?\n\nNo setup uses it; deleting removes it from the catalog (Add from catalog won't offer it) and frees its files.`
      : `Delete ${KIND[u.kind]?.toLowerCase() ?? 'these files'}: ${u.name} (${u.files} file${u.files === 1 ? '' : 's'}, ${size(u.size)})?\n\nNo setup uses ${u.files === 1 ? 'it' : 'them'}.`;
    if (!(await ask(what))) return;
    try {
      const freed = await App.DeleteUnused(u.kind, u.kind === 'shared-files' ? [] : [u.id]);
      notify(`Freed ${size(freed)}.`, 'ok');
    } catch (e) { notify(errText(e), 'error'); }
    if (alive) await measure();
  }

  async function deleteAllUnused() {
    const list = (report?.unused ?? []).filter((u: any) => !u.inCatalog);
    if (!list.length) return;
    if (!(await ask(`Delete everything no setup uses (${list.length} entr${list.length === 1 ? 'y' : 'ies'}, ${size(list.reduce((n: number, u: any) => n + u.size, 0))})?\n\n` +
      `Catalog entries are kept (delete those one by one).`))) return;
    let freed = 0;
    try {
      for (const kind of [...new Set(list.map((u: any) => u.kind))] as string[])
        freed += await App.DeleteUnused(kind, kind === 'shared-files' ? [] : list.filter((u: any) => u.kind === kind).map((u: any) => u.id));
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
    How much space everything takes, and ways to make it smaller. Card art, pack &amp; box art and accessory files are kept once in
    shared storage and used by every setup that has them; each setup keeps its game info (names, prices, tiers) and saves.
    Nothing here runs until you press its button.
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
        <div class="muted small">shared: sets {size(report.library)} · accessory files {size(report.assets)} · catalog {size(report.catalog)}</div></div>
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
      <h3>Move everything to shared</h3>
      {#if report.moveFiles > 0}
        <p class="small">{report.moveFiles} files (card art, pack &amp; box art, accessory files, leftovers) are still kept inside your
          setups. Moving them puts them in shared folders so setups keep only their game info{#if report.moveSaves > 0}, and frees
          about <b>{size(report.moveSaves)}</b>{/if}.</p>
        <div class="row"><button class="primary" onclick={moveEverything} disabled={!!running}>Move everything to shared…</button></div>
      {:else}
        <p class="small ok">✓ Everything that can be shared is shared. New imports go there too.</p>
      {/if}
      {#if report.differ?.length}
        <h4>Differs from the shared art — your choice</h4>
        <p class="muted small">These sets have their own copy of art that's also shared (imported separately, or changed by hand).
          <b>Click the one to keep</b> — or keep both.</p>
        {#each report.differ as d (dkey(d))}
          <div class="group differ">
            <div class="row"><b class="grow">{d.setName} <span class="muted">in {d.setupName}</span></b>
              <span class="muted small">{d.files} file{d.files === 1 ? '' : 's'}{d.cards ? ` (${d.cards} cards)` : ''} · {size(d.bytes)}</span></div>
            {#if differPv[dkey(d)]}
              <div class="choose">
                <button class="pick-img" onclick={() => useShared(d)} disabled={!!running}
                  title="Keep the shared art: {d.setupName} uses it too, its copies are deleted">
                  <img src={differPv[dkey(d)].shared} alt="" /><span>Shared</span></button>
                <button class="pick-img" onclick={() => replaceShared(d)} disabled={!!running}
                  title="Keep {d.setupName}'s art: it replaces the shared art for every setup using it">
                  <img src={differPv[dkey(d)].own} alt="" /><span>{d.setupName}'s</span></button>
              </div>
              <div class="row keepboth">
                {#if keeping === dkey(d)}
                  <span class="small">Save {d.setupName}'s as</span>
                  <input class="name" bind:value={differName[dkey(d)]} disabled={!!running} />
                  <button class="small" onclick={() => keepOwn(d)} disabled={!!running}>Save</button>
                  <button class="small link" onclick={() => (keeping = '')}>Cancel</button>
                {:else}
                  <button class="small link" onclick={() => (keeping = dkey(d))} disabled={!!running}
                    title="Keep both: {d.setupName}'s art is saved separately under its own name">or keep both — save {d.setupName}'s separately…</button>
                {/if}
              </div>
            {:else}<div class="muted small">Loading preview…</div>{/if}
          </div>
        {/each}
      {/if}
    </section>

    <section>
      <h3>Shrink card art (JPEG)</h3>
      <p class="muted small">Optional. JPEG card art is about 6× smaller than PNG; card scans lose a little detail. New imports can
        use JPEG too (Import → Card image format).</p>
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
      <h3>Setups</h3>
      <p class="muted small">What each setup's own folder holds, and everything it uses. Shared files are stored once however many
        setups use them.</p>
      {#each report.setups as s (s.id)}
        <div class="setup">
          <div class="row">
            <button class="link grow" onclick={() => (open[s.id] = !open[s.id])}>{open[s.id] ? '▾' : '▸'} {s.name}
              {#if s.active}<span class="badge ok">In the game</span>{/if}
              <span class="muted small">· {s.sets.length} set{s.sets.length === 1 ? '' : 's'} · {s.items.length} item{s.items.length === 1 ? '' : 's'}</span></button>
            <b class="num">{size(s.size)}</b>
          </div>
          <div class="parts small muted">
            {#each [['Game info', s.info], ['Card art kept here', s.cardArt], ['Pack & box art, photos', s.setFiles], ['Leftovers', s.leftovers],
              ['Accessory files kept here', s.itemFiles], ['Saves', s.saves], ['Game settings', s.game], ['Other', s.other]].filter((x) => x[1] > 0) as [label, n]}
              <span>{label} <b>{size(n as number)}</b></span>
            {/each}
          </div>
          {#if open[s.id]}
            <table>
              <thead><tr><th>Set / item</th><th class="num">Kept in this setup</th><th class="num">Shared</th></tr></thead>
              <tbody>
                {#each s.sets as x (x.id)}
                  <tr><td>{x.name} <span class="muted small">· {x.id}{x.artFolder ? ` · own art “${x.artFolder}”` : ''}{x.differs ? ` · ${x.differs} files differ` : ''}</span></td>
                    <td class="num">{size(x.own)}</td><td class="num muted">{size(x.shared)}</td></tr>
                {/each}
                {#each s.items as it (it.id)}
                  <tr><td>{it.name} <span class="muted small">· {it.kind}</span></td>
                    <td class="num">{size(it.own)}</td><td class="num muted">{size(it.shared)}</td></tr>
                {/each}
              </tbody>
            </table>
          {/if}
        </div>
      {/each}
    </section>

    <section>
      <div class="row"><h3 class="grow">Not used by any setup</h3>
        {#if report.unused.some((u: any) => !u.inCatalog)}<button class="small danger" onclick={deleteAllUnused} disabled={!!running}>Delete all…</button>{/if}</div>
      {#if staleUnused}<p class="small">The art you replaced is now unused — <button class="small link" onclick={measure}
        disabled={!!running}>Refresh</button> to list it here.</p>{/if}
      {#if report.unused.length}
        <p class="muted small">{size(unusedTotal)} in all. Entries “kept in the catalog” can still be added to a setup with Add from
          catalog; deleting them removes them from the catalog too.</p>
        {#each report.unused as u (u.kind + '/' + u.id)}
          <div class="row pick">
            <span class="grow">{u.name} <span class="muted small">· {KIND[u.kind] ?? u.kind} · {u.files} file{u.files === 1 ? '' : 's'}{u.note ? ` · ${u.note}` : ''}</span>
              {#if u.inCatalog}<span class="badge">kept in the catalog</span>{/if}</span>
            <span class="num">{size(u.size)}</span>
            <button class="small" onclick={() => deleteUnused(u)} disabled={!!running}>Delete…</button>
          </div>
        {/each}
      {:else}
        <p class="small ok">✓ Nothing stored that no setup uses.</p>
      {/if}
    </section>
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
  h4 { margin: 6px 0 0; font-size: 13px; }
  .setup { border-top: 1px solid var(--line); padding: 6px 0; display: flex; flex-direction: column; gap: 4px; }
  .parts { display: flex; flex-wrap: wrap; gap: 4px 14px; padding-left: 18px; }
  .choose { display: flex; gap: 14px; flex-wrap: wrap; }
  .pick-img { display: flex; flex-direction: column; align-items: center; gap: 6px; padding: 6px; height: auto; background: var(--bg);
    border: 2px solid var(--line); border-radius: 10px; cursor: pointer; transition: border-color 0.12s, transform 0.12s; }
  .pick-img img { width: 150px; border-radius: 6px; display: block; }
  .pick-img span { font-size: 12px; color: var(--muted); }
  .pick-img:hover:not(:disabled) { border-color: var(--accent); transform: translateY(-2px); }
  .pick-img:hover:not(:disabled) span { color: var(--text); }
  .pick-img:hover:not(:disabled) span::after { content: ' — keep this'; }
  .keepboth { gap: 8px; align-items: center; min-height: 28px; }
  button.link.small { font-weight: normal; color: var(--muted); text-decoration: underline; }
  input.name { width: 240px; }
  .badge { font-size: 11px; border: 1px solid var(--line); border-radius: 8px; padding: 0 6px; margin-left: 6px; }
  table { border-collapse: collapse; width: 100%; font-size: 13px; }
  th { text-align: left; font-weight: 600; color: var(--muted); font-size: 12px; border-bottom: 1px solid var(--line); padding: 4px 6px; }
  td { padding: 4px 6px; border-bottom: 1px solid var(--line); }
  button.link { background: none; border: none; padding: 0; color: var(--text); font-weight: 600; cursor: pointer; }
  button.link:disabled { cursor: default; opacity: 1; }
</style>
