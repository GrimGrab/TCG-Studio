<script lang="ts">
  // The chips for a person's traits or perks: on / off, and greyed out when an opposite one is on.
  type Item = { id: string; name: string; opposite?: string };
  let { items, picked, titleOf, onchange }: { items: Item[]; picked: string[]; titleOf: (i: Item) => string; onchange: (next: string[]) => void } = $props();

  function blockedBy(t: Item): string | null {
    if (picked.includes(t.id)) return null;
    for (const id of picked) {
      const other = items.find((x) => x.id === id);
      if (other && (other.opposite === t.id || t.opposite === id)) return other.name;
    }
    return null;
  }
</script>

<div class="chips">
  {#each items as t}
    {@const block = blockedBy(t)}
    <button class="chip" class:on={picked.includes(t.id)} disabled={!!block} title={block ? `Can't be both ${t.name} and ${block}` : titleOf(t)}
      onclick={() => onchange(picked.includes(t.id) ? picked.filter((x) => x !== t.id) : [...picked, t.id])}>{t.name}</button>
  {/each}
</div>

<style>
  .chips { display: flex; flex-wrap: wrap; gap: 6px; }
  .chip { border-radius: 14px; padding: 4px 12px; font-size: 13px; }
  .chip.on { border-color: var(--accent); background: #22304d; }
</style>
