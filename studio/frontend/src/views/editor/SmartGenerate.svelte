<script lang="ts">
  // Smart generate dialog: pick which product photos a pack and its box are built from (pre-selected like an import does:
  // pack n ↔ n-th booster), or card art instead, and check what is cut out of them, dragging any corner: Pack view = the
  // pack (crimps included) in the pack photo; Box view = the front panel (front & back of the box) and the lid (top and
  // sides) in the box photo. Returns the sources for applySmartArt.
  import { onMount, untrack } from 'svelte';
  import { App, errText, projectFile } from '../../lib/api';
  import { loadImage } from '../../lib/accessoryArt';
  import type { SmartSources } from '../../lib/smartArt';

  let { project, pack, packIndex, hasBox, ongenerate, oncancel }: {
    project: any; pack: any; packIndex: number; hasBox: boolean;
    ongenerate: (src: SmartSources) => void; oncancel: () => void;
  } = $props();

  type Product = { id: number; name: string; kind: string; thumb: string; image: string };
  let games = $state<{ category: number; name: string }[]>([]);
  let category = $state(0);
  let groups = $state<{ id: number; name: string; code: string; released: string }[]>([]);
  let group = $state(0);
  let filter = $state('');
  let products = $state<Product[]>([]);
  let showAll = $state(false);
  let packPick = $state<Product | null>(null);   // null = card art
  let boxPick = $state<Product | null>(null);    // null = no box photo
  let loading = $state('');
  let error = $state('');
  let busy = $state(false);

  // Photos and outlines being edited (photo pixels, tl tr br bl).
  let tab = $state<'pack' | 'box'>('pack');
  let packImg = $state.raw<HTMLImageElement | null>(null);
  let packQuad = $state<number[]>([]);
  let boxImg = $state.raw<HTMLImageElement | null>(null);
  let front = $state<number[]>([]);
  let lid = $state<number[]>([]);
  let detected: { front: number[]; lid: number[] } | null = null;
  let view = $state<HTMLCanvasElement>();

  const whole = (img: HTMLImageElement) => [0, 0, img.width, 0, img.width, img.height, 0, img.height];

  async function loadSets(cat: number) {
    loading = 'Loading sets…'; error = '';
    try {
      const r = await App.ProductPhotoSets(project.id, cat);
      games = r.games; category = r.category; groups = r.groups ?? []; group = r.match;
      if (!r.match) error = "Couldn't find this set on TCGplayer by itself — pick it from the list (or use card art).";
    } catch (e) { error = errText(e); }
    loading = '';
    await loadProducts(true);
  }

  async function loadProducts(pickDefaults = false) {
    products = [];
    if (!group) { await choose(null, null); return; }
    loading = 'Loading products…'; error = '';
    try {
      products = ((await App.ProductPhotos(category, group)) ?? []) as Product[];
      if (pickDefaults || !packPick) {
        const p = await App.PickPackAndBox(products as any, packIndex);
        await choose((p.pack as Product) ?? null, hasBox ? ((p.box as Product) ?? null) : null);
      }
    } catch (e) { error = errText(e); }
    loading = '';
  }

  async function choose(p: Product | null, b: Product | null) {
    if (p?.id !== packPick?.id || (p && !packImg)) {
      packPick = p;
      packImg = null; packQuad = [];
      if (p) {
        loading = 'Reading the pack photo…';
        try {
          const img = await loadImage(projectFile(project.id, await App.UseProductPhoto(project.id, p as any)));
          if (packPick?.id === p.id) { packImg = img; packQuad = whole(img); } // the whole (trimmed) photo is the pack
        } catch (e) { error = errText(e); }
        loading = '';
      }
    }
    if (b?.id === boxPick?.id && (boxImg || !b)) return;
    boxPick = b;
    boxImg = null; front = []; lid = []; detected = null;
    if (!b) return;
    loading = 'Reading the box photo…';
    try {
      const rel = await App.UseProductPhoto(project.id, b as any);
      const f: any = await App.DetectBoxFaces(project.id, rel);
      const img = await loadImage(projectFile(project.id, rel));
      if (boxPick?.id !== b.id) return; // another box was picked meanwhile
      boxImg = img;
      front = [...f.front]; lid = [...f.lid];
      detected = { front: [...f.front], lid: [...f.lid] };
    } catch (e) { error = errText(e); }
    loading = '';
  }

  function reset() {
    if (tab === 'pack') { if (packImg) packQuad = whole(packImg); }
    else if (detected) { front = [...detected.front]; lid = [...detected.lid]; }
  }

  async function generate() {
    busy = true; error = '';
    try {
      const src = (await App.SmartArtSources(project.id, pack.id, { manual: true, pack: packPick, box: boxPick } as any)) as unknown as SmartSources;
      if (src.box && front.length === 8) src.box = { ...src.box, front: [...front], lid: [...lid] };
      if (src.packPhoto && packQuad.length === 8) src.packQuad = [...packQuad];
      ongenerate(src);
    } catch (e) { error = errText(e); busy = false; }
  }

  onMount(() => { loadSets(0); });

  const shownGroups = $derived.by(() => {
    const f = filter.trim().toLowerCase();
    const gs = f ? groups.filter((g) => g.name.toLowerCase().includes(f) || g.code.toLowerCase() === f) : groups;
    const cur = groups.find((g) => g.id === group);
    return cur && !gs.includes(cur) ? [cur, ...gs] : gs;
  });
  const packs = $derived(products.filter((p) => showAll || p.kind === 'pack'));
  const boxes = $derived(products.filter((p) => showAll || p.kind === 'box'));

  // ---------------------------------------------------------------- outline editor

  type Key = 'pack' | 'front' | 'lid';
  const COLORS: Record<Key, string> = { pack: '#f5b942', front: '#2fd65a', lid: '#3d8bff' };
  const LABELS: Record<Key, string> = { pack: 'Pack (crimps included)', front: 'Front panel → front & back', lid: 'Lid → top & sides' };
  const shownImg = $derived(tab === 'pack' ? packImg : boxImg);
  const keys = (): Key[] => (tab === 'pack' ? ['pack'] : ['lid', 'front']);
  const get = (q: Key) => (q === 'pack' ? packQuad : q === 'front' ? front : lid);
  function set(q: Key, v: number[]) { if (q === 'pack') packQuad = v; else if (q === 'front') front = v; else lid = v; }

  let k = 1, ox = 0, oy = 0;              // photo → canvas
  let drag: { q: Key; i: number } | null = null;

  function draw() {
    const c = view, img = shownImg;
    if (!c || !img) return;
    const r = c.getBoundingClientRect();
    c.width = Math.max(1, Math.round(r.width * devicePixelRatio)); c.height = Math.max(1, Math.round(r.height * devicePixelRatio));
    const ctx = c.getContext('2d')!;
    ctx.clearRect(0, 0, c.width, c.height);
    k = Math.min(c.width / img.width, c.height / img.height) * 0.94; // a margin so corners at the photo's edge can be grabbed
    ox = (c.width - img.width * k) / 2; oy = (c.height - img.height * k) / 2;
    ctx.drawImage(img, ox, oy, img.width * k, img.height * k);
    for (const key of keys()) {
      const q = get(key), color = COLORS[key];
      if (q.length !== 8) continue;
      ctx.lineWidth = 2.5 * devicePixelRatio;
      ctx.strokeStyle = color;
      ctx.beginPath();
      for (let i = 0; i < 4; i++) ctx[i ? 'lineTo' : 'moveTo'](ox + q[i * 2] * k, oy + q[i * 2 + 1] * k);
      ctx.closePath();
      ctx.stroke();
      ctx.fillStyle = color;
      for (let i = 0; i < 4; i++) {
        ctx.beginPath();
        ctx.arc(ox + q[i * 2] * k, oy + q[i * 2 + 1] * k, 7 * devicePixelRatio, 0, Math.PI * 2);
        ctx.fill();
      }
      ctx.font = `bold ${13 * devicePixelRatio}px sans-serif`;
      const x = ox + q[0] * k + 10 * devicePixelRatio, y = oy + q[1] * k + 20 * devicePixelRatio;
      ctx.lineWidth = 3 * devicePixelRatio; ctx.strokeStyle = 'rgba(0,0,0,0.7)';
      ctx.strokeText(LABELS[key], x, y);
      ctx.fillText(LABELS[key], x, y);
    }
  }

  $effect(() => { tab; packQuad; front; lid; shownImg; view; untrack(draw); });

  function at(e: PointerEvent) {
    const r = view!.getBoundingClientRect();
    return [((e.clientX - r.left) * devicePixelRatio - ox) / k, ((e.clientY - r.top) * devicePixelRatio - oy) / k];
  }

  function down(e: PointerEvent) {
    if (!shownImg) return;
    const [x, y] = at(e);
    let best = Infinity;
    drag = null;
    for (const q of keys()) {
      const pts = get(q);
      for (let i = 0; i < 4; i++) {
        const d = Math.hypot(pts[i * 2] - x, pts[i * 2 + 1] - y);
        if (d < best) { best = d; drag = { q, i }; }
      }
    }
    if (best * k > 30 * devicePixelRatio) drag = null; // not near a corner
    if (drag) view!.setPointerCapture(e.pointerId);
  }

  function move(e: PointerEvent) {
    const img = shownImg;
    if (!drag || !img) return;
    const [x, y] = at(e);
    const pts = [...get(drag.q)];
    pts[drag.i * 2] = Math.max(0, Math.min(img.width, x));
    pts[drag.i * 2 + 1] = Math.max(0, Math.min(img.height, y));
    set(drag.q, pts);
  }

  const key = (e: KeyboardEvent) => { if (e.key === 'Escape' && !busy) oncancel(); };
</script>

<svelte:window onkeydown={key} onresize={draw} />

<div class="backdrop" role="presentation">
  <div class="dialog">
    <div class="row head">
      <h3 class="grow">Smart generate — {pack.name || pack.id}</h3>
      <button onclick={oncancel} disabled={busy}>Cancel</button>
      <button class="primary" onclick={generate} disabled={busy || !!loading}>{busy ? 'Generating…' : 'Generate'}</button>
    </div>
    <div class="row pick">
      <label class="field">Game
        <select value={category} onchange={(e) => loadSets(+e.currentTarget.value)}>
          {#each games as g}<option value={g.category}>{g.name}</option>{/each}
        </select>
      </label>
      <label class="field">Search sets<input placeholder="Name or code" bind:value={filter} /></label>
      <label class="field grow">Set on TCGplayer
        <select bind:value={group} onchange={() => loadProducts(true)}>
          <option value={0}>— none: use card art —</option>
          {#each shownGroups as g}<option value={g.id}>{g.name}{g.code ? ` (${g.code})` : ''}{g.released ? ` · ${g.released.slice(0, 4)}` : ''}</option>{/each}
        </select>
      </label>
      <label class="check small"><input type="checkbox" bind:checked={showAll} /> All products</label>
    </div>
    {#if loading}<p class="small muted">{loading}</p>{/if}
    {#if error}<p class="small err">{error}</p>{/if}

    <div class="body">
      <div class="lists">
        <h4>Pack photo</h4>
        <div class="grid">
          <button class="card" class:on={!packPick} onclick={() => { tab = 'pack'; choose(null, boxPick); }}><span class="none">Card art</span><span class="small">The set's best card</span></button>
          {#each packs as p (p.id)}
            <button class="card" class:on={packPick?.id === p.id} onclick={() => { tab = 'pack'; choose(p, boxPick); }} title={p.name}>
              <img src={p.thumb} alt="" loading="lazy" /><span class="small">{p.name}</span></button>
          {/each}
        </div>
        {#if hasBox}
          <h4>Box photo</h4>
          <div class="grid">
            <button class="card" class:on={!boxPick} onclick={() => { tab = 'box'; choose(packPick, null); }}><span class="none">No photo</span><span class="small">Card art + packs</span></button>
            {#each boxes as p (p.id)}
              <button class="card" class:on={boxPick?.id === p.id} onclick={() => { tab = 'box'; choose(packPick, p); }} title={p.name}>
                <img src={p.thumb} alt="" loading="lazy" /><span class="small">{p.name}</span></button>
            {/each}
          </div>
        {/if}
      </div>
      <div class="faces">
        <div class="row tabs">
          <button class:on={tab === 'pack'} onclick={() => (tab = 'pack')}>Pack</button>
          {#if hasBox}<button class:on={tab === 'box'} onclick={() => (tab = 'box')}>Box</button>{/if}
          <div class="grow"></div>
          {#if shownImg}<button class="small" onclick={reset}>{tab === 'pack' ? 'Whole photo' : 'Reset to detected'}</button>{/if}
        </div>
        {#if shownImg}
          <canvas bind:this={view} onpointerdown={down} onpointermove={move} onpointerup={() => (drag = null)}></canvas>
          <p class="small muted">
            {#if tab === 'pack'}Drag the corners onto the pack's corners (crimps included); it is straightened onto the pack's front.
            {:else}Drag the corners: <b class="g">green</b> = the box's front panel (set name), <b class="b">blue</b> = the lid art.{/if}
          </p>
        {:else if tab === 'pack'}
          <p class="muted small">{packPick ? 'Reading the pack photo…' : "Card art: the pack's front is the set's most valuable card."}</p>
        {:else}
          <p class="muted small">{boxPick ? 'Reading the box photo…' : 'No box photo: the box is made from card art and the pack photo.'}</p>
        {/if}
      </div>
    </div>
    <p class="small muted">Photos from TCGplayer's catalog (via tcgcsv.com). Every part ends up as a layer you can still adjust in the 3D editor.</p>
  </div>
</div>

<style>
  .backdrop { position: fixed; inset: 0; background: rgba(0, 0, 0, 0.6); display: flex; align-items: center; justify-content: center; z-index: 100; }
  .dialog { width: min(1280px, 96vw); height: min(860px, 92vh); background: var(--panel); border: 1px solid var(--line); border-radius: var(--radius);
    padding: 12px; display: flex; flex-direction: column; gap: 8px; }
  .head h3 { margin: 0; }
  .pick { align-items: flex-end; flex-wrap: wrap; }
  .body { flex: 1; min-height: 0; display: flex; gap: 12px; }
  .lists { width: 400px; flex-shrink: 0; overflow: auto; }
  .lists h4 { margin: 6px 0; }
  .grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(116px, 1fr)); gap: 8px; }
  .card { display: flex; flex-direction: column; align-items: center; gap: 4px; padding: 6px; height: auto; text-align: center; }
  .card.on { border-color: var(--accent); background: #22304d; }
  .card img { width: 100%; height: 100px; object-fit: contain; background: #fff; border-radius: 4px; }
  .none { height: 100px; display: flex; align-items: center; justify-content: center; font-weight: bold; }
  .faces { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 6px; }
  .faces canvas { flex: 1; min-height: 0; width: 100%; background: repeating-conic-gradient(#1b1f27 0 25%, #232834 0 50%) 0 0 / 24px 24px;
    border-radius: 6px; cursor: crosshair; touch-action: none; }
  .g { color: #2fd65a; } .b { color: #3d8bff; }
  .tabs button.on { border-color: var(--accent); background: #22304d; }
  .small { font-size: 12px; margin: 0; }
  .err { color: var(--danger); }
</style>
