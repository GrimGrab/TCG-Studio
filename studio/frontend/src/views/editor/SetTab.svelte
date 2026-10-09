<script lang="ts">
  import { App, projectFile, FRAME_TEMPLATES, BORDERS, errText } from '../../lib/api';
  import RaritiesEditor from './RaritiesEditor.svelte';
  import VariantsEditor from './VariantsEditor.svelte';

  let { project, notify, imgBust }: { project: any; notify: (t: string, k?: string) => void; imgBust: number } = $props();

  let backBust = $state(0);
  let backColor = $state('#3a7bd5');
  let globalBack = $state('');
  $effect(() => { App.DefaultArtColor(project.id).then((c: string) => (backColor = c)); App.GlobalCardBack().then((u: string) => (globalBack = u)); });

  async function pickBack() {
    try {
      const rel = await App.ChooseSetCardBack(project.id);
      if (rel) { project.set.cardBack = rel; backBust = Date.now(); notify('Card back set — save to keep it', 'ok'); }
    } catch (e) { notify(errText(e), 'error'); }
  }

  async function generateBack() {
    try {
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
    <div class="row">
      <div class="back">
        {#if project.set.cardBack}<img src={projectFile(project.id, project.set.cardBack, imgBust + backBust)} alt="card back" />
        {:else if globalBack}<img src={globalBack} alt="global card back" />
        {:else}<span class="muted">template's back</span>{/if}
      </div>
      <div class="col">
        <span class="muted small">{project.set.cardBack ? 'This set has its own back.' : globalBack ? 'Using the global back (Settings).' : `Using the ${project.set.frameTemplate} back.`}</span>
        <button onclick={pickBack}>Choose image…</button>
        <div class="row"><input type="color" bind:value={backColor} /><button onclick={generateBack}>Generate</button></div>
        {#if project.set.cardBack}<button onclick={() => (project.set.cardBack = undefined)}>Use {globalBack ? 'global' : "template's"} back</button>{/if}
      </div>
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
  .col { display: flex; flex-direction: column; gap: 6px; }
  input[type="color"] { width: 48px; height: 32px; padding: 2px; }
  .small { font-size: 12px; }
</style>
