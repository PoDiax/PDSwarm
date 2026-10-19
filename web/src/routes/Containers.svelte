<script lang="ts">
  import {
    onMount,
    onDestroy } from "svelte";
  import {
    Box,
    Plus,
    Search,
    RefreshCw,
    Play,
    Square,
    RotateCw,
    Trash,
    X,
    CircleAlert,
    CircleCheck,
    LoaderCircle,
    Scissors,
    Sparkles,
  } from '@lucide/svelte';
  import ContainerTable, {
    type ContainerSortKey,
    type ContainerSortDirection,
  } from "../lib/components/ContainerTable.svelte";
  import ContainerCreateModal from "../lib/components/ContainerCreateModal.svelte";
  import UpdateContainerModal from "../lib/components/UpdateContainerModal.svelte";
  import { containerApi, taskApi } from "../lib/api";
  import { imageUpdateStore } from "../lib/stores/imageUpdates.svelte";
  import { tasksStore } from "../lib/stores/tasks.svelte";
  import type {
    Container,
    PortMapping,
    ContainerInspectInfo,
  } from "../lib/types";

  interface Props {
    onSelectContainer?: (containerId: string) => void;
    onNavigate?: (tab: string) => void;
    onNavigateToStack?: (stackName: string) => void;
  }

  let { onSelectContainer, onNavigate, onNavigateToStack }: Props = $props();

  // State
  let containers = $state<Container[]>([]);
  let isLoading = $state(true);
  let isRefreshing = $state(false);
  let error = $state<string | null>(null);
  let feedbackMessage = $state<{
    type: "success" | "error" | "info";
    text: string;
  } | null>(null);

  // Filters & Search
  let searchQuery = $state("");
  let statusFilter = $state<"all" | "running" | "exited" | "paused">("all");
  let sortBy = $state<
    | "created-desc"
    | "created-asc"
    | "name-asc"
    | "name-desc"
    | "status-asc"
    | "status-desc"
    | "image-asc"
    | "image-desc"
    | "updates"
  >("created-desc");

  let sortKey = $derived.by<ContainerSortKey>(() => {
    if (sortBy.startsWith("name")) return "name";
    if (sortBy.startsWith("status")) return "status";
    if (sortBy.startsWith("image")) return "image";
    if (sortBy.startsWith("created")) return "created";
    return "created";
  });

  let sortDirection = $derived.by<ContainerSortDirection>(() => {
    return sortBy.endsWith("-asc") ? "asc" : "desc";
  });

  function handleSort(key: ContainerSortKey) {
    if (sortKey === key) {
      const nextDir = sortDirection === "asc" ? "desc" : "asc";
      sortBy = `${key}-${nextDir}` as any;
    } else {
      const defaultDir = key === "created" ? "desc" : "asc";
      sortBy = `${key}-${defaultDir}` as any;
    }
  }

  // Selection & Batch actions
  let selectedIds = $state<string[]>([]);
  let actionLoading = $state<Record<string, string>>({});
  let isBatchBusy = $state(false);

  // Modals
  let isCreateModalOpen = $state(false);
  let duplicateSource = $state<ContainerInspectInfo | Container | null>(null);
  let modalMode = $state<"create" | "duplicate" | "edit">("create");
  let isPruneConfirmOpen = $state(false);
  let isBatchDeleteConfirmOpen = $state(false);
  let updateContainerTarget = $state<Container | null>(null);
  let isUpdateModalOpen = $state(false);
  let isConfirmUpdating = $state(false);

  function handleOpenCreate() {
    duplicateSource = null;
    modalMode = "create";
    isCreateModalOpen = true;
  }

  async function handleDuplicate(c: Container) {
    try {
      actionLoading = { ...actionLoading, [c.Id]: "inspect" };
      const details = await containerApi.inspect(c.Id);
      duplicateSource = details;
    } catch {
      duplicateSource = c;
    } finally {
      const next = { ...actionLoading };
      delete next[c.Id];
      actionLoading = next;
    }
    modalMode = "duplicate";
    isCreateModalOpen = true;
  }

  async function handleEdit(c: Container) {
    try {
      actionLoading = { ...actionLoading, [c.Id]: "inspect" };
      const details = await containerApi.inspect(c.Id);
      duplicateSource = details;
    } catch {
      duplicateSource = c;
    } finally {
      const next = { ...actionLoading };
      delete next[c.Id];
      actionLoading = next;
    }
    modalMode = "edit";
    isCreateModalOpen = true;
  }

  let isCheckingUpdates = $state(false);

  async function handleCheckAllUpdates() {
    isCheckingUpdates = true;
    try {
      const imagesToCheck = containers.map((c) => c.Image).filter(Boolean);
      await imageUpdateStore.checkMultiple(imagesToCheck, true);
      const updatesFound = containers.filter((c) =>
        imageUpdateStore.hasUpdate(c.Image),
      ).length;
      if (updatesFound > 0) {
        showFeedback(
          "success",
          `Found ${updatesFound} container(s) with available image updates!`,
        );
      } else {
        showFeedback(
          "success",
          `All ${imagesToCheck.length} container images are up to date!`,
        );
      }
    } catch {
      showFeedback("error", "Failed to check remote registry updates.");
    } finally {
      isCheckingUpdates = false;
    }
  }

  function handleUpdateContainer(c: Container) {
    isConfirmUpdating = false;
    updateContainerTarget = c;
    isUpdateModalOpen = true;
  }

  async function confirmUpdateContainer() {
    if (!updateContainerTarget) return;
    const c = updateContainerTarget;
    const cleanName = c.Names?.[0]
      ? c.Names[0].replace(/^\//, "")
      : c.Id.slice(0, 12);
    const targetImage = c.Image;
    const containerId = c.Id;

    isUpdateModalOpen = false;
    updateContainerTarget = null;
    isConfirmUpdating = false;

    actionLoading = { ...actionLoading, [containerId]: "updating" };
    showFeedback(
      "info",
      `Updating and recreating "${cleanName}" in the background...`,
    );

    try {
      const res = await containerApi.updateImage(containerId);
      tasksStore.poll();

      // Poll until task completes
      if (res && res.task_id) {
        let isDone = false;
        let attempts = 0;
        while (!isDone && attempts < 180) {
          attempts++;
          await new Promise((r) => setTimeout(r, 1200));
          try {
            const task = await taskApi.get(res.task_id);
            if (task.status === "completed") {
              isDone = true;
              break;
            } else if (
              task.status === "failed" ||
              task.status === "cancelled"
            ) {
              throw new Error(task.error || `Update ${task.status}`);
            }
          } catch (e: unknown) {
            if (
              e instanceof Error &&
              (e.message.includes("failed") ||
                e.message.includes("cancelled") ||
                e.message.includes("Update "))
            ) {
              throw e;
            }
          }
        }
      }

      // Re-query image update status and refresh containers
      if (targetImage) {
        await imageUpdateStore.checkUpdate(targetImage);
      }
      await fetchContainers(true);
      showFeedback(
        "success",
        `Container "${cleanName}" was successfully updated with the latest image and recreated!`,
      );
    } catch (err: unknown) {
      showFeedback(
        "error",
        err instanceof Error
          ? err.message
          : `Failed to update container ${cleanName}`,
      );
    } finally {
      const next = { ...actionLoading };
      delete next[containerId];
      actionLoading = next;
    }
  }

  let refreshTimer: ReturnType<typeof setInterval> | null = null;

  async function fetchContainers(silent = false) {
    if (!silent) isRefreshing = true;
    error = null;

    try {
      const data = await containerApi.list(true);
      containers = data || [];
      // Clean up selectedIds that no longer exist
      const existingIds = new Set(containers.map((c) => c.Id));
      selectedIds = selectedIds.filter((id) => existingIds.has(id));

      // Automatically check updates for container images if needed/due
      const imagesToCheck = containers.map((c) => c.Image).filter(Boolean);
      if (imagesToCheck.length > 0) {
        imageUpdateStore.checkIfNeeded(imagesToCheck);
      }
    } catch (err: unknown) {
      if (err instanceof Error) {
        error = err.message;
      } else {
        error = "Failed to load containers from Docker daemon.";
      }
    } finally {
      isLoading = false;
      isRefreshing = false;
    }
  }

  onMount(() => {
    fetchContainers();
    // Auto-refresh every 10 seconds
    refreshTimer = setInterval(() => {
      fetchContainers(true);
    }, 10000);
  });

  onDestroy(() => {
    if (refreshTimer) clearInterval(refreshTimer);
  });

  function showFeedback(type: "success" | "error" | "info", text: string) {
    feedbackMessage = { type, text };
    setTimeout(() => {
      if (feedbackMessage?.text === text) {
        feedbackMessage = null;
      }
    }, 4000);
  }

  // Filtered containers
  let filteredContainers = $derived(
    containers.filter((c) => {
      // Status filter
      const state = (c.State || "").toLowerCase();
      if (statusFilter === "running" && state !== "running") return false;
      if (statusFilter === "paused" && state !== "paused") return false;
      if (
        statusFilter === "exited" &&
        (state === "running" || state === "paused")
      )
        return false;

      // Search query
      if (!searchQuery.trim()) return true;
      const q = searchQuery.toLowerCase().trim();

      const nameMatch = c.Names?.some((n: string) =>
        n.toLowerCase().includes(q),
      );
      const imageMatch = (c.Image || "").toLowerCase().includes(q);
      const idMatch = (c.Id || "").toLowerCase().includes(q);
      const portMatch = c.Ports?.some(
        (p: PortMapping) =>
          (p.PublicPort && String(p.PublicPort).includes(q)) ||
          (p.PrivatePort && String(p.PrivatePort).includes(q)),
      );

      return Boolean(nameMatch || imageMatch || idMatch || portMatch);
    }),
  );

  // Sorted containers
  let sortedContainers = $derived.by(() => {
    const list = [...filteredContainers];
    list.sort((a, b) => {
      if (sortBy === "updates") {
        const aUp = imageUpdateStore.hasUpdate(a.Image) ? 1 : 0;
        const bUp = imageUpdateStore.hasUpdate(b.Image) ? 1 : 0;
        if (aUp !== bUp) return bUp - aUp;
        return (b.Created || 0) - (a.Created || 0);
      }
      if (sortBy === "name-asc") {
        const nameA = (a.Names?.[0] || "").replace(/^\//, "");
        const nameB = (b.Names?.[0] || "").replace(/^\//, "");
        return nameA.localeCompare(nameB);
      }
      if (sortBy === "name-desc") {
        const nameA = (a.Names?.[0] || "").replace(/^\//, "");
        const nameB = (b.Names?.[0] || "").replace(/^\//, "");
        return nameB.localeCompare(nameA);
      }
      if (sortBy === "status-asc") {
        return (a.State || "").localeCompare(b.State || "");
      }
      if (sortBy === "status-desc") {
        return (b.State || "").localeCompare(a.State || "");
      }
      if (sortBy === "image-asc") {
        return (a.Image || "").localeCompare(b.Image || "");
      }
      if (sortBy === "image-desc") {
        return (b.Image || "").localeCompare(a.Image || "");
      }
      if (sortBy === "created-asc") {
        return (a.Created || 0) - (b.Created || 0);
      }
      // created-desc
      return (b.Created || 0) - (a.Created || 0);
    });
    return list;
  });

  // Status statistics
  let stats = $derived({
    total: containers.length,
    running: containers.filter(
      (c) => (c.State || "").toLowerCase() === "running",
    ).length,
    paused: containers.filter((c) => (c.State || "").toLowerCase() === "paused")
      .length,
    stopped: containers.filter((c) => {
      const s = (c.State || "").toLowerCase();
      return s !== "running" && s !== "paused";
    }).length,
  });

  // Selection handlers
  function handleToggleSelect(containerId: string) {
    if (selectedIds.includes(containerId)) {
      selectedIds = selectedIds.filter((id) => id !== containerId);
    } else {
      selectedIds = [...selectedIds, containerId];
    }
  }

  function handleToggleSelectAll() {
    const currentFilteredIds = sortedContainers.map((c) => c.Id);
    const allFilteredSelected =
      currentFilteredIds.length > 0 &&
      currentFilteredIds.every((id) => selectedIds.includes(id));

    if (allFilteredSelected) {
      selectedIds = selectedIds.filter(
        (id) => !currentFilteredIds.includes(id),
      );
    } else {
      const merged = new Set([...selectedIds, ...currentFilteredIds]);
      selectedIds = Array.from(merged);
    }
  }

  function handleClearSelection() {
    selectedIds = [];
  }

  // Row-level action handler
  async function handleRowAction(
    action:
      | "start"
      | "stop"
      | "restart"
      | "pause"
      | "unpause"
      | "kill"
      | "remove",
    containerId: string,
  ) {
    actionLoading = { ...actionLoading, [containerId]: action };

    try {
      if (action === "start") {
        await containerApi.start(containerId);
        showFeedback(
          "success",
          `Container ${containerId.slice(0, 12)} started.`,
        );
      } else if (action === "stop") {
        await containerApi.stop(containerId);
        showFeedback(
          "success",
          `Container ${containerId.slice(0, 12)} stopped.`,
        );
      } else if (action === "restart") {
        await containerApi.restart(containerId);
        showFeedback(
          "success",
          `Container ${containerId.slice(0, 12)} restarted.`,
        );
      } else if (action === "pause") {
        await containerApi.pause(containerId);
        showFeedback(
          "success",
          `Container ${containerId.slice(0, 12)} paused.`,
        );
      } else if (action === "unpause") {
        await containerApi.unpause(containerId);
        showFeedback(
          "success",
          `Container ${containerId.slice(0, 12)} unpaused.`,
        );
      } else if (action === "kill") {
        await containerApi.kill(containerId);
        showFeedback(
          "success",
          `Container ${containerId.slice(0, 12)} killed.`,
        );
      } else if (action === "remove") {
        await containerApi.remove(containerId, true, true);
        showFeedback(
          "success",
          `Container ${containerId.slice(0, 12)} removed.`,
        );
        selectedIds = selectedIds.filter((id) => id !== containerId);
      }
      await fetchContainers(true);
    } catch (err: unknown) {
      const msg =
        err instanceof Error ? err.message : `Failed to ${action} container`;
      showFeedback("error", msg);
    } finally {
      const next = { ...actionLoading };
      delete next[containerId];
      actionLoading = next;
    }
  }

  // Batch actions handler
  async function handleBatchAction(
    action: "start" | "stop" | "restart" | "remove",
  ) {
    if (selectedIds.length === 0) return;
    if (action === "remove") {
      isBatchDeleteConfirmOpen = true;
      return;
    }

    await executeBatch(action);
  }

  async function executeBatch(action: "start" | "stop" | "restart" | "remove") {
    isBatchBusy = true;
    const count = selectedIds.length;

    try {
      const results = await Promise.allSettled(
        selectedIds.map((id) => {
          if (action === "start") return containerApi.start(id);
          if (action === "stop") return containerApi.stop(id);
          if (action === "restart") return containerApi.restart(id);
          return containerApi.remove(id, true, true);
        }),
      );

      const succeeded = results.filter((r) => r.status === "fulfilled").length;
      const failed = results.filter((r) => r.status === "rejected").length;

      if (failed === 0) {
        showFeedback(
          "success",
          `Successfully ${action}ed ${succeeded} containers.`,
        );
      } else {
        showFeedback(
          "error",
          `${action} completed: ${succeeded} succeeded, ${failed} failed.`,
        );
      }

      if (action === "remove") {
        selectedIds = [];
      }
      await fetchContainers(true);
    } catch {
      showFeedback("error", `Failed during batch ${action} execution.`);
    } finally {
      isBatchBusy = false;
      isBatchDeleteConfirmOpen = false;
    }
  }

  // Prune containers handler
  async function handlePrune() {
    try {
      const res = await containerApi.prune();
      const count = res.ContainersDeleted?.length || 0;
      showFeedback("success", `Pruned ${count} stopped containers.`);
      await fetchContainers(true);
    } catch (err: unknown) {
      const msg =
        err instanceof Error ? err.message : "Failed to prune containers.";
      showFeedback("error", msg);
    } finally {
      isPruneConfirmOpen = false;
    }
  }
</script>

<div class="space-y-6 animate-in fade-in duration-200">
  <!-- Top Navigation & Title Bar -->
  <div
    class="flex flex-col md:flex-row md:items-center md:justify-between gap-4"
  >
    <div>
      <div class="flex items-center gap-2.5">
        <div
          class="w-8 h-8 rounded-xl bg-violet-600/20 border border-violet-500/30 flex items-center justify-center text-violet-400 shadow-[0_0_15px_rgba(124,58,237,0.3)]"
        >
          <Box class="w-4 h-4" />
        </div>
        <h1 class="text-xl font-bold text-white tracking-tight">Containers</h1>
        <span
          class="px-2 py-0.5 rounded-full text-[11px] font-mono bg-violet-500/15 text-violet-300 border border-violet-500/30"
        >
          {stats.total} Total
        </span>
      </div>
      <p class="text-xs text-slate-400 mt-1">
        Manage, inspect, and deploy standalone Docker host workloads
      </p>
    </div>

    <!-- Top Action Buttons -->
    <div class="flex items-center gap-2.5">
      <!-- Prune button -->
      <button
        type="button"
        onclick={() => (isPruneConfirmOpen = true)}
        class="px-3 py-2 rounded-xl bg-[#12131c] hover:bg-white/10 text-slate-300 hover:text-white border border-white/[0.08] text-xs font-semibold transition flex items-center gap-1.5"
        title="Remove all stopped containers"
      >
        <Scissors class="w-3.5 h-3.5 text-slate-400" />
        <span class="hidden sm:inline">Prune Stopped</span>
      </button>

      <!-- Check Image Updates button -->
      <button
        type="button"
        onclick={handleCheckAllUpdates}
        disabled={isCheckingUpdates || isRefreshing}
        class="px-3 py-2 rounded-xl bg-cyan-500/10 hover:bg-cyan-500/20 text-cyan-300 hover:text-white border border-cyan-500/30 text-xs font-semibold transition flex items-center gap-1.5 shadow-[0_0_12px_rgba(6,182,212,0.15)] disabled:opacity-50 cursor-pointer"
        title="Check all container images against remote registries for updates"
      >
        <Sparkles
          class="w-3.5 h-3.5 text-cyan-400 {isCheckingUpdates
            ? 'animate-spin'
            : ''}"
        />
        <span class="hidden sm:inline">Check Updates</span>
      </button>

      <!-- Refresh button -->
      <button
        type="button"
        onclick={() => fetchContainers()}
        disabled={isRefreshing}
        class="p-2 rounded-xl bg-[#12131c] hover:bg-white/10 text-slate-300 hover:text-white border border-white/[0.08] text-xs transition disabled:opacity-50"
        title="Refresh container list"
      >
        <RefreshCw
          class="w-4 h-4 {isRefreshing ? 'animate-spin text-cyan-400' : ''}"
        />
      </button>

      <!-- Launch Container button -->
      <button
        type="button"
        onclick={handleOpenCreate}
        class="px-4 py-2 rounded-xl bg-gradient-to-r from-violet-600 to-indigo-600 hover:from-violet-500 hover:to-indigo-500 text-white text-xs font-semibold shadow-[0_0_20px_rgba(124,58,237,0.4)] transition flex items-center gap-2"
      >
        <Plus class="w-4 h-4" />
        <span>Add Container</span>
      </button>
    </div>
  </div>

  <!-- Feedback Toast Banner -->
  {#if feedbackMessage}
    <div
      class="p-3 rounded-xl border flex items-center justify-between gap-3 text-xs animate-in slide-in-from-top-2 duration-150 {feedbackMessage.type ===
      'success'
        ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-300'
        : feedbackMessage.type === 'info'
          ? 'bg-cyan-500/10 border-cyan-500/30 text-cyan-300 shadow-[0_0_15px_rgba(6,182,212,0.15)]'
          : 'bg-rose-500/10 border-rose-500/30 text-rose-300'}"
    >
      <div class="flex items-center gap-2">
        {#if feedbackMessage.type === "success"}
          <CircleCheck class="w-4 h-4 text-emerald-400 shrink-0" />
        {:else if feedbackMessage.type === "info"}
          <Sparkles class="w-4 h-4 text-cyan-400 shrink-0 animate-pulse" />
        {:else}
          <CircleAlert class="w-4 h-4 text-rose-400 shrink-0" />
        {/if}
        <span>{feedbackMessage.text}</span>
      </div>
      <button
        type="button"
        onclick={() => (feedbackMessage = null)}
        class="p-1 rounded hover:bg-white/10 text-slate-400 hover:text-white transition"
      >
        <X class="w-3.5 h-3.5" />
      </button>
    </div>
  {/if}

  {#if error}
    <div
      class="p-4 rounded-xl bg-rose-500/10 border border-rose-500/30 flex items-center justify-between text-xs text-rose-300"
    >
      <div class="flex items-center gap-2">
        <CircleAlert class="w-4 h-4 text-rose-400 shrink-0" />
        <span>{error}</span>
      </div>
      <button
        type="button"
        onclick={() => fetchContainers()}
        class="px-3 py-1 rounded-lg bg-rose-500/20 hover:bg-rose-500/30 text-rose-300 transition text-[11px]"
      >
        Retry
      </button>
    </div>
  {/if}

  <!-- Filters & Search Toolbar -->
  <div
    class="flex flex-col md:flex-row md:items-center justify-between gap-3 p-3 rounded-2xl bg-[#12131c]/70 border border-white/[0.08] backdrop-blur-md"
  >
    <!-- Status Funnel Tabs -->
    <div class="flex items-center gap-1.5 overflow-x-auto pb-1 md:pb-0">
      <button
        type="button"
        onclick={() => (statusFilter = "all")}
        class="px-3 py-1.5 rounded-xl text-xs font-mono transition flex items-center gap-1.5 whitespace-nowrap {statusFilter ===
        'all'
          ? 'bg-violet-600 text-white shadow-[0_0_12px_rgba(124,58,237,0.35)]'
          : 'text-slate-400 hover:text-white hover:bg-white/5'}"
      >
        <span>All</span>
        <span
          class="px-1.5 py-0.2 rounded-full text-[10px] {statusFilter === 'all'
            ? 'bg-white/20'
            : 'bg-white/5 text-slate-500'}"
        >
          {stats.total}
        </span>
      </button>

      <button
        type="button"
        onclick={() => (statusFilter = "running")}
        class="px-3 py-1.5 rounded-xl text-xs font-mono transition flex items-center gap-1.5 whitespace-nowrap {statusFilter ===
        'running'
          ? 'bg-emerald-600 text-white shadow-[0_0_12px_rgba(16,185,129,0.35)]'
          : 'text-slate-400 hover:text-white hover:bg-white/5'}"
      >
        <span class="w-1.5 h-1.5 rounded-full bg-emerald-400"></span>
        <span>Running</span>
        <span
          class="px-1.5 py-0.2 rounded-full text-[10px] {statusFilter ===
          'running'
            ? 'bg-white/20'
            : 'bg-white/5 text-slate-500'}"
        >
          {stats.running}
        </span>
      </button>

      <button
        type="button"
        onclick={() => (statusFilter = "exited")}
        class="px-3 py-1.5 rounded-xl text-xs font-mono transition flex items-center gap-1.5 whitespace-nowrap {statusFilter ===
        'exited'
          ? 'bg-rose-600 text-white shadow-[0_0_12px_rgba(244,63,94,0.35)]'
          : 'text-slate-400 hover:text-white hover:bg-white/5'}"
      >
        <span class="w-1.5 h-1.5 rounded-full bg-rose-400"></span>
        <span>Stopped</span>
        <span
          class="px-1.5 py-0.2 rounded-full text-[10px] {statusFilter ===
          'exited'
            ? 'bg-white/20'
            : 'bg-white/5 text-slate-500'}"
        >
          {stats.stopped}
        </span>
      </button>

      <button
        type="button"
        onclick={() => (statusFilter = "paused")}
        class="px-3 py-1.5 rounded-xl text-xs font-mono transition flex items-center gap-1.5 whitespace-nowrap {statusFilter ===
        'paused'
          ? 'bg-amber-600 text-white shadow-[0_0_12px_rgba(245,158,11,0.35)]'
          : 'text-slate-400 hover:text-white hover:bg-white/5'}"
      >
        <span class="w-1.5 h-1.5 rounded-full bg-amber-400"></span>
        <span>Paused</span>
        <span
          class="px-1.5 py-0.2 rounded-full text-[10px] {statusFilter ===
          'paused'
            ? 'bg-white/20'
            : 'bg-white/5 text-slate-500'}"
        >
          {stats.paused}
        </span>
      </button>
    </div>

    <!-- Search Input & Sort Dropdown -->
    <div class="flex items-center gap-2 w-full md:w-auto">
      <div class="relative flex-1 md:w-64">
        <Search
          class="absolute left-3 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-slate-500"
        />
        <input
          type="text"
          bind:value={searchQuery}
          placeholder="Funnel by name, image, port..."
          class="w-full pl-9 pr-8 py-1.5 rounded-xl bg-[#0a0b10] border border-white/[0.08] text-white placeholder-slate-500 focus:outline-none focus:border-cyan-400 text-xs font-mono transition"
        />
        {#if searchQuery}
          <button
            type="button"
            onclick={() => (searchQuery = "")}
            class="absolute right-2.5 top-1/2 -translate-y-1/2 p-0.5 rounded-md hover:bg-white/10 text-slate-500 hover:text-white"
          >
            <X class="w-3 h-3" />
          </button>
        {/if}
      </div>

      <select
        bind:value={sortBy}
        class="px-3 py-1.5 rounded-xl bg-[#0a0b10] border border-white/[0.08] text-slate-300 text-xs focus:outline-none focus:border-cyan-400 font-mono cursor-pointer shrink-0"
        title="Sort containers"
      >
        <option value="created-desc">Newest First</option>
        <option value="created-asc">Oldest First</option>
        <option value="name-asc">Name (A-Z)</option>
        <option value="name-desc">Name (Z-A)</option>
        <option value="status-asc">Status</option>
        <option value="image-asc">Image (A-Z)</option>
        <option value="updates">Updates Available First</option>
      </select>
    </div>
  </div>

  <!-- Batch Actions Toolbar (Visible when 1+ selected) -->
  {#if selectedIds.length > 0}
    <div
      class="p-3 rounded-2xl bg-gradient-to-r from-violet-950/70 via-[#12131c] to-indigo-950/70 border border-violet-500/40 shadow-[0_0_25px_rgba(124,58,237,0.25)] backdrop-blur-md flex flex-wrap items-center justify-between gap-3 animate-in fade-in slide-in-from-top-2 duration-150"
    >
      <div class="flex items-center gap-2.5">
        <span class="w-2 h-2 rounded-full bg-cyan-400 shadow-[0_0_8px_#00f5ff]"
        ></span>
        <span class="text-xs font-bold text-white font-mono">
          {selectedIds.length}
          {selectedIds.length === 1 ? "container" : "containers"} selected
        </span>
        <button
          type="button"
          onclick={handleClearSelection}
          class="text-[11px] font-mono text-slate-400 hover:text-cyan-300 underline ml-2"
        >
          Deselect
        </button>
      </div>

      <div class="flex items-center gap-2">
        {#if isBatchBusy}
          <div
            class="flex items-center gap-2 px-3 py-1.5 text-xs text-violet-300 font-mono"
          >
            <LoaderCircle class="w-4 h-4 animate-spin text-cyan-400" />
            <span>Processing batch action...</span>
          </div>
        {:else}
          <!-- Start Selected -->
          <button
            type="button"
            onclick={() => handleBatchAction("start")}
            class="px-3 py-1.5 rounded-xl bg-emerald-600/20 hover:bg-emerald-600/30 text-emerald-300 border border-emerald-500/30 text-xs font-semibold transition inline-flex items-center gap-1.5"
            title="Start selected containers"
          >
            <Play class="w-3.5 h-3.5 fill-current" />
            <span>Start</span>
          </button>

          <!-- Stop Selected -->
          <button
            type="button"
            onclick={() => handleBatchAction("stop")}
            class="px-3 py-1.5 rounded-xl bg-rose-600/20 hover:bg-rose-600/30 text-rose-300 border border-rose-500/30 text-xs font-semibold transition inline-flex items-center gap-1.5"
            title="Stop selected containers"
          >
            <Square class="w-3.5 h-3.5 fill-current" />
            <span>Stop</span>
          </button>

          <!-- Restart Selected -->
          <button
            type="button"
            onclick={() => handleBatchAction("restart")}
            class="px-3 py-1.5 rounded-xl bg-cyan-600/20 hover:bg-cyan-600/30 text-cyan-300 border border-cyan-500/30 text-xs font-semibold transition inline-flex items-center gap-1.5"
            title="Restart selected containers"
          >
            <RotateCw class="w-3.5 h-3.5" />
            <span>Restart</span>
          </button>

          <!-- Remove Selected -->
          <button
            type="button"
            onclick={() => handleBatchAction("remove")}
            class="px-3 py-1.5 rounded-xl bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold transition inline-flex items-center gap-1.5 shadow-[0_0_12px_rgba(244,63,94,0.3)]"
            title="Force remove selected containers"
          >
            <Trash class="w-3.5 h-3.5" />
            <span>Remove</span>
          </button>
        {/if}
      </div>
    </div>
  {/if}

  <!-- Containers Table -->
  <ContainerTable
    containers={sortedContainers}
    {selectedIds}
    {actionLoading}
    {sortKey}
    {sortDirection}
    onSort={handleSort}
    onSelect={(id) => onSelectContainer?.(id)}
    onToggleSelect={handleToggleSelect}
    onToggleSelectAll={handleToggleSelectAll}
    onAction={handleRowAction}
    onDuplicate={handleDuplicate}
    onEdit={handleEdit}
    onUpdateContainer={handleUpdateContainer}
    onCreateClick={handleOpenCreate}
  />
</div>

<!-- Container Creation / Duplication Modal -->
<ContainerCreateModal
  isOpen={isCreateModalOpen}
  {duplicateSource}
  mode={modalMode}
  {onNavigateToStack}
  onClose={() => {
    isCreateModalOpen = false;
    duplicateSource = null;
    modalMode = "create";
  }}
  onCreated={(_id) => {
    fetchContainers(true);
    showFeedback(
      "success",
      modalMode === "duplicate"
        ? "Container duplicated successfully."
        : modalMode === "edit"
          ? "Container recreation initiated in background."
          : "Container deployed successfully.",
    );
  }}
/>

<!-- Prune Stopped Confirmation Modal -->
{#if isPruneConfirmOpen}
  <div
    class="fixed inset-0 z-50 flex items-center justify-center p-4"
    role="dialog"
    aria-modal="true"
    aria-labelledby="prune-title"
    tabindex="-1"
  >
    <button
      type="button"
      class="fixed inset-0 bg-black/75 backdrop-blur-md cursor-default border-0"
      onclick={() => (isPruneConfirmOpen = false)}
      aria-label="Close prune modal"
      tabindex="-1"
    ></button>

    <div
      class="relative w-full max-w-md rounded-2xl bg-[#12131c] border border-white/[0.12] p-6 space-y-4 shadow-[0_10px_50px_rgba(0,0,0,0.8)]"
    >
      <div class="flex items-center gap-3">
        <div
          class="w-10 h-10 rounded-xl bg-rose-500/20 border border-rose-500/30 flex items-center justify-center text-rose-400"
        >
          <Scissors class="w-5 h-5" />
        </div>
        <div>
          <h3 id="prune-title" class="text-sm font-bold text-white">
            Prune Stopped Containers?
          </h3>
          <p class="text-xs text-slate-400">
            This will remove all containers in stopped / exited state.
          </p>
        </div>
      </div>

      <p
        class="text-xs text-slate-300 leading-relaxed bg-[#0a0b10] p-3 rounded-xl border border-white/[0.06]"
      >
        Running containers will not be affected. Any non-persistent data stored
        inside stopped container file systems will be deleted permanently.
      </p>

      <div class="flex items-center justify-end gap-3 pt-2">
        <button
          type="button"
          onclick={() => (isPruneConfirmOpen = false)}
          class="px-4 py-2 rounded-xl hover:bg-white/10 text-slate-300 text-xs font-semibold transition"
        >
          Cancel
        </button>
        <button
          type="button"
          onclick={handlePrune}
          class="px-4 py-2 rounded-xl bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold shadow-[0_0_15px_rgba(244,63,94,0.4)] transition"
        >
          Confirm Prune
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Batch Delete Confirmation Modal -->
{#if isBatchDeleteConfirmOpen}
  <div
    class="fixed inset-0 z-50 flex items-center justify-center p-4"
    role="dialog"
    aria-modal="true"
    aria-labelledby="batch-delete-title"
    tabindex="-1"
  >
    <button
      type="button"
      class="fixed inset-0 bg-black/75 backdrop-blur-md cursor-default border-0"
      onclick={() => (isBatchDeleteConfirmOpen = false)}
      aria-label="Close batch delete modal"
      tabindex="-1"
    ></button>

    <div
      class="relative w-full max-w-md rounded-2xl bg-[#12131c] border border-white/[0.12] p-6 space-y-4 shadow-[0_10px_50px_rgba(0,0,0,0.8)]"
    >
      <div class="flex items-center gap-3">
        <div
          class="w-10 h-10 rounded-xl bg-rose-500/20 border border-rose-500/30 flex items-center justify-center text-rose-400"
        >
          <Trash class="w-5 h-5" />
        </div>
        <div>
          <h3 id="batch-delete-title" class="text-sm font-bold text-white">
            Delete Selected Containers?
          </h3>
          <p class="text-xs text-slate-400">
            {selectedIds.length} container(s) will be forcefully deleted.
          </p>
        </div>
      </div>

      <p
        class="text-xs text-slate-300 leading-relaxed bg-[#0a0b10] p-3 rounded-xl border border-white/[0.06]"
      >
        This will stop and remove all selected containers and associated
        anonymous volumes. This operation cannot be undone.
      </p>

      <div class="flex items-center justify-end gap-3 pt-2">
        <button
          type="button"
          onclick={() => (isBatchDeleteConfirmOpen = false)}
          class="px-4 py-2 rounded-xl hover:bg-white/10 text-slate-300 text-xs font-semibold transition"
        >
          Cancel
        </button>
        <button
          type="button"
          onclick={() => executeBatch("remove")}
          class="px-4 py-2 rounded-xl bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold shadow-[0_0_15px_rgba(244,63,94,0.4)] transition"
        >
          Delete All Selected
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Update & Recreate Container Confirmation Modal -->
<UpdateContainerModal
  isOpen={isUpdateModalOpen}
  container={updateContainerTarget}
  isUpdating={Boolean(
    updateContainerTarget &&
      actionLoading[updateContainerTarget.Id] === "updating",
  )}
  onConfirm={confirmUpdateContainer}
  onClose={() => {
    isUpdateModalOpen = false;
    updateContainerTarget = null;
    isConfirmUpdating = false;
  }}
/>
