<script lang="ts">
  import { onMount } from 'svelte';
  import { Ev } from '../lib/events';
  import { App, EventsOn, errText, ask } from '../lib/api';

  let { notify, onchange }: { notify: (t: string, k?: string) => void; onchange: () => void } = $props();

  let settings = $state<any>(null);
  let status = $state<any>(null);
  let tpl = $state<any>(null); // game templates (read from the game files, or the mod's in-game export)
  let libraryFolder = $state('');

  async function load() {
    settings = await App.GetSettings(); settings.imageFormat ||= 'png';
    status = await App.GameStatus();
    tpl = await App.TemplatesStatus();
    libraryFolder = await App.LibraryFolder();
  }

  async function rereadTemplates() {
    try { tpl = await App.RefreshTemplates(true); } catch (e) { notify(errText(e), 'error'); }
  }

  async function locate() {
    status = await App.LocateGame();
    settings = await App.GetSettings(); settings.imageFormat ||= 'png';
    tpl = await App.TemplatesStatus();
    notify(status.found ? 'Game found' : 'Game not found in Steam libraries — use Browse', status.found ? 'ok' : 'error');
    onchange();
  }

  async function browse() {
    try {
      status = await App.BrowseGameFolder();
      settings = await App.GetSettings(); settings.imageFormat ||= 'png';
      tpl = await App.TemplatesStatus();
      onchange();
    } catch (e) {
      notify(errText(e), 'error');
    }
  }

  async function saveWorkspace() {
    try {
      const old = (await App.GetSettings()).workspace;
      if (settings.workspace && settings.workspace !== old && !(await App.WorkspaceHasSetups(settings.workspace)) &&
        !(await ask(`There is no TCG Studio workspace in ${settings.workspace} yet.

TCG Studio starts an empty one there and ` +
          `the game then reads it: your current sets and items stay in ${old} and don't show in the game until you switch back ` +
          `or move them there. Use this folder?`))) return;
      settings = await App.SaveSettings(settings);
      libraryFolder = await App.LibraryFolder();
      notify('Saved', 'ok');
    } catch (e) {
      notify(errText(e), 'error');
    }
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

<div class="page">
  <h2>Settings</h2>
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
      <h3>Downloads</h3>
      <label class="field">Default card image format for imports
        <select bind:value={settings.imageFormat} onchange={saveWorkspace}>
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
      <div class="row"><input class="grow" bind:value={settings.workspace} /><button onclick={saveWorkspace}>Save</button></div>
      <h4>Shared library</h4>
      <p class="muted small">Every set's card art, card back and pack &amp; box art, stored once for all your setups. The game reads
        them from here (no copies in the game folder). It is always the workspace's <code>library</code> folder.</p>
      <div class="row"><input class="grow" value={libraryFolder} readonly /><button onclick={() => App.OpenFolder(libraryFolder)}>Open folder</button></div>
    </section>
  {/if}
</div>

<style>
  .page { padding: 20px 24px; overflow: auto; height: 100%; display: flex; flex-direction: column; gap: 18px; }
  section { background: var(--panel); border: 1px solid var(--line); border-radius: var(--radius); padding: 14px; display: flex; flex-direction: column; gap: 10px; max-width: 900px; }
  ul { margin: 0; padding-left: 20px; line-height: 1.8; }
  .small { font-size: 12px; margin: 0; }
</style>
