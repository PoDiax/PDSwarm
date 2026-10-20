<script lang="ts">
  import type { Snippet } from 'svelte';
  import Sidebar from './Sidebar.svelte';
  import Navbar from './Navbar.svelte';
  import FloatingTasksBar from './FloatingTasksBar.svelte';
  import type { User } from '../types';

  interface Props {
    children?: Snippet;
    activeTab?: string;
    user?: User | null;
    hostStatus?: 'connected' | 'disconnected' | 'connecting';
    hostVersion?: string;
    appVersion?: string;
    onNavigate?: (tab: string) => void;
    onLogout?: () => void;
  }

  let {
    children,
    activeTab = 'dashboard',
    user = { username: 'admin', role: 'admin' },
    hostStatus = 'connected',
    hostVersion = 'Docker Ready',
    appVersion = typeof __APP_VERSION__ !== 'undefined' ? __APP_VERSION__ : '0.2.6',
    onNavigate,
    onLogout,
  }: Props = $props();

  let isMobileSidebarOpen = $state(false);

  function toggleMobileSidebar() {
    isMobileSidebarOpen = !isMobileSidebarOpen;
  }

  function closeMobileSidebar() {
    isMobileSidebarOpen = false;
  }
</script>

<div class="app-layout h-screen bg-[#0a0b10] text-[#f8fafc] flex relative overflow-hidden selection:bg-violet-600/30 selection:text-white">
  <!-- Subtle ambient decorative glow -->
  <div class="theme-glow-1 fixed top-0 left-0 w-96 h-96 bg-violet-600/10 rounded-full blur-[128px] pointer-events-none"></div>
  <div class="theme-glow-2 fixed top-0 right-0 w-96 h-96 bg-cyan-500/10 rounded-full blur-[128px] pointer-events-none"></div>

  <!-- Desktop Sidebar -->
  <div class="hidden md:block shrink-0 h-screen z-20">
    <Sidebar
      {activeTab}
      {onNavigate}
    />
  </div>

  <!-- Mobile Sidebar Drawer Overlay -->
  {#if isMobileSidebarOpen}
    <div
      class="fixed inset-0 bg-black/70 backdrop-blur-sm z-40 md:hidden animate-in fade-in duration-200"
      onclick={closeMobileSidebar}
      onkeydown={(e) => e.key === 'Escape' && closeMobileSidebar()}
      role="button"
      tabindex="0"
      aria-label="Close mobile sidebar backdrop"
    ></div>

    <div class="fixed inset-y-0 left-0 w-64 z-50 md:hidden animate-in slide-in-from-left duration-200">
      <Sidebar
        {activeTab}
        {onNavigate}
        onCloseMobile={closeMobileSidebar}
      />
    </div>
  {/if}

  <!-- Main viewport content -->
  <div class="flex-1 flex flex-col min-w-0 h-screen overflow-hidden">
    <Navbar
      {user}
      {hostStatus}
      {hostVersion}
      {onLogout}
      onToggleSidebar={toggleMobileSidebar}
    />

    <!-- Floating Top-Right Background Tasks Notification Bar -->
    <FloatingTasksBar />

    <main class="flex-1 min-h-0 w-full max-w-7xl mx-auto flex flex-col {activeTab === 'templates' ? 'p-3 md:p-5 overflow-hidden' : 'p-4 md:p-8 overflow-y-auto'}">
      {#if children}
        {@render children()}
      {/if}
    </main>

    <!-- Footer -->
    <footer class="app-footer shrink-0 w-full border-t border-white/[0.04] bg-[#0a0b10]/80 backdrop-blur-sm px-4 md:px-8 py-2.5 flex items-center justify-between text-[11px] font-mono text-slate-500 z-10">
      <!-- Left spacer for balancing center text -->
      <div class="hidden sm:block w-24"></div>

      <!-- Center Author Credit -->
      <div class="flex items-center gap-1.5">
        <span>Made by</span>
        <a href="https://github.com/PoDiax/" target="_blank" rel="noopener noreferrer" class="footer-link text-slate-400 hover:text-cyan-400 transition font-semibold">
          PoDiax
        </a>
      </div>

      <!-- Bottom Right: Live App Version -->
      <div class="flex items-center justify-end sm:w-24">
        <span
          class="px-2 py-0.5 text-[10px] font-mono tracking-wider rounded-md bg-violet-500/10 text-violet-300 border border-violet-500/20 shadow-[0_0_10px_rgba(124,58,237,0.1)] select-none hover:border-violet-500/40 transition-colors"
          title="PDSwarm v{appVersion.replace(/^v/, '')}"
        >
          {appVersion.startsWith('v') ? appVersion : `v${appVersion}`}
        </span>
      </div>
    </footer>
  </div>
</div>
