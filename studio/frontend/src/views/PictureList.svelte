<script lang="ts">
  // Several pictures cut with the crop box (the loading screens): a strip of thumbnails to add, pick and remove pictures, and
  // the picked one's crop box. Every change only edits `items` (the page's draft).
  import { errText } from '../lib/api';
  import { cropFor } from '../lib/shopLook';
  import type { Picture } from '../lib/shopLook';
  import PictureSlot from './PictureSlot.svelte';

  let { items = $bindable(), selected = $bindable(0), aspect, pick, notify, empty = '' }: {
    items: Picture[];
    empty?: string; // shown while there are no pictures (what the game shows then)
    selected?: number; // the picture whose crop box is shown (the preview follows it)
    aspect: number;
    pick: () => Promise<{ file: string; url: string; crop?: number[] | null }>;
    notify: (t: string, k?: string) => void;
  } = $props();

  async function add() {
    try {
      const f = await pick();
      if (!f.file) return;
      items = [...items, { file: f.file, url: f.url, crop: f.crop ?? null }];
      selected = items.length - 1;
    } catch (e) { notify(errText(e), 'error'); }
  }

  // The slot below edits the picked picture; "Remove" there (file '') takes it out of the list.
  function edited(i: number, v: Picture) {
    if (!v.file) {
      items = items.filter((_, j) => j !== i);
      selected = Math.min(selected, Math.max(0, items.length - 1));
    } else items = items.map((it, j) => (j === i ? v : it));
  }

  /** CSS for a thumbnail showing the crop: the image scaled so the crop fills the box. */
  function thumbStyle(it: Picture, img: HTMLImageElement | undefined) {
    if (!img?.naturalWidth) return '';
    const [x, y, w, h] = cropFor(img.naturalWidth, img.naturalHeight, it.crop, aspect);
    return `background-image:url("${it.url}");background-size:${100 / w}% ${100 / h}%;` +
      `background-position:${w < 1 ? (x / (1 - w)) * 100 : 0}% ${h < 1 ? (y / (1 - h)) * 100 : 0}%`;
  }
  let loaded = $state<Record<string, HTMLImageElement>>({});
  $effect(() => {
    for (const it of items) {
      if (loaded[it.url]) continue;
      const img = new Image();
      img.onload = () => (loaded = { ...loaded, [it.url]: img });
      img.src = it.url;
    }
  });
</script>

<div class="list">
  <div class="strip">
    {#each items as it, i (it.file + i)}
      <button class="thumb" class:on={i === selected} style:aspect-ratio={aspect} style={thumbStyle(it, loaded[it.url])}
        title={it.file} onclick={() => (selected = i)} aria-label="Picture {i + 1}"></button>
    {/each}
    <button class="thumb add" style:aspect-ratio={aspect} onclick={add} title="Add a picture">+</button>
  </div>
  {#if items[selected]}
    <PictureSlot {aspect} {pick} {notify} chooseLabel="Replace…"
      bind:value={() => items[selected], (v) => edited(selected, v)} />
  {:else}
    <p class="muted small">{empty || 'No pictures yet.'}</p>
  {/if}
</div>

<style>
  .list { display: flex; flex-direction: column; gap: 10px; min-width: 0; }
  .strip { display: flex; flex-wrap: wrap; gap: 8px; }
  .thumb { width: 112px; padding: 0; border-radius: 6px; background: var(--bg) no-repeat; border: 2px solid transparent; }
  .thumb.on { border-color: var(--accent); }
  .add { font-size: 26px; color: var(--muted, #999); border: 2px dashed var(--line); }
</style>
