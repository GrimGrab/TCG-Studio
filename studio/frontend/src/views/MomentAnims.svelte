<script lang="ts">
  // Which animations play at each moment, like LineList does for lines: the ones handed down (built-in, the setup's), each switched off
  // or back on by itself, then the ones added here. Everything adds up; the mod picks one at random (Moves.Pick). onchange gets the
  // moment's new switched-off and added lists.
  let { moments, anims, inherited, off, added, onchange }: {
    moments: { id: string; label: string }[];
    anims: { id: string; name: string }[];
    inherited: (moment: string) => { id: string; from: string }[];
    off: (moment: string) => string[];
    added: (moment: string) => string[];
    onchange: (moment: string, off: string[], added: string[]) => void;
  } = $props();

  let search = $state('');
  let withAnims = $state(false);
  const nameOf = (id: string) => anims.find((a) => a.id === id)?.name ?? '?';
  const shown = $derived(moments.filter((m) => (!search || m.label.toLowerCase().includes(search.toLowerCase()))
    && (!withAnims || inherited(m.id).length > 0 || added(m.id).length > 0)));
  const playing = (m: string) => inherited(m).filter((a) => !off(m).includes(a.id)).length + added(m).length;

  function toggle(m: string, id: string) {
    const o = off(m);
    onchange(m, o.includes(id) ? o.filter((x) => x !== id) : [...o, id], added(m));
  }
  function add(m: string, id: string) {
    if (id && !added(m).includes(id)) onchange(m, off(m), [...added(m), id]);
  }
</script>

<div class="tools">
  <input type="search" placeholder="Find a moment" bind:value={search} />
  <label class="small"><input type="checkbox" bind:checked={withAnims} /> Only moments with animations</label>
</div>
{#each shown as m (m.id)}
  <details open={off(m.id).length > 0 || added(m.id).length > 0}>
    <summary>{m.label}<span class="n">{playing(m.id)}</span></summary>
    <div class="rows">
      {#each inherited(m.id) as a (a.id)}
        {@const isOff = off(m.id).includes(a.id)}
        <div class="row" class:isOff>
          <span class="name">{nameOf(a.id)}</span><span class="from">{a.from}</span>
          <button class="mini" title={isOff ? 'Switch back on' : 'Switch off'} onclick={() => toggle(m.id, a.id)}>{isOff ? '↺' : '✕'}</button>
        </div>
      {/each}
      {#each added(m.id) as id (id)}
        <div class="row">
          <span class="name">{nameOf(id)}</span><span class="from">added here</span>
          <button class="mini" title="Remove" onclick={() => onchange(m.id, off(m.id), added(m.id).filter((x) => x !== id))}>✕</button>
        </div>
      {/each}
      <select value="" onchange={(e) => { add(m.id, e.currentTarget.value); e.currentTarget.value = ''; }}>
        <option value="">Add an animation…</option>
        {#each anims.filter((a) => !added(m.id).includes(a.id) && !inherited(m.id).some((i) => i.id === a.id)) as a (a.id)}<option value={a.id}>{a.name}</option>{/each}
      </select>
    </div>
  </details>
{/each}

<style>
  .tools { display: flex; gap: 10px; align-items: center; margin-bottom: 6px; }
  .tools input[type='search'] { flex: 1; max-width: 320px; }
  summary { cursor: pointer; padding: 3px 0; }
  .n { margin-left: 8px; opacity: 0.6; font-size: 0.85em; }
  .rows { display: flex; flex-direction: column; gap: 3px; padding: 2px 0 8px 16px; }
  .row { display: flex; gap: 8px; align-items: center; }
  .row.isOff .name { text-decoration: line-through; opacity: 0.5; }
  .from { opacity: 0.6; font-size: 0.85em; margin-left: auto; }
  .mini { padding: 0 6px; }
  select { max-width: 260px; }
</style>
