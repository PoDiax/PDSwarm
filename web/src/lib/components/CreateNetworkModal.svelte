<script lang="ts">
  import {
    X,
    Network as NetworkIcon,
    Plus,
    AlertCircle,
    Loader2,
    Shield,
    Globe,
    Cpu,
  } from '@lucide/svelte';
  import { networkApi } from '../api';
  import type { CreateNetworkRequest } from '../types';

  interface Props {
    isOpen: boolean;
    onClose: () => void;
    onCreated?: (networkId: string) => void;
  }

  let {
    isOpen = false,
    onClose,
    onCreated,
  }: Props = $props();

  // Form states
  let networkName = $state('');
  let driver = $state<'bridge' | 'host' | 'macvlan' | 'overlay'>('bridge');
  let subnet = $state('');
  let gateway = $state('');

  let isSubmitting = $state(false);
  let error = $state<string | null>(null);

  const driverOptions: Array<{ id: 'bridge' | 'host' | 'macvlan' | 'overlay'; label: string; desc: string }> = [
    { id: 'bridge', label: 'Bridge', desc: 'Isolated software bridge for standalone containers on the same host' },
    { id: 'host', label: 'Host', desc: 'Directly share the host networking namespace (no isolation)' },
    { id: 'macvlan', label: 'Macvlan', desc: 'Assign a physical MAC address to containers for direct LAN routing' },
    { id: 'overlay', label: 'Overlay', desc: 'Multi-host network communication across Swarm cluster nodes' },
  ];

  function resetForm() {
    networkName = '';
    driver = 'bridge';
    subnet = '';
    gateway = '';
    error = null;
    isSubmitting = false;
  }

  async function handleSubmit(e: SubmitEvent) {
    e.preventDefault();
    const trimmedName = networkName.trim();
    if (!trimmedName) {
      error = 'Network name is required.';
      return;
    }

    if (!/^[a-zA-Z0-9][a-zA-Z0-9_.-]*$/.test(trimmedName)) {
      error = 'Network name may only contain alphanumeric characters, dots, dashes, and underscores.';
      return;
    }

    isSubmitting = true;
    error = null;

    try {
      const req: CreateNetworkRequest = {
        name: trimmedName,
        Name: trimmedName,
        driver: driver,
        Driver: driver,
        Subnet: subnet.trim() || undefined,
        Gateway: gateway.trim() || undefined,
      };

      const res = await networkApi.create(req);
      const networkId = res.id || res.Id || '';
      resetForm();
      if (onCreated) {
        onCreated(networkId);
      }
      onClose();
    } catch (err: unknown) {
      if (err instanceof Error) {
        error = err.message;
      } else {
        error = 'Failed to create Docker network.';
      }
    } finally {
      isSubmitting = false;
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape' && isOpen && !isSubmitting) {
      handleClose();
    }
  }

  function handleClose() {
    resetForm();
    onClose();
  }
</script>

<svelte:window onkeydown={handleKeydown} />

{#if isOpen}
  <div
    class="fixed inset-0 z-50 flex items-center justify-center p-4"
    role="dialog"
    aria-modal="true"
    aria-labelledby="create-network-modal-title"
    tabindex="-1"
  >
    <!-- Backdrop button -->
    <button
      type="button"
      class="fixed inset-0 bg-black/80 backdrop-blur-md cursor-default border-0"
      onclick={handleClose}
      aria-label="Close modal backdrop"
      tabindex="-1"
    ></button>

    <div
      class="relative w-full max-w-lg flex flex-col rounded-2xl bg-[#12131c] border border-white/[0.12] shadow-[0_10px_50px_rgba(0,0,0,0.8)] overflow-hidden animate-in fade-in zoom-in-95 duration-150"
    >
      <!-- Modal Header -->
      <div class="px-6 py-4 flex items-center justify-between border-b border-white/[0.08] bg-[#0c0d14]/80">
        <div class="flex items-center gap-3">
          <div class="w-8 h-8 rounded-xl bg-violet-600/20 border border-violet-500/30 flex items-center justify-center text-violet-400 shadow-[0_0_15px_rgba(124,58,237,0.3)]">
            <NetworkIcon class="w-4 h-4" />
          </div>
          <div>
            <h2 id="create-network-modal-title" class="text-sm font-bold text-white tracking-wide">Create Docker Network</h2>
            <p class="text-[11px] text-slate-400">Configure isolated bridging or inter-container routing</p>
          </div>
        </div>

        <button
          type="button"
          onclick={handleClose}
          disabled={isSubmitting}
          class="p-1.5 rounded-lg hover:bg-white/10 text-slate-400 hover:text-white transition disabled:opacity-50"
          aria-label="Close modal"
        >
          <X class="w-4 h-4" />
        </button>
      </div>

      <!-- Modal Body -->
      <form onsubmit={handleSubmit} class="p-6 space-y-5 text-xs">
        {#if error}
          <div class="p-3 rounded-xl bg-rose-500/10 border border-rose-500/30 flex items-start gap-2.5 text-rose-300 animate-in fade-in">
            <AlertCircle class="w-4 h-4 shrink-0 mt-0.5 text-rose-400" />
            <div class="flex-1 leading-relaxed">
              <span class="font-semibold block">Network Creation Error</span>
              <span class="text-[11px]">{error}</span>
            </div>
          </div>
        {/if}

        <!-- Network Name -->
        <div class="space-y-1.5">
          <label for="network-name" class="block font-medium text-slate-300">
            Network Name <span class="text-violet-400">*</span>
          </label>
          <input
            id="network-name"
            type="text"
            bind:value={networkName}
            placeholder="e.g. frontend-net, microservices, backend-bridge"
            required
            class="w-full px-3.5 py-2.5 rounded-xl bg-[#0c0d14] border border-white/10 text-white placeholder-slate-500 focus:outline-none focus:border-violet-500 focus:ring-1 focus:ring-violet-500 font-mono text-xs transition"
          />
        </div>

        <!-- Driver Selector -->
        <div class="space-y-2">
          <span class="block font-medium text-slate-300">
            Network Driver
          </span>
          <div class="grid grid-cols-1 gap-2">
            {#each driverOptions as opt}
              <label
                class="p-3 rounded-xl border cursor-pointer transition flex items-start gap-3 {driver === opt.id
                  ? 'bg-violet-600/15 border-violet-500/50 shadow-[0_0_12px_rgba(124,58,237,0.2)]'
                  : 'bg-[#0c0d14] border-white/10 hover:border-white/20'}"
              >
                <input
                  type="radio"
                  name="network-driver"
                  value={opt.id}
                  bind:group={driver}
                  class="mt-0.5 text-violet-600 focus:ring-0 focus:ring-offset-0"
                />
                <div class="flex-1">
                  <div class="flex items-center gap-2">
                    <span class="font-semibold text-white font-mono text-xs">{opt.label}</span>
                    {#if opt.id === 'bridge'}
                      <span class="px-1.5 py-0.2 rounded bg-violet-500/20 text-violet-300 text-[10px] font-mono border border-violet-500/30">default</span>
                    {/if}
                  </div>
                  <p class="text-[11px] text-slate-400 mt-0.5 leading-relaxed">{opt.desc}</p>
                </div>
              </label>
            {/each}
          </div>
        </div>

        <!-- Optional IPAM Subnet / Gateway (Bridge & Macvlan) -->
        {#if driver === 'bridge' || driver === 'macvlan'}
          <div class="space-y-3 pt-2 border-t border-white/[0.08]">
            <span class="text-[11px] font-mono uppercase tracking-wider text-slate-400 font-semibold block">
              IPAM Configuration (Optional)
            </span>

            <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
              <div class="space-y-1">
                <label for="net-subnet" class="block text-[11px] text-slate-300">Subnet</label>
                <input
                  id="net-subnet"
                  type="text"
                  bind:value={subnet}
                  placeholder="e.g. 172.28.0.0/16"
                  class="w-full px-3 py-2 rounded-xl bg-[#0c0d14] border border-white/10 text-white placeholder-slate-500 font-mono text-xs focus:outline-none focus:border-violet-500"
                />
              </div>

              <div class="space-y-1">
                <label for="net-gateway" class="block text-[11px] text-slate-300">Gateway</label>
                <input
                  id="net-gateway"
                  type="text"
                  bind:value={gateway}
                  placeholder="e.g. 172.28.0.1"
                  class="w-full px-3 py-2 rounded-xl bg-[#0c0d14] border border-white/10 text-white placeholder-slate-500 font-mono text-xs focus:outline-none focus:border-violet-500"
                />
              </div>
            </div>
          </div>
        {/if}

        <!-- Footer Actions -->
        <div class="flex items-center justify-end gap-2 pt-4 border-t border-white/[0.08]">
          <button
            type="button"
            onclick={handleClose}
            disabled={isSubmitting}
            class="px-4 py-2 rounded-xl bg-white/5 hover:bg-white/10 text-slate-300 hover:text-white text-xs font-medium transition disabled:opacity-50"
          >
            Cancel
          </button>
          <button
            type="submit"
            disabled={isSubmitting || !networkName.trim()}
            class="px-5 py-2 rounded-xl bg-violet-600 hover:bg-violet-500 text-white text-xs font-semibold shadow-[0_0_15px_rgba(124,58,237,0.35)] transition flex items-center gap-2 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {#if isSubmitting}
              <Loader2 class="w-3.5 h-3.5 animate-spin" />
              Creating Network...
            {:else}
              <Plus class="w-3.5 h-3.5" />
              Create Network
            {/if}
          </button>
        </div>
      </form>
    </div>
  </div>
{/if}
