<script lang="ts">
  import {
    onMount,
    onDestroy } from "svelte";
  import {
    Layers,
    Boxes,
    Plus,
    RefreshCw,
    Play,
    RotateCw,
    Square,
    ArrowDownToLine,
    Trash,
    PenLine,
    Search,
    Box,
    CircleCheck,
    CircleAlert,
    LoaderCircle,
    ChevronDown,
    ChevronRight,
    Copy,
    Check,
    TriangleAlert,
    Clock,
    ShieldCheck,
    ShieldAlert,
    Sparkles,
    CircleArrowUp,
    Folder,
  } from '@lucide/svelte';
  import AdoptStackModal from "../lib/components/AdoptStackModal.svelte";
  import ViewportGate from "../lib/components/ViewportGate.svelte";
  import { stackApi, containerApi } from "../lib/api";
  import { imageUpdateStore } from "../lib/stores/imageUpdates.svelte";
  import { tasksStore } from "../lib/stores/tasks.svelte";
  import type { Stack, Container } from "../lib/types";
  import {
    detectRelativeVolumes,
    resolveRelativeVolumesInYaml,
    getStackComposeDir,
  } from "../lib/utils/compose";

  interface Props {
    onNewStack?: () => void;
    onEditStack?: (name: string) => void;
    onNavigate?: (tab: string) => void;
    onSelectContainer?: (id: string) => void;
  }

  let { onNewStack, onEditStack, onNavigate, onSelectContainer }: Props =
    $props();

  // State
  let stacks = $state<Stack[]>([]);
  let containers = $state<Container[]>([]);
  let isLoading = $state(true);
  let isRefreshing = $state(false);
  let error = $state<string | null>(null);
  let feedback = $state<{ type: "success" | "error"; message: string } | null>(
    null,
  );

  // Search & Funnel
  let searchQuery = $state("");
  let statusFilter = $state<"all" | "running" | "stopped" | "partial" | "down">(
    "all",
  );
  let sortBy = $state<
    | "updated-desc"
    | "updated-asc"
    | "name-asc"
    | "name-desc"
    | "services-desc"
    | "services-asc"
    | "status"
    | "updates"
  >("updated-desc");

  // Expanded Stack accordion rows
  let expandedStacks = $state<Record<string, boolean>>({});
  let loadingDetails = $state<Record<string, boolean>>({});

  // Action busy states
  let actionBusy = $state<Record<string, string>>({});

  // Down & Delete Confirmation Modals
  let downModalStack = $state<Stack | null>(null);
  let downRemoveVolumes = $state(false);
  let isDownSubmitting = $state(false);

  let deleteModalStack = $state<Stack | null>(null);
  let isDeleteSubmitting = $state(false);

  // Adopt External Stack Modal
  let adoptModalStack = $state<Stack | null>(null);
  let isAdoptModalOpen = $state(false);

  function openAdoptModal(stack: Stack) {
    adoptModalStack = stack;
    isAdoptModalOpen = true;
  }

  async function handleAdopted(adopted: Stack) {
    showFeedback(
      "success",
      `Stack "${adopted.name}" is now fully managed by PDSwarm!`,
    );
    await loadData(true);
  }

  // Auto-refresh timer
  let refreshTimer: ReturnType<typeof setInterval> | null = null;
  let copiedYamlStack = $state<string | null>(null);

  // Helper to parse service names from Compose YAML
  function parseServiceNames(yaml: string): string[] {
    if (!yaml) return [];
    const names: string[] = [];
    const lines = yaml.split("\n");
    let inServices = false;

    for (let i = 0; i < lines.length; i++) {
      const line = lines[i];
      if (/^services:\s*$/.test(line)) {
        inServices = true;
        continue;
      }
      if (
        inServices &&
        /^[a-zA-Z0-9_\-]+:\s*$/.test(line) &&
        !line.startsWith(" ")
      ) {
        inServices = false;
        continue;
      }
      if (inServices) {
        const match = line.match(/^  ([a-zA-Z0-9_\-\.]+):\s*$/);
        if (match) {
          names.push(match[1]);
        }
      }
    }
    return names;
  }

  // Load stacks and containers
  async function loadData(silent = false) {
    if (!silent) isRefreshing = true;
    error = null;

    try {
      const [stacksData, containersData] = await Promise.all([
        stackApi.list(),
        containerApi.list(true).catch(() => [] as Container[]),
      ]);
      stacks = stacksData || [];
      containers = containersData || [];
    } catch (err: unknown) {
      if (err instanceof Error) {
        error = err.message;
      } else {
        error = "Failed to load Compose stacks from daemon.";
      }
    } finally {
      isLoading = false;
      isRefreshing = false;
    }
  }

  onMount(() => {
    loadData();
    refreshTimer = setInterval(() => {
      loadData(true);
    }, 10000);
  });

  onDestroy(() => {
    if (refreshTimer) clearInterval(refreshTimer);
  });

  function showFeedback(type: "success" | "error", message: string) {
    feedback = { type, message };
    setTimeout(() => {
      if (feedback?.message === message) {
        feedback = null;
      }
    }, 4500);
  }

  // Correlate stacks with containers & services
  interface StackWithDetails extends Stack {
    serviceNames: string[];
    matchingContainers: Container[];
    runningCount: number;
    effectiveStatus: "running" | "stopped" | "partial" | "deploying" | "down";
  }

  let enrichedStacks = $derived.by<StackWithDetails[]>(() => {
    const knownNames = new Set(stacks.map((s) => s.name));
    const allStacks = [...stacks];

    // Discover any compose projects from containers list that aren't in stacks yet
    const externalProjects = new Map<string, Container[]>();
    for (const c of containers) {
      const proj = c.Labels?.["com.docker.compose.project"];
      if (proj && !knownNames.has(proj)) {
        const list = externalProjects.get(proj) || [];
        list.push(c);
        externalProjects.set(proj, list);
      }
    }

    for (const [proj, conts] of externalProjects.entries()) {
      allStacks.push({
        name: proj,
        compose_yaml: "",
        status: "running",
        is_external: true,
        config_file:
          conts[0]?.Labels?.["com.docker.compose.project.config_files"],
        working_dir:
          conts[0]?.Labels?.["com.docker.compose.project.working_dir"],
        created_at: new Date((conts[0].Created || 0) * 1000).toISOString(),
        updated_at: new Date((conts[0].Created || 0) * 1000).toISOString(),
      });
    }

    return allStacks.map((st) => {
      let serviceNames = parseServiceNames(st.compose_yaml);
      const matching = containers.filter(
        (c) => c.Labels && c.Labels["com.docker.compose.project"] === st.name,
      );
      if (serviceNames.length === 0 && matching.length > 0) {
        const svcs = new Set<string>();
        for (const c of matching) {
          if (c.Labels && c.Labels["com.docker.compose.service"]) {
            svcs.add(c.Labels["com.docker.compose.service"]);
          }
        }
        serviceNames = Array.from(svcs);
      }

      const runningCount = matching.filter(
        (c) =>
          typeof c.State === "string" && c.State.toLowerCase() === "running",
      ).length;

      let effectiveStatus:
        | "running"
        | "stopped"
        | "partial"
        | "deploying"
        | "down" = "stopped";

      if (st.status === "deploying") {
        effectiveStatus = "deploying";
      } else if (matching.length > 0) {
        if (runningCount === matching.length && runningCount > 0) {
          effectiveStatus = "running";
        } else if (runningCount > 0) {
          effectiveStatus = "partial";
        } else {
          effectiveStatus = "stopped";
        }
      } else if (st.status === "running") {
        effectiveStatus = "running";
      } else if (st.status === "down") {
        effectiveStatus = "down";
      } else {
        effectiveStatus = "stopped";
      }

      return {
        ...st,
        serviceNames,
        matchingContainers: matching,
        runningCount,
        effectiveStatus,
      };
    });
  });

  // Filtered & sorted stacks
  let filteredStacks = $derived.by(() => {
    const query = searchQuery.trim().toLowerCase();
    let result = enrichedStacks.filter((st) => {
      // Status filter
      if (statusFilter !== "all" && st.effectiveStatus !== statusFilter) {
        return false;
      }
      // Search query
      if (query) {
        const matchesName = st.name.toLowerCase().includes(query);
        const matchesServices = st.serviceNames.some((s) =>
          s.toLowerCase().includes(query),
        );
        return matchesName || matchesServices;
      }
      return true;
    });

    result.sort((a, b) => {
      if (sortBy === "updates") {
        const aHasUpdate = a.matchingContainers.some((c) =>
          imageUpdateStore.hasUpdate(c.Image),
        )
          ? 1
          : 0;
        const bHasUpdate = b.matchingContainers.some((c) =>
          imageUpdateStore.hasUpdate(c.Image),
        )
          ? 1
          : 0;
        if (aHasUpdate !== bHasUpdate) return bHasUpdate - aHasUpdate;
        const dateA = new Date(a.updated_at || a.created_at || 0).getTime();
        const dateB = new Date(b.updated_at || b.created_at || 0).getTime();
        return dateB - dateA;
      }
      if (sortBy === "name-asc") {
        return a.name.localeCompare(b.name);
      }
      if (sortBy === "name-desc") {
        return b.name.localeCompare(a.name);
      }
      if (sortBy === "services-desc") {
        if (b.serviceNames.length !== a.serviceNames.length) {
          return b.serviceNames.length - a.serviceNames.length;
        }
        return a.name.localeCompare(b.name);
      }
      if (sortBy === "services-asc") {
        if (a.serviceNames.length !== b.serviceNames.length) {
          return a.serviceNames.length - b.serviceNames.length;
        }
        return a.name.localeCompare(b.name);
      }
      if (sortBy === "status") {
        const score = (status: string) => {
          if (status === "running") return 4;
          if (status === "partial") return 3;
          if (status === "stopped") return 2;
          if (status === "down") return 1;
          return 0;
        };
        const diff = score(b.effectiveStatus) - score(a.effectiveStatus);
        if (diff !== 0) return diff;
        return a.name.localeCompare(b.name);
      }
      if (sortBy === "updated-asc") {
        const dateA = new Date(a.updated_at || a.created_at || 0).getTime();
        const dateB = new Date(b.updated_at || b.created_at || 0).getTime();
        return dateA - dateB;
      }
      // updated-desc (default)
      const dateA = new Date(a.updated_at || a.created_at || 0).getTime();
      const dateB = new Date(b.updated_at || b.created_at || 0).getTime();
      return dateB - dateA;
    });

    return result;
  });

  // Summary Metrics
  let totalStacksCount = $derived(stacks.length);
  let runningStacksCount = $derived(
    enrichedStacks.filter((s) => s.effectiveStatus === "running").length,
  );
  let partialStacksCount = $derived(
    enrichedStacks.filter((s) => s.effectiveStatus === "partial").length,
  );
  let stoppedStacksCount = $derived(
    enrichedStacks.filter(
      (s) => s.effectiveStatus === "stopped" || s.effectiveStatus === "down",
    ).length,
  );
  let totalManagedContainers = $derived(
    enrichedStacks.reduce((acc, s) => acc + s.matchingContainers.length, 0),
  );

  // Timestamp formatter
  function formatTimestamp(isoStr?: string): string {
    if (!isoStr) return "Unknown";
    try {
      const date = new Date(isoStr);
      if (isNaN(date.getTime())) return isoStr;
      const now = new Date();
      const diffMs = now.getTime() - date.getTime();
      const diffMins = Math.floor(diffMs / (1000 * 60));
      const diffHours = Math.floor(diffMins / 60);
      const diffDays = Math.floor(diffHours / 24);

      if (diffMins < 1) return "Just now";
      if (diffMins < 60) return `${diffMins}m ago`;
      if (diffHours < 24) return `${diffHours}h ago`;
      if (diffDays < 7) return `${diffDays}d ago`;
      return date.toLocaleDateString();
    } catch {
      return isoStr;
    }
  }

  // Toggle accordion expand
  async function toggleExpand(name: string) {
    const isNowExpanded = !expandedStacks[name];
    expandedStacks[name] = isNowExpanded;

    if (isNowExpanded) {
      const targetStack = stacks.find((s) => s.name === name);
      if (targetStack && !targetStack.compose_yaml && !loadingDetails[name]) {
        loadingDetails[name] = true;
        try {
          const full = await stackApi.get(name);
          if (full && full.compose_yaml) {
            stacks = stacks.map((s) =>
              s.name === name
                ? {
                    ...s,
                    compose_yaml: full.compose_yaml,
                    env: full.env || s.env,
                    config_file: full.config_file || s.config_file,
                    working_dir: full.working_dir || s.working_dir,
                  }
                : s,
            );
          }
        } catch {
          // Keep fast representation on error
        } finally {
          loadingDetails[name] = false;
        }
      }
    }
  }

  let isCheckingUpdates = $state(false);

  async function handleCheckStackUpdates() {
    isCheckingUpdates = true;
    try {
      const allImages = enrichedStacks
        .flatMap((s) => s.matchingContainers.map((c) => c.Image))
        .filter(Boolean);
      await imageUpdateStore.checkMultiple(allImages);
      const stacksWithUpdates = enrichedStacks.filter((s) =>
        imageUpdateStore.hasAnyUpdate(s.matchingContainers.map((c) => c.Image)),
      ).length;
      if (stacksWithUpdates > 0) {
        showFeedback(
          "success",
          `Found image updates for ${stacksWithUpdates} stack(s)! Click "Redeploy" to pull & launch latest images.`,
        );
      } else {
        showFeedback("success", "All stack container images are up to date!");
      }
    } catch {
      showFeedback("error", "Failed to check remote registry updates.");
    } finally {
      isCheckingUpdates = false;
    }
  }

  // Modal state for redeploy confirmation & relative volumes resolution
  let redeployModalStack = $state<Stack | null>(null);
  let redeployRelativePaths = $state<string[]>([]);
  let redeployComposeDir = $state<string>("");
  let redeployAutoConvert = $state<boolean>(true);
  let redeployPullImages = $state<boolean>(true);
  let isRedeploySubmitting = $state<boolean>(false);

  // Action: Deploy / Redeploy
  async function handleRedeploy(stack: Stack) {
    if (!stack.compose_yaml || stack.env === undefined) {
      try {
        actionBusy[stack.name] = "deploying";
        const detailed = await stackApi.get(stack.name);
        if (detailed) {
          if (detailed.compose_yaml) stack.compose_yaml = detailed.compose_yaml;
          if (detailed.env !== undefined) stack.env = detailed.env;
          if (detailed.working_dir) stack.working_dir = detailed.working_dir;
          if (detailed.config_file) stack.config_file = detailed.config_file;
        }
      } catch {
        // proceed with whatever is present
      } finally {
        delete actionBusy[stack.name];
      }
    }
    const relPaths = detectRelativeVolumes(stack.compose_yaml || "");
    const composeDir = getStackComposeDir(stack);
    redeployModalStack = stack;
    redeployRelativePaths = relPaths;
    redeployComposeDir = composeDir || "";
    redeployAutoConvert = true;
    redeployPullImages = true;
  }

  async function executeRedeploy(
    stack: Stack,
    yamlToDeploy: string,
    newWorkingDir?: string,
    pullImages = true,
  ) {
    actionBusy[stack.name] = "deploying";
    try {
      await stackApi.update(stack.name, yamlToDeploy, true, stack.env || undefined, pullImages);
      showFeedback("success", `Stack "${stack.name}" redeployment started in background.`);
      tasksStore.poll();
      await loadData(true);
    } catch (err: unknown) {
      showFeedback(
        "error",
        err instanceof Error ? err.message : "Failed to redeploy stack.",
      );
    } finally {
      delete actionBusy[stack.name];
    }
  }

  async function confirmRedeploy() {
    if (!redeployModalStack) return;
    const stack = redeployModalStack;
    isRedeploySubmitting = true;

    try {
      let yaml = stack.compose_yaml || "";
      const dir = redeployComposeDir.trim();
      if (redeployAutoConvert && dir && redeployRelativePaths.length > 0) {
        yaml = resolveRelativeVolumesInYaml(yaml, dir);
      }
      redeployModalStack = null;
      await executeRedeploy(stack, yaml, dir || undefined, redeployPullImages);
    } catch (err: unknown) {
      showFeedback(
        "error",
        err instanceof Error ? err.message : "Failed to redeploy stack.",
      );
    } finally {
      isRedeploySubmitting = false;
    }
  }

  async function confirmRedeployAsIs() {
    if (!redeployModalStack) return;
    const stack = redeployModalStack;
    redeployModalStack = null;
    await executeRedeploy(stack, stack.compose_yaml || "", undefined, redeployPullImages);
  }

  // Action: Stop
  async function handleStop(stack: Stack) {
    actionBusy[stack.name] = "stopping";
    try {
      await stackApi.stop(stack.name);
      showFeedback("success", `Stack "${stack.name}" stopped.`);
      await loadData(true);
    } catch (err: unknown) {
      showFeedback(
        "error",
        err instanceof Error ? err.message : "Failed to stop stack.",
      );
    } finally {
      delete actionBusy[stack.name];
    }
  }

  // Action: Down
  function promptDown(stack: Stack) {
    downModalStack = stack;
    downRemoveVolumes = false;
  }

  async function confirmDown() {
    if (!downModalStack) return;
    const name = downModalStack.name;
    isDownSubmitting = true;
    actionBusy[name] = "down";

    try {
      await stackApi.down(name, downRemoveVolumes);
      showFeedback("success", `Stack "${name}" brought down.`);
      downModalStack = null;
      await loadData(true);
    } catch (err: unknown) {
      showFeedback(
        "error",
        err instanceof Error ? err.message : "Failed to bring down stack.",
      );
    } finally {
      isDownSubmitting = false;
      delete actionBusy[name];
    }
  }

  // Action: Delete
  function promptDelete(stack: Stack) {
    deleteModalStack = stack;
  }

  async function confirmDelete() {
    if (!deleteModalStack) return;
    const name = deleteModalStack.name;
    isDeleteSubmitting = true;
    actionBusy[name] = "deleting";

    try {
      await stackApi.delete(name);
      showFeedback("success", `Stack "${name}" and its resources deleted.`);
      deleteModalStack = null;
      await loadData(true);
    } catch (err: unknown) {
      showFeedback(
        "error",
        err instanceof Error ? err.message : "Failed to delete stack.",
      );
    } finally {
      isDeleteSubmitting = false;
      delete actionBusy[name];
    }
  }

  // Copy Compose YAML
  function copyYaml(stack: Stack) {
    navigator.clipboard.writeText(stack.compose_yaml);
    copiedYamlStack = stack.name;
    setTimeout(() => {
      if (copiedYamlStack === stack.name) {
        copiedYamlStack = null;
      }
    }, 2000);
  }
</script>

<div class="space-y-6 animate-in fade-in duration-200">
  <!-- Header Bar -->
  <div class="flex flex-wrap items-center justify-between gap-4">
    <div class="flex items-center gap-3">
      <div
        class="w-10 h-10 rounded-2xl bg-gradient-to-br from-violet-600 to-indigo-700 flex items-center justify-center text-white shadow-[0_0_20px_rgba(124,58,237,0.4)] border border-violet-400/30"
      >
        <Layers class="w-5 h-5 text-cyan-300" />
      </div>
      <div>
        <h1
          class="text-xl font-bold text-white tracking-tight flex items-center gap-2"
        >
          <span>Compose Stacks</span>
          <span
            class="px-2 py-0.5 text-[10px] font-mono rounded-full bg-cyan-500/10 text-cyan-300 border border-cyan-500/25"
          >
            Docker Compose
          </span>
        </h1>
        <p class="text-xs text-slate-400">
          Orchestrate, monitor, and configure multi-container application stacks
        </p>
      </div>
    </div>

    <!-- Action Buttons -->
    <div class="flex items-center gap-2.5">
      <button
        onclick={() => loadData(false)}
        disabled={isRefreshing}
        class="p-2.5 rounded-xl bg-[#12131c] hover:bg-[#181a26] text-slate-300 hover:text-white border border-white/[0.08] transition shadow-sm disabled:opacity-50"
        title="Refresh stacks"
      >
        <RefreshCw
          class="w-4 h-4 {isRefreshing ? 'animate-spin text-cyan-400' : ''}"
        />
      </button>

      <!-- Check Stack Image Updates button -->
      <button
        onclick={handleCheckStackUpdates}
        disabled={isCheckingUpdates || isRefreshing}
        class="px-3.5 py-2.5 rounded-xl bg-cyan-500/10 hover:bg-cyan-500/20 text-cyan-300 hover:text-white border border-cyan-500/30 text-xs font-semibold transition flex items-center gap-1.5 shadow-[0_0_12px_rgba(6,182,212,0.15)] disabled:opacity-50 cursor-pointer"
        title="Check all stack service images against remote registries for updates"
      >
        <Sparkles
          class="w-4 h-4 text-cyan-400 {isCheckingUpdates
            ? 'animate-spin'
            : ''}"
        />
        <span class="hidden sm:inline">Check Updates</span>
      </button>

      {#if onNewStack}
        <button
          onclick={onNewStack}
          class="flex items-center gap-2 px-4 py-2.5 rounded-xl bg-gradient-to-r from-violet-600 via-indigo-600 to-violet-700 hover:from-violet-500 hover:to-indigo-500 text-white text-xs font-semibold shadow-[0_0_20px_rgba(124,58,237,0.4)] border border-violet-400/30 transition transform hover:-translate-y-0.5"
        >
          <Plus class="w-4 h-4" />
          <span>New Stack</span>
        </button>
      {/if}
    </div>
  </div>

  <!-- Metric / Overview Cards -->
  <div class="grid grid-cols-2 md:grid-cols-4 gap-4">
    <!-- Total Stacks -->
    <div
      class="p-4 rounded-2xl bg-[#12131c] border border-white/[0.08] relative overflow-hidden shadow-md"
    >
      <div class="flex items-center justify-between text-slate-400 text-xs">
        <span>Total Stacks</span>
        <Layers class="w-4 h-4 text-violet-400" />
      </div>
      <div class="mt-2 text-2xl font-bold text-white font-mono">
        {totalStacksCount}
      </div>
      <div class="mt-1 text-[11px] text-slate-500">
        Configured Compose environments
      </div>
      <div
        class="absolute -right-4 -bottom-4 w-16 h-16 rounded-full bg-violet-600/10 blur-xl pointer-events-none"
      ></div>
    </div>

    <!-- Running Stacks -->
    <div
      class="p-4 rounded-2xl bg-[#12131c] border border-white/[0.08] relative overflow-hidden shadow-md"
    >
      <div class="flex items-center justify-between text-slate-400 text-xs">
        <span>Running Stacks</span>
        <span
          class="w-2 h-2 rounded-full bg-emerald-400 shadow-[0_0_8px_#10b981]"
        ></span>
      </div>
      <div class="mt-2 text-2xl font-bold text-emerald-400 font-mono">
        {runningStacksCount}
      </div>
      <div class="mt-1 text-[11px] text-slate-500">
        All services healthy & online
      </div>
      <div
        class="absolute -right-4 -bottom-4 w-16 h-16 rounded-full bg-emerald-600/10 blur-xl pointer-events-none"
      ></div>
    </div>

    <!-- Partial or Stopped -->
    <div
      class="p-4 rounded-2xl bg-[#12131c] border border-white/[0.08] relative overflow-hidden shadow-md"
    >
      <div class="flex items-center justify-between text-slate-400 text-xs">
        <span>Stopped / Partial</span>
        <span
          class="w-2 h-2 rounded-full {partialStacksCount > 0
            ? 'bg-amber-400 shadow-[0_0_8px_#f59e0b]'
            : 'bg-rose-400'}"
        ></span>
      </div>
      <div
        class="mt-2 text-2xl font-bold font-mono {partialStacksCount > 0
          ? 'text-amber-400'
          : 'text-slate-300'}"
      >
        {stoppedStacksCount + partialStacksCount}
      </div>
      <div class="mt-1 text-[11px] text-slate-500">
        {#if partialStacksCount > 0}
          {partialStacksCount} partially degraded
        {:else}
          Inactive or down
        {/if}
      </div>
      <div
        class="absolute -right-4 -bottom-4 w-16 h-16 rounded-full bg-amber-600/10 blur-xl pointer-events-none"
      ></div>
    </div>

    <!-- Managed Containers -->
    <div
      class="p-4 rounded-2xl bg-[#12131c] border border-white/[0.08] relative overflow-hidden shadow-md"
    >
      <div class="flex items-center justify-between text-slate-400 text-xs">
        <span>Stack Containers</span>
        <Box class="w-4 h-4 text-cyan-400" />
      </div>
      <div class="mt-2 text-2xl font-bold text-cyan-300 font-mono">
        {totalManagedContainers}
      </div>
      <div class="mt-1 text-[11px] text-slate-500">
        Containers labeled by compose
      </div>
      <div
        class="absolute -right-4 -bottom-4 w-16 h-16 rounded-full bg-cyan-600/10 blur-xl pointer-events-none"
      ></div>
    </div>
  </div>

  <!-- Feedback Notice Banner -->
  {#if feedback}
    <div
      class="p-3.5 rounded-xl border flex items-center justify-between text-xs animate-in fade-in duration-150 {feedback.type ===
      'success'
        ? 'bg-emerald-500/15 border-emerald-500/30 text-emerald-300'
        : 'bg-rose-500/15 border-rose-500/30 text-rose-300'}"
    >
      <div class="flex items-center gap-2.5">
        {#if feedback.type === "success"}
          <CircleCheck class="w-4 h-4 text-emerald-400 shrink-0" />
        {:else}
          <CircleAlert class="w-4 h-4 text-rose-400 shrink-0" />
        {/if}
        <span>{feedback.message}</span>
      </div>
      <button
        onclick={() => (feedback = null)}
        class="hover:text-white font-bold">&times;</button
      >
    </div>
  {/if}

  {#if error}
    <div
      class="p-4 rounded-2xl bg-rose-500/15 border border-rose-500/30 text-rose-300 flex items-start gap-3 text-xs"
    >
      <CircleAlert class="w-4 h-4 text-rose-400 shrink-0 mt-0.5" />
      <div>
        <p class="font-bold">Failed to communicate with Docker API</p>
        <p class="text-rose-400/90 mt-0.5">{error}</p>
      </div>
    </div>
  {/if}

  <!-- Search & Funnel Controls -->
  <div
    class="flex flex-wrap items-center justify-between gap-3 p-3.5 rounded-2xl bg-[#12131c] border border-white/[0.08]"
  >
    <!-- Search Bar -->
    <div class="relative flex-1 min-w-[220px]">
      <Search
        class="w-4 h-4 text-slate-400 absolute left-3.5 top-1/2 -translate-y-1/2 pointer-events-none"
      />
      <input
        type="text"
        bind:value={searchQuery}
        placeholder="Funnel stacks by name or service..."
        class="w-full pl-9 pr-4 py-2 rounded-xl bg-[#090a10] border border-white/[0.08] text-white text-xs placeholder:text-slate-500 focus:outline-none focus:border-violet-500 transition"
      />
    </div>

    <!-- Funnel Pills -->
    <div class="flex items-center gap-1.5 overflow-x-auto py-1">
      <button
        onclick={() => (statusFilter = "all")}
        class="px-3 py-1.5 rounded-lg text-xs font-medium transition {statusFilter ===
        'all'
          ? 'bg-violet-600/30 text-violet-300 border border-violet-500/40 shadow-[0_0_12px_rgba(124,58,237,0.25)]'
          : 'text-slate-400 hover:text-white hover:bg-white/[0.04]'}"
      >
        All ({enrichedStacks.length})
      </button>

      <button
        onclick={() => (statusFilter = "running")}
        class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium transition {statusFilter ===
        'running'
          ? 'bg-emerald-500/20 text-emerald-300 border border-emerald-500/30'
          : 'text-slate-400 hover:text-white hover:bg-white/[0.04]'}"
      >
        <span class="w-1.5 h-1.5 rounded-full bg-emerald-400"></span>
        <span>Running</span>
      </button>

      <button
        onclick={() => (statusFilter = "partial")}
        class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium transition {statusFilter ===
        'partial'
          ? 'bg-amber-500/20 text-amber-300 border border-amber-500/30'
          : 'text-slate-400 hover:text-white hover:bg-white/[0.04]'}"
      >
        <span class="w-1.5 h-1.5 rounded-full bg-amber-400"></span>
        <span>Partial</span>
      </button>

      <button
        onclick={() => (statusFilter = "stopped")}
        class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium transition {statusFilter ===
        'stopped'
          ? 'bg-rose-500/20 text-rose-300 border border-rose-500/30'
          : 'text-slate-400 hover:text-white hover:bg-white/[0.04]'}"
      >
        <span class="w-1.5 h-1.5 rounded-full bg-rose-400"></span>
        <span>Stopped</span>
      </button>

      <!-- Sort Dropdown -->
      <select
        bind:value={sortBy}
        class="px-3 py-1.5 rounded-lg bg-[#090a10] border border-white/[0.08] text-slate-300 text-xs focus:outline-none focus:border-violet-500 font-mono cursor-pointer shrink-0 ml-1"
        title="Sort compose stacks"
      >
        <option value="updated-desc">Recently Updated</option>
        <option value="updated-asc">Oldest Updated</option>
        <option value="name-asc">Name (A-Z)</option>
        <option value="name-desc">Name (Z-A)</option>
        <option value="services-desc">Most Services</option>
        <option value="services-asc">Least Services</option>
        <option value="status">Status (Running First)</option>
        <option value="updates">Updates Available First</option>
      </select>
    </div>
  </div>

  <!-- Stacks List / Cards -->
  {#if isLoading}
    <div
      class="p-16 rounded-2xl bg-[#12131c] border border-white/[0.08] text-center space-y-3"
    >
      <LoaderCircle class="w-8 h-8 mx-auto text-violet-400 animate-spin" />
      <p class="text-xs font-mono text-slate-400">
        Discovering Compose stacks...
      </p>
    </div>
  {:else if filteredStacks.length === 0}
    <!-- Empty State -->
    <div
      class="p-12 rounded-2xl bg-[#12131c] border border-white/[0.08] text-center space-y-4 animate-in fade-in"
    >
      <div
        class="w-14 h-14 mx-auto rounded-2xl bg-violet-600/20 border border-violet-500/30 flex items-center justify-center text-violet-400 shadow-[0_0_25px_rgba(124,58,237,0.3)]"
      >
        <Layers class="w-7 h-7 text-cyan-300" />
      </div>
      <div>
        <h2 class="text-lg font-bold text-white">No Compose Stacks Found</h2>
        <p class="text-xs text-slate-400 max-w-sm mx-auto mt-1">
          {#if searchQuery || statusFilter !== "all"}
            No stacks match your current search query or filter criteria.
          {:else}
            You haven't deployed any multi-container Docker Compose stacks yet.
          {/if}
        </p>
      </div>

      {#if searchQuery || statusFilter !== "all"}
        <button
          onclick={() => {
            searchQuery = "";
            statusFilter = "all";
          }}
          class="px-4 py-2 rounded-xl bg-white/[0.06] hover:bg-white/[0.1] text-xs font-semibold text-slate-300 transition"
        >
          Reset Filters
        </button>
      {:else}
        <div class="flex items-center justify-center gap-3 flex-wrap">
          {#if onNewStack}
            <button
              onclick={onNewStack}
              class="inline-flex items-center gap-2 px-5 py-2.5 rounded-xl bg-gradient-to-r from-violet-600 to-indigo-600 hover:from-violet-500 hover:to-indigo-500 text-white text-xs font-semibold shadow-[0_0_20px_rgba(124,58,237,0.4)] border border-violet-400/30 transition cursor-pointer"
            >
              <Plus class="w-4 h-4" />
              <span>Create First Stack</span>
            </button>
          {/if}
          {#if onNavigate}
            <button
              onclick={() => onNavigate("templates")}
              class="inline-flex items-center gap-2 px-4 py-2.5 rounded-xl bg-[#181a26] hover:bg-[#202334] text-violet-300 hover:text-violet-200 text-xs font-semibold border border-violet-500/30 transition cursor-pointer"
            >
              <Boxes class="w-4 h-4 text-violet-400" />
              <span>Browse 1-Click Templates</span>
            </button>
          {/if}
        </div>
      {/if}
    </div>
  {:else}
    <div class="space-y-3.5">
      {#each filteredStacks as stack (stack.name)}
        {@const isExpanded = !!expandedStacks[stack.name]}
        {@const isBusy = !!actionBusy[stack.name]}
        {@const currentBusyAction = actionBusy[stack.name]}
        {@const stackImages = stack.matchingContainers
          .map((c) => c.Image)
          .filter(Boolean)}

        <ViewportGate minHeight="78px" rootMargin="350px 0px" keepMounted={isExpanded}>
          <div
            class="rounded-2xl bg-[#12131c] border border-white/[0.08] shadow-lg transition-all duration-150 overflow-hidden hover:border-violet-500/30"
          >
          <!-- Main Stack Row Header -->
          <div
            class="p-4 sm:p-5 flex flex-wrap items-center justify-between gap-4"
          >
            <!-- Left: Toggle & Stack Identity -->
            <div class="flex items-center gap-3.5 min-w-[260px] flex-1">
              <!-- Expand / Collapse chevron button -->
              <button
                onclick={() => toggleExpand(stack.name)}
                class="p-1 rounded-lg text-slate-400 hover:text-white hover:bg-white/[0.06] transition"
                title={isExpanded
                  ? "Collapse stack details"
                  : "Expand stack details"}
              >
                {#if isExpanded}
                  <ChevronDown class="w-5 h-5 text-cyan-400" />
                {:else}
                  <ChevronRight class="w-5 h-5" />
                {/if}
              </button>

              <!-- Stack status badge dot & name -->
              <div class="space-y-1">
                <div class="flex items-center gap-2.5">
                  <span
                    class="text-sm font-bold text-white font-mono tracking-tight"
                    >{stack.name}</span
                  >
                  {#if stack.is_external}
                    <span
                      class="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[10px] font-semibold bg-amber-500/15 text-amber-400 border border-amber-500/30 shadow-[0_0_10px_rgba(245,158,11,0.15)]"
                      title="External stack launched outside PDSwarm"
                    >
                      <ShieldAlert class="w-3 h-3 text-amber-400" />
                      <span>External (Unmanaged)</span>
                    </span>
                  {:else}
                    <span
                      class="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[10px] font-semibold bg-emerald-500/15 text-emerald-400 border border-emerald-500/30 shadow-[0_0_10px_rgba(16,185,129,0.15)]"
                      title="Native PDSwarm managed stack"
                    >
                      <ShieldCheck class="w-3 h-3 text-emerald-400" />
                      <span>Managed by PDSwarm</span>
                    </span>
                  {/if}

                  <!-- Status Badge -->
                  {#if stack.effectiveStatus === "running"}
                    <span
                      class="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[10px] font-semibold bg-emerald-500/15 text-emerald-400 border border-emerald-500/30 shadow-[0_0_10px_rgba(16,185,129,0.2)]"
                    >
                      <span
                        class="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-pulse"
                      ></span>
                      <span>running</span>
                    </span>
                  {:else if stack.effectiveStatus === "partial"}
                    <span
                      class="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[10px] font-semibold bg-amber-500/15 text-amber-400 border border-amber-500/30"
                    >
                      <span class="w-1.5 h-1.5 rounded-full bg-amber-400"
                      ></span>
                      <span>partial</span>
                    </span>
                  {:else if stack.effectiveStatus === "deploying"}
                    <span
                      class="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[10px] font-semibold bg-violet-500/15 text-violet-400 border border-violet-500/30 animate-pulse"
                    >
                      <LoaderCircle class="w-2.5 h-2.5 animate-spin" />
                      <span>deploying</span>
                    </span>
                  {:else if stack.effectiveStatus === "down"}
                    <span
                      class="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[10px] font-semibold bg-slate-500/15 text-slate-400 border border-slate-500/30"
                    >
                      <span class="w-1.5 h-1.5 rounded-full bg-slate-400"
                      ></span>
                      <span>down</span>
                    </span>
                  {:else}
                    <span
                      class="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[10px] font-semibold bg-rose-500/15 text-rose-400 border border-rose-500/30"
                    >
                      <span class="w-1.5 h-1.5 rounded-full bg-rose-400"></span>
                      <span>stopped</span>
                    </span>
                  {/if}

                  <!-- Stack Image Update Status Badge -->
                  {#if stackImages.length > 0 && imageUpdateStore.hasAnyUpdate(stackImages)}
                    <span
                      class="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-[10px] font-semibold bg-amber-500/20 text-amber-300 border border-amber-500/40 shadow-[0_0_10px_rgba(245,158,11,0.25)] animate-pulse"
                      title="One or more service images have updates available on remote registries"
                    >
                      <CircleArrowUp class="w-3 h-3 text-amber-400" />
                      <span>Update Available</span>
                    </span>
                  {:else if stackImages.length > 0 && stackImages.every( (img) => imageUpdateStore.isUpToDate(img), )}
                    <span
                      class="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-[10px] font-semibold bg-emerald-500/15 text-emerald-300 border border-emerald-500/30"
                      title="All service images match remote registries"
                    >
                      <CircleCheck class="w-3 h-3 text-emerald-400" />
                      <span>Up to date</span>
                    </span>
                  {/if}
                </div>

                <!-- Services preview pills & container count -->
                <div
                  class="flex flex-wrap items-center gap-2 text-xs text-slate-400"
                >
                  <span class="font-mono text-[11px] text-cyan-300">
                    {stack.serviceNames.length}
                    {stack.serviceNames.length === 1 ? "service" : "services"}
                  </span>
                  <span>&bull;</span>
                  <span
                    class="font-mono text-[11px] {stack.runningCount > 0
                      ? 'text-emerald-400'
                      : 'text-slate-400'}"
                  >
                    {stack.runningCount} / {Math.max(
                      stack.serviceNames.length,
                      stack.matchingContainers.length,
                    )} containers running
                  </span>
                  <span>&bull;</span>
                  <span
                    class="text-[11px] text-slate-500 flex items-center gap-1"
                  >
                    <Clock class="w-3 h-3" />
                    <span
                      >{formatTimestamp(
                        stack.updated_at || stack.created_at,
                      )}</span
                    >
                  </span>
                </div>
              </div>
            </div>

            <!-- Services tag cloud summary -->
            <div class="hidden lg:flex items-center gap-1.5 max-w-sm flex-wrap">
              {#each stack.serviceNames.slice(0, 4) as svcName}
                <span
                  class="px-2 py-0.5 rounded-md text-[10px] font-mono bg-[#090a10] border border-white/[0.08] text-slate-300"
                >
                  {svcName}
                </span>
              {/each}
              {#if stack.serviceNames.length > 4}
                <span class="text-[10px] text-slate-500 font-mono"
                  >+{stack.serviceNames.length - 4} more</span
                >
              {/if}
            </div>

            <!-- Right Action Buttons -->
            <div class="flex items-center gap-1.5">
              {#if stack.is_external}
                <!-- Take Full Control (Adopt) Button -->
                <button
                  onclick={() => openAdoptModal(stack)}
                  class="flex items-center gap-1.5 px-3 py-1.5 rounded-xl bg-gradient-to-r from-violet-600/30 to-cyan-600/30 hover:from-violet-600/50 hover:to-cyan-600/50 text-cyan-300 border border-cyan-500/40 text-xs font-semibold shadow-[0_0_12px_rgba(6,182,212,0.2)] transition cursor-pointer"
                  title="Take full control of this external stack with zero downtime"
                >
                  <ShieldCheck class="w-3.5 h-3.5 text-cyan-400" />
                  <span class="hidden sm:inline">Take Full Control</span>
                </button>
              {/if}

              <!-- Deploy / Redeploy Button -->
              <button
                onclick={() => handleRedeploy(stack)}
                disabled={isBusy}
                class="flex items-center gap-1 px-3 py-1.5 rounded-xl bg-violet-600/20 hover:bg-violet-600/30 text-violet-300 border border-violet-500/30 text-xs font-semibold transition disabled:opacity-50"
                title={stack.effectiveStatus === "running"
                  ? "Redeploy stack containers"
                  : "Deploy stack"}
              >
                {#if currentBusyAction === "deploying"}
                  <LoaderCircle class="w-3.5 h-3.5 animate-spin" />
                  <span class="hidden sm:inline">Deploying...</span>
                {:else if stack.effectiveStatus === "running"}
                  <RotateCw class="w-3.5 h-3.5 text-cyan-400" />
                  <span class="hidden sm:inline">Redeploy</span>
                {:else}
                  <Play class="w-3.5 h-3.5 text-cyan-400 fill-cyan-400" />
                  <span class="hidden sm:inline">Deploy</span>
                {/if}
              </button>

              <!-- Stop Button -->
              {#if stack.effectiveStatus === "running" || stack.effectiveStatus === "partial"}
                <button
                  onclick={() => handleStop(stack)}
                  disabled={isBusy}
                  class="flex items-center gap-1 px-3 py-1.5 rounded-xl bg-white/[0.05] hover:bg-amber-500/20 text-slate-300 hover:text-amber-300 border border-white/[0.08] hover:border-amber-500/30 text-xs font-semibold transition disabled:opacity-50"
                  title="Stop running stack containers"
                >
                  {#if currentBusyAction === "stopping"}
                    <LoaderCircle class="w-3.5 h-3.5 animate-spin text-amber-400" />
                  {:else}
                    <Square class="w-3.5 h-3.5" />
                  {/if}
                  <span class="hidden sm:inline">Stop</span>
                </button>
              {/if}

              <!-- Down Button -->
              <button
                onclick={() => promptDown(stack)}
                disabled={isBusy}
                class="flex items-center gap-1 px-3 py-1.5 rounded-xl bg-white/[0.05] hover:bg-rose-500/20 text-slate-300 hover:text-rose-300 border border-white/[0.08] hover:border-rose-500/30 text-xs font-semibold transition disabled:opacity-50"
                title="Stop and remove containers and networks"
              >
                <ArrowDownToLine class="w-3.5 h-3.5" />
                <span class="hidden sm:inline">Down</span>
              </button>

              <!-- Edit Button -->
              {#if onEditStack}
                <button
                  onclick={() => onEditStack(stack.name)}
                  class="flex items-center gap-1 px-3 py-1.5 rounded-xl bg-white/[0.05] hover:bg-cyan-500/20 text-slate-300 hover:text-cyan-300 border border-white/[0.08] hover:border-cyan-500/30 text-xs font-semibold transition"
                  title="Open in Compose Studio"
                >
                  <PenLine class="w-3.5 h-3.5" />
                  <span class="hidden sm:inline">Edit</span>
                </button>
              {/if}

              <!-- Delete Button -->
              <button
                onclick={() => promptDelete(stack)}
                disabled={isBusy}
                class="p-2 rounded-xl bg-white/[0.04] hover:bg-rose-500/20 text-slate-400 hover:text-rose-400 border border-white/[0.06] hover:border-rose-500/30 transition disabled:opacity-50"
                title="Delete stack configuration"
              >
                <Trash class="w-3.5 h-3.5" />
              </button>
            </div>
          </div>

          <!-- Expanded Details Section -->
          {#if isExpanded}
            <div
              class="px-5 pb-5 pt-3 border-t border-white/[0.06] bg-[#0c0d15] space-y-4 animate-in fade-in"
            >
              <!-- External Stack Ownership Notice & Quick Adopt Banner -->
              {#if stack.is_external}
                <div
                  class="p-4 rounded-xl bg-gradient-to-r from-amber-500/10 via-[#181a26] to-violet-950/20 border border-amber-500/30 flex flex-col sm:flex-row sm:items-center justify-between gap-4"
                >
                  <div class="flex items-start gap-3">
                    <div
                      class="w-8 h-8 rounded-lg bg-amber-500/20 border border-amber-500/30 flex items-center justify-center text-amber-400 shrink-0 mt-0.5"
                    >
                      <ShieldAlert class="w-4 h-4" />
                    </div>
                    <div class="space-y-1">
                      <div class="flex items-center gap-2">
                        <h4 class="text-xs font-bold text-amber-300">
                          External Stack (Not Owned by PDSwarm)
                        </h4>
                        <span
                          class="px-1.5 py-0.2 rounded text-[9px] font-mono font-bold bg-amber-500/20 text-amber-400 border border-amber-500/30"
                          >UNMANAGED</span
                        >
                      </div>
                      <p
                        class="text-xs text-slate-300 max-w-2xl leading-relaxed"
                      >
                        This stack was created outside PDSwarm (e.g. host
                        docker-compose CLI). Unlike Portainer, which locks
                        external stacks in read-only mode, PDSwarm can adopt
                        this stack and grant you full editing, environment
                        control, and redeployment capabilities with zero
                        container downtime.
                      </p>
                      {#if stack.config_file}
                        <p class="text-[11px] font-mono text-slate-400">
                          Host Compose file: <span class="text-cyan-400"
                            >{stack.config_file}</span
                          >
                        </p>
                      {/if}
                      {#if detectRelativeVolumes(stack.compose_yaml).length > 0}
                        <div
                          class="mt-2.5 p-2.5 rounded-lg bg-amber-950/40 border border-amber-500/30 flex items-center gap-2 text-xs text-amber-200"
                        >
                          <TriangleAlert
                            class="w-4 h-4 text-amber-400 shrink-0"
                          />
                          <span>
                            Detected <strong
                              >{detectRelativeVolumes(stack.compose_yaml)
                                .length} relative volume path(s)</strong
                            >
                            (e.g.
                            <code class="text-cyan-300 font-mono"
                              >{detectRelativeVolumes(
                                stack.compose_yaml,
                              )[0]}</code
                            >). Taking full control lets you resolve them to
                            absolute paths so restarts and redeployments won't
                            break.
                          </span>
                        </div>
                      {/if}
                    </div>
                  </div>
                  <button
                    onclick={() => openAdoptModal(stack)}
                    class="inline-flex items-center gap-2 px-4 py-2 rounded-xl bg-gradient-to-r from-violet-600 to-cyan-600 hover:from-violet-500 hover:to-cyan-500 text-white text-xs font-semibold shadow-[0_0_15px_rgba(124,58,237,0.35)] shrink-0 transition cursor-pointer"
                  >
                    <ShieldCheck class="w-4 h-4" />
                    <span>Take Full Control</span>
                  </button>
                </div>
              {/if}

              <!-- Running Containers Table for this Stack -->
              <div>
                <div class="flex items-center justify-between mb-2.5">
                  <h3
                    class="text-xs font-bold text-white tracking-wide flex items-center gap-2"
                  >
                    <Box class="w-3.5 h-3.5 text-cyan-400" />
                    <span>Associated Containers</span>
                  </h3>
                  <span class="text-[11px] font-mono text-slate-500">
                    {stack.matchingContainers.length} active
                  </span>
                </div>

                {#if stack.matchingContainers.length === 0}
                  <div
                    class="p-4 rounded-xl bg-[#090a10] border border-white/[0.06] text-center text-xs text-slate-500 font-mono"
                  >
                    No active containers for this stack. Click "Deploy" to
                    create and launch them.
                  </div>
                {:else}
                  <div
                    class="overflow-x-auto rounded-xl border border-white/[0.06] bg-[#090a10]"
                  >
                    <table class="w-full text-left text-xs text-slate-300">
                      <thead
                        class="bg-white/[0.02] text-[11px] font-mono uppercase text-slate-400 border-b border-white/[0.06]"
                      >
                        <tr>
                          <th class="px-3.5 py-2">Service</th>
                          <th class="px-3.5 py-2">Container Name</th>
                          <th class="px-3.5 py-2">Image</th>
                          <th class="px-3.5 py-2">Status</th>
                          <th class="px-3.5 py-2">Ports</th>
                          <th class="px-3.5 py-2 text-right">Actions</th>
                        </tr>
                      </thead>
                      <tbody
                        class="divide-y divide-white/[0.04] font-mono text-[11px]"
                      >
                        {#each stack.matchingContainers as c}
                          {@const serviceLabel =
                            c.Labels?.["com.docker.compose.service"] ||
                            "unknown"}
                          {@const isRunning =
                            typeof c.State === "string" &&
                            c.State.toLowerCase() === "running"}
                          <tr class="hover:bg-white/[0.02] transition">
                            <td
                              class="px-3.5 py-2.5 text-cyan-300 font-semibold"
                              >{serviceLabel}</td
                            >
                            <td class="px-3.5 py-2.5 text-white">
                              {c.Names && c.Names[0]
                                ? c.Names[0].replace(/^\//, "")
                                : c.Id.slice(0, 12)}
                            </td>
                            <td class="px-3.5 py-2.5 text-slate-400">
                              <span class="truncate block max-w-[200px]"
                                >{c.Image}</span
                              >
                              {#if c.Image && imageUpdateStore.hasUpdate(c.Image)}
                                <span
                                  class="inline-flex items-center gap-1 text-[9px] text-amber-300 font-semibold mt-0.5"
                                >
                                  <CircleArrowUp
                                    class="w-2.5 h-2.5 text-amber-400"
                                  />
                                  <span>Update available</span>
                                </span>
                              {:else if c.Image && imageUpdateStore.isUpToDate(c.Image)}
                                <span
                                  class="inline-flex items-center gap-1 text-[9px] text-emerald-400 mt-0.5"
                                >
                                  <CircleCheck
                                    class="w-2.5 h-2.5 text-emerald-400"
                                  />
                                  <span>Up to date</span>
                                </span>
                              {/if}
                            </td>
                            <td class="px-3.5 py-2.5">
                              {#if isRunning}
                                <span
                                  class="inline-flex items-center gap-1 text-emerald-400"
                                >
                                  <span
                                    class="w-1.5 h-1.5 rounded-full bg-emerald-400"
                                  ></span>
                                  <span>{c.Status || "running"}</span>
                                </span>
                              {:else}
                                <span
                                  class="inline-flex items-center gap-1 text-rose-400"
                                >
                                  <span
                                    class="w-1.5 h-1.5 rounded-full bg-rose-400"
                                  ></span>
                                  <span>{c.Status || "stopped"}</span>
                                </span>
                              {/if}
                            </td>
                            <td class="px-3.5 py-2.5 text-slate-400">
                              {#if c.Ports && c.Ports.length > 0}
                                {c.Ports.map((p) =>
                                  p.PublicPort
                                    ? `${p.PublicPort}:${p.PrivatePort}`
                                    : `${p.PrivatePort}`,
                                ).join(", ")}
                              {:else}
                                <span class="text-slate-600">&mdash;</span>
                              {/if}
                            </td>
                            <td class="px-3.5 py-2.5 text-right">
                              {#if onSelectContainer}
                                <button
                                  onclick={() => onSelectContainer(c.Id)}
                                  class="px-2 py-1 rounded-lg bg-white/[0.05] hover:bg-violet-600/30 text-slate-300 hover:text-violet-300 transition text-[10px]"
                                >
                                  Inspect
                                </button>
                              {/if}
                            </td>
                          </tr>
                        {/each}
                      </tbody>
                    </table>
                  </div>
                {/if}
              </div>

              <!-- Raw Compose YAML Preview with Copy -->
              <div>
                <div class="flex items-center justify-between mb-2">
                  <span class="text-xs font-bold text-white tracking-wide"
                    >Compose Definition</span
                  >
                  <div class="flex items-center gap-2">
                    <button
                      onclick={() => copyYaml(stack)}
                      class="flex items-center gap-1 px-2.5 py-1 rounded-lg bg-white/[0.05] hover:bg-white/[0.1] text-[11px] text-slate-300 hover:text-white transition"
                    >
                      {#if copiedYamlStack === stack.name}
                        <Check class="w-3.5 h-3.5 text-emerald-400" />
                        <span class="text-emerald-400">Copied!</span>
                      {:else}
                        <Copy class="w-3.5 h-3.5" />
                        <span>Copy YAML</span>
                      {/if}
                    </button>

                    {#if onEditStack}
                      <button
                        onclick={() => onEditStack(stack.name)}
                        class="flex items-center gap-1 px-2.5 py-1 rounded-lg bg-cyan-500/10 hover:bg-cyan-500/20 text-[11px] text-cyan-300 border border-cyan-500/25 transition"
                      >
                        <PenLine class="w-3.5 h-3.5" />
                        <span>Edit in Studio</span>
                      </button>
                    {/if}
                  </div>
                </div>

                {#if loadingDetails[stack.name]}
                  <div
                    class="p-6 rounded-xl bg-[#090a10] border border-white/[0.06] flex items-center justify-center gap-2.5 text-xs text-slate-400 font-mono"
                  >
                    <LoaderCircle class="w-4 h-4 text-violet-400 animate-spin" />
                    <span>Loading compose configuration...</span>
                  </div>
                {:else if stack.compose_yaml}
                  <div
                    class="max-h-60 overflow-y-auto rounded-xl bg-[#090a10] border border-white/[0.06] p-3 text-xs font-mono text-slate-300 whitespace-pre"
                  >
                    {stack.compose_yaml}
                  </div>
                {:else}
                  <div
                    class="p-4 rounded-xl bg-[#090a10] border border-white/[0.06] flex flex-col sm:flex-row sm:items-center justify-between gap-3 text-xs text-slate-400 font-mono"
                  >
                    <div class="flex items-center gap-2">
                      <CircleAlert class="w-4 h-4 text-amber-400 shrink-0" />
                      <span
                        >Compose file not accessible directly on host. Click to
                        reconstruct from live containers.</span
                      >
                    </div>
                    <button
                      onclick={() => openAdoptModal(stack)}
                      class="px-3.5 py-1.5 rounded-xl bg-gradient-to-r from-violet-600 to-cyan-600 hover:from-violet-500 hover:to-cyan-500 text-white text-xs font-sans font-semibold shadow-[0_0_15px_rgba(124,58,237,0.35)] shrink-0 transition cursor-pointer"
                    >
                      Reconstruct &amp; Adopt
                    </button>
                  </div>
                {/if}

                {#if stack.env}
                  <div class="mt-4 pt-4 border-t border-white/[0.06]">
                    <div class="flex items-center justify-between mb-2">
                      <div class="flex items-center gap-2">
                        <span class="text-xs font-bold text-white tracking-wide"
                          >Environment Variables (.env)</span
                        >
                        <span
                          class="px-2 py-0.5 rounded-full text-[10px] font-mono bg-amber-500/20 text-amber-300 border border-amber-500/30"
                        >
                          Active
                        </span>
                      </div>
                      <button
                        onclick={() => {
                          navigator.clipboard.writeText(stack.env || "");
                        }}
                        class="flex items-center gap-1 px-2.5 py-1 rounded-lg bg-white/[0.05] hover:bg-white/[0.1] text-[11px] text-slate-300 hover:text-white transition cursor-pointer"
                      >
                        <Copy class="w-3.5 h-3.5" />
                        <span>Copy .env</span>
                      </button>
                    </div>
                    <div
                      class="max-h-48 overflow-y-auto rounded-xl bg-[#090a10] border border-amber-500/20 p-3 text-xs font-mono text-amber-100 whitespace-pre selection:bg-amber-500/30"
                    >
                      {stack.env}
                    </div>
                  </div>
                {/if}
              </div>
            </div>
          {/if}
        </div>
      </ViewportGate>
    {/each}
    </div>
  {/if}

  <!-- Down Stack Confirmation Modal -->
  {#if downModalStack}
    <div
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/75 backdrop-blur-sm animate-in fade-in"
    >
      <div
        class="w-full max-w-md rounded-2xl bg-[#12131c] border border-white/[0.1] p-6 space-y-4 shadow-2xl"
      >
        <div class="flex items-center gap-3 text-amber-400">
          <div
            class="w-10 h-10 rounded-xl bg-amber-500/15 border border-amber-500/30 flex items-center justify-center"
          >
            <ArrowDownToLine class="w-5 h-5" />
          </div>
          <div>
            <h3 class="text-base font-bold text-white">Bring Down Stack?</h3>
            <p class="text-xs text-slate-400">
              Stack: <span class="font-mono text-amber-300 font-semibold"
                >{downModalStack.name}</span
              >
            </p>
          </div>
        </div>

        <p class="text-xs text-slate-300 leading-relaxed">
          Bringing down this stack will stop and remove all associated
          containers and networks.
        </p>

        <!-- Volume deletion toggle -->
        <label
          class="flex items-start gap-2.5 p-3 rounded-xl bg-[#090a10] border border-white/[0.06] cursor-pointer"
        >
          <input
            type="checkbox"
            bind:checked={downRemoveVolumes}
            class="mt-0.5 rounded border-white/20 bg-black/40 text-rose-500 focus:ring-0"
          />
          <div class="text-xs">
            <span class="font-semibold text-rose-300"
              >Also remove anonymous and named volumes</span
            >
            <p class="text-[11px] text-slate-400 mt-0.5">
              Warning: Any persistent database or application state will be
              destroyed.
            </p>
          </div>
        </label>

        <!-- Modal Actions -->
        <div class="flex items-center justify-end gap-2.5 pt-2">
          <button
            onclick={() => (downModalStack = null)}
            disabled={isDownSubmitting}
            class="px-4 py-2 rounded-xl bg-white/[0.05] hover:bg-white/[0.1] text-slate-300 text-xs font-semibold transition"
          >
            Cancel
          </button>
          <button
            onclick={confirmDown}
            disabled={isDownSubmitting}
            class="flex items-center gap-1.5 px-4 py-2 rounded-xl bg-amber-600 hover:bg-amber-500 text-white text-xs font-semibold shadow-[0_0_15px_rgba(245,158,11,0.4)] transition disabled:opacity-50"
          >
            {#if isDownSubmitting}
              <LoaderCircle class="w-3.5 h-3.5 animate-spin" />
              <span>Bringing Down...</span>
            {:else}
              <ArrowDownToLine class="w-3.5 h-3.5" />
              <span>Confirm Down</span>
            {/if}
          </button>
        </div>
      </div>
    </div>
  {/if}

  <!-- Delete Stack Confirmation Modal -->
  {#if deleteModalStack}
    <div
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/75 backdrop-blur-sm animate-in fade-in"
    >
      <div
        class="w-full max-w-md rounded-2xl bg-[#12131c] border border-rose-500/30 p-6 space-y-4 shadow-2xl"
      >
        <div class="flex items-center gap-3 text-rose-400">
          <div
            class="w-10 h-10 rounded-xl bg-rose-500/15 border border-rose-500/30 flex items-center justify-center"
          >
            <Trash class="w-5 h-5" />
          </div>
          <div>
            <h3 class="text-base font-bold text-white">
              Delete Compose Stack?
            </h3>
            <p class="text-xs text-slate-400">
              Stack: <span class="font-mono text-rose-300 font-semibold"
                >{deleteModalStack.name}</span
              >
            </p>
          </div>
        </div>

        <p class="text-xs text-slate-300 leading-relaxed">
          Are you sure you want to permanently delete this stack definition and
          bring down all its containers? This action cannot be undone.
        </p>

        <!-- Modal Actions -->
        <div class="flex items-center justify-end gap-2.5 pt-2">
          <button
            onclick={() => (deleteModalStack = null)}
            disabled={isDeleteSubmitting}
            class="px-4 py-2 rounded-xl bg-white/[0.05] hover:bg-white/[0.1] text-slate-300 text-xs font-semibold transition"
          >
            Cancel
          </button>
          <button
            onclick={confirmDelete}
            disabled={isDeleteSubmitting}
            class="flex items-center gap-1.5 px-4 py-2 rounded-xl bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold shadow-[0_0_15px_rgba(244,63,94,0.4)] transition disabled:opacity-50"
          >
            {#if isDeleteSubmitting}
              <LoaderCircle class="w-3.5 h-3.5 animate-spin" />
              <span>Deleting...</span>
            {:else}
              <Trash class="w-3.5 h-3.5" />
              <span>Confirm Delete</span>
            {/if}
          </button>
        </div>
      </div>
    </div>
  {/if}

  <!-- Redeploy Confirmation Modal -->
  {#if redeployModalStack}
    {@const modalContainers = containers.filter(
      (c) =>
        c.labels?.["com.docker.compose.project"] === redeployModalStack?.name ||
        c.labels?.["io.kompose.service"] === redeployModalStack?.name ||
        (redeployModalStack?.is_external &&
          (c.labels?.["com.docker.compose.project"] || "").toLowerCase() ===
            redeployModalStack?.name.toLowerCase()),
    )}
    <div
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-md animate-in fade-in"
    >
      <div
        class="w-full max-w-lg rounded-2xl bg-[#12131c] border {redeployRelativePaths.length > 0 ? 'border-amber-500/40 shadow-[0_0_50px_rgba(245,158,11,0.2)]' : 'border-white/[0.1] shadow-2xl'} p-6 space-y-5 animate-in zoom-in-95 duration-150"
      >
        <!-- Header -->
        <div class="flex items-center gap-3">
          <div
            class="w-10 h-10 rounded-xl {redeployRelativePaths.length > 0 ? 'bg-amber-500/15 border border-amber-500/30 text-amber-400' : 'bg-violet-500/15 border border-violet-500/30 text-violet-400'} flex items-center justify-center shrink-0"
          >
            {#if redeployRelativePaths.length > 0}
              <TriangleAlert class="w-5 h-5 text-amber-400" />
            {:else}
              <RotateCw class="w-5 h-5 text-violet-400" />
            {/if}
          </div>
          <div>
            <h3 class="text-base font-bold text-white">
              {redeployRelativePaths.length > 0
                ? "Relative Volume Paths & Redeploy"
                : `Redeploy Stack: ${redeployModalStack.name}`}
            </h3>
            <p class="text-xs text-slate-400">
              {redeployRelativePaths.length > 0
                ? `Stack: ${redeployModalStack.name}`
                : "Recreate container services with your updated Compose configuration."}
            </p>
          </div>
        </div>

        <!-- Environment Variables Summary -->
        {#if redeployModalStack.env && redeployModalStack.env.trim()}
          {@const envLines = redeployModalStack.env.trim().split('\n').filter(l => l.trim() && !l.startsWith('#'))}
          <div class="flex items-center justify-between p-3 rounded-xl bg-cyan-500/10 border border-cyan-500/20 text-xs">
            <div class="flex items-center gap-2 text-cyan-300 font-medium">
              <span class="w-2 h-2 rounded-full bg-cyan-400 animate-pulse"></span>
              <span>Environment Configuration Active</span>
            </div>
            <span class="text-[11px] font-mono text-cyan-400 bg-cyan-500/15 px-2 py-0.5 rounded-md border border-cyan-500/30">
              {envLines.length} variable{envLines.length === 1 ? '' : 's'} loaded
            </span>
          </div>
        {/if}

        <!-- Relative Volume Warnings (if detected) -->
        {#if redeployRelativePaths.length > 0}
          <p class="text-xs text-slate-300 leading-relaxed">
            This stack's Compose file uses relative paths for host directory mounts:
          </p>

          <div
            class="p-3 rounded-xl bg-[#090a10] border border-white/[0.08] space-y-2 max-h-36 overflow-y-auto font-mono text-xs"
          >
            {#each redeployRelativePaths as relPath}
              <div class="flex items-center justify-between text-[11px]">
                <span class="text-amber-300 font-semibold">{relPath}</span>
                {#if redeployComposeDir}
                  <span class="text-slate-500 mx-2">→</span>
                  <span
                    class="text-cyan-300 truncate max-w-[240px]"
                    title="{redeployComposeDir.replace(/\/+$/, '')}/{relPath.replace(/^\.\/?/, '')}"
                  >
                    {redeployComposeDir.replace(/\/+$/, "")}/{relPath.replace(/^\.\/?/, "")}
                  </span>
                {/if}
              </div>
            {/each}
          </div>

          <div class="space-y-1.5 text-xs">
            <label
              for="redeploy-compose-dir"
              class="font-semibold text-slate-300 flex items-center gap-1.5"
            >
              <Folder class="w-3.5 h-3.5 text-cyan-400" />
              <span>Host Compose Directory (YAML Location)</span>
            </label>
            <input
              id="redeploy-compose-dir"
              type="text"
              bind:value={redeployComposeDir}
              placeholder="e.g. /mnt/nvme_data/gitea"
              class="w-full px-3 py-2 rounded-xl bg-[#090a10] border border-white/[0.1] text-xs font-mono text-white focus:outline-none focus:border-cyan-500/50"
            />
          </div>

          {#if redeployComposeDir.trim()}
            <label
              class="flex items-start gap-2.5 p-3 rounded-xl bg-amber-500/10 border border-amber-500/25 cursor-pointer select-none"
            >
              <input
                type="checkbox"
                bind:checked={redeployAutoConvert}
                class="mt-0.5 rounded border-amber-500/40 bg-black/40 text-cyan-500 focus:ring-0 cursor-pointer"
              />
              <div class="text-xs">
                <span class="font-semibold text-amber-200"
                  >Convert relative paths to absolute host paths</span
                >
                <p class="text-[11px] text-slate-300 mt-0.5">
                  Permanently updates this stack's YAML so all containers and restarts reliably find their data.
                </p>
              </div>
            </label>
          {/if}
        {/if}

        <!-- Affected Containers Preview -->
        {#if modalContainers.length > 0}
          <div class="space-y-1.5">
            <div
              class="text-[11px] font-semibold text-slate-400 uppercase tracking-wider font-mono flex items-center justify-between"
            >
              <span>Affected Containers ({modalContainers.length})</span>
            </div>
            <div
              class="p-2.5 rounded-xl bg-[#090a10] border border-white/[0.06] max-h-32 overflow-y-auto space-y-1.5"
            >
              {#each modalContainers as c}
                <div class="flex items-center justify-between text-xs font-mono">
                  <span class="text-cyan-300 font-semibold">{c.names?.[0]?.replace(/^\//, "") || c.id.slice(0, 12)}</span>
                  <span class="text-slate-400 text-[11px] truncate max-w-[220px]">{c.image}</span>
                </div>
              {/each}
            </div>
          </div>
        {/if}

        <!-- Pull Latest Images Toggle Checkbox -->
        <label
          class="flex items-start gap-3 p-3.5 rounded-xl bg-violet-500/10 border border-violet-500/25 hover:border-violet-500/40 transition cursor-pointer select-none"
        >
          <input
            type="checkbox"
            bind:checked={redeployPullImages}
            class="mt-0.5 rounded border-violet-500/40 bg-black/40 text-violet-500 focus:ring-0 cursor-pointer"
          />
          <div class="text-xs space-y-0.5">
            <div class="font-semibold text-violet-200 flex items-center gap-1.5">
              <span>Re-pull latest images before redeploying</span>
              <span
                class="px-1.5 py-0.2 rounded text-[9px] font-mono bg-violet-500/25 text-violet-300 border border-violet-500/40"
                >Recommended</span
              >
            </div>
            <p class="text-[11px] text-slate-400 leading-relaxed">
              Pulls newest image tags from remote registries before restarting containers. If unchecked, uses existing cached local images.
            </p>
          </div>
        </label>

        <!-- Actions -->
        <div class="flex flex-wrap items-center justify-end gap-2.5 pt-2">
          <button
            type="button"
            onclick={() => (redeployModalStack = null)}
            disabled={isRedeploySubmitting}
            class="px-4 py-2 rounded-xl bg-white/[0.05] hover:bg-white/[0.1] text-slate-300 text-xs font-semibold transition cursor-pointer"
          >
            Cancel
          </button>

          {#if redeployRelativePaths.length > 0}
            <button
              type="button"
              onclick={confirmRedeployAsIs}
              disabled={isRedeploySubmitting}
              class="px-3.5 py-2 rounded-xl bg-white/[0.05] hover:bg-white/[0.1] border border-white/[0.1] text-slate-300 hover:text-white text-xs font-semibold transition cursor-pointer"
              title="Deploy using existing YAML with backend working directory fallback"
            >
              Redeploy As-Is
            </button>
          {/if}

          <button
            type="button"
            onclick={confirmRedeploy}
            disabled={isRedeploySubmitting ||
              (redeployRelativePaths.length > 0 && redeployAutoConvert && !redeployComposeDir.trim())}
            class="flex items-center gap-2 px-5 py-2 rounded-xl bg-gradient-to-r from-violet-600 to-indigo-600 hover:from-violet-500 hover:to-indigo-500 text-white text-xs font-semibold shadow-[0_0_20px_rgba(124,58,237,0.4)] border border-violet-400/30 transition disabled:opacity-50 cursor-pointer"
          >
            {#if isRedeploySubmitting}
              <LoaderCircle class="w-3.5 h-3.5 animate-spin" />
              <span>Deploying...</span>
            {:else}
              <RotateCw class="w-3.5 h-3.5 text-cyan-300" />
              <span>
                {redeployRelativePaths.length > 0 && redeployAutoConvert && redeployComposeDir.trim()
                  ? "Update Paths & Redeploy"
                  : redeployPullImages
                    ? "Pull Images & Redeploy"
                    : "Redeploy with Existing Images"}
              </span>
            {/if}
          </button>
        </div>
      </div>
    </div>
  {/if}

  <!-- Adopt External Stack Modal -->
  <AdoptStackModal
    stack={adoptModalStack}
    isOpen={isAdoptModalOpen}
    onClose={() => {
      isAdoptModalOpen = false;
      adoptModalStack = null;
    }}
    onAdopted={handleAdopted}
  />
</div>
