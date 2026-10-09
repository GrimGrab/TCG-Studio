<script lang="ts">
  // The card versions the set has (set.json "variants"): 6 borders × normal/foil. Versions that are off are never made by
  // packs or trade customers and are left out of the binder and Check Price — unless the player already owns one. All on =
  // the field is left out (the game's 12 versions).
  import { BORDERS } from '../../lib/api';

  let { project }: { project: any } = $props();

  const LABELS: Record<string, string> = { Base: 'Base', FirstEdition: 'First Edition', Silver: 'Silver', Gold: 'Gold', EX: 'EX', FullArt: 'Full Art' };
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

<section>
  <h3>Card versions <span class="muted small">({count} of 12)</span></h3>
  <p class="muted small">Which of the game's 12 versions (border × foil) this set's cards come in. Versions that are off never come
    out of packs or trade customers and don't take binder or Check Price slots — cards a player already owns in them stay
    visible. The binder's total counts only the versions that are on.</p>
  <button class="small" onclick={() => preset(all())} disabled={count === 12}>Select all</button>
  <table class="variants">
    <thead><tr><th></th><th>Normal</th><th>Foil</th></tr></thead>
    <tbody>
      {#each BORDERS as b}
        <tr>
          <td>{LABELS[b] ?? b}</td>
          {#each [false, true] as foil}
            <td><input type="checkbox" checked={on.has(key(b, foil).toLowerCase())} disabled={count === 1 && on.has(key(b, foil).toLowerCase())}
              onchange={() => toggle(b, foil)} aria-label="{LABELS[b] ?? b} {foil ? 'foil' : 'normal'}" /></td>
          {/each}
        </tr>
      {/each}
    </tbody>
  </table>
</section>

<style>
  .variants { border-collapse: collapse; margin-top: 4px; }
  .variants th, .variants td { padding: 3px 14px 3px 0; text-align: left; }
  .variants td:not(:first-child), .variants th:not(:first-child) { text-align: center; }
</style>
