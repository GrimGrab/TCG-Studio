<script lang="ts">
  import { onMount } from 'svelte';
  import { App, errText } from '../lib/api';
  import { ClipboardSetText } from '../../wailsjs/runtime/runtime';

  // Setup page section: the game's log of its last session + a summary, to copy and send for help.
  let { notify }: { notify: (text: string, kind?: string) => void } = $props();
  let r = $state<any>(null);
  let loading = $state(false);
  let showLog = $state(false);

  async function refresh() {
    loading = true;
    try { r = await App.DebugReport(); } catch (e) { notify(errText(e), 'error'); }
    loading = false;
  }

  async function copy() {
    if (!r) return;
    const ok = await ClipboardSetText(r.header + '\n----- BepInEx\\LogOutput.log -----\n' + (r.log ?? ''));
    notify(ok ? 'Copied — paste it in Discord (long logs become a text file there).' : "Couldn't copy to the clipboard — use Save as file.", ok ? 'ok' : 'error');
  }

  async function save() {
    try {
      const path = await App.SaveDebugReport();
      if (path) notify('Saved ' + path, 'ok');
    } catch (e) { notify(errText(e), 'error'); }
  }

  onMount(refresh);
</script>

<section>
  <h3>Debug log</h3>
  <p class="small muted intro">Something doesn't work in game? Close the game, then copy this and send it to us on Discord. It's the game's log from its last session with a short summary; your Windows user name is replaced with %USERPROFILE%.</p>
  <div class="row">
    <button class="primary" onclick={copy} disabled={!r}>Copy to clipboard</button>
    <button onclick={save} disabled={!r}>Save as file…</button>
    <button onclick={refresh} disabled={loading}>{loading ? 'Reading…' : 'Refresh'}</button>
    {#if r?.found}<button class="linkish" onclick={() => (showLog = !showLog)}>{showLog ? 'Hide log' : 'Show log'}</button>{/if}
  </div>
  {#if r}
    {#each r.notes ?? [] as n}
      <p class="note {n.level}">{n.level === 'ok' ? '✓' : n.level === 'err' ? '✕' : '!'} {n.text}</p>
    {/each}
    {#if showLog}
      <pre>{r.header}</pre>
      <pre class="log">{r.log}</pre>
    {/if}
  {/if}
</section>

<style>
  section { background: var(--panel); border: 1px solid var(--line); border-radius: var(--radius); padding: 12px; display: flex; flex-direction: column; gap: 10px; }
  h3 { margin: 0; }
  .row { display: flex; gap: 8px; flex-wrap: wrap; }
  .intro { margin: 0; max-width: 820px; }
  .small { font-size: 12px; }
  .linkish { cursor: pointer; }
  .note { margin: 0; padding: 6px 10px; border-radius: 6px; background: var(--panel-2); border-left: 3px solid var(--line); font-size: 13px; }
  .note.ok { border-left-color: var(--ok); }
  .note.warn { border-left-color: #d9a400; }
  .note.err { border-left-color: var(--danger); }
  pre { margin: 0; background: var(--bg); border: 1px solid var(--line); border-radius: 6px; padding: 8px; font-size: 12px; white-space: pre-wrap; user-select: text; }
  .log { max-height: 420px; overflow: auto; }
</style>
