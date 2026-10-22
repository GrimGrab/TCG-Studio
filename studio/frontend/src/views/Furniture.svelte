<script lang="ts">
  // Furniture page: custom furniture pieces (list + editor in FurnitureTab, which keeps the draft). Stored in the accessory
  // library file.
  import { App } from '../lib/api';
  import FurnitureTab from './FurnitureTab.svelte';
  import SaveBar from './SaveBar.svelte';

  let { notify }: { notify: (t: string, k?: string) => void } = $props();

  let tab = $state<FurnitureTab | null>(null);
  let dirty = $state(false);
  let saving = $state(false);
</script>

<SaveBar {dirty} {saving} onsave={() => tab?.save()} onrevert={() => tab?.revert()}
  leaveMessage="Leave Furniture without saving? Your unsaved changes will be lost."
  revertMessage="Throw away every unsaved change on this page (edits, new and deleted pieces)?">
  {#snippet title()}<h2>Furniture</h2>{/snippet}
  {#snippet extra()}<button onclick={() => App.OpenAccessoriesFolder()} title="Furniture is saved with the accessories (accessories.json and images/).">Open folder</button>{/snippet}
</SaveBar>
<div class="page">
  <FurnitureTab bind:this={tab} bind:dirty bind:saving {notify} />
</div>

<style>
  .page { padding: 16px 20px; flex: 1; min-height: 0; display: flex; flex-direction: column; gap: 10px; overflow: auto; }
</style>
