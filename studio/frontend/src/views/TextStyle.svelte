<script lang="ts">
  // Text style of the Game look page (shop name, door sign words): own colour or the default, size, outline and its colour.
  // `style` follows every move (live previews); `oncommit` runs when a control is released.
  import type { TextStyleValue } from '../lib/shopLook';

  let { style = $bindable(), defaultColor, defaultLabel, oncommit = () => {} }: {
    style: TextStyleValue;
    defaultColor: string; // where "Own colour" starts
    defaultLabel: string; // shown while the default colour is used, e.g. "game gold"
    oncommit?: () => void; // when a control is released (the Game look page saves with its Save button instead)
  } = $props();

  function ownColor(on: boolean) {
    style.color = on ? defaultColor : '';
    oncommit();
  }
</script>

<label><input type="checkbox" checked={!!style.color} onchange={(ev) => ownColor(ev.currentTarget.checked)} /> Own colour</label>
{#if style.color}<input type="color" bind:value={style.color} onchange={oncommit} />{:else}<span class="muted small">{defaultLabel}</span>{/if}
<label class="field">Size {Math.round(style.size * 100)}%
  <input type="range" min="0.5" max="2" step="0.05" bind:value={style.size} onchange={oncommit} />
</label>
<label class="field">Outline {style.outline > 0 ? Math.round(style.outline * 100) + '%' : 'none'}
  <input type="range" min="0" max="0.3" step="0.01" bind:value={style.outline} onchange={oncommit} />
</label>
{#if style.outline > 0}<input type="color" bind:value={style.outlineColor} onchange={oncommit} title="Outline colour" />{/if}

<style>
  input[type="color"] { width: 48px; height: 32px; padding: 2px; }
  .small { font-size: 12px; margin: 0; }
</style>
