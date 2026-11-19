<script lang="ts">
  // A button that opens a panel under it (filters, view options, menus). Closes on a click outside or Esc; the panel's
  // content gets `close` for buttons that should shut it.
  import type { Snippet } from 'svelte';

  let { label, panel, title = '', cls = '', align = 'left', width = '', open = $bindable(false), disabled = false, caret = true }: {
    label: Snippet; panel: Snippet<[() => void]>; title?: string; cls?: string; align?: 'left' | 'right';
    width?: string; open?: boolean; disabled?: boolean; caret?: boolean;
  } = $props();

  let root: HTMLElement;
  const close = () => (open = false);
  function outside(e: MouseEvent) { if (open && root && !root.contains(e.target as Node)) open = false; }
  function key(e: KeyboardEvent) { if (open && e.key === 'Escape') { e.stopPropagation(); open = false; } }
</script>

<svelte:window onmousedown={outside} onkeydown={key} />

<div class="dd more" bind:this={root}>
  <button class="{cls} ddbtn" class:on={open} {title} {disabled} aria-expanded={open} onclick={() => (open = !open)}>
    {@render label()}{#if caret}<span class="caret">▾</span>{/if}</button>
  {#if open}
    <div class="menu ddpanel" class:right={align === 'right'} style={width ? `width:${width}` : ''}>{@render panel(close)}</div>
  {/if}
</div>

<style>
  .dd { position: relative; display: inline-flex; }
  .ddbtn { display: inline-flex; align-items: center; gap: 6px; white-space: nowrap; }
  .ddbtn.on { border-color: var(--accent); }
  .caret { font-size: 9px; color: var(--muted); }
  /* The look is the shared ⋯ menu's (.more .menu); here only where it opens and its spacing. */
  .ddpanel { top: calc(100% + 4px); left: 0; right: auto; z-index: 40; min-width: 220px; padding: 10px; gap: 8px; }
  .ddpanel.right { left: auto; right: 0; }
  .ddpanel :global(.ddmenu) { display: flex; flex-direction: column; gap: 2px; margin: -4px; }
  .ddpanel :global(.ddmenu > button) { text-align: left; background: none; border-color: transparent; }
  .ddpanel :global(.ddmenu > button:hover:not(:disabled)) { background: var(--panel); }
  .ddpanel :global(.ddhead) { font-size: 11px; text-transform: uppercase; letter-spacing: 0.06em; color: var(--muted); }
  .ddpanel :global(.ddsep) { border-top: 1px solid var(--line); margin: 2px -10px; }
</style>
