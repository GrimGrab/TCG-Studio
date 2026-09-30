<script lang="ts">
  import { onMount } from 'svelte';
  import { App, EventsOn, errText, ask } from '../lib/api';
  import { newLayout, type Layout } from '../lib/accessoryArt';
  import AccessoryEditor from './AccessoryEditor.svelte';
  import FigurineEditor, { newFigLayout, type FigLayout } from './FigurineEditor.svelte';

  let { notify }: { notify: (t: string, k?: string) => void } = $props();

  let view = $state<any>(null);          // AccessoryView from Go
  let templates = $state<any>(null);     // mod export (templates\accessories\accessories.json), null if missing
  let selectedId = $state('');
  let TABS = $state<any[]>([]);           // accessory kinds from Go (setfmt.AccessoryKinds): kind, title, one, toggle, bases
  const kindInfo = (k: string) => TABS.find((tb) => tb.kind === k);
  let kind = $state('Deckbox');          // current tab
  let items = $derived((view?.accessories ?? []).filter((a: any) => a.kind === kind));
  let acc = $state<any>(null);           // working copy of the selected accessory
  let layout = $state<Layout>(newLayout());
  let figLayout = $state<FigLayout>(newFigLayout()); // figurines: own model instead of a texture design
  let dirty = $state(false);
  let saving = $state(false);
  let bust = $state(Date.now());
  let editor = $state<any>(null);
  let figEditor = $state<any>(null);
  let editorKey = $state(0);

  const accUrl = (rel?: string) => (rel ? `/acc/${rel.split('/').map(encodeURIComponent).join('/')}?v=${bust}` : '');
  const tplUrl = (file?: string) => (file ? `/acctemplates/${encodeURIComponent(file)}` : '');
  const baseOf = (a: any) => a?.base || kindInfo(a?.kind)?.bases?.[0] || '';
  const tplItem = (a: any) => templates?.items?.find((i: any) => i.type === baseOf(a));

  async function load(keepSelection = true) {
    view = await App.Accessories();
    const ofKind = view.accessories.filter((a: any) => a.kind === kind);
    if (!keepSelection || !ofKind.some((a: any) => a.id === selectedId)) selectedId = ofKind[0]?.id ?? '';
    select(selectedId);
  }

  function select(id: string) {
    selectedId = id;
    const a = view?.accessories.find((x: any) => x.id === id);
    acc = a ? JSON.parse(JSON.stringify(a)) : null;
    let l: any = null;
    try { l = a && view.layouts[id] ? JSON.parse(view.layouts[id]) : null; } catch { l = null; }
    layout = l && l.version === 2 ? l : newLayout();
    figLayout = l && l.version === 'fig1' ? { ...newFigLayout(), ...l } : newFigLayout();
    dirty = false;
    editorKey++;
  }

  async function trySelect(id: string) {
    if (dirty && !(await ask('Discard unsaved changes to this accessory?'))) return;
    select(id);
  }

  async function add(kind: string) {
    if (dirty && !(await ask('Discard unsaved changes to this accessory?'))) return;
    try {
      const a = await App.NewAccessory(kind, '', '');
      await load();
      select(a.id);
      notify(`Added ${a.name}.`, 'ok');
    } catch (e) { notify(errText(e), 'error'); }
  }

  async function remove() {
    if (!acc || !(await ask(`Delete "${acc.name}"? Decks using it will show no deck box / playmat.`))) return;
    try { view = await App.DeleteAccessory(acc.id); dirty = false; await load(false); } catch (e) { notify(errText(e), 'error'); }
  }

  /** Swap with the previous/next accessory of the same kind (both kinds share one library order). */
  async function move(delta: number) {
    const i = items.findIndex((a: any) => a.id === selectedId), j = i + delta;
    if (j < 0 || j >= items.length) return;
    try { view = await App.MoveAccessory(selectedId, view.accessories.findIndex((a: any) => a.id === items[j].id)); } catch (e) { notify(errText(e), 'error'); }
  }

  async function switchTab(k: string) {
    if (k === kind) return;
    if (dirty && !(await ask('Discard unsaved changes to this accessory?'))) return;
    kind = k;
    const first = view?.accessories.find((a: any) => a.kind === k);
    select(first?.id ?? '');
  }

  /** True when the design is just the untouched vanilla art (then no texture/icon is written). */
  const isVanilla = (l: Layout) => l.base === 'vanilla' && !l.layers.some((x) => x.visible);

  async function save() {
    if (!acc) return;
    saving = true;
    try {
      let texture = '', icon = '';
      if (acc.kind === 'Figurine') {
        // Bake the model into the base toy's mesh space (Go), then render the shop icon.
        if (!figLayout.model) { acc.mesh = ''; acc.texture = ''; acc.icon = ''; }
        else if (figEditor) {
          const problem = figEditor.bakeProblem();
          if (problem) throw new Error(problem);
          const b = await App.BakeFigurine(acc.id, figLayout.model, figLayout.texture, figEditor.bakeParams());
          acc.mesh = b.mesh;
          acc.texture = b.texture;
          icon = await figEditor.exportIcon(t?.iconRect ?? [512, 512]);
        }
        view = await App.SaveAccessory(acc, JSON.stringify(figLayout), '', icon);
      } else {
        if (isVanilla(layout)) { acc.texture = ''; acc.icon = ''; }
        else if (editor) ({ texture, icon } = await editor.exportImages());
        view = await App.SaveAccessory(acc, JSON.stringify(layout), texture, icon);
      }
      bust = Date.now();
      dirty = false;
      const id = acc.id;
      acc = JSON.parse(JSON.stringify(view.accessories.find((x: any) => x.id === id)));
      notify(view.installed ? 'Saved and installed — restart the game to see it.' : 'Saved. Set the game folder in Settings to install it.', 'ok');
    } catch (e) { notify(errText(e), 'error'); }
    saving = false;
  }

  function opt(v: string): number | undefined {
    return v === '' || isNaN(+v) ? undefined : +v;
  }

  // Game templates (vanilla art, models, prices): TCG Studio reads them from the game files in the background.
  let tplStatus = $state<any>(null);
  async function loadTemplates() {
    const raw = await App.AccessoryTemplates();
    templates = raw ? JSON.parse(raw) : null;
    tplStatus = await App.TemplatesStatus();
  }

  onMount(() => {
    const offs = [
      EventsOn('templates:progress', (m: string) => (tplStatus = { ...tplStatus, busy: true, message: m })),
      EventsOn('templates:ready', () => loadTemplates().catch((e) => notify(errText(e), 'error'))),
    ];
    (async () => {
      try {
        TABS = await App.AccessoryKinds();
        await loadTemplates();
        await load(false);
      } catch (e) { notify(errText(e), 'error'); }
    })();
    return () => offs.forEach((off) => off());
  });

  let t = $derived(tplItem(acc));
  let hasBig = $derived(acc?.kind === 'Deckbox');
</script>

<div class="page">
  <header class="row">
    <h2 class="grow">Accessories</h2>
    <button onclick={() => App.OpenAccessoriesFolder()}>Open folder</button>
  </header>
  <p class="muted small">
    Your own deck boxes, playmats, sleeves, dice, comics, collection books, battle decks (on the booster-pack tab) and figurines
    (your own 3D models), sold in the shop next to the vanilla ones. They are
    global (not part of a set) and installed into the game on every save — restart the game to see changes. To sell only yours, turn
    off <b>Mod settings → Content → {kindInfo(kind)?.toggle ?? 'ShowVanilla…'}</b> (one switch per type).
    {#if view && !view.installed && view.accessories.length}<span class="warn">Not installed yet — set the game folder in Settings.</span>{/if}
  </p>
  {#if !templates}
    {#if tplStatus?.busy}
      <p class="muted small">Reading the game's models and art… {tplStatus.message ?? ''}</p>
    {:else}
      <p class="warn small">The game's models and art aren't available{tplStatus?.error ? `: ${tplStatus.error}` : ' — set the game folder in Settings'}.</p>
    {/if}
  {/if}
  {#if view?.errors?.length}<div class="err small">{#each view.errors as e}<div>{e}</div>{/each}</div>{/if}

  <div class="row tabs">
    {#each TABS as tb}
      <button class:on={kind === tb.kind} onclick={() => switchTab(tb.kind)}>
        {tb.title} <span class="muted small">({(view?.accessories ?? []).filter((a: any) => a.kind === tb.kind).length})</span>
      </button>
    {/each}
  </div>

  <div class="wrap">
    <div class="list">
      {#each items as a (a.id)}
        <button class:active={a.id === selectedId} onclick={() => trySelect(a.id)}>
          <span class="ico">{#if a.icon}<img src={accUrl(a.icon)} alt="" />{:else if tplItem(a)?.icon}<img src={tplUrl(tplItem(a).icon)} alt="" />{/if}</span>
          <span class="grow">{a.name}<br /><span class="muted small">Level {a.license.level}</span></span>
        </button>
      {/each}
      <button onclick={() => add(kind)}>+ New {TABS.find((tb) => tb.kind === kind)?.one}</button>
    </div>

    {#if acc}
      <div class="form">
        <section>
          <div class="row">
            <label class="field grow">Name<input bind:value={acc.name} oninput={() => (dirty = true)} /></label>
            <label class="field" title={acc.kind === 'Figurine'
              ? 'Vanilla toy whose shelf slot (how many fit and how big), shop tab and cost / market range are used.'
              : 'Vanilla item whose art is the starting point (and whose cost / market range are the defaults). All deck boxes share one model, all playmats another.'}>Start from
              <select value={baseOf(acc)} onchange={(e) => { acc.base = e.currentTarget.value; dirty = true; }}>
                {#each kindInfo(acc.kind)?.bases ?? [] as b}<option>{b}</option>{/each}
              </select>
            </label>
            <label class="field" style="width:150px">Id<input value={acc.id} readonly /></label>
          </div>
          <div class="row">
            <button class="small" onclick={() => move(-1)} disabled={items[0]?.id === acc.id}>▲ Earlier</button>
            <button class="small" onclick={() => move(1)} disabled={items.at(-1)?.id === acc.id}>▼ Later</button>
            <span class="muted small">List order = this type's ladder order in Gamify → Accessories.</span>
            <div class="grow"></div>
            <button class="danger small" onclick={remove}>Delete</button>
          </div>
        </section>

        <section>
          <div class="row">
            <h3 class="grow">Design</h3>
            <span class="thumbs">
              <span class="small muted">Saved:</span>
              {#if acc.icon}<img src={accUrl(acc.icon)} alt="icon" title="Shop icon" />{:else if t?.icon}<img src={tplUrl(t.icon)} alt="vanilla icon" title="Vanilla icon" />{/if}
              {#if acc.texture}<img src={accUrl(acc.texture)} alt="texture" title="Texture" />{/if}
            </span>
          </div>
          {#key editorKey}
            {#if acc.kind === 'Figurine'}
              <FigurineEditor bind:this={figEditor} base={baseOf(acc)} {templates} bind:layout={figLayout} {notify}
                onchange={() => (dirty = true)}
                onimported={(n) => { if (/^Custom figurine( \d+)?$/i.test(acc.name)) { acc.name = n; } }} />
            {:else}
            <AccessoryEditor bind:this={editor} kind={acc.kind} base={baseOf(acc)} vanillaUrl={tplUrl(t?.texture)} vanillaIconUrl={tplUrl(t?.icon)} bind:layout
              iconSize={t?.iconRect ?? [512, 512]} {notify} onchange={() => (dirty = true)} />
            {/if}
          {/key}
        </section>

        <section>
          <h3>Shop</h3>
          <div class="grid4">
            <label class="field">Cost<input type="number" step="0.5" min="0" placeholder={t ? String(t.baseCost) : 'model'} value={acc.cost ?? ''} oninput={(e) => { acc.cost = opt(e.currentTarget.value); dirty = true; }} /></label>
            <label class="field">Market min ×<input type="number" step="0.1" placeholder={t ? String(t.marketPriceMinPercent) : 'model'} value={acc.marketMin ?? ''} oninput={(e) => { acc.marketMin = opt(e.currentTarget.value); dirty = true; }} /></label>
            <label class="field">Market max ×<input type="number" step="0.1" placeholder={t ? String(t.marketPriceMaxPercent) : 'model'} value={acc.marketMax ?? ''} oninput={(e) => { acc.marketMax = opt(e.currentTarget.value); dirty = true; }} /></label>
            <div></div>
            <label class="field">{hasBig ? 'Small box license level' : 'License level'}<input type="number" min="1" bind:value={acc.license.level} oninput={() => (dirty = true)} /></label>
            <label class="field">{hasBig ? 'Small box license price' : 'License price'}<input type="number" min="0" bind:value={acc.license.price} oninput={() => (dirty = true)} /></label>
            {#if hasBig}
              <label class="field">Big box license level<input type="number" min="1" placeholder={String(acc.license.level + 3)} value={acc.license.bigLevel ?? ''} oninput={(e) => { acc.license.bigLevel = opt(e.currentTarget.value); dirty = true; }} /></label>
              <label class="field">Big box license price<input type="number" min="0" placeholder={String(Math.round(acc.license.price * 2))} value={acc.license.bigPrice ?? ''} oninput={(e) => { acc.license.bigPrice = opt(e.currentTarget.value); dirty = true; }} /></label>
            {/if}
          </div>
          <p class="muted small">Empty cost/market = the "Start from" item's vanilla values. {hasBig ? 'Deck boxes sell in a small and a big delivery box (two licenses).' : `${kindInfo(acc.kind)?.title} sell in one big delivery box (one license).`}</p>
        </section>

        <div class="row savebar">
          <div class="grow"></div>
          {#if dirty}<span class="muted small">Unsaved changes</span>{/if}
          <button onclick={() => select(acc.id)} disabled={!dirty || saving}>Revert</button>
          <button class="primary" onclick={save} disabled={!dirty || saving}>{saving ? 'Saving…' : 'Save & install'}</button>
        </div>
      </div>
    {:else if view}
      <p class="muted" style="padding:20px">No {TABS.find((tb) => tb.kind === kind)?.title.toLowerCase()} yet — click “+ New {TABS.find((tb) => tb.kind === kind)?.one}”.</p>
    {/if}
  </div>
</div>

<style>
  .page { padding: 16px 20px; height: 100%; display: flex; flex-direction: column; gap: 10px; overflow: auto; }
  .tabs { border-bottom: 1px solid var(--line); padding-bottom: 8px; flex-wrap: wrap; }
  .tabs button.on { border-color: var(--accent); background: #22304d; }
  .wrap { display: flex; flex: 1; min-height: 0; }
  .list { width: 230px; flex-shrink: 0; border-right: 1px solid var(--line); padding: 10px; display: flex; flex-direction: column; gap: 4px; overflow: auto; }
  .list button { text-align: left; display: flex; gap: 8px; align-items: center; }
  .list button.active { border-color: var(--accent); background: #22304d; }
  .ico { width: 36px; height: 36px; flex-shrink: 0; display: flex; align-items: center; justify-content: center; }
  .ico img { max-width: 100%; max-height: 100%; }
  .form { flex: 1; min-width: 0; overflow: auto; padding: 14px 18px; display: flex; flex-direction: column; gap: 14px; }
  section { background: var(--panel); border: 1px solid var(--line); border-radius: var(--radius); padding: 12px; display: flex; flex-direction: column; gap: 10px; }
  .grid4 { display: grid; grid-template-columns: repeat(4, 1fr); gap: 8px; }
  .thumbs { display: flex; gap: 8px; align-items: center; }
  .thumbs img { height: 44px; border-radius: 4px; background: var(--bg); }
  .savebar { position: sticky; bottom: 0; background: var(--bg); padding: 8px 0; }
  .small { font-size: 12px; }
  .warn { color: #d9a400; }
  .err { color: var(--danger); }
</style>
