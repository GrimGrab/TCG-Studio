<script lang="ts" module>
  // Last view, kept while the page is closed so coming back shows it at once.
  let lastView: any = null;
</script>

<script lang="ts">
  // Storage: how much disk the workspace, every shared set and setup take, and the player's actions on shared storage.
  // Every set's files live once in <workspace>\library\<setId> (and <plugin>\Library in the game), accessory files once in
  // <workspace>\assets; each setup keeps only its game info and saves. Older workspaces get one "Upgrade storage" card.
  // Nothing here runs unless its button is pressed.
  import { onMount } from 'svelte';
  import { Ev } from '../lib/events';
  import { App, EventsOn, errText, ask } from '../lib/api';
  import { accFile, projectFile } from '../lib/urls';
  import { grouped, groupOf, sourceGames, OWN_GROUP } from '../lib/catalog';

  let { notify }: { notify: (t: string, k?: string) => void } = $props();

  let view = $state<any>(lastView);
  const report = $derived(view?.report);
  const up = $derived(view?.upgrade);
  let running = $state(''); // what's running ("" = idle)
  let alive = true; // false once the page is closed: a task it started then doesn't start a refresh behind the next page
  let adopted = false; // showing a task started before this page was opened (it reports through "storage:done")
  let prog = $state<any>(null);
  let stopping = $state(false);
  let pick = $state<Record<string, boolean>>({}); // shrink selection by set id

  const size = (n: number) => {
    if (!n) return '0 MB';
    const u = ['B', 'KB', 'MB', 'GB', 'TB'];
    let i = 0;
    while (n >= 1024 && i < u.length - 1) { n /= 1024; i++; }
    return `${n.toFixed(n >= 100 || i === 0 ? 0 : 1)} ${u[i]}`;
  };
  const pct = (p: any) => (p?.total ? Math.min(100, (100 * p.done) / p.total) : 0);
  const plural = (n: number, one: string, many = one + 's') => `${n} ${n === 1 ? one : many}`;

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

  async function measure(label = 'Measuring…') {
    const v = await run(label, () => App.StorageReport());
    if (v) view = lastView = v;
  }

  function done(r: any, what: string) {
    if (!r) return;
    const parts = [what];
    if (r.freed > 0) parts.push(`${size(r.freed)} freed.`);
    if (r.note) parts.push(r.note);
    notify(parts.join(' '), 'ok');
  }

  // ---------------------------------------------------------------- upgrade storage (older workspaces only)

  const upgradeNeeded = $derived(!!up && (up.files > 0 || up.differ.length > 0 || up.named.length > 0 || up.game.sets.length > 0 ||
    up.game.cardBack || up.game.shopLook.length > 0));
  const actionText: Record<string, string> = { move: 'moves to shared', duplicate: 'already shared — deleted here', leftover: 'unused — deleted' };
  let choice = $state<Record<string, string>>({}); // differ/named key → "shared" | "replace"
  let pv = $state<Record<string, any>>({}); // key → preview
  const choicesLeft = $derived(up ? [...up.differ, ...up.named].filter((d: any) => !choice[d.key]).length : 0);

  $effect(() => {
    if (!up) return;
    for (const d of up.differ) if (!(d.key in pv)) {
      pv[d.key] = null;
      App.UpgradePreviewDiffer(d.key).then((p: any) => (pv[d.key] = p)).catch((e: any) => notify(errText(e), 'error'));
    }
    for (const n of up.named) if (!(n.key in pv)) {
      pv[n.key] = null;
      App.UpgradePreviewNamed(n.set, n.name, n.sample).then((p: any) => (pv[n.key] = p)).catch((e: any) => notify(errText(e), 'error'));
    }
  });

  function fileGroups(list: any[]) {
    const out: { key: string; setup: string; owner: string; files: any[] }[] = [];
    for (const f of list) {
      const key = `${f.setup}/${f.owner}`;
      let g = out.find((x) => x.key === key);
      if (!g) out.push((g = { key, setup: f.setup, owner: f.owner, files: [] }));
      g.files.push(f);
    }
    return out;
  }

  async function upgrade() {
    const replaced = Object.values(choice).filter((c) => c === 'replace').length;
    if (!(await ask(
      `Upgrade storage?\n\n` +
      (up.files ? `• ${plural(up.files, 'file')} (${size(up.bytes)}) move into shared storage or are deleted as copies / unused.\n` : '') +
      (up.differ.length + up.named.length ? `• Your ${plural(up.differ.length + up.named.length, 'choice')} for art that differs` +
        (replaced ? ` (${replaced} replace the shared art for every setup using it)` : '') + `.\n` : '') +
      `• Afterwards each setup keeps only its game info and saves.\n\nYou can cancel at any time; what's done is kept.`))) return;
    const r = await run('Upgrading storage…', () => App.UpgradeStorage({ differ: pickChoices(up.differ), named: pickChoices(up.named) }));
    done(r, 'Storage upgraded.');
    choice = {};
    pv = {};
    if (alive) await measure();
  }
  const pickChoices = (list: any[]) => Object.fromEntries(list.map((d: any) => [d.key, choice[d.key]]));

  // ---------------------------------------------------------------- the game's old copies of set files

  async function removeGameCopies() {
    if (!(await ask(`Move the game's old copies of your content (${size(report.gameLibrary)}) out of the mod's folder?\n\n` +
      `The game reads everything from your TCG Studio workspace now. The copies go to a TCGStudio_backup folder in the game ` +
      `folder: delete it yourself once the game shows all your sets and items.`))) return;
    done(await run("Moving the game's copies…", () => App.RemoveGameCopies()), "The game's old copies are moved out of the mod's folder.");
    if (alive) await measure('Updating sizes…');
  }

  // ---------------------------------------------------------------- backup folders in the game folder

  const backupWhat = (b: any) => b.kind === 'install' ? 'Mod files replaced by Install / Repair' : "The game's old copies of your content";
  const backupWhen = (b: any) => new Date(b.time).toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' });

  async function deleteBackups(list: any[]) {
    const total = list.reduce((n, b) => n + b.bytes, 0);
    if (!(await ask((list.length === 1 ? `Delete the backup folder ${list[0].name} (${size(total)})?`
      : `Delete ${plural(list.length, 'backup folder')} (${size(total)})?`) +
      `\n\nThe game and TCG Studio don't use them. They can't be brought back afterwards.`))) return;
    done(await run('Deleting backup folders…', () => App.DeleteBackups(list.map((b) => b.name))), 'Backup folders deleted.');
    // Backups are outside the workspace: no other size on the page changes, so only the list is read again.
    const backups = await App.GameBackups();
    if (view?.report) {
      view.report.backups = backups;
      lastView = view;
    }
  }

  async function deleteKept() {
    if (!(await ask(`Delete the files the upgrade kept (${size(report.kept)})?

They can't be brought back afterwards.`))) return;
    done(await run('Deleting the kept files…', () => App.DeleteKept()), 'Kept files deleted.');
    if (alive) await measure();
  }

  // ---------------------------------------------------------------- mod save files

  async function compressSaves() {
    const p = report.plainSaves;
    if (!(await ask(`Compress ${plural(p.files, 'mod save file')}?

About ${size(p.bytes)} → ${size(p.after)}. The saves stay the same; ` +
      `the mod reads compressed files like before.`))) return;
    done(await run('Compressing mod save files…', () => App.CompressSaves()), 'Mod save files compressed.');
    if (alive) await measure();
  }

  // ---------------------------------------------------------------- shrink

  const pngSets = $derived((report?.sets ?? []).filter((x: any) => x.pngFiles > 0));
  const picked = $derived(Object.keys(pick).filter((k) => pick[k]));
  const pickedSize = $derived.by(() => {
    let before = 0, after = 0;
    for (const x of pngSets) if (pick[x.id]) { before += x.png; after += x.shrinkTo; }
    return { before, after };
  });

  // Before/after sample of one card (converted in memory).
  let preview = $state<any>(null); // { id, setName, ...ShrinkPreview }
  let previewing = $state('');
  let zoom = $state(false);

  async function showPreview(x: any) {
    previewing = x.id;
    try {
      preview = { id: x.id, setName: x.name, ...(await App.ShrinkPreview(x.id)) };
    } catch (e) { notify(errText(e), 'error'); }
    previewing = '';
  }

  // Show an example as soon as there is PNG art to shrink (tried once per page visit).
  let autoTried = false;
  $effect(() => {
    if (!report || preview || previewing || autoTried) return;
    autoTried = true;
    if (pngSets[0]) showPreview(pngSets[0]);
  });

  async function shrink() {
    const { before, after } = pickedSize;
    if (!(await ask(
      `Convert the card art of ${plural(picked.length, 'set')} to JPEG?\n\n` +
      `About ${size(before)} → ${size(after)}. Card scans lose a little detail (hard to see in game).\n` +
      `Every setup with these sets uses the converted art. Art you changed in Studio under another name is left alone.\n\n` +
      `To get PNG art back later, delete the set and import it again.`))) return;
    const ids = picked;
    pick = {};
    done(await run('Converting card art…', () => App.ShrinkSets(ids)), 'Card art converted to JPEG.');
    if (alive) await measure();
  }

  // ---------------------------------------------------------------- not used by any setup

  const KIND: Record<string, string> = {
    'set-art': 'Shared files of a set no setup has',
    'set-files': 'in its shared folder',
    'shared-files': 'not used by any accessory, furniture piece or decoration',
    'catalog-set': 'set',
    'catalog-item': 'item',
  };
  /** Shared storage as one number (on disk it is the library, assets and catalog folders). */
  const sharedBytes = $derived(report ? report.library + report.assets + report.catalog : 0);

  // Lists grouped by mod / import source like the Catalog page, A–Z inside a group. Files no set or item names get
  // their own groups (they have no origin).
  let games = $state<Record<string, string>>({}); // import source id → game
  const LOOSE: Record<string, string> = {
    'shared-files': 'Loose files (no accessory, furniture piece or decoration uses them)',
    'set-art': 'Folders of sets no setup or the catalog has',
  };
  const byName = (a: any, b: any) => (a.name || a.id).localeCompare(b.name || b.id);
  const groupsOf = (list: any[]) => grouped([...list].sort(byName), games, (e) => LOOSE[e.kind] ?? groupOf(e, games));
  const groupBytes = (list: any[], f: string) => list.reduce((n: number, x: any) => n + (x[f] ?? 0), 0);
  /** Workspace files outside shared storage, setups and the cache (e.g. old save backups). */
  const otherBytes = $derived(report ? Math.max(0, report.workspace - sharedBytes - report.setupsSize - report.cache) : 0);
  const unusedTotal = $derived((report?.unused ?? []).reduce((n: number, u: any) => n + u.size, 0));

  // Master–detail: one section at a time (remembered); "To do" only shows while something needs you.
  const NAV = [
    { id: 'overview', title: 'Overview', glyph: '📊' },
    { id: 'todo', title: 'To do', glyph: '⚠' },
    { id: 'shrink', title: 'Shrink card art', glyph: '🗜' },
    { id: 'setups', title: 'Setups', glyph: '🗂' },
    { id: 'shared', title: 'Shared storage', glyph: '📦' },
    { id: 'unused', title: 'Not used', glyph: '🧹' },
  ];
  let sec = $state((() => { try { return localStorage.getItem('storage.section') || 'overview'; } catch { return 'overview'; } })());
  $effect(() => { try { localStorage.setItem('storage.section', sec); } catch {} });
  const todo = $derived.by(() => {
    if (!report) return [] as string[];
    const t: string[] = [];
    if (upgradeNeeded) t.push('Finish the upgrade');
    if (report.gameFound && report.gameLibrary > 0 && !upgradeNeeded && report.modReadsLibrary) t.push(`Remove the game's old copies (${size(report.gameLibrary)})`);
    if (report.backups?.length) t.push(`${plural(report.backups.length, 'backup folder')}`);
    if (report.kept > 0) t.push(`Kept files (${size(report.kept)})`);
    if (report.plainSaves?.files) t.push(`Compress ${plural(report.plainSaves.files, 'save file')}`);
    return t;
  });
  $effect(() => { if (sec === 'todo' && report && !todo.length) sec = 'overview'; });
  const spaceParts = $derived(report ? [
    { label: 'Shared storage', bytes: sharedBytes, color: '#5b8cff' },
    { label: 'Setups & saves', bytes: report.setupsSize, color: '#3ecf8e' },
    { label: 'Download cache', bytes: report.cache, color: '#b66dff' },
    { label: 'Other', bytes: otherBytes, color: '#9aa4b2' },
  ].filter((x) => x.bytes > 0) : []);
  function navSub(id: string): string {
    if (!report) return '';
    switch (id) {
      case 'overview': return `${size(report.workspace)} in all`;
      case 'todo': return todo[0] ?? '';
      case 'shrink': return pngSets.length ? `${plural(pngSets.length, 'set')} with PNG art` : 'no PNG card art left';
      case 'setups': return plural(report.setups.length, 'setup');
      case 'shared': return size(sharedBytes);
      case 'unused': return report.unused.length ? `${report.unused.length} · ${size(unusedTotal)}` : 'nothing to clean up';
    }
    return '';
  }

  // The list in the same two tabs as Shared storage: sets (and unused files in a set's folder), items (and their files).
  const SET_KINDS = ['catalog-set', 'set-art', 'set-files'];
  let unusedTab = $state<'sets' | 'items'>('sets');
  const unusedOf = (tab: string) => (report?.unused ?? []).filter((u: any) => SET_KINDS.includes(u.kind) === (tab === 'sets'));
  const unusedSets = $derived(unusedOf('sets'));
  const unusedItems = $derived(unusedOf('items'));
  const unusedShown = $derived(unusedTab === 'sets' ? unusedSets : unusedItems);
  const bytesOf = (list: any[]) => list.reduce((n: number, u: any) => n + u.size, 0);
  const tabPicked = $derived(unusedShown.filter((u: any) => upick[ukey(u)]).length);
  function pickTab(on: boolean, list: any[] = unusedShown) {
    for (const u of list) upick[ukey(u)] = on;
  }
  const isImage = (u: any) => u.kind === 'shared-files' && u.note === 'Image';
  const IMAGE = /\.(png|jpe?g|webp|gif|bmp|svg)$/i;
  let filesOpen = $state<Record<string, boolean>>({}); // set-files entries showing their file list

  // Selection for "Delete selected" (by kind/id).
  const ukey = (u: any) => `${u.kind}/${u.id}`;
  let upick = $state<Record<string, boolean>>({});
  const unusedPicked = $derived((report?.unused ?? []).filter((u: any) => upick[ukey(u)]));

  /** Deletes unused entries (one kind per call to the backend). */
  async function deleteUnused(list: any[]) {
    if (!list.length) return;
    const total = list.reduce((n: number, u: any) => n + u.size, 0);
    const inCatalog = list.filter((u: any) => u.inCatalog).length;
    const what = list.length === 1
      ? (list[0].inCatalog
        ? `Delete “${list[0].name}” from the catalog?\n\nNo setup uses it; deleting removes it from the catalog (Add from catalog won't offer it) and frees its files.`
        : `Delete ${KIND[list[0].kind]?.toLowerCase() ?? 'these files'}: ${list[0].name} (${plural(list[0].files, 'file')}, ${size(list[0].size)})?\n\nNo setup uses ${list[0].files === 1 ? 'it' : 'them'}.`)
      : `Delete ${list.length} entries no setup uses (${size(total)})?` +
        (inCatalog ? `\n\n${inCatalog} of them are kept in the catalog: they are removed from the catalog too (Add from catalog won't offer them).` : '');
    if (!(await ask(what))) return;
    const r = await run('Deleting…', () => App.DeleteUnused(list.map((u: any) => ({ kind: u.kind, id: u.id, paths: u.paths ?? [] }))));
    upick = {};
    if (!r) { if (alive) await measure(); return; }
    // The list shows the result at once; the sizes elsewhere on the page follow from a fresh measure.
    const gone = new Set(r.deleted.map(ukey));
    if (view?.report) view.report.unused = view.report.unused.filter((u: any) => !gone.has(ukey(u)));
    notify(`Deleted ${plural(r.deleted.length, 'entry', 'entries')}, freed ${size(r.freed)}.`, 'ok');
    if (alive) await measure('Updating sizes…');
  }

  // ---------------------------------------------------------------- shared storage

  /** Every accessory, furniture piece and decoration once, with the setups using it. */
  const sharedItems = $derived.by(() => {
    const byId = new Map<string, any>();
    for (const s of report?.setups ?? [])
      for (const it of s.items) {
        const e = byId.get(it.id) ?? { ...it, usedBy: [] as string[] };
        e.usedBy.push(s.name);
        byId.set(it.id, e);
      }
    return [...byId.values()].sort((a, b) => b.shared - a.shared);
  });
  // A set's users are setups; "Catalog" only means the catalog keeps it — such a set is under "Not used by any setup".
  const setupsOf = (x: any) => x.usedBy.filter((n: string) => n !== 'Catalog');
  const sharedSets = $derived((report?.sets ?? []).filter((x: any) => setupsOf(x).length > 0));
  const sharedSetBytes = $derived(sharedSets.reduce((n: number, x: any) => n + x.shared, 0));
  const sharedItemBytes = $derived(sharedItems.reduce((n: number, x: any) => n + x.shared, 0));
  let sharedTab = $state<'sets' | 'items'>('sets');

  function cancel() {
    stopping = true;
    App.CancelStorage();
  }

  onMount(() => {
    sourceGames().then((g) => (games = g));
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') zoom = false; };
    window.addEventListener('keydown', onKey);
    const offs = [
      () => window.removeEventListener('keydown', onKey),
      EventsOn(Ev.StorageProgress, (p: any) => (prog = p)),
      // A task started before the page was opened ended: its result toast came from where it started; refresh here.
      EventsOn(Ev.StorageDone, () => {
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
  <header class="page-head">
    <div>
      <h2>Storage</h2>
      <div class="page-summary">
        {#if report}<span><b>{size(report.workspace)}</b> in this workspace</span>{#if todo.length}<span class="warn">{todo.length} thing{todo.length === 1 ? '' : 's'} to do</span>{/if}{/if}
        <span>Nothing here runs until you press its button.</span>
      </div>
    </div>
    <span class="grow"></span>
    <button onclick={() => measure()} disabled={!!running}>Refresh</button>
  </header>

  {#if running}
    <section class="progress strip">
      <div class="row"><span class="grow">{prog?.message || running}</span>
        {#if prog?.total}<span class="muted small">{prog.done} / {prog.total}</span>{/if}
        <button class="small" onclick={cancel} disabled={stopping}>{stopping ? 'Stopping…' : 'Cancel'}</button>
      </div>
      <div class="bar"><div class:indet={!prog?.total} style="width:{prog?.total ? pct(prog) : 30}%"></div></div>
    </section>
  {/if}


  {#if report}
  <div class="md">
    <nav class="nav sidelist">
      {#each NAV as n (n.id)}
        {#if n.id !== 'todo' || todo.length}
          <button class="item" class:active={sec === n.id} onclick={() => (sec = n.id)}>
            <span class="glyph">{n.glyph}</span>
            <span class="info"><span class="name">{n.title}</span><span class="sub">{navSub(n.id)}</span></span>
            {#if n.id === 'todo'}<span class="count warn">{todo.length}</span>{/if}
          </button>
        {/if}
      {/each}
    </nav>
    <div class="detail">
    {#if sec === 'overview'}
      <section class="block">
        <div class="bhead"><h2>Space</h2><span class="muted small">Card art, pack &amp; box art and accessory files are kept once in shared storage and
          used by every setup that has them; each setup keeps its game info and saves.</span></div>
        <div class="big">{size(report.workspace)} <span class="muted small">in this workspace</span></div>
        <div class="spacebar">
          {#each spaceParts as part (part.label)}
            <span style="flex:{Math.max(part.bytes, 1)}; background:{part.color}" title="{part.label}: {size(part.bytes)}"></span>
          {/each}
        </div>
        <div class="legend">
          {#each spaceParts as part (part.label)}
            <span><i style="background:{part.color}"></i>{part.label} <b>{size(part.bytes)}</b></span>
          {/each}
        </div>
        {#if report.gameFound && report.gameLibrary > 0}
          <p class="muted small">Plus <b>{size(report.gameLibrary)}</b> of old copies of your content in the game folder (the game no longer reads them).</p>
        {/if}
      </section>
    {#if report.gameFound && report.modReadsLibrary && !report.libraryLinked}
      <div class="notice">The game isn't reading this workspace: open <b>Game &amp; mod</b> and click Install / Repair, or restart TCG Studio.</div>
    {/if}
    {#if report.gameFound && !report.modReadsLibrary}
      <div class="notice">The mod in the game is older than this TCG Studio: open <b>Game &amp; mod</b> and click Install / Repair (TCG Studio
        also does it on start while the game is closed). Until then sets can't be installed, setups can't be switched, and the
        game's copies and mod save files are left as they are.</div>
    {/if}

      {#if todo.length}
        <button class="block tododash" onclick={() => (sec = 'todo')}>
          <span class="count warn">{todo.length}</span>
          <span class="grow"><b>Things to do</b><span class="muted small">{todo.join(' · ')}</span></span>
          <span class="muted">Open →</span>
        </button>
      {/if}
      {#if report.unused.length}
        <button class="block tododash" onclick={() => (sec = 'unused')}>
          <span class="count">{report.unused.length}</span>
          <span class="grow"><b>Not used by any setup</b><span class="muted small">{size(unusedTotal)} you could free</span></span>
          <span class="muted">Review →</span>
        </button>
      {/if}
    {:else if sec === 'todo'}
    {#if upgradeNeeded}
      <section class="block upgrade">
        <div class="bhead"><h2>Finish the upgrade</h2></div>
        <p class="small">TCG Studio moved this workspace's files into shared storage when it started. What's left below needs you:
          until then the game shows the shared art for these sets.</p>
        {#if up.files}
          <p class="small">{plural(up.files, 'file')} ({size(up.bytes)}) move into shared storage, or go as copies and unused leftovers.</p>
          <details class="movelist">
            <summary>Show the files</summary>
            {#each fileGroups(up.list) as g (g.key)}
              <div class="mg"><b>{g.owner}</b> <span class="muted">in {g.setup}</span></div>
              <table>
                <tbody>{#each g.files as f (f.file)}
                  <tr><td class="f">{f.file}</td><td class="muted">{size(f.bytes)}</td><td class={f.action}>{actionText[f.action] ?? f.action}</td></tr>
                {/each}</tbody>
              </table>
            {/each}
          </details>
        {/if}
        {#if up.differ.length || up.named.length}
          <h4>Art that differs from the shared art — click the one to keep</h4>
          <p class="muted small">Each set has one shared version of its art. Keeping another version makes it the shared art for every
            setup that has the set.</p>
          {#each [...up.differ.map((d: any) => ({ ...d, label: `${d.setName}`, where: `in ${d.setupName}`, own: `${d.setupName}'s` })),
                  ...up.named.map((n: any) => ({ ...n, label: `${n.setName}`, where: `“${n.name}”, used by ${n.usedBy.join(', ') || 'no setup'}`, own: `“${n.name}”` }))] as d (d.key)}
            <div class="group differ">
              <div class="row"><b class="grow">{d.label} <span class="muted">{d.where}</span></b>
                <span class="muted small">{plural(d.files, 'file')} · {size(d.bytes)}</span></div>
              {#if pv[d.key]}
                <div class="choose">
                  <button class="pick-img" class:on={choice[d.key] === 'shared'} onclick={() => (choice[d.key] = 'shared')} disabled={!!running}
                    title="Keep the shared art: this version's files are deleted">
                    {#if pv[d.key].shared}<img src={pv[d.key].shared} alt="" />{:else}<div class="noimg">no shared file</div>{/if}<span>Shared</span></button>
                  <button class="pick-img" class:on={choice[d.key] === 'replace'} onclick={() => (choice[d.key] = 'replace')} disabled={!!running}
                    title="Keep this version: it becomes the shared art for every setup using the set">
                    <img src={pv[d.key].own} alt="" /><span>{d.own}</span></button>
                </div>
              {:else}<div class="muted small">Loading preview…</div>{/if}
            </div>
          {/each}
        {/if}
        {#if up.game.sets.length || up.game.imported?.length || up.game.accessories || up.game.cardBack || up.game.shopLook.length || up.game.others?.length}
          <p class="muted small">Some content is still only in the game folder, where older TCG Studio versions put it. TCG Studio
            takes it over on its own when it starts while the game is closed.</p>
        {/if}
        <div class="row">
          <button class="primary" onclick={upgrade} disabled={!!running || choicesLeft > 0}>Upgrade storage…</button>
          {#if choicesLeft}<span class="muted small">Choose the art to keep for {plural(choicesLeft, 'set')} first.</span>{/if}
        </div>
      </section>
    {/if}

    {#if report.gameFound && report.gameLibrary > 0 && !upgradeNeeded && report.modReadsLibrary}
      <section class="block">
        <div class="bhead"><h2>Remove the game's copies of your content</h2></div>
        <p class="small">Older TCG Studio versions copied your content into the game folder
          (<code>BepInEx\plugins\TCGCustomCards</code>): every installed set with its card art, card backs and pack &amp; box art,
          every accessory's, furniture piece's and decoration's models and textures, the global card back and the Game look
          pictures. So each file was stored twice: once here and once in the game. The game now reads everything straight from
          your TCG Studio workspace (see Settings → Workspace) and the game folder only holds the mod itself, so those copies
          are no longer used: {size(report.gameLibrary)} you can free.</p>
        <p class="muted small">They're moved to a backup folder in the game folder, not deleted: it shows up under "Backup folders"
          below, where you can delete it once the game shows everything. Your sets, items, saves and TCG Studio's files are not
          touched. Close the game first.</p>
        <div class="row"><button class="primary" onclick={removeGameCopies} disabled={!!running}>Move the game's copies out…</button></div>
      </section>
    {/if}

    {#if report.backups?.length}
      <section class="block">
        <div class="bhead"><h2>Backup folders in the game folder</h2></div>
        <p class="small">TCG Studio never deletes files from the game folder: what Install / Repair replaces and what "Move the game's
          copies out" moves goes into a <code>TCGStudio_backup_…</code> folder there instead. The game and TCG Studio don't use them;
          delete them once everything works. Close the game first.</p>
        <table class="backups">
          <tbody>
            {#each report.backups as b (b.name)}
              <tr>
                <td>{backupWhen(b)}</td>
                <td>{backupWhat(b)}<div class="muted small"><code>{b.name}</code></div></td>
                <td class="num">{size(b.bytes)}</td>
                <td class="num">
                  <button onclick={() => App.OpenFolder(b.path)}>Open</button>
                  <button class="danger" onclick={() => deleteBackups([b])} disabled={!!running}>Delete…</button>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
        {#if report.backups.length > 1}
          <div class="row"><button class="danger" onclick={() => deleteBackups(report.backups)} disabled={!!running}>
            Delete all {report.backups.length} ({size(report.backups.reduce((n: number, b: any) => n + b.bytes, 0))})…</button></div>
        {/if}
      </section>
    {/if}

    {#if report.kept > 0}
      <section class="block">
        <div class="bhead"><h2>Kept by the upgrade</h2></div>
        <p class="small">When TCG Studio moved your files into shared storage, it didn't delete anything that wasn't an exact copy:
          images nothing pointed at, art you chose not to keep, and older card backs or Game look pictures it replaced went
          into a folder instead ({size(report.kept)}). Look through it, then delete it when you're sure.</p>
        <div class="row">
          <button onclick={() => App.OpenFolder(report.keptDir)}>Open folder</button>
          <button class="danger" onclick={deleteKept} disabled={!!running}>Delete kept files…</button>
        </div>
      </section>
    {/if}

    {#if report.plainSaves?.files}
      <section class="block">
        <div class="bhead"><h2>Compress mod save files</h2></div>
        <p class="small">{plural(report.plainSaves.files, 'save file')} of the mod (your custom cards, prices and decks per save slot) are
          still in the older, uncompressed form: {size(report.plainSaves.bytes)} → about {size(report.plainSaves.after)}. The game
          converts a slot only when it saves it; this does all of them at once, in every setup. Only the file size changes.</p>
        {#if report.plainSaves.gameFiles}<p class="muted small">{plural(report.plainSaves.gameFiles, 'file')} are in the game's save folder:
          those are done while the game is closed.</p>{/if}
        <div class="row"><button class="primary" onclick={compressSaves} disabled={!!running || (report.gameFound && !report.modReadsLibrary)}
          title={report.gameFound && !report.modReadsLibrary ? 'Update the mod in the game first: an older mod can\'t read compressed files' : ''}>
          Compress {plural(report.plainSaves.files, 'file')}…</button></div>
      </section>
    {/if}

      {#if !todo.length}<div class="notice ok">✓ Nothing to do.</div>{/if}
    {:else if sec === 'shrink'}
    <section class="block">
      <div class="bhead"><h2>Shrink card art (JPEG)</h2></div>
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
      {#if !pngSets.length}
        <p class="small ok">✓ No PNG card art left.</p>
      {:else}
        <div class="group">
          <div class="row"><span class="grow muted small">Sets with PNG card art</span>
            <button class="small" onclick={() => pngSets.forEach((x: any) => (pick[x.id] = true))}>Select all</button>
            <button class="small" onclick={() => (pick = {})}>None</button></div>
          {#each pngSets as x (x.id)}
            <label class="check pick"><input type="checkbox" bind:checked={pick[x.id]} />
              <span class="grow">{x.name} <span class="muted small">· {x.pngFiles} images · {setupsOf(x).join(', ') || 'no setup (only in the catalog)'}</span></span>
              <span class="num">{size(x.png)} → ≈ {size(x.shrinkTo)}</span>
              <button class="small" class:on={preview?.id === x.id} disabled={!!previewing}
                onclick={(e) => { e.preventDefault(); showPreview(x); }}>{previewing === x.id ? '…' : 'Preview'}</button></label>
          {/each}
        </div>
        <div class="row">
          <button class="primary" onclick={shrink} disabled={!!running || !picked.length}>Convert {picked.length || ''} selected to JPEG…</button>
          {#if picked.length}<span class="muted small">{size(pickedSize.before)} → ≈ {size(pickedSize.after)}</span>{/if}
        </div>
      {/if}
    </section>

    {:else if sec === 'setups'}
    <section class="block">
      <div class="bhead"><h2>Setups</h2></div>
      <p class="muted small">What each setup's own folder holds: its game info (names, prices, tiers), saves and game settings. The
        files of its sets and items are in shared storage (below), stored once however many setups use them.</p>
      {#each report.setups as s (s.id)}
        <div class="setup">
          <div class="row">
            <b class="grow">{s.name}
              {#if s.active}<span class="badge ok">In the game</span>{/if}
              <span class="muted small">· uses {plural(s.sets.length, 'set')} · {plural(s.items.length, 'item')}</span></b>
            <b class="num">{size(s.size)}</b>
          </div>
          <div class="parts small muted">
            {#each [['Game info', s.info], ['Saves', s.saves], ['Game settings', s.game], ['Other', s.other]].filter((x) => x[1] > 0) as [label, n]}
              <span>{label} <b>{size(n as number)}</b></span>
            {/each}
          </div>
        </div>
      {/each}
    </section>

    {:else if sec === 'shared'}
    <section class="block">
      <div class="bhead"><h2>Shared storage</h2></div>
      <p class="muted small">The sets and items your setups use: their files (card art, pack &amp; box art, models, textures) are
        stored once and shared by the setups listed. What no setup uses is under “Not used by any setup” below.</p>
      <div class="row tabs">
        <button class:on={sharedTab === 'sets'} onclick={() => (sharedTab = 'sets')}>Sets ({sharedSets.length}) · {size(sharedSetBytes)}</button>
        <button class:on={sharedTab === 'items'} onclick={() => (sharedTab = 'items')}>Accessories &amp; furniture ({sharedItems.length}) · {size(sharedItemBytes)}</button>
      </div>
      <div class="shared">
        <table>
          <thead><tr><th>{sharedTab === 'sets' ? 'Set' : 'Item'}</th><th>Used by</th><th class="num">Size</th></tr></thead>
          <tbody>
            {#each groupsOf(sharedTab === 'sets' ? sharedSets : sharedItems) as [g, list] (g)}
              <tr class="gh"><td colspan="3">{g || OWN_GROUP} <span class="muted small">· {list.length} · {size(groupBytes(list, 'shared'))}</span></td></tr>
              {#if sharedTab === 'sets'}
                {#each list as x (x.id)}
                  <tr><td><div class="nm">{#if x.cover}<img class="thumb" src={projectFile(x.id, x.cover)} alt="" loading="lazy" />{:else}<span class="thumb"></span>{/if}
                    {x.name} <span class="muted small">· {x.id}</span></div></td><td class="muted small">{setupsOf(x).join(', ')}</td>
                    <td class="num">{size(x.shared)}</td></tr>
                {/each}
              {:else}
                {#each list as it (it.id)}
                  <tr><td><div class="nm">{#if it.icon}<img class="thumb" src={accFile(it.icon)} alt="" loading="lazy" />{:else}<span class="thumb"></span>{/if}
                    {it.name} <span class="muted small">· {it.kind}</span></div></td><td class="muted small">{it.usedBy.join(', ')}</td>
                    <td class="num">{size(it.shared)}</td></tr>
                {/each}
              {/if}
            {/each}
          </tbody>
        </table>
      </div>
    </section>

    {:else if sec === 'unused'}
    <section class="block">
      <div class="bhead"><h2>Not used by any setup</h2><span class="grow"></span>
        {#if report.unused.length}
          <button class="small" onclick={() => pickTab(true)}>Select all</button>
          <button class="small" onclick={() => pickTab(false)} disabled={!tabPicked}>None</button>
          <button class="small danger" onclick={() => deleteUnused(unusedPicked)} disabled={!!running || !unusedPicked.length}>
            Delete {unusedPicked.length ? `${unusedPicked.length} selected` : 'selected'}…</button>
        {/if}</div>
      {#if report.unused.length}
        <p class="muted small">{size(unusedTotal)} in all. Sets and items here are only in the catalog: Add from catalog brings
          them back into a setup; deleting them removes them from the catalog too. With pictures: <b>Catalog</b> → tick
          “Only what no setup uses”.</p>
        <div class="row tabs">
          <button class:on={unusedTab === 'sets'} onclick={() => (unusedTab = 'sets')}>Sets ({unusedSets.length}) · {size(bytesOf(unusedSets))}</button>
          <button class:on={unusedTab === 'items'} onclick={() => (unusedTab = 'items')}>Accessories &amp; furniture ({unusedItems.length}) · {size(bytesOf(unusedItems))}</button>
        </div>
        <div class="shared">
          {#each groupsOf(unusedShown) as [g, list] (g)}
            <div class="row ghead small">
              <b class="grow">{g || OWN_GROUP} <span class="muted">· {list.length} · {size(groupBytes(list, 'size'))}</span></b>
              <button class="small link" onclick={() => pickTab(true, list)}>Select all</button>
              <button class="small link" onclick={() => pickTab(false, list)}>None</button>
            </div>
          {#each list as u (ukey(u))}
            <label class="row pick">
              <input type="checkbox" bind:checked={upick[ukey(u)]} />
              {#if isImage(u)}<img class="thumb" src={accFile(u.id)} alt="" loading="lazy" />
              {:else if u.icon}<img class="thumb" src={accFile(u.icon)} alt="" loading="lazy" />
              {:else if u.cover}<img class="thumb" src={projectFile(u.id, u.cover)} alt="" loading="lazy" />{/if}
              <span class="grow">{u.name} <span class="muted small">· {KIND[u.kind] ?? u.kind}{u.kind === 'shared-files' ? '' : ` · ${plural(u.files, 'file')}`}{u.note ? ` · ${u.note}` : ''}</span></span>
              <span class="num">{size(u.size)}</span>
              {#if u.paths?.length}
                <button class="small" onclick={(e) => { e.preventDefault(); filesOpen[ukey(u)] = !filesOpen[ukey(u)]; }}>
                  {filesOpen[ukey(u)] ? 'Hide files' : 'Show files'}</button>
              {/if}
              <button class="small" onclick={(e) => { e.preventDefault(); deleteUnused([u]); }} disabled={!!running}>Delete…</button>
            </label>
            {#if filesOpen[ukey(u)]}
              <div class="flist">
                {#each u.paths as f (f)}
                  <div class="row">
                    {#if IMAGE.test(f)}<img class="thumb" src={projectFile(u.id, f)} alt="" loading="lazy" />{:else}<span class="thumb"></span>{/if}
                    <span class="small">{f}</span>
                  </div>
                {/each}
              </div>
            {/if}
          {/each}
          {:else}
            <p class="small ok">✓ Nothing here that no setup uses.</p>
          {/each}
        </div>
      {:else}
        <p class="small ok">✓ Nothing stored that no setup uses.</p>
      {/if}
    </section>
    {/if}
    </div>
  </div>
  {:else}
    <p class="muted pad">Measuring…</p>
  {/if}
</div>

<style>
  .page { display: flex; flex-direction: column; height: 100%; overflow: hidden; }
  .strip { margin: 10px 24px 0; }
  section.progress { background: var(--panel); border: 1px solid var(--accent-2); border-radius: 10px; padding: 10px 14px; display: flex; flex-direction: column; gap: 8px; }
  section.upgrade { border-color: var(--accent-2); }
  .md { flex: 1; min-height: 0; display: flex; }
  .nav { width: 260px; flex-shrink: 0; border-right: 1px solid var(--line); padding: 8px; display: flex; flex-direction: column; gap: 2px; overflow: auto; }
  .glyph { font-size: 18px; width: 28px; text-align: center; flex: none; }
  .count { font-size: 11px; font-weight: 700; border-radius: 10px; padding: 1px 8px; background: var(--panel-2); color: var(--muted); }
  .count.warn { background: var(--warn); color: #2a1d00; }
  .detail { flex: 1; min-width: 0; overflow: auto; padding: 18px 24px 28px; display: flex; flex-direction: column; gap: 14px; }
  .pad { margin: 20px 24px; }
  .big { font-size: 30px; font-weight: 600; }
  .spacebar { display: flex; height: 14px; border-radius: 7px; overflow: hidden; gap: 2px; }
  .spacebar span { min-width: 3px; }
  .legend { display: flex; flex-wrap: wrap; gap: 6px 18px; font-size: 13px; }
  .legend span { display: inline-flex; align-items: center; gap: 6px; }
  .legend i { width: 10px; height: 10px; border-radius: 3px; display: inline-block; }
  .tododash { flex-direction: row; align-items: center; gap: 14px; text-align: left; cursor: pointer; }
  .tododash .grow { display: flex; flex-direction: column; gap: 2px; }
  .tododash:hover:not(:disabled) { border-color: var(--accent); }
  .small { margin: 0; }
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
  .movelist { font-size: 12px; }
  .movelist summary { cursor: pointer; color: var(--muted); }
  .movelist .mg { margin-top: 8px; }
  .movelist table { border-collapse: collapse; width: 100%; max-height: 320px; }
  .movelist td { padding: 1px 8px 1px 0; white-space: nowrap; }
  .movelist td.f { width: 100%; white-space: normal; word-break: break-all; }
  .movelist .duplicate, .movelist .leftover { color: var(--muted); }
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
  .tabs { gap: 6px; }
  .flist { padding-left: 46px; display: flex; flex-direction: column; gap: 2px; max-height: 300px; overflow: auto; }
  .flist .row { gap: 8px; align-items: center; }
  .thumb { flex: none; display: inline-block; width: 28px; height: 28px; object-fit: contain; border-radius: 4px; background: var(--bg); }
  .shared { max-height: 420px; overflow: auto; }
  .shared th { position: sticky; top: 0; background: var(--panel); }
  .shared tr.gh td { font-weight: 600; padding-top: 10px; border-bottom: 1px solid var(--line); }
  .shared .nm { display: flex; gap: 8px; align-items: center; }
  .ghead { gap: 8px; align-items: center; padding: 8px 0 2px; border-bottom: 1px solid var(--line); }
  .setup { border-top: 1px solid var(--line); padding: 6px 0; display: flex; flex-direction: column; gap: 4px; }
  .parts { display: flex; flex-wrap: wrap; gap: 4px 14px; padding-left: 18px; }
  .choose { display: flex; gap: 14px; flex-wrap: wrap; }
  .pick-img { display: flex; flex-direction: column; align-items: center; gap: 6px; padding: 6px; height: auto; background: var(--bg);
    border: 2px solid var(--line); border-radius: 10px; cursor: pointer; transition: border-color 0.12s, transform 0.12s; }
  .pick-img img, .pick-img .noimg { width: 150px; border-radius: 6px; display: block; }
  .pick-img .noimg { height: 210px; display: flex; align-items: center; justify-content: center; color: var(--muted); font-size: 12px; }
  .pick-img span { font-size: 12px; color: var(--muted); }
  .pick-img:hover:not(:disabled) { border-color: var(--accent); transform: translateY(-2px); }
  .pick-img.on { border-color: var(--accent); }
  .pick-img.on span { color: var(--text); }
  .pick-img.on span::after { content: ' — keep this'; }
  .badge { font-size: 11px; border: 1px solid var(--line); border-radius: 8px; padding: 0 6px; margin-left: 6px; }
  table { border-collapse: collapse; width: 100%; font-size: 13px; }
  th { text-align: left; font-weight: 600; color: var(--muted); font-size: 12px; border-bottom: 1px solid var(--line); padding: 4px 6px; }
  td { padding: 4px 6px; border-bottom: 1px solid var(--line); }
</style>
