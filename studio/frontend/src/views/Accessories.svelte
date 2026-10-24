<script lang="ts">
  // Accessories: every edit (fields, design, new / deleted / moved entries) stays in the page's draft until Save
  // (lib/libraryDraft: the entry you leave is finished and parked; Save stores the whole draft at once).
  import { onMount } from 'svelte';
  import { Ev } from '../lib/events';
  import { LibraryDraft } from '../lib/libraryDraft.svelte';
  import SaveBar from './SaveBar.svelte';
  import { accFile, templateFile } from '../lib/urls';
  import { App, EventsOn, errText, originName } from '../lib/api';
  import { matchSearch } from '../lib/search';
  import { newLayout, type Layout } from '../lib/accessoryArt';
  import AccessoryEditor from './AccessoryEditor.svelte';
  import CatalogPicker from './CatalogPicker.svelte';
  import FigurineEditor, { newFigLayout, type FigLayout } from './FigurineEditor.svelte';

  let { notify }: { notify: (t: string, k?: string) => void } = $props();

  let view = $state<any>(null);          // AccessoryView from Go
  let templates = $state<any>(null);     // mod export (templates\accessories\accessories.json), null if missing
  let selectedId = $state('');
  // Ticked in the list for Delete (Ctrl+click adds/removes, Shift+click a range); the open accessory is selectedId.
  let picked = $state<string[]>([]);
  let anchor = $state('');
  let TABS = $state<any[]>([]);           // accessory kinds from Go (setfmt.AccessoryKinds): kind, title, one, toggle, bases
  const kindInfo = (k: string) => TABS.find((tb) => tb.kind === k);
  let SHOPS = $state<Record<string, string>>({}); // shop lists an accessory can be sold from (setfmt.AccessoryShops)
  let kind = $state('Deckbox');          // current tab
  const draft = new LibraryDraft();
  let all = $derived(view ? draft.apply(view.accessories) : []); // every accessory with the draft applied (library order)
  let items = $derived(all.filter((a: any) => a.kind === kind));
  // Display order of the list (the library order itself stays the Gamify ladder order): library order, name, or source
  // (where it came from: converted mods grouped by mod, then things made in Studio).
  const SORT_LABELS: Record<string, string> = { order: 'List order', name: 'Name', source: 'Source' };
  const originOf = (x: any) => view?.origins?.[x.id];
  const sourceKey = (x: any) => (originOf(x) ? '0 ' + originName(originOf(x)).toLowerCase() : '1');
  function sortShown(list: any[], by: string): any[] {
    if (by === 'name') return [...list].sort((a, b) => a.name.localeCompare(b.name));
    if (by === 'source') return [...list].sort((a, b) => sourceKey(a).localeCompare(sourceKey(b)) || a.name.localeCompare(b.name));
    return list;
  }
  let sortBy = $state((() => { try { return localStorage.getItem('accessories.sort') || 'order'; } catch { return 'order'; } })());
  let query = $state(''); // list search (name or id)
  let shown = $derived(sortShown(items.filter((a: any) => matchSearch(query, a.name, a.id)), sortBy));
  let acc = $state<any>(null);           // working copy of the selected accessory
  let layout = $state<Layout>(newLayout());
  let figLayout = $state<FigLayout>(newFigLayout()); // figurines: own model instead of a texture design
  let useOwn = $state(false);            // board games / card supplies: own model (figLayout) instead of painting the game's
  let saving = $state(false);
  let bust = $state(Date.now());
  let editor = $state<any>(null);
  let figEditor = $state<any>(null);
  let editorKey = $state(0);

  // The open accessory's state when it was opened: it differs = unsaved edits (undoing them makes it clean again).
  let start = $state('');
  const snap = () => JSON.stringify({ acc, layout, figLayout, useOwn });
  $effect(() => { draft.shownDirty = !!acc && start !== '' && snap() !== start; });

  const accUrl = (rel?: string) => accFile(rel, bust);
  const tplUrl = (file?: string) => templateFile('acc', file);
  const baseOf = (a: any) => a?.base || kindInfo(a?.kind)?.bases?.[0] || '';
  const tplItem = (a: any) => templates?.items?.find((i: any) => i.type === baseOf(a));

  async function load(keepSelection = true) {
    view = await App.Accessories();
    const ofKind = all.filter((a: any) => a.kind === kind);
    if (!keepSelection || !ofKind.some((a: any) => a.id === selectedId)) selectedId = ofKind[0]?.id ?? '';
    select(selectedId);
  }

  /** Opens an accessory: its parked draft if it has one, else the stored one. */
  function select(id: string) {
    selectedId = id;
    picked = id ? [id] : [];
    anchor = id;
    const p = draft.parked[id];
    const a = p?.item ?? view?.accessories.find((x: any) => x.id === id);
    acc = a ? JSON.parse(JSON.stringify(a)) : null;
    let l: any = null;
    const raw = p ? p.layout : view?.layouts[id];
    try { l = a && raw ? JSON.parse(raw) : null; } catch { l = null; }
    layout = l && l.version === 2 ? l : newLayout();
    figLayout = l && l.version === 'fig1' ? { ...newFigLayout(), ...l } : newFigLayout();
    useOwn = !!l && l.version === 'fig1';
    start = acc ? snap() : '';
    editorKey++;
  }

  /** Finishes the open accessory's edits (texture + icon from the editor, or the own model baked) and parks them in the
   *  draft. Runs before another accessory opens and on Save. */
  async function finishShown() {
    if (!acc || !draft.shownDirty) return;
    let texture = '', icon = '';
    if (ownModel(acc)) {
      // Bake the model into the base toy's mesh space (Go), then render the shop icon.
      if (!figLayout.model) { acc.mesh = ''; acc.texture = ''; acc.icon = ''; }
      else if (figEditor) {
        const problem = figEditor.bakeProblem();
        if (problem) throw new Error(`${acc.name}: ${problem}`);
        const b = await App.BakeFigurine(acc.id, figLayout.model, figLayout.texture, figEditor.bakeParams());
        acc.mesh = b.mesh;
        acc.texture = b.texture;
        icon = await figEditor.exportIcon(t?.iconRect ?? [512, 512]);
      }
    } else {
      acc.mesh = ''; // painted on the game's model
      if (isVanilla(layout)) { acc.texture = ''; acc.icon = ''; }
      else if (editor) ({ texture, icon } = await editor.exportImages());
    }
    const prev = draft.parked[acc.id];
    draft.park(acc.id, { item: $state.snapshot(acc), layout: JSON.stringify(ownModel(acc) ? figLayout : layout),
      texture: texture || prev?.texture, icon: icon || prev?.icon });
    start = snap();
  }

  /** Finishes the open accessory before something else opens. One that can't be finished yet is parked as it is, with the
   *  reason (Save opens it and says so). */
  async function leaveShown(): Promise<boolean> {
    try { await finishShown(); } catch (e) {
      if (!acc) return true;
      draft.park(acc.id, { item: $state.snapshot(acc), layout: JSON.stringify(ownModel(acc) ? figLayout : layout), problem: errText(e) });
      start = snap();
    }
    return true;
  }

  async function trySelect(id: string) {
    if (id !== selectedId && (await leaveShown())) select(id);
  }

  async function add(kind: string) {
    if (!(await leaveShown())) return;
    try {
      const a = await App.NewAccessory(kind, '', '');
      draft.add(a);
      select(a.id);
    } catch (e) { notify(errText(e), 'error'); }
  }

  let fromCatalog = $state(false);

  async function addedFromCatalog(ids: string[]) {
    fromCatalog = false;
    if (!(await leaveShown())) return;
    await load();
    if (ids.length) notify(`Added ${ids.length} item${ids.length === 1 ? '' : 's'} from the catalog.`, 'ok');
  }

  async function saveToCatalog() {
    if (!acc) return;
    if (draft.shownDirty || draft.parked[acc.id]) { notify('Save your changes first — the catalog takes the saved version.', 'warn'); return; }
    try { await App.SaveToCatalog('accessory', [acc.id]); notify(`"${acc.name}" is now the catalog's version.`, 'ok'); }
    catch (e) { notify(errText(e), 'error'); }
  }

  /** Deletes accessories from the draft (Save deletes them in the game; Revert brings them back). */
  function removeIds(ids: string[]) {
    const names = all.filter((a: any) => ids.includes(a.id)).map((a: any) => a.name);
    const next = items.find((a: any) => !ids.includes(a.id))?.id ?? '';
    draft.remove(ids);
    select(ids.includes(selectedId) ? next : selectedId);
    const one = names.length === 1;
    notify(`Deleted ${one ? `"${names[0]}"` : `${names.length} accessories`} — Save to remove ${one ? 'it' : 'them'} (decks using ${one ? 'it' : 'them'} show no deck box / playmat), Revert to bring ${one ? 'it' : 'them'} back.`, 'ok');
  }

  function remove() {
    if (acc) removeIds([acc.id]);
  }

  /** List click: plain = open it; Ctrl = add/remove it from the selection; Shift = select the range from the last click. */
  async function clickItem(e: MouseEvent, id: string) {
    if (e.ctrlKey || e.metaKey) {
      picked = picked.includes(id) ? picked.filter((p) => p !== id) : [...picked, id];
      anchor = id;
      return;
    }
    if (e.shiftKey && anchor) {
      const ids = shown.map((a: any) => a.id);
      const [i, j] = [ids.indexOf(anchor), ids.indexOf(id)].sort((x, y) => x - y);
      if (i >= 0) { picked = ids.slice(i, j + 1); return; }
    }
    await trySelect(id);
  }

  /** Delete key in the list: deletes the selected accessories from the draft. */
  function listKey(e: KeyboardEvent) {
    if (e.key !== 'Delete' || !picked.length) return;
    e.preventDefault(); // the texture editor's Delete (remove layer) must not run too
    const ids = picked.filter((id) => shown.some((x: any) => x.id === id)); // not the ones a search hides
    if (ids.length) removeIds(ids);
  }

  /** Swap with the previous/next accessory of the same kind (both kinds share one library order). */
  function move(delta: number) {
    const i = items.findIndex((a: any) => a.id === selectedId), j = i + delta;
    if (i < 0 || j < 0 || j >= items.length) return;
    draft.swap(all, selectedId, items[j].id);
  }

  async function switchTab(k: string) {
    if (k === kind || !(await leaveShown())) return;
    kind = k;
    select(all.find((a: any) => a.kind === k)?.id ?? '');
  }

  /** True when the design is just the untouched vanilla art (then no texture/icon is written). */
  const isVanilla = (l: Layout) => l.base === 'vanilla' && !l.textureFile && !l.layers.some((x) => x.visible);

  async function save() {
    saving = true;
    try {
      await finishShown();
      const bad = draft.firstProblem();
      if (bad) {
        const k = all.find((x: any) => x.id === bad[0])?.kind;
        if (k) kind = k;
        select(bad[0]);
        throw new Error(bad[1]);
      }
      view = await App.SaveLibraryDraft('accessories', draft.request() as any);
      draft.clear();
      bust = Date.now();
      select(view.accessories.some((a: any) => a.id === selectedId) ? selectedId : (items[0]?.id ?? ''));
      notify(view.installed ? 'Saved — restart the game to see it.' : 'Saved. Set the game folder in Settings so the game reads it.', 'ok');
    } catch (e) { notify(errText(e), 'error'); }
    saving = false;
  }

  function revert() {
    draft.clear();
    select(view?.accessories.some((a: any) => a.id === selectedId) ? selectedId : (items[0]?.id ?? ''));
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
      EventsOn(Ev.TemplatesProgress, (m: string) => (tplStatus = { ...tplStatus, busy: true, message: m })),
      EventsOn(Ev.TemplatesReady, () => loadTemplates().catch((e) => notify(errText(e), 'error'))),
    ];
    (async () => {
      try {
        TABS = await App.AccessoryKinds();
        SHOPS = await App.AccessoryShops();
        await loadTemplates();
        await load(false);
      } catch (e) { notify(errText(e), 'error'); }
    })();
    return () => offs.forEach((off) => off());
  });

  let t = $derived(tplItem(acc));
  let hasBig = $derived(acc?.kind === 'Deckbox');
  /** Shown with the own-model editor: figurines always, board games / card supplies when chosen. */
  const ownModel = (a: any) => a?.kind === 'Figurine' || (!!kindInfo(a?.kind)?.ownModel && useOwn);
  /** Vanilla item's display name from the game templates ("System Gate 1"), else its type. */
  const baseName = (b: string) => templates?.items?.find((i: any) => i.type === b)?.name || b;

</script>

<SaveBar dirty={draft.dirty} {saving} onsave={save} onrevert={revert}
  leaveMessage="Leave Accessories without saving? Your unsaved changes will be lost."
  revertMessage="Throw away every unsaved change on this page (edits, new, deleted and moved accessories)?">
  {#snippet title()}<h2>Accessories</h2>{/snippet}
</SaveBar>
<div class="page libpage">
  <!-- Kind tabs with counts; one short line about where these show up, the per-kind vanilla switch, problems. -->
  <nav class="tabbar">
    {#each TABS as tb}
      <button class:on={kind === tb.kind} onclick={() => switchTab(tb.kind)}>
        {tb.title}<span class="n">{all.filter((a: any) => a.kind === tb.kind).length}</span>
      </button>
    {/each}
  </nav>
  <div class="intro">
    <span class="muted">Sold next to the vanilla ones; the game reads them when you click <b>Save</b> (restart the game to see changes).
      To sell only yours: <b>Mod settings → Content → {kindInfo(kind)?.toggle ?? 'ShowVanilla…'}</b>.</span>
    {#if view && !view.installed && view.accessories.length}<span class="badge warn">The game doesn't read this setup yet — set the game folder in Settings</span>{/if}
  </div>
  {#if !templates}
    {#if tplStatus?.busy}
      <div class="notice info bar">Reading the game's models and art… {tplStatus.message ?? ''}</div>
    {:else}
      <div class="notice bar">The game's models and art aren't available{tplStatus?.error ? `: ${tplStatus.error}` : ' — set the game folder in Settings'}.</div>
    {/if}
  {/if}
  {#if view?.errors?.length}<div class="notice err bar">{#each view.errors as e}<div>{e}</div>{/each}</div>{/if}

  <div class="wrap">
    <aside class="side sidelist">
      <div class="sidetools">
        <input type="search" bind:value={query} placeholder="Search {TABS.find((tb) => tb.kind === kind)?.title.toLowerCase() ?? ''}…" />
        <select bind:value={sortBy} title="Sort" onchange={() => { try { localStorage.setItem('accessories.sort', sortBy); } catch {} }}>
          {#each Object.entries(SORT_LABELS) as [k, label]}<option value={k}>{label}</option>{/each}
        </select>
      </div>
      <div class="list" role="listbox" tabindex="-1" onkeydown={listKey} title="Ctrl+click or Shift+click to select several; Delete removes them">
        {#each shown as a (a.id)}
          <button class="item" class:active={a.id === selectedId} class:picked={picked.includes(a.id) && a.id !== selectedId} onclick={(e) => clickItem(e, a.id)}>
            <span class="ico">{#if a.icon}<img src={accUrl(a.icon)} alt="" />{:else if tplItem(a)?.icon}<img src={tplUrl(tplItem(a).icon)} alt="" />{/if}</span>
            <span class="info">
              <span class="name">{a.id === selectedId && acc ? acc.name : a.name}
                {#if draft.parked[a.id]?.problem}<span class="warn" title={draft.parked[a.id].problem}>⚠</span>{/if}
                {#if draft.isNew(a.id)}<span class="tag" title="New — not saved yet">new</span>{:else if draft.isEdited(a.id) || (a.id === selectedId && draft.shownDirty)}<span class="dot" title="Unsaved changes">•</span>{/if}</span>
              <span class="sub">Level {a.license.level}{originOf(a) ? ` · ${originName(originOf(a))}${originOf(a).author ? ` (${originOf(a).author})` : ''}` : ''}</span>
            </span>
          </button>
        {:else}
          <p class="muted small empty">{query ? 'Nothing matches.' : `No ${TABS.find((tb) => tb.kind === kind)?.title.toLowerCase() ?? 'items'} yet.`}</p>
        {/each}
      </div>
      <div class="sidefoot">
        <button class="primary" onclick={() => add(kind)}>+ New {TABS.find((tb) => tb.kind === kind)?.one}</button>
        <button onclick={() => (fromCatalog = true)} title="Accessories from your other setups and earlier imports — no re-import">Add from catalog…</button>
      </div>
    </aside>

    {#if acc}
      <div class="form">
        <header class="pane-hero">
          <div class="art">
            {#if acc.icon}<img src={accUrl(acc.icon)} alt="" title="Shop icon (saved)" />{:else if t?.icon}<img src={tplUrl(t.icon)} alt="" title="Vanilla icon" />{/if}
          </div>
          <div class="grow titlebox">
            <input class="title" bind:value={acc.name} aria-label="Name" placeholder="Name" />
            <div class="meta">
              <span>{kindInfo(acc.kind)?.one ?? acc.kind}</span>
              <span>from {baseName(baseOf(acc))}</span>
              <span>License level {acc.license.level}</span>
              <span class="code" title="Id">{acc.id}</span>
            </div>
          </div>
          <div class="heroactions">
            <button onclick={saveToCatalog} title="Make this setup's saved version the one other setups get when they add it from the catalog (setups that already have it keep theirs).">Update catalog</button>
            <button class="danger" onclick={remove}>Delete</button>
          </div>
        </header>

        <section class="block">
          <div class="bhead"><h2>Basics</h2></div>
          <div class="row head">
            <label class="field" title={acc.kind === 'Figurine'
              ? 'Vanilla toy whose shelf slot (how many fit and how big), shop tab and cost / market range are used.'
              : kindInfo(acc.kind)?.perBase
                ? 'Vanilla item whose model you paint (each has its own model) — or whose shelf slot your own model uses — and whose shop, cost and market range are the defaults.'
                : 'Vanilla item whose art is the starting point (and whose cost / market range are the defaults). All deck boxes share one model, all playmats another.'}>Start from
              <select value={baseOf(acc)} onchange={(e) => { acc.base = e.currentTarget.value; }}>
                {#each kindInfo(acc.kind)?.bases ?? [] as b}<option value={b}>{baseName(b)}{baseName(b) !== b ? ` (${b})` : ''}</option>{/each}
              </select>
            </label>
            <label class="field" title="Shop list it is sold from. Default: wherever the “Start from” item is sold.">Sold in
              <select value={acc.shop ?? ''} onchange={(e) => { acc.shop = e.currentTarget.value || undefined; }}>
                <option value="">Same as {baseName(baseOf(acc))}</option>
                {#each Object.entries(SHOPS) as [v, label]}<option value={v}>{label}</option>{/each}
              </select>
            </label>
            <div class="field order">Position in the list
              <div class="row">
                <button class="small" onclick={() => move(-1)} disabled={sortBy !== 'order' || items[0]?.id === acc.id}>▲ Earlier</button>
                <button class="small" onclick={() => move(1)} disabled={sortBy !== 'order' || items.at(-1)?.id === acc.id}>▼ Later</button>
              </div>
            </div>
          </div>
          <p class="muted small">{sortBy === 'order' ? "List order = this type's ladder order in Gamify → Accessories."
            : 'Sorted by ' + SORT_LABELS[sortBy].toLowerCase() + ' — choose “List order” to reorder the ladder.'}</p>
        </section>

        <section class="block">
          <div class="bhead"><h2>Design</h2><span class="grow"></span>
            <span class="thumbs">
              <span class="small muted">Saved:</span>
              {#if acc.icon}<img src={accUrl(acc.icon)} alt="icon" title="Shop icon" />{:else if t?.icon}<img src={tplUrl(t.icon)} alt="vanilla icon" title="Vanilla icon" />{/if}
              {#if acc.texture}<img src={accUrl(acc.texture)} alt="texture" title="Texture" />{/if}
            </span>
          </div>
          {#if kindInfo(acc.kind)?.perBase}
            <div class="row bases">
              {#each kindInfo(acc.kind)?.bases ?? [] as b}
                {@const it = templates?.items?.find((i: any) => i.type === b)}
                <button class:on={baseOf(acc) === b} title={`${baseName(b)} (${b})`} onclick={() => { acc.base = b; }}>
                  {#if it?.icon}<img src={tplUrl(it.icon)} alt="" />{/if}<span class="small">{baseName(b)}</span>
                </button>
              {/each}
            </div>
          {/if}
          {#if kindInfo(acc.kind)?.ownModel}
            <div class="seg">
              <button class:on={!useOwn} onclick={() => { useOwn = false; }}>Paint the game's model</button>
              <button class:on={useOwn} onclick={() => { useOwn = true; }}>My own 3D model (.glb / .gltf / .obj)</button>
            </div>
          {/if}
          {#key `${editorKey}|${kindInfo(acc.kind)?.perBase ? baseOf(acc) : ''}|${ownModel(acc)}`}
            {#if ownModel(acc)}
              <FigurineEditor bind:this={figEditor} base={baseOf(acc)} {templates} bind:layout={figLayout} {notify}
                noun={kindInfo(acc.kind)?.one ?? 'toy'}
                onimported={(n) => { if (/^Custom figurine( \d+)?$/i.test(acc.name)) { acc.name = n; } }} />
            {:else}
            <AccessoryEditor bind:this={editor} kind={acc.kind} base={baseOf(acc)} vanillaUrl={tplUrl(t?.texture)} vanillaIconUrl={tplUrl(t?.icon)} bind:layout
              iconSize={t?.iconRect ?? [512, 512]} {notify} onbaseline={() => (start = snap())} />
            {/if}
          {/key}
        </section>

        <section class="block">
          <div class="bhead"><h2>Shop &amp; license</h2>
            <span class="muted small">{hasBig ? 'Sells in a small and a big delivery box (two licenses).' : `Sells in one big delivery box (one license).`}
              Empty = the “Start from” item's vanilla value.</span></div>
          <div class="grid4">
            <label class="field">Cost<input type="number" step="0.5" min="0" placeholder={t ? String(t.baseCost) : 'model'} value={acc.cost ?? ''} oninput={(e) => { acc.cost = opt(e.currentTarget.value); }} /></label>
            <label class="field">Market min ×<input type="number" step="0.1" placeholder={t ? String(t.marketPriceMinPercent) : 'model'} value={acc.marketMin ?? ''} oninput={(e) => { acc.marketMin = opt(e.currentTarget.value); }} /></label>
            <label class="field">Market max ×<input type="number" step="0.1" placeholder={t ? String(t.marketPriceMaxPercent) : 'model'} value={acc.marketMax ?? ''} oninput={(e) => { acc.marketMax = opt(e.currentTarget.value); }} /></label>
            <div></div>
            <label class="field">{hasBig ? 'Small box license level' : 'License level'}<input type="number" min="1" bind:value={acc.license.level} /></label>
            <label class="field">{hasBig ? 'Small box license price' : 'License price'}<input type="number" min="0" bind:value={acc.license.price} /></label>
            {#if hasBig}
              <label class="field">Big box license level<input type="number" min="1" placeholder={String(acc.license.level + 3)} value={acc.license.bigLevel ?? ''} oninput={(e) => { acc.license.bigLevel = opt(e.currentTarget.value); }} /></label>
              <label class="field">Big box license price<input type="number" min="0" placeholder={String(Math.round(acc.license.price * 2))} value={acc.license.bigPrice ?? ''} oninput={(e) => { acc.license.bigPrice = opt(e.currentTarget.value); }} /></label>
            {/if}
          </div>
        </section>
      </div>
    {:else if view}
      <div class="emptypane">
        <h1>No {TABS.find((tb) => tb.kind === kind)?.title.toLowerCase()} yet</h1>
        <p class="muted">Make your own — paint the game's model or bring your own — or add one you made in another setup.</p>
        <div class="row center">
          <button onclick={() => (fromCatalog = true)}>Add from catalog…</button>
          <button class="primary" onclick={() => add(kind)}>+ New {TABS.find((tb) => tb.kind === kind)?.one}</button>
        </div>
      </div>
    {/if}
  </div>
</div>

{#if fromCatalog}<CatalogPicker kind="accessory" sub={kind} onadded={addedFromCatalog} onclose={() => (fromCatalog = false)} />{/if}

<style>
  .page { flex: 1; min-height: 0; display: flex; flex-direction: column; overflow: hidden; }
  .wrap { display: flex; flex: 1; min-height: 0; }

  .side { width: 280px; flex-shrink: 0; border-right: 1px solid var(--line); display: flex; flex-direction: column; min-height: 0; }

  .head { flex-wrap: wrap; align-items: flex-end; gap: 12px; } /* fields wrap instead of running past the box */
  .head > .field { min-width: 0; max-width: 100%; }
  .order .row { gap: 4px; }
  .bases { flex-wrap: wrap; gap: 6px; }
  .bases button { display: flex; flex-direction: column; align-items: center; gap: 2px; width: 92px; padding: 4px; }
  .bases button img { width: 56px; height: 56px; object-fit: contain; }
  .bases button.on { border-color: var(--accent); background: #22304d; }
  .grid4 { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 10px; }
  .thumbs { display: flex; gap: 8px; align-items: center; }
  .thumbs img { height: 40px; border-radius: 4px; background: var(--bg); }
</style>
