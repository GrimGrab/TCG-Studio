<script lang="ts">
  import { App, setRarities, rarityName, ELEMENTS, LANES, money, errText } from '../../lib/api';

  let { project, cards, notify, onimages, ondelete }: {
    project: any; cards: any[]; notify: (t: string, k?: string) => void; onimages: (what: string) => Promise<void>; ondelete: () => void;
  } = $props();

  let rotating = $state(false);
  async function rotate(turns: number, onlyLandscape: boolean) {
    rotating = true;
    try {
      const n = await App.RotateCardImages(project.id, cards.map((c) => c.image).filter(Boolean), turns, onlyLandscape);
      if (n) await onimages(`Rotated ${n} image(s).`);
      else notify(onlyLandscape ? 'No landscape images among the selected cards.' : 'Nothing to rotate.', 'ok');
    } catch (e) { notify(errText(e), 'error'); }
    rotating = false;
  }

  let rarity = $state('');
  let element = $state('');
  let factor = $state(1.25);
  let setBase = $state<number | null>(null);
  let foil = $state<number | null>(null);
  let lanes = $state<(number | null)[]>([null, null, null, null]);
  let packId = $state('');

  /** Shows a field's value when every selected card agrees, otherwise "mixed". */
  function common<T>(get: (c: any) => T): T | 'mixed' {
    const v = get(cards[0]);
    return cards.every((c) => JSON.stringify(get(c)) === JSON.stringify(v)) ? v : 'mixed';
  }

  const locked = (c: any) => !!project.meta?.cards?.[c.id]?.locked;
  const unlocked = $derived(cards.filter((c) => !locked(c)));
  const total = $derived(cards.reduce((s, c) => s + c.price.base, 0));

  function apply(label: string, fn: (c: any) => void, list = cards) {
    list.forEach(fn);
    notify(`${label} → ${list.length} card(s)`, 'ok');
  }

  function setLock(v: boolean) {
    project.meta.cards ??= {};
    for (const c of cards) project.meta.cards[c.id] = { ...(project.meta.cards[c.id] ?? {}), locked: v };
    notify(v ? `Locked ${cards.length} card(s)` : `Unlocked ${cards.length} card(s)`, 'ok');
  }

  function addToPack() {
    const p = project.set.packs.find((p: any) => p.id === packId);
    if (!p) return;
    const ids = new Set([...(p.cards ?? []), ...cards.map((c) => c.id)]);
    p.cards = project.set.cards.map((c: any) => c.id).filter((id: string) => ids.has(id));
    notify(`Pack "${p.name}" now draws from ${p.cards.length} specific cards`, 'ok');
  }
</script>

<div class="bulk">
  <h3>{cards.length} cards selected</h3>
  <div class="muted small">Total base value {money(total)} · {cards.length - unlocked.length} locked (price changes skip locked cards)</div>

  <section>
    <b>Rarity</b> <span class="muted small">now: {common((c) => rarityName(project.set, c.rarity))}</span>
    <div class="row">
      <select class="grow" bind:value={rarity}><option value="">Choose…</option>{#each setRarities(project.set) as r}<option value={r.id}>{r.name}</option>{/each}</select>
      <button disabled={!rarity} onclick={() => apply(`Rarity ${rarityName(project.set, rarity)}`, (c) => (c.rarity = rarity))}>Apply</button>
    </div>
  </section>

  <section>
    <b>Price</b> <span class="muted small">base now: {common((c) => c.price.base) === 'mixed' ? 'mixed' : money(cards[0].price.base)}</span>
    <div class="row">
      <input type="number" step="0.05" min="0" bind:value={factor} style="width:80px" />
      <button onclick={() => apply(`Price ×${factor}`, (c) => (c.price.base = Math.round(c.price.base * factor * 100) / 100), unlocked)}>Multiply base</button>
    </div>
    <div class="row">
      <input type="number" step="0.01" min="0" placeholder="e.g. 0.25" bind:value={setBase} style="width:80px" />
      <button disabled={setBase === null} onclick={() => apply(`Base ${money(setBase)}`, (c) => (c.price.base = setBase!), unlocked)}>Set base</button>
    </div>
    <div class="row">
      <input type="number" step="0.1" min="1" placeholder="foil ×" bind:value={foil} style="width:80px" />
      <button onclick={() => apply(foil ? `Foil ×${foil}` : 'Foil = set default', (c) => (c.price.foilMultiplier = foil || undefined), unlocked)}>Set foil multiplier</button>
    </div>
    <div class="row">
      <button onclick={() => setLock(true)}>Lock prices</button>
      <button onclick={() => setLock(false)}>Unlock</button>
    </div>
  </section>

  <section>
    <b>Play table</b> <span class="muted small">lanes now: {JSON.stringify(common((c) => c.play.laneAttack))}</span>
    <div class="lanes">
      {#each LANES as l, i}<label class="field">{l}<input type="number" min="0" placeholder="keep" bind:value={lanes[i]} /></label>{/each}
    </div>
    <button onclick={() => apply('Lane attack', (c) => lanes.forEach((v, i) => { if (v !== null && v !== undefined && !isNaN(v as number)) c.play.laneAttack[i] = v; }))}>Apply lanes (blank = keep)</button>
    <div class="row">
      <select class="grow" bind:value={element}><option value="">Element…</option>{#each ELEMENTS as e}<option>{e}</option>{/each}</select>
      <button disabled={!element} onclick={() => apply(`Element ${element}`, (c) => (c.play.element = element))}>Apply</button>
    </div>
  </section>

  {#if project.set.packs.length}
    <section>
      <b>Pack pool</b>
      <div class="row">
        <select class="grow" bind:value={packId}><option value="">Pack…</option>{#each project.set.packs as p}<option value={p.id}>{p.name}</option>{/each}</select>
        <button disabled={!packId} onclick={addToPack}>Add to pack's card list</button>
      </div>
      <div class="muted small">A pack with an empty card list draws from the whole set.</div>
    </section>
  {/if}

  <section>
    <b>Image</b>
    <div class="row">
      <button disabled={rotating} onclick={() => rotate(-1, false)}>⟲ Rotate left</button>
      <button disabled={rotating} onclick={() => rotate(1, false)}>⟳ Rotate right</button>
    </div>
    <button disabled={rotating} onclick={() => rotate(1, true)} title="Turns only the images that are wider than tall">Turn landscape cards upright</button>
  </section>

  <section>
    <button class="danger" onclick={ondelete}>Delete {cards.length} cards</button>
  </section>
</div>

<style>
  .bulk { padding: 14px; display: flex; flex-direction: column; gap: 12px; }
  section { display: flex; flex-direction: column; gap: 6px; padding-top: 10px; border-top: 1px solid var(--line); }
  .lanes { display: grid; grid-template-columns: repeat(4, 1fr); gap: 6px; }
  .small { font-size: 12px; }
</style>
