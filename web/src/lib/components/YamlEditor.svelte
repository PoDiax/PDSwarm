<script lang="ts">
  import {
    onMount,
    tick } from 'svelte';
  import {
    Copy,
    Check,
    CircleAlert,
    CircleCheck,
    Code2,
    FileCode,
    Sparkles,
    Trash,
    AlertTriangle,
  } from '@lucide/svelte';

  interface Props {
    value?: string;
    readonly?: boolean;
    placeholder?: string;
    minHeight?: string;
    onChange?: (val: string) => void;
    onValidate?: (isValid: boolean, error: string | null) => void;
    class?: string;
  }

  let {
    value = $bindable(''),
    readonly = false,
    placeholder = '# Write or paste Docker Compose YAML here...',
    minHeight = '420px',
    onChange,
    onValidate,
    class: customClass = '',
  }: Props = $props();

  let textareaRef: HTMLTextAreaElement | null = $state(null);
  let highlightRef: HTMLPreElement | null = $state(null);
  let gutterRef: HTMLDivElement | null = $state(null);

  let isCopied = $state(false);
  let cursorLine = $state(1);
  let cursorCol = $state(1);

  // Line count
  let lines = $derived(value.split('\n'));
  let lineCount = $derived(lines.length);
  let charCount = $derived(value.length);
  let byteSizeFormatted = $derived.by(() => {
    const bytes = new Blob([value]).size;
    if (bytes < 1024) return `${bytes} B`;
    return `${(bytes / 1024).toFixed(1)} KB`;
  });

  // Client-side YAML Validation
  interface ValidationError {
    line: number;
    message: string;
    type: 'error' | 'warning';
  }

  let validationResult = $derived.by<ValidationError | null>(() => {
    if (!value.trim()) return null;

    const rawLines = value.split('\n');

    for (let i = 0; i < rawLines.length; i++) {
      const lineNum = i + 1;
      const raw = rawLines[i];

      // 1. Check for tab characters in indentation
      if (/^\s*\t/.test(raw)) {
        return {
          line: lineNum,
          message: 'Tabs are forbidden in YAML indentation. Use 2 spaces.',
          type: 'error',
        };
      }

      const trimmed = raw.trim();
      if (!trimmed || trimmed.startsWith('#')) continue;

      // 2. Check for unbalanced quotes on single line
      const doubleQuotes = (trimmed.match(/"/g) || []).length;
      if (doubleQuotes % 2 !== 0 && !trimmed.endsWith('\\')) {
        return {
          line: lineNum,
          message: 'Unclosed double quote (") detected.',
          type: 'error',
        };
      }

      const singleQuotes = (trimmed.match(/'/g) || []).length;
      if (singleQuotes % 2 !== 0 && !trimmed.endsWith('\\')) {
        return {
          line: lineNum,
          message: "Unclosed single quote (') detected.",
          type: 'error',
        };
      }

      // 3. Check for missing space after colon in key-value mapping
      // e.g. "image:nginx" (ignore http:// or https:// or port strings like 80:80)
      if (/^[a-zA-Z0-9_\-\.]+:[^\s\/]/.test(trimmed)) {
        // Exclude if it looks like a list item port "80:80" or url
        if (!/^[0-9]+:[0-9]+/.test(trimmed) && !trimmed.includes('://')) {
          return {
            line: lineNum,
            message: 'Missing space after colon in mapping key.',
            type: 'warning',
          };
        }
      }
    }

    // 4. Compose structure recommendation
    if (!value.includes('services:') && !value.includes('version:')) {
      return {
        line: 1,
        message: "Compose file typically begins with 'services:' definition.",
        type: 'warning',
      };
    }

    return null;
  });

  $effect(() => {
    if (onValidate) {
      const err = validationResult;
      onValidate(err?.type !== 'error', err?.message || null);
    }
  });

  // Helpers for syntax highlighting
  function escapeHtml(str: string): string {
    return str
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#39;');
  }

  function findCommentIndex(str: string): number {
    let inSingle = false;
    let inDouble = false;
    for (let i = 0; i < str.length; i++) {
      const ch = str[i];
      if (ch === "'" && !inDouble) inSingle = !inSingle;
      else if (ch === '"' && !inSingle) inDouble = !inDouble;
      else if (ch === '#' && !inSingle && !inDouble) {
        if (i === 0 || /\s/.test(str[i - 1])) {
          return i;
        }
      }
    }
    return -1;
  }

  function highlightValue(val: string): string {
    const trimmed = val.trim();
    if (!trimmed) return escapeHtml(val);

    // Quoted strings
    if (
      (trimmed.startsWith('"') && trimmed.endsWith('"')) ||
      (trimmed.startsWith("'") && trimmed.endsWith("'"))
    ) {
      return `<span class="text-emerald-300">${escapeHtml(val)}</span>`;
    }

    // Booleans
    if (/^(true|false|yes|no|on|off)$/i.test(trimmed)) {
      return `<span class="text-violet-400 font-semibold">${escapeHtml(val)}</span>`;
    }

    // Numbers
    if (/^-?\d+(\.\d+)?$/.test(trimmed)) {
      return `<span class="text-amber-400 font-mono">${escapeHtml(val)}</span>`;
    }

    // Environment variables like ${PORT} or $PORT
    if (/\$\{[^}]+\}|\$[A-Z0-9_]+/i.test(trimmed)) {
      return `<span class="text-pink-300 font-mono">${escapeHtml(val)}</span>`;
    }

    // General scalar / identifier
    return `<span class="text-slate-200">${escapeHtml(val)}</span>`;
  }

  function highlightYamlLine(rawLine: string): string {
    if (!rawLine) return '<br/>';

    const commentIdx = findCommentIndex(rawLine);
    let codePart = rawLine;
    let commentHtml = '';

    if (commentIdx !== -1) {
      codePart = rawLine.slice(0, commentIdx);
      commentHtml = `<span class="text-slate-500 italic">${escapeHtml(rawLine.slice(commentIdx))}</span>`;
    }

    let highlighted = '';

    // Extract leading whitespace
    const indentMatch = codePart.match(/^(\s+)/);
    let rest = codePart;
    if (indentMatch) {
      highlighted += escapeHtml(indentMatch[1]);
      rest = codePart.slice(indentMatch[1].length);
    }

    // Check list hyphen '- '
    const listMatch = rest.match(/^(-\s+)(.*)$/);
    if (listMatch) {
      highlighted += `<span class="text-fuchsia-400 font-bold">${escapeHtml(listMatch[1])}</span>`;
      rest = listMatch[2];
    }

    // Check key: value
    const kvMatch = rest.match(/^([a-zA-Z0-9_\-\.\/]+)(\s*:)(\s*)(.*)$/);
    if (kvMatch) {
      const key = kvMatch[1];
      const colon = kvMatch[2];
      const space = kvMatch[3];
      const val = kvMatch[4];

      highlighted += `<span class="text-cyan-400 font-medium">${escapeHtml(key)}</span><span class="text-slate-400">${escapeHtml(colon)}</span>${escapeHtml(space)}`;
      if (val) {
        highlighted += highlightValue(val);
      }
    } else {
      highlighted += highlightValue(rest);
    }

    return highlighted + commentHtml;
  }

  let highlightedHtml = $derived.by(() => {
    return lines.map(line => highlightYamlLine(line)).join('\n');
  });

  // Scroll synchronization
  function handleScroll(e: Event) {
    const target = e.target as HTMLTextAreaElement;
    if (highlightRef) {
      highlightRef.scrollTop = target.scrollTop;
      highlightRef.scrollLeft = target.scrollLeft;
    }
    if (gutterRef) {
      gutterRef.scrollTop = target.scrollTop;
    }
  }

  // Update cursor position
  function updateCursor() {
    if (!textareaRef) return;
    const pos = textareaRef.selectionStart;
    const textUpToCursor = value.slice(0, pos);
    const lineArr = textUpToCursor.split('\n');
    cursorLine = lineArr.length;
    cursorCol = lineArr[lineArr.length - 1].length + 1;
  }

  // Handle Tab and Enter key navigation
  function handleKeyDown(e: KeyboardEvent) {
    if (readonly || !textareaRef) return;

    if (e.key === 'Tab') {
      e.preventDefault();
      const start = textareaRef.selectionStart;
      const end = textareaRef.selectionEnd;

      if (start === end) {
        // Single cursor position: insert 2 spaces
        if (e.shiftKey) {
          // Unindent current line
          const textBefore = value.slice(0, start);
          const lastNewline = textBefore.lastIndexOf('\n');
          const lineStart = lastNewline === -1 ? 0 : lastNewline + 1;
          const linePrefix = value.slice(lineStart, lineStart + 2);
          if (linePrefix === '  ') {
            value = value.slice(0, lineStart) + value.slice(lineStart + 2);
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
          value = value.substring(0, start) + '  ' + value.substring(end);
          tick().then(() => {
            if (textareaRef) {
              textareaRef.selectionStart = textareaRef.selectionEnd = start + 2;
              updateCursor();
            }
          });
        }
      } else {
        // Multi-line selection indent / unindent
        const linesBefore = value.substring(0, start).split('\n');
        const startLineIdx = linesBefore.length - 1;
        const allLines = value.split('\n');
        const linesSelection = value.substring(start, end).split('\n');
        const endLineIdx = startLineIdx + linesSelection.length - 1;

        if (e.shiftKey) {
          // Unindent all selected lines
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
          value = allLines.join('\n');
          tick().then(() => {
            if (textareaRef) {
              textareaRef.setSelectionRange(start, Math.max(start, end - charsRemoved));
              updateCursor();
            }
          });
        } else {
          // Indent all selected lines
          let charsAdded = 0;
          for (let i = startLineIdx; i <= endLineIdx; i++) {
            allLines[i] = '  ' + allLines[i];
            charsAdded += 2;
          }
          value = allLines.join('\n');
          tick().then(() => {
            if (textareaRef) {
              textareaRef.setSelectionRange(start + 2, end + charsAdded);
              updateCursor();
            }
          });
        }
      }

      if (onChange) onChange(value);
    } else if (e.key === 'Enter') {
      // Auto-indent on Enter
      e.preventDefault();
      const start = textareaRef.selectionStart;
      const textBefore = value.slice(0, start);
      const currentLine = textBefore.split('\n').pop() || '';
      const indentMatch = currentLine.match(/^(\s*)/);
      let indent = indentMatch ? indentMatch[1] : '';

      // If line ends with colon, add 2 additional spaces
      if (currentLine.trim().endsWith(':')) {
        indent += '  ';
      }

      const insertText = '\n' + indent;
      value = value.slice(0, start) + insertText + value.slice(textareaRef.selectionEnd);

      tick().then(() => {
        if (textareaRef) {
          const newPos = start + insertText.length;
          textareaRef.selectionStart = textareaRef.selectionEnd = newPos;
          updateCursor();
        }
      });

      if (onChange) onChange(value);
    }
  }

  function handleInput(e: Event) {
    const target = e.target as HTMLTextAreaElement;
    value = target.value;
    updateCursor();
    if (onChange) onChange(value);
  }

  function handleCopy() {
    if (!value) return;
    navigator.clipboard.writeText(value);
    isCopied = true;
    setTimeout(() => {
      isCopied = false;
    }, 2000);
  }

  function handleFormat() {
    if (readonly) return;
    // Replace tabs with 2 spaces and trim trailing whitespace
    const formatted = value
      .split('\n')
      .map(line => line.replace(/\t/g, '  ').replace(/\s+$/, ''))
      .join('\n');
    value = formatted;
    if (onChange) onChange(value);
  }

  function handleClear() {
    if (readonly) return;
    value = '';
    if (onChange) onChange('');
    if (textareaRef) {
      textareaRef.focus();
      updateCursor();
    }
  }

  function jumpToLine(targetLine: number) {
    if (!textareaRef) return;
    const allLines = value.split('\n');
    let charOffset = 0;
    for (let i = 0; i < Math.min(targetLine - 1, allLines.length); i++) {
      charOffset += allLines[i].length + 1;
    }
    textareaRef.focus();
    textareaRef.setSelectionRange(charOffset, charOffset + (allLines[targetLine - 1]?.length || 0));
    updateCursor();
  }
</script>

<div
  class="flex flex-col rounded-2xl bg-[#0e1017] border border-white/[0.08] shadow-2xl overflow-hidden {customClass}"
>
  <!-- Top Editor Header / Toolbar -->
  <div
    class="flex flex-wrap items-center justify-between gap-3 px-4 py-2.5 bg-[#090a10] border-b border-white/[0.08] text-xs select-none"
  >
    <div class="flex items-center gap-3">
      <!-- File type indicator -->
      <div class="flex items-center gap-1.5 text-slate-300 font-medium">
        <FileCode class="w-4 h-4 text-cyan-400" />
        <span class="font-mono text-cyan-300 font-semibold tracking-wide">docker-compose.yml</span>
      </div>

      <span class="w-1 h-3 rounded-full bg-white/10"></span>

      <!-- Statistics -->
      <div class="flex items-center gap-2 font-mono text-[11px] text-slate-400">
        <span>{lineCount} {lineCount === 1 ? 'line' : 'lines'}</span>
        <span>&bull;</span>
        <span>{byteSizeFormatted}</span>
      </div>
    </div>

    <!-- Right toolbar tools & Validation Badge -->
    <div class="flex items-center gap-2.5">
      <!-- Status Badge -->
      {#if validationResult}
        <button
          onclick={() => jumpToLine(validationResult.line)}
          title="Click to jump to line {validationResult.line}"
          class="flex items-center gap-1.5 px-2.5 py-1 rounded-lg text-[11px] font-mono transition {validationResult.type === 'error'
            ? 'bg-rose-500/15 text-rose-300 border border-rose-500/30 hover:bg-rose-500/25'
            : 'bg-amber-500/15 text-amber-300 border border-amber-500/30 hover:bg-amber-500/25'}"
        >
          {#if validationResult.type === 'error'}
            <CircleAlert class="w-3.5 h-3.5 text-rose-400 shrink-0" />
          {:else}
            <AlertTriangle class="w-3.5 h-3.5 text-amber-400 shrink-0" />
          {/if}
          <span class="font-semibold">Ln {validationResult.line}:</span>
          <span class="truncate max-w-[220px]">{validationResult.message}</span>
        </button>
      {:else if value.trim()}
        <div
          class="flex items-center gap-1.5 px-2.5 py-1 rounded-lg text-[11px] font-mono bg-emerald-500/10 text-emerald-300 border border-emerald-500/20"
        >
          <CircleCheck class="w-3.5 h-3.5 text-emerald-400 shrink-0" />
          <span>Valid Compose YAML</span>
        </div>
      {/if}

      <!-- Action Buttons -->
      {#if !readonly}
        <button
          onclick={handleFormat}
          title="Format YAML: Replace tabs with spaces and trim trailing whitespace"
          class="flex items-center gap-1 px-2.5 py-1 rounded-lg bg-white/[0.04] hover:bg-white/[0.08] text-slate-300 hover:text-white border border-white/[0.06] transition text-[11px]"
        >
          <Sparkles class="w-3 h-3 text-cyan-400" />
          <span>Format</span>
        </button>

        <button
          onclick={handleClear}
          title="Clear editor contents"
          class="p-1 rounded-lg bg-white/[0.04] hover:bg-rose-500/20 text-slate-400 hover:text-rose-300 border border-white/[0.06] transition"
        >
          <Trash class="w-3.5 h-3.5" />
        </button>
      {/if}

      <button
        onclick={handleCopy}
        title="Copy YAML to clipboard"
        class="flex items-center gap-1 px-2.5 py-1 rounded-lg bg-white/[0.04] hover:bg-white/[0.08] text-slate-300 hover:text-white border border-white/[0.06] transition text-[11px]"
      >
        {#if isCopied}
          <Check class="w-3.5 h-3.5 text-emerald-400" />
          <span class="text-emerald-400 font-semibold">Copied</span>
        {:else}
          <Copy class="w-3.5 h-3.5" />
          <span>Copy</span>
        {/if}
      </button>
    </div>
  </div>

  <!-- Main Editor Area with Gutter & Overlay -->
  <div class="relative flex flex-1 overflow-hidden" style="min-height: {minHeight};">
    <!-- Line numbers gutter -->
    <div
      bind:this={gutterRef}
      class="w-12 select-none overflow-hidden bg-[#090a0f] border-r border-white/[0.06] py-3 text-right font-mono text-[13px] leading-6 text-slate-600 transition-colors shrink-0"
      aria-hidden="true"
    >
      {#each lines as _, i}
        {@const lineNum = i + 1}
        {@const isCurrent = lineNum === cursorLine}
        {@const isError = validationResult?.line === lineNum}
        <div
          class="px-2 transition-colors {isError
            ? 'text-rose-400 font-bold bg-rose-500/10'
            : isCurrent
              ? 'text-cyan-400 font-bold bg-white/[0.04]'
              : ''}"
        >
          {lineNum}
        </div>
      {/each}
    </div>

    <!-- Code Display & Input Wrapper -->
    <div class="relative flex-1 overflow-hidden bg-[#0c0d14]">
      <!-- Syntax Highlighted Code Backdrop -->
      <pre
        bind:this={highlightRef}
        class="absolute inset-0 m-0 py-3 px-4 overflow-hidden pointer-events-none font-mono text-[13px] leading-6 text-slate-200 whitespace-pre tab-[2] select-none"
        aria-hidden="true"><code>{@html highlightedHtml}</code></pre>

      <!-- Transparent Textarea for Editing -->
      <textarea
        bind:this={textareaRef}
        {value}
        {readonly}
        {placeholder}
        oninput={handleInput}
        onkeydown={handleKeyDown}
        onscroll={handleScroll}
        onclick={updateCursor}
        onkeyup={updateCursor}
        onselect={updateCursor}
        spellcheck="false"
        autocapitalize="off"
        autocomplete="off"
        class="absolute inset-0 w-full h-full m-0 py-3 px-4 bg-transparent font-mono text-[13px] leading-6 text-transparent caret-cyan-400 whitespace-pre overflow-auto resize-none outline-none border-none tab-[2] selection:bg-violet-600/35 selection:text-transparent"
      ></textarea>
    </div>
  </div>

  <!-- Bottom Status Bar -->
  <div
    class="flex items-center justify-between px-4 py-1.5 bg-[#090a10] border-t border-white/[0.08] text-[11px] font-mono text-slate-400 select-none"
  >
    <div class="flex items-center gap-3">
      <div class="flex items-center gap-1.5">
        <span class="w-1.5 h-1.5 rounded-full bg-cyan-400 shadow-[0_0_8px_#06b6d4]"></span>
        <span>UTF-8</span>
      </div>
      <span>&bull;</span>
      <span>YAML</span>
      <span>&bull;</span>
      <span>Spaces: 2</span>
    </div>

    <div class="flex items-center gap-4">
      {#if readonly}
        <span class="text-amber-400/90 font-sans text-[10px] tracking-wide uppercase font-semibold">
          Read Only
        </span>
      {:else}
        <span>Ln {cursorLine}, Col {cursorCol}</span>
      {/if}
    </div>
  </div>
</div>

<style>
  /* Custom scrollbar matching dark theme aesthetic */
  textarea::-webkit-scrollbar {
    width: 8px;
    height: 8px;
  }
  textarea::-webkit-scrollbar-track {
    background: #090a0f;
  }
  textarea::-webkit-scrollbar-thumb {
    background: rgba(255, 255, 255, 0.12);
    border-radius: 4px;
  }
  textarea::-webkit-scrollbar-thumb:hover {
    background: rgba(124, 58, 237, 0.4);
  }
</style>
