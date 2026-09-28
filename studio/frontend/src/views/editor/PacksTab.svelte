<script lang="ts">
  import { App, projectFile, RARITIES, BORDERS, errText, ask } from '../../lib/api';
  import PackArtEditor from './PackArtEditor.svelte';

  let { project, notify, imgBust, save, hooks }: {
    project: any; notify: (t: string, k?: string) => void; imgBust: number;
    save: () => Promise<void>; hooks: Set<() => Promise<void>>;
  } = $props();
  let artEditor = $state<any>();

  /** Switches packs after applying the art editor's unapplied edits to the current one. */
  async function selectPack(i: number) {
    if (i === index) return;
    await artEditor?.flush();
    index = i;
  }

  let index = $state(0);
  const pack = $derived(project.set.packs[index]);
  const slotSum = $derived(pack ? pack.slots.reduce((s: number, x: any) => s + (x.count || 0), 0) : 0);

  const PRESETS: Record<string, any[]> = {
    'MTG-like (5 C · 1 U · 1 R/M)': [
      { count: 5, weights: { Common: 1 } }, { count: 1, weights: { Rare: 1 } }, { count: 1, weights: { Epic: 7, Legendary: 1 } }],
    'Vanilla-like odds (all slots)': [],
    'Rare pack (4 C · 2 R · 1 E/L)': [
      { count: 4, weights: { Common: 1 } }, { count: 2, weights: { Rare: 1 } }, { count: 1, weights: { Epic: 4, Legendary: 1 } }],
    'Premium (3 R · 3 E · 1 L)': [
      { count: 3, weights: { Rare: 1 } }, { count: 3, weights: { Epic: 1 } }, { count: 1, weights: { Legendary: 1 } }]
  };

  async function newPack() {
    await artEditor?.flush();
    const n = project.set.packs.length + 1;
    let id = 'booster';
    for (let i = 2; project.set.packs.some((p: any) => p.id === id); i++) id = `booster-${i}`;
    project.set.packs.push({
      id, name: `${project.set.name} Booster${n > 1 ? ' ' + n : ''}`, cardsPerPack: 7, starter: false, hasBox: true,
      packCost: 1.5, marketMin: 1.5, marketMax: 2, license: { packLevel: 1, packPrice: 100, boxLevel: 3, boxPrice: 200 },
      slots: structuredClone(PRESETS['MTG-like (5 C · 1 U · 1 R/M)']), foilChance: 5,
      borderOdds: { FullArt: 0.25, EX: 1, Gold: 4, Silver: 8, FirstEdition: 20 }, allowDuplicates: false, cards: []
    });
    index = project.set.packs.length - 1;
  }

  async function removePack() {
    if (!(await ask(`Delete pack "${pack.name}"? Licenses players bought for it are kept in their saves.`))) return;
    project.set.packs.splice(index, 1);
    index = Math.max(0, index - 1);
  }

  function applyPreset(name: string) { pack.slots = structuredClone(PRESETS[name]); }

  function weight(slot: any, r: string, v: string) {
    const n = parseFloat(v);
    if (isNaN(n) || n <= 0) delete slot.weights[r];
    else slot.weights[r] = n;
    slot.weights = { ...slot.weights };
  }

  async function pick(field: string, title: string) {
    try {
      const rel = await App.PickImage(project.id, title);
      if (rel) pack[field] = rel;
    } catch (e) { notify(errText(e), 'error'); }
  }

  function optNumber(v: string) { const n = parseFloat(v); return isNaN(n) ? undefined : n; }

  let artColor = $state('#3a7bd5');
  let artTitle = $state('');
  let artBust = $state(0);
  let generating = $state(false);
  $effect(() => { App.DefaultArtColor(project.id).then((c: string) => (artColor = c)); });

  let frontImage = $state('');
  let titleOnImage = $state(false);

  async function pickFront() {
    try {
      const rel = await App.PickImage(project.id, 'Choose artwork for the pack front (e.g. a booster photo)');
      if (rel) frontImage = rel;
    } catch (e) { notify(errText(e), 'error'); }
  }

  async function generateArt() {
    generating = true;
    try {
      const r = await App.GeneratePackArt(project.id, pack.id, { color: artColor, title: artTitle, icon: '', filePrefix: '', frontImage, titleOnImage } as any);
      pack.packTexture = r.packTexture; pack.packIcon = r.packIcon; pack.boxTexture = r.boxTexture; pack.boxIcon = r.boxIcon;
      artBust = Date.now();
      await save(); // keeps it and updates the game like any other save
    } catch (e) { notify(errText(e), 'error'); }
    generating = false;
  }
</script>

<div class="wrap">
  <div class="packs">
    {#each project.set.packs as p, i}
      <button class:active={i === index} onclick={() => selectPack(i)}>{p.name}{p.starter ? ' ★' : ''}</button>
    {/each}
    <button onclick={newPack}>+ New pack</button>
  </div>

  {#if pack}
    <div class="form">
      <section>
        <div class="row">
          <label class="field grow">Pack name<input bind:value={pack.name} /></label>
          <label class="field grow">Box name<input bind:value={pack.boxName} placeholder="(pack name + Box)" /></label>
          <label class="field" style="width:130px">Id<input value={pack.id} readonly /></label>
        </div>
        <div class="row">
          <label class="check"><input type="checkbox" bind:checked={pack.starter} /> Starter (unlocked on new games)</label>
          <label class="check"><input type="checkbox" bind:checked={pack.hasBox} /> Also sell a box of 8</label>
          <label class="check"><input type="checkbox" bind:checked={pack.allowDuplicates} /> Allow duplicates in one pack</label>
          <div class="grow"></div>
          <button class="danger small" onclick={removePack}>Delete pack</button>
        </div>
      </section>

      <section>
        <h3>Shop</h3>
        <div class="grid4">
          <label class="field">Pack cost<input type="number" step="0.1" bind:value={pack.packCost} /></label>
          <label class="field">Box cost<input type="number" step="1" placeholder="8 × pack" value={pack.boxCost ?? ''} oninput={(e) => (pack.boxCost = optNumber(e.currentTarget.value))} /></label>
          <label class="field">Market min ×<input type="number" step="0.1" bind:value={pack.marketMin} /></label>
          <label class="field">Market max ×<input type="number" step="0.1" bind:value={pack.marketMax} /></label>
          <label class="field">Pack license level<input type="number" min="1" bind:value={pack.license.packLevel} /></label>
          <label class="field">Pack license price<input type="number" min="0" bind:value={pack.license.packPrice} /></label>
          <label class="field">Box license level<input type="number" min="1" bind:value={pack.license.boxLevel} /></label>
          <label class="field">Box license price<input type="number" min="0" bind:value={pack.license.boxPrice} /></label>
          <label class="field">Big pack delivery level<input type="number" min="1" placeholder={String(pack.license.packLevel + 1)} value={pack.license.packBigLevel ?? ''} oninput={(e) => (pack.license.packBigLevel = optNumber(e.currentTarget.value))} /></label>
          <label class="field">Big pack delivery price<input type="number" min="0" placeholder={String(Math.round(pack.license.packPrice * 1.5))} value={pack.license.packBigPrice ?? ''} oninput={(e) => (pack.license.packBigPrice = optNumber(e.currentTarget.value))} /></label>
          <label class="field">Big box delivery level<input type="number" min="1" placeholder={String(pack.license.boxLevel + 1)} value={pack.license.boxBigLevel ?? ''} oninput={(e) => (pack.license.boxBigLevel = optNumber(e.currentTarget.value))} /></label>
          <label class="field">Big box delivery price<input type="number" min="0" placeholder={String(Math.round(pack.license.boxPrice * 1.5))} value={pack.license.boxBigPrice ?? ''} oninput={(e) => (pack.license.boxBigPrice = optNumber(e.currentTarget.value))} /></label>
        </div>
        <p class="muted small">Like vanilla, each product is sold in a small and a big delivery box with separate licenses. Gamify fills all four from the vanilla table.</p>
      </section>

      <section>
        <div class="row"><h3 class="grow">Contents ({pack.cardsPerPack} cards)</h3>
          <select onchange={(e) => { if (e.currentTarget.value) applyPreset(e.currentTarget.value); e.currentTarget.value = ''; }}>
            <option value="">Apply preset…</option>{#each Object.keys(PRESETS) as k}<option>{k}</option>{/each}
          </select>
        </div>
        {#if pack.slots.length === 0}
          <p class="muted">No slots: every card uses vanilla-like odds (Rare 10%, Epic 2%, Legendary 0.1%).</p>
        {/if}
        {#each pack.slots as slot, si}
          <div class="slot row">
            <label class="field" style="width:70px">Cards<input type="number" min="1" bind:value={slot.count} /></label>
            {#each RARITIES as r}
              <label class="field">{r} weight<input type="number" min="0" step="0.5" value={slot.weights[r] ?? ''} placeholder="0" oninput={(e) => weight(slot, r, e.currentTarget.value)} /></label>
            {/each}
            <button class="small" onclick={() => pack.slots.splice(si, 1)}>✕</button>
          </div>
        {/each}
        <div class="row">
          <button class="small" onclick={() => pack.slots.push({ count: 1, weights: { Common: 1 } })}>+ Slot</button>
          {#if pack.slots.length && slotSum !== pack.cardsPerPack}<span class="err">Slot counts add up to {slotSum}, must be {pack.cardsPerPack}</span>{/if}
        </div>
        <div class="grid4">
          <label class="field">Foil chance %<input type="number" step="0.5" min="0" max="100" bind:value={pack.foilChance} /></label>
          {#each BORDERS.slice(1) as b}
            <label class="field">{b} %<input type="number" step="0.05" min="0" value={pack.borderOdds[b] ?? 0} oninput={(e) => (pack.borderOdds[b] = optNumber(e.currentTarget.value) ?? 0)} /></label>
          {/each}
        </div>
        <p class="muted small">Card pool: {pack.cards.length ? `${pack.cards.length} specific cards` : 'whole set'}
          {#if pack.cards.length}<button class="small" onclick={() => (pack.cards = [])}>Use whole set</button>{/if}
          — select cards in the Cards tab to add them to this pack's list.</p>
      </section>

      <section>
        <h3>Art</h3>
        <PackArtEditor bind:this={artEditor} {project} {pack} {notify} {save} {hooks} onapplied={() => (artBust = Date.now())} />
        <h4>Quick start</h4>
        <div class="row gen">
          <label class="field">Color<input type="color" bind:value={artColor} /></label>
          <label class="field grow">Title on the pack<input bind:value={artTitle} placeholder={project.set.name} /></label>
          <button class="primary" disabled={generating} onclick={generateArt}>{generating ? 'Generating…' : 'Generate pack & box art'}</button>
        </div>
        <div class="row">
          <span class="muted small">Pack front:</span>
          {#if frontImage}
            <img class="front-thumb" src={projectFile(project.id, frontImage, artBust)} alt="front art" />
            <span class="small">{frontImage}</span>
            <label class="check small"><input type="checkbox" bind:checked={titleOnImage} /> Print title over it</label>
            <button class="small" onclick={() => (frontImage = '')}>Use generated design</button>
          {:else}
            <span class="small">generated (colour + set icon + title)</span>
          {/if}
          <button class="small" onclick={pickFront}>Use my own image…</button>
        </div>
        <p class="muted small">Generated from the game's templates with the set icon and name. Or choose your own images (1024² in the vanilla layout — templates are in BepInEx\plugins\TCGCustomCards\templates). Empty = vanilla art.</p>
        <div class="art">
          {#each [['packTexture', 'Pack texture'], ['packIcon', 'Pack icon'], ['boxTexture', 'Box texture'], ['boxIcon', 'Box icon']] as [field, label]}
            <div class="artbox">
              <div class="thumb">{#if pack[field]}<img src={projectFile(project.id, pack[field], imgBust + artBust)} alt={label} />{:else}<span class="muted">vanilla</span>{/if}</div>
              <div class="small">{label}</div>
              <div class="row">
                <button class="small" onclick={() => pick(field, `Choose ${label.toLowerCase()}`)}>Choose…</button>
                {#if pack[field]}<button class="small" onclick={() => (pack[field] = undefined)}>Clear</button>{/if}
              </div>
            </div>
          {/each}
        </div>
      </section>
    </div>
  {:else}
    <p class="muted" style="padding:20px">This set has no packs yet. Cards can still be granted, but players can't buy them.</p>
  {/if}
</div>

<style>
  .wrap { display: flex; height: 100%; }
  .packs { width: 220px; border-right: 1px solid var(--line); padding: 10px; display: flex; flex-direction: column; gap: 4px; overflow: auto; }
  .packs button { text-align: left; }
  .packs button.active { border-color: var(--accent); background: #22304d; }
  .form { flex: 1; overflow: auto; padding: 14px 18px; display: flex; flex-direction: column; gap: 14px; }
  section { background: var(--panel); border: 1px solid var(--line); border-radius: var(--radius); padding: 12px; display: flex; flex-direction: column; gap: 10px; }
  .grid4 { display: grid; grid-template-columns: repeat(4, 1fr); gap: 8px; }
  .slot { align-items: flex-end; flex-wrap: wrap; }
  .slot .field { width: 110px; }
  .gen { align-items: flex-end; }
  h4 { margin: 6px 0 0; font-size: 13px; color: var(--muted, #9aa4b2); }
  .front-thumb { height: 48px; border-radius: 4px; }
  .gen input[type="color"] { width: 60px; height: 32px; padding: 2px; }
  .art { display: grid; grid-template-columns: repeat(4, 1fr); gap: 12px; }
  .artbox { display: flex; flex-direction: column; gap: 6px; }
  .thumb { aspect-ratio: 1; background: var(--bg); border-radius: 6px; display: flex; align-items: center; justify-content: center; overflow: hidden; }
  .thumb img { max-width: 100%; max-height: 100%; }
  .small { font-size: 12px; }
  .err { color: var(--danger); font-size: 12px; }
</style>
