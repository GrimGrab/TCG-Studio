<script lang="ts">
  // One picture of the Shop look page: choose an image, drag its crop box, centre it or go back to vanilla. Every change only edits
  // `value` (the page's draft); the page's Save stores it.
  import { App, errText } from '../lib/api';
  import CropBox from './CropBox.svelte';

  let { slot, value = $bindable(), aspect, label = '', notify }: {
    slot: string; // sign | wallpaper (app_shoplook.go shopPictures)
    value: { file: string; url: string; crop: number[] | null }; // file '' = vanilla
    aspect: number;
    label?: string; // drawn in the crop box (what the game puts on top)
    notify: (t: string, k?: string) => void;
  } = $props();

  async function choose() {
    try {
      const f = await App.ImportShopPicture(slot);
      if (f.file) value = { file: f.file, url: f.url, crop: null };
    } catch (e) { notify(errText(e), 'error'); }
  }
</script>

<div class="slot">
  {#if value.file}
    <CropBox src={value.url} {aspect} crop={value.crop} {label} onchange={(c) => (value = { ...value, crop: c })} />
  {/if}
  <div class="row">
    <button onclick={choose}>Choose image…</button>
    {#if value.file}
      <button onclick={() => (value = { ...value, crop: null })} disabled={!value.crop}>Centre</button>
      <button class="danger" onclick={() => (value = { file: '', url: '', crop: null })}>Remove</button>
    {/if}
  </div>
</div>

<style>
  .slot { display: flex; flex-direction: column; gap: 8px; min-width: 0; }
</style>
