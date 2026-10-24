<script lang="ts">
  // The one save bar of every editor page: sits above the page's scroll area (a sibling of it inside <main>), shows whether
  // the page has unsaved changes, and owns Save (also Ctrl+S), Revert (asks first) and the leave guard. Nothing on a page
  // reaches the game before Save.
  import type { Snippet } from 'svelte';
  import { ask } from '../lib/api';
  import { leaveGuard } from '../lib/leaveGuard';

  let { dirty, saving = false, onsave, onrevert, leaveMessage = 'Leave without saving? Your unsaved changes will be lost.',
    revertMessage = 'Throw away your unsaved changes on this page?', saveTitle = '', title, extra }: {
    dirty: boolean; saving?: boolean; onsave: () => void | Promise<void>; onrevert: () => void | Promise<void>;
    leaveMessage?: string; revertMessage?: string; saveTitle?: string; title?: Snippet; extra?: Snippet;
  } = $props();

  $effect(() => leaveGuard(() => dirty, leaveMessage));

  async function revert() {
    if (dirty && (await ask(revertMessage))) await onrevert();
  }
  function onKey(e: KeyboardEvent) {
    if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 's') {
      e.preventDefault();
      if (dirty && !saving) onsave();
    }
  }
</script>

<svelte:window onkeydown={onKey} />

<div class="savebar">
  <div class="title grow">{@render title?.()}</div>
  {@render extra?.()}
  <span class="status" class:dirty>{saving ? 'Saving…' : dirty ? '● Unsaved changes' : 'Saved'}</span>
  <button onclick={revert} disabled={!dirty || saving}>Revert</button>
  <button class="primary" onclick={() => onsave()} disabled={!dirty || saving} title={saveTitle || 'Save (Ctrl+S)'}>Save</button>
</div>

<style>
  .savebar {
    flex: none; display: flex; align-items: center; gap: 8px; padding: 10px 20px; background: var(--panel);
    border-bottom: 1px solid var(--line); min-height: 52px;
  }
  .title { display: flex; align-items: center; gap: 8px; min-width: 0; }
  .status { font-size: 12px; color: var(--muted); white-space: nowrap; }
  .status.dirty { color: var(--accent-2, #d9a400); }
</style>
