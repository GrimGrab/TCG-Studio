<script lang="ts">
  import { onMount } from 'svelte';
  import { App, EventsOn, errText } from '../lib/api';
  import CropBox from './CropBox.svelte';

  let { notify, onchange }: { notify: (t: string, k?: string) => void; onchange: () => void } = $props();

  let settings = $state<any>(null);
  let status = $state<any>(null);
  let tpl = $state<any>(null); // game templates (read from the game files, or the mod's in-game export)

  async function load() {
    settings = await App.GetSettings(); settings.imageFormat ||= 'png';
    status = await App.GameStatus();
    tpl = await App.TemplatesStatus();
    globalBack = await App.GlobalCardBack();
    await loadSign();
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

  // Shop sign: stored as mod settings, so a running game applies changes right away.
  let sign = $state<any>(null);
  let signError = $state('');
  async function loadSign() {
    try { sign = await App.ShopSign(); signError = ''; } catch (e) { sign = null; signError = errText(e); }
  }
  async function signDo(f: () => Promise<any>) {
    try { sign = await f(); } catch (e) { notify(errText(e), 'error'); }
  }

  async function saveWorkspace() {
    try {
      settings = await App.SaveSettings(settings);
      notify('Saved', 'ok');
    } catch (e) {
      notify(errText(e), 'error');
    }
  }

  onMount(() => {
    load();
    const offs = [
      EventsOn('templates:progress', (m: string) => (tpl = { ...tpl, busy: true, message: m })),
      EventsOn('templates:ready', async (t: any) => { tpl = t; status = await App.GameStatus(); }),
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
      <h3>Shop sign</h3>
      <p class="muted">The sign above your shop's entrance. Pick any picture, then drag the box to choose the part that's shown (drag a corner
        to resize it). It glows at night like the vanilla sign. Changes show in a running game right away.</p>
      {#if sign}
        {#if sign.image}
          <CropBox src={sign.image} aspect={sign.aspect} crop={sign.crop} label={sign.showName ? 'Shop name' : ''}
            onchange={(c) => signDo(() => App.SetShopSignCrop(c))} />
        {:else}
          <div class="sign"><span class="muted">vanilla sign</span></div>
        {/if}
        <div class="row">
          <button onclick={() => signDo(App.ChooseShopSign)}>Choose image…</button>
          {#if sign.image}
            <button onclick={() => signDo(() => App.SetShopSignCrop([]))} disabled={!sign.crop}>Centre</button>
            <button class="danger" onclick={() => signDo(App.RemoveShopSign)}>Remove</button>
          {/if}
        </div>
        <label><input type="checkbox" checked={sign.showName} onchange={(ev) => signDo(() => App.SetShopSignShowName(ev.currentTarget.checked))} /> Show the shop name on the sign
          <span class="muted small">(turn off when your picture already has a name or logo)</span></label>
      {:else}
        <p class="muted small">{signError || 'Loading…'}</p>
      {/if}
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
  .sign { width: 360px; height: 88px; background: var(--bg); border-radius: 6px; display: flex; align-items: center; justify-content: center; overflow: hidden; }
  .col { display: flex; flex-direction: column; gap: 6px; }
  .small { font-size: 12px; margin: 0; }
  input[type="color"] { width: 48px; height: 32px; padding: 2px; }
</style>
