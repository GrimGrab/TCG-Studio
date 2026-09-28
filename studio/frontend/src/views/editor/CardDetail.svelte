<script lang="ts">
  import { App, projectFile, money, RARITIES, ELEMENTS, BORDERS, LANES, errText } from '../../lib/api';

  let { project, card, imgBust, notify }: { project: any; card: any; imgBust: number; notify: (t: string, k?: string) => void } = $props();

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
      const rel = await App.PickImage(project.id, 'Choose the card image');
      if (rel) card.image = rel;
    } catch (e) { notify(errText(e), 'error'); }
  }

  const variantKeys = BORDERS.flatMap((b) => [b, b + '_foil']);
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
</script>

<div class="detail">
  <div class="preview" class:framed={project.set.renderMode === 'Framed'}>
    {#if card.image}<img src={projectFile(project.id, card.image, imgBust)} alt={card.name} />{/if}
  </div>
  <div class="row"><button class="small" onclick={changeImage}>Change image…</button><span class="muted small">{card.image}</span></div>

  <label class="field">Name<input bind:value={card.name} /></label>
  <div class="row">
    <label class="field grow">Id (stable)<input value={card.id} readonly title="Ids are save keys and cannot be changed here" /></label>
    <label class="field" style="width:90px">Number<input bind:value={card.number} /></label>
  </div>
  <div class="row">
    <label class="field grow">Rarity
      <select bind:value={card.rarity}>{#each RARITIES as r}<option>{r}</option>{/each}</select>
    </label>
    <label class="field grow">Artist<input bind:value={card.artist} /></label>
  </div>
  <label class="field">Description<textarea rows="4" bind:value={card.description}></textarea></label>

  <h3>Price</h3>
  {#if meta && (meta.usd !== undefined || meta.usdFoil !== undefined)}
    <div class="muted small">Scryfall: {money(meta.usd)} · foil {money(meta.usdFoil)} ({meta.srcRarity})</div>
  {/if}
  <div class="row">
    <label class="field grow">Base price<input type="number" step="0.01" min="0" bind:value={card.price.base} /></label>
    <label class="field grow">Foil ×<input type="number" step="0.1" placeholder={String(project.set.priceDefaults.foilMultiplier)}
      value={card.price.foilMultiplier ?? ''} oninput={(e) => (card.price.foilMultiplier = optNumber(e.currentTarget.value))} /></label>
    <label class="check" title="Gamify and price refresh skip locked cards"><input type="checkbox" checked={!!meta?.locked} onchange={(e) => setLocked(e.currentTarget.checked)} /> Lock</label>
  </div>
  <label class="check small"><input type="checkbox" checked={!!card.price.borderMultipliers} onchange={toggleBorderMults} /> Own border multipliers</label>
  {#if card.price.borderMultipliers}
    <div class="mults">
      {#each BORDERS as b, i}<label class="field">{b}<input type="number" step="0.05" bind:value={card.price.borderMultipliers[i]} /></label>{/each}
    </div>
  {/if}
  <div class="overrides">
    {#each Object.entries(card.price.overrides ?? {}) as [k, v]}
      <div class="row"><span class="grow small">{k}</span>
        <input type="number" step="0.01" style="width:100px" value={v} oninput={(e) => (card.price.overrides[k] = optNumber(e.currentTarget.value) ?? 0)} />
        <button class="small" onclick={() => removeOverride(k)}>✕</button></div>
    {/each}
    <div class="row">
      <select class="grow" bind:value={newOverride}>{#each variantKeys as k}<option>{k}</option>{/each}</select>
      <button class="small" onclick={addOverride}>Add exact price</button>
    </div>
  </div>
  <table class="pricetable">
    <thead><tr><th>Border</th><th>Normal</th><th>Foil</th></tr></thead>
    <tbody>{#each preview as [b, n, f]}<tr><td>{b}</td><td>{money(n as number)}</td><td>{money(f as number)}</td></tr>{/each}</tbody>
  </table>

  <h3>Play table</h3>
  <div class="lanes">
    {#each LANES as l, i}<label class="field">{l}<input type="number" min="0" bind:value={card.play.laneAttack[i]} /></label>{/each}
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
  <label class="field">Effect (raw PlayEffectData JSON — see docs/set-format.md; empty = no effect)
    <textarea rows="6" spellcheck="false" class="mono" bind:value={effectText} onblur={setEffect}
      placeholder={'{ "playEffectQueueDataList": [ { "playEffectType": "DrawCard", "playEffectTypeSecondary": "None", "countList": [1] } ] }'}></textarea>
  </label>
  {#if effectError}<div class="err small">{effectError}</div>{/if}
</div>

<style>
  .detail { padding: 14px; display: flex; flex-direction: column; gap: 10px; }
  .preview { display: flex; justify-content: center; background: var(--bg); border-radius: var(--radius); padding: 10px; }
  .preview img { max-width: 240px; max-height: 330px; border-radius: 10px; }
  .preview.framed img { border-radius: 4px; }
  .small { font-size: 12px; }
  .mults { display: grid; grid-template-columns: repeat(3, 1fr); gap: 6px; }
  .lanes { display: grid; grid-template-columns: repeat(4, 1fr); gap: 6px; }
  .overrides { display: flex; flex-direction: column; gap: 4px; }
  .pricetable { width: 100%; border-collapse: collapse; font-size: 12px; }
  .pricetable th, .pricetable td { text-align: left; padding: 2px 4px; border-bottom: 1px solid var(--line); }
  .mono { font-family: Consolas, monospace; font-size: 12px; }
  .err { color: var(--danger); }
  h3 { margin-top: 8px; }
</style>
