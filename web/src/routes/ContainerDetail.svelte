<script lang="ts">
  import {
    onMount } from 'svelte';
  import {
    ArrowLeft,
    Box,
    Layers,
    Play,
    Square,
    RotateCw,
    Pause,
    Skull,
    Trash,
    RefreshCw,
    ExternalLink,
    Copy,
    Check,
    Terminal,
    FileText,
    Activity,
    Cpu,
    HardDrive,
    Network,
    Clock,
    ShieldAlert,
    CircleAlert,
    CircleCheck,
    LoaderCircle,
    Download,
    Braces,
    SlidersHorizontal,
    FolderTree,
    PenLine,
    Sparkles,
    CircleArrowUp,
    Globe,
    Tag,
  } from '@lucide/svelte';
  import { containerApi, taskApi } from '../lib/api';
  import { imageUpdateStore } from '../lib/stores/imageUpdates.svelte';
  import { tasksStore } from '../lib/stores/tasks.svelte';
  import { getPortOpenUrl, getContainerCustomUrl, formatUrlBadge } from '../lib/utils/portLink';
  import type { ContainerInspectInfo } from '../lib/types';
  import TerminalView from '../lib/components/Terminal.svelte';
  import LogsViewer from '../lib/components/LogsViewer.svelte';
  import LiveStatsChart from '../lib/components/LiveStatsChart.svelte';
  import FileBrowser from '../lib/components/FileBrowser.svelte';
  import ContainerCreateModal from '../lib/components/ContainerCreateModal.svelte';
  import UpdateContainerModal from '../lib/components/UpdateContainerModal.svelte';

  interface Props {
    containerId: string;
    onBack: () => void;
    onDeleted?: () => void;
    onReplaced?: (newContainerId: string) => void;
    onNavigateToStack?: (stackName: string) => void;
  }

  let { containerId, onBack, onDeleted, onReplaced, onNavigateToStack }: Props = $props();

  // State
  let replacedContainerId = $state<string | null>(null);
  let effectiveContainerId = $derived(replacedContainerId || containerId);
  let inspectData = $state<ContainerInspectInfo | null>(null);
  let isLoading = $state(true);
  let isActionLoading = $state<string | null>(null);
  let error = $state<string | null>(null);
  let feedbackMessage = $state<{ type: 'success' | 'error' | 'info'; text: string } | null>(null);
  let isUpdateModalOpen = $state(false);

  // Tabs
  const validTabs = ['details', 'json', 'logs', 'terminal', 'stats', 'files'] as const;
  type ContainerTab = typeof validTabs[number];
  const savedTab = (typeof localStorage !== 'undefined'
    ? localStorage.getItem('pdswarm_container_tab')
    : null) as ContainerTab | null;

  let activeTab = $state<ContainerTab>(
    savedTab && validTabs.includes(savedTab) ? savedTab : 'details'
  );

  function setActiveTab(tab: ContainerTab) {
    activeTab = tab;
    try {
      localStorage.setItem('pdswarm_container_tab', tab);
    } catch {
      // ignore
    }
  }

  // Modals & UI
  let isDeleteConfirmOpen = $state(false);
  let isDuplicateModalOpen = $state(false);
  let isCopied = $state(false);
  let envSearchQuery = $state('');
  let envFilterType = $state<'all' | 'user' | 'image'>('all');

  function showFeedback(type: 'success' | 'error' | 'info', text: string) {
    feedbackMessage = { type, text };
    setTimeout(() => {
      if (feedbackMessage?.text === text) {
        feedbackMessage = null;
      }
    }, 4000);
  }

  async function loadContainer() {
    isLoading = true;
    error = null;
    try {
      const data = await containerApi.inspect(effectiveContainerId);
      inspectData = data;
    } catch (err: unknown) {
      if (err instanceof Error) {
        error = err.message;
      } else {
        error = 'Failed to inspect container metadata.';
      }
    } finally {
      isLoading = false;
    }
  }

  let containerImage = $derived(
    inspectData?.Config?.Image || inspectData?.Image || ''
  );
  let isUpdatingImage = $state(false);

  async function handleCheckImageUpdate() {
    if (!containerImage) return;
    await imageUpdateStore.checkUpdate(containerImage);
  }

  function handleUpdateImage() {
    if (!containerImage) return;
    isUpdateModalOpen = true;
  }

  async function confirmUpdateImage() {
    if (!containerImage) return;
    isUpdatingImage = true;
    isUpdateModalOpen = false;
    showFeedback('info', `Updating and recreating container with latest ${containerImage}...`);
    try {
      const res = await containerApi.updateImage(effectiveContainerId);
      tasksStore.poll();

      if (res && res.task_id) {
        let isDone = false;
        let attempts = 0;
        while (!isDone && attempts < 180) {
          attempts++;
          await new Promise((r) => setTimeout(r, 1200));
          try {
            const task = await taskApi.get(res.task_id);
            if (task.status === 'completed') {
              isDone = true;
              break;
            } else if (task.status === 'failed' || task.status === 'cancelled') {
              throw new Error(task.error || `Update ${task.status}`);
            }
          } catch (e: unknown) {
            if (e instanceof Error && (e.message.includes('failed') || e.message.includes('cancelled') || e.message.includes('Update '))) {
              throw e;
            }
          }
        }
      }

      await imageUpdateStore.checkUpdate(containerImage);
      await loadContainer();
      showFeedback('success', `Container successfully updated to latest ${containerImage} and recreated!`);
    } catch (err: unknown) {
      showFeedback('error', err instanceof Error ? err.message : 'Failed to initiate container update');
    } finally {
      isUpdatingImage = false;
    }
  }

  onMount(() => {
    loadContainer();
  });

  async function handleAction(action: 'start' | 'stop' | 'restart' | 'pause' | 'unpause' | 'kill' | 'remove') {
    isActionLoading = action;
    try {
      if (action === 'start') {
        await containerApi.start(effectiveContainerId);
        showFeedback('success', 'Container started.');
      } else if (action === 'stop') {
        await containerApi.stop(effectiveContainerId);
        showFeedback('success', 'Container stopped.');
      } else if (action === 'restart') {
        await containerApi.restart(effectiveContainerId);
        showFeedback('success', 'Container restarted.');
      } else if (action === 'pause') {
        await containerApi.pause(effectiveContainerId);
        showFeedback('success', 'Container paused.');
      } else if (action === 'unpause') {
        await containerApi.unpause(effectiveContainerId);
        showFeedback('success', 'Container unpaused.');
      } else if (action === 'kill') {
        await containerApi.kill(effectiveContainerId);
        showFeedback('success', 'SIGKILL sent to container.');
      } else if (action === 'remove') {
        await containerApi.remove(effectiveContainerId, true, true);
        showFeedback('success', 'Container removed.');
        if (onDeleted) {
          onDeleted();
        } else {
          onBack();
        }
        return;
      }
      await loadContainer();
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : `Failed to execute ${action}`;
      showFeedback('error', msg);
    } finally {
      isActionLoading = null;
    }
  }

  function copyText(text: string) {
    navigator.clipboard.writeText(text);
    isCopied = true;
    setTimeout(() => {
      isCopied = false;
    }, 2000);
  }

  function getStatusStyle(status?: string): {
    badge: string;
    dot: string;
    glow: string;
  } {
    const s = (status || '').toLowerCase();
    if (s === 'running') {
      return {
        badge: 'bg-emerald-500/15 text-emerald-400 border-emerald-500/30',
        dot: 'bg-emerald-400',
        glow: 'shadow-[0_0_10px_#34d399]',
      };
    }
    if (s === 'paused') {
      return {
        badge: 'bg-amber-500/15 text-amber-400 border-amber-500/30',
        dot: 'bg-amber-400',
        glow: 'shadow-[0_0_10px_#fbbf24]',
      };
    }
    if (s === 'restarting' || s === 'created') {
      return {
        badge: 'bg-cyan-500/15 text-cyan-400 border-cyan-500/30',
        dot: 'bg-cyan-400',
        glow: 'shadow-[0_0_10px_#22d3ee]',
      };
    }
    return {
      badge: 'bg-rose-500/15 text-rose-400 border-rose-500/30',
      dot: 'bg-rose-400',
      glow: '',
    };
  }

  function formatDate(isoOrUnix?: string | number): string {
    if (!isoOrUnix) return 'N/A';
    try {
      const date = typeof isoOrUnix === 'number'
        ? new Date(isoOrUnix * 1000)
        : new Date(isoOrUnix);
      if (isNaN(date.getTime())) return String(isoOrUnix);
      return date.toLocaleString();
    } catch {
      return String(isoOrUnix);
    }
  }

  let cleanName = $derived(
    inspectData?.Name ? inspectData.Name.replace(/^\//, '') : effectiveContainerId.slice(0, 12)
  );

  let isRunning = $derived(inspectData?.State?.Running === true);
  let isPaused = $derived(inspectData?.State?.Paused === true);
  let statusStyle = $derived(getStatusStyle(inspectData?.State?.Status));
  let stackProject = $derived(inspectData?.Config?.Labels?.['com.docker.compose.project']);
  let stackService = $derived(inspectData?.Config?.Labels?.['com.docker.compose.service']);
  let customAppUrl = $derived(getContainerCustomUrl(inspectData?.Config?.Labels));

  // Flattened port mappings
  let portList = $derived.by(() => {
    if (!inspectData?.NetworkSettings?.Ports) return [];
    const ports = inspectData.NetworkSettings.Ports;
    const list: Array<{ containerPort: string; hostIp: string; hostPort: string }> = [];

    for (const [cPort, bindings] of Object.entries(ports)) {
      if (bindings && bindings.length > 0) {
        for (const b of bindings) {
          list.push({
            containerPort: cPort,
            hostIp: b.HostIp || '0.0.0.0',
            hostPort: b.HostPort,
          });
        }
      } else {
        list.push({
          containerPort: cPort,
          hostIp: '-',
          hostPort: '-',
        });
      }
    }
    return list;
  });

  // Environment variables breakdown
  interface AnalyzedEnv {
    raw: string;
    key: string;
    val: string;
    origin: 'user' | 'image' | 'override';
  }

  let analyzedEnvs = $derived.by<AnalyzedEnv[]>(() => {
    if (!inspectData?.Config?.Env) return [];
    const containerEnvs = inspectData.Config.Env;
    const imgEnvMap = new Map<string, string>();
    for (const e of inspectData.ImageEnv || []) {
      const idx = e.indexOf('=');
      const k = idx !== -1 ? e.slice(0, idx) : e;
      const v = idx !== -1 ? e.slice(idx + 1) : '';
      imgEnvMap.set(k, v);
    }

    return containerEnvs.map(envStr => {
      const idx = envStr.indexOf('=');
      const k = idx !== -1 ? envStr.slice(0, idx) : envStr;
      const val = idx !== -1 ? envStr.slice(idx + 1) : '';
      let origin: 'user' | 'image' | 'override' = 'user';

      if (imgEnvMap.has(k)) {
        if (imgEnvMap.get(k) === val) {
          origin = 'image';
        } else {
          origin = 'override';
        }
      } else {
        origin = 'user';
      }

      return {
        raw: envStr,
        key: k,
        val,
        origin,
      };
    });
  });

  let userEnvCount = $derived(analyzedEnvs.filter(e => e.origin === 'user' || e.origin === 'override').length);
  let imageEnvCount = $derived(analyzedEnvs.filter(e => e.origin === 'image').length);

  // Filtered environment variables
  let filteredEnv = $derived.by(() => {
    let list = analyzedEnvs;
    if (envFilterType === 'user') {
      list = list.filter(e => e.origin === 'user' || e.origin === 'override');
    } else if (envFilterType === 'image') {
      list = list.filter(e => e.origin === 'image');
    }

    if (!envSearchQuery.trim()) return list;
    const q = envSearchQuery.toLowerCase();
    return list.filter(e => e.key.toLowerCase().includes(q) || e.val.toLowerCase().includes(q));
  });

  // Container Labels list
  let labelSearchQuery = $state('');
  let containerLabelsList = $derived.by(() => {
    const labels = inspectData?.Config?.Labels || {};
    const entries = Object.entries(labels).map(([key, value]) => ({ key, value }));
    if (!labelSearchQuery.trim()) return entries;
    const q = labelSearchQuery.toLowerCase();
    return entries.filter(e => e.key.toLowerCase().includes(q) || e.value.toLowerCase().includes(q));
  });
  let totalLabelsCount = $derived(Object.keys(inspectData?.Config?.Labels || {}).length);
</script>

<div class="space-y-6 animate-in fade-in duration-200">
  <!-- Top Navigation & Actions Bar -->
  <div class="flex flex-col md:flex-row md:items-center md:justify-between gap-4">
    <div class="flex items-center gap-3">
      <button
        type="button"
        onclick={onBack}
        class="p-2 rounded-xl bg-[#12131c] hover:bg-white/10 text-slate-300 hover:text-white border border-white/[0.08] transition group"
        title="Back to Containers List"
      >
        <ArrowLeft class="w-4 h-4 group-hover:-translate-x-0.5 transition-transform" />
      </button>

      <div>
        <div class="flex items-center gap-2.5">
          <h1 class="text-xl font-bold text-white tracking-tight">{cleanName}</h1>
          <span class="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-mono uppercase font-semibold border {statusStyle.badge}">
            <span class="w-1.5 h-1.5 rounded-full {statusStyle.dot} {statusStyle.glow}"></span>
            {inspectData?.State?.Status || 'inspecting'}
          </span>
        </div>
        <div class="flex items-center gap-2 mt-1">
          <span class="text-[11px] font-mono text-slate-400">ID:</span>
          <span class="text-[11px] font-mono text-cyan-400">{effectiveContainerId.slice(0, 12)}</span>
          <button
            type="button"
            onclick={() => copyText(effectiveContainerId)}
            class="text-slate-500 hover:text-slate-300 transition p-0.5 rounded"
            title="Copy full container ID"
          >
            {#if isCopied}
              <Check class="w-3 h-3 text-emerald-400" />
            {:else}
              <Copy class="w-3 h-3" />
            {/if}
          </button>
        </div>
      </div>
    </div>

    <!-- Action Toolbar -->
    <div class="flex flex-wrap items-center gap-2">
      <!-- Refresh Button -->
      <button
        type="button"
        onclick={loadContainer}
        disabled={isLoading}
        class="p-2 rounded-xl bg-[#12131c] hover:bg-white/10 text-slate-300 hover:text-white border border-white/[0.08] text-xs transition disabled:opacity-50"
        title="Refresh inspect details"
      >
        <RefreshCw class="w-4 h-4 {isLoading ? 'animate-spin text-cyan-400' : ''}" />
      </button>

      {#if isActionLoading}
        <div class="px-3 py-1.5 rounded-xl bg-[#12131c] border border-white/[0.08] flex items-center gap-2 text-xs text-violet-300 font-mono">
          <LoaderCircle class="w-3.5 h-3.5 animate-spin text-cyan-400" />
          <span class="capitalize">{isActionLoading}...</span>
        </div>
      {:else}
        <!-- Start / Stop Button -->
        {#if isRunning}
          <button
            type="button"
            onclick={() => handleAction('stop')}
            class="px-3 py-1.5 rounded-xl bg-rose-600/20 hover:bg-rose-600/30 text-rose-300 border border-rose-500/30 text-xs font-semibold transition inline-flex items-center gap-1.5"
          >
            <Square class="w-3.5 h-3.5 fill-current" />
            <span>Stop</span>
          </button>
        {:else}
          <button
            type="button"
            onclick={() => handleAction('start')}
            class="px-3 py-1.5 rounded-xl bg-emerald-600/20 hover:bg-emerald-600/30 text-emerald-300 border border-emerald-500/30 text-xs font-semibold transition inline-flex items-center gap-1.5"
          >
            <Play class="w-3.5 h-3.5 fill-current" />
            <span>Start</span>
          </button>
        {/if}

        <!-- Restart Button -->
        <button
          type="button"
          onclick={() => handleAction('restart')}
          class="px-3 py-1.5 rounded-xl bg-cyan-600/20 hover:bg-cyan-600/30 text-cyan-300 border border-cyan-500/30 text-xs font-semibold transition inline-flex items-center gap-1.5"
        >
          <RotateCw class="w-3.5 h-3.5" />
          <span>Restart</span>
        </button>

        <!-- Duplicate / Edit Button -->
        <button
          type="button"
          onclick={() => isDuplicateModalOpen = true}
          class="px-3 py-1.5 rounded-xl bg-violet-600/20 hover:bg-violet-600/30 text-violet-300 border border-violet-500/30 text-xs font-semibold transition inline-flex items-center gap-1.5"
          title="Duplicate or Edit & Recreate Container"
        >
          <SlidersHorizontal class="w-3.5 h-3.5" />
          <span>Duplicate / Edit</span>
        </button>

        <!-- Update Image & Recreate Button (Visible when update available) -->
        {#if containerImage && imageUpdateStore.hasUpdate(containerImage)}
          <button
            type="button"
            onclick={handleUpdateImage}
            disabled={isUpdatingImage}
            class="px-3 py-1.5 rounded-xl bg-amber-500/20 hover:bg-amber-500/30 text-amber-300 border border-amber-500/40 text-xs font-semibold transition inline-flex items-center gap-1.5 shadow-[0_0_15px_rgba(245,158,11,0.3)] animate-pulse"
            title="Pull latest image and recreate container"
          >
            {#if isUpdatingImage}
              <LoaderCircle class="w-3.5 h-3.5 animate-spin text-amber-400" />
              <span>Updating...</span>
            {:else}
              <CircleArrowUp class="w-3.5 h-3.5 text-amber-400" />
              <span>Update Image</span>
            {/if}
          </button>
        {/if}

        <!-- Pause / Unpause Button -->
        {#if isPaused}
          <button
            type="button"
            onclick={() => handleAction('unpause')}
            class="px-3 py-1.5 rounded-xl bg-emerald-600/20 hover:bg-emerald-600/30 text-emerald-300 border border-emerald-500/30 text-xs font-semibold transition inline-flex items-center gap-1.5"
          >
            <Play class="w-3.5 h-3.5" />
            <span>Unpause</span>
          </button>
        {:else if isRunning}
          <button
            type="button"
            onclick={() => handleAction('pause')}
            class="px-3 py-1.5 rounded-xl bg-amber-600/20 hover:bg-amber-600/30 text-amber-300 border border-amber-500/30 text-xs font-semibold transition inline-flex items-center gap-1.5"
          >
            <Pause class="w-3.5 h-3.5" />
            <span>Pause</span>
          </button>
        {/if}

        <!-- Kill Button -->
        {#if isRunning}
          <button
            type="button"
            onclick={() => handleAction('kill')}
            class="p-2 rounded-xl bg-rose-950/40 hover:bg-rose-900/60 text-rose-400 border border-rose-500/30 text-xs transition"
            title="Force Kill (SIGKILL)"
          >
            <Skull class="w-3.5 h-3.5" />
          </button>
        {/if}

        <!-- Remove Button -->
        <button
          type="button"
          onclick={() => isDeleteConfirmOpen = true}
          class="p-2 rounded-xl bg-rose-600/20 hover:bg-rose-600/30 text-rose-400 border border-rose-500/30 text-xs transition"
          title="Delete Container"
        >
          <Trash class="w-3.5 h-3.5" />
        </button>
      {/if}
    </div>
  </div>

  <!-- Feedback Banner -->
  {#if feedbackMessage}
    <div
      class="p-3 rounded-xl border flex items-center justify-between gap-3 text-xs animate-in slide-in-from-top-2 duration-150 {feedbackMessage.type === 'success'
        ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-300'
        : feedbackMessage.type === 'info'
        ? 'bg-cyan-500/10 border-cyan-500/30 text-cyan-300 shadow-[0_0_15px_rgba(6,182,212,0.15)]'
        : 'bg-rose-500/10 border-rose-500/30 text-rose-300'}"
    >
      <div class="flex items-center gap-2">
        {#if feedbackMessage.type === 'success'}
          <CircleCheck class="w-4 h-4 text-emerald-400 shrink-0" />
        {:else if feedbackMessage.type === 'info'}
          <Sparkles class="w-4 h-4 text-cyan-400 shrink-0 animate-pulse" />
        {:else}
          <CircleAlert class="w-4 h-4 text-rose-400 shrink-0" />
        {/if}
        <span>{feedbackMessage.text}</span>
      </div>
    </div>
  {/if}

  {#if error}
    <div class="p-4 rounded-xl bg-rose-500/10 border border-rose-500/30 flex items-center justify-between text-xs text-rose-300">
      <div class="flex items-center gap-2">
        <CircleAlert class="w-4 h-4 text-rose-400 shrink-0" />
        <span>{error}</span>
      </div>
      <button
        type="button"
        onclick={loadContainer}
        class="px-3 py-1 rounded-lg bg-rose-500/20 text-rose-300 text-[11px]"
      >
        Retry
      </button>
    </div>
  {/if}

  <!-- Stack Association Notice -->
  {#if stackProject}
    <div class="p-3.5 rounded-2xl bg-gradient-to-r from-violet-950/60 via-[#12131c] to-indigo-950/60 border border-violet-500/30 flex flex-wrap items-center justify-between gap-3 text-xs animate-in fade-in duration-150">
      <div class="flex items-center gap-3">
        <div class="w-8 h-8 rounded-xl bg-violet-600/20 border border-violet-500/30 flex items-center justify-center text-cyan-400">
          <Layers class="w-4 h-4" />
        </div>
        <div>
          <div class="flex items-center gap-2">
            <span class="font-bold text-white">Managed by Compose Stack</span>
            <span class="px-2 py-0.5 rounded-full text-[10px] font-mono bg-violet-500/20 text-cyan-300 border border-violet-500/30">
              {stackProject}
            </span>
          </div>
          <span class="text-[11px] text-slate-400">
            Service: <code class="text-violet-300 font-mono">{stackService || 'service'}</code> &bull; Manual container edits will be overwritten if the stack is redeployed
          </span>
        </div>
      </div>
      {#if onNavigateToStack}
        <button
          type="button"
          onclick={() => onNavigateToStack?.(stackProject)}
          class="px-3 py-1.5 rounded-xl bg-cyan-500/10 hover:bg-cyan-500/20 text-cyan-300 border border-cyan-500/30 text-xs font-semibold transition inline-flex items-center gap-1.5 shrink-0 cursor-pointer"
        >
          <PenLine class="w-3.5 h-3.5" />
          <span>Edit Stack in Studio</span>
        </button>
      {/if}
    </div>
  {/if}

  <!-- Quick Telemetry Cards -->
  <div class="grid grid-cols-2 md:grid-cols-4 gap-4">
    <!-- Image Card -->
    <div class="p-4 rounded-2xl bg-[#12131c] border border-white/[0.08] space-y-2">
      <div class="flex items-center justify-between">
        <span class="text-[10px] font-mono uppercase tracking-wider text-slate-400 block">Image</span>
        {#if containerImage}
          {@const isChecking = imageUpdateStore.isChecking(containerImage)}
          <button
            type="button"
            onclick={handleCheckImageUpdate}
            disabled={isChecking}
            class="text-[10px] font-mono text-slate-400 hover:text-cyan-400 flex items-center gap-1 transition cursor-pointer"
            title="Check remote registry for newer version"
          >
            <Sparkles class="w-3 h-3 {isChecking ? 'animate-spin text-cyan-400' : ''}" />
            <span>Check</span>
          </button>
        {/if}
      </div>

      <span class="text-xs font-mono font-bold text-white truncate block" title={containerImage}>
        {containerImage || 'Unknown'}
      </span>

      <!-- Update status pill -->
      {#if containerImage}
        {@const update = imageUpdateStore.getStatus(containerImage)}
        {@const isChecking = imageUpdateStore.isChecking(containerImage)}

        {#if isChecking}
          <div class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-md text-[10px] font-mono bg-cyan-500/10 text-cyan-300 border border-cyan-500/20">
            <LoaderCircle class="w-2.5 h-2.5 animate-spin text-cyan-400" />
            <span>Checking registry...</span>
          </div>
        {:else if update}
          {#if update.status === 'update_available'}
            <div class="space-y-1.5 pt-0.5">
              <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-md text-[10px] font-mono font-semibold bg-amber-500/20 text-amber-300 border border-amber-500/40 shadow-[0_0_8px_rgba(245,158,11,0.25)]">
                <CircleArrowUp class="w-3 h-3 text-amber-400" />
                <span>Update Available</span>
              </span>
              <button
                type="button"
                onclick={handleUpdateImage}
                disabled={isUpdatingImage}
                class="w-full py-1 px-2 rounded-lg bg-amber-500/20 hover:bg-amber-500/30 text-amber-200 border border-amber-500/40 text-[10px] font-semibold flex items-center justify-center gap-1.5 transition cursor-pointer shadow-[0_0_10px_rgba(245,158,11,0.2)]"
              >
                <CircleArrowUp class="w-3 h-3 text-amber-400" />
                <span>Pull & Recreate</span>
              </button>
            </div>
          {:else if update.status === 'up_to_date'}
            <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-md text-[10px] font-mono bg-emerald-500/15 text-emerald-300 border border-emerald-500/30">
              <CircleCheck class="w-3 h-3 text-emerald-400" />
              <span>Up to date</span>
            </span>
          {/if}
        {/if}
      {/if}

      <span class="text-[10px] font-mono text-slate-500 block truncate">
        {inspectData?.Platform || 'linux/amd64'}
      </span>
    </div>

    <!-- IP & Network Card -->
    <div class="p-4 rounded-2xl bg-[#12131c] border border-white/[0.08] space-y-1">
      <span class="text-[10px] font-mono uppercase tracking-wider text-slate-400 block">IP Address</span>
      <span class="text-xs font-mono font-bold text-cyan-400 truncate block">
        {inspectData?.NetworkSettings?.IPAddress || 'None (Host/Bridge)'}
      </span>
      <span class="text-[10px] font-mono text-slate-500 block">
        Mode: {inspectData?.HostConfig?.NetworkMode || 'default'}
      </span>
    </div>

    <!-- Restart Policy -->
    <div class="p-4 rounded-2xl bg-[#12131c] border border-white/[0.08] space-y-1">
      <span class="text-[10px] font-mono uppercase tracking-wider text-slate-400 block">Restart Policy</span>
      <span class="text-xs font-mono font-bold text-violet-300 truncate block capitalize">
        {inspectData?.HostConfig?.RestartPolicy?.Name || 'no'}
      </span>
      <span class="text-[10px] font-mono text-slate-500 block">
        Restarts: {inspectData?.RestartCount ?? 0}
      </span>
    </div>

    <!-- Started At -->
    <div class="p-4 rounded-2xl bg-[#12131c] border border-white/[0.08] space-y-1">
      <span class="text-[10px] font-mono uppercase tracking-wider text-slate-400 block">Created</span>
      <span class="text-xs font-mono font-bold text-slate-200 truncate block">
        {formatDate(inspectData?.Created)}
      </span>
      <span class="text-[10px] font-mono text-slate-500 block truncate">
        PID: {inspectData?.State?.Pid || 0}
      </span>
    </div>
  </div>

  <!-- Navigation Tabs -->
  <div class="flex items-center gap-2 border-b border-white/[0.08] pb-1 overflow-x-auto">
    <button
      type="button"
      onclick={() => setActiveTab('details')}
      class="px-4 py-2 rounded-xl text-xs font-medium font-mono transition flex items-center gap-2 whitespace-nowrap {activeTab === 'details' ? 'bg-violet-600 text-white shadow-[0_0_15px_rgba(124,58,237,0.35)]' : 'text-slate-400 hover:text-white hover:bg-white/5'}"
    >
      <SlidersHorizontal class="w-3.5 h-3.5" />
      <span>Configuration & Details</span>
    </button>

    <button
      type="button"
      onclick={() => setActiveTab('json')}
      class="px-4 py-2 rounded-xl text-xs font-medium font-mono transition flex items-center gap-2 whitespace-nowrap {activeTab === 'json' ? 'bg-violet-600 text-white shadow-[0_0_15px_rgba(124,58,237,0.35)]' : 'text-slate-400 hover:text-white hover:bg-white/5'}"
    >
      <Braces class="w-3.5 h-3.5" />
      <span>Inspect JSON</span>
    </button>

    <button
      type="button"
      onclick={() => setActiveTab('logs')}
      class="px-4 py-2 rounded-xl text-xs font-medium font-mono transition flex items-center gap-2 whitespace-nowrap {activeTab === 'logs' ? 'bg-violet-600 text-white shadow-[0_0_15px_rgba(124,58,237,0.35)]' : 'text-slate-400 hover:text-white hover:bg-white/5'}"
    >
      <FileText class="w-3.5 h-3.5" />
      <span>Logs</span>
    </button>

    <button
      type="button"
      onclick={() => setActiveTab('terminal')}
      class="px-4 py-2 rounded-xl text-xs font-medium font-mono transition flex items-center gap-2 whitespace-nowrap {activeTab === 'terminal' ? 'bg-violet-600 text-white shadow-[0_0_15px_rgba(124,58,237,0.35)]' : 'text-slate-400 hover:text-white hover:bg-white/5'}"
    >
      <Terminal class="w-3.5 h-3.5" />
      <span>Terminal</span>
    </button>

    <button
      type="button"
      onclick={() => setActiveTab('stats')}
      class="px-4 py-2 rounded-xl text-xs font-medium font-mono transition flex items-center gap-2 whitespace-nowrap {activeTab === 'stats' ? 'bg-violet-600 text-white shadow-[0_0_15px_rgba(124,58,237,0.35)]' : 'text-slate-400 hover:text-white hover:bg-white/5'}"
    >
      <Activity class="w-3.5 h-3.5" />
      <span>Live Stats</span>
    </button>

    <button
      type="button"
      onclick={() => setActiveTab('files')}
      class="px-4 py-2 rounded-xl text-xs font-medium font-mono transition flex items-center gap-2 whitespace-nowrap {activeTab === 'files' ? 'bg-violet-600 text-white shadow-[0_0_15px_rgba(124,58,237,0.35)]' : 'text-slate-400 hover:text-white hover:bg-white/5'}"
    >
      <FolderTree class="w-3.5 h-3.5" />
      <span>Files</span>
    </button>
  </div>

  <!-- Tab Content -->
  {#if activeTab === 'details'}
    <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
      <!-- Container Execution & State Details -->
      <div class="p-6 rounded-2xl bg-[#12131c] border border-white/[0.08] space-y-4">
        <h3 class="text-xs font-mono uppercase tracking-wider text-violet-400 font-semibold flex items-center gap-2">
          <Activity class="w-4 h-4" />
          Container State & Runtime
        </h3>

        <div class="grid grid-cols-2 gap-4 text-xs font-mono">
          <div class="space-y-1">
            <span class="text-slate-500 text-[11px]">Process ID (PID)</span>
            <p class="text-slate-200 font-bold">{inspectData?.State?.Pid || 0}</p>
          </div>
          <div class="space-y-1">
            <span class="text-slate-500 text-[11px]">Exit Code</span>
            <p class="text-slate-200 font-bold">{inspectData?.State?.ExitCode ?? 0}</p>
          </div>
          <div class="space-y-1">
            <span class="text-slate-500 text-[11px]">Started At</span>
            <p class="text-slate-200">{formatDate(inspectData?.State?.StartedAt)}</p>
          </div>
          <div class="space-y-1">
            <span class="text-slate-500 text-[11px]">Finished At</span>
            <p class="text-slate-200">{formatDate(inspectData?.State?.FinishedAt)}</p>
          </div>
          <div class="space-y-1">
            <span class="text-slate-500 text-[11px]">OOM Killed</span>
            <p class="text-slate-200">{inspectData?.State?.OOMKilled ? 'Yes (Out of Memory)' : 'No'}</p>
          </div>
          <div class="space-y-1">
            <span class="text-slate-500 text-[11px]">Restarting</span>
            <p class="text-slate-200">{inspectData?.State?.Restarting ? 'Yes' : 'No'}</p>
          </div>
        </div>

        {#if inspectData?.State?.Error}
          <div class="p-3 rounded-xl bg-rose-500/10 border border-rose-500/25 text-rose-300 text-xs font-mono">
            <span class="font-bold block">Fatal Error:</span>
            <span>{inspectData.State.Error}</span>
          </div>
        {/if}
      </div>

      <!-- Execution Command & Config -->
      <div class="p-6 rounded-2xl bg-[#12131c] border border-white/[0.08] space-y-4">
        <h3 class="text-xs font-mono uppercase tracking-wider text-cyan-400 font-semibold flex items-center gap-2">
          <Terminal class="w-4 h-4" />
          Command & Execution
        </h3>

        <div class="space-y-3 text-xs font-mono">
          <div>
            <span class="text-slate-500 text-[11px] block">Command (CMD)</span>
            <div class="p-2.5 rounded-xl bg-[#0a0b10] border border-white/[0.06] text-cyan-300 overflow-x-auto mt-1">
              {#if inspectData?.Config?.Cmd && inspectData.Config.Cmd.length > 0}
                {inspectData.Config.Cmd.join(' ')}
              {:else}
                <span class="text-slate-600">None</span>
              {/if}
            </div>
          </div>

          <div>
            <span class="text-slate-500 text-[11px] block">Entrypoint</span>
            <div class="p-2.5 rounded-xl bg-[#0a0b10] border border-white/[0.06] text-slate-300 overflow-x-auto mt-1">
              {#if inspectData?.Config?.Entrypoint && inspectData.Config.Entrypoint.length > 0}
                {inspectData.Config.Entrypoint.join(' ')}
              {:else}
                <span class="text-slate-600">None</span>
              {/if}
            </div>
          </div>

          <div class="grid grid-cols-2 gap-4">
            <div>
              <span class="text-slate-500 text-[11px] block">Working Directory</span>
              <p class="text-slate-300 mt-1">{inspectData?.Config?.WorkingDir || '/'}</p>
            </div>
            <div>
              <span class="text-slate-500 text-[11px] block">Execution User</span>
              <p class="text-slate-300 mt-1">{inspectData?.Config?.User || 'root'}</p>
            </div>
          </div>
        </div>
      </div>

      <!-- Port Forwarding Table -->
      <div class="p-6 rounded-2xl bg-[#12131c] border border-white/[0.08] space-y-4">
        <div class="flex items-center justify-between">
          <h3 class="text-xs font-mono uppercase tracking-wider text-emerald-400 font-semibold flex items-center gap-2">
            <Network class="w-4 h-4" />
            Port Bindings
          </h3>
          {#if customAppUrl}
            <a
              href={customAppUrl}
              target="_blank"
              rel="noopener noreferrer"
              class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-lg text-xs font-mono bg-violet-500/15 hover:bg-violet-500/25 text-violet-300 border border-violet-500/30 transition group/applink"
              title="Open App: {customAppUrl}"
            >
              <Globe class="w-3.5 h-3.5 text-violet-400 group-hover/applink:text-violet-200" />
              <span>Open App ({formatUrlBadge(customAppUrl)})</span>
              <ExternalLink class="w-3 h-3 opacity-60 group-hover/applink:opacity-100" />
            </a>
          {/if}
        </div>

        {#if portList.length === 0}
          <p class="text-xs text-slate-500 font-mono italic">No mapped network ports.</p>
        {:else}
          <div class="overflow-x-auto">
            <table class="w-full text-left text-xs font-mono">
              <thead>
                <tr class="border-b border-white/[0.08] text-slate-500 text-[10px] uppercase">
                  <th class="py-2">Container Port</th>
                  <th class="py-2">Host Address</th>
                  <th class="py-2 text-right">Host Port</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-white/[0.05]">
                {#each portList as port}
                  <tr>
                    <td class="py-2 text-slate-200">{port.containerPort}</td>
                    <td class="py-2 text-slate-400">{port.hostIp}</td>
                    <td class="py-2 text-right">
                      {#if port.hostPort !== '-'}
                        {@const portUrl = getPortOpenUrl(port.hostPort, inspectData?.Config?.Labels)}
                        <a
                          href={portUrl}
                          target="_blank"
                          rel="noopener noreferrer"
                          class="inline-flex items-center gap-1 text-cyan-400 hover:text-cyan-300 transition"
                          title="Open {portUrl}"
                        >
                          <span>:{port.hostPort}</span>
                          <ExternalLink class="w-3 h-3" />
                        </a>
                      {:else}
                        <span class="text-slate-600">-</span>
                      {/if}
                    </td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        {/if}
      </div>

      <!-- Volume Mounts Table -->
      <div class="p-6 rounded-2xl bg-[#12131c] border border-white/[0.08] space-y-4">
        <h3 class="text-xs font-mono uppercase tracking-wider text-amber-400 font-semibold flex items-center gap-2">
          <HardDrive class="w-4 h-4" />
          Storage & Mounts
        </h3>

        {#if !inspectData?.Mounts || inspectData.Mounts.length === 0}
          <p class="text-xs text-slate-500 font-mono italic">No volume mounts or storage bindings attached.</p>
        {:else}
          <div class="space-y-3">
            {#each inspectData.Mounts as mount}
              <div class="p-3 rounded-xl bg-[#0a0b10] border border-white/[0.06] text-xs font-mono space-y-1.5">
                <div class="flex items-center justify-between">
                  <span class="px-2 py-0.5 rounded-full text-[10px] uppercase font-semibold bg-amber-500/10 text-amber-400 border border-amber-500/20">
                    {mount.Type}
                  </span>
                  <span class="text-[10px] text-slate-400">
                    {mount.RW ? 'Read/Write' : 'Read-Only'}
                  </span>
                </div>
                <div class="grid grid-cols-1 gap-1 text-[11px]">
                  <span class="text-slate-400 truncate" title={mount.Source}>
                    <span class="text-slate-600">Host:</span> {mount.Source}
                  </span>
                  <span class="text-cyan-300 truncate" title={mount.Destination}>
                    <span class="text-slate-600">Dest:</span> {mount.Destination}
                  </span>
                </div>
              </div>
            {/each}
          </div>
        {/if}
      </div>

      <!-- Environment Variables -->
      <div class="lg:col-span-2 p-6 rounded-2xl bg-[#12131c] border border-white/[0.08] space-y-4">
        <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
          <div class="flex flex-wrap items-center gap-3">
            <h3 class="text-xs font-mono uppercase tracking-wider text-emerald-400 font-semibold flex items-center gap-2">
              <FileText class="w-4 h-4" />
              Environment Variables ({filteredEnv.length})
            </h3>

            <!-- Funnel Pills -->
            <div class="flex items-center gap-1 bg-[#0a0b10] p-0.5 rounded-lg border border-white/[0.06] text-[11px] font-mono">
              <button
                type="button"
                onclick={() => envFilterType = 'all'}
                class="px-2 py-0.5 rounded-md transition {envFilterType === 'all' ? 'bg-white/10 text-white font-semibold' : 'text-slate-400 hover:text-slate-200'}"
              >
                All ({analyzedEnvs.length})
              </button>
              <button
                type="button"
                onclick={() => envFilterType = 'user'}
                class="px-2 py-0.5 rounded-md transition flex items-center gap-1 {envFilterType === 'user' ? 'bg-emerald-500/20 text-emerald-300 font-semibold border border-emerald-500/30' : 'text-slate-400 hover:text-slate-200'}"
              >
                User Defined ({userEnvCount})
              </button>
              <button
                type="button"
                onclick={() => envFilterType = 'image'}
                class="px-2 py-0.5 rounded-md transition flex items-center gap-1 {envFilterType === 'image' ? 'bg-slate-700/60 text-slate-200 font-semibold border border-slate-600/40' : 'text-slate-400 hover:text-slate-200'}"
              >
                Image Defaults ({imageEnvCount})
              </button>
            </div>
          </div>

          <div class="flex items-center gap-2">
            <input
              type="text"
              bind:value={envSearchQuery}
              placeholder="Search env keys..."
              class="px-3 py-1 rounded-xl bg-[#0a0b10] border border-white/[0.08] text-white text-xs font-mono focus:outline-none focus:border-cyan-400 w-48"
            />
            <button
              type="button"
              onclick={() => copyText(filteredEnv.map(e => e.raw).join('\n'))}
              class="px-2.5 py-1 rounded-xl bg-white/[0.04] hover:bg-white/[0.08] text-slate-300 text-xs font-mono border border-white/[0.08] flex items-center gap-1.5 transition"
              title="Copy visible environment variables"
            >
              <Copy class="w-3 h-3" />
              <span>Copy</span>
            </button>
          </div>
        </div>

        {#if filteredEnv.length === 0}
          <p class="text-xs text-slate-500 font-mono italic">No matching environment variables.</p>
        {:else}
          <div class="max-h-72 overflow-y-auto space-y-1.5 font-mono text-xs pr-1">
            {#each filteredEnv as item}
              <div class="p-2 rounded-xl bg-[#0a0b10] border border-white/[0.04] flex items-center justify-between gap-3 group">
                <div class="flex items-center gap-2 shrink-0 min-w-0 max-w-[50%]">
                  {#if item.origin === 'user'}
                    <span class="px-1.5 py-0.5 rounded text-[9px] font-mono bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 shrink-0" title="User defined in Compose or run command">
                      USER
                    </span>
                  {:else if item.origin === 'override'}
                    <span class="px-1.5 py-0.5 rounded text-[9px] font-mono bg-cyan-500/10 text-cyan-300 border border-cyan-500/20 shrink-0" title="Overridden by user from base image default">
                      OVERRIDE
                    </span>
                  {:else}
                    <span class="px-1.5 py-0.5 rounded text-[9px] font-mono bg-slate-700/40 text-slate-400 border border-slate-600/30 shrink-0" title="Baked into upstream base image Dockerfile">
                      IMAGE
                    </span>
                  {/if}
                  <span class="truncate {item.origin === 'image' ? 'text-slate-300' : 'text-emerald-400 font-bold'}">{item.key}=</span>
                </div>
                <span class="text-slate-300 truncate text-right flex-1 select-all">{item.val}</span>
              </div>
            {/each}
          </div>
        {/if}
      </div>

      <!-- Container Labels Card -->
      <div class="p-6 rounded-2xl bg-[#12131c] border border-white/[0.08] space-y-4">
        <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-3 border-b border-white/[0.08]">
          <div class="flex items-center gap-2">
            <Tag class="w-4 h-4 text-violet-400" />
            <h3 class="text-xs font-mono uppercase tracking-wider text-white font-bold">
              Container Labels ({totalLabelsCount})
            </h3>
          </div>

          <div class="flex items-center gap-2">
            <input
              type="text"
              bind:value={labelSearchQuery}
              placeholder="Search labels..."
              class="px-3 py-1 rounded-xl bg-[#0a0b10] border border-white/[0.08] text-white text-xs font-mono focus:outline-none focus:border-violet-400 w-48"
            />
            {#if totalLabelsCount > 0}
              <button
                type="button"
                onclick={() => copyText(containerLabelsList.map(l => `${l.key}=${l.value}`).join('\n'))}
                class="px-2.5 py-1 rounded-xl bg-white/[0.04] hover:bg-white/[0.08] text-slate-300 text-xs font-mono border border-white/[0.08] flex items-center gap-1.5 transition"
                title="Copy visible labels"
              >
                <Copy class="w-3 h-3" />
                <span>Copy</span>
              </button>
            {/if}
          </div>
        </div>

        {#if containerLabelsList.length === 0}
          <p class="text-xs text-slate-500 font-mono italic">
            {totalLabelsCount === 0 ? 'No labels attached to this container.' : 'No matching labels found.'}
          </p>
        {:else}
          <div class="max-h-72 overflow-y-auto space-y-1.5 font-mono text-xs pr-1">
            {#each containerLabelsList as item}
              <div class="p-2.5 rounded-xl bg-[#0a0b10] border border-white/[0.04] flex items-center justify-between gap-3 group">
                <div class="flex items-center gap-2 shrink-0 min-w-0 max-w-[45%]">
                  <span class="px-1.5 py-0.5 rounded text-[9px] font-mono font-medium {item.key.startsWith('pdswarm.') ? 'bg-violet-500/10 text-violet-400 border border-violet-500/20' : 'bg-slate-800 text-slate-400 border border-slate-700/50'} shrink-0">
                    {item.key.startsWith('pdswarm.') ? 'PDSWARM' : 'LABEL'}
                  </span>
                  <span class="truncate font-semibold {item.key.startsWith('pdswarm.') ? 'text-violet-300' : 'text-slate-300'}" title={item.key}>
                    {item.key}
                  </span>
                </div>
                <div class="flex items-center gap-2 flex-1 justify-end min-w-0">
                  {#if item.key === 'pdswarm.url' || item.value.startsWith('http://') || item.value.startsWith('https://')}
                    <a
                      href={item.value}
                      target="_blank"
                      rel="noopener noreferrer"
                      class="inline-flex items-center gap-1 text-cyan-400 hover:text-cyan-300 hover:underline truncate text-right font-medium"
                      title="Open external URL"
                    >
                      <span class="truncate">{item.value}</span>
                      <ExternalLink class="w-3 h-3 shrink-0" />
                    </a>
                  {:else}
                    <span class="text-slate-400 truncate text-right select-all" title={item.value}>
                      {item.value}
                    </span>
                  {/if}
                </div>
              </div>
            {/each}
          </div>
        {/if}
      </div>
    </div>
  {:else if activeTab === 'json'}
    <!-- Inspect JSON Viewer -->
    <div class="p-6 rounded-2xl bg-[#12131c] border border-white/[0.08] space-y-4">
      <div class="flex items-center justify-between pb-3 border-b border-white/[0.08]">
        <div class="flex items-center gap-2">
          <Braces class="w-4 h-4 text-violet-400" />
          <h3 class="text-xs font-mono uppercase tracking-wider text-white font-bold">Raw Container Inspect Payload</h3>
        </div>
        <button
          type="button"
          onclick={() => copyText(JSON.stringify(inspectData, null, 2))}
          class="px-3 py-1.5 rounded-xl bg-violet-600 hover:bg-violet-500 text-white text-xs font-mono transition flex items-center gap-1.5 shadow-[0_0_15px_rgba(124,58,237,0.3)]"
        >
          {#if isCopied}
            <Check class="w-3.5 h-3.5" />
            <span>Copied!</span>
          {:else}
            <Copy class="w-3.5 h-3.5" />
            <span>Copy JSON</span>
          {/if}
        </button>
      </div>

      <pre class="p-4 rounded-xl bg-[#0a0b10] border border-white/[0.06] text-cyan-300 text-xs font-mono overflow-x-auto max-h-[600px] select-all leading-relaxed">{JSON.stringify(inspectData, null, 2)}</pre>
    </div>
  {:else if activeTab === 'logs'}
    <LogsViewer containerId={effectiveContainerId} containerName={cleanName} />
  {:else if activeTab === 'terminal'}
    <TerminalView containerId={effectiveContainerId} containerName={cleanName} />
  {:else if activeTab === 'stats'}
    <LiveStatsChart containerId={effectiveContainerId} containerName={cleanName} />
  {:else if activeTab === 'files'}
    <FileBrowser containerId={effectiveContainerId} isRunning={inspectData?.State?.Running ?? false} />
  {/if}
</div>

<!-- Delete Confirmation Modal -->
{#if isDeleteConfirmOpen}
  <div
    class="fixed inset-0 z-50 flex items-center justify-center p-4"
    role="dialog"
    aria-modal="true"
    aria-labelledby="delete-detail-title"
    tabindex="-1"
  >
    <button
      type="button"
      class="fixed inset-0 bg-black/75 backdrop-blur-md cursor-default border-0"
      onclick={() => isDeleteConfirmOpen = false}
      aria-label="Close delete modal"
      tabindex="-1"
    ></button>

    <div class="relative w-full max-w-md rounded-2xl bg-[#12131c] border border-white/[0.12] p-6 space-y-4 shadow-[0_10px_50px_rgba(0,0,0,0.8)]">
      <div class="flex items-center gap-3">
        <div class="w-10 h-10 rounded-xl bg-rose-500/20 border border-rose-500/30 flex items-center justify-center text-rose-400">
          <Trash class="w-5 h-5" />
        </div>
        <div>
          <h3 id="delete-detail-title" class="text-sm font-bold text-white">Delete Container?</h3>
          <p class="text-xs text-slate-400">{cleanName} ({effectiveContainerId.slice(0, 12)})</p>
        </div>
      </div>

      <p class="text-xs text-slate-300 leading-relaxed bg-[#0a0b10] p-3 rounded-xl border border-white/[0.06]">
        This will forcefully stop and delete the container and remove its filesystem layer. This action cannot be reversed.
      </p>

      <div class="flex items-center justify-end gap-3 pt-2">
        <button
          type="button"
          onclick={() => isDeleteConfirmOpen = false}
          class="px-4 py-2 rounded-xl hover:bg-white/10 text-slate-300 text-xs font-semibold transition"
        >
          Cancel
        </button>
        <button
          type="button"
          onclick={() => {
            isDeleteConfirmOpen = false;
            handleAction('remove');
          }}
          class="px-4 py-2 rounded-xl bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold shadow-[0_0_15px_rgba(244,63,94,0.4)] transition"
        >
          Confirm Delete
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Duplicate / Edit Container Modal -->
<ContainerCreateModal
  isOpen={isDuplicateModalOpen}
  duplicateSource={inspectData}
  mode="edit"
  onNavigateToStack={onNavigateToStack}
  onClose={() => isDuplicateModalOpen = false}
  onCreated={async (newId) => {
    isDuplicateModalOpen = false;
    showFeedback('info', 'Container recreation initiated in background...');
    if (newId === effectiveContainerId) {
      await loadContainer();
    } else {
      replacedContainerId = newId;
      if (onReplaced) {
        onReplaced(newId);
      }
      await loadContainer();
    }
  }}
/>

<!-- Update & Recreate Container Confirmation Modal -->
<UpdateContainerModal
  isOpen={isUpdateModalOpen}
  container={inspectData}
  isUpdating={isUpdatingImage}
  onConfirm={confirmUpdateImage}
  onClose={() => {
    isUpdateModalOpen = false;
  }}
/>

