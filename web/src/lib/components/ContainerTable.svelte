<script lang="ts">
  import {
    Play,
    Square,
    RotateCw,
    Pause,
    Trash2,
    ExternalLink,
    Box,
    Skull,
    Eye,
    Plus,
    Loader2,
    Copy,
    Sliders,
    Layers,
    Sparkles,
    ArrowUpCircle,
    CheckCircle2,
    ArrowUpDown,
    ArrowUp,
    ArrowDown,
  } from '@lucide/svelte';
  import { imageUpdateStore } from '../stores/imageUpdates.svelte';
  import type { Container } from '../types';

  export type ContainerSortKey = 'name' | 'status' | 'image' | 'created';
  export type ContainerSortDirection = 'asc' | 'desc';

  interface Props {
    containers: Container[];
    selectedIds?: string[];
    actionLoading?: Record<string, string>; // containerId -> action name
    sortKey?: ContainerSortKey;
    sortDirection?: ContainerSortDirection;
    onSort?: (key: ContainerSortKey) => void;
    onSelect?: (containerId: string) => void;
    onToggleSelect?: (containerId: string) => void;
    onToggleSelectAll?: () => void;
    onAction?: (action: 'start' | 'stop' | 'restart' | 'pause' | 'unpause' | 'kill' | 'remove', containerId: string) => void;
    onDuplicate?: (container: Container) => void;
    onEdit?: (container: Container) => void;
    onUpdateContainer?: (container: Container) => void;
    onCreateClick?: () => void;
  }

  let {
    containers = [],
    selectedIds = [],
    actionLoading = {},
    sortKey,
    sortDirection,
    onSort,
    onSelect,
    onToggleSelect,
    onToggleSelectAll,
    onAction,
    onDuplicate,
    onEdit,
    onUpdateContainer,
    onCreateClick,
  }: Props = $props();

  let internalSortKey = $state<ContainerSortKey>('created');
  let internalSortDirection = $state<ContainerSortDirection>('desc');

  let effectiveSortKey = $derived(sortKey ?? internalSortKey);
  let effectiveSortDirection = $derived(sortDirection ?? internalSortDirection);

  function handleHeaderSort(key: ContainerSortKey) {
    if (onSort) {
      onSort(key);
    } else {
      if (internalSortKey === key) {
        internalSortDirection = internalSortDirection === 'asc' ? 'desc' : 'asc';
      } else {
        internalSortKey = key;
        internalSortDirection = key === 'created' ? 'desc' : 'asc';
      }
    }
  }

  let displayedContainers = $derived.by(() => {
    if (onSort) return containers;
    const list = [...containers];
    list.sort((a, b) => {
      let res = 0;
      if (effectiveSortKey === 'name') {
        const nameA = getCleanName(a.Names);
        const nameB = getCleanName(b.Names);
        res = nameA.localeCompare(nameB);
      } else if (effectiveSortKey === 'status') {
        res = (a.State || '').localeCompare(b.State || '');
      } else if (effectiveSortKey === 'image') {
        res = (a.Image || '').localeCompare(b.Image || '');
      } else if (effectiveSortKey === 'created') {
        res = (a.Created || 0) - (b.Created || 0);
      }
      return effectiveSortDirection === 'asc' ? res : -res;
    });
    return list;
  });

  let allSelected = $derived(
    containers.length > 0 && containers.every(c => selectedIds.includes(c.Id))
  );

  let someSelected = $derived(
    containers.some(c => selectedIds.includes(c.Id)) && !allSelected
  );

  function getCleanName(names?: string[]): string {
    if (!names || names.length === 0) return 'unnamed';
    return names[0].replace(/^\//, '');
  }

  function getStatusStyle(state: string): {
    badge: string;
    dot: string;
    glow: string;
  } {
    const s = (state || '').toLowerCase();
    if (s === 'running') {
      return {
        badge: 'bg-emerald-500/15 text-emerald-400 border-emerald-500/30',
        dot: 'bg-emerald-400',
        glow: 'shadow-[0_0_8px_#34d399]',
      };
    }
    if (s === 'paused') {
      return {
        badge: 'bg-amber-500/15 text-amber-400 border-amber-500/30',
        dot: 'bg-amber-400',
        glow: 'shadow-[0_0_8px_#fbbf24]',
      };
    }
    if (s === 'restarting' || s === 'created') {
      return {
        badge: 'bg-cyan-500/15 text-cyan-400 border-cyan-500/30',
        dot: 'bg-cyan-400',
        glow: 'shadow-[0_0_8px_#22d3ee]',
      };
    }
    // exited / stopped / dead
    return {
      badge: 'bg-rose-500/15 text-rose-400 border-rose-500/30',
      dot: 'bg-rose-400',
      glow: '',
    };
  }

  function formatRelativeTime(createdUnixSeconds: number): string {
    if (!createdUnixSeconds) return 'Unknown';
    const now = Math.floor(Date.now() / 1000);
    const diff = Math.max(0, now - createdUnixSeconds);

    if (diff < 60) return `${diff}s ago`;
    const mins = Math.floor(diff / 60);
    if (mins < 60) return `${mins}m ago`;
    const hours = Math.floor(mins / 60);
    if (hours < 24) return `${hours}h ago`;
    const days = Math.floor(hours / 24);
    if (days < 30) return `${days}d ago`;
    const months = Math.floor(days / 30);
    return `${months}mo ago`;
  }
</script>

<div class="w-full overflow-hidden rounded-2xl bg-[#12131c]/90 border border-white/[0.08] shadow-[0_4px_30px_rgba(0,0,0,0.5)] backdrop-blur-md">
  {#if containers.length === 0}
    <div class="py-16 text-center space-y-4">
      <div class="w-12 h-12 mx-auto rounded-2xl bg-violet-600/15 border border-violet-500/30 flex items-center justify-center text-violet-400 shadow-[0_0_20px_rgba(124,58,237,0.25)]">
        <Box class="w-6 h-6" />
      </div>
      <div>
        <h3 class="text-sm font-bold text-white">No containers found</h3>
        <p class="text-xs text-slate-400 max-w-sm mx-auto mt-1">
          No Docker containers match your current query or no containers are currently present on this host.
        </p>
      </div>
      {#if onCreateClick}
        <button
          type="button"
          onclick={onCreateClick}
          class="px-4 py-2 rounded-xl bg-violet-600 hover:bg-violet-500 text-white text-xs font-semibold shadow-[0_0_15px_rgba(124,58,237,0.35)] transition inline-flex items-center gap-1.5"
        >
          <Plus class="w-4 h-4" />
          <span>Launch New Container</span>
        </button>
      {/if}
    </div>
  {:else}
    <div class="overflow-x-auto">
      <table class="w-full text-left border-collapse">
        <thead>
          <tr class="border-b border-white/[0.08] bg-[#0c0d14]/70 text-[11px] font-mono uppercase tracking-wider text-slate-400 select-none">
            <!-- Checkbox column -->
            <th class="py-3 px-4 w-10 text-center">
              <label class="relative inline-flex items-center cursor-pointer">
                <input
                  type="checkbox"
                  checked={allSelected}
                  indeterminate={someSelected}
                  onchange={() => onToggleSelectAll?.()}
                  class="sr-only peer"
                />
                <div class="w-4 h-4 rounded border border-white/20 peer-checked:border-violet-500 peer-checked:bg-violet-600 flex items-center justify-center transition bg-[#181926]">
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

            <!-- Name & ID -->
            <th class="py-3 px-4">
              <button
                type="button"
                onclick={() => handleHeaderSort('name')}
                class="flex items-center gap-1.5 hover:text-white transition group focus:outline-none"
                title="Sort by container name"
              >
                <span>Container</span>
                {#if effectiveSortKey === 'name'}
                  {#if effectiveSortDirection === 'asc'}
                    <ArrowUp class="w-3.5 h-3.5 text-cyan-400" />
                  {:else}
                    <ArrowDown class="w-3.5 h-3.5 text-cyan-400" />
                  {/if}
                {:else}
                  <ArrowUpDown class="w-3 h-3 text-slate-600 group-hover:text-slate-400 transition" />
                {/if}
              </button>
            </th>

            <!-- Status -->
            <th class="py-3 px-4">
              <button
                type="button"
                onclick={() => handleHeaderSort('status')}
                class="flex items-center gap-1.5 hover:text-white transition group focus:outline-none"
                title="Sort by container status"
              >
                <span>Status</span>
                {#if effectiveSortKey === 'status'}
                  {#if effectiveSortDirection === 'asc'}
                    <ArrowUp class="w-3.5 h-3.5 text-cyan-400" />
                  {:else}
                    <ArrowDown class="w-3.5 h-3.5 text-cyan-400" />
                  {/if}
                {:else}
                  <ArrowUpDown class="w-3 h-3 text-slate-600 group-hover:text-slate-400 transition" />
                {/if}
              </button>
            </th>

            <!-- Image -->
            <th class="py-3 px-4">
              <button
                type="button"
                onclick={() => handleHeaderSort('image')}
                class="flex items-center gap-1.5 hover:text-white transition group focus:outline-none"
                title="Sort by image name"
              >
                <span>Image</span>
                {#if effectiveSortKey === 'image'}
                  {#if effectiveSortDirection === 'asc'}
                    <ArrowUp class="w-3.5 h-3.5 text-cyan-400" />
                  {:else}
                    <ArrowDown class="w-3.5 h-3.5 text-cyan-400" />
                  {/if}
                {:else}
                  <ArrowUpDown class="w-3 h-3 text-slate-600 group-hover:text-slate-400 transition" />
                {/if}
              </button>
            </th>

            <!-- Ports -->
            <th class="py-3 px-4">Ports</th>

            <!-- Created -->
            <th class="py-3 px-4">
              <button
                type="button"
                onclick={() => handleHeaderSort('created')}
                class="flex items-center gap-1.5 hover:text-white transition group focus:outline-none"
                title="Sort by creation date"
              >
                <span>Created</span>
                {#if effectiveSortKey === 'created'}
                  {#if effectiveSortDirection === 'asc'}
                    <ArrowUp class="w-3.5 h-3.5 text-cyan-400" />
                  {:else}
                    <ArrowDown class="w-3.5 h-3.5 text-cyan-400" />
                  {/if}
                {:else}
                  <ArrowUpDown class="w-3 h-3 text-slate-600 group-hover:text-slate-400 transition" />
                {/if}
              </button>
            </th>

            <!-- Actions -->
            <th class="py-3 px-4 text-right">Actions</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-white/[0.05] text-xs">
          {#each displayedContainers as container (container.Id)}
            {@const isSelected = selectedIds.includes(container.Id)}
            {@const stateLower = (container.State || '').toLowerCase()}
            {@const isRunning = stateLower === 'running'}
            {@const isPaused = stateLower === 'paused'}
            {@const statusStyle = getStatusStyle(container.State)}
            {@const cleanName = getCleanName(container.Names)}
            {@const currentAction = actionLoading[container.Id]}
            {@const stackName = container.Labels?.['com.docker.compose.project']}

            <tr
              class="group hover:bg-violet-950/20 transition cursor-pointer {isSelected ? 'bg-violet-900/15' : ''}"
              onclick={() => onSelect?.(container.Id)}
            >
              <!-- Checkbox -->
              <td
                class="py-3.5 px-4 text-center"
                onclick={(e) => e.stopPropagation()}
              >
                <label class="relative inline-flex items-center cursor-pointer">
                  <input
                    type="checkbox"
                    checked={isSelected}
                    onchange={() => onToggleSelect?.(container.Id)}
                    class="sr-only peer"
                  />
                  <div class="w-4 h-4 rounded border border-white/20 peer-checked:border-violet-500 peer-checked:bg-violet-600 flex items-center justify-center transition bg-[#181926]">
                    {#if isSelected}
                      <svg class="w-3 h-3 text-white" viewBox="0 0 14 14" fill="none">
                        <path d="M3 7.5L5.5 10L11 4.5" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
                      </svg>
                    {/if}
                  </div>
                </label>
              </td>

              <!-- Container Name & Short ID -->
              <td class="py-3.5 px-4 min-w-[180px]">
                <div class="flex items-center gap-2.5">
                  <div class="w-2 h-2 rounded-full shrink-0 {statusStyle.dot} {statusStyle.glow}"></div>
                  <div class="min-w-0">
                    <span class="font-semibold text-slate-100 group-hover:text-cyan-300 transition truncate block">
                      {cleanName}
                    </span>
                    <div class="flex items-center gap-1.5 flex-wrap">
                      <span class="text-[11px] font-mono text-slate-500">
                        {container.Id.slice(0, 12)}
                      </span>
                      {#if stackName}
                        <span class="inline-flex items-center gap-1 px-1.5 py-0.2 rounded bg-violet-500/15 text-cyan-300 border border-violet-500/30 text-[9px] font-mono" title="Managed by Compose stack '{stackName}'">
                          <Layers class="w-2.5 h-2.5 text-cyan-400" />
                          <span>{stackName}</span>
                        </span>
                      {/if}
                    </div>
                  </div>
                </div>
              </td>

              <!-- Status badge -->
              <td class="py-3.5 px-4 whitespace-nowrap">
                <span class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[10px] font-mono uppercase font-semibold border {statusStyle.badge}">
                  <span class="w-1.5 h-1.5 rounded-full {statusStyle.dot}"></span>
                  {container.State || 'unknown'}
                </span>
                {#if container.Status}
                  <span class="block text-[10px] font-mono text-slate-500 truncate max-w-[160px] mt-0.5" title={container.Status}>
                    {container.Status}
                  </span>
                {/if}
              </td>

              <!-- Image -->
              <td class="py-3.5 px-4 max-w-[220px]" onclick={(e) => e.stopPropagation()}>
                <div class="space-y-1">
                  <span class="font-mono text-[11px] text-slate-300 truncate block" title={container.Image}>
                    {container.Image}
                  </span>

                  <!-- Up to date indicator badge -->
                  {#if container.Image && !container.Image.startsWith('<none>')}
                    {@const update = imageUpdateStore.getStatus(container.Image)}
                    {@const isChecking = imageUpdateStore.isChecking(container.Image)}
                    {@const isUpdating = actionLoading[container.Id] === 'updating'}

                    {#if isUpdating}
                      <span class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[9px] font-mono font-semibold bg-amber-500/20 text-amber-300 border border-amber-500/40 shadow-[0_0_8px_rgba(245,158,11,0.25)] animate-pulse">
                        <Loader2 class="w-2.5 h-2.5 animate-spin text-amber-400" />
                        <span>Updating & Recreating...</span>
                      </span>
                    {:else if isChecking}
                      <span class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[9px] font-mono bg-cyan-500/10 text-cyan-300 border border-cyan-500/20">
                        <Loader2 class="w-2.5 h-2.5 animate-spin text-cyan-400" />
                        <span>Checking...</span>
                      </span>
                    {:else if update}
                      {#if update.status === 'update_available'}
                        <button
                          type="button"
                          onclick={() => onUpdateContainer ? onUpdateContainer(container) : imageUpdateStore.checkUpdate(container.Image)}
                          class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[9px] font-mono font-semibold bg-amber-500/20 text-amber-300 border border-amber-500/40 hover:bg-amber-500/30 transition shadow-[0_0_8px_rgba(245,158,11,0.25)] cursor-pointer"
                          title="New version available on registry! Click to update."
                        >
                          <ArrowUpCircle class="w-2.5 h-2.5 text-amber-400" />
                          <span>Update Available</span>
                        </button>
                      {:else if update.status === 'up_to_date'}
                        <span
                          class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[9px] font-mono bg-emerald-500/15 text-emerald-300 border border-emerald-500/30"
                          title="Image is up to date with registry"
                        >
                          <CheckCircle2 class="w-2.5 h-2.5 text-emerald-400" />
                          <span>Up to date</span>
                        </span>
                      {/if}
                    {:else}
                      <button
                        type="button"
                        onclick={() => imageUpdateStore.checkUpdate(container.Image)}
                        class="inline-flex items-center gap-1 text-[10px] font-mono text-slate-500 hover:text-cyan-400 transition cursor-pointer"
                        title="Check if remote registry has a newer version of this image"
                      >
                        <Sparkles class="w-2.5 h-2.5 text-slate-500 hover:text-cyan-400" />
                        <span>Check update</span>
                      </button>
                    {/if}
                  {/if}
                </div>
              </td>

              <!-- Ports -->
              <td class="py-3.5 px-4 whitespace-nowrap" onclick={(e) => e.stopPropagation()}>
                {#if container.Ports && container.Ports.length > 0}
                  <div class="flex flex-wrap items-center gap-1.5 max-w-[220px]">
                    {#each container.Ports.slice(0, 3) as port}
                      {@const hasPublic = port.PublicPort !== undefined && port.PublicPort !== null}
                      {#if hasPublic}
                        <a
                          href="http://localhost:{port.PublicPort}"
                          target="_blank"
                          rel="noopener noreferrer"
                          class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded-md text-[10px] font-mono bg-cyan-500/10 hover:bg-cyan-500/20 text-cyan-400 border border-cyan-500/25 transition group/port"
                          title="Open http://localhost:{port.PublicPort}"
                        >
                          <span>{port.PublicPort}:{port.PrivatePort}/{port.Type}</span>
                          <ExternalLink class="w-2.5 h-2.5 opacity-60 group-hover/port:opacity-100" />
                        </a>
                      {:else}
                        <span class="inline-flex items-center px-1.5 py-0.5 rounded-md text-[10px] font-mono bg-white/[0.04] text-slate-400 border border-white/[0.08]">
                          {port.PrivatePort}/{port.Type}
                        </span>
                      {/if}
                    {/each}
                    {#if container.Ports.length > 3}
                      <span class="text-[10px] font-mono text-slate-500" title={container.Ports.map(p => `${p.PublicPort ? p.PublicPort + ':' : ''}${p.PrivatePort}/${p.Type}`).join(', ')}>
                        +{container.Ports.length - 3} more
                      </span>
                    {/if}
                  </div>
                {:else}
                  <span class="text-slate-600 font-mono text-[11px]">—</span>
                {/if}
              </td>

              <!-- Created -->
              <td class="py-3.5 px-4 whitespace-nowrap text-slate-400 font-mono text-[11px]">
                {formatRelativeTime(container.Created)}
              </td>

              <!-- Actions -->
              <td
                class="py-3.5 px-4 text-right whitespace-nowrap"
                onclick={(e) => e.stopPropagation()}
              >
                <div class="inline-flex items-center gap-1 bg-[#0a0b10]/60 p-1 rounded-xl border border-white/[0.06]">
                  {#if currentAction}
                    <div class="px-2 py-1 flex items-center gap-1.5 text-xs text-violet-400 font-mono">
                      <Loader2 class="w-3.5 h-3.5 animate-spin" />
                      <span class="capitalize text-[10px]">{currentAction}...</span>
                    </div>
                  {:else}
                    <!-- Detail View button -->
                    <button
                      type="button"
                      onclick={() => onSelect?.(container.Id)}
                      class="p-1.5 rounded-lg hover:bg-white/10 text-slate-400 hover:text-white transition"
                      title="Inspect Details"
                    >
                      <Eye class="w-3.5 h-3.5" />
                    </button>

                    <!-- Start or Stop -->
                    {#if isRunning}
                      <button
                        type="button"
                        onclick={() => onAction?.('stop', container.Id)}
                        class="p-1.5 rounded-lg hover:bg-rose-500/20 text-rose-400 hover:text-rose-300 transition"
                        title="Stop Container"
                      >
                        <Square class="w-3.5 h-3.5 fill-current" />
                      </button>
                    {:else}
                      <button
                        type="button"
                        onclick={() => onAction?.('start', container.Id)}
                        class="p-1.5 rounded-lg hover:bg-emerald-500/20 text-emerald-400 hover:text-emerald-300 transition"
                        title="Start Container"
                      >
                        <Play class="w-3.5 h-3.5 fill-current" />
                      </button>
                    {/if}

                    <!-- Restart -->
                    <button
                      type="button"
                      onclick={() => onAction?.('restart', container.Id)}
                      class="p-1.5 rounded-lg hover:bg-cyan-500/20 text-slate-400 hover:text-cyan-300 transition"
                      title="Restart Container"
                    >
                      <RotateCw class="w-3.5 h-3.5" />
                    </button>

                    <!-- Edit & Reconfigure -->
                    <button
                      type="button"
                      onclick={() => onEdit?.(container)}
                      class="p-1.5 rounded-lg hover:bg-violet-500/20 text-slate-400 hover:text-violet-300 transition"
                      title="Edit & Reconfigure Container"
                    >
                      <Sliders class="w-3.5 h-3.5" />
                    </button>

                    <!-- Update Image & Recreate Container -->
                    {#if container.Image && imageUpdateStore.hasUpdate(container.Image)}
                      {@const isUpdating = actionLoading[container.Id] === 'updating'}
                      <button
                        type="button"
                        onclick={() => onUpdateContainer?.(container)}
                        disabled={isUpdating}
                        class="p-1.5 rounded-lg bg-amber-500/20 hover:bg-amber-500/30 text-amber-300 border border-amber-500/40 transition shadow-[0_0_10px_rgba(245,158,11,0.3)] disabled:opacity-50 cursor-pointer"
                        title="Update container with latest image and recreate"
                      >
                        {#if isUpdating}
                          <Loader2 class="w-3.5 h-3.5 animate-spin text-amber-400" />
                        {:else}
                          <ArrowUpCircle class="w-3.5 h-3.5 text-amber-400" />
                        {/if}
                      </button>
                    {/if}

                    <!-- Duplicate -->
                    <button
                      type="button"
                      onclick={() => onDuplicate?.(container)}
                      class="p-1.5 rounded-lg hover:bg-violet-500/20 text-slate-400 hover:text-violet-300 transition"
                      title="Duplicate Container"
                    >
                      <Copy class="w-3.5 h-3.5" />
                    </button>

                    <!-- Pause / Unpause -->
                    {#if isPaused}
                      <button
                        type="button"
                        onclick={() => onAction?.('unpause', container.Id)}
                        class="p-1.5 rounded-lg hover:bg-emerald-500/20 text-emerald-400 hover:text-emerald-300 transition"
                        title="Unpause Container"
                      >
                        <Play class="w-3.5 h-3.5" />
                      </button>
                    {:else if isRunning}
                      <button
                        type="button"
                        onclick={() => onAction?.('pause', container.Id)}
                        class="p-1.5 rounded-lg hover:bg-amber-500/20 text-slate-400 hover:text-amber-300 transition"
                        title="Pause Container"
                      >
                        <Pause class="w-3.5 h-3.5" />
                      </button>
                    {/if}

                    <!-- Kill -->
                    {#if isRunning}
                      <button
                        type="button"
                        onclick={() => onAction?.('kill', container.Id)}
                        class="p-1.5 rounded-lg hover:bg-rose-500/20 text-slate-400 hover:text-rose-400 transition"
                        title="Kill Container (SIGKILL)"
                      >
                        <Skull class="w-3.5 h-3.5" />
                      </button>
                    {/if}

                    <!-- Remove -->
                    <button
                      type="button"
                      onclick={() => onAction?.('remove', container.Id)}
                      class="p-1.5 rounded-lg hover:bg-rose-500/20 text-slate-500 hover:text-rose-400 transition"
                      title="Remove Container"
                    >
                      <Trash2 class="w-3.5 h-3.5" />
                    </button>
                  {/if}
                </div>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</div>
