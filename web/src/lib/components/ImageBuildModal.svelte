<script lang="ts">
  import {
    tick,
    onDestroy } from 'svelte';
  import {
    X,
    Hammer,
    Sparkles,
    CircleCheck,
    CircleAlert,
    LoaderCircle,
    Terminal,
    ArrowDown,
    Copy,
    Check,
    RotateCcw,
    Square,
    Play,
    Tag,
    Layers,
    FileCode,
    Download,
    FolderOpen,
  } from '@lucide/svelte';
  import { imageApi } from '../api';
  import HostPathInput from './HostPathInput.svelte';

  interface Props {
    isOpen: boolean;
    onClose: () => void;
    onBuilt?: (imageTag: string) => void;
  }

  let {
    isOpen = false,
    onClose,
    onBuilt,
  }: Props = $props();

  interface Template {
    id: string;
    name: string;
    badge: string;
    tag: string;
    dockerfile: string;
  }

  const templates: Template[] = [
    {
      id: 'alpine',
      name: 'Minimal Alpine',
      badge: 'Lightweight',
      tag: 'pdswarm-alpine:latest',
      dockerfile: `FROM alpine:latest
RUN apk add --no-cache curl
CMD ["echo", "Hello from PDSwarm!"]
`,
    },
    {
      id: 'go',
      name: 'Go Static Binary',
      badge: 'Multi-stage',
      tag: 'pdswarm-go-app:latest',
      dockerfile: `FROM golang:1.24-alpine AS build
WORKDIR /src
RUN printf 'package main\\nimport "fmt"\\nfunc main(){ fmt.Println("Hello from Go in PDSwarm!") }\\n' > main.go
RUN CGO_ENABLED=0 go build -o /app main.go

FROM scratch
COPY --from=build /app /app
ENTRYPOINT ["/app"]
`,
    },
    {
      id: 'node',
      name: 'Node.js Web App',
      badge: 'Node 20',
      tag: 'pdswarm-node-app:latest',
      dockerfile: `FROM node:20-alpine
WORKDIR /app
RUN printf '{"name":"pdswarm-node","version":"1.0.0","scripts":{"start":"node server.js"}}\\n' > package.json
RUN printf 'const http = require("http");\\nconst server = http.createServer((req,res)=>{res.writeHead(200,{"Content-Type":"text/plain"});res.end("Hello from Node.js in PDSwarm!\\\\n");});\\nserver.listen(3000,()=>console.log("Listening on :3000"));\\n' > server.js
EXPOSE 3000
CMD ["node", "server.js"]
`,
    },
    {
      id: 'fastapi',
      name: 'Python FastAPI',
      badge: 'Python 3.12',
      tag: 'pdswarm-fastapi:latest',
      dockerfile: `FROM python:3.12-slim
WORKDIR /app
RUN pip install --no-cache-dir fastapi uvicorn
RUN printf 'from fastapi import FastAPI\\napp = FastAPI()\\n@app.get("/")\\ndef root(): return {"message": "Hello from FastAPI in PDSwarm!"}\\n' > main.py
EXPOSE 8000
CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8000"]
`,
    },
    {
      id: 'nginx',
      name: 'Nginx Static Site',
      badge: 'Web Server',
      tag: 'pdswarm-nginx:latest',
      dockerfile: `FROM nginx:alpine
RUN printf '<!DOCTYPE html><html><head><title>PDSwarm</title></head><body style="background:#12131c;color:#a78bfa;font-family:sans-serif;text-align:center;padding:50px;"><h1>Hello from Nginx in PDSwarm!</h1><p>Built with PDSwarm Dockerfile Studio</p></body></html>' > /usr/share/nginx/html/index.html
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
`,
    },
  ];

  // Component state
  let tag = $state('pdswarm-alpine:latest');
  let dockerfile = $state(templates[0].dockerfile);
  let activeTemplate = $state('alpine');
  let isBuilding = $state(false);
  let buildStatus = $state<'idle' | 'building' | 'success' | 'error'>('idle');
  let buildLogs = $state<string[]>([]);
  let errorMessage = $state<string | null>(null);
  let autoScroll = $state(true);
  let buildDuration = $state(0);
  let isCopied = $state(false);
  let isLogsCopied = $state(false);
  let contextDir = $state('');
  let downloadSuffix = $state('');

  // References
  let textareaRef = $state<HTMLTextAreaElement | null>(null);
  let gutterRef = $state<HTMLDivElement | null>(null);
  let terminalRef = $state<HTMLDivElement | null>(null);
  let abortController: AbortController | null = null;
  let timerInterval: ReturnType<typeof setInterval> | null = null;
  let rawLogsBuffer = '';

  // Derived metrics
  let lines = $derived(dockerfile.split('\n'));
  let lineCount = $derived(lines.length);
  let byteSizeFormatted = $derived.by(() => {
    const bytes = new Blob([dockerfile]).size;
    if (bytes < 1024) return `${bytes} B`;
    return `${(bytes / 1024).toFixed(1)} KB`;
  });

  onDestroy(() => {
    if (timerInterval) clearInterval(timerInterval);
    if (abortController) abortController.abort();
  });

  function selectTemplate(tmpl: Template) {
    if (isBuilding) return;
    activeTemplate = tmpl.id;
    dockerfile = tmpl.dockerfile;
    // Update tag if previous tag matched one of the template tags or is blank
    const isStandardTag = templates.some(t => t.tag === tag.trim()) || tag.trim() === '';
    if (isStandardTag) {
      tag = tmpl.tag;
    }
  }

  function handleScroll(e: Event) {
    const target = e.target as HTMLTextAreaElement;
    if (gutterRef) {
      gutterRef.scrollTop = target.scrollTop;
    }
  }

  function handleKeyDown(e: KeyboardEvent) {
    if (!textareaRef) return;

    if (e.key === 'Tab') {
      e.preventDefault();
      const start = textareaRef.selectionStart;
      const end = textareaRef.selectionEnd;

      if (start === end) {
        if (e.shiftKey) {
          const textBefore = dockerfile.slice(0, start);
          const lastNewline = textBefore.lastIndexOf('\n');
          const lineStart = lastNewline === -1 ? 0 : lastNewline + 1;
          if (dockerfile.slice(lineStart, lineStart + 2) === '  ') {
            dockerfile = dockerfile.slice(0, lineStart) + dockerfile.slice(lineStart + 2);
            tick().then(() => {
              if (textareaRef) {
                const newPos = Math.max(lineStart, start - 2);
                textareaRef.setSelectionRange(newPos, newPos);
              }
            });
          }
        } else {
          dockerfile = dockerfile.substring(0, start) + '  ' + dockerfile.substring(end);
          tick().then(() => {
            if (textareaRef) {
              textareaRef.selectionStart = textareaRef.selectionEnd = start + 2;
            }
          });
        }
      } else {
        const textBefore = dockerfile.slice(0, start);
        const lineStart = textBefore.lastIndexOf('\n') === -1 ? 0 : textBefore.lastIndexOf('\n') + 1;
        const selected = dockerfile.slice(lineStart, end);
        const linesArr = selected.split('\n');

        if (e.shiftKey) {
          const unindented = linesArr.map(l => l.startsWith('  ') ? l.slice(2) : (l.startsWith(' ') ? l.slice(1) : l)).join('\n');
          dockerfile = dockerfile.slice(0, lineStart) + unindented + dockerfile.slice(end);
        } else {
          const indented = linesArr.map(l => '  ' + l).join('\n');
          dockerfile = dockerfile.slice(0, lineStart) + indented + dockerfile.slice(end);
        }
      }
    }
  }

  function handleLogChunk(chunk: string) {
    rawLogsBuffer += chunk;
    const parts = rawLogsBuffer.split('\n');
    rawLogsBuffer = parts.pop() || '';
    for (const part of parts) {
      const trimmed = part.trimEnd();
      if (trimmed !== '') {
        buildLogs = [...buildLogs, trimmed];
      }
    }
    if (autoScroll && terminalRef) {
      tick().then(() => {
        if (terminalRef) terminalRef.scrollTop = terminalRef.scrollHeight;
      });
    }
  }

  async function startBuild() {
    const trimmedTag = tag.trim();
    const trimmedDockerfile = dockerfile.trim();

    if (!trimmedTag) {
      errorMessage = 'Image tag is required (e.g. my-app:latest).';
      return;
    }
    if (!trimmedDockerfile) {
      errorMessage = 'Dockerfile content cannot be empty.';
      return;
    }

    isBuilding = true;
    buildStatus = 'building';
    errorMessage = null;
    buildLogs = [];
    rawLogsBuffer = '';
    buildDuration = 0;
    abortController = new AbortController();

    timerInterval = setInterval(() => {
      buildDuration += 1;
    }, 1000);

    buildLogs = [
      `[PDSwarm] Initiating build for tag: ${trimmedTag}`,
      contextDir.trim()
        ? `[PDSwarm] Including local build context directory: ${contextDir.trim()}`
        : `[PDSwarm] Packing Dockerfile context (${byteSizeFormatted}, ${lineCount} lines)...`,
    ];

    try {
      await imageApi.build(
        trimmedTag,
        trimmedDockerfile,
        (chunk) => {
          handleLogChunk(chunk);
        },
        abortController.signal,
        contextDir.trim() || undefined
      );

      if (rawLogsBuffer.trim()) {
        buildLogs = [...buildLogs, rawLogsBuffer.trim()];
        rawLogsBuffer = '';
      }

      buildStatus = 'success';
      buildLogs = [
        ...buildLogs,
        `✓ [PDSwarm] Successfully built and tagged image: ${trimmedTag}`,
      ];

      if (onBuilt) {
        onBuilt(trimmedTag);
      }
    } catch (err: unknown) {
      if (rawLogsBuffer.trim()) {
        buildLogs = [...buildLogs, rawLogsBuffer.trim()];
        rawLogsBuffer = '';
      }
      buildStatus = 'error';

      if (err instanceof Error) {
        if (err.name === 'AbortError') {
          errorMessage = 'Build aborted by user.';
          buildLogs = [...buildLogs, '✖ [PDSwarm] Build cancelled by user.'];
        } else {
          errorMessage = err.message;
          buildLogs = [...buildLogs, `✖ [PDSwarm Error] ${err.message}`];
        }
      } else {
        errorMessage = 'Image build failed with an unknown error.';
        buildLogs = [...buildLogs, '✖ [PDSwarm Error] An unknown error occurred.'];
      }
    } finally {
      isBuilding = false;
      if (timerInterval) {
        clearInterval(timerInterval);
        timerInterval = null;
      }
      abortController = null;
      if (autoScroll && terminalRef) {
        tick().then(() => {
          if (terminalRef) terminalRef.scrollTop = terminalRef.scrollHeight;
        });
      }
    }
  }

  function cancelBuild() {
    if (abortController) {
      abortController.abort();
      abortController = null;
    }
  }

  function handleClose() {
    if (isBuilding) {
      if (!confirm('An image build is actively running. Do you want to abort the build and close?')) {
        return;
      }
      cancelBuild();
    }
    onClose();
  }

  function downloadDockerfile() {
    const rawSuffix = downloadSuffix.trim().replace(/^\.+/, '');
    const filename = rawSuffix ? `Dockerfile.${rawSuffix}` : 'Dockerfile';
    const blob = new Blob([dockerfile], { type: 'text/plain;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = filename;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    URL.revokeObjectURL(url);
  }

  async function copyDockerfile() {
    try {
      await navigator.clipboard.writeText(dockerfile);
      isCopied = true;
      setTimeout(() => { isCopied = false; }, 2000);
    } catch {
      // ignore
    }
  }

  async function copyLogs() {
    try {
      await navigator.clipboard.writeText(buildLogs.join('\n'));
      isLogsCopied = true;
      setTimeout(() => { isLogsCopied = false; }, 2000);
    } catch {
      // ignore
    }
  }

  function clearLogs() {
    if (isBuilding) return;
    buildLogs = [];
    buildStatus = 'idle';
    errorMessage = null;
  }
</script>

{#if isOpen}
  <div
    class="fixed inset-0 z-50 flex items-center justify-center p-2 sm:p-5"
    role="dialog"
    aria-modal="true"
    aria-labelledby="image-builder-modal-title"
    tabindex="-1"
  >
    <!-- Backdrop -->
    <button
      type="button"
      class="fixed inset-0 bg-black/80 backdrop-blur-md cursor-default border-0"
      onclick={handleClose}
      aria-label="Close modal backdrop"
      tabindex="-1"
    ></button>

    <!-- Modal Container -->
    <div
      class="relative w-full max-w-6xl h-[92vh] max-h-[900px] flex flex-col rounded-2xl bg-[#12131c] border border-white/[0.12] shadow-[0_25px_80px_rgba(0,0,0,0.85)] overflow-hidden animate-in fade-in zoom-in-95 duration-150 z-10"
    >
      <!-- Modal Header -->
      <div class="px-5 py-3.5 border-b border-white/[0.08] bg-[#161824] flex flex-wrap items-center justify-between gap-3 shrink-0">
        <div class="flex items-center gap-3 min-w-0">
          <div class="w-9 h-9 rounded-xl bg-violet-500/15 border border-violet-500/30 flex items-center justify-center text-violet-400 shadow-[0_0_15px_rgba(124,58,237,0.3)] shrink-0">
            <Hammer class="w-4 h-4" />
          </div>
          <div class="min-w-0">
            <div class="flex items-center gap-2">
              <h2 id="image-builder-modal-title" class="text-sm font-bold text-white tracking-wide">
                Dockerfile Studio
              </h2>
              <span class="px-2 py-0.5 rounded-full text-[10px] font-mono font-semibold bg-violet-500/15 text-violet-300 border border-violet-500/30">
                Image Builder
              </span>
            </div>
            <p class="text-[11px] text-slate-400 truncate">
              Write custom Dockerfiles and build streaming Docker images directly in browser
            </p>
          </div>
        </div>

        <!-- Tag input and Build Action Button -->
        <div class="flex items-center gap-2.5 flex-1 max-w-md justify-end">
          <div class="relative flex-1 min-w-[180px] max-w-[260px]">
            <div class="absolute inset-y-0 left-0 pl-2.5 flex items-center pointer-events-none text-slate-500">
              <Tag class="w-3.5 h-3.5 text-violet-400" />
            </div>
            <input
              type="text"
              bind:value={tag}
              disabled={isBuilding}
              placeholder="e.g. my-app:latest"
              class="w-full bg-[#0d0e17] border border-white/[0.12] focus:border-violet-500/70 rounded-xl pl-8 pr-3 py-1.5 text-xs font-mono text-violet-200 placeholder-slate-600 outline-none transition disabled:opacity-50"
              title="Destination image tag (e.g. repo:tag)"
            />
          </div>

          {#if isBuilding}
            <button
              type="button"
              onclick={cancelBuild}
              class="px-3.5 py-1.5 rounded-xl bg-rose-600/20 hover:bg-rose-600/30 text-rose-300 border border-rose-500/30 text-xs font-medium transition flex items-center gap-1.5 shrink-0"
              title="Abort current build"
            >
              <Square class="w-3 h-3 fill-rose-400" />
              <span>Cancel</span>
            </button>
          {:else}
            <button
              type="button"
              onclick={startBuild}
              class="px-4 py-1.5 rounded-xl bg-violet-600 hover:bg-violet-500 text-white text-xs font-semibold shadow-[0_0_20px_rgba(124,58,237,0.4)] transition flex items-center gap-1.5 shrink-0"
            >
              <Play class="w-3.5 h-3.5 fill-white" />
              <span>Build Image</span>
            </button>
          {/if}

          <button
            type="button"
            onclick={handleClose}
            class="p-1.5 rounded-xl bg-white/[0.04] hover:bg-rose-500/20 text-slate-400 hover:text-rose-300 border border-white/[0.08] transition shrink-0"
            title="Close modal"
          >
            <X class="w-4 h-4" />
          </button>
        </div>
      </div>

      <!-- Template Selector Chips -->
      <div class="px-5 py-2.5 bg-[#12131c] border-b border-white/[0.06] flex items-center gap-2 overflow-x-auto shrink-0 scrollbar-none">
        <span class="text-[11px] font-medium text-slate-400 uppercase tracking-wider shrink-0 mr-1 flex items-center gap-1">
          <Sparkles class="w-3 h-3 text-cyan-400" />
          Templates:
        </span>
        {#each templates as tmpl}
          <button
            type="button"
            onclick={() => selectTemplate(tmpl)}
            disabled={isBuilding}
            class="px-2.5 py-1 rounded-lg text-xs font-medium transition flex items-center gap-1.5 shrink-0 border {activeTemplate === tmpl.id
              ? 'bg-violet-500/20 text-violet-200 border-violet-500/40 shadow-[0_0_12px_rgba(124,58,237,0.25)]'
              : 'bg-white/[0.03] hover:bg-white/[0.06] text-slate-300 border-white/[0.06]'}"
          >
            <span>{tmpl.name}</span>
            <span class="text-[9px] px-1 py-0.2 rounded bg-black/30 text-slate-400 font-mono">
              {tmpl.badge}
            </span>
          </button>
        {/each}
      </div>

      <!-- Build Context Directory (Optional) -->
      <div class="px-5 py-2 bg-[#0e1018] border-b border-white/[0.06] flex flex-wrap items-center justify-between gap-3 shrink-0">
        <div class="flex items-center gap-2 flex-1 min-w-[280px]">
          <FolderOpen class="w-3.5 h-3.5 text-cyan-400 shrink-0" />
          <span class="text-xs font-medium text-slate-300 shrink-0">Context Directory:</span>
          <div class="flex-1 max-w-lg">
            <HostPathInput
              bind:value={contextDir}
              placeholder="Host directory path for COPY/ADD files (e.g. /opt/myapp/src, optional)"
              disabled={isBuilding}
            />
          </div>
          {#if contextDir}
            <button
              type="button"
              onclick={() => (contextDir = '')}
              disabled={isBuilding}
              class="text-[11px] text-slate-400 hover:text-white px-1.5 py-0.5 rounded hover:bg-white/[0.06] transition"
              title="Clear context directory"
            >
              Clear
            </button>
          {/if}
        </div>
        <span class="text-[11px] text-slate-400 font-mono hidden md:inline">
          {contextDir.trim() ? 'Local files from host included in build context' : 'Optional: point to host dir to pull files for build'}
        </span>
      </div>

      <!-- Error / Alert Banner -->
      {#if errorMessage}
        <div class="px-5 py-2 bg-rose-500/10 border-b border-rose-500/25 flex items-center justify-between text-xs text-rose-300 shrink-0">
          <div class="flex items-center gap-2 truncate">
            <CircleAlert class="w-4 h-4 text-rose-400 shrink-0" />
            <span class="truncate">{errorMessage}</span>
          </div>
          <button
            type="button"
            onclick={() => errorMessage = null}
            class="text-rose-400 hover:text-white text-xs shrink-0 ml-2"
          >
            Dismiss
          </button>
        </div>
      {/if}

      <!-- Main Workspace: Split Editor & Terminal -->
      <div class="flex-1 min-h-0 grid grid-cols-1 lg:grid-cols-2 divide-y lg:divide-y-0 lg:divide-x divide-white/[0.08] overflow-hidden">
        
        <!-- Left Column: Dockerfile Studio Editor -->
        <div class="flex flex-col min-h-0 bg-[#0a0b10]">
          <!-- Editor Sub-Header -->
          <div class="px-4 py-2 border-b border-white/[0.06] bg-[#0e1018] flex items-center justify-between text-xs shrink-0">
            <div class="flex items-center gap-2 text-slate-300">
              <FileCode class="w-3.5 h-3.5 text-cyan-400" />
              <span class="font-mono font-medium text-cyan-200">Dockerfile</span>
              <span class="text-[10px] font-mono text-slate-500">
                ({lineCount} lines, {byteSizeFormatted})
              </span>
            </div>
            <div class="flex items-center gap-2">
              <!-- Download Dockerfile with optional name suffix -->
              <div class="flex items-center rounded-lg bg-[#07080d] border border-white/[0.08] px-2 py-0.5" title="Optional file extension suffix (e.g. 'prod' produces Dockerfile.prod)">
                <span class="text-[10px] font-mono text-slate-500 select-none">Dockerfile</span>
                {#if downloadSuffix.trim()}
                  <span class="text-[10px] font-mono text-violet-400 select-none">.</span>
                {/if}
                <input
                  type="text"
                  bind:value={downloadSuffix}
                  placeholder="(suffix)"
                  class="w-14 bg-transparent text-[10px] font-mono text-cyan-300 placeholder-slate-600 outline-none px-1"
                />
              </div>

              <button
                type="button"
                onclick={downloadDockerfile}
                class="px-2 py-1 rounded bg-white/[0.04] hover:bg-white/[0.08] text-slate-300 hover:text-white text-[11px] font-mono transition flex items-center gap-1 border border-white/[0.06]"
                title="Download Dockerfile with optional suffix"
              >
                <Download class="w-3 h-3 text-cyan-400" />
                <span>Download</span>
              </button>

              <button
                type="button"
                onclick={copyDockerfile}
                class="px-2 py-1 rounded bg-white/[0.04] hover:bg-white/[0.08] text-slate-300 hover:text-white text-[11px] font-mono transition flex items-center gap-1 border border-white/[0.06]"
                title="Copy Dockerfile to clipboard"
              >
                {#if isCopied}
                  <Check class="w-3 h-3 text-emerald-400" />
                  <span class="text-emerald-400">Copied</span>
                {:else}
                  <Copy class="w-3 h-3" />
                  <span>Copy</span>
                {/if}
              </button>
              <button
                type="button"
                onclick={() => { dockerfile = ''; }}
                disabled={isBuilding}
                class="px-2 py-1 rounded bg-white/[0.04] hover:bg-rose-500/20 text-slate-400 hover:text-rose-300 text-[11px] font-mono transition border border-white/[0.06] disabled:opacity-40"
                title="Clear editor"
              >
                Clear
              </button>
            </div>
          </div>

          <!-- Code Editor Workspace with Line Numbers Gutter -->
          <div class="relative flex-1 flex overflow-hidden">
            <!-- Gutter -->
            <div
              bind:this={gutterRef}
              class="w-11 sm:w-12 select-none overflow-hidden bg-[#07080d] border-r border-white/[0.06] py-3 text-right pr-2.5 font-mono text-xs leading-6 text-slate-600 shrink-0"
              aria-hidden="true"
            >
              {#each lines as _, idx}
                <div>{idx + 1}</div>
              {/each}
            </div>

            <!-- Textarea -->
            <textarea
              bind:this={textareaRef}
              bind:value={dockerfile}
              onscroll={handleScroll}
              onkeydown={handleKeyDown}
              disabled={isBuilding}
              spellcheck="false"
              placeholder="# Write Dockerfile instructions here...&#10;FROM alpine:latest&#10;CMD [&quot;echo&quot;, &quot;hello&quot;]"
              class="flex-1 h-full w-full bg-transparent p-3 font-mono text-xs sm:text-[13px] leading-6 text-slate-200 placeholder-slate-700 outline-none resize-none overflow-y-auto selection:bg-violet-600/40 selection:text-white disabled:opacity-60"
            ></textarea>
          </div>
        </div>

        <!-- Right Column: Live Build Terminal Console -->
        <div class="flex flex-col min-h-0 bg-[#08090e]">
          <!-- Terminal Sub-Header -->
          <div class="px-4 py-2 border-b border-white/[0.06] bg-[#0c0d15] flex items-center justify-between text-xs shrink-0">
            <div class="flex items-center gap-2">
              <Terminal class="w-3.5 h-3.5 text-violet-400" />
              <span class="font-mono font-medium text-slate-200">Build Console</span>

              <!-- Status Badge -->
              {#if buildStatus === 'building'}
                <span class="px-2 py-0.5 rounded-full text-[10px] font-mono font-semibold bg-violet-500/20 text-violet-300 border border-violet-500/30 flex items-center gap-1.5 animate-pulse">
                  <LoaderCircle class="w-2.5 h-2.5 animate-spin" />
                  Building ({buildDuration}s)
                </span>
              {:else if buildStatus === 'success'}
                <span class="px-2 py-0.5 rounded-full text-[10px] font-mono font-semibold bg-emerald-500/15 text-emerald-300 border border-emerald-500/30 flex items-center gap-1">
                  <CircleCheck class="w-2.5 h-2.5 text-emerald-400" />
                  Success ({buildDuration}s)
                </span>
              {:else if buildStatus === 'error'}
                <span class="px-2 py-0.5 rounded-full text-[10px] font-mono font-semibold bg-rose-500/15 text-rose-300 border border-rose-500/30 flex items-center gap-1">
                  <CircleAlert class="w-2.5 h-2.5 text-rose-400" />
                  Failed
                </span>
              {:else}
                <span class="px-2 py-0.5 rounded-full text-[10px] font-mono text-slate-500 bg-white/[0.03] border border-white/[0.06]">
                  Ready
                </span>
              {/if}
            </div>

            <!-- Terminal Tools -->
            <div class="flex items-center gap-2">
              <button
                type="button"
                onclick={() => autoScroll = !autoScroll}
                class="px-2 py-1 rounded text-[11px] font-mono transition flex items-center gap-1 border {autoScroll
                  ? 'bg-cyan-500/15 text-cyan-300 border-cyan-500/30'
                  : 'bg-white/[0.04] text-slate-400 border-white/[0.06]'}"
                title="Toggle auto-scroll to bottom"
              >
                <ArrowDown class="w-3 h-3 {autoScroll ? 'text-cyan-400' : ''}" />
                <span class="hidden sm:inline">Auto-scroll</span>
              </button>

              <button
                type="button"
                onclick={copyLogs}
                disabled={buildLogs.length === 0}
                class="px-2 py-1 rounded bg-white/[0.04] hover:bg-white/[0.08] text-slate-300 hover:text-white text-[11px] font-mono transition flex items-center gap-1 border border-white/[0.06] disabled:opacity-40"
                title="Copy logs"
              >
                {#if isLogsCopied}
                  <Check class="w-3 h-3 text-emerald-400" />
                  <span class="text-emerald-400">Copied</span>
                {:else}
                  <Copy class="w-3 h-3" />
                  <span>Copy</span>
                {/if}
              </button>

              <button
                type="button"
                onclick={clearLogs}
                disabled={isBuilding || buildLogs.length === 0}
                class="px-2 py-1 rounded bg-white/[0.04] hover:bg-white/[0.08] text-slate-400 hover:text-white text-[11px] font-mono transition border border-white/[0.06] disabled:opacity-40"
                title="Clear console"
              >
                <RotateCcw class="w-3 h-3" />
              </button>
            </div>
          </div>

          <!-- Terminal Output Box -->
          <div
            bind:this={terminalRef}
            class="flex-1 min-h-0 p-4 font-mono text-xs leading-5 overflow-y-auto space-y-1 select-text scrollbar-thin scrollbar-thumb-white/10"
          >
            {#if buildLogs.length === 0}
              <div class="h-full flex flex-col items-center justify-center text-center p-6 text-slate-600 select-none">
                <Terminal class="w-10 h-10 text-slate-700 mb-2 stroke-[1.2]" />
                <p class="text-xs font-mono text-slate-400">Ready to build image</p>
                <p class="text-[11px] text-slate-600 max-w-xs mt-1">
                  Click "Build Image" above to submit the Dockerfile to the Docker daemon and stream live step output.
                </p>
              </div>
            {:else}
              {#each buildLogs as logLine}
                {#if logLine.startsWith('Step ') || logLine.startsWith('[PDSwarm]')}
                  <div class="text-cyan-300 font-semibold py-0.5 border-b border-cyan-500/10">
                    {logLine}
                  </div>
                {:else if logLine.startsWith('--->')}
                  <div class="text-slate-500 pl-2">
                    {logLine}
                  </div>
                {:else if logLine.includes('Successfully built') || logLine.includes('Successfully tagged') || logLine.startsWith('✓')}
                  <div class="text-emerald-300 font-semibold bg-emerald-500/10 px-2 py-1 rounded border border-emerald-500/25 my-1">
                    {logLine}
                  </div>
                {:else if logLine.toLowerCase().includes('error') || logLine.toLowerCase().includes('failed') || logLine.startsWith('✖')}
                  <div class="text-rose-300 font-medium bg-rose-500/10 px-2 py-0.5 rounded border-l-2 border-rose-500 my-0.5">
                    {logLine}
                  </div>
                {:else}
                  <div class="text-slate-300 break-words">
                    {logLine}
                  </div>
                {/if}
              {/each}
            {/if}
          </div>
        </div>
      </div>

      <!-- Modal Footer -->
      <div class="px-5 py-2.5 bg-[#0e1018] border-t border-white/[0.08] flex items-center justify-between text-xs text-slate-500 shrink-0">
        <div class="flex items-center gap-2 text-[11px]">
          <span class="inline-block w-1.5 h-1.5 rounded-full bg-violet-500"></span>
          <span>Tip: Tab indents 2 spaces. Multi-stage builds are fully supported.</span>
        </div>
        <div class="flex items-center gap-3 font-mono text-[11px] text-slate-400">
          <span>Target: <strong class="text-violet-300">{tag || 'none'}</strong></span>
        </div>
      </div>
    </div>
  </div>
{/if}
