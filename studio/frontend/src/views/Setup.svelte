<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { App, EventsOn, EventsOff, errText, ask } from '../lib/api';
  import DebugLog from './DebugLog.svelte';

  let { notify, onchange }: { notify: (t: string, k?: string) => void; onchange: () => void } = $props();

  let st = $state<any>(null);
  let busy = $state(false);
  let log = $state<string[]>([]);
  let backup = $state('');
  let check = $state<any>(null);
  let forge = $state<any>(null);
  let forgeBusy = $state(false);
  let forgeProg = $state<any>(null);

  const ICON: Record<string, string> = { ok: '✓', missing: '✗', outdated: '!', conflict: '!', blocked: '⏸', info: 'i' };
  const needsWork = $derived(st?.found && !st?.ready);

  async function refresh() {
    try { st = await App.SetupState(); } catch (e) { notify(errText(e), 'error'); }
    try { forge = await App.ForgeStatus(); } catch { forge = null; }
    onchange();
  }

  async function locate() { await App.LocateGame(); await refresh(); }
  async function browse() {
    try { await App.BrowseGameFolder(); } catch (e) { notify(errText(e), 'error'); }
    await refresh();
  }

  async function repair() {
    busy = true; log = []; backup = '';
    try {
      backup = await App.SetupRepair();
      notify('Setup finished — launch the game once to finish.', 'ok');
    } catch (e) { notify(errText(e), 'error'); }
    busy = false;
    await refresh();
  }

  async function uninstall(all: boolean) {
    const what = all ? 'the mod AND BepInEx (all mods)' : 'the TCG Custom Cards mod and its installed sets';
    if (!(await ask(`Remove ${what} from the game? Files are moved to a backup folder in the game directory; saves are not touched.`))) return;
    busy = true; log = []; backup = '';
    try { backup = await App.SetupUninstall(all); notify('Removed.', 'ok'); } catch (e) { notify(errText(e), 'error'); }
    busy = false;
    await refresh();
  }

  async function installForge() {
    if (!forge?.installed && !(await ask(`Download Forge ${forge?.pinned} and Java (about ${forge?.sizeMB} MB) into the game folder for MTG mode?`))) return;
    forgeBusy = true; forgeProg = null;
    try { notify(await App.InstallForge(), 'ok'); } catch (e) { notify(errText(e), 'error'); }
    forgeBusy = false; forgeProg = null;
    await refresh();
  }

  async function removeForge() {
    if (!(await ask('Remove Forge, its Java and the decks exported to it from the game folder?'))) return;
    try { await App.RemoveForge(); notify('MTG mode removed.', 'ok'); } catch (e) { notify(errText(e), 'error'); }
    await refresh();
  }

  async function launch() { await App.LaunchGame(); notify('Starting the game through Steam…'); }
  async function verify() { check = await App.CheckModLoaded(); }

  onMount(() => {
    EventsOn('setup:progress', (line: string) => (log = [...log, line]));
    EventsOn('forge:progress', (p: any) => (forgeProg = p));
    refresh();
  });
  onDestroy(() => { EventsOff('setup:progress'); EventsOff('forge:progress'); });
</script>

<div class="page">
  <header class="row"><h2 class="grow">Setup</h2>
    <button onclick={refresh} disabled={busy}>Re-check</button>
  </header>
  <p class="muted intro">Installs everything needed to play with custom cards: the BepInEx mod loader, the in-game settings menu (F1)
    and the TCG Custom Cards mod. Safe to run any time — it only changes what's missing or out of date, keeps your other mods
    and settings, and backs up anything it replaces.</p>

  {#if st}
    {#if !st.found}
      <section>
        <h3>Find the game</h3>
        <p>TCG Card Shop Simulator wasn't found automatically.</p>
        <div class="row"><button class="primary" onclick={locate}>Search Steam libraries</button><button onclick={browse}>Choose folder…</button></div>
      </section>
    {:else}
      <section>
        <div class="row"><span class="muted small grow">{st.gameDir}</span>
          {#if st.version}<span class="muted small">installer {st.version}</span>{/if}</div>
        <ul class="items">
          {#each st.items as it}
            <li class={it.state}>
              <span class="icon">{ICON[it.state] ?? '·'}</span>
              <div><b>{it.title}</b><div class="small">{it.message}</div></div>
            </li>
          {/each}
        </ul>
        {#if !st.payload}
          <p class="err small">This build of TCG Studio doesn't include the installer files (build it with tools\package.ps1).</p>
        {/if}
        <div class="row">
          <button class="primary big" onclick={repair} disabled={busy || !st.payload || st.running || !needsWork}>
            {busy ? 'Working…' : needsWork ? 'Install / Repair everything' : 'Everything is installed'}
          </button>
          {#if st.running}<span class="warn small">Close the game first, then Re-check.</span>{/if}
        </div>
      </section>

      {#if log.length || backup}
        <section>
          <h3>What happened</h3>
          <pre class="log">{log.join('\n')}</pre>
          {#if backup}<div class="row"><span class="small grow">Backup: {backup}</span><button class="small" onclick={() => App.OpenFolder(backup)}>Open backup folder</button></div>{/if}
        </section>
      {/if}

      {#if forge}
        <section>
          <h3>MTG mode (optional)</h3>
          <p class="small muted">Play real Magic: The Gathering against the AI with the cards in your in-game decks. Uses
            <b>Forge</b> (open-source MTG rules engine, GPL-3.0) with its own Java — nothing else on your PC is changed.
            In game, sit at a play table with a deck that contains MTG cards and pick <b>Magic (Forge)</b>.</p>
          <ul class="items">
            <li class={forge.installed ? (forge.outdated ? 'outdated' : 'ok') : 'info'}>
              <span class="icon">{forge.installed ? (forge.outdated ? '!' : '✓') : 'i'}</span>
              <div><b>Forge {forge.installed ? forge.forgeVersion : ''}</b>
                <div class="small">{forge.installed
                  ? (forge.outdated ? `Update available (${forge.pinned}).` : `Installed with Java ${forge.javaVersion} in ${forge.dir}`)
                  : `Not installed (about ${forge.sizeMB} MB download).`}</div></div>
            </li>
          </ul>
          {#if forgeBusy && forgeProg}
            <div class="small">{forgeProg.message}</div>
            {#if forgeProg.total > 0}<progress max={forgeProg.total} value={forgeProg.done}></progress>{/if}
          {/if}
          <div class="row">
            <button class="primary" onclick={installForge} disabled={forgeBusy || !st.found}>
              {forgeBusy ? 'Installing…' : forge.installed ? (forge.outdated ? 'Update Forge' : 'Reinstall Forge') : 'Install MTG mode'}
            </button>
            {#if forgeBusy}<button onclick={() => App.CancelForgeInstall()}>Cancel</button>{/if}
            {#if forge.installed && !forgeBusy}
              <button onclick={() => App.OpenFolder(forge.dir)}>Open folder</button>
              <button class="danger small" onclick={removeForge}>Remove</button>
            {/if}
          </div>
        </section>
      {/if}

      <section>
        <h3>Next steps</h3>
        <ol class="small steps">
          <li>Use <b>Import</b> to download card sets (Magic, Pokémon, Yu-Gi-Oh!, One Piece, Star Wars: Unlimited, Lorcana, Flesh and Blood, Union Arena), then <b>Install</b> them from <b>Sets</b>.</li>
          <li>Restart the game — your sets are in the shop and binder. Press <b>F1</b> in game for mod settings.</li>
        </ol>
        <div class="row">
          <button onclick={launch} disabled={!st.ready}>Launch game</button>
          <button onclick={verify}>Check it worked</button>
        </div>
        {#if check}
          {#if !check.logFound}
            <p class="small warn">No BepInEx log yet — launch the game once.</p>
          {:else if check.loaded}
            <p class="small ok">✓ Last launch ({check.when}): {check.summary}</p>
          {:else}
            <p class="small err">The last launch ({check.when}) didn't load the mod.</p>
          {/if}
          {#if check.errors?.length}<pre class="log">{check.errors.join('\n')}</pre>{/if}
        {/if}
      </section>

      <DebugLog {notify} />

      <section class="danger-zone">
        <h3>Remove</h3>
        <div class="row">
          <button class="danger small" onclick={() => uninstall(false)} disabled={busy || st.running}>Remove the mod</button>
          <button class="danger small" onclick={() => uninstall(true)} disabled={busy || st.running}>Remove mod + BepInEx</button>
        </div>
      </section>
    {/if}
  {:else}
    <p class="muted">Checking…</p>
  {/if}
</div>

<style>
  .page { padding: 20px 24px; overflow: auto; height: 100%; display: flex; flex-direction: column; gap: 14px; }
  .intro { max-width: 820px; margin: 0; }
  section { background: var(--panel); border: 1px solid var(--line); border-radius: var(--radius); padding: 12px; display: flex; flex-direction: column; gap: 10px; }
  .items { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 6px; }
  .items li { display: flex; gap: 10px; align-items: flex-start; padding: 8px; border-radius: 6px; background: var(--panel-2); }
  .icon { width: 22px; height: 22px; border-radius: 50%; display: inline-flex; align-items: center; justify-content: center; font-weight: 700; flex-shrink: 0; background: var(--line); }
  li.ok .icon { background: var(--ok); color: #fff; }
  li.missing .icon, li.blocked .icon { background: var(--danger); color: #fff; }
  li.outdated .icon, li.conflict .icon { background: #d9a400; color: #000; }
  li.info .icon { background: var(--line); }
  .big { font-size: 15px; padding: 10px 18px; }
  .log { background: var(--bg); padding: 8px; border-radius: 6px; max-height: 220px; overflow: auto; margin: 0; font-size: 12px; white-space: pre-wrap; }
  .steps { margin: 0; padding-left: 20px; display: flex; flex-direction: column; gap: 4px; }
  .small { font-size: 12px; }
  .ok { color: var(--ok); }
  .err { color: var(--danger); }
  .warn { color: #d9a400; }
  .danger-zone { border-color: #5a2a2a; }
  progress { width: 100%; height: 10px; }
</style>
