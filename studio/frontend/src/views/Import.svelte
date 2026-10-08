<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { App, EventsOn, EventsOff, OnFileDrop, OnFileDropOff, errText, ask } from '../lib/api';
  import { smartArtForProject, boxesFromPacks } from '../lib/smartArt';
  import { renderAccessoryIcon } from '../lib/accessoryArt';

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
  // Sets the shared catalog has but this setup doesn't: added instantly instead of imported again.
  let inCatalog = $state<Record<string, boolean>>({});
  async function loadCatalog() {
    try { inCatalog = Object.fromEntries((await App.CatalogList('set')).filter((e: any) => !e.here).map((e: any) => [e.id, true])); } catch { /* optional */ }
  }
  async function addFromCatalog(row: any) {
    try {
      const r = await App.AddFromCatalog('set', [row.projectId]);
      if (r.added?.length) { row.imported = true; notify(`Added "${row.name ?? row.projectId}" from the catalog — Install it on the Sets page to play.`, 'ok'); }
      await loadCatalog();
    } catch (e) { notify(errText(e), 'error'); }
  }
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
    if (source.local) {
      sets = []; loading = false;
      if (source.id === 'epl') { if (eplPath) previewEPL(); } else if (folderPath) previewFolder();
      return;
    }
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
    // Keep the set's own rarities: on by default for every source, remembered per source once changed.
    options.keepRarities = remembered('keepRarities.' + id, '1') === '1';
    if (langs.length && !langs.includes(lang)) lang = langs[0];
    load();
  }

  function pickLang() {
    remember('lang', lang);
    load();
  }

  // Image folder (the player's own card images): pick a folder, preview what it makes, import.
  let folderPath = $state(remembered('folder', ''));
  let folderName = $state('');
  let preview = $state<any>(null);
  let previewErr = $state('');
  let stripNumbers = $state(remembered('stripNumbers', '') === '1');

  function toggleStrip() {
    remember('stripNumbers', stripNumbers ? '1' : '');
    if (folderPath) previewFolder();
  }

  async function chooseFolder() {
    try {
      const dir = await App.PickImportFolder();
      if (!dir) return;
      folderPath = dir;
      folderName = '';
      remember('folder', dir);
      await previewFolder();
    } catch (e) { notify(errText(e), 'error'); }
  }

  async function previewFolder() {
    const dir = folderPath;
    try {
      const p = await App.PreviewImportFolder(dir, { ...options, setName: folderName.trim(), stripNumbers } as any);
      if (dir !== folderPath) return;
      preview = p;
      previewErr = '';
      if (!folderName) folderName = p.name;
    } catch (e) {
      preview = null;
      previewErr = errText(e);
    }
  }

  // EPL mod (experimental): a mod made for Enhanced Prefab Loader, converted once into Studio sets.
  let eplPath = $state(remembered('epl', ''));
  let epl = $state<any>(null);
  let eplErr = $state('');
  let eplBusy = $state(false);
  let rarityChoice = $state<Record<string, Record<string, string>>>({}); // set code → tier → game rarity
  let pickSets = $state<Record<string, boolean>>({}); // set code → import it
  let pickItems = $state<Record<string, boolean>>({}); // item key → convert it
  // Where the content came from, saved with everything the import makes (shown as "from <mod>" and used by the Source sort).
  let originMod = $state('');
  let originAuthor = $state('');
  let originLink = $state('');
  let originFor = ''; // the mod path these were filled for
  const eplCount = $derived(Object.values(pickSets).filter(Boolean).length + Object.values(pickItems).filter(Boolean).length);
  const GAME_RARITIES = ['Common', 'Rare', 'Epic', 'Legendary'];

  async function useEPL(path: string) {
    if (!path) return;
    eplPath = path;
    remember('epl', path);
    await previewEPL();
  }

  async function chooseEPL(folder: boolean) {
    try { await useEPL(await (folder ? App.PickEPLFolder() : App.PickEPLMod())); } catch (e) { notify(errText(e), 'error'); }
  }

  // Dropping the mod's folder or .zip (or a file inside the folder) anywhere on the page while EPL mod is chosen.
  async function dropped(paths: string[]) {
    if (source?.id !== 'epl' || importing || eplBusy || !paths?.length) return;
    try { await useEPL(await App.EPLModPath(paths[0])); } catch (e) { notify(errText(e), 'error'); }
  }

  async function previewEPL() {
    const path = eplPath;
    eplBusy = true;
    try {
      const p = await App.PreviewEPL(path, stripNumbers);
      if (path !== eplPath) return;
      epl = p;
      eplErr = '';
      const rc: Record<string, Record<string, string>> = {};
      for (const set of p.sets ?? []) rc[set.code] = Object.fromEntries((set.tiers ?? []).map((t: any) => [t.name, t.rarity]));
      rarityChoice = rc;
      await loadCatalog();
      pickSets = Object.fromEntries((p.sets ?? []).map((x: any) => [x.code, !x.imported && !inCatalog[x.projectId]]));
      if (originFor !== path) { originMod = p.modName ?? ''; originAuthor = ''; originLink = ''; originFor = path; }
      pickItems = Object.fromEntries((p.items ?? []).map((x: any) => [x.key, !x.exists]));
    } catch (e) {
      epl = null;
      eplErr = errText(e);
    }
    eplBusy = false;
    progress = null;
  }

  // Import EPL mod: every ticked set (one after another) and accessory in one go.
  async function importEPL() {
    const sets = (epl?.sets ?? []).filter((x: any) => pickSets[x.code]);
    const items = (epl?.items ?? []).filter((x: any) => pickItems[x.key]).map((x: any) => x.key);
    if (sets.some((x: any) => x.imported) && !(await ask('Some ticked sets are already imported. Delete them first to import them again — they will be skipped now. Continue?'))) return;
    importing = epl.name;
    progress = { stage: 'cards', done: 0, total: 0, message: 'Starting…' };
    try {
      const r: any = await App.ImportEPL(eplPath, { sets: sets.filter((x: any) => !x.imported).map((x: any) => x.code), rarity: rarityChoice, items,
        options: { ...options, stripNumbers, originMod: originMod.trim(), originAuthor: originAuthor.trim(), originLink: originLink.trim() } } as any);
      const parts = [];
      if (r.sets.length) parts.push(`${r.sets.length} set${r.sets.length === 1 ? '' : 's'}`);
      if (r.items) parts.push(`${r.items} accessor${r.items === 1 ? 'y' : 'ies'}`);
      notify(parts.length ? `Imported ${parts.join(' and ')} from ${epl.name}` : 'Nothing was imported', parts.length ? 'ok' : 'error');
      for (const msg of r.problems ?? []) notify(msg, 'error');
      // Texture variants (e.g. each comic cover) get a shop icon of their own, rendered from their texture.
      if ((r.icons ?? []).length) {
        const tpl = await App.AccessoryTemplates().then((raw: string) => (raw ? JSON.parse(raw) : null)).catch(() => null);
        const models: Record<string, any> = {};
        const icons: Record<string, string> = {};
        let made = 0;
        for (const job of r.icons) {
          progress = { stage: 'items', done: made, total: r.icons.length, message: `Making icons… ${made + 1} / ${r.icons.length}` };
          try {
            models[job.kind] ??= await App.AccessoryModel(job.kind);
            const size = tpl?.items?.find((i: any) => i.type === job.base)?.iconRect ?? [512, 512];
            const url = `/acc/${job.texture.split('/').map(encodeURIComponent).join('/')}`;
            icons[job.id] = await renderAccessoryIcon(models[job.kind], url, size[0], size[1]);
            made++;
          } catch (e) { notify(`Icon for ${job.id}: ${errText(e)}`, 'error'); }
        }
        if (made) await App.SetAccessoryIcons(icons).catch((e: any) => notify('Saving icons: ' + errText(e), 'error'));
      }
      // Packs the mod sells without a box get one with the pack's front on it.
      for (const [k, id] of r.sets.entries()) {
        progress = { stage: 'art', done: k, total: r.sets.length, message: `Making boxes… ${k + 1} / ${r.sets.length}` };
        try {
          const p: any = await App.LoadProject(id);
          if (await boxesFromPacks(p)) await App.SaveProject(p);
        } catch (e) { notify(`Box art for ${id}: ${errText(e)}`, 'error'); }
      }
      if (r.sets.length === 1 && !r.items) open(r.sets[0]);
    } catch (e) {
      notify(errText(e), 'error');
    }
    importing = null;
    progress = null;
    await previewEPL();
  }

  const pct = (v: number) => (v >= 1 ? v.toFixed(2) : v >= 0.01 ? v.toFixed(3) : v.toFixed(4));

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
    const art = source?.local ? null : await App.LibraryArt(sourceId, s.code, l).catch(() => null);
    if (art && (art.format !== (options.imageFormat || 'png') || art.width !== options.imageWidth)) {
      const v = await askLibrary(s, art);
      if (!v) return;
      useLibrary = v === 'use';
    }
    importing = source?.local ? s.name : s.code;
    progress = { stage: 'cards', done: 0, total: s.cards, message: 'Starting…' };
    try {
      const folder = source?.id === 'folder';
      const id = await App.ImportSet(sourceId, s.code, { ...options, lang: l, useLibrary,
        setName: folder ? folderName.trim() : '', stripNumbers: folder && stripNumbers,
        rarityMap: source?.id === 'epl' ? rarityChoice[s.code] : options.rarityMap });
      const done = progress?.message ?? 'Imported';
      // Pack & box art from the set's product photos (or its best card art); the import's simple art stays if this fails.
      // Sources that bring their own pack art (EPL mods) keep it.
      if (!source?.ownArt) try {
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
    loadCatalog();
    EventsOn('import:progress', (p: any) => (progress = p));
    sources = await App.ImportSources();
    if (!sources.some((s) => s.id === sourceId)) sourceId = sources[0]?.id;
    await pickSource(sourceId);
  });
  onMount(() => OnFileDrop((_x: number, _y: number, paths: string[]) => dropped(paths), false));
  onDestroy(() => { EventsOff('import:progress'); OnFileDropOff(); });
</script>

<div class="page">
  <header class="row">
    <h2 class="grow">Import</h2>
    {#if !source?.local}<button onclick={() => load(true)} disabled={loading}>Refresh list</button>{/if}
  </header>

  <div class="sources row">
    {#each sources as s (s.id)}
      <button class="source" class:active={s.id === sourceId} disabled={!!importing} onclick={() => pickSource(s.id)}>
        <b>{s.game}</b><span class="muted small">from {s.name}</span>
      </button>
    {/each}
  </div>

  <div class="filters row">
    {#if !source?.local}
      <input class="grow" placeholder="Search sets by name or code…" bind:value={query} />
      <select bind:value={group}>
        <option value="main">Main sets</option>
        <option value="all">All sets</option>
        {#each groups as g}<option value={g}>{g}</option>{/each}
      </select>
    {/if}
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
      <label class="check" title="On: the set keeps the source's own rarities (Secret Rare, SR, Mythic…, or a mod's tiers) as rarities of its own in game — their names on the cards, in the binder and in Check Price, and pack odds per rarity. Off: every card gets one of the game's 4 rarities.">
        <input type="checkbox" bind:checked={options.keepRarities} onchange={() => remember('keepRarities.' + sourceId, options.keepRarities ? '1' : '')} />
        Keep the set's own rarities</label>
    {/if}
  </div>

  {#if progress}
    <div class="progress">
      <div class="row"><b class="grow">Importing {source?.local ? importing : importing?.toUpperCase()}</b><button class="small" onclick={() => App.CancelImport()}>Cancel</button></div>
      <div class="bar"><div style="width:{progress.total ? (100 * progress.done) / progress.total : 5}%"></div></div>
      <div class="muted">{progress.stage === 'cards' ? 'Card list' : progress.stage === 'items' ? 'Accessories' : 'Images'} — {progress.message}</div>
    </div>
  {/if}

  {#if source?.id === 'epl'}
    <div class="folder">
      <p class="note"><b>Experimental.</b> Converts the card sets of a mod made for Enhanced Prefab Loader into Studio sets, once:
        cards, the mod's pack &amp; box art and card back with its pack odds turned into ours, plus its accessories, figurines and
        furniture. EPL isn't needed in the game.
        The art belongs to the mod's authors, so converted sets stay on this PC (they're left out of setup exports).</p>
      <div class="row">
        <button class="primary" disabled={!!importing || eplBusy} onclick={() => chooseEPL(false)}
          title="Pick the mod's download: .zip, .rar or .7z — or drop it on this page">Choose download…</button>
        <button class="primary" disabled={!!importing || eplBusy} onclick={() => chooseEPL(true)}
          title="Pick the mod's folder after unpacking it (the one with BepInEx or plugins in it, or the mod's own folder) — or drop it on this page">Choose folder…</button>
        <span class="grow path" title={eplPath}>{eplPath || 'No mod chosen'}</span>
        {#if eplPath}<button class="small" disabled={!!importing || eplBusy} onclick={previewEPL}>Re-read</button>{/if}
      </div>
      <p class="muted small">Pick the mod's download (<b>.zip</b>, <b>.rar</b> or <b>.7z</b>) or, if you unpacked it, its folder — or just drag either onto this page.</p>
      <label class="check opt">
        <input type="checkbox" bind:checked={stripNumbers} onchange={() => { remember('stripNumbers', stripNumbers ? '1' : ''); if (eplPath) previewEPL(); }} disabled={!!importing || eplBusy} />
        <span><b>Strip leading numbers</b><br />
          <span class="muted small">Leaves out the number many mods put before their names (“001 - 2019 Base Playmat” → “2019 Base Playmat”)
            for the sets, packs, boxes, accessories, figurines and furniture. Only a number followed by a separator (- : . ) |) counts,
            so names like “2019 Base” stay whole. Card names are never changed.</span></span>
      </label>
      {#if eplBusy}<p class="muted">Reading the mod… {progress?.stage === 'unpack' && progress.total ? `(${progress.done} / ${progress.total} MB unpacked)` : ''}</p>{/if}
      {#if eplErr}<p class="err">{eplErr}</p>{/if}
      {#if epl}
        <div class="eplset">
          <b>About this mod</b>
          <p class="muted small">Saved with everything this import makes, so you can tell later where it came from (shown as “from …”,
            used by the Source sort). The name starts as the one inside the mod.</p>
          <div class="row origin">
            <label class="field grow">Mod name<input bind:value={originMod} placeholder={epl.modName} disabled={!!importing} /></label>
            <label class="field">Author<input bind:value={originAuthor} placeholder="optional" disabled={!!importing} /></label>
          </div>
          <label class="field">Link<input bind:value={originLink} placeholder="optional, e.g. the mod's Nexus page" disabled={!!importing} /></label>
        </div>
        {#each epl.warnings ?? [] as w}<p class="warn">⚠ {w}</p>{/each}
        {#if !(epl.sets ?? []).length}<p class="muted">This mod has no card sets.</p>{/if}
        {#each epl.sets ?? [] as set (set.code)}
          <div class="eplset">
            <div class="row">
              <label class="check grow"><input type="checkbox" bind:checked={pickSets[set.code]} disabled={!!importing || set.imported} /> <b>{set.name}</b></label>
              <span class="muted small">{set.cards} cards · {set.renderMode === 'FullImage' ? 'full card images' : 'art in the game frame'} · project {set.projectId}</span>
            </div>
            {#each set.packs ?? [] as pk}
              <div class="muted small">Pack <b>{pk.name}</b>{pk.box ? ` + box ${pk.box}` : ''} · {pk.strategy} odds · foil {pk.foilChance}% per card</div>
            {/each}
            {#each set.warnings ?? [] as w}<p class="warn">⚠ {w}</p>{/each}
            {#if set.problemCount}
              <p class="warn">⚠ {set.problemCount} card(s) can't be read and will be left out: {(set.problems ?? []).join('; ')}{set.problemCount > (set.problems ?? []).length ? ' …' : ''}</p>
            {/if}
            <details>
              {#if options?.keepRarities}
                <summary>Rarities ({(set.tiers ?? []).length} tiers in the mod, each kept as a rarity of its own)</summary>
                <p class="muted small">Every tier becomes a rarity of the set with the mod's own name, pack odds and card prices.
                  Per pack = cards of that tier in an average pack.</p>
                <table class="rar">
                  <thead><tr><th>Mod tier</th><th>Cards</th><th>Per pack</th></tr></thead>
                  <tbody>{#each set.tiers ?? [] as t}<tr><td>{t.name}</td><td>{t.cards}</td><td>{pct(t.perPack)}</td></tr>{/each}</tbody>
                </table>
              {:else}
                <summary>Rarities ({(set.tiers ?? []).length} tiers in the mod → our 4)</summary>
                <p class="muted small">Picked by how often the mod's packs give a card of each tier; change any of them. Per pack = cards of that tier in an average pack.
                  Tick “Keep the set's own rarities” above to keep every tier instead.</p>
                <table class="rar">
                  <thead><tr><th>Mod tier</th><th>Cards</th><th>Per pack</th><th>Game rarity</th></tr></thead>
                  <tbody>
                    {#each set.tiers ?? [] as t}
                      <tr><td>{t.name}</td><td>{t.cards}</td><td>{pct(t.perPack)}</td>
                        <td><select bind:value={rarityChoice[set.code][t.name]} disabled={!!importing}>
                          {#each GAME_RARITIES as r}<option value={r}>{r}</option>{/each}
                        </select></td></tr>
                    {/each}
                  </tbody>
                </table>
              {/if}
            </details>
            {#if set.imported}
              <div class="row">
                <span class="muted grow">Already imported — open it, or delete it first to import it again.</span>
                <button onclick={() => open(set.projectId)}>Open</button>
              </div>
            {:else if inCatalog[set.projectId]}
              <div class="row">
                <span class="muted grow">In the catalog already (imported in another setup or earlier) — add it instantly instead of converting again.</span>
                <button onclick={() => addFromCatalog(set)} title="Already imported in another setup or earlier — added instantly, no download">Add from catalog</button>
              </div>
            {/if}
          </div>
        {/each}
        {#if (epl.items ?? []).length}
          <div class="eplset">
            <div class="row"><b class="grow">Accessories, figurines &amp; furniture ({epl.items.length})</b>
              <button class="small" disabled={!!importing} onclick={() => (pickItems = Object.fromEntries(epl.items.map((x: any) => [x.key, true])))}>Select all</button>
              <button class="small" disabled={!!importing} onclick={() => (pickItems = Object.fromEntries(epl.items.map((x: any) => [x.key, false])))}>Deselect all</button></div>
            <p class="muted small">Go to the Accessories and Furniture pages with the mod's own textures and models; fine-tune them there
              (figurine size on the shelf, furniture item spots).</p>
            {#each epl.items as it (it.key)}
              <label class="check"><input type="checkbox" bind:checked={pickItems[it.key]} disabled={!!importing} />
                {it.name} <span class="muted small">— {it.kind}{it.note ? ` (${it.note})` : ''}{it.exists ? ' · already converted (replaced)' : ''}</span></label>
            {/each}
          </div>
        {/if}
        <div class="row">
          <span class="grow muted">{eplCount} selected</span>
          <button class="primary" disabled={!!importing || eplBusy || !eplCount} onclick={importEPL}>Import EPL mod</button>
        </div>
        {#if (epl.skipped ?? []).length}
          <details>
            <summary>Not imported ({epl.skipped.length})</summary>
            <ul>{#each epl.skipped as k}<li>{k.kind}: {k.name} <span class="muted">— {k.reason}</span></li>{/each}</ul>
          </details>
        {/if}
      {/if}
    </div>
  {:else if source?.id === 'folder'}
    <div class="folder">
      <div class="row">
        <button class="primary" disabled={!!importing} onclick={chooseFolder}>Choose folder…</button>
        <span class="grow path" title={folderPath}>{folderPath || 'No folder chosen'}</span>
        {#if folderPath}<button class="small" disabled={!!importing} onclick={previewFolder} title="Read the folder again">Re-read</button>{/if}
      </div>
      <label class="check opt">
        <input type="checkbox" bind:checked={stripNumbers} onchange={toggleStrip} disabled={!!importing} />
        <span><b>Strip leading numbers</b><br />
          <span class="muted small">For files named with a card number first, like <code>001 Captain Marvel.png</code>: that number
            becomes the card's number and is left out of its name (“Captain Marvel”, card 001). Leave it off when the file names are
            just the card names: names that start with a number then stay whole (<code>2099 Spider-Man.png</code> stays
            “2099 Spider-Man”) and cards are numbered 1, 2, 3… in file-name order.</span></span>
      </label>
      {#if previewErr}<p class="err">{previewErr}</p>{/if}
      {#if preview}
        <div class="row">
          <label class="check grow">Set name <input class="grow" bind:value={folderName} disabled={!!importing} /></label>
          <span class="muted small">project {preview.projectId}</span>
        </div>
        <div class="muted">
          <b>{preview.cards}</b> card images{preview.logo ? ' · set logo' : ''}{preview.csv ? ` · cards.csv: ${preview.csvMatched} of ${preview.csvRows} rows matched` : ''}{preview.rotated ? ` · ${preview.rotated} landscape (turned upright)` : ''}
        </div>
        {#if preview.samples?.length}
          <table class="rar">
            <thead><tr><th>File</th><th>Card #</th><th>Name</th></tr></thead>
            <tbody>{#each preview.samples as c}<tr><td class="muted">{c.file}</td><td>{c.number}</td><td>{c.name}</td></tr>{/each}
              {#if preview.cards > preview.samples.length}<tr><td class="muted" colspan="3">… {preview.cards - preview.samples.length} more</td></tr>{/if}</tbody>
          </table>
        {/if}
        <table class="rar">
          <thead><tr><th>Rarity</th><th>Cards</th><th>{options?.keepRarities ? "Counts as" : "In game"}</th></tr></thead>
          <tbody>{#each preview.rarities as r}<tr><td>{r.name}</td><td>{r.cards}</td><td>{r.game}</td></tr>{/each}</tbody>
        </table>
        {#each preview.warnings ?? [] as w}<p class="warn">⚠ {w}</p>{/each}
        {#if preview.unmatched?.length}<p class="muted small">Not found: {preview.unmatched.slice(0, 8).join(', ')}{preview.unmatched.length > 8 ? '…' : ''}</p>{/if}
        {#if preview.skipped?.length}<p class="muted small">Skipped: {preview.skipped.slice(0, 8).join(', ')}{preview.skipped.length > 8 ? '…' : ''}</p>{/if}
        <div class="row">
          {#if preview.imported}
            <span class="muted grow">Already imported — open it, or delete it first to import again.</span>
            <button onclick={() => open(preview.projectId)}>Open</button>
          {:else}
            <span class="grow"></span>
            <div class="btnstack">
              {#if inCatalog[preview.projectId]}
                <button class="primary" onclick={() => addFromCatalog(preview)} title="Already imported in another setup or earlier — added instantly, no download">Add from catalog</button>
              {:else}
                <button class="primary" disabled={!!importing || !preview.cards}
                  onclick={() => doImport({ code: preview.dir, name: folderName.trim() || preview.name, cards: preview.cards })}>Import</button>
              {/if}
            </div>
          {/if}
        </div>
      {/if}
      <details open={!folderPath}>
        <summary>How to set up the folder</summary>
        <p>Use card images you own (scans or screenshots). Studio copies them into a new set; your folder isn't changed.</p>
        <pre>My Set\                  ← set name = folder name
  logo.png               ← optional: set logo for the pack art
  cards.csv              ← optional, see below
  1 Common\  Captain Marvel.png …
  2 Rare\    …
  3 Legendary\ …</pre>
        <ul>
          <li><b>Rarity</b> = the subfolder, lowest first by name (a leading number like “1 ” only sets the order and is
            dropped from the name). Images not in a subfolder are Common. With <b>Keep the set's own rarities</b> ticked, each
            subfolder becomes a rarity of the set with that name (“Secret Rare”, “Gold Foil”…), in that order; unticked, they're
            put on the game's Common / Rare / Epic / Legendary (known names like Uncommon, Mythic, Secret by meaning, others
            by folder order).</li>
          <li><b>Name</b> = the file name (<code>Captain Marvel.png</code> → “Captain Marvel”; underscores become spaces).
            Cards are numbered 1, 2, 3… in file-name order — or tick <b>Strip leading numbers</b> when the files start with
            the card number. Adding or renaming other images later never changes which card is which in saves.</li>
          <li><b>cards.csv</b> (comma or semicolon separated): a <code>file</code> column plus any of <code>name, number,
            rarity, price, foilPrice, artist, text, type</code>. Its values beat the folder and file name; <code>price</code>
            (USD) is the real price Gamify uses, read again by Refresh prices.</li>
          <li>Card-shaped images (5:7, like 63×88 mm) fill the card; landscape ones are turned upright.</li>
        </ul>
      </details>
    </div>
  {:else if loading}
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
            <div class="btnstack">
              {#if inCatalog[s.projectId]}
                <button class="primary" onclick={() => addFromCatalog(s)} title="Already imported in another setup or earlier — added instantly, no download">Add from catalog</button>
              {:else}
                <button class="primary" disabled={!!importing} onclick={() => doImport(s)}>Import</button>
              {/if}
            </div>
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
  .btnstack { display: flex; flex-direction: column; gap: 6px; }
  .set { display: flex; gap: 12px; align-items: center; background: var(--panel); border: 1px solid var(--line); border-radius: var(--radius); padding: 10px; }
  .set img.mono { width: 32px; height: 32px; filter: invert(1) opacity(0.85); }
  .set img.logo { width: 64px; height: 32px; object-fit: contain; }
  .noicon { width: 64px; height: 32px; display: grid; place-items: center; font-size: 11px; color: var(--muted, #888); border: 1px dashed var(--line); border-radius: 4px; }
  .name { font-weight: 600; }
  .upcoming { margin-left: 8px; font-size: 11px; font-weight: 500; padding: 1px 6px; border-radius: 8px; border: 1px solid var(--accent-2); color: var(--muted, #aaa); }
  .small { font-size: 12px; }
  .folder { background: var(--panel); border: 1px solid var(--line); border-radius: var(--radius); padding: 14px; display: flex; flex-direction: column; gap: 10px; max-width: 820px; }
  .folder .path { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .folder .err { color: var(--danger, #e66); }
  .folder p { margin: 0; }
  .folder .note { color: var(--muted, #aaa); }
  .origin .field:last-child { width: 220px; }
  .eplset { border: 1px solid var(--line); border-radius: var(--radius); padding: 10px; display: flex; flex-direction: column; gap: 6px; }
  .folder .opt { align-items: flex-start; gap: 8px; max-width: 720px; }
  .folder .opt input { margin-top: 2px; width: 18px; height: 18px; }
  .rar { border-collapse: collapse; width: max-content; }
  .rar th, .rar td { text-align: left; padding: 2px 14px 2px 0; }
  .rar th { font-weight: 500; color: var(--muted, #aaa); }
  .folder pre { margin: 6px 0; padding: 8px; background: var(--bg); border-radius: 4px; }
  .folder ul { margin: 6px 0; padding-left: 20px; display: flex; flex-direction: column; gap: 4px; }
</style>
