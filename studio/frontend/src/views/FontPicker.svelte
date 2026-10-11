<script lang="ts">
  // Font choice of the Shop look page (shop name, door sign text): the game's own default, the game's fonts, fonts installed on
  // Windows or a font file. `onpick` gets source "" (default), "game", "windows" or "file" (+ the App.ShopSignFonts id).
  let { fonts, current = '', defaultLabel, onpick }: {
    fonts: { source: string; id: string; name: string }[]; // App.ShopSignFonts
    current?: string; // display name of the chosen font, '' = the default
    defaultLabel: string;
    onpick: (source: string, id: string) => void;
  } = $props();

  function pick(value: string) {
    if (value === 'current') return;
    const i = value.indexOf('|');
    onpick(value.slice(0, i), value.slice(i + 1));
  }
</script>

<label class="field">Font
  <select value={current ? 'current' : '|'} onchange={(ev) => pick(ev.currentTarget.value)}>
    {#if current}<option value="current">{current}</option>{/if}
    <option value="|">{defaultLabel}</option>
    {#each [['game', 'From the game'], ['windows', 'Installed on Windows']] as [source, title] (source)}
      {#if fonts.some((f) => f.source === source)}
        <optgroup label={title}>
          {#each fonts.filter((f) => f.source === source) as f (f.id)}<option value={source + '|' + f.id}>{f.name}</option>{/each}
        </optgroup>
      {/if}
    {/each}
  </select>
</label>
<button onclick={() => onpick('file', '')}>Font file…</button>
