<script lang="ts">
  import type { Snippet } from 'svelte';
  import { App, errText, RARITY_COLORS } from '../lib/api';
  import { matchSearch } from '../lib/search';

  // Detail pane of the Import page: what importing a set would make, read from the source's card list (nothing is
  // downloaded or written). The Import / Queue / Add-from-catalog / Open buttons come from the page (actions).
  let { source, set, options, lang, keepRarities, actions }: {
    source: any; set: any; options: any; lang: string; keepRarities: boolean; actions: Snippet<[any, boolean]>;
  } = $props();

  let pv = $state<any>(null);
  let err = $state('');
  let loading = $state(false);
  let tab = $state<'overview' | 'cards'>('overview');
  let only = $state(''); // rarity filter of the Cards tab
  let q = $state('');
  let sort = $state<'set' | 'price' | 'name'>('set');
  let big = $state<any>(null); // the card shown full size
  let ticket = 0;

  // Derived so a reloaded list row (a new object, same set) doesn't fetch again.
  const srcId = $derived(source.id);
  const code = $derived(set.code);
  $effect(() => {
    // Only what changes the result (not image width/format), so changing those doesn't reload the preview.
    const req = { src: srcId, code,
      opt: { includeVariants: options.includeVariants, rarityMap: $state.snapshot(options.rarityMap), lang } };
    const t = ++ticket;
    pv = null; err = ''; only = ''; q = ''; tab = 'overview'; loading = true;
    App.PreviewSet(req.src, req.code, req.opt as any)
      .then((r) => { if (t === ticket) pv = r; })
      .catch((e) => { if (t === ticket) err = errText(e); })
      .finally(() => { if (t === ticket) loading = false; });
  });

  const money = (v: number | null | undefined) => (v == null ? '—' : v >= 100 ? `$${Math.round(v).toLocaleString()}` : `$${v.toFixed(2)}`);
  const pct = (n: number, total: number) => (total ? Math.round((n / total) * 100) : 0);
  const tierColor = (g: string) => RARITY_COLORS[g] ?? RARITY_COLORS.Common;
  const longDate = (d: string) => {
    const t = d ? new Date(d + 'T00:00:00') : null;
    return t && !isNaN(+t) ? t.toLocaleDateString(undefined, { day: 'numeric', month: 'long', year: 'numeric' }) : d;
  };

  // Booster drawn as one pill per card; a mixed slot ("Epic 80% / Legendary 20%") shows its likeliest tier with the rest
  // in its tooltip.
  const packCards = $derived.by(() => {
    const out: { tier: string; tip: string; mixed: boolean }[] = [];
    for (const s of pv?.pack?.slots ?? []) {
      const w = Object.entries(s.weights ?? {}) as [string, number][];
      const total = w.reduce((a, [, v]) => a + v, 0);
      w.sort((a, b) => b[1] - a[1]);
      const tip = w.map(([k, v]) => `${k} ${pct(v, total)}%`).join(' · ');
      for (let i = 0; i < s.count; i++) out.push({ tier: w[0]?.[0] ?? 'Common', tip, mixed: w.length > 1 });
    }
    return out;
  });
  const avg = $derived(pv?.priced ? pv.value / pv.priced : 0);
  const cards = $derived.by(() => {
    const needle = q.trim().toLowerCase();
    const list = (pv?.all ?? []).filter((c: any) => (!only || c.rarity === only) &&
      (!needle || matchSearch(needle, c.name) || (c.number ?? '').toLowerCase().includes(needle)));
    if (sort === 'price') return [...list].sort((a, b) => (b.price ?? -1) - (a.price ?? -1));
    if (sort === 'name') return [...list].sort((a, b) => a.name.localeCompare(b.name));
    return list;
  });
  function showRarity(r: string) {
    only = r;
    tab = 'cards';
  }
</script>

<svelte:window onkeydown={(e) => { if (e.key === 'Escape') big = null; }} />

<section class="pane">
  <!-- Hero: the set's picture (logo or booster pack) in its own column; title, actions and compact stats beside it. -->
  <header class="hero">
    <div class="art">
      {#if pv?.logo || set.icon}<img class:mono={!pv?.logo && set.iconMono} src={pv?.logo || set.icon} alt="" />
      {:else}<span>{set.code.slice(0, 5).toUpperCase()}</span>{/if}
    </div>
    <div class="body">
      <div class="titlerow">
        <div class="title">
          <h1>{set.name}</h1>
          <div class="meta">
            <span class="code">{set.code.toUpperCase()}</span>
            {#if set.releasedAt}<span>{longDate(set.releasedAt)}</span>{/if}
            {#if set.group}<span>{set.group}</span>{/if}
            <span>{source.game} · {source.name}</span>
          </div>
        </div>
        <div class="cta">{@render actions(set, false)}</div>
      </div>
      {#if !err}
        <div class="mini" class:skeleton={loading}>
          <div><span class="label">Cards</span><b>{pv?.cards ?? '…'}</b>
            <span class="sub">{pv ? (pv.variants ? `${pv.variants} variants` : 'no variants') : ''}</span></div>
          <div><span class="label">Real price</span><b>{pv ? `${pct(pv.priced, pv.cards)}%` : '…'}</b>
            <span class="sub">{pv ? `${pv.priced} of ${pv.cards}` : ''}</span></div>
          <div><span class="label">Whole set</span><b>{pv ? money(pv.value) : '…'}</b>
            <span class="sub">{pv?.priced ? `~${money(avg)} a card` : ''}</span></div>
          <div><span class="label">Booster</span><b>{pv ? `${pv.pack.cards} cards` : '…'}</b>
            <span class="sub">{pv?.pack.foilChance ? `foil ${pv.pack.foilChance}%` : ''}</span></div>
        </div>
      {/if}
    </div>
  </header>

  {#if err}
    <div class="notice err">{err}</div>
  {:else}
    {#if loading}
      <p class="muted loadnote">Reading the card list from {source.name}…{source.id === 'tcgdex' ? ' Pokémon sets take a few seconds.' : ''}</p>
    {:else if pv}
      {#each pv.notes ?? [] as n}<div class="notice">{n}</div>{/each}

      <nav class="tabbar">
        <button class:on={tab === 'overview'} onclick={() => (tab = 'overview')}>Overview</button>
        <button class:on={tab === 'cards'} onclick={() => (tab = 'cards')}>Cards <span class="count">{pv.cards}</span></button>
      </nav>

      {#if tab === 'overview'}
        <div class="block">
          <div class="bhead"><h2>Rarities</h2>
            <span class="muted small">{keepRarities ? "Kept as the set's own rarities in game · right: the booster slots they fill"
              : "Every card becomes one of the game's 4 rarities"}</span></div>
          <div class="distbar">
            {#each pv.rarities as r}
              <span style="flex:{r.count}; background:{tierColor(r.game)}" title="{r.name || '(none)'}: {r.count} cards"></span>
            {/each}
          </div>
          <table class="rar">
            <tbody>
              {#each pv.rarities as r}
                <tr onclick={() => showRarity(r.name)} title="Show these cards">
                  <td class="dot"><i style="background:{tierColor(r.game)}"></i></td>
                  <td class="rname">{r.name || '(none)'}</td>
                  <td class="num">{r.count}</td>
                  <td class="tier" title={keepRarities ? `Stays "${r.name || '(none)'}" in game; in packs it fills the ${r.game} slots at their odds` : `Becomes ${r.game}`}>
                    <span class="tierchip" style="--c:{tierColor(r.game)}">{keepRarities ? `${r.game} slots` : r.game}</span></td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>

        <div class="block">
          <div class="bhead"><h2>Booster pack</h2><span class="muted small">{pv.pack.cards} cards, opened left to right</span></div>
          <div class="pack">
            {#each packCards as p}
              <span class="slot" class:mixed={p.mixed} style="--c:{tierColor(p.tier)}" title={p.tip}></span>
            {/each}
          </div>
          <div class="legend">
            {#each ['Common', 'Rare', 'Epic', 'Legendary'] as t}
              {#if packCards.some((p) => p.tier === t)}<span><i style="background:{tierColor(t)}"></i>{t}</span>{/if}
            {/each}
            {#if packCards.some((p) => p.mixed)}<span class="muted">striped = mixed slot (hover for odds)</span>{/if}
          </div>
        </div>

        {#if pv.top?.length}
          <div class="block">
            <div class="bhead"><h2>Most valuable</h2></div>
            <div class="topcards">
              {#each pv.top.slice(0, 6) as c}
                <button class="card" onclick={() => (big = c)} title="{c.name} · #{c.number}">
                  {#if c.thumb}<img src={c.thumb} alt={c.name} loading="lazy" />{/if}
                  <span class="price">{money(c.price)}</span>
                  <span class="cname">{c.name}</span>
                </button>
              {/each}
            </div>
          </div>
        {/if}
      {:else}
        <div class="cardbar">
          <input placeholder="Search cards by name or number…" bind:value={q} />
          <select bind:value={sort} title="Sort">
            <option value="set">Set order</option><option value="price">Price, high first</option><option value="name">Name</option>
          </select>
        </div>
        <div class="chips">
          <button class:active={!only} onclick={() => (only = '')}>All <span class="count">{pv.cards}</span></button>
          {#each pv.rarities as r}
            <button class:active={only === r.name} onclick={() => (only = only === r.name ? '' : r.name)}>
              <i style="background:{tierColor(r.game)}"></i>{r.name || '(none)'} <span class="count">{r.count}</span></button>
          {/each}
        </div>
        <div class="gallery">
          {#each cards as c}
            <button class="card" onclick={() => (big = c)} title="{c.name}{c.variant ? ` (${c.variant})` : ''} · #{c.number} · {c.rarity}">
              {#if c.thumb}<img src={c.thumb} alt={c.name} loading="lazy" />{:else}<span class="nothumb">{c.name}</span>{/if}
              {#if c.price != null}<span class="price">{money(c.price)}</span>{/if}
            </button>
          {:else}
            <p class="muted">No card matches.</p>
          {/each}
        </div>
      {/if}
    {/if}
  {/if}
</section>

{#if big}
  <div class="lightbox" role="presentation" onclick={() => (big = null)}>
    <div class="lbcard" role="presentation" onclick={(e) => e.stopPropagation()}>
      <img src={big.image || big.thumb} alt={big.name} />
      <div class="lbinfo">
        <h2>{big.name}</h2>
        {#if big.variant}<div class="muted">{big.variant}</div>{/if}
        <dl>
          <dt>Number</dt><dd>{big.number}</dd>
          <dt>Rarity</dt><dd>{big.rarity || '(none)'}</dd>
          <dt>In game</dt><dd><span class="tierchip" style="--c:{tierColor(big.game)}">{keepRarities ? `own rarity · ${big.game} slots` : big.game}</span></dd>
          <dt>Price</dt><dd>{money(big.price)}</dd>
        </dl>
        <button onclick={() => (big = null)}>Close</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .hero { display: flex; gap: 20px; align-items: stretch; }
  .art { width: 150px; height: 150px; flex: none; display: grid; place-items: center; background: var(--panel); border: 1px solid var(--line);
    border-radius: 12px; padding: 10px; }
  .art img { width: 130px; height: 130px; object-fit: contain; } /* fixed box: wide logos and tall pack pictures both fit */
  .art img.mono { filter: invert(1) opacity(0.9); width: 72px; height: 72px; }
  .art span { font-weight: 700; color: var(--muted); letter-spacing: 0.05em; }
  .body { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 12px; justify-content: space-between; }
  .titlerow { display: flex; gap: 16px; align-items: flex-start; }
  .title { flex: 1; min-width: 0; }
  .title h1 { font-size: 24px; line-height: 1.2; margin-bottom: 6px; }
  .meta { display: flex; flex-wrap: wrap; gap: 6px 14px; color: var(--muted); font-size: 13px; }
  .mini { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 8px; }
  .mini > div { background: var(--panel); border: 1px solid var(--line); border-radius: 8px; padding: 7px 10px; display: flex; flex-direction: column; min-width: 0; }
  .mini .sub { font-size: 11px; color: var(--muted); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .cta :global(button) { padding: 9px 22px; font-size: 15px; }

  .skeleton > div { animation: pulse 1.2s ease-in-out infinite; }
  @keyframes pulse { 50% { opacity: 0.55; } }
  .loadnote { margin: 0; }

  .tabbar { padding: 0; gap: 4px; }
  .count { font-size: 11px; color: var(--muted); background: var(--panel-2); border-radius: 8px; padding: 0 6px; margin-left: 4px; }

  .distbar { display: flex; height: 10px; border-radius: 5px; overflow: hidden; gap: 2px; }
  .distbar span { min-width: 3px; }
  .rar { width: 100%; border-collapse: collapse; font-size: 13px; }
  .rar tr { cursor: pointer; }
  .rar tr:hover { background: var(--panel-2); }
  .rar td { padding: 5px 6px; border-top: 1px solid var(--line); }
  .rar tr:first-child td { border-top: none; }
  .dot { width: 18px; }
  .dot i, .chips i, .legend i { display: inline-block; width: 9px; height: 9px; border-radius: 50%; }
  .num { text-align: right; width: 54px; font-variant-numeric: tabular-nums; }
  .tier { text-align: right; width: 150px; }
  .tierchip { font-size: 11px; padding: 1px 8px; border-radius: 10px; border: 1px solid var(--c); color: var(--c); white-space: nowrap; }

  .pack { display: flex; flex-wrap: wrap; gap: 5px; }
  .slot { width: 26px; height: 36px; border-radius: 4px; background: var(--c); opacity: 0.9; box-shadow: inset 0 0 0 1px rgba(0, 0, 0, 0.25); }
  .slot.mixed { background: repeating-linear-gradient(135deg, var(--c) 0 5px, rgba(255, 255, 255, 0.25) 5px 8px); }
  .legend { display: flex; flex-wrap: wrap; gap: 14px; font-size: 12px; }
  .legend span { display: inline-flex; align-items: center; gap: 6px; }

  .topcards { display: grid; grid-template-columns: repeat(auto-fill, minmax(118px, 1fr)); gap: 12px; }
  .card { position: relative; padding: 0; border: none; background: none; display: flex; flex-direction: column; gap: 4px; text-align: left; }
  .card img, .nothumb { width: 100%; aspect-ratio: 63 / 88; object-fit: cover; border-radius: 6px; background: var(--panel-2);
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.35); transition: transform 0.12s; }
  .card:hover img { transform: translateY(-2px); }
  .nothumb { display: block; font-size: 10px; padding: 6px; overflow: hidden; }
  .price { position: absolute; right: 5px; top: 5px; font-size: 11px; font-weight: 600; background: rgba(10, 12, 16, 0.85);
    border-radius: 4px; padding: 1px 5px; }
  .cname { font-size: 12px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }

  .cardbar { display: flex; gap: 8px; }
  .cardbar input { flex: 1; }
  .chips { display: flex; flex-wrap: wrap; gap: 6px; }
  .chips button { display: inline-flex; align-items: center; gap: 6px; border-radius: 14px; padding: 3px 10px; font-size: 12px; }
  .chips button.active { border-color: var(--accent); background: var(--panel); }
  .gallery { display: grid; grid-template-columns: repeat(auto-fill, minmax(104px, 1fr)); gap: 10px; }

  .lightbox { position: fixed; inset: 0; z-index: 200; background: rgba(0, 0, 0, 0.75); display: grid; place-items: center; }
  .lbcard { display: flex; gap: 22px; background: var(--panel); border: 1px solid var(--line); border-radius: 12px; padding: 18px; max-width: 760px; }
  .lbcard img { height: min(70vh, 560px); aspect-ratio: 63 / 88; object-fit: contain; border-radius: 10px; }
  .lbinfo { display: flex; flex-direction: column; gap: 8px; min-width: 220px; }
  .lbinfo h2 { font-size: 20px; }
  dl { display: grid; grid-template-columns: auto 1fr; gap: 6px 14px; margin: 6px 0; }
  dt { color: var(--muted); }
  dd { margin: 0; }
  .lbinfo button { margin-top: auto; align-self: flex-start; }
</style>
