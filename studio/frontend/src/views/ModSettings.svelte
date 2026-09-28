<script lang="ts">
  // The mod's settings (same as the in-game F1 menu), read from and written to <game>\BepInEx\config\tcgcustomcards.cfg.
  // Rendered from the file's own descriptions, so new mod settings show up without studio changes.
  import { onMount } from 'svelte';
  import { App, errText, ask } from '../lib/api';

  let { notify }: { notify: (t: string, k?: string) => void } = $props();

  let sections = $state<any[]>([]);
  let gameRunning = $state(false);
  let error = $state('');
  let loading = $state(true);

  async function load() {
    try {
      const v = await App.ModSettings();
      sections = v.sections;
      gameRunning = v.gameRunning;
      error = '';
    } catch (e) { error = errText(e); }
    loading = false;
  }

  async function set(e: any, value: string) {
    if (value === e.value) return;
    try {
      const r = await App.SetModSetting(e.section, e.key, value);
      e.value = r.value;
    } catch (err) {
      notify(errText(err), 'error');
      await load(); // put the control back to the stored value
    }
  }

  async function restore(section: string) {
    const what = section ? `every setting in "${section}"` : 'ALL mod settings';
    if (!(await ask(`Restore ${what} to the mod's defaults?`))) return;
    try {
      const n = await App.RestoreModDefaults(section);
      await load();
      notify(n ? `Restored ${n} setting${n === 1 ? '' : 's'} to default.` : 'Everything was already at its default.', 'ok');
    } catch (e) { notify(errText(e), 'error'); }
  }

  /** "ShowVanillaCards" → "Show vanilla cards" */
  const label = (k: string) => {
    const s = k.replace(/([a-z0-9])([A-Z])/g, '$1 $2').replace(/([A-Z])([A-Z][a-z])/g, '$1 $2');
    return s.charAt(0) + s.slice(1).toLowerCase();
  };
  const isNumber = (e: any) => e.type === 'Single' || e.type === 'Double' || e.type === 'Int32';
  const step = (e: any) => (e.type === 'Int32' ? 1 : e.max !== undefined && e.min !== undefined && e.max - e.min <= 2 ? 0.01 : 0.05);
  const round = (v: string) => { const n = +v; return isNaN(n) ? v : String(Math.round(n * 1000) / 1000); };

  // BepInEx colours are RRGGBBAA hex.
  const rgb = (v: string) => '#' + (v || '000000').slice(0, 6).toLowerCase();
  const alpha = (v: string) => (v?.length === 8 ? parseInt(v.slice(6, 8), 16) : 255);
  const hex2 = (n: number) => Math.max(0, Math.min(255, Math.round(n))).toString(16).padStart(2, '0').toUpperCase();
  const withRgb = (v: string, c: string) => c.slice(1).toUpperCase() + hex2(alpha(v));
  const withAlpha = (v: string, pct: number) => (v || '000000').slice(0, 6).toUpperCase() + hex2((pct / 100) * 255);

  const SECTION_ORDER = ['Content', 'Visuals', 'Foil', 'Debug'];
  let ordered = $derived([...sections].sort((a, b) => (SECTION_ORDER.indexOf(a.name) + 1 || 99) - (SECTION_ORDER.indexOf(b.name) + 1 || 99)));

  onMount(load);
</script>

<div class="page">
  <header class="row">
    <h2 class="grow">Mod settings</h2>
    <button onclick={load}>Reload</button>
    <button class="danger" onclick={() => restore('')} disabled={!sections.length}>Restore all defaults</button>
  </header>
  <p class="muted intro">
    The same settings as the in-game <b>F1</b> menu (TCG Custom Cards), saved straight into the mod's config file. Changes save as you make
    them. {gameRunning ? 'The game is running: settings marked "Applies live" update right away, the rest on the next start.' : 'They take effect the next time the game starts.'}
  </p>

  {#if loading}
    <p class="muted">Loading…</p>
  {:else if error}
    <p class="warn">{error}</p>
  {:else}
    {#each ordered as s (s.name)}
      <section>
        <div class="row">
          <h3 class="grow">{s.name}</h3>
          <button class="small" onclick={() => restore(s.name)}>Restore section defaults</button>
        </div>
        {#each s.entries as e (e.key)}
          <div class="entry">
            <div class="head row">
              <span class="grow name">{label(e.key)}
                {#if /applies live/i.test(e.description)}<span class="badge ok" title="Updates while the game is running">live</span>{/if}
              </span>
              <span class="control">
                {#if e.type === 'Boolean'}
                  <label class="switch"><input type="checkbox" checked={e.value === 'true'} onchange={(ev) => set(e, ev.currentTarget.checked ? 'true' : 'false')} /> {e.value === 'true' ? 'On' : 'Off'}</label>
                {:else if e.options?.length}
                  <select value={e.value} onchange={(ev) => set(e, ev.currentTarget.value)}>
                    {#each e.options as o}<option>{o}</option>{/each}
                  </select>
                {:else if isNumber(e)}
                  {#if e.min !== undefined && e.max !== undefined}
                    <input type="range" min={e.min} max={e.max} step={step(e)} value={e.value} oninput={(ev) => (e.value = ev.currentTarget.value)} onchange={(ev) => set({ ...e, value: '' }, round(ev.currentTarget.value))} />
                  {/if}
                  <input class="num" type="number" min={e.min} max={e.max} step={step(e)} value={round(e.value)} onchange={(ev) => set(e, round(ev.currentTarget.value))} />
                {:else if e.type === 'Color'}
                  <input type="color" value={rgb(e.value)} onchange={(ev) => set(e, withRgb(e.value, ev.currentTarget.value))} />
                  <label class="small muted">opacity <input class="num" type="number" min="0" max="100" step="5" value={Math.round((alpha(e.value) / 255) * 100)} onchange={(ev) => set(e, withAlpha(e.value, +ev.currentTarget.value))} />%</label>
                {:else}
                  <input value={e.value} onchange={(ev) => set(e, ev.currentTarget.value)} />
                {/if}
                <button class="tiny" title="Restore default ({e.default})" disabled={e.value === e.default || round(e.value) === round(e.default)} onclick={() => set(e, e.default)}>↺</button>
              </span>
            </div>
            {#if e.description}<div class="desc muted small">{e.description}</div>{/if}
          </div>
        {/each}
      </section>
    {/each}
  {/if}
</div>

<style>
  .page { padding: 20px 24px; overflow: auto; height: 100%; display: flex; flex-direction: column; gap: 14px; }
  .intro { max-width: 900px; margin: 0; }
  section { background: var(--panel); border: 1px solid var(--line); border-radius: var(--radius); padding: 12px 14px; display: flex; flex-direction: column; gap: 4px; max-width: 980px; }
  .entry { padding: 8px 0; border-top: 1px solid var(--line); }
  .entry:first-of-type { border-top: none; }
  .name { font-weight: 600; }
  .control { display: flex; align-items: center; gap: 8px; }
  .control input[type='range'] { width: 180px; }
  .num { width: 80px; }
  .control input[type='color'] { width: 44px; height: 28px; padding: 2px; }
  .desc { margin-top: 2px; max-width: 760px; }
  .badge { font-size: 11px; margin-left: 6px; }
  button.tiny { padding: 2px 8px; }
  .small { font-size: 12px; }
  .warn { color: #d9a400; }
</style>
