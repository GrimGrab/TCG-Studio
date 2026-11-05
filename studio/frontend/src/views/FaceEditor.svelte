<script lang="ts" module>
  export type { Face, Characters } from '../lib/faceLook';
</script>

<script lang="ts">
  // A regular's own face and body (Customers page), over a full window: a live 3D preview of the game's character
  // (lib/characterView.ts) and the settings the game's character presets use, grouped as Face / Hair / Skin / Body. It
  // edits `face` in place (the page's draft); the mod applies it with the game's own character customizer.
  import { onMount, untrack } from 'svelte';
  import { CharacterView, loadCharMesh, type CameraFrame, type Cloth } from '../lib/characterView';
  import { describeShapes, type ShapeInfo } from '../lib/faceShapes';
  import { charTexUrl, faceLook, facePieces, type Characters, type Face, type Preset } from '../lib/faceLook';

  let { chars, body, preset, face = $bindable(), clothes, title, onclose }: {
    chars: Characters; body: string; preset: string; face: Face | undefined;
    clothes: Cloth[]; title: string; onclose: () => void;
  } = $props();

  const B = $derived(chars.bodies[body]);
  const base = $derived(B?.presets.find((p) => p.name === preset) ?? B?.presets[0]);

  /** A copy of the character's preset as a face to edit. */
  function fromPreset(p: Preset | undefined): Face {
    return {
      shapes: { ...(p?.shapes ?? {}) }, hair: [...(p?.hair ?? [])], hairColor: p?.hairColor ?? '#3a2a20ff',
      floats: { ...(p?.floats ?? {}) }, textures: { ...(p?.textures ?? {}) }, colors: { ...(p?.colors ?? {}) },
    };
  }
  // Editing starts from their face, or their character's.
  if (!face) face = untrack(() => fromPreset(base));
  const F = $derived(face!);

  // ---- settings
  const GROUPS: { id: string; title: string; mods: string[]; shp?: string }[] = [
    { id: 'jaw', title: 'Jaw & chin', mods: ['mod_jaw_height', 'mod_jaw_width', 'mod_underbite', 'mod_chin_size', 'mod_chin_width', 'mod_chin_height'] },
    { id: 'face', title: 'Face shape', mods: [], shp: 'shp_face' },
    { id: 'eyes', title: 'Eyes', mods: ['mod_eyes_size', 'mod_eyes_height', 'mod_eyes_depth', 'mod_eyes_narrow'], shp: 'shp_eyes' },
    { id: 'nose', title: 'Nose', mods: ['mod_nose_size', 'mod_nose_curve', 'mod_nose_tip', 'mod_nose_tip_angle'], shp: 'shp_nose' },
    { id: 'mouth', title: 'Mouth', mods: ['mod_mouth_size', 'mod_mouth_height', 'mod_mouth_depth'], shp: 'shp_mouth' },
    { id: 'brow', title: 'Brow', mods: ['mod_brow_height', 'mod_brow_depth'], shp: 'shp_brow' },
    { id: 'ears', title: 'Ears', mods: [], shp: 'shp_ears' },
  ];
  // The numbered shapes, named from what they do to this body's mesh (lib/faceShapes.ts); loaded with the mesh.
  let shapeInfo = $state<Record<string, ShapeInfo[]>>({});
  $effect(() => {
    const url = `/chartemplates/${B.mesh}`;
    loadCharMesh(url).then((m) => {
      const info: Record<string, ShapeInfo[]> = {};
      for (const g of GROUPS) if (g.shp) info[g.shp] = describeShapes(m, g.shp, g.title);
      shapeInfo = info;
    }).catch(() => {});
  });
  // Resting expression: the mesh's face-tracking shapes. The game never animates them, so what is set stays on the face.
  // Left/right pairs move together; `neg` = the shapes the slider's negative side uses (sideways motions).
  type Expr = { label: string; keys: string[]; neg?: string[] };
  const both = (a: string) => [a + 'Left', a + 'Right'];
  const EXPRESSIONS: { title: string; items: Expr[] }[] = [
    { title: 'Mouth', items: [
      { label: 'Smile', keys: both('mouthSmile') }, { label: 'Frown', keys: both('mouthFrown') },
      { label: 'Dimples', keys: both('mouthDimple') }, { label: 'Stretch', keys: both('mouthStretch') },
      { label: 'Press lips', keys: both('mouthPress') }, { label: 'Pucker', keys: ['mouthPucker'] },
      { label: 'Funnel', keys: ['mouthFunnel'] }, { label: 'Upper lip up', keys: both('mouthUpperUp') },
      { label: 'Lower lip down', keys: both('mouthLowerDown') }, { label: 'Roll upper lip', keys: ['mouthRollUpper'] },
      { label: 'Roll lower lip', keys: ['mouthRollLower'] }, { label: 'Upper lip out', keys: ['mouthShrugUpper'] },
      { label: 'Lower lip out', keys: ['mouthShrugLower'] }, { label: 'Lips closed', keys: ['mouthClose'] },
      { label: 'Mouth sideways', keys: ['mouthRight'], neg: ['mouthLeft'] },
    ] },
    { title: 'Jaw', items: [
      { label: 'Open', keys: ['jawOpen'] }, { label: 'Forward', keys: ['jawForward'] },
      { label: 'Sideways', keys: ['jawRight'], neg: ['jawLeft'] },
    ] },
    { title: 'Cheeks & nose', items: [
      { label: 'Cheek puff', keys: ['cheekPuff'] }, { label: 'Cheek squint', keys: both('cheekSquint') },
      { label: 'Sneer', keys: both('noseSneer') },
    ] },
    { title: 'Eyes', items: [
      { label: 'Squint', keys: both('eyeSquint') }, { label: 'Wide', keys: both('eyeWide') },
      { label: 'Sleepy', keys: both('eyeBlink') },
      { label: 'Look up / down', keys: both('eyeLookUp'), neg: both('eyeLookDown') },
      { label: 'Look sideways', keys: ['eyeLookInLeft', 'eyeLookOutRight'], neg: ['eyeLookOutLeft', 'eyeLookInRight'] },
    ] },
    { title: 'Brows', items: [
      { label: 'Inner brows up', keys: ['browInnerUp'] }, { label: 'Outer brows up', keys: both('browOuterUp') },
      { label: 'Brows down', keys: both('browDown') },
    ] },
  ];
  const exprValue = (e: Expr) => (F.shapes[e.keys[0]] ?? 0) - (e.neg ? F.shapes[e.neg[0]] ?? 0 : 0);
  function setExpr(e: Expr, v: number) {
    for (const k of e.keys) F.shapes[k] = Math.max(v, 0);
    for (const k of e.neg ?? []) F.shapes[k] = Math.max(-v, 0);
  }
  function resetExpression() {
    for (const g of EXPRESSIONS) for (const e of g.items) setExpr(e, 0);
  }
  const shapeKeys = (prefix: string) => Array.from({ length: 9 }, (_, i) => `${prefix}_0${i + 1}`);
  /** Shapes to show: duplicates only when this face already uses them. */
  const shownShapes = (prefix: string) => (shapeInfo[prefix] ?? []).filter((s) => !s.same || (F.shapes[s.key] ?? 0) !== 0);
  const label = (k: string) => {
    const w = k.replace(/^mod_/, '').split('_');
    const s = w.join(' ').replace('tip angle', 'tip angle');
    return s.charAt(0).toUpperCase() + s.slice(1);
  };
  const pretty = (n: string) => (n === 'None' ? 'None' : n.replace(/_0?(\d+)$/, ' $1').replace(/_/g, ' '));
  const rgb = (h: string | undefined) => (h ?? '#ffffff').slice(0, 7);
  const withAlpha = (v: string, old: string | undefined) => v + ((old ?? '').length === 9 ? old!.slice(7) : 'ff');

  let tab = $state<'face' | 'hair' | 'skin' | 'body'>('face');
  let frame = $state<CameraFrame>('face');

  // ---- preview
  let canvas = $state<HTMLCanvasElement>();
  let view: CharacterView | null = null;
  const texUrl = $derived(charTexUrl(chars));
  const look = () => faceLook(B, F);
  const pieces = () => facePieces(B, F);

  let error = $state('');
  onMount(() => {
    if (!canvas || !B) return;
    try { view = new CharacterView(canvas, texUrl); } catch (e: any) { error = String(e?.message ?? e); return; }
    view.setLook(look());
    view.setBody(`/chartemplates/${B.mesh}`, B.parts.map((p) => p.material)).catch((e) => (error = String(e?.message ?? e)));
    const ro = new ResizeObserver(() => view?.draw());
    ro.observe(canvas);
    return () => { ro.disconnect(); view?.dispose(); view = null; };
  });
  // Every change redraws; hair pieces and clothes reload when they change.
  // Reads every slider here (not only inside the view), so each change re-runs this.
  $effect(() => { const l = look(); JSON.stringify(l.shapes); view?.setLook(l); });
  let pieceKey = '';
  $effect(() => {
    const p = pieces(), k = JSON.stringify(p);
    if (view && k !== pieceKey) { pieceKey = k; view.setPieces(p).catch((e) => (error = String(e?.message ?? e))); }
  });
  let clothKey = '';
  $effect(() => {
    const c = clothes, k = JSON.stringify(c);
    if (view && k !== clothKey) { clothKey = k; view.setClothes(c).catch(() => {}); }
  });
  $effect(() => { if (view) { view.frame = frame; view.draw(); } });

  // Drag turns, the wheel zooms.
  let drag: { x: number; y: number } | null = null;
  function down(e: PointerEvent) { drag = { x: e.clientX, y: e.clientY }; (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId); }
  function move(e: PointerEvent) {
    if (!drag || !view) return;
    view.ry += (e.clientX - drag.x) * 0.01;
    view.rx = Math.max(-1.2, Math.min(1.2, view.rx + (e.clientY - drag.y) * 0.01));
    drag = { x: e.clientX, y: e.clientY };
    view.draw();
  }
  function wheel(e: WheelEvent) {
    if (!view) return;
    e.preventDefault();
    view.zoom = Math.max(0.5, Math.min(4, view.zoom * (e.deltaY < 0 ? 1.1 : 1 / 1.1)));
    view.draw();
  }

  // ---- actions
  function reset() { face = fromPreset(base); }
  function randomise() {
    const r = (a: number, b: number) => a + Math.random() * (b - a);
    const pick = <T,>(l: T[]) => l[Math.floor(Math.random() * l.length)];
    const f = fromPreset(base);
    const round = (v: number) => Math.round(v * 100) / 100;
    // Some rolls are mild and some wild, and on any face some features stand out more than others. The slider's whole range, leaning away from the middle.
    const wild = r(0.45, 1);
    for (const g of GROUPS) {
      const feature = wild * r(0.5, 1);
      for (const k of g.mods) f.shapes[k] = round(3 * feature * Math.sign(r(-1, 1)) * Math.sqrt(Math.random()));
      if (g.shp) {
        for (const k of shapeKeys(g.shp)) f.shapes[k] = 0;
        f.shapes[pick(shapeKeys(g.shp))] = round(r(0.5, 3) * feature);
      }
    }
    f.shapes['body_fat'] = round(r(0, 1));
    B.hair.forEach((h, slot) => { f.hair[slot] = pick(h.items).name; });
    const donor = () => pick(B.presets);
    f.hairColor = donor().hairColor ?? f.hairColor;
    for (const k of ['_Skin_Tint', '_Eye_Color', '_Lips_Tint']) f.colors[k] = donor().colors[k] ?? f.colors[k];
    for (const k of ['_Skin_Texture', '_Make_Up']) f.textures[k] = donor().textures[k] ?? f.textures[k];
    for (const k of ['_Skin_Tint_Alpha', '_Freckles', '_Wrinkles', '_Make_Up_Alpha', '_Lipstick_Alpha']) f.floats[k] = donor().floats[k] ?? f.floats[k] ?? 0;
    f.floats['Height'] = round(r(0.9, 1.12));
    f.floats['Width'] = round(r(0.85, 1.15));
    face = f;
  }

  const slotTitle = (table: string) => (table.startsWith('Eyebrows') ? 'Eyebrows' : table.startsWith('Beards') ? 'Beard' : 'Hair');
  const SKINS = ['T_Skin_01', 'T_Skin_02', 'T_Skin_03'];
  const MAKEUP = ['T_Flat_White_sRGB', 'T_Make_Up_01', 'T_Make_Up_02', 'T_Make_Up_03', 'T_Make_Up_04'];
</script>

<div class="fe">
  <header>
    <h2 class="grow">{title}: face &amp; body</h2>
    <div class="seg">
      <button class:on={frame === 'face'} onclick={() => (frame = 'face')}>Face</button>
      <button class:on={frame === 'body'} onclick={() => (frame = 'body')}>Whole body</button>
    </div>
    <button onclick={randomise} title="A new random face and body">🎲 Randomise</button>
    <button onclick={reset} title="Start again from the game's own face for this character">Revert to vanilla</button>
    <button class="primary" onclick={onclose}>Done</button>
  </header>
  <div class="main">
    <div class="stage">
      <canvas bind:this={canvas} onpointerdown={down} onpointermove={move} onpointerup={() => (drag = null)} onwheel={wheel}></canvas>
      {#if error}<p class="err small">{error}</p>{/if}
      <p class="hint muted small">Drag to turn, scroll to zoom. A close look: the game's own lighting and skin look a little different.</p>
    </div>
    <div class="panel">
      <nav class="tabs">
        {#each [['face', 'Face'], ['hair', 'Hair'], ['skin', 'Skin & colours'], ['body', 'Body']] as [id, t]}
          <button class:on={tab === id} onclick={() => (tab = id as any)}>{t}</button>
        {/each}
      </nav>
      <div class="scroll">
        {#if tab === 'face'}
          {#each GROUPS as g (g.id)}
            <details open={g.id === 'jaw'}>
              <summary>{g.title}</summary>
              {#each g.mods as k (k)}
                <label class="sl"><span>{label(k)}</span>
                  <input type="range" min="-3" max="3" step="0.01" bind:value={F.shapes[k]} />
                  <span class="v">{(F.shapes[k] ?? 0).toFixed(2)}</span></label>
              {/each}
              {#if g.shp && shownShapes(g.shp).length}
                <details class="fine">
                  <summary class="muted small">Fine-tune (subtle)</summary>
                  {#each shownShapes(g.shp) as s (s.key)}
                    <label class="sl" title={s.same ? 'Almost the same as another shape here' : s.key}><span>{s.label.replace(`${g.title}: `, '')}</span>
                      <input type="range" min="-1" max="3" step="0.01" bind:value={F.shapes[s.key]} />
                      <span class="v">{(F.shapes[s.key] ?? 0).toFixed(2)}</span></label>
                  {/each}
                </details>
              {/if}
            </details>
          {/each}
          <details>
            <summary>Expression</summary>
            <p class="muted small">A look they always wear: the game never animates faces, so it stays.
              <button class="link" onclick={resetExpression}>Reset expression</button></p>
            {#each EXPRESSIONS as g (g.title)}
              <h4>{g.title}</h4>
              {#each g.items as e (e.label)}
                <label class="sl"><span>{e.label}</span>
                  <input type="range" min={e.neg ? -1 : 0} max="1" step="0.01" value={exprValue(e)}
                    oninput={(ev) => setExpr(e, +ev.currentTarget.value)} />
                  <span class="v">{exprValue(e).toFixed(2)}</span></label>
              {/each}
            {/each}
          </details>
        {:else if tab === 'hair'}
          {#each B.hair as h, slot (h.table)}
            <h4>{slotTitle(h.table)}</h4>
            <div class="chips">
              {#each h.items as it (it.name)}
                <button class="chip" class:on={F.hair[slot] === it.name} onclick={() => (F.hair[slot] = it.name)}>{pretty(it.name)}</button>
              {/each}
            </div>
          {/each}
          <label class="sl"><span>Hair colour</span><input type="color" value={rgb(F.hairColor)}
            oninput={(e) => (F.hairColor = withAlpha(e.currentTarget.value, F.hairColor))} /></label>
          <p class="muted small">The colour goes on the hair, eyebrows, beard and lashes.</p>
        {:else if tab === 'skin'}
          <h4>Skin</h4>
          <div class="chips">
            {#each SKINS as s, i (s)}<button class="chip" class:on={(F.textures['_Skin_Texture'] || 'T_Skin_01') === s} onclick={() => (F.textures['_Skin_Texture'] = s)}>Skin {i + 1}</button>{/each}
          </div>
          <label class="sl"><span>Skin tone</span><input type="color" value={rgb(F.colors['_Skin_Tint'])}
            oninput={(e) => (F.colors['_Skin_Tint'] = withAlpha(e.currentTarget.value, F.colors['_Skin_Tint']))} /></label>
          <label class="sl" title="The game's own characters use 0–1; above 1 is stronger than any of them."><span>Tone strength</span><input type="range" min="0" max="5" step="0.01" bind:value={F.floats['_Skin_Tint_Alpha']} /><span class="v">{(F.floats['_Skin_Tint_Alpha'] ?? 0).toFixed(2)}</span></label>
          <label class="sl"><span>Freckles</span><input type="range" min="0" max="1" step="0.01" bind:value={F.floats['_Freckles']} /><span class="v">{(F.floats['_Freckles'] ?? 0).toFixed(2)}</span></label>
          <label class="sl"><span>Wrinkles</span><input type="range" min="0" max="1" step="0.01" bind:value={F.floats['_Wrinkles']} /><span class="v">{(F.floats['_Wrinkles'] ?? 0).toFixed(2)}</span></label>
          <label class="sl"><span>Skin shine</span><input type="range" min="0" max="1" step="0.01" bind:value={F.floats['_Skin_Smoothness_Offset']} /><span class="v">{(F.floats['_Skin_Smoothness_Offset'] ?? 0).toFixed(2)}</span></label>
          <p class="muted small">Wrinkles and shine show in the game only.</p>
          <h4>Eyes &amp; lips</h4>
          <label class="sl"><span>Eye colour</span><input type="color" value={rgb(F.colors['_Eye_Color'])}
            oninput={(e) => (F.colors['_Eye_Color'] = withAlpha(e.currentTarget.value, F.colors['_Eye_Color']))} /></label>
          <label class="sl"><span>Lip colour</span><input type="color" value={rgb(F.colors['_Lips_Tint'])}
            oninput={(e) => (F.colors['_Lips_Tint'] = withAlpha(e.currentTarget.value, F.colors['_Lips_Tint']))} /></label>
          <label class="sl"><span>Lipstick</span><input type="range" min="0" max="1" step="0.01" bind:value={F.floats['_Lipstick_Alpha']} /><span class="v">{(F.floats['_Lipstick_Alpha'] ?? 0).toFixed(2)}</span></label>
          <h4>Make-up</h4>
          <div class="chips">
            {#each MAKEUP as s, i (s)}<button class="chip" class:on={(F.textures['_Make_Up'] || MAKEUP[0]) === s} onclick={() => (F.textures['_Make_Up'] = s)}>{i === 0 ? 'None' : `Style ${i}`}</button>{/each}
          </div>
          <label class="sl"><span>Make-up amount</span><input type="range" min="0" max="1" step="0.01" bind:value={F.floats['_Make_Up_Alpha']} /><span class="v">{(F.floats['_Make_Up_Alpha'] ?? 0).toFixed(2)}</span></label>
        {:else}
          <label class="sl"><span>Height</span><input type="range" min="0.9" max="1.12" step="0.01" bind:value={F.floats['Height']} /><span class="v">{(F.floats['Height'] ?? 1).toFixed(2)}</span></label>
          <label class="sl"><span>Width</span><input type="range" min="0.85" max="1.15" step="0.01" bind:value={F.floats['Width']} /><span class="v">{(F.floats['Width'] ?? 1).toFixed(2)}</span></label>
          <label class="sl"><span>Body fat</span><input type="range" min="0" max="1" step="0.01" bind:value={F.shapes['body_fat']} /><span class="v">{(F.shapes['body_fat'] ?? 0).toFixed(2)}</span></label>
          {#each [['body_muscular', 'Muscular'], ['body_skinny', 'Skinny'], ['body_curvy', 'Curvy'], ['body_breastsize', 'Chest size']] as [k, t] (k)}
            <label class="sl"><span>{t}</span><input type="range" min="0" max="1" step="0.01" bind:value={F.shapes[k]} /><span class="v">{(F.shapes[k] ?? 0).toFixed(2)}</span></label>
          {/each}
          <p class="muted small">The game's characters stay between about 0.96 and 1.07 tall and 0.9 and 1.1 wide.</p>
        {/if}
      </div>
    </div>
  </div>
</div>

<style>
  .fe { position: fixed; inset: 0; z-index: 50; background: var(--bg); display: flex; flex-direction: column; }
  header { display: flex; gap: 8px; align-items: center; padding: 10px 14px; border-bottom: 1px solid var(--line); }
  header h2 { margin: 0; font-size: 18px; }
  .main { flex: 1; min-height: 0; display: flex; }
  .stage { flex: 1; min-width: 0; position: relative; background: radial-gradient(circle at 50% 40%, #3a4660, #151a24); }
  canvas { width: 100%; height: 100%; display: block; cursor: grab; touch-action: none; }
  .hint { position: absolute; left: 12px; bottom: 8px; margin: 0; }
  .err { position: absolute; left: 12px; top: 8px; color: var(--danger, #f66); }
  .panel { width: 380px; flex: none; display: flex; flex-direction: column; border-left: 1px solid var(--line); }
  .tabs { display: flex; gap: 4px; padding: 8px; flex-wrap: wrap; }
  .tabs button.on, .seg button.on { border-color: var(--accent); background: #22304d; }
  .scroll { flex: 1; overflow-y: auto; padding: 0 12px 12px; }
  details { margin: 6px 0; }
  summary { cursor: pointer; font-weight: 600; padding: 4px 0; }
  .sl { display: flex; gap: 8px; align-items: center; margin: 3px 0; font-size: 13px; }
  .sl > span:first-child { width: 110px; flex: none; }
  .sl input[type='range'] { flex: 1; }
  .v { width: 38px; text-align: right; color: var(--muted); font-size: 12px; }
  h4 { margin: 12px 0 4px; }
  .chips { display: flex; flex-wrap: wrap; gap: 4px; }
  .chip.on { border-color: var(--accent); background: #22304d; }
</style>
