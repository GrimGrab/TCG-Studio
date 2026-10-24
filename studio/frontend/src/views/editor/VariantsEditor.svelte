<script lang="ts">
  // The card versions the set has (set.json "variants"): 6 borders × normal/foil. Versions that are off are never made by
  // packs or trade customers and are left out of the binder and Check Price — unless the player already owns one. All on =
  // the field is left out (the game's 12 versions).
  import { BORDERS, BORDER_LABELS } from '../../lib/api';

  let { project }: { project: any } = $props();

  const key = (b: string, foil: boolean) => (foil ? `${b}_foil` : b);
  const all = () => BORDERS.flatMap((b: string) => [key(b, false), key(b, true)]);

  /** The versions that are on (no list = all 12). */
  const on = $derived<Set<string>>(new Set((project.set.variants?.length ? project.set.variants : all()).map((v: string) => v.toLowerCase())));
  const count = $derived(on.size);

  function write(next: Set<string>) {
    // At least one version must stay on; all 12 = leave the field out.
    if (next.size === 0) return;
    const list = all().filter((v) => next.has(v.toLowerCase()));
    project.set.variants = list.length === 12 ? undefined : list;
  }

  function toggle(b: string, foil: boolean) {
    const next = new Set(on);
    const k = key(b, foil).toLowerCase();
    if (next.has(k)) next.delete(k); else next.add(k);
    write(next);
  }

  const preset = (keys: string[]) => write(new Set(keys.map((k) => k.toLowerCase())));
</script>

<section class="block">
  <div class="bhead"><h2>Card versions</h2><span class="muted small">{count} of 12 on</span><span class="grow"></span>
    <button class="small" onclick={() => preset(all())} disabled={count === 12}>All on</button></div>
  <p class="muted small">Which of the game's 12 versions (border × foil) this set's cards come in. Versions that are off never come
    out of packs or trade customers and don't take binder or Check Price slots — cards a player already owns in them stay
    visible. The binder's total counts only the versions that are on.</p>
  <div class="vgrid">
    <span></span><span class="colhead">Normal</span><span class="colhead">Foil</span>
    {#each BORDERS as b}
      <span class="rowhead">{BORDER_LABELS[b]}</span>
      {#each [false, true] as foil}
        {@const isOn = on.has(key(b, foil).toLowerCase())}
        <button class="v" class:on={isOn} class:foil disabled={count === 1 && isOn} onclick={() => toggle(b, foil)}
          aria-pressed={isOn} aria-label="{BORDER_LABELS[b]} {foil ? 'foil' : 'normal'}">{isOn ? '✓ On' : 'Off'}</button>
      {/each}
    {/each}
  </div>
</section>

<style>
  .vgrid { display: grid; grid-template-columns: minmax(90px, auto) 1fr 1fr; gap: 4px 6px; align-items: center; }
  .colhead { font-size: 12px; color: var(--muted); text-align: center; }
  .rowhead { font-size: 13px; }
  .v { padding: 4px 0; font-size: 12px; color: var(--muted); background: var(--bg); }
  .v.on { color: var(--text); background: #1c2740; border-color: var(--accent-2); }
  .v.on.foil { background: linear-gradient(120deg, #1c2740, #2e2148 55%, #1d3a3a); }
</style>
