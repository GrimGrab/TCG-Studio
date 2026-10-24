<script lang="ts">
  // Setups: complete studio workspaces (sets, accessory library, mod settings, card back) that can be swapped in and out of
  // the game. Each setup keeps its own game saves; exports (.tcgsetup) never contain saves.
  import { onMount } from 'svelte';
  import { Ev } from '../lib/events';
  import { App, EventsOn, errText, ask } from '../lib/api';

  let { notify, onswitched, action = null, ondone = () => {} }: {
    notify: (t: string, k?: string) => void; onswitched: () => void;
    action?: { create?: boolean; switchTo?: string } | null; ondone?: () => void;
  } = $props();

  let view = $state<any>(null);
  let loading = $state(true);
  let busy = $state('');          // blocking overlay text (switch / export / import)
  let exportProgress = $state(''); // "12 / 340 files"

  // New setup form
  let creating = $state(false);
  let newName = $state('');
  let newFrom = $state('');
  let newSaves = $state(false);

  // Edit form
  let editId = $state('');
  let editName = $state('');
  let editDesc = $state('');

  // Import preview
  let pick = $state<any>(null);
  let menuFor = $state(''); // the setup card whose ⋯ menu is open
  let moreInfo = $state(false); // the intro's "Sharing and saves"

  async function load() {
    try {
      view = await App.ListSetups();
      if (view.message) notify(view.message, 'ok');
    } catch (e) { notify(errText(e), 'error'); }
    loading = false;
  }

  async function retry() {
    try { view = await App.RetrySetups(); onswitched(); } catch (e) { notify(errText(e), 'error'); }
  }

  function startNew(from = '') {
    creating = true;
    newFrom = from;
    newSaves = false;
    const src = view?.setups.find((s: any) => s.id === from);
    newName = src ? src.name + ' copy' : '';
  }

  async function create() {
    try {
      view = await App.CreateSetup(newName.trim() || 'New setup', newFrom, newSaves);
      creating = false;
      notify('Setup created.', 'ok');
    } catch (e) { notify(errText(e), 'error'); }
  }

  function startEdit(s: any) {
    editId = s.id;
    editName = s.name;
    editDesc = s.description ?? '';
  }

  async function saveEdit() {
    try {
      view = await App.UpdateSetup(editId, editName, editDesc);
      editId = '';
      onswitched(); // refreshes the setup name in the sidebar
    } catch (e) { notify(errText(e), 'error'); }
  }

  async function move(id: string, to: number) {
    try { view = await App.MoveSetup(id, to); } catch (e) { notify(errText(e), 'error'); }
  }

  async function remove(s: any) {
    const saves = s.hasSaves ? '\n\nIts game saves are deleted too — they exist nowhere else.' : '';
    if (!(await ask(`Delete the setup "${s.name}" with its settings and its own prices, tiers and changes?${saves}\n\nIts sets, accessories and furniture stay in the shared catalog, so your other setups can still add them. This can't be undone.`))) return;
    try {
      view = await App.DeleteSetup(s.id);
      notify(`Deleted "${s.name}".`, 'ok');
    } catch (e) { notify(errText(e), 'error'); }
  }

  async function switchTo(s: any) {
    const cur = view.setups.find((x: any) => x.active);
    const msg = `Switch the game to "${s.name}"?\n\n` +
      `"${cur?.name ?? 'The current setup'}" is taken out of the game with its saves, and "${s.name}" is installed ` +
      (s.hasSaves ? 'with its own saves.' : 'with no saves yet (a fresh start).');
    if (!(await ask(msg))) return;
    busy = 'Switching…';
    try {
      const r = await App.SwitchSetup(s.id);
      notify(`"${s.name}" is now in the game.` + (r.warnings?.length ? '\n' + r.warnings.join('\n') : ''), r.warnings?.length ? 'error' : 'ok');
      onswitched();
      App.SyncInstalledSets().then((m: string) => m && notify(m, 'ok')).catch(() => {});
    } catch (e) {
      notify(errText(e), 'error');
      onswitched();
    }
    busy = '';
    await load();
  }

  async function exportSetup(s: any) {
    busy = `Exporting "${s.name}"…`;
    exportProgress = '';
    try {
      const file = await App.ExportSetup(s.id);
      if (file) notify(`Exported to ${file}`, 'ok');
    } catch (e) { notify(errText(e), 'error'); }
    busy = '';
  }

  async function pickImport() {
    try { pick = await App.PickSetupFile(); } catch (e) { notify(errText(e), 'error'); }
  }

  async function doImport() {
    const file = pick.file;
    busy = 'Importing…';
    let id = '';
    try {
      const r = await App.ImportSetup(file);
      id = r.id;
      if (r.note) notify(r.note, 'ok');
      pick = null;
    } catch (e) { notify(errText(e), 'error'); }
    busy = '';
    await load();
    const s = view?.setups.find((x: any) => x.id === id);
    if (s) {
      notify(`Imported "${s.name}".`, 'ok');
      if (!view.gameRunning && view.gameFound && (await ask(`Switch the game to "${s.name}" now?`))) await switchTo(s);
    }
  }

  const fmtDate = (d: string) => (d ? new Date(d).toLocaleDateString() : '');
  const counts = (s: any) =>
    [`${s.sets} set${s.sets === 1 ? '' : 's'}`, s.accessories ? `${s.accessories} accessor${s.accessories === 1 ? 'y' : 'ies'}` : '',
     s.furniture ? `${s.furniture} furniture` : '', s.decorations ? `${s.decorations} decoration${s.decorations === 1 ? '' : 's'}` : ''].filter(Boolean).join(' · ');

  // A request from the sidebar setup menu: open the New form or start a switch.
  async function runAction() {
    const a = action;
    if (!a) return;
    ondone();
    if (a.create) startNew();
    const s = a.switchTo && view?.setups.find((x: any) => x.id === a.switchTo);
    if (s) {
      if (view.gameRunning) notify('Close the game first to switch setups.', 'error');
      else if (!view.gameFound) notify('Set the game folder (Settings) to switch setups.', 'error');
      else await switchTo(s);
    }
  }

  // Runs once the list is loaded, and again whenever the sidebar sends a new request while this page is open.
  $effect(() => {
    if (action && view && !busy) runAction();
  });

  onMount(() => {
    load();
    const offs = [
      EventsOn(Ev.SetupsProgress, (t: string) => (busy = t)),
      EventsOn(Ev.SetupsExport, (p: any) => (exportProgress = `${p.done} / ${p.total} files`)),
    ];
    return () => offs.forEach((off) => off());
  });
</script>

<svelte:window onmousedown={(e) => { if (menuFor && !(e.target as HTMLElement).closest('.more')) menuFor = ''; }} />

<div class="page">
  <header class="page-head">
    <div class="title">
      <h2>My Setups</h2>
      {#if view && !view.error}
        <div class="page-summary">
          <span><b>{view.setups.length}</b> setup{view.setups.length === 1 ? '' : 's'}</span>
          {#if view.setups.find((s: any) => s.active)}<span>in the game: <b>{view.setups.find((s: any) => s.active).name}</b></span>{/if}
        </div>
      {/if}
    </div>
    <span class="grow"></span>
    <button onclick={pickImport} disabled={!view || !!view.error} title="Add a .tcgsetup file someone shared as a new setup">Import…</button>
    <button class="primary" onclick={() => startNew()} disabled={!view || !!view.error}>New setup</button>
  </header>
  <!-- Fixed intro strip under the header (not part of the scrolling body). -->
  <div class="intro">
    <p><span class="muted">A setup is a complete collection of your custom content — sets, accessories, furniture, decorations, mod settings
      and the global card back — with <b>its own game saves</b>. Switch setups to change what's in the game.</span>
      <button class="link" onclick={() => (moreInfo = !moreInfo)}>{moreInfo ? 'Hide' : 'Sharing and saves'}</button></p>
    {#if moreInfo}<p class="muted">Export a setup to share it with friends as one <code>.tcgsetup</code> file — saves are never included.
      Setups are kept in {view?.workspace ?? 'the workspace'}\setups (change the workspace folder in Settings).</p>{/if}
  </div>

  <div class="main">
  {#if loading}
    <div class="grid">{#each Array(3) as _}<div class="card skeleton"><div class="bar"></div><div class="bar short"></div></div>{/each}</div>
  {:else if view}
    {#if view.error}
      <div class="notice err"><p><b>Setups aren't available:</b> {view.error}</p>
        <div class="row"><button onclick={retry}>Try again</button></div></div>
    {/if}
    {#if view.pending}
      <div class="notice err"><p>A setup switch didn't finish. Close the game, then repair it.</p>
        <div class="row"><button onclick={retry} disabled={view.gameRunning}>Repair now</button></div></div>
    {/if}
    {#if view.gameRunning}
      <div class="notice">The game is running — close it to switch setups.</div>
    {:else if !view.gameFound}
      <div class="notice">Set the game folder (Settings) to switch setups.</div>
    {/if}
    {#if view.steamCloud}
      <div class="notice">
        <b>Steam Cloud</b> syncs this game's saves. When you switch to a setup that has no saves yet, Steam may put the previous
        setup's saves back when the game starts. To keep setups apart, turn off Steam Cloud for TCG Card Shop Simulator
        (Steam → right-click the game → Properties → General).
      </div>
    {/if}

    {#if creating}
      <section class="block form">
        <div class="bhead"><h2>New setup</h2></div>
        <div class="formrow">
          <label class="field">Name <input bind:value={newName} placeholder="e.g. Vintage Magic" /></label>
          <label class="field">Start from
            <select bind:value={newFrom}>
              <option value="">Empty (no custom content)</option>
              {#each view.setups as s (s.id)}<option value={s.id}>A copy of "{s.name}"</option>{/each}
            </select>
          </label>
        </div>
        {#if newFrom}<label class="check"><input type="checkbox" bind:checked={newSaves} /> Also copy its game saves</label>{/if}
        <div class="row"><button class="primary" onclick={create}>Create</button><button onclick={() => (creating = false)}>Cancel</button></div>
      </section>
    {/if}

    {#if pick}
      <section class="block form">
        <div class="bhead"><h2>Import setup</h2>{#if pick.manifest.studioVersion}<span class="muted small">made with TCG Studio v{pick.manifest.studioVersion}</span>{/if}</div>
        <div class="name">{pick.manifest.name}</div>
        {#if pick.manifest.description}<div class="muted">{pick.manifest.description}</div>{/if}
        <div class="muted small">{counts(pick.manifest)} · {pick.file}</div>
        <div class="muted small">It's added as a new setup with no saves; your current setup isn't changed.</div>
        <div class="row"><button class="primary" onclick={doImport}>Import</button><button onclick={() => (pick = null)}>Cancel</button></div>
      </section>
    {/if}

    {#snippet stats(s: any)}
      <div class="stats-mini">
        <div><b>{s.sets ?? 0}</b><span>sets</span></div>
        <div><b>{s.accessories ?? 0}</b><span>accessories</span></div>
        <div><b>{s.furniture ?? 0}</b><span>furniture</span></div>
        <div><b>{s.decorations ?? 0}</b><span>decorations</span></div>
      </div>
    {/snippet}

    {#snippet card(s: any, i: number)}
      <section class="card" class:active={s.active}>
        {#if editId === s.id}
          <label class="field">Name <input bind:value={editName} /></label>
          <label class="field">Description <textarea rows="2" bind:value={editDesc}></textarea></label>
          <div class="row"><button class="primary" onclick={saveEdit}>Save</button><button onclick={() => (editId = '')}>Cancel</button></div>
        {:else}
          <div class="top">
            <span class="avatar">{s.name.slice(0, 1).toUpperCase()}</span>
            <div class="grow info">
              <div class="name">{s.name}
                {#if s.active}<span class="badge ok">In the game</span>{/if}
                {#if s.hasSaves}<span class="badge" title="This setup has parked game saves">saves</span>{/if}
              </div>
              <div class="muted small">{s.description || (s.created ? `Created ${fmtDate(s.created)}` : '')}</div>
            </div>
            <div class="more">
              <button class="dots" onclick={() => (menuFor = menuFor === s.id ? '' : s.id)} aria-label="More" title="More">⋯</button>
              {#if menuFor === s.id}
                <div class="menu">
                  <button onclick={() => { menuFor = ''; startEdit(s); }}>Rename…</button>
                  <button onclick={() => { menuFor = ''; startNew(s.id); }}>Duplicate…</button>
                  <button onclick={() => { menuFor = ''; exportSetup(s); }}>Export…</button>
                  <button onclick={() => { menuFor = ''; App.OpenSetupFolder(s.id); }}>Open folder</button>
                  <button disabled={i === 0} onclick={() => { menuFor = ''; move(s.id, i - 1); }}>Move earlier</button>
                  <button disabled={i === view.setups.length - 1} onclick={() => { menuFor = ''; move(s.id, i + 1); }}>Move later</button>
                  {#if !s.active}<button class="danger" onclick={() => { menuFor = ''; remove(s); }}>Delete…</button>{/if}
                </div>
              {/if}
            </div>
          </div>
          {@render stats(s)}
          {#if s.active}
            <div class="row"><span class="muted small grow">The game plays this setup — its sets, accessories and saves.</span>
              <button onclick={() => exportSetup(s)}>Export…</button><button onclick={() => startNew(s.id)}>Duplicate</button></div>
          {:else}
            <button class="primary switch" onclick={() => switchTo(s)} disabled={view.gameRunning || !view.gameFound || !!view.error}>Switch to this setup</button>
          {/if}
        {/if}
      </section>
    {/snippet}

    {@const activeIdx = view.setups.findIndex((s: any) => s.active)}
    {#if activeIdx >= 0}
      <div class="activecard">{@render card(view.setups[activeIdx], activeIdx)}</div>
    {/if}
    {#if view.setups.some((s: any) => !s.active)}
      <h3 class="sectiontitle">Other setups</h3>
      <div class="grid">
        {#each view.setups as s, i (s.id)}
          {#if !s.active}{@render card(s, i)}{/if}
        {/each}
      </div>
    {/if}
  {/if}
  </div>
</div>

{#if busy}
  <div class="backdrop" role="presentation">
    <div class="box">
      <div class="spinner"></div>
      <div>{busy}</div>
      {#if exportProgress}<div class="muted small">{exportProgress}</div>{/if}
    </div>
  </div>
{/if}

<style>
  .page { display: flex; flex-direction: column; height: 100%; overflow: hidden; }
  .main { flex: 1; min-height: 0; overflow: auto; padding: 18px 24px 28px; display: flex; flex-direction: column; gap: 14px; }
  .notice p { margin: 0; }
  .notice .row { margin-top: 8px; }
  .intro { flex: none; padding: 8px 24px; font-size: 13px; background: var(--panel); border-bottom: 1px solid var(--line); }
  .intro p { margin: 0; max-width: 1000px; }
  .intro p + p { margin-top: 6px; }
  .link { padding: 0 0 0 6px; font-size: 13px; }
  .form { border-color: var(--accent-2); max-width: 760px; }
  .formrow { display: grid; grid-template-columns: repeat(auto-fit, minmax(240px, 1fr)); gap: 12px; }
  .sectiontitle { margin: 8px 0 0; }
  .grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(320px, 1fr)); gap: 14px; }
  .activecard .card { border-color: #1f6b4b; background: linear-gradient(90deg, rgba(62, 207, 142, 0.1), var(--panel) 55%); }
  .card { position: relative; background: var(--panel); border: 1px solid var(--line); border-radius: 12px; padding: 14px 16px; display: flex; flex-direction: column; gap: 12px; }
  .card:hover { border-color: #46506a; }
  .top { display: flex; align-items: flex-start; gap: 12px; }
  .avatar { width: 44px; height: 44px; flex: none; border-radius: 12px; display: grid; place-items: center; font-size: 20px; font-weight: 700;
    background: var(--panel-2); border: 1px solid var(--line); }
  .active .avatar { background: var(--ok); color: #0b2018; border-color: transparent; }
  .info { min-width: 0; }
  .name { font-weight: 600; font-size: 16px; display: flex; flex-wrap: wrap; gap: 8px; align-items: center; }
  .stats-mini { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 8px; }
  .stats-mini div { background: var(--panel-2); border-radius: 8px; padding: 6px 8px; display: flex; flex-direction: column; }
  .stats-mini b { font-size: 18px; }
  .stats-mini span { font-size: 11px; color: var(--muted); }
  .switch { align-self: flex-start; }
  .dots { padding: 4px 10px; }
  .menu { top: calc(100% + 4px); min-width: 170px; }
  .skeleton { animation: pulse 1.2s ease-in-out infinite; min-height: 140px; }
  .skeleton .bar { height: 14px; background: var(--panel-2); border-radius: 4px; }
  .skeleton .bar.short { width: 60%; }
  @keyframes pulse { 50% { opacity: 0.55; } }
  code { background: var(--panel-2); padding: 0 4px; border-radius: 4px; }
  .box { background: var(--panel); border: 1px solid var(--line); border-radius: var(--radius); padding: 20px 28px; display: flex; flex-direction: column; align-items: center; gap: 10px; min-width: 280px; }
  .spinner { width: 28px; height: 28px; border: 3px solid var(--line); border-top-color: var(--accent); border-radius: 50%; animation: spin 0.9s linear infinite; }
  @keyframes spin { to { transform: rotate(360deg); } }
</style>
