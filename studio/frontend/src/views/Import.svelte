<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { App, EventsOn, EventsOff, errText } from '../lib/api';
  import { smartArtForProject } from '../lib/smartArt';

  let { open, notify }: { open: (id: string) => void; notify: (t: string, k?: string) => void } = $props();

  const LANG_NAMES: Record<string, string> = { en: 'English', fr: 'Français', de: 'Deutsch', it: 'Italiano', es: 'Español', pt: 'Português', ja: '日本語' };

  function remembered(key: string, def: string): string {
    try { return localStorage.getItem('import.' + key) ?? def; } catch { return def; }
  }
  function remember(key: string, v: string) {
    try { localStorage.setItem('import.' + key, v); } catch {}
  }

  let sources = $state<any[]>([]);
  let sourceId = $state(remembered('source', 'scryfall'));
  let lang = $state(remembered('lang', 'en'));
  let sets = $state<any[]>([]);
  let loading = $state(true);
  let query = $state('');
  let group = $state('main');
  let options = $state<any>(null);
  let importing = $state<string | null>(null);
  let progress = $state<any>(null);

  const today = new Date().toISOString().slice(0, 10);
  const source = $derived(sources.find((s) => s.id === sourceId));
  const langs = $derived<string[]>(source?.languages ?? []);
  const groups = $derived([...new Set(sets.map((s) => s.group).filter(Boolean))]);
  const shown = $derived(
    sets.filter((s) => {
      if (group === 'main' && !s.main) return false;
      if (group !== 'main' && group !== 'all' && s.group !== group) return false;
      const q = query.trim().toLowerCase();
      return !q || s.name.toLowerCase().includes(q) || s.code.toLowerCase() === q;
    })
  );

  async function load(refresh = false) {
    if (!source) return;
    loading = true;
    const id = sourceId, l = langs.length ? lang : '';
    try {
      const got = await App.SourceSets(id, l, refresh);
      if (id === sourceId) sets = got;
    } catch (e) {
      notify(`Could not reach ${source.name}: ` + errText(e), 'error');
      sets = [];
    }
    loading = false;
  }

  async function pickSource(id: string) {
    if (importing || id === sourceId && options) return;
    sourceId = id;
    remember('source', id);
    sets = [];
    group = 'main';
    options = await App.DefaultImportOptions(id);
    if (langs.length && !langs.includes(lang)) lang = langs[0];
    load();
  }

  function pickLang() {
    remember('lang', lang);
    load();
  }

  // The shared library already has this set made another way (other format or size): ask what to do.
  let choice = $state<any>(null); // { set, art, resolve }
  const fmtName = (f: string) => (f === 'jpg' ? 'JPEG' : 'PNG');
  const widthName = (w: number) => (w ? `${w} px` : 'original size');

  function askLibrary(set: any, art: any): Promise<'use' | 'download' | ''> {
    return new Promise((resolve) => (choice = { set, art, resolve }));
  }
  function answer(v: 'use' | 'download' | '') {
    choice?.resolve(v);
    choice = null;
  }

  async function doImport(s: any) {
    const l = langs.length ? lang : '';
    let useLibrary = false;
    const art = await App.LibraryArt(sourceId, s.code, l).catch(() => null);
    if (art && (art.format !== (options.imageFormat || 'png') || art.width !== options.imageWidth)) {
      const v = await askLibrary(s, art);
      if (!v) return;
      useLibrary = v === 'use';
    }
    importing = s.code;
    progress = { stage: 'cards', done: 0, total: s.cards, message: 'Starting…' };
    try {
      const id = await App.ImportSet(sourceId, s.code, { ...options, lang: l, useLibrary });
      const done = progress?.message ?? 'Imported';
      // Pack & box art from the set's product photos (or its best card art); the import's simple art stays if this fails.
      try {
        const p: any = await App.LoadProject(id);
        await smartArtForProject(p, (m) => (progress = { ...progress, stage: 'art', message: m }));
        await App.SaveProject(p);
      } catch (e) { notify('Pack art: ' + errText(e) + ' — kept the simple generated art', 'error'); }
      notify(done, 'ok');
      open(id);
    } catch (e) {
      notify(errText(e), 'error');
    }
    importing = null;
    progress = null;
    load();
  }

  onMount(async () => {
    EventsOn('import:progress', (p: any) => (progress = p));
    sources = await App.ImportSources();
    if (!sources.some((s) => s.id === sourceId)) sourceId = sources[0]?.id;
    await pickSource(sourceId);
  });
  onDestroy(() => EventsOff('import:progress'));
</script>

<div class="page">
  <header class="row">
    <h2 class="grow">Import sets</h2>
    <button onclick={() => load(true)} disabled={loading}>Refresh list</button>
  </header>

  <div class="sources row">
    {#each sources as s (s.id)}
      <button class="source" class:active={s.id === sourceId} disabled={!!importing} onclick={() => pickSource(s.id)}>
        <b>{s.game}</b><span class="muted small">from {s.name}</span>
      </button>
    {/each}
  </div>

  <div class="filters row">
    <input class="grow" placeholder="Search sets by name or code…" bind:value={query} />
    <select bind:value={group}>
      <option value="main">Main sets</option>
      <option value="all">All sets</option>
      {#each groups as g}<option value={g}>{g}</option>{/each}
    </select>
    {#if langs.length}
      <select bind:value={lang} onchange={pickLang} title="Card language">
        {#each langs as l}<option value={l}>{LANG_NAMES[l] ?? l}</option>{/each}
      </select>
    {/if}
    {#if options}
      {#if source?.variants}
        <label class="check"><input type="checkbox" bind:checked={options.includeVariants} /> Include variant printings</label>
      {/if}
      <label class="check">Image width
        <select bind:value={options.imageWidth}>
          <option value={384}>384</option><option value={512}>512</option><option value={0}>Original</option>
        </select>
      </label>
      <label class="check" title="JPEG card art is about 6× smaller; card scans lose a little detail. Pack art always stays PNG. The default is set in Settings → Downloads.">Card image format
        <select bind:value={options.imageFormat}>
          <option value="png">PNG (best quality)</option><option value="jpg">JPEG (about 6× smaller)</option>
        </select>
      </label>
    {/if}
  </div>

  {#if progress}
    <div class="progress">
      <div class="row"><b class="grow">Importing {importing?.toUpperCase()}</b><button class="small" onclick={() => App.CancelImport()}>Cancel</button></div>
      <div class="bar"><div style="width:{progress.total ? (100 * progress.done) / progress.total : 5}%"></div></div>
      <div class="muted">{progress.stage === 'cards' ? 'Card list' : 'Images'} — {progress.message}</div>
    </div>
  {/if}

  {#if loading}
    <p class="muted">Loading sets from {source?.name ?? '…'}…</p>
  {:else}
    <p class="muted">{shown.length} sets</p>
    <div class="grid">
      {#each shown as s (s.code)}
        <div class="set">
          {#if s.icon}
            <img class:mono={s.iconMono} class:logo={!s.iconMono} src={s.icon} alt="" />
          {:else}
            <div class="noicon">{s.code.slice(0, 4).toUpperCase()}</div>
          {/if}
          <div class="grow">
            <div class="name">{s.name}{#if s.releasedAt && s.releasedAt > today}<span class="upcoming" title="Not released yet — card images may be missing">upcoming</span>{/if}</div>
            <div class="muted small">{[s.code.toUpperCase(), s.releasedAt, s.cards ? `${s.cards} cards` : ''].filter(Boolean).join(' · ')}{s.group ? ` · ${s.group}` : ''}</div>
          </div>
          {#if s.imported}
            <button onclick={() => open(s.projectId)}>Open</button>
          {:else}
            <button class="primary" disabled={!!importing} onclick={() => doImport(s)}>Import</button>
          {/if}
        </div>
      {/each}
    </div>
  {/if}
</div>


{#if choice}
  <div class="backdrop" role="presentation">
    <div class="dialog">
      <h3>{choice.set.name} is already on this PC</h3>
      <p>Its card art is in the shared library as <b>{fmtName(choice.art.format)}</b> ({widthName(choice.art.width)},
        {choice.art.cards} images), used by your other setups. You picked <b>{fmtName(options.imageFormat || 'png')}</b>
        ({widthName(options.imageWidth)}).</p>
      <div class="col">
        <button class="primary" onclick={() => answer('use')}>Use the {fmtName(choice.art.format)} art on this PC <span class="muted">— nothing to download, no extra space</span></button>
        <button onclick={() => answer('download')}>Download {fmtName(options.imageFormat || 'png')} <span class="muted">— a separate copy for this setup only</span></button>
        <button onclick={() => answer('')}>Cancel</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .backdrop { position: fixed; inset: 0; background: rgba(0, 0, 0, 0.6); display: flex; align-items: center; justify-content: center; z-index: 100; }
  .dialog { background: var(--panel); border: 1px solid var(--line); border-radius: var(--radius); padding: 18px 22px; max-width: 520px; display: flex; flex-direction: column; gap: 10px; }
  .dialog h3 { margin: 0; }
  .dialog p { margin: 0; }
  .col { display: flex; flex-direction: column; gap: 6px; }
  .col button { text-align: left; }
  .page { padding: 20px 24px; overflow: auto; height: 100%; }
  header { margin-bottom: 12px; }
  .sources { margin-bottom: 12px; flex-wrap: wrap; gap: 8px; }
  .source { display: flex; flex-direction: column; align-items: flex-start; gap: 2px; padding: 8px 14px; }
  .source.active { border-color: var(--accent); background: var(--panel); }
  .filters { margin-bottom: 12px; flex-wrap: wrap; }
  .progress { background: var(--panel); border: 1px solid var(--accent-2); border-radius: var(--radius); padding: 12px; margin-bottom: 12px; display: flex; flex-direction: column; gap: 8px; }
  .bar { height: 8px; background: var(--bg); border-radius: 4px; overflow: hidden; }
  .bar div { height: 100%; background: var(--accent); transition: width 0.2s; }
  .grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(360px, 1fr)); gap: 8px; }
  .set { display: flex; gap: 12px; align-items: center; background: var(--panel); border: 1px solid var(--line); border-radius: var(--radius); padding: 10px; }
  .set img.mono { width: 32px; height: 32px; filter: invert(1) opacity(0.85); }
  .set img.logo { width: 64px; height: 32px; object-fit: contain; }
  .noicon { width: 64px; height: 32px; display: grid; place-items: center; font-size: 11px; color: var(--muted, #888); border: 1px dashed var(--line); border-radius: 4px; }
  .name { font-weight: 600; }
  .upcoming { margin-left: 8px; font-size: 11px; font-weight: 500; padding: 1px 6px; border-radius: 8px; border: 1px solid var(--accent-2); color: var(--muted, #aaa); }
  .small { font-size: 12px; }
</style>
