<script lang="ts">
  // A person's Voice section: automatic or set, with a preview. The same on the Customers and Employees pages. Moving a slider sets their own voice.
  import { automaticVoice, previewVoice, sampleLine, MOOD_NAMES, type Mood, type Voice } from '../lib/talkVoice';

  let { id, female, voice, sample, onchange }: { id: string; female?: boolean; voice?: Voice; sample: string; onchange: (v: Voice | undefined) => void } = $props();
  const VOICE_KNOBS: { key: keyof Voice; label: string; min: number; max: number; hint: string }[] = [
    { key: 'pitch', label: 'Pitch', min: 0.5, max: 2, hint: 'How high their voice is' },
    { key: 'tone', label: 'Tone', min: 0, max: 1, hint: 'Muffled to crisp; the middle is the recording as it is' },
  ];
  const rounded = (v: Voice) => Object.fromEntries(Object.entries(v).map(([k, n]) => [k, Math.round(n * 100) / 100])) as Voice;
  const shown = $derived(voice ?? rounded(automaticVoice(id, !!female)));
  // svelte-ignore state_referenced_locally
  let voiceText = $state(sample);
  let voiceMood = $state<Mood>('neutral');
</script>

<h3>Voice</h3>
<div class="seg">
  <button class:on={!voice} onclick={() => onchange(undefined)}>Automatic</button>
  <button class:on={!!voice} onclick={() => onchange({ ...shown })}>Set</button>
</div>
<div class="voiceknobs">
  {#each VOICE_KNOBS as k}
    <label title={k.hint}><span>{k.label}</span>
      <input type="range" min={k.min} max={k.max} step="0.01" value={shown[k.key]} oninput={(e) => onchange({ ...shown, [k.key]: +e.currentTarget.value })} />
      <b class="num">{shown[k.key].toFixed(2)}</b></label>
  {/each}
</div>
<div class="row">
  <input class="grow" bind:value={voiceText} aria-label="What they say in the preview" />
  <select bind:value={voiceMood} aria-label="Mood">{#each MOOD_NAMES as m}<option value={m}>{m}</option>{/each}</select>
  <button class="small" onclick={() => previewVoice(shown, sampleLine(voiceText), voiceMood)}>▶ Hear it</button>
</div>
<p class="muted small">How they sound in the shop: Animal Crossing style, a quick recorded sound for each letter, so a word sounds the same each time, and a line's mood changes how it comes out. Automatic gives each person their own pitch, within their body's range. Press ▶ next to any line below to hear it.</p>

<style>
  .voiceknobs { display: flex; flex-direction: column; gap: 4px; margin-top: 6px; }
  .voiceknobs label { display: flex; align-items: center; gap: 8px; }
  .voiceknobs span { width: 64px; }
  .voiceknobs input { flex: 1; }
  .num { width: 70px; }
  h3 { font-size: 15px; margin: 0; }
</style>
