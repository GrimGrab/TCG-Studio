<script lang="ts">
  // Change a card's id (card panel → click the id). What it does is said before the player confirms: the set's own
  // references follow (lib/cardId), saved games and everything outside this setup don't.
  import { untrack } from 'svelte';
  import { cardIdProblem } from '../../lib/cardId';

  let { set, card, onchange, oncancel }: {
    set: any; card: any;
    onchange: (newId: string) => void;
    oncancel: () => void;
  } = $props();

  let value = $state(untrack(() => card.id)); // starts as the current id
  const problem = $derived(value === card.id ? '' : cardIdProblem(set, card.id, value));
  const ok = $derived(value !== card.id && !problem);

  function focus(el: HTMLInputElement) { el.focus(); el.select(); }
  function key(e: KeyboardEvent) {
    if (e.key === 'Escape') oncancel();
    else if (e.key === 'Enter' && ok) onchange(value);
  }
</script>

<div class="backdrop" role="presentation">
  <div class="dialog" role="dialog" aria-label="Change card id">
    <h3>Change the id of {card.name || card.id}</h3>
    <label class="field">New id
      <input bind:value use:focus onkeydown={key} spellcheck="false" />
    </label>
    {#if problem}<p class="small err">{problem}</p>{/if}
    <div class="notice">
      <b>The game knows this card by its id.</b> Changing it:
      <ul>
        <li><b>Saved games lose this card.</b> Every copy players own, its prices and graded copies, cards on shelves and in
          decks, binder favourites and customers' wishlists stop showing. The game keeps that data aside: it comes back only if
          the id is changed back to <code>{card.id}</code>.</li>
        <li><b>Copies elsewhere aren't updated.</b> Your other setups with this set, anything you've already exported or
          shared, and copied deck lists keep the old ID.</li>
        <li><b>Its shader pattern may change.</b> If the card uses a shader with a random look per card (sparkles, noise),
          it gets a new pattern.</li>
      </ul>
      This set's own references follow: its packs' card lists, evolves-from and play effects. Nothing changes in the game
      until you save the set. Best done before anyone plays the set.
    </div>
    <div class="row end">
      <button onclick={oncancel}>Cancel</button>
      <button class="primary" disabled={!ok} onclick={() => onchange(value)}>Change id</button>
    </div>
  </div>
</div>

<style>
  .dialog { background: var(--panel); border: 1px solid var(--line); border-radius: var(--radius); padding: 18px 22px; width: min(560px, 94vw);
    display: flex; flex-direction: column; gap: 10px; }
  .dialog h3 { margin: 0; }
  .notice ul { margin: 6px 0; padding-left: 18px; display: flex; flex-direction: column; gap: 4px; }
  .err { color: var(--danger); margin: 0; }
  .end { justify-content: flex-end; }
</style>
