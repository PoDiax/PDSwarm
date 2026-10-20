<script lang="ts">
  import { onMount } from 'svelte';
  import Layout from './lib/components/Layout.svelte';
  import Setup from './routes/Setup.svelte';
  import Login from './routes/Login.svelte';
  import Dashboard from './routes/Dashboard.svelte';
  import Containers from './routes/Containers.svelte';
  import ContainerDetail from './routes/ContainerDetail.svelte';
  import Stacks from './routes/Stacks.svelte';
  import StackEditor from './routes/StackEditor.svelte';
  import Images from './routes/Images.svelte';
  import Volumes from './routes/Volumes.svelte';
  import Networks from './routes/Networks.svelte';
  import Templates from './routes/Templates.svelte';
  import SettingsView from './routes/Settings.svelte';
  import { auth } from './lib/stores/auth.svelte';
  import { environmentStore } from './lib/stores/environment.svelte';
  import {
    Box,
    Layers,
    Disc,
    HardDrive,
    Network,
    Settings,
    Cpu,
  } from '@lucide/svelte';
  import type { SystemInfo } from './lib/types';
  import { systemApi } from './lib/api';

  let activeTab = $state('dashboard');
  let selectedContainerId = $state<string | null>(null);
  let isStackEditorOpen = $state(false);
  let selectedStackName = $state<string | null>(null);
  let hostStatus = $state<'connected' | 'disconnected' | 'connecting'>('connected');
  let hostVersion = $state('v27.0.3 (Engine)');
  let systemInfo = $state<SystemInfo>({
    Containers: 14,
    ContainersRunning: 11,
    ContainersPaused: 1,
    ContainersStopped: 2,
    Images: 28,
    NCPU: 16,
    MemTotal: 34359738368, // 32 GB
    OperatingSystem: 'Linux 6.8 (x86_64)',
    ServerVersion: 'Docker Engine 27.0.3',
  });

  const defaultVersion = typeof __APP_VERSION__ !== 'undefined' ? __APP_VERSION__ : '0.2.6';
  let appVersion = $state<string>(defaultVersion);

  async function loadTelemetry() {
    try {
      const ping = await systemApi.ping();
      if (ping && (ping.status === 'OK' || ping.status === 'ok')) {
        hostStatus = 'connected';
        if (ping.version) {
          appVersion = ping.version;
        }
      }
      const info = await systemApi.getInfo();
      if (info) {
        systemInfo = info;
        if (info.version || info.app_version || info.pdswarm_version) {
          appVersion = info.version || info.app_version || info.pdswarm_version || appVersion;
        }
        if (info.server_version || info.ServerVersion) {
          hostVersion = info.server_version || info.ServerVersion || 'Engine Ready';
        }
      }
    } catch {
      // Offline or mock mode in development
    }
  }

  onMount(() => {
    if (typeof window !== 'undefined') {
      const savedTheme = localStorage.getItem('pdswarm_theme') || 'obsidian-violet';
      document.documentElement.setAttribute('data-theme', savedTheme);
    }

    (async () => {
      await auth.checkAuth();
      if (!auth.needsSetup && (!auth.authEnabled || auth.isAuthenticated)) {
        await loadTelemetry();
      }
    })();

    const handleEnvChange = () => {
      selectedContainerId = null;
      selectedStackName = null;
      isStackEditorOpen = false;
      loadTelemetry();
    };
    window.addEventListener('pdswarm:environment-changed', handleEnvChange);
    return () => {
      window.removeEventListener('pdswarm:environment-changed', handleEnvChange);
    };
  });

  function handleNavigate(tab: string) {
    activeTab = tab;
    if (tab === 'containers') {
      selectedContainerId = null;
    }
    if (tab === 'stacks') {
      isStackEditorOpen = false;
      selectedStackName = null;
    }
  }

  function handleSelectContainer(id: string) {
    selectedContainerId = id;
    activeTab = 'containers';
  }

  function handleBackToContainers() {
    selectedContainerId = null;
  }

  function handleOpenNewStack() {
    selectedStackName = null;
    isStackEditorOpen = true;
  }

  function handleOpenEditStack(name: string) {
    selectedStackName = name;
    isStackEditorOpen = true;
  }

  function handleNavigateToStack(name: string) {
    selectedContainerId = null;
    activeTab = 'stacks';
    handleOpenEditStack(name);
  }

  function handleCloseStackEditor() {
    isStackEditorOpen = false;
    selectedStackName = null;
  }

  function handleStackDeployed() {
    isStackEditorOpen = false;
    selectedStackName = null;
  }

  async function handleLogout() {
    await auth.logout();
  }
</script>

{#if auth.isLoading && !auth.initialized}
  <div class="min-h-screen flex items-center justify-center bg-[#0a0b10] text-white">
    <div class="flex flex-col items-center gap-3">
      <div class="w-12 h-12 rounded-2xl bg-gradient-to-br from-violet-600 to-indigo-700 text-white shadow-[0_0_25px_rgba(124,58,237,0.5)] flex items-center justify-center border border-violet-400/30">
        <Cpu class="w-6 h-6 animate-pulse" />
      </div>
      <span class="text-xs font-mono text-slate-400 animate-pulse">Initializing PDSwarm...</span>
    </div>
  </div>
{:else if auth.needsSetup}
  <Setup onComplete={() => loadTelemetry()} />
{:else if auth.authEnabled && !auth.isAuthenticated}
  <Login onSuccess={() => loadTelemetry()} />
{:else}
  <Layout
    {activeTab}
    {appVersion}
    user={auth.user}
    {hostStatus}
    {hostVersion}
    onNavigate={handleNavigate}
    onLogout={handleLogout}
  >
  {#key environmentStore.activeId}
    {#if activeTab === 'dashboard'}
      <Dashboard onNavigate={handleNavigate} />
    {:else if activeTab === 'containers'}
      {#if selectedContainerId}
        <ContainerDetail
          containerId={selectedContainerId}
          onBack={handleBackToContainers}
          onDeleted={handleBackToContainers}
          onReplaced={(newId) => selectedContainerId = newId}
          onNavigateToStack={handleNavigateToStack}
        />
      {:else}
        <Containers
          onSelectContainer={handleSelectContainer}
          onNavigate={handleNavigate}
          onNavigateToStack={handleNavigateToStack}
        />
      {/if}
    {:else if activeTab === 'stacks'}
      {#if isStackEditorOpen}
        <StackEditor
          stackName={selectedStackName}
          onClose={handleCloseStackEditor}
          onDeployed={handleStackDeployed}
        />
      {:else}
        <Stacks
          onNewStack={handleOpenNewStack}
          onEditStack={handleOpenEditStack}
          onNavigate={handleNavigate}
          onSelectContainer={handleSelectContainer}
        />
      {/if}
    {:else if activeTab === 'templates'}
      <Templates
        onNavigate={handleNavigate}
        onDeploySuccess={() => {
          handleNavigate('dashboard');
        }}
      />
    {:else if activeTab === 'images'}
      <Images onNavigate={handleNavigate} />
    {:else if activeTab === 'volumes'}
      <Volumes
        onNavigate={handleNavigate}
        onSelectContainer={handleSelectContainer}
      />
    {:else if activeTab === 'networks'}
      <Networks
        onNavigate={handleNavigate}
        onSelectContainer={handleSelectContainer}
      />
    {:else if activeTab === 'settings'}
      <SettingsView onNavigate={handleNavigate} />
    {:else}
      <!-- Secondary View Placeholder -->
      <div class="p-8 rounded-2xl bg-[#12131c] border border-white/[0.08] text-center space-y-4 animate-in fade-in duration-150">
        <div class="w-12 h-12 mx-auto rounded-2xl bg-violet-600/20 border border-violet-500/30 flex items-center justify-center text-violet-400 shadow-[0_0_20px_rgba(124,58,237,0.3)]">
          {#if activeTab === 'containers'}<Box class="w-6 h-6" />
          {:else if activeTab === 'stacks'}<Layers class="w-6 h-6" />
          {:else if activeTab === 'images'}<Disc class="w-6 h-6" />
          {:else if activeTab === 'volumes'}<HardDrive class="w-6 h-6" />
          {:else if activeTab === 'networks'}<Network class="w-6 h-6" />
          {:else}<Settings class="w-6 h-6" />
          {/if}
        </div>

        <div>
          <h2 class="text-xl font-bold text-white capitalize">{activeTab} Management</h2>
          <p class="text-xs text-slate-400 max-w-md mx-auto mt-1">
            Connected to PDSwarm API. The full {activeTab} view will be wired in Task 10-15.
          </p>
        </div>

        <div class="pt-2">
          <button
            onclick={() => activeTab = 'dashboard'}
            class="px-4 py-2 rounded-xl bg-violet-600 hover:bg-violet-500 text-white text-xs font-semibold shadow-[0_0_15px_rgba(124,58,237,0.35)] transition"
          >
            &larr; Back to Dashboard
          </button>
        </div>
      </div>
    {/if}
  {/key}
</Layout>
{/if}
