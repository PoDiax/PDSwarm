<script lang="ts">
  import {
    X,
    CircleArrowUp,
    LoaderCircle,
    AlertTriangle,
    Layers,
    CircleCheck,
    Box,
    HardDrive,
    Info,
    ShieldAlert,
    ExternalLink,
    FileText,
    Sparkles,
    Clock,
    Tag,
    GitBranch,
    RefreshCw,
  } from '@lucide/svelte';
  import { portal } from '../actions/portal';
  import { imageUpdateStore } from '../stores/imageUpdates.svelte';
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

  // Live update details from store
  let updateStatus = $derived(imageUpdateStore.getStatus(imageRef));
  let isCheckingStatus = $derived(imageUpdateStore.isChecking(imageRef));

  $effect(() => {
    if (isOpen && imageRef) {
      const trimmed = imageRef.trim();
      if (!imageUpdateStore.getStatus(trimmed)) {
        imageUpdateStore.checkUpdate(trimmed);
      }
    }
  });

  function formatBytes(bytes?: number): string {
    if (!bytes || bytes <= 0) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(1024));
    return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`;
  }

  function formatDate(dateStr?: string): string {
    if (!dateStr) return '—';
    try {
      const d = new Date(dateStr);
      if (isNaN(d.getTime())) return dateStr;
      return d.toLocaleDateString(undefined, {
        year: 'numeric',
        month: 'short',
        day: 'numeric',
      });
    } catch {
      return dateStr;
    }
  }

  function formatRelativeAge(dateStr?: string): string {
    if (!dateStr) return '';
    try {
      const d = new Date(dateStr);
      if (isNaN(d.getTime())) return '';
      const diffSec = Math.max(0, Math.floor((Date.now() - d.getTime()) / 1000));
      if (diffSec < 60) return 'just now';
      const mins = Math.floor(diffSec / 60);
      if (mins < 60) return `${mins}m ago`;
      const hours = Math.floor(mins / 60);
      if (hours < 24) return `${hours}h ago`;
      const days = Math.floor(hours / 24);
      if (days < 30) return `${days}d ago`;
      const months = Math.floor(days / 30);
      return `${months}mo ago`;
    } catch {
      return '';
    }
  }
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
      class="relative w-full max-w-xl max-h-[90vh] flex flex-col rounded-2xl bg-[#12131c] border border-white/10 shadow-[0_0_50px_rgba(0,0,0,0.8)] overflow-hidden z-10 animate-in fade-in zoom-in-95 duration-200"
    >
      <!-- Top Neon Header Accent Line -->
      <div class="h-1 w-full bg-gradient-to-r from-amber-500 via-violet-500 to-cyan-500 shrink-0"></div>

      <div class="p-6 space-y-5 overflow-y-auto">
        <!-- Header -->
        <div class="flex items-start justify-between gap-4">
          <div class="flex items-center gap-3">
            <div class="p-3 rounded-xl bg-amber-500/15 border border-amber-500/30 text-amber-400 shadow-[0_0_20px_rgba(245,158,11,0.25)]">
              <CircleArrowUp class="w-6 h-6" />
            </div>
            <div>
              <h2 class="text-base font-bold text-white tracking-wide">
                Update Docker Image
              </h2>
              <p class="text-xs text-slate-400 mt-0.5">
                Compare versions, inspect change logs, and pull remote updates
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

        <!-- Target Image Header Badge -->
        <div class="flex items-center justify-between px-3.5 py-2.5 rounded-xl bg-[#0a0b10] border border-white/[0.08] text-xs font-mono">
          <span class="text-slate-400">Target Reference:</span>
          <span class="font-bold text-cyan-300 truncate max-w-[340px]" title={imageRef}>
            {imageRef}
          </span>
        </div>

        <!-- Version Comparison: Old (Local) vs New (Remote Registry) -->
        <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
          <!-- Current Local Version Card -->
          <div class="p-3.5 rounded-xl bg-white/[0.02] border border-white/[0.08] space-y-2.5">
            <div class="flex items-center justify-between">
              <span class="text-[11px] font-bold text-slate-400 uppercase tracking-wider flex items-center gap-1.5">
                <Box class="w-3.5 h-3.5 text-violet-400" />
                Current (Local)
              </span>
              <span class="px-2 py-0.5 rounded-md text-[10px] font-mono font-semibold bg-violet-500/15 text-violet-300 border border-violet-500/20">
                Installed
              </span>
            </div>

            <div class="space-y-1.5 text-xs font-mono">
              <div class="flex items-center justify-between">
                <span class="text-slate-400">Version/Tag:</span>
                <span class="text-white font-semibold truncate max-w-[150px]">
                  {updateStatus?.local_version || imageRef.split(':')[1] || 'latest'}
                </span>
              </div>
              <div class="flex items-center justify-between">
                <span class="text-slate-400">Image ID:</span>
                <span class="text-violet-300 font-semibold">{shortId || updateStatus?.local_id || '—'}</span>
              </div>
              {#if updateStatus?.local_digest}
                <div class="flex items-center justify-between">
                  <span class="text-slate-400">Digest:</span>
                  <span class="text-slate-300 truncate max-w-[130px]" title={updateStatus.local_digest}>
                    {updateStatus.local_digest.replace(/^sha256:/, '').slice(0, 12)}
                  </span>
                </div>
              {/if}
              <div class="flex items-center justify-between">
                <span class="text-slate-400">Image Size:</span>
                <span class="text-slate-300 font-semibold">
                  {formatBytes(updateStatus?.local_size || image?.Size)}
                </span>
              </div>
              {#if updateStatus?.local_created}
                <div class="flex items-center justify-between">
                  <span class="text-slate-400">Created:</span>
                  <span class="text-slate-300 text-[11px]" title={updateStatus.local_created}>
                    {formatDate(updateStatus.local_created)}
                    {#if formatRelativeAge(updateStatus.local_created)}
                      <span class="text-slate-500">({formatRelativeAge(updateStatus.local_created)})</span>
                    {/if}
                  </span>
                </div>
              {/if}
            </div>
          </div>

          <!-- Remote Registry Version Card -->
          <div class="p-3.5 rounded-xl bg-white/[0.02] border border-white/[0.08] space-y-2.5">
            <div class="flex items-center justify-between">
              <span class="text-[11px] font-bold text-slate-400 uppercase tracking-wider flex items-center gap-1.5">
                <Sparkles class="w-3.5 h-3.5 text-cyan-400" />
                Remote Registry
              </span>
              {#if isCheckingStatus}
                <span class="px-2 py-0.5 rounded-md text-[10px] font-mono font-semibold bg-cyan-500/15 text-cyan-300 border border-cyan-500/20 inline-flex items-center gap-1">
                  <LoaderCircle class="w-3 h-3 animate-spin" />
                  Checking
                </span>
              {:else if updateStatus?.status === 'update_available'}
                <span class="px-2 py-0.5 rounded-md text-[10px] font-mono font-semibold bg-emerald-500/15 text-emerald-300 border border-emerald-500/30 shadow-[0_0_8px_rgba(16,185,129,0.25)]">
                  Update Available
                </span>
              {:else if updateStatus?.status === 'up_to_date'}
                <span class="px-2 py-0.5 rounded-md text-[10px] font-mono font-semibold bg-white/10 text-slate-300 border border-white/15">
                  Up To Date
                </span>
              {:else}
                <span class="px-2 py-0.5 rounded-md text-[10px] font-mono font-semibold bg-amber-500/15 text-amber-300 border border-amber-500/20">
                  Ready to pull
                </span>
              {/if}
            </div>

            <div class="space-y-1.5 text-xs font-mono">
              <div class="flex items-center justify-between">
                <span class="text-slate-400">Remote Digest:</span>
                {#if updateStatus?.remote_digest}
                  <span class="text-cyan-300 font-semibold truncate max-w-[150px]" title={updateStatus.remote_digest}>
                    {updateStatus.remote_digest.replace(/^sha256:/, '').slice(0, 12)}
                  </span>
                {:else if isCheckingStatus}
                  <span class="text-slate-500 italic">Querying manifest...</span>
                {:else}
                  <span class="text-slate-500">Determined on pull</span>
                {/if}
              </div>

              <div class="flex items-center justify-between">
                <span class="text-slate-400">Sync Status:</span>
                <span class="text-slate-300 text-[11px] truncate max-w-[160px]">
                  {updateStatus?.message || 'Ready to pull latest layers'}
                </span>
              </div>

              <!-- Quick Links for Changelog & Source -->
              <div class="pt-2 flex flex-wrap gap-1.5">
                {#if updateStatus?.changelog_url}
                  <a
                    href={updateStatus.changelog_url}
                    target="_blank"
                    rel="noreferrer"
                    class="px-2 py-1 rounded bg-violet-600/20 hover:bg-violet-600/30 text-violet-300 border border-violet-500/30 text-[11px] font-medium transition inline-flex items-center gap-1"
                    title="View releases and changelogs on GitHub"
                  >
                    <FileText class="w-3 h-3 text-violet-400" />
                    Changelog
                    <ExternalLink class="w-2.5 h-2.5 opacity-70" />
                  </a>
                {/if}
                {#if updateStatus?.hub_url}
                  <a
                    href={updateStatus.hub_url}
                    target="_blank"
                    rel="noreferrer"
                    class="px-2 py-1 rounded bg-cyan-600/20 hover:bg-cyan-600/30 text-cyan-300 border border-cyan-500/30 text-[11px] font-medium transition inline-flex items-center gap-1"
                    title="View tags and layers on Registry"
                  >
                    <ExternalLink class="w-3 h-3 text-cyan-400" />
                    Registry Details
                  </a>
                {/if}
                {#if updateStatus?.source_url && updateStatus.source_url !== updateStatus.changelog_url}
                  <a
                    href={updateStatus.source_url}
                    target="_blank"
                    rel="noreferrer"
                    class="px-2 py-1 rounded bg-white/5 hover:bg-white/10 text-slate-300 border border-white/10 text-[11px] font-medium transition inline-flex items-center gap-1"
                    title="View source repository"
                  >
                    <GitBranch class="w-3 h-3 text-slate-400" />
                    Source
                  </a>
                {/if}
              </div>
            </div>
          </div>
        </div>

        <!-- Upstream Changelog / Release Notes Preview -->
        {#if updateStatus?.releases && updateStatus.releases.length > 0}
          <div class="p-4 rounded-xl bg-violet-500/[0.08] border border-violet-500/25 space-y-3">
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-2">
                <FileText class="w-4 h-4 text-violet-400" />
                <h3 class="text-xs font-bold text-violet-200">
                  Recent Upstream Releases & Changelogs
                </h3>
              </div>
              {#if updateStatus.changelog_url}
                <a
                  href={updateStatus.changelog_url}
                  target="_blank"
                  rel="noreferrer"
                  class="text-[11px] text-cyan-400 hover:text-cyan-300 hover:underline inline-flex items-center gap-1"
                >
                  View All on GitHub <ExternalLink class="w-3 h-3" />
                </a>
              {/if}
            </div>

            <div class="space-y-2.5 max-h-48 overflow-y-auto pr-1">
              {#each updateStatus.releases as rel}
                <div class="p-3 rounded-lg bg-black/40 border border-white/[0.06] space-y-1.5 text-xs">
                  <div class="flex items-center justify-between font-mono">
                    <span class="font-bold text-white text-xs flex items-center gap-1.5">
                      <Tag class="w-3 h-3 text-cyan-400" />
                      {rel.name || rel.tag_name}
                    </span>
                    <span class="text-[10px] text-slate-400">
                      {rel.published_at ? formatDate(rel.published_at) : ''}
                    </span>
                  </div>
                  {#if rel.body}
                    <div class="text-[11px] text-slate-300 whitespace-pre-wrap font-sans max-h-24 overflow-y-auto leading-relaxed border-t border-white/[0.04] pt-1.5 text-opacity-90">
                      {rel.body}
                    </div>
                  {/if}
                </div>
              {/each}
            </div>
          </div>
        {/if}

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
                  ⚠️ Caution: Newer images may introduce breaking changes or require database migrations.
                </p>
              </div>
            </div>

            <!-- List of affected containers -->
            <div class="max-h-32 overflow-y-auto space-y-1.5 pt-1 border-t border-amber-500/20">
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
            <CircleCheck class="w-4 h-4 text-emerald-400 shrink-0 mt-0.5" />
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
              <LoaderCircle class="w-4 h-4 animate-spin" />
              <span>Pulling Image...</span>
            {:else}
              <CircleArrowUp class="w-4 h-4" />
              <span>Pull & Update Image</span>
            {/if}
          </button>
        </div>
      </div>
    </div>
  </div>
{/if}
