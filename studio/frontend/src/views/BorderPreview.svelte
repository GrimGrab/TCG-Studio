<script lang="ts">
  // A card with a border, live, like the mod: the card's picture over the whole card (cropped to the card's shape), or that
  // same picture resized into the border's box, then the border's layers back to front: image layers as pictures, shader
  // layers running (BorderShaderCanvas) on the picture under them. A clipped shader layer only shows where that picture is (the game clips at its edge; here its soft edges stay soft).
  // rim: show the game's own rim instead of a border (a slot without one): the game's sheet is the card, the picture on top
  // where the mod puts it (THIN_RIM_WINDOW: 729 × 522 of the 785 × 561.5 card at the default FullImageInset, centred on the
  // card); '' = the game draws no rim there (Full Art): the picture is the card.
  // picture: the shader of the card's picture itself (versionShaders), drawn on the picture in its window: with it this is the
  // whole card as the game draws it. Without a border and a rim the picture is the whole card (framed sets, rims not read).
  // still: the border's shader layers aren't run (each is a WebGL canvas, and a page only gets about 16): for previews of
  // versions that are switched off.
  // label: the edition label on top, as the mod draws it (EditionLabelOverlay); with label.onmove it can be dragged (the
  // card holds still while it is). The mouse tilts the card.
  // onwindow: the border's box can be moved (drag inside it) and sized (drag a corner) on the card, which then lies flat.
  import { CARD_SIZE, MIN_BOX, THIN_RIM_WINDOW, borderFile, clampBox, under, windowOf, type Border, type Box, type PictureShader } from '../lib/borders';
  import { cardFit } from '../lib/cardLooks';
  import { maximizable } from '../lib/maximize';
  import type { CardFacts, ShaderParam } from '../lib/cardShaderView';
  import BorderShaderCanvas from './BorderShaderCanvas.svelte';
  import EditionLabelOverlay from './EditionLabelOverlay.svelte';
  import type { LabelChoice } from '../lib/editionLabel';

  type Shader = { id: string; name: string; code: string; params?: ShaderParam[]; mix?: string; mixStrength?: number };
  let { border, shaders, art, width = 300, facts = { border: 0, foil: false, rarity: 0, grade: 0 }, rim, picture, label, still = false, onwindow }: {
    border: Border | null; shaders: Shader[]; art: string; width?: number; facts?: CardFacts; rim?: string; picture?: PictureShader;
    still?: boolean; onwindow?: (box: Box) => void;
    label?: { text: string; choice: LabelChoice; onmove?: (x: number, y: number) => void };
  } = $props();

  // Full view (the ⛶ button on the stage, Esc to leave): the card as large as the window allows.
  let full = $state(false);
  let screen = $state({ w: window.innerWidth, h: window.innerHeight });
  const shownWidth = $derived(full ? Math.max(200, Math.floor(Math.min(screen.w - 80, ((screen.h - 80) * CARD_SIZE[0]) / CARD_SIZE[1]))) : width);
  const height = $derived(Math.round((shownWidth * CARD_SIZE[1]) / CARD_SIZE[0]));
  const pct = (v: number) => `${(v * 100).toFixed(3)}%`;
  let tilt = $state<[number, number]>([0, 0]);
  let hovering = $state(false);
  // Why shader layers can't run, by layer (shown under the card).
  let failed = $state<Record<number, string>>({});
  // Key -1 = the picture's own shader.
  const problems = $derived(Object.entries(failed).filter(([i, t]) => t && (Number(i) < 0 ? !!picture : border?.layers[Number(i)]?.kind === 'shader'))
    .map(([i, t]) => (Number(i) < 0 ? `Picture shader: ${t}` : `Layer ${Number(i) + 1}: ${t}`)));
  function report(i: number, text: string) {
    if ((failed[i] ?? '') !== text) failed[i] = text;
  }

  const shaderOf = (id: string | undefined) => shaders.find((s) => s.id === id);

  let frame = $state<HTMLDivElement>();
  // A label being dragged: the card lies flat until it's dropped, so the drag follows the mouse.
  let placing = $state(false);
  function onPlacing(on: boolean) { placing = on; if (on) tilt = [0, 0]; }
  function onMove(e: PointerEvent) {
    if (placing || editBox) return;
    const r = frame!.getBoundingClientRect();
    const clamp = (v: number) => Math.max(-1, Math.min(1, v));
    const x = clamp(((e.clientX - r.left) / r.width - 0.5) * 2), y = clamp((0.5 - (e.clientY - r.top) / r.height) * 2);
    tilt = [-x * 0.5, -y * 0.5];
  }
  const win = $derived(windowOf(border));
  // The picture's box: inside the game's rim (THIN_RIM_WINDOW), else the border's box (the whole card without one).
  const box = $derived(rim !== undefined ? (rim ? THIN_RIM_WINDOW : [0, 0, 1, 1]) : win);
  // The size the picture is drawn at: under a border always the whole card's, then scaled into the box (.fit), so a box
  // only resizes the picture (stretched to its shape) and never shows another part of it, like the mod.
  const picBox = $derived(rim !== undefined ? box : [0, 0, 1, 1]);

  // Editing the border's box: the card lies flat while it can be dragged.
  const editBox = $derived(!!onwindow && rim === undefined && border?.window?.length === 4);
  $effect(() => { if (editBox) tilt = [0, 0]; });
  let drag: { part: string; x: number; y: number; start: Box } | null = null;
  function grab(e: PointerEvent, part: string) {
    e.preventDefault(); e.stopPropagation();
    (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
    drag = { part, x: e.clientX, y: e.clientY, start: [...win] };
  }
  function dragging(e: PointerEvent) {
    if (!drag || !onwindow) return;
    const r = frame!.getBoundingClientRect();
    const dx = (e.clientX - drag.x) / r.width, dy = (e.clientY - drag.y) / r.height;
    let [x, y, w, h] = drag.start;
    if (drag.part === 'move') {
      x = Math.min(1 - w, Math.max(0, x + dx)); y = Math.min(1 - h, Math.max(0, y + dy));
    } else {
      // A corner: the opposite corner stays put.
      let x1 = x + w, y1 = y + h;
      if (drag.part.includes('w')) x = Math.min(x1 - MIN_BOX, Math.max(0, x + dx)); else x1 = Math.max(x + MIN_BOX, Math.min(1, x1 + dx));
      if (drag.part.includes('n')) y = Math.min(y1 - MIN_BOX, Math.max(0, y + dy)); else y1 = Math.max(y + MIN_BOX, Math.min(1, y1 + dy));
      w = x1 - x; h = y1 - y;
    }
    onwindow(clampBox([x, y, w, h]));
  }
  const drop = () => { drag = null; };
  // Without a border every picture is card-shaped, like in the game (lib/cardLooks cardFit): scans stretched, anything else cropped.
  let wide = $state(false);
  const measure = (e: Event) => { const i = e.currentTarget as HTMLImageElement; wide = cardFit(i.naturalWidth / Math.max(1, i.naturalHeight)) === 'crop'; };

</script>

{#snippet pic()}
  {#if art && picture}
    <BorderShaderCanvas code={picture.code} params={picture.params ?? []} own={picture.own} mix={picture.mix} mixStrength={picture.mixStrength}
      draws={picture.draws ?? ''} mask={picture.mask ?? ''} {art} width={Math.round(shownWidth * picBox[2])} height={Math.round(height * picBox[3])}
      {tilt} {facts} {hovering} onproblem={(t) => report(-1, t)} />
  {:else if art}<img src={art} alt="" onload={measure} />{/if}
{/snippet}

<svelte:window onresize={() => (screen = { w: window.innerWidth, h: window.innerHeight })} />

<div class="stage" style:min-height="{height + 30}px" use:maximizable={{ onchange: (on) => (full = on) }}>
  <div class="frame" bind:this={frame} style:width="{shownWidth}px" style:height="{height}px" role="presentation"
    onpointermove={onMove} onpointerenter={() => (hovering = true)} onpointerleave={() => { tilt = [0, 0]; hovering = false; }}>
    <div class="card" style:transform="perspective(900px) rotateY({-tilt[0] * 50}deg) rotateX({tilt[1] * 50}deg)">
      {#if rim !== undefined}
        {#if rim}<img class="layer" src={rim} alt="" />{/if}
        <div class="inset" class:wide style:left={pct(box[0])} style:top={pct(box[1])} style:width={pct(box[2])} style:height={pct(box[3])}>{@render pic()}</div>
      {:else}
        <div class="win" style:left={pct(win[0])} style:top={pct(win[1])} style:width={pct(win[2])} style:height={pct(win[3])}>
          <div class="fit" style:width="{shownWidth}px" style:height="{height}px" style:transform="scale({win[2]}, {win[3]})">{@render pic()}</div>
        </div>
      {/if}
      {#if border}
        {#each border.layers as layer, i (i)}
          {#if layer.kind === 'image' && layer.file}
            <img class="layer" src={borderFile(border, layer.file)} alt="" style:opacity={layer.opacity ?? 1} />
          {:else if layer.kind === 'shader' && !still && shaderOf(layer.shader)}
            {@const s = shaderOf(layer.shader)!}
            {@const below = under(border, i)}
            {@const belowUrl = below ? borderFile(border, below.file) : ''}
            <div class="layer" style:opacity={layer.opacity ?? 1}
              style:mask-image={layer.clip && belowUrl ? `url("${belowUrl}")` : 'none'}
              style:-webkit-mask-image={layer.clip && belowUrl ? `url("${belowUrl}")` : 'none'}>
              <BorderShaderCanvas code={s.code} params={s.params ?? []} own={layer.params} mix={s.mix} mixStrength={s.mixStrength}
                art={belowUrl || art} width={shownWidth} {height} {tilt} {facts} {hovering} onproblem={(t) => report(i, t)} />
            </div>
          {/if}
        {/each}
      {/if}
      {#if editBox}
        <div class="boxedit" style:left={pct(win[0])} style:top={pct(win[1])} style:width={pct(win[2])} style:height={pct(win[3])}
          role="presentation" title="Drag to move the card picture's box; drag a corner to size it"
          onpointerdown={(e) => grab(e, 'move')} onpointermove={dragging} onpointerup={drop} onpointercancel={drop}>
          {#each ['nw', 'ne', 'sw', 'se'] as c}
            <span class="handle {c}" role="presentation" onpointerdown={(e) => grab(e, c)} onpointermove={dragging} onpointerup={drop} onpointercancel={drop}></span>
          {/each}
        </div>
      {/if}
      {#if label?.text}<EditionLabelOverlay text={label.text} choice={label.choice} width={shownWidth} {height} onmove={label.onmove} ondrag={onPlacing} />{/if}
    </div>
  </div>
</div>
{#each problems as p}<p class="notice err small">{p}</p>{/each}

<style>
  .stage { display: flex; align-items: center; justify-content: center; border-radius: 10px;
    background: repeating-conic-gradient(#2a2d36 0% 25%, #23262e 0% 50%) 0 0 / 24px 24px; }
  .frame { position: relative; }
  .card { position: absolute; inset: 0; transition: transform 0.08s linear; }
  .win { position: absolute; overflow: hidden; background: #3b4256; }
  .boxedit { position: absolute; outline: 1px dashed rgba(255, 255, 255, 0.85); box-shadow: 0 0 0 1px rgba(0, 0, 0, 0.6);
    cursor: move; touch-action: none; }
  .handle { position: absolute; width: 12px; height: 12px; background: #fff; border: 1px solid #222; border-radius: 2px;
    box-sizing: border-box; }
  .handle.nw { left: -6px; top: -6px; cursor: nwse-resize; }
  .handle.ne { right: -6px; top: -6px; cursor: nesw-resize; }
  .handle.sw { left: -6px; bottom: -6px; cursor: nesw-resize; }
  .handle.se { right: -6px; bottom: -6px; cursor: nwse-resize; }
  .fit { position: absolute; left: 0; top: 0; transform-origin: 0 0; }
  .win img { width: 100%; height: 100%; object-fit: cover; display: block; }
  .layer { position: absolute; inset: 0; width: 100%; height: 100%; pointer-events: none;
    mask-size: 100% 100%; -webkit-mask-size: 100% 100%; mask-repeat: no-repeat; -webkit-mask-repeat: no-repeat; }
  img.layer { object-fit: fill; }
  .inset { position: absolute; }
  .inset img { width: 100%; height: 100%; object-fit: fill; display: block; }
  .inset.wide img { object-fit: cover; }
</style>
