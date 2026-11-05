<script lang="ts">
  // One situation's lines on the Customers page: the lines handed down (built-in, the setup's, a trait's), each switched off
  // or back on by itself, then the lines added here (edit or delete each) and a box to add one. Everything adds up; the
  // mod picks one line at random (Traits.LinesFor). onchange gets the new switched-off and added lists.
  let { inherited, removed, added, onchange, onplay, noun = 'line', sayOn = 'Say this line again', sayOff = 'Never say this line' }: {
    inherited: { text: string; from: string }[];
    removed: string[];
    added: string[];
    onchange: (removed: string[], added: string[]) => void;
    onplay?: (text: string) => void; // hear a line in this voice
    noun?: string; // what's listed, in the buttons' words
    sayOn?: string;
    sayOff?: string;
  } = $props();

  let draft = $state('');

  function toggle(text: string) {
    onchange(removed.includes(text) ? removed.filter((r) => r !== text) : [...removed, text], added);
  }
  function edit(i: number, text: string) {
    const next = [...added];
    next[i] = text;
    onchange(removed, next);
  }
  function del(i: number) {
    onchange(removed, added.filter((_, j) => j !== i));
  }
  function add() {
    const t = draft.trim();
    if (!t) return;
    onchange(removed, [...added, t]);
    draft = '';
  }
</script>

<div class="lines">
  {#each inherited as l (l.from + '|' + l.text)}
    {@const off = removed.includes(l.text)}
    <div class="line" class:off>
      {#if onplay}<button class="mini" title="Hear this line" onclick={() => onplay(l.text)}>▶</button>{/if}
      <span class="text">{l.text}</span>
      <span class="from">{l.from}</span>
      <button class="mini" title={off ? sayOn : sayOff} onclick={() => toggle(l.text)}>{off ? 'Restore' : '✕'}</button>
    </div>
  {/each}
  {#each added as text, i}
    <div class="line own">
      {#if onplay}<button class="mini" title="Hear this line" disabled={!text.trim()} onclick={() => onplay(text)}>▶</button>{/if}
      <input value={text} oninput={(e) => edit(i, e.currentTarget.value)} />
      <button class="mini" title={`Delete this ${noun}`} onclick={() => del(i)}>✕</button>
    </div>
  {/each}
  <div class="line">
    <input placeholder={`Add a ${noun}…`} bind:value={draft} onkeydown={(e) => { if (e.key === 'Enter') add(); }} />
    <button class="mini" onclick={add} disabled={!draft.trim()}>Add</button>
  </div>
</div>

<style>
  .lines { display: flex; flex-direction: column; gap: 3px; margin: 4px 0 8px; }
  .line { display: flex; gap: 8px; align-items: center; }
  .line .text { flex: 1; min-width: 0; }
  .line input { flex: 1; min-width: 0; }
  .line.off .text { text-decoration: line-through; color: var(--muted); }
  .from { color: var(--muted); font-size: 12px; white-space: nowrap; }
  .mini { padding: 1px 8px; margin: 0; font-size: 12px; }
</style>
