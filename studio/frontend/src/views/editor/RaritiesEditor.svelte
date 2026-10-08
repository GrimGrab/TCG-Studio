<script lang="ts">
  // The set's rarities (set.json "rarities"), lowest first. Each is a rarity of its own in game: its name on the card, binder and
  // Check Price, and its own weight in pack slots. A set without a list has Common, Rare, Epic, Legendary; the first edit writes them.
  import { App, setRarities, rarityColor, rarityId, errText, ask, type RarityDef } from '../../lib/api';

  let { project, notify }: { project: any; notify: (t: string, k?: string) => void } = $props();

  const rows = $derived<RarityDef[]>(setRarities(project.set));
  const counts = $derived.by(() => {
    const m: Record<string, number> = {};
    for (const c of project.set.cards) m[c.rarity] = (m[c.rarity] ?? 0) + 1;
    return m;
  });
  const hasSource = $derived(!project.set.rarities?.length && project.set.cards.some((c: any) => project.meta?.cards?.[c.id]?.srcRarity));
  let busy = $state(false);

  /** The set's own list, written from the rows shown on the first edit. */
  function list(): RarityDef[] {
    if (!project.set.rarities?.length) project.set.rarities = rows.map((r) => ({ ...r }));
    return project.set.rarities;
  }

  async function useSource() {
    busy = true;
    try {
      const p: any = await App.UseSourceRarities(project);
      project.set.rarities = p.set.rarities;
      project.set.cards = p.set.cards;
      project.set.packs = p.set.packs;
      project.meta.rarityOrder = p.meta.rarityOrder;
      notify(`The set now has the source's ${p.set.rarities.length} rarities — save to keep them`, 'ok');
    } catch (e) { notify(errText(e), 'error'); }
    busy = false;
  }

  function add() {
    const l = list();
    let name = 'New rarity', n = 2;
    while (l.some((r) => r.name === name)) name = `New rarity ${n++}`;
    let id = rarityId(name), k = 2;
    while (l.some((r) => r.id === id)) id = `${rarityId(name)}-${k++}`;
    l.push({ id, name });
  }

  function move(i: number, d: number) {
    const l = list();
    [l[i], l[i + d]] = [l[i + d], l[i]];
  }

  async function remove(i: number) {
    const l = list();
    if (l.length === 1) return;
    const r = l[i];
    const to = l[i > 0 ? i - 1 : 1];
    const n = counts[r.id] ?? 0;
    if (n && !(await ask(`Delete “${r.name}”? Its ${n} card(s) and its pack weights move to “${to.name}”.`))) return;
    for (const c of project.set.cards) if (c.rarity === r.id) c.rarity = to.id;
    for (const p of project.set.packs)
      for (const s of p.slots ?? [])
        if (s.weights[r.id] !== undefined) {
          s.weights[to.id] = (s.weights[to.id] ?? 0) + s.weights[r.id];
          delete s.weights[r.id];
          s.weights = { ...s.weights };
        }
    l.splice(i, 1);
  }

  const set = (i: number, field: keyof RarityDef, v: string | undefined) => { (list()[i] as any)[field] = v; };
</script>

<section>
  <h3>Rarities</h3>
  <p class="muted small">Lowest first. Each rarity is its own in game: its name shows on the cards, in the binder and in Check Price, and each
    has its own weight in pack slots (Packs tab). Cards show the game's rarity icon for the rarity's place in the list.</p>
  {#if hasSource}
    <div class="row"><button onclick={useSource} disabled={busy}>Use the source's rarities</button>
      <span class="muted small">the import's own names (e.g. Secret Rare, SR, a mod's tiers) instead of these four</span></div>
  {/if}
  <table class="rar">
    <thead><tr><th></th><th>Name</th><th>Colour</th><th>Cards</th><th></th></tr></thead>
    <tbody>
      {#each rows as r, i (r.id)}
        <tr>
          <td class="nowrap">
            <button class="small" onclick={() => move(i, -1)} disabled={i === 0}>↑</button>
            <button class="small" onclick={() => move(i, 1)} disabled={i === rows.length - 1}>↓</button>
          </td>
          <td><input value={r.name} oninput={(e) => set(i, 'name', e.currentTarget.value)} title="Id (stable): {r.id}" /></td>
          <td><input type="color" value={rarityColor(project.set, r.id)} oninput={(e) => set(i, 'color', e.currentTarget.value)} />
            {#if r.color}<button class="small" title="Automatic colour" onclick={() => set(i, 'color', undefined)}>✕</button>{/if}</td>
          <td>{counts[r.id] ?? 0}</td>
          <td><button class="small" onclick={() => remove(i)} disabled={rows.length === 1}>Delete</button></td>
        </tr>
      {/each}
    </tbody>
  </table>
  <div class="row"><button class="small" onclick={add}>+ Rarity</button></div>
</section>

<style>
  .rar { border-collapse: collapse; width: 100%; }
  .rar th { text-align: left; font-weight: 500; color: var(--muted, #8a93a3); font-size: 12px; padding: 2px 6px; }
  .rar td { padding: 2px 6px; vertical-align: middle; }
  .rar input:not([type='color']) { width: 100%; min-width: 120px; }
  .rar input[type='color'] { width: 34px; height: 24px; padding: 0; vertical-align: middle; }
  .nowrap { white-space: nowrap; }
</style>
