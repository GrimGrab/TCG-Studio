<script lang="ts">
  import { onMount } from 'svelte';
  import { App, EventsOn, errText } from './lib/api';
  import Projects from './views/Projects.svelte';
  import Import from './views/Import.svelte';
  import Settings from './views/Settings.svelte';
  import Editor from './views/Editor.svelte';
  import Gamify from './views/Gamify.svelte';
  import Setup from './views/Setup.svelte';
  import Accessories from './views/Accessories.svelte';
  import Furniture from './views/Furniture.svelte';
  import ModSettings from './views/ModSettings.svelte';
  import Setups from './views/Setups.svelte';

  type View = 'setup' | 'setups' | 'projects' | 'import' | 'accessories' | 'furniture' | 'gamify' | 'modsettings' | 'settings' | 'editor';
  let view = $state<View>('projects');
  let openId = $state('');
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
  let setupKey = $state(0);     // bumped after a setup switch so every view reloads
  let setupList = $state<any[]>([]);
  let setupMenu = $state(false);
  let setupAction = $state<any>(null); // one-shot request for the Setups page: { create: true } or { switchTo: id }
  let toastTimer: number | undefined;

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
  }

  function toggleSetupMenu() {
    setupMenu = !setupMenu;
    if (setupMenu) refreshSetup();
  }

  function openSetups(action: any = null) {
    setupMenu = false;
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

  // Open on Setup when something isn't installed yet (e.g. a friend's first run) or right after an update
  // (the mod in the game then needs updating too).
  onMount(async () => {
    EventsOn('update:progress', (f: number) => (updateProgress = f));
    version = await App.AppVersion();
    await refreshStatus();
    refreshSetup();
    if (version.updatedTo) {
      notify(`Updated to v${version.updatedTo}.` + (setupReady ? '' : ' Update the mod in the game on the Setup screen.'), 'ok');
    }
    if (!setupReady) view = 'setup';
    checkUpdates(false);
    // Once per Studio version: add what this version needs to the sets already installed in the game (no clicks needed).
    App.SyncInstalledSets()
      .then((msg: string) => msg && notify(msg, 'ok'))
      .catch((e: unknown) => notify(errText(e), 'error'));
  });
</script>

<div class="shell">
  <nav>
    <div class="brand">TCG <span>Studio</span></div>
    {#if version}
      <div class="setup-pick">
        <button class="setup-current" class:open={setupMenu} title="Switch setup or make a new one" onclick={toggleSetupMenu}>
          <span class="name" class:muted={!setupName}>{setupName || 'Setups unavailable'}</span><span class="caret">▾</span>
        </button>
        {#if setupMenu}
          <div class="menu-catcher" role="presentation" onclick={() => (setupMenu = false)}></div>
          <div class="menu" role="menu">
            {#each setupList as s (s.id)}
              <button role="menuitem" class:current={s.active} disabled={s.active} onclick={() => openSetups({ switchTo: s.id })}>
                <span class="check">{s.active ? '✓' : ''}</span><span class="name">{s.name}</span>
              </button>
            {/each}
            <div class="sep"></div>
            <button role="menuitem" onclick={() => openSetups({ create: true })}><span class="check">+</span>New setup…</button>
            <button role="menuitem" onclick={() => openSetups()}><span class="check"></span>Manage setups…</button>
          </div>
        {/if}
      </div>
    {/if}
    <button class:active={view === 'setup'} onclick={() => (view = 'setup')}>Setup{#if !setupReady}<span class="dot" title="Something needs installing"></span>{/if}</button>
    <button class:active={view === 'projects' || view === 'editor'} onclick={() => (view = 'projects')}>Sets</button>
    <button class:active={view === 'import'} onclick={() => (view = 'import')}>Import from Scryfall</button>
    <button class:active={view === 'accessories'} onclick={() => (view = 'accessories')}>Accessories</button>
    <button class:active={view === 'furniture'} onclick={() => (view = 'furniture')}>Furniture</button>
    <button class:active={view === 'gamify'} onclick={() => (view = 'gamify')}>Gamify</button>
    <button class:active={view === 'modsettings'} onclick={() => (view = 'modsettings')}>Mod settings</button>
    <button class:active={view === 'settings'} onclick={() => (view = 'settings')}>Settings</button>
    <div class="grow"></div>
    {#if gameStatus}
      <div class="status">
        {#if !gameStatus.found}
          <span class="badge err">Game not found</span>
        {:else if !setupReady}
          <button class="badge warn linkish" onclick={() => (view = 'setup')}>Setup needed</button>
        {:else}
          <span class="badge ok">Game + mod ready</span>
        {/if}
      </div>
    {/if}
    {#if version}
      <div class="version">
        <span class="muted">v{version.version}</span>
        <button class="small linkish" onclick={() => checkUpdates(true)} disabled={checking || updating}>{checking ? 'Checking…' : 'Check for updates'}</button>
      </div>
    {/if}
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
    {#key setupKey}
    {#if view === 'setup'}
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
    {:else if view === 'gamify'}
      <Gamify {notify} />
    {:else if view === 'modsettings'}
      <ModSettings {notify} />
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

<style>
  .shell { display: flex; height: 100vh; }
  nav {
    width: 200px; flex-shrink: 0; background: var(--panel); border-right: 1px solid var(--line);
    display: flex; flex-direction: column; gap: 4px; padding: 14px 10px;
  }
  .brand { font-size: 20px; font-weight: 700; padding: 4px 8px 14px; }
  .brand span { color: var(--accent); }
  .setup-pick { position: relative; margin: -6px 0 10px; }
  nav .setup-current {
    width: 100%; display: flex; align-items: center; gap: 6px; background: var(--panel-2); border-color: var(--line);
    font-weight: 600;
  }
  nav .setup-current:hover, nav .setup-current.open { border-color: var(--accent); }
  .setup-current .name, .menu .name { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .caret { color: var(--muted); font-size: 11px; }
  .menu-catcher { position: fixed; inset: 0; z-index: 49; }
  .menu {
    position: absolute; left: 0; right: -60px; top: calc(100% + 4px); z-index: 50; background: var(--panel-2);
    border: 1px solid var(--line); border-radius: var(--radius); box-shadow: 0 6px 24px #0008; padding: 4px;
    display: flex; flex-direction: column; max-height: 60vh; overflow: auto;
  }
  nav .menu button { display: flex; align-items: center; gap: 6px; border-color: transparent; padding: 5px 8px; }
  nav .menu button:hover:not(:disabled) { background: var(--panel); border-color: transparent; }
  nav .menu button.current { opacity: 1; cursor: default; color: var(--ok); }
  .menu .check { width: 14px; text-align: center; color: var(--muted); }
  .menu button.current .check { color: var(--ok); }
  .menu .sep { height: 1px; background: var(--line); margin: 4px 2px; }
  nav button { text-align: left; background: transparent; border-color: transparent; }
  nav button.active { background: var(--panel-2); border-color: var(--line); }
  .status { padding: 8px; }
  .dot { display: inline-block; width: 8px; height: 8px; border-radius: 50%; background: #d9a400; margin-left: 8px; vertical-align: middle; }
  .linkish { cursor: pointer; }
  .version { padding: 4px 8px; display: flex; flex-direction: column; gap: 4px; font-size: 12px; }
  .version button { font-size: 12px; padding: 4px 8px; }
  .update-banner {
    display: flex; gap: 10px; align-items: flex-start; padding: 10px 16px; background: var(--panel-2);
    border-bottom: 1px solid var(--accent);
  }
  .update-banner .notes { font-size: 12px; white-space: pre-wrap; max-height: 90px; overflow: auto; margin-top: 4px; }
  .bar { height: 6px; background: var(--line); border-radius: 3px; margin-top: 6px; overflow: hidden; }
  .bar div { height: 100%; background: var(--accent); }
  main { flex: 1; min-width: 0; overflow: hidden; display: flex; flex-direction: column; }
  .toast {
    position: fixed; bottom: 18px; right: 18px; max-width: 520px; padding: 10px 14px; border-radius: var(--radius);
    background: var(--panel-2); border: 1px solid var(--line); box-shadow: 0 6px 24px #0008; white-space: pre-wrap;
  }
  .toast.error { border-color: var(--danger); }
  .toast.ok { border-color: var(--ok); }
</style>
