<script lang="ts">
  import {
    LayoutDashboard,
    Box,
    Layers,
    Boxes,
    Disc,
    HardDrive,
    Network,
    Settings,
    X,
  } from '@lucide/svelte';

  interface Props {
    activeTab?: string;
    collapsed?: boolean;
    onNavigate?: (tab: string) => void;
    onCloseMobile?: () => void;
  }

  let {
    activeTab = 'dashboard',
    collapsed = false,
    onNavigate,
    onCloseMobile,
  }: Props = $props();

  const navItems = [
    { id: 'dashboard', label: 'Dashboard', icon: LayoutDashboard },
    { id: 'containers', label: 'Containers', icon: Box },
    { id: 'stacks', label: 'Stacks', icon: Layers },
    { id: 'templates', label: 'App Store', icon: Boxes },
    { id: 'images', label: 'Images', icon: Disc },
    { id: 'volumes', label: 'Volumes', icon: HardDrive },
    { id: 'networks', label: 'Networks', icon: Network },
    { id: 'settings', label: 'Settings', icon: Settings },
  ];

  function handleSelect(id: string) {
    if (onNavigate) {
      onNavigate(id);
    }
    if (onCloseMobile) {
      onCloseMobile();
    }
  }
</script>

<aside
  class="app-sidebar h-full flex flex-col justify-between bg-[#0a0b10]/95 backdrop-blur-xl border-r border-white/[0.08] transition-all duration-200 select-none {collapsed ? 'w-20' : 'w-64'}"
>
  <!-- Top brand header -->
  <div>
    <div class="h-16 flex items-center justify-between px-5 border-b border-white/[0.08]">
      {#if collapsed}
        <div class="w-full flex justify-center">
          <button
            onclick={() => handleSelect('dashboard')}
            class="flex items-center justify-center p-1 rounded-xl hover:bg-white/[0.04] transition group"
            title="PDSwarm Dashboard"
          >
            <img src="/icon.svg" alt="PD" class="w-9 h-9 rounded-xl shadow-[0_0_15px_rgba(124,58,237,0.3)] group-hover:scale-105 transition-transform" />
          </button>
        </div>
      {:else}
        <div class="flex items-center gap-3 min-w-0">
          <button
            onclick={() => handleSelect('dashboard')}
            class="flex items-center gap-2.5 text-left group focus:outline-none"
            title="PDSwarm Dashboard"
          >
            <img src="/logo.svg" alt="PDSwarm" class="h-7 w-auto max-w-[145px] object-contain transition-transform group-hover:scale-[1.02]" />
          </button>
        </div>
      {/if}

      <!-- Mobile close button -->
      {#if onCloseMobile}
        <button
          onclick={onCloseMobile}
          class="md:hidden p-1.5 rounded-lg text-slate-400 hover:text-white hover:bg-white/5 transition"
          aria-label="Close sidebar"
        >
          <X class="w-5 h-5" />
        </button>
      {/if}
    </div>

    <!-- Navigation links -->
    <nav class="p-3 space-y-1">
      {#each navItems as item}
        {@const Icon = item.icon}
        {@const isActive = activeTab === item.id}
        <button
          onclick={() => handleSelect(item.id)}
          title={collapsed ? item.label : undefined}
          class="group w-full flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium transition-all duration-150 relative text-left {isActive
            ? 'nav-item-active bg-gradient-to-r from-violet-600/20 via-violet-600/10 to-transparent text-white border-l-2 border-violet-500 shadow-[0_0_15px_rgba(124,58,237,0.25)]'
            : 'text-slate-400 hover:text-slate-200 hover:bg-white/[0.04] border-l-2 border-transparent'}"
        >
          <div class="{isActive ? 'nav-item-icon text-violet-400 drop-shadow-[0_0_8px_rgba(124,58,237,0.6)]' : 'text-slate-400 group-hover:text-slate-200'}">
            <Icon class="w-5 h-5 shrink-0" />
          </div>
          
          {#if !collapsed}
            <span class="truncate">{item.label}</span>
            {#if isActive}
              <div class="nav-item-dot ml-auto w-1.5 h-1.5 rounded-full bg-violet-400 shadow-[0_0_8px_#8b5cf6]"></div>
            {/if}
          {/if}
        </button>
      {/each}
    </nav>
  </div>
</aside>
