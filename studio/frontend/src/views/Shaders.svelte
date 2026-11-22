<script lang="ts">
  // Shaders page: card shaders written by the player (the setup's item library). Code on the left, the shader running
  // on a card on the right, updated while typing. One draft for the page; Save stores it and compiles every changed
  // shader for the game. Card versions get a shader on a set's Set tab and a card's Versions grid.
  import { onMount, tick, untrack } from 'svelte';
  import { App, ask, errText } from '../lib/api';
  import { accFile, projectFile } from '../lib/urls';
  import SaveBar from './SaveBar.svelte';
  import CodeEditor from './CodeEditor.svelte';
  import ShaderPreview from './ShaderPreview.svelte';
  import ShaderTutorial, { type TutorialStep } from './ShaderTutorial.svelte';
  import { declares, guessKind, isShadertoy, type ShaderMessage, type ShaderParam } from '../lib/cardShaderView';

  let { notify }: { notify: (t: string, k?: string) => void } = $props();

  type Shader = { id: string; name: string; code: string; params?: ShaderParam[]; bundle?: string; built?: string; icon?: string;
    draws?: string; mix?: string; mixStrength?: number };
  type Starter = { name: string; about: string; code: string; params?: ShaderParam[]; draws?: string; mix?: string; mixStrength?: number };
  type Stock = { id: string; name: string; border: string; code: string; params: ShaderParam[] };
  type Problem = { id: string; name: string; messages: string };

  let shaders = $state<Shader[]>([]);
  let starters = $state<Starter[]>([]);
  let stock = $state<Stock[]>([]);
  // This setup's own values for the stock shaders (stock id → parameter → values): the game uses them instead.
  let stockValues = $state<Record<string, Record<string, number[]>>>({});
  // The guided tutorial: its steps, and the shader it is running on (it shows while that shader is selected).
  let tutorialSteps = $state<TutorialStep[]>([]);
  let tutorial = $state<{ id: string; step: number } | null>(null);
  let stockCode = $state('');
  let limits = $state({ values: 16, images: 4 });
  let noTemplate = $state('');
  let baseline = $state('');
  let loading = $state(true);
  let saving = $state(false);
  let selectedId = $state<string | null>(null);
  let query = $state('');
  let nameBox = $state<HTMLInputElement>();
  let picking = $state(false);
  let messages = $state<ShaderMessage[]>([]);
  let paramError = $state('');
  let problems = $state<Problem[]>([]);
  let covers = $state<{ name: string; url: string }[]>([]);
  let art = $state('');
  let preview = $state<ReturnType<typeof ShaderPreview>>();
  let help = $state(true);

  /** JSON with sorted keys, so the same values always give the same text. */
  const stable = (v: unknown): string => JSON.stringify(v, (_k, x) =>
    x && typeof x === 'object' && !Array.isArray(x) ? Object.fromEntries(Object.entries(x).sort(([a], [b]) => a.localeCompare(b))) : x);
  const snapshot = () => stable([shaders, stockValues]);
  const dirty = $derived(!loading && snapshot() !== baseline);
  const selected = $derived(shaders.find((s) => s.id === selectedId) ?? null);
  /** A stock shader being looked at (read-only). */
  const viewing = $derived(stock.find((s) => s.id === selectedId) ?? null);
  $effect(() => { stockCode = viewing?.code ?? ''; });
  // A stock shader's values: the stock ones with this setup's own on top. Changing them changes what the game uses (after
  // Save); Reset values goes back to the stock ones; Duplicate makes a shader of your own with them.
  let tryParams = $state<ShaderParam[]>([]);
  const withOwn = (s: Stock): ShaderParam[] =>
    s.params.map((p) => ({ ...p, value: [...(stockValues[s.id]?.[p.name] ?? p.value ?? [])] }));
  let tryFor = ''; // the stock shader tryParams were filled for
  $effect(() => { const v = viewing; untrack(() => { tryParams = v ? withOwn(v) : []; tryFor = v?.id ?? ''; }); });
  // Keep stockValues in step with the sliders: only values that differ from the stock ones are kept.
  $effect(() => {
    const v = viewing;
    const now = JSON.parse(JSON.stringify(tryParams)) as ShaderParam[];
    untrack(() => {
      if (!v || v.id !== tryFor || now.length !== v.params.length) return;
      const own: Record<string, number[]> = {};
      v.params.forEach((p, i) => {
        const val = now[i]?.value;
        if (p.kind !== 'image' && val && p.value && val.length === p.value.length && val.some((x, j) => Math.abs(x - p.value![j]) > 1e-6)) own[p.name] = val;
      });
      if (stable(own) === stable(stockValues[v.id] ?? {})) return;
      const next = { ...stockValues };
      if (Object.keys(own).length) next[v.id] = own; else delete next[v.id];
      stockValues = next;
    });
  });
  const tried = $derived(!!viewing && !!stockValues[viewing.id]);
  const shown = $derived(shaders.filter((s) => !query || s.name.toLowerCase().includes(query.toLowerCase())));
  const valueCount = $derived((selected?.params ?? []).filter((p) => p.kind !== 'image').length);
  const imageCount = $derived((selected?.params ?? []).filter((p) => p.kind === 'image').length);
  const problemOf = $derived(problems.find((p) => p.id === selectedId));

  async function load() {
    loading = true;
    try {
      const page = await App.LoadShadersPage();
      shaders = (page.shaders ?? []) as unknown as Shader[];
      starters = (page.starters ?? []) as unknown as Starter[];
      stock = (page.stock ?? []) as unknown as Stock[];
      stockValues = (page.stockValues ?? {}) as Record<string, Record<string, number[]>>;
      tutorialSteps = (page.tutorial ?? []) as unknown as TutorialStep[];
      limits = { values: page.maxValues, images: page.maxImages };
      noTemplate = page.noTemplate ?? '';
      if (!shaders.some((s) => s.id === selectedId) && !stock.some((s) => s.id === selectedId)) selectedId = shaders[0]?.id ?? null;
      const sets = await App.ListProjects();
      covers = (sets ?? []).filter((s: any) => s.cover).map((s: any) => ({ name: s.name, url: projectFile(s.id, s.cover) }));
      if (!covers.some((c) => c.url === art)) art = covers[0]?.url ?? placeholderArt();
    } catch (e) {
      notify(errText(e), 'error');
    }
    loading = false;
    problems = [];
    baseline = snapshot();
  }

  /** A stand-in picture when the setup has no sets yet. */
  function placeholderArt(): string {
    const c = document.createElement('canvas');
    c.width = 500; c.height = 700;
    const g = c.getContext('2d')!;
    const grad = g.createLinearGradient(0, 0, 500, 700);
    grad.addColorStop(0, '#3b5b8c'); grad.addColorStop(0.5, '#8c4f7d'); grad.addColorStop(1, '#d9a14a');
    g.fillStyle = grad;
    g.beginPath(); g.roundRect(0, 0, 500, 700, 28); g.fill();
    g.fillStyle = 'rgba(255,255,255,0.85)';
    g.beginPath(); g.arc(250, 300, 120, 0, Math.PI * 2); g.fill();
    g.fillStyle = 'rgba(0,0,0,0.35)';
    g.fillRect(40, 520, 420, 130);
    return c.toDataURL('image/png');
  }

  async function save() {
    const unnamed = shaders.find((s) => !s.name?.trim());
    if (unnamed) {
      query = '';
      selectedId = unnamed.id;
      await tick();
      nameBox?.focus();
      notify("There's a shader without a name: selected it. Name it or delete it, then save.", 'error');
      return;
    }
    saving = true;
    try {
      // The list picture of the shader on screen, when it is running.
      const frame = selected && preview ? preview.snapshot() : '';
      if (selected && frame) selected.icon = frame;
      const res = await App.SaveShaders(shaders as any, stockValues as any);
      const found = (res.problems ?? []) as Problem[];
      await load();
      problems = found;
      const warns = res.warnings ?? [];
      if (found.length) {
        notify(`Saved, but ${found.length === 1 ? `"${found[0].name}" has` : `${found.length} shaders have`} mistakes in the code: the game keeps drawing the last version that worked.`, 'warn');
        if (!found.some((p) => p.id === selectedId)) selectedId = found[0].id;
      } else {
        notify(warns.length ? `Saved. ${warns.join(' ')}` : 'Saved. The game uses it at its next start.', warns.length ? 'warn' : 'ok');
      }
    } catch (e) {
      notify(errText(e), 'error');
    }
    saving = false;
  }

  /** New shader = the plain picture (the first starter); the other starters are the examples. */
  const blank = (): Starter => starters[0] ?? { name: '', about: '', code: 'vec4 card(vec2 uv) { return art(uv); }' };

  async function add(st: Starter) {
    picking = false;
    try {
      const id = await App.NewShaderID();
      shaders.push({ id, name: st.name === starters[0]?.name ? '' : st.name, code: st.code, params: JSON.parse(JSON.stringify(st.params ?? [])),
        draws: st.draws, mix: st.mix, mixStrength: st.mixStrength });
      selectedId = id;
      query = '';
      await tick();
      nameBox?.focus();
      nameBox?.select();
    } catch (e) {
      notify(errText(e), 'error');
    }
  }

  /** The parameter the tutorial's current step is about (the one the step adds), outlined in the Parameters list. */
  const spotParam = $derived.by(() => {
    if (!tutorial || tutorial.id !== selectedId) return '';
    const now = tutorialSteps[tutorial.step]?.params ?? [], before = tutorial.step > 0 ? tutorialSteps[tutorial.step - 1]?.params ?? [] : [];
    return now.length > before.length ? (now[now.length - 1].name as string) : '';
  });

  /** Starts the tutorial on a new shader (the plain picture, which is its first step). */
  async function startTutorial() {
    picking = false;
    try {
      const id = await App.NewShaderID();
      shaders.push({ id, name: 'My first shader', code: tutorialSteps[0]?.code ?? blank().code, params: [] });
      selectedId = id;
      query = '';
      tutorial = { id, step: 0 };
    } catch (e) {
      notify(errText(e), 'error');
    }
  }

  /** "Do it for me": the step's code, its parameters (the ones already there keep their values) and its Draws setting. */
  function applyStep(s: TutorialStep) {
    if (!selected) return;
    selected.code = s.code;
    const params = (selected.params ??= []);
    for (const p of s.params ?? []) if (!params.some((x) => x.name === p.name)) params.push(JSON.parse(JSON.stringify(p)));
    if (s.draws) selected.draws = s.draws;
  }

  async function duplicate(s: { name: string; code: string; params?: ShaderParam[]; draws?: string; mix?: string; mixStrength?: number }) {
    const id = await App.NewShaderID();
    shaders.push({ id, name: `${s.name} copy`, code: s.code, params: JSON.parse(JSON.stringify(s.params ?? [])),
      draws: s.draws, mix: s.mix, mixStrength: s.mixStrength });
    selectedId = id;
  }

  async function remove(s: Shader) {
    if (!(await ask(`Delete ${s.name ? `"${s.name}"` : 'this shader'}? Card versions that use it are drawn normally again. It stays in the Catalog, from where it can be added back.`))) return;
    shaders = shaders.filter((x) => x.id !== s.id);
    selectedId = shaders[0]?.id ?? null;
  }

  // ---- settings: what the shader draws on, and how Shadertoy code is mixed with the picture
  const DRAWS = [
    { id: '', label: "The card's picture", about: "The shader draws the picture. Where it returns less than full alpha, the game's own card frame under the picture shows." },
    { id: 'seeThrough', label: 'The whole card, see-through', about: 'The rest of the card front (frame, border, text) is hidden while the shader draws. Where it returns less than full alpha you see what is behind the card: use it for dissolving, fading or ghost cards.' },
    { id: 'over', label: 'On top of the card', about: "The shader is drawn over the whole card front, frame and text included; the picture under it stays as it is. Return alpha 0 where nothing should cover the card, and add light with cardGlow." },
  ];
  const MIXES = [
    { id: 'add', label: 'Add light' }, { id: 'screen', label: 'Screen (soft light)' }, { id: 'overlay', label: 'Overlay' },
    { id: 'multiply', label: 'Multiply (darken)' }, { id: 'replace', label: 'Replace the picture' },
  ];
  const toy = $derived(!!selected && isShadertoy(selected.code));

  // ---- parameters
  const KINDS: { id: ShaderParam['kind']; label: string }[] = [
    { id: 'number', label: 'Number' }, { id: 'vec2', label: 'Two numbers (vec2)' }, { id: 'colour', label: 'Colour (vec4)' }, { id: 'image', label: 'Picture (sampler2D)' },
  ];
  const firstValue = (kind: string): number[] | undefined => ({ number: [0.5], vec2: [0, 0], colour: [1, 1, 1, 1] } as Record<string, number[]>)[kind];

  // Parameters follow the code: a name the code uses that nothing declares becomes a parameter once typing stops, of
  // the kind its use suggests. One made this way that the code no longer mentions, and whose value was never changed,
  // goes again (a half-typed name leaves nothing behind). Parameters added or changed by hand are never touched.
  let autoMade: Record<string, string[]> = {}; // shader id → names made here
  const untouched = (p: ShaderParam) => !p.image && JSON.stringify(p.value) === JSON.stringify(firstValue(p.kind));
  function followCode(s: Shader, found: ShaderMessage[]) {
    const params = (s.params ??= []);
    const mine = new Set(autoMade[s.id] ?? []);
    for (let i = params.length - 1; i >= 0; i--) {
      const p = params[i];
      // Gone from the code, or the code now makes a variable of that name itself (the name was typed before its own
      // "float name = …" line): the parameter would only get in the way.
      if (mine.has(p.name) && untouched(p) && (!new RegExp(`\\b${p.name}\\b`).test(s.code) || declares(s.code, p.name))) {
        params.splice(i, 1);
        mine.delete(p.name);
      }
    }
    for (const name of new Set(found.map((m) => m.unknown).filter((n): n is string => !!n))) {
      if (params.some((p) => p.name === name)) continue;
      const kind = guessKind(s.code, name);
      const room = kind === 'image' ? params.filter((p) => p.kind === 'image').length < limits.images
        : params.filter((p) => p.kind !== 'image').length < limits.values;
      if (!room) continue;
      params.push(kind === 'number' ? { name, kind, value: [0.5], min: 0, max: 1 } : { name, kind, value: firstValue(kind) });
      mine.add(name);
    }
    autoMade[s.id] = [...mine];
  }
  $effect(() => {
    const s = selected, found = messages;
    if (!s || paramError) return;
    void s.code;
    const timer = setTimeout(() => followCode(s, found), 900);
    return () => clearTimeout(timer);
  });

  /** Parameters whose name the code also makes a variable of: the two clash, and the compiler's message doesn't say so. */
  const clashes = $derived((selected?.params ?? []).filter((p) => p.name && declares(selected!.code, p.name)).map((p) => p.name));
  function dropParam(name: string) {
    const i = selected?.params?.findIndex((p) => p.name === name) ?? -1;
    if (i >= 0) selected!.params!.splice(i, 1);
  }

  function addParam() {
    if (!selected) return;
    const used = new Set((selected.params ?? []).map((p) => p.name));
    let n = 1;
    while (used.has(`value${n}`)) n++;
    (selected.params ??= []).push({ name: `value${n}`, kind: 'number', value: [0.5], min: 0, max: 1 });
  }
  function setKind(p: ShaderParam, kind: ShaderParam['kind']) {
    p.kind = kind;
    p.value = firstValue(kind);
    p.image = undefined;
    if (kind === 'number') { p.min = 0; p.max = 1; } else { p.min = undefined; p.max = undefined; }
  }
  function removeParam(i: number) { selected?.params?.splice(i, 1); }

  const hex = (v: number[] = [1, 1, 1]) => '#' + v.slice(0, 3).map((x) => Math.round(Math.min(1, Math.max(0, x)) * 255).toString(16).padStart(2, '0')).join('');
  function setColour(p: ShaderParam, h: string) {
    const n = parseInt(h.slice(1), 16);
    p.value = [((n >> 16) & 255) / 255, ((n >> 8) & 255) / 255, (n & 255) / 255, p.value?.[3] ?? 1];
  }
  async function pickImage(p: ShaderParam) {
    try {
      const ref = await App.PickShaderImage();
      if (ref) p.image = ref;
    } catch (e) {
      notify(errText(e), 'error');
    }
  }
  const usage = (p: ShaderParam) => (p.kind === 'image' ? `texture(${p.name}, uv)` : p.kind === 'colour' ? `${p.name}.rgb` : p.name);

  onMount(load);
</script>

{#snippet valueControls(p: ShaderParam)}
  {#if p.kind === 'number' && p.value}
    <div class="row">
      <input type="range" class="grow" min={p.min ?? 0} max={p.max ?? 1} step="any" bind:value={p.value[0]} />
      <input type="number" class="num" step="any" bind:value={p.value[0]} />
    </div>
  {:else if p.kind === 'vec2' && p.value}
    <div class="row">x <input type="number" class="num" step="any" bind:value={p.value[0]} /> y <input type="number" class="num" step="any" bind:value={p.value[1]} /></div>
  {:else if p.kind === 'colour' && p.value}
    <div class="row">
      <input type="color" value={hex(p.value)} oninput={(e) => setColour(p, e.currentTarget.value)} />
      <span class="muted small">Strength (alpha)</span>
      <input type="range" class="grow" min="0" max="1" step="0.01" bind:value={p.value[3]} />
    </div>
  {/if}
{/snippet}

<SaveBar {dirty} {saving} onsave={save} onrevert={load}
  leaveMessage="Leave Shaders without saving? Your unsaved changes will be lost."
  revertMessage="Throw away every unsaved change to your shaders?">
  {#snippet title()}<h2>Shaders</h2>{/snippet}
</SaveBar>
<div class="page libpage shaders">
  <div class="intro">
    <span class="muted">A shader is a small program that draws a card's picture: holo effects, light sweeps, anything you can write.
      Give one to card versions on a set's <b>Set</b> tab (all cards) or a card's <b>Versions</b> grid. On framed sets it draws the artwork inside the game's frame.</span>
  </div>
  {#if noTemplate}
    <p class="notice bar">{noTemplate}. You can write and preview shaders; the game will not show them yet.</p>
  {/if}

  {#if loading}
    <p class="muted emptypane">Loading…</p>
  {:else}
    <div class="cols">
      <aside class="lside sidelist">
        <div class="sidetools"><input type="search" bind:value={query} placeholder="Search shaders…" /></div>
        <div class="list" role="listbox" tabindex="-1">
          {#each shown as s (s.id)}
            <button class="item" class:active={s.id === selectedId} onclick={() => (selectedId = s.id)}>
              {#if s.icon}<img class="thumb" src={s.icon.startsWith('data:') ? s.icon : accFile(s.icon)} alt="" />
              {:else}<span class="thumb blank">fx</span>{/if}
              <span class="info">
                <span class="name">{s.name || 'Unnamed'}</span>
                <span class="sub">{problems.some((p) => p.id === s.id) ? 'has mistakes' : !s.bundle && !noTemplate && baseline.includes(s.id) ? 'not in the game yet' : `${(s.params ?? []).length} parameter${(s.params ?? []).length === 1 ? '' : 's'}`}</span>
              </span>
            </button>
          {:else}
            <p class="muted small empty">{query ? 'Nothing matches.' : 'No shaders of your own yet.'}</p>
          {/each}
          <div class="grouphead muted small" title="The holo foils foil cards get when nothing else is chosen for them. Change their values here, or duplicate one to change its code.">Comes with the mod</div>
          {#each stock.filter((s) => !query || s.name.toLowerCase().includes(query.toLowerCase())) as s (s.id)}
            <button class="item" class:active={s.id === selectedId} onclick={() => (selectedId = s.id)}>
              <span class="thumb blank" style:background="rgb({(s.params.find((p) => p.kind === 'colour')?.value ?? [0.55, 0.55, 0.6]).slice(0, 3).map((v) => Math.round(v * 255)).join(',')})"></span>
              <span class="info"><span class="name">{s.name}</span><span class="sub">{stockValues[s.id] ? 'stock · your values' : 'stock'}</span></span>
            </button>
          {/each}
        </div>
        <div class="sidefoot">
          <button class="primary" onclick={() => add(blank())}>New shader</button>
          <button onclick={() => (picking = !picking)} title="Ready-made shaders to look at and change">Examples…</button>
          {#if tutorialSteps.length}<button onclick={startTutorial} title="Build your first shader step by step, on a new shader">Tutorial</button>{/if}
          {#if picking}
            <div class="starters">
              {#each starters.slice(1) as st}
                <button onclick={() => add(st)}><b>{st.name}</b><span class="muted small">{st.about}</span></button>
              {/each}
            </div>
          {/if}
        </div>
      </aside>

      {#if selected}
        {#key selected.id}
          <section class="codepane">
            <div class="row">
              <label class="field grow">Name<input bind:this={nameBox} bind:value={selected.name} placeholder="Name of the shader" /></label>
              <button onclick={() => duplicate(selected!)}>Duplicate</button>
              <button class="danger" onclick={() => remove(selected!)}>Delete</button>
            </div>
            {#if tutorial && tutorial.id === selected.id && tutorialSteps.length}
              <ShaderTutorial steps={tutorialSteps} bind:step={tutorial.step} code={selected.code} draws={selected.draws ?? ''}
                running={!messages.length && !paramError} onapply={applyStep} onclose={() => (tutorial = null)} />
            {/if}
            <CodeEditor bind:value={selected.code} messages={messages} />
            {#each clashes as name}
              <div class="notice err clash"><span class="grow"><b>{name}</b> is one of your parameters (on the right) and also a variable your code makes
                (<code>float {name} = …</code>). They can't share a name; that is what the message below is about.</span>
                <button class="small" onclick={() => dropParam(name)}>Remove the parameter</button></div>
            {/each}
            {#if paramError}
              <p class="notice err">{paramError}</p>
            {:else if messages.length}
              <div class="notice err msgs">
                {#each messages as m}<div><b>Line {m.line}</b> {m.text}</div>{/each}
              </div>
            {:else if problemOf}
              <div class="notice err msgs"><b>The game's compiler refused this code</b> (the preview accepts it):<pre>{problemOf.messages}</pre></div>
            {:else}
              <p class="muted small ok">No mistakes found. The preview shows what you typed.</p>
            {/if}
            <button class="link" onclick={() => (help = !help)}>{help ? 'Hide' : 'Show'} what a shader can use</button>
            {#if help && !(tutorial && tutorial.id === selected.id)}
              <div class="help small">
                <h4>The function</h4>
                <p>Write GLSL (the language of Shadertoy and WebGL). The shader needs this function; it runs for every pixel of the card
                  and returns that pixel's colour (rgb, 0..1) and how solid it is (a):</p>
                <pre>vec4 card(vec2 uv) {'{'}
    vec4 picture = art(uv);
    return picture;   // rgb + alpha
{'}'}</pre>
                <p>Return the picture's own alpha (<code>art(uv).a</code>) to keep the card's rounded corners. You can write your own
                  helper functions above <code>card</code>. Mistakes show under the editor with their line while you type.</p>

                <h4>What the code can read</h4>
                <table><tbody>
                  <tr><td><code>uv</code></td><td>vec2</td><td>the pixel's place on the card: (0, 0) bottom left, (1, 1) top right</td></tr>
                  <tr><td><code>art(uv)</code></td><td>vec4</td><td>the card's picture at a place, with alpha; places outside the card give its edge</td></tr>
                  <tr><td><code>iTime</code></td><td>float</td><td>seconds passing: the clock for anything that moves</td></tr>
                  <tr><td><code>iTilt</code></td><td>vec2</td><td>how far the card is seen from the side (0 = straight on, about ±0.6 from the side); in the preview: the mouse</td></tr>
                  <tr><td><code>iFacing</code></td><td>float</td><td>1 seen straight on, towards 0 seen edge-on</td></tr>
                  <tr><td><code>iResolution</code></td><td>vec3</td><td>the picture's size in pixels (xy)</td></tr>
                  <tr><td><code>iSeed</code></td><td>float</td><td>a random number 0..1 of this copy on screen: add it to the time or a position so copies side by side don't move in step</td></tr>
                  <tr><td><code>iCard</code></td><td>float</td><td>a random number 0..1 of the card itself, the same for every copy: a pattern of its own per card</td></tr>
                  <tr><td><code>iShown</code></td><td>float</td><td>seconds since this card appeared (pack reveal, binder page turned to it): for effects that play once</td></tr>
                  <tr><td><code>iHover</code></td><td>float</td><td>seconds the pointer has been on the card, 0 while it is not (3D cards: binder, pack opening)</td></tr>
                  <tr><td><code>iBorder</code></td><td>int</td><td>0 Base, 1 First Edition, 2 Silver, 3 Gold, 4 EX, 5 Full Art</td></tr>
                  <tr><td><code>iFoil</code></td><td>bool</td><td>the version is foil</td></tr>
                  <tr><td><code>iRarity</code></td><td>int</td><td>the rarity's place in the set's list: 0 = the first (most common)</td></tr>
                  <tr><td><code>iGrade</code></td><td>int</td><td>0 = not graded, else the grade 1..10</td></tr>
                  <tr><td><code>cardMask(uv)</code></td><td>float</td><td>the card's mask picture: 1 where effects belong, 0 where not; 1 everywhere on cards without a mask. Multiply your effect by it</td></tr>
                </tbody></table>
                <p>When the code reads one of the last four, a "Shown as" row appears under the preview to set border, foil, rarity and grade; Restart makes the card appear again (new <code>iSeed</code>, <code>iShown</code> from 0).</p>

                <h4>Light and see-through</h4>
                <table><tbody>
                  <tr><td><code>cardGlow += …;</code></td><td>vec3 of light added on top of what <code>card</code> returns, also where that is see-through (sparks, glints, burning edges). Write it inside <code>card</code></td></tr>
                  <tr><td>alpha below 1</td><td>the pixel is see-through. What shows there depends on <b>Draws</b> (right of the editor):</td></tr>
                </tbody></table>
                <ul>
                  <li><b>The card's picture</b>: the shader replaces the picture; the game's card frame is still under it.</li>
                  <li><b>The whole card, see-through</b>: frame, border and text are hidden, so see-through pixels show what is behind the card (dissolve, fade, ghost).</li>
                  <li><b>On top of the card</b>: drawn over frame and text; return alpha 0 where the card should stay as it is.</li>
                </ul>

                <h4>Parameters</h4>
                <p>Use a new name in the code and it appears on the right by itself after a moment; set its value there without touching the code.
                  Up to {limits.values} values and {limits.images} pictures per shader.</p>
                <table><tbody>
                  <tr><td>Number</td><td><code>speed</code></td><td>float, with a slider (set its range under it)</td></tr>
                  <tr><td>Two numbers</td><td><code>shift.xy</code></td><td>vec2</td></tr>
                  <tr><td>Colour</td><td><code>tint.rgb</code>, <code>tint.a</code></td><td>vec4; alpha = the Strength slider</td></tr>
                  <tr><td>Picture</td><td><code>texture(pattern, uv)</code></td><td>sampler2D; repeats when uv goes past 0..1</td></tr>
                </tbody></table>

                <h4>Shadertoy shaders</h4>
                <p>Paste a shader from shadertoy.com as it is (its <code>void mainImage(out vec4 fragColor, in vec2 fragCoord)</code>, without a <code>card</code> function).
                  Its picture is then mixed with the card's: <b>Mix with the picture</b> and <b>Strength</b> appear on the right (Add light, Screen, Overlay, Multiply, or Replace the picture).
                  A card's mask limits where it shows.</p>
                <p><code>iTime</code>, <code>iResolution</code>, <code>iFrame</code>, <code>iTimeDelta</code>, <code>iFrameRate</code> and <code>iDate</code> work.
                  <code>iChannel0</code> is the card's picture, <code>iChannel1</code>…<code>iChannel3</code> are your first three picture parameters, <code>iMouse</code> follows the card's tilt.
                  A shader with a <b>Common</b> tab works: paste that tab's code above the Image tab's. Shaders with <b>Buffer A–D</b> tabs do not
                  (there is nowhere to add them), nor do sound, keyboard, video and cube maps.
                  To decide the mixing yourself, add a <code>card</code> function that calls <code>mainImage(colour, uv * iResolution.xy)</code>.</p>

                <h4>Using it</h4>
                <ul>
                  <li>New here? <b>Tutorial</b> (bottom left) builds a first shader with you, one idea at a time.</li>
                  <li>Give a shader to card versions on a set's <b>Set</b> tab (all cards) or on a card's <b>Shader</b> tab (that card, per version).</li>
                  <li>A card's <b>Shader</b> tab also holds its <b>mask picture</b> and its <b>own values</b> for the shader's parameters.</li>
                  <li>Save compiles the shader for the game; the game uses it at its next start. The game's compiler (Direct3D) is stricter
                    than the preview: what it refuses is listed here after Save, and the game keeps the last version that worked.</li>
                  <li>Shaders travel with a set you export, and can be exported on their own from the Catalog.</li>
                </ul>
              </div>
            {/if}
          </section>

          <aside class="rside">
            <div class="pin">
              <div class="row">
                <label class="field grow">Shown on
                  <select bind:value={art}>
                    {#each covers as c}<option value={c.url}>{c.name}</option>{/each}
                    {#if !covers.length}<option value={art}>Sample picture</option>{/if}
                  </select>
                </label>
              </div>
              <ShaderPreview bind:this={preview} code={selected.code} params={selected.params ?? []} {art} bind:messages bind:paramError
                look={{ draws: selected.draws, mix: selected.mix, mixStrength: selected.mixStrength }} />
            </div>

            <label class="field">Draws
              <select value={selected.draws ?? ''} onchange={(e) => (selected!.draws = e.currentTarget.value || undefined)}>
                {#each DRAWS as d}<option value={d.id}>{d.label}</option>{/each}
              </select>
            </label>
            <p class="muted small">{DRAWS.find((d) => d.id === (selected.draws ?? ''))?.about}</p>
            {#if toy}
              <label class="field" title="Shadertoy shaders draw a whole picture of their own. This is how that picture goes onto the card's.">Mix with the picture
                <select value={selected.mix ?? 'add'} onchange={(e) => (selected!.mix = e.currentTarget.value)}>
                  {#each MIXES as m}<option value={m.id}>{m.label}</option>{/each}
                </select>
              </label>
              <div class="row">
                <span class="muted small">Strength</span>
                <input type="range" class="grow" min="0.05" max="1" step="0.05" value={selected.mixStrength || 1}
                  oninput={(e) => (selected!.mixStrength = Number(e.currentTarget.value))} />
                <span class="small">{Math.round((selected.mixStrength || 1) * 100)}%</span>
              </div>
            {/if}

            <div class="row phead">
              <h3 class="grow">Parameters</h3>
              <span class="muted small">{valueCount}/{limits.values} values · {imageCount}/{limits.images} pictures</span>
            </div>
            <p class="muted small">Values your code uses by name. Use a new name in the code and it appears here by itself; change the values here without touching the code.</p>
            {#each selected.params ?? [] as p, i (i)}
              <div class="param" class:spot={!!spotParam && p.name === spotParam}>
                <div class="row">
                  <input class="pname" bind:value={p.name} spellcheck="false" title="The name your code uses" />
                  <select value={p.kind} onchange={(e) => setKind(p, e.currentTarget.value as ShaderParam['kind'])}>
                    {#each KINDS as k}<option value={k.id}>{k.label}</option>{/each}
                  </select>
                  <button class="x" title="Remove this parameter" onclick={() => removeParam(i)}>✕</button>
                </div>
                {@render valueControls(p)}
                {#if p.kind === 'number' && p.value}
                  <div class="row muted small">Slider from <input type="number" class="num" step="any" bind:value={p.min} /> to <input type="number" class="num" step="any" bind:value={p.max} /></div>
                {:else if p.kind === 'image'}
                  <div class="row">
                    {#if p.image}<img class="pimg" src={accFile(p.image)} alt="" />{/if}
                    <button onclick={() => pickImage(p)}>{p.image ? 'Change picture…' : 'Choose picture…'}</button>
                    {#if !p.image}<span class="muted small">White until you choose one.</span>{/if}
                  </div>
                {/if}
                <div class="muted small">In the code: <code>{usage(p)}</code></div>
              </div>
            {/each}
            <button onclick={addParam} disabled={valueCount >= limits.values && imageCount >= limits.images}>Add parameter</button>
          </aside>
        {/key}
      {:else if viewing}
        {#key viewing.id}
          <section class="codepane">
            <div class="row">
              <div class="grow"><b>{viewing.name}</b>
                <div class="muted small">Comes with the mod: {#if viewing.border}foil versions of {viewing.border} cards are drawn with it unless a set or card chooses
                  something else{:else}made for the shader layer of a border (Borders page): put it on top of a picture layer{/if}. Its code can't be changed, but its values can: the sliders on the right are what the game uses for this setup
                  after Save, and <b>Reset values</b> goes back to the mod's own. <b>Duplicate</b> makes a shader of your own with them.</div></div>
              <button onclick={() => duplicate({ ...viewing!, params: tryParams })}>Duplicate{tried ? ' with these values' : ' to edit'}</button>
            </div>
            <CodeEditor bind:value={stockCode} readonly />
          </section>
          <aside class="rside">
            <div class="pin">
              <label class="field">Shown on
                <select bind:value={art}>
                  {#each covers as c}<option value={c.url}>{c.name}</option>{/each}
                  {#if !covers.length}<option value={art}>Sample picture</option>{/if}
                </select>
              </label>
              <ShaderPreview code={viewing.code} params={tryParams} {art} look={{}} />
            </div>
            <div class="row phead">
              <h3 class="grow">Values{tried ? ' (yours)' : ''}</h3>
              <button class="small" disabled={!tried} title="Back to the values that come with the mod (Save to use them in the game)"
                onclick={() => (tryParams = JSON.parse(JSON.stringify(viewing!.params)))}>Reset values</button>
            </div>
            {#each tryParams as p, i (i)}
              <div class="param">
                <code>{p.name}</code>
                {@render valueControls(p)}
              </div>
            {/each}
          </aside>
        {/key}
      {:else}
        <div class="emptypane">
          <h1>Write your first shader</h1>
          <p class="muted">A new shader starts as the plain picture; write your own or paste one from shadertoy.com over it. You see it run on a card while you type.</p>
          <div class="row">
            {#if tutorialSteps.length}<button class="primary" onclick={startTutorial}>Start the tutorial</button>{/if}
            <button class:primary={!tutorialSteps.length} onclick={() => add(blank())}>New shader</button>
            <button onclick={() => (picking = true)}>Examples…</button></div>
        </div>
      {/if}
    </div>
  {/if}
</div>

<style>
  .shaders { display: flex; flex-direction: column; min-height: 0; flex: 1; }
  .cols { flex: 1; min-height: 0; display: flex; }
  .lside { width: 250px; flex-shrink: 0; border-right: 1px solid var(--line); display: flex; flex-direction: column; min-height: 0; }
  .codepane { flex: 1; min-width: 0; min-height: 0; display: flex; flex-direction: column; gap: 10px; padding: 14px 16px; }
  .rside { width: 340px; flex-shrink: 0; border-left: 1px solid var(--line); overflow: auto; padding: 14px 16px 24px; display: flex; flex-direction: column; gap: 10px; }
  .thumb { width: 34px; height: 48px; object-fit: cover; border-radius: 4px; flex-shrink: 0; }
  .thumb.blank { display: flex; align-items: center; justify-content: center; background: var(--panel-2); color: var(--muted); font-size: 11px; }
  .info { display: flex; flex-direction: column; min-width: 0; }
  .info .name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .info .sub { font-size: 11px; color: var(--muted); }
  /* The examples open in the side list's footer: they scroll on their own so the footer never grows past the window. */
  .starters { display: flex; flex-direction: column; gap: 4px; max-height: 45vh; overflow-y: auto; min-height: 0; }
  .grouphead { padding: 10px 8px 4px; text-transform: uppercase; letter-spacing: 0.06em; font-size: 11px; }
  /* The card stays in sight while the parameters under it scroll. */
  .pin { position: sticky; top: -14px; z-index: 2; margin: -14px -16px 0; padding: 14px 16px 8px; background: var(--bg);
    border-bottom: 1px solid var(--line); display: flex; flex-direction: column; gap: 10px; }
  .starters button { display: flex; flex-direction: column; align-items: flex-start; gap: 2px; text-align: left; }
  .small { font-size: 12px; }
  .clash { display: flex; align-items: center; gap: 10px; }
  .msgs { display: flex; flex-direction: column; gap: 2px; max-height: 120px; overflow: auto; }
  .msgs pre { margin: 4px 0 0; white-space: pre-wrap; font-size: 12px; }
  .ok { margin: 0; }
  .link { align-self: flex-start; background: none; border: none; padding: 0; color: var(--accent); cursor: pointer; }
  .help { background: var(--panel); border: 1px solid var(--line); border-radius: 8px; padding: 8px 12px; max-height: 260px; overflow: auto; flex-shrink: 0; }
  .help h4 { margin: 10px 0 2px; font-size: 13px; }
  .help h4:first-child { margin-top: 0; }
  .help ul { margin: 4px 0; padding-left: 18px; }
  .help p { margin: 4px 0; }
  .help pre { margin: 4px 0; }
  .help td { padding: 2px 10px 2px 0; vertical-align: top; }
  .phead { margin-top: 6px; }
  .phead h3 { margin: 0; font-size: 14px; }
  .param { display: flex; flex-direction: column; gap: 6px; padding: 8px; border: 1px solid var(--line); border-radius: 8px; background: var(--panel); }
  .param.spot { border-color: var(--accent); box-shadow: 0 0 0 2px color-mix(in srgb, var(--accent) 35%, transparent); }
  .pname { flex: 1; min-width: 0; font-family: Consolas, monospace; }
  .num { width: 76px; }
  .x { padding: 2px 8px; }
  .pimg { width: 40px; height: 40px; object-fit: cover; border-radius: 4px; }
  code { font-family: Consolas, monospace; }
</style>
