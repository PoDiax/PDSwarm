<script lang="ts">
  import { onMount, tick } from 'svelte';
  import {
    FileCode,
    Save,
    Download,
    X,
    Loader2,
    AlertCircle,
    CheckCircle2,
    Copy,
    Check,
    FileText,
  } from '@lucide/svelte';
  import { fileApi } from '../api';

  interface Props {
    containerId: string;
    filePath?: string;
    initialDirectory?: string;
    isNewFile?: boolean;
    onClose: () => void;
    onSaved?: (savedPath: string) => void;
  }

  let {
    containerId,
    filePath = '',
    initialDirectory = '/',
    isNewFile = false,
    onClose,
    onSaved,
  }: Props = $props();

  // Internal states
  let currentPath = $state('');
  let content = $state('');
  let initialContent = $state('');
  let isLoading = $state(false);
  let isSaving = $state(false);
  let error = $state<string | null>(null);
  let successMsg = $state<string | null>(null);
  let isCopied = $state(false);

  $effect(() => {
    currentPath = filePath || (initialDirectory.endsWith('/') ? `${initialDirectory}untitled.txt` : `${initialDirectory}/untitled.txt`);
  });

  let textareaRef = $state<HTMLTextAreaElement | null>(null);
  let gutterRef = $state<HTMLDivElement | null>(null);
  let cursorLine = $state(1);
  let cursorCol = $state(1);

  // Derived
  let isDirty = $derived(content !== initialContent);
  let lines = $derived(content.split('\n'));
  let lineCount = $derived(lines.length);
  let byteSizeFormatted = $derived.by(() => {
    const bytes = new Blob([content]).size;
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  });

  let fileName = $derived.by(() => {
    const trimmed = currentPath.trim();
    const lastSlash = trimmed.lastIndexOf('/');
    return lastSlash === -1 ? trimmed : trimmed.slice(lastSlash + 1) || 'file';
  });

  async function loadFile() {
    if (isNewFile || !filePath) {
      content = '';
      initialContent = '';
      return;
    }

    isLoading = true;
    error = null;
    successMsg = null;

    try {
      const data = await fileApi.read(containerId, filePath);
      content = data.content;
      initialContent = data.content;
      currentPath = data.path || filePath;
    } catch (err: unknown) {
      if (err instanceof Error) {
        error = err.message;
      } else {
        error = 'Failed to read file from container.';
      }
    } finally {
      isLoading = false;
    }
  }

  onMount(() => {
    loadFile();
  });

  function handleScroll(e: Event) {
    const target = e.target as HTMLTextAreaElement;
    if (gutterRef) {
      gutterRef.scrollTop = target.scrollTop;
    }
  }

  function updateCursor() {
    if (!textareaRef) return;
    const pos = textareaRef.selectionStart;
    const textUpToCursor = content.slice(0, pos);
    const lineArr = textUpToCursor.split('\n');
    cursorLine = lineArr.length;
    cursorCol = lineArr[lineArr.length - 1].length + 1;
  }

  function handleInput(e: Event) {
    const target = e.target as HTMLTextAreaElement;
    content = target.value;
    updateCursor();
  }

  function handleKeyDown(e: KeyboardEvent) {
    if (!textareaRef) return;

    if (e.key === 'Tab') {
      e.preventDefault();
      const start = textareaRef.selectionStart;
      const end = textareaRef.selectionEnd;

      if (start === end) {
        // Single cursor position
        if (e.shiftKey) {
          // Unindent current line
          const textBefore = content.slice(0, start);
          const lastNewline = textBefore.lastIndexOf('\n');
          const lineStart = lastNewline === -1 ? 0 : lastNewline + 1;
          const linePrefix = content.slice(lineStart, lineStart + 2);
          if (linePrefix === '  ') {
            content = content.slice(0, lineStart) + content.slice(lineStart + 2);
            tick().then(() => {
              if (textareaRef) {
                const newPos = Math.max(lineStart, start - 2);
                textareaRef.setSelectionRange(newPos, newPos);
                updateCursor();
              }
            });
          }
        } else {
          // Indent 2 spaces
          content = content.substring(0, start) + '  ' + content.substring(end);
          tick().then(() => {
            if (textareaRef) {
              textareaRef.selectionStart = textareaRef.selectionEnd = start + 2;
              updateCursor();
            }
          });
        }
      } else {
        // Multi-line selection
        const linesBefore = content.substring(0, start).split('\n');
        const startLineIdx = linesBefore.length - 1;
        const allLines = content.split('\n');
        const linesSelection = content.substring(start, end).split('\n');
        const endLineIdx = startLineIdx + linesSelection.length - 1;

        if (e.shiftKey) {
          let charsRemoved = 0;
          for (let i = startLineIdx; i <= endLineIdx; i++) {
            if (allLines[i].startsWith('  ')) {
              allLines[i] = allLines[i].slice(2);
              charsRemoved += 2;
            } else if (allLines[i].startsWith(' ')) {
              allLines[i] = allLines[i].slice(1);
              charsRemoved += 1;
            }
          }
          content = allLines.join('\n');
          tick().then(() => {
            if (textareaRef) {
              textareaRef.setSelectionRange(start, Math.max(start, end - charsRemoved));
              updateCursor();
            }
          });
        } else {
          let charsAdded = 0;
          for (let i = startLineIdx; i <= endLineIdx; i++) {
            allLines[i] = '  ' + allLines[i];
            charsAdded += 2;
          }
          content = allLines.join('\n');
          tick().then(() => {
            if (textareaRef) {
              textareaRef.setSelectionRange(start + 2, end + charsAdded);
              updateCursor();
            }
          });
        }
      }
    } else if (e.key === 'Enter') {
      // Auto-indent on Enter
      e.preventDefault();
      const start = textareaRef.selectionStart;
      const textBefore = content.slice(0, start);
      const currentLine = textBefore.split('\n').pop() || '';
      const indentMatch = currentLine.match(/^(\s*)/);
      let indent = indentMatch ? indentMatch[1] : '';

      if (currentLine.trim().endsWith('{') || currentLine.trim().endsWith(':')) {
        indent += '  ';
      }

      const insertText = '\n' + indent;
      content = content.slice(0, start) + insertText + content.slice(textareaRef.selectionEnd);

      tick().then(() => {
        if (textareaRef) {
          const newPos = start + insertText.length;
          textareaRef.selectionStart = textareaRef.selectionEnd = newPos;
          updateCursor();
        }
      });
    } else if ((e.ctrlKey || e.metaKey) && e.key === 's') {
      e.preventDefault();
      saveFile();
    }
  }

  async function saveFile() {
    const trimmedPath = currentPath.trim();
    if (!trimmedPath) {
      error = 'File path cannot be empty.';
      return;
    }

    if (!trimmedPath.startsWith('/')) {
      error = 'File path must be an absolute path starting with /';
      return;
    }

    isSaving = true;
    error = null;
    successMsg = null;

    try {
      const lastSlash = trimmedPath.lastIndexOf('/');
      const destDir = lastSlash <= 0 ? '/' : trimmedPath.slice(0, lastSlash);
      const destFilename = trimmedPath.slice(lastSlash + 1);

      if (!destFilename) {
        throw new Error('Please specify a valid file name.');
      }

      const file = new File([content], destFilename, { type: 'text/plain' });
      await fileApi.upload(containerId, destDir, file);

      initialContent = content;
      successMsg = `Saved ${destFilename} successfully!`;
      if (onSaved) {
        onSaved(trimmedPath);
      }
      setTimeout(() => {
        if (successMsg) successMsg = null;
      }, 3500);
    } catch (err: unknown) {
      if (err instanceof Error) {
        error = err.message;
      } else {
        error = 'Failed to save file to container.';
      }
    } finally {
      isSaving = false;
    }
  }

  function handleDownload() {
    const blob = new Blob([content], { type: 'text/plain;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = fileName;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  }

  function handleCopy() {
    if (!content) return;
    navigator.clipboard.writeText(content);
    isCopied = true;
    setTimeout(() => {
      isCopied = false;
    }, 2000);
  }

  function handleCloseRequest() {
    if (isDirty) {
      if (!confirm('You have unsaved changes. Are you sure you want to discard them and close?')) {
        return;
      }
    }
    onClose();
  }

  function handleBackdropClick(e: MouseEvent) {
    if (e.target === e.currentTarget) {
      handleCloseRequest();
    }
  }
</script>

<div
  class="fixed inset-0 z-50 flex items-center justify-center p-4 sm:p-6"
  role="dialog"
  aria-modal="true"
  aria-labelledby="file-editor-modal-title"
  tabindex="-1"
>
  <!-- Backdrop -->
  <button
    type="button"
    class="fixed inset-0 bg-black/80 backdrop-blur-md cursor-default border-0"
    onclick={handleCloseRequest}
    aria-label="Close file editor"
    tabindex="-1"
  ></button>

  <!-- Modal Container -->
  <div
    class="relative w-full max-w-5xl h-[88vh] max-h-[850px] flex flex-col rounded-2xl bg-[#0c0d14] border border-white/[0.12] shadow-[0_15px_60px_rgba(0,0,0,0.85)] overflow-hidden animate-in fade-in zoom-in-95 duration-150"
  >
    <!-- Modal Header -->
    <div
      class="flex flex-wrap items-center justify-between gap-3 px-5 py-3.5 bg-[#090a10] border-b border-white/[0.08] select-none shrink-0"
    >
      <div class="flex items-center gap-3 min-w-0 flex-1">
        <div class="w-8 h-8 rounded-xl bg-violet-500/10 border border-violet-500/20 flex items-center justify-center text-violet-400 shrink-0">
          <FileCode class="w-4 h-4 text-cyan-400" />
        </div>

        <!-- Editable Path Input or Static Path -->
        <div class="flex-1 min-w-0">
          <div class="flex items-center gap-2">
            <span id="file-editor-modal-title" class="text-xs font-mono font-bold text-slate-400 uppercase tracking-wider">
              {isNewFile ? 'New File' : 'Edit File'}:
            </span>
            <input
              type="text"
              bind:value={currentPath}
              placeholder="/path/to/file.txt"
              class="flex-1 min-w-[200px] bg-[#12131c] border border-white/[0.08] focus:border-cyan-500/50 rounded-lg px-2.5 py-1 text-xs font-mono text-cyan-300 outline-none transition"
            />
          </div>
        </div>

        <!-- Badges -->
        <div class="hidden sm:flex items-center gap-2">
          {#if isDirty}
            <span class="px-2 py-0.5 rounded-full text-[10px] font-mono font-semibold bg-amber-500/15 text-amber-300 border border-amber-500/30">
              Unsaved Changes
            </span>
          {:else}
            <span class="px-2 py-0.5 rounded-full text-[10px] font-mono font-medium bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
              Clean
            </span>
          {/if}

          <span class="px-2 py-0.5 rounded-full text-[10px] font-mono bg-white/[0.04] text-slate-300 border border-white/[0.06]">
            {byteSizeFormatted}
          </span>
        </div>
      </div>

      <!-- Header Action Buttons -->
      <div class="flex items-center gap-2 shrink-0">
        <button
          type="button"
          onclick={handleCopy}
          class="p-2 rounded-xl bg-white/[0.04] hover:bg-white/[0.08] text-slate-300 hover:text-white border border-white/[0.06] text-xs transition inline-flex items-center gap-1.5"
          title="Copy file content to clipboard"
        >
          {#if isCopied}
            <Check class="w-3.5 h-3.5 text-emerald-400" />
            <span class="hidden sm:inline text-emerald-400 text-[11px] font-mono">Copied</span>
          {:else}
            <Copy class="w-3.5 h-3.5" />
            <span class="hidden sm:inline text-[11px] font-mono">Copy</span>
          {/if}
        </button>

        <button
          type="button"
          onclick={handleDownload}
          class="p-2 rounded-xl bg-white/[0.04] hover:bg-white/[0.08] text-slate-300 hover:text-white border border-white/[0.06] text-xs transition inline-flex items-center gap-1.5"
          title="Download file to computer"
        >
          <Download class="w-3.5 h-3.5 text-cyan-400" />
          <span class="hidden sm:inline text-[11px] font-mono">Download</span>
        </button>

        <button
          type="button"
          onclick={handleCloseRequest}
          class="p-2 rounded-xl bg-white/[0.04] hover:bg-rose-500/20 text-slate-400 hover:text-rose-300 border border-white/[0.06] text-xs transition"
          title="Close editor (Esc)"
        >
          <X class="w-4 h-4" />
        </button>
      </div>
    </div>

    <!-- Notification / Status Banner -->
    {#if error}
      <div class="px-5 py-2.5 bg-rose-500/10 border-b border-rose-500/25 flex items-center justify-between text-xs text-rose-300">
        <div class="flex items-center gap-2">
          <AlertCircle class="w-4 h-4 text-rose-400 shrink-0" />
          <span>{error}</span>
        </div>
        <button
          type="button"
          onclick={() => error = null}
          class="text-rose-400 hover:text-white text-xs"
        >
          Dismiss
        </button>
      </div>
    {:else if successMsg}
      <div class="px-5 py-2 bg-emerald-500/10 border-b border-emerald-500/25 flex items-center gap-2 text-xs text-emerald-300">
        <CheckCircle2 class="w-4 h-4 text-emerald-400 shrink-0" />
        <span>{successMsg}</span>
      </div>
    {/if}

    <!-- Editor Main Workspace -->
    <div class="relative flex-1 flex overflow-hidden bg-[#0a0b10]">
      {#if isLoading}
        <div class="absolute inset-0 flex flex-col items-center justify-center gap-3 bg-[#0a0b10]/90 z-20">
          <Loader2 class="w-8 h-8 animate-spin text-cyan-400" />
          <span class="text-xs font-mono text-slate-300">Reading container file...</span>
        </div>
      {/if}

      <!-- Line numbers gutter -->
      <div
        bind:this={gutterRef}
        class="w-12 sm:w-14 select-none overflow-hidden bg-[#08090e] border-r border-white/[0.06] py-3 text-right font-mono text-[13px] leading-6 text-slate-600 transition-colors shrink-0"
        aria-hidden="true"
      >
        {#each lines as _, i}
          {@const lineNum = i + 1}
          {@const isCurrent = lineNum === cursorLine}
          <div
            class="px-2.5 transition-colors {isCurrent ? 'text-cyan-400 font-bold bg-white/[0.04]' : ''}"
          >
            {lineNum}
          </div>
        {/each}
      </div>

      <!-- Editor Textarea Area -->
      <div class="relative flex-1 overflow-hidden">
        <textarea
          bind:this={textareaRef}
          value={content}
          oninput={handleInput}
          onkeydown={handleKeyDown}
          onscroll={handleScroll}
          onclick={updateCursor}
          onkeyup={updateCursor}
          onselect={updateCursor}
          placeholder="# Empty file - start typing or paste content..."
          spellcheck="false"
          autocapitalize="off"
          autocomplete="off"
          class="w-full h-full m-0 py-3 px-4 bg-transparent font-mono text-[13px] leading-6 text-slate-200 caret-cyan-400 whitespace-pre overflow-auto resize-none outline-none border-none tab-[2] selection:bg-violet-600/35 selection:text-white"
        ></textarea>
      </div>
    </div>

    <!-- Modal Footer / Status Bar -->
    <div
      class="flex flex-wrap items-center justify-between gap-3 px-5 py-3 bg-[#090a10] border-t border-white/[0.08] text-xs font-mono select-none shrink-0"
    >
      <!-- Telemetry info -->
      <div class="flex items-center gap-3 text-slate-400 text-[11px]">
        <div class="flex items-center gap-1.5">
          <span class="w-1.5 h-1.5 rounded-full bg-cyan-400 shadow-[0_0_8px_#06b6d4]"></span>
          <span>UTF-8</span>
        </div>
        <span>&bull;</span>
        <span>{lineCount} {lineCount === 1 ? 'line' : 'lines'}</span>
        <span>&bull;</span>
        <span>Ln {cursorLine}, Col {cursorCol}</span>
      </div>

      <!-- Modal Bottom Actions -->
      <div class="flex items-center gap-3">
        <button
          type="button"
          onclick={handleCloseRequest}
          class="px-4 py-2 rounded-xl hover:bg-white/10 text-slate-300 text-xs font-semibold transition"
        >
          Cancel
        </button>

        <button
          type="button"
          onclick={saveFile}
          disabled={isSaving || isLoading}
          class="px-5 py-2 rounded-xl bg-violet-600 hover:bg-violet-500 disabled:opacity-50 text-white text-xs font-semibold shadow-[0_0_15px_rgba(124,58,237,0.4)] transition inline-flex items-center gap-2"
        >
          {#if isSaving}
            <Loader2 class="w-3.5 h-3.5 animate-spin" />
            <span>Saving...</span>
          {:else}
            <Save class="w-3.5 h-3.5" />
            <span>Save Changes</span>
          {/if}
        </button>
      </div>
    </div>
  </div>
</div>

<style>
  textarea::-webkit-scrollbar {
    width: 8px;
    height: 8px;
  }
  textarea::-webkit-scrollbar-track {
    background: #08090e;
  }
  textarea::-webkit-scrollbar-thumb {
    background: rgba(255, 255, 255, 0.12);
    border-radius: 4px;
  }
  textarea::-webkit-scrollbar-thumb:hover {
    background: rgba(124, 58, 237, 0.4);
  }
</style>
