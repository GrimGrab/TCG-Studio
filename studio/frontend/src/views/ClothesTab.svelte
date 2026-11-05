<script lang="ts">
  // Game look → Clothes: new looks of the customers' clothing pieces (accessory library "clothes"). A look recolours a piece
  // and lays a print on its own UVs, painted in the shared face editor (AccessoryEditor) over a transparent texture with the
  // cloth colour shown beneath. The tab keeps its own draft; the page's Save calls `pending()` and stores it
  // (App.SaveClothes), `discard()` drops it. One editor is mounted at a time: switching keeps the left part's print.
  import { onMount, untrack } from 'svelte';
  import { App, errText } from '../lib/api';
  import { newLayout, type Layout, type Model } from '../lib/accessoryArt';
  import AccessoryEditor from './AccessoryEditor.svelte';

  type Def = { tint: string; tintR: string; tintG: string; tintB: string; print?: string };
  type Variant = { label: string; defs: Def[] };
  type Piece = { name: string; display: string; materials: number; mesh?: string; variants: Variant[] | null; islands?: unknown[][]; printScale?: number[] };
  type Slot = { table: string; label: string; items: Piece[] };
  type Templates = { bodies: Record<string, { slots: Slot[] }> };
  type Part = { tint: string; tintR?: string; tintG?: string; tintB?: string; print?: string; design?: any };
  type Look = { id: string; name: string; body: string; piece: string; parts: Part[]; worn: number; replaces?: string };

  let { notify, dirty = $bindable(false) }: { notify: (t: string, k?: string) => void; dirty?: boolean } = $props();

  let stored = $state<Look[]>([]);
  let looks = $state<Look[]>([]);
  let tpl = $state<Templates | null>(null);
  let error = $state('');
  let selected = $state('');
  let part = $state(0);
  // Prints painted since the last save: `${id}|${part}` → edited; finished PNGs of parts no longer shown.
  let edited = $state<Record<string, boolean>>({});
  let finished: Record<string, string> = {};
  let layouts = $state<Record<string, Layout>>({});

  const look = $derived(looks.find((l) => l.id === selected));
  const key = (id: string, j: number) => `${id}|${j}`;
  $effect(() => { dirty = JSON.stringify(looks) !== JSON.stringify(stored) || Object.values(edited).some(Boolean); });

  async function load() {
    try {
      const page = await App.LoadClothes();
      stored = JSON.parse(JSON.stringify(page.clothes ?? []));
      looks = JSON.parse(JSON.stringify(page.clothes ?? []));
      if (page.templates) tpl = await (await fetch(page.templates)).json();
      else error = "The clothes haven't been read from the game files yet (Game & mod page).";
      if (!looks.some((l) => l.id === selected)) selected = looks[0]?.id ?? '';
    } catch (e) { error = errText(e); }
  }
  onMount(load);

  /** The template piece of a look. */
  function pieceOf(body: string, name: string): Piece | undefined {
    for (const s of tpl?.bodies[body]?.slots ?? []) for (const it of s.items) if (it.name === name) return it;
  }
  /** Pieces that have colours and a model (the game can restyle them), by slot. */
  function paintable(body: string) {
    return (tpl?.bodies[body]?.slots ?? []).map((s) => ({ label: s.label, items: s.items.filter((it) => it.materials > 0 && it.mesh && (it.variants?.length ?? 0) > 0) }))
      .filter((s) => s.items.length > 0);
  }
  /** The piece has tint-mask areas (some vanilla variant colours R/G/B). */
  function hasMask(p: Piece | undefined, j: number) {
    return !!p?.variants?.some((v) => ['tintR', 'tintG', 'tintB'].some((k) => !/^#......00$/i.test((v.defs[j] as any)?.[k] ?? '#00000000')));
  }

  // ---- new look
  let newBody = $state('male');
  let newPiece = $state('');
  let newFrom = $state(0);
  const newPieceInfo = $derived(pieceOf(newBody, newPiece));
  $effect(() => {
    const b = newBody;
    if (tpl && !pieceOf(b, untrack(() => newPiece))) newPiece = paintable(b)[0]?.items[0]?.name ?? '';
  });
  async function addLook() {
    const p = newPieceInfo;
    if (!p) return;
    const v = p.variants?.[newFrom] ?? p.variants?.[0];
    const id = await App.NewClothingID();
    const parts: Part[] = [];
    for (let j = 0; j < p.materials; j++) {
      const d = v?.defs[j];
      const lay: any = newLayout();
      if (d?.print) lay.startPrint = d.print; // the vanilla print it starts from (the editor's base)
      parts.push({ tint: d?.tint ?? '#ffffffff', tintR: d?.tintR, tintG: d?.tintG, tintB: d?.tintB, design: lay });
      if (d?.print) edited[key(id, j)] = true; // the print must be made from the vanilla one
    }
    looks.push({ id, name: `${p.display} (${v?.label ?? 'new'})`, body: newBody, piece: p.name, parts, worn: 10 });
    await select(id);
  }

  // ---- editing
  async function finishShown() {
    if (look && edited[key(look.id, part)] && editor) finished[key(look.id, part)] = (await editor.exportImages()).texture;
  }
  async function select(id: string, j = 0) {
    if (id === selected && j === part) return;
    try { await finishShown(); } catch (e) { notify(errText(e), 'error'); return; }
    selected = id;
    part = j;
  }
  function remove(l: Look) {
    looks = looks.filter((x) => x.id !== l.id);
    for (const k of Object.keys(edited)) if (k.startsWith(l.id + '|')) delete edited[k];
    if (selected === l.id) { selected = looks[0]?.id ?? ''; part = 0; }
  }

  let models = $state<Record<string, Model>>({});
  let modelError = $state('');
  const modelKey = (l: Look, j: number) => `${l.body}|${l.piece}|${j}`;
  $effect(() => {
    const l = look, j = part;
    if (!l || !tpl || models[modelKey(l, j)]) return;
    App.ClothingModel(l.body, l.piece, j).then((m) => { models[modelKey(l, j)] = m as unknown as Model; modelError = ''; })
      .catch((e) => (modelError = errText(e)));
  });
  // The shown part's draft layout, from its stored design the first time.
  $effect(() => {
    const l = look, j = part;
    if (!l || layouts[key(l.id, j)]) return;
    untrack(() => { layouts[key(l.id, j)] = l.parts[j]?.design ? JSON.parse(JSON.stringify(l.parts[j].design)) : newLayout(); });
  });

  let editor = $state<AccessoryEditor | null>(null);
  function changed() {
    if (!look) return;
    edited[key(look.id, part)] = true;
    delete finished[key(look.id, part)];
  }

  // The editor's base: the vanilla print a look started from, else a transparent texture (a print shows only what is painted).
  // The editor paints on the piece's UVs and Save bakes that into the texture the game samples (half the texture across and
  // down, App.SaveClothes); a vanilla print is that texture already, so it's shown moved back by the same half.
  const blank = (() => { const c = document.createElement('canvas'); c.width = c.height = 1024; return c.toDataURL('image/png'); })();
  let uvPrints = $state<Record<string, string>>({});
  async function toUvSpace(url: string): Promise<string> {
    const img = new Image();
    img.src = url;
    await img.decode();
    const w = img.naturalWidth, h = img.naturalHeight, c = document.createElement('canvas');
    c.width = w; c.height = h;
    const g = c.getContext('2d')!;
    for (const dx of [0, w]) for (const dy of [0, h]) g.drawImage(img, dx - w / 2, dy - h / 2);
    return c.toDataURL('image/png');
  }
  function baseUrl(j: number) {
    const start = (layouts[look ? key(look.id, j) : ''] as any)?.startPrint;
    if (!start) return blank;
    if (!uvPrints[start]) toUvSpace(`/clothtemplates/${start}`).then((u) => (uvPrints[start] = u)).catch((e) => notify(errText(e), 'error'));
    return uvPrints[start] ?? '';
  }
  /** How many times the game repeats this part's print (1 = once over the UVs, the only kind Studio paints). */
  const printScale = (p: Piece | undefined, j: number) => p?.printScale?.[j] ?? 1;
  const rgb = (hex: string | undefined) => (hex ?? '#ffffff').slice(0, 7);
  /** A colour input's value back into #RRGGBBAA, keeping the alpha the shader reads. */
  const withAlpha = (v: string, old: string | undefined) => v + ((old ?? '').length === 9 ? old!.slice(7) : 'ff');

  /** The draft for the page's Save: every look, with the prints painted since the last save. */
  export async function pending(): Promise<{ item: Look; prints: Record<string, string> }[]> {
    await finishShown();
    return looks.map((l) => {
      const prints: Record<string, string> = {};
      const item: Look = JSON.parse(JSON.stringify(l));
      item.parts.forEach((p, j) => {
        const k = key(l.id, j);
        if (layouts[k]) p.design = layouts[k];
        if (edited[k] && finished[k]) prints[String(j)] = finished[k];
        // Started from a vanilla print and never opened: that print as it is.
        else if (edited[k] && (p.design as any)?.startPrint && !(p.design as any).layers?.length) prints[String(j)] = 'tpl:' + (p.design as any).startPrint;
      });
      return { item, prints };
    });
  }
  export function discard() {
    looks = JSON.parse(JSON.stringify(stored));
    layouts = {}; edited = {}; finished = {};
    if (!looks.some((l) => l.id === selected)) { selected = looks[0]?.id ?? ''; part = 0; }
  }
  /** After a save: the stored list is what was saved. Reload first: the editor's layouts are then rebuilt from the saved
   *  designs (cleared before, they came back from the draft's older copies and lost what was painted). */
  export async function saved() {
    await load();
    layouts = {}; edited = {}; finished = {};
  }
</script>

<div class="wrap">
  <div class="list">
    {#each ['male', 'female'] as body (body)}
      {#if looks.some((l) => l.body === body)}<h4>{body === 'male' ? 'Men' : 'Women'}</h4>{/if}
      {#each looks.filter((l) => l.body === body) as l (l.id)}
        <button class="item" class:on={l.id === selected} onclick={() => select(l.id)}>
          <span class="swatch" style:background={rgb(l.parts[0]?.tint)}></span>
          <span class="grow">{l.name || 'Unnamed'}</span>
          {#if l.parts.some((_, j) => edited[key(l.id, j)])}<span class="dot" title="Unsaved changes">•</span>{/if}
        </button>
      {/each}
    {/each}
    {#if tpl}
      <div class="new">
        <h4>New look</h4>
        <select bind:value={newBody}><option value="male">Men</option><option value="female">Women</option></select>
        <select bind:value={newPiece} onchange={() => (newFrom = 0)}>
          {#each paintable(newBody) as s}<optgroup label={s.label}>{#each s.items as it}<option value={it.name}>{it.display}</option>{/each}</optgroup>{/each}
        </select>
        {#if newPieceInfo?.variants?.length}
          <select bind:value={newFrom} title="The game's colours it starts from">
            {#each newPieceInfo.variants as v, i}<option value={i}>Start from {v.label}</option>{/each}
          </select>
        {/if}
        <button class="primary" onclick={addLook} disabled={!newPieceInfo}>Add</button>
      </div>
    {/if}
  </div>
  <div class="edit">
    {#if error}
      <p class="muted small">{error}</p>
    {:else if look}
      {@const p = pieceOf(look.body, look.piece)}
      <div class="row">
        <input class="name grow" bind:value={look.name} placeholder="Name of this look" maxlength="60" />
        <button class="small" onclick={() => remove(look)}>Delete</button>
      </div>
      <p class="muted small">{look.body === 'male' ? "Men's" : "Women's"} {p?.display ?? look.piece}.</p>
      <div class="row">
        <span class="lbl">Worn by</span>
        <input type="range" min="0" max="100" bind:value={look.worn} />
        <span class="num">{look.worn}%</span>
        <span class="muted small grow">of the customers dressed in a {p?.display ?? 'piece like this'}. 0% = only regulars you dress in it (Customers page).</span>
      </div>
      <div class="row">
        <span class="lbl">Replaces</span>
        <select bind:value={look.replaces}>
          <option value={undefined}>Nothing: it's an extra look</option>
          {#each p?.variants ?? [] as v}<option value={v.label}>The game's {v.label} {p?.display}</option>{/each}
        </select>
      </div>
      {#if look.parts.length > 1}
        <div class="seg">
          {#each look.parts as _, j}<button class:on={part === j} onclick={() => select(look.id, j)}>Part {j + 1}</button>{/each}
        </div>
      {/if}
      {@const pt = look.parts[part]}
      {#if pt}
        <div class="row colors">
          <label>Colour <input type="color" value={rgb(pt.tint)} oninput={(e) => (pt.tint = withAlpha(e.currentTarget.value, pt.tint))} /></label>
          {#if hasMask(p, part)}
            {#each [['tintR', 'Detail 1'], ['tintG', 'Detail 2'], ['tintB', 'Detail 3']] as [k, label]}
              <label>{label} <input type="color" value={rgb((pt as any)[k])} oninput={(e) => ((pt as any)[k] = withAlpha(e.currentTarget.value, (pt as any)[k]))} /></label>
            {/each}
          {/if}
        </div>
        {#if !p?.printScale}
          <p class="muted small">The game's clothes need reading again before prints can be painted (Game & mod page).</p>
        {:else if printScale(p, part) !== 1}
          <p class="muted small">The game repeats this {p?.display ?? 'piece'}'s print {printScale(p, part)} times over it, so it takes colours only for now.</p>
        {:else if modelError}
          <p class="muted small">{modelError}</p>
        {:else if models[modelKey(look, part)] && layouts[key(look.id, part)] && baseUrl(part)}
          <p class="muted small">Paint the print: images, text and colours on the piece's parts (outlined). What you leave empty shows the colour above.</p>
          {#key key(look.id, part)}
            <AccessoryEditor bind:this={editor} kind="Clothing" givenModel={models[modelKey(look, part)]} vanillaUrl={baseUrl(part)}
              bind:layout={layouts[key(look.id, part)]} {notify} onchange={changed} defaultText="MY SHOP" underColor={rgb(pt.tint)} />
          {/key}
        {:else}
          <p class="muted small">Loading…</p>
        {/if}
      {/if}
    {:else if tpl}
      <p class="muted">No clothes yet. Pick a piece on the left and <b>Add</b> it: then give it colours and paint a print.</p>
    {:else}
      <p class="muted small">Loading…</p>
    {/if}
  </div>
</div>

<style>
  .wrap { display: flex; gap: 14px; align-items: flex-start; min-height: 0; }
  .list { display: flex; flex-direction: column; gap: 4px; width: 220px; flex: none; }
  .list h4 { margin: 8px 0 2px; font-size: 13px; }
  .item { display: flex; gap: 6px; align-items: center; text-align: left; }
  .item.on { border-color: var(--accent); background: #22304d; }
  .swatch { width: 14px; height: 14px; border-radius: 3px; border: 1px solid var(--line); flex: none; }
  .new { display: flex; flex-direction: column; gap: 6px; margin-top: 10px; padding-top: 8px; border-top: 1px solid var(--line); }
  .edit { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 8px; }
  .row { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; }
  .lbl { width: 80px; color: var(--muted); }
  .num { width: 40px; }
  .name { font-size: 18px; font-weight: 600; }
  .colors label { display: flex; gap: 6px; align-items: center; }
  .dot { color: var(--accent-2); }
  .small { margin: 0; }
</style>
