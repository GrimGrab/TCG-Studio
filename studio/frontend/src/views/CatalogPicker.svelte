<script lang="ts">
  // "Add from catalog…": everything that entered Studio in any setup (the shared catalog's templates, plus what other
  // setups have), to copy into this setup without importing again. Files stay shared; this setup gets its own copy of
  // the game info (names, prices, tiers…), so editing it here never changes other setups.
  import { App, errText } from '../lib/api';
  import { grouped, groupOf as groupFor, sourceGames, OWN_GROUP } from '../lib/catalog';

  let { kind, sub = '', onadded, onclose }: {
    kind: 'set' | 'accessory' | 'furniture';
    sub?: string; // accessory kind to show first (the current tab), '' = all
    onadded: (ids: string[]) => void; onclose: () => void;
  } = $props();

  const noun = $derived(kind === 'set' ? 'sets' : kind === 'accessory' ? 'accessories' : 'furniture');
  let entries = $state<any[]>([]);
  let loading = $state(true);
  let error = $state('');
  let filter = $state('');
  let onlyKind = $state(true); // accessories: only the current tab's kind
  let showHere = $state(false);
  let picked = $state<Record<string, boolean>>({});
  let busy = $state(false);

  let games = $state<Record<string, string>>({}); // import source id → game ("Magic: The Gathering")

  async function load() {
    loading = true; error = '';
    try { entries = await App.CatalogList(kind); } catch (e) { error = errText(e); }
    loading = false;
  }
  load();
  sourceGames().then((g) => (games = g));
  const groupOf = (e: any): string => groupFor(e, games);

  const shown = $derived(entries.filter((e) => {
    if (e.here && !showHere) return false;
    if (kind === 'accessory' && sub && onlyKind && e.sub !== sub) return false;
    const q = filter.trim().toLowerCase();
    return !q || `${e.name} ${e.id} ${e.origin} ${e.author} ${e.sub} ${groupOf(e)}`.toLowerCase().includes(q);
  }));
  const groups = $derived(grouped(shown, games));
  const chosen = $derived(Object.keys(picked).filter((id) => picked[id] && entries.some((e) => e.id === id && !e.here)));

  function toggleGroup(list: any[], on: boolean) {
    for (const e of list) if (!e.here) picked[e.id] = on;
  }

  async function add() {
    busy = true; error = '';
    try {
      const r = await App.AddFromCatalog(kind, chosen);
      onadded(r.added ?? []);
    } catch (e) { error = errText(e); busy = false; }
  }

</script>

<div class="backdrop" role="presentation">
  <div class="dialog">
    <div class="row head">
      <h3 class="grow">Add {noun} from the catalog</h3>
      <button onclick={onclose} disabled={busy}>Cancel</button>
      <button class="primary" onclick={add} disabled={busy || !chosen.length}>
        {busy ? 'Adding…' : `Add ${chosen.length || ''} to this setup`}
      </button>
    </div>
    <p class="small muted">Everything imported or made in any of your setups. Adding is instant: the images and models are shared,
      and this setup gets its own copy of the rest — names, prices, tiers and levels you change here stay in this setup.
      {kind === 'set' ? 'Added sets show up on the Sets page; Install them as usual.' : 'Added items go at the end of the list.'}</p>
    <div class="row pick">
      <label class="field grow">Search<input placeholder="Name, mod or author" bind:value={filter} /></label>
      {#if kind === 'accessory' && sub}<label class="check"><input type="checkbox" bind:checked={onlyKind} /> Only {sub}</label>{/if}
      <label class="check"><input type="checkbox" bind:checked={showHere} /> Show what this setup already has</label>
    </div>
    {#if error}<p class="small err">{error}</p>{/if}
    <div class="list">
      {#if loading}<p class="small muted">Loading…</p>
      {:else if !shown.length}
        <p class="small muted">{entries.length ? `Nothing else to add — this setup has all ${noun} that match.` :
          `No ${noun} in the catalog yet. Anything you import from now on is added to it automatically.`}</p>
      {/if}
      {#each groups as [mod, list] (mod)}
        <div class="group">
          <div class="row ghead">
            <b class="grow">{mod || OWN_GROUP}{list[0].author && list[0].origin ? ` · ${list[0].author}` : ''}</b>
            <button class="small link" onclick={() => toggleGroup(list, true)}>Select all</button>
            <button class="small link" onclick={() => toggleGroup(list, false)}>None</button>
          </div>
          {#each list as e (e.id)}
            <div class="row item" class:here={e.here}>
              <label class="check grow">
                <input type="checkbox" disabled={e.here} checked={!!picked[e.id]} onchange={(ev) => (picked[e.id] = ev.currentTarget.checked)} />
                <span class="grow">{e.name || e.id}
                  <span class="small muted">· {e.detail}{e.here ? ' · already in this setup' : ''}{e.saved ? '' : ` · from setup “${e.from}”`}</span>
                </span>
              </label>
            </div>
          {/each}
        </div>
      {/each}
    </div>
  </div>
</div>

<style>
  .backdrop { position: fixed; inset: 0; background: rgba(0, 0, 0, 0.6); display: flex; align-items: center; justify-content: center; z-index: 100; }
  .dialog { width: min(820px, 94vw); height: min(760px, 90vh); background: var(--panel); border: 1px solid var(--line); border-radius: var(--radius);
    padding: 12px; display: flex; flex-direction: column; gap: 10px; }
  .head h3 { margin: 0; }
  .pick { align-items: flex-end; gap: 14px; }
  .list { flex: 1; min-height: 0; overflow: auto; display: flex; flex-direction: column; gap: 12px; }
  .group { border: 1px solid var(--line); border-radius: var(--radius); padding: 6px 10px; }
  .ghead { gap: 8px; padding: 2px 0 6px; border-bottom: 1px solid var(--line); margin-bottom: 4px; }
  .item { gap: 8px; padding: 2px 0; }
  .item.here { opacity: 0.55; }
  .small { font-size: 12px; margin: 0; }
  .err { color: var(--danger); }
</style>
