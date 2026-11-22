<script lang="ts">
  // How a set's cards (no card) or one card look, version by version: one row per border slot, a Non-foil and a Foil cell
  // with whether that version exists, its border and the shader of its picture (and, for a card, its own picture). Beside
  // it the clicked version as the game draws it (for a card: every version its own live sample in its cell): the version's picture with its shader, in its border, the border's own
  // shaders on top (BorderPreview). Reads and writes only through lib/cardVersions.
  // Set: set.json "variants", "borders" (slot names + looks), "versionShaders". Card: its "variants", "versionBorders",
  // "versionShaders", "versionImages"; every choice can follow the set.
  import { onMount } from 'svelte';
  import { App, errText, BORDERS, BORDER_LABELS, projectFile } from '../../lib/api';
  import { FOLLOW } from './ShaderGrid.svelte';
  import BorderPreview from '../BorderPreview.svelte';
  import EditionLabelEditor from './EditionLabelEditor.svelte';
  import { labelOn } from '../../lib/editionLabel';
  import { loadLooks, noLooks, nameOf, versionFacts, shownImage, borderFor, shaderFor, rimFor, pictureShader as pictureOf } from '../../lib/cardLooks';
  import { ALL_VERSIONS, MAIN_VERSION, versionKey, versionLabel, isFullImage, setVersions, cardVersions, hasOwnVersions, followSet,
    writeSetVersions, writeCardVersions, totalVersions, slotOf, writeSlot, setBorder, cardBorder, writeCardBorder,
    setShader, cardShader, writeSetShader, writeCardShader, inheritLabel, cardsWithOwnShaders, cardsWithOwnBorders,
    versionImage, setVersionImage, versionsWithArt } from '../../lib/cardVersions';

  let { project, imgBust = 0, card, notify }: {
    project: any; imgBust?: number;
    /** The card edited; none = the set's defaults. */
    card?: any;
    notify?: (t: string, k?: string) => void;
  } = $props();

  let looks = $state(noLooks());
  onMount(async () => { looks = await loadLooks(); });
  const borders = $derived(looks.borders);
  const shaders = $derived(looks.shaders);

  const set = $derived(project.set);
  const full = $derived(isFullImage(set));
  const on = $derived(card ? cardVersions(set, card) : setVersions(set));
  const SAME = '~same';
  const knownBorders = $derived(new Set(borders.map((b) => b.id)));
  const knownShaders = $derived(new Set(shaders.map((s) => s.id)));
  const borderName = (id: string) => nameOf(borders, id, 'Vanilla rim', 'border');
  const shaderName = (id: string) => nameOf(shaders, id, 'No shader', 'shader');

  function toggle(k: string) {
    const next = new Set(on);
    if (next.has(k)) next.delete(k); else next.add(k);
    if (card) writeCardVersions(set, card, next); else writeSetVersions(set, next);
  }

  // A cell's border choice: set = the slot's look ('' = the game's rim; foil also SAME = as non-foil), card = its own or FOLLOW.
  function borderValue(b: string, foil: boolean): string {
    const k = versionKey(b, foil);
    if (card) return cardBorder(card, k) ?? FOLLOW;
    const slot = slotOf(set, b);
    return foil ? (slot?.foilLook === undefined ? SAME : slot.foilLook) : slot?.look ?? '';
  }
  function writeBorder(b: string, foil: boolean, v: string) {
    const k = versionKey(b, foil);
    if (card) writeCardBorder(card, k, v === FOLLOW ? undefined : v);
    else if (foil) writeSlot(set, b, 'foilLook', v === SAME ? undefined : v);
    else writeSlot(set, b, 'look', v || undefined);
  }
  // A cell's shader choice: an id, '' (none) or FOLLOW (set: the default, card: the set's).
  const shaderValue = (k: string) => (card ? cardShader(card, k) : setShader(set, k)) ?? FOLLOW;
  function writeShader(k: string, v: string) {
    const id = v === FOLLOW ? undefined : v;
    if (card) writeCardShader(card, k, id); else writeSetShader(set, k, id);
  }

  // The version shown beside the table: the cell clicked last; at first the first version that is on.
  let sel = $state('');
  const shown = $derived(sel || ALL_VERSIONS.find((k) => on.has(k)) || MAIN_VERSION);
  const shownSlot = $derived(shown.replace('_foil', ''));
  const shownFoil = $derived(shown.endsWith('_foil'));

  // The card the preview is drawn on: the card edited, else one with pictures per version first ("Shown on" picks another).
  const withImage = $derived(set.cards.filter((c: any) => c.image));
  let exampleId = $state('');
  const example = $derived(card ?? withImage.find((c: any) => c.id === exampleId) ?? withImage.find((c: any) => versionsWithArt(c).size > 0) ?? withImage[0]);
  // Choices come from the card edited, or from the set alone (what a card that follows the set gets).
  const chooser = $derived(card ?? {});
  const art = $derived(!card && example?.image ? projectFile(project.id, shownImage(example, shown), imgBust) : '');
  // The set's side preview (a card shows a sample per version instead, so none of this is worked out for it).
  const previewBorder = $derived(card ? null : borderFor(looks, set, chooser, shown));
  const pictureShader = $derived(card ? undefined : shaderFor(looks, set, chooser, shown));
  const picture = $derived(card ? undefined : pictureOf(pictureShader, card, project.id, imgBust));
  // Card: every version has its own live sample in its cell (no side preview); a shader can be tried per version.
  let tryingAt = $state<Record<string, string>>({});
  $effect(() => { void card?.id; tryingAt = {}; });
  function useTriedAt(k: string, forSet: boolean) {
    const id = tryingAt[k];
    if (forSet) { writeSetShader(set, k, id); writeCardShader(card, k, undefined); }
    else writeCardShader(card, k, id);
    notify?.(`${shaderName(id)} is now ${forSet ? "the set's" : "this card's"} ${versionLabel(k)} shader. Save to use it in the game.`, 'ok');
    tryingAt[k] = '';
  }
  // A version's sample as the game draws it, on the card edited (or the set's example card), with its edition label.
  // Switched-off versions skip the picture's shader (each live shader is a WebGL canvas, and the browser only keeps so many).
  function sampleOf(k: string) {
    const b = borderFor(looks, set, chooser, k);
    const sh = on.has(k) ? shaderFor(looks, set, chooser, k, tryingAt[k] ?? '') : undefined;
    return { art: example?.image ? projectFile(project.id, shownImage(example, k), imgBust) : '', border: b,
      rim: b ? undefined : rimFor(looks, set, k), picture: pictureOf(sh, card, project.id, imgBust, !tryingAt[k]), facts: versionFacts(k),
      label: labelOn(set, card, k) };
  }
  // The version the edition label block shows: the first switched-on one that has a label.
  const labelKey = $derived(['FirstEdition', 'Silver', 'Gold', 'EX'].flatMap((b) => [b, b + '_foil']).find((k) => on.has(k)) ?? 'FirstEdition');
  const gameRim = $derived(card || previewBorder ? undefined : rimFor(looks, set, shown));
  const facts = $derived(versionFacts(shown));

  // Set: presets for the versions every card comes in.
  const PRESETS: [string, string, string[]][] = [
    ['all', 'All 12', ALL_VERSIONS], ['nofoil', 'No foil', BORDERS], ['base', 'Base only', ['Base']], ['basefoil', 'Base + foil', ['Base', 'Base_foil']]
  ];
  const preset = $derived(PRESETS.find(([, , keys]) => on.size === keys.length && keys.every((k) => on.has(k)))?.[0] ?? 'custom');

  // Cards with choices of their own (set), or what this card has of its own.
  const ownVersions = $derived(set.cards.filter(hasOwnVersions).length);
  const ownBorders = $derived(cardsWithOwnBorders(set));
  const ownShaders = $derived(cardsWithOwnShaders(set));
  const cardOwn = $derived(card ? [hasOwnVersions(card) && 'versions', card.versionBorders && 'borders', card.versionShaders && 'shaders'].filter(Boolean) as string[] : []);
  function cardFollowsSet() {
    followSet(card);
    card.versionBorders = undefined;
    card.versionShaders = undefined;
  }

  async function pickVersionImage(k: string) {
    try {
      const rel = await App.PickCardImage(project.id);
      if (rel) { setVersionImage(card, k, rel); sel = k; }
    } catch (e) { notify?.(errText(e), 'error'); }
  }
  async function changeImage() {
    try {
      const rel = await App.PickCardImage(project.id);
      if (rel) card.image = rel;
    } catch (e) { notify?.(errText(e), 'error'); }
  }
</script>

<!-- noNone: the "nothing" choice is already the first option (following a set that has nothing), so it isn't listed twice. -->
{#snippet borderOptions(v: string, noNone = false)}
  {#if !noNone || v === ''}<option value="">Vanilla rim</option>{/if}
  {#each borders.filter((b) => !b.builtIn) as b}<option value={b.id}>{b.name || 'Unnamed'}</option>{/each}
  {#if borders.some((b) => b.builtIn)}
    <optgroup label="Comes with the mod">{#each borders.filter((b) => b.builtIn) as b}<option value={b.id}>{b.name}</option>{/each}</optgroup>
  {/if}
  {#if v && v !== SAME && v !== FOLLOW && !knownBorders.has(v)}<option value={v}>Missing border ({v})</option>{/if}
{/snippet}

{#snippet shaderOptions(v: string, noNone = false)}
  {#if !noNone || v === ''}<option value="">No shader</option>{/if}
  {#each shaders.filter((s) => !s.builtIn) as s}<option value={s.id}>{s.name || 'Unnamed'}</option>{/each}
  {#if shaders.some((s) => s.builtIn)}
    <optgroup label="Comes with the mod">{#each shaders.filter((s) => s.builtIn) as s}<option value={s.id}>{s.name}</option>{/each}</optgroup>
  {/if}
  {#if v && v !== FOLLOW && !knownShaders.has(v)}<option value={v}>Missing shader ({v})</option>{/if}
{/snippet}

{#snippet cell(b: string, foil: boolean)}
  {@const k = versionKey(b, foil)}
  {@const bv = borderValue(b, foil)}
  {@const sv = shaderValue(k)}
  {@const follow = card ? inheritLabel(set, k, setShader(set, k), shaderName) : inheritLabel(set, k, undefined, shaderName)}
  <td>
    <div class="cell" class:sel={!card && shown === k} class:off={!on.has(k)} class:withsample={!!card} role="presentation" onclick={() => (sel = k)} onfocusin={() => (sel = k)}>
      {#if card}
        {@const s = sampleOf(k)}
        <div class="sample" title={on.has(k) ? `${versionLabel(k)} as the game draws it` : `${versionLabel(k)} is off`}>
          {#if s.art}<BorderPreview border={s.border} rim={s.rim} {shaders} art={s.art} picture={s.picture} width={96} facts={s.facts} label={s.label} still={!on.has(k)} />
          {:else}<div class="nosample muted small">No picture</div>{/if}
        </div>
      {/if}
      <div class="controls">
      <!-- Whether the version exists: the same On / Off toggle as the card panel's Versions grid. -->
      <button class="v" class:on={on.has(k)} class:foil aria-pressed={on.has(k)} disabled={on.size === 1 && on.has(k)}
        title={on.size === 1 && on.has(k) ? 'Every card needs at least one version' : on.has(k) ? `Cards come in ${versionLabel(k)}: click to switch it off` : `Cards never come in ${versionLabel(k)}: click to switch it on`}
        onclick={() => toggle(k)}>{on.has(k) ? '✓ On' : 'Off'}</button>
      {#if full}
        <label class="field">Border
          <select value={bv} onchange={(e) => writeBorder(b, foil, e.currentTarget.value)}>
            {#if card}<option value={FOLLOW}>{setBorder(set, k) ? `Set: ${borderName(setBorder(set, k))}` : 'Vanilla rim'}</option>
            {:else if foil}<option value={SAME}>Same as non-foil</option>{/if}
            {@render borderOptions(bv, !!card && !setBorder(set, k))}
          </select>
        </label>
      {/if}
      <label class="field">Shader
        <select value={sv} onchange={(e) => writeShader(k, e.currentTarget.value)}>
          <option value={FOLLOW}>{follow}</option>
          {@render shaderOptions(sv, follow === 'No shader')}
        </select>
      </label>
      {#if card}
        {@const own = versionImage(card, k)}
        {#if on.has(k)}
          <label class="field">Try
            <select value={tryingAt[k] ?? ''} onchange={(e) => (tryingAt[k] = e.currentTarget.value)} title="Preview another shader on this version; nothing changes until you use it">
              <option value="">— its own —</option>
              {@render shaderOptions('', true)}
            </select>
          </label>
          {#if tryingAt[k]}
            <div class="row tryrow">
              <button class="small primary" onclick={() => useTriedAt(k, false)} title="Use it for this card's {versionLabel(k)}">Use</button>
              <button class="small" onclick={() => useTriedAt(k, true)} title="Use it for {versionLabel(k)} on every card of the set that follows it">For set</button>
              <button class="small" onclick={() => (tryingAt[k] = '')}>Stop</button>
            </div>
          {/if}
        {/if}
        <div class="pic">
          <span class="muted small grow" title={k === MAIN_VERSION ? 'The main image: every version without its own picture shows it' : own ? `Own picture: ${own}` : 'No own picture: uses the main image (a foil first uses its non-foil picture)'}>
            {k === MAIN_VERSION ? 'Main picture' : own ? 'Own picture' : 'Main picture'}</span>
          {#if k === MAIN_VERSION}
            <button class="small" onclick={changeImage}>{card.image ? 'Change' : '+ art'}</button>
          {:else if own}
            <button class="small" onclick={() => setVersionImage(card, k, undefined)} title="Remove this version's own picture">✕</button>
          {:else}
            <button class="small" onclick={() => pickVersionImage(k)} title="Give the {versionLabel(k)} version its own picture">+ art</button>
          {/if}
        </div>
      {/if}
      </div>
    </div>
  </td>
{/snippet}

<!-- A card: the table takes the whole width (each version has its own sample), the notes go under it. -->
<div class={card ? 'cardlook' : 'page-cols'}>
  <div class="maincol">
  <div class="block">
    <div class="bhead"><h2>{card ? "This card's versions & looks" : 'Versions & looks'}</h2>
      {#if card}
        <span class="muted small">{cardOwn.length ? `Own ${cardOwn.join(', ')}` : 'Follows the set'} · {on.size} of 12</span><span class="grow"></span>
        {#if cardOwn.length}<button class="small" onclick={cardFollowsSet} title="The card uses the set's versions, borders and shaders again">Follow set</button>{/if}
        <button class="small" disabled={on.size === 12} onclick={() => writeCardVersions(set, card, new Set(ALL_VERSIONS))}>All on</button>
      {:else}
        <span class="grow"></span>
        <div class="seg" role="group" aria-label="Presets">
          {#each PRESETS as [id, label, keys]}
            <button class:on={preset === id} onclick={() => writeSetVersions(set, new Set(keys))}>{label}</button>
          {/each}
          <button class:on={preset === 'custom'} disabled title="Tick versions in the table to make your own mix">Custom</button>
        </div>
      {/if}
    </div>
    {#if !full}<div class="notice">This set is framed: the game draws its own frame, so there are no borders to choose. Shaders still apply.</div>{/if}
    <div class="tablewrap"><table class="mults looks">
      <thead><tr><th>Slot</th><th>Non-foil</th><th>Foil</th></tr></thead>
      <tbody>
        {#each BORDERS as b}
          <tr>
            <td class="bname"><span class="bdot b-{b}"></span>{BORDER_LABELS[b]}</td>
            {@render cell(b, false)}
            {@render cell(b, true)}
          </tr>
        {/each}
      </tbody>
    </table></div>
    {#if !card && (ownVersions || ownBorders || ownShaders)}
      <div class="bfoot muted small">
        <span class="grow">Cards with choices of their own ignore these for them:
          {[ownVersions && `${ownVersions} versions`, ownBorders && `${ownBorders} borders`, ownShaders && `${ownShaders} shaders`].filter(Boolean).join(' · ')}.</span>
        {#if ownVersions}<button class="small" onclick={() => { for (const c of set.cards) followSet(c); }} title="Those cards follow the set's versions again">Reset their versions</button>{/if}
        {#if ownBorders}<button class="small" onclick={() => { for (const c of set.cards) c.versionBorders = undefined; }} title="Those cards follow the set's borders again">Reset their borders</button>{/if}
        {#if ownShaders}<button class="small" onclick={() => { for (const c of set.cards) c.versionShaders = undefined; }} title="Those cards follow the set's shaders again">Reset their shaders</button>{/if}
      </div>
    {/if}
  </div>
  {#if full}{@const ls = sampleOf(labelKey)}
    <EditionLabelEditor {project} {card} sample={ls} sampleKey={labelKey} />
  {/if}
  </div>

  <aside class:page-side={!card}>
    {#if !card}
    <div class="block">
      <div class="bhead"><h2>{versionLabel(shown)}</h2><span class="grow"></span>
        <div class="seg" role="group" aria-label="Version shown">
          <button class:on={!shownFoil} onclick={() => (sel = shownSlot)}>Non-foil</button>
          <button class:on={shownFoil} onclick={() => (sel = shownSlot + '_foil')}>Foil</button>
        </div>
      </div>
      {#if !card && withImage.length > 1}
        <label class="field">Shown on
          <select value={example?.id ?? ''} onchange={(e) => (exampleId = e.currentTarget.value)}>
            {#each withImage.slice(0, 500) as c (c.id)}<option value={c.id}>{c.name || c.id}{versionsWithArt(c).size ? ' · own version art' : ''}</option>{/each}
          </select>
        </label>
      {/if}
      {#if art}
        <BorderPreview border={previewBorder} rim={gameRim} {shaders} {art} {picture} width={260} {facts} label={labelOn(set, undefined, shown)} />
        <div class="muted small">
          {full ? (previewBorder ? previewBorder.name : gameRim === undefined ? 'Vanilla rim (shown once Studio has read the game files)' : gameRim ? `Vanilla ${BORDER_LABELS[shownSlot]} rim` : 'No rim: the picture is the whole card') : "Artwork only: the game puts it in its frame"}
          · {pictureShader ? pictureShader.name : 'no shader'}{!on.has(shown) ? ' · this version is off' : ''}
        </div>
      {:else}
        <p class="muted small">Add cards with pictures to see the looks on one.</p>
      {/if}
    </div>
      <div class="block">
        <div class="facts">
          <div><span>Versions on</span><b>{on.size}<small> / 12</small></b></div>
          <div><span>Foil versions</span><b>{[...on].filter((k) => k.endsWith('_foil')).length}<small> / 6</small></b></div>
          <div><span>Binder slots</span><b>{totalVersions(set).toLocaleString()}</b><small>over {set.cards.length.toLocaleString()} cards</small></div>
        </div>
      </div>
    {/if}
    <div class="notice info howto"><b>How card looks work</b>
      <ul>
        <li>A version is a border slot × non-foil or foil. Untick one and it never comes out of packs or from trade customers and takes
          no binder slot (cards players already own stay).</li>
        {#if full}
          <li><b>Border</b>: drawn around the picture (made on the <b>Borders</b> page). The vanilla rim shows where none is chosen.</li>
          <li><b>Shader</b>: the effect on the picture (made on the <b>Shaders</b> page). Foils without a choice get the vanilla holo.</li>
        {:else}
          <li><b>Shader</b>: the effect on the artwork inside the game's frame. Foils without a choice keep the game's own foil.</li>
        {/if}
        {#if card}
          <li>Each choice can follow the set (Set → Card look). A foil without its own picture shows its border's non-foil one, then the main image.</li>
        {:else}
          <li>Every card follows these choices unless it has its own (open a card's editor → Look).</li>
          <li>Odds per version are set per pack (Packs → Foil &amp; borders), prices on Prices.</li>
        {/if}
      </ul>
    </div>
  </aside>
</div>

<style>
  .looks td { vertical-align: top; }
  .cell { display: flex; flex-direction: column; gap: 4px; min-width: 150px; padding: 4px; border-radius: 6px; }
  .cell { gap: 6px; }
  .cell select { width: 100%; min-width: 0; font-size: 13px; }
  .cell.sel { outline: 2px solid var(--accent); outline-offset: 1px; }
  .cell.off .field, .cell.off .pic { opacity: 0.45; }
  /* The version toggle: the look of VersionGrid's buttons. */
  .v { padding: 4px 0; font-size: 12px; color: var(--muted); background: var(--bg); }
  .v.on { color: var(--text); background: #1c2740; border-color: var(--accent-2); }
  .v.on.foil { background: linear-gradient(120deg, #1c2740, #2e2148 55%, #1d3a3a); }
  .tryrow { flex-wrap: wrap; }
  .pic { display: flex; align-items: center; gap: 6px; }
  /* A card's cell: its live sample beside the choices. */
  .cardlook, .maincol { display: flex; flex-direction: column; gap: 14px; min-width: 0; }
  .cell.withsample { flex-direction: row; align-items: flex-start; min-width: 250px; }
  .controls { display: flex; flex-direction: column; gap: 6px; flex: 1; min-width: 0; }
  .sample { flex: none; width: 104px; }
  .sample :global(.stage) { border-radius: 6px; }
  .cell.off .sample { opacity: 0.45; }
  .nosample { width: 96px; height: 134px; display: grid; place-items: center; border: 1px dashed var(--line); border-radius: 6px; text-align: center; }
  .facts { display: grid; grid-template-columns: 1fr; }
  .facts > div { display: grid; grid-template-columns: 1fr auto; align-items: baseline; gap: 0 10px; padding: 8px 0; }
  .facts > div + div { border-top: 1px solid var(--line); }
  .facts span { color: var(--muted); font-size: 12px; }
  .facts b { font-size: 20px; font-weight: 600; font-variant-numeric: tabular-nums; }
  .facts b small { font-size: 12px; color: var(--muted); font-weight: 400; }
  .facts > div > small { grid-column: 1 / -1; color: var(--muted); font-size: 11px; text-align: right; }
</style>
