<script lang="ts">
  import { App, projectFile, setRarities, splitWeights, BORDERS, BORDER_LABELS, ask, money } from '../../lib/api';
  import { packSlot, pickSetPicture, putSetPicture, setPicture } from '../../lib/setPictures';
  import PictureSlot from '../PictureSlot.svelte';
  import PackArtEditor from './PackArtEditor.svelte';
  import PackContents from './PackContents.svelte';

  let { project, notify, imgBust, hooks, rebaseArt }: {
    project: any; notify: (t: string, k?: string) => void; imgBust: number;
    hooks: Set<() => Promise<void>>;
    rebaseArt?: (packId: string, which: string, layout: any) => void; // a design converted on opening counts as saved
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
  const rarities = $derived(setRarities(project.set));
  const ownRarities = $derived(!!project.set.rarities?.length);
  // Presets are written in the game's 4 rarities; sets with their own rarities share each one's weight over theirs by place in the list.
  // Real boosters per game come from Go (importer/boosters.go, the same table the imports use); game-style packs live here.
  type Preset = { game: string; name: string; slots: any[]; foilChance?: number; estimate?: boolean };
  const GAME_STYLE: Preset[] = [
    { game: 'Game style', name: 'Vanilla-like odds (all slots)', slots: [] },
    { game: 'Game style', name: 'Rare pack (4 C · 2 R · 1 E/L)', slots: [
      { count: 4, weights: { Common: 1 } }, { count: 2, weights: { Rare: 1 } }, { count: 1, weights: { Epic: 4, Legendary: 1 } }] },
    { game: 'Game style', name: 'Premium (3 R · 3 E · 1 L)', slots: [
      { count: 3, weights: { Rare: 1 } }, { count: 3, weights: { Epic: 1 } }, { count: 1, weights: { Legendary: 1 } }] }
  ];
  // Plain data ($state.raw): a deep $state proxy can't go through structuredClone (it threw, so presets did nothing).
  let presets = $state.raw<Preset[]>(GAME_STYLE);

  /** Whether the set has a version (Set tab → Card versions): border null = any border, foil null = normal or foil. */
  function hasVersion(border: string | null, foil: boolean | null): boolean {
    const v: string[] | undefined = project.set.variants;
    if (!v?.length) return true;
    const on = new Set(v.map((x) => x.toLowerCase()));
    return BORDERS.some((b: string) => (border == null || b === border) &&
      [false, true].some((f) => (foil == null || f === foil) && on.has((f ? `${b}_foil` : b).toLowerCase())));
  }
  App.PackPresets().then((list: any[]) => { presets = [...(list ?? []), ...GAME_STYLE]; }).catch(() => {});
  const presetGames = $derived([...new Set(presets.map((p) => p.game))]);

  // A preset brings its own card count (e.g. a 15-card booster) and the real pack's foil odds; "all slots" keeps the count.
  function applyPreset(name: string) {
    const p = presets.find((x) => x.name === name);
    if (!p) return;
    pack.slots = p.slots.map((s: any) => ({ count: s.count, weights: splitWeights(project.set, { ...s.weights }) }));
    const n = pack.slots.reduce((a: number, s: any) => a + s.count, 0);
    if (n > 0) pack.cardsPerPack = n;
    if (p.foilChance != null && p.game !== 'Game style') pack.foilChance = p.foilChance;
    if (p.estimate) notify(`${p.name}: the publisher doesn't publish the slot odds, so these are estimates.`);
  }

  async function newPack() {
    await artEditor?.flush();
    const n = project.set.packs.length + 1;
    let id = 'booster';
    for (let i = 2; project.set.packs.some((p: any) => p.id === id); i++) id = `booster-${i}`;
    project.set.packs.push({
      id, name: `${project.set.name} Booster${n > 1 ? ' ' + n : ''}`, cardsPerPack: 7, starter: false, hasBox: true,
      packCost: 1.5, marketMin: 1.5, marketMax: 2, license: { packLevel: 1, packPrice: 100, boxLevel: 3, boxPrice: 200 },
      slots: [], foilChance: 5,
      borderOdds: { FullArt: 0.25, EX: 1, Gold: 4, Silver: 8, FirstEdition: 20 }, allowDuplicates: false, cards: []
    });
    index = project.set.packs.length - 1;
  }

  async function removePack() {
    if (!(await ask(`Delete pack "${pack.name}"? Licenses players bought for it are kept in their saves.`))) return;
    project.set.packs.splice(index, 1);
    index = Math.max(0, index - 1);
  }


  let aspects = $state<Record<string, number>>({});
  $effect(() => { App.SetPictureAspects().then((a: Record<string, number>) => (aspects = a)); });

  function optNumber(v: string) { const n = parseFloat(v); return isNaN(n) ? undefined : n; }

  let artBust = $state(0);

  /** The pack's shop icon for the list and header: a picture waiting to be cut, else the saved icon ('' = vanilla). */
  function iconOf(p: any): string {
    const pic = setPicture(project, packSlot(p.id, 'packIcon'), imgBust + artBust);
    return pic.file ? pic.url : p.packIcon ? projectFile(project.id, p.packIcon, imgBust + artBust) : '';
  }
</script>

<div class="wrap">
  <aside class="side sidelist">
    <div class="list">
      {#each project.set.packs as p, i}
        <button class="item" class:active={i === index} onclick={() => selectPack(i)}>
          <span class="ico">{#if iconOf(p)}<img src={iconOf(p)} alt="" />{:else}<span class="glyph"></span>{/if}</span>
          <span class="info">
            <span class="name">{p.name}{#if p.starter}<span class="star" title="Starter: unlocked on new games">★</span>{/if}</span>
            <span class="sub">{p.cardsPerPack} cards · {money(p.packCost)} · level {p.license.packLevel}</span>
          </span>
        </button>
      {:else}
        <p class="muted small empty">No packs yet.</p>
      {/each}
    </div>
    <div class="sidefoot"><button class="primary" onclick={newPack}>+ New pack</button></div>
  </aside>

  {#if pack}
    <div class="pane form">
      <header class="pane-hero">
        <div class="art">{#if iconOf(pack)}<img src={iconOf(pack)} alt="" title="Shop icon" />{:else}<span class="glyph big"></span>{/if}</div>
        <div class="grow titlebox">
          <input class="title" bind:value={pack.name} aria-label="Pack name" placeholder="Pack name" />
          <div class="meta">
            <span>{pack.cardsPerPack} cards per pack</span>
            <span>{pack.cards.length ? `${pack.cards.length} specific cards` : 'whole set'}</span>
            {#if pack.starter}<span class="ok">★ Starter</span>{/if}
            {#if pack.hasBox}<span>box of 8</span>{/if}
            <span class="code" title="Pack id (stable)">{pack.id}</span>
          </div>
        </div>
        <div class="heroactions"><button class="danger" onclick={removePack}>Delete pack</button></div>
      </header>

      <section class="block">
        <div class="bhead"><h2>Basics</h2></div>
        <div class="row wrapr">
          <label class="field grow">Box name<input bind:value={pack.boxName} placeholder={pack.name.endsWith(' Pack') ? pack.name.slice(0, -5) + ' Box' : pack.name + ' Box'} /></label>
        </div>
        <div class="toggles">
          <label class="toggle" class:on={pack.starter}><input type="checkbox" bind:checked={pack.starter} />
            <span><b>Starter</b><span class="muted small">Unlocked on new games</span></span></label>
          <label class="toggle" class:on={pack.hasBox}><input type="checkbox" bind:checked={pack.hasBox} />
            <span><b>Sell a box</b><span class="muted small">A box of 8 packs, with its own license</span></span></label>
          <label class="toggle" class:on={pack.allowDuplicates}><input type="checkbox" bind:checked={pack.allowDuplicates} />
            <span><b>Duplicates</b><span class="muted small">The same card can come twice in one pack</span></span></label>
        </div>
      </section>

      <section class="block">
        <div class="bhead"><h2>Contents</h2>
          {#if pack.slots.length}
            <span class="muted small" title="The slots decide the pack's size: change a slot's card count to change it. Up to 24; the opening's final reveal shows up to 8 in a row, bigger packs make a grid of smaller cards. Gamify prices packs by their card count.">{pack.cardsPerPack} cards, from the slots</span>
          {:else}
            <label class="check small" title="Cards in one pack (1–24) when every card uses vanilla-like odds. With slots, their counts decide the size. Gamify prices packs by their card count.">Cards per pack
              <input type="number" min="1" max="24" style="width:4.5em" bind:value={pack.cardsPerPack} /></label>
          {/if}
          <span class="grow"></span>
          <select onchange={(e) => { if (e.currentTarget.value) applyPreset(e.currentTarget.value); e.currentTarget.value = ''; }}>
            <option value="">Apply a real booster or preset…</option>
            {#each presetGames as g}<optgroup label={g}>{#each presets.filter((p) => p.game === g) as p}<option value={p.name}>{p.name}{p.estimate ? ' (estimate)' : ''}</option>{/each}</optgroup>{/each}
          </select>
        </div>
        {#if pack.slots.length === 0}
          <div class="notice info">{ownRarities ? 'No slots: every card uses vanilla-like odds, each rarity by its place in the list.'
            : 'No slots: every card uses vanilla-like odds (Rare 10%, Epic 2%, Legendary 0.1%).'} Apply a preset or add a slot to choose your own weights.</div>
          <div class="row"><button class="small" onclick={() => pack.slots.push({ count: pack.cardsPerPack, weights: { [rarities[0]?.id ?? 'Common']: 1 } })}>+ Slot</button></div>
        {:else}
          <PackContents {project} {pack} />
        {/if}
        <div class="pool">
          <span><b>Card pool:</b> {pack.cards.length ? `${pack.cards.length} specific cards` : 'the whole set'}</span>
          {#if pack.cards.length}<button class="small" onclick={() => (pack.cards = [])}>Use the whole set</button>{/if}
          <span class="muted small">Select cards in the Cards tab to add them to this pack's list.</span>
        </div>
      </section>

      <section class="block">
        <div class="bhead"><h2>Foil &amp; borders</h2><span class="muted small">Chance per card, on top of its rarity.</span></div>
        <div class="odds">
          <label class="odd foil" class:off={!hasVersion(null, true)} title={hasVersion(null, true) ? '' : 'The set has no foil versions (Set details → Card versions): no card comes out foil.'}>
            <span class="lbl">Foil</span>
            <span class="in"><input type="number" step="0.5" min="0" max="100" bind:value={pack.foilChance} disabled={!hasVersion(null, true)} />%</span>
            {#if !hasVersion(null, true)}<span class="muted small">no foil versions</span>{/if}
          </label>
          {#each BORDERS.slice(1) as b}
            <label class="odd b-{b}" class:off={!hasVersion(b, null)} title={hasVersion(b, null) ? '' : 'Off for this set (Set details → Card versions): this border never comes out of packs.'}>
              <span class="lbl">{BORDER_LABELS[b]}</span>
              <span class="in"><input type="number" step="0.05" min="0" value={pack.borderOdds[b] ?? 0} disabled={!hasVersion(b, null)} oninput={(e) => (pack.borderOdds[b] = optNumber(e.currentTarget.value) ?? 0)} />%</span>
              {#if !hasVersion(b, null)}<span class="muted small">off for this set</span>{/if}
            </label>
          {/each}
        </div>
      </section>

      <section class="block">
        <div class="bhead"><h2>Shop &amp; license</h2>
          <span class="muted small">Like vanilla, each product sells in a small and a big delivery box with separate licenses. Gamify fills all of them from the vanilla table.</span></div>
        <table class="shop">
          <thead><tr><th></th><th>Cost</th><th>License level</th><th>License price</th><th>Big delivery level</th><th>Big delivery price</th></tr></thead>
          <tbody>
            <tr>
              <td class="rh">Pack</td>
              <td><input type="number" step="0.1" bind:value={pack.packCost} /></td>
              <td><input type="number" min="1" bind:value={pack.license.packLevel} /></td>
              <td><input type="number" min="0" bind:value={pack.license.packPrice} /></td>
              <td><input type="number" min="1" placeholder={String(pack.license.packLevel + 1)} value={pack.license.packBigLevel ?? ''} oninput={(e) => (pack.license.packBigLevel = optNumber(e.currentTarget.value))} /></td>
              <td><input type="number" min="0" placeholder={String(Math.round(pack.license.packPrice * 1.5))} value={pack.license.packBigPrice ?? ''} oninput={(e) => (pack.license.packBigPrice = optNumber(e.currentTarget.value))} /></td>
            </tr>
            <tr class:dim={!pack.hasBox}>
              <td class="rh">Box</td>
              <td><input type="number" step="1" placeholder="8 × pack" value={pack.boxCost ?? ''} oninput={(e) => (pack.boxCost = optNumber(e.currentTarget.value))} /></td>
              <td><input type="number" min="1" bind:value={pack.license.boxLevel} /></td>
              <td><input type="number" min="0" bind:value={pack.license.boxPrice} /></td>
              <td><input type="number" min="1" placeholder={String(pack.license.boxLevel + 1)} value={pack.license.boxBigLevel ?? ''} oninput={(e) => (pack.license.boxBigLevel = optNumber(e.currentTarget.value))} /></td>
              <td><input type="number" min="0" placeholder={String(Math.round(pack.license.boxPrice * 1.5))} value={pack.license.boxBigPrice ?? ''} oninput={(e) => (pack.license.boxBigPrice = optNumber(e.currentTarget.value))} /></td>
            </tr>
          </tbody>
        </table>
        <div class="row market">
          <label class="field" title="The pack's market price moves between these multiples of its cost">Market min ×<input type="number" step="0.1" bind:value={pack.marketMin} /></label>
          <label class="field">Market max ×<input type="number" step="0.1" bind:value={pack.marketMax} /></label>
          <span class="muted small">Market price: {money(pack.packCost * pack.marketMin)} – {money(pack.packCost * pack.marketMax)} per pack</span>
        </div>
      </section>

      <section class="block">
        <div class="bhead"><h2>Art</h2></div>
        <PackArtEditor bind:this={artEditor} {project} {pack} {notify} {hooks} {rebaseArt} onapplied={() => (artBust = Date.now())} />
        <h3 class="sub">Finished images</h3>
        <p class="muted small">What the pack uses (made by the editor above), or your own picture: textures in the
          vanilla layout (cut to 1024²), icons in the vanilla icon's shape. Drag the box to pick the part used; saving the set cuts it.
          Empty = vanilla art.</p>
        <div class="art-grid">
          {#each [['packTexture', 'Pack texture'], ['packIcon', 'Pack icon'], ...(pack.hasBox ? [['boxTexture', 'Box texture'], ['boxIcon', 'Box icon']] : [])] as [field, label] (pack.id + field)}
            {@const slot = packSlot(pack.id, field)}
            <div class="artbox">
              <div class="small lbl">{label}</div>
              {#if !pack[field] && !project.meta.pictures?.[slot]}<div class="thumb"><span class="muted small">vanilla</span></div>{/if}
              <PictureSlot compact aspect={aspects[field] ?? 1} {notify} pick={() => pickSetPicture(project, slot)} chooseLabel="Choose…"
                removeLabel="Clear" fallback={pack[field] ? projectFile(project.id, pack[field], imgBust + artBust) : ''}
                bind:value={() => setPicture(project, slot, imgBust + artBust), (v) => putSetPicture(project, slot, v)} />
            </div>
          {/each}
        </div>
      </section>
    </div>
  {:else}
    <div class="emptypane">
      <div class="emptyglyph"><span class="glyph big"></span></div>
      <h1>No packs yet</h1>
      <p class="muted">Cards can still be granted, but players can't buy them. Add a pack to sell this set in the shop.</p>
      <div class="row center"><button class="primary" onclick={newPack}>+ New pack</button></div>
    </div>
  {/if}
</div>

<style>
  .wrap { display: flex; height: 100%; }
  .side { width: 270px; flex-shrink: 0; border-right: 1px solid var(--line); display: flex; flex-direction: column; min-height: 0; }
  .ico { width: 40px; height: 52px; flex-shrink: 0; display: grid; place-items: center; background: var(--bg); border-radius: 6px; overflow: hidden; }
  .ico img { width: 100%; height: 100%; object-fit: contain; }
  .star { color: var(--warn); margin-left: 5px; font-size: 12px; }
  /* A pack shape, for packs with the vanilla icon. */
  .glyph { display: block; width: 24px; height: 34px; border-radius: 3px; background: linear-gradient(160deg, #3d4a6b, #262a34);
    clip-path: polygon(0 6%, 12% 0, 25% 6%, 37% 0, 50% 6%, 62% 0, 75% 6%, 87% 0, 100% 6%, 100% 94%, 87% 100%, 75% 94%, 62% 100%, 50% 94%, 37% 100%, 25% 94%, 12% 100%, 0 94%); }
  .glyph.big { width: 52px; height: 74px; }

  .form { flex: 1; min-width: 0; overflow: auto; }
  .art { width: 88px; height: 110px; flex: none; display: grid; place-items: center; background: var(--panel); border: 1px solid var(--line); border-radius: 12px; overflow: hidden; }
  .art img { width: 80px; height: 100px; object-fit: contain; }
  .meta .ok { color: var(--warn); }
  .wrapr { flex-wrap: wrap; }
  .wrapr .field { max-width: 480px; }

  .toggles { display: grid; grid-template-columns: repeat(auto-fill, minmax(220px, 1fr)); gap: 8px; }
  .toggle { display: flex; align-items: flex-start; gap: 10px; padding: 9px 12px; border: 1px solid var(--line); border-radius: 8px; background: var(--bg); cursor: pointer; }
  .toggle > span { display: flex; flex-direction: column; gap: 1px; }
  .toggle input { margin-top: 3px; flex: none; }
  .toggle.on { border-color: var(--accent-2); background: #1c2740; }

  .pool { display: flex; flex-wrap: wrap; align-items: center; gap: 6px 12px; padding-top: 10px; border-top: 1px solid var(--line); font-size: 13px; }

  .odds { display: grid; grid-template-columns: repeat(auto-fill, minmax(150px, 1fr)); gap: 8px; }
  .odd { display: flex; flex-direction: column; gap: 6px; padding: 10px 12px; border-radius: 8px; border: 1px solid var(--line); background: var(--bg);
    border-top: 3px solid var(--line); }
  .odd .lbl { font-size: 12px; color: var(--muted); text-transform: uppercase; letter-spacing: 0.05em; }
  .odd .in { display: flex; align-items: center; gap: 4px; color: var(--muted); }
  .odd input { width: 80px; font-size: 16px; }
  .odd.off { opacity: 0.55; }
  .odd.foil { border-top-color: #b66dff; }
  .odd.b-FirstEdition { border-top-color: #8a93a3; }
  .odd.b-Silver { border-top-color: #c9d1dc; }
  .odd.b-Gold { border-top-color: #e2b53c; }
  .odd.b-EX { border-top-color: #ff5e6a; }
  .odd.b-FullArt { border-top-color: #3ecf8e; }

  .shop { border-collapse: collapse; font-size: 13px; width: 100%; max-width: 820px; }
  .shop th { text-align: left; font-weight: 500; font-size: 12px; color: var(--muted); padding: 0 8px 6px 0; }
  .shop td { padding: 3px 8px 3px 0; }
  .shop input { width: 100%; min-width: 70px; }
  .shop .rh { font-weight: 600; width: 60px; }
  .shop tr.dim { opacity: 0.5; }
  .market { flex-wrap: wrap; align-items: flex-end; gap: 10px 14px; }
  .market .field { width: 130px; }

  h3.sub { font-size: 12px; margin-top: 4px; }
  .art-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(170px, 1fr)); gap: 12px; }
  .artbox { display: flex; flex-direction: column; gap: 6px; padding: 10px; background: var(--bg); border: 1px solid var(--line); border-radius: 8px; }
  .artbox .lbl { font-weight: 600; }
  .thumb { aspect-ratio: 1; background: var(--panel); border-radius: 6px; display: flex; align-items: center; justify-content: center; overflow: hidden; }

  .emptypane { flex: 1; margin-top: 14vh; align-items: center; }
</style>
