<script lang="ts">
  import {
    onMount } from "svelte";
  import {
    Activity,
    Box,
    Boxes,
    Layers,
    Disc,
    HardDrive,
    Network,
    RefreshCw,
    Plus,
    Play,
    Download,
    Cpu,
    Server,
    Zap,
    CircleCheck,
    CircleAlert,
    ExternalLink,
    ShieldCheck,
    Rocket,
    Terminal,
    ChevronDown,
    ChevronUp,
    X,
    LoaderCircle,
    ArrowRight,
    Trash,
  } from '@lucide/svelte';
  import MetricCard from "../lib/components/MetricCard.svelte";
  import Gauge from "../lib/components/Gauge.svelte";
  import ActiveTasksCard from "../lib/components/ActiveTasksCard.svelte";
  import {
    systemApi,
    containerApi,
    imageApi,
    volumeApi,
    networkApi,
    stackApi,
  } from "../lib/api";
  import { deploymentStore } from "../lib/stores/deployments.svelte";
  import type {
    SystemInfo,
    Container,
    Image,
    Volume,
    Network as DockerNetwork,
    Stack,
  } from "../lib/types";

  interface Props {
    onNavigate?: (tab: string) => void;
  }

  let { onNavigate }: Props = $props();

  let isLoading = $state(true);
  let isRefreshing = $state(false);
  let error = $state<string | null>(null);
  let lastUpdated = $state<Date | null>(null);

  let systemInfo = $state<SystemInfo | null>(null);
  let containers = $state<Container[]>([]);
  let images = $state<Image[]>([]);
  let volumes = $state<Volume[]>([]);
  let networks = $state<DockerNetwork[]>([]);
  let stacks = $state<Stack[]>([]);

  function formatBytes(bytes: number | undefined): string {
    if (!bytes || isNaN(bytes) || bytes === 0) return "0 B";
    const k = 1024;
    const sizes = ["B", "KB", "MB", "GB", "TB"];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + " " + sizes[i];
  }

  async function fetchDashboardData(silent = false) {
    if (!silent) isRefreshing = true;
    error = null;

    try {
      const [infoRes, contRes, imgRes, volRes, netRes, stackRes] =
        await Promise.allSettled([
          systemApi.getInfo(),
          containerApi.list(true),
          imageApi.list(),
          volumeApi.list(),
          networkApi.list(),
          stackApi.list(),
        ]);

      if (infoRes.status === "fulfilled") {
        systemInfo = infoRes.value;
      }
      if (contRes.status === "fulfilled") {
        containers = contRes.value || [];
      }
      if (imgRes.status === "fulfilled") {
        images = imgRes.value || [];
      }
      if (volRes.status === "fulfilled") {
        volumes = volRes.value?.Volumes || [];
      }
      if (netRes.status === "fulfilled") {
        networks = netRes.value || [];
      }
      if (stackRes.status === "fulfilled") {
        stacks = stackRes.value || [];
      }

      lastUpdated = new Date();
    } catch (err: unknown) {
      if (err instanceof Error) {
        error = err.message;
      } else {
        error = "Failed to load telemetry data from host.";
      }
    } finally {
      isLoading = false;
      isRefreshing = false;
    }
  }

  onMount(() => {
    fetchDashboardData();
  });

  // Derived telemetry metrics
  const osName = $derived(
    systemInfo?.os || systemInfo?.OperatingSystem || "Linux Host",
  );
  const kernelVersion = $derived(
    systemInfo?.kernel_version || systemInfo?.KernelVersion || "6.8+",
  );
  const engineType = $derived(
    systemInfo?.engine_type ||
      ((systemInfo?.ServerVersion || "").toLowerCase().includes("podman") ||
      (systemInfo?.server_version || "").toLowerCase().includes("podman")
        ? "podman"
        : "docker"),
  );
  const isPodman = $derived(engineType === "podman");
  const engineName = $derived(
    systemInfo?.engine_name || (isPodman ? "Podman" : "Docker Engine"),
  );
  const dockerVersion = $derived(
    systemInfo?.server_version ||
      systemInfo?.ServerVersion ||
      (isPodman ? "Podman Engine" : "Docker Engine"),
  );
  const engineVersion = $derived(
    systemInfo?.engine_version ||
      systemInfo?.server_version ||
      systemInfo?.ServerVersion ||
      "",
  );
  const engineDisplay = $derived.by(() => {
    const name = engineName;
    let ver = engineVersion.trim();
    if (ver.toLowerCase().startsWith(name.toLowerCase())) {
      ver = ver.slice(name.length).trim();
    }
    return ver ? `${name} ${ver}` : name;
  });
  const storageDriver = $derived(
    systemInfo?.Driver || systemInfo?.driver || "overlay2",
  );
  const memoryLimitSupported = $derived(
    systemInfo?.MemoryLimit ?? systemInfo?.memory_limit ?? null,
  );
  const swapLimitSupported = $derived(
    systemInfo?.SwapLimit ?? systemInfo?.swap_limit ?? null,
  );
  const rootDir = $derived(
    systemInfo?.DockerRootDir ||
      systemInfo?.docker_root_dir ||
      (isPodman ? "Podman Storage" : "/var/lib/docker"),
  );
  const arch = $derived(systemInfo?.Architecture || "x86_64");
  const cpuCount = $derived(systemInfo?.cpus || systemInfo?.NCPU || 4);
  const memTotal = $derived(
    systemInfo?.memory_bytes || systemInfo?.MemTotal || 16 * 1024 * 1024 * 1024,
  );

  const totalContainers = $derived(
    systemInfo?.containers ?? systemInfo?.Containers ?? containers.length,
  );
  const runningContainers = $derived(
    containers.filter((c) => String(c.State).toLowerCase() === "running")
      .length ||
      (systemInfo?.containers_running ?? systemInfo?.ContainersRunning ?? 0),
  );
  const stoppedContainers = $derived(
    containers.filter(
      (c) =>
        String(c.State).toLowerCase() === "exited" ||
        String(c.State).toLowerCase() === "stopped",
    ).length ||
      (systemInfo?.containers_stopped ?? systemInfo?.ContainersStopped ?? 0),
  );
  const pausedContainers = $derived(
    containers.filter((c) => String(c.State).toLowerCase() === "paused")
      .length ||
      (systemInfo?.containers_paused ?? systemInfo?.ContainersPaused ?? 0),
  );

  const totalImages = $derived(
    images.length || (systemInfo?.images ?? systemInfo?.Images ?? 0),
  );
  const totalImageSize = $derived(
    images.reduce((sum, img) => sum + (img.Size || 0), 0),
  );
  const totalVolumes = $derived(volumes.length);
  const totalVolumeSize = $derived(
    volumes.reduce(
      (sum, v) =>
        sum +
        (v.UsageData?.Size && v.UsageData.Size > 0 ? v.UsageData.Size : 0),
      0,
    ),
  );
  const totalNetworks = $derived(networks.length);
  const totalStacks = $derived(stacks.length);
  const activeStacks = $derived(
    stacks.filter((s) => s.status === "running" || s.status === "deployed")
      .length,
  );

  // Utilization calculation for CPU & RAM meters
  const cpuUtilization = $derived.by(() => {
    if (systemInfo?.cpu_percent !== undefined && systemInfo.cpu_percent >= 0) {
      return Math.min(100, Math.round(systemInfo.cpu_percent));
    }
    return runningContainers > 0
      ? Math.min(95, Math.round((runningContainers / (cpuCount * 2)) * 100) + 5)
      : 0;
  });

  const ramUtilization = $derived.by(() => {
    if (
      systemInfo?.memory_percent !== undefined &&
      systemInfo.memory_percent > 0
    ) {
      return Math.min(100, Math.round(systemInfo.memory_percent));
    }
    if (systemInfo?.memory_used_bytes && memTotal > 0) {
      return Math.min(
        100,
        Math.round((systemInfo.memory_used_bytes / memTotal) * 100),
      );
    }
    return runningContainers > 0
      ? Math.min(
          92,
          Math.round(
            ((runningContainers * 384 * 1024 * 1024 + 1024 * 1024 * 1024) /
              memTotal) *
              100,
          ),
        )
      : 0;
  });

  const usedMemBytes = $derived.by(() => {
    if (systemInfo?.memory_used_bytes && systemInfo.memory_used_bytes > 0) {
      return systemInfo.memory_used_bytes;
    }
    return Math.round((ramUtilization / 100) * memTotal);
  });

  function navigateTo(tab: string) {
    if (onNavigate) {
      onNavigate(tab);
    }
  }

  let expandedLogs = $state<Record<string, boolean>>({});

  function toggleTaskLogs(id: string) {
    expandedLogs[id] = !(expandedLogs[id] ?? false);
  }
</script>

<div class="space-y-6 animate-in fade-in duration-200">
  <!-- System Telemetry Header & Quick Navigation -->
  <div
    class="flex flex-col lg:flex-row lg:items-center lg:justify-between gap-4 pb-1"
  >
    <div>
      <div class="flex items-center gap-3">
        <h1
          class="text-2xl font-black tracking-tight text-white flex items-center gap-2.5"
        >
          Host Overview
        </h1>
      </div>

      <!-- Host spec bar -->
      <div
        class="flex flex-wrap items-center gap-x-3 gap-y-1 mt-2 text-xs text-slate-400 font-mono"
      >
        <span class="text-slate-200 font-semibold">{dockerVersion}</span>
        <span class="text-slate-600">•</span>
        <span>{osName}</span>
        <span class="text-slate-600">•</span>
        <span>Kernel {kernelVersion}</span>
        <span class="text-slate-600">•</span>
        <span>{arch}</span>
      </div>
    </div>

    <!-- Quick Action Navigation Buttons & Refresh -->
    <div class="flex flex-wrap items-center gap-2.5">
      <button
        type="button"
        onclick={() => fetchDashboardData()}
        disabled={isRefreshing}
        class="flex items-center gap-1.5 px-3 py-1.5 rounded-xl bg-[#181a26] hover:bg-[#202334] border border-white/10 text-xs font-medium text-slate-200 hover:text-white transition disabled:opacity-50"
        title="Refresh Telemetry"
      >
        <RefreshCw
          class="w-3.5 h-3.5 text-cyan-400 {isRefreshing ? 'animate-spin' : ''}"
        />
        <span>{isRefreshing ? "Syncing..." : "Refresh"}</span>
      </button>

      <button
        type="button"
        onclick={() => navigateTo("stacks")}
        class="flex items-center gap-1.5 px-3.5 py-1.5 rounded-xl bg-[#181a26] hover:bg-[#202334] border border-violet-500/30 text-xs font-medium text-violet-300 hover:text-violet-200 shadow-[0_0_12px_rgba(124,58,237,0.2)] transition"
      >
        <Layers class="w-3.5 h-3.5 text-violet-400" />
        <span>Deploy Stack</span>
      </button>

      <button
        type="button"
        onclick={() => navigateTo("templates")}
        class="flex items-center gap-1.5 px-3.5 py-1.5 rounded-xl bg-[#181a26] hover:bg-[#202334] border border-emerald-500/30 text-xs font-medium text-emerald-300 hover:text-emerald-200 shadow-[0_0_12px_rgba(16,185,129,0.2)] transition cursor-pointer"
      >
        <Boxes class="w-3.5 h-3.5 text-emerald-400" />
        <span>App Store</span>
      </button>

      <button
        type="button"
        onclick={() => navigateTo("containers")}
        class="flex items-center gap-1.5 px-3.5 py-1.5 rounded-xl bg-violet-600 hover:bg-violet-500 text-white text-xs font-semibold shadow-[0_0_15px_rgba(124,58,237,0.4)] transition"
      >
        <Plus class="w-3.5 h-3.5" />
        <span>Launch Container</span>
      </button>

      <button
        type="button"
        onclick={() => navigateTo("images")}
        class="flex items-center gap-1.5 px-3 py-1.5 rounded-xl bg-[#181a26] hover:bg-[#202334] border border-cyan-500/30 text-xs font-medium text-cyan-300 hover:text-cyan-200 shadow-[0_0_12px_rgba(6,182,212,0.2)] transition"
      >
        <Download class="w-3.5 h-3.5 text-cyan-400" />
        <span>Pull Image</span>
      </button>
    </div>
  </div>

  {#if error}
    <div
      class="p-4 rounded-2xl bg-rose-500/10 border border-rose-500/30 flex items-center justify-between text-rose-300 text-xs"
    >
      <div class="flex items-center gap-2">
        <CircleAlert class="w-4 h-4 text-rose-400 shrink-0" />
        <span>{error}</span>
      </div>
      <button
        type="button"
        onclick={() => fetchDashboardData()}
        class="underline font-semibold hover:text-white"
      >
        Retry
      </button>
    </div>
  {/if}

  <!-- Active Operations / Background Tasks Card -->
  <ActiveTasksCard />

  <!-- Active Deployments & Background Tasks Section -->
  {#if deploymentStore.allTasks.length > 0}
    <div class="space-y-3 animate-in fade-in slide-in-from-top-2 duration-200">
      <div class="flex items-center justify-between px-1">
        <div class="flex items-center gap-2.5">
          <div
            class="w-7 h-7 rounded-lg bg-violet-600/20 border border-violet-500/30 flex items-center justify-center text-violet-400"
          >
            <Rocket class="w-4 h-4" />
          </div>
          <div>
            <div class="flex items-center gap-2">
              <h2 class="text-sm font-bold text-white tracking-wide">
                Application Deployments
              </h2>
              {#if deploymentStore.activeCount > 0}
                <span
                  class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-mono bg-violet-500/20 text-violet-300 border border-violet-500/30"
                >
                  <span
                    class="w-1.5 h-1.5 rounded-full bg-violet-400 animate-pulse"
                  ></span>
                  {deploymentStore.activeCount} deploying
                </span>
              {/if}
            </div>
            <p class="text-[11px] text-slate-400">
              Background Compose stack creation and image sync tasks
            </p>
          </div>
        </div>

        {#if deploymentStore.allTasks.some((t) => t.status !== "deploying")}
          <button
            type="button"
            onclick={() => deploymentStore.clearFinished()}
            class="flex items-center gap-1.5 px-2.5 py-1 rounded-lg text-[11px] font-medium bg-white/[0.04] hover:bg-white/[0.08] text-slate-400 hover:text-slate-200 border border-white/[0.06] transition cursor-pointer"
          >
            <Trash class="w-3 h-3" />
            <span>Clear Finished</span>
          </button>
        {/if}
      </div>

      <div class="space-y-3">
        {#each deploymentStore.allTasks as task (task.id)}
          {@const isExpanded =
            expandedLogs[task.id] ?? task.status === "deploying"}
          <div
            class="rounded-2xl bg-[#12131c] border transition-all duration-200 overflow-hidden {task.status ===
            'deploying'
              ? 'border-violet-500/40 shadow-[0_0_20px_rgba(124,58,237,0.15)]'
              : task.status === 'success'
                ? 'border-emerald-500/30'
                : 'border-red-500/30'}"
          >
            <!-- Card Header -->
            <div
              class="p-4 sm:p-5 flex flex-col sm:flex-row sm:items-center justify-between gap-3.5 bg-[#181a26]/40"
            >
              <div class="flex items-center gap-3.5 min-w-0">
                <!-- App Logo / Letter Avatar -->
                <div class="relative shrink-0">
                  {#if task.templateLogo}
                    <div
                      class="w-11 h-11 rounded-xl bg-[#0a0b10] border border-white/[0.08] p-1.5 flex items-center justify-center overflow-hidden"
                    >
                      <img
                        src={task.templateLogo}
                        alt={task.templateTitle}
                        class="w-full h-full object-contain"
                      />
                    </div>
                  {:else}
                    <div
                      class="w-11 h-11 rounded-xl bg-gradient-to-br from-violet-600/30 to-indigo-600/20 border border-violet-500/40 text-violet-300 font-mono font-bold text-base flex items-center justify-center"
                    >
                      {(task.templateTitle || task.stackName)
                        .charAt(0)
                        .toUpperCase()}
                    </div>
                  {/if}
                </div>

                <div class="min-w-0">
                  <div class="flex items-center gap-2">
                    <h3 class="text-sm font-bold text-white truncate">
                      {task.stackName}
                    </h3>
                    <span class="text-xs text-slate-400 font-mono"
                      >({task.templateTitle})</span
                    >
                  </div>

                  <div class="flex items-center gap-2 mt-1">
                    {#if task.status === "deploying"}
                      <span
                        class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded text-[10px] font-mono font-semibold uppercase tracking-wider bg-violet-500/20 text-violet-300 border border-violet-500/30"
                      >
                        <LoaderCircle class="w-3 h-3 animate-spin text-violet-400" />
                        <span>{task.step}</span>
                      </span>
                      <span class="text-xs text-slate-300 font-mono truncate"
                        >{task.currentStatus}</span
                      >
                    {:else if task.status === "success"}
                      <span
                        class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded text-[10px] font-mono font-semibold uppercase tracking-wider bg-emerald-500/20 text-emerald-300 border border-emerald-500/30"
                      >
                        <CircleCheck class="w-3 h-3 text-emerald-400" />
                        <span>Deployed</span>
                      </span>
                      <span class="text-xs text-emerald-300 font-mono"
                        >Application running</span
                      >
                    {:else}
                      <span
                        class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded text-[10px] font-mono font-semibold uppercase tracking-wider bg-red-500/20 text-red-300 border border-red-500/30"
                      >
                        <CircleAlert class="w-3 h-3 text-red-400" />
                        <span>Failed</span>
                      </span>
                      <span class="text-xs text-red-300 font-mono truncate"
                        >{task.error || "Deployment error"}</span
                      >
                    {/if}
                  </div>
                </div>
              </div>

              <!-- Action buttons on card -->
              <div
                class="flex items-center gap-2 self-end sm:self-center shrink-0"
              >
                <button
                  type="button"
                  onclick={() => toggleTaskLogs(task.id)}
                  class="flex items-center gap-1.5 px-3 py-1.5 rounded-xl text-xs font-mono bg-white/[0.04] hover:bg-white/[0.08] text-slate-300 border border-white/[0.06] transition cursor-pointer"
                >
                  <Terminal class="w-3.5 h-3.5 text-violet-400" />
                  <span>Logs ({task.logs.length})</span>
                  {#if isExpanded}
                    <ChevronUp class="w-3.5 h-3.5 text-slate-400" />
                  {:else}
                    <ChevronDown class="w-3.5 h-3.5 text-slate-400" />
                  {/if}
                </button>

                {#if task.status === "success"}
                  <button
                    type="button"
                    onclick={() => navigateTo("stacks")}
                    class="flex items-center gap-1.5 px-3.5 py-1.5 rounded-xl text-xs font-semibold bg-emerald-600 hover:bg-emerald-500 text-white shadow-[0_0_15px_rgba(16,185,129,0.3)] transition cursor-pointer"
                  >
                    <span>View in Stacks</span>
                    <ArrowRight class="w-3.5 h-3.5" />
                  </button>
                {/if}

                {#if task.status !== "deploying"}
                  <button
                    type="button"
                    onclick={() => deploymentStore.dismiss(task.id)}
                    class="p-1.5 rounded-xl text-slate-500 hover:text-slate-300 hover:bg-white/[0.05] transition cursor-pointer"
                    title="Dismiss notification"
                  >
                    <X class="w-4 h-4" />
                  </button>
                {/if}
              </div>
            </div>

            <!-- Optional Progress bar for image pulling -->
            {#if task.status === "deploying" && task.progress}
              <div
                class="px-5 py-2 bg-[#0d0e16] border-t border-white/[0.04] flex items-center justify-between text-[11px] font-mono"
              >
                <span class="text-slate-400"
                  >{task.detail || "Synchronizing layers..."}</span
                >
                <span class="text-cyan-400 font-bold">{task.progress}</span>
              </div>
            {/if}

            <!-- Expandable Live Terminal Console -->
            {#if isExpanded}
              <div
                class="border-t border-white/[0.08] bg-[#07080c] p-4 max-h-56 overflow-y-auto space-y-1.5 font-mono text-xs select-text"
              >
                {#each task.logs as log (log.id)}
                  <div
                    class="flex items-start gap-2.5 {log.isError
                      ? 'text-red-400'
                      : 'text-slate-300'}"
                  >
                    <span
                      class="text-[10px] text-slate-600 shrink-0 select-none"
                      >[{log.time}]</span
                    >
                    <span
                      class="px-1.5 py-0.2 rounded text-[10px] uppercase font-bold shrink-0 select-none {log.step ===
                      'pull'
                        ? 'bg-cyan-500/20 text-cyan-300'
                        : log.step === 'network'
                          ? 'bg-blue-500/20 text-blue-300'
                          : log.step === 'create'
                            ? 'bg-amber-500/20 text-amber-300'
                            : log.step === 'start'
                              ? 'bg-violet-500/20 text-violet-300'
                              : log.step === 'done'
                                ? 'bg-emerald-500/20 text-emerald-300'
                                : log.step === 'error'
                                  ? 'bg-red-500/20 text-red-300'
                                  : 'bg-slate-700 text-slate-300'}"
                    >
                      {log.step}
                    </span>
                    <div class="flex-1 break-all">
                      <span>{log.status}</span>
                      {#if log.progress}
                        <span class="text-cyan-400 font-bold ml-1.5"
                          >{log.progress}</span
                        >
                      {/if}
                      {#if log.detail && log.detail !== log.status}
                        <span class="text-slate-500 ml-1.5 font-normal"
                          >({log.detail})</span
                        >
                      {/if}
                    </div>
                  </div>
                {/each}
              </div>
            {/if}
          </div>
        {/each}
      </div>
    </div>
  {/if}

  <!-- Resource Meters: CPU & RAM Gauges -->
  <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
    <!-- CPU Allocation & Utilization Gauge Card -->
    <div
      class="p-6 rounded-2xl bg-[#12131c] border border-white/[0.08] relative overflow-hidden backdrop-blur-md group hover:border-cyan-500/30 transition-all duration-200"
    >
      <div
        class="flex items-center justify-between pb-3 border-b border-white/[0.06]"
      >
        <div class="flex items-center gap-2.5">
          <div
            class="w-8 h-8 rounded-xl bg-cyan-500/10 border border-cyan-500/25 flex items-center justify-center text-cyan-400"
          >
            <Cpu class="w-4 h-4" />
          </div>
          <div>
            <h2 class="text-sm font-bold text-white tracking-wide">
              CPU Telemetry
            </h2>
            <p class="text-xs text-slate-400 font-mono">
              {cpuCount} Logical Cores Available
            </p>
          </div>
        </div>
      </div>

      <div
        class="pt-4 flex flex-col sm:flex-row items-center justify-around gap-6"
      >
        <Gauge
          value={cpuUtilization}
          label="CPU Load"
          sublabel="{cpuCount} Cores Active"
          color="cyan"
          unit="%"
          size={180}
        />

        <div class="space-y-3 w-full sm:w-48 text-xs font-mono">
          <div
            class="p-2.5 rounded-xl bg-[#0a0b10] border border-white/5 flex items-center justify-between"
          >
            <span class="text-slate-400">Architecture</span>
            <span class="text-slate-200 font-semibold">{arch}</span>
          </div>
          <div
            class="p-2.5 rounded-xl bg-[#0a0b10] border border-white/5 flex items-center justify-between"
          >
            <span class="text-slate-400">Containers Active</span>
            <span class="text-cyan-400 font-semibold"
              >{runningContainers} Running</span
            >
          </div>
          <div
            class="p-2.5 rounded-xl bg-[#0a0b10] border border-white/5 flex items-center justify-between"
          >
            <span class="text-slate-400">Scheduler</span>
            <span class="text-emerald-400 font-semibold">CFS Ready</span>
          </div>
        </div>
      </div>

      <div
        class="absolute bottom-0 left-0 right-0 h-0.5 bg-gradient-to-r from-cyan-500 via-teal-400 to-indigo-500 opacity-50 group-hover:opacity-100 transition"
      ></div>
    </div>

    <!-- RAM Allocation & Utilization Gauge Card -->
    <div
      class="p-6 rounded-2xl bg-[#12131c] border border-white/[0.08] relative overflow-hidden backdrop-blur-md group hover:border-violet-500/30 transition-all duration-200"
    >
      <div
        class="flex items-center justify-between pb-3 border-b border-white/[0.06]"
      >
        <div class="flex items-center gap-2.5">
          <div
            class="w-8 h-8 rounded-xl bg-violet-500/10 border border-violet-500/25 flex items-center justify-center text-violet-400"
          >
            <Server class="w-4 h-4" />
          </div>
          <div>
            <h2 class="text-sm font-bold text-white tracking-wide">
              Memory Telemetry
            </h2>
            <p class="text-xs text-slate-400 font-mono">
              Installed Capacity: {formatBytes(memTotal)}
            </p>
          </div>
        </div>
      </div>

      <div
        class="pt-4 flex flex-col sm:flex-row items-center justify-around gap-6"
      >
        <Gauge
          value={ramUtilization}
          label="RAM Utilization"
          sublabel="{formatBytes(usedMemBytes)} Used"
          color="violet"
          unit="%"
          size={180}
        />

        <div class="space-y-3 w-full sm:w-48 text-xs font-mono">
          <div
            class="p-2.5 rounded-xl bg-[#0a0b10] border border-white/5 flex items-center justify-between"
          >
            <span class="text-slate-400">Total Installed</span>
            <span class="text-slate-200 font-semibold"
              >{formatBytes(memTotal)}</span
            >
          </div>
          <div
            class="p-2.5 rounded-xl bg-[#0a0b10] border border-white/5 flex items-center justify-between"
          >
            <span class="text-slate-400">Used Memory</span>
            <span class="text-violet-400 font-semibold"
              >{formatBytes(usedMemBytes)}</span
            >
          </div>
          <div
            class="p-2.5 rounded-xl bg-[#0a0b10] border border-white/5 flex items-center justify-between"
          >
            <span class="text-slate-400">Memory Limits</span>
            {#if memoryLimitSupported === null}
              <span class="text-slate-400 font-semibold">—</span>
            {:else if memoryLimitSupported}
              <span class="text-emerald-400 font-semibold">Enabled</span>
            {:else}
              <span class="text-amber-400 font-semibold">Disabled</span>
            {/if}
          </div>
        </div>
      </div>

      <div
        class="absolute bottom-0 left-0 right-0 h-0.5 bg-gradient-to-r from-violet-600 via-indigo-500 to-pink-500 opacity-50 group-hover:opacity-100 transition"
      ></div>
    </div>
  </div>

  <!-- Resource Summary Grid -->
  <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-4">
    <!-- Containers Card -->
    <MetricCard
      title="Containers"
      value="{runningContainers} / {totalContainers}"
      subtitle="{stoppedContainers} stopped, {pausedContainers} paused"
      icon={Box}
      variant="emerald"
      clickable
      onclick={() => navigateTo("containers")}
    >
      <div class="flex items-center gap-1.5 text-[11px] font-mono">
        <span
          class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded-md bg-emerald-500/15 text-emerald-400 border border-emerald-500/30"
        >
          <span class="w-1.5 h-1.5 rounded-full bg-emerald-400"></span>
          {runningContainers} Run
        </span>
        <span
          class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded-md bg-rose-500/15 text-rose-400 border border-rose-500/30"
        >
          <span class="w-1.5 h-1.5 rounded-full bg-rose-400"></span>
          {stoppedContainers} Off
        </span>
      </div>
    </MetricCard>

    <!-- Stacks Card -->
    <MetricCard
      title="Active Stacks"
      value="{activeStacks} / {totalStacks}"
      subtitle="Docker Compose topologies"
      icon={Layers}
      variant="violet"
      clickable
      onclick={() => navigateTo("stacks")}
    />

    <!-- Images Card -->
    <MetricCard
      title="Images"
      value={totalImages}
      subtitle="{formatBytes(totalImageSize)} cached locally"
      icon={Disc}
      variant="cyan"
      clickable
      onclick={() => navigateTo("images")}
      badge={totalImageSize > 0 ? formatBytes(totalImageSize) : "0 B"}
    />

    <!-- Volumes Card -->
    <MetricCard
      title="Volumes"
      value={totalVolumes}
      subtitle={totalVolumeSize > 0
        ? `${formatBytes(totalVolumeSize)} disk usage`
        : "Persistent storage mounts"}
      icon={HardDrive}
      variant="amber"
      clickable
      onclick={() => navigateTo("volumes")}
      badge={totalVolumeSize > 0 ? formatBytes(totalVolumeSize) : "Named"}
    />

    <!-- Networks Card -->
    <MetricCard
      title="Networks"
      value={totalNetworks}
      subtitle="Virtual bridges & overlays"
      icon={Network}
      variant="slate"
      clickable
      onclick={() => navigateTo("networks")}
    />
  </div>

  <!-- Workload Status & System Environment Specifications -->
  <div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
    <!-- Recent Workload / Containers List Overview -->
    <div
      class="lg:col-span-2 p-6 rounded-2xl bg-[#12131c] border border-white/[0.08] flex flex-col justify-between"
    >
      <div>
        <div
          class="flex items-center justify-between pb-3 border-b border-white/[0.08]"
        >
          <div class="flex items-center gap-2.5">
            <div
              class="w-7 h-7 rounded-lg bg-violet-500/10 border border-violet-500/20 flex items-center justify-center text-violet-400"
            >
              <Activity class="w-4 h-4" />
            </div>
            <div>
              <h2 class="text-sm font-bold text-white">Workload Overview</h2>
              <p class="text-xs text-slate-400">
                Running instances and active Docker container health
              </p>
            </div>
          </div>
          <button
            type="button"
            onclick={() => navigateTo("containers")}
            class="text-xs font-semibold text-cyan-400 hover:text-cyan-300 flex items-center gap-1 transition"
          >
            <span>View All</span>
            <ExternalLink class="w-3 h-3" />
          </button>
        </div>

        {#if containers.length === 0}
          <div class="py-12 text-center text-slate-400 space-y-3">
            <Box class="w-8 h-8 mx-auto text-slate-600" />
            <p class="text-xs">
              No active containers found on this Docker host.
            </p>
            <button
              type="button"
              onclick={() => navigateTo("containers")}
              class="px-3.5 py-1.5 rounded-xl bg-violet-600 hover:bg-violet-500 text-white text-xs font-medium shadow-[0_0_12px_rgba(124,58,237,0.3)] transition inline-flex items-center gap-1.5"
            >
              <Plus class="w-3.5 h-3.5" />
              Launch First Container
            </button>
          </div>
        {:else}
          <div class="mt-4 divide-y divide-white/[0.05]">
            {#each containers.slice(0, 5) as container}
              <div class="py-3 flex items-center justify-between gap-3 text-xs">
                <div class="flex items-center gap-3 min-w-0">
                  <div
                    class="w-2 h-2 rounded-full shrink-0 {String(
                      container.State,
                    ).toLowerCase() === 'running'
                      ? 'bg-emerald-400 shadow-[0_0_8px_#34d399]'
                      : 'bg-rose-400'}"
                  ></div>
                  <div class="min-w-0">
                    <span class="font-bold text-slate-200 truncate block">
                      {container.Names && container.Names[0]
                        ? container.Names[0].replace(/^\//, "")
                        : container.Id.slice(0, 12)}
                    </span>
                    <span
                      class="text-[11px] font-mono text-slate-500 truncate block"
                    >
                      {container.Image}
                    </span>
                  </div>
                </div>

                <div class="flex items-center gap-2 shrink-0">
                  <span
                    class="px-2 py-0.5 rounded-full text-[10px] font-mono uppercase font-semibold border {String(
                      container.State,
                    ).toLowerCase() === 'running'
                      ? 'bg-emerald-500/15 text-emerald-400 border-emerald-500/30'
                      : 'bg-rose-500/15 text-rose-400 border-rose-500/30'}"
                  >
                    {container.State}
                  </span>
                  {#if container.Ports && container.Ports.length > 0}
                    <span
                      class="hidden sm:inline-flex px-2 py-0.5 rounded-full text-[10px] font-mono text-slate-400 bg-white/[0.04] border border-white/10"
                    >
                      {container.Ports[0].PublicPort
                        ? `:${container.Ports[0].PublicPort}`
                        : container.Ports[0].PrivatePort}
                    </span>
                  {/if}
                </div>
              </div>
            {/each}
          </div>
        {/if}
      </div>

      <div
        class="mt-4 pt-3 border-t border-white/[0.06] flex items-center justify-between text-xs text-slate-400"
      >
        <span class="font-mono text-[11px]">
          Showing top {Math.min(5, containers.length)} of {containers.length} containers
        </span>
        <button
          type="button"
          onclick={() => navigateTo("containers")}
          class="font-semibold text-violet-400 hover:text-violet-300 transition"
        >
          Manage Workloads &rarr;
        </button>
      </div>
    </div>

    <!-- Daemon Specifications & Runtime Spec Card -->
    <div
      class="p-6 rounded-2xl bg-[#12131c] border border-white/[0.08] flex flex-col justify-between"
    >
      <div>
        <div
          class="flex items-center gap-2 text-white font-bold text-sm pb-3 border-b border-white/[0.08]"
        >
          <ShieldCheck class="w-4 h-4 text-cyan-400" />
          <span>Daemon Specifications</span>
        </div>

        <div class="mt-4 space-y-2.5 text-xs">
          <div class="flex justify-between py-1.5 border-b border-white/[0.04]">
            <span class="text-slate-400">Container Engine</span>
            <span
              class="font-mono font-semibold flex items-center gap-1.5 {isPodman
                ? 'text-purple-400'
                : 'text-cyan-400'}"
            >
              <span
                class="w-1.5 h-1.5 rounded-full {isPodman
                  ? 'bg-purple-400'
                  : 'bg-cyan-400'}"
              ></span>
              {engineDisplay}
            </span>
          </div>
          <div class="flex justify-between py-1.5 border-b border-white/[0.04]">
            <span class="text-slate-400">Storage Driver</span>
            <span class="font-mono text-slate-200">{storageDriver}</span>
          </div>
          <div class="flex justify-between py-1.5 border-b border-white/[0.04]">
            <span class="text-slate-400">OS / Distro</span>
            <span class="font-mono text-slate-200">{osName}</span>
          </div>
          <div class="flex justify-between py-1.5 border-b border-white/[0.04]">
            <span class="text-slate-400">Kernel Release</span>
            <span class="font-mono text-slate-200">{kernelVersion}</span>
          </div>
          <div class="flex justify-between py-1.5 border-b border-white/[0.04]">
            <span class="text-slate-400">Memory Limits</span>
            {#if memoryLimitSupported === null}
              <span class="font-mono text-slate-400">—</span>
            {:else if memoryLimitSupported}
              <span class="font-mono text-emerald-400">Supported</span>
            {:else}
              <span class="font-mono text-amber-400">Unsupported</span>
            {/if}
          </div>
          <div class="flex justify-between py-1.5 border-b border-white/[0.04]">
            <span class="text-slate-400">Swap Limits</span>
            {#if swapLimitSupported === null}
              <span class="font-mono text-slate-400">—</span>
            {:else if swapLimitSupported}
              <span class="font-mono text-emerald-400">Supported</span>
            {:else}
              <span class="font-mono text-amber-400">Unsupported</span>
            {/if}
          </div>
          <div class="flex justify-between py-1.5">
            <span class="text-slate-400"
              >{isPodman ? "Storage Root Dir" : "Docker Root Dir"}</span
            >
            <span
              class="font-mono text-slate-300 truncate max-w-[130px]"
              title={rootDir}
            >
              {rootDir}
            </span>
          </div>
        </div>
      </div>

      <div class="mt-6 pt-4 border-t border-white/[0.06]">
        <button
          type="button"
          onclick={() => navigateTo("settings")}
          class="w-full py-2 px-3 rounded-xl bg-white/[0.04] hover:bg-white/[0.08] border border-white/10 text-xs font-medium text-slate-300 hover:text-white transition text-center"
        >
          View Daemon Configuration &rarr;
        </button>
      </div>
    </div>
  </div>
</div>
