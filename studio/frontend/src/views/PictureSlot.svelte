<script lang="ts">
  // One picture cut with the crop box: choose an image, drag its crop box, centre it or remove it. Used by every picture Studio
  // cuts (Game look pictures, the global and per-set card back, door sign backgrounds, pack art). Every change only edits
  // `value` (the page's draft); the page's Save stores it.
  import type { Snippet } from 'svelte';
  import { errText } from '../lib/api';
  import type { Picture } from '../lib/shopLook';
  import CropBox from './CropBox.svelte';

  let { value = $bindable(), aspect, pick, label = '', fallback = '', removeLabel = 'Remove', chooseLabel = 'Choose image…', compact = false,
    notify, extra }: {
    value: Picture;
    aspect: number; // width ÷ height of the crop box
    pick: () => Promise<{ file: string; url: string; crop?: number[] | null }>; // copies a chosen file (file '' = cancelled)
    label?: string; // drawn in the crop box (what the game puts on top)
    fallback?: string; // image shown without a picture (e.g. a back made before its picture was kept)
    removeLabel?: string;
    chooseLabel?: string;
    compact?: boolean; // small buttons under a small box (pack art grid)
    notify: (t: string, k?: string) => void;
    extra?: Snippet; // more buttons of the page (Generate, Use a colour)
  } = $props();

  async function choose() {
    try {
      const f = await pick();
      if (f.file) value = { file: f.file, url: f.url, crop: f.crop ?? null };
    } catch (e) { notify(errText(e), 'error'); }
  }
</script>

<div class="slot" class:compact>
  {#if value.file}
    <CropBox src={value.url} {aspect} crop={value.crop} {label} onchange={(c) => (value = { ...value, crop: c })} />
  {:else if fallback}
    <div class="fallback"><img src={fallback} alt="" /></div>
  {/if}
  <div class="row">
    <button class:small={compact} onclick={choose}>{chooseLabel}</button>
    {#if value.file}
      <button class:small={compact} onclick={() => (value = { ...value, crop: null })} disabled={!value.crop}>Centre</button>
    {/if}
    {#if value.file || fallback}
      <button class="danger" class:small={compact} onclick={() => (value = { file: '', url: '', crop: null })}>{removeLabel}</button>
    {/if}
    {@render extra?.()}
  </div>
</div>

<style>
  .slot { display: flex; flex-direction: column; gap: 8px; min-width: 0; }
  .row { flex-wrap: wrap; }
  .fallback { width: 130px; max-height: 180px; background: var(--bg); border-radius: 6px; display: flex; align-items: center;
    justify-content: center; overflow: hidden; }
  .fallback img { max-width: 100%; max-height: 180px; }
  .compact .fallback { width: 100%; }
</style>
