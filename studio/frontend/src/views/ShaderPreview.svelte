<script lang="ts">
  // A card picture drawn with a card shader, live (lib/cardShaderView). Move the mouse over the card to tilt it the way
  // looking at it from the side does in the game; the pointer being on it is also the game's "pointer on the card"
  // (iHover). `messages` is the compiler's verdict on the code. `look` = the shader's settings, `mask` = the card's mask
  // picture, `own` = the card's own parameter values.
  import { onMount } from 'svelte';
  import { App, errText } from '../lib/api';
  import { accFile } from '../lib/urls';
  import { maximizable } from '../lib/maximize';
  import { CardShaderView, withCardValues, type CardFacts, type ShaderLook, type ShaderMessage, type ShaderParam } from '../lib/cardShaderView';

  let { code, params, art, width = 300, controls = true, look = {}, mask = '', own = undefined, facts = $bindable({ border: 0, foil: true, rarity: 0, grade: 0 }),
    messages = $bindable([]), paramError = $bindable('') }: {
    code: string; params: ShaderParam[]; art: string; width?: number; controls?: boolean;
    look?: ShaderLook; mask?: string; own?: Record<string, number[]>; facts?: CardFacts;
    messages?: ShaderMessage[]; paramError?: string;
  } = $props();

  let canvas = $state<HTMLCanvasElement>();
  let view = $state<CardShaderView>();
  let failed = $state('');
  let paused = $state(false);
  let tilt = $state<[number, number]>([0, 0]);
  let wrapping = $state<{ head: string; tail: string } | null>(null);

  // Full view (the button on the stage, Esc to leave): the card as large as the window allows.
  let full = $state(false);
  let win = $state({ w: window.innerWidth, h: window.innerHeight });
  const shownWidth = $derived(full ? Math.max(200, Math.floor(Math.min(win.w - 80, (win.h - 80) / 1.4))) : width);
  const height = $derived(Math.round(shownWidth * 1.4));
  // The wrapping depends on the parameters' names and kinds and on how Shadertoy code is mixed with the picture.
  const shape = $derived(JSON.stringify([params.map((p) => [p.name, p.kind]), look.mix ?? '', look.mixStrength ?? 0]));

  onMount(() => {
    try {
      view = new CardShaderView(canvas!);
      view.start();
    } catch (e) {
      failed = errText(e);
    }
    return () => view?.dispose();
  });

  $effect(() => {
    const [names, mix, strength] = JSON.parse(shape) as [[string, string][], string, number];
    let stale = false;
    App.ShaderWrapping(names.map(([name, kind]) => ({ name, kind })) as any, mix, strength)
      .then((w) => { if (!stale) { wrapping = w; paramError = ''; } })
      .catch((e) => { if (!stale) paramError = errText(e); });
    return () => { stale = true; };
  });

  $effect(() => { if (view && art) view.setArt(art); });
  $effect(() => { if (view) view.setMask(mask); });
  $effect(() => {
    if (!view) return;
    view.setParams(JSON.parse(JSON.stringify(withCardValues(params, own))), (p) => accFile(p.image));
  });
  $effect(() => { if (view) { view.draws = look.draws ?? ''; view.facts = { ...facts }; view.draw(); } });
  $effect(() => {
    if (!view || !wrapping) return;
    messages = view.setCode(code, wrapping);
  });
  $effect(() => { if (view) view.tilt = tilt; });

  let hit = $state<HTMLDivElement>();
  function onMove(e: PointerEvent) {
    const r = hit!.getBoundingClientRect(); // the untilted frame: the measure doesn't move as the card tilts
    const clamp = (v: number) => Math.max(-1, Math.min(1, v));
    const x = clamp(((e.clientX - r.left) / r.width - 0.5) * 2), y = clamp((0.5 - (e.clientY - r.top) / r.height) * 2);
    tilt = [-x * 0.5, -y * 0.5];
  }

  // "Shown as" only matters to code that reads what the card is; other shaders don't get the row.
  const readsFacts = $derived(/\bi(Border|Foil|Rarity|Grade)\b/.test(code));
  const BORDERS = ['Base', 'First Edition', 'Silver', 'Gold', 'EX', 'Full Art'];

  /** The current frame as a small picture (for the shader's list entry); "" when nothing is showing. */
  export function snapshot(): string {
    return view && !messages.length ? view.snapshot() : '';
  }
</script>

<svelte:window onresize={() => (win = { w: window.innerWidth, h: window.innerHeight })} />

<div class="preview" style:width="{width}px">
  {#if failed}
    <p class="notice err">{failed}</p>
  {/if}
  <div class="stage" style:min-height="{Math.round(width * 1.4) + 30}px" use:maximizable={{ onchange: (on) => (full = on) }}>
    <!-- The pointer is followed on a still frame around the card, not on the card itself: a tilted card slides out from
         under the pointer near its edges, which used to snap it flat and back again (jerky). -->
    <div class="hit" bind:this={hit} style:width="{shownWidth}px" style:height="{height}px"
      onpointermove={onMove} onpointerenter={() => view?.hover(true)}
      onpointerleave={() => { tilt = [0, 0]; view?.hover(false); }} role="presentation">
      <canvas bind:this={canvas} width={shownWidth * 2} height={height * 2} style:width="{shownWidth}px" style:height="{height}px"
        style:transform="perspective(900px) rotateY({-tilt[0] * 50}deg) rotateX({tilt[1] * 50}deg)"></canvas>
    </div>
  </div>
  {#if controls}
    <div class="row tools">
      <button onclick={() => { paused = !paused; view?.pause(paused); }}>{paused ? 'Play' : 'Pause'}</button>
      <button onclick={() => view?.restart()} title="The card appears again: the clocks (iTime, iShown) start at zero and it gets another random number (iSeed)">Restart</button>
      <span class="muted small grow hint">Mouse on the card = tilt and pointer. ⛶ = full view.</span>
    </div>
    {#if readsFacts}
    <div class="row tools small" title="What the shader is told about the card (iBorder, iFoil, iRarity, iGrade). Only matters to shaders that read them.">
      <span class="muted">Shown as</span>
      <select bind:value={facts.border}>{#each BORDERS as b, i}<option value={i}>{b}</option>{/each}</select>
      <label class="check"><input type="checkbox" bind:checked={facts.foil} /> foil</label>
      <label class="check" title="The rarity's place in the set's list: 0 = the first (most common)">rarity <input type="number" class="tiny" min="0" max="20" bind:value={facts.rarity} /></label>
      <label class="check" title="0 = not graded">grade <input type="number" class="tiny" min="0" max="10" bind:value={facts.grade} /></label>
    </div>
    {/if}
  {/if}
</div>

<style>
  .preview { display: flex; flex-direction: column; gap: 8px; }
  .stage { display: flex; align-items: center; justify-content: center; border-radius: 10px;
    background: repeating-conic-gradient(#2a2d36 0% 25%, #23262e 0% 50%) 0 0 / 24px 24px; }
  .hit { display: flex; align-items: center; justify-content: center; }
  canvas { display: block; transition: transform 0.08s linear; pointer-events: none; }
  .tools { flex-wrap: wrap; }
  .hint { text-align: right; }
  .small { font-size: 12px; }
  .check { display: inline-flex; align-items: center; gap: 4px; }
  .tiny { width: 46px; }
</style>
