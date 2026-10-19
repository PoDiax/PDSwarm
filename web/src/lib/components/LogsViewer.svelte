<script lang="ts">
  import {
    onDestroy,
    tick,
    untrack } from 'svelte';
  import {
    FileText,
    Search,
    Download,
    Copy,
    Check,
    Trash,
    RefreshCw,
    ArrowDown,
    SlidersHorizontal,
    Wifi,
    WifiOff,
    CircleCheck,
    CircleAlert,
    Clock,
  } from '@lucide/svelte';
  import { getWebSocketUrl } from '../api';

  interface Props {
    containerId: string;
    containerName?: string;
  }

  type TimeFormat = 'short' | 'compact' | 'time' | 'datetime' | 'iso' | 'off';

  interface LogLine {
    id: number;
    timestamp: string | null;
    date: Date | null;
    raw: string;
    text: string;
    html: string;
  }

  let { containerId, containerName }: Props = $props();

  let logContainer: HTMLDivElement | null = $state(null);
  let ws: WebSocket | null = null;

  let lines = $state<LogLine[]>([]);
  let filterQuery = $state('');

  const savedRegex = typeof localStorage !== 'undefined' ? localStorage.getItem('pdswarm_logs_regex') : null;
  let isRegex = $state(savedRegex === 'true');

  function toggleRegex() {
    isRegex = !isRegex;
    try {
      localStorage.setItem('pdswarm_logs_regex', String(isRegex));
    } catch {
      // ignore
    }
  }

  const savedAutoScroll = typeof localStorage !== 'undefined' ? localStorage.getItem('pdswarm_logs_autoscroll') : null;
  let autoScroll = $state(savedAutoScroll !== null ? savedAutoScroll === 'true' : true);

  function toggleAutoScroll() {
    autoScroll = !autoScroll;
    try {
      localStorage.setItem('pdswarm_logs_autoscroll', String(autoScroll));
    } catch {
      // ignore
    }
    if (autoScroll) scrollToBottom();
  }

  const savedFormat = (typeof localStorage !== 'undefined'
    ? localStorage.getItem('pdswarm_logs_time_format')
    : null) as TimeFormat | null;
  const validFormats: TimeFormat[] = ['short', 'compact', 'time', 'datetime', 'iso', 'off'];
  let timeFormat = $state<TimeFormat>(
    savedFormat && validFormats.includes(savedFormat) ? savedFormat : 'short'
  );

  function setTimeFormat(fmt: TimeFormat) {
    timeFormat = fmt;
    try {
      localStorage.setItem('pdswarm_logs_time_format', fmt);
    } catch {
      // ignore
    }
  }

  const savedTail = typeof localStorage !== 'undefined' ? Number(localStorage.getItem('pdswarm_logs_tail')) : null;
  const validTails = [50, 100, 200, 500, 1000];
  let tailCount = $state<number>(savedTail && validTails.includes(savedTail) ? savedTail : 200);

  function setTailCount(val: number) {
    tailCount = val;
    try {
      localStorage.setItem('pdswarm_logs_tail', String(val));
    } catch {
      // ignore
    }
  }

  let connectionStatus = $state<'connecting' | 'connected' | 'disconnected' | 'error'>('connecting');
  let isCopied = $state(false);

  let nextId = 0;

  // ANSI to HTML parser
  const ansiColorMap: Record<number, string> = {
    30: 'text-slate-600',
    31: 'text-rose-400',
    32: 'text-emerald-400',
    33: 'text-amber-400',
    34: 'text-blue-400',
    35: 'text-purple-400',
    36: 'text-cyan-400',
    37: 'text-slate-100',
    90: 'text-slate-500',
    91: 'text-rose-300',
    92: 'text-emerald-300',
    93: 'text-amber-300',
    94: 'text-blue-300',
    95: 'text-purple-300',
    96: 'text-cyan-300',
    97: 'text-white font-bold',
  };

  function escapeHtml(str: string): string {
    return str
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#039;');
  }

  function ansiToHtml(ansiText: string): string {
    // Regex for ANSI escape codes: \x1b[...m
    const regex = /\x1b\[([0-9;]*)m/g;
    let result = '';
    let lastIndex = 0;
    let currentColorClass = '';
    let isBold = false;
    let isDim = false;

    let match: RegExpExecArray | null;
    while ((match = regex.exec(ansiText)) !== null) {
      const textChunk = ansiText.substring(lastIndex, match.index);
      if (textChunk) {
        const classes = [currentColorClass, isBold ? 'font-bold' : '', isDim ? 'opacity-70' : '']
          .filter(Boolean)
          .join(' ');
        result += classes ? `<span class="${classes}">${escapeHtml(textChunk)}</span>` : escapeHtml(textChunk);
      }

      const codes = match[1] ? match[1].split(';').map(Number) : [0];
      for (const code of codes) {
        if (code === 0) {
          currentColorClass = '';
          isBold = false;
          isDim = false;
        } else if (code === 1) {
          isBold = true;
        } else if (code === 2) {
          isDim = true;
        } else if (ansiColorMap[code]) {
          currentColorClass = ansiColorMap[code];
        }
      }

      lastIndex = regex.lastIndex;
    }

    const remaining = ansiText.substring(lastIndex);
    if (remaining) {
      const classes = [currentColorClass, isBold ? 'font-bold' : '', isDim ? 'opacity-70' : '']
        .filter(Boolean)
        .join(' ');
      result += classes ? `<span class="${classes}">${escapeHtml(remaining)}</span>` : escapeHtml(remaining);
    }

    return result;
  }

  function parseLine(rawLine: string): LogLine {
    const id = ++nextId;
    let timestamp: string | null = null;
    let date: Date | null = null;
    let content = rawLine;

    // Standard Docker logs format: 2026-10-15T17:46:00.123456789Z <message>
    const tsMatch = rawLine.match(/^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z?)\s*(.*)$/);
    if (tsMatch) {
      timestamp = tsMatch[1];
      content = tsMatch[2];
      try {
        const normalized = timestamp.replace(/\.(\d{3})\d+Z?$/, '.$1Z');
        const d = new Date(normalized);
        date = isNaN(d.getTime()) ? null : d;
      } catch {
        date = null;
      }
    }

    // Clean text without ANSI for search
    const cleanText = content.replace(/\x1b\[[0-9;]*m/g, '');
    const html = ansiToHtml(content);

    return {
      id,
      timestamp,
      date,
      raw: rawLine,
      text: cleanText,
      html,
    };
  }

  function formatTimestamp(line: LogLine, format: TimeFormat): string {
    if (format === 'off' || !line.timestamp) return '';
    if (format === 'iso') {
      return line.timestamp.replace(/(\.\d{3})\d+Z?$/, '$1Z');
    }
    const d = line.date;
    if (!d) return line.timestamp;

    const pad = (n: number) => (n < 10 ? '0' + n : String(n));
    const day = pad(d.getDate());
    const month = pad(d.getMonth() + 1);
    const hours = pad(d.getHours());
    const minutes = pad(d.getMinutes());

    if (format === 'short') {
      return `${day}/${month} ${hours}:${minutes}`;
    }
    const seconds = pad(d.getSeconds());
    if (format === 'compact') {
      return `${day}/${month} ${hours}:${minutes}:${seconds}`;
    }
    if (format === 'time') {
      return `${hours}:${minutes}:${seconds}`;
    }
    if (format === 'datetime') {
      return `${d.getFullYear()}-${month}-${day} ${hours}:${minutes}:${seconds}`;
    }
    return `${day}/${month} ${hours}:${minutes}`;
  }

  let isProgrammaticScroll = false;

  function connectLogs() {
    disconnectLogs();
    connectionStatus = 'connecting';

    const wsUrl = getWebSocketUrl(`/api/ws/containers/${containerId}/logs`, {
      tail: tailCount,
      follow: 'true',
      timestamps: 'true',
    });

    try {
      ws = new WebSocket(wsUrl);

      ws.onopen = () => {
        connectionStatus = 'connected';
      };

      ws.onmessage = async (event) => {
        let text = '';
        if (typeof event.data === 'string') {
          text = event.data;
        } else if (event.data instanceof ArrayBuffer) {
          text = new TextDecoder().decode(event.data);
        } else if (event.data instanceof Blob) {
          text = await event.data.text();
        } else {
          try {
            text = String(event.data);
          } catch {
            return;
          }
        }

        const incomingLines = text.split(/\r?\n/);
        const parsed = incomingLines
          .filter((l: string) => l.length > 0)
          .map((l: string) => parseLine(l));

        if (parsed.length > 0) {
          // Cap total lines at 5000 to prevent browser slowdown
          const newLines = [...lines, ...parsed];
          if (newLines.length > 5000) {
            lines = newLines.slice(newLines.length - 5000);
          } else {
            lines = newLines;
          }

          if (autoScroll) {
            await tick();
            scrollToBottom();
          }
        }
      };

      ws.onerror = () => {
        connectionStatus = 'error';
      };

      ws.onclose = () => {
        connectionStatus = 'disconnected';
      };
    } catch {
      connectionStatus = 'error';
    }
  }

  function disconnectLogs() {
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

  function scrollToBottom() {
    if (logContainer) {
      isProgrammaticScroll = true;
      logContainer.scrollTop = logContainer.scrollHeight;
      requestAnimationFrame(() => {
        isProgrammaticScroll = false;
      });
    }
  }

  function handleScroll() {
    if (!logContainer || isProgrammaticScroll) return;
    const threshold = 60;
    const isAtBottom = logContainer.scrollHeight - logContainer.scrollTop - logContainer.clientHeight < threshold;
    autoScroll = isAtBottom;
  }

  function clearLogs() {
    lines = [];
  }

  function copyLogs() {
    const text = filteredLines
      .map((l) => (timeFormat !== 'off' && l.timestamp ? `[${formatTimestamp(l, timeFormat)}] ${l.text}` : l.text))
      .join('\n');
    navigator.clipboard.writeText(text);
    isCopied = true;
    setTimeout(() => (isCopied = false), 2000);
  }

  function downloadLogs() {
    const text = filteredLines
      .map((l) => (timeFormat !== 'off' && l.timestamp ? `[${formatTimestamp(l, timeFormat)}] ${l.text}` : l.text))
      .join('\n');
    const blob = new Blob([text], { type: 'text/plain;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `${containerName || containerId.slice(0, 12)}-logs.log`;
    a.click();
    URL.revokeObjectURL(url);
  }

  // Regex compiler & validation (pure derived, no side-effects)
  let regexComputation = $derived.by<{ regex: RegExp | null; error: string | null }>(() => {
    if (!isRegex || !filterQuery.trim()) {
      return { regex: null, error: null };
    }
    try {
      return { regex: new RegExp(filterQuery, 'i'), error: null };
    } catch (err: unknown) {
      return {
        regex: null,
        error: err instanceof Error ? err.message : 'Invalid regex pattern',
      };
    }
  });

  let regexError = $derived(regexComputation.error);

  // Filtered log lines
  let filteredLines = $derived.by(() => {
    if (!filterQuery.trim()) return lines;

    if (isRegex) {
      if (regexComputation.regex) {
        const re = regexComputation.regex;
        return lines.filter((l) => {
          const formatted = timeFormat !== 'off' && l.timestamp ? formatTimestamp(l, timeFormat) : '';
          return (
            re.test(l.text) ||
            (l.timestamp && re.test(l.timestamp)) ||
            (formatted && re.test(formatted))
          );
        });
      }
      return lines;
    } else {
      const q = filterQuery.toLowerCase();
      return lines.filter((l) => {
        const formatted = timeFormat !== 'off' && l.timestamp ? formatTimestamp(l, timeFormat) : '';
        return (
          l.text.toLowerCase().includes(q) ||
          (l.timestamp && l.timestamp.toLowerCase().includes(q)) ||
          (formatted && formatted.toLowerCase().includes(q))
        );
      });
    }
  });

  $effect(() => {
    const id = containerId;
    if (id) {
      untrack(() => {
        lines = [];
        connectLogs();
      });
    }
    return () => {
      disconnectLogs();
    };
  });

  onDestroy(() => {
    disconnectLogs();
  });
</script>

<div class="rounded-2xl bg-[#0a0b10] border border-white/[0.08] overflow-hidden flex flex-col h-[580px] max-h-[750px] min-h-[420px]">
  <!-- Log Viewer Toolbar -->
  <div class="p-3 bg-[#0c0d14] border-b border-white/[0.08] flex flex-wrap items-center justify-between gap-3 select-none">
    <div class="flex items-center gap-3">
      <!-- Mac-style window dots -->
      <div class="flex items-center gap-1.5">
        <span class="w-3 h-3 rounded-full bg-rose-500/80 shadow-[0_0_6px_rgba(244,63,94,0.4)]"></span>
        <span class="w-3 h-3 rounded-full bg-amber-500/80 shadow-[0_0_6px_rgba(245,158,11,0.4)]"></span>
        <span class="w-3 h-3 rounded-full bg-emerald-500/80 shadow-[0_0_6px_rgba(16,185,129,0.4)]"></span>
      </div>

      <div class="flex items-center gap-2">
        <FileText class="w-3.5 h-3.5 text-cyan-400" />
        <span class="text-xs font-mono text-slate-300 font-semibold truncate max-w-[180px] sm:max-w-none">
          docker logs -f {containerName || containerId.slice(0, 12)}
        </span>
      </div>

      <!-- Live badge -->
      {#if connectionStatus === 'connected'}
        <span class="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[10px] font-mono bg-emerald-500/15 text-emerald-400 border border-emerald-500/30">
          <span class="w-1.5 h-1.5 rounded-full bg-emerald-400 shadow-[0_0_8px_#34d399] animate-pulse"></span>
          Streaming
        </span>
      {:else if connectionStatus === 'connecting'}
        <span class="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[10px] font-mono bg-amber-500/15 text-amber-400 border border-amber-500/30">
          <span class="w-1.5 h-1.5 rounded-full bg-amber-400 animate-ping"></span>
          Connecting...
        </span>
      {:else}
        <span class="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[10px] font-mono {lines.length > 0 ? 'bg-slate-800/80 text-slate-300 border-white/10' : 'bg-rose-500/15 text-rose-400 border-rose-500/30'}">
          <span class="w-1.5 h-1.5 rounded-full {lines.length > 0 ? 'bg-slate-400' : 'bg-rose-400'}"></span>
          {lines.length > 0 ? 'Stream Ended' : 'Disconnected'}
        </span>
      {/if}
    </div>

    <!-- Center Search & Filter Input -->
    <div class="flex items-center gap-2 flex-1 max-w-md mx-2">
      <div class="relative w-full">
        <Search class="w-3.5 h-3.5 text-slate-400 absolute left-3 top-2.5 pointer-events-none" />
        <input
          type="text"
          bind:value={filterQuery}
          placeholder="Filter logs or /regex/..."
          class="w-full pl-9 pr-14 py-1.5 rounded-xl bg-[#12131c] border border-white/[0.08] text-white text-xs font-mono focus:outline-none focus:border-cyan-400 transition"
        />
        <button
          type="button"
          onclick={toggleRegex}
          class="absolute right-2 top-1.5 px-1.5 py-0.5 rounded text-[10px] font-mono font-bold transition {isRegex
            ? 'bg-cyan-500 text-black shadow-[0_0_8px_rgba(6,182,212,0.4)]'
            : 'text-slate-500 hover:text-slate-300'}"
          title="Toggle regular expression search"
        >
          .*
        </button>
      </div>
    </div>

    <!-- Action Toolbar Buttons -->
    <div class="flex items-center gap-1.5">
      <!-- Tail count dropdown -->
      <select
        value={tailCount}
        onchange={(e) => {
          setTailCount(Number((e.currentTarget as HTMLSelectElement).value));
          connectLogs();
        }}
        class="bg-[#12131c] text-slate-300 text-[11px] font-mono px-2 py-1 rounded-lg border border-white/[0.08] focus:outline-none focus:border-cyan-400"
        title="Lines to tail on connect"
      >
        <option value={50}>Tail 50</option>
        <option value={100}>Tail 100</option>
        <option value={200}>Tail 200</option>
        <option value={500}>Tail 500</option>
        <option value={1000}>Tail 1000</option>
      </select>

      <!-- Time format selector -->
      <div class="relative flex items-center">
        <select
          value={timeFormat}
          onchange={(e) => setTimeFormat((e.currentTarget as HTMLSelectElement).value as TimeFormat)}
          class="bg-[#12131c] {timeFormat !== 'off'
            ? 'text-violet-300 border-violet-500/30 bg-violet-600/15'
            : 'text-slate-400 border-white/[0.08] hover:text-white'} text-[11px] font-mono pl-6 pr-2 py-1 rounded-lg border focus:outline-none focus:border-cyan-400 transition cursor-pointer"
          title="Switch timestamp format (e.g. 21/09 22:55)"
        >
          <option value="short" class="bg-[#12131c] text-slate-200">21/09 22:55 (Short)</option>
          <option value="compact" class="bg-[#12131c] text-slate-200">21/09 22:55:00 (Compact)</option>
          <option value="time" class="bg-[#12131c] text-slate-200">22:55:00 (Time only)</option>
          <option value="datetime" class="bg-[#12131c] text-slate-200">2026-09-21 22:55:00 (Full)</option>
          <option value="iso" class="bg-[#12131c] text-slate-200">ISO 8601 (Raw UTC)</option>
          <option value="off" class="bg-[#12131c] text-slate-400">Off (No time)</option>
        </select>
        <Clock class="w-3 h-3 {timeFormat !== 'off' ? 'text-violet-400' : 'text-slate-400'} absolute left-2 pointer-events-none" />
      </div>

      <!-- Auto-scroll lock toggle -->
      <button
        type="button"
        onclick={toggleAutoScroll}
        class="px-2 py-1 rounded-lg text-[11px] font-mono border transition flex items-center gap-1 {autoScroll
          ? 'bg-cyan-600/20 text-cyan-300 border-cyan-500/30'
          : 'bg-[#12131c] text-slate-400 border-white/[0.08] hover:text-white'}"
        title="Auto-scroll lock to bottom"
      >
        <ArrowDown class="w-3 h-3 {autoScroll ? 'text-cyan-400' : ''}" />
        <span class="hidden sm:inline">Scroll Lock</span>
      </button>

      <!-- Clear logs -->
      <button
        type="button"
        onclick={clearLogs}
        class="p-1.5 rounded-lg bg-[#12131c] hover:bg-white/10 text-slate-400 hover:text-white border border-white/[0.08] transition"
        title="Clear current view"
      >
        <Trash class="w-3.5 h-3.5" />
      </button>

      <!-- Copy logs -->
      <button
        type="button"
        onclick={copyLogs}
        class="p-1.5 rounded-lg bg-[#12131c] hover:bg-white/10 text-slate-400 hover:text-white border border-white/[0.08] transition"
        title="Copy logs to clipboard"
      >
        {#if isCopied}
          <Check class="w-3.5 h-3.5 text-emerald-400" />
        {:else}
          <Copy class="w-3.5 h-3.5" />
        {/if}
      </button>

      <!-- Download logs -->
      <button
        type="button"
        onclick={downloadLogs}
        class="p-1.5 rounded-lg bg-[#12131c] hover:bg-white/10 text-slate-400 hover:text-white border border-white/[0.08] transition"
        title="Download logs file"
      >
        <Download class="w-3.5 h-3.5" />
      </button>

      <!-- Reconnect logs -->
      <button
        type="button"
        onclick={() => connectLogs()}
        class="p-1.5 rounded-lg bg-[#12131c] hover:bg-white/10 text-slate-400 hover:text-cyan-400 border border-white/[0.08] transition"
        title="Reconnect log stream"
      >
        <RefreshCw class="w-3.5 h-3.5" />
      </button>
    </div>
  </div>

  {#if regexError}
    <div class="px-4 py-1.5 bg-rose-500/10 border-b border-rose-500/30 text-rose-300 text-xs font-mono flex items-center gap-2">
      <CircleAlert class="w-3.5 h-3.5 text-rose-400 shrink-0" />
      <span>{regexError}</span>
    </div>
  {/if}

  <!-- Log Content Window -->
  <div
    bind:this={logContainer}
    onscroll={handleScroll}
    class="flex-1 min-h-0 w-full p-4 bg-[#0a0b10] font-mono text-xs overflow-y-auto space-y-0.5 select-text leading-relaxed"
  >
    {#if filteredLines.length === 0}
      <div class="h-64 flex flex-col items-center justify-center text-slate-500 space-y-2">
        <FileText class="w-8 h-8 text-slate-600" />
        <p class="text-xs">
          {#if filterQuery}
            No log entries match "{filterQuery}"
          {:else if connectionStatus === 'connecting'}
            Establishing WebSocket connection to container logs...
          {:else}
            No logs captured yet from container.
          {/if}
        </p>
      </div>
    {:else}
      {#each filteredLines as line (line.id)}
        <div class="flex items-start gap-3 hover:bg-white/[0.02] py-0.5 px-1 rounded transition-colors group">
          <span class="text-[10px] text-slate-600 select-none w-10 text-right shrink-0 pt-0.5 font-mono">{line.id}</span>
          {#if timeFormat !== 'off' && line.timestamp}
            <span
              class="text-[11px] text-cyan-400/80 select-none shrink-0 pt-0.5 font-mono cursor-default hover:text-cyan-300 transition-colors"
              title="Full timestamp: {line.timestamp}"
            >
              [{formatTimestamp(line, timeFormat)}]
            </span>
          {/if}
          <div class="text-slate-300 break-all flex-1 whitespace-pre-wrap font-mono">
            {@html line.html}
          </div>
        </div>
      {/each}
    {/if}
  </div>

  <!-- Bottom Log Stats Bar -->
  <div class="px-4 py-1.5 bg-[#0a0b10] border-t border-white/[0.06] flex items-center justify-between text-[11px] font-mono text-slate-500">
    <div class="flex items-center gap-3">
      <span>Lines: <strong class="text-slate-300 font-bold">{lines.length}</strong></span>
      {#if filterQuery}
        <span>Matched: <strong class="text-cyan-400 font-bold">{filteredLines.length}</strong></span>
      {/if}
    </div>
    <div class="flex items-center gap-3">
      <span>Auto-scroll: <strong class={autoScroll ? 'text-emerald-400' : 'text-amber-400'}>{autoScroll ? 'Locked' : 'Paused'}</strong></span>
      <span>Stream: /api/ws/containers/{containerId.slice(0, 12)}/logs</span>
    </div>
  </div>
</div>
