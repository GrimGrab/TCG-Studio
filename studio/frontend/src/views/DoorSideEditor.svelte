<script lang="ts">
  // One side of the door sign: a background (board colour or your own picture, cropped) with your words on top. The draft is
  // drawn into `board` (the preview shows it as you edit). Nothing is stored here: the page's Save takes `pending()` (the drawn
  // picture + its design, or going back to the game's sign) and sends it with the rest of the page (App.SaveShopLook).
  import { App, errText } from '../lib/api';
  import { shopSignFile } from '../lib/urls';
  import { loadImage } from '../lib/accessoryArt';
  import { doorSignDefaults, drawSignText, fontFamily, latestPerFrame, type Picture, type SignTextDesign } from '../lib/shopLook';
  import PictureSlot from './PictureSlot.svelte';
  import FontPicker from './FontPicker.svelte';
  import TextStyle from './TextStyle.svelte';

  let { side, saved = '', custom = false, aspect, fonts, notify, board = $bindable(null), dirty = $bindable(false),
    removed = $bindable(false) }: {
    side: 'open' | 'closed';
    custom?: boolean; // the side has a picture now (designed or not)
    saved?: string; // the stored design (JSON), '' = none
    aspect: number;
    fonts: { source: string; id: string; name: string }[];
    notify: (t: string, k?: string) => void;
    board?: HTMLCanvasElement | null; // the drawn draft for the preview; null = show what the side has now
    dirty?: boolean; // out: the side has unsaved changes
    removed?: boolean; // out: the draft goes back to the game's own sign
  } = $props();

  const parse = (json: string): SignTextDesign => {
    try { return { ...doorSignDefaults[side], ...JSON.parse(json) }; } catch { return { ...doorSignDefaults[side] }; }
  };
  let draft = $state<SignTextDesign>(parse(''));
  $effect(() => { draft = parse(saved); }); // a new stored design (after the page's Save) replaces the draft
  const changed = $derived(JSON.stringify(draft) !== JSON.stringify(parse(saved)));
  $effect(() => { dirty = removed ? custom : changed; });
  const url = (file: string) => shopSignFile(file);

  // The chosen font and picture, once loaded (file = what they were loaded for; `pending` waits until they match the draft).
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
    if (removed || (!saved && !changed)) { board = null; return; } // the preview shows the side as stored / the game's sign
    later(() => (board = render(656)));
  });

  /** What the page's Save stores for this side: the drawn picture + design, file '' = the game's sign; null = the chosen font
   *  or picture is still loading. Only meaningful while `dirty`. */
  export function pending(): { file?: string; png?: string; design?: string } | null {
    if (removed) return { file: '' };
    if (!ready) return null;
    return { png: render(1312).toDataURL('image/png').split(',')[1], design: JSON.stringify(draft) };
  }

  /** Drops the unsaved changes. */
  export function discard() {
    draft = parse(saved);
    removed = false;
  }

  async function pickFont(source: string, id: string) {
    if (!source) { draft.font = null; return; }
    try {
      const f = await App.ImportShopFont(source, id);
      if (f.file) draft.font = { file: f.file, name: f.name };
    } catch (e) { notify(errText(e), 'error'); }
  }

  // The background picture as the crop box slot sees it; removing it goes back to the board colour.
  const background = (): Picture => draft.background ? { file: draft.background.file, url: url(draft.background.file), crop: draft.background.crop }
    : { file: '', url: '', crop: null };
  const setBackground = (v: Picture) => { draft.background = v.file ? { file: v.file, crop: v.crop } : null; };
</script>

<div class="editor">
  {#if removed}
    <p class="muted small">This side goes back to the game's own sign when you save.</p>
    <div class="row"><button onclick={() => (removed = false)}>Keep my design</button></div>
  {:else}
    <div class="row wrap">
      <span class="muted small">Background</span>
      {#if !draft.background}<label>Board colour <input type="color" bind:value={draft.boardColor} /></label>{/if}
    </div>
    <PictureSlot {aspect} {notify} pick={() => App.ImportDoorBackground(side)} chooseLabel={draft.background ? 'Other picture…' : 'Picture…'}
      removeLabel="Use a colour" bind:value={background, setBackground} />
    <textarea rows="3" bind:value={draft.text} placeholder="Your words, one line per row (empty = picture only)"></textarea>
    <div class="row wrap">
      <FontPicker {fonts} current={draft.font?.name ?? ''} defaultLabel="Default (Impact)" onpick={pickFont} />
      <TextStyle bind:style={draft} defaultColor="#ffffff" defaultLabel="white" />
    </div>
    {#if custom}<div class="row"><button class="danger" onclick={() => { draft = parse(saved); removed = true; }}>Remove</button></div>{/if}
  {/if}
</div>

<style>
  .editor { display: flex; flex-direction: column; gap: 8px; }
  textarea { font: inherit; resize: vertical; }
  .wrap { flex-wrap: wrap; align-items: flex-end; gap: 8px 12px; }
  label { display: flex; align-items: center; gap: 6px; }
  input[type="color"] { width: 48px; height: 32px; padding: 2px; }
  .small { font-size: 12px; margin: 0; }
</style>
