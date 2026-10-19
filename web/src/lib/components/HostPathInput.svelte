<script lang="ts">
  import {
    Folder,
    HardDrive,
    LoaderCircle,
    FolderOpen,
    CornerDownLeft,
  } from '@lucide/svelte';
  import { systemApi } from '../api';
  import type { HostPathSuggestion } from '../types';

  interface Props {
    value?: string;
    placeholder?: string;
    disabled?: boolean;
    class?: string;
    onchange?: (val: string) => void;
  }

  let {
    value = $bindable(''),
    placeholder = 'Host path (e.g. /data) or Volume...',
    disabled = false,
    class: className = '',
    onchange,
  }: Props = $props();

  let suggestions = $state<HostPathSuggestion[]>([]);
  let isOpen = $state(false);
  let isLoading = $state(false);
  let selectedIndex = $state(-1);
  let inputElement = $state<HTMLInputElement | null>(null);
  let containerElement = $state<HTMLDivElement | null>(null);
  let debounceTimer: ReturnType<typeof setTimeout> | null = null;

  async function fetchSuggestions(query: string) {
    isLoading = true;
    try {
      const res = await systemApi.getHostPaths(query);
      suggestions = res?.suggestions || [];
    } catch {
      suggestions = [];
    } finally {
      isLoading = false;
    }
  }

  function handleInput() {
    selectedIndex = -1;
    if (debounceTimer) {
      clearTimeout(debounceTimer);
    }
    debounceTimer = setTimeout(() => {
      fetchSuggestions(value);
      isOpen = true;
    }, 150);
    if (onchange) {
      onchange(value);
    }
  }

  function handleFocus() {
    if (!disabled) {
      fetchSuggestions(value);
      isOpen = true;
    }
  }

  function handleSelect(sug: HostPathSuggestion) {
    if (sug.type === 'dir') {
      const targetPath = sug.path.endsWith('/') ? sug.path : sug.path + '/';
      value = targetPath;
      // Immediately fetch subfolders so user can drill down
      fetchSuggestions(targetPath);
      isOpen = true;
      inputElement?.focus();
    } else {
      value = sug.path;
      isOpen = false;
    }
    if (onchange) {
      onchange(value);
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (!isOpen || suggestions.length === 0) {
      if (e.key === 'ArrowDown' || e.key === 'Enter') {
        fetchSuggestions(value);
        isOpen = true;
      }
      return;
    }

    if (e.key === 'ArrowDown') {
      e.preventDefault();
      selectedIndex = (selectedIndex + 1) % suggestions.length;
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      selectedIndex = (selectedIndex - 1 + suggestions.length) % suggestions.length;
    } else if (e.key === 'Enter') {
      if (selectedIndex >= 0 && selectedIndex < suggestions.length) {
        e.preventDefault();
        handleSelect(suggestions[selectedIndex]);
      }
    } else if (e.key === 'Escape') {
      isOpen = false;
    }
  }

  function handleWindowClick(e: MouseEvent) {
    if (containerElement && !containerElement.contains(e.target as Node)) {
      isOpen = false;
    }
  }
</script>

<svelte:window onclick={handleWindowClick} />

<div bind:this={containerElement} class="relative w-full">
  <div class="relative flex items-center">
    <input
      bind:this={inputElement}
      type="text"
      bind:value
      oninput={handleInput}
      onfocus={handleFocus}
      onkeydown={handleKeydown}
      {placeholder}
      {disabled}
      class="{className} pr-7"
      autocomplete="off"
      spellcheck="false"
    />

    <div class="absolute right-2 flex items-center pointer-events-none text-slate-500">
      {#if isLoading}
        <LoaderCircle class="w-3.5 h-3.5 animate-spin text-cyan-400" />
      {:else}
        <FolderOpen class="w-3.5 h-3.5 opacity-40" />
      {/if}
    </div>
  </div>

  <!-- Autocomplete Suggestions Dropdown -->
  {#if isOpen && (suggestions.length > 0 || isLoading)}
    <div
      class="absolute left-0 top-full mt-1.5 w-full min-w-[260px] max-w-lg z-50 max-h-60 overflow-y-auto rounded-xl bg-[#12131c]/95 border border-white/[0.12] shadow-[0_10px_35px_rgba(0,0,0,0.6)] backdrop-blur-xl divide-y divide-white/[0.04] animate-in fade-in zoom-in-95 duration-100"
    >
      {#if isLoading && suggestions.length === 0}
        <div class="p-3 text-center text-xs text-slate-400 flex items-center justify-center gap-2">
          <LoaderCircle class="w-3.5 h-3.5 animate-spin text-cyan-400" />
          <span>Scanning host folders...</span>
        </div>
      {:else}
        <div class="p-1 space-y-0.5">
          {#each suggestions as sug, idx (sug.path + idx)}
            <button
              type="button"
              onclick={() => handleSelect(sug)}
              class="w-full text-left px-2.5 py-1.5 rounded-lg flex items-center justify-between gap-2 text-xs font-mono transition cursor-pointer {selectedIndex === idx
                ? 'bg-violet-600/30 text-white border border-violet-500/40'
                : 'text-slate-300 hover:bg-white/[0.06] hover:text-white'}"
            >
              <div class="flex items-center gap-2 min-w-0">
                {#if sug.type === 'volume'}
                  <HardDrive class="w-3.5 h-3.5 text-cyan-400 shrink-0" />
                {:else}
                  <Folder class="w-3.5 h-3.5 text-amber-400 shrink-0" />
                {/if}
                <span class="truncate font-medium">{sug.path}</span>
              </div>

              <div class="flex items-center gap-1.5 shrink-0">
                <span
                  class="text-[9px] uppercase px-1.5 py-0.5 rounded font-mono {sug.type === 'volume'
                    ? 'bg-cyan-500/10 text-cyan-300 border border-cyan-500/20'
                    : 'bg-amber-500/10 text-amber-300 border border-amber-500/20'}"
                >
                  {sug.type === 'volume' ? 'Volume' : 'Dir'}
                </span>
                {#if sug.type === 'dir'}
                  <span class="text-[10px] text-slate-500">&rsaquo;</span>
                {/if}
              </div>
            </button>
          {/each}
        </div>

        <div class="px-2.5 py-1.5 bg-[#0a0b10]/60 flex items-center justify-between text-[10px] text-slate-500 font-mono">
          <span>Click a folder to navigate into it</span>
          <span class="flex items-center gap-1"><CornerDownLeft class="w-2.5 h-2.5" /> Select</span>
        </div>
      {/if}
    </div>
  {/if}
</div>
