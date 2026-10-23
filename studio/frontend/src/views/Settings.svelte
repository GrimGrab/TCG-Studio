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
    <section>
      <h3>Game</h3>
      <div class="row"><input class="grow" readonly value={settings.gameDir || '(not set)'} /><button onclick={locate}>Find automatically</button><button onclick={browse}>Browse…</button></div>
      <ul>
        <li>{status.found ? '✅' : '❌'} TCG Card Shop Simulator</li>
        <li>{status.bepInEx ? '✅' : '❌'} BepInEx</li>
        <li>{status.modInstalled ? '✅' : '❌'} TCG Custom Cards mod</li>
        <li>
          {#if tpl?.busy}⏳ Game templates — {tpl.message}
          {:else if tpl?.ready}✅ Game templates <span class="muted">({tpl.source === 'mod' ? 'old in-game export — use Read again' : 'read from the game files'})</span>
          {:else}⚠️ Game templates <span class="muted">{tpl?.error || (status.found ? 'not read yet' : 'set the game folder first')}</span>{/if}
          {#if status.found && !tpl?.busy}<button class="small" onclick={rereadTemplates} title="Read the vanilla art, models and prices from the game files again">Read again</button>{/if}
        </li>
      </ul>
    </section>
    <section>
      <h3>Interface size</h3>
      <label class="field">Size of text, buttons and panels in TCG Studio
        <select bind:value={draft.value.uiScale}>
          {#each UI_SCALES as v}<option value={v}>{Math.round(v * 100)} %{v === 1 ? ' (default)' : ''}</option>{/each}
        </select>
      </label>
      {#if needsRestart}
        <div class="row"><span class="grow">Saved. The new size shows after TCG Studio restarts.</span><button class="primary" onclick={restart} disabled={draft.dirty}
          title={draft.dirty ? 'Save or revert your other changes on this page first' : 'Start TCG Studio again with the new size'}>Restart now</button></div>
      {:else}
        <p class="muted small">Takes effect when TCG Studio restarts (you can restart right after saving).</p>
      {/if}
    </section>
    <section>
      <h3>Downloads</h3>
      <label class="field">Default card image format for imports
        <select bind:value={draft.value.imageFormat}>
          <option value="png">PNG — best quality (default)</option>
          <option value="jpg">JPEG — about 6× smaller, card scans lose a little detail</option>
        </select>
      </label>
      <p class="muted small">The Import page starts with this format; you can still pick the other one for a single import.
        Existing sets can be converted on the Storage page.</p>
    </section>
    <section>
      <h3>Workspace</h3>
      <p class="muted">The folder that holds all your setups (each with its sets, images and accessories) — see My Setups.</p>
      <div class="row"><input class="grow" bind:value={draft.value.workspace} /></div>
      <h4>Shared library</h4>
      <p class="muted small">Every set's card art, card back and pack &amp; box art, stored once for all your setups. The game reads
        them from here (no copies in the game folder). It is always the workspace's <code>library</code> folder.</p>
      <div class="row"><input class="grow" value={libraryFolder} readonly /><button onclick={() => App.OpenFolder(libraryFolder)}>Open folder</button></div>
    </section>
  {/if}
</div>

<style>
  .page { padding: 20px 24px; overflow: auto; flex: 1; min-height: 0; display: flex; flex-direction: column; gap: 18px; }
  section { background: var(--panel); border: 1px solid var(--line); border-radius: var(--radius); padding: 14px; display: flex; flex-direction: column; gap: 10px; max-width: 900px; }
  ul { margin: 0; padding-left: 20px; line-height: 1.8; }
  .small { font-size: 12px; margin: 0; }
</style>
