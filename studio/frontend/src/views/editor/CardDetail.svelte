<script lang="ts">
  import { App, projectFile, money, setRarities, rarityName, rarityColor, ELEMENTS, BORDERS, BORDER_LABELS, LANES, errText } from '../../lib/api';

  let { project, card, imgBust, notify, ondelete, onzoom }: {
    project: any; card: any; imgBust: number; notify: (t: string, k?: string) => void;
    ondelete: () => void; onzoom: () => void;
  } = $props();

  const meta = $derived(project.meta?.cards?.[card.id]);
  let effectText = $state('');
  let effectError = $state('');
  let newOverride = $state('Base_foil');

  // Reset the effect editor when a different card is selected.
  $effect(() => {
    effectText = card.play.effect ? JSON.stringify(card.play.effect, null, 2) : '';
    effectError = '';
  });

  function setEffect() {
    const t = effectText.trim();
    if (!t) { card.play.effect = undefined; effectError = ''; return; }
    try { card.play.effect = JSON.parse(t); effectError = ''; }
    catch (e) { effectError = 'Invalid JSON: ' + errText(e); }
  }

  function optNumber(v: string): number | undefined {
    const n = parseFloat(v);
    return isNaN(n) ? undefined : n;
  }

  function toggleBorderMults() {
    card.price.borderMultipliers = card.price.borderMultipliers ? undefined : [...project.set.priceDefaults.borderMultipliers];
  }

  function addOverride() {
    card.price.overrides = { ...(card.price.overrides ?? {}), [newOverride]: card.price.base };
  }

  function removeOverride(k: string) {
    const o = { ...card.price.overrides };
    delete o[k];
    card.price.overrides = Object.keys(o).length ? o : undefined;
  }

  function setLocked(v: boolean) {
    project.meta.cards ??= {};
    project.meta.cards[card.id] = { ...(project.meta.cards[card.id] ?? {}), locked: v };
  }

  async function changeImage() {
    try {
      const rel = await App.PickCardImage(project.id);
      if (rel) card.image = rel;
    } catch (e) { notify(errText(e), 'error'); }
  }

  let rotating = $state(false);
  async function rotate(turns: number) {
    rotating = true;
    try {
      const turned = await App.RotateCardImages(project.id, [card.image], turns, false);
      if (turned[card.image]) card.image = turned[card.image]; // a new file: the game sees it once the set is saved
    } catch (e) { notify(errText(e), 'error'); }
    rotating = false;
  }

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

  const variantKeys =BORDERS.flatMap((b) => [b, b + '_foil']);
  const others = $derived(project.set.cards.filter((c: any) => c.id !== card.id));
  const preview = $derived(
    BORDERS.slice(0, 6).map((b, i) => {
      const bm = (card.price.borderMultipliers ?? project.set.priceDefaults.borderMultipliers)[i];
      const fm = card.price.foilMultiplier ?? project.set.priceDefaults.foilMultiplier;
      const o = card.price.overrides ?? {};
      const min = project.set.priceDefaults.minimum;
      return [b, o[b] ?? Math.max(min, card.price.base * bm), o[b + '_foil'] ?? Math.max(min, card.price.base * bm * fm)];
    })
  );
  // Double-faced cards: which side the preview shows (back faces come from the Scryfall import / "Download back faces").
  let showBack = $state(false);
  let lastCardId = '';
  $effect(() => { if (card.id !== lastCardId) { lastCardId = card.id; showBack = false; } });
  const LANE_COLORS: Record<string, string> = { Fire: '#ff7a45', Earth: '#c9a227', Water: '#4fa3ff', Wind: '#3ecf8e' };
</script>

<div class="detail">
  <div class="preview" class:framed={project.set.renderMode === 'Framed'} style="--rc:{rarityColor(project.set, card.rarity)}">
    {#if card.image}
      <button class="zoom" title="View full screen" onclick={onzoom}><img src={projectFile(project.id, showBack && card.backImage ? card.backImage : card.image, imgBust)} alt={card.name} /></button>
    {:else}
      <button class="noimg" onclick={changeImage}>No image — choose one…</button>
    {/if}
    {#if card.backImage}
      <button class="flip small" onclick={() => (showBack = !showBack)} title="Double-faced card: show the other side">
        ⇄ {showBack ? 'Front' : 'Back'}</button>
    {/if}
  </div>
  {#if card.backImage && showBack && card.mtg?.backName}<div class="muted small center">Back: {card.mtg.backName}</div>{/if}
  <div class="imgtools">
    <button class="small" onclick={changeImage}>Change image…</button>
    {#if canRedownload}
      <button class="small" disabled={redownloading} onclick={redownload}
        title="Download this card's original image again from where the set was imported">
        {redownloading ? 'Downloading…' : 'Re-download'}</button>
    {/if}
    <span class="grow"></span>
    <button class="small" disabled={!card.image || rotating} onclick={() => rotate(-1)} title="Rotate left">⟲</button>
    <button class="small" disabled={!card.image || rotating} onclick={() => rotate(1)} title="Rotate right">⟳</button>
  </div>
  {#if card.image}<div class="muted path" title={card.image}>{card.image}</div>{/if}

  <div class="head">
    <input class="title" bind:value={card.name} aria-label="Card name" placeholder="Card name" />
    <div class="meta">
      <span class="rar"><span class="dot" style="background:{rarityColor(project.set, card.rarity)}"></span>{rarityName(project.set, card.rarity)}</span>
      {#if meta?.srcRarity}<span>{meta.srcRarity}</span>{/if}
      {#if card.number}<span>#{card.number}</span>{/if}
      <span class="code" title="Card id (stable)">{card.id}</span>
    </div>
  </div>

  <section>
    <h4>Card</h4>
    <div class="row">
      <label class="field grow">Rarity
        <select bind:value={card.rarity}>{#each setRarities(project.set) as r}<option value={r.id}>{r.name}</option>{/each}</select>
      </label>
      <label class="field" style="flex: 0 0 90px">Number<input bind:value={card.number} /></label>
    </div>
    <label class="field">Artist<input bind:value={card.artist} /></label>
    <label class="field">Description<textarea rows="4" bind:value={card.description}></textarea></label>
  </section>

  <section>
    <div class="shead"><h4>Price</h4><span class="grow"></span>
      <label class="check small lock" class:on={!!meta?.locked} title="Gamify and price refresh skip locked cards"><input type="checkbox" checked={!!meta?.locked} onchange={(e) => setLocked(e.currentTarget.checked)} /> 🔒 Lock</label>
    </div>
    {#if meta && (meta.usd !== undefined || meta.usdFoil !== undefined)}
      <div class="real">Real price <b>{money(meta.usd)}</b> · foil <b>{money(meta.usdFoil)}</b></div>
    {/if}
    <div class="row">
      <label class="field grow">Base price<input type="number" step="0.01" min="0" bind:value={card.price.base} /></label>
      <label class="field grow">Foil ×<input type="number" step="0.1" placeholder={String(project.set.priceDefaults.foilMultiplier)}
        value={card.price.foilMultiplier ?? ''} oninput={(e) => (card.price.foilMultiplier = optNumber(e.currentTarget.value))} /></label>
    </div>
    <table class="pricetable">
      <thead><tr><th>Version</th><th>Normal</th><th>Foil</th></tr></thead>
      <tbody>{#each preview as [b, n, f]}<tr><td>{BORDER_LABELS[b as string]}</td><td>{money(n as number)}</td><td class="foil">{money(f as number)}</td></tr>{/each}</tbody>
    </table>
    <label class="check small"><input type="checkbox" checked={!!card.price.borderMultipliers} onchange={toggleBorderMults} /> Own border multipliers</label>
    {#if card.price.borderMultipliers}
      <div class="mults">
        {#each BORDERS as b, i}<label class="field">{BORDER_LABELS[b]}<input type="number" step="0.05" bind:value={card.price.borderMultipliers[i]} /></label>{/each}
      </div>
    {/if}
    <div class="overrides">
      {#each Object.entries(card.price.overrides ?? {}) as [k, v]}
        <div class="row ov"><span class="grow small">Exact: {k.replace('_foil', ' foil')}</span>
          <input type="number" step="0.01" style="width:100px" value={v} oninput={(e) => (card.price.overrides[k] = optNumber(e.currentTarget.value) ?? 0)} />
          <button class="small" onclick={() => removeOverride(k)} title="Remove this exact price">✕</button></div>
      {/each}
      <div class="row">
        <select class="grow" bind:value={newOverride}>{#each variantKeys as k}<option value={k}>{k.replace('_foil', ' foil')}</option>{/each}</select>
        <button class="small" onclick={addOverride}>Add exact price</button>
      </div>
    </div>
  </section>

  <section>
    <h4>Play table</h4>
    <div class="lanes">
      {#each LANES as l, i}<label class="field lane" style="--lc:{LANE_COLORS[l] ?? 'var(--line)'}">{l}<input type="number" min="0" bind:value={card.play.laneAttack[i]} /></label>{/each}
    </div>
    <div class="row">
      <label class="field grow">Element
        <select bind:value={card.play.element}>{#each ELEMENTS as e}<option>{e}</option>{/each}</select>
      </label>
      <label class="field grow">Evolves from
        <select value={card.play.evolvesFrom ?? ''} onchange={(e) => (card.play.evolvesFrom = e.currentTarget.value || undefined)}>
          <option value="">— basic card —</option>
          {#each others as o}<option value={o.id}>{o.name} ({o.id})</option>{/each}
        </select>
      </label>
    </div>
    <label class="field">Effect (raw PlayEffectData JSON, the game's own format; empty = no effect)
      <textarea rows="6" spellcheck="false" class="mono" bind:value={effectText} onblur={setEffect}
        placeholder={'{ "playEffectQueueDataList": [ { "playEffectType": "DrawCard", "playEffectTypeSecondary": "None", "countList": [1] } ] }'}></textarea>
    </label>
    {#if effectError}<div class="err small">{effectError}</div>{/if}
  </section>

  <div class="delete"><button class="danger" onclick={ondelete}>Delete card</button></div>
</div>

<style>
  .detail { padding: 14px 16px 20px; display: flex; flex-direction: column; gap: 10px; }
  .preview { position: relative; display: flex; justify-content: center; border-radius: 12px; padding: 18px 10px;
    background: radial-gradient(ellipse at 50% 35%, color-mix(in srgb, var(--rc) 22%, #2b3040), #15171d 75%); border: 1px solid var(--line); }
  .preview img { max-width: 240px; max-height: 330px; border-radius: 10px; display: block; box-shadow: 0 10px 28px rgba(0, 0, 0, 0.55); }
  .preview.framed img { border-radius: 4px; }
  .zoom { padding: 0; border: none; background: none; cursor: zoom-in; }
  .noimg { width: 170px; height: 238px; border-style: dashed; color: var(--muted); background: transparent; }
  .flip { position: absolute; bottom: 8px; right: 8px; background: rgba(12, 14, 18, 0.85); }
  .center { text-align: center; }
  .imgtools { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; }
  .path { font-size: 11px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; min-width: 0; margin-top: -4px; }
  .head { display: flex; flex-direction: column; gap: 4px; margin-top: 4px; }
  .title { font-size: 19px; font-weight: 600; background: transparent; border: 1px solid transparent; padding: 2px 6px; margin-left: -6px; }
  .title:hover { border-color: var(--line); }
  .title:focus { border-color: var(--accent); background: var(--bg); }
  .meta { display: flex; flex-wrap: wrap; align-items: center; gap: 4px 12px; color: var(--muted); font-size: 12px; }
  .meta .rar { display: inline-flex; align-items: center; gap: 5px; color: var(--text); }
  .dot { width: 8px; height: 8px; border-radius: 50%; display: inline-block; }
  section { display: flex; flex-direction: column; gap: 8px; background: var(--bg); border: 1px solid var(--line); border-radius: 10px; padding: 12px; }
  h4 { margin: 0; font-size: 13px; font-weight: 600; }
  .shead { display: flex; align-items: center; gap: 8px; }
  .lock { padding: 2px 8px; border: 1px solid var(--line); border-radius: 999px; cursor: pointer; }
  .lock.on { border-color: var(--warn); color: var(--warn); }
  .real { font-size: 12px; color: var(--muted); }
  .real b { color: var(--ok); font-weight: 600; }
  .mults { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 6px; }
  .lanes { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 6px; }
  /* The side panel is narrow: fields share a row and shrink (number boxes and selects keep a wide natural size otherwise). */
  .row > .field { flex: 1 1 0; min-width: 0; }
  .field { min-width: 0; }
  .field > input, .field > select, .field > textarea { width: 100%; }
  .row > select { min-width: 0; }
  .lane { border-top: 3px solid var(--lc); padding-top: 4px; border-radius: 2px; }
  .overrides { display: flex; flex-direction: column; gap: 4px; }
  .ov { background: var(--panel); border-radius: 6px; padding: 2px 2px 2px 8px; }
  .pricetable { width: 100%; border-collapse: collapse; font-size: 12px; font-variant-numeric: tabular-nums; }
  .pricetable th { text-align: left; font-weight: 500; color: var(--muted); padding: 2px 6px 4px; }
  .pricetable td { text-align: left; padding: 3px 6px; border-top: 1px solid var(--line); }
  .pricetable td.foil { color: #c9a8ff; }
  .mono { font-family: Consolas, monospace; font-size: 12px; }
  .delete { padding-top: 6px; }
</style>
