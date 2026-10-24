<script lang="ts">
  import { App, projectFile, setRarities, rarityName, ELEMENTS, LANES, money, errText } from '../../lib/api';

  let { project, cards, notify, ondelete, imgBust = 0 }: {
    project: any; cards: any[]; notify: (t: string, k?: string) => void; ondelete: () => void; imgBust?: number;
  } = $props();

  let rotating = $state(false);
  async function rotate(turns: number, onlyLandscape: boolean) {
    rotating = true;
    try {
      const turned = await App.RotateCardImages(project.id, cards.map((c) => c.image).filter(Boolean), turns, onlyLandscape);
      const n = Object.keys(turned).length;
      for (const c of cards) if (turned[c.image]) c.image = turned[c.image]; // new files: the game sees them once the set is saved
      if (n) notify(`Rotated ${n} image(s) — Save to put ${n === 1 ? 'it' : 'them'} in the game.`, 'ok');
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
  <div class="stack" aria-hidden="true">
    {#each cards.slice(0, 5) as c, i (c.id)}
      <span class="sc" style="--i:{i}; --n:{Math.min(5, cards.length)}">
        {#if c.image}<img src={projectFile(project.id, c.image, imgBust)} alt="" />{/if}
      </span>
    {/each}
  </div>
  <div class="head">
    <h2>{cards.length} cards selected</h2>
    <div class="meta"><span>Base value <b>{money(total)}</b></span><span>{cards.length - unlocked.length} locked</span></div>
    <span class="muted small">Price changes skip locked cards.</span>
  </div>

  <section>
    <h4>Rarity <span class="muted small">now: {common((c) => rarityName(project.set, c.rarity))}</span></h4>
    <div class="row">
      <select class="grow" bind:value={rarity}><option value="">Choose…</option>{#each setRarities(project.set) as r}<option value={r.id}>{r.name}</option>{/each}</select>
      <button disabled={!rarity} onclick={() => apply(`Rarity ${rarityName(project.set, rarity)}`, (c) => (c.rarity = rarity))}>Apply</button>
    </div>
  </section>

  <section>
    <h4>Price <span class="muted small">base now: {common((c) => c.price.base) === 'mixed' ? 'mixed' : money(cards[0].price.base)}</span></h4>
    <div class="pgrid">
      <span class="lbl">Multiply base</span>
      <input type="number" step="0.05" min="0" bind:value={factor} />
      <button onclick={() => apply(`Price ×${factor}`, (c) => (c.price.base = Math.round(c.price.base * factor * 100) / 100), unlocked)}>Apply</button>
      <span class="lbl">Set base to</span>
      <input type="number" step="0.01" min="0" placeholder="e.g. 0.25" bind:value={setBase} />
      <button disabled={setBase === null} onclick={() => apply(`Base ${money(setBase)}`, (c) => (c.price.base = setBase!), unlocked)}>Apply</button>
      <span class="lbl">Foil multiplier</span>
      <input type="number" step="0.1" min="1" placeholder="set default" bind:value={foil} />
      <button onclick={() => apply(foil ? `Foil ×${foil}` : 'Foil = set default', (c) => (c.price.foilMultiplier = foil || undefined), unlocked)}>Apply</button>
    </div>
    <div class="row">
      <button class="small" onclick={() => setLock(true)}>🔒 Lock prices</button>
      <button class="small" onclick={() => setLock(false)}>Unlock</button>
    </div>
  </section>

  <section>
    <h4>Play table <span class="muted small">lanes now: {JSON.stringify(common((c) => c.play.laneAttack))}</span></h4>
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
      <h4>Pack pool</h4>
      <div class="row">
        <select class="grow" bind:value={packId}><option value="">Pack…</option>{#each project.set.packs as p}<option value={p.id}>{p.name}</option>{/each}</select>
        <button disabled={!packId} onclick={addToPack}>Add to pack's list</button>
      </div>
      <div class="muted small">A pack with an empty card list draws from the whole set.</div>
    </section>
  {/if}

  <section>
    <h4>Images</h4>
    <div class="row">
      <button class="small" disabled={rotating} onclick={() => rotate(-1, false)}>⟲ Rotate left</button>
      <button class="small" disabled={rotating} onclick={() => rotate(1, false)}>⟳ Rotate right</button>
    </div>
    <button class="small" disabled={rotating} onclick={() => rotate(1, true)} title="Turns only the images that are wider than tall">Turn landscape cards upright</button>
  </section>

  <div><button class="danger" onclick={ondelete}>Delete {cards.length} cards</button></div>
</div>

<style>
  .bulk { padding: 14px 16px 20px; display: flex; flex-direction: column; gap: 10px; }
  /* A fanned stack of the first selected cards. */
  .stack { position: relative; height: 170px; border-radius: 12px; border: 1px solid var(--line);
    background: radial-gradient(ellipse at 50% 35%, #2b3040, #15171d 75%); overflow: hidden; }
  .sc { position: absolute; left: 50%; top: 22px; width: 92px; height: 128px; border-radius: 6px; background: var(--panel-2); overflow: hidden;
    box-shadow: 0 6px 18px rgba(0, 0, 0, 0.55);
    transform: translateX(calc(-50% + (var(--i) - (var(--n) - 1) / 2) * 34px)) rotate(calc((var(--i) - (var(--n) - 1) / 2) * 7deg)); }
  .sc img { width: 100%; height: 100%; object-fit: cover; }
  .head { display: flex; flex-direction: column; gap: 3px; }
  .head h2 { font-size: 18px; }
  .meta { display: flex; gap: 14px; font-size: 13px; color: var(--muted); }
  .meta b { color: var(--text); }
  section { display: flex; flex-direction: column; gap: 8px; background: var(--bg); border: 1px solid var(--line); border-radius: 10px; padding: 12px; }
  h4 { margin: 0; font-size: 13px; font-weight: 600; display: flex; align-items: baseline; gap: 8px; }
  h4 .muted { font-weight: 400; }
  .pgrid { display: grid; grid-template-columns: auto 1fr auto; gap: 6px 8px; align-items: center; }
  .pgrid .lbl { font-size: 12px; color: var(--muted); }
  .pgrid input { min-width: 0; }
  .lanes { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 6px; }
  .lanes input, .row > select { min-width: 0; width: 100%; }
  .row > select { flex: 1 1 0; }
</style>
