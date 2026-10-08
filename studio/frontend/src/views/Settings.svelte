<script lang="ts">
  import { onMount } from 'svelte';
  import { App, EventsOn, errText } from '../lib/api';
  import CropBox from './CropBox.svelte';
  import SignPreview from './SignPreview.svelte';

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
    await loadFonts();
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
  // Shop name style: edited here (the preview follows every move), saved when a control is released.
  let signStyle = $state<any>(null);
  let previewName = $state('');
  let fonts = $state<any[]>([]);
  $effect(() => { if (sign) signStyle = { ...sign.text, ownColor: !!sign.text.color, color: sign.text.color || '#ffd23c' }; });
  $effect(() => { if (sign && !previewName) previewName = sign.shopName || 'My Card Shop'; });
  async function loadFonts() {
    try { fonts = await App.ShopSignFonts(); } catch { fonts = []; }
  }
  function saveStyle() {
    const s = signStyle;
    signDo(() => App.SetShopSignText({ ...s, color: s.ownColor ? s.color : '' }));
  }
  function pickFont(value: string) {
    if (value === 'current') return;
    const i = value.indexOf('|');
    signDo(() => App.SetShopSignFont(value.slice(0, i), value.slice(i + 1)));
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
      EventsOn('templates:ready', async (t: any) => { tpl = t; status = await App.GameStatus(); await loadSign(); await loadFonts(); }),
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
        {#if sign.showName && signStyle}
          <div class="sign-text">
            <label class="field">Font
              <select value={sign.text.font ? 'current' : '|'} onchange={(ev) => pickFont(ev.currentTarget.value)}>
                {#if sign.text.font}<option value="current">{sign.text.font}</option>{/if}
                <option value="|">Game default (Fredoka One)</option>
                {#if fonts.some((f) => f.source === 'game')}
                  <optgroup label="From the game">
                    {#each fonts.filter((f) => f.source === 'game') as f (f.id)}<option value={'game|' + f.id}>{f.name}</option>{/each}
                  </optgroup>
                {/if}
                {#if fonts.some((f) => f.source === 'windows')}
                  <optgroup label="Installed on Windows">
                    {#each fonts.filter((f) => f.source === 'windows') as f (f.id)}<option value={'windows|' + f.id}>{f.name}</option>{/each}
                  </optgroup>
                {/if}
              </select>
            </label>
            <button onclick={() => pickFont('file|')}>Font file…</button>
            <label><input type="checkbox" bind:checked={signStyle.ownColor} onchange={saveStyle} /> Own colour</label>
            {#if signStyle.ownColor}<input type="color" bind:value={signStyle.color} onchange={saveStyle} />{:else}<span class="muted small">game gold</span>{/if}
            <label class="field">Size {Math.round(signStyle.size * 100)}%
              <input type="range" min="0.5" max="2" step="0.05" bind:value={signStyle.size} onchange={saveStyle} />
            </label>
            <label class="field">Outline {signStyle.outline > 0 ? Math.round(signStyle.outline * 100) + '%' : 'none'}
              <input type="range" min="0" max="0.3" step="0.01" bind:value={signStyle.outline} onchange={saveStyle} />
            </label>
            {#if signStyle.outline > 0}<input type="color" bind:value={signStyle.outlineColor} onchange={saveStyle} title="Outline colour" />{/if}
            <button class="small" onclick={() => { signStyle = { ...signStyle, ownColor: false, size: 1, outline: 0, outlineColor: '#000000' }; saveStyle(); if (sign.text.font) pickFont('|'); }}
              disabled={!sign.text.font && !sign.text.color && sign.text.size === 1 && !sign.text.outline}>Game look</button>
          </div>
        {/if}
        <div class="sign-preview">
          <div class="row"><span class="muted small">Preview (daylight; long names shrink to fit, like in the game)</span>
            {#if sign.showName}<input class="small" bind:value={previewName} placeholder="Shop name" title="Text shown in the preview" />{/if}</div>
          <SignPreview layoutUrl={sign.layout} image={sign.image} crop={sign.crop} aspect={sign.aspect} showName={sign.showName} name={previewName}
            style={{ fontUrl: sign.text.fontUrl, color: signStyle?.ownColor ? signStyle.color : '', size: signStyle?.size ?? 1,
              outline: signStyle?.outline ?? 0, outlineColor: signStyle?.outlineColor ?? '#000000' }} />
        </div>
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
  .sign-text { display: flex; flex-wrap: wrap; align-items: flex-end; gap: 8px 14px; margin: 6px 0; }
  .sign-preview { display: flex; flex-direction: column; gap: 4px; margin-top: 6px; }
  .col { display: flex; flex-direction: column; gap: 6px; }
  .small { font-size: 12px; margin: 0; }
  input[type="color"] { width: 48px; height: 32px; padding: 2px; }
</style>
