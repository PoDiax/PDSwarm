<script lang="ts">
  import {
    X,
    ArrowUpCircle,
    Loader2,
    AlertTriangle,
    Layers,
    CheckCircle2,
    RefreshCw,
    ShieldAlert,
  } from '@lucide/svelte';
  import { portal } from '../actions/portal';
  interface Props {
    isOpen: boolean;
    container: {
      Id?: string;
      Name?: string;
      Names?: string[];
      Image?: string;
      Config?: { Image?: string };
      State?: unknown;
    } | null;
    isUpdating?: boolean;
    onConfirm: () => void;
    onClose: () => void;
  }

  let {
    isOpen = false,
    container = null,
    isUpdating = false,
    onConfirm,
    onClose,
  }: Props = $props();

  let cleanName = $derived(
    container?.Name
      ? container.Name.replace(/^\//, '')
      : container?.Names?.[0]
      ? container.Names[0].replace(/^\//, '')
      : container?.Id?.slice(0, 12) || 'Container'
  );
  let stateStr = $derived(
    typeof container?.State === 'string'
      ? container.State
      : typeof container?.State === 'object' && container?.State !== null && 'Status' in container.State
      ? String((container.State as { Status: unknown }).Status || 'unknown')
      : 'unknown'
  );
  let isRunning = $derived(
    stateStr === 'running' ||
    Boolean(typeof container?.State === 'object' && container?.State !== null && 'Running' in container.State && (container.State as { Running: unknown }).Running)
  );
  let imageRef = $derived(container?.Image || container?.Config?.Image || '');
</script>

{#if isOpen && container}
  <div use:portal class="fixed inset-0 z-50 flex items-center justify-center p-4">
    <!-- Backdrop overlay -->
    <div
      role="button"
      tabindex="-1"
      onclick={onClose}
      onkeydown={(e) => (e.key === 'Escape' ? onClose() : null)}
      class="fixed inset-0 bg-black/80 backdrop-blur-md transition-opacity"
    ></div>

    <!-- Modal Dialog Surface -->
    <div
      class="relative w-full max-w-lg rounded-2xl bg-[#12131c] border border-white/10 shadow-[0_0_50px_rgba(0,0,0,0.8)] overflow-hidden z-10 animate-in fade-in zoom-in-95 duration-200"
    >
      <!-- Top Neon Header Accent Line -->
      <div class="h-1 w-full bg-gradient-to-r from-amber-500 via-violet-500 to-cyan-500"></div>

      <div class="p-6 space-y-5">
        <!-- Header -->
        <div class="flex items-start justify-between gap-4">
          <div class="flex items-center gap-3">
            <div class="p-3 rounded-xl bg-amber-500/15 border border-amber-500/30 text-amber-400 shadow-[0_0_20px_rgba(245,158,11,0.25)]">
              <ArrowUpCircle class="w-6 h-6" />
            </div>
            <div>
              <h2 class="text-base font-bold text-white tracking-wide flex items-center gap-2">
                Update & Recreate Container
              </h2>
              <p class="text-xs text-slate-400 mt-0.5">
                Pull remote updates and seamlessly rebuild this workload
              </p>
            </div>
          </div>

          <button
            type="button"
            onclick={onClose}
            class="p-1.5 rounded-lg text-slate-400 hover:text-white hover:bg-white/10 transition cursor-pointer"
            aria-label="Close dialog"
          >
            <X class="w-5 h-5" />
          </button>
        </div>

        <!-- Target Workload Details Card -->
        <div class="p-4 rounded-xl bg-[#0a0b10] border border-white/[0.08] space-y-3 font-mono text-xs">
          <div class="flex items-center justify-between">
            <span class="text-slate-400">Container Name:</span>
            <span class="font-bold text-white tracking-tight">{cleanName}</span>
          </div>

          <div class="flex items-center justify-between">
            <span class="text-slate-400">Image Reference:</span>
            <span class="font-bold text-cyan-300 truncate max-w-[260px]" title={imageRef}>
              {imageRef}
            </span>
          </div>

          <div class="flex items-center justify-between">
            <span class="text-slate-400">Current State:</span>
            <span class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[10px] font-semibold {isRunning ? 'bg-emerald-500/15 text-emerald-300 border border-emerald-500/30' : 'bg-slate-700/40 text-slate-400 border border-slate-600/30'}">
              <span class="w-1.5 h-1.5 rounded-full {isRunning ? 'bg-emerald-400 animate-pulse' : 'bg-slate-400'}"></span>
              {container.State || 'unknown'}
            </span>
          </div>
        </div>

        <!-- Automated Execution Steps -->
        <div class="space-y-2 text-xs">
          <span class="text-slate-300 font-semibold block text-[11px] uppercase tracking-wider">
            Recreation Process:
          </span>
          <div class="grid grid-cols-1 gap-2">
            <div class="p-2.5 rounded-lg bg-white/[0.02] border border-white/[0.05] flex items-center gap-2.5 text-slate-300">
              <span class="w-5 h-5 rounded-full bg-violet-500/20 text-violet-300 border border-violet-500/30 flex items-center justify-center font-mono text-[10px] shrink-0 font-bold">1</span>
              <span>Pull latest image layers from remote registry</span>
            </div>
            <div class="p-2.5 rounded-lg bg-white/[0.02] border border-white/[0.05] flex items-center gap-2.5 text-slate-300">
              <span class="w-5 h-5 rounded-full bg-violet-500/20 text-violet-300 border border-violet-500/30 flex items-center justify-center font-mono text-[10px] shrink-0 font-bold">2</span>
              <span>Gracefully stop old container instance</span>
            </div>
            <div class="p-2.5 rounded-lg bg-white/[0.02] border border-white/[0.05] flex items-center gap-2.5 text-slate-300">
              <span class="w-5 h-5 rounded-full bg-violet-500/20 text-violet-300 border border-violet-500/30 flex items-center justify-center font-mono text-[10px] shrink-0 font-bold">3</span>
              <span>Recreate preserving original ports, volumes, environment, and limits</span>
            </div>
            <div class="p-2.5 rounded-lg bg-white/[0.02] border border-white/[0.05] flex items-center gap-2.5 text-slate-300">
              <span class="w-5 h-5 rounded-full bg-violet-500/20 text-violet-300 border border-violet-500/30 flex items-center justify-center font-mono text-[10px] shrink-0 font-bold">4</span>
              <span>Restart container automatically</span>
            </div>
          </div>
        </div>

        <!-- Warning Notice -->
        <div class="p-3 rounded-xl bg-amber-500/10 border border-amber-500/30 flex items-start gap-2.5 text-amber-300 text-xs">
          <AlertTriangle class="w-4 h-4 shrink-0 mt-0.5 text-amber-400" />
          <p>
            The container will experience brief downtime while the new instance is started. If the remote pull fails, the original container will remain untouched.
          </p>
        </div>

        <!-- Actions -->
        <div class="flex items-center justify-end gap-3 pt-2">
          <button
            type="button"
            onclick={onClose}
            class="px-4 py-2 rounded-xl text-xs font-semibold text-slate-300 hover:text-white bg-white/[0.05] hover:bg-white/[0.1] border border-white/10 transition cursor-pointer"
          >
            Cancel
          </button>

          <button
            type="button"
            onclick={onConfirm}
            disabled={isUpdating}
            class="px-5 py-2 rounded-xl text-xs font-semibold text-white bg-gradient-to-r from-amber-600 via-violet-600 to-cyan-600 hover:from-amber-500 hover:to-cyan-500 shadow-[0_0_20px_rgba(245,158,11,0.3)] transition inline-flex items-center gap-2 disabled:opacity-50 cursor-pointer"
          >
            {#if isUpdating}
              <Loader2 class="w-4 h-4 animate-spin" />
              <span>Updating & Recreating...</span>
            {:else}
              <ArrowUpCircle class="w-4 h-4" />
              <span>Update & Recreate</span>
            {/if}
          </button>
        </div>
      </div>
    </div>
  </div>
{/if}
