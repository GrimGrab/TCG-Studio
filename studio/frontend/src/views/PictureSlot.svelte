<script lang="ts">
  // One picture of the Shop look page: choose an image, drag its crop box, centre it or go back to vanilla. Every change is
  // saved as mod settings right away and returns the page's new state through `onlook`.
  import { App, errText } from '../lib/api';
  import CropBox from './CropBox.svelte';

  let { slot, picture, label = '', notify, onlook }: {
    slot: string; // sign | wallpaper | open | closed (app_shoplook.go shopPictures)
    picture: { image: string; crop: number[] | null; aspect: number };
    label?: string; // drawn in the crop box (what the game puts on top)
    notify: (t: string, k?: string) => void;
    onlook: (look: any) => void;
  } = $props();

  async function run(f: () => Promise<any>) {
    try { onlook(await f()); } catch (e) { notify(errText(e), 'error'); }
  }
</script>

<div class="slot">
  {#if picture.image}
    <CropBox src={picture.image} aspect={picture.aspect} crop={picture.crop} {label}
      onchange={(c) => run(() => App.SetShopPictureCrop(slot, c))} />
  {/if}
  <div class="row">
    <button onclick={() => run(() => App.ChooseShopPicture(slot))}>Choose image…</button>
    {#if picture.image}
      <button onclick={() => run(() => App.SetShopPictureCrop(slot, []))} disabled={!picture.crop}>Centre</button>
      <button class="danger" onclick={() => run(() => App.RemoveShopPicture(slot))}>Remove</button>
    {/if}
  </div>
</div>

<style>
  .slot { display: flex; flex-direction: column; gap: 8px; min-width: 0; }
</style>
