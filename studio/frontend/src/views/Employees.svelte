<script lang="ts">
  // Employees page: hireable employees made by the player (the setup's item library) and the setup's employee lines. One draft for the page; nothing
  // reaches the game before Save. The game reads it at its next start. Faces are shared with the Customers page.
  import { onMount, tick } from 'svelte';
  import { App, ask, errText } from '../lib/api';
  import SaveBar from './SaveBar.svelte';
  import LineList from './LineList.svelte';
  import MomentAnims from './MomentAnims.svelte';
  import AnimationsTab, { type Animation } from './AnimationsTab.svelte';
  import PersonLook from './PersonLook.svelte';
  import TraitChips from './TraitChips.svelte';
  import VoicePicker from './VoicePicker.svelte';
  import type { Characters, Face } from './FaceEditor.svelte';
  import { automaticVoice, previewVoice, sampleLine, PLAIN_VOICE, type Mood, type Voice } from '../lib/talkVoice';
  import { changed, pictureUrl, portraitKeysOf, refreshPortraits, setupAnimsAt, setupLines, withKey, personPortraitKey, type ClothesTpl, type Look, type OutfitSlot, type SavedFace } from '../lib/personLook';

  let { notify }: { notify: (t: string, k?: string) => void } = $props();

  type Employee = {
    id: string; name: string; female?: boolean; model: number; description?: string; restockSpeed: number; checkoutSpeed: number;
    walkSpeed?: number; wage: number; hireCost: number; level: number; perks: string[]; voice?: Voice; lines?: Record<string, string[]>; removed?: Record<string, string[]>; anims?: Record<string, string[]>; animsOff?: Record<string, string[]>;
    outfit?: OutfitSlot[]; face?: Face; faceId?: string; portrait?: string;
  };
  type Perk = { id: string; name: string; description: string; opposite?: string; lines?: Record<string, string[]> };
  type Situation = { id: string; label: string; mood?: string };

  let employees = $state<Employee[]>([]);
  let lines = $state<Record<string, string[]>>({});
  let removed = $state<Record<string, string[]>>({});
  let defaults = $state<any>(null);
  let models = $state<Record<string, number>>({ male: 36, female: 16 });
  let faces = $state<SavedFace[]>([]);
  let looks = $state<Look[]>([]);
  let clothesTpl = $state<ClothesTpl | null>(null);
  let baseline = $state('');
  let saving = $state(false);
  let loading = $state(true);
  let tab = $state<'employees' | 'lines' | 'animations'>('employees');
  let animations = $state<Animation[]>([]);
  let defaultAnims = $state<Animation[]>([]);
  let defaultMoves = $state<Record<string, string[]>>({});
  let momentAnims = $state<Record<string, string[]>>({});
  let momentOff = $state<Record<string, string[]>>({});
  let selectedId = $state<string | null>(null);
  let nameBox = $state<HTMLInputElement>();
  let query = $state('');
  let chars = $state<Characters | null>(null);

  const dirty = $derived(!loading && snapshot() !== baseline);
  const selected = $derived(employees.find((e) => e.id === selectedId) ?? null);
  const shown = $derived(employees.filter((e) => !query || e.name.toLowerCase().includes(query.toLowerCase())));
  const perks = $derived<Perk[]>(defaults?.employeePerks ?? []);
  const situations = $derived<Situation[]>(defaults?.employeeSituations ?? []);
  const allAnims = $derived([...defaultAnims, ...animations]);
  /** The animations every employee of this setup can play at a moment. */
  const setupAnims = (m: string) => setupAnimsAt(m, defaultMoves, momentOff, momentAnims);

  function snapshot() {
    return JSON.stringify({ employees, lines, removed, faces, animations, momentAnims, momentOff });
  }

  async function load() {
    loading = true;
    try {
      const page = await App.LoadEmployeesPage();
      employees = (page.employees ?? []) as unknown as Employee[];
      lines = page.lines ?? {};
      removed = page.removed ?? {};
      defaults = page.defaults;
      models = page.models ?? models;
      faces = (page.faces ?? []) as unknown as SavedFace[];
      looks = (page.clothes ?? []) as unknown as Look[];
      animations = (page.animations ?? []) as unknown as Animation[];
      defaultAnims = (page.defaultAnimations ?? []) as unknown as Animation[];
      defaultMoves = page.defaultMoves ?? {};
      momentAnims = page.momentAnims ?? {};
      momentOff = page.momentOff ?? {};
      if (page.clothesTemplates && !clothesTpl) clothesTpl = await (await fetch(page.clothesTemplates)).json();
      if (page.charactersTemplates && !chars) chars = await (await fetch(page.charactersTemplates)).json();
      if (!employees.some((e) => e.id === selectedId)) selectedId = employees[0]?.id ?? null;
    } catch (e) {
      notify(errText(e), 'error');
    }
    loading = false;
    baseline = snapshot();
    portraitKeys = portraitKeysOf(employees, faces, looks);
    portraitFresh = {};
  }

  let portraitKeys: Record<string, string> = {};
  let portraitFresh: Record<string, string> = {};

  async function save() {
    // Show the one that needs a name, rather than a number in an error.
    const unnamed = employees.find((e) => !e.name?.trim());
    if (unnamed) {
      query = '';
      tab = 'employees';
      selectedId = unnamed.id;
      await tick();
      nameBox?.focus();
      notify(`There's an employee without a name (${employees.length} in the list): selected it. Name it or delete it, then save.`, 'error');
      return;
    }
    saving = true;
    try {
      // Portraits of their own faces, shown in the hire app.
      await refreshPortraits(employees, chars, clothesTpl, looks, portraitFresh, portraitKeys, (name, e) => notify(`${name}: the portrait couldn't be made (${errText(e)})`, 'warn'));
      const warns = await App.SaveEmployees(employees as any, lines, removed, faces as any, animations as any, momentAnims, momentOff);
      await load();
      notify(warns?.length ? `Saved. ${warns.join(' ')}` : 'Saved. The game uses it at its next start.', warns?.length ? 'warn' : 'ok');
    } catch (e) {
      notify(errText(e), 'error');
    }
    saving = false;
  }

  async function add() {
    const id = await App.NewCustomerID();
    employees.push({ id, name: '', model: -1, restockSpeed: 2, checkoutSpeed: 2, wage: 100, hireCost: 500, level: 1, perks: [] });
    selectedId = id;
    tab = 'employees';
    await tick();
    nameBox?.focus();
  }

  async function remove(e: Employee) {
    if (!(await ask(`Delete ${e.name || 'this employee'}? If they're hired in a save, they stop coming in. Saves remember them in case you make them again.`))) return;
    employees = employees.filter((x) => x.id !== e.id);
    selectedId = employees[0]?.id ?? null;
  }

  /** The game's words for a speed (WorkerData.GetCheckoutSpeedText). */
  function speedWord(s: number) {
    return s < 1 ? 'Very fast' : s < 1.75 ? 'Fast' : s < 2.5 ? 'Normal' : s < 3 ? 'Slow' : 'Very slow';
  }

  // ---- voice and lines
  const speak = (e: Employee, text: string, mood?: string) =>
    previewVoice(e.voice ?? automaticVoice(e.id, !!e.female), sampleLine(text), (mood as Mood) || 'neutral');
  const builtIn = (id: string): string[] => defaults?.employeeLines?.[id] ?? [];
  const setupPool = (id: string) => setupLines(builtIn(id), lines[id], removed[id]);
  function inheritedOf(e: Employee, id: string) {
    const out = setupPool(id);
    for (const pid of e.perks) {
      const p = perks.find((x) => x.id === pid);
      for (const text of p?.lines?.[id] ?? []) out.push({ text, from: `${p!.name} perk` });
    }
    return out;
  }

  onMount(load);
</script>

<SaveBar {dirty} {saving} onsave={save} onrevert={load}
  leaveMessage="Leave Employees without saving? Your unsaved changes will be lost."
  revertMessage="Throw away every unsaved change on this page (employees, lines and animations)?">
  {#snippet title()}<h2>Employees</h2>{/snippet}
</SaveBar>
<div class="page libpage">
  <nav class="tabbar">
    <button class:on={tab === 'employees'} onclick={() => (tab = 'employees')}>Your employees<span class="n">{employees.length}</span></button>
    <button class:on={tab === 'lines'} onclick={() => (tab = 'lines')}>What employees say<span class="n">{changed(lines, removed)}</span></button>
    <button class:on={tab === 'animations'} onclick={() => (tab = 'animations')}>Animations<span class="n">{animations.length}</span></button>
  </nav>
  <div class="intro">
    <span class="muted">Your employees appear in the game's hire app next to its own. To hide the game's, turn off <b>Mod settings → Content → Show vanilla employees</b>.
      Talking is set under <b>Customers → Employees</b> there.</span>
  </div>

  {#if loading}
    <p class="muted emptypane">Loading…</p>
  {:else if tab === 'employees'}
    <div class="fwrap personform">
      <aside class="lside sidelist">
        <div class="sidetools"><input type="search" bind:value={query} placeholder="Search employees…" /></div>
        <div class="list" role="listbox" tabindex="-1">
          {#each shown as e (e.id)}
            <button class="item" class:active={e.id === selectedId} onclick={() => (selectedId = e.id)}>
              {#if pictureUrl(e.portrait)}<img class="ico portrait" src={pictureUrl(e.portrait)} alt="" />
              {:else}<span class="ico body">{e.female ? '♀' : '♂'}</span>{/if}
              <span class="info">
                <span class="name">{e.name || 'Unnamed'}</span>
                <span class="sub">{e.perks.map((p) => perks.find((x) => x.id === p)?.name ?? p).join(', ') || 'no perks'}</span>
              </span>
            </button>
          {:else}
            <p class="muted small empty">{query ? 'Nothing matches.' : 'No employees made here yet.'}</p>
          {/each}
        </div>
        <div class="sidefoot"><button class="primary" onclick={add}>+ New employee</button></div>
      </aside>

      {#if selected}
        {@const e = selected}
        <div class="form">
          <div class="pane-hero row">
            <div class="titlebox grow">
              <input class="title" bind:this={nameBox} bind:value={e.name} placeholder="Type their name…" maxlength="40" />
              <input bind:value={e.description} placeholder="Description in the hire app (optional)" maxlength="160" />
            </div>
            <div class="heroactions"><button onclick={() => remove(e)}>Delete</button></div>
          </div>

          <PersonLook person={e} people={employees} bind:faces {chars} {clothesTpl} {looks} {models} {notify} onfresh={(u) => (portraitFresh[u.id] = personPortraitKey(u, looks))}
            noPickLabel="Staff look" noPickHint="One of the game's staff looks. Pick a character to give them a customer's look or a face of your own."
            pickHint="They look like this every day. ★ = your own faces (made with New face): give one to any regular or employee of this body. Walk-ins can roll the ones you haven't given to anyone, unless you switch that off with 🚶 on the face." />

          <section>
            <h3>Perks</h3>
            <TraitChips items={perks} picked={e.perks} titleOf={(p) => (p as Perk).description} onchange={(next) => (e.perks = next)} />
            {#each e.perks as id}
              {@const p = perks.find((x) => x.id === id)}
              {#if p}<p class="muted small"><b>{p.name}:</b> {p.description}</p>{/if}
            {/each}
            <p class="muted small">Hover a perk to see what it does. Perks also bring their own lines.</p>
          </section>

          <section>
            <h3>The job</h3>
            <div class="row"><span class="lbl wide">Checkout speed</span><input type="range" min="0.5" max="4" step="0.05" bind:value={e.checkoutSpeed} /><b class="num">{speedWord(e.checkoutSpeed)}</b></div>
            <div class="row"><span class="lbl wide">Restock speed</span><input type="range" min="0.5" max="4" step="0.05" bind:value={e.restockSpeed} /><b class="num">{speedWord(e.restockSpeed)}</b></div>
            <div class="row"><span class="lbl wide">Walk speed</span><input type="range" min="0" max="1" step="0.05" value={e.walkSpeed ?? 0} oninput={(ev) => (e.walkSpeed = +ev.currentTarget.value)} /><b class="num">+{Math.round((e.walkSpeed ?? 0) * 100)}%</b></div>
            <div class="row"><span class="lbl wide">Wage per day</span><input type="number" min="0" step="10" bind:value={e.wage} /></div>
            <div class="row"><span class="lbl wide">Hiring cost</span><input type="number" min="0" step="50" bind:value={e.hireCost} /></div>
            <div class="row"><span class="lbl wide">Shop level</span><input type="number" min="0" step="1" bind:value={e.level} /></div>
          </section>

          <section>
            <VoicePicker id={e.id} female={e.female} voice={e.voice} sample="Next, please! Did you find everything?" onchange={(v) => (e.voice = v)} />
          </section>

          <section>
            <h3>Their own lines <span class="muted small">({changed(e.lines, e.removed)} situation(s) changed)</span></h3>
            {#each situations as s}
              {@const mine = !!(e.lines?.[s.id]?.length || e.removed?.[s.id]?.length)}
              <details class="linegroup" open={mine}>
                <summary>{s.label}{#if mine}<span class="tagn">changed</span>{/if}</summary>
                <LineList inherited={inheritedOf(e, s.id)} removed={e.removed?.[s.id] ?? []} added={e.lines?.[s.id] ?? []}
                  onchange={(r, a) => { e.removed = withKey(e.removed, s.id, r); e.lines = withKey(e.lines, s.id, a); }}
                  onplay={(text) => speak(e, text, s.mood)} />
              </details>
            {/each}
          </section>
          <section>
            <h3>Their own animations <span class="muted small">({changed(e.anims, e.animsOff)} moment(s) changed)</span></h3>
            <p class="muted small">They play the setup's animations. Switch any off (just for them) and add their own; a random one is picked.</p>
            <MomentAnims moments={situations} anims={allAnims} inherited={setupAnims} off={(m) => e.animsOff?.[m] ?? []} added={(m) => e.anims?.[m] ?? []}
              onchange={(m, o, a) => { e.animsOff = withKey(e.animsOff, m, o); e.anims = withKey(e.anims, m, a); }} />
          </section>
        </div>
      {:else}
        <div class="emptypane">
          <h1>Make your own employees</h1>
          <p class="muted">Give them a name, a look, a voice and some perks. They show up in the game's hire app.</p>
          <div class="row"><button class="primary" onclick={add}>+ New employee</button></div>
        </div>
      {/if}
    </div>
  {:else if tab === 'lines'}
    <div class="form personform">
      <p class="muted">What every employee says in this setup, the game's own included: the built-in lines plus yours. Switch any built-in line off with ✕ and add your own;
        a random one is picked. {'{shop}'} = your shop's name and {'{name}'} (someone else) are filled in where they fit. Perks add their own lines, and each employee's page can switch lines off or add more just for them.</p>
      {#each situations as s}
        {@const mine = !!(lines[s.id]?.length || removed[s.id]?.length)}
        <details class="linegroup" open={mine}>
          <summary>{s.label}{#if mine}<span class="tagn">changed</span>{/if}</summary>
          <LineList inherited={builtIn(s.id).map((text) => ({ text, from: 'built-in' }))} removed={removed[s.id] ?? []} added={lines[s.id] ?? []}
            onchange={(r, a) => { removed = withKey(removed, s.id, r) ?? {}; lines = withKey(lines, s.id, a) ?? {}; }}
            onplay={(text) => previewVoice(PLAIN_VOICE, sampleLine(text), (s.mood as Mood) || 'neutral')} />
        </details>
      {/each}
    </div>
  {:else}
    {#if chars}<AnimationsTab {chars} bind:animations {defaultAnims} {defaultMoves} bind:momentAnims bind:momentOff moments={situations} newId={() => App.NewCustomerID()} />{:else}<p class="muted emptypane">The game templates aren't read yet.</p>{/if}
  {/if}
</div>

<style>
  .page { flex: 1; min-height: 0; display: flex; flex-direction: column; overflow: hidden; }
</style>
