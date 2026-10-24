<script lang="ts">
  import { App, projectFile, FRAME_TEMPLATES, BORDERS, BORDER_LABELS, errText, money, setRarities } from '../../lib/api';
  import { pickSetPicture, putSetPicture, setPicture } from '../../lib/setPictures';
  import PictureSlot from '../PictureSlot.svelte';
  import CardBackPreview from '../CardBackPreview.svelte';
  import RaritiesEditor from './RaritiesEditor.svelte';
  import VariantsEditor from './VariantsEditor.svelte';

  let { project, notify, imgBust }: { project: any; notify: (t: string, k?: string) => void; imgBust: number } = $props();

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
  async function saveToCatalog() {
    try { await App.SaveToCatalog('set', [project.set.id]); notify(`The catalog now has this setup's saved version of "${project.set.name}".`, 'ok'); }
    catch (e) { notify(errText(e), 'error'); }
  }

  // Overview numbers.
  const value = $derived(project.set.cards.reduce((s: number, c: any) => s + (c.price?.base ?? 0), 0));
  const versions = $derived(project.set.variants?.length ?? 12);
  /** What a card with a $1.00 base sells for in each version, with the set's defaults. */
  const example = $derived(BORDERS.map((b: string, i: number) => {
    const d = project.set.priceDefaults;
    return [b, Math.max(d.minimum, d.borderMultipliers[i]), Math.max(d.minimum, d.borderMultipliers[i] * d.foilMultiplier)] as [string, number, number];
  }));
</script>

<div class="pane form">
  <div class="stats">
    <div class="stat"><span class="label">Cards</span><b>{project.set.cards.length.toLocaleString()}</b><span class="sub">{setRarities(project.set).length} rarities</span></div>
    <div class="stat"><span class="label">Packs</span><b>{project.set.packs.length}</b><span class="sub">{project.set.packs.filter((p: any) => p.starter).length} starter</span></div>
    <div class="stat"><span class="label">Card versions</span><b>{versions}<small> / 12</small></b><span class="sub">border × foil</span></div>
    <div class="stat"><span class="label">Base value</span><b>{money(value)}</b><span class="sub">one of every card, base version</span></div>
  </div>

  <section class="block">
    <div class="bhead"><h2>Basics</h2><span class="grow"></span>
      <button class="small" onclick={saveToCatalog} title="Make this setup's saved version the one other setups get when they add it from the catalog (setups that already have it keep theirs).">Update catalog</button>
    </div>
    <div class="grid basics">
      <label class="field span2">Set name<input bind:value={project.set.name} /></label>
      <label class="field">Id (stable)<input class="mono" value={project.set.id} readonly title="Ids are save keys and can't be changed" /></label>
      <label class="field" title="Progression order used by Gamify">Gamify tier<input type="number" min="0" bind:value={project.meta.tier} /></label>
      <label class="field span2" title="The game or series this set belongs to. The in-game binder groups sets by it, so you can show e.g. every Gundam set at once.">Collection (binder group)
        <input list="tcg-collections" placeholder="e.g. Magic: The Gathering, Gundam" bind:value={project.set.collection} />
        <datalist id="tcg-collections">{#each collections as c}<option value={c}></option>{/each}</datalist>
      </label>
      <label class="field span2" title="Orders sets inside their collection in the binder (oldest first).">Release date
        <input type="date" bind:value={project.set.released} />
      </label>
    </div>
    <span class="muted small">This setup's own copy: names, prices and tiers you change here don't affect your other setups.</span>
  </section>

  <section class="block">
    <div class="bhead"><h2>Card look</h2><span class="muted small">How the game draws this set's cards.</span></div>
    <div class="choices">
      <button class="choice" class:on={project.set.renderMode === 'FullImage'} onclick={() => (project.set.renderMode = 'FullImage')}>
        <span class="glyph full"></span>
        <span><b>Full image</b><span class="muted small">The picture is the whole card — scans of real cards.</span></span>
      </button>
      <button class="choice" class:on={project.set.renderMode === 'Framed'} onclick={() => (project.set.renderMode = 'Framed')}>
        <span class="glyph framed"><span></span></span>
        <span><b>Framed</b><span class="muted small">The picture is artwork inside the game's own card frame.</span></span>
      </button>
    </div>
    <label class="field frame">Frame template (borders, foil, card back)
      <select bind:value={project.set.frameTemplate}>{#each FRAME_TEMPLATES as f}<option>{f}</option>{/each}</select>
    </label>
  </section>

  <div class="cols">
    <RaritiesEditor {project} {notify} />
    <VariantsEditor {project} />
  </div>

  <section class="block">
    <div class="bhead"><h2>Card back</h2>
      <span class="muted small">{own.file || project.set.cardBack ? 'This set has its own back (saving the set cuts it from your picture).'
        : globalBack ? 'Using the global back (Game look → Card back).' : `Using the ${project.set.frameTemplate} back.`}</span></div>
    <!-- Like the Game look pictures: the picture and its crop box beside a preview of the back as the game shows it. -->
    <div class="back-pair">
      <div class="back-slot">
        <PictureSlot aspect={aspects.cardBack ?? 718 / 1006} {notify} pick={() => pickSetPicture(project, 'cardBack')}
          bind:value={() => own, (v) => putSetPicture(project, 'cardBack', v)}
          fallback={project.set.cardBack ? projectFile(project.id, project.set.cardBack, imgBust + backBust) : ''}
          removeLabel="Use {globalBack ? 'global' : "template's"} back">
          {#snippet extra()}
            <span class="row"><input type="color" bind:value={backColor} title="Colour of a generated back" /><button onclick={generateBack}>Generate</button></span>
          {/snippet}
        </PictureSlot>
      </div>
      <CardBackPreview image={own.file ? own.url : ''} crop={own.crop} aspect={aspects.cardBack ?? 718 / 1006}
        fallback={own.file ? '' : project.set.cardBack ? projectFile(project.id, project.set.cardBack, imgBust + backBust) : globalBack || ''}
        caption="The {project.set.frameTemplate} frame's own back" />
    </div>
  </section>

  <section class="block">
    <div class="bhead"><h2>Default prices</h2>
      <span class="muted small">Market price = card base × border × foil (cards can override). The game's daily price changes apply on top.</span></div>
    <div class="prices">
      <table class="mults">
        <thead><tr><th>Border</th><th>Multiplier</th><th class="ex" title="What a card with a $1.00 base sells for">$1 card</th><th class="ex">Foil</th></tr></thead>
        <tbody>
          {#each BORDERS as b, i}
            <tr>
              <td>{BORDER_LABELS[b]}</td>
              <td><input type="number" step="0.05" bind:value={project.set.priceDefaults.borderMultipliers[i]} /></td>
              <td class="ex">{money(example[i][1])}</td><td class="ex">{money(example[i][2])}</td>
            </tr>
          {/each}
        </tbody>
      </table>
      <div class="side">
        <label class="field">Foil ×<input type="number" step="0.1" bind:value={project.set.priceDefaults.foilMultiplier} /></label>
        <label class="field">Minimum price<input type="number" step="0.01" bind:value={project.set.priceDefaults.minimum} /></label>
      </div>
    </div>
  </section>
</div>

<style>
  .form { overflow: auto; height: 100%; max-width: 1100px; }
  .stats small { font-size: 14px; color: var(--muted); font-weight: 400; }
  .grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 10px; }
  .span2 { grid-column: span 2; }
  .mono { font-family: Consolas, monospace; }
  .choices { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; }
  .choice { display: flex; align-items: center; gap: 12px; text-align: left; padding: 10px 12px; background: var(--bg); border-radius: 10px; }
  .choice > span:last-child { display: flex; flex-direction: column; gap: 2px; }
  .choice.on { border-color: var(--accent); background: #1c2740; }
  .glyph { width: 30px; height: 42px; flex: none; border-radius: 4px; background: linear-gradient(160deg, #5b8cff, #b66dff); }
  .glyph.framed { background: #c9b98a; padding: 4px 4px 14px; }
  .glyph.framed span { display: block; width: 100%; height: 100%; border-radius: 2px; background: linear-gradient(160deg, #5b8cff, #3ecf8e); }
  .frame { max-width: 360px; }
  .cols { display: grid; grid-template-columns: repeat(auto-fit, minmax(380px, 1fr)); gap: 16px; align-items: start; }
  .back-pair { display: flex; flex-wrap: wrap; gap: 16px; align-items: flex-start; justify-content: space-between; }
  .back-slot { flex: 1 1 320px; min-width: 0; display: flex; flex-direction: column; gap: 8px; }
  input[type="color"] { width: 48px; height: 32px; padding: 2px; }
  .prices { display: flex; flex-wrap: wrap; gap: 24px; align-items: flex-start; }
  .mults { border-collapse: collapse; font-size: 13px; }
  .mults th { text-align: left; font-weight: 500; font-size: 12px; color: var(--muted); padding: 2px 12px 6px 0; }
  .mults td { padding: 3px 12px 3px 0; }
  .mults input { width: 90px; }
  .mults .ex { color: var(--muted); text-align: right; min-width: 64px; }
  .side { display: flex; flex-direction: column; gap: 10px; width: 180px; }
</style>
