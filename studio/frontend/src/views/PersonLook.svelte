<script lang="ts">
  // A person's Look and Outfit sections and the face editor: the same on the Customers and Employees pages. Faces are saved for the whole setup, so a face
  // made on one page can be given to anyone on either. Changes the person it's given, as part of the page's draft.
  import { App } from '../lib/api';
  import FaceEditor, { type Characters, type Face } from './FaceEditor.svelte';
  import { customerPortrait } from '../lib/urls';
  import { bodyOf, confirmDeleteFace, outfitOf, pictureUrl, previewClothes, shareFaceEdit, type ClothesTpl, type Look, type OutfitSlot, type Person, type Piece, type SavedFace } from '../lib/personLook';

  let { person, people, faces = $bindable(), chars, clothesTpl, looks, models, notify, onfresh, noPickLabel, noPickHint, pickHint }: {
    person: Person;
    people: Person[]; // everyone on this page: a saved face's edit reaches them
    faces: SavedFace[];
    chars: Characters | null;
    clothesTpl: ClothesTpl | null;
    looks: Look[];
    models: Record<string, number>;
    notify: (t: string, k?: string) => void;
    onfresh?: (p: Person) => void; // their portrait was made just now
    noPickLabel: string;
    noPickHint: string;
    pickHint: string;
  } = $props();

  const c = $derived(person);
  let faceOpen = $state(false);

  const copy = <T,>(v: T): T => JSON.parse(JSON.stringify(v));
  const facesOf = (c: Person) => faces.filter((f) => f.body === bodyOf(c));
  const savedFace = (c: Person) => (c.face && c.faceId ? faces.find((f) => f.id === c.faceId) : undefined);

  /** One of the game's characters (their face goes back to the character's own). */
  function pickCharacter(c: Person, i: number) {
    c.model = i;
    c.face = undefined; c.faceId = undefined; c.portrait = undefined;
  }
  /** One of your saved faces: they get a copy of it (and its character's body and clothes). */
  function pickFace(c: Person, f: SavedFace) {
    c.model = f.model;
    c.face = copy(f.face); c.faceId = f.id; c.portrait = f.portrait;
  }
  /** A new saved face, starting from what they look like now (a game character or one of your faces). */
  async function newFace(c: Person) {
    const id = await App.NewCustomerID();
    faces.push({ id, body: bodyOf(c), model: c.model, face: c.face ? copy(c.face) : (undefined as unknown as Face) });
    c.faceId = id;
    c.face = c.face ? copy(c.face) : undefined; // the editor starts from the character's own face when there is none
    faceOpen = true;
  }
  async function deleteFace(c: Person, f: SavedFace) {
    if (await confirmDeleteFace(f, people)) faces = faces.filter((x) => x.id !== f.id);
  }

  /** Done in the face editor: the saved face takes the edit (with a new portrait), and so does every regular using it. */
  async function closeFace(c: Person) {
    faceOpen = false;
    const f = faces.find((x) => x.id === c.faceId);
    if (!c.face || !chars || !f) return;
    f.face = copy(c.face);
    f.model = c.model;
    await shareFaceEdit(f, c, people, chars, clothesTpl, looks, notify, onfresh);
  }

  function maxModel(c: Person) {
    return (c.female ? models.female : models.male) - 1;
  }
  function setBody(c: Person, female: boolean) {
    if (!!c.female !== female) { c.outfit = undefined; c.face = undefined; c.faceId = undefined; c.portrait = undefined; } // the other body has its own pieces and faces
    c.female = female;
    if (c.model > maxModel(c)) c.model = -1;
  }

  // ---- outfit
  const slotsOf = (c: Person) => clothesTpl?.bodies[bodyOf(c)]?.slots ?? [];
  const wearable = (p: Piece) => p.name !== 'None';
  /** What the picked character wears in a slot (its preset), as "Hoodie, Gray"; '' = random character or unknown. */
  function presetWears(c: Person, slot: number): string {
    if (c.model < 0 || !clothesTpl) return '';
    const b = clothesTpl.bodies[bodyOf(c)];
    const pre = b?.presets.find((p) => p.name === (c.female ? 'Female' : 'Male') + c.model);
    const piece = b?.slots[slot]?.items.find((it) => it.name === pre?.apparel[slot]);
    if (!piece || piece.name === 'None' || !pre) return piece?.name === 'None' ? 'nothing' : '';
    const v = piece.variants?.[pre.variants[slot]]?.label;
    return v ? `${piece.display}, ${v}` : piece.display;
  }
  function setPiece(c: Person, slot: number, piece: string) {
    const rest = (c.outfit ?? []).filter((o) => o.slot !== slot);
    c.outfit = piece ? [...rest, { slot, piece }].sort((a, b) => a.slot - b.slot) : rest.length ? rest : undefined;
  }
  /** The look picker's value: "c:<id>" (one of the clothes), "v:<label>" (a game colour) or "" (the piece's first colours). */
  const lookValue = (o: OutfitSlot) => (o.clothing ? 'c:' + o.clothing : o.variant ? 'v:' + o.variant : '');
  function setLook(o: OutfitSlot, v: string) {
    o.clothing = v.startsWith('c:') ? v.slice(2) : undefined;
    o.variant = v.startsWith('v:') ? v.slice(2) : undefined;
  }
  /** The clothing slot a piece goes in (-1 = not this body's). */
  const slotOfPiece = (c: Person, piece: string) => slotsOf(c).findIndex((s) => s.items.some((it) => it.name === piece));
  const wearing = (c: Person, l: Look) => !!c.outfit?.some((o) => o.clothing === l.id);
  /** Puts one of your looks on (its piece's slot), or takes it off again. */
  function toggleLook(c: Person, l: Look) {
    const slot = slotOfPiece(c, l.piece);
    if (slot < 0) return;
    if (wearing(c, l)) { setPiece(c, slot, ''); return; }
    setPiece(c, slot, l.piece);
    const o = outfitOf(c, slot);
    if (o) o.clothing = l.id;
  }

</script>

          <section>
            <h3>Look</h3>
            <div class="row">
              <span class="lbl">Body</span>
              <div class="seg">
                <button class:on={!c.female} onclick={() => setBody(c, false)}>Male</button>
                <button class:on={!!c.female} onclick={() => setBody(c, true)}>Female</button>
              </div>
            </div>
            <div class="row">
              <span class="lbl">Character</span>
              <div class="seg">
                <button class:on={c.model < 0} onclick={() => { c.model = -1; c.face = undefined; c.faceId = undefined; c.portrait = undefined; }}>{noPickLabel}</button>
                <button class:on={c.model >= 0} onclick={() => (c.model = Math.max(0, c.model))}>Pick one</button>
              </div>
            </div>
            {#if c.model >= 0}
              <div class="faces">
                {#each Array.from({ length: maxModel(c) + 1 }, (_, i) => i) as i (i)}
                  <button class="face" class:on={c.model === i && !c.face} title="The game's character {i}" onclick={() => pickCharacter(c, i)}>
                    <img src={customerPortrait(!!c.female, i)} alt="" loading="lazy"
                      onerror={(e) => ((e.currentTarget as HTMLImageElement).style.visibility = 'hidden')} />
                    <span>{i}</span>
                  </button>
                {/each}
                {#each facesOf(c) as f (f.id)}
                  <div class="face own" class:on={!!c.face && c.faceId === f.id}>
                    <button class="pick" title="Your face" onclick={() => pickFace(c, f)}>
                      {#if pictureUrl(f.portrait)}<img src={pictureUrl(f.portrait)} alt="" />{:else}<span class="noimg">★</span>{/if}
                    </button>
                    <span>★</span>
                    <button class="walk" class:off={f.noWalkIns} title={f.noWalkIns ? 'Walk-ins never get this face: click to allow it' : 'Walk-ins can get this face unless it has been given to someone: click to stop that'}
                      onclick={() => (f.noWalkIns = !f.noWalkIns || undefined)}>🚶</button>
                    <button class="del" title="Delete this saved face" onclick={() => deleteFace(c, f)}>✕</button>
                  </div>
                {/each}
              </div>
            {/if}
            <p class="muted small">
              {c.model < 0 ? noPickHint : pickHint}
            </p>
            <div class="row">
              <span class="lbl">Face &amp; body</span>
              {#if c.model < 0}
                <span class="muted small">Pick a character first: a new face starts from it.</span>
              {:else if !chars}
                <span class="muted small">The game's characters haven't been read from the game files yet (Game &amp; mod page).</span>
              {:else}
                <button onclick={() => newFace(c)} title="A new face of your own, starting from the selected one">New face…</button>
                {#if savedFace(c)}
                  <button onclick={() => (faceOpen = true)} title="Changes this saved face for every regular who has it">Edit face…</button>
                {:else if c.face}
                  <span class="muted small">Their own copy of a deleted face: <b>New face</b> saves it again to edit it.</span>
                {:else}
                  <span class="muted small">The game's characters can't be edited: make a new face from one.</span>
                {/if}
              {/if}
            </div>
          </section>

          <section>
            <h3>Outfit</h3>
            {#if !clothesTpl}
              <p class="muted small">The game's clothes haven't been read from the game files yet (Game &amp; mod page).</p>
            {:else}
              {@const mine = looks.filter((l) => l.body === bodyOf(c) && slotOfPiece(c, l.piece) >= 0)}
              <div class="row">
                <span class="lbl">Your clothes</span>
                {#if mine.length}
                  <div class="chips">
                    {#each mine as l (l.id)}
                      <button class="chip" class:on={wearing(c, l)} onclick={() => toggleLook(c, l)}
                        title={wearing(c, l) ? 'Take it off' : 'Wear it'}><span class="swatch" style:background={(l.parts[0]?.tint ?? '#888').slice(0, 7)}></span>{l.name}</button>
                    {/each}
                  </div>
                {:else}
                  <span class="muted small">None for {c.female ? 'women' : 'men'} yet: make them on <b>Game look → Clothes</b> and save.</span>
                {/if}
              </div>
              {#each slotsOf(c) as s, slot}
                {@const o = outfitOf(c, slot)}
                {@const piece = o ? s.items.find((it) => it.name === o.piece) : undefined}
                <div class="row">
                  <span class="lbl">{s.label}</span>
                  <select value={o?.piece ?? ''} onchange={(e) => setPiece(c, slot, e.currentTarget.value)}>
                    <option value="">Their character's{presetWears(c, slot) ? ` (${presetWears(c, slot)})` : ''}</option>
                    {#each s.items as it}<option value={it.name}>{wearable(it) ? it.display : 'Nothing'}</option>{/each}
                  </select>
                  {#if o && piece && wearable(piece) && ((piece.variants?.length ?? 0) > 0 || looks.some((l) => l.body === bodyOf(c) && l.piece === piece.name))}
                    <select value={lookValue(o)} onchange={(e) => setLook(o, e.currentTarget.value)}>
                      {#each piece.variants ?? [] as v, i}<option value={i === 0 && !o.variant ? '' : 'v:' + v.label}>{v.label}</option>{/each}
                      {#each looks.filter((l) => l.body === bodyOf(c) && l.piece === piece.name) as l}<option value={'c:' + l.id}>★ {l.name}</option>{/each}
                    </select>
                  {/if}
                </div>
              {/each}
              <p class="muted small">Pieces you don't change stay as their character wears them. ★ = your own looks from <b>Game look → Clothes</b> (saved ones).
                Needs Personalities on, like everything on this page.</p>
            {/if}
          </section>

        {#if faceOpen && chars && c.model >= 0}
          <FaceEditor {chars} body={bodyOf(c)} preset={(c.female ? 'Female' : 'Male') + c.model} bind:face={c.face}
            clothes={previewClothes(c, clothesTpl, looks)} title={c.name || 'New person'} onclose={() => closeFace(c)} />
        {/if}

<style>
  .chips { display: flex; flex-wrap: wrap; gap: 6px; }
  .swatch { display: inline-block; width: 10px; height: 10px; border-radius: 2px; margin-right: 5px; border: 1px solid var(--line); vertical-align: -1px; }
  .chip { border-radius: 14px; padding: 4px 12px; font-size: 13px; }
  .chip.on { border-color: var(--accent); background: #22304d; }
  .face:hover .del { opacity: 1; }
</style>
