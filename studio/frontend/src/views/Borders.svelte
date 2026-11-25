<script lang="ts">
  // Borders page: card borders of the setup (item library "borders"): a stack of image and shader layers drawn over
  // full-image cards in place of the game's border rim. Sets give them to their border slots (Set → Card look),
  // cards per version (card → Versions). One draft for the page; Save stores it (pictures into the shared store).
  import { onMount, tick } from 'svelte';
  import { App, ask, errText } from '../lib/api';
  import { projectFile } from '../lib/urls';
  import { asDataUrl, borderFile, borderIcon, hasSeeThrough, newBorder, setBoxSide, templatePng, under, windowOf, CARD_SIZE, DEFAULT_BOX, type Border, type BorderLayer } from '../lib/borders';
  import type { ShaderParam } from '../lib/cardShaderView';
  import SaveBar from './SaveBar.svelte';
  import BorderPreview from './BorderPreview.svelte';
  import Dropdown from './Dropdown.svelte';

  let { notify }: { notify: (t: string, k?: string) => void } = $props();

  type Shader = { id: string; name: string; code: string; params?: ShaderParam[]; builtIn?: boolean; mix?: string; mixStrength?: number };

  let borders = $state<Border[]>([]);
  let stock = $state<Border[]>([]);
  let shaders = $state<Shader[]>([]);
  let baseline = $state('');
  let loading = $state(true);
  let saving = $state(false);
  let selectedId = $state<string | null>(null);
  let query = $state('');
  let nameBox = $state<HTMLInputElement>();
  let covers = $state<{ name: string; url: string }[]>([]);
  let art = $state('');
  let foil = $state(false);
  let slot = $state(3); // Gold

  const snapshot = () => JSON.stringify(borders);
  const dirty = $derived(!loading && snapshot() !== baseline);
  const selected = $derived(borders.find((b) => b.id === selectedId) ?? null);
  const viewing = $derived(stock.find((b) => b.id === selectedId) ?? null);
  const shown = $derived(borders.filter((b) => !query || b.name.toLowerCase().includes(query.toLowerCase())));
  const shaderOf = (id?: string) => shaders.find((s) => s.id === id);

  async function load() {
    loading = true;
    try {
      const page = await App.LoadBordersPage();
      borders = (page.borders ?? []) as unknown as Border[];
      stock = ((page.stock ?? []) as unknown as Border[]).map((b) => ({ ...b, builtIn: true }));
      shaders = ((await App.ListShaders()) ?? []) as unknown as Shader[];
      if (!borders.some((b) => b.id === selectedId) && !stock.some((b) => b.id === selectedId)) selectedId = borders[0]?.id ?? stock[0]?.id ?? null;
      const sets = await App.ListProjects();
      covers = (sets ?? []).filter((s: any) => s.cover).map((s: any) => ({ name: s.name, url: projectFile(s.id, s.cover) }));
      if (!covers.some((c) => c.url === art)) art = covers[0]?.url ?? sample();
    } catch (e) {
      notify(errText(e), 'error');
    }
    loading = false;
    baseline = snapshot();
  }

  /** A stand-in card picture when the setup has no sets yet. */
  function sample(): string {
    const c = document.createElement('canvas');
    c.width = 500; c.height = 700;
    const g = c.getContext('2d')!;
    const grad = g.createLinearGradient(0, 0, 500, 700);
    grad.addColorStop(0, '#3b5b8c'); grad.addColorStop(0.5, '#8c4f7d'); grad.addColorStop(1, '#d9a14a');
    g.fillStyle = grad;
    g.fillRect(0, 0, 500, 700);
    g.fillStyle = 'rgba(255,255,255,0.85)';
    g.beginPath(); g.arc(250, 300, 120, 0, Math.PI * 2); g.fill();
    return c.toDataURL('image/png');
  }

  async function save() {
    const unnamed = borders.find((b) => !b.name?.trim());
    if (unnamed) {
      query = '';
      selectedId = unnamed.id;
      await tick();
      nameBox?.focus();
      notify("There's a border without a name: selected it. Name it or delete it, then save.", 'error');
      return;
    }
    saving = true;
    try {
      for (const b of borders) b.icon = (await borderIcon(b)) || undefined;
      const res = await App.SaveBorders(borders as any);
      await load();
      const warns = res.warnings ?? [];
      notify(warns.length ? `Saved. ${warns.join(' ')}` : 'Saved. The game uses it at its next start.', warns.length ? 'warn' : 'ok');
    } catch (e) {
      notify(errText(e), 'error');
    }
    saving = false;
  }

  async function add() {
    try {
      const id = await App.NewBorderID();
      borders.push(newBorder(id));
      selectedId = id;
      query = '';
      await tick();
      nameBox?.focus();
    } catch (e) {
      notify(errText(e), 'error');
    }
  }

  /** A border of your own from another one (a stock border's pictures are copied in on Save). */
  async function duplicate(b: Border) {
    try {
      const id = await App.NewBorderID();
      const copy: Border = JSON.parse(JSON.stringify({ ...b, id, name: `${b.name} copy`, builtIn: undefined, icon: undefined }));
      if (b.builtIn) for (const l of copy.layers) if (l.kind === 'image' && l.file) l.file = await asDataUrl(borderFile(b, l.file));
      borders.push(copy);
      selectedId = id;
    } catch (e) {
      notify(errText(e), 'error');
    }
  }

  async function remove(b: Border) {
    if (!(await ask(`Delete ${b.name ? `"${b.name}"` : 'this border'}? Border slots and cards that use it get the vanilla rim again. It stays in the Catalog, from where it can be added back.`))) return;
    borders = borders.filter((x) => x.id !== b.id);
    selectedId = borders[0]?.id ?? null;
  }

  // ---- layers
  async function addImage(at?: BorderLayer) {
    if (!selected) return;
    try {
      const ref = await App.PickBorderImage();
      if (!ref) return;
      if (at) at.file = ref;
      else selected.layers.push({ kind: 'image', file: ref, opacity: 1 });
    } catch (e) {
      notify(errText(e), 'error');
    }
  }
  function addShader() {
    if (!selected) return;
    const first = shaders[0]?.id;
    selected.layers.push({ kind: 'shader', shader: first, opacity: 1, clip: selected.layers.some((l) => l.kind === 'image') });
  }
  function move(i: number, d: number) {
    const ls = selected!.layers, j = i + d;
    if (j < 0 || j >= ls.length) return;
    [ls[i], ls[j]] = [ls[j], ls[i]];
  }
  const removeLayer = (i: number) => selected!.layers.splice(i, 1);

  /** A shader layer's value for a parameter: its own, else the shader's. */
  const valueOf = (l: BorderLayer, p: ShaderParam) => l.params?.[p.name] ?? p.value ?? [];
  function setValue(l: BorderLayer, p: ShaderParam, i: number, v: number) {
    const now = [...valueOf(l, p)];
    now[i] = v;
    l.params = { ...(l.params ?? {}), [p.name]: now };
  }

  // ---- transparency: the card's picture lies under every layer, so a picture without see-through parts hides it.
  // Picture URLs (borderFile) that have none.
  let opaque = $state<Record<string, boolean>>({});
  $effect(() => {
    const b = selected ?? viewing;
    if (!b) return;
    for (const l of b.layers) {
      const url = l.kind === 'image' ? borderFile(b, l.file) : '';
      if (url && !(url in opaque)) hasSeeThrough(url).then((yes) => { opaque[url] = !yes; });
    }
  });

  // Two templates: 'panel' = a framed card (this border's box, else an art box over a text panel); 'thin' = a thin rim round the whole picture, like
  // the game's own rim.
  async function exportTemplate(kind: 'panel' | 'thin') {
    try {
      const name = `${(selected?.name || 'border').replace(/[^\w -]/g, '')}_${kind === 'thin' ? 'thin_rim' : 'panel'}_template.png`;
      const path = await App.SaveTemplateImage(name, templatePng(selected, kind));
      if (path) notify(`Template saved to ${path} (${CARD_SIZE[0]}×${CARD_SIZE[1]}). Paint over it, keep the middle see-through, then add it as a picture layer.`);
    } catch (e) {
      notify(errText(e), 'error');
    }
  }

  const facts = $derived({ border: slot, foil, rarity: 0, grade: 0 });
  /** Whether any of a border's shaders reads what the card is (iBorder, iFoil…): only then does "Shown as" change anything. */
  const readsFacts = (b: Border) => b.layers.some((l) => l.kind === 'shader' && /\bi(Border|Foil|Rarity|Grade)\b/.test(shaderOf(l.shader)?.code ?? ''));
  const SLOTS = ['Base', 'First Edition', 'Silver', 'Gold', 'EX', 'Full Art'];
  onMount(load);
</script>

{#snippet preview(b: Border)}
  <aside class="side">
    <div class="block">
      <div class="bhead"><h2>Preview</h2></div>
      <label class="field">Shown on
        <select bind:value={art}>
          {#each covers as c}<option value={c.url}>{c.name}</option>{/each}
          {#if !covers.length}<option value={art}>Sample picture</option>{/if}
        </select>
      </label>
      <BorderPreview border={b} {shaders} {art} {facts} width={280}
        onwindow={b.builtIn || !b.window ? undefined : (box) => (b.window = box)} />
      {#if !b.builtIn && b.window}<p class="muted small">Drag the dashed box to move the card picture, a corner to size it.</p>{/if}
      {#if readsFacts(b)}
        <div class="row small" title="What this border's shaders are told about the card (iBorder, iFoil). Which border a slot's normal and foil versions get is chosen on a set's Card look page.">
          <span class="muted">Shown as</span>
          <select bind:value={slot}>{#each SLOTS as n, i}<option value={i}>{n}</option>{/each}</select>
          <label class="check"><input type="checkbox" bind:checked={foil} /> foil</label>
        </div>
      {/if}
      <p class="muted small">Mouse on the card = tilt.</p>
    </div>
    <div class="notice info">Normal and foil versions can have different borders: choose them per slot on a set's <b>Card look</b> page.</div>
  </aside>
{/snippet}

{#snippet hero(b: Border, stock: boolean)}
  <header class="pane-hero">
    <div class="art">{#if b.icon}<img src={borderFile(b, b.icon)} alt="" />{/if}</div>
    <div class="grow titlebox">
      {#if stock}
        <input class="title" value={b.name} readonly aria-label="Name" title="Comes with the mod: duplicate it to change it." />
      {:else}
        <input class="title" bind:this={nameBox} bind:value={b.name} aria-label="Name" placeholder="Name of the border" />
      {/if}
      <div class="meta">
        <span>{b.layers.length} layer{b.layers.length === 1 ? '' : 's'}</span>
        {#if stock}<span>Comes with the mod</span>{:else}<span class="code" title="Fixed id that sets use to name this border. Renaming is safe.">{b.id}</span>{/if}
      </div>
    </div>
    <div class="heroactions">
      {#if stock}
        <button onclick={() => duplicate(b)}>Duplicate to edit</button>
      {:else}
        <button onclick={() => duplicate(b)}>Duplicate</button>
        <button class="danger" onclick={() => remove(b)}>Delete</button>
      {/if}
    </div>
  </header>
{/snippet}

<SaveBar {dirty} {saving} onsave={save} onrevert={load}
  leaveMessage="Leave Borders without saving? Your unsaved changes will be lost."
  revertMessage="Throw away every unsaved change to your borders?">
  {#snippet title()}<h2>Borders</h2>{/snippet}
</SaveBar>
<div class="page libpage">
  <div class="intro">
    <span class="muted">Drawn over full-image cards instead of the game's border rim: picture layers (a card-shaped PNG, see-through where
      the card shows) and shader layers on top of them. Give borders to a set's slots on <b>Set → Card look</b>, or to one
      card's versions on its <b>Versions</b> tab.</span>
  </div>

  {#if loading}
    <p class="muted emptypane">Loading…</p>
  {:else}
    <div class="fwrap">
      <aside class="lside sidelist">
        <div class="sidetools"><input type="search" bind:value={query} placeholder="Search borders…" /></div>
        <div class="list" role="listbox" tabindex="-1">
          {#each shown as b (b.id)}
            <button class="item" class:active={b.id === selectedId} onclick={() => (selectedId = b.id)}>
              <span class="ico">{#if b.icon}<img src={borderFile(b, b.icon)} alt="" />{/if}</span>
              <span class="info"><span class="name">{b.name || 'Unnamed'}</span>
                <span class="sub">{b.layers.length} layer{b.layers.length === 1 ? '' : 's'}</span></span>
            </button>
          {:else}
            <p class="muted small empty">{query ? 'Nothing matches.' : 'No borders of your own yet.'}</p>
          {/each}
          {#if stock.length}
            <div class="grouphead muted small" title="Borders that come with the mod. Use them as they are, or duplicate one to change it.">Comes with the mod</div>
            {#each stock.filter((b) => !query || b.name.toLowerCase().includes(query.toLowerCase())) as b (b.id)}
              <button class="item" class:active={b.id === selectedId} onclick={() => (selectedId = b.id)}>
                <span class="ico">{#if b.icon}<img src={borderFile(b, b.icon)} alt="" />{/if}</span>
                <span class="info"><span class="name">{b.name}</span><span class="sub">stock</span></span>
              </button>
            {/each}
          {/if}
        </div>
        <div class="sidefoot"><button class="primary" onclick={add}>+ New border</button></div>
      </aside>

      {#if selected}
        {#key selected.id}
          <div class="form">
            {@render hero(selected, false)}
            <div class="cols2">
              <div class="main">
                <div class="block entries">
                  <div class="bhead"><h2>Layers</h2><span class="muted small">back to front: the last one is on top</span></div>
                  {#each selected.layers as l, i (i)}
                    {@const s = shaderOf(l.shader)}
                    <div class="entry">
                      <div class="head row">
                        <span class="badge">{i + 1}</span>
                        {#if l.kind === 'image'}
                          {#if l.file}<img class="lthumb" src={borderFile(selected, l.file)} alt="" />{/if}
                          <span class="grow name">Picture</span>
                          <span class="control"><button class="small" onclick={() => addImage(l)}>Change…</button></span>
                        {:else}
                          <span class="name">Shader</span>
                          <span class="control grow">
                            <select class="grow" bind:value={l.shader}>
                              {#each shaders.filter((x) => !x.builtIn) as x}<option value={x.id}>{x.name || 'Unnamed'}</option>{/each}
                              <optgroup label="Comes with the mod">{#each shaders.filter((x) => x.builtIn) as x}<option value={x.id}>{x.name}</option>{/each}</optgroup>
                              {#if l.shader && !s}<option value={l.shader}>Missing shader ({l.shader})</option>{/if}
                            </select>
                          </span>
                        {/if}
                        <span class="control">
                          <button class="small" disabled={i === 0} onclick={() => move(i, -1)} title="Further back">▲</button>
                          <button class="small" disabled={i === selected.layers.length - 1} onclick={() => move(i, 1)} title="Further forward">▼</button>
                          <button class="small" onclick={() => removeLayer(i)} title="Remove this layer">✕</button>
                        </span>
                      </div>
                      <div class="desc layerbody">
                        {#if l.kind === 'image' && l.file && opaque[borderFile(selected, l.file)]}
                          <div class="notice warn small">This picture has no see-through parts (a JPG, or a PNG without transparency):
                            the card's picture can't show through it. Use a PNG with a transparent middle.</div>
                        {/if}
                        <div class="row small">
                          <span class="muted">Opacity</span>
                          <input type="range" class="grow" min="0" max="1" step="0.01" bind:value={l.opacity} />
                          <span>{Math.round((l.opacity ?? 1) * 100)}%</span>
                        </div>
                        {#if l.kind === 'shader'}
                          {#if under(selected, i)}
                            <label class="check small"><input type="checkbox" bind:checked={l.clip} /> Keep the effect on the border</label>
                            <div class="muted small">{l.clip
                              ? `The effect only shows where picture layer ${selected.layers.indexOf(under(selected, i)!) + 1} is painted, never over the card's own picture in the middle.`
                              : `The effect may spread over the whole card, the card's own picture too (for shaders that paint everywhere).`}</div>
                          {:else}
                            <div class="muted small">No picture layer under this shader: it draws over the whole card. Move it above a picture layer to put it on the border.</div>
                          {/if}
                          {#each (s?.params ?? []).filter((p) => p.kind === 'number' || p.kind === 'colour') as p}
                            <div class="row small">
                              <code class="pname">{p.name}</code>
                              {#if p.kind === 'number'}
                                <input type="range" class="grow" min={p.min ?? 0} max={p.max ?? 1} step="any" value={valueOf(l, p)[0] ?? 0}
                                  oninput={(e) => setValue(l, p, 0, Number(e.currentTarget.value))} />
                                <span>{(valueOf(l, p)[0] ?? 0).toFixed(2)}</span>
                              {:else}
                                <input type="color" value={'#' + valueOf(l, p).slice(0, 3).map((x) => Math.round(Math.min(1, Math.max(0, x)) * 255).toString(16).padStart(2, '0')).join('')}
                                  oninput={(e) => { const n = parseInt(e.currentTarget.value.slice(1), 16); l.params = { ...(l.params ?? {}), [p.name]: [((n >> 16) & 255) / 255, ((n >> 8) & 255) / 255, (n & 255) / 255, valueOf(l, p)[3] ?? 1] }; }} />
                              {/if}
                            </div>
                          {/each}
                          {#if l.params && Object.keys(l.params).length}<div><button class="link small" onclick={() => (l.params = undefined)}>Back to the shader's own values</button></div>{/if}
                        {/if}
                      </div>
                    </div>
                  {:else}
                    <div class="entry"><span class="muted small">No layers yet. Start with a picture: a card-shaped PNG with a see-through middle.</span></div>
                  {/each}
                  <div class="entry row">
                    <button onclick={() => addImage()}>Add picture…</button>
                    <button onclick={addShader} disabled={!shaders.length} title={shaders.length ? '' : 'No shaders yet: write one on the Shaders page'}>Add shader</button>
                    <span class="grow"></span>
                    <Dropdown align="right" title="A picture of the card's shape with a see-through middle, to paint a border on in any editor">
                      {#snippet label()}Export template{/snippet}
                      {#snippet panel(close)}
                        <div class="ddmenu">
                          <button onclick={() => { close(); exportTemplate('thin'); }} title="A thin edge round the whole card: the picture fills almost all of it, like the game's own rim">Thin rim…</button>
                          <button onclick={() => { close(); exportTemplate('panel'); }} title="A framed card: an art box at the top and room for a panel below (or this border's own box)">Card panel…</button>
                        </div>
                      {/snippet}
                    </Dropdown>
                  </div>
                </div>

                <div class="block entries">
                  <div class="bhead"><h2>On the card</h2></div>
                  <div class="entry">
                    <div class="head row"><span class="grow name">Card picture</span>
                      <span class="control seg">
                        <button class:on={!selected.window} onclick={() => (selected.window = undefined)}>Whole card</button>
                        <button class:on={!!selected.window} onclick={() => { if (!selected.window) selected.window = DEFAULT_BOX(); }}>In a box</button>
                      </span></div>
                    <div class="desc">
                      {#if selected.window}
                        {@const w = windowOf(selected)}
                        <p class="muted small">The card's picture is resized into this box (stretched to its shape, nothing cut off), under
                          every layer. Drag it on the preview, or type it in % of the card.</p>
                        <div class="row small boxnums">
                          {#each ['Left', 'Top', 'Width', 'Height'] as name, k}
                            <label class="field">{name}
                              <input type="number" min="0" max="100" step="0.1" value={(w[k] * 100).toFixed(1)}
                                onchange={(e) => (selected.window = setBoxSide(w, k, Number((e.currentTarget as HTMLInputElement).value) / 100))} />
                            </label>
                          {/each}
                        </div>
                      {:else}
                        <p class="muted small">The card's picture covers the whole card, under every layer. The pictures' transparency
                          decides what of it shows: leave see-through wherever the card should show.</p>
                      {/if}
                    </div>
                  </div>
                </div>
              </div>
              {@render preview(selected)}
            </div>
          </div>
        {/key}
      {:else if viewing}
        {#key viewing.id}
          <div class="form">
            {@render hero(viewing, true)}
            <div class="cols2">
              <div class="main">
                <div class="block entries">
                  <div class="bhead"><h2>Layers</h2><span class="muted small">back to front: the last one is on top</span></div>
                  {#each viewing.layers as l, i}
                    <div class="entry">
                      <div class="head row">
                        <span class="badge">{i + 1}</span>
                        {#if l.kind === 'image'}
                          {#if l.file}<img class="lthumb" src={borderFile(viewing, l.file)} alt="" />{/if}
                          <span class="grow name">Picture</span>
                        {:else}
                          <span class="grow name">Shader: {shaderOf(l.shader)?.name ?? l.shader}</span>
                          {#if l.clip}<span class="muted small">kept on the border</span>{/if}
                        {/if}
                      </div>
                    </div>
                  {/each}
                </div>
              </div>
              {@render preview(viewing)}
            </div>
          </div>
        {/key}
      {:else}
        <div class="emptypane">
          <h1>Make your first border</h1>
          <p class="muted">Paint a card-shaped picture with a see-through middle (Export template gives you one to start from: a thin rim or a card panel), add it as a
            picture layer, then put a shader on top of it.</p>
          <div class="row"><button class="primary" onclick={add}>+ New border</button></div>
        </div>
      {/if}
    </div>
  {/if}
</div>

<style>
  .page { flex: 1; min-height: 0; display: flex; flex-direction: column; overflow: hidden; }
  .fwrap { display: flex; flex: 1; min-height: 0; }
  .lside { width: 270px; flex-shrink: 0; border-right: 1px solid var(--line); display: flex; flex-direction: column; min-height: 0; }
  .grouphead { padding: 10px 8px 4px; text-transform: uppercase; letter-spacing: 0.06em; font-size: 11px; }
  .form { container-type: inline-size; }
  .cols2 { display: grid; grid-template-columns: minmax(0, 1fr) 320px; gap: 14px; align-items: start; }
  @container (max-width: 760px) { .cols2 { grid-template-columns: minmax(0, 1fr); } }
  .main { display: flex; flex-direction: column; gap: 14px; min-width: 0; }
  .side { display: flex; flex-direction: column; gap: 14px; position: sticky; top: 0; }
  .layerbody { display: flex; flex-direction: column; gap: 6px; }
  .lthumb { height: 40px; border-radius: 4px; background: var(--bg); }
  .boxnums { flex-wrap: wrap; }
  .boxnums input { width: 5.5em; }
  .pname { min-width: 90px; font-family: Consolas, monospace; }
</style>
