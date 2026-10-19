<script lang="ts">
  import {
    onMount,
    onDestroy } from "svelte";
  import {
    Network as NetworkIcon,
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
    TriangleAlert,
    X,
    Shield,
    Box,
    Globe,
    ChevronDown,
    ChevronUp,
    Info,
  } from '@lucide/svelte';
  import { networkApi, containerApi } from "../lib/api";
  import type { Network, Container } from "../lib/types";
  import CreateNetworkModal from "../lib/components/CreateNetworkModal.svelte";

  interface Props {
    onNavigate?: (tab: string) => void;
    onSelectContainer?: (containerId: string) => void;
  }

  let { onNavigate, onSelectContainer }: Props = $props();

  // State
  let networks = $state<Network[]>([]);
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

  // Modals
  let isCreateModalOpen = $state(false);
  let isPruneModalOpen = $state(false);
  let isPruning = $state(false);

  let deleteNetworkTarget = $state<Network | null>(null);
  let isDeleting = $state(false);

  // Expanded row details for attached containers
  let expandedNetworks = $state<Record<string, boolean>>({});

  // Clipboard & Timers
  let copiedId = $state<string | null>(null);
  let refreshTimer: ReturnType<typeof setInterval> | null = null;

  const systemNetworkNames = new Set(["bridge", "host", "none"]);

  function showFeedback(type: "success" | "error", message: string) {
    feedback = { type, message };
    setTimeout(() => {
      if (feedback?.message === message) {
        feedback = null;
      }
    }, 4500);
  }

  async function fetchNetworks(silent = false) {
    if (!silent) isRefreshing = true;
    error = null;

    try {
      const [nets, conts] = await Promise.all([
        networkApi.list(),
        containerApi.list(true).catch(() => [] as Container[]),
      ]);
      networks = nets || [];
      containers = conts || [];
    } catch (err: unknown) {
      if (err instanceof Error) {
        error = err.message;
      } else {
        error = "Failed to load Docker networks.";
      }
    } finally {
      isLoading = false;
      isRefreshing = false;
    }
  }

  onMount(() => {
    fetchNetworks();
    refreshTimer = setInterval(() => {
      fetchNetworks(true);
    }, 10000);
  });

  onDestroy(() => {
    if (refreshTimer) clearInterval(refreshTimer);
  });

  function getShortId(fullId?: string): string {
    if (!fullId) return "unknown";
    return fullId.slice(0, 12);
  }

  function copyToClipboard(text: string, id: string) {
    navigator.clipboard.writeText(text);
    copiedId = id;
    setTimeout(() => {
      if (copiedId === id) copiedId = null;
    }, 2000);
  }

  function isSystemNetwork(net: Network): boolean {
    return (
      systemNetworkNames.has(net.Name.toLowerCase()) || net.Driver === "null"
    );
  }

  function getDriverStyle(driver: string): { badge: string; text: string } {
    const d = (driver || "").toLowerCase();
    if (d === "bridge") {
      return {
        badge: "bg-violet-500/15 text-violet-300 border-violet-500/30",
        text: "Bridge",
      };
    }
    if (d === "host") {
      return {
        badge: "bg-cyan-500/15 text-cyan-300 border-cyan-500/30",
        text: "Host",
      };
    }
    if (d === "overlay") {
      return {
        badge: "bg-emerald-500/15 text-emerald-300 border-emerald-500/30",
        text: "Overlay",
      };
    }
    if (d === "macvlan") {
      return {
        badge: "bg-amber-500/15 text-amber-300 border-amber-500/30",
        text: "Macvlan",
      };
    }
    return {
      badge: "bg-slate-500/15 text-slate-300 border-slate-500/30",
      text: driver || "null",
    };
  }

  function getSubnetInfo(net: Network): { subnet: string; gateway: string } {
    const config = net.IPAM?.Config;
    if (config && config.length > 0) {
      return {
        subnet: config[0].Subnet || "—",
        gateway: config[0].Gateway || "—",
      };
    }
    return { subnet: "—", gateway: "—" };
  }

  interface AttachedEndpoint {
    id: string;
    name: string;
    ipv4?: string;
    mac?: string;
  }

  function getAttachedContainers(net: Network): AttachedEndpoint[] {
    const list: AttachedEndpoint[] = [];
    if (net.Containers) {
      for (const [id, c] of Object.entries(net.Containers)) {
        list.push({
          id: id.slice(0, 12),
          name: c.Name || id.slice(0, 12),
          ipv4: c.IPv4Address || undefined,
          mac: c.MacAddress || undefined,
        });
      }
    }
    return list;
  }

  let availableDrivers = $derived.by(() => {
    const set = new Set<string>();
    for (const n of networks) {
      if (n.Driver) set.add(n.Driver);
    }
    return Array.from(set);
  });

  let filteredNetworks = $derived.by(() => {
    return networks.filter((n) => {
      if (driverFilter !== "all" && n.Driver !== driverFilter) {
        return false;
      }
      if (!searchQuery.trim()) return true;
      const q = searchQuery.toLowerCase().trim();
      const nameMatch = n.Name.toLowerCase().includes(q);
      const idMatch = n.Id.toLowerCase().includes(q);
      const driverMatch = (n.Driver || "").toLowerCase().includes(q);
      const subnetInfo = getSubnetInfo(n);
      const subnetMatch =
        subnetInfo.subnet.includes(q) || subnetInfo.gateway.includes(q);
      return nameMatch || idMatch || driverMatch || subnetMatch;
    });
  });

  function toggleExpand(netId: string) {
    expandedNetworks = {
      ...expandedNetworks,
      [netId]: !expandedNetworks[netId],
    };
  }

  // Actions
  function openDeleteModal(net: Network) {
    if (isSystemNetwork(net)) {
      showFeedback(
        "error",
        `Cannot delete built-in system network "${net.Name}".`,
      );
      return;
    }
    deleteNetworkTarget = net;
  }

  function closeDeleteModal() {
    deleteNetworkTarget = null;
    isDeleting = false;
  }

  async function confirmDeleteNetwork() {
    if (!deleteNetworkTarget) return;
    isDeleting = true;

    try {
      await networkApi.remove(deleteNetworkTarget.Id);
      showFeedback("success", `Network "${deleteNetworkTarget.Name}" removed.`);
      closeDeleteModal();
      await fetchNetworks(true);
    } catch (err: unknown) {
      if (err instanceof Error) {
        showFeedback("error", `Failed to remove network: ${err.message}`);
      } else {
        showFeedback("error", "Failed to remove network.");
      }
    } finally {
      isDeleting = false;
    }
  }

  async function confirmPrune() {
    isPruning = true;
    try {
      const res = await networkApi.prune();
      const count = res.NetworksDeleted?.length || 0;
      showFeedback(
        "success",
        `Pruned ${count} unused network${count === 1 ? "" : "s"}.`,
      );
      isPruneModalOpen = false;
      await fetchNetworks(true);
    } catch (err: unknown) {
      if (err instanceof Error) {
        showFeedback("error", `Failed to prune networks: ${err.message}`);
      } else {
        showFeedback("error", "Failed to prune networks.");
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
        <NetworkIcon class="w-5 h-5" />
      </div>
      <div>
        <div class="flex items-center gap-2">
          <h1 class="text-xl font-bold tracking-tight text-white">Networks</h1>
          <span
            class="px-2 py-0.5 text-[10px] font-mono rounded-full bg-violet-500/20 text-violet-300 border border-violet-500/30 font-semibold"
          >
            {networks.length}
          </span>
        </div>
        <p class="text-xs text-slate-400">
          Docker virtual bridges, host interfaces, and container
          inter-communication topologies
        </p>
      </div>
    </div>

    <!-- Header Actions -->
    <div class="flex items-center gap-2.5">
      <button
        onclick={() => (isPruneModalOpen = true)}
        class="px-3.5 py-2 rounded-xl bg-white/[0.04] hover:bg-white/[0.08] text-slate-300 hover:text-white border border-white/[0.08] text-xs font-medium transition flex items-center gap-2"
        title="Prune unused networks"
      >
        <Scissors class="w-3.5 h-3.5 text-amber-400" />
        <span>Prune Unused</span>
      </button>

      <button
        onclick={() => fetchNetworks()}
        disabled={isRefreshing}
        class="p-2 rounded-xl bg-white/[0.04] hover:bg-white/[0.08] text-slate-400 hover:text-white border border-white/[0.08] transition disabled:opacity-50"
        title="Refresh network list"
        aria-label="Refresh networks"
      >
        <RefreshCw
          class="w-4 h-4 {isRefreshing ? 'animate-spin text-violet-400' : ''}"
        />
      </button>

      <button
        onclick={() => (isCreateModalOpen = true)}
        class="px-4 py-2 rounded-xl bg-violet-600 hover:bg-violet-500 text-white text-xs font-semibold shadow-[0_0_20px_rgba(124,58,237,0.4)] transition flex items-center gap-2 group"
      >
        <Plus class="w-3.5 h-3.5 group-hover:scale-110 transition-transform" />
        Create Network
      </button>
    </div>
  </div>

  <!-- Feedback Toast / Banner -->
  {#if feedback}
    <div
      class="p-3.5 rounded-xl border flex items-center justify-between text-xs animate-in fade-in duration-200 {feedback.type ===
      'success'
        ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-300'
        : 'bg-rose-500/10 border-rose-500/30 text-rose-300'}"
    >
      <div class="flex items-center gap-2">
        {#if feedback.type === "success"}
          <CircleCheck class="w-4 h-4 text-emerald-400 shrink-0" />
        {:else}
          <CircleAlert class="w-4 h-4 text-rose-400 shrink-0" />
        {/if}
        <span>{feedback.message}</span>
      </div>
      <button
        onclick={() => (feedback = null)}
        class="p-1 hover:bg-white/10 rounded-lg text-slate-400 hover:text-white"
        aria-label="Close notification"
      >
        <X class="w-3.5 h-3.5" />
      </button>
    </div>
  {/if}

  {#if error}
    <div
      class="p-4 rounded-xl bg-rose-500/10 border border-rose-500/30 flex items-start gap-3 text-rose-300"
    >
      <CircleAlert class="w-5 h-5 shrink-0 text-rose-400 mt-0.5" />
      <div>
        <h3 class="text-xs font-bold text-rose-200">Network Service Error</h3>
        <p class="text-xs mt-0.5">{error}</p>
      </div>
    </div>
  {/if}

  <!-- Stats Strip -->
  <div class="grid grid-cols-2 md:grid-cols-4 gap-3">
    <div
      class="p-4 rounded-xl bg-[#12131c] border border-white/[0.08] flex items-center gap-3"
    >
      <div
        class="w-9 h-9 rounded-xl bg-violet-500/10 border border-violet-500/20 flex items-center justify-center text-violet-400"
      >
        <NetworkIcon class="w-4 h-4" />
      </div>
      <div>
        <span class="text-[11px] text-slate-400 block font-mono"
          >Total Networks</span
        >
        <span class="text-lg font-bold text-white font-mono"
          >{networks.length}</span
        >
      </div>
    </div>

    <div
      class="p-4 rounded-xl bg-[#12131c] border border-white/[0.08] flex items-center gap-3"
    >
      <div
        class="w-9 h-9 rounded-xl bg-cyan-500/10 border border-cyan-500/20 flex items-center justify-center text-cyan-400"
      >
        <Globe class="w-4 h-4" />
      </div>
      <div>
        <span class="text-[11px] text-slate-400 block font-mono"
          >Bridge Networks</span
        >
        <span class="text-lg font-bold text-cyan-400 font-mono">
          {networks.filter((n) => n.Driver === "bridge").length}
        </span>
      </div>
    </div>

    <div
      class="p-4 rounded-xl bg-[#12131c] border border-white/[0.08] flex items-center gap-3"
    >
      <div
        class="w-9 h-9 rounded-xl bg-emerald-500/10 border border-emerald-500/20 flex items-center justify-center text-emerald-400"
      >
        <Shield class="w-4 h-4" />
      </div>
      <div>
        <span class="text-[11px] text-slate-400 block font-mono"
          >System Networks</span
        >
        <span class="text-lg font-bold text-emerald-400 font-mono">
          {networks.filter((n) => isSystemNetwork(n)).length}
        </span>
      </div>
    </div>

    <div
      class="p-4 rounded-xl bg-[#12131c] border border-white/[0.08] flex items-center gap-3"
    >
      <div
        class="w-9 h-9 rounded-xl bg-amber-500/10 border border-amber-500/20 flex items-center justify-center text-amber-400"
      >
        <Box class="w-4 h-4" />
      </div>
      <div>
        <span class="text-[11px] text-slate-400 block font-mono"
          >Custom Bridges</span
        >
        <span class="text-lg font-bold text-amber-400 font-mono">
          {networks.filter((n) => !isSystemNetwork(n)).length}
        </span>
      </div>
    </div>
  </div>

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
        placeholder="Funnel by network name, ID, driver, or subnet..."
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

    <!-- Driver Selector -->
    <div class="flex items-center gap-2 w-full md:w-auto">
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

  <!-- Networks Table Container -->
  <div
    class="rounded-2xl bg-[#12131c] border border-white/[0.08] overflow-hidden shadow-xl"
  >
    {#if isLoading}
      <div
        class="p-12 flex flex-col items-center justify-center gap-3 text-slate-400"
      >
        <LoaderCircle class="w-7 h-7 text-violet-400 animate-spin" />
        <span class="text-xs font-mono">Querying Docker Engine networks...</span
        >
      </div>
    {:else if filteredNetworks.length === 0}
      <div class="p-12 text-center space-y-3">
        <div
          class="w-12 h-12 mx-auto rounded-2xl bg-white/[0.04] border border-white/[0.08] flex items-center justify-center text-slate-500"
        >
          <NetworkIcon class="w-6 h-6" />
        </div>
        <h3 class="text-sm font-semibold text-white">No networks found</h3>
        <p class="text-xs text-slate-400 max-w-sm mx-auto">
          {#if searchQuery}
            No networks match your search query "{searchQuery}".
          {:else}
            There are no custom Docker networks found.
          {/if}
        </p>
        <div class="pt-2">
          <button
            onclick={() => (isCreateModalOpen = true)}
            class="px-4 py-2 rounded-xl bg-violet-600 hover:bg-violet-500 text-white text-xs font-semibold shadow-[0_0_15px_rgba(124,58,237,0.35)] transition inline-flex items-center gap-2"
          >
            <Plus class="w-3.5 h-3.5" />
            Create Your First Network
          </button>
        </div>
      </div>
    {:else}
      <div class="overflow-x-auto">
        <table class="w-full text-left border-collapse text-xs">
          <thead>
            <tr
              class="border-b border-white/[0.08] bg-[#0c0d14]/60 text-slate-400 font-mono text-[11px] uppercase tracking-wider"
            >
              <th class="py-3 px-4 font-semibold">Network Name</th>
              <th class="py-3 px-4 font-semibold">Network ID</th>
              <th class="py-3 px-4 font-semibold">Driver</th>
              <th class="py-3 px-4 font-semibold">Subnet / Gateway</th>
              <th class="py-3 px-4 font-semibold">Scope</th>
              <th class="py-3 px-4 font-semibold">Attached</th>
              <th class="py-3 px-4 font-semibold text-right">Actions</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-white/[0.04]">
            {#each filteredNetworks as net (net.Id)}
              {@const isSystem = isSystemNetwork(net)}
              {@const driverStyle = getDriverStyle(net.Driver)}
              {@const ipam = getSubnetInfo(net)}
              {@const attached = getAttachedContainers(net)}
              {@const isExpanded = !!expandedNetworks[net.Id]}
              <tr class="hover:bg-white/[0.02] transition-colors group">
                <!-- Name -->
                <td class="py-3 px-4">
                  <div class="flex items-center gap-2.5">
                    <div
                      class="w-8 h-8 rounded-xl bg-[#090a0f] border border-white/[0.08] flex items-center justify-center text-slate-400 group-hover:text-violet-400 group-hover:border-violet-500/30 transition shrink-0"
                    >
                      <NetworkIcon class="w-4 h-4" />
                    </div>
                    <div class="min-w-0">
                      <div
                        class="flex items-center gap-2 font-mono font-semibold text-white tracking-wide"
                      >
                        <span class="truncate">{net.Name}</span>
                        {#if isSystem}
                          <span
                            class="px-1.5 py-0.2 rounded bg-cyan-500/10 text-cyan-300 border border-cyan-500/20 text-[10px] font-mono"
                          >
                            system
                          </span>
                        {/if}
                        {#if net.Internal}
                          <span
                            class="px-1.5 py-0.2 rounded bg-amber-500/10 text-amber-300 border border-amber-500/20 text-[10px] font-mono"
                          >
                            internal
                          </span>
                        {/if}
                      </div>
                    </div>
                  </div>
                </td>

                <!-- ID -->
                <td
                  class="py-3 px-4 font-mono text-slate-300 whitespace-nowrap"
                >
                  <div class="flex items-center gap-1.5">
                    <span class="text-violet-300 font-semibold"
                      >{getShortId(net.Id)}</span
                    >
                    <button
                      onclick={() =>
                        copyToClipboard(net.Id, `net-id-${net.Id}`)}
                      class="p-1 rounded hover:bg-white/10 text-slate-500 hover:text-white transition"
                      title="Copy full network ID"
                      aria-label="Copy full network ID"
                    >
                      {#if copiedId === `net-id-${net.Id}`}
                        <Check class="w-3 h-3 text-emerald-400" />
                      {:else}
                        <Copy class="w-3 h-3" />
                      {/if}
                    </button>
                  </div>
                </td>

                <!-- Driver -->
                <td class="py-3 px-4 whitespace-nowrap">
                  <span
                    class="px-2 py-0.5 rounded-md border font-mono text-[10px] {driverStyle.badge}"
                  >
                    {driverStyle.text}
                  </span>
                </td>

                <!-- Subnet / Gateway -->
                <td
                  class="py-3 px-4 font-mono text-slate-300 whitespace-nowrap"
                >
                  {#if ipam.subnet !== "—"}
                    <div class="flex flex-col">
                      <span class="text-white font-medium">{ipam.subnet}</span>
                      {#if ipam.gateway !== "—"}
                        <span class="text-[10px] text-slate-500"
                          >gw: {ipam.gateway}</span
                        >
                      {/if}
                    </div>
                  {:else}
                    <span class="text-slate-500">—</span>
                  {/if}
                </td>

                <!-- Scope -->
                <td
                  class="py-3 px-4 font-mono text-slate-400 whitespace-nowrap"
                >
                  <span class="text-slate-300">{net.Scope || "local"}</span>
                </td>

                <!-- Attached -->
                <td class="py-3 px-4 whitespace-nowrap">
                  {#if attached.length > 0}
                    <button
                      type="button"
                      onclick={() => toggleExpand(net.Id)}
                      class="inline-flex items-center gap-1 px-2.5 py-1 rounded-lg bg-emerald-500/10 hover:bg-emerald-500/20 text-emerald-300 border border-emerald-500/20 font-mono text-[10px] transition"
                    >
                      <span class="w-1.5 h-1.5 rounded-full bg-emerald-400"
                      ></span>
                      {attached.length} container{attached.length === 1
                        ? ""
                        : "s"}
                      {#if isExpanded}
                        <ChevronUp class="w-3 h-3 ml-0.5" />
                      {:else}
                        <ChevronDown class="w-3 h-3 ml-0.5" />
                      {/if}
                    </button>
                  {:else}
                    <span class="text-slate-500 font-mono text-[11px]"
                      >0 containers</span
                    >
                  {/if}
                </td>

                <!-- Actions -->
                <td class="py-3 px-4 text-right whitespace-nowrap">
                  {#if isSystem}
                    <span
                      class="p-1.5 rounded-lg text-slate-600 cursor-not-allowed inline-block"
                      title="System default networks cannot be deleted"
                    >
                      <Trash class="w-3.5 h-3.5 opacity-40" />
                    </span>
                  {:else}
                    <button
                      onclick={() => openDeleteModal(net)}
                      class="p-1.5 rounded-lg bg-rose-500/10 hover:bg-rose-500/20 text-rose-400 border border-rose-500/20 hover:border-rose-500/40 transition"
                      title="Delete network"
                      aria-label="Delete network"
                    >
                      <Trash class="w-3.5 h-3.5" />
                    </button>
                  {/if}
                </td>
              </tr>

              <!-- Expandable attached containers sub-row -->
              {#if isExpanded && attached.length > 0}
                <tr class="bg-[#0c0d14]/80 border-b border-white/[0.04]">
                  <td colspan="7" class="py-3 px-6">
                    <div class="space-y-2">
                      <span
                        class="text-[11px] font-mono text-slate-400 font-semibold uppercase tracking-wider flex items-center gap-1.5"
                      >
                        <Box class="w-3.5 h-3.5 text-cyan-400" />
                        Attached Containers in {net.Name}
                      </span>
                      <div
                        class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-2"
                      >
                        {#each attached as item}
                          <div
                            class="p-2 rounded-xl bg-[#12131c] border border-white/[0.06] flex items-center justify-between font-mono text-[11px]"
                          >
                            <div class="flex items-center gap-2 min-w-0">
                              <Box
                                class="w-3.5 h-3.5 text-slate-500 shrink-0"
                              />
                              <span class="text-white font-medium truncate"
                                >{item.name}</span
                              >
                            </div>
                            <div
                              class="text-right text-[10px] text-cyan-300 font-semibold shrink-0 ml-2"
                            >
                              {item.ipv4 || "auto"}
                            </div>
                          </div>
                        {/each}
                      </div>
                    </div>
                  </td>
                </tr>
              {/if}
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  </div>
</div>

<!-- Create Network Modal -->
<CreateNetworkModal
  isOpen={isCreateModalOpen}
  onClose={() => (isCreateModalOpen = false)}
  onCreated={(netId) => {
    showFeedback("success", `Network created successfully.`);
    fetchNetworks(true);
  }}
/>

<!-- Delete Network Confirmation Modal -->
{#if deleteNetworkTarget}
  {@const attached = getAttachedContainers(deleteNetworkTarget)}
  <div
    class="fixed inset-0 z-50 flex items-center justify-center p-4"
    role="dialog"
    aria-modal="true"
    aria-labelledby="delete-network-modal-title"
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
            id="delete-network-modal-title"
            class="text-sm font-bold text-white"
          >
            Delete Docker Network
          </h2>
          <p class="text-[11px] text-slate-400 font-mono truncate max-w-xs">
            {deleteNetworkTarget.Name} ({getShortId(deleteNetworkTarget.Id)})
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
              This network currently has <strong
                >{attached.length} active endpoint{attached.length === 1
                  ? ""
                  : "s"}</strong
              > connected.
            </span>
            <span class="block mt-1 text-[11px] text-amber-400">
              Docker will fail to delete this network until all containers
              disconnect from it.
            </span>
          </div>
        </div>
      {/if}

      <p class="text-xs text-slate-300 leading-relaxed">
        Are you sure you want to remove network <strong
          class="text-white font-mono">{deleteNetworkTarget.Name}</strong
        >?
      </p>

      <div
        class="flex items-center justify-end gap-2 pt-2 border-t border-white/[0.08]"
      >
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
          onclick={confirmDeleteNetwork}
          disabled={isDeleting}
          class="px-4 py-2 rounded-xl bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold shadow-[0_0_15px_rgba(244,63,94,0.35)] transition flex items-center gap-2"
        >
          {#if isDeleting}
            <LoaderCircle class="w-3.5 h-3.5 animate-spin" />
            Deleting...
          {:else}
            <Trash class="w-3.5 h-3.5" />
            Delete Network
          {/if}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Prune Networks Modal -->
{#if isPruneModalOpen}
  <div
    class="fixed inset-0 z-50 flex items-center justify-center p-4"
    role="dialog"
    aria-modal="true"
    aria-labelledby="prune-networks-modal-title"
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
            id="prune-networks-modal-title"
            class="text-sm font-bold text-white"
          >
            Prune Unused Networks
          </h2>
          <p class="text-[11px] text-slate-400">
            Clean up unreferenced Docker networks
          </p>
        </div>
      </div>

      <p class="text-xs text-slate-300 leading-relaxed">
        This will remove all custom Docker networks that are not currently used
        by at least one container. Pre-defined system networks (<code
          class="text-cyan-300 font-mono">bridge</code
        >, <code class="text-cyan-300 font-mono">host</code>,
        <code class="text-cyan-300 font-mono">none</code>) will never be
        removed.
      </p>

      <div
        class="flex items-center justify-end gap-2 pt-2 border-t border-white/[0.08]"
      >
        <button
          type="button"
          onclick={() => (isPruneModalOpen = false)}
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
            <LoaderCircle class="w-3.5 h-3.5 animate-spin" />
            Pruning...
          {:else}
            <Scissors class="w-3.5 h-3.5" />
            Prune Networks
          {/if}
        </button>
      </div>
    </div>
  </div>
{/if}
