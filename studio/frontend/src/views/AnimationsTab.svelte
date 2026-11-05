<script lang="ts" module>
  /** setfmt.Animation: keyed local rotations per bone name. */
  export type Animation = { id: string; name: string; length: number; mask: string; bones: Record<string, { t: number; r: number[] }[]>; femaleBones?: Record<string, { t: number; r: number[] }[]> };
</script>

<script lang="ts">
  // Customers → Animations: customer animations made on the game's own skeleton (templates characters.json `skeleton`) and
  // which moment plays each. Laid out like the texture editor's workspace: toolbar, bones on the left, the view, properties
  // on the right, a dope sheet below; ⛶ Workspace fills the window (lib/maximize fullWindow). Posing: click a body part to
  // pick its bone, drag to turn it (the part follows the mouse), Shift+drag to twist; right-drag or drag on empty space turns
  // the camera. Every change keys the bone at the playhead (auto-key). The page's draft holds `animations` and `moves`; Save
  // writes them to the setup's item library and the mod plays them (Runtime/Customers/Moves).
  import { onMount, untrack } from 'svelte';
  import MomentAnims from './MomentAnims.svelte';
  import { CharacterView } from '../lib/characterView';
  import { charTexUrl, faceLook, facePieces, type Characters, type Face } from '../lib/faceLook';
  import { fullWindow } from '../lib/maximize';
  import { createHistory } from '../lib/history.svelte';
  import { animatableBones, chainFor, euler, maskedBones, qconj, qmul, samplePose, toEuler, turnBone, swingBy, twoBoneIK, worldMatrices, type Pose } from '../lib/skeleton';

  let { chars, animations = $bindable(), defaultAnims, defaultMoves, momentAnims = $bindable(), momentOff = $bindable(), moments, newId }: {
    chars: Characters; animations: Animation[]; defaultAnims: Animation[]; defaultMoves: Record<string, string[]>;
    momentAnims: Record<string, string[]>; momentOff: Record<string, string[]>;
    moments: { id: string; label: string }[]; newId: () => Promise<string>;
  } = $props();

  const MASKS = [
    { id: 'upper', label: 'Upper body' },
    { id: 'arms', label: 'Arms only' },
    { id: 'full', label: 'Whole body' },
  ];
  const MASK_HELP: Record<string, string> = {
    upper: 'Moves the spine, arms and head; the legs keep the game\'s animation.',
    arms: 'Moves only the arms (from the shoulders); everything else keeps the game\'s animation.',
    full: 'Moves everything; while the customer walks it plays on the upper body only.',
  };
  const FPS = 30;
  const KEY_SNAP = 1 / FPS; // a change within a frame of a key replaces it

  let body = $state<'female' | 'male'>('female');
  const B = $derived(chars.bodies[body]);
  const bones = $derived(B?.skeleton ?? []);
  const boneIdx = $derived(new Map(bones.map((b, i) => [b.name, i])));

  let currentId = $state('');
  const allAnims = $derived([...defaultAnims, ...animations]);
  const anim = $derived(allAnims.find((a) => a.id === currentId) ?? null);
  // Built-in ones can be looked at and played; Duplicate gives an editable copy.
  const builtIn = $derived(!!anim && defaultAnims.some((a) => a.id === anim.id));
  $effect(() => { if (!anim && allAnims.length) currentId = allAnims[0].id; });
  let page = $state<'edit' | 'moments'>('edit');
  const masked = $derived(anim ? maskedBones(bones, anim.mask) : new Set<string>());

  let time = $state(0);
  let playing = $state(false);
  let loop = $state(true);
  let selected = $state(-1);
  let search = $state('');
  const sel = $derived(selected >= 0 ? bones[selected] : undefined);

  // ---- the pose at the playhead
  const pose = $derived.by((): Pose => (anim ? samplePose((body === 'female' && anim.femaleBones) || anim.bones, Math.min(time, anim.length)) : {}));
  const rotOf = (i: number) => pose[bones[i].name] ?? bones[i].rot;
  const snapT = (t: number) => Math.round(Math.min(Math.max(t, 0), anim?.length ?? 0) * 1000) / 1000;

  /** Keys bones at the playhead (replacing a key within a frame), keeping each bone's keys in time order. */
  function setKeys(rots: Record<string, number[]>) {
    if (!anim || builtIn) return;
    const t = snapT(time);
    const next = { ...anim.bones };
    for (const [name, r] of Object.entries(rots)) {
      const keys = [...(next[name] ?? [])];
      const at = keys.findIndex((k) => Math.abs(k.t - t) < KEY_SNAP);
      const key = { t: at >= 0 ? keys[at].t : t, r: r.map((v) => Math.round(v * 1e5) / 1e5) };
      if (at >= 0) keys[at] = key; else { keys.push(key); keys.sort((a, b) => a.t - b.t); }
      next[name] = keys;
    }
    anim.bones = next;
  }
  const setKey = (name: string, r: number[]) => setKeys({ [name]: r });
  /** Every bone this animation can move, as it stands now, at the playhead. */
  function keyWholeBody() {
    if (!anim) return;
    setKeys(Object.fromEntries(bones.map((b, i) => [b, i] as const).filter(([b]) => masked.has(b.name) && animatable(b) && !b.name.startsWith('ik_') && b.name !== 'root').map(([b, i]) => [b.name, rotOf(i)])));
  }

  // Natural posing: hands / feet reach with their limb (IK), the spine and neck share a turn, fingers curl.
  let natural = $state(true);

  // ---- animations
  async function add() {
    const id = await newId();
    animations.push({ id, name: 'New animation', length: 2, mask: 'upper', bones: {} });
    currentId = id;
    time = 0;
    playing = false;
  }
  async function duplicate() {
    if (!anim) return;
    const id = await newId();
    animations.push({ ...JSON.parse(JSON.stringify(anim)), femaleBones: undefined, id, name: anim.name + ' (copy)' });
    currentId = id;
  }
  function remove() {
    if (!anim || builtIn || !confirm(`Delete "${anim.name}"?`)) return;
    const gone = anim.id;
    animations = animations.filter((a) => a.id !== gone);
    currentId = '';
  }

  // ---- keys
  const keyedBones = $derived(anim ? Object.keys(anim.bones).filter((n) => anim.bones[n].length)
    .sort((a, b) => (boneIdx.get(a) ?? 0) - (boneIdx.get(b) ?? 0)) : []);
  const lanes = $derived(sel && !keyedBones.includes(sel.name) ? [...keyedBones, sel.name] : keyedBones);
  const allTimes = $derived.by(() => {
    if (!anim) return [] as number[];
    const s = new Set<number>();
    for (const keys of Object.values(anim.bones)) for (const k of keys) s.add(k.t);
    return [...s].sort((a, b) => a - b);
  });
  const hasKeyAt = (name: string) => !!anim?.bones[name]?.some((k) => Math.abs(k.t - time) < KEY_SNAP);

  function deleteKeys(only?: string) {
    if (!anim || builtIn) return;
    const out: Animation['bones'] = {};
    for (const [name, keys] of Object.entries(anim.bones)) {
      const left = !only || name === only ? keys.filter((k) => Math.abs(k.t - time) >= KEY_SNAP) : keys;
      if (left.length) out[name] = left;
    }
    anim.bones = out;
  }
  /** Moves the keys at time `from` (all bones', or one bone's) to `to`; a key already there is replaced. */
  function moveKeys(from: number, to: number, only?: string) {
    if (!anim || builtIn) return from;
    to = snapT(to);
    if (Math.abs(to - from) < 1e-4) return from;
    const out: Animation['bones'] = {};
    for (const [name, keys] of Object.entries(anim.bones)) {
      if (only && name !== only) { out[name] = keys; continue; }
      if (!keys.some((k) => Math.abs(k.t - from) < 1e-4)) { out[name] = keys; continue; }
      out[name] = keys.filter((k) => Math.abs(k.t - to) >= 1e-4).map((k) => (Math.abs(k.t - from) < 1e-4 ? { ...k, t: to } : k)).sort((a, b) => a.t - b.t);
    }
    anim.bones = out;
    time = to;
    return to;
  }
  let clip = $state<Record<string, number[]> | null>(null);
  function copyPose() { clip = Object.fromEntries(keyedBones.map((n) => [n, [...pose[n]]])); }
  function pastePose() { if (clip) for (const [n, r] of Object.entries(clip)) setKey(n, r); }
  function keySelected() { if (sel) setKey(sel.name, rotOf(selected)); }

  // Sliders: degrees from the bone's rest rotation (Unity's order), keyed when moved.
  const turns = $derived(sel ? toEuler(qmul(qconj(sel.rot), rotOf(selected))).map((v) => Math.round(v)) : [0, 0, 0]);
  function slide(axis: number, v: number) {
    if (!sel) return;
    const t = [...turns];
    t[axis] = v;
    setKey(sel.name, qmul(sel.rot, euler(t[0], t[1], t[2])));
  }

  // ---- moments
  let momentSearch = $state('');
  let momentsUsedOnly = $state(false);
  const momentsShown = $derived(moments.filter((m) => (!momentSearch || m.label.toLowerCase().includes(momentSearch.toLowerCase()))
    && (!momentsUsedOnly || (!!anim && plays(m.id, anim.id)))));
  const usedBy = (id: string) => moments.filter((m) => plays(m.id, id)).length;
  const isDefault = (m: string, id: string) => (defaultMoves[m] ?? []).includes(id);
  const plays = (m: string, id: string) => (isDefault(m, id) && !(momentOff[m] ?? []).includes(id)) || (momentAnims[m] ?? []).includes(id);
  const withId = (map: Record<string, string[]>, m: string, id: string, on: boolean) => {
    const list = (map[m] ?? []).filter((x) => x !== id);
    if (on) list.push(id);
    const next = { ...map };
    if (list.length) next[m] = list; else delete next[m];
    return next;
  };
  function toggleMoment(m: string, on: boolean) {
    if (!anim) return;
    if (isDefault(m, anim.id)) momentOff = withId(momentOff, m, anim.id, !on);
    else momentAnims = withId(momentAnims, m, anim.id, on);
  }

  // ---- bone list
  const depth = (i: number) => { let d = 0; for (let p = bones[i].parent; p >= 0; p = bones[p].parent) d++; return d; };
  const animatable = $derived(animatableBones(bones));
  const shown = $derived(bones.map((b, i) => ({ b, i, d: depth(i) }))
    .filter(({ b }) => animatable(b) && !b.name.startsWith('ik_') && b.name !== 'root' && (!search || b.name.toLowerCase().includes(search.toLowerCase()))));

  // ---- preview
  const face = $derived.by((): Face => {
    const p = B?.presets[0];
    return {
      shapes: { ...(p?.shapes ?? {}) }, hair: [...(p?.hair ?? [])], hairColor: p?.hairColor ?? '#3a2a20ff',
      floats: { ...(p?.floats ?? {}) }, textures: { ...(p?.textures ?? {}) }, colors: { ...(p?.colors ?? {}) },
    };
  });
  let canvas = $state<HTMLCanvasElement>();
  let view: CharacterView | null = null;
  let error = $state('');
  let camTick = $state(0); // the view moved: the gizmo follows
  const bump = () => untrack(() => { camTick++; });
  onMount(() => {
    if (!canvas) return;
    try { view = new CharacterView(canvas, charTexUrl(chars)); } catch (e: any) { error = String(e?.message ?? e); return; }
    view.frame = 'body';
    const ro = new ResizeObserver(() => { view?.draw(); bump(); });
    ro.observe(canvas);
    return () => { ro.disconnect(); view?.dispose(); view = null; playing = false; };
  });
  let loaded = '';
  $effect(() => {
    const b = B, key = body;
    if (!view || !b || key === loaded) return;
    loaded = key;
    untrack(() => {
      const f = face;
      view!.setLook(faceLook(b, f));
      view!.setSkeleton(b.skeleton ?? []);
      view!.setPose(pose);
      view!.setBody(`/chartemplates/${b.mesh}`, b.parts.map((p) => p.material)).catch((e) => (error = String(e?.message ?? e)));
      view!.setPieces(facePieces(b, f)).catch((e) => (error = String(e?.message ?? e)));
    });
  });
  $effect(() => { const p = pose; view?.setPose(p); bump(); });

  // Play: the playhead moves in real time; stops at the end unless looping.
  $effect(() => {
    if (!playing) return;
    const a = untrack(() => anim);
    if (!a) return;
    let last = performance.now(), raf = 0;
    const step = (now: number) => {
      let t = untrack(() => time) + (now - last) / 1000;
      last = now;
      if (t >= a.length) { if (untrack(() => loop)) t %= a.length; else { time = a.length; playing = false; return; } }
      time = t;
      raf = requestAnimationFrame(step);
    };
    raf = requestAnimationFrame(step);
    return () => cancelAnimationFrame(raf);
  });

  // ---- gizmo: the picked bone's joint and a line to where it points
  const MIN_LEVER = 0.3; // metres: a bone shorter than this turns as if it were this long
  const worldPos = (m: Float32Array) => [m[12], m[13], m[14]];
  function tipOf(i: number, W: Float32Array[]): number[] {
    const child = bones.findIndex((b) => b.parent === i && !b.name.includes('twist'));
    if (child >= 0) return worldPos(W[child]);
    // A last bone (hand, head, toe): 12 cm on from its parent. Its own axes differ between the left and right side.
    const at = worldPos(W[i]), from = bones[i].parent >= 0 ? worldPos(W[bones[i].parent]) : [at[0], at[1] - 1, at[2]];
    const d = at.map((v, j) => v - from[j]), l = Math.hypot(d[0], d[1], d[2]) || 1;
    return at.map((v, j) => v + (d[j] / l) * 0.12);
  }
  const gizmo = $derived.by(() => {
    camTick;
    if (!view || selected < 0 || !bones.length) return null;
    const W = worldMatrices(bones, pose);
    return { a: view.project(worldPos(W[selected])), b: view.project(tipOf(selected, W)) };
  });

  // ---- pointer in the view: pick + turn bones, or turn the camera
  type Drag = { kind: 'cam' | 'bone'; x: number; y: number; twist: boolean };
  let ikTarget: number[] | null = null; // where the dragged hand / foot is heading (from the drag's start)
  let drag: Drag | null = null;
  function local(e: PointerEvent) { const r = canvas!.getBoundingClientRect(); return [e.clientX - r.left, e.clientY - r.top]; }
  function down(e: PointerEvent) {
    if (!view) return;
    const [x, y] = local(e);
    let kind: Drag['kind'] = 'cam';
    if (e.button === 0 && anim) {
      const hit = view.pick(x, y);
      if (hit >= 0 && !bones[hit].name.startsWith('ik_') && bones[hit].name !== 'root') { selected = hit; kind = 'bone'; playing = false; }
    }
    drag = { kind, x, y, twist: e.shiftKey };
    ikTarget = null;
    canvas!.setPointerCapture(e.pointerId);
    e.preventDefault();
  }
  let pendingTurn: { dx: number; dy: number; twist: boolean } | null = null;
  const boneOf = (name: string) => boneIdx.get(name) ?? -1;
  /** Metres per CSS pixel at a point of the body (for moving an IK target with the mouse). */
  function metresPerPixel(p: number[]) {
    const ax = view!.viewAxes();
    const a = view!.project(p), b = view!.project([p[0] + ax.right[0] * 0.1, p[1] + ax.right[1] * 0.1, p[2] + ax.right[2] * 0.1]);
    return 0.1 / (Math.hypot(b.x - a.x, b.y - a.y) || 1);
  }
  function move(e: PointerEvent) {
    if (!drag || !view) return;
    const [x, y] = local(e);
    const dx = x - drag.x, dy = y - drag.y;
    drag.x = x; drag.y = y;
    if (drag.kind === 'cam') {
      view.ry += dx * 0.01;
      view.rx = Math.max(-1.2, Math.min(1.2, view.rx + dy * 0.01));
      view.draw();
      bump();
    } else if (sel && (dx || dy)) {
      // Pointer events can come faster than frames: add them up and turn once per frame.
      if (pendingTurn) { pendingTurn.dx += dx; pendingTurn.dy += dy; return; }
      pendingTurn = { dx, dy, twist: drag.twist || e.shiftKey };
      requestAnimationFrame(() => { const t = pendingTurn; pendingTurn = null; if (t && sel) turnSelected(t.dx, t.dy, t.twist); });
    }
  }
  /** The picked bone follows the mouse: turned about the camera's up / right axes by whichever sign moves its tip the way
   *  the mouse went (or twisted about itself). */
  function turnSelected(dx: number, dy: number, twist: boolean) {
    const i = selected, ax = view!.viewAxes();
    const W = worldMatrices(bones, pose);
    const chain = natural && !twist ? chainFor(sel!.name) : null;
    if (chain?.kind === 'ik') {
      const [u, l, e] = [boneOf(chain.upper), boneOf(chain.lower), boneOf(chain.end)];
      if (u >= 0 && l >= 0 && e >= 0) {
        // The end joint follows the mouse in the screen's plane.
        const start = ikTarget ?? worldPos(W[e]);
        const k = metresPerPixel(start);
        const target = start.map((v, j) => v + (ax.right[j] * dx - ax.up[j] * dy) * k);
        ikTarget = target;
        setKeys(twoBoneIK(bones, pose, u, l, e, target, ax.forward) as Record<string, number[]>);
        return;
      }
    }
    if (twist) {
      const a = worldPos(W[i]), t = tipOf(i, W);
      const d = [t[0] - a[0], t[1] - a[1], t[2] - a[2]], l = Math.hypot(d[0], d[1], d[2]) || 1;
      setKey(sel!.name, turnBone(bones, pose, i, [d[0] / l, d[1] / l, d[2] / l], dx * 0.6));
      return;
    }
    // The tip goes where the mouse went, and the bone turns towards it.
    const tip = tipOf(i, W);
    const k = metresPerPixel(tip);
    const swing = swingBy(worldPos(W[i]), tip, ax.right.map((v, j) => (v * dx - ax.up[j] * dy) * k), MIN_LEVER);
    if (!swing) return;
    let p = pose;
    if (chain && chain.kind !== 'ik') {
      // Spread: the turn shared between the chain's bones; curl: repeated down the finger.
      const chained = chain.bones.filter((n) => boneOf(n) >= 0);
      const each = chain.kind === 'spread' ? swing.deg / chained.length : swing.deg;
      for (const n of chained) p = { ...p, [n]: turnBone(bones, p, boneOf(n), swing.axis, each) };
    } else p = { ...p, [sel!.name]: turnBone(bones, p, i, swing.axis, swing.deg) };
    const names = chain && chain.kind !== 'ik' ? chain.bones.filter((n) => boneOf(n) >= 0) : [sel!.name];
    setKeys(Object.fromEntries(names.map((n) => [n, p[n]])));
  }
  function wheel(e: WheelEvent) {
    if (!view) return;
    e.preventDefault();
    view.zoom = Math.max(0.5, Math.min(4, view.zoom * (e.deltaY < 0 ? 1.1 : 1 / 1.1)));
    view.draw();
    bump();
  }

  // ---- dope sheet
  let sheet = $state<HTMLDivElement>();
  const pct = (t: number) => (anim ? `${(t / anim.length) * 100}%` : '0%');
  const ticks = $derived.by(() => {
    if (!anim) return [] as { t: number; major: boolean }[];
    const step = anim.length > 10 ? 1 : anim.length > 4 ? 0.5 : 0.25;
    const out: { t: number; major: boolean }[] = [];
    for (let t = 0; t <= anim.length + 1e-6; t += step) out.push({ t: Math.round(t * 1000) / 1000, major: Math.abs(t - Math.round(t)) < 1e-6 });
    return out;
  });
  const timeAt = (e: PointerEvent) => {
    const r = sheet!.getBoundingClientRect();
    return Math.min(Math.max((e.clientX - r.left) / r.width, 0), 1) * (anim?.length ?? 0);
  };
  let sheetDrag: { mode: 'scrub' } | { mode: 'key'; from: number; bone?: string } | null = null;
  function sheetDown(e: PointerEvent) {
    playing = false;
    time = snapT(timeAt(e));
    sheetDrag = { mode: 'scrub' };
    sheet!.setPointerCapture(e.pointerId);
  }
  function keyDown(e: PointerEvent, t: number, bone?: string) {
    e.stopPropagation();
    playing = false;
    time = t;
    if (bone) selected = boneIdx.get(bone) ?? selected;
    sheetDrag = { mode: 'key', from: t, bone };
    sheet!.setPointerCapture(e.pointerId);
  }
  function sheetMove(e: PointerEvent) {
    if (!sheetDrag) return;
    if (sheetDrag.mode === 'scrub') time = snapT(timeAt(e));
    else sheetDrag.from = moveKeys(sheetDrag.from, timeAt(e), sheetDrag.bone);
  }

  // ---- undo (Ctrl+Z, Ctrl+Y / Ctrl+Shift+Z) of whatever changed the animation: edits close together are one step, a whole drag is one step
  const hist = createHistory(() => (anim ? JSON.stringify(anim) : ''), (state) => {
    if (!anim) return;
    const was = JSON.parse(state);
    const target = anim as Record<string, unknown>;
    for (const k of Object.keys(target)) if (!(k in was)) delete target[k];
    Object.assign(target, was);
    time = Math.min(time, anim.length);
  }, () => !!drag);
  let histFor = '';
  let dragStepped = false;
  $effect(() => {
    const id = anim?.id ?? '', now = anim ? JSON.stringify(anim) : '';
    untrack(() => {
      if (id !== histFor) { histFor = id; hist.reset(); return; }
      if (now === hist.base) return;
      if (!drag) hist.edited();
      else if (!dragStepped) { hist.push(); dragStepped = true; }
      hist.base = now;
    });
  });
  function endDrag() {
    drag = null;
    dragStepped = false;
    hist.split();
  }

  // ---- keys
  function keydown(e: KeyboardEvent) {
    const typing = e.target instanceof HTMLTextAreaElement || (e.target instanceof HTMLInputElement && !['range', 'checkbox'].includes(e.target.type));
    if ((e.ctrlKey || e.metaKey) && !typing && ['z', 'y'].includes(e.key.toLowerCase())) {
      e.preventDefault();
      if (e.key.toLowerCase() === 'y' || e.shiftKey) hist.redo(); else hist.undo();
      return;
    }
    const tag = (e.target as HTMLElement)?.tagName;
    if (tag === 'INPUT' || tag === 'SELECT' || tag === 'TEXTAREA' || !anim) return;
    if (e.key === ' ') { e.preventDefault(); (document.activeElement as HTMLElement | null)?.blur?.(); playing = !playing; } // blur: a focused button would toggle it back
    else if (e.key === 'Delete') { deleteKeys(sel?.name); e.preventDefault(); }
    else if (e.key === 'k' || e.key === 'K') keySelected();
    else if (e.key === 'ArrowLeft' || e.key === 'ArrowRight') {
      playing = false;
      const step = e.shiftKey ? 10 / FPS : 1 / FPS;
      time = snapT(time + (e.key === 'ArrowLeft' ? -step : step));
      e.preventDefault();
    } else if (e.key === 'Home') time = 0;
    else if (e.key === 'End') time = anim.length;
  }

  // ---- workspace: fills the window like the texture editor's
  let editorEl = $state<HTMLElement>();
  let ws = $state(false);
  let wsCtl: ReturnType<typeof fullWindow> | null = null;
  $effect(() => {
    const el = editorEl;
    if (!el) return;
    const c = fullWindow(el, (on) => { ws = on; requestAnimationFrame(() => { view?.draw(); bump(); }); });
    wsCtl = c;
    return () => { c.destroy(); if (wsCtl === c) wsCtl = null; ws = false; };
  });
</script>

<svelte:window onkeydown={keydown} />

{#if !bones.length}
  <p class="muted">The game templates don't have the customers' skeleton yet. Open <b>Game &amp; mod</b> and let Studio read the
    game files again (it does this by itself after an update).</p>
{:else}
  <div class="pages">
    <button class:on={page === 'edit'} onclick={() => (page = 'edit')}>Animations</button>
    <button class:on={page === 'moments'} onclick={() => (page = 'moments')}>When they play</button>
  </div>
  {#if page === 'moments'}
    <div class="momentsPage">
      <p class="muted small">What every customer plays at each moment. Switch a built-in one off and add your own; with several, one is picked at random. A customer can change this for themselves on Your regulars.</p>
      <MomentAnims {moments} anims={allAnims} off={(m) => momentOff[m] ?? []} added={(m) => momentAnims[m] ?? []}
        inherited={(m) => (defaultMoves[m] ?? []).map((id) => ({ id, from: 'built-in' }))}
        onchange={(m, o, a) => { const off = { ...momentOff }, add = { ...momentAnims };
          if (o.length) off[m] = o; else delete off[m];
          if (a.length) add[m] = a; else delete add[m];
          momentOff = off; momentAnims = add; }} />
    </div>
  {/if}
  <div class="editor" class:ws class:hidden={page !== 'edit'} bind:this={editorEl}>
    <div class="bar">
      <select bind:value={currentId} onchange={() => { time = 0; playing = false; }} title="Animation">
        <optgroup label="Built-in">{#each defaultAnims as a (a.id)}<option value={a.id}>{a.name}{usedBy(a.id) ? '' : ' (not used)'}</option>{/each}</optgroup>
        {#if animations.length}<optgroup label="Yours">{#each animations as a (a.id)}<option value={a.id}>{a.name}{usedBy(a.id) ? '' : ' (not used)'}</option>{/each}</optgroup>{/if}
      </select>
      <button onclick={add}>＋ New</button>
      <button onclick={duplicate} disabled={!anim}>Duplicate</button>
      <button class="danger" onclick={remove} disabled={!anim || builtIn}>Delete</button>
      {#if anim}
        <span class="sep"></span>
        <label class="inl">Name <input class="name" bind:value={anim.name} disabled={builtIn} /></label>
        <label class="inl">Length <input class="num" type="number" min="0.1" max="30" step="0.1" bind:value={anim.length} disabled={builtIn}
          onchange={() => { anim.length = Math.min(30, Math.max(0.1, +anim.length || 1)); }} /> s</label>
        <label class="inl" title={MASK_HELP[anim.mask]}>Moves <select bind:value={anim.mask} disabled={builtIn}>{#each MASKS as m (m.id)}<option value={m.id}>{m.label}</option>{/each}</select></label>
      {/if}
      <span class="grow"></span>
      <label class="inl" title="Hands and feet pull their arm or leg along (IK), the spine and neck bend together, fingers curl. Off: one bone at a time.">
        <input type="checkbox" bind:checked={natural} /> Natural posing</label>
      <span class="seg" title="Preview body (one animation fits both)">
        <button class:on={body === 'female'} onclick={() => (body = 'female')}>Female</button><button class:on={body === 'male'} onclick={() => (body = 'male')}>Male</button>
      </span>
      <button class="primary" onclick={() => wsCtl?.toggle()}
        title={ws ? 'Back to the page (Esc)' : 'Fill the whole window: the view big in the middle, bones, properties and the dope sheet docked around it'}>{ws ? '✕ Close workspace' : '⛶ Workspace'}</button>
    </div>

    <aside class="outliner panel">
      <div class="ph">Bones</div>
      <input class="search" placeholder="Find a bone" bind:value={search} />
      <div class="blist" role="listbox" tabindex="-1">
        {#each shown as { b, i, d } (b.name)}
          <button class:sel={i === selected} class:keyed={!!anim?.bones[b.name]} class:off={anim && !masked.has(b.name)}
            style:padding-left="{6 + d * 8}px" onclick={() => (selected = i)}
            title={anim && !masked.has(b.name) ? 'Outside this animation\'s body parts: the game ignores it' : b.path}>
            {#if anim?.bones[b.name]}<span class="dia"></span>{/if}{b.name}</button>
        {/each}
      </div>
    </aside>

    <div class="viewport panel">
      {#if error}<p class="err">{error}</p>{/if}
      <canvas bind:this={canvas} onpointerdown={down} onpointermove={move} onpointerup={endDrag}
        oncontextmenu={(e) => e.preventDefault()} onwheel={wheel}></canvas>
      {#if gizmo}
        <svg class="gizmo">
          <line x1={gizmo.a.x} y1={gizmo.a.y} x2={gizmo.b.x} y2={gizmo.b.y} />
          <circle cx={gizmo.a.x} cy={gizmo.a.y} r="22" />
          <circle class="dot" cx={gizmo.a.x} cy={gizmo.a.y} r="4" />
        </svg>
      {/if}
      <div class="overlay small">
        {#if anim}
          {sel ? sel.name : 'Click a body part'} · drag turns{natural ? ' (hands & feet pull the limb)' : ''} · Shift+drag twists · right-drag / background turns the view · wheel zooms
        {:else}
          Make an animation with ＋ New to start posing
        {/if}
      </div>
      {#if builtIn}<div class="empty small">Built-in: play it here, or Duplicate it to make your own version.</div>{:else if anim && !keyedBones.length}<div class="empty small">Pose the customer: every change is keyed at the playhead.</div>{/if}
    </div>

    <aside class="props panel">
      {#if anim}
        <div class="ph">Bone{sel ? ` · ${sel.name}` : ''}</div>
        {#if sel}
          {#each ['X', 'Y', 'Z'] as axis, k (axis)}
            <label class="slider">{axis} <input type="range" min="-180" max="180" step="1" value={turns[k]}
              oninput={(e) => slide(k, +e.currentTarget.value)} /> <span>{turns[k]}°</span></label>
          {/each}
          <div class="btns">
            <button onclick={keySelected} title="Key this bone's pose at the playhead (K)">Key (K)</button>
            <button onclick={() => setKey(sel.name, sel.rot)} title="Key this bone back at its rest rotation">Rest pose</button>
            <button onclick={() => deleteKeys(sel.name)} disabled={!hasKeyAt(sel.name)} title="Delete this bone's key at the playhead (Delete)">Delete key</button>
          </div>
          {#if !masked.has(sel.name)}<p class="warn small">Outside “{MASKS.find((m) => m.id === anim.mask)?.label}”: the game ignores this bone.</p>{/if}
        {:else}
          <p class="muted small">Click a body part in the view, or a bone in the list.</p>
        {/if}
        <p class="muted small">{MASK_HELP[anim.mask]}</p>

        <div class="ph">When customers play it</div>
        <div class="mtools">
          <input class="search" placeholder="Find a moment" bind:value={momentSearch} />
          <label class="small"><input type="checkbox" bind:checked={momentsUsedOnly} /> Ticked only</label>
        </div>
        <div class="moments">
          {#each momentsShown as m (m.id)}
            <label class="small">
              <input type="checkbox" checked={plays(m.id, anim.id)} onchange={(e) => toggleMoment(m.id, e.currentTarget.checked)} />
              <span>{m.label}</span>
            </label>
          {/each}
        </div>
        <p class="muted small">A moment with several animations plays one at random. Moments with a line play when the customer would say it.</p>
      {:else}
        <p class="muted small">Make an animation with ＋ New.</p>
      {/if}
    </aside>

    <div class="dope panel">
      <div class="transport">
        <button onclick={() => (time = 0)} title="Start (Home)" disabled={!anim}>⏮</button>
        <button onclick={() => (time = snapT(time - 1 / FPS))} title="Previous frame (←)" disabled={!anim}>−1f</button>
        <button class="play primary" onclick={(e) => { playing = !playing; e.currentTarget.blur(); }} title="Play / pause (Space)" disabled={!anim}>{playing ? '⏸ Pause' : '▶ Play'}</button>
        <button onclick={() => (time = snapT(time + 1 / FPS))} title="Next frame (→)" disabled={!anim}>+1f</button>
        <button onclick={() => { time = anim?.length ?? 0; playing = false; }} title="End (End)" disabled={!anim}>⏭</button>
        <label class="small"><input type="checkbox" bind:checked={loop} /> Loop</label>
        <span class="t">{time.toFixed(2)} / {(anim?.length ?? 0).toFixed(2)} s</span>
        <span class="grow"></span>
        <button onclick={copyPose} disabled={!keyedBones.length} title="Copy the pose at the playhead">Copy pose</button>
        <button onclick={pastePose} disabled={!clip || !anim} title="Key the copied pose at the playhead">Paste pose</button>
        <button onclick={() => hist.undo()} disabled={!hist.canUndo} title="Undo the last change to this animation (Ctrl+Z)">Undo</button>
        <button onclick={() => hist.redo()} disabled={!hist.canRedo} title="Redo (Ctrl+Y)">Redo</button>
        <button onclick={keyWholeBody} disabled={!anim || builtIn} title="Key every bone's current pose at the playhead, so later keys start from a known pose">Key whole body</button>
        <button onclick={() => deleteKeys()} disabled={!allTimes.some((t) => Math.abs(t - time) < KEY_SNAP)} title="Delete every bone's key at the playhead">Delete all keys here</button>
      </div>
      {#if anim && allTimes.length === 1}
        <p class="tip small">All keys are at {allTimes[0].toFixed(2)} s, so there's nothing to move between: click later on the ruler (or press →), pose
          again, and the animation moves from one pose to the next.</p>
      {/if}
      {#if anim}
        <div class="sheetwrap">
          <div class="names">
            <div class="rulerlbl"></div>
            <div class="lname all">All keys</div>
            {#each lanes as n (n)}<button class="lname" class:sel={n === sel?.name} onclick={() => (selected = boneIdx.get(n) ?? -1)}>{n}</button>{/each}
          </div>
          <div class="sheet" bind:this={sheet} role="slider" aria-valuenow={time} tabindex="-1"
            onpointerdown={sheetDown} onpointermove={sheetMove} onpointerup={() => (sheetDrag = null)}>
            <div class="ruler">
              {#each ticks as tk (tk.t)}<span class="tick" class:major={tk.major} style:left={pct(tk.t)}>{tk.major ? `${tk.t}s` : ''}</span>{/each}
            </div>
            <div class="lane all">
              {#each allTimes as t (t)}<button class="key" style:left={pct(t)} title="{t.toFixed(2)} s · drag to move every key here"
                onpointerdown={(e) => keyDown(e, t)} aria-label="keys at {t.toFixed(2)} s"></button>{/each}
            </div>
            {#each lanes as n (n)}
              <div class="lane" class:sel={n === sel?.name}>
                {#each anim.bones[n] ?? [] as k (k.t)}<button class="key bone" style:left={pct(k.t)} title="{n} · {k.t.toFixed(2)} s · drag to move"
                  onpointerdown={(e) => keyDown(e, k.t, n)} aria-label="key"></button>{/each}
              </div>
            {/each}
            <div class="playhead" style:left={pct(Math.min(time, anim.length))}></div>
          </div>
        </div>
      {/if}
    </div>
  </div>
{/if}

<style>
  .pages { display: flex; gap: 4px; margin-bottom: 6px; }
  .pages button.on { font-weight: 600; text-decoration: underline; }
  .momentsPage { overflow-y: auto; padding: 4px; }
  .editor.hidden { display: none; }
  .editor { display: grid; gap: 8px; height: calc(100vh - 210px); min-height: 520px; background: var(--bg);
    grid-template-columns: 220px minmax(0, 1fr) 300px; grid-template-rows: auto minmax(0, 1fr) 210px;
    grid-template-areas: 'bar bar bar' 'out view props' 'out dope props'; }
  .editor.ws { height: auto; padding: 8px 10px 10px; grid-template-columns: 240px minmax(0, 1fr) 320px; grid-template-rows: auto minmax(0, 1fr) 260px; }
  .panel { background: var(--panel); border: 1px solid var(--line); border-radius: var(--radius, 8px); min-height: 0; min-width: 0; }
  .ph { font-weight: 600; font-size: 13px; padding: 6px 2px 2px; }

  .bar { grid-area: bar; display: flex; align-items: center; gap: 6px; flex-wrap: wrap; padding-bottom: 8px; border-bottom: 1px solid var(--line); }
  .bar select { max-width: 220px; }
  .inl { display: flex; align-items: center; gap: 4px; font-size: 13px; }
  .name { width: 180px; }
  .num { width: 64px; }
  .sep { width: 1px; height: 22px; background: var(--line); margin: 0 4px; }
  .grow { flex: 1; }
  .seg { display: inline-flex; }
  .seg button { border-radius: 0; }
  .seg button:first-child { border-radius: 6px 0 0 6px; }
  .seg button:last-child { border-radius: 0 6px 6px 0; }
  .seg button.on { background: var(--accent-soft, #2a4b7c); }

  .outliner { grid-area: out; display: flex; flex-direction: column; gap: 6px; padding: 6px; }
  .search { width: 100%; box-sizing: border-box; }
  .blist { overflow: auto; flex: 1; display: flex; flex-direction: column; }
  .blist button { text-align: left; background: none; border: 0; border-radius: 4px; padding: 2px 6px; font-size: 12px; display: flex; align-items: center; gap: 4px; }
  .blist button.sel { background: var(--accent-soft, #2a4b7c); }
  .blist button.keyed { font-weight: 700; }
  .blist button.off { opacity: 0.45; }
  .dia { width: 6px; height: 6px; transform: rotate(45deg); background: #ffcc33; flex: none; }

  .viewport { grid-area: view; position: relative; overflow: hidden; }
  canvas { width: 100%; height: 100%; display: block; cursor: crosshair; touch-action: none; }
  .gizmo { position: absolute; inset: 0; width: 100%; height: 100%; pointer-events: none; }
  .gizmo line { stroke: #ffcc33; stroke-width: 2; }
  .gizmo circle { fill: none; stroke: #ffcc33; stroke-width: 1.5; opacity: 0.8; }
  .gizmo circle.dot { fill: #ffcc33; }
  .overlay { position: absolute; left: 10px; bottom: 8px; opacity: 0.65; pointer-events: none; }
  .empty { position: absolute; left: 50%; top: 12px; transform: translateX(-50%); opacity: 0.7; pointer-events: none; }
  .err { position: absolute; left: 10px; top: 8px; color: var(--danger, #e66); }

  .props { grid-area: props; display: flex; flex-direction: column; gap: 6px; padding: 6px 8px; overflow-y: auto; }
  .slider { display: grid; grid-template-columns: 14px 1fr 46px; align-items: center; gap: 6px; }
  .slider span { text-align: right; font-variant-numeric: tabular-nums; }
  .btns { display: flex; gap: 6px; flex-wrap: wrap; }
  .mtools { display: flex; flex-direction: column; gap: 4px; }
  .moments { display: flex; flex-direction: column; gap: 2px; min-height: 120px; flex: 1; overflow-y: auto; border: 1px solid var(--line); border-radius: 6px; padding: 4px; }
  .moments label { display: flex; gap: 6px; align-items: flex-start; }
  .warn { color: var(--warn, #e0b050); }
  .tip { margin: 0; padding: 4px 8px; border-radius: 6px; background: var(--accent-soft, #2a4b7c); }

  .dope { grid-area: dope; display: flex; flex-direction: column; gap: 6px; padding: 6px; }
  .transport { display: flex; align-items: center; gap: 4px; flex-wrap: wrap; }
  .transport .play { min-width: 40px; }
  .transport .t { font-variant-numeric: tabular-nums; margin-left: 6px; font-size: 13px; }
  .sheetwrap { flex: 1; min-height: 0; overflow-y: auto; display: grid; grid-template-columns: 130px 1fr; border: 1px solid var(--line); border-radius: 6px; }
  .names { display: flex; flex-direction: column; border-right: 1px solid var(--line); }
  .rulerlbl, .ruler { height: 20px; border-bottom: 1px solid var(--line); flex: none; }
  .lname { height: 22px; line-height: 22px; padding: 0 6px; font-size: 12px; text-align: left; background: none; border: 0; border-radius: 0;
    border-bottom: 1px solid var(--line); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; box-sizing: border-box; }
  .lname.all { font-weight: 600; opacity: 0.8; }
  .lname.sel, .lane.sel { background: var(--accent-soft, #2a4b7c); }
  .sheet { position: relative; cursor: ew-resize; user-select: none; touch-action: none; }
  .ruler { position: relative; }
  .tick { position: absolute; top: 0; height: 6px; border-left: 1px solid var(--line); font-size: 10px; padding-left: 2px; opacity: 0.7; }
  .tick.major { height: 20px; }
  .lane { position: relative; height: 22px; border-bottom: 1px solid var(--line); box-sizing: border-box; }
  .key { position: absolute; top: 5px; width: 11px; height: 11px; margin-left: -5px; padding: 0; transform: rotate(45deg); background: #9aa4b2;
    border: 1px solid #222; border-radius: 1px; cursor: grab; }
  .key.bone { background: #ffcc33; }
  .playhead { position: absolute; top: 0; bottom: 0; width: 2px; margin-left: -1px; background: #e55; pointer-events: none; }
</style>
