<script lang="ts">
  // Customers page: regulars made by the player (the setup's item library), the setup's own lines, and regulars the game
  // made in this setup's saves, which can be adopted (same id, so they keep their memory in the save). One draft for the
  // whole page; nothing reaches the game before Save. The game reads it at its next start.
  import { onMount, tick } from 'svelte';
  import { App, ask, errText } from '../lib/api';
  import SaveBar from './SaveBar.svelte';
  import LineList from './LineList.svelte';
  import MomentAnims from './MomentAnims.svelte';
  import PersonLook from './PersonLook.svelte';
  import TraitChips from './TraitChips.svelte';
  import VoicePicker from './VoicePicker.svelte';
  import type { Characters, Face } from './FaceEditor.svelte';
  import { changed, pictureUrl, portraitKeysOf, refreshPortraits, setupAnimsAt, setupLines, withKey, personPortraitKey, type ClothesTpl, type Look, type OutfitSlot, type SavedFace } from '../lib/personLook';
  import AnimationsTab, { type Animation } from './AnimationsTab.svelte';
  import { automaticVoice, previewVoice, sampleLine, PLAIN_VOICE, type Mood, type Voice } from '../lib/talkVoice';

  let { notify }: { notify: (t: string, k?: string) => void } = $props();

  type Customer = { id: string; name: string; nickname?: string; female?: boolean; model: number; traits: string[]; skill: number; anims?: Record<string, string[]>; animsOff?: Record<string, string[]>; lines?: Record<string, string[]>; removed?: Record<string, string[]>; outfit?: OutfitSlot[]; face?: Face; portrait?: string; faceId?: string; voice?: Voice };
  type Trait = { id: string; name: string; opposite?: string; requires?: string; effects: Record<string, number>; lines?: Record<string, string[]> };

  let customers = $state<Customer[]>([]);
  // The setup's lines: added on top of the built-in ones, and built-in ones switched off (per situation).
  let lines = $state<Record<string, string[]>>({});
  let removedLines = $state<Record<string, string[]>>({});
  // Generated customers' names: added to the built-in lists and built-in ones switched off (per list).
  let names = $state<Record<string, string[]>>({});
  let namesOff = $state<Record<string, string[]>>({});
  const NAME_LISTS = [{ id: 'female', label: 'First names (female)' }, { id: 'male', label: 'First names (male)' }, { id: 'last', label: 'Last names' }];
  let defaults = $state<any>(null);
  let saved = $state<any[]>([]);
  let models = $state<Record<string, number>>({ male: 36, female: 16 });
  let baseline = $state('');
  let saving = $state(false);
  let loading = $state(true);
  let tab = $state<'customers' | 'saved' | 'lines' | 'names' | 'animations'>('customers');
  let selectedId = $state<string | null>(null);
  let query = $state('');
  let nameBox = $state<HTMLInputElement>();
  let clothesTpl = $state<ClothesTpl | null>(null);
  let chars = $state<Characters | null>(null);
  let faces = $state<SavedFace[]>([]);
  let looks = $state<Look[]>([]);
  let animations = $state<Animation[]>([]);
  // The built-in animations; the setup adds to them and switches them off per moment, a customer on top of that.
  let defaultAnims = $state<Animation[]>([]);
  let defaultMoves = $state<Record<string, string[]>>({});
  let momentAnims = $state<Record<string, string[]>>({});
  let momentOff = $state<Record<string, string[]>>({});
  let moments = $state<{ id: string; label: string }[]>([]);

  const dirty = $derived(!loading && snapshot() !== baseline);
  const selected = $derived(customers.find((c) => c.id === selectedId) ?? null);
  const shown = $derived(customers.filter((c) => !query || c.name.toLowerCase().includes(query.toLowerCase())));
  const traits = $derived<Trait[]>(defaults?.traits ?? []);
  /** A line in this customer's voice. */
  function speak(c: Customer, text: string, mood: Mood | string | undefined) {
    previewVoice(c.voice ?? automaticVoice(c.id, !!c.female), sampleLine(text), (mood as Mood) || 'neutral');
  }
  const situations = $derived<{ id: string; label: string; mood?: string }[]>(defaults?.situations ?? []);
  const allMoments = $derived([...moments, ...situations]);
  const allAnims = $derived([...defaultAnims, ...animations]);
  /** The animations every customer of this setup can play at a moment. */
  const setupAnims = (m: string) => setupAnimsAt(m, defaultMoves, momentOff, momentAnims);
  const adoptable = $derived(saved.filter((s) => !customers.some((c) => c.id === s.id)));

  function snapshot() {
    return JSON.stringify({ customers, lines, removedLines, names, namesOff, faces, animations, momentAnims, momentOff });
  }

  async function load() {
    loading = true;
    try {
      const page = await App.LoadCustomersPage();
      customers = (page.customers ?? []) as Customer[];
      lines = page.lines?.lines ?? {};
      removedLines = page.lines?.removed ?? {};
      names = page.lines?.names ?? {};
      namesOff = page.lines?.namesOff ?? {};
      defaults = page.defaults;
      saved = page.saved ?? [];
      models = page.models ?? models;
      looks = (page.clothes ?? []) as Look[];
      if (page.clothesTemplates && !clothesTpl) clothesTpl = await (await fetch(page.clothesTemplates)).json();
      faces = (page.faces ?? []) as unknown as SavedFace[];
      animations = (page.animations ?? []) as unknown as Animation[];
      defaultAnims = (page.defaultAnimations ?? []) as unknown as Animation[];
      defaultMoves = page.defaultMoves ?? {};
      momentAnims = page.momentAnims ?? {};
      momentOff = page.momentOff ?? {};
      moments = page.moments ?? [];
      if (page.charactersTemplates && !chars) chars = await (await fetch(page.charactersTemplates)).json();
      if (!customers.some((c) => c.id === selectedId)) selectedId = customers[0]?.id ?? null;
    } catch (e) {
      notify(errText(e), 'error');
    }
    loading = false;
    baseline = snapshot();
    portraitKeys = portraitKeysOf(customers, faces, looks);
    portraitFresh = {};
  }

  // What a portrait shows, per customer as loaded: a portrait is made again on Save when this changed.
  let portraitKeys: Record<string, string> = {};
  let portraitFresh: Record<string, string> = {}; // made since loading (face editor Done)
  const portraitKey = (c: Customer) => personPortraitKey(c, looks);

  async function save() {
    // Show the one that needs a name, rather than a number in an error.
    const unnamed = customers.find((c) => !c.name?.trim());
    if (unnamed) {
      query = '';
      tab = 'customers';
      selectedId = unnamed.id;
      await tick();
      nameBox?.focus();
      notify(`There's a customer without a name (${customers.length} in the list): selected it. Name it or delete it, then save.`, 'error');
      return;
    }
    saving = true;
    try {
      // Portraits of their own faces, like the game's (the mod shows them on the tournament screen and the profile).
      await refreshPortraits(customers, chars, clothesTpl, looks, portraitFresh, portraitKeys, (name, e) => notify(`${name}: the portrait couldn't be made (${errText(e)})`, 'warn'));
      const warns = await App.SaveCustomers(customers as any, { lines, removed: removedLines, names, namesOff } as any, faces as any, animations as any, momentAnims, momentOff);
      await load();
      notify(warns?.length ? `Saved. ${warns.join(' ')}` : 'Saved. The game uses it at its next start.', warns?.length ? 'warn' : 'ok');
    } catch (e) {
      notify(errText(e), 'error');
    }
    saving = false;
  }

  async function add() {
    const id = await App.NewCustomerID();
    customers.push({ id, name: '', model: -1, traits: [], skill: -1 });
    selectedId = id;
    tab = 'customers';
    await tick();
    nameBox?.focus();
  }

  async function remove(c: Customer) {
    if (!(await ask(`Delete ${c.name}? If they visited in a save, they stay in that save as a regular the game made.`))) return;
    customers = customers.filter((x) => x.id !== c.id);
    selectedId = customers[0]?.id ?? null;
  }

  function adopt(s: any) {
    customers.push({ id: s.id, name: s.name, female: s.female, model: s.model, traits: [...(s.traits ?? [])], skill: s.skill, lines: s.lines ?? undefined, removed: s.removed ?? undefined });
    selectedId = s.id;
    tab = 'customers';
  }

  const EFFECT_NAMES: Record<string, string> = {
    money: 'budget', itemChance: 'buys items', cardChance: 'buys cards', talk: 'talks', rip: 'opens packs in the shop', wish: 'wishlist cards',
    skill: 'table skill', review: 'review swing', lostToYou: 'loyalty when you beat them', beatYou: 'loyalty when they beat you', loyaltyGain: 'grows loyal faster', visit: 'visits', tip: 'tips', grudge: 'grudges', steal: 'tries to steal', showOff: 'shows off their prized card', sell: 'offers to sell you cards', crowd: 'more people come to see big pulls', chase: 'comes back for more after a bad pack', bond: 'makes friends faster', followers: 'big pulls bring people in', shopExp: 'extra shop experience',
    patience: 'patience at the till', chat: 'starts chats', browse: 'browses shelves', hustle: 'rips people off in trades', table: 'heads for a table',
  };
  const ADDITIVE = ['itemChance', 'cardChance', 'rip', 'wish', 'skill', 'steal', 'showOff', 'sell', 'crowd', 'shopExp', 'chase', 'lostToYou', 'beatYou', 'bond', 'followers', 'browse', 'hustle', 'table'];
  function effectText(t: Trait): string {
    return Object.entries(t.effects ?? {}).map(([k, v]) => {
      const name = EFFECT_NAMES[k] ?? k;
      if (!ADDITIVE.includes(k)) return `${name} ×${v}`;
      if (k === 'crowd' || k === 'lostToYou' || k === 'beatYou' || k === 'bond' || k === 'followers') return `${name} +${v}`;
      if (k === 'rip' || k === 'steal' || k === 'showOff' || k === 'sell' || k === 'shopExp' || k === 'chase' || k === 'browse' || k === 'hustle' || k === 'table') return `${name} ${Math.round(v * 100)}%`;
      if (k === 'itemChance' || k === 'cardChance') return `${name} ${v > 0 ? '+' : ''}${v}%`;
      return `${name} ${v > 0 ? '+' : ''}${v}`;
    }).join(', ') + (t.requires ? ` (only with ${({ shoplifting: 'Shoplifting', trading: 'Trading' } as Record<string, string>)[t.requires] ?? t.requires} on)` : '');
  }

  // ---- lines: everything adds up (built-in − switched off + the setup's + traits' − a customer's switched off + their own)
  const builtIn = (id: string): string[] => defaults?.lines?.[id] ?? [];
  /** What every customer of this setup can say in a situation, with where each line comes from. */
  const setupPool = (id: string) => setupLines(builtIn(id), lines[id], removedLines[id]);
  /** A customer's inherited lines: the setup's plus their traits'. */
  function inheritedOf(c: Customer, id: string) {
    const out = setupPool(id);
    for (const tid of c.traits) {
      const t = traits.find((x) => x.id === tid);
      for (const text of t?.lines?.[id] ?? []) out.push({ text, from: `${t!.name} trait` });
    }
    return out;
  }

  onMount(load);
</script>

<SaveBar {dirty} {saving} onsave={save} onrevert={load}
  leaveMessage="Leave Customers without saving? Your unsaved changes will be lost."
  revertMessage="Throw away every unsaved change on this page (customers, lines and animations)?">
  {#snippet title()}<h2>Customers</h2>{/snippet}
</SaveBar>
<div class="page libpage">
  <nav class="tabbar">
    <button class:on={tab === 'customers'} onclick={() => (tab = 'customers')}>Your regulars<span class="n">{customers.length}</span></button>
    <button class:on={tab === 'saved'} onclick={() => (tab = 'saved')}>From your saves<span class="n">{adoptable.length}</span></button>
    <button class:on={tab === 'lines'} onclick={() => (tab = 'lines')}>What customers say<span class="n">{changed(lines, removedLines)}</span></button>
    <button class:on={tab === 'names'} onclick={() => (tab = 'names')}>Names<span class="n">{changed(names, namesOff)}</span></button>
    <button class:on={tab === 'animations'} onclick={() => (tab = 'animations')}>Animations<span class="n">{animations.length}</span></button>
  </nav>
  <div class="intro">
    <span class="muted">Named customers in the shop: turn them on in <b>Mod settings → Customers → Personalities</b>. Regulars you make here
      come back to the shop with this name, look and personality; the game remembers their visits, mood and table record in each save.</span>
  </div>

  {#if loading}
    <p class="muted emptypane">Loading…</p>
  {:else if tab === 'customers'}
    <div class="fwrap personform">
      <aside class="lside sidelist">
        <div class="sidetools"><input type="search" bind:value={query} placeholder="Search customers…" /></div>
        <div class="list" role="listbox" tabindex="-1">
          {#each shown as c (c.id)}
            <button class="item" class:active={c.id === selectedId} onclick={() => (selectedId = c.id)}>
              {#if pictureUrl(c.portrait)}<img class="ico portrait" src={pictureUrl(c.portrait)} alt="" />
              {:else}<span class="ico body">{c.female ? '♀' : '♂'}</span>{/if}
              <span class="info">
                <span class="name">{c.name || 'Unnamed'}</span>
                <span class="sub">{c.traits.map((t) => traits.find((x) => x.id === t)?.name ?? t).join(', ') || 'no traits'}</span>
              </span>
            </button>
          {:else}
            <p class="muted small empty">{query ? 'Nothing matches.' : 'No regulars made here yet.'}</p>
          {/each}
        </div>
        <div class="sidefoot">
          <button class="primary" onclick={add}>+ New customer</button>
          {#if adoptable.length}<button onclick={() => (tab = 'saved')}>Add from your saves ({adoptable.length})…</button>{/if}
        </div>
      </aside>

      {#if selected}
        {@const c = selected}
        <div class="form personform">
          <div class="pane-hero row">
            <div class="titlebox grow">
              <input class="title" bind:this={nameBox} bind:value={c.name} placeholder="Type their name…" maxlength="40" />
              <input bind:value={c.nickname} placeholder="Nickname (optional: otherwise they might earn one)" maxlength="30" />
              <div class="meta"><span>id <span class="code">{c.id}</span></span></div>
            </div>
            <div class="heroactions"><button onclick={() => remove(c)}>Delete</button></div>
          </div>

          <PersonLook person={c} people={customers} bind:faces {chars} {clothesTpl} {looks} {models} {notify} onfresh={(u) => (portraitFresh[u.id] = portraitKey(u as Customer))}
            noPickLabel="Random" noPickHint="Random picks a character on their first visit, and they keep that look on every visit after."
            pickHint="They look like this on every visit. ★ = your own faces (made with New face): give one to any regular or employee of this body. Walk-ins can roll the ones you haven't given to anyone, unless you switch that off with 🚶 on the face." />

          <section>
            <h3>Personality</h3>
            <TraitChips items={traits} picked={c.traits} titleOf={(t) => effectText(t as Trait)} onchange={(next) => (c.traits = next)} />
            <p class="muted small">Hover a trait to see what it changes. Traits also bring their own lines.</p>
            <div class="row">
              <span class="lbl">Table skill</span>
              <div class="seg">
                <button class:on={c.skill < 0} onclick={() => (c.skill = -1)}>Rolled by the game</button>
                <button class:on={c.skill >= 0} onclick={() => (c.skill = c.skill >= 0 ? c.skill : 50)}>Set</button>
              </div>
              {#if c.skill >= 0}
                <input type="range" min="1" max="100" bind:value={c.skill} />
                <b class="num">{c.skill}</b>
              {/if}
            </div>
            <p class="muted small">With Rivalries on, decides who wins when two customers play Tetramon against each other, at casual tables and Tetramon tournaments. 50 is average. It doesn't change your own games or Magic tournaments, where deck strength decides.</p>
          </section>

          <section>
            <VoicePicker id={c.id} female={c.female} voice={c.voice} sample="Hey, how much is this card?" onchange={(v) => (c.voice = v)} />
          </section>

          <section>
            <h3>Their own lines <span class="muted small">({changed(c.lines, c.removed)} situation(s) changed)</span></h3>
            <p class="muted small">They say the setup's lines plus their traits' lines. Switch any of them off with ✕ (just for them) and add their own;
              a random one is picked. {'{shop}'}, {'{card}'}, {'{price}'}, {'{rival}'} and {'{name}'} are filled in.</p>
            {#each situations as s}
              {@const mine = !!(c.lines?.[s.id]?.length || c.removed?.[s.id]?.length)}
              <details class="linegroup" open={mine}>
                <summary>{s.label}{#if mine}<span class="tagn">changed</span>{/if}</summary>
                <LineList inherited={inheritedOf(c, s.id)} removed={c.removed?.[s.id] ?? []} added={c.lines?.[s.id] ?? []}
                  onchange={(r, a) => { c.removed = withKey(c.removed, s.id, r); c.lines = withKey(c.lines, s.id, a); }}
                  onplay={(text) => speak(c, text, s.mood)} />
              </details>
            {/each}
          </section>
          <section>
            <h3>Their own animations <span class="muted small">({changed(c.anims, c.animsOff)} moment(s) changed)</span></h3>
            <p class="muted small">They play the setup's animations. Switch any off (just for them) and add their own; a random one is picked.</p>
            <MomentAnims moments={allMoments} anims={allAnims} inherited={setupAnims} off={(m) => c.animsOff?.[m] ?? []} added={(m) => c.anims?.[m] ?? []}
              onchange={(m, o, a) => { c.animsOff = withKey(c.animsOff, m, o); c.anims = withKey(c.anims, m, a); }} />
          </section>
        </div>
      {:else}
        <div class="emptypane">
          <h1>Make your own regulars</h1>
          <p class="muted">Give them a name, a look and a personality. Or adopt the regulars the game made in your saves.</p>
          <div class="row"><button class="primary" onclick={add}>+ New customer</button>
            {#if adoptable.length}<button onclick={() => (tab = 'saved')}>From your saves…</button>{/if}</div>
        </div>
      {/if}
    </div>
  {:else if tab === 'saved'}
    <div class="form personform">
      <p class="muted">Walk-ins who became regulars in this setup's saves. <b>Add</b> one to edit their name, look and personality here; they keep
        their visits, mood and record in the save (same customer).</p>
      {#each adoptable as s (s.id)}
        <div class="savedrow">
          <span class="body">{s.female ? '♀' : '♂'}</span>
          <div class="grow">
            <b>{s.name}</b>
            <div class="muted small">{(s.traits ?? []).map((t: string) => traits.find((x) => x.id === t)?.name ?? t).join(', ') || 'no traits'}
              · {s.visits} visit(s){s.lastDay >= 0 ? ` · last day ${s.lastDay}` : ''} · spent ${Math.round(s.spent ?? 0)} · {s.slot}</div>
          </div>
          <button onclick={() => adopt(s)}>Add</button>
        </div>
      {:else}
        <p class="muted small">None yet: play with Personalities on, and customers who spend well start coming back. Saves are read from the game's
          save folder (the active setup).</p>
      {/each}
    </div>
  {:else if tab === 'names'}
    <div class="form personform">
      <p class="muted">The names new customers in this setup get, one first name plus one last name at random. Switch any built-in name off with ✕ and add your own.
        Customers who already have a name keep it, and regulars you name yourself aren't affected. Mod settings → Customers → LastNameInitialOnly shortens last names to an initial.</p>
      <h3>Nicknames</h3>
      <p class="muted small">The words regulars can earn, rarely. A title has one holder at a time; a named one (Lucky Sam) at most two. With none left to give, nobody earns that one.</p>
      {#each defaults?.nicknames ?? [] as d (d.id)}
        {@const key = `nick:${d.id}`}
        <details class="linegroup" open={!!(names[key]?.length || namesOff[key]?.length)}>
          <summary>{d.label}</summary>
          <LineList noun="nickname" sayOn="Use this nickname again" sayOff="Never use this nickname"
            inherited={(d.words ?? []).map((text: string) => ({ text, from: 'built-in' }))}
            removed={namesOff[key] ?? []} added={names[key] ?? []}
            onchange={(r, a) => { namesOff = withKey(namesOff, key, r) ?? {}; names = withKey(names, key, a) ?? {}; }} />
        </details>
      {/each}
      <h3>Names</h3>
      {#each NAME_LISTS as l (l.id)}
        <details class="linegroup" open={!!(names[l.id]?.length || namesOff[l.id]?.length)}>
          <summary>{l.label} <span class="muted small">({(defaults?.names?.[l.id] ?? []).length - (namesOff[l.id]?.length ?? 0) + (names[l.id]?.length ?? 0)})</span></summary>
          <LineList noun="name" sayOn="Use this name again" sayOff="Never use this name"
            inherited={(defaults?.names?.[l.id] ?? []).map((text: string) => ({ text, from: 'built-in' }))}
            removed={namesOff[l.id] ?? []} added={names[l.id] ?? []}
            onchange={(r, a) => { namesOff = withKey(namesOff, l.id, r) ?? {}; names = withKey(names, l.id, a) ?? {}; }} />
        </details>
      {/each}
    </div>
  {:else if tab === 'animations'}
    {#if chars}<AnimationsTab {chars} bind:animations {defaultAnims} {defaultMoves} bind:momentAnims bind:momentOff moments={allMoments} newId={() => App.NewCustomerID()} />{:else}<p class="muted emptypane">The game templates aren't read yet.</p>{/if}
  {:else}
    <div class="form personform">
      <p class="muted">What every customer says in this setup: the built-in lines plus yours. Switch any built-in line off with ✕ and add your own;
        a random one is picked. {'{shop}'} = your shop's name, {'{card}'}, {'{price}'}, {'{rival}'} and {'{name}'} (someone else) are filled in
        where they fit. Personality traits add their own lines, and each regular's page can switch lines off or add more just for them.</p>
      {#each situations as s}
        {@const mine = !!(lines[s.id]?.length || removedLines[s.id]?.length)}
        <details class="linegroup" open={mine}>
          <summary>{s.label}{#if mine}<span class="tagn">changed</span>{/if}</summary>
          <LineList inherited={builtIn(s.id).map((text) => ({ text, from: 'built-in' }))} removed={removedLines[s.id] ?? []} added={lines[s.id] ?? []}
            onchange={(r, a) => { removedLines = withKey(removedLines, s.id, r) ?? {}; lines = withKey(lines, s.id, a) ?? {}; }}
            onplay={(text) => previewVoice(PLAIN_VOICE, sampleLine(text), (s.mood as Mood) || 'neutral')} />
        </details>
      {/each}
    </div>
  {/if}
</div>

<style>
  .page { flex: 1; min-height: 0; display: flex; flex-direction: column; overflow: hidden; }
</style>
