<script lang="ts">
  // A code editor (CodeMirror 6) for shader code: C-like highlighting, line numbers, and the compiler's messages shown
  // on their lines. `value` is two-way; the owner passes the messages of its last compile.
  import { onMount } from 'svelte';
  import { EditorView, basicSetup } from 'codemirror';
  import { EditorState } from '@codemirror/state';
  import { keymap } from '@codemirror/view';
  import { indentWithTab, redo } from '@codemirror/commands';
  import { cpp } from '@codemirror/lang-cpp';
  import { oneDark } from '@codemirror/theme-one-dark';
  import { setDiagnostics, type Diagnostic } from '@codemirror/lint';

  let { value = $bindable(''), messages = [], readonly = false }: { value: string; messages?: { line: number; text: string }[]; readonly?: boolean } = $props();

  let host = $state<HTMLDivElement>();
  let view: EditorView | undefined;

  onMount(() => {
    view = new EditorView({
      parent: host!,
      state: EditorState.create({
        doc: value,
        extensions: [
          basicSetup,
          // Redo is Ctrl+Y in CodeMirror on Windows; Ctrl+Shift+Z is what most editors use, so both work here.
          keymap.of([indentWithTab, { key: 'Mod-Shift-z', run: redo, preventDefault: true }]), cpp(), oneDark,
          EditorState.tabSize.of(4), EditorState.readOnly.of(readonly), EditorView.editable.of(!readonly),
          EditorView.updateListener.of((u) => { if (u.docChanged) value = u.state.doc.toString(); }),
          EditorView.theme({ '&': { height: '100%', fontSize: '13px' }, '.cm-scroller': { fontFamily: 'Consolas, "Cascadia Mono", monospace' } }),
        ],
      }),
    });
    return () => view?.destroy();
  });

  // Another text from outside (another shader selected, Revert): replace the document.
  $effect(() => {
    const v = value;
    if (view && v !== view.state.doc.toString()) view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: v } });
  });

  $effect(() => {
    const list = messages;
    if (!view) return;
    const doc = view.state.doc;
    const diags: Diagnostic[] = list.map((m) => {
      const line = doc.line(Math.min(Math.max(1, m.line), doc.lines));
      return { from: line.from, to: Math.max(line.to, line.from), severity: 'error', message: m.text };
    });
    view.dispatch(setDiagnostics(view.state, diags));
  });
</script>

<div class="code" bind:this={host}></div>

<style>
  .code { flex: 1; min-height: 0; overflow: hidden; border: 1px solid var(--line); border-radius: 8px; }
  .code :global(.cm-editor) { height: 100%; }
</style>
