<script lang="ts">
  import {
    Menu,
    LogOut,
    User as UserIcon,
    ChevronDown,
    Shield,
  } from '@lucide/svelte';
  import EnvironmentSwitcher from './EnvironmentSwitcher.svelte';
  import NavbarTasksWidget from './NavbarTasksWidget.svelte';
  import type { User } from '../types';

  interface Props {
    user?: User | null;
    hostStatus?: 'connected' | 'disconnected' | 'connecting';
    hostVersion?: string;
    onLogout?: () => void;
    onToggleSidebar?: () => void;
  }

  let {
    user = { username: 'admin', role: 'admin' },
    hostStatus = 'connected',
    hostVersion = 'Docker Ready',
    onLogout,
    onToggleSidebar,
  }: Props = $props();

  let isUserMenuOpen = $state(false);

  function toggleUserMenu() {
    isUserMenuOpen = !isUserMenuOpen;
  }

  function handleLogout() {
    isUserMenuOpen = false;
    if (onLogout) {
      onLogout();
    }
  }

  function closeMenu() {
    isUserMenuOpen = false;
  }
</script>

<svelte:window onclick={(e) => {
  const target = e.target as HTMLElement;
  if (!target.closest('#user-menu-container')) {
    closeMenu();
  }
}} />

<header class="app-navbar h-16 flex items-center justify-between px-4 md:px-6 bg-[#0a0b10]/80 backdrop-blur-xl border-b border-white/[0.08] sticky top-0 z-30">
  <!-- Left section: Hamburger button & Host quick switcher -->
  <div class="flex items-center gap-3">
    {#if onToggleSidebar}
      <button
        onclick={onToggleSidebar}
        class="p-2 rounded-lg text-slate-400 hover:text-white hover:bg-white/5 md:hidden transition"
        aria-label="Toggle menu"
      >
        <Menu class="w-5 h-5" />
      </button>
    {/if}

    <!-- Environment Switcher -->
    <EnvironmentSwitcher />
  </div>

  <!-- Right section: System telemetry and User Profile Dropdown -->
  <div class="flex items-center gap-3">
    <!-- Active background operations pill / dropdown widget -->
    <NavbarTasksWidget />

    <!-- User dropdown -->
    <div class="relative" id="user-menu-container">
      <button
        onclick={toggleUserMenu}
        class="flex items-center gap-2.5 p-1.5 pr-2.5 rounded-xl bg-[#12131c] hover:bg-[#181a26] border border-white/[0.08] hover:border-violet-500/40 transition-all duration-150"
        aria-expanded={isUserMenuOpen}
      >
        <!-- Avatar Initial with glowing gradient -->
        <div class="w-7 h-7 rounded-lg bg-gradient-to-br from-violet-600 to-cyan-500 flex items-center justify-center font-bold text-xs text-white shadow-[0_0_10px_rgba(124,58,237,0.4)]">
          {user?.username ? user.username.charAt(0).toUpperCase() : 'A'}
        </div>

        <div class="hidden md:flex flex-col text-left leading-none">
          <span class="text-xs font-semibold text-slate-200">{user?.username || 'admin'}</span>
          <span class="text-[10px] text-violet-400 font-mono mt-0.5 uppercase tracking-wide">{user?.role || 'admin'}</span>
        </div>

        <ChevronDown class="w-3.5 h-3.5 text-slate-400 transition-transform duration-200 {isUserMenuOpen ? 'rotate-180 text-white' : ''}" />
      </button>

      <!-- Dropdown Popup -->
      {#if isUserMenuOpen}
        <div
          class="absolute right-0 mt-2 w-56 rounded-xl bg-[#12131c]/95 backdrop-blur-xl border border-white/[0.08] shadow-[0_10px_30px_rgba(0,0,0,0.5)] p-2 z-50 animate-in fade-in duration-100"
        >
          <!-- User header info -->
          <div class="px-3 py-2 border-b border-white/[0.06] mb-1">
            <p class="text-xs font-semibold text-white flex items-center gap-1.5">
              <Shield class="w-3.5 h-3.5 text-violet-400" />
              {user?.username || 'admin'}
            </p>
            <p class="text-[11px] text-slate-400 font-mono mt-0.5">Role: {user?.role || 'admin'}</p>
          </div>

          <!-- Actions -->
          <button
            onclick={handleLogout}
            class="w-full flex items-center gap-2 px-3 py-2 rounded-lg text-xs font-medium text-rose-400 hover:text-rose-300 hover:bg-rose-500/10 transition"
          >
            <LogOut class="w-4 h-4" />
            <span>Sign Out</span>
          </button>
        </div>
      {/if}
    </div>
  </div>
</header>
