<script lang="ts">
  // A card's mask picture and its own values for shader parameters (set.json "shaderMask", "shaderParams"). On the card
  // editor's Look page. Both belong to the whole card: the mask serves every version, own values are kept by parameter name
  // and reach every shader of the card that has a parameter of that name. used = the shaders the card's versions use (with
  // which versions); the dropdown picks whose parameters are listed. known = parameter names any of the card's shaders has
  // (switched-off versions too): own values for others are leftovers.
  import { App, projectFile, errText } from '../../lib/api';
  import { setCardValue } from '../../lib/cardShaderView';

  let { project, card, imgBust, notify, used, known }: {
    project: any; card: any; imgBust: number; notify: (t: string, k?: string) => void;
    used: { shader: { id: string; name: string; params?: any[] }; versions: string[] }[];
    known: Set<string>;
  } = $props();

  let pickedId = $state('');
  const picked = $derived(used.find((u) => u.shader.id === pickedId) ?? used[0]);
  const shader = $derived(picked?.shader);
  // Parameter names other shaders of this card share with the picked one (one value serves them all).
  const sharedWith = (name: string) => used.filter((u) => u !== picked && (u.shader.params ?? []).some((p: any) => p.name === name)).map((u) => u.shader.name);

  async function pickShaderMask() {
    try {
      const rel = await App.PickCardImage(project.id);
      if (rel) card.shaderMask = rel;
    } catch (e) { notify(errText(e), 'error'); }
  }
  const ownValue = (name: string): number[] | undefined => card.shaderParams?.[name];
  function setOwnValue(name: string, value: number[] | undefined) {
    card.shaderParams = setCardValue(card.shaderParams, name, value);
  }
  const hex = (v: number[] = [1, 1, 1]) => '#' + v.slice(0, 3).map((x) => Math.round(Math.min(1, Math.max(0, x)) * 255).toString(16).padStart(2, '0')).join('');
  function setOwnColour(name: string, h: string) {
    const n = parseInt(h.slice(1), 16);
    setOwnValue(name, [((n >> 16) & 255) / 255, ((n >> 8) & 255) / 255, (n & 255) / 255, ownValue(name)?.[3] ?? 1]);
  }
  const valueParams = $derived(((shader?.params ?? []) as any[]).filter((p) => p.kind !== 'image'));
  // Values no shader of this card uses any more (not even on a switched-off version).
  const strayValues = $derived(Object.keys(card.shaderParams ?? {}).filter((n) => !known.has(n)));
</script>

<div class="fields">
  <h4 class="sub">Mask picture <span class="muted small">· every version of this card</span></h4>
  <p class="muted small">A picture the size of the card: white where shader effects show, black where they don't (grey = partly).
    The stock holo foils follow it, so the foil can sit on parts of the art only; your own shaders read it as <code>cardMask(uv)</code>.</p>
  <div class="row">
    {#if card.shaderMask}<img class="maskthumb" src={projectFile(project.id, card.shaderMask, imgBust)} alt="" />{/if}
    <button class="small" onclick={pickShaderMask}>{card.shaderMask ? 'Change mask…' : 'Add mask…'}</button>
    {#if card.shaderMask}<button class="small" onclick={() => (card.shaderMask = undefined)} title="Effects show everywhere on the card again">Remove mask</button>{/if}
  </div>

  <h4 class="sub">This card's own shader values</h4>
  {#if !used.length}
    <p class="muted small">None of this card's switched-on versions has a shader.</p>
  {:else}
    <label class="field">Shader
      <select value={picked?.shader.id} onchange={(e) => (pickedId = e.currentTarget.value)}>
        {#each used as u (u.shader.id)}<option value={u.shader.id}>{u.shader.name} — {u.versions.join(', ')}</option>{/each}
      </select>
    </label>
    {#if valueParams.length}
      <p class="muted small">Tick a value to give this card its own; unticked ones use the shader's. It applies to every version of
        this card that uses "{shader?.name}" ({picked?.versions.join(', ')}).</p>
    {:else}
      <p class="muted small">"{shader?.name}" has no values to change.</p>
    {/if}
  {/if}
  {#if valueParams.length || strayValues.length}
    {#each valueParams as p (p.name)}
      {@const v = ownValue(p.name)}
      {@const shared = sharedWith(p.name)}
      <div class="row ownval">
        <label class="check" title={shared.length ? `Also used by ${shared.join(', ')} on this card: one value serves them all` : ''}><input type="checkbox" checked={!!v} onchange={(e) => setOwnValue(p.name, e.currentTarget.checked ? [...(p.value ?? [0])] : undefined)} /> <code>{p.name}</code>{#if shared.length}<span class="badge">shared</span>{/if}</label>
        {#if v && p.kind === 'number'}
          <input type="range" class="grow" min={p.min ?? 0} max={p.max ?? 1} step="any" value={v[0]} oninput={(e) => setOwnValue(p.name, [Number(e.currentTarget.value)])} />
          <input type="number" class="num" step="any" value={v[0]} onchange={(e) => setOwnValue(p.name, [Number(e.currentTarget.value)])} />
        {:else if v && p.kind === 'vec2'}
          x <input type="number" class="num" step="any" value={v[0]} onchange={(e) => setOwnValue(p.name, [Number(e.currentTarget.value), v[1] ?? 0])} />
          y <input type="number" class="num" step="any" value={v[1]} onchange={(e) => setOwnValue(p.name, [v[0] ?? 0, Number(e.currentTarget.value)])} />
        {:else if v && p.kind === 'colour'}
          <input type="color" value={hex(v)} oninput={(e) => setOwnColour(p.name, e.currentTarget.value)} />
          <input type="range" class="grow" min="0" max="1" step="0.01" value={v[3] ?? 1} title="Strength (alpha)"
            oninput={(e) => setOwnValue(p.name, [v[0], v[1], v[2], Number(e.currentTarget.value)])} />
        {:else if !v}
          <span class="muted small">{p.kind === 'colour' ? hex(p.value) : (p.value ?? []).join(', ')}</span>
        {/if}
      </div>
    {/each}
    {#each strayValues as name (name)}
      <div class="row ownval muted small"><code>{name}</code> <span class="grow">a value no shader of this card uses now</span>
        <button class="small" onclick={() => setOwnValue(name, undefined)} title="Remove this value from the card">✕</button></div>
    {/each}
  {/if}
</div>

<style>
  .fields { display: flex; flex-direction: column; gap: 10px; }
  h4.sub { margin: 12px 0 2px; font-size: 13px; }
  .fields > h4.sub:first-child { margin-top: 0; }
  .maskthumb { width: 34px; height: 48px; object-fit: cover; border-radius: 4px; background: #000; }
  .ownval { gap: 6px; align-items: center; }
  .ownval .num { width: 70px; }
  .ownval .check { display: inline-flex; align-items: center; gap: 4px; }
</style>
