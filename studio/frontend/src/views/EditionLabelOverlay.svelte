<script lang="ts">
  // The edition label on a card preview, as the mod draws it (lib/editionLabel): the game's spot moved by the choice, the
  // game's font (white, dark outline: the mod's FullImageGradeLabel). Covers the card (width × height css px); with onmove
  // the label can be dragged (x / y back in % of the card from the game's spot). Draws nothing until Studio has read the
  // game files.
  import { labelCentre, labelFontSize, labelOffset, labelTextWidth, watchLabelLayout, type LabelChoice, type LabelGame } from '../lib/editionLabel';
  import { drawSdfText } from '../lib/sdfText';

  let { text, choice, width, height, onmove, ondrag }: {
    text: string; choice: LabelChoice; width: number; height: number; onmove?: (x: number, y: number) => void;
    /** A drag starts (true) or ends (false): the card holds still meanwhile. */
    ondrag?: (on: boolean) => void;
  } = $props();

  let game = $state<LabelGame | null>(null);
  $effect(() => watchLabelLayout((g) => (game = g)));

  let canvas = $state<HTMLCanvasElement>();
  // The label's box on the card (css px), for dragging.
  const box = $derived.by(() => {
    if (!game) return null;
    const [fx, fy] = labelCentre(game.layout, choice);
    const px = (labelFontSize(game.layout, text) * choice.size * width) / game.layout.cardWidth;
    // The drawn text's own width and line height, with a few pixels to grab even when the label is small.
    const f = game.layout.font;
    return { cx: fx * width, cy: fy * height, w: Math.max(24, px * labelTextWidth(game.layout, text) + 8),
      h: Math.max(14, (px * (f.ascent - f.descent)) / f.pointSize + 6), px };
  });

  $effect(() => {
    const cv = canvas, g = game, b = box;
    if (!cv) return;
    const dpr = window.devicePixelRatio || 1;
    cv.width = Math.round(width * dpr); cv.height = Math.round(height * dpr);
    const ctx = cv.getContext('2d')!;
    ctx.clearRect(0, 0, cv.width, cv.height);
    if (!g || !b || !text || !choice.show) return;
    drawSdfText(ctx, g.layout.font, g.atlas, text, b.cx * dpr, b.cy * dpr,
      { px: b.px * dpr, color: g.layout.color, outline: 0.25, outlineColor: [0, 0, 0, 220 / 255] });
  });

  let dragging = $state(false);
  let host = $state<HTMLDivElement>();
  function down(e: PointerEvent) {
    if (!onmove) return;
    e.stopPropagation();
    (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
    dragging = true;
    ondrag?.(true);
  }
  function up() {
    if (!dragging) return;
    dragging = false;
    ondrag?.(false);
  }
  function move(e: PointerEvent) {
    if (!dragging || !game || !host) return;
    const r = host.getBoundingClientRect();
    const [x, y] = labelOffset(game.layout, (e.clientX - r.left) / r.width, (e.clientY - r.top) / r.height);
    onmove?.(x, y);
  }
</script>

<div class="overlay" bind:this={host}>
  <canvas bind:this={canvas}></canvas>
  {#if onmove && box && choice.show}
    <div class="handle" class:dragging role="slider" tabindex="-1" aria-label="Edition label position" aria-valuenow={choice.y}
      title="Drag to move the edition label"
      style:left="{box.cx - box.w / 2}px" style:top="{box.cy - box.h / 2}px" style:width="{box.w}px" style:height="{box.h}px"
      onpointerdown={down} onpointermove={move} onpointerup={up} onpointercancel={up}></div>
  {/if}
</div>

<style>
  .overlay { position: absolute; inset: 0; pointer-events: none; }
  canvas { position: absolute; inset: 0; width: 100%; height: 100%; }
  .handle { position: absolute; pointer-events: auto; cursor: grab; border: 1px dashed transparent; border-radius: 4px; touch-action: none; }
  .handle:hover, .handle.dragging { border-color: var(--accent); background: rgba(91, 140, 255, 0.12); }
  .handle.dragging { cursor: grabbing; }
</style>
