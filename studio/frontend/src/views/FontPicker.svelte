<script lang="ts" module>
  type Font = { source: string; id: string; name: string };
  // Font files picked this run (App.PickFontFile), offered by every picker with pickFiles.
  let picked = $state<Font[]>([]);
</script>

<script lang="ts">
  // The font choice (Game look: shop name, door signs; the pack/box art: classic generator, text layers): the default, the
  // game's fonts, fonts installed on Windows, font files. Every font shows itself: its row draws `sample` in that font, loaded
  // as the row scrolls into view. `onpick` gets source "" (default), "game", "windows" or "file" with the font's id and name.
  // "Font file…": with pickFiles this picker asks for the file (App.PickFontFile) and offers it again later; without, it
  // calls onpick("file", "") and the page opens its own dialog (the shop sign copies the file into the setup).
  import { App, errText } from '../lib/api';
  import { fontFile } from '../lib/urls';
  import { fontFamily } from '../lib/shopLook';
  import Dropdown from './Dropdown.svelte';

  let { fonts = undefined, pickFiles = false, current = '', currentUrl = '', selected = null, defaultLabel, sample = 'Aa Bb Cc 123',
    onpick, notify }: {
    fonts?: Font[];                                // App.ShopSignFonts (loaded here when not given)
    pickFiles?: boolean;                           // "Font file…" picks here; picked files are listed ("Your font files")
    current?: string;                              // display name of the chosen font, '' = the default
    currentUrl?: string;                           // its file, so the button shows it in that font
    selected?: { source: string; id: string } | null; // the chosen listed font, marked in the list
    defaultLabel: string;
    sample?: string;
    onpick: (source: string, id: string, name: string) => void;
    notify?: (t: string, k?: string) => void;
  } = $props();

  let loaded = $state<Font[]>([]);
  $effect(() => { if (!fonts) App.ShopSignFonts().then((l: Font[]) => (loaded = l ?? [])).catch(() => (loaded = [])); });
  const all = $derived([...(pickFiles ? picked : []), ...(fonts ?? loaded)]);

  async function fromFile() {
    if (!pickFiles) { onpick('file', '', ''); return; }
    try {
      const f: Font = await App.PickFontFile();
      if (!f.id) return;
      if (!picked.some((p) => p.id === f.id)) picked = [...picked, f];
      onpick(f.source, f.id, f.name);
    } catch (e) { notify?.(errText(e), 'error'); }
  }

  let open = $state(false);
  let query = $state('');
  const shown = $derived(all.filter((f) => !query.trim() || f.name.toLowerCase().includes(query.trim().toLowerCase())));
  const GROUPS: [string, string][] = [['file', 'Your font files'], ['game', 'From the game'], ['windows', 'Installed on Windows']];

  // CSS family per font file URL once loaded ('loading', or '' = it failed: `why` says what the file request and the
  // browser answered, so a failure can be told apart: no file / an error answer / a font the browser refuses).
  let families = $state<Record<string, string>>({});
  let why = $state<Record<string, string>>({});
  function load(url: string) {
    if (!url || url in families) return;
    families[url] = 'loading';
    fontFamily(url).then((fam) => (families[url] = fam)).catch(async (e) => {
      families[url] = '';
      let file = '';
      try {
        const r = await fetch(url, { cache: 'no-store' });
        const b = await r.arrayBuffer();
        file = r.ok ? `HTTP ${r.status}, ${r.headers.get('content-type') || 'no type'}, ${b.byteLength} bytes`
          : `HTTP ${r.status} for ${url}: ${new TextDecoder().decode(b).slice(0, 200).trim()}`;
      } catch (f) { file = `request failed: ${f}`; }
      why[url] = `${file} · ${e?.name ?? 'Error'}: ${e?.message ?? e}`;
    });
  }
  /** Loads a row's font when it scrolls into view. */
  function lazy(node: HTMLElement, url: string) {
    const io = new IntersectionObserver((es) => { if (es.some((e) => e.isIntersecting)) { load(url); io.disconnect(); } });
    io.observe(node);
    return { destroy: () => io.disconnect() };
  }
  const css = (url: string) => (families[url] && families[url] !== 'loading' ? `"${families[url]}", sans-serif` : '');
  $effect(() => load(currentUrl));
  const isSelected = (f: Font) => !!selected && selected.source === f.source && selected.id === f.id;
</script>

<div class="field fontfield">Font
  <Dropdown bind:open width="380px" title="Pick a font: each one shows itself" cls="fontbtn">
    {#snippet label()}<span class="cur" class:dflt={!current} style:font-family={css(currentUrl)}>{current || defaultLabel}</span>{/snippet}
    {#snippet panel(close)}
      <input type="search" placeholder="Search fonts…" bind:value={query} />
      <div class="list">
        {#if !query.trim()}
          <button class="font" class:on={!current} onclick={() => { close(); onpick('', '', ''); }}>
            <span class="nm">{defaultLabel}</span><span class="sample dflt">{sample}</span></button>
        {/if}
        {#each GROUPS as [source, title] (source)}
          {#if shown.some((f) => f.source === source)}
            <div class="group">{title}</div>
            {#each shown.filter((f) => f.source === source) as f (f.source + '|' + f.id)}
              {@const url = fontFile(f)}
              <button class="font" class:on={isSelected(f)} use:lazy={url} onclick={() => { close(); onpick(f.source, f.id, f.name); }}>
                <span class="nm">{f.name}</span>
                {#if families[url] === ''}
                  <span class="why" title={why[url] ?? ''}>can't preview: {why[url] ?? 'checking…'}</span>
                {:else}
                  <span class="sample" style:font-family={css(url)} class:wait={families[url] === 'loading'}>{sample}</span>
                {/if}
              </button>
            {/each}
          {/if}
        {:else}
          <p class="muted small">No font matches.</p>
        {/each}
      </div>
      <button onclick={() => { close(); fromFile(); }}>Font file… <span class="muted small">(.ttf / .otf)</span></button>
    {/snippet}
  </Dropdown>
</div>

<style>
  .fontfield { display: flex; flex-direction: column; gap: 4px; font-size: 12px; color: var(--muted); }
  :global(.ddbtn.fontbtn) { min-width: 200px; justify-content: space-between; color: var(--text); }
  .cur { font-size: 15px; max-width: 260px; overflow: hidden; text-overflow: ellipsis; }
  .dflt { font-weight: 700; }
  input[type="search"] { width: 100%; }
  .list { max-height: 360px; overflow: auto; display: flex; flex-direction: column; gap: 2px; margin: 0 -6px; }
  .group { font-size: 11px; text-transform: uppercase; letter-spacing: 0.06em; color: var(--muted); padding: 8px 8px 2px; }
  .font { display: flex; flex-direction: column; gap: 2px; text-align: left; background: none; border: 1px solid transparent; padding: 6px 8px; border-radius: 6px; }
  .font:hover:not(:disabled) { background: var(--panel); border-color: transparent; }
  .font.on { border-color: var(--accent-2); background: #1c2740; }
  .nm { font-size: 11px; color: var(--muted); }
  .sample { font-size: 22px; line-height: 1.2; color: var(--text); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; min-height: 27px; }
  .sample.wait { opacity: 0.25; }
  .why { font-size: 11px; color: var(--warn); white-space: normal; user-select: text; }
</style>
