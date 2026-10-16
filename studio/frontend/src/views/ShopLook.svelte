<script lang="ts">
  // Shop look: the sign above the entrance, the phone wallpaper, the open/closed sign at the door and the global card back. Every edit goes into the
  // page's draft (the previews follow it); nothing reaches the game until Save, which stores the whole draft as mod settings in
  // one go (App.SaveShopLook) — a running game then applies it right away. No autosave (user's choice: overlapping saves lost
  // edits and brought back older door sign drafts).
  import { onMount } from 'svelte';
  import { leaveGuard } from '../lib/leaveGuard';
  import { Ev } from '../lib/events';
  import { App, EventsOn, ask, errText } from '../lib/api';
  import PictureSlot from './PictureSlot.svelte';
  import CropBox from './CropBox.svelte';
  import SignPreview from './SignPreview.svelte';
  import PhonePreview from './PhonePreview.svelte';
  import DoorSignPreview from './DoorSignPreview.svelte';
  import DoorSideEditor from './DoorSideEditor.svelte';
  import FontPicker from './FontPicker.svelte';
  import TextStyle from './TextStyle.svelte';
  import type { TextStyleValue } from '../lib/shopLook';

  let { notify }: { notify: (t: string, k?: string) => void } = $props();

  let dirty = $state(false); // unsaved changes: leaving the page asks first
  $effect(() => leaveGuard(() => dirty, 'Leave Shop look without saving? Your unsaved changes will be lost.'));

  let look = $state<any>(null);
  let error = $state('');
  let fonts = $state<any[]>([]);
  async function load() {
    try { look = await App.ShopLook(); error = ''; } catch (e) { look = null; error = errText(e); }
    try { fonts = await App.ShopSignFonts(); } catch { fonts = []; }
  }

  type Pic = { file: string; url: string; crop: number[] | null };
  // The card back: its source picture (file/src, '' = none) and crop; result = the stored card back (shown when its
  // picture is unknown: made before Studio kept it).
  type Back = { file: string; src: string; result: string; crop: number[] | null };
  type Draft = { sign: Pic; wallpaper: Pic; showName: boolean; shopName: string; font: { file: string; name: string; url: string }; style: TextStyleValue; back: Back };
  const pic = (p: any): Pic => ({ file: p.file, url: p.image, crop: p.crop ?? null });
  function stored(l: any): Draft {
    const t = l.sign.text;
    return {
      sign: pic(l.pictures.sign), wallpaper: pic(l.pictures.wallpaper), showName: l.sign.showName, shopName: l.sign.shopName ?? '',
      font: { file: t.fontFile, name: t.font, url: t.fontUrl },
      style: { color: t.color, size: t.size, outline: t.outline, outlineColor: t.outlineColor || '#000000' },
      back: { file: l.cardBack.file, src: l.cardBack.source, result: l.cardBack.image, crop: l.cardBack.crop ?? null },
    };
  }
  // The draft is replaced only when what's stored changes (after a Save), not when the page reloads the same state.
  let draft = $state<Draft | null>(null);
  let storedKey = '';
  $effect(() => {
    if (!look) return;
    const s = stored(look), key = JSON.stringify(s);
    if (key !== storedKey) { storedKey = key; draft = s; }
  });

  // Door sign sides: each editor keeps its own draft (background + words) and hands it to Save through `pending()`.
  const sides = ['open', 'closed'] as const;
  let boards = $state<Record<string, HTMLCanvasElement | null>>({ open: null, closed: null });
  let doorDirty = $state<Record<string, boolean>>({ open: false, closed: false });
  let doorRemoved = $state<Record<string, boolean>>({ open: false, closed: false });
  let editors = $state<Record<string, DoorSideEditor>>({});

  const same = (a: any, b: any) => JSON.stringify(a) === JSON.stringify(b);
  $effect(() => {
    dirty = !!draft && !!look && (!same(draft, stored(look)) || doorDirty.open || doorDirty.closed);
  });

  // One tab per part of the shop. Every tab stays mounted (only hidden), so switching keeps each tab's unsaved edits;
  // Save stores them all.
  const TABS = [{ id: 'sign', title: 'Shop sign' }, { id: 'wallpaper', title: 'Phone wallpaper' }, { id: 'door', title: 'Open / closed sign' },
    { id: 'back', title: 'Card back' }] as const;
  let tab = $state<string>((() => { try { return localStorage.getItem('shoplook.tab') || 'sign'; } catch { return 'sign'; } })());
  function setTab(id: string) {
    tab = id;
    try { localStorage.setItem('shoplook.tab', id); } catch {}
  }
  /** Unsaved changes in one tab (a dot on its button). */
  function tabDirty(id: string): boolean {
    if (!draft || !look) return false;
    const st = stored(look);
    if (id === 'sign') return !same([draft.sign, draft.showName, draft.shopName, draft.font, draft.style], [st.sign, st.showName, st.shopName, st.font, st.style]);
    if (id === 'wallpaper') return !same(draft.wallpaper, st.wallpaper);
    if (id === 'back') return !same(draft.back, st.back);
    return doorDirty.open || doorDirty.closed;
  }

  let saving = $state(false);
  async function save() {
    if (!draft || !look || saving) return;
    const st = stored(look);
    const req: any = { pictures: {} };
    for (const slot of ['sign', 'wallpaper'] as const) {
      if (!same(draft[slot], st[slot])) req.pictures[slot] = { file: draft[slot].file, crop: draft[slot].crop ?? [] };
    }
    if (draft.showName !== st.showName) req.showName = draft.showName;
    if (draft.shopName.trim() !== st.shopName) {
      if (!draft.shopName.trim()) { notify('Give your shop a name first.', 'error'); return; }
      req.shopName = draft.shopName;
    }
    if (draft.font.file !== st.font.file) req.font = draft.font.file;
    if (!same(draft.style, st.style)) req.text = { ...look.sign.text, ...draft.style };
    if (!same(draft.back, st.back)) req.cardBack = { file: draft.back.file, crop: draft.back.crop ?? [] };
    for (const side of sides) {
      if (!doorDirty[side]) continue;
      const p = editors[side]?.pending();
      if (!p) { notify('The door sign\'s font or picture is still loading — try again in a moment.', 'warn'); return; }
      req.pictures[side] = p;
    }
    saving = true;
    try {
      look = await App.SaveShopLook(req);
      for (const side of sides) editors[side]?.discard(); // the editors now start from the stored designs
      notify('Shop look saved.', 'ok');
    } catch (e) { notify(errText(e), 'error'); }
    saving = false;
  }

  async function discard() {
    if (!(await ask('Throw away your unsaved Shop look changes (in every tab)?'))) return;
    if (look) draft = stored(look);
    for (const side of sides) editors[side]?.discard();
  }

  async function pickFont(source: string, id: string) {
    if (!draft) return;
    if (!source) { draft.font = { file: '', name: '', url: '' }; return; }
    try {
      const f = await App.ImportShopFont(source, id);
      if (f.file && draft) draft.font = { file: f.file, name: f.name, url: f.url };
    } catch (e) { notify(errText(e), 'error'); }
  }

  let backColor = $state('#5b3a8c');
  let backTitle = $state('');
  async function chooseBack() {
    try {
      const f = await App.ImportCardBack();
      if (f.file && draft) draft.back = { file: f.file, src: f.url, result: '', crop: f.crop };
    } catch (e) { notify(errText(e), 'error'); }
  }
  async function generateBack() {
    try {
      const f = await App.GenerateCardBack(backColor, backTitle);
      if (draft) draft.back = { file: f.file, src: f.url, result: '', crop: f.crop };
    } catch (e) { notify(errText(e), 'error'); }
  }
  function removeBack() {
    if (draft) draft.back = { file: '', src: '', result: '', crop: null };
  }

  function gameLook() {
    if (!draft) return;
    draft.style = { color: '', size: 1, outline: 0, outlineColor: '#000000' };
    draft.font = { file: '', name: '', url: '' };
  }


  onMount(() => {
    load();
    return EventsOn(Ev.TemplatesReady, load);
  });
</script>

<div class="page">
  <header class="row"><h2 class="grow">Shop look</h2></header>
  <p class="muted small">Your own pictures for the shop. Choose any image, then drag the box to pick the part that's shown (drag a corner to
    resize it). The previews follow your edits; click <b>Save</b> to put them in the game (a running game shows them right away).
    Save stores the changes of every tab.</p>
  {#if look && draft}
    {@const pics = look.pictures}
    <div class="row tabs">
      {#each TABS as tb}
        <button class:on={tab === tb.id} onclick={() => setTab(tb.id)}>{tb.title}{#if tabDirty(tb.id)}<span class="dot" title="Unsaved changes">•</span>{/if}</button>
      {/each}
    </div>
    <div class="body">
    <section hidden={tab !== 'sign'}>
      <p class="muted">The sign above your shop's entrance. It glows at night like the vanilla sign.</p>
      <PictureSlot slot="sign" bind:value={draft.sign} aspect={pics.sign.aspect} label={draft.showName ? 'Shop name' : ''} {notify} />
      <label><input type="checkbox" bind:checked={draft.showName} /> Show the shop name on the sign
        <span class="muted small">(turn off when your picture already has a name or logo)</span></label>
      {#if draft.showName}
        <div class="sign-text">
          <FontPicker {fonts} current={draft.font.name} defaultLabel="Game default (Fredoka One)" onpick={pickFont} />
          <TextStyle bind:style={draft.style} defaultColor="#ffd23c" defaultLabel="game gold" />
          <button class="small" onclick={gameLook}
            disabled={!draft.font.file && !draft.style.color && draft.style.size === 1 && !draft.style.outline}>Game look</button>
        </div>
      {/if}
      <div class="sign-preview">
        <div class="row"><span class="muted small">Preview (daylight; long names shrink to fit, like in the game)</span>
          {#if draft.showName}<input class="small" bind:value={draft.shopName} placeholder="My Card Shop" disabled={!look.sign.shopName}
            title={look.sign.shopName ? "Your shop's name: Save renames it in your game saves (close the game first)" : 'Play the game once to get a save, then you can rename your shop here'} />{/if}</div>
        <SignPreview layoutUrl={look.signLayout} image={draft.sign.url} crop={draft.sign.crop} aspect={pics.sign.aspect} showName={draft.showName} name={draft.shopName || 'My Card Shop'}
          style={{ fontUrl: draft.font.url, color: draft.style.color, size: draft.style.size,
            outline: draft.style.outline, outlineColor: draft.style.outlineColor }} />
      </div>
    </section>
    <section hidden={tab !== 'wallpaper'}>
      <p class="muted">The background of your phone's home screen.</p>
      <div class="pair">
        <PictureSlot slot="wallpaper" bind:value={draft.wallpaper} aspect={pics.wallpaper.aspect} {notify} />
        <PhonePreview layoutUrl={look.phoneLayout} image={draft.wallpaper.url} crop={draft.wallpaper.crop} aspect={pics.wallpaper.aspect} />
      </div>
    </section>
    <section hidden={tab !== 'door'}>
      <p class="muted">The sign you flip at the door. Give each side a board colour or your own picture, and write your words on
        top. It's drawn inside the black frame; the frame and screws stay.</p>
      <div class="sides">
        {#each [['open', 'OPEN side'], ['closed', 'CLOSED side']] as [side, title] (side)}
          <div class="side">
            <h4>{title}</h4>
            <DoorSignPreview layoutUrl={look.doorLayout} side={side as 'open' | 'closed'} image={doorRemoved[side] ? '' : pics[side].image}
              crop={pics[side].crop} aspect={pics[side].aspect} drawn={boards[side]} />
            <DoorSideEditor bind:this={editors[side]} side={side as 'open' | 'closed'} saved={pics[side].design} custom={!!pics[side].image}
              aspect={pics[side].aspect} {fonts} {notify} bind:board={boards[side]} bind:dirty={doorDirty[side]} bind:removed={doorRemoved[side]} />
          </div>
        {/each}
      </div>
    </section>
    <section hidden={tab !== 'back'}>
      <p class="muted">The back of every card from your custom sets. Vanilla sets keep their own backs. Choose any image, then drag
        the box to pick the part that's shown (drag a corner to resize it); it starts inside plain borders, like a scan's padding.
        The game keeps its own rounded edge around it and shows a new back after a restart.</p>
      <p class="muted">A set can still have its own back instead: open the set on the <b>Sets</b> page and choose it on its
        <b>Set</b> tab. Sets without one use this back (or, when there's none here, their frame's back).</p>
      <div class="row back-row">
        {#if draft.back.src}
          <CropBox src={draft.back.src} aspect={look.cardBack.aspect} crop={draft.back.crop}
            onchange={(c) => { if (draft) draft.back = { ...draft.back, crop: c }; }} />
        {:else}
          <div class="back">{#if draft.back.result}<img src={draft.back.result} alt="global card back" />{:else}<span class="muted">none</span>{/if}</div>
        {/if}
        <div class="col">
          <button onclick={chooseBack}>Choose image…</button>
          <div class="row"><input type="color" bind:value={backColor} /><input placeholder="Title (optional)" bind:value={backTitle} /><button onclick={generateBack}>Generate</button></div>
          <div class="row">
            {#if draft.back.src}<button onclick={() => { if (draft) draft.back = { ...draft.back, crop: null }; }} disabled={!draft.back.crop}>Centre</button>{/if}
            {#if draft.back.src || draft.back.result}<button class="danger" onclick={removeBack}>Remove</button>{/if}
          </div>
          {#if !draft.back.src && draft.back.result}<p class="muted small">Made before Studio kept the picture: choose it again to move or resize the part that's shown.</p>{/if}
        </div>
      </div>
    </section>
    <div class="row savebar">
      <div class="grow"></div>
      {#if dirty}<span class="muted small">Unsaved changes</span>{/if}
      <button onclick={discard} disabled={!dirty || saving}>Revert</button>
      <button class="primary" onclick={save} disabled={!dirty || saving}>{saving ? 'Saving…' : 'Save'}</button>
    </div>
    </div>
  {:else}
    <p class="muted small">{error || 'Loading…'}</p>
  {/if}
</div>

<style>
  .page { padding: 16px 20px; height: 100%; display: flex; flex-direction: column; gap: 10px; overflow: hidden; }
  .page > p { margin: 0; max-width: 900px; }
  .tabs { border-bottom: 1px solid var(--line); padding-bottom: 8px; flex-wrap: wrap; }
  .tabs button.on { border-color: var(--accent); background: #22304d; }
  .dot { color: var(--accent-2); margin-left: 6px; }
  .body { flex: 1; min-height: 0; overflow: auto; display: flex; flex-direction: column; gap: 14px; }
  section { background: var(--panel); border: 1px solid var(--line); border-radius: var(--radius); padding: 14px; display: flex; flex-direction: column; gap: 10px; max-width: 900px; }
  section[hidden] { display: none; }
  h4 { margin: 0; }
  .savebar { position: sticky; bottom: 0; background: var(--bg); padding: 8px 0; max-width: 900px; }
  .grow { flex: 1; }
  .back-row { align-items: flex-start; gap: 14px; }
  .back { width: 130px; height: 130px; background: var(--bg); border-radius: 6px; display: flex; align-items: center; justify-content: center; overflow: hidden; }
  .back img { max-width: 100%; max-height: 100%; }
  .col { display: flex; flex-direction: column; gap: 8px; max-width: 360px; }
  input[type="color"] { width: 48px; height: 32px; padding: 2px; }
  .sign-text { display: flex; flex-wrap: wrap; align-items: flex-end; gap: 8px 14px; margin: 6px 0; }
  .sign-preview { display: flex; flex-direction: column; gap: 4px; margin-top: 6px; }
  .pair { display: flex; flex-wrap: wrap; gap: 16px; align-items: flex-start; justify-content: space-between; }
  .sides { display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap: 16px; }
  .side { display: flex; flex-direction: column; gap: 8px; min-width: 0; }
  .small { font-size: 12px; margin: 0; }
</style>
