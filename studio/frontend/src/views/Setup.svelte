<script lang="ts">
  import { onMount } from 'svelte';
  import { Ev } from '../lib/events';
  import { App, EventsOn, errText, ask } from '../lib/api';
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
    const offs = [
      EventsOn(Ev.SetupProgress, (line: string) => (log = [...log, line])),
      EventsOn(Ev.ForgeProgress, (p: any) => (forgeProg = p)),
    ];
    refresh();
    return () => offs.forEach((off) => off());
  });
</script>

<div class="page">
  <header class="head">
    <div><h2>Game &amp; mod</h2>
      <div class="muted small">BepInEx, the in-game settings menu (F1) and the TCG Custom Cards mod — only what's missing or out of date is
        changed, your other mods and settings stay, and anything replaced is backed up.</div></div>
    <span class="grow"></span>
    <button onclick={refresh} disabled={busy}>Re-check</button>
  </header>

  <div class="main">
  {#if st}
    {#if !st.found}
      <div class="banner bad">
        <span class="bigicon">?</span>
        <div class="grow"><h1>Find the game</h1><p class="muted">TCG Card Shop Simulator wasn't found automatically.</p></div>
        <button class="primary big" onclick={locate}>Search Steam libraries</button><button class="big" onclick={browse}>Choose folder…</button>
      </div>
    {:else}
      {@const problems = st.items.filter((it: any) => it.state !== 'ok' && it.state !== 'info').length}
      <!-- One line that says whether you can play, with the one button that fixes it. -->
      <div class="banner" class:good={!needsWork} class:bad={needsWork}>
        <span class="bigicon">{needsWork ? '!' : '✓'}</span>
        <div class="grow">
          <h1>{needsWork ? `${problems || 'Some'} thing${problems === 1 ? '' : 's'} need${problems === 1 ? 's' : ''} attention` : 'Everything is installed'}</h1>
          <p class="muted path" title={st.gameDir}>{st.gameDir}{st.version ? ` · installer ${st.version}` : ''}</p>
          {#if st.running}<p class="warn small">The game is running — close it first, then Re-check.</p>{/if}
          {#if !st.payload}<p class="err small">This build of TCG Studio doesn't include the installer files (build it with tools\package.ps1).</p>{/if}
        </div>
        <button class="primary big" onclick={repair} disabled={busy || !st.payload || st.running || !needsWork}>
          {busy ? 'Working…' : needsWork ? 'Install / Repair everything' : 'Up to date'}
        </button>
      </div>

      <div class="cards">
        <section class="block">
          <div class="bhead"><h2>Components</h2></div>
          <ul class="items">
            {#each st.items as it}
              <li class={it.state}>
                <span class="icon">{ICON[it.state] ?? '·'}</span>
                <div><b>{it.title}</b><div class="small muted">{it.message}</div></div>
              </li>
            {/each}
          </ul>
        </section>

        <section class="block">
          <div class="bhead"><h2>Play</h2></div>
          <ol class="small steps">
            <li>Use <b>Import</b> to bring in card sets, then <b>Install</b> them on <b>Sets</b>.</li>
            <li>Restart the game — your sets are in the shop and binder. Press <b>F1</b> in game for mod settings.</li>
          </ol>
          <div class="row">
            <button class="primary" onclick={launch} disabled={!st.ready}>Launch game</button>
            <button onclick={verify} title="Reads the game's last log: did the mod load, and were there errors?">Check it worked</button>
          </div>
          {#if check}
            {#if !check.logFound}
              <div class="notice">No BepInEx log yet — launch the game once.</div>
            {:else if check.loaded}
              <div class="notice ok">✓ Last launch ({check.when}): {check.summary}</div>
            {:else}
              <div class="notice err">The last launch ({check.when}) didn't load the mod.</div>
            {/if}
            {#if check.errors?.length}<pre class="log">{check.errors.join('\n')}</pre>{/if}
          {/if}
        </section>

        {#if forge}
          <section class="block">
            <div class="bhead"><h2>MTG mode</h2><span class="muted small">optional</span></div>
            <p class="small muted">Play real Magic: The Gathering against the AI with the cards in your in-game decks, using <b>Forge</b>
              (open-source rules engine, GPL-3.0) with its own Java — nothing else on your PC changes. In game, sit at a play table with a
              deck that has MTG cards and pick <b>Magic (Forge)</b>.</p>
            <ul class="items">
              <li class={forge.installed ? (forge.outdated ? 'outdated' : 'ok') : 'info'}>
                <span class="icon">{forge.installed ? (forge.outdated ? '!' : '✓') : 'i'}</span>
                <div><b>Forge {forge.installed ? forge.forgeVersion : ''}</b>
                  <div class="small muted">{forge.installed
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
                <button class="danger" onclick={removeForge}>Remove</button>
              {/if}
            </div>
          </section>
        {/if}

        {#if log.length || backup}
          <section class="block">
            <div class="bhead"><h2>What happened</h2></div>
            <pre class="log">{log.join('\n')}</pre>
            {#if backup}<div class="row"><span class="small grow">Backup: {backup}</span><button class="small" onclick={() => App.OpenFolder(backup)}>Open backup folder</button></div>{/if}
          </section>
        {/if}
      </div>

      <DebugLog {notify} />

      <section class="block danger-zone">
        <div class="bhead"><h2>Remove</h2><span class="muted small">Takes the mod out of the game (your setups and sets stay in TCG Studio).</span></div>
        <div class="row">
          <button class="danger" onclick={() => uninstall(false)} disabled={busy || st.running}>Remove the mod</button>
          <button class="danger" onclick={() => uninstall(true)} disabled={busy || st.running}>Remove mod + BepInEx</button>
        </div>
      </section>
    {/if}
  {:else}
    <div class="banner"><span class="bigicon">…</span><div class="grow"><h1>Checking the game…</h1></div></div>
  {/if}
  </div>
</div>

<style>
  .page { display: flex; flex-direction: column; height: 100%; overflow: hidden; }
  .head { display: flex; align-items: flex-start; gap: 16px; padding: 16px 24px 14px; border-bottom: 1px solid var(--line); }
  .head h2 { font-size: 20px; margin-bottom: 4px; }
  .head .small { max-width: 760px; }
  .main { flex: 1; min-height: 0; overflow: auto; padding: 18px 24px 28px; display: flex; flex-direction: column; gap: 14px; }
  .banner { display: flex; align-items: center; gap: 18px; padding: 18px 20px; border-radius: 12px; background: var(--panel); border: 1px solid var(--line); }
  .banner.good { border-color: #1f6b4b; background: linear-gradient(90deg, rgba(62, 207, 142, 0.12), var(--panel) 60%); }
  .banner.bad { border-color: #6b5320; background: linear-gradient(90deg, rgba(255, 176, 32, 0.12), var(--panel) 60%); }
  .banner h1 { font-size: 20px; }
  .banner p { margin: 4px 0 0; }
  .banner .path { font-family: Consolas, monospace; font-size: 12px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .banner .grow { min-width: 0; }
  .bigicon { width: 52px; height: 52px; flex: none; border-radius: 50%; display: grid; place-items: center; font-size: 26px; font-weight: 700; background: var(--line); }
  .good .bigicon { background: var(--ok); color: #0b2018; }
  .bad .bigicon { background: var(--warn); color: #2a1d00; }
  .cards { display: grid; grid-template-columns: repeat(auto-fit, minmax(380px, 1fr)); gap: 14px; align-items: start; }
  .items { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 6px; }
  .items li { display: flex; gap: 10px; align-items: flex-start; padding: 8px 10px; border-radius: 8px; background: var(--panel-2); }
  .icon { width: 22px; height: 22px; border-radius: 50%; display: inline-flex; align-items: center; justify-content: center; font-weight: 700; flex-shrink: 0; background: var(--line); }
  li.ok .icon { background: var(--ok); color: #fff; }
  li.missing .icon, li.blocked .icon { background: var(--danger); color: #fff; }
  li.outdated .icon, li.conflict .icon { background: #d9a400; color: #000; }
  li.info .icon { background: var(--line); }
  .big { font-size: 15px; padding: 10px 18px; }
  .log { background: var(--bg); padding: 8px; border-radius: 6px; max-height: 220px; overflow: auto; margin: 0; font-size: 12px; white-space: pre-wrap; }
  .steps { margin: 0; padding-left: 20px; display: flex; flex-direction: column; gap: 4px; }
  .danger-zone { border-color: #5a2a2a; }
  progress { width: 100%; height: 10px; }
</style>
