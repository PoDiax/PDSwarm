<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import {
    Activity,
    Cpu,
    HardDrive,
    Network,
    RefreshCw,
    Sliders,
    Layers,
    Clock,
    Zap,
    TrendingUp,
  } from '@lucide/svelte';
  import { getWebSocketUrl } from '../api';
  import type { ContainerStatsPayload } from '../types';

  interface Props {
    containerId: string;
    containerName?: string;
  }

  interface TelemetryPoint {
    time: number;
    cpu: number;
    memoryMb: number;
    memoryPercent: number;
    networkRx: number;
    networkTx: number;
    blockRead: number;
    blockWrite: number;
    pids: number;
  }

  let { containerId, containerName }: Props = $props();

  const MAX_POINTS = 60; // 60 seconds rolling window

  let ws: WebSocket | null = null;
  let connectionStatus = $state<'connecting' | 'connected' | 'disconnected' | 'error'>('connecting');
  let currentStats = $state<ContainerStatsPayload | null>(null);
  let history = $state<TelemetryPoint[]>([]);

  // Rates (bytes per second)
  let netRxRate = $state<number>(0);
  let netTxRate = $state<number>(0);
  let blockReadRate = $state<number>(0);
  let blockWriteRate = $state<number>(0);

  function formatBytes(bytes: number, decimals = 1): string {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const dm = decimals < 0 ? 0 : decimals;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    const idx = Math.min(i, sizes.length - 1);
    return parseFloat((bytes / Math.pow(k, idx)).toFixed(dm)) + ' ' + sizes[idx];
  }

  function formatRate(bytesPerSec: number): string {
    return `${formatBytes(bytesPerSec)}/s`;
  }

  function connectStats() {
    disconnectStats();
    connectionStatus = 'connecting';

    const wsUrl = getWebSocketUrl(`/api/ws/containers/${containerId}/stats`);

    try {
      ws = new WebSocket(wsUrl);

      ws.onopen = () => {
        connectionStatus = 'connected';
      };

      ws.onmessage = (event) => {
        try {
          const data: ContainerStatsPayload = JSON.parse(event.data);
          const now = Date.now();

          // Calculate rates if previous data exists
          if (history.length > 0) {
            const prev = history[history.length - 1];
            const dtSec = Math.max(0.5, (now - prev.time) / 1000);
            netRxRate = Math.max(0, (data.network_rx - prev.networkRx) / dtSec);
            netTxRate = Math.max(0, (data.network_tx - prev.networkTx) / dtSec);
            blockReadRate = Math.max(0, (data.block_read - prev.blockRead) / dtSec);
            blockWriteRate = Math.max(0, (data.block_write - prev.blockWrite) / dtSec);
          }

          currentStats = data;

          const point: TelemetryPoint = {
            time: now,
            cpu: data.cpu_percent,
            memoryMb: data.memory_mb,
            memoryPercent: data.memory_percent,
            networkRx: data.network_rx,
            networkTx: data.network_tx,
            blockRead: data.block_read,
            blockWrite: data.block_write,
            pids: data.pids,
          };

          history = [...history.slice(-(MAX_POINTS - 1)), point];
        } catch {
          // Ignore invalid JSON frames
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

  function disconnectStats() {
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

  // CPU Chart Geometry
  let cpuStatsSummary = $derived.by(() => {
    if (history.length === 0) return { max: 100, current: 0, avg: 0, min: 0 };
    const values = history.map((p) => p.cpu);
    const current = values[values.length - 1];
    const maxVal = Math.max(...values);
    const minVal = Math.min(...values);
    const avg = values.reduce((a, b) => a + b, 0) / values.length;
    return {
      current,
      max: Math.max(10, Math.ceil(maxVal * 1.2)),
      min: minVal,
      avg,
    };
  });

  // Memory Chart Geometry
  let memStatsSummary = $derived.by(() => {
    if (history.length === 0) return { maxMb: 512, currentMb: 0, currentPercent: 0, avgMb: 0 };
    const values = history.map((p) => p.memoryMb);
    const currentMb = values[values.length - 1];
    const limitMb = currentStats?.memory_limit_mb || 512;
    const maxMb = Math.max(limitMb > 0 ? limitMb : 512, Math.max(...values) * 1.2);
    const avgMb = values.reduce((a, b) => a + b, 0) / values.length;
    return {
      currentMb,
      currentPercent: currentStats?.memory_percent ?? 0,
      limitMb,
      maxMb,
      avgMb,
    };
  });

  function buildChartPath(values: number[], maxY: number, width = 500, height = 130) {
    if (values.length < 2) return { line: '', area: '', lastX: 0, lastY: height / 2 };

    const topPad = 15;
    const bottomPad = 15;
    const usableHeight = height - topPad - bottomPad;

    const points = values.map((val, idx) => {
      const x = (idx / (MAX_POINTS - 1)) * width;
      const normalized = Math.min(1, Math.max(0, maxY > 0 ? val / maxY : 0));
      const y = height - bottomPad - normalized * usableHeight;
      return { x, y };
    });

    const linePath = points.reduce((acc, pt, i) => `${acc} ${i === 0 ? 'M' : 'L'} ${pt.x.toFixed(1)},${pt.y.toFixed(1)}`, '');
    const firstPt = points[0];
    const lastPt = points[points.length - 1];
    const areaPath = `${linePath} L ${lastPt.x.toFixed(1)},${height} L ${firstPt.x.toFixed(1)},${height} Z`;

    return {
      line: linePath,
      area: areaPath,
      lastX: lastPt.x,
      lastY: lastPt.y,
    };
  }

  let cpuPath = $derived(buildChartPath(history.map((p) => p.cpu), cpuStatsSummary.max));
  let memPath = $derived(buildChartPath(history.map((p) => p.memoryMb), memStatsSummary.maxMb));

  onMount(() => {
    connectStats();
  });

  onDestroy(() => {
    disconnectStats();
  });
</script>

<div class="space-y-6">
  <!-- Status & Quick Header Bar -->
  <div class="p-4 rounded-2xl bg-[#12131c] border border-white/[0.08] flex flex-wrap items-center justify-between gap-4">
    <div class="flex items-center gap-3">
      <div class="w-10 h-10 rounded-xl bg-violet-600/20 border border-violet-500/30 flex items-center justify-center text-violet-400">
        <Activity class="w-5 h-5" />
      </div>
      <div>
        <h3 class="text-sm font-bold text-white flex items-center gap-2">
          <span>Real-Time Container Telemetry</span>
          <span class="text-xs font-mono text-cyan-400 font-normal">({history.length}s history)</span>
        </h3>
        <p class="text-xs text-slate-400 font-mono">Live stats via /api/ws/containers/{containerId.slice(0, 12)}/stats</p>
      </div>
    </div>

    <div class="flex items-center gap-3">
      {#if connectionStatus === 'connected'}
        <span class="inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-mono bg-emerald-500/15 text-emerald-400 border border-emerald-500/30">
          <span class="w-2 h-2 rounded-full bg-emerald-400 shadow-[0_0_8px_#34d399] animate-pulse"></span>
          Live Stream
        </span>
      {:else if connectionStatus === 'connecting'}
        <span class="inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-mono bg-amber-500/15 text-amber-400 border border-amber-500/30">
          <span class="w-2 h-2 rounded-full bg-amber-400 animate-ping"></span>
          Connecting...
        </span>
      {:else}
        <span class="inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-mono bg-rose-500/15 text-rose-400 border border-rose-500/30">
          <span class="w-2 h-2 rounded-full bg-rose-400"></span>
          Disconnected
        </span>
      {/if}

      <button
        type="button"
        onclick={connectStats}
        class="p-2 rounded-xl bg-[#0a0b10] hover:bg-white/10 text-slate-400 hover:text-cyan-400 border border-white/[0.08] transition"
        title="Reconnect telemetry stream"
      >
        <RefreshCw class="w-4 h-4" />
      </button>
    </div>
  </div>

  <!-- Telemetry SVG Charts (CPU & Memory) -->
  <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
    <!-- CPU Chart Card -->
    <div class="p-6 rounded-2xl bg-[#12131c] border border-white/[0.08] space-y-4 shadow-[0_10px_30px_rgba(0,0,0,0.5)]">
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-2.5">
          <div class="w-8 h-8 rounded-xl bg-cyan-600/20 border border-cyan-500/30 flex items-center justify-center text-cyan-400">
            <Cpu class="w-4 h-4" />
          </div>
          <div>
            <h4 class="text-xs font-mono font-bold text-white uppercase tracking-wider">CPU Utilization</h4>
            <p class="text-[11px] font-mono text-slate-400">Core throttling and time slice</p>
          </div>
        </div>

        <div class="text-right">
          <span class="text-xl font-mono font-bold text-cyan-400 drop-shadow-[0_0_10px_rgba(6,182,212,0.4)]">
            {(currentStats?.cpu_percent ?? 0).toFixed(2)}%
          </span>
          <span class="text-[10px] font-mono text-slate-500 block">Avg: {cpuStatsSummary.avg.toFixed(1)}%</span>
        </div>
      </div>

      <!-- SVG Chart Canvas -->
      <div class="relative h-36 w-full bg-[#0a0b10] rounded-xl border border-white/[0.06] overflow-hidden p-2">
        <svg viewBox="0 0 500 130" preserveAspectRatio="none" class="w-full h-full overflow-visible">
          <defs>
            <linearGradient id="cyanAreaGradient" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stop-color="#00f5ff" stop-opacity="0.35" />
              <stop offset="80%" stop-color="#06b6d4" stop-opacity="0.05" />
              <stop offset="100%" stop-color="#06b6d4" stop-opacity="0" />
            </linearGradient>
            <filter id="cyanGlow" x="-20%" y="-20%" width="140%" height="140%">
              <feGaussianBlur stdDeviation="3" result="blur" />
              <feMerge>
                <feMergeNode in="blur" />
                <feMergeNode in="SourceGraphic" />
              </feMerge>
            </filter>
          </defs>

          <!-- Horizontal Grid Lines -->
          <line x1="0" y1="20" x2="500" y2="20" stroke="rgba(255,255,255,0.05)" stroke-dasharray="4 4" />
          <line x1="0" y1="65" x2="500" y2="65" stroke="rgba(255,255,255,0.05)" stroke-dasharray="4 4" />
          <line x1="0" y1="110" x2="500" y2="110" stroke="rgba(255,255,255,0.05)" stroke-dasharray="4 4" />

          {#if history.length >= 2}
            <!-- Area fill -->
            <path d={cpuPath.area} fill="url(#cyanAreaGradient)" />
            <!-- Glowing line -->
            <path
              d={cpuPath.line}
              fill="none"
              stroke="#00f5ff"
              stroke-width="2.5"
              stroke-linecap="round"
              stroke-linejoin="round"
              filter="url(#cyanGlow)"
            />
            <!-- Current Pulse Marker -->
            <circle cx={cpuPath.lastX} cy={cpuPath.lastY} r="4" fill="#00f5ff" class="animate-pulse" />
          {/if}
        </svg>

        <div class="absolute bottom-1 left-3 text-[10px] font-mono text-slate-500">60s ago</div>
        <div class="absolute bottom-1 right-3 text-[10px] font-mono text-cyan-400">Now</div>
      </div>

      <!-- Quick Metrics Row -->
      <div class="grid grid-cols-3 gap-2 text-center text-xs font-mono pt-1">
        <div class="p-2 rounded-xl bg-[#0a0b10] border border-white/[0.04]">
          <span class="text-[10px] text-slate-500 block uppercase">Min</span>
          <span class="text-slate-300 font-bold">{cpuStatsSummary.min.toFixed(1)}%</span>
        </div>
        <div class="p-2 rounded-xl bg-[#0a0b10] border border-white/[0.04]">
          <span class="text-[10px] text-slate-500 block uppercase">Peak</span>
          <span class="text-cyan-400 font-bold">{Math.max(...(history.map(p => p.cpu) || [0])).toFixed(1)}%</span>
        </div>
        <div class="p-2 rounded-xl bg-[#0a0b10] border border-white/[0.04]">
          <span class="text-[10px] text-slate-500 block uppercase">Scale</span>
          <span class="text-slate-400 font-bold">0 - {cpuStatsSummary.max}%</span>
        </div>
      </div>
    </div>

    <!-- Memory Chart Card -->
    <div class="p-6 rounded-2xl bg-[#12131c] border border-white/[0.08] space-y-4 shadow-[0_10px_30px_rgba(0,0,0,0.5)]">
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-2.5">
          <div class="w-8 h-8 rounded-xl bg-violet-600/20 border border-violet-500/30 flex items-center justify-center text-violet-400">
            <HardDrive class="w-4 h-4" />
          </div>
          <div>
            <h4 class="text-xs font-mono font-bold text-white uppercase tracking-wider">Memory Allocation</h4>
            <p class="text-[11px] font-mono text-slate-400">RSS resident and cache usage</p>
          </div>
        </div>

        <div class="text-right">
          <span class="text-xl font-mono font-bold text-violet-400 drop-shadow-[0_0_10px_rgba(168,85,247,0.4)]">
            {(currentStats?.memory_mb ?? 0).toFixed(1)} MB
          </span>
          <span class="text-[10px] font-mono text-slate-500 block">
            {(currentStats?.memory_percent ?? 0).toFixed(1)}% of limit
          </span>
        </div>
      </div>

      <!-- SVG Chart Canvas -->
      <div class="relative h-36 w-full bg-[#0a0b10] rounded-xl border border-white/[0.06] overflow-hidden p-2">
        <svg viewBox="0 0 500 130" preserveAspectRatio="none" class="w-full h-full overflow-visible">
          <defs>
            <linearGradient id="violetAreaGradient" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stop-color="#a855f7" stop-opacity="0.35" />
              <stop offset="80%" stop-color="#7c3aed" stop-opacity="0.05" />
              <stop offset="100%" stop-color="#7c3aed" stop-opacity="0" />
            </linearGradient>
            <filter id="violetGlow" x="-20%" y="-20%" width="140%" height="140%">
              <feGaussianBlur stdDeviation="3" result="blur" />
              <feMerge>
                <feMergeNode in="blur" />
                <feMergeNode in="SourceGraphic" />
              </feMerge>
            </filter>
          </defs>

          <!-- Horizontal Grid Lines -->
          <line x1="0" y1="20" x2="500" y2="20" stroke="rgba(255,255,255,0.05)" stroke-dasharray="4 4" />
          <line x1="0" y1="65" x2="500" y2="65" stroke="rgba(255,255,255,0.05)" stroke-dasharray="4 4" />
          <line x1="0" y1="110" x2="500" y2="110" stroke="rgba(255,255,255,0.05)" stroke-dasharray="4 4" />

          {#if history.length >= 2}
            <!-- Area fill -->
            <path d={memPath.area} fill="url(#violetAreaGradient)" />
            <!-- Glowing line -->
            <path
              d={memPath.line}
              fill="none"
              stroke="#c084fc"
              stroke-width="2.5"
              stroke-linecap="round"
              stroke-linejoin="round"
              filter="url(#violetGlow)"
            />
            <!-- Current Pulse Marker -->
            <circle cx={memPath.lastX} cy={memPath.lastY} r="4" fill="#c084fc" class="animate-pulse" />
          {/if}
        </svg>

        <div class="absolute bottom-1 left-3 text-[10px] font-mono text-slate-500">60s ago</div>
        <div class="absolute bottom-1 right-3 text-[10px] font-mono text-violet-400">Now</div>
      </div>

      <!-- Quick Metrics Row -->
      <div class="grid grid-cols-3 gap-2 text-center text-xs font-mono pt-1">
        <div class="p-2 rounded-xl bg-[#0a0b10] border border-white/[0.04]">
          <span class="text-[10px] text-slate-500 block uppercase">Limit</span>
          <span class="text-slate-300 font-bold">
            {currentStats?.memory_limit_mb ? `${Math.round(currentStats.memory_limit_mb)} MB` : 'Host'}
          </span>
        </div>
        <div class="p-2 rounded-xl bg-[#0a0b10] border border-white/[0.04]">
          <span class="text-[10px] text-slate-500 block uppercase">Usage %</span>
          <span class="text-violet-300 font-bold">{(currentStats?.memory_percent ?? 0).toFixed(1)}%</span>
        </div>
        <div class="p-2 rounded-xl bg-[#0a0b10] border border-white/[0.04]">
          <span class="text-[10px] text-slate-500 block uppercase">Raw</span>
          <span class="text-slate-400 font-bold">{formatBytes(currentStats?.memory_usage ?? 0)}</span>
        </div>
      </div>
    </div>
  </div>

  <!-- Live Network I/O & Block I/O Cards -->
  <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
    <!-- Network I/O Card -->
    <div class="p-6 rounded-2xl bg-[#12131c] border border-white/[0.08] space-y-4">
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-2.5">
          <div class="w-8 h-8 rounded-xl bg-emerald-600/20 border border-emerald-500/30 flex items-center justify-center text-emerald-400">
            <Network class="w-4 h-4" />
          </div>
          <div>
            <h4 class="text-xs font-mono font-bold text-white uppercase tracking-wider">Network Throughput</h4>
            <p class="text-[11px] font-mono text-slate-400">Real-time container network traffic</p>
          </div>
        </div>

        <div class="text-right">
          <span class="text-xs font-mono text-emerald-400 font-bold">
            Rx: {formatRate(netRxRate)}
          </span>
          <span class="text-[10px] font-mono text-cyan-400 block">
            Tx: {formatRate(netTxRate)}
          </span>
        </div>
      </div>

      <div class="grid grid-cols-2 gap-4 font-mono text-xs">
        <div class="p-3.5 rounded-xl bg-[#0a0b10] border border-white/[0.04] space-y-1">
          <span class="text-[10px] text-slate-500 block uppercase">Received (Rx)</span>
          <span class="text-emerald-400 font-bold text-base block">{formatBytes(currentStats?.network_rx ?? 0)}</span>
          <span class="text-[10px] text-slate-500 block">Speed: {formatRate(netRxRate)}</span>
        </div>

        <div class="p-3.5 rounded-xl bg-[#0a0b10] border border-white/[0.04] space-y-1">
          <span class="text-[10px] text-slate-500 block uppercase">Transmitted (Tx)</span>
          <span class="text-cyan-400 font-bold text-base block">{formatBytes(currentStats?.network_tx ?? 0)}</span>
          <span class="text-[10px] text-slate-500 block">Speed: {formatRate(netTxRate)}</span>
        </div>
      </div>
    </div>

    <!-- Disk Block I/O Card -->
    <div class="p-6 rounded-2xl bg-[#12131c] border border-white/[0.08] space-y-4">
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-2.5">
          <div class="w-8 h-8 rounded-xl bg-amber-600/20 border border-amber-500/30 flex items-center justify-center text-amber-400">
            <HardDrive class="w-4 h-4" />
          </div>
          <div>
            <h4 class="text-xs font-mono font-bold text-white uppercase tracking-wider">Disk & Storage I/O</h4>
            <p class="text-[11px] font-mono text-slate-400">Synchronous block operations</p>
          </div>
        </div>

        <div class="text-right">
          <span class="text-xs font-mono text-amber-400 font-bold">
            Read: {formatRate(blockReadRate)}
          </span>
          <span class="text-[10px] font-mono text-slate-300 block">
            Write: {formatRate(blockWriteRate)}
          </span>
        </div>
      </div>

      <div class="grid grid-cols-2 gap-4 font-mono text-xs">
        <div class="p-3.5 rounded-xl bg-[#0a0b10] border border-white/[0.04] space-y-1">
          <span class="text-[10px] text-slate-500 block uppercase">Disk Read</span>
          <span class="text-amber-400 font-bold text-base block">{formatBytes(currentStats?.block_read ?? 0)}</span>
          <span class="text-[10px] text-slate-500 block">Speed: {formatRate(blockReadRate)}</span>
        </div>

        <div class="p-3.5 rounded-xl bg-[#0a0b10] border border-white/[0.04] space-y-1">
          <span class="text-[10px] text-slate-500 block uppercase">Disk Write</span>
          <span class="text-slate-200 font-bold text-base block">{formatBytes(currentStats?.block_write ?? 0)}</span>
          <span class="text-[10px] text-slate-500 block">Speed: {formatRate(blockWriteRate)}</span>
        </div>
      </div>
    </div>
  </div>

  <!-- Additional Telemetry Counters -->
  <div class="p-4 rounded-2xl bg-[#12131c] border border-white/[0.08] grid grid-cols-2 sm:grid-cols-4 gap-4 font-mono text-xs">
    <div class="p-2 text-center">
      <span class="text-[10px] text-slate-500 uppercase block">Active PIDs</span>
      <span class="text-sm font-bold text-cyan-400">{currentStats?.pids ?? 0}</span>
    </div>
    <div class="p-2 text-center">
      <span class="text-[10px] text-slate-500 uppercase block">Samples Collected</span>
      <span class="text-sm font-bold text-white">{history.length}</span>
    </div>
    <div class="p-2 text-center">
      <span class="text-[10px] text-slate-500 uppercase block">Network Total</span>
      <span class="text-sm font-bold text-emerald-400">
        {formatBytes((currentStats?.network_rx ?? 0) + (currentStats?.network_tx ?? 0))}
      </span>
    </div>
    <div class="p-2 text-center">
      <span class="text-[10px] text-slate-500 uppercase block">I/O Operations Total</span>
      <span class="text-sm font-bold text-amber-400">
        {formatBytes((currentStats?.block_read ?? 0) + (currentStats?.block_write ?? 0))}
      </span>
    </div>
  </div>
</div>
