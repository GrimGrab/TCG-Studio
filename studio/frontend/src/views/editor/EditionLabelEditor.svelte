<script lang="ts">
  // The edition label block of the Card look page (set) and the card editor's Look page (card): show / hide, place and size
  // the label (1st Edition, Silver, Gold, EX) the mod draws on full-image cards. Drag it on the preview or use the sliders.
  // Writes set.json "editionLabel" on the set, or on the card (each field the card leaves out is the set's).
  import BorderPreview from '../BorderPreview.svelte';
  import { cardLabel, hasOwnLabel, labelText, setLabel, writeLabel, LABEL_DEFAULT, LABEL_SIZE, LABEL_X, LABEL_Y, type LabelFields } from '../../lib/editionLabel';
  import { versionLabel } from '../../lib/cardVersions';
  import { loadLooks, noLooks } from '../../lib/cardLooks';

  let { project, card, sample, sampleKey }: {
    project: any; card?: any;
    /** The version shown (one that has a label) as LookEditor draws it. */
    sample: { art: string; border: any; rim?: string; picture?: any; facts: any };
    sampleKey: string;
  } = $props();

  let looks = $state(noLooks());
  $effect(() => { loadLooks().then((l) => (looks = l)); });

  const set = $derived(project.set);
  const target = $derived(card ?? set);
  const choice = $derived(card ? cardLabel(set, card) : setLabel(set));
  const fromSet = $derived(setLabel(set));
  const own = (f: keyof LabelFields) => target.editionLabel?.[f] !== undefined;
  const ownCards = $derived(card ? 0 : set.cards.filter(hasOwnLabel).length);

  function put(f: keyof LabelFields, v: boolean | number) { writeLabel(target, f, v); }
  function moved(x: number, y: number) { writeLabel(target, 'x', x); writeLabel(target, 'y', y); }
  const num = (e: Event) => Number((e.currentTarget as HTMLInputElement).value);
  const above = (f: 'x' | 'y' | 'size') => (card ? `set: ${fromSet[f]}` : `default: ${LABEL_DEFAULT[f]}`);
</script>

<div class="block">
  <div class="bhead"><h2>{card ? "This card's edition label" : 'Edition label'}</h2>
    <span class="muted small">{card ? (hasOwnLabel(card) ? 'Own place' : 'Follows the set') : 'the 1st Edition, Silver, Gold and EX text on the cards'}</span>
    <span class="grow"></span>
    {#if card && hasOwnLabel(card)}<button class="small" onclick={() => (card.editionLabel = undefined)} title="The card uses the set's label again">Follow set</button>{/if}
    {#if !card && set.editionLabel}<button class="small" onclick={() => (set.editionLabel = undefined)} title="Shown, at the mod's default place and size">Reset</button>{/if}
  </div>
  <div class="cols">
    <div class="pv">
      {#if sample.art}
        <BorderPreview border={sample.border} rim={sample.rim} shaders={looks.shaders} art={sample.art} picture={sample.picture} width={200}
          facts={sample.facts} label={{ text: labelText(sampleKey), choice, onmove: moved }} />
        <div class="muted small">{versionLabel(sampleKey)} · drag the label to move it</div>
      {:else}
        <p class="muted small">{card ? 'This card has no picture yet.' : 'Add cards with pictures to see the label on one.'}</p>
      {/if}
    </div>
    <div class="fields">
      {#if card}
        <div class="seg" role="group" aria-label="Show the label">
          <button class:on={!own('show')} onclick={() => writeLabel(card, 'show', undefined)}>Set: {fromSet.show ? 'shown' : 'hidden'}</button>
          <button class:on={own('show') && card.editionLabel.show} onclick={() => put('show', true)}>Show</button>
          <button class:on={own('show') && !card.editionLabel.show} onclick={() => put('show', false)}>Hide</button>
        </div>
      {:else}
        <label class="check"><input type="checkbox" checked={choice.show} onchange={(e) => put('show', e.currentTarget.checked)} /> Show the label on this set's cards</label>
      {/if}
      {#each [['x', 'Sideways', '% of the card width, + = right'], ['y', 'Up / down', '% of the card height, + = up']] as [f, name, help] (f)}
        {@const k = f as 'x' | 'y'}
        {@const [lo, hi] = k === 'x' ? LABEL_X : LABEL_Y}
        <label class="field" title={help}>{name} <span class="muted small">{own(k) ? '' : `(${above(k)})`}</span>
          <span class="row">
            <input type="range" class="grow" min={lo} max={hi} step="0.1" value={choice[k]} disabled={!choice.show} oninput={(e) => put(k, num(e))} />
            <input type="number" class="numbox" min={lo} max={hi} step="0.1" value={choice[k]} disabled={!choice.show} onchange={(e) => put(k, num(e))} />
          </span>
        </label>
      {/each}
      <label class="field">Size <span class="muted small">{own('size') ? '' : `(${above('size')})`}</span>
        <span class="row">
          <input type="range" class="grow" min={LABEL_SIZE[0]} max={LABEL_SIZE[1]} step="0.05" value={choice.size} disabled={!choice.show} oninput={(e) => put('size', num(e))} />
          <input type="number" class="numbox" min={LABEL_SIZE[0]} max={LABEL_SIZE[1]} step="0.05" value={choice.size} disabled={!choice.show} onchange={(e) => put('size', num(e))} />
        </span>
      </label>
      {#if ownCards}
        <div class="row muted small"><span class="grow">{ownCards} card{ownCards === 1 ? ' has its' : 's have their'} own label place.</span>
          <button class="small" onclick={() => { for (const c of set.cards) c.editionLabel = undefined; }}>Reset them</button></div>
      {/if}
    </div>
  </div>
</div>

<style>
  .cols { display: grid; grid-template-columns: auto minmax(0, 1fr); gap: 16px; align-items: start; }
  @container (max-width: 560px) { .cols { grid-template-columns: minmax(0, 1fr); } }
  .pv { display: flex; flex-direction: column; gap: 6px; width: 220px; }
  .fields { display: flex; flex-direction: column; gap: 10px; min-width: 0; }
  .numbox { width: 76px; }
  .check { display: inline-flex; align-items: center; gap: 6px; }
</style>
