<script lang="ts">
  import { matchSearch } from '../lib/search';
  // Searchable game picker of the Import page: recently used games on top, then every game A–Z, the card database it
  // comes from shown small. Enter picks the first match, Escape or a click outside closes it.
  let { games, value, recent, disabled = false, onpick }: {
    games: any[]; value: string; recent: string[]; disabled?: boolean; onpick: (id: string) => void;
  } = $props();

  let open = $state(false);
  let q = $state('');
  let root: HTMLDivElement;
  let input = $state<HTMLInputElement>();

  const current = $derived(games.find((g) => g.id === value));
  const sorted = $derived([...games].sort((a, b) => a.game.localeCompare(b.game)));
  const matches = $derived(sorted.filter((g) => matchSearch(q, g.game)));
  const recentGames = $derived(q.trim() ? [] : recent.map((id) => games.find((g) => g.id === id)).filter(Boolean));

  function toggle() {
    open = !open;
    q = '';
    if (open) queueMicrotask(() => input?.focus());
  }
  function pick(id: string) {
    open = false;
    onpick(id);
  }
  function key(e: KeyboardEvent) {
    if (e.key === 'Escape') open = false;
    else if (e.key === 'Enter' && matches.length) pick(matches[0].id);
  }
  function outside(e: MouseEvent) {
    if (open && root && !root.contains(e.target as Node)) open = false;
  }
</script>

<svelte:window onmousedown={outside} />

<div class="picker" bind:this={root}>
  <button class="current" {disabled} onclick={toggle} aria-expanded={open}>
    <b>{current?.game ?? 'Choose a game'}</b>
    {#if current}<span class="muted small">{current.name}</span>{/if}
    <span class="caret">▾</span>
  </button>
  {#if open}
    <div class="menu">
      <input bind:this={input} placeholder="Search {games.length} games…" bind:value={q} onkeydown={key} />
      <div class="list">
        {#if recentGames.length}
          <div class="head">Recent</div>
          {#each recentGames as g (g.id)}
            <button class="item" class:active={g.id === value} onclick={() => pick(g.id)}><span>{g.game}</span><span class="muted small">{g.name}</span></button>
          {/each}
          <div class="head">All games</div>
        {/if}
        {#each matches as g (g.id)}
          <button class="item" class:active={g.id === value} onclick={() => pick(g.id)}><span>{g.game}</span><span class="muted small">{g.name}</span></button>
        {:else}
          <p class="muted small none">No game matches.</p>
        {/each}
      </div>
    </div>
  {/if}
</div>

<style>
  .picker { position: relative; }
  .current { display: flex; align-items: baseline; gap: 8px; min-width: 320px; padding: 8px 12px; text-align: left; }
  .current b { flex: 1; }
  .caret { opacity: 0.7; }
  .menu { position: absolute; top: calc(100% + 4px); left: 0; z-index: 50; width: 380px; background: var(--panel); border: 1px solid var(--line);
    border-radius: var(--radius); padding: 8px; display: flex; flex-direction: column; gap: 6px; box-shadow: 0 8px 24px rgba(0, 0, 0, 0.4); }
  .list { max-height: 420px; overflow: auto; display: flex; flex-direction: column; }
  .head { font-size: 11px; text-transform: uppercase; letter-spacing: 0.05em; opacity: 0.6; padding: 6px 8px 2px; }
  .item { display: flex; justify-content: space-between; gap: 8px; border: none; background: none; text-align: left; padding: 5px 8px; border-radius: 4px; }
  .item:hover { background: var(--bg); }
  .item.active { color: var(--accent); }
  .none { padding: 6px 8px; margin: 0; }
</style>
