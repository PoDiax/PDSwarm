<script lang="ts">
  import {
    onMount,
    onDestroy } from 'svelte';
  import { Terminal } from '@xterm/xterm';
  import { FitAddon } from '@xterm/addon-fit';
  import { WebLinksAddon } from '@xterm/addon-web-links';
  import '@xterm/xterm/css/xterm.css';
  import {
    Terminal as TerminalIcon,
    RefreshCw,
    Trash,
    Maximize2,
    Minimize2,
    Wifi,
    WifiOff,
    Check,
    Copy,
    CornerDownLeft,
  } from '@lucide/svelte';
  import { getWebSocketUrl } from '../api';

  interface Props {
    containerId: string;
    containerName?: string;
  }

  let { containerId, containerName }: Props = $props();

  let terminalContainer: HTMLDivElement | null = $state(null);
  let term: Terminal | null = null;
  let fitAddon: FitAddon | null = null;
  let ws: WebSocket | null = null;
  let resizeObserver: ResizeObserver | null = null;

  let connectionStatus = $state<'connecting' | 'connected' | 'disconnected' | 'error'>('connecting');
  let statusMessage = $state<string>('Initializing terminal...');
  let isFullscreen = $state(false);
  let selectedShell = $state<string>(
    typeof window !== 'undefined' ? localStorage.getItem('pdswarm_default_shell') || '/bin/sh' : '/bin/sh'
  );
  let isCopied = $state(false);

  const midnightTheme = {
    background: '#0a0b10',
    foreground: '#f8fafc',
    cursor: '#00f5ff',
    cursorAccent: '#0a0b10',
    selectionBackground: 'rgba(124, 58, 237, 0.4)',
    black: '#12131c',
    red: '#f43f5e',
    green: '#10b981',
    yellow: '#f59e0b',
    blue: '#3b82f6',
    magenta: '#a855f7',
    cyan: '#06b6d4',
    white: '#f8fafc',
    brightBlack: '#475569',
    brightRed: '#fb7185',
    brightGreen: '#34d399',
    brightYellow: '#fbbf24',
    brightBlue: '#60a5fa',
    brightMagenta: '#c084fc',
    brightCyan: '#22d3ee',
    brightWhite: '#ffffff',
  };

  let lastCols = 0;
  let lastRows = 0;
  let resizeTimeout: ReturnType<typeof setTimeout> | null = null;

  function sendResize(cols: number, rows: number) {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: 'resize', cols, rows }));
    }
  }

  function handleResize() {
    if (!fitAddon || !term || !terminalContainer) return;
    try {
      fitAddon.fit();
      if (term.cols > 0 && term.rows > 0) {
        if (term.cols !== lastCols || term.rows !== lastRows) {
          lastCols = term.cols;
          lastRows = term.rows;
          sendResize(term.cols, term.rows);
        }
      }
    } catch {
      // Ignore fit errors during transitions
    }
  }

  function queueResize() {
    if (resizeTimeout) clearTimeout(resizeTimeout);
    resizeTimeout = setTimeout(() => {
      handleResize();
    }, 100);
  }

  function initTerminal() {
    if (!terminalContainer) return;

    // Teardown existing instance if any
    cleanup();

    term = new Terminal({
      cursorBlink: true,
      cursorStyle: 'block',
      fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace',
      fontSize: 13,
      lineHeight: 1.25,
      theme: midnightTheme,
      allowTransparency: true,
      convertEol: true,
      scrollback: 5000,
    });

    fitAddon = new FitAddon();
    const webLinksAddon = new WebLinksAddon();

    term.loadAddon(fitAddon);
    term.loadAddon(webLinksAddon);

    term.open(terminalContainer);
    fitAddon.fit();
    lastCols = term.cols;
    lastRows = term.rows;

    connectWebSocket();

    // Bi-directional keystroke routing
    term.onData((data) => {
      if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(data);
      }
    });

    // Resize observer on container element with debounce to prevent layout feedback loops
    if (typeof window !== 'undefined' && window.ResizeObserver && terminalContainer) {
      resizeObserver = new ResizeObserver(() => {
        queueResize();
      });
      resizeObserver.observe(terminalContainer);
    }
  }

  function connectWebSocket() {
    connectionStatus = 'connecting';
    statusMessage = 'Connecting to container shell...';

    const wsUrl = getWebSocketUrl(`/api/ws/containers/${containerId}/exec`, {
      cmd: selectedShell,
      cols: term ? term.cols : 80,
      rows: term ? term.rows : 24,
    });

    try {
      ws = new WebSocket(wsUrl);
      ws.binaryType = 'arraybuffer';

      ws.onopen = () => {
        connectionStatus = 'connected';
        statusMessage = 'Connected to exec session';
        if (term && fitAddon) {
          fitAddon.fit();
          sendResize(term.cols, term.rows);
          term.focus();
        }
      };

      ws.onmessage = (event) => {
        if (!term) return;
        if (typeof event.data === 'string') {
          term.write(event.data);
        } else if (event.data instanceof ArrayBuffer) {
          term.write(new Uint8Array(event.data));
        } else if (event.data instanceof Blob) {
          event.data.arrayBuffer().then((buf) => {
            term?.write(new Uint8Array(buf));
          });
        }
      };

      ws.onerror = () => {
        connectionStatus = 'error';
        statusMessage = 'WebSocket connection error';
        term?.write('\r\n\x1b[31m[PDSwarm] Connection error occurred.\x1b[0m\r\n');
      };

      ws.onclose = (event) => {
        connectionStatus = 'disconnected';
        statusMessage = event.wasClean
          ? 'Terminal session terminated'
          : `Session closed (${event.code})`;
        term?.write('\r\n\x1b[33m[PDSwarm] Container exec session disconnected.\x1b[0m\r\n');
      };
    } catch (err: unknown) {
      connectionStatus = 'error';
      statusMessage = err instanceof Error ? err.message : 'Failed to establish WebSocket';
    }
  }

  function clearTerminal() {
    term?.clear();
    term?.focus();
  }

  function reconnect() {
    cleanupWebSocket();
    term?.write('\r\n\x1b[36m[PDSwarm] Reconnecting...\x1b[0m\r\n');
    connectWebSocket();
  }

  function cleanupWebSocket() {
    if (ws) {
      ws.onopen = null;
      ws.onmessage = null;
      ws.onerror = null;
      ws.onclose = null;
      try {
        ws.close();
      } catch {
        // ignore
      }
      ws = null;
    }
  }

  function cleanup() {
    if (resizeObserver) {
      resizeObserver.disconnect();
      resizeObserver = null;
    }
    cleanupWebSocket();
    if (term) {
      try {
        term.dispose();
      } catch {
        // ignore
      }
      term = null;
    }
    fitAddon = null;
  }

  function copySelection() {
    if (!term) return;
    const sel = term.getSelection();
    if (sel) {
      navigator.clipboard.writeText(sel);
      isCopied = true;
      setTimeout(() => (isCopied = false), 2000);
    }
  }

  function toggleFullscreen() {
    isFullscreen = !isFullscreen;
    setTimeout(() => {
      handleResize();
      term?.focus();
    }, 120);
  }

  onMount(() => {
    initTerminal();
    window.addEventListener('resize', queueResize);
  });

  onDestroy(() => {
    if (typeof window !== 'undefined') {
      window.removeEventListener('resize', queueResize);
    }
    if (resizeTimeout) {
      clearTimeout(resizeTimeout);
      resizeTimeout = null;
    }
    cleanup();
  });
</script>

<div
  class="rounded-2xl bg-[#0a0b10] border border-white/[0.08] overflow-hidden flex flex-col transition-all {isFullscreen
    ? 'fixed inset-4 z-50 shadow-[0_20px_70px_rgba(0,0,0,0.95)] h-[calc(100vh-2rem)]'
    : 'h-[580px] max-h-[750px] min-h-[420px]'}"
>
  <!-- Window Header Bar -->
  <div class="px-4 py-2.5 bg-[#0c0d14] border-b border-white/[0.08] flex flex-wrap items-center justify-between gap-3 select-none">
    <div class="flex items-center gap-3">
      <!-- Mac-style window controls -->
      <div class="flex items-center gap-1.5">
        <span class="w-3 h-3 rounded-full bg-rose-500/80 shadow-[0_0_6px_rgba(244,63,94,0.4)]"></span>
        <span class="w-3 h-3 rounded-full bg-amber-500/80 shadow-[0_0_6px_rgba(245,158,11,0.4)]"></span>
        <span class="w-3 h-3 rounded-full bg-emerald-500/80 shadow-[0_0_6px_rgba(16,185,129,0.4)]"></span>
      </div>

      <div class="flex items-center gap-2">
        <TerminalIcon class="w-3.5 h-3.5 text-cyan-400" />
        <span class="text-xs font-mono text-slate-300 font-semibold truncate max-w-[200px] sm:max-w-none">
          {containerName ? `${containerName} (${selectedShell})` : `exec -it ${containerId.slice(0, 12)}`}
        </span>
      </div>
    </div>

    <!-- Status Badge & Quick Actions -->
    <div class="flex items-center gap-2">
      <!-- Connection Status Badge -->
      {#if connectionStatus === 'connected'}
        <span class="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[10px] font-mono bg-emerald-500/15 text-emerald-400 border border-emerald-500/30">
          <span class="w-1.5 h-1.5 rounded-full bg-emerald-400 shadow-[0_0_8px_#34d399] animate-pulse"></span>
          Connected
        </span>
      {:else if connectionStatus === 'connecting'}
        <span class="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[10px] font-mono bg-amber-500/15 text-amber-400 border border-amber-500/30">
          <span class="w-1.5 h-1.5 rounded-full bg-amber-400 animate-ping"></span>
          Connecting...
        </span>
      {:else}
        <span class="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[10px] font-mono bg-rose-500/15 text-rose-400 border border-rose-500/30">
          <span class="w-1.5 h-1.5 rounded-full bg-rose-400"></span>
          Disconnected
        </span>
      {/if}

      <!-- Shell selection -->
      <select
        bind:value={selectedShell}
        onchange={() => {
          reconnect();
        }}
        class="bg-[#12131c] text-slate-300 text-[11px] font-mono px-2 py-1 rounded-lg border border-white/[0.08] focus:outline-none focus:border-cyan-400"
      >
        <option value="/bin/sh">/bin/sh</option>
        <option value="/bin/bash">/bin/bash</option>
        <option value="/bin/zsh">/bin/zsh</option>
        <option value="sh">sh</option>
        <option value="bash">bash</option>
      </select>

      <!-- Clear Terminal -->
      <button
        type="button"
        onclick={clearTerminal}
        class="p-1.5 rounded-lg bg-[#12131c] hover:bg-white/10 text-slate-400 hover:text-white border border-white/[0.08] transition"
        title="Clear terminal screen"
      >
        <Trash class="w-3.5 h-3.5" />
      </button>

      <!-- Reconnect -->
      <button
        type="button"
        onclick={reconnect}
        class="p-1.5 rounded-lg bg-[#12131c] hover:bg-white/10 text-slate-400 hover:text-cyan-400 border border-white/[0.08] transition"
        title="Reconnect terminal session"
      >
        <RefreshCw class="w-3.5 h-3.5" />
      </button>

      <!-- Copy Selected -->
      <button
        type="button"
        onclick={copySelection}
        class="p-1.5 rounded-lg bg-[#12131c] hover:bg-white/10 text-slate-400 hover:text-white border border-white/[0.08] transition"
        title="Copy selected text"
      >
        {#if isCopied}
          <Check class="w-3.5 h-3.5 text-emerald-400" />
        {:else}
          <Copy class="w-3.5 h-3.5" />
        {/if}
      </button>

      <!-- Fullscreen toggle -->
      <button
        type="button"
        onclick={toggleFullscreen}
        class="p-1.5 rounded-lg bg-[#12131c] hover:bg-white/10 text-slate-400 hover:text-white border border-white/[0.08] transition"
        title={isFullscreen ? 'Exit fullscreen' : 'Enter fullscreen'}
      >
        {#if isFullscreen}
          <Minimize2 class="w-3.5 h-3.5" />
        {:else}
          <Maximize2 class="w-3.5 h-3.5" />
        {/if}
      </button>
    </div>
  </div>

  <!-- Terminal Display Frame -->
  <div
    bind:this={terminalContainer}
    class="flex-1 w-full min-h-0 h-full bg-[#0a0b10] p-3 overflow-hidden cursor-text relative"
    onclick={() => term?.focus()}
    onkeydown={(e) => {
      if (e.key === 'Enter') term?.focus();
    }}
    role="button"
    tabindex="0"
    aria-label="Interactive Terminal Output"
  ></div>

  <!-- Bottom Info Bar -->
  <div class="px-4 py-1.5 bg-[#0a0b10] border-t border-white/[0.06] flex items-center justify-between text-[11px] font-mono text-slate-500">
    <div class="flex items-center gap-2">
      <span class="inline-block w-1.5 h-1.5 rounded-full bg-cyan-400 shadow-[0_0_6px_#00f5ff]"></span>
      <span class="text-slate-400">{statusMessage}</span>
    </div>
    <div class="flex items-center gap-3">
      <span>Theme: Midnight</span>
      <span>UTF-8 / PTY</span>
    </div>
  </div>
</div>

<style>
  :global(.xterm) {
    height: 100% !important;
    width: 100% !important;
    padding: 2px;
  }
  :global(.xterm-screen) {
    height: 100% !important;
  }
  :global(.xterm .xterm-viewport) {
    overflow-y: auto !important;
  }
</style>
