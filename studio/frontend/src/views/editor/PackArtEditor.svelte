<script lang="ts">
  // 3D pack / box art editor: the accessory face editor on the game's card pack and card box models. Layouts are kept in
  // studio.json (project.meta.packArt[packId].pack|box); "Apply & save" writes the composed texture + icon into the project,
  // points the pack at them and saves the set (which updates an installed set in the game). Unapplied edits are applied
  // automatically before any save, and before closing the editor or switching pack/box/tab (flush), so nothing is lost.
  import { App, projectFile, errText, ask } from '../../lib/api';
  import { applySmartArt, hasOwnLayers, type SmartSources } from '../../lib/smartArt';
  import SmartGenerate from './SmartGenerate.svelte';
  import { newLayout, edgeColors, loadImage, type Layout, type Model } from '../../lib/accessoryArt';
  import AccessoryEditor from '../AccessoryEditor.svelte';
  import ProductPhotos from './ProductPhotos.svelte';

  let { project, pack, notify, onapplied, save, hooks }: {
    project: any; pack: any; notify: (t: string, k?: string) => void; onapplied: () => void;
    save: () => Promise<void>;                    // the set editor's save (also updates the game)
    hooks: Set<() => Promise<void>>;              // run by the set editor before saving / switching tabs
  } = $props();

  let which = $state<'pack' | 'box'>('pack');
  let open = $state(false);
  let models = $state<Record<string, Model>>({});
  let editor = $state<any>();
  let applying = $state(false);
  let bust = $state(Date.now());
  let pending = $state(false);                    // edits not yet turned into the pack's texture/icon
  let photos = $state(false);                     // product photo picker open
  let generating = $state('');                    // Smart generate progress text
  let smartOpen = $state(false);                 // Smart generate dialog

  $effect(() => {
    const f = () => flush();
    hooks.add(f);
    return () => hooks.delete(f);
  });

  const kind = $derived(which === 'pack' ? 'Pack' : 'Box');
  const model = $derived(models[kind]);
  const layout = $derived(project.meta?.packArt?.[pack.id]?.[which] as Layout | undefined);
  const currentArt = $derived(which === 'pack' ? pack.packTexture : pack.boxTexture);

  $effect(() => {
    for (const k of ['Pack', 'Box'])
      if (!models[k]) App.AccessoryModel(k).then((m: any) => (models[k] = m)).catch((e: unknown) => notify(errText(e), 'error'));
  });

  // Packs switch (the tab keeps this component): close the editor so the next pack starts fresh.
  let lastPack = '';
  $effect(() => { if (pack.id !== lastPack) { lastPack = pack.id; open = false; } });

  function ensureLayout() {
    project.meta ??= {};
    project.meta.packArt ??= {};
    project.meta.packArt[pack.id] ??= {};
    if (!project.meta.packArt[pack.id][which]) {
      const l = newLayout();
      l.baseItem = which === 'pack' ? 'BasicCardPack' : 'BasicCardBox';
      project.meta.packArt[pack.id][which] = l;
    }
  }

  async function show(w: 'pack' | 'box') {
    if (open && w === which) return;
    await flush();
    which = w;
    ensureLayout();
    open = true;
  }

  const vanillaUrl = $derived.by(() => {
    if (!layout || !model) return '';
    if (layout.baseItem === 'file') return layout.baseFile ? projectFile(project.id, layout.baseFile, bust) : '';
    const b = model.bases?.find((b) => b.id === layout.baseItem) ?? model.bases?.[0];
    return b ? `/templates/${encodeURIComponent(b.texture)}` : '';
  });

  async function setBase(v: string) {
    if (!layout) return;
    if (v === 'snapshot') {
      if (!currentArt) return;
      try {
        layout.baseFile = await App.SnapshotPackBase(project.id, pack.id, which, currentArt);
        layout.baseItem = 'file';
        bust = Date.now();
        pending = true;
      } catch (e) { notify(errText(e), 'error'); }
      return;
    }
    layout.baseItem = v;
    pending = true;
  }

  /** Writes the composed texture + icon and points the pack at them (no save). */
  async function applyImages() {
    if (!editor) return;
    const { texture, icon } = await editor.exportImages();
    const r = await App.SavePackArt(project.id, pack.id, which, texture, icon);
    if (which === 'pack') { pack.packTexture = r.texture; pack.packIcon = r.icon; }
    else { pack.boxTexture = r.texture; pack.boxIcon = r.icon; }
    pending = false;
    onapplied();
  }

  /** Applies unapplied edits (before a save, closing the editor or switching pack/box/tab). */
  export async function flush() {
    if (!open || !pending) return;
    try { await applyImages(); } catch (e) { notify(errText(e), 'error'); }
  }

  async function apply() {
    applying = true;
    try {
      await applyImages();
      await save();
    } catch (e) { notify(errText(e), 'error'); }
    applying = false;
  }

  async function closeEditor() {
    await flush();
    open = false;
  }

  /** A product photo from the picker: a pack photo includes its crimps, so it is stretched over crimps + front (nearly the
   *  same shape); a box photo covers the front. */
  async function usePhoto(rel: string, name: string) {
    photos = false;
    try {
      if (which === 'pack') await editor?.addImageFile(rel, name, 'stretch', ['crimpTop', 'front', 'crimpBottom']);
      else await editor?.addImageFile(rel, name, 'cover');
      await tintFrom(rel);
      notify('Photo added and the other faces tinted to match — adjust them, then Apply & save');
    } catch (e) { notify(errText(e), 'error'); }
  }

  const TINT = 'Tint from art';

  /** Colours the faces the art doesn't cover (pack: back, edges, seam; box: sides, top & bottom) with gradients sampled from
   *  the image's edges — the packaging around the artwork. Earlier tints are replaced. */
  async function tintFrom(rel: string) {
    const c = edgeColors(await loadImage(projectFile(project.id, rel)));
    const grad = { color: c.top, color2: c.bottom };
    editor?.addFills(which === 'pack'
      ? [{ face: 'back', ...grad, name: `${TINT} (back)` }, { face: 'edges', ...grad, name: `${TINT} (edges)` },
         { face: 'seam', ...grad, name: `${TINT} (seam)` }]
      : [{ face: 'right', ...grad, name: `${TINT} (sides)` }, { face: 'top', color: c.top, name: `${TINT} (top)` }], TINT);
  }

  /** "Match colours": tints from the topmost visible image layer. */
  async function matchColours() {
    const img = [...(layout?.layers ?? [])].reverse().find((l) => l.kind === 'image' && l.visible && l.src);
    if (!img?.src) { notify('Add an image layer (e.g. a product photo) first', 'error'); return; }
    try { await tintFrom(img.src); notify(`Faces tinted to match ${img.name}`); }
    catch (e) { notify(errText(e), 'error'); }
  }

  /** Smart generate: opens the dialog (pick the photos, check the box faces); asks first if hand-made layers would go. */
  async function smart() {
    const art = project.meta?.packArt?.[pack.id];
    if ((hasOwnLayers(art?.pack) || hasOwnLayers(art?.box)) &&
      !(await ask('Replace the layers of this pack and its box with generated ones? (Your own layers are removed.)'))) return;
    smartOpen = true;
  }

  /** Builds and applies the art from the dialog's choice, then saves. The editor is closed first; it opens on the new layers. */
  async function runSmart(src: SmartSources) {
    smartOpen = false;
    try {
      await flush();
      open = false;
      generating = 'Building the pack and box…';
      const notes = await applySmartArt(project, pack, src);
      onapplied();
      generating = 'Saving…';
      await save();
      notify(['Pack' + (pack.hasBox ? ' and box' : '') + ' generated — open the 3D editor to adjust any part.', ...notes].join(' '));
    } catch (e) { notify(errText(e), 'error'); }
    generating = '';
  }

  async function pickImage(): Promise<string> {
    return (await App.PickImage(project.id, 'Choose an image for the layer')) || '';
  }
</script>

<div class="row">
  <button class="primary" disabled={!!generating} onclick={smart}
    title="Pick the product photos (or card art) and check the box faces, then build the pack{pack.hasBox ? ' and box' : ''} as layers you can still edit">
    {generating ? 'Generating…' : 'Smart generate…'}</button>
  <button class:primary={!open || which !== 'pack'} class:on={open && which === 'pack'} onclick={() => show('pack')}>Edit pack in 3D editor</button>
  {#if pack.hasBox}
    <button class:primary={!open || which !== 'box'} class:on={open && which === 'box'} onclick={() => show('box')}>Edit box in 3D editor</button>
  {/if}
  {#if open}
    <div class="grow"></div>
    <button class="small" onclick={closeEditor} title={pending ? 'Your changes are applied to the ' + which + ' when you close' : ''}>Close editor</button>
  {/if}
</div>

{#if generating}<p class="muted small">{generating}</p>{/if}
{#if smartOpen}
  <SmartGenerate {project} {pack} packIndex={Math.max(0, project.set.packs.findIndex((p: any) => p.id === pack.id))} hasBox={!!pack.hasBox}
    ongenerate={runSmart} oncancel={() => (smartOpen = false)} />
{/if}

{#if open && layout && model}
  <div class="row base">
    <label class="field grow">Start from
      <select value={layout.baseItem === 'file' ? 'file' : layout.baseItem} onchange={(e) => setBase(e.currentTarget.value)}>
        {#each model.bases ?? [] as b}<option value={b.id}>Vanilla: {b.label}</option>{/each}
        {#if layout.baseItem === 'file'}<option value="file">Earlier art (snapshot)</option>{/if}
        {#if currentArt}<option value="snapshot">This {which}'s current art (take a snapshot)</option>{/if}
      </select>
    </label>
    <button onclick={() => (photos = true)} title="Photos of this set's real booster packs and boxes from TCGplayer">Find product photos…</button>
    <button onclick={matchColours} title="Colours the {which === 'pack' ? 'back and edges' : 'sides and top'} to match the top image layer's packaging">Match colours</button>
    {#if pending}<span class="pending small" title="Applied automatically when you save, close the editor or switch">• unapplied changes</span>{/if}
    <button class="primary" disabled={applying} onclick={apply} title="Puts this art on the {which}, saves the set and updates the game">
      {applying ? 'Saving…' : `Apply & save`}</button>
  </div>
  <p class="muted small">
    {#if which === 'pack'}
      The crimps and the back print show on both sides of the pack; the pack torn open in the opening animation uses the same texture.
    {:else}
      The box is a simple block: the front print is also on the back, the side print on both sides, the top print on the bottom.
    {/if}
    "Current art" starts from the pack's current images (e.g. ones you chose yourself).
  </p>
  {#key pack.id + which}
    <AccessoryEditor bind:this={editor} {kind} {vanillaUrl} bind:layout={project.meta.packArt[pack.id][which]}
      {notify} onchange={() => (pending = true)} imageUrl={(rel) => projectFile(project.id, rel)} {pickImage}
      saveImage={(name, png) => App.SaveProjectImage(project.id, name, png)}
      defaultText={which === 'pack' ? (project.set.name || 'BOOSTER').toUpperCase() : 'BOOSTER BOX'} />
  {/key}
  {#if photos}<ProductPhotos projectId={project.id} {which} onpick={usePhoto} oncancel={() => (photos = false)} />{/if}
{/if}

<style>
  .base { align-items: flex-end; }
  .pending { color: var(--warn, #d9a400); align-self: center; white-space: nowrap; }
  button.on { border-color: var(--accent); background: #22304d; }
  .small { font-size: 12px; }
</style>
