<script lang="ts">
  // The set's card shaders (set.json "versionShaders"): which shader draws each version's picture, for every card that
  // doesn't choose its own (card detail, or several cards in the Cards tab). Shaders are written on the Shaders page.
  import { onMount } from 'svelte';
  import { App } from '../../lib/api';
  import ShaderGrid, { FOLLOW } from './ShaderGrid.svelte';
  import { cardsWithOwnShaders, inheritLabel, setShader, setVersions, writeSetShader } from '../../lib/cardVersions';

  let { project }: { project: any } = $props();

  let shaders = $state<{ id: string; name: string; builtIn?: boolean }[]>([]);
  onMount(async () => { try { shaders = (await App.ListShaders()) ?? []; } catch { shaders = []; } });

  const on = $derived(setVersions(project.set));
  const own = $derived(cardsWithOwnShaders(project.set));
  const framed = $derived(project.set.renderMode !== 'FullImage');

  function resetCards() {
    for (const c of project.set.cards) c.versionShaders = undefined;
  }
</script>

<section class="block">
  <div class="bhead"><h2>Card shaders</h2><span class="muted small">default for all cards</span></div>
  <p class="muted small">A shader draws a version's picture with an effect (holo, light sweep, anything written on the <b>Shaders</b> page).
    Choose one per version here for the whole set; a card can choose its own.
    {#if framed}On this framed set the shader draws the artwork inside the game's frame; left on Default, the game's own foil stays.
    {:else}Left on Default, foil versions get the stock holo of their border (Mod settings → Foil) and normal versions none.{/if}</p>
  <ShaderGrid value={(k) => setShader(project.set, k) ?? FOLLOW} {shaders} dim={(k) => !on.has(k)}
    follow={(k) => inheritLabel(project.set, k, undefined, () => '')}
    onchange={(k, v) => writeSetShader(project.set, k, v === FOLLOW ? undefined : v)} />
  {#if own}
    <div class="foot muted small">
      <span>{own} card{own === 1 ? ' has' : 's have'} their own shader choices</span>
      <button class="small" onclick={resetCards} title="Those cards follow this list again">Reset all to this default</button>
    </div>
  {/if}
</section>

<style>
  .foot { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; margin-top: 8px; }
</style>
