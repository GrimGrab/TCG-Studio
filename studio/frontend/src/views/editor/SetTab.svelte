<script lang="ts">
  import { App, projectFile, FRAME_TEMPLATES, BORDERS, errText } from '../../lib/api';
  import { pickSetPicture, putSetPicture, setPicture } from '../../lib/setPictures';
  import PictureSlot from '../PictureSlot.svelte';
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
</script>

<div class="form">
  <section>
    <div class="row">
      <label class="field grow">Set name<input bind:value={project.set.name} /></label>
      <label class="field" style="width:200px">Id (stable)<input value={project.set.id} readonly /></label>
      <label class="field" style="width:110px" title="Progression order used by Gamify">Tier<input type="number" min="0" bind:value={project.meta.tier} /></label>
    </div>
    <div class="row">
      <label class="field grow" title="The game or series this set belongs to. The in-game binder groups sets by it, so you can show e.g. every Gundam set at once.">Collection (binder group)
        <input list="tcg-collections" placeholder="e.g. Magic: The Gathering, Gundam" bind:value={project.set.collection} />
        <datalist id="tcg-collections">{#each collections as c}<option value={c}></option>{/each}</datalist>
      </label>
      <label class="field" style="width:170px" title="Orders sets inside their collection in the binder (oldest first).">Release date
        <input type="date" bind:value={project.set.released} />
      </label>
    </div>
    <div class="row">
      <span class="muted small grow">This setup's own copy: names, prices and tiers you change here don't affect your other setups.</span>
      <button class="small" onclick={saveToCatalog} title="Make this setup's saved version the one other setups get when they add it from the catalog (setups that already have it keep theirs).">Update catalog</button>
    </div>
    <div class="row">
      <label class="field grow">Card style
        <select bind:value={project.set.renderMode}>
          <option value="FullImage">Full image — the picture is the whole card (MTG scans)</option>
          <option value="Framed">Framed — the picture is artwork inside the game's frame</option>
        </select>
      </label>
      <label class="field grow">Frame template (borders, foil, card back)
        <select bind:value={project.set.frameTemplate}>{#each FRAME_TEMPLATES as f}<option>{f}</option>{/each}</select>
      </label>
    </div>
  </section>

  <RaritiesEditor {project} {notify} />

  <VariantsEditor {project} />

  <section>
    <h3>Card back</h3>
    <span class="muted small">{own.file || project.set.cardBack ? 'This set has its own back (saving the set cuts it from your picture).'
      : globalBack ? 'Using the global back (Game look → Card back).' : `Using the ${project.set.frameTemplate} back.`}</span>
    <div class="row back-row">
      {#if !own.file && !project.set.cardBack}
        <div class="back">{#if globalBack}<img src={globalBack} alt="global card back" />{:else}<span class="muted">template's back</span>{/if}</div>
      {/if}
      <PictureSlot aspect={aspects.cardBack ?? 718 / 1006} {notify} pick={() => pickSetPicture(project, 'cardBack')}
        bind:value={() => own, (v) => putSetPicture(project, 'cardBack', v)}
        fallback={project.set.cardBack ? projectFile(project.id, project.set.cardBack, imgBust + backBust) : ''}
        removeLabel="Use {globalBack ? 'global' : "template's"} back">
        {#snippet extra()}
          <span class="row"><input type="color" bind:value={backColor} /><button onclick={generateBack}>Generate</button></span>
        {/snippet}
      </PictureSlot>
    </div>
  </section>

  <section>
    <h3>Default price multipliers</h3>
    <p class="muted small">Market price = card base × border multiplier × foil multiplier (cards can override). The game's daily price changes apply on top.</p>
    <div class="grid">
      {#each BORDERS as b, i}<label class="field">{b}<input type="number" step="0.05" bind:value={project.set.priceDefaults.borderMultipliers[i]} /></label>{/each}
      <label class="field">Foil ×<input type="number" step="0.1" bind:value={project.set.priceDefaults.foilMultiplier} /></label>
      <label class="field">Minimum price<input type="number" step="0.01" bind:value={project.set.priceDefaults.minimum} /></label>
    </div>
  </section>
</div>

<style>
  .form { overflow: auto; height: 100%; padding: 14px 18px; display: flex; flex-direction: column; gap: 14px; }
  section { background: var(--panel); border: 1px solid var(--line); border-radius: var(--radius); padding: 12px; display: flex; flex-direction: column; gap: 10px; max-width: 1000px; }
  .grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: 8px; }
  .back { width: 120px; height: 168px; background: var(--bg); border-radius: 6px; display: flex; align-items: center; justify-content: center; overflow: hidden; }
  .back img { max-width: 100%; max-height: 100%; }
  .back-row { align-items: flex-start; gap: 14px; }
  input[type="color"] { width: 48px; height: 32px; padding: 2px; }
  .small { font-size: 12px; }
</style>
