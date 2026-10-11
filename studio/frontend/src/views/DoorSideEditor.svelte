<script lang="ts">
  // One side of the door sign: a background (board colour or your own picture, cropped) with your words on top. The draft is
  // drawn into `board` (the preview shows it as you edit) and saved by itself shortly after the last change, like the shop sign's
  // settings: as the side's picture plus its design (App.SaveShopPictureText), so the mod paints it like any picture.
  import { App, errText } from '../lib/api';
  import { loadImage } from '../lib/accessoryArt';
  import { doorSignDefaults, drawSignText, fontFamily, latestPerFrame, type SignTextDesign } from '../lib/shopLook';
  import CropBox from './CropBox.svelte';
  import FontPicker from './FontPicker.svelte';
  import TextStyle from './TextStyle.svelte';

  let { side, saved = '', custom = false, aspect, fonts, notify, onlook, board = $bindable(null) }: {
    side: 'open' | 'closed';
    custom?: boolean; // the side has a picture now (designed or not)
    saved?: string; // the stored design (JSON), '' = none
    aspect: number;
    fonts: { source: string; id: string; name: string }[];
    notify: (t: string, k?: string) => void;
    onlook: (look: any) => void;
    board?: HTMLCanvasElement | null; // the drawn draft for the preview; null = show what the side has now
  } = $props();

  const parse = (json: string): SignTextDesign => {
    try { return { ...doorSignDefaults[side], ...JSON.parse(json) }; } catch { return { ...doorSignDefaults[side] }; }
  };
  let draft = $state<SignTextDesign>(parse(''));
  let sent = ''; // the design this editor saved last: its own save coming back must not reset what is being typed
  $effect(() => { if (saved !== sent) draft = parse(saved); });
  const changed = $derived(JSON.stringify(draft) !== JSON.stringify(parse(saved)));
  const url = (file: string) => '/shopsign/' + encodeURIComponent(file);

  // The chosen font and picture, once loaded (file = what they were loaded for; a save waits until they match the draft).
  let font = $state({ file: '', family: '' });
  $effect(() => {
    const f = draft.font;
    if (!f) { font = { file: '', family: '' }; return; }
    fontFamily(url(f.file)).then((family) => (font = { file: f.file, family })).catch(() => (font = { file: f.file, family: '' }));
  });
  let pic = $state<{ file: string; img: HTMLImageElement | null }>({ file: '', img: null });
  $effect(() => {
    const b = draft.background;
    if (!b) { pic = { file: '', img: null }; return; }
    loadImage(url(b.file)).then((img) => (pic = { file: b.file, img })).catch(() => (pic = { file: b.file, img: null }));
  });
  const family = $derived(font.file === (draft.font?.file ?? '') ? font.family : '');
  const bg = $derived(pic.file === (draft.background?.file ?? '') ? pic.img : null);
  const ready = $derived(font.file === (draft.font?.file ?? '') && pic.file === (draft.background?.file ?? ''));

  function render(W: number) {
    const H = Math.round(W / aspect), cv = document.createElement('canvas');
    cv.width = W; cv.height = H;
    drawSignText(cv.getContext('2d')!, W, H, $state.snapshot(draft) as SignTextDesign, family, bg, aspect);
    return cv;
  }
  // The preview shows the board at its texture size (328 × 206): drawing it at 2× while editing keeps dragging a colour smooth;
  // the saved picture is drawn at 4× so the mod's down-sampling stays sharp.
  const later = latestPerFrame();
  $effect(() => {
    JSON.stringify(draft); family; bg; // redraw on any change
    if (!saved && !changed) { board = null; return; } // nothing designed yet: the preview shows the side as it is
    later(() => (board = render(656)));
  });

  // Save shortly after the last change (typing or dragging a colour fires many changes).
  $effect(() => {
    if (!changed || !ready) return;
    const t = setTimeout(save, 500);
    return () => clearTimeout(t);
  });

  async function pickFont(source: string, id: string) {
    if (!source) { draft.font = null; return; }
    try {
      const f = await App.ImportShopFont(source, id);
      if (f.file) draft.font = { file: f.file, name: f.name };
    } catch (e) { notify(errText(e), 'error'); }
  }

  async function pickBackground() {
    try {
      const f = await App.ImportDoorBackground(side);
      if (f.file) draft.background = { file: f.file, crop: null };
    } catch (e) { notify(errText(e), 'error'); }
  }

  async function save() {
    const design = JSON.stringify(draft);
    try {
      sent = design;
      onlook(await App.SaveShopPictureText(side, render(1312).toDataURL('image/png').split(',')[1], design));
    } catch (e) { sent = ''; notify(errText(e), 'error'); }
  }

  async function remove() {
    try { sent = ''; onlook(await App.RemoveShopPicture(side)); } catch (e) { notify(errText(e), 'error'); }
  }
</script>

<div class="editor">
  <div class="row wrap">
    <span class="muted small">Background</span>
    {#if draft.background}
      <button onclick={pickBackground}>Other picture…</button>
      <button onclick={() => draft.background && (draft.background = { ...draft.background, crop: null })} disabled={!draft.background.crop}>Centre</button>
      <button onclick={() => (draft.background = null)}>Use a colour</button>
    {:else}
      <label>Board colour <input type="color" bind:value={draft.boardColor} /></label>
      <button onclick={pickBackground}>Picture…</button>
    {/if}
  </div>
  {#if draft.background}
    <CropBox src={url(draft.background.file)} {aspect} crop={draft.background.crop}
      onchange={(c) => draft.background && (draft.background = { ...draft.background, crop: c })} />
  {/if}
  <textarea rows="3" bind:value={draft.text} placeholder="Your words, one line per row (empty = picture only)"></textarea>
  <div class="row wrap">
    <FontPicker {fonts} current={draft.font?.name ?? ''} defaultLabel="Default (Impact)" onpick={pickFont} />
    <TextStyle bind:style={draft} defaultColor="#ffffff" defaultLabel="white" />
  </div>
  {#if custom}<div class="row"><button class="danger" onclick={remove}>Remove</button></div>{/if}
</div>

<style>
  .editor { display: flex; flex-direction: column; gap: 8px; }
  textarea { font: inherit; resize: vertical; }
  .wrap { flex-wrap: wrap; align-items: flex-end; gap: 8px 12px; }
  label { display: flex; align-items: center; gap: 6px; }
  input[type="color"] { width: 48px; height: 32px; padding: 2px; }
</style>
