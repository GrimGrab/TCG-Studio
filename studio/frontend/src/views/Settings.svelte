<script lang="ts">
  import { onMount } from 'svelte';
  import { App, errText } from '../lib/api';

  let { notify, onchange }: { notify: (t: string, k?: string) => void; onchange: () => void } = $props();

  let settings = $state<any>(null);
  let status = $state<any>(null);

  async function load() {
    settings = await App.GetSettings();
    status = await App.GameStatus();
    globalBack = await App.GlobalCardBack();
  }

  async function locate() {
    status = await App.LocateGame();
    settings = await App.GetSettings();
    notify(status.found ? 'Game found' : 'Game not found in Steam libraries — use Browse', status.found ? 'ok' : 'error');
    onchange();
  }

  async function browse() {
    try {
      status = await App.BrowseGameFolder();
      settings = await App.GetSettings();
      onchange();
    } catch (e) {
      notify(errText(e), 'error');
    }
  }

  let globalBack = $state('');
  let backColor = $state('#5b3a8c');
  let backTitle = $state('');

  async function chooseGlobal() {
    try { globalBack = await App.ChooseGlobalCardBack(); } catch (e) { notify(errText(e), 'error'); }
  }
  async function generateGlobal() {
    try { globalBack = await App.GenerateGlobalCardBack(backColor, backTitle); } catch (e) { notify(errText(e), 'error'); }
  }
  async function removeGlobal() {
    try { await App.RemoveGlobalCardBack(); globalBack = ''; } catch (e) { notify(errText(e), 'error'); }
  }

  async function saveWorkspace() {
    try {
      settings = await App.SaveSettings(settings);
      notify('Saved', 'ok');
    } catch (e) {
      notify(errText(e), 'error');
    }
  }

  onMount(load);
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
        <li>{status.templatesFound ? '✅' : '⚠️'} Pack/box templates {status.templatesFound ? '' : '(load a save once with the mod installed to export them)'}</li>
      </ul>
    </section>
    <section>
      <h3>Global card back</h3>
      <p class="muted">Used by every custom set that doesn't choose its own back (Set tab). Vanilla sets keep their backs. Any image size works: plain borders are trimmed and it's cropped to the card shape, and the game keeps its own rounded edge around it. Written straight into the mod folder — restart the game to see it.</p>
      <div class="row back-row">
        <div class="back">{#if globalBack}<img src={globalBack} alt="global card back" />{:else}<span class="muted">none</span>{/if}</div>
        <div class="col">
          <button onclick={chooseGlobal} disabled={!status.found}>Choose image…</button>
          <div class="row"><input type="color" bind:value={backColor} /><input placeholder="Title (optional)" bind:value={backTitle} /><button onclick={generateGlobal} disabled={!status.found}>Generate</button></div>
          {#if globalBack}<button class="danger" onclick={removeGlobal}>Remove</button>{/if}
        </div>
      </div>
    </section>
    <section>
      <h3>Workspace</h3>
      <p class="muted">Where the studio keeps your sets and their images before they are installed into the game.</p>
      <div class="row"><input class="grow" bind:value={settings.workspace} /><button onclick={saveWorkspace}>Save</button></div>
    </section>
  {/if}
</div>

<style>
  .page { padding: 20px 24px; overflow: auto; height: 100%; display: flex; flex-direction: column; gap: 18px; }
  section { background: var(--panel); border: 1px solid var(--line); border-radius: var(--radius); padding: 14px; display: flex; flex-direction: column; gap: 10px; max-width: 900px; }
  ul { margin: 0; padding-left: 20px; line-height: 1.8; }
  .back-row { align-items: flex-start; gap: 14px; }
  .back { width: 130px; height: 130px; background: var(--bg); border-radius: 6px; display: flex; align-items: center; justify-content: center; overflow: hidden; }
  .back img { max-width: 100%; max-height: 100%; }
  .col { display: flex; flex-direction: column; gap: 6px; }
  input[type="color"] { width: 48px; height: 32px; padding: 2px; }
</style>
