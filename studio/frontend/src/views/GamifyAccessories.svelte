<script lang="ts">
  // Gamify → Accessories tab: a license ladder per kind (deck boxes, playmats, sleeves, dice, comics, collection books) over the
  // custom accessories in library order, independent of set tiers.
  import { onMount } from 'svelte';
  import { App, money, errText, ask } from '../lib/api';

  let { notify }: { notify: (t: string, k?: string) => void } = $props();

  let KINDS = $state<any[]>([]);  // setfmt.AccessoryKinds
  // Vanilla ranges (game 1.02 restock table) shown as a hint per kind.
  const NOTES: Record<string, string> = {
    Deckbox: 'Vanilla: levels 5–8, licenses 100–200. Each also gets a big delivery box 3 levels later at twice the price.',
    Playmat: 'Vanilla: levels 7–77, licenses 500–15 000.',
    Sleeve: 'Vanilla: levels 2–30, licenses 50–800.',
    Dice: 'Vanilla: levels 3–4, licenses 50 (the ladder keeps rising beyond that).',
    Comic: 'Vanilla: levels 13–72, licenses 900–16 000.',
    Binder: 'Vanilla: levels 11 and 50, licenses 1 000 and 10 000.',
    BattleDeck: 'Vanilla: levels 9–22 at 1 000 (elements), 33–50 at 5 000 (Destiny).',
  };

  let list = $state<any[]>([]);
  let settings = $state<any>(null);
  let preview = $state<any[] | null>(null);
  let busy = $state(false);

  async function load() {
    list = (await App.Accessories()).accessories;
  }

  /** Swap with the previous/next accessory of the same kind (the library order is shared by both kinds). */
  async function move(id: string, kind: string, d: number) {
    const same = list.filter((a) => a.kind === kind);
    const i = same.findIndex((a) => a.id === id), j = i + d;
    if (j < 0 || j >= same.length) return;
    try {
      list = (await App.MoveAccessory(id, list.findIndex((a) => a.id === same[j].id))).accessories;
      preview = null;
    } catch (e) { notify(errText(e), 'error'); }
  }

  async function doPreview() {
    busy = true;
    try { preview = await App.GamifyAccessoryPreview(settings); } catch (e) { notify(errText(e), 'error'); }
    busy = false;
  }

  async function apply() {
    if (!(await ask(`Set the license level and price of all ${list.length} accessories from their ladders? They are reinstalled in the game.`))) return;
    busy = true;
    try {
      preview = await App.GamifyAccessoryApply(settings);
      await load();
      notify(`Updated ${list.length} accessories (restart the game to see it).`, 'ok');
    } catch (e) { notify(errText(e), 'error'); }
    busy = false;
  }

  const lic = (l: any, big: boolean) => (l ? `L${l.level} ${money(l.price)}${big && l.bigLevel ? ` / L${l.bigLevel} ${money(l.bigPrice)}` : ''}` : '–');
  const after = (id: string) => preview?.find((r: any) => r.id === id);

  onMount(async () => {
    try {
      KINDS = await App.AccessoryKinds();
      settings = await App.GamifyAccessoryDefaults();
      await load();
    } catch (e) { notify(errText(e), 'error'); }
  });
</script>

<p class="muted intro">
  Custom accessories (made under <b>Accessories</b>) get their own progression, separate from the set tiers — one ladder per
  type; types you haven't made anything for are hidden. In each list, license levels are spread from the first to the last level and prices follow the vanilla license curve.
</p>

{#if settings && list.length === 0}
  <p class="muted">No accessories yet — make some under <b>Accessories</b>.</p>
{/if}
{#if settings}
  <div class="kinds">
    {#each KINDS.filter((k) => list.some((a) => a.kind === k.kind)) as k (k.kind)}
      {@const items = list.filter((a) => a.kind === k.kind)}
      {@const s = settings[k.kind]}
      <section>
        <h3>{k.title} <span class="muted small">({items.length})</span></h3>
        <p class="muted small">{NOTES[k.kind] ?? ''}</p>
        <div class="settings">
          <label class="field">First level<input type="number" min="1" bind:value={s.minLevel} oninput={() => (preview = null)} /></label>
          <label class="field">Last level<input type="number" min="1" bind:value={s.maxLevel} oninput={() => (preview = null)} /></label>
          <label class="field" title="Multiplies the license price curve; 100% = vanilla">Price vs vanilla (%)<input type="number" min="10" step="10" value={Math.round(s.priceScale * 100)} oninput={(e) => { s.priceScale = (+e.currentTarget.value || 100) / 100; preview = null; }} /></label>
        </div>
        {#each items as a, i (a.id)}
          {@const r = after(a.id)}
          <div class="tier row">
            <b class="num">{i + 1}</b>
            <span class="grow">{a.name}
              <span class="muted small">{lic(a.license, a.kind === 'Deckbox')}</span>
              {#if r}<span class="small">→ <b>{lic(r.after, r.hasBig)}</b></span>{/if}
            </span>
            <button class="small" onclick={() => move(a.id, k.kind, -1)} disabled={busy || i === 0}>↑</button>
            <button class="small" onclick={() => move(a.id, k.kind, 1)} disabled={busy || i === items.length - 1}>↓</button>
          </div>
        {/each}
        {#if items.length === 0}<p class="muted small">None yet — make one under Accessories.</p>{/if}
      </section>
    {/each}
  </div>
{/if}

<div class="row">
  <button onclick={doPreview} disabled={busy || !list.length}>Preview</button>
  <button class="primary" onclick={apply} disabled={busy || !list.length}>Apply to {list.length} accessor{list.length === 1 ? 'y' : 'ies'}</button>
  {#if preview}<span class="muted small">Preview shown next to each item (now → after).</span>{/if}
</div>

<style>
  .intro { max-width: 900px; margin: 0; }
  .kinds { display: grid; grid-template-columns: 1fr 1fr; gap: 14px; align-items: start; }
  section { background: var(--panel); border: 1px solid var(--line); border-radius: var(--radius); padding: 12px; display: flex; flex-direction: column; gap: 8px; }
  .settings { display: grid; grid-template-columns: repeat(3, 1fr); gap: 8px; }
  .tier { padding: 6px 8px; border-radius: 6px; background: var(--panel-2); }
  .num { width: 24px; text-align: center; }
  .small { font-size: 12px; }
</style>
