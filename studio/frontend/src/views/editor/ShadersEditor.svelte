<script lang="ts">
  // The set's card shaders (set.json "versionShaders"): which shader draws each version's picture, for every card that
  // doesn't choose its own (card detail, or several cards in the Cards tab). Shaders are written on the Shaders page.
  // Beside the grid: the clicked version running live on one of the set's cards, as the game would draw it.
  import { onMount } from 'svelte';
  import { App, projectFile } from '../../lib/api';
  import ShaderGrid, { FOLLOW } from './ShaderGrid.svelte';
  import ShaderPreview from '../ShaderPreview.svelte';
  import { ALL_VERSIONS, cardsWithOwnShaders, inheritLabel, setShader, setVersions, shownShader, versionLabel, writeSetShader } from '../../lib/cardVersions';

  let { project, imgBust = 0 }: { project: any; imgBust?: number } = $props();

  type ShaderInfo = { id: string; name: string; code: string; params?: any[]; builtIn?: boolean; draws?: string; mix?: string; mixStrength?: number };
  let shaders = $state<ShaderInfo[]>([]);
  onMount(async () => { try { shaders = ((await App.ListShaders()) ?? []) as unknown as ShaderInfo[]; } catch { shaders = []; } });

  const on = $derived(setVersions(project.set));
  const own = $derived(cardsWithOwnShaders(project.set));
  const framed = $derived(project.set.renderMode !== 'FullImage');

  function resetCards() {
    for (const c of project.set.cards) c.versionShaders = undefined;
  }

  // The previewed version: the one clicked last; at first the first version on that has a shader.
  let sel = $state('');
  const shown = $derived(sel || ALL_VERSIONS.find((k) => on.has(k) && shownShader(project.set, {}, k)) || ALL_VERSIONS.find((k) => on.has(k)) || 'Base');
  const shaderId = $derived(shownShader(project.set, {}, shown)); // what a card without its own choice gets
  const shader = $derived(shaders.find((s) => s.id === shaderId));
  const source = $derived(setShader(project.set, shown) !== undefined ? 'chosen' : shaderId ? 'stock' : 'none');
  // Without a shader the preview still draws the plain picture, so the card tilts with the mouse the same way.
  const PLAIN = { code: 'vec4 card(vec2 uv) { return art(uv); }', params: [] as any[], draws: undefined, mix: undefined, mixStrength: undefined };
  const drawn = $derived(shader ?? PLAIN);

  // The example card: any of the set's cards with a picture.
  const withArt = $derived(project.set.cards.filter((c: any) => c.image));
  let ex = $state(0);
  const card = $derived(withArt.length ? withArt[((ex % withArt.length) + withArt.length) % withArt.length] : null);
  const facts = $derived({ border: Math.max(0, ['Base', 'FirstEdition', 'Silver', 'Gold', 'EX', 'FullArt'].indexOf(shown.replace('_foil', ''))),
    foil: shown.endsWith('_foil'), rarity: 0, grade: 0 });
</script>

<div class="block">
  <div class="bhead"><h2>Shaders</h2><span class="muted small">effect drawn on each version's picture · click a version to see it</span></div>
  <div class="split">
    <div class="gridcol">
      <ShaderGrid value={(k) => setShader(project.set, k) ?? FOLLOW} {shaders} dim={(k) => !on.has(k)} selected={shown}
        follow={(k) => inheritLabel(project.set, k, undefined, () => '')} onfocus={(k) => (sel = k)}
        onchange={(k, v) => writeSetShader(project.set, k, v === FOLLOW ? undefined : v)} />
    </div>
    <div class="example">
      {#if card}
        <ShaderPreview code={drawn.code} params={drawn.params ?? []} art={projectFile(project.id, card.image, imgBust)} width={200} controls={false}
          look={{ draws: drawn.draws, mix: drawn.mix, mixStrength: drawn.mixStrength }}
          mask={shader && card.shaderMask ? projectFile(project.id, card.shaderMask, imgBust) : ''} {facts} />
        <div class="cap">
          <b>{versionLabel(shown)}</b>
          <span class="muted small">{shader ? shader.name : 'No shader'}{source === 'stock' ? ' (stock holo)' : source === 'chosen' ? '' : framed && shown.endsWith('_foil') ? " (the game's own foil)" : ''}</span>
          {#if !on.has(shown)}<span class="warn small">This version is off for the set</span>{/if}
        </div>
        <div class="cardnav">
          <button class="small" onclick={() => ex--} disabled={withArt.length < 2} title="Another card">‹</button>
          <span class="muted small" title={card.name}>{card.name}</span>
          <button class="small" onclick={() => ex++} disabled={withArt.length < 2} title="Another card">›</button>
        </div>
      {:else}
        <p class="muted small">Add cards with pictures to see the shaders on one.</p>
      {/if}
    </div>
  </div>
  <div class="bfoot muted small">
    <span class="grow">{#if framed}Framed set: the shader draws the artwork inside the game's frame; on Default the game's own foil stays.
      {:else}On Default, foil versions get their border's stock holo and normal versions none. Write your own on the Shaders page.{/if}</span>
    {#if own}
      <span>{own} card{own === 1 ? ' has' : 's have'} own choices</span>
      <button class="small" onclick={resetCards} title="Those cards follow these choices again">Make every card follow this</button>
    {/if}
  </div>
</div>

<style>
  .split { display: grid; grid-template-columns: minmax(0, 1fr) 232px; gap: 20px; align-items: start; }
  .gridcol { min-width: 0; }
  .gridcol :global(.sgrid) { max-width: 760px; }
  .example { display: flex; flex-direction: column; align-items: center; gap: 8px; padding: 12px; border-radius: 10px;
    background: radial-gradient(ellipse at 50% 35%, #2b3040, #15171d 80%); border: 1px solid var(--line); }
  .cap { display: flex; flex-direction: column; align-items: center; gap: 1px; text-align: center; }
  .cardnav { display: flex; align-items: center; gap: 6px; width: 100%; }
  .cardnav span { flex: 1; min-width: 0; text-align: center; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .warn { color: var(--warn); }
  @container (max-width: 640px) { .split { grid-template-columns: minmax(0, 1fr); } .example { justify-self: center; width: 232px; } }
</style>
