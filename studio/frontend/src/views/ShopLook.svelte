<script lang="ts">
  // Shop look: the sign above the entrance, the phone wallpaper and the open/closed sign at the door. Everything is stored as mod
  // settings, so a running game applies changes right away.
  import { onMount } from 'svelte';
  import { App, EventsOn, errText } from '../lib/api';
  import PictureSlot from './PictureSlot.svelte';
  import SignPreview from './SignPreview.svelte';
  import PhonePreview from './PhonePreview.svelte';
  import DoorSignPreview from './DoorSignPreview.svelte';
  import DoorSideEditor from './DoorSideEditor.svelte';
  import FontPicker from './FontPicker.svelte';
  import TextStyle from './TextStyle.svelte';
  import type { TextStyleValue } from '../lib/shopLook';

  let { notify }: { notify: (t: string, k?: string) => void } = $props();

  let look = $state<any>(null);
  let error = $state('');
  async function load() {
    try { look = await App.ShopLook(); error = ''; } catch (e) { look = null; error = errText(e); }
    try { fonts = await App.ShopSignFonts(); } catch { fonts = []; }
  }
  async function run(f: () => Promise<any>) {
    try { look = await f(); } catch (e) { notify(errText(e), 'error'); }
  }
  const onlook = (l: any) => (look = l);

  // Door sign sides: designed here (background + words) and saved as the side's picture.
  let drafts = $state<Record<string, HTMLCanvasElement | null>>({ open: null, closed: null });

  // Shop name style: edited here (the preview follows every move), saved when a control is released.
  let signStyle = $state<TextStyleValue | null>(null);
  let previewName = $state('');
  let fonts = $state<any[]>([]);
  $effect(() => {
    if (!look) return;
    const t = look.sign.text;
    signStyle = { color: t.color, size: t.size, outline: t.outline, outlineColor: t.outlineColor || '#000000' };
  });
  $effect(() => { if (look && !previewName) previewName = look.sign.shopName || 'My Card Shop'; });
  function saveStyle() {
    const s = signStyle;
    if (s) run(() => App.SetShopSignText({ ...look.sign.text, ...s }));
  }
  const pickFont = (source: string, id: string) => run(() => App.SetShopSignFont(source, id));

  onMount(() => {
    load();
    return EventsOn('templates:ready', load);
  });
</script>

<div class="page">
  <h2>Shop look</h2>
  <p class="muted">Your own pictures for the shop. Choose any image, then drag the box to pick the part that's shown (drag a corner to
    resize it). Changes show in a running game right away.</p>
  {#if look}
    {@const sign = look.sign}
    {@const pics = look.pictures}
    <section>
      <h3>Shop sign</h3>
      <p class="muted">The sign above your shop's entrance. It glows at night like the vanilla sign.</p>
      <PictureSlot slot="sign" picture={pics.sign} label={sign.showName ? 'Shop name' : ''} {notify} {onlook} />
      <label><input type="checkbox" checked={sign.showName} onchange={(ev) => run(() => App.SetShopSignShowName(ev.currentTarget.checked))} /> Show the shop name on the sign
        <span class="muted small">(turn off when your picture already has a name or logo)</span></label>
      {#if sign.showName && signStyle}
        <div class="sign-text">
          <FontPicker {fonts} current={sign.text.font} defaultLabel="Game default (Fredoka One)" onpick={pickFont} />
          <TextStyle bind:style={signStyle} defaultColor="#ffd23c" defaultLabel="game gold" oncommit={saveStyle} />
          <button class="small" onclick={() => { signStyle = { color: '', size: 1, outline: 0, outlineColor: '#000000' }; saveStyle(); if (sign.text.font) pickFont('', ''); }}
            disabled={!sign.text.font && !sign.text.color && sign.text.size === 1 && !sign.text.outline}>Game look</button>
        </div>
      {/if}
      <div class="sign-preview">
        <div class="row"><span class="muted small">Preview (daylight; long names shrink to fit, like in the game)</span>
          {#if sign.showName}<input class="small" bind:value={previewName} placeholder="Shop name" title="Text shown in the preview" />{/if}</div>
        <SignPreview layoutUrl={look.signLayout} image={pics.sign.image} crop={pics.sign.crop} aspect={pics.sign.aspect} showName={sign.showName} name={previewName}
          style={{ fontUrl: sign.text.fontUrl, color: signStyle?.color ?? '', size: signStyle?.size ?? 1,
            outline: signStyle?.outline ?? 0, outlineColor: signStyle?.outlineColor ?? '#000000' }} />
      </div>
    </section>
    <section>
      <h3>Phone wallpaper</h3>
      <p class="muted">The background of your phone's home screen.</p>
      <div class="pair">
        <PictureSlot slot="wallpaper" picture={pics.wallpaper} {notify} {onlook} />
        <PhonePreview layoutUrl={look.phoneLayout} image={pics.wallpaper.image} crop={pics.wallpaper.crop} aspect={pics.wallpaper.aspect} />
      </div>
    </section>
    <section>
      <h3>Open / closed sign</h3>
      <p class="muted">The sign you flip at the door. Give each side a board colour or your own picture, and write your words on
        top. It's drawn inside the black frame; the frame and screws stay.</p>
      <div class="sides">
        {#each [['open', 'OPEN side'], ['closed', 'CLOSED side']] as [side, title] (side)}
          <div class="side">
            <h4>{title}</h4>
            <DoorSignPreview layoutUrl={look.doorLayout} side={side as 'open' | 'closed'} image={pics[side].image} crop={pics[side].crop} aspect={pics[side].aspect}
              drawn={drafts[side]} />
            <DoorSideEditor side={side as 'open' | 'closed'} saved={pics[side].design} custom={!!pics[side].image} aspect={pics[side].aspect}
              {fonts} {notify} {onlook} bind:board={drafts[side]} />
          </div>
        {/each}
      </div>
    </section>
  {:else}
    <p class="muted small">{error || 'Loading…'}</p>
  {/if}
</div>

<style>
  .page { padding: 20px 24px; overflow: auto; height: 100%; display: flex; flex-direction: column; gap: 18px; }
  .page > p { margin: 0; max-width: 900px; }
  section { background: var(--panel); border: 1px solid var(--line); border-radius: var(--radius); padding: 14px; display: flex; flex-direction: column; gap: 10px; max-width: 900px; }
  h4 { margin: 0; }
  .sign-text { display: flex; flex-wrap: wrap; align-items: flex-end; gap: 8px 14px; margin: 6px 0; }
  .sign-preview { display: flex; flex-direction: column; gap: 4px; margin-top: 6px; }
  .pair { display: flex; flex-wrap: wrap; gap: 16px; align-items: flex-start; justify-content: space-between; }
  .sides { display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap: 16px; }
  .side { display: flex; flex-direction: column; gap: 8px; min-width: 0; }
  .small { font-size: 12px; margin: 0; }
</style>
