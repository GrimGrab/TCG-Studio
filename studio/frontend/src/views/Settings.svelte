<script lang="ts">
  import { onMount } from 'svelte';
  import { Ev } from '../lib/events';
  import { App, EventsOn, errText, ask } from '../lib/api';
  import { Draft } from '../lib/draft.svelte';
  import SaveBar from './SaveBar.svelte';
  import { UI_SCALES } from '../lib/uiScale';

  let { notify, onchange }: { notify: (t: string, k?: string) => void; onchange: () => void } = $props();

  let settings = $state<any>(null);
  let status = $state<any>(null);
  let tpl = $state<any>(null); // game templates (read from the game files, or the mod's in-game export)
  let libraryFolder = $state('');
  // The settings this page edits (Save stores them); the game folder is set by its own buttons straight away.
  const draft = new Draft<{ workspace: string; imageFormat: string; uiScale: number }>();
  const fromSettings = (s: any) => ({ workspace: s.workspace ?? '', imageFormat: s.imageFormat || 'png', uiScale: s.uiScale || 1 });
  async function readSettings() {
    settings = await App.GetSettings();
    if (!draft.dirty) draft.reset(fromSettings(settings));
  }
  // The interface size this run started with: a saved different one needs a restart (WebView2 takes it on start).
  let startScale = $state(1);
  App.UIScaleAtStart().then((v) => (startScale = v || 1)).catch(() => {});
  let needsRestart = $derived(!!settings && Math.abs((settings.uiScale || 1) - startScale) > 0.001);
  async function restart() {
    try { await App.RestartStudio(); } catch (e) { notify(errText(e), 'error'); }
  }

  async function load() {
    await readSettings();
    status = await App.GameStatus();
    tpl = await App.TemplatesStatus();
    libraryFolder = await App.LibraryFolder();
  }

  async function rereadTemplates() {
    try { tpl = await App.RefreshTemplates(true); } catch (e) { notify(errText(e), 'error'); }
  }

  async function locate() {
    status = await App.LocateGame();
    await readSettings();
    tpl = await App.TemplatesStatus();
    notify(status.found ? 'Game found' : 'Game not found in Steam libraries — use Browse', status.found ? 'ok' : 'error');
    onchange();
  }

  async function browse() {
    try {
      status = await App.BrowseGameFolder();
      await readSettings();
      tpl = await App.TemplatesStatus();
      onchange();
    } catch (e) {
      notify(errText(e), 'error');
    }
  }

  let saving = $state(false);
  async function save() {
    const want = draft.value;
    try {
      const old = (await App.GetSettings()).workspace;
      if (want.workspace && want.workspace !== old && !(await App.WorkspaceHasSetups(want.workspace)) &&
        !(await ask(`There is no TCG Studio workspace in ${want.workspace} yet.

TCG Studio starts an empty one there and ` +
          `the game then reads it: your current sets and items stay in ${old} and don't show in the game until you switch back ` +
          `or move them there. Use this folder?`))) return;
      saving = true;
      settings = await App.SaveSettings({ ...(await App.GetSettings()), ...want });
      draft.reset(fromSettings(settings));
      libraryFolder = await App.LibraryFolder();
      notify('Saved', 'ok');
    } catch (e) {
      notify(errText(e), 'error');
    }
    saving = false;
  }

  onMount(() => {
    load();
    const offs = [
      EventsOn(Ev.TemplatesProgress, (m: string) => (tpl = { ...tpl, busy: true, message: m })),
      EventsOn(Ev.TemplatesReady, async (t: any) => { tpl = t; status = await App.GameStatus(); }),
    ];
    return () => offs.forEach((off) => off());
  });
</script>

<SaveBar dirty={draft.dirty} {saving} onsave={save} onrevert={() => draft.revert()}
  leaveMessage="Leave Settings without saving? Your unsaved changes will be lost.">
  {#snippet title()}<h2>Settings</h2>{/snippet}
</SaveBar>
<div class="page">
  {#if settings && status}
    <!-- One card per topic; each setting is a row: what it is on the left, the control on the right. -->
    <section class="block">
      <div class="bhead"><h2>Game</h2><span class="muted small">Where TCG Card Shop Simulator is installed</span></div>
      <div class="srow">
        <div class="label"><b>Game folder</b><span class="muted small">Studio reads the game's art and models from here and installs the mod into it.</span></div>
        <div class="control">
          <input class="path" readonly value={settings.gameDir || '(not set)'} title={settings.gameDir} />
          <div class="row"><button onclick={locate}>Find automatically</button><button onclick={browse}>Browse…</button></div>
        </div>
      </div>
      <ul class="items">
        <li class={status.found ? 'ok' : 'missing'}><span class="icon">{status.found ? '✓' : '✗'}</span><b>TCG Card Shop Simulator</b></li>
        <li class={status.bepInEx ? 'ok' : 'missing'}><span class="icon">{status.bepInEx ? '✓' : '✗'}</span><b>BepInEx</b></li>
        <li class={status.modInstalled ? 'ok' : 'missing'}><span class="icon">{status.modInstalled ? '✓' : '✗'}</span><b>TCG Custom Cards mod</b>
          {#if !status.modInstalled}<span class="muted small">install it on <b>Game &amp; mod</b></span>{/if}</li>
        <li class={tpl?.busy ? 'info' : tpl?.ready ? 'ok' : 'outdated'}>
          <span class="icon">{tpl?.busy ? '…' : tpl?.ready ? '✓' : '!'}</span>
          <b>Game templates</b>
          <span class="muted small grow">{tpl?.busy ? tpl.message : tpl?.ready ? (tpl.source === 'mod' ? 'old in-game export — use Read again' : 'read from the game files')
            : (tpl?.error || (status.found ? 'not read yet' : 'set the game folder first'))}</span>
          {#if status.found && !tpl?.busy}<button class="small" onclick={rereadTemplates} title="Read the vanilla art, models and prices from the game files again">Read again</button>{/if}
        </li>
      </ul>
    </section>

    <section class="block">
      <div class="bhead"><h2>Interface</h2></div>
      <div class="srow">
        <div class="label"><b>Interface size</b><span class="muted small">Size of text, buttons and panels in TCG Studio. Takes effect when Studio restarts.</span></div>
        <div class="control">
          <select bind:value={draft.value.uiScale}>
            {#each UI_SCALES as v}<option value={v}>{Math.round(v * 100)} %{v === 1 ? ' (default)' : ''}</option>{/each}
          </select>
          {#if needsRestart}
            <div class="row"><span class="small grow">Saved — shows after a restart.</span><button class="primary" onclick={restart} disabled={draft.dirty}
              title={draft.dirty ? 'Save or revert your other changes on this page first' : 'Start TCG Studio again with the new size'}>Restart now</button></div>
          {/if}
        </div>
      </div>
    </section>

    <section class="block">
      <div class="bhead"><h2>Downloads</h2></div>
      <div class="srow">
        <div class="label"><b>Card image format for imports</b><span class="muted small">The Import page starts with this; you can still pick the other for
          one import. Existing sets can be converted on the Storage page.</span></div>
        <div class="control">
          <select bind:value={draft.value.imageFormat}>
            <option value="png">PNG — best quality (default)</option>
            <option value="jpg">JPEG — about 6× smaller, scans lose a little detail</option>
          </select>
        </div>
      </div>
    </section>

    <section class="block">
      <div class="bhead"><h2>Workspace</h2><span class="muted small">Where all your content lives</span></div>
      <div class="srow">
        <div class="label"><b>Workspace folder</b><span class="muted small">Holds all your setups (each with its sets and accessories) — see the setup picker.</span></div>
        <div class="control"><input class="path" bind:value={draft.value.workspace} /></div>
      </div>
      <div class="srow">
        <div class="label"><b>Shared library</b><span class="muted small">Every set's card art, card back and pack &amp; box art, stored once for all your
          setups. The game reads it from here (no copies in the game folder). Always the workspace's <code>library</code> folder.</span></div>
        <div class="control">
          <input class="path" value={libraryFolder} readonly title={libraryFolder} />
          <div class="row"><button onclick={() => App.OpenFolder(libraryFolder)}>Open folder</button></div>
        </div>
      </div>
    </section>
  {:else}
    <p class="muted">Loading…</p>
  {/if}
</div>

<style>
  .page { padding: 20px 24px 28px; overflow: auto; flex: 1; min-height: 0; display: flex; flex-direction: column; gap: 16px; }
  .block { max-width: 1000px; }
  .srow { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1.1fr); gap: 20px; align-items: start; padding: 10px 0; border-top: 1px solid var(--line); }
  .bhead + .srow { border-top: none; }
  .label { display: flex; flex-direction: column; gap: 3px; }
  .control { display: flex; flex-direction: column; gap: 8px; min-width: 0; }
  .control select, .control .path { width: 100%; min-width: 0; }
  .path { font-family: Consolas, monospace; font-size: 12px; }
  .items { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 6px; }
  .items li { display: flex; gap: 10px; align-items: center; padding: 7px 10px; border-radius: 8px; background: var(--panel-2); }
  .icon { width: 22px; height: 22px; border-radius: 50%; display: inline-flex; align-items: center; justify-content: center; font-weight: 700; flex-shrink: 0; background: var(--line); }
  li.ok .icon { background: var(--ok); color: #fff; }
  li.missing .icon { background: var(--danger); color: #fff; }
  li.outdated .icon { background: #d9a400; color: #000; }
  .small { margin: 0; }
</style>
