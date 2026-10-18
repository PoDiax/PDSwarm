<script lang="ts">
  import { onMount } from 'svelte';
  import {
    Key,
    Plus,
    RefreshCw,
    Trash2,
    Edit3,
    CheckCircle2,
    AlertCircle,
    Loader2,
    X,
    ExternalLink,
    Globe,
    Lock,
    Shield,
    Sparkles,
    Check,
  } from '@lucide/svelte';
  import { registryApi } from '../api';
  import { portal } from '../actions/portal';
  import type { Registry, CreateRegistryRequest, UpdateRegistryRequest } from '../types';

  interface Props {
    isOpen?: boolean;
    isModal?: boolean;
    onClose?: () => void;
    onChanged?: () => void;
  }

  let {
    isOpen = true,
    isModal = false,
    onClose,
    onChanged,
  }: Props = $props();

  let registries = $state<Registry[]>([]);
  let isLoading = $state(true);
  let isRefreshing = $state(false);
  let feedback = $state<{ type: 'success' | 'error'; message: string } | null>(null);

  // Add / Edit Modal state
  let isFormOpen = $state(false);
  let editingRegistry = $state<Registry | null>(null);
  let selectedPreset = $state<'dockerhub' | 'ghcr' | 'quay' | 'custom'>('dockerhub');
  let formName = $state('');
  let formURL = $state('docker.io');
  let formUsername = $state('');
  let formPassword = $state('');
  let isTesting = $state(false);
  let testResult = $state<{ success: boolean; message: string } | null>(null);
  let isSaving = $state(false);
  let formError = $state<string | null>(null);

  // Delete modal state
  let deleteTarget = $state<Registry | null>(null);
  let isDeleting = $state(false);

  function showFeedback(type: 'success' | 'error', message: string) {
    feedback = { type, message };
    setTimeout(() => {
      if (feedback?.message === message) feedback = null;
    }, 4000);
  }

  async function loadRegistries(silent = false) {
    if (!silent) isLoading = true;
    else isRefreshing = true;
    try {
      const data = await registryApi.list();
      registries = data || [];
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to load registries';
      showFeedback('error', msg);
    } finally {
      isLoading = false;
      isRefreshing = false;
    }
  }

  onMount(() => {
    loadRegistries();
  });

  function selectPreset(preset: 'dockerhub' | 'ghcr' | 'quay' | 'custom') {
    selectedPreset = preset;
    testResult = null;
    formError = null;

    if (preset === 'dockerhub') {
      if (!editingRegistry || formURL === 'ghcr.io' || formURL === 'quay.io') {
        formURL = 'docker.io';
        if (!formName || formName === 'GitHub Container Registry' || formName === 'Quay.io') {
          formName = 'Docker Hub';
        }
      }
    } else if (preset === 'ghcr') {
      if (!editingRegistry || formURL === 'docker.io' || formURL === 'quay.io') {
        formURL = 'ghcr.io';
        if (!formName || formName === 'Docker Hub' || formName === 'Quay.io') {
          formName = 'GitHub Container Registry';
        }
      }
    } else if (preset === 'quay') {
      if (!editingRegistry || formURL === 'docker.io' || formURL === 'ghcr.io') {
        formURL = 'quay.io';
        if (!formName || formName === 'Docker Hub' || formName === 'GitHub Container Registry') {
          formName = 'Quay.io';
        }
      }
    } else {
      if (!editingRegistry && (formURL === 'docker.io' || formURL === 'ghcr.io' || formURL === 'quay.io')) {
        formURL = '';
        formName = '';
      }
    }
  }

  function openCreateModal() {
    editingRegistry = null;
    selectedPreset = 'dockerhub';
    formName = 'Docker Hub';
    formURL = 'docker.io';
    formUsername = '';
    formPassword = '';
    testResult = null;
    formError = null;
    isFormOpen = true;
  }

  function openEditModal(reg: Registry) {
    editingRegistry = reg;
    formName = reg.name;
    formURL = reg.url;
    formUsername = reg.username;
    formPassword = ''; // empty means keep existing
    testResult = null;
    formError = null;

    const lower = reg.url.toLowerCase();
    if (lower.includes('docker.io')) selectedPreset = 'dockerhub';
    else if (lower.includes('ghcr.io')) selectedPreset = 'ghcr';
    else if (lower.includes('quay.io')) selectedPreset = 'quay';
    else selectedPreset = 'custom';

    isFormOpen = true;
  }

  function closeFormModal() {
    isFormOpen = false;
    editingRegistry = null;
    testResult = null;
    formError = null;
  }

  async function handleTestConnection() {
    testResult = null;
    formError = null;

    const targetUrl = formURL.trim();
    const targetUser = formUsername.trim();
    if (!targetUrl) {
      formError = 'Registry URL is required to test connection.';
      return;
    }
    if (!targetUser) {
      formError = 'Username is required to test connection.';
      return;
    }
    if (!formPassword && !editingRegistry) {
      formError = 'Password or Access Token is required.';
      return;
    }

    isTesting = true;
    try {
      const res = await registryApi.test({
        id: editingRegistry?.id,
        url: targetUrl,
        username: targetUser,
        password: formPassword,
      });

      if (res.status === 'success' || (res.status && !res.error)) {
        testResult = {
          success: true,
          message: res.message || 'Authentication verified successfully!',
        };
      } else {
        testResult = {
          success: false,
          message: res.message || res.error || 'Connection failed.',
        };
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Connection test failed';
      testResult = {
        success: false,
        message: msg,
      };
    } finally {
      isTesting = false;
    }
  }

  async function handleSaveRegistry(e: SubmitEvent) {
    e.preventDefault();
    formError = null;

    const name = formName.trim();
    const url = formURL.trim();
    const username = formUsername.trim();
    const password = formPassword;

    if (!name) {
      formError = 'Registry display name is required.';
      return;
    }
    if (!url) {
      formError = 'Registry URL is required.';
      return;
    }
    if (!username) {
      formError = 'Username is required.';
      return;
    }
    if (!editingRegistry && !password) {
      formError = 'Password or Access Token is required.';
      return;
    }

    isSaving = true;
    try {
      if (editingRegistry) {
        const updateData: UpdateRegistryRequest = {
          name,
          url,
          username,
        };
        if (password) {
          updateData.password = password;
        }
        await registryApi.update(editingRegistry.id, updateData);
        showFeedback('success', `Registry "${name}" updated successfully.`);
      } else {
        const createData: CreateRegistryRequest = {
          name,
          url,
          username,
          password,
        };
        await registryApi.create(createData);
        showFeedback('success', `Registry "${name}" configured successfully.`);
      }

      closeFormModal();
      await loadRegistries(true);
      if (onChanged) onChanged();
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to save registry.';
      formError = msg;
    } finally {
      isSaving = false;
    }
  }

  async function handleDeleteConfirm() {
    if (!deleteTarget) return;
    isDeleting = true;
    try {
      await registryApi.delete(deleteTarget.id);
      showFeedback('success', `Registry "${deleteTarget.name}" deleted.`);
      deleteTarget = null;
      await loadRegistries(true);
      if (onChanged) onChanged();
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to delete registry';
      showFeedback('error', msg);
    } finally {
      isDeleting = false;
    }
  }

  function getProviderMeta(url: string) {
    const lower = url.toLowerCase();
    if (lower.includes('docker.io')) {
      return {
        label: 'Docker Hub',
        badgeClass: 'bg-cyan-500/15 text-cyan-300 border-cyan-500/30',
        dotClass: 'bg-cyan-400 shadow-[0_0_8px_#06b6d4]',
      };
    }
    if (lower.includes('ghcr.io')) {
      return {
        label: 'GitHub GHCR',
        badgeClass: 'bg-violet-500/15 text-violet-300 border-violet-500/30',
        dotClass: 'bg-violet-400 shadow-[0_0_8px_#8b5cf6]',
      };
    }
    if (lower.includes('quay.io')) {
      return {
        label: 'Quay.io',
        badgeClass: 'bg-rose-500/15 text-rose-300 border-rose-500/30',
        dotClass: 'bg-rose-400 shadow-[0_0_8px_#f43f5e]',
      };
    }
    return {
      label: 'Custom Registry',
      badgeClass: 'bg-emerald-500/15 text-emerald-300 border-emerald-500/30',
      dotClass: 'bg-emerald-400 shadow-[0_0_8px_#10b981]',
    };
  }
</script>

{#snippet content()}
  <!-- Component Body -->
  <div class="space-y-6">
    <!-- Header Controls -->
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
      <div class="flex items-center gap-3">
        <div class="w-10 h-10 rounded-2xl bg-gradient-to-br from-violet-600/30 to-indigo-700/20 border border-violet-500/40 flex items-center justify-center text-violet-400 shadow-[0_0_20px_rgba(124,58,237,0.3)]">
          <Key class="w-5 h-5" />
        </div>
        <div>
          <div class="flex items-center gap-2">
            <h2 class="text-lg font-bold tracking-tight text-white">Private Registries</h2>
            <span class="px-2 py-0.5 text-[10px] font-mono rounded-full bg-violet-500/20 text-violet-300 border border-violet-500/30 font-semibold">
              {registries.length}
            </span>
          </div>
          <p class="text-xs text-slate-400">
            Securely authenticate pulls and deployments from Docker Hub, GHCR, Quay, and private Harbor instances
          </p>
        </div>
      </div>

      <div class="flex items-center gap-2.5">
        <button
          onclick={() => loadRegistries(true)}
          disabled={isRefreshing}
          class="px-3 py-2 rounded-xl bg-white/[0.04] hover:bg-white/[0.08] text-slate-300 hover:text-white border border-white/[0.08] text-xs font-medium transition flex items-center gap-1.5"
          title="Refresh Registries"
        >
          <RefreshCw class="w-3.5 h-3.5 {isRefreshing ? 'animate-spin text-cyan-400' : ''}" />
          <span>Refresh</span>
        </button>

        <button
          onclick={openCreateModal}
          class="px-3.5 py-2 rounded-xl bg-gradient-to-r from-violet-600 to-indigo-600 hover:from-violet-500 hover:to-indigo-500 text-white text-xs font-semibold shadow-[0_0_20px_rgba(124,58,237,0.35)] transition flex items-center gap-1.5"
        >
          <Plus class="w-4 h-4" />
          <span>Add Registry</span>
        </button>
      </div>
    </div>

    <!-- Alert / Feedback Notification -->
    {#if feedback}
      <div
        class="p-3.5 rounded-xl text-xs flex items-center justify-between border animate-in fade-in duration-200 {feedback.type === 'success'
          ? 'bg-emerald-500/10 border-emerald-500/25 text-emerald-300'
          : 'bg-rose-500/10 border-rose-500/25 text-rose-300'}"
      >
        <div class="flex items-center gap-2.5">
          {#if feedback.type === 'success'}
            <CheckCircle2 class="w-4 h-4 shrink-0 text-emerald-400" />
          {:else}
            <AlertCircle class="w-4 h-4 shrink-0 text-rose-400" />
          {/if}
          <span>{feedback.message}</span>
        </div>
        <button
          onclick={() => feedback = null}
          class="text-slate-400 hover:text-white"
        >
          <X class="w-4 h-4" />
        </button>
      </div>
    {/if}

    <!-- Registries Table / Empty State -->
    {#if isLoading}
      <div class="p-12 text-center rounded-2xl bg-[#12131c]/60 border border-white/[0.08]">
        <Loader2 class="w-8 h-8 animate-spin mx-auto text-violet-400 mb-3" />
        <span class="text-xs text-slate-400 font-mono">Loading configured registries...</span>
      </div>
    {:else if registries.length === 0}
      <div class="p-12 text-center rounded-2xl bg-[#12131c]/60 border border-white/[0.08] border-dashed">
        <div class="w-12 h-12 rounded-2xl bg-violet-600/10 border border-violet-500/20 text-violet-400 flex items-center justify-center mx-auto mb-3 shadow-[0_0_25px_rgba(124,58,237,0.15)]">
          <Key class="w-6 h-6" />
        </div>
        <h3 class="text-sm font-semibold text-white">No Private Registries Configured</h3>
        <p class="text-xs text-slate-400 max-w-md mx-auto mt-1 mb-5">
          Images will be pulled anonymously from public Docker Hub. Add credentials for GHCR, Quay, or your private repository to enable authenticated pulls.
        </p>
        <button
          onclick={openCreateModal}
          class="px-4 py-2 rounded-xl bg-violet-600 hover:bg-violet-500 text-white text-xs font-semibold shadow-[0_0_15px_rgba(124,58,237,0.3)] transition inline-flex items-center gap-2"
        >
          <Plus class="w-4 h-4" />
          <span>Add First Registry</span>
        </button>
      </div>
    {:else}
      <div class="rounded-2xl bg-[#12131c] border border-white/[0.08] overflow-hidden shadow-xl">
        <div class="overflow-x-auto">
          <table class="w-full text-left text-xs">
            <thead>
              <tr class="border-b border-white/[0.08] bg-white/[0.02] text-slate-400 font-mono uppercase text-[10px] tracking-wider">
                <th class="py-3 px-4">Provider</th>
                <th class="py-3 px-4">Registry Name</th>
                <th class="py-3 px-4">Registry URL</th>
                <th class="py-3 px-4">Username / Identity</th>
                <th class="py-3 px-4">Auth Status</th>
                <th class="py-3 px-4 text-right">Actions</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-white/[0.04]">
              {#each registries as reg (reg.id)}
                {@const meta = getProviderMeta(reg.url)}
                <tr class="hover:bg-white/[0.02] transition-colors group">
                  <td class="py-3.5 px-4 whitespace-nowrap">
                    <span class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[10px] font-mono border {meta.badgeClass}">
                      <span class="w-1.5 h-1.5 rounded-full {meta.dotClass}"></span>
                      {meta.label}
                    </span>
                  </td>
                  <td class="py-3.5 px-4 whitespace-nowrap">
                    <span class="font-medium text-white group-hover:text-violet-300 transition-colors">
                      {reg.name}
                    </span>
                  </td>
                  <td class="py-3.5 px-4 whitespace-nowrap">
                    <span class="font-mono text-cyan-300 text-[11px] bg-cyan-950/30 px-2 py-0.5 rounded border border-cyan-800/30">
                      {reg.url}
                    </span>
                  </td>
                  <td class="py-3.5 px-4 whitespace-nowrap text-slate-300">
                    <div class="flex items-center gap-1.5">
                      <Lock class="w-3 h-3 text-slate-500" />
                      <span>{reg.username}</span>
                    </div>
                  </td>
                  <td class="py-3.5 px-4 whitespace-nowrap">
                    <span class="inline-flex items-center gap-1 text-[11px] text-emerald-400 font-mono">
                      <CheckCircle2 class="w-3.5 h-3.5" />
                      Token Saved
                    </span>
                  </td>
                  <td class="py-3.5 px-4 whitespace-nowrap text-right">
                    <div class="inline-flex items-center gap-1">
                      <button
                        onclick={() => openEditModal(reg)}
                        class="p-1.5 rounded-lg text-slate-400 hover:text-white hover:bg-white/[0.06] transition"
                        title="Edit Registry"
                      >
                        <Edit3 class="w-3.5 h-3.5" />
                      </button>
                      <button
                        onclick={() => deleteTarget = reg}
                        class="p-1.5 rounded-lg text-slate-400 hover:text-rose-400 hover:bg-rose-500/10 transition"
                        title="Delete Registry"
                      >
                        <Trash2 class="w-3.5 h-3.5" />
                      </button>
                    </div>
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      </div>
    {/if}
  </div>
{/snippet}

<!-- Inline view or Modal wrapper -->
{#if isModal}
  {#if isOpen}
    <div use:portal class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-md animate-in fade-in duration-200">
      <div class="relative w-full max-w-4xl max-h-[90vh] overflow-y-auto rounded-3xl bg-[#0a0b10] border border-white/[0.1] shadow-2xl p-6 md:p-8">
        <button
          onclick={onClose}
          class="absolute top-5 right-5 p-2 rounded-xl text-slate-400 hover:text-white hover:bg-white/[0.06] transition"
        >
          <X class="w-5 h-5" />
        </button>
        {@render content()}
      </div>
    </div>
  {/if}
{:else}
  {@render content()}
{/if}

<!-- Add / Edit Registry Modal -->
{#if isFormOpen}
  <div use:portal class="fixed inset-0 z-[60] flex items-center justify-center p-4 bg-black/80 backdrop-blur-md animate-in fade-in duration-200">
    <div class="relative w-full max-w-lg rounded-3xl bg-[#12131c] border border-white/[0.1] shadow-2xl p-6 md:p-7 space-y-5">
      <!-- Modal Header -->
      <div class="flex items-center justify-between border-b border-white/[0.08] pb-4">
        <div class="flex items-center gap-2.5">
          <div class="w-9 h-9 rounded-xl bg-violet-600/20 border border-violet-500/30 text-violet-400 flex items-center justify-center shadow-[0_0_15px_rgba(124,58,237,0.25)]">
            <Key class="w-4 h-4" />
          </div>
          <div>
            <h3 class="text-base font-bold text-white">
              {editingRegistry ? 'Edit Registry Credentials' : 'Add Private Registry'}
            </h3>
            <p class="text-[11px] text-slate-400">
              Configure credentials for image pull authentication
            </p>
          </div>
        </div>
        <button
          onclick={closeFormModal}
          class="p-1.5 rounded-lg text-slate-400 hover:text-white hover:bg-white/[0.06] transition"
        >
          <X class="w-4 h-4" />
        </button>
      </div>

      <!-- Presets Selector -->
      <div>
        <span class="block text-[11px] font-mono uppercase text-slate-400 mb-2">
          Provider Preset
        </span>
        <div class="grid grid-cols-2 sm:grid-cols-4 gap-2">
          <button
            type="button"
            onclick={() => selectPreset('dockerhub')}
            class="px-2.5 py-2 rounded-xl border text-xs font-medium text-center transition flex flex-col items-center gap-1 {selectedPreset === 'dockerhub'
              ? 'bg-cyan-500/15 border-cyan-400/50 text-cyan-300 shadow-[0_0_12px_rgba(6,182,212,0.2)]'
              : 'bg-white/[0.02] border-white/[0.08] text-slate-400 hover:text-slate-200'}"
          >
            <span class="text-[11px] font-bold">Docker Hub</span>
            <span class="text-[9px] font-mono text-slate-500">docker.io</span>
          </button>

          <button
            type="button"
            onclick={() => selectPreset('ghcr')}
            class="px-2.5 py-2 rounded-xl border text-xs font-medium text-center transition flex flex-col items-center gap-1 {selectedPreset === 'ghcr'
              ? 'bg-violet-500/15 border-violet-400/50 text-violet-300 shadow-[0_0_12px_rgba(124,58,237,0.2)]'
              : 'bg-white/[0.02] border-white/[0.08] text-slate-400 hover:text-slate-200'}"
          >
            <span class="text-[11px] font-bold">GitHub GHCR</span>
            <span class="text-[9px] font-mono text-slate-500">ghcr.io</span>
          </button>

          <button
            type="button"
            onclick={() => selectPreset('quay')}
            class="px-2.5 py-2 rounded-xl border text-xs font-medium text-center transition flex flex-col items-center gap-1 {selectedPreset === 'quay'
              ? 'bg-rose-500/15 border-rose-400/50 text-rose-300 shadow-[0_0_12px_rgba(244,63,94,0.2)]'
              : 'bg-white/[0.02] border-white/[0.08] text-slate-400 hover:text-slate-200'}"
          >
            <span class="text-[11px] font-bold">Quay.io</span>
            <span class="text-[9px] font-mono text-slate-500">quay.io</span>
          </button>

          <button
            type="button"
            onclick={() => selectPreset('custom')}
            class="px-2.5 py-2 rounded-xl border text-xs font-medium text-center transition flex flex-col items-center gap-1 {selectedPreset === 'custom'
              ? 'bg-emerald-500/15 border-emerald-400/50 text-emerald-300 shadow-[0_0_12px_rgba(16,185,129,0.2)]'
              : 'bg-white/[0.02] border-white/[0.08] text-slate-400 hover:text-slate-200'}"
          >
            <span class="text-[11px] font-bold">Custom</span>
            <span class="text-[9px] font-mono text-slate-500">Harbor / Private</span>
          </button>
        </div>
      </div>

      <form onsubmit={handleSaveRegistry} class="space-y-4">
        <!-- Form Fields -->
        <div>
          <label class="block text-[11px] font-mono uppercase text-slate-400 mb-1.5" for="reg-name">
            Display Name
          </label>
          <input
            id="reg-name"
            type="text"
            bind:value={formName}
            placeholder="e.g. My Organization GHCR"
            required
            class="w-full px-3.5 py-2.5 rounded-xl bg-black/40 border border-white/[0.08] focus:border-violet-500 focus:outline-none text-white text-xs placeholder:text-slate-600 transition"
          />
        </div>

        <div>
          <label class="block text-[11px] font-mono uppercase text-slate-400 mb-1.5" for="reg-url">
            Registry URL / Host
          </label>
          <input
            id="reg-url"
            type="text"
            bind:value={formURL}
            placeholder="e.g. ghcr.io or harbor.mycompany.com:8443"
            required
            class="w-full px-3.5 py-2.5 rounded-xl bg-black/40 border border-white/[0.08] focus:border-violet-500 focus:outline-none text-white font-mono text-xs placeholder:text-slate-600 transition"
          />
        </div>

        <div>
          <label class="block text-[11px] font-mono uppercase text-slate-400 mb-1.5" for="reg-user">
            Username / Robot Account
          </label>
          <input
            id="reg-user"
            type="text"
            bind:value={formUsername}
            placeholder="e.g. octocat or robot$mycompany"
            required
            class="w-full px-3.5 py-2.5 rounded-xl bg-black/40 border border-white/[0.08] focus:border-violet-500 focus:outline-none text-white text-xs placeholder:text-slate-600 transition"
          />
        </div>

        <div>
          <div class="flex items-center justify-between mb-1.5">
            <label class="block text-[11px] font-mono uppercase text-slate-400" for="reg-pass">
              Password / Access Token
            </label>
            {#if editingRegistry}
              <span class="text-[10px] text-slate-500 font-mono">Leave blank to keep existing</span>
            {/if}
          </div>
          <input
            id="reg-pass"
            type="password"
            bind:value={formPassword}
            placeholder={editingRegistry ? '•••••••••••••••• (Unchanged)' : 'Enter token or password'}
            required={!editingRegistry}
            class="w-full px-3.5 py-2.5 rounded-xl bg-black/40 border border-white/[0.08] focus:border-violet-500 focus:outline-none text-white font-mono text-xs placeholder:text-slate-600 transition"
          />
        </div>

        <!-- Connection Test Result -->
        {#if testResult}
          <div
            class="p-3 rounded-xl text-xs flex items-center gap-2.5 border animate-in fade-in duration-150 {testResult.success
              ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-300'
              : 'bg-rose-500/10 border-rose-500/30 text-rose-300'}"
          >
            {#if testResult.success}
              <CheckCircle2 class="w-4 h-4 shrink-0 text-emerald-400" />
            {:else}
              <AlertCircle class="w-4 h-4 shrink-0 text-rose-400" />
            {/if}
            <span class="break-all">{testResult.message}</span>
          </div>
        {/if}

        {#if formError}
          <div class="p-3 rounded-xl bg-rose-500/10 border border-rose-500/25 text-rose-300 text-xs flex items-center gap-2">
            <AlertCircle class="w-4 h-4 shrink-0 text-rose-400" />
            <span>{formError}</span>
          </div>
        {/if}

        <!-- Actions -->
        <div class="pt-3 border-t border-white/[0.08] flex items-center justify-between gap-3">
          <button
            type="button"
            onclick={handleTestConnection}
            disabled={isTesting || isSaving}
            class="px-3.5 py-2 rounded-xl bg-white/[0.04] hover:bg-white/[0.08] text-slate-300 hover:text-white border border-white/[0.08] text-xs font-medium transition flex items-center gap-1.5"
          >
            {#if isTesting}
              <Loader2 class="w-3.5 h-3.5 animate-spin text-cyan-400" />
              <span>Verifying...</span>
            {:else}
              <span>Test Connection</span>
            {/if}
          </button>

          <div class="flex items-center gap-2">
            <button
              type="button"
              onclick={closeFormModal}
              disabled={isSaving}
              class="px-3.5 py-2 rounded-xl text-slate-400 hover:text-white text-xs font-medium transition"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={isSaving}
              class="px-4 py-2 rounded-xl bg-gradient-to-r from-violet-600 to-indigo-600 hover:from-violet-500 hover:to-indigo-500 text-white text-xs font-semibold shadow-[0_0_15px_rgba(124,58,237,0.35)] transition flex items-center gap-1.5"
            >
              {#if isSaving}
                <Loader2 class="w-3.5 h-3.5 animate-spin" />
                <span>Saving...</span>
              {:else}
                <Check class="w-3.5 h-3.5" />
                <span>Save Registry</span>
              {/if}
            </button>
          </div>
        </div>
      </form>
    </div>
  </div>
{/if}

<!-- Delete Confirmation Modal -->
{#if deleteTarget}
  <div use:portal class="fixed inset-0 z-[70] flex items-center justify-center p-4 bg-black/80 backdrop-blur-md animate-in fade-in duration-150">
    <div class="w-full max-w-sm rounded-3xl bg-[#12131c] border border-rose-500/20 shadow-2xl p-6 text-center space-y-4">
      <div class="w-12 h-12 rounded-2xl bg-rose-500/10 border border-rose-500/20 text-rose-400 flex items-center justify-center mx-auto shadow-[0_0_20px_rgba(244,63,94,0.2)]">
        <Trash2 class="w-6 h-6" />
      </div>
      <div>
        <h4 class="text-sm font-bold text-white">Delete Registry Credentials?</h4>
        <p class="text-xs text-slate-400 mt-1">
          Are you sure you want to delete <span class="text-white font-medium">"{deleteTarget.name}"</span> ({deleteTarget.url})? Subsequent private image pulls from this registry will fail unless public.
        </p>
      </div>
      <div class="flex items-center justify-center gap-2.5 pt-2">
        <button
          onclick={() => deleteTarget = null}
          disabled={isDeleting}
          class="px-4 py-2 rounded-xl text-slate-300 hover:text-white bg-white/[0.04] text-xs font-medium transition"
        >
          Cancel
        </button>
        <button
          onclick={handleDeleteConfirm}
          disabled={isDeleting}
          class="px-4 py-2 rounded-xl bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold shadow-[0_0_15px_rgba(244,63,94,0.35)] transition flex items-center gap-1.5"
        >
          {#if isDeleting}
            <Loader2 class="w-3.5 h-3.5 animate-spin" />
            <span>Deleting...</span>
          {:else}
            <span>Delete Registry</span>
          {/if}
        </button>
      </div>
    </div>
  </div>
{/if}
