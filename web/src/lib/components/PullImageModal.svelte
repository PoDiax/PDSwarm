<script lang="ts">
  import {
    X,
    Disc,
    Download,
    CheckCircle2,
    AlertCircle,
    Loader2,
    Layers,
    Terminal,
    Sparkles,
    Trash2,
    Key,
    Minimize2,
  } from '@lucide/svelte';
  import { imageApi, registryApi } from '../api';
  import { tasksStore } from '../stores/tasks.svelte';
  import type { DockerPullProgress, Registry } from '../types';

  interface Props {
    isOpen: boolean;
    onClose: () => void;
    onPulled?: (imageRef: string) => void;
    initialRef?: string;
  }

  let {
    isOpen = false,
    onClose,
    onPulled,
    initialRef = '',
  }: Props = $props();

  // Form states
  let imageRef = $state('');
  let isPulling = $state(false);
  let isSuccess = $state(false);
  let error = $state<string | null>(null);

  // Registry authentication
  let configuredRegistries = $state<Registry[]>([]);
  let selectedRegistryId = $state<string>('');

  $effect(() => {
    if (isOpen) {
      if (initialRef && !imageRef) {
        imageRef = initialRef;
      }
      registryApi.list().then((list) => {
        configuredRegistries = list || [];
      }).catch(() => {
        configuredRegistries = [];
      });
    }
  });

  // Streaming layer progress
  interface LayerProgress {
    id: string;
    status: string;
    current: number;
    total: number;
    percent: number;
    textProgress?: string;
  }

  let layers = $state<Record<string, LayerProgress>>({});
  let logMessages = $state<string[]>([]);
  let abortController: AbortController | null = null;
  let logsContainer = $state<HTMLDivElement | null>(null);

  const popularPresets = [
    { label: 'nginx:alpine', ref: 'nginx:alpine' },
    { label: 'redis:alpine', ref: 'redis:alpine' },
    { label: 'postgres:16-alpine', ref: 'postgres:16-alpine' },
    { label: 'node:20-alpine', ref: 'node:20-alpine' },
    { label: 'busybox:latest', ref: 'busybox:latest' },
    { label: 'python:3.12-slim', ref: 'python:3.12-slim' },
  ];

  function resetState() {
    imageRef = '';
    selectedRegistryId = '';
    isPulling = false;
    isSuccess = false;
    error = null;
    layers = {};
    logMessages = [];
    if (abortController) {
      abortController.abort();
      abortController = null;
    }
  }

  function formatBytes(bytes: number): string {
    if (!bytes || bytes <= 0) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(1024));
    return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`;
  }

  function selectPreset(ref: string) {
    if (isPulling) return;
    imageRef = ref;
  }

  function handleProgress(event: DockerPullProgress) {
    if (event.id) {
      const cur = event.progressDetail?.current || 0;
      const tot = event.progressDetail?.total || 0;
      const pct = tot > 0 ? Math.min(100, Math.round((cur / tot) * 100)) : 0;

      layers = {
        ...layers,
        [event.id]: {
          id: event.id,
          status: event.status,
          current: cur,
          total: tot,
          percent: pct,
          textProgress: event.progress,
        },
      };
    }

    if (event.status) {
      const line = event.id ? `[${event.id}] ${event.status}` : event.status;
      logMessages = [...logMessages.slice(-50), line];
      
      // Auto-scroll
      if (logsContainer) {
        setTimeout(() => {
          if (logsContainer) {
            logsContainer.scrollTop = logsContainer.scrollHeight;
          }
        }, 10);
      }
    }
  }

  async function handlePull(e: SubmitEvent) {
    e.preventDefault();
    const target = imageRef.trim();
    if (!target) {
      error = 'Please enter a valid Docker image reference (e.g. "redis:alpine").';
      return;
    }

    isPulling = true;
    isSuccess = false;
    error = null;
    layers = {};
    logMessages = [`Initiating pull for ${target}...`];

    abortController = new AbortController();

    try {
      await imageApi.pullStream(target, handleProgress, abortController.signal, selectedRegistryId || undefined);
      isSuccess = true;
      logMessages = [...logMessages, `Successfully pulled ${target}`];
      if (onPulled) {
        onPulled(target);
      }
    } catch (err: unknown) {
      if (err instanceof Error) {
        if (err.name === 'AbortError') {
          logMessages = [...logMessages, 'Pull cancelled by user.'];
        } else {
          error = err.message;
          logMessages = [...logMessages, `Error: ${err.message}`];
        }
      } else {
        error = 'Failed to pull image. Please verify registry connection and image tag.';
      }
    } finally {
      isPulling = false;
      abortController = null;
    }
  }

  function handleCancel() {
    if (abortController) {
      abortController.abort();
      abortController = null;
    }
    isPulling = false;
  }

  function runInBackground() {
    // Disconnect browser SSE/stream without aborting backend daemon worker
    tasksStore.poll();
    resetState();
    onClose();
  }

  function handleClose() {
    if (isPulling) {
      runInBackground();
      return;
    }
    resetState();
    onClose();
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape' && isOpen) {
      handleClose();
    }
  }

  let layerList = $derived(Object.values(layers));
</script>

<svelte:window onkeydown={handleKeydown} />

{#if isOpen}
  <div
    class="fixed inset-0 z-50 flex items-center justify-center p-4"
    role="dialog"
    aria-modal="true"
    aria-labelledby="pull-modal-title"
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
      class="relative w-full max-w-2xl max-h-[90vh] flex flex-col rounded-2xl bg-[#12131c] border border-white/[0.12] shadow-[0_10px_50px_rgba(0,0,0,0.8)] overflow-hidden animate-in fade-in zoom-in-95 duration-150"
    >
      <!-- Modal Header -->
      <div class="px-6 py-4 flex items-center justify-between border-b border-white/[0.08] bg-[#0c0d14]/80">
        <div class="flex items-center gap-3">
          <div class="w-8 h-8 rounded-xl bg-violet-600/20 border border-violet-500/30 flex items-center justify-center text-violet-400 shadow-[0_0_15px_rgba(124,58,237,0.3)]">
            <Download class="w-4 h-4" />
          </div>
          <div>
            <h2 id="pull-modal-title" class="text-sm font-bold text-white tracking-wide">Pull Docker Image</h2>
            <p class="text-[11px] text-slate-400">Fetch images from Docker Hub or custom container registries</p>
          </div>
        </div>

        <button
          type="button"
          onclick={handleClose}
          class="p-1.5 rounded-lg hover:bg-white/10 text-slate-400 hover:text-white transition"
          aria-label="Close modal"
          title={isPulling ? 'Run in background' : 'Close'}
        >
          {#if isPulling}
            <Minimize2 class="w-4 h-4 text-cyan-400" />
          {:else}
            <X class="w-4 h-4" />
          {/if}
        </button>
      </div>

      <!-- Modal Body -->
      <div class="flex-1 overflow-y-auto p-6 space-y-5 text-xs">
        {#if error}
          <div class="p-3 rounded-xl bg-rose-500/10 border border-rose-500/30 flex items-start gap-2.5 text-rose-300 animate-in fade-in">
            <AlertCircle class="w-4 h-4 shrink-0 mt-0.5 text-rose-400" />
            <div class="flex-1 leading-relaxed">
              <span class="font-semibold block">Pull Error</span>
              <span class="text-[11px]">{error}</span>
            </div>
          </div>
        {/if}

        {#if isSuccess}
          <div class="p-3 rounded-xl bg-emerald-500/10 border border-emerald-500/30 flex items-center justify-between gap-2.5 text-emerald-300 animate-in fade-in">
            <div class="flex items-center gap-2">
              <CheckCircle2 class="w-4 h-4 shrink-0 text-emerald-400" />
              <span class="text-xs font-semibold">Image pulled successfully and ready to use!</span>
            </div>
            <button
              type="button"
              onclick={handleClose}
              class="px-3 py-1 rounded-lg bg-emerald-500/20 hover:bg-emerald-500/30 text-emerald-300 text-[11px] font-medium border border-emerald-500/30 transition"
            >
              Done
            </button>
          </div>
        {/if}

        <!-- Pull Form -->
        <form onsubmit={handlePull} class="space-y-4">
          <!-- Registry Selector Dropdown -->
          <div class="space-y-1.5">
            <div class="flex items-center justify-between">
              <label for="registry-selector" class="block font-medium text-slate-300">
                Registry Credentials
              </label>
              {#if selectedRegistryId}
                <span class="text-[10px] text-violet-300 font-mono">Authenticated</span>
              {:else}
                <span class="text-[10px] text-slate-500 font-mono">Public / Anonymous</span>
              {/if}
            </div>
            <div class="relative">
              <select
                id="registry-selector"
                bind:value={selectedRegistryId}
                disabled={isPulling}
                class="w-full px-3.5 py-2.5 rounded-xl bg-[#0c0d14] border border-white/10 text-white font-mono text-xs focus:outline-none focus:border-violet-500 focus:ring-1 focus:ring-violet-500 transition disabled:opacity-50 appearance-none cursor-pointer"
              >
                <option value="">Docker Hub (Anonymous / Public)</option>
                {#each configuredRegistries as reg (reg.id)}
                  <option value={reg.id}>{reg.name} &bull; {reg.url} ({reg.username})</option>
                {/each}
              </select>
              <div class="absolute inset-y-0 right-3 flex items-center pointer-events-none text-slate-400">
                <Key class="w-3.5 h-3.5" />
              </div>
            </div>
          </div>

          <div class="space-y-1.5">
            <label for="image-ref-input" class="block font-medium text-slate-300">
              Image Reference <span class="text-violet-400">*</span>
            </label>
            <div class="relative">
              <input
                id="image-ref-input"
                type="text"
                bind:value={imageRef}
                disabled={isPulling}
                placeholder="e.g. redis:alpine, postgres:16-alpine, ghcr.io/owner/image:tag"
                required
                class="w-full px-3.5 py-2.5 rounded-xl bg-[#0c0d14] border border-white/10 text-white placeholder-slate-500 focus:outline-none focus:border-violet-500 focus:ring-1 focus:ring-violet-500 font-mono text-xs transition disabled:opacity-50"
              />
              {#if imageRef && !isPulling}
                <button
                  type="button"
                  onclick={() => imageRef = ''}
                  class="absolute right-3 top-1/2 -translate-y-1/2 text-slate-500 hover:text-white"
                  aria-label="Clear image input"
                >
                  <X class="w-3.5 h-3.5" />
                </button>
              {/if}
            </div>
          </div>

          <!-- Quick Presets -->
          <div class="space-y-1.5">
            <span class="text-[11px] font-medium text-slate-400 flex items-center gap-1">
              <Sparkles class="w-3 h-3 text-cyan-400" />
              Quick Presets
            </span>
            <div class="flex flex-wrap gap-1.5">
              {#each popularPresets as preset}
                <button
                  type="button"
                  disabled={isPulling}
                  onclick={() => selectPreset(preset.ref)}
                  class="px-2.5 py-1 rounded-lg bg-white/[0.04] hover:bg-white/[0.08] border border-white/[0.08] text-slate-300 hover:text-white text-[11px] font-mono transition disabled:opacity-50 {imageRef === preset.ref ? 'border-violet-500/50 bg-violet-500/10 text-violet-300' : ''}"
                >
                  {preset.label}
                </button>
              {/each}
            </div>
          </div>

          <!-- Submit / Action row -->
          <div class="flex items-center justify-end gap-2 pt-2">
            {#if isPulling}
              <button
                type="button"
                onclick={handleCancel}
                class="px-4 py-2 rounded-xl bg-white/5 hover:bg-white/10 border border-white/10 text-slate-300 hover:text-white text-xs font-medium transition"
              >
                Cancel Pull
              </button>
            {/if}

            <button
              type="submit"
              disabled={isPulling || !imageRef.trim()}
              class="px-5 py-2 rounded-xl bg-violet-600 hover:bg-violet-500 text-white text-xs font-semibold shadow-[0_0_15px_rgba(124,58,237,0.35)] transition flex items-center gap-2 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {#if isPulling}
                <Loader2 class="w-3.5 h-3.5 animate-spin" />
                Pulling Image...
              {:else}
                <Download class="w-3.5 h-3.5" />
                Pull Image
              {/if}
            </button>
          </div>
        </form>

        <!-- Real-time Layer Download & Extract Progress -->
        {#if layerList.length > 0}
          <div class="space-y-2 pt-2 border-t border-white/[0.08]">
            <div class="flex items-center justify-between text-[11px] text-slate-400 font-mono">
              <span class="flex items-center gap-1.5 text-violet-400 font-semibold uppercase tracking-wider">
                <Layers class="w-3.5 h-3.5" />
                Image Layers ({layerList.length})
              </span>
              {#if isPulling}
                <span class="flex items-center gap-1.5 text-cyan-400">
                  <span class="w-1.5 h-1.5 rounded-full bg-cyan-400 animate-ping"></span>
                  Streaming download
                </span>
              {/if}
            </div>

            <div class="space-y-2 max-h-48 overflow-y-auto pr-1">
              {#each layerList as layer (layer.id)}
                <div class="p-2.5 rounded-xl bg-[#0c0d14] border border-white/[0.06] space-y-1.5">
                  <div class="flex items-center justify-between text-[11px] font-mono">
                    <div class="flex items-center gap-2">
                      <span class="text-violet-300 font-semibold">{layer.id}</span>
                      <span class="text-slate-400">{layer.status}</span>
                    </div>
                    <div class="flex items-center gap-2 text-slate-400">
                      {#if layer.total > 0}
                        <span>{formatBytes(layer.current)} / {formatBytes(layer.total)}</span>
                        <span class="text-cyan-400 font-semibold">{layer.percent}%</span>
                      {:else if layer.textProgress}
                        <span class="text-[10px]">{layer.textProgress}</span>
                      {/if}
                    </div>
                  </div>

                  <!-- Progress Bar -->
                  <div class="w-full h-1.5 rounded-full bg-white/5 overflow-hidden">
                    <div
                      class="h-full rounded-full transition-all duration-300 {layer.status.includes('complete') ? 'bg-emerald-500 shadow-[0_0_8px_#10b981]' : 'bg-gradient-to-r from-violet-500 to-cyan-400 shadow-[0_0_8px_rgba(6,182,212,0.5)]'}"
                      style="width: {layer.status.includes('complete') ? '100' : layer.percent}%"
                    ></div>
                  </div>
                </div>
              {/each}
            </div>
          </div>
        {/if}

        <!-- Pull Stream Console / Logs -->
        {#if logMessages.length > 0}
          <div class="space-y-1.5 pt-2 border-t border-white/[0.08]">
            <div class="flex items-center justify-between text-[11px] text-slate-400 font-mono">
              <span class="flex items-center gap-1.5 text-slate-300 font-semibold uppercase tracking-wider">
                <Terminal class="w-3.5 h-3.5 text-cyan-400" />
                Pull Stream Output
              </span>
              <span class="text-[10px] text-slate-500 font-mono">{logMessages.length} events</span>
            </div>

            <div
              bind:this={logsContainer}
              class="h-32 overflow-y-auto p-3 rounded-xl bg-[#090a0f] border border-white/[0.06] font-mono text-[11px] text-slate-300 space-y-1 select-text"
            >
              {#each logMessages as msg, idx}
                <div class="leading-relaxed whitespace-pre-wrap {msg.includes('Error') ? 'text-rose-400' : msg.includes('Successfully') ? 'text-emerald-400' : 'text-slate-300'}">
                  <span class="text-slate-600 select-none mr-2">&rsaquo;</span>{msg}
                </div>
              {/each}
            </div>
          </div>
        {/if}
      </div>

      <!-- Modal Footer -->
      <div class="px-6 py-3.5 border-t border-white/[0.08] bg-[#0c0d14]/80 flex items-center justify-between">
        <span class="text-[11px] text-slate-500 font-mono">
          {#if isPulling}
            <span class="text-cyan-400 flex items-center gap-1.5">
              <span class="w-1.5 h-1.5 rounded-full bg-cyan-400 animate-ping"></span>
              Pulling detached in background...
            </span>
          {:else if isSuccess}
            <span class="text-emerald-400">Pull completed successfully</span>
          {:else}
            Ready
          {/if}
        </span>
        <div class="flex items-center gap-2">
          {#if isPulling}
            <button
              type="button"
              onclick={handleCancel}
              class="px-3.5 py-1.5 rounded-xl bg-rose-500/10 hover:bg-rose-500/20 border border-rose-500/20 text-rose-300 hover:text-rose-200 text-xs font-medium transition cursor-pointer"
            >
              Cancel
            </button>
            <button
              type="button"
              onclick={runInBackground}
              class="px-4 py-1.5 rounded-xl bg-cyan-500/15 hover:bg-cyan-500/25 border border-cyan-500/30 text-cyan-300 hover:text-white text-xs font-semibold shadow-[0_0_12px_rgba(6,182,212,0.2)] transition flex items-center gap-1.5 cursor-pointer"
            >
              <Minimize2 class="w-3.5 h-3.5" />
              <span>Run in Background</span>
            </button>
          {:else}
            <button
              type="button"
              onclick={handleClose}
              class="px-4 py-1.5 rounded-xl bg-white/5 hover:bg-white/10 border border-white/10 text-slate-300 hover:text-white text-xs font-medium transition cursor-pointer"
            >
              Close
            </button>
          {/if}
        </div>
      </div>
    </div>
  </div>
{/if}
