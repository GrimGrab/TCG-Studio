<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { App, EventsOn, EventsOff, errText } from '../lib/api';

  let { open, notify }: { open: (id: string) => void; notify: (t: string, k?: string) => void } = $props();

  let sets = $state<any[]>([]);
  let loading = $state(true);
  let query = $state('');
  let type = $state('main');
  let options = $state<any>(null);
  let importing = $state<string | null>(null);
  let progress = $state<any>(null);

  const MAIN_TYPES = ['core', 'expansion', 'masters', 'draft_innovation', 'commander', 'starter', 'funny'];

  const types = $derived([...new Set(sets.map((s) => s.set_type))].sort());
  const shown = $derived(
    sets.filter((s) => {
      if (type === 'main' && !MAIN_TYPES.includes(s.set_type)) return false;
      if (type !== 'main' && type !== 'all' && s.set_type !== type) return false;
      const q = query.trim().toLowerCase();
      return !q || s.name.toLowerCase().includes(q) || s.code.toLowerCase() === q;
    })
  );

  async function load(refresh = false) {
    loading = true;
    try {
      sets = await App.ScryfallSets(refresh);
    } catch (e) {
      notify('Could not reach Scryfall: ' + errText(e), 'error');
    }
    loading = false;
  }

  async function doImport(s: any) {
    importing = s.code;
    progress = { stage: 'cards', done: 0, total: s.card_count, message: 'Starting…' };
    try {
      const id = await App.ImportScryfallSet(s.code, options);
      notify(progress?.message ?? 'Imported', 'ok');
      open(id);
    } catch (e) {
      notify(errText(e), 'error');
    }
    importing = null;
    progress = null;
    load();
  }

  onMount(async () => {
    EventsOn('import:progress', (p: any) => (progress = p));
    options = await App.DefaultImportOptions();
    load();
  });
  onDestroy(() => EventsOff('import:progress'));
</script>

<div class="page">
  <header class="row">
    <h2 class="grow">Import from Scryfall</h2>
    <button onclick={() => load(true)} disabled={loading}>Refresh list</button>
  </header>

  <div class="filters row">
    <input class="grow" placeholder="Search sets by name or code…" bind:value={query} />
    <select bind:value={type}>
      <option value="main">Main sets</option>
      <option value="all">All types</option>
      {#each types as t}<option value={t}>{t.replaceAll('_', ' ')}</option>{/each}
    </select>
    {#if options}
      <label class="check"><input type="checkbox" bind:checked={options.includeVariants} /> Include variant printings</label>
      <label class="check">Image width
        <select bind:value={options.imageWidth}>
          <option value={384}>384</option><option value={512}>512</option><option value={0}>745 (original)</option>
        </select>
      </label>
    {/if}
  </div>

  {#if progress}
    <div class="progress">
      <div class="row"><b class="grow">Importing {importing?.toUpperCase()}</b><button class="small" onclick={() => App.CancelImport()}>Cancel</button></div>
      <div class="bar"><div style="width:{progress.total ? (100 * progress.done) / progress.total : 5}%"></div></div>
      <div class="muted">{progress.stage === 'cards' ? 'Card list' : 'Images'} — {progress.message}</div>
    </div>
  {/if}

  {#if loading}
    <p class="muted">Loading sets from Scryfall…</p>
  {:else}
    <p class="muted">{shown.length} sets</p>
    <div class="grid">
      {#each shown as s (s.code)}
        <div class="set">
          <img src={s.icon_svg_uri} alt="" />
          <div class="grow">
            <div class="name">{s.name}</div>
            <div class="muted small">{s.code.toUpperCase()} · {s.released_at ?? '?'} · {s.card_count} cards · {s.set_type.replaceAll('_', ' ')}</div>
          </div>
          {#if s.imported}
            <button onclick={() => open('mtg-' + s.code)}>Open</button>
          {:else}
            <button class="primary" disabled={!!importing} onclick={() => doImport(s)}>Import</button>
          {/if}
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .page { padding: 20px 24px; overflow: auto; height: 100%; }
  header { margin-bottom: 12px; }
  .filters { margin-bottom: 12px; flex-wrap: wrap; }
  .progress { background: var(--panel); border: 1px solid var(--accent-2); border-radius: var(--radius); padding: 12px; margin-bottom: 12px; display: flex; flex-direction: column; gap: 8px; }
  .bar { height: 8px; background: var(--bg); border-radius: 4px; overflow: hidden; }
  .bar div { height: 100%; background: var(--accent); transition: width 0.2s; }
  .grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(360px, 1fr)); gap: 8px; }
  .set { display: flex; gap: 12px; align-items: center; background: var(--panel); border: 1px solid var(--line); border-radius: var(--radius); padding: 10px; }
  .set img { width: 32px; height: 32px; filter: invert(1) opacity(0.85); }
  .name { font-weight: 600; }
  .small { font-size: 12px; }
</style>
