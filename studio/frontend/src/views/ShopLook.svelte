<script lang="ts">
  // Game look: the sign above the entrance, the open/closed sign at the door, the phone wallpaper, the global card back, the
  // customers' credit card, the loading screen and the main menu. Every edit goes into the page's draft (the previews follow
  // it); nothing reaches the game until Save, which stores the whole draft as mod settings in one go (App.SaveShopLook) — a
  // running game then applies it right away. No autosave (user's choice: overlapping saves lost edits and brought back older
  // door sign drafts).
  import { onMount } from 'svelte';
  import { leaveGuard } from '../lib/leaveGuard';
  import { Ev } from '../lib/events';
  import { App, EventsOn, ask, errText } from '../lib/api';
  import type { Picture, TextStyleValue } from '../lib/shopLook';
  import PictureSlot from './PictureSlot.svelte';
  import PictureList from './PictureList.svelte';
  import SignPreview from './SignPreview.svelte';
  import ScreenPreview from './ScreenPreview.svelte';
  import DoorSignPreview from './DoorSignPreview.svelte';
  import CreditCardPreview from './CreditCardPreview.svelte';
  import DoorSideEditor from './DoorSideEditor.svelte';
  import FontPicker from './FontPicker.svelte';
  import TextStyle from './TextStyle.svelte';

  let { notify }: { notify: (t: string, k?: string) => void } = $props();

  let dirty = $state(false); // unsaved changes: leaving the page asks first
  $effect(() => leaveGuard(() => dirty, 'Leave Game look without saving? Your unsaved changes will be lost.'));

  let look = $state<any>(null);
  let error = $state('');
  let fonts = $state<any[]>([]);
  async function load() {
    try { look = await App.ShopLook(); error = ''; } catch (e) { look = null; error = errText(e); }
    try { fonts = await App.ShopSignFonts(); } catch { fonts = []; }
  }

  // Picture slots edited with one crop box each, and the list slots (app_shoplook.go shopPictures).
  const SINGLE = ['sign', 'wallpaper', 'menu', 'loadCard'] as const;
  const LISTS = ['loading', 'card'] as const;
  // The card back: its source picture (file/url, '' = none) and crop; result = the stored card back (shown when its
  // picture is unknown: made before Studio kept it).
  type Back = Picture & { result: string };
  type Draft = Record<(typeof SINGLE)[number], Picture> & Record<(typeof LISTS)[number], Picture[]> & {
    showName: boolean; shopName: string; font: { file: string; name: string; url: string }; style: TextStyleValue; back: Back;
    options: Record<string, boolean>; choices: Record<string, string>;
  };
  const pic = (p: any): Picture => ({ file: p.file, url: p.image, crop: p.crop ?? null });
  function stored(l: any): Draft {
    const t = l.sign.text;
    return {
      sign: pic(l.pictures.sign), wallpaper: pic(l.pictures.wallpaper), menu: pic(l.pictures.menu), loadCard: pic(l.pictures.loadCard),
      loading: (l.pictures.loading.items ?? []).map(pic), card: (l.pictures.card.items ?? []).map(pic),
      showName: l.sign.showName, shopName: l.sign.shopName ?? '',
      font: { file: t.fontFile, name: t.font, url: t.fontUrl },
      style: { color: t.color, size: t.size, outline: t.outline, outlineColor: t.outlineColor || '#000000' },
      back: { file: l.cardBack.file, url: l.cardBack.source, result: l.cardBack.image, crop: l.cardBack.crop ?? null },
      options: { ...l.options }, choices: { ...l.choices },
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

  // One tab per part of the game. Every tab stays mounted (only hidden), so switching keeps each tab's unsaved edits; Save
  // stores them all. `fields` = the draft fields a tab edits (its unsaved-changes dot).
  const TABS: { id: string; title: string; fields: (keyof Draft)[] }[] = [
    { id: 'sign', title: 'Shop sign', fields: ['sign', 'showName', 'shopName', 'font', 'style'] },
    { id: 'door', title: 'Open / closed sign', fields: [] },
    { id: 'wallpaper', title: 'Phone wallpaper', fields: ['wallpaper'] },
    { id: 'back', title: 'Card back', fields: ['back'] },
    { id: 'card', title: 'Credit card', fields: ['card', 'options'] },
    { id: 'loading', title: 'Loading screen', fields: ['loading', 'loadCard', 'choices'] },
    { id: 'menu', title: 'Main menu', fields: ['menu'] },
  ];
  let tab = $state<string>((() => { try { return localStorage.getItem('shoplook.tab') || 'sign'; } catch { return 'sign'; } })());
  function setTab(id: string) {
    tab = id;
    try { localStorage.setItem('shoplook.tab', id); } catch {}
  }
  /** Unsaved changes in one tab (a dot on its button). */
  function tabDirty(id: string): boolean {
    if (!draft || !look) return false;
    if (id === 'door') return doorDirty.open || doorDirty.closed;
    const st = stored(look), fields = TABS.find((t) => t.id === id)?.fields ?? [];
    return fields.some((f) => !same(draft![f], st[f]));
  }

  let saving = $state(false);
  async function save() {
    if (!draft || !look || saving) return;
    const st = stored(look);
    const req: any = { pictures: {} };
    for (const slot of SINGLE) {
      if (!same(draft[slot], st[slot])) req.pictures[slot] = { file: draft[slot].file, crop: draft[slot].crop ?? [] };
    }
    for (const slot of LISTS) {
      if (!same(draft[slot], st[slot])) req.pictures[slot] = { items: draft[slot].map((p) => ({ file: p.file, crop: p.crop ?? [] })) };
    }
    if (!same(draft.options, st.options)) req.options = draft.options;
    if (!same(draft.choices, st.choices)) req.choices = draft.choices;
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
      notify('Game look saved.', 'ok');
    } catch (e) { notify(errText(e), 'error'); }
    saving = false;
  }

  async function discard() {
    if (!(await ask('Throw away your unsaved Game look changes (in every tab)?'))) return;
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

  const importPicture = (slot: string) => () => App.ImportShopPicture(slot);

  let backColor = $state('#5b3a8c');
  let backTitle = $state('');
  async function generateBack() {
    try {
      const f = await App.GenerateCardBack(backColor, backTitle);
      if (draft) draft.back = { file: f.file, url: f.url, result: '', crop: f.crop };
    } catch (e) { notify(errText(e), 'error'); }
  }
  // Any change in the card back's slot replaces the stored result (a new picture, or none).
  const setBack = (v: Picture) => { if (draft) draft.back = { ...v, result: '' }; };

  function gameLook() {
    if (!draft) return;
    draft.style = { color: '', size: 1, outline: 0, outlineColor: '#000000' };
    draft.font = { file: '', name: '', url: '' };
  }

  let loadingShown = $state(0); // the loading picture in the crop box and preview
  let cardShown = $state(0); // the credit card design in the crop box and preview

  /** How the loading screen preview draws the spinning card. */
  function loadingCard() {
    const mode = draft?.choices.loadingCard;
    if (mode === 'Hidden') return { show: 'none' as const };
    if (mode === 'Picture' && draft?.loadCard.file)
      return { show: 'picture' as const, image: draft.loadCard.url, crop: draft.loadCard.crop, aspect: look?.pictures.loadCard.aspect };
    if (mode === 'GlobalCardBack' && look?.cardBack.image)
      return { show: 'picture' as const, image: look.cardBack.image, crop: null, aspect: look.pictures.loadCard.aspect };
    return { show: 'outline' as const, label: mode === 'RandomFromMySets' ? 'a random card' : 'vanilla card back' };
  }

  onMount(() => {
    load();
    return EventsOn(Ev.TemplatesReady, load);
  });
</script>

<div class="page">
  <header class="row"><h2 class="grow">Game look</h2></header>
  <p class="muted small">Your own pictures for the shop and the game's screens. Choose any image, then drag the box to pick the part
    that's shown (drag a corner to resize it). The previews follow your edits; click <b>Save</b> to put them in the game (a running
    game shows them right away). Save stores the changes of every tab.</p>
  {#if look && draft}
    {@const pics = look.pictures}
    {@const lay = look.layouts}
    <div class="row tabs">
      {#each TABS as tb}
        <button class:on={tab === tb.id} onclick={() => setTab(tb.id)}>{tb.title}{#if tabDirty(tb.id)}<span class="dot" title="Unsaved changes">•</span>{/if}</button>
      {/each}
    </div>
    <div class="body">
    <section hidden={tab !== 'sign'}>
      <p class="muted">The sign above your shop's entrance. It glows at night like the vanilla sign.</p>
      <PictureSlot bind:value={draft.sign} aspect={pics.sign.aspect} pick={importPicture('sign')} label={draft.showName ? 'Shop name' : ''} {notify} />
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
        <SignPreview layoutUrl={lay.sign ?? ''} image={draft.sign.url} crop={draft.sign.crop} aspect={pics.sign.aspect} showName={draft.showName} name={draft.shopName || 'My Card Shop'}
          style={{ fontUrl: draft.font.url, color: draft.style.color, size: draft.style.size,
            outline: draft.style.outline, outlineColor: draft.style.outlineColor }} />
      </div>
    </section>
    <section hidden={tab !== 'door'}>
      <p class="muted">The sign you flip at the door. Give each side a board colour or your own picture, and write your words on
        top. It's drawn inside the black frame; the frame and screws stay.</p>
      <div class="sides">
        {#each [['open', 'OPEN side'], ['closed', 'CLOSED side']] as [side, title] (side)}
          <div class="side">
            <h4>{title}</h4>
            <DoorSignPreview layoutUrl={lay.door ?? ''} side={side as 'open' | 'closed'} image={doorRemoved[side] ? '' : pics[side].image}
              crop={pics[side].crop} aspect={pics[side].aspect} drawn={boards[side]} />
            <DoorSideEditor bind:this={editors[side]} side={side as 'open' | 'closed'} saved={pics[side].design} custom={!!pics[side].image}
              aspect={pics[side].aspect} {fonts} {notify} bind:board={boards[side]} bind:dirty={doorDirty[side]} bind:removed={doorRemoved[side]} />
          </div>
        {/each}
      </div>
    </section>
    <section hidden={tab !== 'wallpaper'}>
      <p class="muted">The background of your phone's home screen.</p>
      <div class="pair">
        <PictureSlot bind:value={draft.wallpaper} aspect={pics.wallpaper.aspect} pick={importPicture('wallpaper')} {notify} />
        <ScreenPreview frame="phone" layoutUrl={lay.phone ?? ''} image={draft.wallpaper.url} crop={draft.wallpaper.crop} aspect={pics.wallpaper.aspect} />
      </div>
    </section>
    <section hidden={tab !== 'back'}>
      <p class="muted">The back of every card from your custom sets. Vanilla sets keep their own backs. Choose any image, then drag
        the box to pick the part that's shown (drag a corner to resize it); it starts inside plain borders, like a scan's padding.
        The game keeps its own rounded edge around it and shows a new back after a restart.</p>
      <p class="muted">A set can still have its own back instead: open the set on the <b>Sets</b> page and choose it on its
        <b>Set</b> tab. Sets without one use this back (or, when there's none here, their frame's back).</p>
      <PictureSlot bind:value={() => draft!.back, setBack} aspect={look.cardBack.aspect} pick={() => App.ImportCardBack()}
        fallback={draft.back.result} {notify}>
        {#snippet extra()}
          <span class="gen"><input type="color" bind:value={backColor} /><input placeholder="Title (optional)" bind:value={backTitle} />
            <button onclick={generateBack}>Generate</button></span>
        {/snippet}
      </PictureSlot>
      {#if !draft.back.file && draft.back.result}<p class="muted small">Made before Studio kept the picture: choose it again to move or resize the part that's shown.</p>{/if}
    </section>
    <section hidden={tab !== 'card'}>
      <p class="muted">The card your customers pay with. Add several designs: each time a customer pays by card, they hand you a
        random one (the card at the counter shows the same). Your picture covers the card's printed side; keep the chip and the
        embossed number, expiry date and name on top, or show the picture alone.</p>
      <div class="pair">
        <div class="col grow">
          <PictureList bind:items={draft.card} bind:selected={cardShown} aspect={pics.card.aspect} pick={importPicture('card')} {notify}
            empty="No designs: customers pay with the game's own card. Add one or more; each payment picks one at random." />
          <label><input type="checkbox" bind:checked={draft.options.cardKeepChip} /> Keep the chip</label>
          <label><input type="checkbox" bind:checked={draft.options.cardKeepPrint} /> Keep the embossed number, expiry date and name</label>
        </div>
        <div class="col grow">
          <CreditCardPreview layoutUrl={lay.card ?? ''} image={draft.card[cardShown]?.url ?? ''} crop={draft.card[cardShown]?.crop ?? null} aspect={pics.card.aspect}
            keepChip={draft.options.cardKeepChip} keepPrint={draft.options.cardKeepPrint} />
        </div>
      </div>
    </section>
    <section hidden={tab !== 'loading'}>
      <p class="muted">The screen shown while the game loads your shop. Add several pictures: each time it opens, the game picks one
        at random. Its "Now Loading…" text, the percent and the spinning card stay on top.</p>
      <PictureList bind:items={draft.loading} bind:selected={loadingShown} aspect={pics.loading.aspect} pick={importPicture('loading')} {notify}
        empty="No pictures: the game shows its black screen. Add one or more; each time it opens the game picks one at random." />
      <div class="row card-mode">
        <label class="field">Spinning card
          <select bind:value={draft.choices.loadingCard}>
            <option value="RandomFromMySets">A random card from my sets</option>
            <option value="GlobalCardBack">Global card back</option>
            <option value="Picture">My own picture</option>
            <option value="Vanilla">Vanilla game card back</option>
            <option value="Hidden">No card</option>
          </select>
        </label>
        {#if draft.choices.loadingCard === 'RandomFromMySets'}<span class="muted small">Picked each time the screen opens (full-card scans first).</span>
        {:else if draft.choices.loadingCard === 'GlobalCardBack'}<span class="muted small">{look.cardBack.image ? 'The back from the Card back tab.' : 'No global card back yet (Card back tab): the vanilla back shows.'}</span>{/if}
      </div>
      {#if draft.choices.loadingCard === 'Picture'}
        <PictureSlot bind:value={draft.loadCard} aspect={pics.loadCard.aspect} pick={importPicture('loadCard')} {notify} />
      {/if}
      <ScreenPreview layoutUrl={lay.loading ?? ''} image={draft.loading[loadingShown]?.url ?? ''} crop={draft.loading[loadingShown]?.crop ?? null}
        aspect={pics.loading.aspect} card={loadingCard()} />
    </section>
    <section hidden={tab !== 'menu'}>
      <p class="muted">The background of the main menu. The logo and buttons stay on top.</p>
      <PictureSlot bind:value={draft.menu} aspect={pics.menu.aspect} pick={importPicture('menu')} {notify} />
      <ScreenPreview layoutUrl={lay.menu ?? ''} image={draft.menu.url} crop={draft.menu.crop} aspect={pics.menu.aspect} />
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
  .col { display: flex; flex-direction: column; gap: 8px; min-width: 260px; }
  .gen { display: inline-flex; gap: 6px; align-items: center; }
  input[type="color"] { width: 48px; height: 32px; padding: 2px; }
  .card-mode { align-items: flex-end; gap: 12px; }
  .sign-text { display: flex; flex-wrap: wrap; align-items: flex-end; gap: 8px 14px; margin: 6px 0; }
  .sign-preview { display: flex; flex-direction: column; gap: 4px; margin-top: 6px; }
  .pair { display: flex; flex-wrap: wrap; gap: 16px; align-items: flex-start; justify-content: space-between; }
  .sides { display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap: 16px; }
  .side { display: flex; flex-direction: column; gap: 8px; min-width: 0; }
  .small { font-size: 12px; margin: 0; }
</style>
