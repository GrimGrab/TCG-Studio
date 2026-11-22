<script lang="ts">
  // The card in the play-table game: lane attack, element, evolution and its effect (the game's own PlayEffectData JSON).
  // Shared by the card panel and the card editor (Play page).
  import { ELEMENTS, LANES, errText } from '../../lib/api';

  let { project, card }: { project: any; card: any } = $props();

  let effectText = $state('');
  let effectError = $state('');
  // Reset the effect editor when a different card is shown.
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

  const others = $derived(project.set.cards.filter((c: any) => c.id !== card.id));
  const LANE_COLORS: Record<string, string> = { Fire: '#ff7a45', Earth: '#c9a227', Water: '#4fa3ff', Wind: '#3ecf8e' };
</script>

<div class="fields">
  <div class="lanes">
    {#each LANES as l, i}<label class="field lane" style="--lc:{LANE_COLORS[l] ?? 'var(--line)'}">{l}<input type="number" min="0" bind:value={card.play.laneAttack[i]} /></label>{/each}
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
  <label class="field">Effect (raw PlayEffectData JSON, the game's own format; empty = no effect)
    <textarea rows="6" spellcheck="false" class="mono" bind:value={effectText} onblur={setEffect}
      placeholder={'{ "playEffectQueueDataList": [ { "playEffectType": "DrawCard", "playEffectTypeSecondary": "None", "countList": [1] } ] }'}></textarea>
  </label>
  {#if effectError}<div class="err small">{effectError}</div>{/if}
</div>

<style>
  .fields { display: flex; flex-direction: column; gap: 10px; }
  .lanes { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 6px; }
  .lane { border-top: 3px solid var(--lc); padding-top: 4px; border-radius: 2px; }
  .row > .field { flex: 1 1 0; min-width: 0; }
  .field { min-width: 0; }
  .field > input, .field > select, .field > textarea { width: 100%; }
  .mono { font-family: Consolas, monospace; font-size: 12px; }
</style>
