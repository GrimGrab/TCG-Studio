<script lang="ts" module>
  type DetailTab = 'card' | 'look' | 'price' | 'play';
  // The open tab stays the same while going from card to card.
  let rememberedTab: DetailTab = 'card';
</script>

<script lang="ts">
  import { App, projectFile, rarityName, rarityColor, errText } from '../../lib/api';
  import CardInfoFields from './CardInfoFields.svelte';
  import CardPriceFields from './CardPriceFields.svelte';
  import CardPlayFields from './CardPlayFields.svelte';
  import BorderPreview from '../BorderPreview.svelte';
  import { loadLooks, noLooks, nameOf, versionFacts, shownImage, borderFor, shaderFor, rimFor, pictureShader } from '../../lib/cardLooks';
  import { cardVersions, hasOwnVersions, setVersionImage, versionImage, versionLabel, versionsWithArt, shaderSource, shownShader,
    ALL_VERSIONS, fallbackImage, shownBorder, isFullImage } from '../../lib/cardVersions';
  import Dropdown from '../Dropdown.svelte';
  import ChangeCardId from './ChangeCardId.svelte';
  import { renameCard } from '../../lib/cardId';
  import type { Goto } from '../../lib/setNav';

  // showAs: the version the preview (and the Cards grid) shows, '' = the main picture; shared with the Cards tab, so it
  // stays while you go from card to card.
  // onopen: open the full card editor (CardEditor) over the Cards tab.
  let { project, card, imgBust, notify, ondelete, onrenamed, goto, onopen, showAs = $bindable('') }: {
    project: any; card: any; imgBust: number; notify: (t: string, k?: string) => void;
    ondelete: () => void; onrenamed?: (oldId: string, newId: string) => void; goto?: Goto; onopen?: () => void; showAs?: string;
  } = $props();

  // Change id (click the id): a dialog with what it does; the set's own references follow (lib/cardId).
  let changingId = $state(false);
  function changeId(newId: string) {
    const old = card.id;
    changingId = false;
    if (renameCard(project, old, newId)) {
      onrenamed?.(old, newId);
      notify(`Card id changed to ${newId}: save the set to use it`, 'ok');
    }
  }

  const meta = $derived(project.meta?.cards?.[card.id]);

  async function changeImage() {
    try {
      const rel = await App.PickCardImage(project.id);
      if (rel) card.image = rel;
    } catch (e) { notify(errText(e), 'error'); }
  }

  async function changeBackImage() {
    try {
      const rel = await App.PickCardImage(project.id);
      if (rel) { card.backImage = rel; showBack = true; }
    } catch (e) { notify(errText(e), 'error'); }
  }

  function removeBackImage() {
    card.backImage = undefined;
    showBack = false;
  }

  // Double-faced cards: which side the preview shows (back faces come from the Scryfall import / "Download back faces").
  let showBack = $state(false);
  let rotating = $state(false);
  // Turns the picture the preview shows (back, a version's own picture or the main image).
  async function rotate(turns: number) {
    rotating = true;
    try {
      const back = showBack && !!card.backImage;
      const rel = previewImage;
      // The version whose own picture is shown (a version without one shows another version's or the main picture).
      const owner = back || rel === card.image ? undefined : ALL_VERSIONS.find((k) => versionImage(card, k) === rel);
      const turned = await App.RotateCardImages(project.id, [rel], turns, false);
      if (turned[rel]) { // a new file: the game sees it once the set is saved
        if (back) card.backImage = turned[rel];
        else if (owner) setVersionImage(card, owner, turned[rel]);
        else card.image = turned[rel];
      }
    } catch (e) { notify(errText(e), 'error'); }
    rotating = false;
  }

  // The card's look is edited in one place, the card editor (onopen); the panel shows the version picked in "Preview as"
  // (showAs) as the game draws it: its picture with its shader, in its border.
  const versionsOn = $derived(cardVersions(project.set, card));
  const withArt = $derived(versionsWithArt(card));
  const previewImage = $derived(showBack && card.backImage ? card.backImage : shownImage(card, showAs));

  let looks = $state(noLooks());
  $effect(() => { loadLooks().then((l) => (looks = l)); });
  const shaders = $derived(looks.shaders);
  const borderName = (id: string) => nameOf(looks.borders, id, 'Vanilla rim', 'border');
  const shaderName = (id: string) => nameOf(looks.shaders, id, 'No shader', 'shader');
  // The main picture is the Base version: it shows Base's border (or the game's rim), like the Cards tab's tiles.
  const previewBorder = $derived(showBack ? null : borderFor(looks, project.set, card, showAs));
  const gameRim = $derived(showBack || previewBorder ? undefined : rimFor(looks, project.set, showAs));
  const liveShader = $derived(showBack || !showAs ? undefined : shaderFor(looks, project.set, card, showAs));
  const SOURCE: Record<string, string> = { card: 'this card', set: 'the set', default: 'vanilla holo' };

  // Cards imported from a card database or image folder can fetch their art again (e.g. after it was overwritten).
  const canRedownload = $derived(!!(meta?.scryfallId || meta?.sourceId) && project.meta?.source !== 'epl');
  let redownloading = $state(false);
  async function redownload() {
    redownloading = true;
    try {
      const got = (await App.RedownloadCardImages(project.id, [card.id]))[card.id];
      if (got) {
        card.image = got.image; // new files: the game sees them once the set is saved
        if (got.backImage) card.backImage = got.backImage;
        notify('Downloaded the card image again — Save to use it in the game.');
      }
    } catch (e) { notify(errText(e), 'error'); }
    redownloading = false;
  }

  let tab = $state<DetailTab>(rememberedTab);
  function setTab(v: DetailTab) { tab = rememberedTab = v; }
  let lastCardId = '';
  $effect(() => { if (card.id !== lastCardId) { lastCardId = card.id; showBack = false; } });
</script>

<div class="detail">
  <div class="preview" class:framed={project.set.renderMode === 'Framed'} style="--rc:{rarityColor(project.set, card.rarity)}">
    {#if card.image && showBack}
      <img src={projectFile(project.id, previewImage, imgBust)} alt="" />
    {:else if card.image}
      <!-- The shown version as the game draws it: its picture with its shader, in its border. -->
      <BorderPreview border={previewBorder} rim={gameRim} {shaders} art={projectFile(project.id, previewImage, imgBust)} width={240} facts={versionFacts(showAs)}
        picture={pictureShader(liveShader, card, project.id, imgBust)} />
    {:else}
      <button class="noimg" onclick={changeImage}>No image — choose one…</button>
    {/if}
    {#if card.backImage}
      <button class="flip small" onclick={() => (showBack = !showBack)} title="Show the other side">
        ⇄ {showBack ? 'Front' : 'Back'}</button>
    {/if}
    {#if showAs && !showBack}
      <button class="vshown small" class:warnchip={!versionsOn.has(showAs)} onclick={() => (showAs = '')}
        title={versionsOn.has(showAs) ? 'Show the main picture' : `This card has no ${versionLabel(showAs)} version — click to show the main picture`}>
        {versionLabel(showAs)} · ✕</button>
    {/if}
  </div>
  {#if card.backImage && showBack && card.mtg?.backName}<div class="muted small center">Back: {card.mtg.backName}</div>{/if}
  <div class="imgtools">
    <Dropdown cls="small" title="Change, add or rotate this card's pictures">
      {#snippet label()}Image{/snippet}
      {#snippet panel(close)}
        <div class="ddmenu">
          <button onclick={() => { close(); changeImage(); }}>{card.image ? 'Change image…' : 'Choose image…'}</button>
          <button onclick={() => { close(); changeBackImage(); }} title="The picture on this card's back, instead of the set's card back">
            {card.backImage ? 'Change back picture…' : 'Add a back picture…'}</button>
          {#if card.backImage}<button onclick={() => { close(); removeBackImage(); }} title="Use the set's card back again">Remove back picture</button>{/if}
          {#if canRedownload}
            <button disabled={redownloading} onclick={() => { close(); redownload(); }}
              title="Download this card's original image again from where the set was imported">Download the original again</button>
          {/if}
        </div>
      {/snippet}
    </Dropdown>
    {#if onopen}<button class="small" onclick={onopen} title="Edit this card on a full page: its details, look per version, price and play">⤢ Open editor</button>{/if}
    <span class="grow"></span>
    <button class="small" disabled={!card.image || rotating} onclick={() => rotate(-1)} title="Rotate the shown picture left">⟲</button>
    <button class="small" disabled={!card.image || rotating} onclick={() => rotate(1)} title="Rotate the shown picture right">⟳</button>
  </div>

  <div class="head">
    <input class="title" bind:value={card.name} aria-label="Card name" placeholder="Card name" />
    <div class="meta">
      <span class="rar"><span class="dot" style="background:{rarityColor(project.set, card.rarity)}"></span>{rarityName(project.set, card.rarity)}</span>
      {#if meta?.srcRarity}<span>{meta.srcRarity}</span>{/if}
      {#if card.number}<span>#{card.number}</span>{/if}
      <span class="idfield" title="The card's ID: what saved games, decks and packs know the card by">ID <span class="code">{card.id}</span>
        <button class="tiny" onclick={() => (changingId = true)} title="Change this card's ID (shows what changing it does first)">✎ Change…</button></span>
    </div>
  </div>

  {#if changingId}<ChangeCardId set={project.set} {card} onchange={changeId} oncancel={() => (changingId = false)} />{/if}

  <nav class="dtabs" aria-label="Card settings">
    <button class:on={tab === 'card'} onclick={() => setTab('card')}>Card</button>
    <button class:on={tab === 'look'} onclick={() => setTab('look')} title="Versions, borders and shaders">Look<span class="n">{versionsOn.size}</span></button>
    <button class:on={tab === 'price'} onclick={() => setTab('price')}>Price{#if meta?.locked}<span class="n">🔒</span>{/if}</button>
    <button class:on={tab === 'play'} onclick={() => setTab('play')} title="The card in the play-table game">Play</button>
  </nav>

  {#if tab === 'look'}
  <section>
    <label class="field">Preview as
      <select bind:value={showAs} title="The version the preview above (and the card grid) shows, as the game draws it">
        <option value="">Main picture</option>
        {#each ALL_VERSIONS as k}<option value={k}>{versionLabel(k)}{versionsOn.has(k) ? '' : ' (not a version of this card)'}</option>{/each}
      </select></label>
    {#if showAs}
      {@const src = shaderSource(project.set, card, showAs)}
      <table class="looksum">
        <tbody>
          <tr><td class="muted">Version</td><td>{versionsOn.has(showAs) ? 'On' : 'Off for this card'}</td></tr>
          {#if isFullImage(project.set)}<tr><td class="muted">Border</td><td>{borderName(shownBorder(project.set, card, showAs))}</td></tr>{/if}
          <tr><td class="muted">Shader</td><td>{shaderName(shownShader(project.set, card, showAs))}{#if SOURCE[src]} <span class="muted">({SOURCE[src]})</span>{/if}</td></tr>
          <tr><td class="muted">Picture</td><td>{versionImage(card, showAs) ? 'Its own' : fallbackImage(card, showAs) ? 'Its non-foil picture' : 'The main picture'}</td></tr>
        </tbody>
      </table>
    {/if}
    <div class="muted small">{hasOwnVersions(card) || card.versionBorders || card.versionShaders ? 'This card has looks of its own.' : 'Follows the set (Set → Card look).'}
      {versionsOn.size} of 12 versions{withArt.size ? ` · ${withArt.size} with own picture` : ''}.</div>
    {#if onopen}<button class="primary" onclick={onopen}>⤢ Edit the look in the card editor</button>{/if}
  </section>
  {/if}

  {#if tab === 'card'}<section><CardInfoFields {project} {card} {notify} /></section>{/if}
  {#if tab === 'price'}<section><CardPriceFields {project} {card} /></section>{/if}
  {#if tab === 'play'}<section><CardPlayFields {project} {card} /></section>{/if}

  {#if tab === 'card'}<div class="delete"><button class="danger" onclick={ondelete}>Delete card</button></div>{/if}
</div>

<style>
  .detail { padding: 14px 16px 20px; display: flex; flex-direction: column; gap: 10px; }
  .preview { position: relative; display: flex; justify-content: center; border-radius: 12px; padding: 18px 10px;
    background: radial-gradient(ellipse at 50% 35%, color-mix(in srgb, var(--rc) 22%, #2b3040), #15171d 75%); border: 1px solid var(--line); }
  .preview img { max-width: 240px; max-height: 330px; border-radius: 10px; display: block; box-shadow: 0 10px 28px rgba(0, 0, 0, 0.55); }
  .preview.framed img { border-radius: 4px; }
  .looksum { width: 100%; border-collapse: collapse; font-size: 13px; }
  .looksum td { padding: 4px 6px 4px 0; border-top: 1px solid var(--line); }
  .looksum td:first-child { width: 70px; }
  .vshown.warnchip { color: #f2c14e; }
  .noimg { width: 170px; height: 238px; border-style: dashed; color: var(--muted); background: transparent; }
  .flip { position: absolute; bottom: 8px; right: 8px; background: rgba(12, 14, 18, 0.85); }
  .center { text-align: center; }
  .vshown { position: absolute; bottom: 8px; left: 8px; background: rgba(12, 14, 18, 0.85); }
  .imgtools { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; }
  .head { display: flex; flex-direction: column; gap: 4px; margin-top: 4px; }
  .title { font-size: 19px; font-weight: 600; background: transparent; border: 1px solid transparent; padding: 2px 6px; margin-left: -6px; }
  .title:hover { border-color: var(--line); }
  .title:focus { border-color: var(--accent); background: var(--bg); }
  .meta { display: flex; flex-wrap: wrap; align-items: center; gap: 4px 12px; color: var(--muted); font-size: 12px; }
  .meta .rar { display: inline-flex; align-items: center; gap: 5px; color: var(--text); }
  .idfield { display: inline-flex; align-items: center; gap: 6px; }
  .dot { width: 8px; height: 8px; border-radius: 50%; display: inline-block; }
  section { display: flex; flex-direction: column; gap: 10px; padding: 4px 0 0; }
  .dtabs { display: flex; gap: 2px; border-bottom: 1px solid var(--line); margin: 4px -16px 0; padding: 0 12px; }
  .dtabs button { flex: 1 1 auto; border: none; border-bottom: 2px solid transparent; border-radius: 0; background: none; padding: 8px 6px 7px;
    color: var(--muted); font-size: 13px; white-space: nowrap; }
  .dtabs button:hover { color: var(--text); }
  .dtabs button.on { color: var(--text); border-bottom-color: var(--accent); }
  .dtabs .n { font-size: 10px; color: var(--muted); background: var(--panel-2); border-radius: 8px; padding: 0 5px; margin-left: 5px; }
  /* The side panel is narrow: fields share a row and shrink (number boxes and selects keep a wide natural size otherwise). */
  .field { min-width: 0; }
  .field > select { width: 100%; }
  .delete { padding-top: 6px; }
</style>
