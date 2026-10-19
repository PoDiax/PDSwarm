<script lang="ts">
  import {
    onMount,
    onDestroy,
  } from "svelte";
  import {
    HardDrive,
    Database,
    Plus,
    Search,
    RefreshCw,
    Trash,
    Scissors,
    CircleAlert,
    CircleCheck,
    LoaderCircle,
    Copy,
    Check,
    Clock,
    TriangleAlert,
    X,
    Folder,
    Box,
    ArrowUpDown,
  } from '@lucide/svelte';
  import { volumeApi, containerApi } from "../lib/api";
  import type { Volume, Container } from "../lib/types";
  import CreateVolumeModal from "../lib/components/CreateVolumeModal.svelte";

  interface Props {
    onNavigate?: (tab: string) => void;
    onSelectContainer?: (containerId: string) => void;
  }

  let { onNavigate, onSelectContainer }: Props = $props();

  // State
  let volumes = $state<Volume[]>([]);
  let containers = $state<Container[]>([]);
  let isLoading = $state(true);
  let isRefreshing = $state(false);
  let error = $state<string | null>(null);
  let feedback = $state<{ type: "success" | "error"; message: string } | null>(
    null,
  );

  // Search & Filters
  let searchQuery = $state("");
  let driverFilter = $state<string>("all");
  let sortBy = $state<"name" | "size-desc" | "size-asc" | "created">("name");

  // Multi-Selection State
  let selectedNames = $state<string[]>([]);
  let isBatchDeleteModalOpen = $state(false);
  let isBatchDeleting = $state(false);
  let batchForce = $state(false);

  // Single Delete & Prune Modals
  let isCreateModalOpen = $state(false);
  let isPruneModalOpen = $state(false);
  let isPruning = $state(false);

  let deleteVolumeTarget = $state<Volume | null>(null);
  let deleteForce = $state(false);
  let isDeleting = $state(false);

  // Clipboard & Timers
  let copiedId = $state<string | null>(null);
  let refreshTimer: ReturnType<typeof setInterval> | null = null;

  // Column Resizing State
  type VolumeColumnKey = 'name' | 'attached' | 'size' | 'driver' | 'mountpoint' | 'created';

  const defaultColWidths: Record<VolumeColumnKey, number> = {
    name: 260,
    attached: 180,
    size: 85,
    driver: 80,
    mountpoint: 120,
    created: 110,
  };

  const minColWidths: Record<VolumeColumnKey, number> = {
    name: 130,
    attached: 100,
    size: 60,
    driver: 60,
    mountpoint: 80,
    created: 80,
  };

  function loadSavedColumnWidths(): Record<VolumeColumnKey, number> {
    if (typeof window === 'undefined') return { ...defaultColWidths };
    try {
      const stored = localStorage.getItem('pdswarm_volume_col_widths');
      if (stored) {
        const parsed = JSON.parse(stored);
        if (parsed.mountpoint === 230) {
          parsed.mountpoint = defaultColWidths.mountpoint;
        }
        return { ...defaultColWidths, ...parsed };
      }
    } catch {}
    return { ...defaultColWidths };
  }

  let colWidths = $state<Record<VolumeColumnKey, number>>(loadSavedColumnWidths());

  let tableTotalWidth = $derived(
    44 +
    colWidths.name +
    colWidths.attached +
    colWidths.size +
    colWidths.driver +
    colWidths.mountpoint +
    colWidths.created +
    80
  );

  function saveColumnWidths() {
    if (typeof window === 'undefined') return;
    try {
      localStorage.setItem('pdswarm_volume_col_widths', JSON.stringify($state.snapshot(colWidths)));
    } catch {}
  }

  let resizingCol = $state<VolumeColumnKey | null>(null);
  let resizeStartX = 0;
  let startColWidth = 0;

  function startResize(col: VolumeColumnKey, e: MouseEvent) {
    e.preventDefault();
    e.stopPropagation();
    resizingCol = col;
    resizeStartX = e.clientX;
    startColWidth = colWidths[col];

    const onMouseMove = (moveEvent: MouseEvent) => {
      if (!resizingCol) return;
      const diff = moveEvent.clientX - resizeStartX;
      const newWidth = Math.max(minColWidths[resizingCol], Math.round(startColWidth + diff));
      colWidths[resizingCol] = newWidth;
    };

    const onMouseUp = () => {
      resizingCol = null;
      saveColumnWidths();
      document.body.style.cursor = '';
      document.body.style.userSelect = '';
      window.removeEventListener('mousemove', onMouseMove);
      window.removeEventListener('mouseup', onMouseUp);
    };

    document.body.style.cursor = 'col-resize';
    document.body.style.userSelect = 'none';
    window.addEventListener('mousemove', onMouseMove);
    window.addEventListener('mouseup', onMouseUp);
  }

  let tableContainerEl = $state<HTMLDivElement | null>(null);

  function syncDefaultWidthsToContainer() {
    if (typeof window === 'undefined') return;
    try {
      const hasSaved = !!localStorage.getItem('pdswarm_volume_col_widths');
      if (hasSaved) return; // Do not overwrite user-customized widths
    } catch {}

    if (!tableContainerEl) return;
    const containerW = tableContainerEl.clientWidth;
    if (containerW <= 0) return;

    const fixedOther = 44 + colWidths.size + colWidths.driver + colWidths.created + 80;
    const remaining = containerW - fixedOther;
    if (remaining > (minColWidths.name + minColWidths.attached + minColWidths.mountpoint)) {
      colWidths.name = Math.round(remaining * 0.45);
      colWidths.attached = Math.round(remaining * 0.30);
      colWidths.mountpoint = Math.round(remaining * 0.25);
    }
  }

  // Mountpoint Path Preview & Floating Popover
  function getShortMountpoint(path?: string): string {
    if (!path) return "—";
    const parts = path.split("/").filter(Boolean);
    if (parts.length === 0) return path;
    if (parts.length <= 2) return path;
    return `.../${parts.slice(-2).join("/")}`;
  }

  let activeTooltip = $state<{
    path: string;
    name: string;
    x: number;
    y: number;
    showAbove: boolean;
  } | null>(null);
  let tooltipCloseTimeout: ReturnType<typeof setTimeout> | null = null;

  function showMountTooltip(path: string | undefined, name: string, e: MouseEvent) {
    if (!path) return;
    if (tooltipCloseTimeout) {
      clearTimeout(tooltipCloseTimeout);
      tooltipCloseTimeout = null;
    }
    const target = (e.currentTarget as HTMLElement).getBoundingClientRect();
    const showAbove = target.top > 140;
    activeTooltip = {
      path,
      name,
      x: Math.min(Math.max(16, target.left), window.innerWidth - 380),
      y: showAbove ? target.top - 8 : target.bottom + 8,
      showAbove,
    };
  }

  function hideMountTooltip() {
    tooltipCloseTimeout = setTimeout(() => {
      activeTooltip = null;
    }, 150);
  }

  function cancelHideTooltip() {
    if (tooltipCloseTimeout) {
      clearTimeout(tooltipCloseTimeout);
      tooltipCloseTimeout = null;
    }
  }

  function showFeedback(type: "success" | "error", message: string) {
    feedback = { type, message };
    setTimeout(() => {
      if (feedback?.message === message) {
        feedback = null;
      }
    }, 4500);
  }

  async function fetchVolumes(silent = false) {
    if (!silent) isRefreshing = true;
    error = null;

    try {
      const [volResp, conts] = await Promise.all([
        volumeApi.list(),
        containerApi.list(true).catch(() => [] as Container[]),
      ]);
      volumes = volResp?.Volumes || [];
      containers = conts || [];

      // Prune selectedNames that no longer exist
      const existingNames = new Set(volumes.map((v) => v.Name));
      selectedNames = selectedNames.filter((name) => existingNames.has(name));
    } catch (err: unknown) {
      if (err instanceof Error) {
        error = err.message;
      } else {
        error = "Failed to load Docker volumes.";
      }
    } finally {
      isLoading = false;
      isRefreshing = false;
    }
  }

  onMount(() => {
    fetchVolumes();
    refreshTimer = setInterval(() => {
      fetchVolumes(true);
    }, 10000);
    setTimeout(syncDefaultWidthsToContainer, 60);
    window.addEventListener('resize', syncDefaultWidthsToContainer);
  });

  onDestroy(() => {
    if (refreshTimer) clearInterval(refreshTimer);
    if (typeof window !== 'undefined') {
      window.removeEventListener('resize', syncDefaultWidthsToContainer);
    }
  });

  function formatBytes(bytes: number): string {
    if (!bytes || bytes <= 0) return "0 B";
    const units = ["B", "KB", "MB", "GB", "TB"];
    const i = Math.floor(Math.log(bytes) / Math.log(1024));
    return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`;
  }

  function formatDate(dateStr?: string): string {
    if (!dateStr) return "Unknown";
    try {
      const d = new Date(dateStr);
      if (isNaN(d.getTime())) return dateStr;
      return d.toLocaleDateString(undefined, {
        year: "numeric",
        month: "short",
        day: "numeric",
      });
    } catch {
      return dateStr;
    }
  }

  function copyToClipboard(text: string, id: string) {
    navigator.clipboard.writeText(text);
    copiedId = id;
    setTimeout(() => {
      if (copiedId === id) copiedId = null;
    }, 2000);
  }

  function getContainersUsingVolume(volName: string): Container[] {
    return containers.filter((c) =>
      c.Mounts?.some((m) => m.Name === volName || m.Source?.includes(volName)),
    );
  }

  let totalVolumeBytes = $derived.by(() => {
    return volumes.reduce((sum, v) => {
      if (v.UsageData?.Size && v.UsageData.Size > 0) {
        return sum + v.UsageData.Size;
      }
      return sum;
    }, 0);
  });

  let availableDrivers = $derived.by(() => {
    const set = new Set<string>();
    for (const v of volumes) {
      if (v.Driver) set.add(v.Driver);
    }
    return Array.from(set);
  });

  let filteredVolumes = $derived.by(() => {
    const list = volumes.filter((v) => {
      if (driverFilter !== "all" && v.Driver !== driverFilter) {
        return false;
      }
      if (!searchQuery.trim()) return true;
      const q = searchQuery.toLowerCase().trim();
      const nameMatch = v.Name.toLowerCase().includes(q);
      const driverMatch = (v.Driver || "").toLowerCase().includes(q);
      const mountMatch = (v.Mountpoint || "").toLowerCase().includes(q);
      return nameMatch || driverMatch || mountMatch;
    });

    return list.slice().sort((a, b) => {
      if (sortBy === "size-desc") {
        const sA = a.UsageData?.Size ?? -1;
        const sB = b.UsageData?.Size ?? -1;
        return sB - sA;
      }
      if (sortBy === "size-asc") {
        const sA = a.UsageData?.Size ?? -1;
        const sB = b.UsageData?.Size ?? -1;
        return sA - sB;
      }
      if (sortBy === "created") {
        const tA = a.CreatedAt ? new Date(a.CreatedAt).getTime() : 0;
        const tB = b.CreatedAt ? new Date(b.CreatedAt).getTime() : 0;
        return tB - tA;
      }
      return a.Name.localeCompare(b.Name);
    });
  });

  // Multi-Selection Computations
  let allSelected = $derived(
    filteredVolumes.length > 0 &&
      filteredVolumes.every((v) => selectedNames.includes(v.Name))
  );

  let someSelected = $derived(
    filteredVolumes.some((v) => selectedNames.includes(v.Name)) && !allSelected
  );

  let selectedTotalSize = $derived.by(() => {
    return volumes
      .filter((v) => selectedNames.includes(v.Name))
      .reduce((sum, v) => sum + (v.UsageData?.Size || 0), 0);
  });

  let selectedInUse = $derived.by(() => {
    return volumes
      .filter((v) => selectedNames.includes(v.Name))
      .filter((v) => getContainersUsingVolume(v.Name).length > 0);
  });

  function toggleSelectAll() {
    if (allSelected) {
      selectedNames = [];
    } else {
      const names = filteredVolumes.map((v) => v.Name);
      selectedNames = Array.from(new Set([...selectedNames, ...names]));
    }
  }

  function toggleSelect(name: string) {
    if (selectedNames.includes(name)) {
      selectedNames = selectedNames.filter((n) => n !== name);
    } else {
      selectedNames = [...selectedNames, name];
    }
  }

  // Single volume delete actions
  function openDeleteModal(vol: Volume) {
    deleteVolumeTarget = vol;
    deleteForce = false;
  }

  function closeDeleteModal() {
    deleteVolumeTarget = null;
    deleteForce = false;
    isDeleting = false;
  }

  async function confirmDeleteVolume() {
    if (!deleteVolumeTarget) return;
    isDeleting = true;

    try {
      await volumeApi.remove(deleteVolumeTarget.Name, deleteForce);
      showFeedback("success", `Volume "${deleteVolumeTarget.Name}" removed.`);
      closeDeleteModal();
      await fetchVolumes(true);
    } catch (err: unknown) {
      if (err instanceof Error) {
        showFeedback("error", `Failed to remove volume: ${err.message}`);
      } else {
        showFeedback("error", "Failed to remove volume.");
      }
    } finally {
      isDeleting = false;
    }
  }

  // Batch delete actions
  async function confirmBatchDelete() {
    if (selectedNames.length === 0) return;
    isBatchDeleting = true;

    let successCount = 0;
    let failCount = 0;
    const errors: string[] = [];

    for (const name of selectedNames) {
      try {
        await volumeApi.remove(name, batchForce);
        successCount++;
      } catch (err: unknown) {
        failCount++;
        if (err instanceof Error) {
          errors.push(`${name}: ${err.message}`);
        }
      }
    }

    if (successCount > 0 && failCount === 0) {
      showFeedback("success", `Successfully removed ${successCount} volume(s).`);
    } else if (successCount > 0 && failCount > 0) {
      showFeedback("error", `Removed ${successCount} volume(s), but ${failCount} failed: ${errors[0] || ''}`);
    } else {
      showFeedback("error", `Failed to remove selected volume(s): ${errors[0] || 'Unknown error'}`);
    }

    isBatchDeleteModalOpen = false;
    isBatchDeleting = false;
    selectedNames = [];
    await fetchVolumes(true);
  }

  async function confirmPrune() {
    isPruning = true;
    try {
      const res = await volumeApi.prune();
      const count = res.VolumesDeleted?.length || 0;
      const space = formatBytes(res.SpaceReclaimed || 0);
      showFeedback(
        "success",
        `Pruned ${count} unused volume${count === 1 ? "" : "s"}. Reclaimed ${space}.`,
      );
      isPruneModalOpen = false;
      await fetchVolumes(true);
    } catch (err: unknown) {
      if (err instanceof Error) {
        showFeedback("error", `Failed to prune volumes: ${err.message}`);
      } else {
        showFeedback("error", "Failed to prune volumes.");
      }
    } finally {
      isPruning = false;
    }
  }
</script>

<div class="space-y-6">
  <!-- Top Header Banner -->
  <div class="flex flex-col md:flex-row md:items-center justify-between gap-4">
    <div class="flex items-center gap-3">
      <div
        class="w-10 h-10 rounded-2xl bg-gradient-to-br from-violet-600/30 to-indigo-700/20 border border-violet-500/40 flex items-center justify-center text-violet-400 shadow-[0_0_20px_rgba(124,58,237,0.3)]"
      >
        <HardDrive class="w-5 h-5" />
      </div>
      <div>
        <div class="flex items-center gap-2">
          <h1 class="text-xl font-bold tracking-tight text-white">Volumes</h1>
          <span
            class="px-2 py-0.5 text-[10px] font-mono rounded-full bg-violet-500/20 text-violet-300 border border-violet-500/30 font-semibold"
          >
            {volumes.length}
          </span>
        </div>
        <p class="text-xs text-slate-400">
          Persistent storage directories managed independently of container
          lifecycles
        </p>
      </div>
    </div>

    <div class="flex items-center gap-2">
      <!-- Prune Button -->
      <button
        onclick={() => (isPruneModalOpen = true)}
        class="px-3.5 py-2 rounded-xl bg-amber-500/10 hover:bg-amber-500/20 text-amber-300 border border-amber-500/20 hover:border-amber-500/40 text-xs font-semibold transition inline-flex items-center gap-1.5 cursor-pointer"
        title="Prune all unreferenced volumes"
      >
        <Scissors class="w-3.5 h-3.5" />
        <span>Prune Unused</span>
      </button>

      <!-- Refresh Button -->
      <button
        onclick={() => fetchVolumes()}
        disabled={isRefreshing}
        class="p-2 rounded-xl bg-white/[0.05] hover:bg-white/[0.1] text-slate-300 border border-white/10 transition cursor-pointer"
        title="Refresh volumes"
        aria-label="Refresh volumes"
      >
        <RefreshCw
          class="w-4 h-4 {isRefreshing ? 'animate-spin text-violet-400' : ''}"
        />
      </button>

      <!-- Create Volume Button -->
      <button
        onclick={() => (isCreateModalOpen = true)}
        class="px-4 py-2 rounded-xl bg-gradient-to-r from-violet-600 to-indigo-600 hover:from-violet-500 hover:to-indigo-500 text-white text-xs font-semibold shadow-[0_0_20px_rgba(124,58,237,0.4)] transition inline-flex items-center gap-2 cursor-pointer"
      >
        <Plus class="w-4 h-4" />
        <span>Create Volume</span>
      </button>
    </div>
  </div>

  <!-- Feedback Toast -->
  {#if feedback}
    <div
      class="p-3.5 rounded-2xl flex items-center gap-3 text-xs font-mono animate-in fade-in slide-in-from-top-2 duration-200 {feedback.type ===
      'success'
        ? 'bg-emerald-500/15 border border-emerald-500/30 text-emerald-300'
        : 'bg-rose-500/15 border border-rose-500/30 text-rose-300'}"
    >
      {#if feedback.type === "success"}
        <CircleCheck class="w-4 h-4 shrink-0 text-emerald-400" />
      {:else}
        <CircleAlert class="w-4 h-4 shrink-0 text-rose-400" />
      {/if}
      <span class="flex-1">{feedback.message}</span>
      <button
        onclick={() => (feedback = null)}
        class="text-slate-400 hover:text-white"
        aria-label="Dismiss feedback"
      >
        <X class="w-3.5 h-3.5" />
      </button>
    </div>
  {/if}

  {#if error}
    <div
      class="p-4 rounded-2xl bg-rose-500/10 border border-rose-500/20 text-rose-300 flex items-start gap-3 text-xs"
    >
      <CircleAlert class="w-4 h-4 shrink-0 mt-0.5" />
      <div>
        <span class="font-bold">Error loading volumes:</span>
        <span class="font-mono block mt-0.5">{error}</span>
      </div>
    </div>
  {/if}

  <!-- Stats Grid -->
  <div class="grid grid-cols-2 md:grid-cols-4 gap-3">
    <div
      class="p-4 rounded-xl bg-[#12131c] border border-white/[0.08] flex items-center gap-3"
    >
      <div
        class="w-9 h-9 rounded-xl bg-violet-500/10 border border-violet-500/20 flex items-center justify-center text-violet-400"
      >
        <HardDrive class="w-4 h-4" />
      </div>
      <div>
        <span class="text-[11px] text-slate-400 block font-mono"
          >Total Volumes</span
        >
        <span class="text-lg font-bold text-white font-mono"
          >{volumes.length}</span
        >
      </div>
    </div>

    <div
      class="p-4 rounded-xl bg-[#12131c] border border-white/[0.08] flex items-center gap-3"
    >
      <div
        class="w-9 h-9 rounded-xl bg-amber-500/10 border border-amber-500/20 flex items-center justify-center text-amber-400"
      >
        <Database class="w-4 h-4" />
      </div>
      <div>
        <span class="text-[11px] text-slate-400 block font-mono"
          >Total Storage</span
        >
        <span class="text-lg font-bold text-amber-400 font-mono">
          {formatBytes(totalVolumeBytes)}
        </span>
      </div>
    </div>

    <div
      class="p-4 rounded-xl bg-[#12131c] border border-white/[0.08] flex items-center gap-3"
    >
      <div
        class="w-9 h-9 rounded-xl bg-cyan-500/10 border border-cyan-500/20 flex items-center justify-center text-cyan-400"
      >
        <Folder class="w-4 h-4" />
      </div>
      <div>
        <span class="text-[11px] text-slate-400 block font-mono"
          >Local Driver</span
        >
        <span class="text-lg font-bold text-cyan-400 font-mono">
          {volumes.filter((v) => (v.Driver || "local") === "local").length}
        </span>
      </div>
    </div>

    <div
      class="p-4 rounded-xl bg-[#12131c] border border-white/[0.08] flex items-center gap-3"
    >
      <div
        class="w-9 h-9 rounded-xl bg-emerald-500/10 border border-emerald-500/20 flex items-center justify-center text-emerald-400"
      >
        <Box class="w-4 h-4" />
      </div>
      <div>
        <span class="text-[11px] text-slate-400 block font-mono"
          >In-Use Volumes</span
        >
        <span class="text-lg font-bold text-emerald-400 font-mono">
          {volumes.filter((v) => getContainersUsingVolume(v.Name).length > 0)
            .length}
        </span>
      </div>
    </div>
  </div>

  <!-- Multi-Selection Action Bar -->
  {#if selectedNames.length > 0}
    <div
      class="p-3.5 rounded-2xl bg-gradient-to-r from-violet-950/60 via-indigo-950/60 to-purple-950/60 border border-violet-500/30 flex flex-wrap items-center justify-between gap-3 shadow-[0_0_30px_rgba(124,58,237,0.25)] animate-in fade-in duration-200"
    >
      <div class="flex items-center gap-3">
        <span class="w-2.5 h-2.5 rounded-full bg-violet-400 animate-pulse"></span>
        <span class="text-xs font-semibold text-white">
          <strong class="font-bold text-violet-300 font-mono">{selectedNames.length}</strong> volume{selectedNames.length === 1 ? '' : 's'} selected
        </span>
        {#if selectedTotalSize > 0}
          <span class="text-xs text-slate-400 font-mono">
            ({formatBytes(selectedTotalSize)})
          </span>
        {/if}
        {#if selectedInUse.length > 0}
          <span class="px-2 py-0.5 rounded-full text-[10px] font-mono bg-amber-500/15 text-amber-300 border border-amber-500/30 flex items-center gap-1">
            <TriangleAlert class="w-3 h-3 text-amber-400" />
            {selectedInUse.length} in-use
          </span>
        {/if}
      </div>

      <div class="flex items-center gap-2">
        <button
          onclick={() => (selectedNames = [])}
          class="px-3 py-1.5 rounded-xl bg-white/5 hover:bg-white/10 text-slate-300 text-xs font-medium transition cursor-pointer"
        >
          Clear Selection
        </button>
        <button
          onclick={() => { batchForce = false; isBatchDeleteModalOpen = true; }}
          class="px-4 py-1.5 rounded-xl bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold shadow-[0_0_15px_rgba(244,63,94,0.35)] transition flex items-center gap-1.5 cursor-pointer"
        >
          <Trash class="w-3.5 h-3.5" />
          Delete Selected ({selectedNames.length})
        </button>
      </div>
    </div>
  {/if}

  <!-- Search & Funnel Controls -->
  <div
    class="p-3 rounded-2xl bg-[#12131c] border border-white/[0.08] flex flex-col md:flex-row items-center justify-between gap-3"
  >
    <!-- Search Bar -->
    <div class="relative w-full md:w-96">
      <Search
        class="w-4 h-4 text-slate-500 absolute left-3.5 top-1/2 -translate-y-1/2"
      />
      <input
        type="text"
        bind:value={searchQuery}
        placeholder="Funnel by volume name, driver, or mount..."
        class="w-full pl-9 pr-8 py-2 rounded-xl bg-[#0c0d14] border border-white/10 text-white placeholder-slate-500 text-xs focus:outline-none focus:border-violet-500 transition font-mono"
      />
      {#if searchQuery}
        <button
          onclick={() => (searchQuery = "")}
          class="absolute right-3 top-1/2 -translate-y-1/2 text-slate-500 hover:text-white"
          aria-label="Clear search"
        >
          <X class="w-3.5 h-3.5" />
        </button>
      {/if}
    </div>

    <div class="flex items-center gap-3 w-full md:w-auto">
      <!-- Sort Selector -->
      <div class="flex items-center gap-2">
        <ArrowUpDown class="w-3.5 h-3.5 text-slate-400" />
        <span class="text-xs text-slate-400">Sort:</span>
        <select
          bind:value={sortBy}
          class="px-3 py-2 rounded-xl bg-[#0c0d14] border border-white/10 text-slate-300 text-xs focus:outline-none focus:border-violet-500"
        >
          <option value="name">Name (A-Z)</option>
          <option value="size-desc">Size (Largest)</option>
          <option value="size-asc">Size (Smallest)</option>
          <option value="created">Recently Created</option>
        </select>
      </div>

      <!-- Driver Selector -->
      <div class="flex items-center gap-2">
        <span class="text-xs text-slate-400">Driver:</span>
        <select
          bind:value={driverFilter}
          class="px-3 py-2 rounded-xl bg-[#0c0d14] border border-white/10 text-slate-300 text-xs focus:outline-none focus:border-violet-500"
        >
          <option value="all">All Drivers</option>
          {#each availableDrivers as d}
            <option value={d}>{d}</option>
          {/each}
        </select>
      </div>
    </div>
  </div>

  <!-- Volumes Table Container -->
  <div
    class="rounded-2xl bg-[#12131c] border border-white/[0.08] overflow-hidden shadow-xl"
  >
    {#if isLoading}
      <div
        class="p-12 flex flex-col items-center justify-center gap-3 text-slate-400"
      >
        <LoaderCircle class="w-7 h-7 text-violet-400 animate-spin" />
        <span class="text-xs font-mono">Querying Docker Engine volumes...</span>
      </div>
    {:else if filteredVolumes.length === 0}
      <div class="p-12 text-center space-y-3">
        <div
          class="w-12 h-12 mx-auto rounded-2xl bg-white/[0.04] border border-white/[0.08] flex items-center justify-center text-slate-500"
        >
          <HardDrive class="w-6 h-6" />
        </div>
        <h3 class="text-sm font-semibold text-white">No volumes found</h3>
        <p class="text-xs text-slate-400 max-w-sm mx-auto">
          {#if searchQuery}
            No volumes match your search query "{searchQuery}".
          {:else}
            There are no persistent Docker volumes created on this host yet.
          {/if}
        </p>
        <div class="pt-2">
          <button
            onclick={() => (isCreateModalOpen = true)}
            class="px-4 py-2 rounded-xl bg-violet-600 hover:bg-violet-500 text-white text-xs font-semibold shadow-[0_0_15px_rgba(124,58,237,0.35)] transition inline-flex items-center gap-2"
          >
            <Plus class="w-3.5 h-3.5" />
            Create Your First Volume
          </button>
        </div>
      </div>
    {:else}
      <div class="overflow-x-auto w-full" bind:this={tableContainerEl}>
        <table
          class="text-left border-collapse text-xs table-fixed min-w-full"
          style="width: max(100%, {tableTotalWidth}px);"
        >
          <thead>
            <tr
              class="border-b border-white/[0.08] bg-[#0c0d14]/70 text-slate-400 font-mono text-[11px] uppercase tracking-wider select-none"
            >
              <!-- Multi-selection Master Checkbox -->
              <th
                class="py-3 px-4 text-center select-none w-11 min-w-[44px] max-w-[44px]"
                style="width: 44px; min-width: 44px; max-width: 44px;"
              >
                <label class="relative inline-flex items-center cursor-pointer">
                  <input
                    type="checkbox"
                    checked={allSelected}
                    indeterminate={someSelected}
                    onchange={toggleSelectAll}
                    class="sr-only peer"
                  />
                  <div
                    class="w-4 h-4 rounded border border-white/20 peer-checked:border-violet-500 peer-checked:bg-violet-600 flex items-center justify-center transition bg-[#181926]"
                  >
                    {#if allSelected}
                      <svg class="w-3 h-3 text-white" viewBox="0 0 14 14" fill="none">
                        <path d="M3 7.5L5.5 10L11 4.5" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
                      </svg>
                    {:else if someSelected}
                      <div class="w-2 h-0.5 bg-violet-400 rounded-sm"></div>
                    {/if}
                  </div>
                </label>
              </th>

              <!-- Volume Name -->
              <th
                data-col="name"
                class="py-3 px-4 font-semibold relative select-none"
                style="width: {colWidths.name}px; min-width: {colWidths.name}px; max-width: {colWidths.name}px;"
              >
                <span>Volume Name</span>
                <button
                  type="button"
                  aria-label="Resize column"
                  tabindex="-1"
                  class="absolute right-0 top-0 bottom-0 w-2.5 cursor-col-resize z-20 flex items-center justify-center hover:bg-violet-500/20 active:bg-violet-500/40 group/resize bg-transparent border-0 p-0"
                  onmousedown={(e) => startResize('name', e)}
                  title="Drag to resize column"
                >
                  <div class="w-[2px] h-3.5 bg-white/15 group-hover/resize:bg-violet-400 group-hover/resize:h-full transition-all"></div>
                </button>
              </th>

              <!-- Attached Containers -->
              <th
                data-col="attached"
                class="py-3 px-4 font-semibold relative select-none"
                style="width: {colWidths.attached}px; min-width: {colWidths.attached}px; max-width: {colWidths.attached}px;"
              >
                <span>Attached Containers</span>
                <button
                  type="button"
                  aria-label="Resize column"
                  tabindex="-1"
                  class="absolute right-0 top-0 bottom-0 w-2.5 cursor-col-resize z-20 flex items-center justify-center hover:bg-violet-500/20 active:bg-violet-500/40 group/resize bg-transparent border-0 p-0"
                  onmousedown={(e) => startResize('attached', e)}
                  title="Drag to resize column"
                >
                  <div class="w-[2px] h-3.5 bg-white/15 group-hover/resize:bg-violet-400 group-hover/resize:h-full transition-all"></div>
                </button>
              </th>

              <!-- Size -->
              <th
                data-col="size"
                class="py-3 px-4 font-semibold relative select-none"
                style="width: {colWidths.size}px; min-width: {colWidths.size}px; max-width: {colWidths.size}px;"
              >
                <span>Size</span>
                <button
                  type="button"
                  aria-label="Resize column"
                  tabindex="-1"
                  class="absolute right-0 top-0 bottom-0 w-2.5 cursor-col-resize z-20 flex items-center justify-center hover:bg-violet-500/20 active:bg-violet-500/40 group/resize bg-transparent border-0 p-0"
                  onmousedown={(e) => startResize('size', e)}
                  title="Drag to resize column"
                >
                  <div class="w-[2px] h-3.5 bg-white/15 group-hover/resize:bg-violet-400 group-hover/resize:h-full transition-all"></div>
                </button>
              </th>

              <!-- Driver -->
              <th
                data-col="driver"
                class="py-3 px-4 font-semibold relative select-none"
                style="width: {colWidths.driver}px; min-width: {colWidths.driver}px; max-width: {colWidths.driver}px;"
              >
                <span>Driver</span>
                <button
                  type="button"
                  aria-label="Resize column"
                  tabindex="-1"
                  class="absolute right-0 top-0 bottom-0 w-2.5 cursor-col-resize z-20 flex items-center justify-center hover:bg-violet-500/20 active:bg-violet-500/40 group/resize bg-transparent border-0 p-0"
                  onmousedown={(e) => startResize('driver', e)}
                  title="Drag to resize column"
                >
                  <div class="w-[2px] h-3.5 bg-white/15 group-hover/resize:bg-violet-400 group-hover/resize:h-full transition-all"></div>
                </button>
              </th>

              <!-- Mountpoint -->
              <th
                data-col="mountpoint"
                class="py-3 px-4 font-semibold relative select-none"
                style="width: {colWidths.mountpoint}px; min-width: {colWidths.mountpoint}px; max-width: {colWidths.mountpoint}px;"
              >
                <span>Mountpoint</span>
                <button
                  type="button"
                  aria-label="Resize column"
                  tabindex="-1"
                  class="absolute right-0 top-0 bottom-0 w-2.5 cursor-col-resize z-20 flex items-center justify-center hover:bg-violet-500/20 active:bg-violet-500/40 group/resize bg-transparent border-0 p-0"
                  onmousedown={(e) => startResize('mountpoint', e)}
                  title="Drag to resize column"
                >
                  <div class="w-[2px] h-3.5 bg-white/15 group-hover/resize:bg-violet-400 group-hover/resize:h-full transition-all"></div>
                </button>
              </th>

              <!-- Created -->
              <th
                data-col="created"
                class="py-3 px-4 font-semibold relative select-none"
                style="width: {colWidths.created}px; min-width: {colWidths.created}px; max-width: {colWidths.created}px;"
              >
                <span>Created</span>
                <button
                  type="button"
                  aria-label="Resize column"
                  tabindex="-1"
                  class="absolute right-0 top-0 bottom-0 w-2.5 cursor-col-resize z-20 flex items-center justify-center hover:bg-violet-500/20 active:bg-violet-500/40 group/resize bg-transparent border-0 p-0"
                  onmousedown={(e) => startResize('created', e)}
                  title="Drag to resize column"
                >
                  <div class="w-[2px] h-3.5 bg-white/15 group-hover/resize:bg-violet-400 group-hover/resize:h-full transition-all"></div>
                </button>
              </th>

              <!-- Auto-expanding Flex Spacer to absorb excess width without scaling data columns -->
              <th class="p-0 border-0 pointer-events-none"></th>

              <!-- Actions (Sticky Right, Static Fixed Width) -->
              <th
                class="py-3 px-4 font-semibold text-right sticky right-0 z-20 bg-[#0c0d14] border-l border-white/[0.08] shadow-[-10px_0_15px_rgba(0,0,0,0.5)] whitespace-nowrap w-20 min-w-[80px] max-w-[80px] shrink-0"
                style="width: 80px; min-width: 80px; max-width: 80px;"
              >
                Actions
              </th>
            </tr>
          </thead>
          <tbody class="divide-y divide-white/[0.04]">
            {#each filteredVolumes as vol (vol.Name)}
              {@const attached = getContainersUsingVolume(vol.Name)}
              {@const isSelected = selectedNames.includes(vol.Name)}
              <tr class="hover:bg-white/[0.02] transition-colors group {isSelected ? 'bg-violet-950/20' : ''}">
                <!-- Row Checkbox -->
                <td
                  class="py-3 px-4 text-center w-11 min-w-[44px] max-w-[44px]"
                  style="width: 44px; min-width: 44px; max-width: 44px;"
                >
                  <label class="relative inline-flex items-center cursor-pointer">
                    <input
                      type="checkbox"
                      checked={isSelected}
                      onchange={() => toggleSelect(vol.Name)}
                      class="sr-only peer"
                    />
                    <div
                      class="w-4 h-4 rounded border border-white/20 peer-checked:border-violet-500 peer-checked:bg-violet-600 flex items-center justify-center transition bg-[#181926]"
                    >
                      {#if isSelected}
                        <svg class="w-3 h-3 text-white" viewBox="0 0 14 14" fill="none">
                          <path d="M3 7.5L5.5 10L11 4.5" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
                        </svg>
                      {/if}
                    </div>
                  </label>
                </td>

                <!-- Volume Name -->
                <td
                  class="py-3 px-4"
                  style="width: {colWidths.name}px; min-width: {colWidths.name}px; max-width: {colWidths.name}px;"
                >
                  <div class="flex items-center gap-2.5">
                    <div
                      class="w-8 h-8 rounded-xl bg-[#090a0f] border border-white/[0.08] flex items-center justify-center text-slate-400 group-hover:text-violet-400 group-hover:border-violet-500/30 transition shrink-0"
                    >
                      <HardDrive class="w-4 h-4" />
                    </div>
                    <div class="min-w-0 flex-1">
                      <div
                        class="flex items-center gap-1.5 font-mono font-semibold text-white tracking-wide"
                      >
                        <span class="truncate block max-w-full" title={vol.Name}>{vol.Name}</span>
                        <button
                          onclick={() =>
                            copyToClipboard(vol.Name, `vol-name-${vol.Name}`)}
                          class="p-1 rounded hover:bg-white/10 text-slate-500 hover:text-white transition shrink-0"
                          title="Copy volume name"
                          aria-label="Copy volume name"
                        >
                          {#if copiedId === `vol-name-${vol.Name}`}
                            <Check class="w-3 h-3 text-emerald-400" />
                          {:else}
                            <Copy class="w-3 h-3" />
                          {/if}
                        </button>
                      </div>



                      {#if vol.Labels && Object.keys(vol.Labels).length > 0}
                        <div class="flex flex-wrap gap-1 mt-1">
                          {#each Object.entries(vol.Labels) as [k, v]}
                            <span
                              class="inline-block px-1.5 py-0.2 rounded bg-white/5 text-slate-400 border border-white/5 font-mono text-[9px] truncate max-w-xs"
                              title="{k}={v}"
                            >
                              {k}={v}
                            </span>
                          {/each}
                        </div>
                      {/if}
                    </div>
                  </div>
                </td>

                <!-- Attached Containers -->
                <td
                  class="py-3 px-4"
                  style="width: {colWidths.attached}px; min-width: {colWidths.attached}px; max-width: {colWidths.attached}px;"
                >
                  {#if attached.length > 0}
                    <div class="flex flex-wrap items-center gap-1.5">
                      {#each attached as c}
                        {@const cName = c.Names?.[0]?.replace(/^\//, '') || c.Id.slice(0, 12)}
                        {@const isRunning = (c.State || '').toLowerCase() === 'running'}
                        <button
                          type="button"
                          onclick={(e) => {
                            e.stopPropagation();
                            onSelectContainer?.(c.Id);
                          }}
                          class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-lg bg-emerald-500/15 hover:bg-emerald-500/25 text-emerald-300 border border-emerald-500/30 text-[11px] font-mono transition cursor-pointer group/cbtn max-w-full"
                          title="Open container {cName} ({c.State})"
                        >
                          <span class="w-1.5 h-1.5 rounded-full {isRunning ? 'bg-emerald-400 shadow-[0_0_6px_#34d399]' : 'bg-slate-400'} shrink-0"></span>
                          <span class="truncate max-w-[130px]">{cName}</span>
                        </button>
                      {/each}
                    </div>
                  {:else}
                    <span class="text-slate-500 font-mono text-[11px]">Unused</span>
                  {/if}
                </td>

                <!-- Size -->
                <td
                  class="py-3 px-4 whitespace-nowrap"
                  style="width: {colWidths.size}px; min-width: {colWidths.size}px; max-width: {colWidths.size}px;"
                >
                  {#if vol.UsageData && vol.UsageData.Size !== undefined && vol.UsageData.Size >= 0}
                    <span
                      class="px-2 py-0.5 rounded-md font-mono text-[11px] font-semibold border {vol
                        .UsageData.Size > 1073741824
                        ? 'bg-amber-500/10 text-amber-300 border-amber-500/20'
                        : vol.UsageData.Size > 0
                          ? 'bg-white/[0.05] text-slate-200 border-white/[0.08]'
                          : 'bg-white/[0.02] text-slate-500 border-white/[0.04]'}"
                    >
                      {formatBytes(vol.UsageData.Size)}
                    </span>
                  {:else}
                    <span
                      class="text-slate-500 font-mono text-[11px]"
                      title="Usage data unavailable">—</span
                    >
                  {/if}
                </td>

                <!-- Driver -->
                <td
                  class="py-3 px-4 whitespace-nowrap"
                  style="width: {colWidths.driver}px; min-width: {colWidths.driver}px; max-width: {colWidths.driver}px;"
                >
                  <span
                    class="px-2 py-0.5 rounded-md bg-violet-500/10 text-violet-300 border border-violet-500/20 font-mono text-[10px]"
                  >
                    {vol.Driver || "local"}
                  </span>
                </td>

                <!-- Mountpoint (Compact preview by default, reveals full on hover) -->
                <td
                  class="py-3 px-4 font-mono text-slate-400"
                  style="width: {colWidths.mountpoint}px; min-width: {colWidths.mountpoint}px; max-width: {colWidths.mountpoint}px;"
                >
                  {#if vol.Mountpoint}
                    <button
                      type="button"
                      onmouseenter={(e) => showMountTooltip(vol.Mountpoint, vol.Name, e)}
                      onmouseleave={hideMountTooltip}
                      onclick={() => copyToClipboard(vol.Mountpoint, `vol-mount-${vol.Name}`)}
                      class="inline-flex items-center gap-1.5 px-2 py-1 rounded-lg bg-white/[0.03] hover:bg-violet-500/10 border border-white/[0.06] hover:border-violet-500/30 text-slate-400 hover:text-slate-200 transition text-[11px] font-mono cursor-pointer max-w-full text-left group/badge"
                      title={vol.Mountpoint}
                      aria-label="View full mountpoint path"
                    >
                      <Folder class="w-3.5 h-3.5 text-slate-500 group-hover/badge:text-violet-400 shrink-0" />
                      <span class="truncate">{getShortMountpoint(vol.Mountpoint)}</span>
                    </button>
                  {:else}
                    <span class="text-slate-500 font-mono text-[11px]">—</span>
                  {/if}
                </td>

                <!-- Created Date -->
                <td
                  class="py-3 px-4 font-mono text-slate-400 whitespace-nowrap"
                  style="width: {colWidths.created}px; min-width: {colWidths.created}px; max-width: {colWidths.created}px;"
                >
                  <div
                    class="flex items-center gap-1 text-[11px]"
                    title={vol.CreatedAt}
                  >
                    <Clock class="w-3 h-3 text-slate-500 shrink-0" />
                    <span>{formatDate(vol.CreatedAt)}</span>
                  </div>
                </td>

                <!-- Auto-expanding Flex Spacer -->
                <td class="p-0 border-0 pointer-events-none"></td>

                <!-- Actions (Sticky Right, Static Fixed Width) -->
                <td
                  class="py-3 px-4 text-right whitespace-nowrap sticky right-0 z-10 bg-[#12131c] group-hover:bg-[#181926] border-l border-white/[0.08] shadow-[-10px_0_15px_rgba(0,0,0,0.5)] w-20 min-w-[80px] max-w-[80px] shrink-0"
                  style="width: 80px; min-width: 80px; max-width: 80px;"
                >
                  <button
                    onclick={() => openDeleteModal(vol)}
                    class="p-1.5 rounded-lg bg-rose-500/10 hover:bg-rose-500/20 text-rose-400 border border-rose-500/20 hover:border-rose-500/40 transition cursor-pointer"
                    title="Delete volume"
                    aria-label="Delete volume"
                  >
                    <Trash class="w-3.5 h-3.5" />
                  </button>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  </div>
</div>

<!-- Create Volume Modal -->
<CreateVolumeModal
  isOpen={isCreateModalOpen}
  onClose={() => (isCreateModalOpen = false)}
  onCreated={(newVol) => {
    showFeedback("success", `Volume "${newVol.Name}" created successfully.`);
    fetchVolumes(true);
  }}
/>

<!-- Delete Single Volume Confirmation Modal -->
{#if deleteVolumeTarget}
  {@const attached = getContainersUsingVolume(deleteVolumeTarget.Name)}
  <div
    class="fixed inset-0 z-50 flex items-center justify-center p-4"
    role="dialog"
    aria-modal="true"
    aria-labelledby="delete-volume-modal-title"
    tabindex="-1"
  >
    <button
      type="button"
      class="fixed inset-0 bg-black/80 backdrop-blur-md cursor-default border-0"
      onclick={closeDeleteModal}
      aria-label="Close modal backdrop"
      tabindex="-1"
    ></button>

    <div
      class="relative w-full max-w-md rounded-2xl bg-[#12131c] border border-white/[0.12] shadow-[0_10px_50px_rgba(0,0,0,0.8)] overflow-hidden p-6 space-y-4 animate-in fade-in zoom-in-95 duration-150"
    >
      <div class="flex items-center gap-3">
        <div
          class="w-9 h-9 rounded-xl bg-rose-500/15 border border-rose-500/30 flex items-center justify-center text-rose-400 shadow-[0_0_15px_rgba(244,63,94,0.3)]"
        >
          <Trash class="w-4 h-4" />
        </div>
        <div>
          <h2
            id="delete-volume-modal-title"
            class="text-sm font-bold text-white"
          >
            Delete Docker Volume
          </h2>
          <p class="text-[11px] text-slate-400 font-mono truncate max-w-xs">
            {deleteVolumeTarget.Name}
          </p>
        </div>
      </div>

      {#if attached.length > 0}
        <div
          class="p-3 rounded-xl bg-amber-500/10 border border-amber-500/30 flex items-start gap-2 text-amber-300 text-xs"
        >
          <TriangleAlert class="w-4 h-4 text-amber-400 shrink-0 mt-0.5" />
          <div>
            <span>
              This volume is attached to <strong
                >{attached.length} container{attached.length === 1
                  ? ""
                  : "s"}</strong
              >:
            </span>
            <div class="flex flex-wrap gap-1 mt-1 font-mono text-[10px]">
              {#each attached as c}
                <span class="px-1.5 py-0.5 rounded bg-black/40 text-amber-200">
                  {c.Names?.[0]?.replace(/^\//, "") || c.Id.slice(0, 10)}
                </span>
              {/each}
            </div>
            <span class="block mt-1 text-[11px] text-amber-400">
              Docker will reject deleting a volume while containers reference
              it.
            </span>
          </div>
        </div>
      {/if}

      <div class="space-y-3 text-xs">
        <label
          class="flex items-center gap-2 cursor-pointer select-none text-slate-300 hover:text-white"
        >
          <input
            type="checkbox"
            bind:checked={deleteForce}
            class="rounded border-white/20 bg-[#0c0d14] text-violet-600 focus:ring-0 focus:ring-offset-0"
          />
          <span
            >Force delete (<code class="text-violet-300 font-mono">--force</code
            >)</span
          >
        </label>
      </div>

      <div
        class="flex items-center justify-end gap-2 pt-2 border-t border-white/[0.08]"
      >
        <button
          type="button"
          onclick={closeDeleteModal}
          disabled={isDeleting}
          class="px-4 py-2 rounded-xl bg-white/5 hover:bg-white/10 text-slate-300 text-xs font-medium transition cursor-pointer"
        >
          Cancel
        </button>
        <button
          type="button"
          onclick={confirmDeleteVolume}
          disabled={isDeleting}
          class="px-4 py-2 rounded-xl bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold shadow-[0_0_15px_rgba(244,63,94,0.35)] transition flex items-center gap-2 cursor-pointer"
        >
          {#if isDeleting}
            <LoaderCircle class="w-3.5 h-3.5 animate-spin" />
            Deleting...
          {:else}
            <Trash class="w-3.5 h-3.5" />
            Delete Volume
          {/if}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Batch Delete Volumes Modal -->
{#if isBatchDeleteModalOpen}
  <div
    class="fixed inset-0 z-50 flex items-center justify-center p-4"
    role="dialog"
    aria-modal="true"
    aria-labelledby="batch-delete-volumes-title"
    tabindex="-1"
  >
    <button
      type="button"
      class="fixed inset-0 bg-black/80 backdrop-blur-md cursor-default border-0"
      onclick={() => (isBatchDeleteModalOpen = false)}
      aria-label="Close modal backdrop"
      tabindex="-1"
    ></button>

    <div
      class="relative w-full max-w-lg rounded-2xl bg-[#12131c] border border-white/[0.12] shadow-[0_10px_50px_rgba(0,0,0,0.8)] overflow-hidden p-6 space-y-4 animate-in fade-in zoom-in-95 duration-150"
    >
      <div class="flex items-center gap-3">
        <div
          class="w-9 h-9 rounded-xl bg-rose-500/15 border border-rose-500/30 flex items-center justify-center text-rose-400 shadow-[0_0_15px_rgba(244,63,94,0.3)]"
        >
          <Trash class="w-4 h-4" />
        </div>
        <div>
          <h2
            id="batch-delete-volumes-title"
            class="text-sm font-bold text-white"
          >
            Delete Selected Volumes ({selectedNames.length})
          </h2>
          <p class="text-[11px] text-slate-400">
            Permanently remove {selectedNames.length} persistent volume{selectedNames.length === 1 ? '' : 's'}
          </p>
        </div>
      </div>

      {#if selectedInUse.length > 0}
        <div
          class="p-3.5 rounded-xl bg-amber-500/10 border border-amber-500/30 space-y-2 text-amber-300 text-xs"
        >
          <div class="flex items-center gap-2 font-semibold">
            <TriangleAlert class="w-4 h-4 text-amber-400 shrink-0" />
            <span>
              {selectedInUse.length} of the selected volumes {selectedInUse.length === 1 ? 'is' : 'are'} in use by containers!
            </span>
          </div>
          <div class="max-h-28 overflow-y-auto space-y-1 font-mono text-[10px]">
            {#each selectedInUse as vol}
              {@const usingConts = getContainersUsingVolume(vol.Name)}
              <div class="p-1.5 rounded bg-black/40 flex items-center justify-between gap-2">
                <span class="truncate text-white">{vol.Name}</span>
                <span class="text-amber-400 shrink-0">
                  {usingConts.length} container{usingConts.length === 1 ? '' : 's'}
                </span>
              </div>
            {/each}
          </div>
        </div>
      {/if}

      <div class="space-y-2">
        <span class="text-xs text-slate-400 block font-mono">Volumes to delete:</span>
        <div class="max-h-36 overflow-y-auto space-y-1 p-2 rounded-xl bg-black/40 border border-white/5 font-mono text-xs">
          {#each selectedNames as name}
            <div class="flex items-center justify-between text-slate-300">
              <span class="truncate">{name}</span>
            </div>
          {/each}
        </div>
      </div>

      <div class="space-y-3 text-xs">
        <label
          class="flex items-center gap-2 cursor-pointer select-none text-slate-300 hover:text-white"
        >
          <input
            type="checkbox"
            bind:checked={batchForce}
            class="rounded border-white/20 bg-[#0c0d14] text-violet-600 focus:ring-0 focus:ring-offset-0"
          />
          <span
            >Force delete (<code class="text-violet-300 font-mono">--force</code
            >)</span
          >
        </label>
      </div>

      <div
        class="flex items-center justify-end gap-2 pt-2 border-t border-white/[0.08]"
      >
        <button
          type="button"
          onclick={() => (isBatchDeleteModalOpen = false)}
          disabled={isBatchDeleting}
          class="px-4 py-2 rounded-xl bg-white/5 hover:bg-white/10 text-slate-300 text-xs font-medium transition cursor-pointer"
        >
          Cancel
        </button>
        <button
          type="button"
          onclick={confirmBatchDelete}
          disabled={isBatchDeleting}
          class="px-4 py-2 rounded-xl bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold shadow-[0_0_15px_rgba(244,63,94,0.35)] transition flex items-center gap-2 cursor-pointer"
        >
          {#if isBatchDeleting}
            <LoaderCircle class="w-3.5 h-3.5 animate-spin" />
            Deleting ({selectedNames.length})...
          {:else}
            <Trash class="w-3.5 h-3.5" />
            Delete {selectedNames.length} Volume{selectedNames.length === 1 ? '' : 's'}
          {/if}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Prune Volumes Modal -->
{#if isPruneModalOpen}
  <div
    class="fixed inset-0 z-50 flex items-center justify-center p-4"
    role="dialog"
    aria-modal="true"
    aria-labelledby="prune-volumes-modal-title"
    tabindex="-1"
  >
    <button
      type="button"
      class="fixed inset-0 bg-black/80 backdrop-blur-md cursor-default border-0"
      onclick={() => (isPruneModalOpen = false)}
      aria-label="Close modal backdrop"
      tabindex="-1"
    ></button>

    <div
      class="relative w-full max-w-md rounded-2xl bg-[#12131c] border border-white/[0.12] shadow-[0_10px_50px_rgba(0,0,0,0.8)] overflow-hidden p-6 space-y-4 animate-in fade-in zoom-in-95 duration-150"
    >
      <div class="flex items-center gap-3">
        <div
          class="w-9 h-9 rounded-xl bg-amber-500/15 border border-amber-500/30 flex items-center justify-center text-amber-400 shadow-[0_0_15px_rgba(245,158,11,0.3)]"
        >
          <Scissors class="w-4 h-4" />
        </div>
        <div>
          <h2
            id="prune-volumes-modal-title"
            class="text-sm font-bold text-white"
          >
            Prune Unused Volumes
          </h2>
          <p class="text-[11px] text-slate-400">
            Reclaim disk space by purging unreferenced volumes
          </p>
        </div>
      </div>

      <p class="text-xs text-slate-300 leading-relaxed">
        This will delete all local Docker volumes not currently referenced by at
        least one container.
        <strong class="text-amber-300">Warning:</strong> Data inside pruned volumes
        cannot be recovered!
      </p>

      <div
        class="flex items-center justify-end gap-2 pt-2 border-t border-white/[0.08]"
      >
        <button
          type="button"
          onclick={() => (isPruneModalOpen = false)}
          disabled={isPruning}
          class="px-4 py-2 rounded-xl bg-white/5 hover:bg-white/10 text-slate-300 text-xs font-medium transition cursor-pointer"
        >
          Cancel
        </button>
        <button
          type="button"
          onclick={confirmPrune}
          disabled={isPruning}
          class="px-4 py-2 rounded-xl bg-amber-600 hover:bg-amber-500 text-white text-xs font-semibold shadow-[0_0_15px_rgba(245,158,11,0.35)] transition flex items-center gap-2 cursor-pointer"
        >
          {#if isPruning}
            <LoaderCircle class="w-3.5 h-3.5 animate-spin" />
            Pruning...
          {:else}
            <Scissors class="w-3.5 h-3.5" />
            Prune Volumes
          {/if}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Mountpoint Floating Hover Popover -->
{#if activeTooltip}
  <div
    role="tooltip"
    class="fixed z-50 flex flex-col gap-1.5 p-3 rounded-xl bg-[#0d0e16]/95 border border-white/20 shadow-[0_12px_36px_rgba(0,0,0,0.9)] backdrop-blur-xl min-w-[280px] max-w-lg transition-all animate-in fade-in duration-150 {activeTooltip.showAbove ? '-translate-y-full' : ''}"
    style="left: {activeTooltip.x}px; top: {activeTooltip.y}px;"
    onmouseenter={cancelHideTooltip}
    onmouseleave={hideMountTooltip}
  >
    <div class="flex items-center justify-between gap-3 pb-1.5 border-b border-white/[0.08]">
      <div class="flex items-center gap-1.5 text-[10px] font-mono uppercase tracking-wider text-violet-400">
        <Folder class="w-3.5 h-3.5" />
        <span>Full Mountpoint Path</span>
      </div>
      <button
        type="button"
        onclick={() => copyToClipboard(activeTooltip!.path, `vol-mount-${activeTooltip!.name}`)}
        class="inline-flex items-center gap-1 px-2 py-0.5 rounded-md bg-white/5 hover:bg-white/10 text-slate-300 hover:text-white text-[10px] font-mono transition cursor-pointer"
        title="Copy path to clipboard"
      >
        {#if copiedId === `vol-mount-${activeTooltip.name}`}
          <Check class="w-3 h-3 text-emerald-400" />
          <span class="text-emerald-400 font-semibold">Copied!</span>
        {:else}
          <Copy class="w-3 h-3 text-slate-400" />
          <span>Copy</span>
        {/if}
      </button>
    </div>
    <div class="font-mono text-xs text-slate-200 break-all select-all leading-relaxed p-2 rounded-lg bg-black/40 border border-white/5">
      {activeTooltip.path}
    </div>
  </div>
{/if}
