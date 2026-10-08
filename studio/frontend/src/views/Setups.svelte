<script lang="ts">
  // Setups: complete studio workspaces (sets, accessory library, mod settings, card back) that can be swapped in and out of
  // the game. Each setup keeps its own game saves; exports (.tcgsetup) never contain saves.
  import { onMount } from 'svelte';
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
      id = await App.ImportSetup(file);
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
      EventsOn('setups:progress', (t: string) => (busy = t)),
      EventsOn('setups:export', (p: any) => (exportProgress = `${p.done} / ${p.total} files`)),
    ];
    return () => offs.forEach((off) => off());
  });
</script>

<div class="page">
  <header class="row">
    <h2 class="grow">My Setups</h2>
    <button onclick={pickImport} disabled={!view || !!view.error}>Import…</button>
    <button class="primary" onclick={() => startNew()} disabled={!view || !!view.error}>New setup</button>
  </header>
  <p class="muted intro">
    A setup is a complete collection of your custom content: sets, accessories, furniture, mod settings and the global card back.
    Switch between setups to change what's in the game; each setup keeps <b>its own game saves</b>. Export a setup to share it
    with friends as one <code>.tcgsetup</code> file — saves are never included.
  </p>

  {#if loading}
    <p class="muted">Loading…</p>
  {:else if view}
    {#if view.error}
      <div class="notice err">
        <b>Setups aren't available:</b> {view.error}
        <div class="row"><button onclick={retry}>Try again</button></div>
      </div>
    {/if}
    {#if view.pending}
      <div class="notice err">
        A setup switch didn't finish. Close the game, then repair it.
        <div class="row"><button onclick={retry} disabled={view.gameRunning}>Repair now</button></div>
      </div>
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
      <section class="form">
        <h3>New setup</h3>
        <label class="field">Name <input bind:value={newName} placeholder="e.g. Vintage Magic" /></label>
        <label class="field">Start from
          <select bind:value={newFrom}>
            <option value="">Empty (no custom content)</option>
            {#each view.setups as s (s.id)}<option value={s.id}>A copy of "{s.name}"</option>{/each}
          </select>
        </label>
        {#if newFrom}
          <label class="check"><input type="checkbox" bind:checked={newSaves} /> Also copy its game saves</label>
        {/if}
        <div class="row">
          <button class="primary" onclick={create}>Create</button>
          <button onclick={() => (creating = false)}>Cancel</button>
        </div>
      </section>
    {/if}

    {#if pick}
      <section class="form">
        <h3>Import setup</h3>
        <div><b>{pick.manifest.name}</b>{#if pick.manifest.studioVersion}<span class="muted"> · made with TCG Studio v{pick.manifest.studioVersion}</span>{/if}</div>
        {#if pick.manifest.description}<div class="muted">{pick.manifest.description}</div>{/if}
        <div class="muted small">{counts(pick.manifest)} · {pick.file}</div>
        <div class="muted small">It's added as a new setup with no saves; your current setup isn't changed.</div>
        <div class="row">
          <button class="primary" onclick={doImport}>Import</button>
          <button onclick={() => (pick = null)}>Cancel</button>
        </div>
      </section>
    {/if}

    <div class="list">
      {#each view.setups as s, i (s.id)}
        <section class="setup" class:active={s.active}>
          {#if editId === s.id}
            <label class="field">Name <input bind:value={editName} /></label>
            <label class="field">Description <textarea rows="2" bind:value={editDesc}></textarea></label>
            <div class="row">
              <button class="primary" onclick={saveEdit}>Save</button>
              <button onclick={() => (editId = '')}>Cancel</button>
            </div>
          {:else}
            <div class="row">
              <div class="grow">
                <div class="name">{s.name}
                  {#if s.active}<span class="badge ok">In the game</span>{/if}
                  {#if s.hasSaves}<span class="badge" title="This setup has parked game saves">saves</span>{/if}
                </div>
                {#if s.description}<div class="muted">{s.description}</div>{/if}
                <div class="muted small">{counts(s)}{#if s.created} · created {fmtDate(s.created)}{/if}</div>
              </div>
              <div class="order">
                <button class="small" title="Move up" disabled={i === 0} onclick={() => move(s.id, i - 1)}>▲</button>
                <button class="small" title="Move down" disabled={i === view.setups.length - 1} onclick={() => move(s.id, i + 1)}>▼</button>
              </div>
            </div>
            <div class="row actions">
              {#if !s.active}
                <button class="primary" onclick={() => switchTo(s)} disabled={view.gameRunning || !view.gameFound || !!view.error}>Switch to this setup</button>
              {/if}
              <button onclick={() => startEdit(s)}>Rename</button>
              <button onclick={() => startNew(s.id)}>Duplicate</button>
              <button onclick={() => exportSetup(s)}>Export…</button>
              <button onclick={() => App.OpenSetupFolder(s.id)}>Open folder</button>
              <div class="grow"></div>
              {#if !s.active}<button class="danger" onclick={() => remove(s)}>Delete</button>{/if}
            </div>
          {/if}
        </section>
      {/each}
    </div>
    <p class="muted small">Setups are kept in {view.workspace}\setups (change the workspace folder in Settings).</p>
  {/if}
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
  .page { padding: 20px 24px; overflow: auto; height: 100%; display: flex; flex-direction: column; gap: 14px; }
  .intro { max-width: 900px; margin: 0; }
  .notice { max-width: 980px; padding: 10px 12px; border: 1px solid #6b5320; border-radius: var(--radius); background: var(--panel); display: flex; flex-direction: column; gap: 8px; }
  .notice.err { border-color: #6b2a31; }
  section { background: var(--panel); border: 1px solid var(--line); border-radius: var(--radius); padding: 12px 14px; display: flex; flex-direction: column; gap: 8px; max-width: 980px; }
  section.form { border-color: var(--accent-2); }
  section.active { border-color: #1f6b4b; }
  .list { display: flex; flex-direction: column; gap: 10px; }
  .name { font-weight: 600; font-size: 15px; display: flex; gap: 8px; align-items: center; }
  .order { display: flex; flex-direction: column; gap: 2px; }
  .actions { flex-wrap: wrap; }
  .small { font-size: 12px; }
  code { background: var(--panel-2); padding: 0 4px; border-radius: 4px; }
  .backdrop { position: fixed; inset: 0; background: rgba(0, 0, 0, 0.6); display: flex; align-items: center; justify-content: center; z-index: 100; }
  .box { background: var(--panel); border: 1px solid var(--line); border-radius: var(--radius); padding: 20px 28px; display: flex; flex-direction: column; align-items: center; gap: 10px; min-width: 280px; }
  .spinner { width: 28px; height: 28px; border: 3px solid var(--line); border-top-color: var(--accent); border-radius: 50%; animation: spin 0.9s linear infinite; }
  @keyframes spin { to { transform: rotate(360deg); } }
</style>
