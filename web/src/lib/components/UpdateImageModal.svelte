<script lang="ts">
  import {
    X,
    ArrowUpCircle,
    Loader2,
    AlertTriangle,
    Layers,
    CheckCircle2,
    Box,
    HardDrive,
    Info,
    ShieldAlert,
  } from '@lucide/svelte';
  import { portal } from '../actions/portal';
  import type { Image, Container } from '../types';

  interface Props {
    isOpen: boolean;
    imageRef: string;
    image?: Image | null;
    containers?: Container[];
    isUpdating?: boolean;
    onConfirm: () => void;
    onClose: () => void;
  }

  let {
    isOpen = false,
    imageRef = '',
    image = null,
    containers = [],
    isUpdating = false,
    onConfirm,
    onClose,
  }: Props = $props();

  // Find containers using this image
  let affectedContainers = $derived.by<Container[]>(() => {
    if (!imageRef) return [];
    const cleanRef = imageRef.trim();
    const cleanId = image ? image.Id.replace(/^sha256:/, '') : '';
    return containers.filter((c) => {
      if (c.Image === cleanRef) return true;
      if (cleanId && (c.ImageID || '').replace(/^sha256:/, '').startsWith(cleanId)) return true;
      if (image?.RepoTags?.includes(c.Image)) return true;
      return false;
    });
  });

  let runningAffectedCount = $derived(
    affectedContainers.filter((c) => c.State === 'running').length
  );

  let shortId = $derived(
    image?.Id ? image.Id.replace(/^sha256:/, '').slice(0, 12) : ''
  );
</script>

{#if isOpen && imageRef}
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
              <h2 class="text-base font-bold text-white tracking-wide">
                Update Docker Image
              </h2>
              <p class="text-xs text-slate-400 mt-0.5">
                Download the latest image manifest and layers from registry
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

        <!-- Target Image Card -->
        <div class="p-4 rounded-xl bg-[#0a0b10] border border-white/[0.08] space-y-2.5 font-mono text-xs">
          <div class="flex items-center justify-between">
            <span class="text-slate-400">Image Reference:</span>
            <span class="font-bold text-cyan-300 truncate max-w-[280px]" title={imageRef}>
              {imageRef}
            </span>
          </div>

          {#if shortId}
            <div class="flex items-center justify-between">
              <span class="text-slate-400">Current Local ID:</span>
              <span class="font-bold text-violet-300">{shortId}</span>
            </div>
          {/if}
        </div>

        <!-- Containers Impact Assessment -->
        {#if affectedContainers.length > 0}
          <div class="p-4 rounded-xl bg-amber-500/10 border border-amber-500/30 space-y-3">
            <div class="flex items-start gap-2.5">
              <AlertTriangle class="w-5 h-5 text-amber-400 shrink-0 mt-0.5" />
              <div>
                <h3 class="text-xs font-bold text-amber-200">
                  {affectedContainers.length} container{affectedContainers.length === 1 ? '' : 's'} using this image ({runningAffectedCount} running)
                </h3>
                <p class="text-[11px] text-amber-300/90 leading-relaxed mt-1">
                  Pulling the new version will re-tag <span class="font-mono text-white font-semibold">{imageRef}</span>. Existing containers will continue running on the old image until recreated.
                </p>
                <p class="text-[11px] text-amber-300/90 leading-relaxed mt-1 font-semibold">
                  ⚠️ Caution: Recreating containers with newer images may introduce breaking changes, incompatible configuration, or require database migrations.
                </p>
              </div>
            </div>

            <!-- List of affected containers -->
            <div class="max-h-36 overflow-y-auto space-y-1.5 pt-1 border-t border-amber-500/20">
              {#each affectedContainers as c}
                {@const cName = c.Names?.[0] ? c.Names[0].replace(/^\//, '') : c.Id.slice(0, 12)}
                {@const isRunning = c.State === 'running'}
                <div class="flex items-center justify-between px-2.5 py-1.5 rounded-lg bg-black/40 border border-amber-500/20 text-xs font-mono">
                  <div class="flex items-center gap-2 truncate">
                    <span class="w-1.5 h-1.5 rounded-full shrink-0 {isRunning ? 'bg-emerald-400 animate-pulse' : 'bg-slate-500'}"></span>
                    <span class="text-white font-semibold truncate">{cName}</span>
                  </div>
                  <span class="text-[10px] uppercase font-bold shrink-0 {isRunning ? 'text-emerald-400' : 'text-slate-400'}">
                    {c.State || 'unknown'}
                  </span>
                </div>
              {/each}
            </div>
          </div>
        {:else}
          <div class="p-3.5 rounded-xl bg-emerald-500/10 border border-emerald-500/30 flex items-start gap-2.5 text-emerald-300 text-xs">
            <CheckCircle2 class="w-4 h-4 text-emerald-400 shrink-0 mt-0.5" />
            <p>
              No active containers are currently using this image. Updating will download the latest remote layers without impacting any running workloads.
            </p>
          </div>
        {/if}

        <!-- Pull Summary -->
        <div class="p-3 rounded-xl bg-white/[0.02] border border-white/[0.06] text-xs text-slate-400 flex items-center gap-2">
          <Info class="w-4 h-4 text-cyan-400 shrink-0" />
          <span>The image will be pulled in the background and will continue even if this browser tab is closed.</span>
        </div>

        <!-- Footer Actions -->
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
              <span>Pulling Image...</span>
            {:else}
              <ArrowUpCircle class="w-4 h-4" />
              <span>Pull & Update Image</span>
            {/if}
          </button>
        </div>
      </div>
    </div>
  </div>
{/if}
