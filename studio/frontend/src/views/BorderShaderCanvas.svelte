<script lang="ts">
  // A card shader drawn on one picture, for BorderPreview: a border's shader layer on the picture under it (draws
  // 'seeThrough': only the shader, the layers below are other elements; no mask), or the card picture's own shader (its
  // own draws and the card's mask picture).
  import { onMount } from 'svelte';
  import { App } from '../lib/api';
  import { accFile } from '../lib/urls';
  import { CardShaderView, withCardValues, type CardFacts, type ShaderParam } from '../lib/cardShaderView';

  let { code, params, own, mix = '', mixStrength = 0, art, width, height, tilt, facts, hovering = false, draws = 'seeThrough', mask = '', onproblem }: {
    code: string; params: ShaderParam[]; own?: Record<string, number[]>; mix?: string; mixStrength?: number;
    art: string; width: number; height: number; tilt: [number, number]; facts: CardFacts; hovering?: boolean;
    draws?: string; mask?: string;
    /** Why the shader can't run ('' = it runs). */
    onproblem?: (text: string) => void;
  } = $props();

  let canvas = $state<HTMLCanvasElement>();
  let view = $state<CardShaderView>();
  let wrapping = $state<{ head: string; tail: string } | null>(null);
  const shape = $derived(JSON.stringify([params.map((p) => [p.name, p.kind]), mix, mixStrength]));

  onMount(() => {
    try {
      view = new CardShaderView(canvas!);
      view.start();
    } catch (e) {
      onproblem?.(String(e));
    }
    return () => view?.dispose();
  });

  $effect(() => {
    const [names, m, s] = JSON.parse(shape) as [[string, string][], string, number];
    let stale = false;
    App.ShaderWrapping(names.map(([name, kind]) => ({ name, kind })) as any, m, s)
      .then((w) => { if (!stale) wrapping = w; })
      .catch((e) => { if (!stale) onproblem?.(String(e)); });
    return () => { stale = true; };
  });
  $effect(() => { if (view && art) view.setArt(art); });
  $effect(() => { if (view) view.setMask(mask); });
  $effect(() => { if (view) view.draws = draws; });
  $effect(() => { if (view) view.setParams(JSON.parse(JSON.stringify(withCardValues(params, own))), (p) => accFile(p.image)); });
  $effect(() => { if (view) { view.facts = { ...facts }; view.tilt = tilt; } });
  $effect(() => { if (view) view.hover(hovering); });
  $effect(() => {
    if (!view || !wrapping) return;
    const msgs = view.setCode(code, wrapping);
    onproblem?.(msgs.length ? `Line ${msgs[0].line}: ${msgs[0].text}` : '');
  });
</script>

<canvas bind:this={canvas} width={width * 2} height={height * 2} style:width="{width}px" style:height="{height}px"></canvas>

<style>
  canvas { display: block; pointer-events: none; }
</style>
