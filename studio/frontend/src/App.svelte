<script lang="ts">
  import { onMount } from 'svelte';
  import { canLeave } from './lib/leaveGuard';
  import { Ev } from './lib/events';
  import { App, EventsOn, BrowserOpenURL, ask, errText } from './lib/api';
  import Projects from './views/Projects.svelte';
  import Import from './views/Import.svelte';
  import { queue as importQueue } from './lib/importQueue.svelte';
  import Settings from './views/Settings.svelte';
  import Editor from './views/Editor.svelte';
  import Gamify from './views/Gamify.svelte';
  import Setup from './views/Setup.svelte';
  import Accessories from './views/Accessories.svelte';
  import Furniture from './views/Furniture.svelte';
  import Decorations from './views/Decorations.svelte';
  import ShopLook from './views/ShopLook.svelte';
  import ModSettings from './views/ModSettings.svelte';
  import Setups from './views/Setups.svelte';
  import Storage from './views/Storage.svelte';
  import Catalog from './views/Catalog.svelte';

  type View = 'setup' | 'setups' | 'storage' | 'catalog' | 'projects' | 'import' | 'accessories' | 'furniture' | 'decorations' | 'shoplook' | 'gamify' | 'modsettings' | 'settings' | 'editor';
  let view = $state<View>('projects');
  let openId = $state('');
  let storageBusy = $state(false); // a Storage page task is running (it keeps running on other pages)
  const AUTO_TASK = 'Updating your content for the game'; // App.runAutoUpgrade's storage task label
  let upgradeChoices = $state(0); // differing art waiting for the player's choice (Storage → Finish the upgrade)
  let upgradeWaiting = $state(''); // why the start-up upgrade couldn't run (the game is running)

  // Start-up (and after Storage tasks / "Try again"): what the mod needs from an older TCG Studio happens on its own
  // (App.AutoUpgrade), then, once per Studio version, installed sets get what this version adds (App.SyncInstalledSets).
  // One message for all of it.
  async function startupTasks(sync: boolean) {
    const said: string[] = [];
    try {
      const r = await App.AutoUpgrade();
      upgradeWaiting = r.waiting || '';
      upgradeChoices = r.choices || 0;
      if (r.files > 0 || r.notes?.length) said.push('Your content was moved to where the game now reads it' + (r.notes?.length ? ': ' + r.notes.join('; ') : '') + '.');
      if (r.modUpdated) said.push('The mod in the game was updated.');
      if (r.modUpdated || r.files > 0 || r.notes?.length) await refreshStatus();
    } catch (e) {
      notify('Updating your content for the game failed: ' + errText(e), 'error');
    }
    if (sync) {
      try {
        const msg = await App.SyncInstalledSets();
        if (msg) said.push(msg);
      } catch (e) {
        notify(errText(e), 'error');
      }
    }
    if (said.length) notify(said.join(' '), 'ok');
  }
  let gameStatus = $state<any>(null);
  let setupReady = $state(true);
  let version = $state<any>(null);
  let update = $state<any>(null);        // newer release, if any
  let updateHidden = $state(false);      // "Later" for this session
  let updating = $state(false);
  let updateProgress = $state(0);
  let checking = $state(false);
  let toast = $state<{ text: string; kind: string } | null>(null);
  let setupName = $state('');   // active setup, shown under the logo
  let setupLoaded = $state(false); // the first list arrived (it loads after the start-up checks)
  let setupKey = $state(0);     // bumped after a setup switch so every view reloads
  let setupList = $state<any[]>([]);
  let setupMenu = $state(false);
  let setupAction = $state<any>(null); // one-shot request for the Setups page: { create: true } or { switchTo: id }
  let toastTimer: number | undefined;
  const DISCORD_URL = 'https://discord.gg/Pf75vnuN6B';

  function notify(text: string, kind = 'info') {
    toast = { text, kind };
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => (toast = null), kind === 'error' ? 8000 : 3500) as unknown as number;
  }

  function open(id: string) {
    openId = id;
    view = 'editor';
  }

  async function refreshSetup() {
    try {
      const v = await App.ListSetups();
      setupList = v.setups;
      setupName = v.setups.find((s: any) => s.active)?.name ?? '';
      if (v.message) notify(v.message, 'ok');
      if (v.error) notify('Setups: ' + v.error, 'error');
    } catch { setupName = ''; }
    setupLoaded = true;
  }

  // Sidebar: line icons (24×24 paths) and the setup picker's avatars.
  const ICONS: Record<string, string[]> = {
    sets: ['M8 3h10a2 2 0 0 1 2 2v12a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2z', 'M3 7v11a3 3 0 0 0 3 3h9'],
    import: ['M12 3v12', 'M7 10l5 5 5-5', 'M4 20h16'],
    dice: ['M7 3h10a4 4 0 0 1 4 4v10a4 4 0 0 1-4 4H7a4 4 0 0 1-4-4V7a4 4 0 0 1 4-4z', 'M8.5 8.5h.01', 'M15.5 15.5h.01', 'M15.5 8.5h.01', 'M8.5 15.5h.01', 'M12 12h.01'],
    shelf: ['M4 3v18', 'M20 3v18', 'M4 9h16', 'M4 15h16', 'M7 6.5h3', 'M13 12.5h4'],
    sparkle: ['M11 3l1.8 4.7L17.5 9.5l-4.7 1.8L11 16l-1.8-4.7L4.5 9.5l4.7-1.8z', 'M18.5 14.5l.8 2 2 .8-2 .8-.8 2-.8-2-2-.8 2-.8z'],
    store: ['M3 9l1.6-5h14.8L21 9', 'M3 9a3 3 0 0 0 6 0 3 3 0 0 0 6 0 3 3 0 0 0 6 0', 'M5 12v8h14v-8', 'M10 20v-5h4v5'],
    chart: ['M3 21h18', 'M6 17v-5', 'M11 17V7', 'M16 17v-7', 'M21 17V4'],
    sliders: ['M4 6h9', 'M17 6h3', 'M15 4v4', 'M4 12h3', 'M11 12h9', 'M9 10v4', 'M4 18h11', 'M19 18h1', 'M17 16v4'],
    books: ['M4 4h4v16H4z', 'M10 4h4v16h-4z', 'M16.5 5.2l3.8-1 3 14.6-3.8 1z'],
    drive: ['M4 6c0-1.7 3.6-3 8-3s8 1.3 8 3-3.6 3-8 3-8-1.3-8-3z', 'M4 6v12c0 1.7 3.6 3 8 3s8-1.3 8-3V6', 'M4 12c0 1.7 3.6 3 8 3s8-1.3 8-3'],
    pad: ['M7 7h10a5 5 0 0 1 5 5v1a3 3 0 0 1-5.4 1.8L15 13H9l-1.6 1.8A3 3 0 0 1 2 13v-1a5 5 0 0 1 5-5z', 'M7 9.5v3', 'M5.5 11h3', 'M16 10h.01', 'M18 12h.01'],
    gear: ['M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6z', 'M12 2v3', 'M12 19v3', 'M2 12h3', 'M19 12h3', 'M4.9 4.9L7 7', 'M17 17l2.1 2.1', 'M4.9 19.1L7 17', 'M17 7l2.1-2.1'],
    chat: ['M4 5h16v11H9l-5 4z', 'M9 10h.01', 'M15 10h.01']
  };
  const activeSetup = $derived(setupList.find((s: any) => s.active));
  /** A colour per setup (from its id), so setups are easy to tell apart. */
  function hue(id: string): number {
    let h = 0;
    for (const ch of id ?? '') h = (h * 31 + ch.charCodeAt(0)) % 360;
    return h;
  }
  function initials(name: string): string {
    const w = (name || '?').trim().split(/\s+/);
    return (w.length > 1 ? w[0][0] + w[1][0] : w[0].slice(0, 2)) || '?';
  }
  function setupFacts(s: any): string {
    const items = (s.accessories ?? 0) + (s.furniture ?? 0) + (s.decorations ?? 0);
    return [`${s.sets} set${s.sets === 1 ? '' : 's'}`, items ? `${items} item${items === 1 ? '' : 's'}` : '', s.hasSaves ? 'saves' : ''].filter(Boolean).join(' · ');
  }

  // The menu is fixed to the window (it's wider than the sidebar, which scrolls), placed under the picker.
  let setupButton = $state<HTMLElement>();
  let menuPos = $state({ left: 0, top: 0 });
  function toggleSetupMenu() {
    const r = setupButton?.getBoundingClientRect();
    if (r) menuPos = { left: r.left, top: r.bottom + 6 };
    setupMenu = !setupMenu;
    if (setupMenu) refreshSetup();
  }

  // Pages with unsaved changes register a leave guard (lib/leaveGuard.ts); leaving them asks first.
  async function go(v: View) {
    if (v === view || (await canLeave())) view = v;
  }

  async function openSetups(action: any = null) {
    setupMenu = false;
    if (!(await canLeave())) return;
    setupAction = action;
    view = 'setups';
  }

  function setupSwitched() {
    setupKey++;
    refreshSetup();
    refreshStatus();
  }

  async function refreshStatus() {
    gameStatus = await App.GameStatus();
    setupReady = (await App.SetupState()).ready;
  }

  async function checkUpdates(manual: boolean) {
    if (!version?.canUpdate) {
      if (manual) notify('This is a development build — updates are only for released versions.');
      return;
    }
    checking = true;
    try {
      update = await App.CheckForUpdate();
      updateHidden = false;
      if (manual && !update) notify(`You're up to date (v${version.version}).`, 'ok');
    } catch (e) {
      if (manual) notify("Couldn't check for updates: " + errText(e), 'error');
    }
    checking = false;
  }

  async function installUpdate() {
    updating = true;
    updateProgress = 0;
    try {
      await App.InstallUpdate(); // the app restarts itself on success
    } catch (e) {
      notify(errText(e), 'error');
      updating = false;
    }
  }

  // Open on Game & mod when something isn't installed yet (e.g. a friend's first run) or right after an update
  // (the mod in the game then needs updating too).
  let ready = $state(false);
  onMount(async () => {
    await App.WaitReady(); // the backend's startup runs alongside the page load
    ready = true;
    EventsOn(Ev.UpdateProgress, (f: number) => (updateProgress = f));
    EventsOn(Ev.StorageProgress, () => (storageBusy = true));
    // After a Storage task (e.g. Finish the upgrade) the banner follows what's left; not after the start-up run's own task.
    EventsOn(Ev.StorageDone, (label: string) => { storageBusy = false; if (label !== AUTO_TASK) startupTasks(false); });
    version = await App.AppVersion();
    await refreshStatus();
    await startupTasks(true); // may update the mod: before the notice below
    refreshSetup();
    if (version.updatedTo) {
      notify(`Updated to v${version.updatedTo}.` + (setupReady ? '' : ' Update the mod in the game on the Game & mod page.'), 'ok');
    }
    if (!setupReady) view = 'setup';
    checkUpdates(false);

  });
</script>

<div class="shell">
  <nav>
    <div class="brand"><span class="logo" aria-hidden="true"><span></span><span></span></span>TCG <b>Studio</b></div>
    <!-- The active setup's pages sit in one box under its picker; then the pages every setup shares. -->
    <div class="setupbox">
        <div class="setup-pick">
          <!-- Until the first list arrives (after the start-up checks): a quiet placeholder, not "unavailable". -->
          <button class="setup-current" bind:this={setupButton} class:open={setupMenu} class:loading={!setupLoaded} disabled={!setupLoaded}
            title={setupLoaded ? 'Switch setup or make a new one' : 'Loading your setups…'} onclick={toggleSetupMenu}>
            {#if setupLoaded}
              <span class="avatar" style="--h:{hue(activeSetup?.id ?? setupName)}">{initials(setupName)}</span>
            {:else}
              <span class="avatar ph"></span>
            {/if}
            <span class="who">
              <span class="lbl">Setup</span>
              {#if setupLoaded}
                <span class="name" class:muted={!setupName}>{setupName || 'Setups unavailable'}</span>
              {:else}
                <span class="name muted">Loading…</span>
              {/if}
            </span>
            <span class="caret">▾</span>
          </button>
          {#if setupMenu}
            <div class="menu-catcher" role="presentation" onclick={() => (setupMenu = false)}></div>
            <div class="menu" role="menu" style="left:{menuPos.left}px; top:{menuPos.top}px">
              <div class="menu-head">Switch setup</div>
              {#each setupList as s (s.id)}
                <button role="menuitem" class:current={s.active} disabled={s.active} onclick={() => openSetups({ switchTo: s.id })}
                  title={s.description || s.name}>
                  <span class="avatar sm" style="--h:{hue(s.id)}">{initials(s.name)}</span>
                  <span class="who">
                    <span class="name">{s.name}</span>
                    <span class="sub">{setupFacts(s)}</span>
                  </span>
                  {#if s.active}<span class="tick">✓</span>{/if}
                </button>
              {/each}
              <div class="sep"></div>
              <button role="menuitem" onclick={() => openSetups({ create: true })}><span class="mi">+</span>New setup…</button>
              <button role="menuitem" onclick={() => openSetups()}><span class="mi">⚙</span>Manage setups…</button>
            </div>
          {/if}
        </div>
      <div class="group">
        {@render item('projects', 'Sets', 'sets', view === 'projects' || view === 'editor')}
        {@render item('import', 'Import', 'import', view === 'import', importQueue.current ? 'busy' : '',
          importQueue.jobs.length ? `Importing ${importQueue.current} — ${importQueue.jobs.length} more queued` : `Importing ${importQueue.current}`)}
        {@render item('accessories', 'Accessories', 'dice', view === 'accessories')}
        {@render item('furniture', 'Furniture', 'shelf', view === 'furniture')}
        {@render item('decorations', 'Decorations', 'sparkle', view === 'decorations')}
        {@render item('shoplook', 'Game look', 'store', view === 'shoplook')}
        {@render item('gamify', 'Gamify', 'chart', view === 'gamify')}
        {@render item('modsettings', 'Mod settings', 'sliders', view === 'modsettings')}
      </div>
    </div>
    <div class="group-label">All setups</div>
    {@render item('catalog', 'Catalog', 'books', view === 'catalog')}
    {@render item('storage', 'Storage', 'drive', view === 'storage', storageBusy ? 'busy' : '', 'A storage task is running')}
    {@render item('setup', 'Game & mod', 'pad', view === 'setup', setupReady ? '' : 'warn', 'Something needs installing')}
    {@render item('settings', 'Settings', 'gear', view === 'settings')}
    <div class="grow"></div>
    <button class="discord" title="Opens {DISCORD_URL} in your browser" onclick={() => BrowserOpenURL(DISCORD_URL)}>
      <span class="discord-title">{@render icon('chat')}Join our Discord</span>
      <span class="discord-sub">Share workspaces, request features, show off your sets and get help</span>
    </button>
    <div class="foot">
      {#if gameStatus}
        {#if !gameStatus.found}
          <button class="state err" onclick={() => go('settings')} title="Set the game folder in Settings"><span class="sdot"></span>Game not found</button>
        {:else if !setupReady}
          <button class="state warn" onclick={() => go('setup')} title="Open Game & mod"><span class="sdot"></span>Install needed</button>
        {:else}
          <span class="state ok"><span class="sdot"></span>Game + mod ready</span>
        {/if}
      {/if}
      {#if version}
        <div class="version">
          <span>v{version.version}</span>
          <button class="linkish" onclick={() => checkUpdates(true)} disabled={checking || updating}>{checking ? 'Checking…' : 'Check for updates'}</button>
        </div>
      {/if}
    </div>
  </nav>

  <main>
    {#if update && !updateHidden}
      <div class="update-banner">
        <div class="grow">
          <b>TCG Studio v{update.version} is available</b> <span class="muted">(you have v{version.version})</span>
          {#if update.notes}<div class="notes">{update.notes}</div>{/if}
          {#if updating}<div class="bar"><div style="width:{Math.round(updateProgress * 100)}%"></div></div>{/if}
        </div>
        <button class="primary" onclick={installUpdate} disabled={updating}>{updating ? `Updating… ${Math.round(updateProgress * 100)}%` : 'Update now'}</button>
        <button onclick={() => (updateHidden = true)} disabled={updating}>Later</button>
      </div>
    {/if}
    {#if upgradeWaiting}
      <div class="update-banner">
        <div class="grow"><b>The game is running.</b> <span class="muted">{upgradeWaiting}</span></div>
        <button class="primary" onclick={() => startupTasks(false)}>Try again</button>
      </div>
    {:else if upgradeChoices > 0 && view !== 'storage'}
      <div class="update-banner">
        <div class="grow"><b>Some sets have their own art that differs from the shared art.</b>
          <span class="muted">The game shows the shared art until you choose which to keep.</span></div>
        <button class="primary" onclick={() => go('storage')}>Choose on Storage</button>
      </div>
    {/if}
    {#key setupKey}
    {#if !ready}
      <p class="muted" style="padding:20px 24px">Loading…</p>
    {:else if view === 'setup'}
      <Setup {notify} onchange={refreshStatus} />
    {:else if view === 'setups'}
      <Setups {notify} onswitched={setupSwitched} action={setupAction} ondone={() => (setupAction = null)} />
    {:else if view === 'projects'}
      <Projects {open} {notify} />
    {:else if view === 'import'}
      <Import {open} {notify} />
    {:else if view === 'accessories'}
      <Accessories {notify} />
    {:else if view === 'furniture'}
      <Furniture {notify} />
    {:else if view === 'decorations'}
      <Decorations {notify} />
    {:else if view === 'shoplook'}
      <ShopLook {notify} />
    {:else if view === 'gamify'}
      <Gamify {notify} />
    {:else if view === 'modsettings'}
      <ModSettings {notify} />
    {:else if view === 'storage'}
      <Storage {notify} />
    {:else if view === 'catalog'}
      <Catalog {notify} />
    {:else if view === 'settings'}
      <Settings {notify} onchange={refreshStatus} />
    {:else if view === 'editor'}
      {#key openId}
        <Editor id={openId} {notify} close={() => (view = 'projects')} />
      {/key}
    {/if}
    {/key}
  </main>

  {#if toast}
    <div class="toast {toast.kind}" role="status">{toast.text}</div>
  {/if}
</div>

{#snippet icon(name: string)}
  <svg class="ic" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
    {#each ICONS[name] ?? [] as d}<path {d} />{/each}
  </svg>
{/snippet}

{#snippet item(v: View, label: string, ic: string, active: boolean, dot = '', dotTitle = '')}
  <button class="item" class:active onclick={() => go(v)}>
    {@render icon(ic)}<span class="label">{label}</span>
    {#if dot}<span class="dot" class:busy={dot === 'busy'} title={dotTitle}></span>{/if}
  </button>
{/snippet}

<style>
  .shell { display: flex; height: 100vh; }
  nav {
    width: 220px; flex-shrink: 0; background: var(--panel); border-right: 1px solid var(--line);
    display: flex; flex-direction: column; gap: 2px; padding: 14px 10px 12px; overflow-y: auto; overflow-x: hidden;
  }
  .brand { display: flex; align-items: center; gap: 10px; font-size: 18px; font-weight: 600; padding: 2px 6px 14px; letter-spacing: 0.01em; }
  .brand b { color: var(--accent); font-weight: 700; }
  /* Two fanned cards. */
  .logo { position: relative; width: 22px; height: 26px; flex: none; }
  .logo span { position: absolute; width: 16px; height: 22px; border-radius: 3px; top: 2px; }
  .logo span:first-child { left: 0; background: #3f6ee0; transform: rotate(-12deg); }
  .logo span:last-child { left: 6px; background: linear-gradient(160deg, #5b8cff, #b66dff); transform: rotate(8deg); box-shadow: -2px 1px 4px #0006; }

  /* The active setup: its picker on top, its pages inside the same box. */
  .setupbox { background: var(--bg); border: 1px solid var(--line); border-radius: 12px; padding: 6px; display: flex; flex-direction: column; gap: 6px; }
  .setup-pick { position: relative; }
  nav .setup-current {
    width: 100%; display: flex; align-items: center; gap: 9px; padding: 7px 8px; background: var(--panel-2); border-color: var(--line); border-radius: 8px;
  }
  nav .setup-current:hover:not(:disabled), nav .setup-current.open { border-color: var(--accent); }
  .avatar { width: 30px; height: 30px; flex: none; border-radius: 8px; display: grid; place-items: center; font-size: 12px; font-weight: 700; color: #fff;
    background: linear-gradient(145deg, hsl(var(--h) 65% 52%), hsl(calc(var(--h) + 40) 60% 38%)); text-transform: uppercase; letter-spacing: 0.02em; }
  .avatar.sm { width: 26px; height: 26px; border-radius: 7px; font-size: 11px; }
  .avatar.ph { background: var(--line); animation: pulse 1.2s ease-in-out infinite; }
  nav .setup-current.loading { opacity: 1; cursor: default; } /* disabled while loading, but not greyed out */
  .setup-current.loading .caret { visibility: hidden; }
  .who { flex: 1; min-width: 0; display: flex; flex-direction: column; line-height: 1.25; }
  .who .lbl { font-size: 10px; text-transform: uppercase; letter-spacing: 0.08em; color: var(--muted); }
  .who .name { font-weight: 600; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .who .sub { font-size: 11px; color: var(--muted); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .caret { color: var(--muted); font-size: 11px; }
  .menu-catcher { position: fixed; inset: 0; z-index: 49; }
  .menu {
    position: fixed; width: 270px; z-index: 50; background: var(--panel-2);
    border: 1px solid var(--line); border-radius: 10px; box-shadow: 0 10px 30px #000a; padding: 6px;
    display: flex; flex-direction: column; gap: 2px; max-height: 65vh; overflow: auto;
  }
  .menu-head { font-size: 11px; font-weight: 600; letter-spacing: 0.06em; text-transform: uppercase; color: var(--muted); padding: 4px 8px 6px; }
  nav .menu button { display: flex; align-items: center; gap: 9px; border-color: transparent; padding: 6px 8px; border-radius: 7px; text-align: left; }
  nav .menu button:hover:not(:disabled) { background: var(--panel); border-color: transparent; }
  nav .menu button.current { opacity: 1; cursor: default; background: #1c2740; }
  .menu .tick { color: var(--ok); font-weight: 700; }
  .menu .mi { width: 26px; text-align: center; color: var(--muted); }
  .menu .sep { height: 1px; background: var(--line); margin: 4px 2px; }

  .group { display: flex; flex-direction: column; gap: 1px; }
  .group-label { margin: 16px 8px 4px; font-size: 11px; font-weight: 600; letter-spacing: 0.06em; text-transform: uppercase; color: var(--muted); }
  nav button { text-align: left; background: transparent; border-color: transparent; }
  nav .item { position: relative; display: flex; align-items: center; gap: 10px; padding: 6px 10px; border-radius: 7px; color: var(--muted); }
  nav .item:hover:not(:disabled) { color: var(--text); background: var(--panel-2); border-color: transparent; }
  nav .item.active { color: var(--text); background: #22304d; }
  nav .item.active::before { content: ''; position: absolute; left: -6px; top: 7px; bottom: 7px; width: 3px; border-radius: 2px; background: var(--accent); }
  nav .item.active .ic { color: var(--accent); }
  .item .label { flex: 1; min-width: 0; }
  .ic { width: 17px; height: 17px; flex: none; }
  .dot { display: inline-block; width: 8px; height: 8px; border-radius: 50%; background: var(--warn); flex: none; }
  .dot.busy { background: var(--accent); animation: pulse 1s ease-in-out infinite; }
  @keyframes pulse { 50% { opacity: 0.3; } }

  nav button.discord {
    display: flex; flex-direction: column; gap: 3px; margin: 8px 0 6px; padding: 9px 10px; border-radius: 10px;
    background: linear-gradient(140deg, #5865f2, #4752c4); border-color: #5865f2; color: #fff;
  }
  nav button.discord:hover:not(:disabled) { border-color: #8891ff; }
  .discord-title { display: flex; align-items: center; justify-content: center; gap: 7px; font-weight: 700; }
  .discord-sub { font-size: 11px; line-height: 1.3; opacity: 0.88; text-align: center; }
  .foot { display: flex; flex-direction: column; gap: 6px; padding: 8px 4px 0; border-top: 1px solid var(--line); }
  .state { display: flex; align-items: center; gap: 7px; font-size: 12px; padding: 3px 6px; border-radius: 6px; }
  .sdot { width: 8px; height: 8px; border-radius: 50%; flex: none; background: currentColor; box-shadow: 0 0 6px currentColor; }
  .state.ok { color: var(--ok); }
  .state.warn { color: var(--warn); }
  .state.err { color: var(--danger); }
  nav button.state:hover:not(:disabled) { background: var(--panel-2); border-color: transparent; }
  .linkish { cursor: pointer; }
  .version { display: flex; align-items: center; justify-content: space-between; gap: 6px; padding: 0 6px; font-size: 12px; color: var(--muted); }
  nav .version button { font-size: 12px; padding: 2px 4px; color: var(--accent); }
  nav .version button:hover:not(:disabled) { text-decoration: underline; border-color: transparent; }
  .update-banner {
    display: flex; gap: 10px; align-items: flex-start; padding: 10px 16px; background: var(--panel-2);
    border-bottom: 1px solid var(--accent);
  }
  .update-banner .notes { font-size: 12px; white-space: pre-wrap; max-height: 90px; overflow: auto; margin-top: 4px; }
  .bar { height: 6px; background: var(--line); border-radius: 3px; margin-top: 6px; overflow: hidden; }
  .bar div { height: 100%; background: var(--accent); }
  main { flex: 1; min-width: 0; overflow: hidden; display: flex; flex-direction: column; }
  .toast {
    position: fixed; bottom: 18px; right: 18px; z-index: 2000; max-width: 520px; padding: 10px 14px; border-radius: var(--radius);
    background: var(--panel-2); border: 1px solid var(--line); box-shadow: 0 6px 24px #0008; white-space: pre-wrap;
  }
  .toast.error { border-color: var(--danger); }
  .toast.ok { border-color: var(--ok); }
</style>
