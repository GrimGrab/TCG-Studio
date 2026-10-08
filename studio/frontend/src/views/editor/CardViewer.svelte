<script lang="ts">
  // Full-screen view of card images: the shown cards in order, ← / → (or the side arrows) to step, Esc or a click outside to close.
  import { projectFile, rarityName, money } from '../../lib/api';

  let { project, cards, index, imgBust, onindex, onclose }: {
    project: any; cards: any[]; index: number; imgBust: number; onindex: (i: number) => void; onclose: () => void;
  } = $props();

  const card = $derived(cards[index]);
  const step = (d: number) => { if (cards.length) onindex((index + d + cards.length) % cards.length); };

  function key(e: KeyboardEvent) {
    if (e.key === 'Escape') onclose();
    else if (e.key === 'ArrowRight') step(1);
    else if (e.key === 'ArrowLeft') step(-1);
    else return;
    e.preventDefault();
    e.stopPropagation();
  }
</script>

<svelte:window onkeydown={key} />

{#if card}
  <div class="viewer" role="dialog" aria-modal="true" tabindex="-1" onclick={onclose} onkeydown={() => {}}>
    {#if card.image}
      <img src={projectFile(project.id, card.image, imgBust)} alt={card.name} />
    {:else}
      <div class="none">No image</div>
    {/if}
    <div class="cap">
      <b>{card.name}</b> · {rarityName(project.set, card.rarity)}{card.number ? ` · #${card.number}` : ''} · {money(card.price.base)}
      <span class="muted">{index + 1} / {cards.length} · ← → to browse · Esc to close</span>
    </div>
    {#if cards.length > 1}
      <button class="nav prev" title="Previous (←)" onclick={(e) => { e.stopPropagation(); step(-1); }}>‹</button>
      <button class="nav next" title="Next (→)" onclick={(e) => { e.stopPropagation(); step(1); }}>›</button>
    {/if}
    <button class="close" title="Close (Esc)" onclick={onclose}>✕</button>
  </div>
{/if}

<style>
  .viewer { position: fixed; inset: 0; z-index: 1000; background: rgba(0, 0, 0, 0.88); display: flex; flex-direction: column;
    align-items: center; justify-content: center; gap: 10px; padding: 24px 70px 16px; }
  img { max-width: 100%; max-height: calc(100vh - 90px); object-fit: contain; border-radius: 12px; box-shadow: 0 8px 40px rgba(0, 0, 0, 0.6); }
  .none { color: #aaa; }
  .cap { color: #ddd; font-size: 14px; display: flex; gap: 12px; align-items: baseline; }
  .muted { color: #888; font-size: 12px; }
  .nav { position: absolute; top: 50%; transform: translateY(-50%); font-size: 40px; line-height: 1; padding: 4px 14px;
    background: rgba(255, 255, 255, 0.08); border: none; color: #eee; border-radius: 8px; }
  .nav:hover { background: rgba(255, 255, 255, 0.18); }
  .prev { left: 14px; }
  .next { right: 14px; }
  .close { position: absolute; top: 12px; right: 14px; background: rgba(255, 255, 255, 0.08); border: none; color: #eee; }
</style>
