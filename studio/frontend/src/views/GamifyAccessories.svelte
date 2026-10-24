<script lang="ts">
  // Gamify → Accessories tab: a license ladder per kind (deck boxes, playmats, sleeves, dice, comics, collection books) over the
  // custom accessories in library order, independent of set tiers.
  import { onMount } from 'svelte';
  import { App, money, errText, ask } from '../lib/api';
  import { accFile } from '../lib/urls';

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
    BoardGame: 'Vanilla (Tabletop Geek): levels 5–24, licenses 200–1 500.',
    CardSupply: 'Vanilla (Tabletop Geek): levels 10–49, licenses 800–8 000.',
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
    if (!(await ask(`Set the license level and price of all ${list.length} accessories from their ladders? The game shows them after a restart.`))) return;
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

  // One type at a time (remembered): its settings and ladder on the right, every type with a count on the left.
  const used = $derived(KINDS.filter((k) => list.some((a) => a.kind === k.kind)));
  let current = $state((() => { try { return localStorage.getItem('gamify.accKind') || ''; } catch { return ''; } })());
  $effect(() => { try { localStorage.setItem('gamify.accKind', current); } catch {} });
  const kind = $derived(used.find((k) => k.kind === current) ?? used[0]);
  const items = $derived(kind ? list.filter((a) => a.kind === kind.kind) : []);

  onMount(async () => {
    try {
      KINDS = await App.AccessoryKinds();
      settings = await App.GamifyAccessoryDefaults();
      await load();
    } catch (e) { notify(errText(e), 'error'); }
  });
</script>

{#if settings && list.length === 0}
  <p class="muted">No accessories yet — make some under <b>Accessories</b>.</p>
{/if}
{#if settings && kind}
  {@const st = settings[kind.kind]}
  <div class="md">
    <nav class="kindnav">
      {#each used as k (k.kind)}
        <button class="item" class:active={k.kind === kind.kind} onclick={() => (current = k.kind)}>
          <span class="name">{k.title}</span><span class="count">{list.filter((a) => a.kind === k.kind).length}</span>
        </button>
      {/each}
    </nav>
    <div class="detail">
      <section class="block">
        <div class="bhead"><h2>{kind.title}</h2><span class="count">{items.length}</span></div>
        {#if NOTES[kind.kind]}<p class="muted small">{NOTES[kind.kind]}</p>{/if}
        <div class="settings">
          <label class="field">First level<input type="number" min="1" bind:value={st.minLevel} oninput={() => (preview = null)} /></label>
          <label class="field">Last level<input type="number" min="1" bind:value={st.maxLevel} oninput={() => (preview = null)} /></label>
          <label class="field" title="Multiplies the license price curve; 100% = vanilla">Price vs vanilla (%)<input type="number" min="10" step="10" value={Math.round(st.priceScale * 100)} oninput={(e) => { st.priceScale = (+e.currentTarget.value || 100) / 100; preview = null; }} /></label>
        </div>
      </section>
      <section class="block">
        <div class="bhead"><h2>Ladder</h2><span class="muted small">List order = unlock order (also the order on the Accessories page).</span></div>
        <div class="ladder">
          <div class="lhead muted small"><span></span><span></span><span>Name</span><span>License now</span><span>{preview ? 'After' : ''}</span><span></span></div>
          {#each items as a, i (a.id)}
            {@const r = after(a.id)}
            <div class="tier">
              <span class="num">{i + 1}</span>
              <span class="ico">{#if a.icon}<img src={accFile(a.icon)} alt="" loading="lazy" />{/if}</span>
              <span class="name">{a.name}</span>
              <span class="muted small">{lic(a.license, a.kind === 'Deckbox')}</span>
              <span class="small">{#if r}<b>{lic(r.after, r.hasBig)}</b>{/if}</span>
              <span class="arrows">
                <button class="small" onclick={() => move(a.id, kind.kind, -1)} disabled={busy || i === 0} aria-label="Earlier">↑</button>
                <button class="small" onclick={() => move(a.id, kind.kind, 1)} disabled={busy || i === items.length - 1} aria-label="Later">↓</button>
              </span>
            </div>
          {/each}
        </div>
      </section>
    </div>
  </div>
{/if}

<div class="actionbar">
  <span class="grow muted">{preview ? 'Preview shown next to each item (now → after).' : `${list.length} accessor${list.length === 1 ? 'y' : 'ies'} — preview first, nothing changes until Apply.`}</span>
  <button onclick={doPreview} disabled={busy || !list.length}>Preview</button>
  <button class="primary" onclick={apply} disabled={busy || !list.length}>Apply to {list.length} accessor{list.length === 1 ? 'y' : 'ies'}</button>
</div>

<style>
  .md { display: grid; grid-template-columns: 220px minmax(0, 1fr); gap: 14px; align-items: start; }
  .kindnav { position: sticky; top: 0; display: flex; flex-direction: column; gap: 2px; background: var(--panel); border: 1px solid var(--line);
    border-radius: 10px; padding: 6px; }
  .item { display: flex; align-items: center; gap: 8px; text-align: left; padding: 8px 10px; border-radius: 8px; border: 1px solid transparent; background: none; }
  .item:hover:not(:disabled) { background: var(--panel-2); border-color: transparent; }
  .item.active { background: var(--panel-2); border-color: var(--accent-2); }
  .item .name { flex: 1; font-weight: 600; }
  .detail { display: flex; flex-direction: column; gap: 14px; min-width: 0; }
  .count { font-size: 11px; color: var(--muted); background: var(--panel-2); border-radius: 8px; padding: 0 6px; }
  /* Fields shrink with the card instead of running past its edge. */
  .settings { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 10px; max-width: 640px; }
  .settings .field { min-width: 0; }
  .settings input { width: 100%; min-width: 0; }
  .ladder { display: flex; flex-direction: column; gap: 3px; }
  .lhead, .tier { display: grid; grid-template-columns: 32px 36px minmax(0, 1.4fr) minmax(0, 1fr) minmax(0, 1fr) auto; align-items: center; gap: 10px; }
  .lhead { padding: 0 10px 2px; }
  .tier { padding: 5px 10px; border-radius: 8px; background: var(--panel-2); }
  .tier .name { font-weight: 600; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .num { display: inline-grid; place-items: center; min-width: 24px; height: 24px; border-radius: 12px; background: var(--accent-2); font-weight: 700; font-size: 12px; }
  .ico { width: 32px; height: 32px; display: grid; place-items: center; background: var(--bg); border-radius: 5px; }
  .ico img { width: 30px; height: 30px; object-fit: contain; }
  .arrows { display: flex; gap: 4px; }
  .actionbar { position: sticky; bottom: 0; display: flex; align-items: center; gap: 12px; background: var(--panel-2); border: 1px solid var(--line);
    border-radius: 10px; padding: 10px 14px; box-shadow: 0 -6px 18px rgba(0, 0, 0, 0.35); }
</style>
