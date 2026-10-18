<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import {
    HardDrive,
    Database,
    Plus,
    Search,
    RefreshCw,
    Trash2,
    Scissors,
    AlertCircle,
    CheckCircle2,
    Loader2,
    Copy,
    Check,
    Layers,
    Clock,
    AlertTriangle,
    X,
    Filter,
    Folder,
    Box,
    ExternalLink,
    ArrowUpDown,
  } from '@lucide/svelte';
  import { volumeApi, containerApi } from '../lib/api';
  import type { Volume, Container } from '../lib/types';
  import CreateVolumeModal from '../lib/components/CreateVolumeModal.svelte';

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
  let feedback = $state<{ type: 'success' | 'error'; message: string } | null>(null);

  // Search & Filters
  let searchQuery = $state('');
  let driverFilter = $state<string>('all');
  let sortBy = $state<'name' | 'size-desc' | 'size-asc' | 'created'>('name');

  // Modals
  let isCreateModalOpen = $state(false);
  let isPruneModalOpen = $state(false);
  let isPruning = $state(false);

  let deleteVolumeTarget = $state<Volume | null>(null);
  let deleteForce = $state(false);
  let isDeleting = $state(false);

  // Clipboard & Timers
  let copiedId = $state<string | null>(null);
  let refreshTimer: ReturnType<typeof setInterval> | null = null;

  function showFeedback(type: 'success' | 'error', message: string) {
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
    } catch (err: unknown) {
      if (err instanceof Error) {
        error = err.message;
      } else {
        error = 'Failed to load Docker volumes.';
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
  });

  onDestroy(() => {
    if (refreshTimer) clearInterval(refreshTimer);
  });

  function formatBytes(bytes: number): string {
    if (!bytes || bytes <= 0) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(1024));
    return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`;
  }

  function formatDate(dateStr?: string): string {
    if (!dateStr) return 'Unknown';
    try {
      const d = new Date(dateStr);
      if (isNaN(d.getTime())) return dateStr;
      return d.toLocaleDateString(undefined, {
        year: 'numeric',
        month: 'short',
        day: 'numeric',
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
    return containers.filter(c =>
      c.Mounts?.some(m => m.Name === volName || m.Source?.includes(volName))
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
    const list = volumes.filter(v => {
      if (driverFilter !== 'all' && v.Driver !== driverFilter) {
        return false;
      }
      if (!searchQuery.trim()) return true;
      const q = searchQuery.toLowerCase().trim();
      const nameMatch = v.Name.toLowerCase().includes(q);
      const driverMatch = (v.Driver || '').toLowerCase().includes(q);
      const mountMatch = (v.Mountpoint || '').toLowerCase().includes(q);
      return nameMatch || driverMatch || mountMatch;
    });

    return list.slice().sort((a, b) => {
      if (sortBy === 'size-desc') {
        const sA = a.UsageData?.Size ?? -1;
        const sB = b.UsageData?.Size ?? -1;
        return sB - sA;
      }
      if (sortBy === 'size-asc') {
        const sA = a.UsageData?.Size ?? -1;
        const sB = b.UsageData?.Size ?? -1;
        return sA - sB;
      }
      if (sortBy === 'created') {
        const tA = a.CreatedAt ? new Date(a.CreatedAt).getTime() : 0;
        const tB = b.CreatedAt ? new Date(b.CreatedAt).getTime() : 0;
        return tB - tA;
      }
      return a.Name.localeCompare(b.Name);
    });
  });

  // Actions
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
      showFeedback('success', `Volume "${deleteVolumeTarget.Name}" removed.`);
      closeDeleteModal();
      await fetchVolumes(true);
    } catch (err: unknown) {
      if (err instanceof Error) {
        showFeedback('error', `Failed to remove volume: ${err.message}`);
      } else {
        showFeedback('error', 'Failed to remove volume.');
      }
    } finally {
      isDeleting = false;
    }
  }

  async function confirmPrune() {
    isPruning = true;
    try {
      const res = await volumeApi.prune();
      const count = res.VolumesDeleted?.length || 0;
      const space = formatBytes(res.SpaceReclaimed || 0);
      showFeedback('success', `Pruned ${count} unused volume${count === 1 ? '' : 's'}. Reclaimed ${space}.`);
      isPruneModalOpen = false;
      await fetchVolumes(true);
    } catch (err: unknown) {
      if (err instanceof Error) {
        showFeedback('error', `Failed to prune volumes: ${err.message}`);
      } else {
        showFeedback('error', 'Failed to prune volumes.');
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
      <div class="w-10 h-10 rounded-2xl bg-gradient-to-br from-violet-600/30 to-indigo-700/20 border border-violet-500/40 flex items-center justify-center text-violet-400 shadow-[0_0_20px_rgba(124,58,237,0.3)]">
        <HardDrive class="w-5 h-5" />
      </div>
      <div>
        <div class="flex items-center gap-2">
          <h1 class="text-xl font-bold tracking-tight text-white">Volumes</h1>
          <span class="px-2 py-0.5 text-[10px] font-mono rounded-full bg-violet-500/20 text-violet-300 border border-violet-500/30 font-semibold">
            {volumes.length}
          </span>
        </div>
        <p class="text-xs text-slate-400">
          Persistent storage directories managed independently of container lifecycles
        </p>
      </div>
    </div>

    <!-- Header Actions -->
    <div class="flex items-center gap-2.5">
      <button
        onclick={() => isPruneModalOpen = true}
        class="px-3.5 py-2 rounded-xl bg-white/[0.04] hover:bg-white/[0.08] text-slate-300 hover:text-white border border-white/[0.08] text-xs font-medium transition flex items-center gap-2"
        title="Prune unused volumes"
      >
        <Scissors class="w-3.5 h-3.5 text-amber-400" />
        <span>Prune Unused</span>
      </button>

      <button
        onclick={() => fetchVolumes()}
        disabled={isRefreshing}
        class="p-2 rounded-xl bg-white/[0.04] hover:bg-white/[0.08] text-slate-400 hover:text-white border border-white/[0.08] transition disabled:opacity-50"
        title="Refresh volume list"
        aria-label="Refresh volumes"
      >
        <RefreshCw class="w-4 h-4 {isRefreshing ? 'animate-spin text-violet-400' : ''}" />
      </button>

      <button
        onclick={() => isCreateModalOpen = true}
        class="px-4 py-2 rounded-xl bg-violet-600 hover:bg-violet-500 text-white text-xs font-semibold shadow-[0_0_20px_rgba(124,58,237,0.4)] transition flex items-center gap-2 group"
      >
        <Plus class="w-3.5 h-3.5 group-hover:scale-110 transition-transform" />
        Create Volume
      </button>
    </div>
  </div>

  <!-- Feedback Toast / Banner -->
  {#if feedback}
    <div
      class="p-3.5 rounded-xl border flex items-center justify-between text-xs animate-in fade-in duration-200 {feedback.type === 'success'
        ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-300'
        : 'bg-rose-500/10 border-rose-500/30 text-rose-300'}"
    >
      <div class="flex items-center gap-2">
        {#if feedback.type === 'success'}
          <CheckCircle2 class="w-4 h-4 text-emerald-400 shrink-0" />
        {:else}
          <AlertCircle class="w-4 h-4 text-rose-400 shrink-0" />
        {/if}
        <span>{feedback.message}</span>
      </div>
      <button
        onclick={() => feedback = null}
        class="p-1 hover:bg-white/10 rounded-lg text-slate-400 hover:text-white"
        aria-label="Close notification"
      >
        <X class="w-3.5 h-3.5" />
      </button>
    </div>
  {/if}

  {#if error}
    <div class="p-4 rounded-xl bg-rose-500/10 border border-rose-500/30 flex items-start gap-3 text-rose-300">
      <AlertCircle class="w-5 h-5 shrink-0 text-rose-400 mt-0.5" />
      <div>
        <h3 class="text-xs font-bold text-rose-200">Volume Service Error</h3>
        <p class="text-xs mt-0.5">{error}</p>
      </div>
    </div>
  {/if}

  <!-- Stats Strip -->
  <div class="grid grid-cols-2 lg:grid-cols-4 gap-3">
    <div class="p-4 rounded-xl bg-[#12131c] border border-white/[0.08] flex items-center gap-3">
      <div class="w-9 h-9 rounded-xl bg-violet-500/10 border border-violet-500/20 flex items-center justify-center text-violet-400">
        <HardDrive class="w-4 h-4" />
      </div>
      <div>
        <span class="text-[11px] text-slate-400 block font-mono">Total Volumes</span>
        <span class="text-lg font-bold text-white font-mono">{volumes.length}</span>
      </div>
    </div>

    <div class="p-4 rounded-xl bg-[#12131c] border border-white/[0.08] flex items-center gap-3">
      <div class="w-9 h-9 rounded-xl bg-amber-500/10 border border-amber-500/20 flex items-center justify-center text-amber-400">
        <Database class="w-4 h-4" />
      </div>
      <div>
        <span class="text-[11px] text-slate-400 block font-mono">Total Storage</span>
        <span class="text-lg font-bold text-amber-400 font-mono">
          {formatBytes(totalVolumeBytes)}
        </span>
      </div>
    </div>

    <div class="p-4 rounded-xl bg-[#12131c] border border-white/[0.08] flex items-center gap-3">
      <div class="w-9 h-9 rounded-xl bg-cyan-500/10 border border-cyan-500/20 flex items-center justify-center text-cyan-400">
        <Folder class="w-4 h-4" />
      </div>
      <div>
        <span class="text-[11px] text-slate-400 block font-mono">Local Driver</span>
        <span class="text-lg font-bold text-cyan-400 font-mono">
          {volumes.filter(v => (v.Driver || 'local') === 'local').length}
        </span>
      </div>
    </div>

    <div class="p-4 rounded-xl bg-[#12131c] border border-white/[0.08] flex items-center gap-3">
      <div class="w-9 h-9 rounded-xl bg-emerald-500/10 border border-emerald-500/20 flex items-center justify-center text-emerald-400">
        <Box class="w-4 h-4" />
      </div>
      <div>
        <span class="text-[11px] text-slate-400 block font-mono">In-Use Volumes</span>
        <span class="text-lg font-bold text-emerald-400 font-mono">
          {volumes.filter(v => getContainersUsingVolume(v.Name).length > 0).length}
        </span>
      </div>
    </div>
  </div>

  <!-- Search & Filter Controls -->
  <div class="p-3 rounded-2xl bg-[#12131c] border border-white/[0.08] flex flex-col md:flex-row items-center justify-between gap-3">
    <!-- Search Bar -->
    <div class="relative w-full md:w-96">
      <Search class="w-4 h-4 text-slate-500 absolute left-3.5 top-1/2 -translate-y-1/2" />
      <input
        type="text"
        bind:value={searchQuery}
        placeholder="Filter by volume name, driver, or mount..."
        class="w-full pl-9 pr-8 py-2 rounded-xl bg-[#0c0d14] border border-white/10 text-white placeholder-slate-500 text-xs focus:outline-none focus:border-violet-500 transition font-mono"
      />
      {#if searchQuery}
        <button
          onclick={() => searchQuery = ''}
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
  <div class="rounded-2xl bg-[#12131c] border border-white/[0.08] overflow-hidden shadow-xl">
    {#if isLoading}
      <div class="p-12 flex flex-col items-center justify-center gap-3 text-slate-400">
        <Loader2 class="w-7 h-7 text-violet-400 animate-spin" />
        <span class="text-xs font-mono">Querying Docker Engine volumes...</span>
      </div>
    {:else if filteredVolumes.length === 0}
      <div class="p-12 text-center space-y-3">
        <div class="w-12 h-12 mx-auto rounded-2xl bg-white/[0.04] border border-white/[0.08] flex items-center justify-center text-slate-500">
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
            onclick={() => isCreateModalOpen = true}
            class="px-4 py-2 rounded-xl bg-violet-600 hover:bg-violet-500 text-white text-xs font-semibold shadow-[0_0_15px_rgba(124,58,237,0.35)] transition inline-flex items-center gap-2"
          >
            <Plus class="w-3.5 h-3.5" />
            Create Your First Volume
          </button>
        </div>
      </div>
    {:else}
      <div class="overflow-x-auto">
        <table class="w-full text-left border-collapse text-xs">
          <thead>
            <tr class="border-b border-white/[0.08] bg-[#0c0d14]/60 text-slate-400 font-mono text-[11px] uppercase tracking-wider">
              <th class="py-3 px-4 font-semibold">Volume Name</th>
              <th class="py-3 px-4 font-semibold">Driver</th>
              <th class="py-3 px-4 font-semibold">Size</th>
              <th class="py-3 px-4 font-semibold">Mountpoint</th>
              <th class="py-3 px-4 font-semibold">Scope</th>
              <th class="py-3 px-4 font-semibold">Created</th>
              <th class="py-3 px-4 font-semibold">Attached</th>
              <th class="py-3 px-4 font-semibold text-right">Actions</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-white/[0.04]">
            {#each filteredVolumes as vol (vol.Name)}
              {@const attached = getContainersUsingVolume(vol.Name)}
              <tr class="hover:bg-white/[0.02] transition-colors group">
                <!-- Volume Name -->
                <td class="py-3 px-4">
                  <div class="flex items-center gap-2.5">
                    <div class="w-8 h-8 rounded-xl bg-[#090a0f] border border-white/[0.08] flex items-center justify-center text-slate-400 group-hover:text-violet-400 group-hover:border-violet-500/30 transition shrink-0">
                      <HardDrive class="w-4 h-4" />
                    </div>
                    <div class="min-w-0">
                      <div class="flex items-center gap-1.5 font-mono font-semibold text-white tracking-wide">
                        <span class="truncate max-w-xs">{vol.Name}</span>
                        <button
                          onclick={() => copyToClipboard(vol.Name, `vol-name-${vol.Name}`)}
                          class="p-1 rounded hover:bg-white/10 text-slate-500 hover:text-white transition"
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
                            <span class="inline-block px-1.5 py-0.2 rounded bg-white/5 text-slate-400 border border-white/5 font-mono text-[9px]">
                              {k}={v}
                            </span>
                          {/each}
                        </div>
                      {/if}
                    </div>
                  </div>
                </td>

                <!-- Driver -->
                <td class="py-3 px-4 whitespace-nowrap">
                  <span class="px-2 py-0.5 rounded-md bg-violet-500/10 text-violet-300 border border-violet-500/20 font-mono text-[10px]">
                    {vol.Driver || 'local'}
                  </span>
                </td>

                <!-- Size -->
                <td class="py-3 px-4 whitespace-nowrap">
                  {#if vol.UsageData && vol.UsageData.Size !== undefined && vol.UsageData.Size >= 0}
                    <span
                      class="px-2 py-0.5 rounded-md font-mono text-[11px] font-semibold border {vol.UsageData.Size > 1073741824
                        ? 'bg-amber-500/10 text-amber-300 border-amber-500/20'
                        : vol.UsageData.Size > 0
                          ? 'bg-white/[0.05] text-slate-200 border-white/[0.08]'
                          : 'bg-white/[0.02] text-slate-500 border-white/[0.04]'}"
                    >
                      {formatBytes(vol.UsageData.Size)}
                    </span>
                  {:else}
                    <span class="text-slate-500 font-mono text-[11px]" title="Usage data unavailable">—</span>
                  {/if}
                </td>

                <!-- Mountpoint -->
                <td class="py-3 px-4 font-mono text-slate-400">
                  <div class="flex items-center gap-1.5 max-w-xs" title={vol.Mountpoint}>
                    <span class="truncate text-slate-300">{vol.Mountpoint || '—'}</span>
                    {#if vol.Mountpoint}
                      <button
                        onclick={() => copyToClipboard(vol.Mountpoint, `vol-mount-${vol.Name}`)}
                        class="p-1 rounded hover:bg-white/10 text-slate-500 hover:text-white transition shrink-0"
                        title="Copy mountpoint"
                        aria-label="Copy mountpoint"
                      >
                        {#if copiedId === `vol-mount-${vol.Name}`}
                          <Check class="w-3 h-3 text-emerald-400" />
                        {:else}
                          <Copy class="w-3 h-3" />
                        {/if}
                      </button>
                    {/if}
                  </div>
                </td>

                <!-- Scope -->
                <td class="py-3 px-4 font-mono text-slate-400 whitespace-nowrap">
                  <span class="text-slate-300">{vol.Scope || 'local'}</span>
                </td>

                <!-- Created Date -->
                <td class="py-3 px-4 font-mono text-slate-400 whitespace-nowrap">
                  <div class="flex items-center gap-1 text-[11px]" title={vol.CreatedAt}>
                    <Clock class="w-3 h-3 text-slate-500" />
                    <span>{formatDate(vol.CreatedAt)}</span>
                  </div>
                </td>

                <!-- Attached Containers -->
                <td class="py-3 px-4 whitespace-nowrap">
                  {#if attached.length > 0}
                    <div class="flex items-center gap-1.5">
                      <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 font-mono text-[10px]">
                        <span class="w-1.5 h-1.5 rounded-full bg-emerald-400"></span>
                        {attached.length} container{attached.length === 1 ? '' : 's'}
                      </span>
                    </div>
                  {:else}
                    <span class="text-slate-500 font-mono text-[11px]">Unused</span>
                  {/if}
                </td>

                <!-- Actions -->
                <td class="py-3 px-4 text-right whitespace-nowrap">
                  <button
                    onclick={() => openDeleteModal(vol)}
                    class="p-1.5 rounded-lg bg-rose-500/10 hover:bg-rose-500/20 text-rose-400 border border-rose-500/20 hover:border-rose-500/40 transition"
                    title="Delete volume"
                    aria-label="Delete volume"
                  >
                    <Trash2 class="w-3.5 h-3.5" />
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
  onClose={() => isCreateModalOpen = false}
  onCreated={(newVol) => {
    showFeedback('success', `Volume "${newVol.Name}" created successfully.`);
    fetchVolumes(true);
  }}
/>

<!-- Delete Volume Confirmation Modal -->
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
        <div class="w-9 h-9 rounded-xl bg-rose-500/15 border border-rose-500/30 flex items-center justify-center text-rose-400 shadow-[0_0_15px_rgba(244,63,94,0.3)]">
          <Trash2 class="w-4 h-4" />
        </div>
        <div>
          <h2 id="delete-volume-modal-title" class="text-sm font-bold text-white">Delete Docker Volume</h2>
          <p class="text-[11px] text-slate-400 font-mono truncate max-w-xs">{deleteVolumeTarget.Name}</p>
        </div>
      </div>

      {#if attached.length > 0}
        <div class="p-3 rounded-xl bg-amber-500/10 border border-amber-500/30 flex items-start gap-2 text-amber-300 text-xs">
          <AlertTriangle class="w-4 h-4 text-amber-400 shrink-0 mt-0.5" />
          <div>
            <span>
              This volume is attached to <strong>{attached.length} container{attached.length === 1 ? '' : 's'}</strong>:
            </span>
            <div class="flex flex-wrap gap-1 mt-1 font-mono text-[10px]">
              {#each attached as c}
                <span class="px-1.5 py-0.5 rounded bg-black/40 text-amber-200">
                  {c.Names?.[0]?.replace(/^\//, '') || c.Id.slice(0, 10)}
                </span>
              {/each}
            </div>
            <span class="block mt-1 text-[11px] text-amber-400">
              Docker will reject deleting a volume while containers reference it.
            </span>
          </div>
        </div>
      {/if}

      <div class="space-y-3 text-xs">
        <label class="flex items-center gap-2 cursor-pointer select-none text-slate-300 hover:text-white">
          <input
            type="checkbox"
            bind:checked={deleteForce}
            class="rounded border-white/20 bg-[#0c0d14] text-violet-600 focus:ring-0 focus:ring-offset-0"
          />
          <span>Force delete (<code class="text-violet-300 font-mono">--force</code>)</span>
        </label>
      </div>

      <div class="flex items-center justify-end gap-2 pt-2 border-t border-white/[0.08]">
        <button
          type="button"
          onclick={closeDeleteModal}
          disabled={isDeleting}
          class="px-4 py-2 rounded-xl bg-white/5 hover:bg-white/10 text-slate-300 text-xs font-medium transition"
        >
          Cancel
        </button>
        <button
          type="button"
          onclick={confirmDeleteVolume}
          disabled={isDeleting}
          class="px-4 py-2 rounded-xl bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold shadow-[0_0_15px_rgba(244,63,94,0.35)] transition flex items-center gap-2"
        >
          {#if isDeleting}
            <Loader2 class="w-3.5 h-3.5 animate-spin" />
            Deleting...
          {:else}
            <Trash2 class="w-3.5 h-3.5" />
            Delete Volume
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
      onclick={() => isPruneModalOpen = false}
      aria-label="Close modal backdrop"
      tabindex="-1"
    ></button>

    <div
      class="relative w-full max-w-md rounded-2xl bg-[#12131c] border border-white/[0.12] shadow-[0_10px_50px_rgba(0,0,0,0.8)] overflow-hidden p-6 space-y-4 animate-in fade-in zoom-in-95 duration-150"
    >
      <div class="flex items-center gap-3">
        <div class="w-9 h-9 rounded-xl bg-amber-500/15 border border-amber-500/30 flex items-center justify-center text-amber-400 shadow-[0_0_15px_rgba(245,158,11,0.3)]">
          <Scissors class="w-4 h-4" />
        </div>
        <div>
          <h2 id="prune-volumes-modal-title" class="text-sm font-bold text-white">Prune Unused Volumes</h2>
          <p class="text-[11px] text-slate-400">Reclaim disk space by purging unreferenced volumes</p>
        </div>
      </div>

      <p class="text-xs text-slate-300 leading-relaxed">
        This will delete all local Docker volumes not currently referenced by at least one container. 
        <strong class="text-amber-300">Warning:</strong> Data inside pruned volumes cannot be recovered!
      </p>

      <div class="flex items-center justify-end gap-2 pt-2 border-t border-white/[0.08]">
        <button
          type="button"
          onclick={() => isPruneModalOpen = false}
          disabled={isPruning}
          class="px-4 py-2 rounded-xl bg-white/5 hover:bg-white/10 text-slate-300 text-xs font-medium transition"
        >
          Cancel
        </button>
        <button
          type="button"
          onclick={confirmPrune}
          disabled={isPruning}
          class="px-4 py-2 rounded-xl bg-amber-600 hover:bg-amber-500 text-white text-xs font-semibold shadow-[0_0_15px_rgba(245,158,11,0.35)] transition flex items-center gap-2"
        >
          {#if isPruning}
            <Loader2 class="w-3.5 h-3.5 animate-spin" />
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
