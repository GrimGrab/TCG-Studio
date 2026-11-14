<script lang="ts">
  // The Shaders page's guided tutorial: one idea per step, all on the same shader. The steps come from Studio
  // (internal/cardshader's Tutorial, where a test compiles every one). A step is ticked off when the code has what the
  // step adds and the preview runs it; "Do it for me" puts the step's code in the editor.
  export type TutorialStep = { title: string; why: string; try?: string; add?: string; where?: string; code: string;
    params?: any[]; draws?: string; uses: string };

  let { steps, step = $bindable(0), code, draws, running, onapply, onclose }: {
    steps: TutorialStep[]; step?: number; code: string; draws: string; running: boolean;
    onapply: (s: TutorialStep) => void; onclose: () => void;
  } = $props();

  const at = $derived(steps[Math.min(step, steps.length - 1)]);
  const last = $derived(step >= steps.length - 1);
  /** The step's idea is in the code (and its Draws setting chosen) and the preview has no complaints. */
  const done = $derived(running && code.includes(at.uses) && (!at.draws || draws === at.draws));
</script>

<div class="tutorial">
  <div class="thead">
    <span class="count">Step {step + 1} of {steps.length}</span>
    <b class="grow">{at.title}</b>
    {#if done}<span class="tick" title="The preview shows it working">✓ done</span>{/if}
    <button class="x" onclick={onclose} title="Close the tutorial (the shader stays)">✕</button>
  </div>
  <div class="dots">
    {#each steps as s, i}
      <button class="dot" class:on={i === step} class:past={i < step} onclick={() => (step = i)} title={s.title} aria-label={s.title}></button>
    {/each}
  </div>
  <p>{at.why}</p>
  {#if at.add}
    <div class="muted small">Type this. {at.where}</div>
    <pre>{at.add}</pre>
  {/if}
  {#if done && at.try}<p class="try"><b>Try it:</b> {at.try}</p>{/if}
  {#if !running && code.includes(at.uses)}<p class="muted small">Almost: the messages under the editor say what the code still needs.</p>{/if}
  <div class="row">
    <button onclick={() => (step = Math.max(0, step - 1))} disabled={step === 0}>Back</button>
    <span class="grow"></span>
    {#if !done}<button onclick={() => onapply(at)} title="Puts this step's code in the editor (replaces what is there)">Do it for me</button>{/if}
    {#if last}
      <button class="primary" onclick={onclose} disabled={!done}>Finish</button>
    {:else}
      <button class="primary" onclick={() => (step = step + 1)}>{done ? 'Next' : 'Skip'}</button>
    {/if}
  </div>
</div>

<style>
  .tutorial { background: var(--panel); border: 1px solid var(--accent); border-radius: 8px; padding: 10px 12px; display: flex; flex-direction: column; gap: 6px; flex-shrink: 0; }
  .thead { display: flex; align-items: center; gap: 8px; }
  .count { font-size: 11px; color: var(--muted); text-transform: uppercase; letter-spacing: 0.06em; }
  .tick { color: #5fd08a; font-size: 12px; }
  .x { padding: 0 8px; background: none; border: none; color: var(--muted); cursor: pointer; }
  .dots { display: flex; gap: 4px; }
  .dot { width: 22px; height: 5px; padding: 0; border: none; border-radius: 3px; background: var(--line); cursor: pointer; }
  .dot.past { background: color-mix(in srgb, var(--accent) 45%, var(--line)); }
  .dot.on { background: var(--accent); }
  p { margin: 0; font-size: 13px; line-height: 1.45; }
  pre { margin: 0; padding: 6px 8px; background: var(--panel-2); border-radius: 6px; font-family: Consolas, monospace; font-size: 12px; white-space: pre-wrap; user-select: text; }
  .try { font-size: 12px; }
  .small { font-size: 12px; }
</style>
