<script lang="ts">
  import {
    onMount } from 'svelte';
  import {
    Folder,
    FolderOpen,
    FolderUp,
    File,
    FileText,
    FileCode,
    Upload,
    Download,
    Trash,
    PenLine,
    Plus,
    RefreshCw,
    Search,
    ChevronRight,
    CircleAlert,
    CircleCheck,
    LoaderCircle,
    Home,
    AlertTriangle,
  } from '@lucide/svelte';
  import { fileApi } from '../api';
  import type { ContainerFileInfo } from '../types';
  import FileEditorModal from './FileEditorModal.svelte';

  interface Props {
    containerId: string;
    isRunning?: boolean;
  }

  let { containerId, isRunning = true }: Props = $props();

  // Navigation state
  let currentPath = $state('/');
  let pathInput = $state('/');
  let files = $state<ContainerFileInfo[]>([]);
  let isLoading = $state(false);
  let isUploading = $state(false);
  let error = $state<string | null>(null);
  let feedback = $state<{ type: 'success' | 'error'; text: string } | null>(null);

  // Search & Filter
  let searchQuery = $state('');

  // Modals state
  let editorOpen = $state(false);
  let editorFilePath = $state('');
  let editorIsNewFile = $state(false);

  let deleteTarget = $state<ContainerFileInfo | null>(null);
  let isDeleting = $state(false);

  // Hidden file input
  let fileInputRef = $state<HTMLInputElement | null>(null);

  function showFeedback(type: 'success' | 'error', text: string) {
    feedback = { type, text };
    setTimeout(() => {
      if (feedback?.text === text) {
        feedback = null;
      }
    }, 4000);
  }

  // Breadcrumbs
  let breadcrumbSegments = $derived.by(() => {
    const clean = currentPath.trim();
    if (clean === '/' || !clean) return [];
    const parts = clean.split('/').filter(Boolean);
    const result: Array<{ name: string; path: string }> = [];
    let acc = '';
    for (const part of parts) {
      acc += '/' + part;
      result.push({ name: part, path: acc });
    }
    return result;
  });

  // Filtered files
  let filteredFiles = $derived.by(() => {
    let result = files;
    if (searchQuery.trim()) {
      const q = searchQuery.toLowerCase().trim();
      result = result.filter(f => f.name.toLowerCase().includes(q));
    }
    return result;
  });

  // Total directory summary
  let dirCount = $derived(files.filter(f => f.is_dir).length);
  let fileCount = $derived(files.filter(f => !f.is_dir).length);

  async function loadDirectory(targetPath: string) {
    let clean = targetPath.trim();
    if (!clean || clean === '.') clean = '/';
    if (!clean.startsWith('/')) clean = '/' + clean;

    isLoading = true;
    error = null;

    try {
      const data = await fileApi.list(containerId, clean);
      files = data;
      currentPath = clean;
      pathInput = clean;
    } catch (err: unknown) {
      if (err instanceof Error) {
        error = err.message;
      } else {
        error = 'Failed to load directory contents.';
      }
    } finally {
      isLoading = false;
    }
  }

  onMount(() => {
    loadDirectory(currentPath);
  });

  function handleNavigateUp() {
    if (currentPath === '/') return;
    const lastSlash = currentPath.lastIndexOf('/');
    const parent = lastSlash <= 0 ? '/' : currentPath.slice(0, lastSlash);
    loadDirectory(parent);
  }

  function handlePathSubmit(e: SubmitEvent) {
    e.preventDefault();
    loadDirectory(pathInput);
  }

  function handleFileClick(file: ContainerFileInfo) {
    if (file.is_dir) {
      loadDirectory(file.path);
    } else {
      openEditor(file.path, false);
    }
  }

  function openEditor(path: string, isNew: boolean) {
    editorFilePath = path;
    editorIsNewFile = isNew;
    editorOpen = true;
  }

  function handleTriggerUpload() {
    if (fileInputRef) {
      fileInputRef.click();
    }
  }

  async function handleFileSelected(e: Event) {
    const target = e.target as HTMLInputElement;
    const fileList = target.files;
    if (!fileList || fileList.length === 0) return;

    const file = fileList[0];
    isUploading = true;
    error = null;

    try {
      await fileApi.upload(containerId, currentPath, file);
      showFeedback('success', `Uploaded "${file.name}" successfully!`);
      await loadDirectory(currentPath);
    } catch (err: unknown) {
      if (err instanceof Error) {
        error = `Upload failed: ${err.message}`;
      } else {
        error = 'Failed to upload file to container.';
      }
    } finally {
      isUploading = false;
      target.value = '';
    }
  }

  function handleDownloadFile(file: ContainerFileInfo) {
    const url = fileApi.getDownloadUrl(containerId, file.path);
    const a = document.createElement('a');
    a.href = url;
    a.download = file.name;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
  }

  async function confirmDelete() {
    if (!deleteTarget) return;

    isDeleting = true;
    try {
      await fileApi.delete(containerId, deleteTarget.path);
      showFeedback('success', `Deleted "${deleteTarget.name}" successfully.`);
      deleteTarget = null;
      await loadDirectory(currentPath);
    } catch (err: unknown) {
      if (err instanceof Error) {
        error = `Delete failed: ${err.message}`;
      } else {
        error = 'Failed to delete file from container.';
      }
      deleteTarget = null;
    } finally {
      isDeleting = false;
    }
  }

  function formatFileSize(bytes: number): string {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`;
  }

  function formatDate(dateStr: string): string {
    if (!dateStr || dateStr.startsWith('0001-01-01')) return '-';
    try {
      const d = new Date(dateStr);
      return d.toLocaleString(undefined, {
        month: 'short',
        day: 'numeric',
        year: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
      });
    } catch {
      return dateStr;
    }
  }

  function getFileIcon(file: ContainerFileInfo) {
    if (file.is_dir) return Folder;
    const ext = file.name.split('.').pop()?.toLowerCase();
    const codeExts = ['json', 'js', 'ts', 'html', 'css', 'yaml', 'yml', 'xml', 'py', 'go', 'sh', 'c', 'cpp', 'rs', 'php', 'rb', 'sql', 'conf', 'ini', 'env'];
    const textExts = ['log', 'txt', 'md', 'rst', 'csv'];

    if (ext && codeExts.includes(ext)) return FileCode;
    if (ext && textExts.includes(ext)) return FileText;
    return File;
  }
</script>

<div class="space-y-4">
  <!-- Hidden File Input for Uploads -->
  <input
    type="file"
    bind:this={fileInputRef}
    onchange={handleFileSelected}
    class="hidden"
  />

  <!-- Top Container Status Warning if Stopped -->
  {#if !isRunning}
    <div class="p-3 rounded-xl bg-amber-500/10 border border-amber-500/20 flex items-center gap-3 text-xs text-amber-300">
      <AlertTriangle class="w-4 h-4 text-amber-400 shrink-0" />
      <span>
        Container is currently <strong>stopped</strong>. Directory listing and reading files work via container archive, but live deletion requires a running container.
      </span>
    </div>
  {/if}

  <!-- Feedback Banner -->
  {#if feedback}
    <div
      class="p-3 rounded-xl border flex items-center justify-between gap-3 text-xs animate-in slide-in-from-top-2 duration-150 {feedback.type === 'success' ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-300' : 'bg-rose-500/10 border-rose-500/30 text-rose-300'}"
    >
      <div class="flex items-center gap-2">
        {#if feedback.type === 'success'}
          <CircleCheck class="w-4 h-4 text-emerald-400 shrink-0" />
        {:else}
          <CircleAlert class="w-4 h-4 text-rose-400 shrink-0" />
        {/if}
        <span>{feedback.text}</span>
      </div>
    </div>
  {/if}

  <!-- Error Banner -->
  {#if error}
    <div class="p-4 rounded-xl bg-rose-500/10 border border-rose-500/30 flex items-center justify-between text-xs text-rose-300">
      <div class="flex items-center gap-2">
        <CircleAlert class="w-4 h-4 text-rose-400 shrink-0" />
        <span>{error}</span>
      </div>
      <button
        type="button"
        onclick={() => loadDirectory(currentPath)}
        class="px-3 py-1 rounded-lg bg-rose-500/20 hover:bg-rose-500/30 text-rose-300 text-[11px] transition"
      >
        Retry
      </button>
    </div>
  {/if}

  <!-- Navigation Toolbar & Breadcrumbs Bar -->
  <div class="p-4 rounded-2xl bg-[#12131c] border border-white/[0.08] space-y-3">
    <!-- Row 1: Breadcrumbs Trail & Quick Actions -->
    <div class="flex flex-wrap items-center justify-between gap-3">
      <!-- Breadcrumb navigation items -->
      <nav class="flex items-center gap-1.5 flex-wrap text-xs font-mono" aria-label="Breadcrumb">
        <button
          type="button"
          onclick={() => loadDirectory('/')}
          class="inline-flex items-center gap-1 px-2.5 py-1 rounded-lg text-slate-300 hover:text-cyan-400 hover:bg-white/5 transition {currentPath === '/' ? 'text-cyan-300 bg-white/[0.04] font-bold' : ''}"
          title="Root directory"
        >
          <Home class="w-3.5 h-3.5 text-cyan-400" />
          <span>/</span>
        </button>

        {#each breadcrumbSegments as segment, idx}
          <ChevronRight class="w-3.5 h-3.5 text-slate-600 shrink-0" />
          {@const isLast = idx === breadcrumbSegments.length - 1}
          <button
            type="button"
            onclick={() => loadDirectory(segment.path)}
            class="px-2 py-1 rounded-lg transition {isLast ? 'text-cyan-300 bg-white/[0.06] font-bold' : 'text-slate-300 hover:text-cyan-400 hover:bg-white/5'}"
          >
            {segment.name}
          </button>
        {/each}
      </nav>

      <!-- Action Buttons -->
      <div class="flex items-center gap-2">
        <!-- New File Button -->
        <button
          type="button"
          onclick={() => openEditor(`${currentPath.endsWith('/') ? currentPath : currentPath + '/'}newfile.txt`, true)}
          class="px-3 py-1.5 rounded-xl bg-violet-600/20 hover:bg-violet-600/30 text-violet-300 border border-violet-500/30 text-xs font-semibold transition inline-flex items-center gap-1.5"
          title="Create a new file in current directory"
        >
          <Plus class="w-3.5 h-3.5" />
          <span>New File</span>
        </button>

        <!-- Upload File Button -->
        <button
          type="button"
          onclick={handleTriggerUpload}
          disabled={isUploading}
          class="px-3 py-1.5 rounded-xl bg-cyan-600/20 hover:bg-cyan-600/30 text-cyan-300 border border-cyan-500/30 text-xs font-semibold transition inline-flex items-center gap-1.5 disabled:opacity-50"
          title="Upload a file from your computer"
        >
          {#if isUploading}
            <LoaderCircle class="w-3.5 h-3.5 animate-spin" />
            <span>Uploading...</span>
          {:else}
            <Upload class="w-3.5 h-3.5" />
            <span>Upload File</span>
          {/if}
        </button>

        <!-- Refresh Button -->
        <button
          type="button"
          onclick={() => loadDirectory(currentPath)}
          disabled={isLoading}
          class="p-2 rounded-xl bg-[#090a10] hover:bg-white/10 text-slate-300 hover:text-white border border-white/[0.08] text-xs transition disabled:opacity-50"
          title="Reload directory"
        >
          <RefreshCw class="w-4 h-4 {isLoading ? 'animate-spin text-cyan-400' : ''}" />
        </button>
      </div>
    </div>

    <!-- Row 2: Manual Path Input & Filter Search -->
    <div class="flex flex-col sm:flex-row items-center gap-3 pt-2 border-t border-white/[0.06]">
      <!-- Quick Directory Navigation Form -->
      <form onsubmit={handlePathSubmit} class="flex items-center gap-2 w-full sm:flex-1">
        <div class="relative flex-1">
          <input
            type="text"
            bind:value={pathInput}
            placeholder="/etc/nginx"
            class="w-full bg-[#090a10] border border-white/[0.08] focus:border-cyan-500/50 rounded-xl px-3 py-1.5 text-xs font-mono text-slate-200 placeholder-slate-500 outline-none transition"
          />
        </div>
        <button
          type="submit"
          class="px-3 py-1.5 rounded-xl bg-white/[0.04] hover:bg-white/[0.08] text-slate-200 text-xs font-mono transition border border-white/[0.06]"
        >
          Go
        </button>
      </form>

      <!-- Filter Query Input -->
      <div class="relative w-full sm:w-64">
        <Search class="w-3.5 h-3.5 text-slate-500 absolute left-3 top-1/2 -translate-y-1/2" />
        <input
          type="text"
          bind:value={searchQuery}
          placeholder="Filter files..."
          class="w-full bg-[#090a10] border border-white/[0.08] focus:border-cyan-500/50 rounded-xl pl-8 pr-3 py-1.5 text-xs font-mono text-slate-200 placeholder-slate-500 outline-none transition"
        />
      </div>
    </div>
  </div>

  <!-- Files Table Container -->
  <div class="rounded-2xl bg-[#12131c] border border-white/[0.08] overflow-hidden shadow-2xl">
    <div class="overflow-x-auto">
      <table class="w-full text-left text-xs font-mono">
        <thead class="bg-[#090a10] border-b border-white/[0.08] text-slate-400 uppercase text-[10px] tracking-wider select-none">
          <tr>
            <th class="py-3 px-4">Name</th>
            <th class="py-3 px-4 w-28">Size</th>
            <th class="py-3 px-4 w-32 hidden md:table-cell">Permissions</th>
            <th class="py-3 px-4 w-44 hidden sm:table-cell">Modified</th>
            <th class="py-3 px-4 text-right w-44">Actions</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-white/[0.04]">
          <!-- Up-Level Directory Row -->
          {#if currentPath !== '/'}
            <tr
              onclick={handleNavigateUp}
              class="hover:bg-white/[0.02] cursor-pointer text-slate-400 hover:text-white transition group"
            >
              <td colspan="5" class="py-2.5 px-4 flex items-center gap-2.5">
                <FolderUp class="w-4 h-4 text-cyan-400 group-hover:scale-110 transition-transform" />
                <span class="font-bold text-slate-300">.. (Parent Directory)</span>
              </td>
            </tr>
          {/if}

          <!-- File / Folder Rows -->
          {#if isLoading && files.length === 0}
            <tr>
              <td colspan="5" class="py-12 text-center text-slate-400">
                <LoaderCircle class="w-6 h-6 animate-spin text-cyan-400 mx-auto mb-2" />
                <span>Reading directory contents...</span>
              </td>
            </tr>
          {:else if filteredFiles.length === 0}
            <tr>
              <td colspan="5" class="py-12 text-center text-slate-500">
                {#if searchQuery.trim()}
                  <p>No files matching "{searchQuery}"</p>
                {:else}
                  <FolderOpen class="w-8 h-8 text-slate-600 mx-auto mb-2" />
                  <p class="text-slate-400 font-semibold">Directory is empty</p>
                  <p class="text-[11px] text-slate-500 mt-1">Upload a file or create a new file above.</p>
                {/if}
              </td>
            </tr>
          {:else}
            {#each filteredFiles as file}
              {@const IconComponent = getFileIcon(file)}
              <tr
                class="hover:bg-white/[0.02] transition group"
              >
                <!-- Name Column -->
                <td class="py-2.5 px-4">
                  <div class="flex items-center gap-2.5">
                    <button
                      type="button"
                      onclick={() => handleFileClick(file)}
                      class="flex items-center gap-2.5 text-left focus:outline-none"
                    >
                      <IconComponent
                        class="w-4 h-4 shrink-0 transition-transform group-hover:scale-105 {file.is_dir ? 'text-amber-400 fill-amber-400/20' : 'text-cyan-400'}"
                      />
                      <span class="truncate max-w-[280px] sm:max-w-md {file.is_dir ? 'text-white font-semibold hover:text-cyan-300' : 'text-slate-200 hover:text-white'}">
                        {file.name}
                      </span>
                    </button>

                    {#if file.link_target}
                      <span class="text-[10px] text-violet-400 truncate max-w-[150px]">
                        &rarr; {file.link_target}
                      </span>
                    {/if}
                  </div>
                </td>

                <!-- Size Column -->
                <td class="py-2.5 px-4 text-slate-400 whitespace-nowrap">
                  {#if file.is_dir}
                    <span class="text-slate-600">-</span>
                  {:else}
                    <span>{formatFileSize(file.size)}</span>
                  {/if}
                </td>

                <!-- Permissions / Mode Column -->
                <td class="py-2.5 px-4 hidden md:table-cell whitespace-nowrap">
                  <span class="text-[11px] px-2 py-0.5 rounded bg-white/[0.03] border border-white/[0.04] text-slate-400">
                    {file.mode || '-'}
                  </span>
                </td>

                <!-- Modified Column -->
                <td class="py-2.5 px-4 hidden sm:table-cell text-slate-400 whitespace-nowrap text-[11px]">
                  {formatDate(file.mod_time)}
                </td>

                <!-- Actions Column -->
                <td class="py-2.5 px-4 text-right whitespace-nowrap">
                  <div class="flex items-center justify-end gap-1.5">
                    {#if file.is_dir}
                      <button
                        type="button"
                        onclick={() => loadDirectory(file.path)}
                        class="p-1.5 rounded-lg bg-white/[0.04] hover:bg-white/[0.08] text-slate-300 hover:text-cyan-300 transition"
                        title="Open directory"
                      >
                        <ChevronRight class="w-3.5 h-3.5" />
                      </button>
                    {:else}
                      <!-- View / Edit Button -->
                      <button
                        type="button"
                        onclick={() => openEditor(file.path, false)}
                        class="p-1.5 rounded-lg bg-white/[0.04] hover:bg-violet-600/20 text-slate-300 hover:text-violet-300 transition"
                        title="View / Edit file"
                      >
                        <PenLine class="w-3.5 h-3.5" />
                      </button>

                      <!-- Download Button -->
                      <button
                        type="button"
                        onclick={() => handleDownloadFile(file)}
                        class="p-1.5 rounded-lg bg-white/[0.04] hover:bg-cyan-600/20 text-slate-300 hover:text-cyan-300 transition"
                        title="Download file"
                      >
                        <Download class="w-3.5 h-3.5" />
                      </button>
                    {/if}

                    <!-- Delete Button -->
                    <button
                      type="button"
                      onclick={() => deleteTarget = file}
                      class="p-1.5 rounded-lg bg-white/[0.04] hover:bg-rose-500/20 text-slate-400 hover:text-rose-400 transition"
                      title={file.is_dir ? 'Delete directory' : 'Delete file'}
                    >
                      <Trash class="w-3.5 h-3.5" />
                    </button>
                  </div>
                </td>
              </tr>
            {/each}
          {/if}
        </tbody>
      </table>
    </div>

    <!-- Table Footer / Summary -->
    <div class="px-4 py-2.5 bg-[#090a10] border-t border-white/[0.08] text-[11px] font-mono text-slate-500 flex items-center justify-between">
      <span>
        {dirCount} {dirCount === 1 ? 'folder' : 'folders'}, {fileCount} {fileCount === 1 ? 'file' : 'files'}
      </span>
      <span>{currentPath}</span>
    </div>
  </div>
</div>

<!-- File Editor Modal -->
{#if editorOpen}
  <FileEditorModal
    {containerId}
    filePath={editorFilePath}
    initialDirectory={currentPath}
    isNewFile={editorIsNewFile}
    onClose={() => editorOpen = false}
    onSaved={() => loadDirectory(currentPath)}
  />
{/if}

<!-- Delete Confirmation Modal -->
{#if deleteTarget}
  <div
    class="fixed inset-0 z-50 flex items-center justify-center p-4"
    role="dialog"
    aria-modal="true"
    aria-labelledby="delete-file-title"
    tabindex="-1"
  >
    <button
      type="button"
      class="fixed inset-0 bg-black/80 backdrop-blur-md cursor-default border-0"
      onclick={() => deleteTarget = null}
      aria-label="Close delete modal"
      tabindex="-1"
    ></button>

    <div class="relative w-full max-w-md rounded-2xl bg-[#12131c] border border-white/[0.12] p-6 space-y-4 shadow-[0_10px_50px_rgba(0,0,0,0.8)]">
      <div class="flex items-center gap-3">
        <div class="w-10 h-10 rounded-xl bg-rose-500/20 border border-rose-500/30 flex items-center justify-center text-rose-400">
          <Trash class="w-5 h-5" />
        </div>
        <div>
          <h3 id="delete-file-title" class="text-sm font-bold text-white">
            Delete {deleteTarget.is_dir ? 'Directory' : 'File'}?
          </h3>
          <p class="text-xs text-slate-400 truncate max-w-[280px]">
            {deleteTarget.path}
          </p>
        </div>
      </div>

      <p class="text-xs text-slate-300 leading-relaxed bg-[#0a0b10] p-3 rounded-xl border border-white/[0.06]">
        Are you sure you want to delete <code class="text-rose-400">{deleteTarget.path}</code>? This operation cannot be undone.
      </p>

      <div class="flex items-center justify-end gap-3 pt-2">
        <button
          type="button"
          onclick={() => deleteTarget = null}
          class="px-4 py-2 rounded-xl hover:bg-white/10 text-slate-300 text-xs font-semibold transition"
        >
          Cancel
        </button>
        <button
          type="button"
          onclick={confirmDelete}
          disabled={isDeleting}
          class="px-4 py-2 rounded-xl bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold shadow-[0_0_15px_rgba(244,63,94,0.4)] transition inline-flex items-center gap-2 disabled:opacity-50"
        >
          {#if isDeleting}
            <LoaderCircle class="w-3.5 h-3.5 animate-spin" />
            <span>Deleting...</span>
          {:else}
            <span>Confirm Delete</span>
          {/if}
        </button>
      </div>
    </div>
  </div>
{/if}
