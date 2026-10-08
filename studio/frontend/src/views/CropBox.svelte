<script lang="ts">
  // A fixed-shape crop box over an image: drag inside to move it, drag a corner to resize it (the shape stays `aspect`).
  // Everything outside the box is dimmed — the box is what will be shown. `crop` / onchange use fractions of the image
  // [x, y, w, h] from its top-left corner; null = the largest box in the middle.
  let { src, aspect, crop = null, label = '', onchange }: {
    src: string;
    aspect: number; // width ÷ height of the box, in image pixels
    crop?: number[] | null;
    label?: string; // drawn in the middle of the box (e.g. what the game puts on top)
    onchange: (crop: number[]) => void;
  } = $props();

  type R = { x: number; y: number; w: number; h: number }; // fractions of the image
  let img = $state<HTMLImageElement>();
  let natural = $state({ w: 0, h: 0 });
  let box = $state<R | null>(null);
  let drag: { mode: 'move' | 'nw' | 'ne' | 'sw' | 'se'; px: number; py: number; start: R } | null = null;

  /** Box height (fraction) for a width (fraction) that keeps the shape on this image. */
  const hFor = (w: number) => (w * natural.w) / (natural.h * aspect);

  function centred(): R {
    const ia = natural.w / natural.h;
    const w = ia > aspect ? aspect / ia : 1;
    const h = hFor(w);
    return { x: (1 - w) / 2, y: (1 - h) / 2, w, h };
  }

  function fromProp(): R {
    if (!crop || crop.length !== 4) return centred();
    const [x, y, w] = crop;
    const h = hFor(w); // re-derive so the shape is exact
    return { x: Math.min(Math.max(x, 0), 1 - w), y: Math.min(Math.max(y, 0), Math.max(0, 1 - h)), w, h };
  }

  function loaded() {
    if (!img) return;
    natural = { w: img.naturalWidth, h: img.naturalHeight };
    box = fromProp();
  }
  $effect(() => { crop; if (natural.w) box = fromProp(); });

  function down(ev: PointerEvent, mode: 'move' | 'nw' | 'ne' | 'sw' | 'se') {
    if (!box) return;
    ev.preventDefault();
    ev.stopPropagation();
    (ev.currentTarget as HTMLElement).setPointerCapture(ev.pointerId);
    drag = { mode, px: ev.clientX, py: ev.clientY, start: { ...box } };
  }

  function move(ev: PointerEvent) {
    if (!drag || !img || !box) return;
    const W = img.clientWidth, H = img.clientHeight;
    const s = drag.start;
    if (drag.mode === 'move') {
      const x = s.x + (ev.clientX - drag.px) / W, y = s.y + (ev.clientY - drag.py) / H;
      box = { ...s, x: Math.min(Math.max(x, 0), 1 - s.w), y: Math.min(Math.max(y, 0), 1 - s.h) };
      return;
    }
    // Resize around the opposite corner, in display pixels (the image is scaled evenly, so the shape on screen is `aspect` too).
    const left = drag.mode === 'nw' || drag.mode === 'sw', top = drag.mode === 'nw' || drag.mode === 'ne';
    const ax = (left ? s.x + s.w : s.x) * W, ay = (top ? s.y + s.h : s.y) * H;
    const r = img.getBoundingClientRect();
    const px = ev.clientX - r.left, py = ev.clientY - r.top;
    let w = Math.max(left ? ax - px : px - ax, (top ? ay - py : py - ay) * aspect);
    const maxW = Math.min(left ? ax : W - ax, (top ? ay : H - ay) * aspect);
    w = Math.min(Math.max(w, 24), maxW);
    const h = w / aspect;
    box = { x: (left ? ax - w : ax) / W, y: (top ? ay - h : ay) / H, w: w / W, h: h / H };
  }

  function up() {
    if (!drag || !box) return;
    drag = null;
    onchange([box.x, box.y, box.w, box.h]);
  }

  const pct = (f: number) => `${f * 100}%`;
</script>

<div class="crop">
  <img bind:this={img} {src} alt="" onload={loaded} draggable="false" />
  {#if box}
    <div class="box" style:left={pct(box.x)} style:top={pct(box.y)} style:width={pct(box.w)} style:height={pct(box.h)}
      onpointerdown={(e) => down(e, 'move')} onpointermove={move} onpointerup={up} role="presentation">
      {#if label}<span class="label">{label}</span>{/if}
      {#each ['nw', 'ne', 'sw', 'se'] as c (c)}
        <span class="handle {c}" onpointerdown={(e) => down(e, c as any)} onpointermove={move} onpointerup={up} role="presentation"></span>
      {/each}
    </div>
  {/if}
</div>

<style>
  .crop { position: relative; display: inline-block; align-self: flex-start; max-width: 100%; overflow: hidden; border-radius: 6px; background: var(--bg); line-height: 0; user-select: none; }
  img { display: block; max-width: 100%; max-height: 320px; }
  .box { position: absolute; box-sizing: border-box; border: 2px solid #fff; outline: 1px solid rgba(0, 0, 0, 0.6);
    box-shadow: 0 0 0 9999px rgba(0, 0, 0, 0.55); cursor: move; touch-action: none; }
  .label { position: absolute; left: 50%; top: 50%; transform: translate(-50%, -50%); line-height: 1.2; font-size: 12px; font-weight: 600;
    color: #fff; text-shadow: 0 0 3px #000; border: 1px dashed rgba(255, 255, 255, 0.7); padding: 2px 8px; pointer-events: none; white-space: nowrap; }
  .handle { position: absolute; width: 12px; height: 12px; background: #fff; border: 1px solid #000; border-radius: 2px; }
  .nw { left: -1px; top: -1px; cursor: nwse-resize; }
  .ne { right: -1px; top: -1px; cursor: nesw-resize; }
  .sw { left: -1px; bottom: -1px; cursor: nesw-resize; }
  .se { right: -1px; bottom: -1px; cursor: nwse-resize; }
</style>
