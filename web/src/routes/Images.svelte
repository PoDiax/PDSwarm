<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import {
    Disc,
    Plus,
    Search,
    RefreshCw,
    Trash2,
    Scissors,
    Download,
    AlertCircle,
    CheckCircle2,
    Loader2,
    Copy,
    Check,
    HardDrive,
    Layers,
    Tag,
    Clock,
    AlertTriangle,
    X,
    Filter,
    Key,
    Hammer,
    Sparkles,
    ArrowUpCircle,
    ArrowUpDown,
    ArrowUp,
    ArrowDown,
  } from '@lucide/svelte';
  import { imageApi, containerApi, taskApi } from '../lib/api';
  import { imageUpdateStore } from '../lib/stores/imageUpdates.svelte';
  import { tasksStore } from '../lib/stores/tasks.svelte';
  import type { Image, Container, ImageUpdateStatus } from '../lib/types';
  import PullImageModal from '../lib/components/PullImageModal.svelte';
  import ImageBuildModal from '../lib/components/ImageBuildModal.svelte';
  import RegistriesManager from '../lib/components/RegistriesManager.svelte';
  import UpdateImageModal from '../lib/components/UpdateImageModal.svelte';

  interface Props {
    onNavigate?: (tab: string) => void;
  }

  let { onNavigate }: Props = $props();

  // State
  let images = $state<Image[]>([]);
  let containers = $state<Container[]>([]);
  let isLoading = $state(true);
  let isRefreshing = $state(false);
  let error = $state<string | null>(null);
  let feedback = $state<{ type: 'success' | 'error' | 'info'; message: string } | null>(null);

  // Filters & Search
  let searchQuery = $state('');
  let filterType = $state<'all' | 'tagged' | 'dangling'>('all');
  let sortBy = $state<'created' | 'created-asc' | 'size-desc' | 'size-asc' | 'name' | 'name-desc' | 'in-use-desc' | 'in-use-asc'>('created');

  function toggleImageSort(column: 'name' | 'size' | 'created' | 'in-use') {
    if (column === 'name') {
      sortBy = sortBy === 'name' ? 'name-desc' : 'name';
    } else if (column === 'size') {
      sortBy = sortBy === 'size-desc' ? 'size-asc' : 'size-desc';
    } else if (column === 'created') {
      sortBy = sortBy === 'created' ? 'created-asc' : 'created';
    } else if (column === 'in-use') {
      sortBy = sortBy === 'in-use-desc' ? 'in-use-asc' : 'in-use-desc';
    }
  }

  // Modals
  let isBuildModalOpen = $state(false);
  let isPullModalOpen = $state(false);
  let pullModalInitialRef = $state('');
  let isRegistriesModalOpen = $state(false);
  let isPruneModalOpen = $state(false);
  let isPruning = $state(false);

  // Update Image Confirmation Modal
  let updateTarget = $state<{ ref: string; image?: Image | null } | null>(null);
  let isUpdateModalOpen = $state(false);
  let isConfirmUpdating = $state(false);

  let deleteImageTarget = $state<Image | null>(null);
  let deleteForce = $state(false);
  let isDeleting = $state(false);

  // Multi-Selection State
  let selectedIds = $state<string[]>([]);
  let isBatchDeleteModalOpen = $state(false);
  let isBatchBusy = $state(false);
  let batchForce = $state(false);
  let selectAllEl = $state<HTMLInputElement | null>(null);

  // Clipboard feedback
  let copiedId = $state<string | null>(null);
  let refreshTimer: ReturnType<typeof setInterval> | null = null;

  // Remote Registry Up-to-Date Status
  let isCheckingAllUpdates = $state(false);
  let updatingTags = $state<Record<string, boolean>>({});

  function isUpdatingTag(ref?: string): boolean {
    if (!ref) return false;
    if (updatingTags[ref]) return true;
    return tasksStore.activeTasks.some(
      (t) => t.type === 'image_pull' && t.target === ref && (t.status === 'running' || t.status === 'pending')
    );
  }

  function openUpdateModal(imageRef: string, img?: Image | null) {
    const trimmed = (imageRef || '').trim();
    if (!trimmed || isUpdatingTag(trimmed)) return;
    isConfirmUpdating = false;
    updateTarget = { ref: trimmed, image: img || null };
    isUpdateModalOpen = true;
  }

  async function confirmUpdateImage() {
    if (!updateTarget) return;
    const targetRef = updateTarget.ref;
    isUpdateModalOpen = false;
    updateTarget = null;
    isConfirmUpdating = false;
    await handleUpdateImage(targetRef);
  }

  async function handleUpdateImage(imageRef: string) {
    const trimmed = (imageRef || '').trim();
    if (!trimmed || isUpdatingTag(trimmed)) return;

    updatingTags = { ...updatingTags, [trimmed]: true };
    showFeedback('info', `Updating ${trimmed} from remote registry in background...`);

    try {
      const res = await imageApi.pull(trimmed, undefined, true);
      tasksStore.poll();

      // Poll until task finishes if task_id returned
      if (res && res.task_id) {
        let isComplete = false;
        let attempts = 0;
        while (!isComplete && attempts < 120) {
          attempts++;
          await new Promise((r) => setTimeout(r, 1200));
          try {
            const task = await taskApi.get(res.task_id);
            if (task.status === 'completed') {
              isComplete = true;
              break;
            } else if (task.status === 'failed' || task.status === 'cancelled') {
              throw new Error(task.error || `Update ${task.status}`);
            }
          } catch (e: unknown) {
            if (e instanceof Error && (e.message.includes('failed') || e.message.includes('cancelled') || e.message.includes('Update '))) {
              throw e;
            }
          }
        }
      }

      // Re-query digest and refresh image table
      await imageUpdateStore.checkUpdate(trimmed);
      await fetchImages(true);
      showFeedback('success', `Image ${trimmed} updated successfully to latest version!`);
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to update image';
      showFeedback('error', msg);
    } finally {
      const next = { ...updatingTags };
      delete next[trimmed];
      updatingTags = next;
    }
  }

  async function checkUpdateForImage(imageRef: string) {
    const res = await imageUpdateStore.checkUpdate(imageRef);
    if (res?.status === 'update_available') {
      showFeedback('success', `Update available for ${imageRef}!`);
    }
  }

  async function checkAllUpdates() {
    isCheckingAllUpdates = true;
    const tagged = images
      .flatMap((i) => i.RepoTags || [])
      .filter((t) => t && !t.startsWith('<none>'));

    await imageUpdateStore.checkMultiple(tagged);
    isCheckingAllUpdates = false;
    showFeedback('success', `Completed update check for ${tagged.length} image(s).`);
  }

  function showFeedback(type: 'success' | 'error' | 'info', message: string) {
    feedback = { type, message };
    setTimeout(() => {
      if (feedback?.message === message) {
        feedback = null;
      }
    }, 4500);
  }

  async function fetchImages(silent = false) {
    if (!silent) isRefreshing = true;
    error = null;

    try {
      const [imgs, conts] = await Promise.all([
        imageApi.list(),
        containerApi.list(true).catch(() => [] as Container[]),
      ]);
      images = imgs || [];
      containers = conts || [];
      const existingIds = new Set((imgs || []).map((i) => i.Id));
      selectedIds = selectedIds.filter((id) => existingIds.has(id));
    } catch (err: unknown) {
      if (err instanceof Error) {
        error = err.message;
      } else {
        error = 'Failed to load Docker images.';
      }
    } finally {
      isLoading = false;
      isRefreshing = false;
    }
  }

  onMount(() => {
    fetchImages();
    refreshTimer = setInterval(() => {
      fetchImages(true);
    }, 10000);
  });

  onDestroy(() => {
    if (refreshTimer) clearInterval(refreshTimer);
  });

  function formatBytes(bytes: number): string {
    if (!bytes || bytes <= 0) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(1024));
    return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`;
  }

  function formatRelativeTime(createdUnixSeconds: number): string {
    if (!createdUnixSeconds) return 'Unknown';
    const now = Math.floor(Date.now() / 1000);
    const diff = Math.max(0, now - createdUnixSeconds);

    if (diff < 60) return `${diff}s ago`;
    const mins = Math.floor(diff / 60);
    if (mins < 60) return `${mins}m ago`;
    const hours = Math.floor(mins / 60);
    if (hours < 24) return `${hours}h ago`;
    const days = Math.floor(hours / 24);
    if (days < 30) return `${days}d ago`;
    const months = Math.floor(days / 30);
    return `${months}mo ago`;
  }

  function getShortId(fullId?: string): string {
    if (!fullId) return 'unknown';
    const clean = fullId.replace(/^sha256:/, '');
    return clean.slice(0, 12);
  }

  function copyToClipboard(text: string, id: string) {
    navigator.clipboard.writeText(text);
    copiedId = id;
    setTimeout(() => {
      if (copiedId === id) copiedId = null;
    }, 2000);
  }

  function parseRepoTag(repoTag?: string): { repo: string; tag: string; isDangling: boolean } {
    if (!repoTag || repoTag === '<none>:<none>') {
      return { repo: '<none>', tag: '<none>', isDangling: true };
    }
    const lastColon = repoTag.lastIndexOf(':');
    if (lastColon === -1) {
      return { repo: repoTag, tag: 'latest', isDangling: false };
    }
    return {
      repo: repoTag.slice(0, lastColon),
      tag: repoTag.slice(lastColon + 1),
      isDangling: false,
    };
  }

  function getContainersUsingImage(img: Image): number {
    if (typeof img.Containers === 'number' && img.Containers > 0) {
      return img.Containers;
    }
    // Match against loaded containers list
    const cleanId = img.Id.replace(/^sha256:/, '');
    return containers.filter(c => {
      const cImgId = (c.ImageID || '').replace(/^sha256:/, '');
      if (cImgId && cImgId.startsWith(cleanId)) return true;
      if (c.Image && img.RepoTags?.includes(c.Image)) return true;
      return false;
    }).length;
  }

  // Filtered & sorted images
  let filteredImages = $derived.by(() => {
    let result = images.filter(img => {
      const tags = img.RepoTags && img.RepoTags.length > 0 ? img.RepoTags : ['<none>:<none>'];
      const isDangling = tags.every(t => t === '<none>:<none>');

      if (filterType === 'tagged' && isDangling) return false;
      if (filterType === 'dangling' && !isDangling) return false;

      if (!searchQuery.trim()) return true;
      const q = searchQuery.toLowerCase().trim();

      const idMatch = img.Id.toLowerCase().includes(q);
      const tagMatch = tags.some(t => t.toLowerCase().includes(q));
      return idMatch || tagMatch;
    });

    result.sort((a, b) => {
      if (sortBy === 'created') return b.Created - a.Created;
      if (sortBy === 'created-asc') return a.Created - b.Created;
      if (sortBy === 'size-desc') return b.Size - a.Size;
      if (sortBy === 'size-asc') return a.Size - b.Size;
      if (sortBy === 'name') {
        const nameA = a.RepoTags?.[0] || '<none>';
        const nameB = b.RepoTags?.[0] || '<none>';
        return nameA.localeCompare(nameB);
      }
      if (sortBy === 'name-desc') {
        const nameA = a.RepoTags?.[0] || '<none>';
        const nameB = b.RepoTags?.[0] || '<none>';
        return nameB.localeCompare(nameA);
      }
      if (sortBy === 'in-use-desc') {
        const countA = getContainersUsingImage(a);
        const countB = getContainersUsingImage(b);
        if (countA !== countB) return countB - countA;
        return b.Created - a.Created;
      }
      if (sortBy === 'in-use-asc') {
        const countA = getContainersUsingImage(a);
        const countB = getContainersUsingImage(b);
        if (countA !== countB) return countA - countB;
        return a.Created - b.Created;
      }
      return 0;
    });

    return result;
  });

  let totalSize = $derived(
    images.reduce((acc, img) => acc + (img.Size || 0), 0)
  );

  let totalDangling = $derived(
    images.filter(img => !img.RepoTags || img.RepoTags.length === 0 || img.RepoTags.every(t => t === '<none>:<none>')).length
  );

  const allSelected = $derived(
    filteredImages.length > 0 && filteredImages.every((img) => selectedIds.includes(img.Id))
  );

  const isIndeterminate = $derived(
    filteredImages.some((img) => selectedIds.includes(img.Id)) && !allSelected
  );

  const selectedTotalSize = $derived(
    images
      .filter((img) => selectedIds.includes(img.Id))
      .reduce((sum, img) => sum + (img.Size || 0), 0)
  );

  const selectedImagesInUse = $derived(
    images
      .filter((img) => selectedIds.includes(img.Id))
      .filter((img) => getContainersUsingImage(img) > 0)
  );

  $effect(() => {
    if (selectAllEl) {
      selectAllEl.indeterminate = isIndeterminate;
    }
  });

  function toggleSelect(id: string) {
    if (selectedIds.includes(id)) {
      selectedIds = selectedIds.filter((x) => x !== id);
    } else {
      selectedIds = [...selectedIds, id];
    }
  }

  function toggleSelectAll() {
    if (allSelected) {
      const currentFilteredIds = new Set(filteredImages.map((img) => img.Id));
      selectedIds = selectedIds.filter((id) => !currentFilteredIds.has(id));
    } else {
      const currentFilteredIds = filteredImages.map((img) => img.Id);
      const merged = new Set([...selectedIds, ...currentFilteredIds]);
      selectedIds = Array.from(merged);
    }
  }

  function clearSelection() {
    selectedIds = [];
  }

  function openBatchDeleteModal() {
    if (selectedIds.length === 0) return;
    batchForce = false;
    isBatchDeleteModalOpen = true;
  }

  async function executeBatchDelete() {
    if (selectedIds.length === 0) return;
    isBatchBusy = true;

    let successCount = 0;
    let errorCount = 0;
    const errors: string[] = [];

    for (const id of selectedIds) {
      try {
        await imageApi.remove(id, batchForce);
        successCount++;
      } catch (err: unknown) {
        errorCount++;
        const msg = err instanceof Error ? err.message : 'Unknown error';
        errors.push(msg);
      }
    }

    isBatchBusy = false;
    isBatchDeleteModalOpen = false;

    if (errorCount === 0) {
      showFeedback('success', `Successfully removed ${successCount} image${successCount === 1 ? '' : 's'}.`);
      selectedIds = [];
    } else if (successCount > 0) {
      showFeedback('error', `Removed ${successCount} image(s), but ${errorCount} failed: ${errors[0]}`);
    } else {
      showFeedback('error', `Failed to remove images: ${errors[0]}`);
    }

    await fetchImages(true);
  }

  // Actions
  function openDeleteModal(img: Image) {
    deleteImageTarget = img;
    deleteForce = false;
  }

  function closeDeleteModal() {
    deleteImageTarget = null;
    deleteForce = false;
    isDeleting = false;
  }

  async function confirmDeleteImage() {
    if (!deleteImageTarget) return;
    isDeleting = true;

    try {
      await imageApi.remove(deleteImageTarget.Id, deleteForce);
      const label = deleteImageTarget.RepoTags?.[0] || getShortId(deleteImageTarget.Id);
      showFeedback('success', `Image ${label} deleted successfully.`);
      closeDeleteModal();
      await fetchImages(true);
    } catch (err: unknown) {
      if (err instanceof Error) {
        showFeedback('error', `Failed to delete image: ${err.message}`);
      } else {
        showFeedback('error', 'Failed to delete image.');
      }
    } finally {
      isDeleting = false;
    }
  }

  async function confirmPrune() {
    isPruning = true;
    try {
      const res = await imageApi.prune();
      const count = res.ImagesDeleted?.length || 0;
      const space = formatBytes(res.SpaceReclaimed || 0);
      showFeedback('success', `Pruned ${count} unused image${count === 1 ? '' : 's'}. Reclaimed ${space}.`);
      isPruneModalOpen = false;
      await fetchImages(true);
    } catch (err: unknown) {
      if (err instanceof Error) {
        showFeedback('error', `Failed to prune images: ${err.message}`);
      } else {
        showFeedback('error', 'Failed to prune images.');
      }
    } finally {
      isPruning = false;
    }
  }
</script>

<div class="space-y-6">
  <!-- Top Header Banner -->
  <div class="flex flex-col md:flex-row md:items-center justify-between gap-4">
    <div class="flex items-center gap-3">
      <div class="w-10 h-10 rounded-2xl bg-gradient-to-br from-violet-600/30 to-indigo-700/20 border border-violet-500/40 flex items-center justify-center text-violet-400 shadow-[0_0_20px_rgba(124,58,237,0.3)]">
        <Disc class="w-5 h-5" />
      </div>
      <div>
        <div class="flex items-center gap-2">
          <h1 class="text-xl font-bold tracking-tight text-white">Images</h1>
          <span class="px-2 py-0.5 text-[10px] font-mono rounded-full bg-violet-500/20 text-violet-300 border border-violet-500/30 font-semibold">
            {images.length}
          </span>
        </div>
        <p class="text-xs text-slate-400">
          Manage local Docker images, inspect layer storage, and pull remote registries
        </p>
      </div>
    </div>

    <!-- Header Actions -->
    <div class="flex items-center gap-2.5">
      <button
        onclick={() => isPruneModalOpen = true}
        class="px-3.5 py-2 rounded-xl bg-white/[0.04] hover:bg-white/[0.08] text-slate-300 hover:text-white border border-white/[0.08] text-xs font-medium transition flex items-center gap-2"
        title="Prune unused images"
      >
        <Scissors class="w-3.5 h-3.5 text-amber-400" />
        <span>Prune Unused</span>
        {#if totalDangling > 0}
          <span class="px-1.5 py-0.2 rounded-full bg-amber-500/20 text-amber-300 text-[10px] font-mono border border-amber-500/30">
            {totalDangling}
          </span>
        {/if}
      </button>

      <button
        onclick={() => fetchImages()}
        disabled={isRefreshing}
        class="p-2 rounded-xl bg-white/[0.04] hover:bg-white/[0.08] text-slate-400 hover:text-white border border-white/[0.08] transition disabled:opacity-50"
        title="Refresh image list"
        aria-label="Refresh images"
      >
        <RefreshCw class="w-4 h-4 {isRefreshing ? 'animate-spin text-violet-400' : ''}" />
      </button>

      <button
        onclick={checkAllUpdates}
        disabled={isCheckingAllUpdates || isRefreshing}
        class="px-3.5 py-2 rounded-xl bg-cyan-500/10 hover:bg-cyan-500/20 text-cyan-300 hover:text-white border border-cyan-500/30 text-xs font-medium transition flex items-center gap-1.5 cursor-pointer shadow-[0_0_12px_rgba(6,182,212,0.15)] disabled:opacity-50"
        title="Check remote registries for image updates"
      >
        <Sparkles class="w-3.5 h-3.5 text-cyan-400 {isCheckingAllUpdates ? 'animate-spin' : ''}" />
        <span>Check Updates</span>
      </button>

      <button
        onclick={() => isRegistriesModalOpen = true}
        class="px-3.5 py-2 rounded-xl bg-white/[0.04] hover:bg-white/[0.08] text-slate-300 hover:text-white border border-white/[0.08] text-xs font-medium transition flex items-center gap-1.5"
        title="Manage Private Container Registries"
      >
        <Key class="w-3.5 h-3.5 text-violet-400" />
        <span>Registries</span>
      </button>

      <button
        onclick={() => isBuildModalOpen = true}
        class="px-3.5 py-2 rounded-xl bg-white/[0.04] hover:bg-white/[0.08] text-slate-300 hover:text-white border border-white/[0.08] text-xs font-medium transition flex items-center gap-1.5 group"
        title="Open Dockerfile Studio to build a custom Docker image"
      >
        <Hammer class="w-3.5 h-3.5 text-cyan-400 group-hover:rotate-12 transition-transform" />
        <span>Build Image</span>
      </button>

      <button
        onclick={() => isPullModalOpen = true}
        class="px-4 py-2 rounded-xl bg-violet-600 hover:bg-violet-500 text-white text-xs font-semibold shadow-[0_0_20px_rgba(124,58,237,0.4)] transition flex items-center gap-2 group"
      >
        <Download class="w-3.5 h-3.5 group-hover:-translate-y-0.5 transition-transform" />
        Pull Image
      </button>
    </div>
  </div>

  <!-- Feedback Toast / Banner -->
  {#if feedback}
    <div
      class="p-3.5 rounded-xl border flex items-center justify-between text-xs animate-in fade-in duration-200 {feedback.type === 'success'
        ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-300'
        : feedback.type === 'info'
        ? 'bg-cyan-500/10 border-cyan-500/30 text-cyan-300 shadow-[0_0_15px_rgba(6,182,212,0.15)]'
        : 'bg-rose-500/10 border-rose-500/30 text-rose-300'}"
    >
      <div class="flex items-center gap-2">
        {#if feedback.type === 'success'}
          <CheckCircle2 class="w-4 h-4 text-emerald-400 shrink-0" />
        {:else if feedback.type === 'info'}
          <Sparkles class="w-4 h-4 text-cyan-400 shrink-0 animate-pulse" />
        {:else}
          <AlertCircle class="w-4 h-4 text-rose-400 shrink-0" />
        {/if}
        <span>{feedback.message}</span>
      </div>
      <button
        onclick={() => feedback = null}
        class="p-1 hover:bg-white/10 rounded-lg text-slate-400 hover:text-white"
        aria-label="Close notification"
      >
        <X class="w-3.5 h-3.5" />
      </button>
    </div>
  {/if}

  {#if error}
    <div class="p-4 rounded-xl bg-rose-500/10 border border-rose-500/30 flex items-start gap-3 text-rose-300">
      <AlertCircle class="w-5 h-5 shrink-0 text-rose-400 mt-0.5" />
      <div>
        <h3 class="text-xs font-bold text-rose-200">Image Service Error</h3>
        <p class="text-xs mt-0.5">{error}</p>
      </div>
    </div>
  {/if}

  <!-- Stats Strip -->
  <div class="grid grid-cols-2 md:grid-cols-4 gap-3">
    <div class="p-4 rounded-xl bg-[#12131c] border border-white/[0.08] flex items-center gap-3">
      <div class="w-9 h-9 rounded-xl bg-violet-500/10 border border-violet-500/20 flex items-center justify-center text-violet-400">
        <Disc class="w-4 h-4" />
      </div>
      <div>
        <span class="text-[11px] text-slate-400 block font-mono">Total Images</span>
        <span class="text-lg font-bold text-white font-mono">{images.length}</span>
      </div>
    </div>

    <div class="p-4 rounded-xl bg-[#12131c] border border-white/[0.08] flex items-center gap-3">
      <div class="w-9 h-9 rounded-xl bg-cyan-500/10 border border-cyan-500/20 flex items-center justify-center text-cyan-400">
        <HardDrive class="w-4 h-4" />
      </div>
      <div>
        <span class="text-[11px] text-slate-400 block font-mono">Total Size</span>
        <span class="text-lg font-bold text-white font-mono">{formatBytes(totalSize)}</span>
      </div>
    </div>

    <div class="p-4 rounded-xl bg-[#12131c] border border-white/[0.08] flex items-center gap-3">
      <div class="w-9 h-9 rounded-xl bg-emerald-500/10 border border-emerald-500/20 flex items-center justify-center text-emerald-400">
        <Layers class="w-4 h-4" />
      </div>
      <div>
        <span class="text-[11px] text-slate-400 block font-mono">Tagged Images</span>
        <span class="text-lg font-bold text-emerald-400 font-mono">{images.length - totalDangling}</span>
      </div>
    </div>

    <div class="p-4 rounded-xl bg-[#12131c] border border-white/[0.08] flex items-center gap-3">
      <div class="w-9 h-9 rounded-xl bg-amber-500/10 border border-amber-500/20 flex items-center justify-center text-amber-400">
        <Scissors class="w-4 h-4" />
      </div>
      <div>
        <span class="text-[11px] text-slate-400 block font-mono">Dangling / Unused</span>
        <span class="text-lg font-bold text-amber-400 font-mono">{totalDangling}</span>
      </div>
    </div>
  </div>

  <!-- Search & Filter Controls -->
  <div class="p-3 rounded-2xl bg-[#12131c] border border-white/[0.08] flex flex-col md:flex-row items-center justify-between gap-3">
    <!-- Search Bar -->
    <div class="relative w-full md:w-96">
      <Search class="w-4 h-4 text-slate-500 absolute left-3.5 top-1/2 -translate-y-1/2" />
      <input
        type="text"
        bind:value={searchQuery}
        placeholder="Filter by image name, tag, or ID..."
        class="w-full pl-9 pr-8 py-2 rounded-xl bg-[#0c0d14] border border-white/10 text-white placeholder-slate-500 text-xs focus:outline-none focus:border-violet-500 transition font-mono"
      />
      {#if searchQuery}
        <button
          onclick={() => searchQuery = ''}
          class="absolute right-3 top-1/2 -translate-y-1/2 text-slate-500 hover:text-white"
          aria-label="Clear search"
        >
          <X class="w-3.5 h-3.5" />
        </button>
      {/if}
    </div>

    <!-- Filter Buttons & Sort Dropdown -->
    <div class="flex items-center gap-2 w-full md:w-auto overflow-x-auto pb-1 md:pb-0">
      <div class="flex items-center bg-[#0c0d14] p-1 rounded-xl border border-white/10 text-xs">
        <button
          onclick={() => filterType = 'all'}
          class="px-3 py-1 rounded-lg transition font-medium {filterType === 'all' ? 'bg-violet-600 text-white shadow-[0_0_10px_rgba(124,58,237,0.3)]' : 'text-slate-400 hover:text-white'}"
        >
          All ({images.length})
        </button>
        <button
          onclick={() => filterType = 'tagged'}
          class="px-3 py-1 rounded-lg transition font-medium {filterType === 'tagged' ? 'bg-violet-600 text-white shadow-[0_0_10px_rgba(124,58,237,0.3)]' : 'text-slate-400 hover:text-white'}"
        >
          Tagged
        </button>
        <button
          onclick={() => filterType = 'dangling'}
          class="px-3 py-1 rounded-lg transition font-medium {filterType === 'dangling' ? 'bg-violet-600 text-white shadow-[0_0_10px_rgba(124,58,237,0.3)]' : 'text-slate-400 hover:text-white'}"
        >
          Dangling ({totalDangling})
        </button>
      </div>

      <select
        bind:value={sortBy}
        class="px-3 py-2 rounded-xl bg-[#0c0d14] border border-white/10 text-slate-300 text-xs focus:outline-none focus:border-violet-500 cursor-pointer"
      >
        <option value="created">Newest First</option>
        <option value="created-asc">Oldest First</option>
        <option value="size-desc">Size: Largest First</option>
        <option value="size-asc">Size: Smallest First</option>
        <option value="name">Name (A-Z)</option>
        <option value="name-desc">Name (Z-A)</option>
        <option value="in-use-desc">Most Containers Using</option>
        <option value="in-use-asc">Unused / Least Using</option>
      </select>
    </div>
  </div>

  <!-- Batch Actions Toolbar (Visible when 1+ selected) -->
  {#if selectedIds.length > 0}
    <div class="p-3.5 rounded-2xl bg-gradient-to-r from-violet-950/70 via-[#12131c] to-indigo-950/70 border border-violet-500/40 shadow-[0_0_25px_rgba(124,58,237,0.25)] backdrop-blur-md flex flex-wrap items-center justify-between gap-3 animate-in fade-in slide-in-from-top-2 duration-150">
      <div class="flex items-center gap-2.5">
        <span class="w-2 h-2 rounded-full bg-cyan-400 shadow-[0_0_8px_#00f5ff]"></span>
        <span class="text-xs font-bold text-white font-mono">
          {selectedIds.length} {selectedIds.length === 1 ? 'image' : 'images'} selected
        </span>
        <span class="text-xs text-slate-400 font-mono">({formatBytes(selectedTotalSize)})</span>
        <button
          type="button"
          onclick={clearSelection}
          class="text-[11px] font-mono text-slate-400 hover:text-cyan-300 underline ml-2 cursor-pointer"
        >
          Deselect
        </button>
      </div>

      <div class="flex items-center gap-2">
        <button
          type="button"
          onclick={openBatchDeleteModal}
          class="px-3.5 py-1.5 rounded-xl bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold transition inline-flex items-center gap-1.5 cursor-pointer shadow-[0_0_15px_rgba(244,63,94,0.3)]"
          title="Remove selected images"
        >
          <Trash2 class="w-3.5 h-3.5" />
          <span>Remove Selected ({selectedIds.length})</span>
        </button>
      </div>
    </div>
  {/if}

  <!-- Images Table Container -->
  <div class="rounded-2xl bg-[#12131c] border border-white/[0.08] overflow-hidden shadow-xl">
    {#if isLoading}
      <div class="p-12 flex flex-col items-center justify-center gap-3 text-slate-400">
        <Loader2 class="w-7 h-7 text-violet-400 animate-spin" />
        <span class="text-xs font-mono">Querying Docker Engine images...</span>
      </div>
    {:else if filteredImages.length === 0}
      <div class="p-12 text-center space-y-3">
        <div class="w-12 h-12 mx-auto rounded-2xl bg-white/[0.04] border border-white/[0.08] flex items-center justify-center text-slate-500">
          <Disc class="w-6 h-6" />
        </div>
        <h3 class="text-sm font-semibold text-white">No images found</h3>
        <p class="text-xs text-slate-400 max-w-sm mx-auto">
          {#if searchQuery}
            No images match your search query "{searchQuery}".
          {:else}
            There are no Docker images present on this host yet.
          {/if}
        </p>
        <div class="pt-2">
          <button
            onclick={() => isPullModalOpen = true}
            class="px-4 py-2 rounded-xl bg-violet-600 hover:bg-violet-500 text-white text-xs font-semibold shadow-[0_0_15px_rgba(124,58,237,0.35)] transition inline-flex items-center gap-2"
          >
            <Download class="w-3.5 h-3.5" />
            Pull Your First Image
          </button>
        </div>
      </div>
    {:else}
      <div class="overflow-x-auto">
        <table class="w-full text-left border-collapse text-xs">
          <thead>
            <tr class="border-b border-white/[0.08] bg-[#0c0d14]/60 text-slate-400 font-mono text-[11px] uppercase tracking-wider select-none">
              <!-- Checkbox column -->
              <th class="py-3 px-4 w-10 text-center">
                <label class="relative inline-flex items-center cursor-pointer">
                  <input
                    bind:this={selectAllEl}
                    type="checkbox"
                    checked={allSelected}
                    onchange={toggleSelectAll}
                    class="sr-only peer"
                  />
                  <div class="w-4 h-4 rounded border border-white/20 peer-checked:border-violet-500 peer-checked:bg-violet-600 flex items-center justify-center transition bg-[#181926]">
                    {#if allSelected}
                      <svg class="w-3 h-3 text-white" viewBox="0 0 14 14" fill="none">
                        <path d="M3 7.5L5.5 10L11 4.5" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
                      </svg>
                    {:else if isIndeterminate}
                      <div class="w-2 h-0.5 bg-violet-400 rounded-sm"></div>
                    {/if}
                  </div>
                </label>
              </th>
              <th class="py-3 px-4 font-semibold">
                <button
                  type="button"
                  onclick={() => toggleImageSort('name')}
                  class="flex items-center gap-1.5 hover:text-white transition group focus:outline-none"
                  title="Sort by repository & tag"
                >
                  <span>Repository & Tag</span>
                  {#if sortBy === 'name'}
                    <ArrowUp class="w-3.5 h-3.5 text-cyan-400" />
                  {:else if sortBy === 'name-desc'}
                    <ArrowDown class="w-3.5 h-3.5 text-cyan-400" />
                  {:else}
                    <ArrowUpDown class="w-3 h-3 text-slate-600 group-hover:text-slate-400 transition" />
                  {/if}
                </button>
              </th>
              <th class="py-3 px-4 font-semibold">Image ID</th>
              <th class="py-3 px-4 font-semibold">
                <button
                  type="button"
                  onclick={() => toggleImageSort('size')}
                  class="flex items-center gap-1.5 hover:text-white transition group focus:outline-none"
                  title="Sort by size"
                >
                  <span>Size</span>
                  {#if sortBy === 'size-asc'}
                    <ArrowUp class="w-3.5 h-3.5 text-cyan-400" />
                  {:else if sortBy === 'size-desc'}
                    <ArrowDown class="w-3.5 h-3.5 text-cyan-400" />
                  {:else}
                    <ArrowUpDown class="w-3 h-3 text-slate-600 group-hover:text-slate-400 transition" />
                  {/if}
                </button>
              </th>
              <th class="py-3 px-4 font-semibold">
                <button
                  type="button"
                  onclick={() => toggleImageSort('created')}
                  class="flex items-center gap-1.5 hover:text-white transition group focus:outline-none"
                  title="Sort by creation date"
                >
                  <span>Created</span>
                  {#if sortBy === 'created-asc'}
                    <ArrowUp class="w-3.5 h-3.5 text-cyan-400" />
                  {:else if sortBy === 'created'}
                    <ArrowDown class="w-3.5 h-3.5 text-cyan-400" />
                  {:else}
                    <ArrowUpDown class="w-3 h-3 text-slate-600 group-hover:text-slate-400 transition" />
                  {/if}
                </button>
              </th>
              <th class="py-3 px-4 font-semibold">
                <button
                  type="button"
                  onclick={() => toggleImageSort('in-use')}
                  class="flex items-center gap-1.5 hover:text-white transition group focus:outline-none"
                  title="Sort by number of containers using this image"
                >
                  <span>In Use</span>
                  {#if sortBy === 'in-use-asc'}
                    <ArrowUp class="w-3.5 h-3.5 text-cyan-400" />
                  {:else if sortBy === 'in-use-desc'}
                    <ArrowDown class="w-3.5 h-3.5 text-cyan-400" />
                  {:else}
                    <ArrowUpDown class="w-3 h-3 text-slate-600 group-hover:text-slate-400 transition" />
                  {/if}
                </button>
              </th>
              <th class="py-3 px-4 font-semibold text-right">Actions</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-white/[0.04]">
            {#each filteredImages as img (img.Id)}
              {@const isSelected = selectedIds.includes(img.Id)}
              {@const repoTags = img.RepoTags && img.RepoTags.length > 0 ? img.RepoTags : ['<none>:<none>']}
              {@const primaryTag = parseRepoTag(repoTags[0])}
              {@const shortId = getShortId(img.Id)}
              {@const inUseCount = getContainersUsingImage(img)}
              <tr class="hover:bg-white/[0.02] transition-colors group {isSelected ? 'bg-violet-900/15' : ''}">
                <!-- Checkbox -->
                <td class="py-3 px-4 w-10 text-center">
                  <label class="relative inline-flex items-center cursor-pointer">
                    <input
                      type="checkbox"
                      checked={isSelected}
                      onchange={() => toggleSelect(img.Id)}
                      class="sr-only peer"
                    />
                    <div class="w-4 h-4 rounded border border-white/20 peer-checked:border-violet-500 peer-checked:bg-violet-600 flex items-center justify-center transition bg-[#181926]">
                      {#if isSelected}
                        <svg class="w-3 h-3 text-white" viewBox="0 0 14 14" fill="none">
                          <path d="M3 7.5L5.5 10L11 4.5" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
                        </svg>
                      {/if}
                    </div>
                  </label>
                </td>
                <!-- Repository & Tag -->
                <td class="py-3 px-4">
                  <div class="flex items-start gap-2.5">
                    <div class="w-8 h-8 rounded-xl bg-[#090a0f] border border-white/[0.08] flex items-center justify-center text-slate-400 group-hover:text-violet-400 group-hover:border-violet-500/30 transition shrink-0 mt-0.5">
                      <Disc class="w-4 h-4" />
                    </div>
                    <div class="min-w-0">
                      {#if primaryTag.isDangling}
                        <div class="flex items-center gap-1.5">
                          <span class="font-mono text-amber-300 font-semibold">&lt;none&gt;</span>
                          <span class="px-1.5 py-0.2 rounded bg-amber-500/10 text-amber-400 border border-amber-500/20 text-[10px] font-mono">dangling</span>
                        </div>
                      {:else}
                        <div class="font-semibold text-white tracking-wide truncate max-w-md">
                          {primaryTag.repo}
                        </div>
                        <div class="flex flex-wrap items-center gap-1.5 mt-1.5">
                          {#each repoTags as fullTag}
                            {@const parsed = parseRepoTag(fullTag)}
                            {@const update = imageUpdateStore.getStatus(fullTag)}
                            {@const isChecking = imageUpdateStore.isChecking(fullTag)}
                            <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-md bg-cyan-500/10 text-cyan-300 border border-cyan-500/20 font-mono text-[10px]">
                              <Tag class="w-2.5 h-2.5" />
                              {parsed.tag}
                            </span>

                            {#if isUpdatingTag(fullTag)}
                              <span
                                class="inline-flex items-center gap-1 px-2 py-0.5 rounded-md bg-amber-500/20 text-amber-300 border border-amber-500/40 font-mono text-[10px] shadow-[0_0_8px_rgba(245,158,11,0.25)] animate-pulse"
                                title="Updating image from remote registry in background..."
                              >
                                <Loader2 class="w-2.5 h-2.5 animate-spin text-amber-400" />
                                <span>Updating...</span>
                              </span>
                            {:else if isChecking}
                              <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-md bg-cyan-500/10 text-cyan-300 border border-cyan-500/20 font-mono text-[10px]">
                                <Loader2 class="w-2.5 h-2.5 animate-spin text-cyan-400" />
                                Checking registry...
                              </span>
                            {:else if update}
                              {#if update.status === 'update_available'}
                                <button
                                  type="button"
                                  onclick={() => openUpdateModal(fullTag, img)}
                                  class="inline-flex items-center gap-1 px-2 py-0.5 rounded-md bg-amber-500/20 text-amber-300 border border-amber-500/40 hover:bg-amber-500/30 hover:border-amber-500/60 font-mono text-[10px] shadow-[0_0_8px_rgba(245,158,11,0.25)] transition cursor-pointer"
                                  title="Remote registry has a newer version! Click to update now."
                                >
                                  <ArrowUpCircle class="w-2.5 h-2.5 text-amber-400" />
                                  <span>Update Available</span>
                                </button>
                              {:else if update.status === 'up_to_date'}
                                <span
                                  class="inline-flex items-center gap-1 px-2 py-0.5 rounded-md bg-emerald-500/15 text-emerald-300 border border-emerald-500/30 font-mono text-[10px]"
                                  title="Matches remote registry digest"
                                >
                                  <CheckCircle2 class="w-2.5 h-2.5 text-emerald-400" />
                                  <span>Up to date</span>
                                </span>
                              {/if}
                            {/if}
                          {/each}
                        </div>
                      {/if}
                    </div>
                  </div>
                </td>

                <!-- Image ID -->
                <td class="py-3 px-4 font-mono text-slate-300 whitespace-nowrap">
                  <div class="flex items-center gap-1.5">
                    <span class="text-violet-300 font-semibold">{shortId}</span>
                    <button
                      onclick={() => copyToClipboard(img.Id, img.Id)}
                      class="p-1 rounded hover:bg-white/10 text-slate-500 hover:text-white transition"
                      title="Copy full Image ID"
                      aria-label="Copy full Image ID"
                    >
                      {#if copiedId === img.Id}
                        <Check class="w-3 h-3 text-emerald-400" />
                      {:else}
                        <Copy class="w-3 h-3" />
                      {/if}
                    </button>
                  </div>
                </td>

                <!-- Size -->
                <td class="py-3 px-4 font-mono text-slate-300 whitespace-nowrap">
                  <div class="flex items-center gap-1.5">
                    <span class="font-semibold text-white">{formatBytes(img.Size)}</span>
                    {#if img.SharedSize && img.SharedSize > 0}
                      <span class="text-[10px] text-slate-500">({formatBytes(img.SharedSize)} shared)</span>
                    {/if}
                  </div>
                </td>

                <!-- Created -->
                <td class="py-3 px-4 font-mono text-slate-400 whitespace-nowrap">
                  <div class="flex items-center gap-1 text-[11px]" title={new Date(img.Created * 1000).toLocaleString()}>
                    <Clock class="w-3 h-3 text-slate-500" />
                    <span>{formatRelativeTime(img.Created)}</span>
                  </div>
                </td>

                <!-- In Use -->
                <td class="py-3 px-4 whitespace-nowrap">
                  {#if inUseCount > 0}
                    <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 font-mono text-[10px]">
                      <span class="w-1.5 h-1.5 rounded-full bg-emerald-400"></span>
                      {inUseCount} container{inUseCount === 1 ? '' : 's'}
                    </span>
                  {:else}
                    <span class="text-slate-500 font-mono text-[11px]">Unused</span>
                  {/if}
                </td>

                <!-- Actions -->
                <td class="py-3 px-4 text-right whitespace-nowrap">
                  <div class="flex items-center justify-end gap-1.5">
                    {#if !primaryTag.isDangling}
                      {@const targetRef = repoTags[0]}
                      {@const isChecking = imageUpdateStore.isChecking(targetRef)}
                      {@const hasUpdate = imageUpdateStore.hasUpdate(targetRef)}
                      {@const isUpd = isUpdatingTag(targetRef)}

                      {#if hasUpdate}
                        <button
                          type="button"
                          onclick={() => openUpdateModal(targetRef, img)}
                          disabled={isUpd}
                          class="p-1.5 rounded-lg bg-amber-500/15 hover:bg-amber-500/25 text-amber-300 border border-amber-500/30 hover:border-amber-500/50 shadow-[0_0_10px_rgba(245,158,11,0.2)] transition disabled:opacity-50 cursor-pointer animate-pulse"
                          title="Pull latest update for {targetRef}"
                          aria-label="Update image"
                        >
                          {#if isUpd}
                            <Loader2 class="w-3.5 h-3.5 animate-spin text-amber-400" />
                          {:else}
                            <ArrowUpCircle class="w-3.5 h-3.5 text-amber-400" />
                          {/if}
                        </button>
                      {/if}

                      <button
                        type="button"
                        onclick={() => checkUpdateForImage(targetRef)}
                        disabled={isChecking || isUpd}
                        class="p-1.5 rounded-lg bg-cyan-500/10 hover:bg-cyan-500/20 text-cyan-400 border border-cyan-500/20 hover:border-cyan-500/40 transition disabled:opacity-50 cursor-pointer"
                        title="Check remote registry for update"
                        aria-label="Check image update"
                      >
                        <Sparkles class="w-3.5 h-3.5 {isChecking ? 'animate-spin' : ''}" />
                      </button>
                    {/if}

                    <button
                      onclick={() => openDeleteModal(img)}
                      class="p-1.5 rounded-lg bg-rose-500/10 hover:bg-rose-500/20 text-rose-400 border border-rose-500/20 hover:border-rose-500/40 transition cursor-pointer"
                      title="Delete image"
                      aria-label="Delete image"
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
    {/if}
  </div>
</div>

<!-- Pull Image Modal -->
<PullImageModal
  isOpen={isPullModalOpen}
  initialRef={pullModalInitialRef}
  onClose={() => {
    isPullModalOpen = false;
    pullModalInitialRef = '';
  }}
  onPulled={(ref) => {
    showFeedback('success', `Image ${ref} successfully pulled.`);
    fetchImages(true);
    imageUpdateStore.checkUpdate(ref);
  }}
/>

<!-- Build Image Modal -->
<ImageBuildModal
  isOpen={isBuildModalOpen}
  onClose={() => isBuildModalOpen = false}
  onBuilt={(builtTag) => {
    showFeedback('success', `Image ${builtTag} successfully built.`);
    fetchImages(true);
  }}
/>

<!-- Private Registries Modal -->
<RegistriesManager
  isModal={true}
  isOpen={isRegistriesModalOpen}
  onClose={() => isRegistriesModalOpen = false}
/>

<!-- Delete Image Confirmation Modal -->
{#if deleteImageTarget}
  {@const primaryTag = parseRepoTag(deleteImageTarget.RepoTags?.[0])}
  {@const shortId = getShortId(deleteImageTarget.Id)}
  {@const inUse = getContainersUsingImage(deleteImageTarget)}
  <div
    class="fixed inset-0 z-50 flex items-center justify-center p-4"
    role="dialog"
    aria-modal="true"
    aria-labelledby="delete-image-modal-title"
    tabindex="-1"
  >
    <button
      type="button"
      class="fixed inset-0 bg-black/80 backdrop-blur-md cursor-default border-0"
      onclick={closeDeleteModal}
      aria-label="Close modal backdrop"
      tabindex="-1"
    ></button>

    <div
      class="relative w-full max-w-md rounded-2xl bg-[#12131c] border border-white/[0.12] shadow-[0_10px_50px_rgba(0,0,0,0.8)] overflow-hidden p-6 space-y-4 animate-in fade-in zoom-in-95 duration-150"
    >
      <div class="flex items-center gap-3">
        <div class="w-9 h-9 rounded-xl bg-rose-500/15 border border-rose-500/30 flex items-center justify-center text-rose-400 shadow-[0_0_15px_rgba(244,63,94,0.3)]">
          <Trash2 class="w-4 h-4" />
        </div>
        <div>
          <h2 id="delete-image-modal-title" class="text-sm font-bold text-white">Delete Docker Image</h2>
          <p class="text-[11px] text-slate-400 font-mono">{primaryTag.repo}:{primaryTag.tag} ({shortId})</p>
        </div>
      </div>

      {#if inUse > 0}
        <div class="p-3 rounded-xl bg-amber-500/10 border border-amber-500/30 flex items-start gap-2 text-amber-300 text-xs">
          <AlertTriangle class="w-4 h-4 text-amber-400 shrink-0 mt-0.5" />
          <span>
            This image is currently in use by <strong>{inUse} container{inUse === 1 ? '' : 's'}</strong>. Deleting it may fail unless you force removal.
          </span>
        </div>
      {/if}

      <div class="space-y-3 text-xs">
        <label class="flex items-center gap-2 cursor-pointer select-none text-slate-300 hover:text-white">
          <input
            type="checkbox"
            bind:checked={deleteForce}
            class="rounded border-white/20 bg-[#0c0d14] text-violet-600 focus:ring-0 focus:ring-offset-0"
          />
          <span>Force deletion (<code class="text-violet-300 font-mono">--force</code>)</span>
        </label>
      </div>

      <div class="flex items-center justify-end gap-2 pt-2 border-t border-white/[0.08]">
        <button
          type="button"
          onclick={closeDeleteModal}
          disabled={isDeleting}
          class="px-4 py-2 rounded-xl bg-white/5 hover:bg-white/10 text-slate-300 text-xs font-medium transition"
        >
          Cancel
        </button>
        <button
          type="button"
          onclick={confirmDeleteImage}
          disabled={isDeleting}
          class="px-4 py-2 rounded-xl bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold shadow-[0_0_15px_rgba(244,63,94,0.35)] transition flex items-center gap-2"
        >
          {#if isDeleting}
            <Loader2 class="w-3.5 h-3.5 animate-spin" />
            Deleting...
          {:else}
            <Trash2 class="w-3.5 h-3.5" />
            Delete Image
          {/if}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Prune Images Modal -->
{#if isPruneModalOpen}
  <div
    class="fixed inset-0 z-50 flex items-center justify-center p-4"
    role="dialog"
    aria-modal="true"
    aria-labelledby="prune-images-modal-title"
    tabindex="-1"
  >
    <button
      type="button"
      class="fixed inset-0 bg-black/80 backdrop-blur-md cursor-default border-0"
      onclick={() => isPruneModalOpen = false}
      aria-label="Close modal backdrop"
      tabindex="-1"
    ></button>

    <div
      class="relative w-full max-w-md rounded-2xl bg-[#12131c] border border-white/[0.12] shadow-[0_10px_50px_rgba(0,0,0,0.8)] overflow-hidden p-6 space-y-4 animate-in fade-in zoom-in-95 duration-150"
    >
      <div class="flex items-center gap-3">
        <div class="w-9 h-9 rounded-xl bg-amber-500/15 border border-amber-500/30 flex items-center justify-center text-amber-400 shadow-[0_0_15px_rgba(245,158,11,0.3)]">
          <Scissors class="w-4 h-4" />
        </div>
        <div>
          <h2 id="prune-images-modal-title" class="text-sm font-bold text-white">Prune Unused Images</h2>
          <p class="text-[11px] text-slate-400">Reclaim host disk space from dangling image layers</p>
        </div>
      </div>

      <p class="text-xs text-slate-300 leading-relaxed">
        This will remove all dangling images (untagged <code class="text-amber-300 font-mono">&lt;none&gt;</code> images) not associated with at least one container. Active images will not be touched.
      </p>

      <div class="flex items-center justify-end gap-2 pt-2 border-t border-white/[0.08]">
        <button
          type="button"
          onclick={() => isPruneModalOpen = false}
          disabled={isPruning}
          class="px-4 py-2 rounded-xl bg-white/5 hover:bg-white/10 text-slate-300 text-xs font-medium transition"
        >
          Cancel
        </button>
        <button
          type="button"
          onclick={confirmPrune}
          disabled={isPruning}
          class="px-4 py-2 rounded-xl bg-amber-600 hover:bg-amber-500 text-white text-xs font-semibold shadow-[0_0_15px_rgba(245,158,11,0.35)] transition flex items-center gap-2"
        >
          {#if isPruning}
            <Loader2 class="w-3.5 h-3.5 animate-spin" />
            Pruning...
          {:else}
            <Scissors class="w-3.5 h-3.5" />
            Prune Images
          {/if}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Batch Delete Images Confirmation Modal -->
{#if isBatchDeleteModalOpen}
  <div
    class="fixed inset-0 z-50 flex items-center justify-center p-4"
    role="dialog"
    aria-modal="true"
    aria-labelledby="batch-delete-modal-title"
    tabindex="-1"
  >
    <button
      type="button"
      class="fixed inset-0 bg-black/80 backdrop-blur-md cursor-default border-0"
      onclick={() => !isBatchBusy && (isBatchDeleteModalOpen = false)}
      aria-label="Close modal backdrop"
      tabindex="-1"
    ></button>

    <div
      class="relative w-full max-w-md rounded-2xl bg-[#12131c] border border-white/[0.12] shadow-[0_10px_50px_rgba(0,0,0,0.8)] overflow-hidden p-6 space-y-4 animate-in fade-in zoom-in-95 duration-150"
    >
      <div class="flex items-center gap-3">
        <div class="w-9 h-9 rounded-xl bg-rose-500/15 border border-rose-500/30 flex items-center justify-center text-rose-400 shadow-[0_0_15px_rgba(244,63,94,0.3)]">
          <Trash2 class="w-4 h-4" />
        </div>
        <div>
          <h2 id="batch-delete-modal-title" class="text-sm font-bold text-white">Delete Selected Images</h2>
          <p class="text-[11px] text-slate-400 font-mono">
            {selectedIds.length} image{selectedIds.length === 1 ? '' : 's'} selected &bull; {formatBytes(selectedTotalSize)} space freed
          </p>
        </div>
      </div>

      {#if selectedImagesInUse.length > 0}
        <div class="p-3 rounded-xl bg-amber-500/10 border border-amber-500/30 flex items-start gap-2 text-amber-300 text-xs">
          <AlertTriangle class="w-4 h-4 text-amber-400 shrink-0 mt-0.5" />
          <div>
            <strong>{selectedImagesInUse.length} of the selected image{selectedImagesInUse.length === 1 ? ' is' : 's are'} currently in use</strong> by running or stopped containers. Deleting them may fail unless force removal is enabled.
          </div>
        </div>
      {/if}

      <div class="space-y-3 text-xs">
        <label class="flex items-center gap-2 cursor-pointer select-none text-slate-300 hover:text-white">
          <input
            type="checkbox"
            bind:checked={batchForce}
            class="rounded border-white/20 bg-[#0c0d14] text-violet-600 focus:ring-0 focus:ring-offset-0"
          />
          <span>Force deletion (<code class="text-violet-300 font-mono">--force</code>)</span>
        </label>
      </div>

      <div class="flex items-center justify-end gap-2 pt-2 border-t border-white/[0.08]">
        <button
          type="button"
          onclick={() => isBatchDeleteModalOpen = false}
          disabled={isBatchBusy}
          class="px-4 py-2 rounded-xl bg-white/5 hover:bg-white/10 text-slate-300 text-xs font-medium transition cursor-pointer"
        >
          Cancel
        </button>
        <button
          type="button"
          onclick={executeBatchDelete}
          disabled={isBatchBusy}
          class="px-4 py-2 rounded-xl bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold shadow-[0_0_15px_rgba(244,63,94,0.35)] transition flex items-center gap-2 cursor-pointer"
        >
          {#if isBatchBusy}
            <Loader2 class="w-3.5 h-3.5 animate-spin" />
            Deleting ({selectedIds.length})...
          {:else}
            <Trash2 class="w-3.5 h-3.5" />
            Delete {selectedIds.length} Image{selectedIds.length === 1 ? '' : 's'}
          {/if}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Update Image Confirmation Modal -->
<UpdateImageModal
  isOpen={isUpdateModalOpen}
  imageRef={updateTarget?.ref || ''}
  image={updateTarget?.image}
  containers={containers}
  isUpdating={Boolean(updateTarget && isUpdatingTag(updateTarget.ref))}
  onConfirm={confirmUpdateImage}
  onClose={() => {
    isUpdateModalOpen = false;
    updateTarget = null;
    isConfirmUpdating = false;
  }}
/>

