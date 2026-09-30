<script lang="ts">
  // Furniture page: custom furniture pieces (list + editor in FurnitureTab). Stored in the accessory library file.
  import { App } from '../lib/api';
  import FurnitureTab from './FurnitureTab.svelte';

  let { notify }: { notify: (t: string, k?: string) => void } = $props();
</script>

<div class="page">
  <header class="row">
    <h2 class="grow">Furniture</h2>
    <button onclick={() => App.OpenAccessoriesFolder()} title="Furniture is saved with the accessories (accessories.json and images/).">Open folder</button>
  </header>
  <FurnitureTab {notify} />
</div>

<style>
  .page { padding: 16px 20px; height: 100%; display: flex; flex-direction: column; gap: 10px; overflow: auto; }
</style>
