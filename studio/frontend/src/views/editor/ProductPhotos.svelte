<script lang="ts">
  // "Find product photos": TCGplayer's photos of a set's sealed products (via tcgcsv). Opens on the project's own set when
  // it can be matched by code or name; the user can pick another game or set. Picking a photo downloads it into the
  // project (white background cropped away) and hands its path to the pack art editor.
  import { untrack } from 'svelte';
  import { App, errText } from '../../lib/api';

  let { projectId, which, onpick, oncancel }: {
    projectId: string; which: 'pack' | 'box';
    onpick: (rel: string, name: string) => void; oncancel: () => void;
  } = $props();

  let games = $state<{ category: number; name: string }[]>([]);
  let category = $state(0);
  let groups = $state<{ id: number; name: string; code: string; released: string }[]>([]);
  let group = $state(0);
  let filter = $state('');
  let products = $state<any[]>([]);
  let kind = $state<string>(untrack(() => which)); // starts on what is being edited
  let loading = $state('');
  let error = $state('');
  let taking = $state(0);

  async function loadSets(cat: number) {
    loading = 'Loading sets…'; error = '';
    try {
      const r = await App.ProductPhotoSets(projectId, cat);
      games = r.games; category = r.category; groups = r.groups ?? [];
      group = r.match;
      if (!r.match) { products = []; error = 'Couldn\'t find this set on TCGplayer by itself — pick it from the list.'; }
    } catch (e) { error = errText(e); }
    loading = '';
    if (group) await loadProducts();
  }

  async function loadProducts() {
    products = []; error = '';
    if (!group) return;
    loading = 'Loading products…';
    try {
      products = (await App.ProductPhotos(category, group)) ?? [];
      if (!products.length) error = 'TCGplayer has no product photos for this set.';
      else if (!products.some((p) => p.kind === kind)) kind = 'all';
    } catch (e) { error = errText(e); }
    loading = '';
  }

  async function take(p: any) {
    taking = p.id; error = '';
    try { onpick(await App.UseProductPhoto(projectId, p), p.name); }
    catch (e) { error = errText(e); }
    taking = 0;
  }

  loadSets(0);

  const shownGroups = $derived.by(() => {
    const f = filter.trim().toLowerCase();
    const gs = f ? groups.filter((g) => g.name.toLowerCase().includes(f) || g.code.toLowerCase() === f) : groups;
    const cur = groups.find((g) => g.id === group);
    return cur && !gs.includes(cur) ? [cur, ...gs] : gs;
  });
  const shown = $derived(kind === 'all' ? products : products.filter((p) => p.kind === kind));
  const key = (e: KeyboardEvent) => { if (e.key === 'Escape') oncancel(); };
</script>

<svelte:window onkeydown={key} />

<div class="backdrop" role="presentation">
  <div class="dialog">
    <div class="row head">
      <h3 class="grow">Product photos</h3>
      <button onclick={oncancel}>Cancel</button>
    </div>
    <div class="row pick">
      <label class="field">Game
        <select value={category} onchange={(e) => loadSets(+e.currentTarget.value)}>
          {#each games as g}<option value={g.category}>{g.name}</option>{/each}
        </select>
      </label>
      <label class="field">Search sets<input placeholder="Name or code" bind:value={filter} /></label>
      <label class="field grow">Set
        <select bind:value={group} onchange={loadProducts}>
          {#if !group}<option value={0}>— choose a set —</option>{/if}
          {#each shownGroups as g}<option value={g.id}>{g.name}{g.code ? ` (${g.code})` : ''}{g.released ? ` · ${g.released.slice(0, 4)}` : ''}</option>{/each}
        </select>
      </label>
      <label class="field">Show
        <select bind:value={kind}>
          <option value="pack">Booster packs</option>
          <option value="box">Booster boxes</option>
          <option value="all">All products</option>
        </select>
      </label>
    </div>
    {#if loading}<p class="small muted">{loading}</p>{/if}
    {#if error}<p class="small err">{error}</p>{/if}
    <div class="grid">
      {#each shown as p (p.id)}
        <button class="card" disabled={!!taking} onclick={() => take(p)} title="Use this photo on the {which}">
          <img src={p.thumb} alt="" loading="lazy" />
          <span class="small">{taking === p.id ? 'Downloading…' : p.name}</span>
        </button>
      {:else}
        {#if !loading && products.length}<p class="small muted">No {kind === 'pack' ? 'booster packs' : 'booster boxes'} — try All products.</p>{/if}
      {/each}
    </div>
    <p class="small muted">Photos from TCGplayer's catalog (via tcgcsv.com). Older sets only have small photos. The photo is added as a layer:
      move or resize it, and use <b>Straighten…</b> on photos taken at an angle.</p>
  </div>
</div>

<style>
  .backdrop { position: fixed; inset: 0; background: rgba(0, 0, 0, 0.6); display: flex; align-items: center; justify-content: center; z-index: 100; }
  .dialog { width: min(1100px, 94vw); height: min(780px, 90vh); background: var(--panel); border: 1px solid var(--line); border-radius: var(--radius);
    padding: 12px; display: flex; flex-direction: column; gap: 10px; }
  .head h3 { margin: 0; }
  .pick { align-items: flex-end; flex-wrap: wrap; }
  .grid { flex: 1; min-height: 0; overflow: auto; display: grid; grid-template-columns: repeat(auto-fill, minmax(150px, 1fr)); gap: 10px; align-content: start; }
  .card { display: flex; flex-direction: column; align-items: center; gap: 6px; padding: 8px; height: auto; text-align: center; }
  .card img { width: 100%; height: 160px; object-fit: contain; background: #fff; border-radius: 4px; }
  .small { font-size: 12px; margin: 0; }
  .err { color: var(--danger); }
</style>
