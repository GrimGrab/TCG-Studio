<script lang="ts">
  // Game look → Textures: the cardboard boxes, the bulk card boxes, the shopping bag and the street billboards, painted in the
  // shared face editor (AccessoryEditor) over each one's vanilla texture. A look with a twin (the bag: carried closed, open on
  // the counter) gets the same design rendered over the twin's vanilla texture too. Edits stay in this tab's draft; the page's Save takes `pending()` (the finished textures +
  // layouts of every changed box) and `discard()` drops them. One editor is mounted at a time: switching boxes keeps the left
  // box's finished texture.
  import { untrack } from 'svelte';
  import { App, errText } from '../lib/api';
  import { loadImage, netScale, newLayout, renderTexture, type Layout, type Model } from '../lib/accessoryArt';
  import { shopSignFile } from '../lib/urls';
  import AccessoryEditor from './AccessoryEditor.svelte';

  type Box = { id: string; title: string; group: string; model: string; vanilla: string; file: string; image: string; layout: string; twin: string };
  let { boxes, notify, dirty = $bindable(false) }: {
    boxes: Box[]; notify: (t: string, k?: string) => void; dirty?: boolean;
  } = $props();

  let selected = $state('');
  $effect(() => { if (!selected && boxes.length) selected = untrack(() => boxes[0].id); });
  const box = $derived(boxes.find((b) => b.id === selected));

  // Draft per box: its layout, whether it was edited, or reset to vanilla; finished textures of edited boxes not shown now.
  let layouts = $state<Record<string, Layout>>({});
  let edited = $state<Record<string, boolean>>({});
  let reset = $state<Record<string, boolean>>({});
  let finished: Record<string, string> = {};
  $effect(() => { dirty = Object.values(edited).some(Boolean) || Object.values(reset).some(Boolean); });

  function storedLayout(b: Box): Layout {
    if (b.layout) { try { return JSON.parse(b.layout); } catch { /* fall through */ } }
    return newLayout();
  }
  // The shown box's draft layout, made from the stored one the first time it's opened.
  $effect(() => {
    const b = box;
    if (b && !layouts[b.id]) layouts[b.id] = untrack(() => storedLayout(b));
  });

  let models = $state<Record<string, Model>>({});
  let modelError = $state('');
  $effect(() => {
    const b = box;
    if (!b || models[b.model]) return;
    App.BoxLookModel(b.model).then((m) => { models[b.model] = m as unknown as Model; modelError = ''; })
      .catch((e) => (modelError = errText(e)));
  });

  let editor = $state<AccessoryEditor | null>(null);

  async function finishShown() {
    if (box && edited[box.id] && editor) finished[box.id] = (await editor.exportImages()).texture;
  }
  async function select(id: string) {
    if (id === selected) return;
    try { await finishShown(); } catch (e) { notify(errText(e), 'error'); return; }
    selected = id;
  }

  function changed() {
    if (!box) return;
    edited[box.id] = true;
    reset[box.id] = false;
    delete finished[box.id];
  }
  // Bumped when the layout is replaced from here: the editor only redraws on its own edits, so it remounts.
  let editorKey = $state(0);
  function resetBox() {
    if (!box) return;
    layouts[box.id] = newLayout();
    editorKey++;
    reset[box.id] = !!box.file;
    edited[box.id] = false;
    delete finished[box.id];
  }

  /** The same design over a twin's vanilla texture (the open bag), as a PNG data URL. */
  async function twinTexture(b: Box): Promise<string> {
    const m = models[b.model], layout = layouts[b.id];
    if (!m || !layout) throw new Error(`${b.title}: the editor isn't ready`);
    const vanilla = await loadImage(b.twin);
    const painted = layout.textureFile ? await loadImage(shopSignFile(layout.textureFile)) : null;
    const make = await renderTexture(m, layout, vanilla, painted, (rel) => shopSignFile(rel), netScale(m), document.createElement('canvas'));
    return make().toDataURL('image/png');
  }

  /** The changed looks for the page's Save: { id: { png, twinPng, layout } } (png '' = back to vanilla). */
  export async function pending(): Promise<Record<string, { png: string; twinPng: string; layout: string }>> {
    await finishShown();
    const out: Record<string, { png: string; twinPng: string; layout: string }> = {};
    for (const b of boxes) {
      if (reset[b.id] && !edited[b.id]) out[b.id] = { png: '', twinPng: '', layout: '' };
      else if (edited[b.id] && finished[b.id]) out[b.id] = {
        png: finished[b.id], twinPng: b.twin ? await twinTexture(b) : '', layout: JSON.stringify(layouts[b.id]),
      };
    }
    return out;
  }
  export function discard() {
    layouts = {}; edited = {}; reset = {}; finished = {};
  }

  const shown = (b: Box) => (edited[b.id] ? 'edited' : reset[b.id] ? 'vanilla' : b.file ? 'custom' : '');
  const groups = [['cardboard', 'Cardboard boxes'], ['bulk', 'Bulk card boxes'], ['bag', 'Shopping bag'], ['street', 'Street']];
</script>

<div class="wrap">
  <div class="list">
    {#each groups as [g, title] (g)}
      <h4>{title}</h4>
      {#each boxes.filter((b) => b.group === g) as b (b.id)}
        <button class="item" class:on={b.id === selected} onclick={() => select(b.id)}>
          <span class="grow">{b.title}</span>
          {#if shown(b) === 'edited'}<span class="dot" title="Unsaved changes">•</span>
          {:else if shown(b) === 'vanilla'}<span class="muted small" title="Back to vanilla when you save">vanilla</span>
          {:else if shown(b) === 'custom'}<span class="tag small" title="Your texture is in the game">custom</span>{/if}
        </button>
      {/each}
    {/each}
  </div>
  <div class="edit">
    {#if box}
      <div class="row">
        <p class="muted small grow">
          {#if box.group === 'bulk'}The box the workbench packs bulk cards into. Each variant has its own look; the front and the lid carry the print.
          {:else if box.group === 'bag'}The paper bag customers carry out. The same design goes on the open bag on the cashier counter (its handles stay drawn on it).
          {:else if box.group === 'street'}The advertising boards on the building across the road: two top boards on the sides share one picture, the big board wraps around all three sides.
          {:else}Blue lines outline every part of the box, inside and cut edges included: paint over any of them.{/if}
        </p>
        <button class="small" onclick={resetBox} disabled={!box.file && !edited[box.id]} title="Back to the game's own look when you save">Vanilla look</button>
      </div>
      {#if !box.vanilla}
        <p class="muted small">This texture hasn't been read from the game files yet (Game &amp; mod page).</p>
      {:else if modelError}
        <p class="muted small">{modelError}</p>
      {:else if models[box.model] && layouts[box.id]}
        {#key `${box.id}|${editorKey}`}
          <AccessoryEditor bind:this={editor} kind="Box look" givenModel={models[box.model]} vanillaUrl={box.vanilla}
            bind:layout={layouts[box.id]} {notify} onchange={changed} defaultText="MY SHOP"
            imageUrl={(rel) => shopSignFile(rel)}
            pickImage={async () => (await App.ImportBoxImage()).file}
            saveImage={(name, dataUrl) => App.SaveBoxImage(name, dataUrl)} />
        {/key}
      {:else}
        <p class="muted small">Loading…</p>
      {/if}
    {/if}
  </div>
</div>

<style>
  .wrap { display: flex; gap: 14px; align-items: flex-start; min-height: 0; }
  .list { display: flex; flex-direction: column; gap: 4px; width: 200px; flex: none; }
  .list h4 { margin: 8px 0 2px; font-size: 13px; }
  .item { display: flex; gap: 6px; align-items: center; text-align: left; }
  .item.on { border-color: var(--accent); background: #22304d; }
  .edit { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 8px; }
  .dot { color: var(--accent-2); }
  .tag { color: var(--accent); }
  .grow { flex: 1; }
  .small { font-size: 12px; margin: 0; }
</style>
