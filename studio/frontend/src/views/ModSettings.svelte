<script lang="ts">
  // The mod's settings (same as the in-game F1 menu), read from and written to <game>\BepInEx\config\tcgcustomcards.cfg.
  // Rendered from the file's own descriptions, so new mod settings show up without studio changes. Edits stay in the page's
  // draft until Save (one write, App.SaveModSettings).
  import { onMount } from 'svelte';
  import { App, errText, ask } from '../lib/api';
  import { Draft } from '../lib/draft.svelte';
  import SaveBar from './SaveBar.svelte';

  let { notify }: { notify: (t: string, k?: string) => void } = $props();

  const draft = new Draft<any[]>();
  const sections = $derived<any[]>(draft.value ?? []);
  let showWhen = $state<Record<string, { setting: string; is: string[] }>>({});
  let elsewhere = $state<string[]>([]); // settings edited on another page (Game look)
  let gameRunning = $state(false);
  let setup = $state('');
  let error = $state('');
  let loading = $state(true);

  function show(v: any) {
    draft.reset(v.sections);
    showWhen = v.showWhen ?? {};
    elsewhere = v.elsewhere ?? [];
    gameRunning = v.gameRunning;
    setup = v.setup;
  }

  async function load() {
    if (draft.dirty && !(await ask('Reload the settings from the file? Your unsaved changes will be lost.'))) return;
    try {
      show(await App.ModSettings());
      error = '';
    } catch (e) { error = errText(e); }
    loading = false;
  }

  let saving = $state(false);
  async function save() {
    const before = new Map<string, string>();
    for (const s of draft.stored()) for (const e of s.entries) before.set(s.name + '|' + e.key, e.value);
    const changes = sections.flatMap((s) => s.entries
      .filter((e: any) => before.get(s.name + '|' + e.key) !== e.value)
      .map((e: any) => ({ section: s.name, key: e.key, value: String(e.value) })));
    saving = true;
    try {
      show(await App.SaveModSettings(changes));
      notify(gameRunning ? 'Settings saved — live settings update in the game right away.' : 'Settings saved.', 'ok');
    } catch (e) { notify(errText(e), 'error'); }
    saving = false;
  }

  // Edits change the draft only; the file checks each value on Save.
  function set(e: any, value: string) { e.value = value; }

  /** Puts a section and its groups ('' = every setting) back to the mod's defaults in the draft (Save stores it). Settings
   *  edited on another page (Game look) are left alone. */
  function restore(section: string) {
    let n = 0;
    for (const s of sections) {
      if (section && s.name !== section && !s.name.startsWith(section + ' - ')) continue;
      for (const e of s.entries) {
        if (!elsewhere.includes(e.key) && !isDefault(e)) { e.value = e.default; n++; }
      }
    }
    notify(n ? `${n} setting${n === 1 ? '' : 's'} back at the default — Save to keep it.` : 'Everything was already at its default.', 'ok');
  }

  /** "ShowVanillaCards" → "Show vanilla cards" */
  const label = (k: string) => {
    const s = k.replace(/([a-z0-9])([A-Z])/g, '$1 $2').replace(/([A-Z])([A-Z][a-z])/g, '$1 $2');
    return s.charAt(0) + s.slice(1).toLowerCase();
  };
  const isNumber = (e: any) => e.type === 'Single' || e.type === 'Double' || e.type === 'Int32';
  const step = (e: any) => (e.type === 'Int32' ? 1 : e.max !== undefined && e.min !== undefined && e.max - e.min <= 2 ? 0.01 : 0.05);
  const round = (v: string) => { const n = +v; return isNaN(n) ? v : String(Math.round(n * 1000) / 1000); };

  // BepInEx colours are RRGGBBAA hex.
  const rgb = (v: string) => '#' + (v || '000000').slice(0, 6).toLowerCase();
  const alpha = (v: string) => (v?.length === 8 ? parseInt(v.slice(6, 8), 16) : 255);
  const hex2 = (n: number) => Math.max(0, Math.min(255, Math.round(n))).toString(16).padStart(2, '0').toUpperCase();
  const withRgb = (v: string, c: string) => c.slice(1).toUpperCase() + hex2(alpha(v));
  const withAlpha = (v: string, pct: number) => (v || '000000').slice(0, 6).toUpperCase() + hex2((pct / 100) * 255);

  // Sub-sections: the mod names them "<Section> - <Group>" (BepInEx section names; the F1 menu shows them as their own
  // categories). They are shown as collapsible groups inside <Section>.
  const SECTION_ORDER = ['Content', 'Visuals', 'Foil', 'MTG', 'Debug'];
  type Group = { id: string; title: string; entries: any[] };
  type Top = { name: string; entries: any[]; groups: Group[] };
  let query = $state('');
  const q = $derived(query.trim().toLowerCase());
  const matches = (e: any) => !q || label(e.key).toLowerCase().includes(q) || e.key.toLowerCase().includes(q) || (e.description || '').toLowerCase().includes(q);
  // Settings shown only while another setting has certain values (settings-meta.json "showWhen", shared with the mod's F1
  // menu; keys are unique across sections).
  const visible = (e: any, values: Record<string, string>) => {
    const rule = showWhen[e.key];
    if (!rule || values[rule.setting] === undefined) return true;
    return rule.is.some((v) => v.toLowerCase() === String(values[rule.setting]).toLowerCase());
  };
  let tree = $derived.by(() => {
    const values: Record<string, string> = {};
    for (const s of sections) for (const e of s.entries) values[e.key] = e.value;
    const tops: Record<string, Top> = {};
    for (const s of sections) {
      const i = s.name.indexOf(' - ');
      const top = i > 0 ? s.name.slice(0, i) : s.name;
      const t = (tops[top] ??= { name: top, entries: [], groups: [] });
      const entries = s.entries.filter((e: any) => !elsewhere.includes(e.key) && visible(e, values) && matches(e));
      if (i > 0) { if (entries.length) t.groups.push({ id: s.name, title: s.name.slice(i + 3), entries }); }
      else t.entries.push(...entries);
    }
    return Object.values(tops)
      .filter((t) => t.entries.length || t.groups.length)
      .sort((a, b) => (SECTION_ORDER.indexOf(a.name) + 1 || 99) - (SECTION_ORDER.indexOf(b.name) + 1 || 99) || a.name.localeCompare(b.name));
  });
  const allOf = (t: Top) => [...t.entries, ...t.groups.flatMap((g) => g.entries)];

  // Master–detail: the sections on the left, the chosen one on the right (remembered); a search shows matches from every section.
  const CUR_KEY = 'modsettings.section';
  let current = $state((() => { try { return localStorage.getItem(CUR_KEY) || ''; } catch { return ''; } })());
  $effect(() => { try { localStorage.setItem(CUR_KEY, current); } catch {} });
  const shownTops = $derived(q ? tree : tree.filter((t) => t.name === (tree.some((x) => x.name === current) ? current : tree[0]?.name)));
  const isDefault = (e: any) => e.value === e.default || round(e.value) === round(e.default);
  const changed = (entries: any[]) => entries.filter((e) => !isDefault(e)).length;

  onMount(load);
</script>

{#snippet entry(e: any)}
  <div class="entry">
    <div class="head row">
      <span class="grow name">{label(e.key)}
        {#if /applies live/i.test(e.description)}<span class="badge ok" title="Updates while the game is running">live</span>{/if}
        {#if !isDefault(e)}<span class="badge changed" title="Not the mod's default ({e.default}) — ↺ resets it">customised</span>{/if}
      </span>
      <span class="control">
        {#if e.type === 'Boolean'}
          <label class="switch"><input type="checkbox" checked={e.value === 'true'} onchange={(ev) => set(e, ev.currentTarget.checked ? 'true' : 'false')} /> {e.value === 'true' ? 'On' : 'Off'}</label>
        {:else if e.options?.length}
          <select value={e.value} onchange={(ev) => set(e, ev.currentTarget.value)}>
            {#each e.options as o}<option>{o}</option>{/each}
          </select>
        {:else if isNumber(e)}
          {#if e.min !== undefined && e.max !== undefined}
            <input type="range" min={e.min} max={e.max} step={step(e)} value={e.value} oninput={(ev) => (e.value = ev.currentTarget.value)} onchange={(ev) => set(e, round(ev.currentTarget.value))} />
          {/if}
          <input class="num" type="number" min={e.min} max={e.max} step={step(e)} value={round(e.value)} onchange={(ev) => set(e, round(ev.currentTarget.value))} />
        {:else if e.type === 'Color'}
          <input type="color" value={rgb(e.value)} onchange={(ev) => set(e, withRgb(e.value, ev.currentTarget.value))} />
          <label class="small muted">opacity <input class="num" type="number" min="0" max="100" step="5" value={Math.round((alpha(e.value) / 255) * 100)} onchange={(ev) => set(e, withAlpha(e.value, +ev.currentTarget.value))} />%</label>
        {:else}
          <input value={e.value} onchange={(ev) => set(e, ev.currentTarget.value)} />
        {/if}
        <button class="tiny" title="Restore default ({e.default})" disabled={isDefault(e)} onclick={() => set(e, e.default)}>↺</button>
      </span>
    </div>
    {#if e.description}<div class="desc muted small">{e.description}</div>{/if}
  </div>
{/snippet}

{#snippet changedBadge(n: number, where: string)}
  {#if n}<span class="badge changed" title="{n} setting{n === 1 ? '' : 's'} in this {where} differ{n === 1 ? 's' : ''} from the mod's defaults">{n} customised</span>{/if}
{/snippet}

<SaveBar dirty={draft.dirty} {saving} onsave={save} onrevert={() => draft.revert()}
  leaveMessage="Leave Mod settings without saving? Your unsaved changes will be lost."
  revertMessage="Throw away your unsaved mod setting changes?">
  {#snippet title()}<h2>Mod settings</h2>{/snippet}
</SaveBar>
<div class="page">
  <nav class="nav sidelist">
    <div class="navtools"><input type="search" placeholder="Search every setting…" bind:value={query} /></div>
    <div class="navlist">
      {#each tree as t (t.name)}
        <button class="item" class:active={!q && shownTops[0]?.name === t.name} onclick={() => { query = ''; current = t.name; }}>
          <span class="info"><span class="name">{t.name}</span>
            <span class="sub">{allOf(t).length} setting{allOf(t).length === 1 ? '' : 's'}{t.groups.length ? ` · ${t.groups.length} group${t.groups.length === 1 ? '' : 's'}` : ''}</span></span>
          {#if changed(allOf(t))}<span class="badge changed" title="{changed(allOf(t))} differ from the mod's defaults">{changed(allOf(t))}</span>{/if}
        </button>
      {/each}
    </div>
    <div class="navfoot">
      <button onclick={load} title="Read the config file again">Reload</button>
      <button class="danger" onclick={() => restore('')} disabled={!sections.length}>Restore all defaults</button>
    </div>
  </nav>

  <div class="body">
    <div class="intro">
      <span class="muted">The in-game <b>F1</b> menu's settings (TCG Custom Cards), stored when you click <b>Save</b>.
        {gameRunning ? 'The game is running: "live" settings update right away, the rest on the next start.' : 'They take effect the next time the game starts.'}
        {#if setup} Setup <b>{setup}</b> keeps its own copy.{/if}</span>
    </div>
    {#if loading}
      <p class="muted pad">Loading…</p>
    {:else if error}
      <div class="notice err pad">{error}</div>
    {:else}
      {#if q && !tree.length}<p class="muted pad">No setting matches "{query}".</p>{/if}
      {#each shownTops as t (t.name)}
        <section class="pane">
          <header class="pane-hero">
            <div class="grow">
              <h1>{t.name}</h1>
              <div class="muted">{allOf(t).length} setting{allOf(t).length === 1 ? '' : 's'}{q ? ` matching “${query}”` : ''}
                {@render changedBadge(changed(allOf(t)), 'section')}</div>
            </div>
            {#if !q}<button onclick={() => restore(t.name)}>Restore section defaults</button>{/if}
          </header>
          {#if t.entries.length}
            <div class="block">{#each t.entries as e (e.key)}{@render entry(e)}{/each}</div>
          {/if}
          {#each t.groups as g (g.id)}
            <div class="block">
              <div class="bhead"><h2>{g.title}</h2><span class="muted small">{g.entries.length} setting{g.entries.length === 1 ? '' : 's'}</span>
                {@render changedBadge(changed(g.entries), 'group')}<span class="grow"></span>
                {#if !q}<button class="small" onclick={() => restore(g.id)}>Restore group defaults</button>{/if}</div>
              {#each g.entries as e (e.key)}{@render entry(e)}{/each}
            </div>
          {/each}
        </section>
      {/each}
    {/if}
  </div>
</div>

<style>
  .page { flex: 1; min-height: 0; display: flex; overflow: hidden; }
  .nav { width: 260px; flex-shrink: 0; border-right: 1px solid var(--line); display: flex; flex-direction: column; min-height: 0; }
  .navtools { padding: 10px; border-bottom: 1px solid var(--line); }
  .navtools input { width: 100%; }
  .navlist { flex: 1; min-height: 0; overflow: auto; padding: 6px; display: flex; flex-direction: column; gap: 2px; }
  .navfoot { display: flex; flex-direction: column; gap: 6px; padding: 10px; border-top: 1px solid var(--line); background: var(--panel); }
  .info .sub { white-space: normal; }
  .body { flex: 1; min-width: 0; overflow: auto; }
  .intro { padding: 8px 24px; font-size: 12px; background: var(--panel); border-bottom: 1px solid var(--line); }
  .pad { margin: 20px 24px; }
  .pane { max-width: none; }
  .pane-hero h1 { margin-bottom: 4px; }
  .block { gap: 0; }
  .block .bhead { padding-bottom: 6px; }
  .entry { padding: 10px 0; border-top: 1px solid var(--line); }
  .block > .entry:first-child, .bhead + .entry { border-top: none; }
  .badge.changed { font-size: 11px; color: #d9a400; }
  .name { font-weight: 600; }
  .control { display: flex; align-items: center; gap: 8px; }
  .control input[type='range'] { width: 180px; }
  .num { width: 80px; }
  .control input[type='color'] { width: 44px; height: 28px; padding: 2px; }
  .desc { margin-top: 3px; max-width: 760px; }
  .badge { font-size: 11px; margin-left: 6px; }
  button.tiny { padding: 2px 8px; }
</style>
