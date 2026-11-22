<script lang="ts" module>
  type Page = 'card' | 'look' | 'price' | 'play';
  let rememberedPage: Page = 'look';
</script>

<script lang="ts">
  // The full card editor: one card on a whole page, laid out like the Set tab (a side list of pages, a .pane per page).
  // It edits the same card object as the card panel, so Save, drafts and the panel all see the same changes. Opened from
  // the card panel (⤢ Open editor); ‹ › step through the cards the Cards tab shows; Esc or Close returns to the grid.
  import { projectFile, rarityName, rarityColor, BORDERS } from '../../lib/api';
  import { loadLooks, noLooks, shaderFor } from '../../lib/cardLooks';
  import { cardVersions, versionsWithArt, versionLabel, ALL_VERSIONS } from '../../lib/cardVersions';
  import { fullViewOpen } from '../../lib/maximize';
  import LookEditor from './LookEditor.svelte';
  import CardInfoFields from './CardInfoFields.svelte';
  import CardPriceFields from './CardPriceFields.svelte';
  import CardPlayFields from './CardPlayFields.svelte';
  import CardShaderExtras from './CardShaderExtras.svelte';

  let { project, card, imgBust, notify, cards, onpick, onclose }: {
    project: any; card: any; imgBust: number; notify: (t: string, k?: string) => void;
    /** The cards the Cards tab shows, in its order (‹ › step through them). */
    cards: any[];
    onpick: (id: string) => void; onclose: () => void;
  } = $props();

  let page = $state<Page>(rememberedPage);
  function open(p: Page) { page = rememberedPage = p; }

  const index = $derived(cards.findIndex((c) => c.id === card.id));
  const step = (d: number) => { const next = cards[index + d]; if (next) onpick(next.id); };
  const meta = $derived(project.meta?.cards?.[card.id]);
  const versionsOn = $derived(cardVersions(project.set, card));

  // The shaders this card's switched-on versions use, each with its versions (Shader extras lists their values).
  let looks = $state(noLooks());
  $effect(() => { loadLooks().then((l) => (looks = l)); });
  const usedShaders = $derived.by(() => {
    const byId = new Map<string, { shader: any; versions: string[] }>();
    for (const k of ALL_VERSIONS.filter((k) => versionsOn.has(k))) {
      const sh = shaderFor(looks, project.set, card, k);
      if (!sh) continue;
      if (!byId.has(sh.id)) byId.set(sh.id, { shader: sh, versions: [] });
      byId.get(sh.id)!.versions.push(versionLabel(k));
    }
    return [...byId.values()];
  });
  // Parameter names any shader of the card uses, switched-off versions too: own values for those aren't leftovers (a
  // version switched off for now keeps its values for when it's back).
  const usedParams = $derived(new Set(ALL_VERSIONS.flatMap((k) => (shaderFor(looks, project.set, card, k)?.params ?? []).map((p: any) => p.name))));

  const PAGES = $derived<[Page, string, string][]>([
    ['card', 'Card', 'Rarity, number, packs, text'],
    ['look', 'Look', `${versionsOn.size} of 12 versions${versionsWithArt(card).size ? ` · ${versionsWithArt(card).size} own pictures` : ''}`],
    ['price', 'Price', meta?.locked ? 'Locked' : 'Base, multipliers, exact prices'],
    ['play', 'Play', 'Lanes, element, effect']
  ]);

  function keys(e: KeyboardEvent) {
    const t = e.target as HTMLElement;
    if (fullViewOpen(e) || t.closest('input, textarea, select, [contenteditable], .ddpanel')) return;
    if (e.key === 'Escape') { e.stopPropagation(); onclose(); }
    else if (e.key === 'ArrowLeft' && e.altKey) step(-1);
    else if (e.key === 'ArrowRight' && e.altKey) step(1);
  }
</script>

<svelte:window onkeydown={keys} />

<div class="cardeditor settab setpages">
  <nav class="side sidelist" aria-label="Card pages">
    <div class="list">
      {#each PAGES as [id, label, sub]}
        <button class="item" class:active={page === id} onclick={() => open(id)}>
          <span class="info"><span class="name">{label}</span><span class="sub">{sub}</span></span></button>
      {/each}
    </div>
    <div class="sidefoot">
      <button onclick={onclose} title="Back to the cards (Esc)">Close editor</button>
    </div>
  </nav>

  <div class="scroll">
    <div class="pane">
      <header class="pane-hero">
        <div class="art">{#if card.image}<img src={projectFile(project.id, card.image, imgBust)} alt="" />{/if}</div>
        <div class="grow titlebox">
          <div class="scope"><span class="badge">Editing one card</span>
            <span class="muted small">Changes here apply only to this card of {project.set.name}. Defaults for every card: Set tab.</span></div>
          <input class="title" bind:value={card.name} aria-label="Card name" placeholder="Card name" />
          <div class="meta">
            <span><span class="dot" style="background:{rarityColor(project.set, card.rarity)}"></span> {rarityName(project.set, card.rarity)}</span>
            {#if card.number}<span>#{card.number}</span>{/if}
            <span>ID <code>{card.id}</code></span>
            <span>{index + 1} of {cards.length}</span>
          </div>
        </div>
        <div class="heroactions">
          <button disabled={index <= 0} onclick={() => step(-1)} title="Previous card (Alt+←)">‹ Previous</button>
          <button disabled={index < 0 || index >= cards.length - 1} onclick={() => step(1)} title="Next card (Alt+→)">Next ›</button>
          <button onclick={onclose} title="Back to the cards (Esc)">Close</button>
        </div>
      </header>

      {#if page === 'card'}
        <div class="page-cols">
          <div class="block"><div class="bhead"><h2>Details</h2></div><CardInfoFields {project} {card} {notify} /></div>
          <aside class="page-side">
            <div class="notice info howto"><b>Card details</b>
              <ul>
                <li>The rarity decides which pack slots can give the card; the number orders it in the binder's Set order.</li>
                <li>Untick a pack that draws from the whole set and it gets a list of every other card.</li>
              </ul>
            </div>
          </aside>
        </div>
      {:else if page === 'look'}
        <LookEditor {project} {card} {imgBust} {notify} />
        <div class="block"><div class="bhead"><h2>Shader extras</h2><span class="muted small">for the whole card, not one version: a mask picture and own shader values</span></div>
          <CardShaderExtras {project} {card} {imgBust} {notify} used={usedShaders} known={usedParams} />
        </div>
      {:else if page === 'price'}
        <div class="page-cols">
          <div class="block"><div class="bhead"><h2>Price</h2><span class="muted small">market price = base × border × foil</span></div><CardPriceFields {project} {card} /></div>
          <aside class="page-side">
            <div class="notice info howto"><b>How card prices work</b>
              <ul>
                <li>Without its own, the card uses the set's multipliers ({BORDERS.length} borders × foil, Set → Prices).</li>
                <li>An exact price replaces the sum for one version. The game's daily changes apply on top.</li>
                <li>Locked cards are skipped by Gamify and price refreshes.</li>
              </ul>
            </div>
          </aside>
        </div>
      {:else}
        <div class="page-cols">
          <div class="block"><div class="bhead"><h2>Play</h2><span class="muted small">the card in the play-table game</span></div><CardPlayFields {project} {card} /></div>
        </div>
      {/if}
    </div>
  </div>
</div>

<style>
  /* Over the Cards tab's grid and panel (CardsTab's .wrap is the positioned parent). */
  .cardeditor { position: absolute; inset: 0; z-index: 5; background: var(--bg); }
  .settab { display: flex; min-height: 0; }
  .side { width: 240px; flex: none; display: flex; flex-direction: column; min-height: 0; border-right: 1px solid var(--line); }
  .scroll { flex: 1; min-width: 0; overflow: auto; }
  @media (max-width: 1150px) {
    .side { width: 170px; }
    .side .sub { display: none; }
  }
  .scope { display: flex; flex-wrap: wrap; align-items: center; gap: 4px 8px; margin-bottom: 4px; }
  .scope .badge { color: var(--accent); border-color: var(--accent-2); }
  .dot { width: 8px; height: 8px; border-radius: 50%; display: inline-block; }
</style>
