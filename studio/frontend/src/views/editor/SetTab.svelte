<script lang="ts" module>
  type Page = 'overview' | 'basics' | 'look' | 'rarities' | 'versions' | 'prices';
  let rememberedPage: Page = 'overview';
</script>

<script lang="ts">
  // The Set tab: set-wide rules every card follows unless it has its own (versions, shaders, prices), and the set's look.
  // A side list of pages like Mod settings: .sidelist, then a .pane with .pane-hero and .blocks of .entry settings.
  import { App, projectFile, FRAME_TEMPLATES, BORDERS, BORDER_LABELS, money, setRarities, rarityColor, errText } from '../../lib/api';
  import { pickSetPicture, putSetPicture, setPicture } from '../../lib/setPictures';
  import { ALL_VERSIONS, hasOwnVersions, setShader, setVersions } from '../../lib/cardVersions';
  import { untrack } from 'svelte';
  import PictureSlot from '../PictureSlot.svelte';
  import CardBackPreview from '../CardBackPreview.svelte';
  import RaritiesEditor from './RaritiesEditor.svelte';
  import VariantsEditor from './VariantsEditor.svelte';
  import ShadersEditor from './ShadersEditor.svelte';
  import type { NavRequest, TakeNav } from '../../lib/setNav';

  let { project, notify, imgBust, nav, takeNav }: {
    project: any; notify: (t: string, k?: string) => void; imgBust: number; nav?: NavRequest | null; takeNav?: TakeNav;
  } = $props();

  const PAGES: [Page, string, string][] = [
    ['overview', 'Overview', 'Cards, packs, value'], ['basics', 'Basics', 'Name, collection, release'],
    ['look', 'Card look', 'Style, shaders, card back'], ['rarities', 'Rarities', 'Names, colours, order'],
    ['versions', 'Card versions', 'Borders and foil'], ['prices', 'Prices', 'Default multipliers']
  ];

  let backBust = $state(0);
  let backColor = $state('#3a7bd5');
  let aspects = $state<Record<string, number>>({});
  $effect(() => { App.SetPictureAspects().then((a: Record<string, number>) => (aspects = a)); });
  const own = $derived(setPicture(project, 'cardBack', imgBust + backBust)); // the picture this set's back is cut from
  let globalBack = $state('');
  $effect(() => { App.DefaultArtColor(project.id).then((c: string) => (backColor = c)); App.GlobalCardBack().then((u: string) => (globalBack = u)); });
  let collections: string[] = $state([]);
  $effect(() => { App.Collections().then((c: string[]) => (collections = c ?? [])); });

  async function generateBack() {
    try {
      putSetPicture(project, 'cardBack', { file: '', url: '', crop: null }); // the generated back replaces any picture
      project.set.cardBack = await App.GenerateSetCardBack(project.id, backColor, project.set.name);
      backBust = Date.now();
      notify('Card back generated — save to keep it', 'ok');
    } catch (e) { notify(errText(e), 'error'); }
  }

  // Numbers and summaries.
  const cards = $derived(project.set.cards.length);
  const value = $derived(project.set.cards.reduce((s: number, c: any) => s + (c.price?.base ?? 0), 0));
  const versionsOn = $derived(setVersions(project.set).size);
  const rarities = $derived(setRarities(project.set));
  const rarityCounts = $derived(rarities.map((r) => [r, project.set.cards.filter((c: any) => c.rarity === r.id).length] as [any, number]));
  const shadersSet = $derived(ALL_VERSIONS.filter((k) => setShader(project.set, k)).length);
  const backKind = $derived(own.file || project.set.cardBack ? 'own' : globalBack ? 'global' : 'frame');
  const backLabel = $derived(backKind === 'own' ? 'Own back' : backKind === 'global' ? 'Global back' : `${project.set.frameTemplate} frame's back`);
  const frontImage = $derived(project.set.cards.find((c: any) => c.image)?.image ?? '');
  const summary = $derived<Record<Page, string>>({
    overview: '',
    basics: [project.set.collection || 'No collection', project.set.released?.slice(0, 4)].filter(Boolean).join(' · '),
    look: `${project.set.renderMode === 'Framed' ? 'Framed' : 'Full image'} · ${shadersSet ? `${shadersSet} shader${shadersSet === 1 ? '' : 's'}` : 'default shaders'} · ${backLabel}`,
    rarities: rarities.map((r) => r.name).join(' · '),
    versions: `${versionsOn} of 12${project.set.cards.some(hasOwnVersions) ? ' · some cards own' : ''}`,
    prices: `Foil ×${project.set.priceDefaults.foilMultiplier} · min ${money(project.set.priceDefaults.minimum)}`
  });

  /** What a card with the example base sells for in each version, with the set's defaults. */
  let exampleBase = $state(1);
  const example = $derived(BORDERS.map((b: string, i: number) => {
    const d = project.set.priceDefaults;
    const base = exampleBase > 0 ? exampleBase : 1;
    return [b, Math.max(d.minimum, base * d.borderMultipliers[i]), Math.max(d.minimum, base * d.borderMultipliers[i] * d.foilMultiplier)] as [string, number, number];
  }));
  const maxExample = $derived(Math.max(0.01, ...example.map(([, , f]) => f)));

  // The side list opens one page at a time (remembered while Studio runs).
  let page = $state<Page>(rememberedPage);
  function open(p: Page) { page = rememberedPage = p; }
  // Links from other tabs ("Set → Card versions"); shaders and the card back are on the card look page.
  $effect(() => {
    if (nav?.tab !== 'set') return;
    untrack(() => {
      const s = takeNav?.('set')?.section;
      const p = s === 'shaders' || s === 'back' ? 'look' : s;
      if (p && PAGES.some(([id]) => id === p)) open(p as Page);
    });
  });
</script>

<div class="settab setpages">
  <nav class="side sidelist" aria-label="Set pages">
    <div class="list">
      {#each PAGES as [id, label, sub]}
        <button class="item" class:active={page === id} onclick={() => open(id)}>
          <span class="info"><span class="name">{label}</span><span class="sub">{sub}</span></span></button>
      {/each}
    </div>
  </nav>

  <div class="scroll">
    {#if page === 'overview'}
      <div class="pane">
        <header class="pane-hero"><div class="grow"><h1>Overview</h1><div class="muted">The set at a glance. Open a page on the left, or a tile below, to change it.</div></div></header>
        <div class="stats">
          <div class="stat"><span class="label">Cards</span><b>{cards.toLocaleString()}</b><span class="sub">{rarities.length} rarities</span></div>
          <div class="stat"><span class="label">Packs</span><b>{project.set.packs.length}</b><span class="sub">{project.set.packs.filter((p: any) => p.starter).length} starter</span></div>
          <div class="stat"><span class="label">Card versions</span><b>{versionsOn}<small> / 12</small></b><span class="sub">border × foil</span></div>
          <div class="stat"><span class="label">Base value</span><b>{money(value)}</b><span class="sub">one of every card · avg {money(cards ? value / cards : 0)}</span></div>
        </div>
        {#if cards}
          <div class="block">
            <div class="bhead"><h2>Rarity mix</h2><span class="muted small">cards per rarity</span></div>
            <div>
              <div class="stack">
                {#each rarityCounts as [r, n]}{#if n}<span style="flex:{n}; background:{rarityColor(project.set, r.id)}" title="{r.name}: {n}"></span>{/if}{/each}
              </div>
              <div class="legend">
                {#each rarityCounts as [r, n]}
                  <span><i style="background:{rarityColor(project.set, r.id)}"></i>{r.name}<b>{n}</b><em>{Math.round((n / cards) * 100)}%</em></span>
                {/each}
              </div>
            </div>
          </div>
        {/if}
        <div class="tiles">
          {#each PAGES.slice(1) as [id, label]}
            <button class="tile" onclick={() => open(id)}><span><b>{label}</b><span class="muted small">{summary[id]}</span></span><span class="go">›</span></button>
          {/each}
        </div>
      </div>

    {:else if page === 'basics'}
      <div class="pane">
        <header class="pane-hero"><div class="grow"><h1>Basics</h1><div class="muted">What the set is called and where the game files it.</div></div></header>
        <div class="page-cols">
          <div class="block entries">
            <div class="entry"><div class="head row"><span class="grow name">Set name</span><span class="control"><input bind:value={project.set.name} /></span></div><div class="desc muted small">Shown in the shop, the binder and Check Price.</div></div>
            <div class="entry"><div class="head row"><span class="grow name">Collection</span><span class="control"><input list="tcg-collections" placeholder="e.g. Magic: The Gathering, Gundam" bind:value={project.set.collection} />
                <datalist id="tcg-collections">{#each collections as c}<option value={c}></option>{/each}</datalist></span></div><div class="desc muted small">The game or series. The binder groups sets by it, e.g. every Gundam set at once.</div></div>
            <div class="entry"><div class="head row"><span class="grow name">Release date</span><span class="control"><input type="date" class="short" bind:value={project.set.released} /></span></div><div class="desc muted small">Orders sets inside their collection, oldest first.</div></div>
            <div class="entry"><div class="head row"><span class="grow name">Gamify tier</span><span class="control"><input type="number" min="0" class="short" bind:value={project.meta.tier} /></span></div><div class="desc muted small">Its place in the shop's progression when Gamify sets licenses and prices.</div></div>
            <div class="entry"><div class="head row"><span class="grow name">Set id</span><span class="control"><code class="idbox">{project.set.id}</code></span></div><div class="desc muted small">Saves refer to it, so it never changes.</div></div>
          </div>
          <aside class="page-side">
            <div class="notice info howto"><b>This setup's copy</b>
              <ul><li>Names, prices and tiers you change here only affect this setup.</li>
                <li>To hand this version to your other setups, use ⋯ → Update catalog.</li></ul></div>
          </aside>
        </div>
      </div>

    {:else if page === 'look'}
      <div class="pane">
        <header class="pane-hero"><div class="grow"><h1>Card look</h1><div class="muted">How the game draws this set's cards: front style, effects per version and the back.</div></div></header>
        <div class="page-cols">
          <div class="maincol">
            <div class="block">
              <div class="bhead"><h2>Card style</h2></div>
              <div>
                <div class="choices">
                  <button class="choice" class:on={project.set.renderMode === 'FullImage'} onclick={() => (project.set.renderMode = 'FullImage')}>
                    <span class="glyph full"></span>
                    <span><b>Full image</b><span class="muted small">The picture is the whole card — scans of real cards.</span></span>
                    {#if project.set.renderMode === 'FullImage'}<span class="tick">✓</span>{/if}
                  </button>
                  <button class="choice" class:on={project.set.renderMode === 'Framed'} onclick={() => (project.set.renderMode = 'Framed')}>
                    <span class="glyph framed"><span></span></span>
                    <span><b>Framed</b><span class="muted small">The picture is artwork inside the game's own card frame.</span></span>
                    {#if project.set.renderMode === 'Framed'}<span class="tick">✓</span>{/if}
                  </button>
                </div>
              </div>
              <div class="entry"><div class="head row"><span class="grow name">Frame template</span><span class="control"><select bind:value={project.set.frameTemplate}>{#each FRAME_TEMPLATES as f}<option>{f}</option>{/each}</select></span></div><div class="desc muted small">The game set whose borders, foil and default back this set uses.</div></div>
            </div>

            <ShadersEditor {project} {imgBust} />

            <div class="block">
              <div class="bhead"><h2>Card back</h2><span class="badge" class:ok={backKind === 'own'}>{backLabel}</span></div>
              <div class="backbody">
                <PictureSlot aspect={aspects.cardBack ?? 718 / 1006} {notify} pick={() => pickSetPicture(project, 'cardBack')}
                  bind:value={() => own, (v) => putSetPicture(project, 'cardBack', v)}
                  fallback={project.set.cardBack ? projectFile(project.id, project.set.cardBack, imgBust + backBust) : ''}
                  removeLabel="Use {globalBack ? 'global' : "template's"} back">
                  {#snippet extra()}
                    <span class="gen"><input type="color" bind:value={backColor} title="Colour of a generated back" /><button onclick={generateBack}>Generate one</button></span>
                  {/snippet}
                </PictureSlot>
              </div>
              <div class="bfoot muted small">
                {backKind === 'own' ? 'Saving the set cuts the back from your picture.'
                  : backKind === 'global' ? 'Without its own, the set uses the global back (Game look → Card back).'
                  : `Without its own, the set uses the ${project.set.frameTemplate} frame's back.`}
              </div>
            </div>
          </div>

          <aside class="page-side">
            <div class="block">
              <div class="bhead"><h2>Preview</h2><span class="muted small">front · back</span></div>
              <div class="preview">
                <figure class="front-fig">
                  <div class="front">
                    {#if frontImage}<img src={projectFile(project.id, frontImage, imgBust)} alt="" />{:else}<span class="muted small">No card picture yet</span>{/if}
                  </div>
                  <figcaption class="muted small">{project.set.renderMode === 'Framed'
                    ? `Artwork only: the game puts it inside its ${project.set.frameTemplate} frame`
                    : 'The picture is the whole card'}</figcaption>
                </figure>
                <CardBackPreview image={own.file ? own.url : ''} crop={own.crop} aspect={aspects.cardBack ?? 718 / 1006}
                  fallback={own.file ? '' : project.set.cardBack ? projectFile(project.id, project.set.cardBack, imgBust + backBust) : globalBack || ''}
                  caption="The {project.set.frameTemplate} frame's own back" />
              </div>
            </div>
          </aside>
        </div>
      </div>

    {:else if page === 'rarities'}
      <div class="pane">
        <header class="pane-hero"><div class="grow"><h1>Rarities</h1><div class="muted">The ladder from most common to rarest. Each card has one; pack slots weigh them.</div></div></header>
        <RaritiesEditor {project} {notify} />
      </div>

    {:else if page === 'versions'}
      <div class="pane">
        <header class="pane-hero"><div class="grow"><h1>Card versions</h1><div class="muted">Which borders and foils this set's cards come in, unless a card has its own list.</div></div></header>
        <VariantsEditor {project} />
      </div>

    {:else if page === 'prices'}
      <div class="pane">
        <header class="pane-hero"><div class="grow"><h1>Prices</h1><div class="muted">Market price = card base × border × foil. Cards can override on their Price tab; the game's daily changes apply on top.</div></div></header>
        <div class="page-cols">
          <div class="maincol">
            <div class="block">
              <div class="bhead"><h2>Border multipliers</h2><span class="grow"></span>
                <label class="ex-in">Example card worth $<input type="number" step="0.25" min="0.01" bind:value={exampleBase} /></label></div>
              <div class="tablewrap"><table class="mults">
                <thead><tr><th>Border</th><th>Multiplier</th><th class="num">Normal</th><th class="num">Foil</th><th class="barcol"></th></tr></thead>
                <tbody>
                  {#each BORDERS as b, i}
                    <tr>
                      <td class="bname"><span class="bdot b-{b}"></span>{BORDER_LABELS[b]}</td>
                      <td><span class="times">×</span><input type="number" step="0.05" min="0" bind:value={project.set.priceDefaults.borderMultipliers[i]} /></td>
                      <td class="num">{money(example[i][1])}</td>
                      <td class="num foil">{money(example[i][2])}</td>
                      <td class="barcol"><span class="pbar"><span style="width:{(example[i][1] / maxExample) * 100}%"></span><span class="f" style="width:{((example[i][2] - example[i][1]) / maxExample) * 100}%"></span></span></td>
                    </tr>
                  {/each}
                </tbody>
              </table></div>
            </div>
            <div class="block entries">
              <div class="entry"><div class="head row"><span class="grow name">Foil multiplier</span><span class="control"><span class="times">×</span><input type="number" step="0.1" min="1" class="short" bind:value={project.set.priceDefaults.foilMultiplier} /></span></div><div class="desc muted small">Applied on top of the border for every foil version.</div></div>
              <div class="entry"><div class="head row"><span class="grow name">Minimum price</span><span class="control"><span class="times">$</span><input type="number" step="0.01" min="0" class="short" bind:value={project.set.priceDefaults.minimum} /></span></div><div class="desc muted small">No version of any card sells for less.</div></div>
            </div>
          </div>
          <aside class="page-side">
            <div class="notice info howto"><b>Where prices come from</b>
              <ul><li>Each card's base price comes from its import or from Gamify; ⋯ → Refresh prices pulls today's prices.</li>
                <li>These multipliers turn a base into every version's price.</li>
                <li>Lock a card on its Price tab to keep Gamify and refreshes off it.</li></ul></div>
          </aside>
        </div>
      </div>
    {/if}
  </div>
</div>

<style>
  .settab { display: flex; height: 100%; min-height: 0; }
  .side { width: 240px; flex: none; display: flex; flex-direction: column; min-height: 0; border-right: 1px solid var(--line); }
  .scroll { flex: 1; min-width: 0; overflow: auto; }
  @media (max-width: 1150px) {
    .side { width: 170px; }
    .side .sub { display: none; }
  }
  .maincol { display: flex; flex-direction: column; gap: 18px; min-width: 0; }

  /* Overview */
  .stats small { font-size: 14px; color: var(--muted); font-weight: 400; }
  .stack { display: flex; height: 14px; border-radius: 7px; overflow: hidden; gap: 2px; background: var(--bg); }
  .stack span { min-width: 3px; }
  .legend { display: flex; flex-wrap: wrap; gap: 8px 22px; font-size: 13px; }
  .legend span { display: inline-flex; align-items: center; gap: 7px; }
  .legend i { width: 10px; height: 10px; border-radius: 3px; }
  .legend b { font-weight: 600; font-variant-numeric: tabular-nums; }
  .legend em { font-style: normal; color: var(--muted); font-size: 12px; }
  .tiles { display: grid; grid-template-columns: repeat(auto-fill, minmax(min(260px, 100%), 1fr)); gap: 10px; }
  .tile { min-width: 0; display: flex; align-items: center; gap: 12px; text-align: left; padding: 14px 16px; border-radius: 10px; background: var(--panel); }
  .tile > span:first-child { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 3px; }
  .tile .small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .tile .go { font-size: 20px; color: var(--muted); }
  .tile:hover:not(:disabled) { border-color: var(--accent-2); }
  .tile:hover .go { color: var(--accent); }

  /* Basics */
  input.short { width: 140px; min-width: 0; }
  .idbox { max-width: 100%; overflow-wrap: anywhere; font-family: Consolas, monospace; background: var(--bg); border: 1px solid var(--line); border-radius: 6px; padding: 5px 10px; }

  /* Card look */
  .choices { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 10px; }
  .choice { position: relative; min-width: 0; display: flex; align-items: center; gap: 14px; text-align: left; padding: 14px 16px; background: var(--bg); border-radius: 10px; }
  .choice > span:nth-child(2) { display: flex; flex-direction: column; gap: 3px; }
  .choice.on { border-color: var(--accent); background: #1c2740; }
  .tick { position: absolute; top: 8px; right: 10px; width: 20px; height: 20px; border-radius: 50%; background: var(--accent); color: #fff;
    display: grid; place-items: center; font-size: 11px; font-weight: 700; }
  .glyph { width: 34px; height: 48px; flex: none; border-radius: 4px; background: linear-gradient(160deg, #5b8cff, #b66dff); }
  .glyph.framed { background: #c9b98a; padding: 4px 4px 16px; }
  .glyph.framed span { display: block; width: 100%; height: 100%; border-radius: 2px; background: linear-gradient(160deg, #5b8cff, #3ecf8e); }
  .backbody :global(.slot) { max-width: 560px; }
  .gen { display: inline-flex; align-items: center; gap: 6px; }
  .gen input[type="color"] { width: 40px; height: 30px; padding: 2px; }
  .preview { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; align-items: start; max-width: 480px; }
  .front { aspect-ratio: 63 / 88; border-radius: 12px; overflow: hidden; background: var(--bg); display: grid; place-items: center; box-shadow: 0 6px 18px #0009; }
  .front img { width: 100%; height: 100%; object-fit: cover; display: block; }
  .front-fig { margin: 0; display: flex; flex-direction: column; gap: 6px; }

  /* Prices */
  .ex-in { display: inline-flex; align-items: center; gap: 4px; font-size: 12px; color: var(--muted); }
  .ex-in input { width: 80px; }
  .tablewrap { overflow-x: auto; }
  .mults { width: 100%; border-collapse: collapse; font-size: 13px; }
  @container (max-width: 560px) {
    .barcol { display: none; }
    .mults th, .mults td { padding-right: 8px; }
    .mults input { width: 70px; }
    .mults .num { width: auto; }
  }
  .mults th { text-align: left; font-weight: 500; font-size: 11px; text-transform: uppercase; letter-spacing: 0.06em; color: var(--muted); padding: 0 12px 6px 0; }
  .mults td { padding: 6px 12px 6px 0; border-top: 1px solid var(--line); }
  .mults input { width: 90px; }
  .mults .num { text-align: right; font-variant-numeric: tabular-nums; width: 110px; }
  .mults .foil { color: #c9a8ff; }
  .barcol { width: 34%; }
  .pbar { display: flex; height: 6px; border-radius: 3px; overflow: hidden; background: var(--bg); }
  .pbar span { background: var(--accent-2); }
  .pbar span.f { background: #8f6ad8; }
  .bname { white-space: nowrap; }
  .bdot { display: inline-block; width: 10px; height: 10px; border-radius: 3px; margin-right: 8px; vertical-align: -1px; background: var(--line); }
  .bdot.b-FirstEdition { background: #8a93a3; }
  .bdot.b-Silver { background: #c9d1dc; }
  .bdot.b-Gold { background: #e2b53c; }
  .bdot.b-EX { background: #ff5e6a; }
  .bdot.b-FullArt { background: #3ecf8e; }
  .times { color: var(--muted); margin-right: 4px; }
</style>
