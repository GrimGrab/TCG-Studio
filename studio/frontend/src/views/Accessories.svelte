<script lang="ts">
  // Accessories: every edit (fields, design, new / deleted / moved entries) stays in the page's draft until Save
  // (lib/libraryDraft: the entry you leave is finished and parked; Save stores the whole draft at once).
  import { onMount } from 'svelte';
  import { Ev } from '../lib/events';
  import { LibraryDraft } from '../lib/libraryDraft.svelte';
  import SaveBar from './SaveBar.svelte';
  import { accFile, templateFile } from '../lib/urls';
  import { App, EventsOn, errText, originName } from '../lib/api';
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
  let shown = $derived(sortShown(items, sortBy));
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
    removeIds([...picked]);
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
  {#snippet extra()}<button onclick={() => App.OpenAccessoriesFolder()}>Open folder</button>{/snippet}
</SaveBar>
<div class="page">
  <p class="muted small">
    Your own deck boxes, playmats, sleeves, dice, comics, collection books, battle decks (on the booster-pack tab), figurines
    (your own 3D models), and board games and card supplies for the Tabletop Geek shop (paint one of the game's
    models, or bring your own), sold next to the vanilla ones. They are
    global (not part of a set) and read by the game when you click <b>Save</b> — restart the game to see changes. To sell only yours, turn
    off <b>Mod settings → Content → {kindInfo(kind)?.toggle ?? 'ShowVanilla…'}</b> (one switch per type).
    {#if view && !view.installed && view.accessories.length}<span class="warn">The game doesn't read this setup yet — set the game folder in Settings.</span>{/if}
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
        {tb.title} <span class="muted small">({all.filter((a: any) => a.kind === tb.kind).length})</span>
      </button>
    {/each}
  </div>

  <div class="wrap">
    <div class="list" role="listbox" tabindex="-1" onkeydown={listKey} title="Ctrl+click or Shift+click to select several; Delete removes them">
      <label class="sortby small">Sort
        <select bind:value={sortBy} onchange={() => { try { localStorage.setItem('accessories.sort', sortBy); } catch {} }}>
          {#each Object.entries(SORT_LABELS) as [k, label]}<option value={k}>{label}</option>{/each}
        </select></label>
      {#each shown as a (a.id)}
        <button class:active={a.id === selectedId} class:picked={picked.includes(a.id) && a.id !== selectedId} onclick={(e) => clickItem(e, a.id)}>
          <span class="ico">{#if a.icon}<img src={accUrl(a.icon)} alt="" />{:else if tplItem(a)?.icon}<img src={tplUrl(tplItem(a).icon)} alt="" />{/if}</span>
          <span class="grow">{a.id === selectedId && acc ? acc.name : a.name}
            {#if draft.parked[a.id]?.problem}<span class="warn small" title={draft.parked[a.id].problem}>⚠</span>{/if}{#if draft.isNew(a.id)}<span class="tag small" title="New — not saved yet">new</span>{:else if draft.isEdited(a.id) || (a.id === selectedId && draft.shownDirty)}<span class="dot" title="Unsaved changes">•</span>{/if}<br /><span class="muted small">Level {a.license.level}{originOf(a) ? ` · from ${originName(originOf(a))}${originOf(a).author ? ` (${originOf(a).author})` : ''}` : ''}</span></span>
        </button>
      {/each}
      <button onclick={() => add(kind)}>+ New {TABS.find((tb) => tb.kind === kind)?.one}</button>
      <button onclick={() => (fromCatalog = true)} title="Accessories from your other setups and earlier imports — no re-import">Add from catalog…</button>
    </div>

    {#if acc}
      <div class="form">
        <section>
          <div class="row head">
            <label class="field grow">Name<input bind:value={acc.name} /></label>
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
            <label class="field" style="width:150px">Id<input value={acc.id} readonly /></label>
          </div>
          <div class="row">
            <button class="small" onclick={() => move(-1)} disabled={sortBy !== 'order' || items[0]?.id === acc.id}>▲ Earlier</button>
            <button class="small" onclick={() => move(1)} disabled={sortBy !== 'order' || items.at(-1)?.id === acc.id}>▼ Later</button>
            <span class="muted small">{sortBy === 'order' ? "List order = this type's ladder order in Gamify → Accessories."
              : 'Sorted by ' + SORT_LABELS[sortBy].toLowerCase() + ' — choose “List order” to reorder the ladder.'}</span>
            <div class="grow"></div>
            <button class="small" onclick={saveToCatalog} title="Make this setup's saved version the one other setups get when they add it from the catalog (setups that already have it keep theirs).">Update catalog</button>
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
            <div class="row small">
              <label class="check"><input type="radio" checked={!useOwn} onchange={() => { useOwn = false; }} /> Paint the game's model</label>
              <label class="check"><input type="radio" checked={useOwn} onchange={() => { useOwn = true; }} /> My own 3D model (.glb / .gltf / .obj)</label>
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

        <section>
          <h3>Shop</h3>
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
          <p class="muted small">Empty cost/market = the "Start from" item's vanilla values. {hasBig ? 'Deck boxes sell in a small and a big delivery box (two licenses).' : `${kindInfo(acc.kind)?.title} sell in one big delivery box (one license).`}</p>
        </section>

      </div>
    {:else if view}
      <p class="muted" style="padding:20px">No {TABS.find((tb) => tb.kind === kind)?.title.toLowerCase()} yet — click “+ New {TABS.find((tb) => tb.kind === kind)?.one}”.</p>
    {/if}
  </div>
</div>

{#if fromCatalog}<CatalogPicker kind="accessory" sub={kind} onadded={addedFromCatalog} onclose={() => (fromCatalog = false)} />{/if}

<style>
  .page { padding: 16px 20px; flex: 1; min-height: 0; display: flex; flex-direction: column; gap: 10px; overflow: auto; }
  .tabs { border-bottom: 1px solid var(--line); padding-bottom: 8px; flex-wrap: wrap; }
  .tabs button.on { border-color: var(--accent); background: #22304d; }
  .bases { flex-wrap: wrap; gap: 6px; }
  .bases button { display: flex; flex-direction: column; align-items: center; gap: 2px; width: 92px; padding: 4px; }
  .bases button img { width: 56px; height: 56px; object-fit: contain; }
  .bases button.on { border-color: var(--accent); background: #22304d; }
  .check { display: flex; align-items: center; gap: 6px; }
  .wrap { display: flex; flex: 1; min-height: 0; }
  .list { width: 230px; flex-shrink: 0; border-right: 1px solid var(--line); padding: 10px; display: flex; flex-direction: column; gap: 4px; overflow: auto; }
  .list button { text-align: left; display: flex; gap: 8px; align-items: center; }
  .list button.active { border-color: var(--accent); background: #22304d; }
  .list button.picked { border-color: var(--accent); background: #1c2740; }
  .sortby { display: flex; align-items: center; gap: 6px; margin-bottom: 4px; }
  .sortby select { flex: 1; }
  .head { flex-wrap: wrap; align-items: flex-end; } /* fields wrap instead of running past the box */
  .head > .field { min-width: 0; max-width: 100%; }
  .head > .grow { flex: 1 1 180px; }
  .ico { width: 36px; height: 36px; flex-shrink: 0; display: flex; align-items: center; justify-content: center; }
  .ico img { max-width: 100%; max-height: 100%; }
  .form { flex: 1; min-width: 0; overflow: auto; padding: 14px 18px; display: flex; flex-direction: column; gap: 14px; }
  section { background: var(--panel); border: 1px solid var(--line); border-radius: var(--radius); padding: 12px; display: flex; flex-direction: column; gap: 10px; }
  .grid4 { display: grid; grid-template-columns: repeat(4, 1fr); gap: 8px; }
  .thumbs { display: flex; gap: 8px; align-items: center; }
  .thumbs img { height: 44px; border-radius: 4px; background: var(--bg); }
  .dot { color: var(--accent-2); margin-left: 4px; }
  .tag { color: var(--accent); margin-left: 4px; }
  .small { font-size: 12px; }
  .warn { color: #d9a400; }
  .err { color: var(--danger); }
</style>
