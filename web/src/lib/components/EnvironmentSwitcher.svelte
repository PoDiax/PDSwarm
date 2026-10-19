<script lang="ts">
  import {
    onMount } from 'svelte';
  import {
    Server,
    ChevronDown,
    Plus,
    Check,
    Trash,
    RefreshCw,
    Terminal,
    Globe,
    Lock,
    CircleAlert,
    Activity,
  } from '@lucide/svelte';
  import { environmentStore } from '../stores/environment.svelte';
  import CreateEnvironmentModal from './CreateEnvironmentModal.svelte';
  import type { Environment } from '../types';

  let isOpen = $state(false);
  let isAddModalOpen = $state(false);
  let pingingId = $state<string | null>(null);
  let deleteConfirmId = $state<string | null>(null);

  onMount(async () => {
    await environmentStore.fetchEnvironments();
  });

  function toggleDropdown() {
    isOpen = !isOpen;
    deleteConfirmId = null;
  }

  function closeDropdown() {
    isOpen = false;
    deleteConfirmId = null;
  }

  function handleSelectEnvironment(env: Environment) {
    environmentStore.switchEnvironment(env.id);
    closeDropdown();
  }

  async function handlePing(env: Environment, e: MouseEvent) {
    e.stopPropagation();
    pingingId = env.id;
    try {
      await environmentStore.ping(env.id);
    } finally {
      pingingId = null;
    }
  }

  async function handleDelete(env: Environment, e: MouseEvent) {
    e.stopPropagation();
    if (deleteConfirmId !== env.id) {
      deleteConfirmId = env.id;
      return;
    }
    try {
      await environmentStore.deleteEnvironment(env.id);
      deleteConfirmId = null;
    } catch {
      // handled in store
    }
  }

  function handleOpenAddModal(e: MouseEvent) {
    e.stopPropagation();
    closeDropdown();
    isAddModalOpen = true;
  }

  function handleEnvironmentCreated() {
    // Already switched in store
  }

  let active = $derived(environmentStore.activeEnvironment);
  let environments = $derived(environmentStore.environments);
  let isConnected = $derived(active?.status === 'connected');
</script>

<svelte:window onclick={(e) => {
  const target = e.target as HTMLElement;
  if (!target.closest('#environment-switcher-container')) {
    closeDropdown();
  }
}} />

<div class="relative" id="environment-switcher-container">
  <!-- Trigger Button -->
  <button
    onclick={toggleDropdown}
    type="button"
    class="flex items-center gap-2 px-3 py-1.5 rounded-full bg-[#12131c] hover:bg-[#181a26] border border-white/[0.08] hover:border-violet-500/40 transition-all duration-150 shadow-sm group select-none"
    aria-expanded={isOpen}
    aria-haspopup="true"
  >
    <!-- Connection Status Glowing Dot -->
    <span class="relative flex h-2 w-2 shrink-0">
      {#if isConnected}
        <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
        <span class="relative inline-flex rounded-full h-2 w-2 bg-emerald-500 shadow-[0_0_8px_#10b981]"></span>
      {:else}
        <span class="relative inline-flex rounded-full h-2 w-2 bg-rose-500 shadow-[0_0_8px_#f43f5e]"></span>
      {/if}
    </span>

    <!-- Host Name -->
    <div class="flex items-center gap-1.5 text-xs font-medium text-slate-200 group-hover:text-white">
      <Server class="w-3.5 h-3.5 text-violet-400 shrink-0 hidden sm:inline" />
      <span class="font-semibold truncate max-w-[110px] md:max-w-[160px]">
        {active ? active.name : 'Connecting...'}
      </span>
    </div>

    <!-- Type Badge -->
    {#if active}
      <span
        class="text-[10px] font-mono px-1.5 py-0.2 rounded border hidden sm:inline-block {active.type === 'tls' ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20' : active.type === 'tcp' ? 'bg-cyan-500/10 text-cyan-400 border-cyan-500/20' : 'bg-violet-500/10 text-violet-300 border-violet-500/20'}"
      >
        {active.type === 'local' ? 'socket' : active.type}
      </span>
    {/if}

    <!-- Container Count Pill -->
    {#if active && active.containers_count !== undefined}
      <span class="text-[10px] font-mono font-medium px-1.5 py-0.2 rounded-full bg-white/[0.05] text-slate-400 border border-white/[0.06] hidden md:inline-block">
        {active.containers_count} cntrs
      </span>
    {/if}

    <ChevronDown
      class="w-3.5 h-3.5 text-slate-400 transition-transform duration-200 group-hover:text-slate-200 {isOpen ? 'rotate-180 text-white' : ''}"
    />
  </button>

  <!-- Dropdown Menu -->
  {#if isOpen}
    <div
      class="absolute left-0 mt-2 w-80 md:w-96 rounded-2xl bg-[#0e0f17]/95 backdrop-blur-xl border border-white/[0.08] shadow-[0_15px_35px_rgba(0,0,0,0.7)] p-2 z-50 animate-in fade-in duration-150"
      role="menu"
    >
      <!-- Dropdown Header -->
      <div class="px-3 py-2 border-b border-white/[0.06] flex items-center justify-between">
        <span class="text-[11px] font-semibold text-slate-400 uppercase tracking-wider flex items-center gap-1.5">
          <Activity class="w-3.5 h-3.5 text-violet-400" />
          Docker Hosts ({environments.length})
        </span>
        <button
          onclick={() => environmentStore.fetchEnvironments()}
          class="p-1 rounded text-slate-400 hover:text-white hover:bg-white/5 transition text-[11px] flex items-center gap-1"
          title="Refresh environments"
        >
          <RefreshCw class="w-3 h-3 {environmentStore.loading ? 'animate-spin text-cyan-400' : ''}" />
          <span>Refresh</span>
        </button>
      </div>

      <!-- Environment list -->
      <div class="py-1 max-h-64 overflow-y-auto space-y-1">
        {#if environments.length === 0}
          <div class="px-3 py-4 text-center text-xs text-slate-500">
            No environments found.
          </div>
        {:else}
          {#each environments as env (env.id)}
            {@const isItemActive = active?.id === env.id}
            {@const isItemUp = env.status === 'connected'}
            {@const isEnvPodman = (env.url || '').toLowerCase().includes('podman') || (env.name || '').toLowerCase().includes('podman')}
            <div
              role="button"
              tabindex="0"
              onclick={() => handleSelectEnvironment(env)}
              onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') handleSelectEnvironment(env); }}
              class="w-full text-left p-2.5 rounded-xl border transition-all duration-150 flex items-center justify-between group {isItemActive ? 'bg-violet-600/10 border-violet-500/40 shadow-[0_0_12px_rgba(124,58,237,0.1)]' : 'bg-[#12131c]/50 hover:bg-[#181a26] border-transparent hover:border-white/[0.06]'}"
            >
              <div class="flex items-center gap-2.5 min-w-0 flex-1">
                <!-- Status Dot -->
                <span class="relative flex h-2 w-2 shrink-0">
                  {#if isItemUp}
                    <span class="inline-flex rounded-full h-2 w-2 bg-emerald-500 shadow-[0_0_6px_#10b981]"></span>
                  {:else}
                    <span class="inline-flex rounded-full h-2 w-2 bg-rose-500 shadow-[0_0_6px_#f43f5e]"></span>
                  {/if}
                </span>

                <!-- Name & Meta -->
                <div class="min-w-0 flex-1">
                  <div class="flex items-center gap-1.5">
                    <span class="text-xs font-semibold truncate {isItemActive ? 'text-white' : 'text-slate-200 group-hover:text-white'}">
                      {env.name}
                    </span>
                    {#if env.is_default}
                      <span class="text-[9px] font-mono px-1 py-0.1 rounded bg-white/[0.06] text-slate-400 border border-white/[0.08]">
                        def
                      </span>
                    {/if}
                    <span
                      class="text-[9px] font-mono px-1.5 py-0.2 rounded border {isEnvPodman ? 'bg-purple-500/15 text-purple-300 border-purple-500/30' : env.type === 'tls' ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20' : env.type === 'tcp' ? 'bg-cyan-500/10 text-cyan-400 border-cyan-500/20' : 'bg-violet-500/10 text-violet-300 border-violet-500/20'}"
                    >
                      {isEnvPodman ? 'podman' : (env.type === 'local' ? 'socket' : env.type)}
                    </span>
                  </div>
                  <div class="flex items-center gap-2 mt-0.5 text-[11px] text-slate-500 font-mono">
                    <span class="truncate max-w-[150px]">{env.url}</span>
                    <span>•</span>
                    <span class="{isItemUp ? 'text-slate-400' : 'text-rose-400'}">{env.containers_count} containers</span>
                  </div>
                </div>
              </div>

              <!-- Item Actions -->
              <div class="flex items-center gap-1 shrink-0 ml-2">
                <!-- Ping Button -->
                <button
                  type="button"
                  onclick={(e) => handlePing(env, e)}
                  title="Ping host connection"
                  class="p-1.5 rounded-lg text-slate-400 hover:text-cyan-300 hover:bg-cyan-500/10 transition"
                >
                  <RefreshCw class="w-3.5 h-3.5 {pingingId === env.id ? 'animate-spin text-cyan-400' : ''}" />
                </button>

                <!-- Delete Button (Only for >1 environments) -->
                {#if environments.length > 1}
                  <button
                    type="button"
                    onclick={(e) => handleDelete(env, e)}
                    title={deleteConfirmId === env.id ? 'Click again to confirm delete' : 'Delete environment'}
                    class="p-1.5 rounded-lg transition {deleteConfirmId === env.id ? 'bg-rose-500 text-white animate-pulse' : 'text-slate-500 hover:text-rose-400 hover:bg-rose-500/10'}"
                  >
                    <Trash class="w-3.5 h-3.5" />
                  </button>
                {/if}

                <!-- Active check indicator -->
                {#if isItemActive}
                  <div class="p-1 text-cyan-400">
                    <Check class="w-4 h-4" />
                  </div>
                {/if}
              </div>
            </div>
          {/each}
        {/if}
      </div>

      <!-- Footer: Add Environment -->
      <div class="pt-2 mt-1 border-t border-white/[0.06]">
        <button
          type="button"
          onclick={handleOpenAddModal}
          class="w-full flex items-center justify-center gap-2 py-2 px-3 rounded-xl bg-gradient-to-r from-violet-600/20 to-cyan-600/20 hover:from-violet-600/30 hover:to-cyan-600/30 border border-violet-500/30 text-xs font-semibold text-violet-200 hover:text-white transition shadow-sm"
        >
          <Plus class="w-3.5 h-3.5 text-cyan-400" />
          <span>Add Environment</span>
        </button>
      </div>
    </div>
  {/if}
</div>

<!-- Add Environment Modal -->
<CreateEnvironmentModal
  isOpen={isAddModalOpen}
  onClose={() => (isAddModalOpen = false)}
  onCreated={handleEnvironmentCreated}
/>
