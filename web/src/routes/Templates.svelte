<script lang="ts">
  import { onMount } from 'svelte';
  import {
    Boxes,
    Search,
    X,
    RefreshCw,
    Filter,
    Rocket,
    AlertCircle,
    CheckCircle2,
    Loader2,
    Layers,
    Sparkles,
    ExternalLink,
    ChevronLeft,
    ChevronRight,
    ChevronsLeft,
    ChevronsRight,
    LayoutGrid,
    Infinity as InfinityIcon,
  } from '@lucide/svelte';
  import TemplateCard from '../lib/components/TemplateCard.svelte';
  import TemplateDeployModal from '../lib/components/TemplateDeployModal.svelte';
  import { templateApi } from '../lib/api';
  import type { Template } from '../lib/types';

  interface Props {
    onNavigate?: (tab: string) => void;
    onDeploySuccess?: (stackName: string) => void;
  }

  let { onNavigate, onDeploySuccess }: Props = $props();

  // State
  let templates = $state<Template[]>([]);
  let categories = $state<string[]>([]);
  let categoryCounts = $state<Record<string, number>>({});
  let sources = $state<string[]>([]);
  let isLoading = $state(true);
  let isRefreshing = $state(false);
  let error = $state<string | null>(null);

  // Filters
  let searchQuery = $state('');
  let selectedCategory = $state('All');
  let selectedSource = $state('All Sources');

  // Pagination & Display Mode
  let currentPage = $state(1);
  let pageSize = $state(24);
  const pageSizeOptions = [12, 24, 48, 96];
  let viewMode = $state<'paged' | 'infinite'>('paged');
  let infiniteLimit = $state(24);
  let isLoadingMore = $state(false);
  let appsBoxElement = $state<HTMLElement | null>(null);
  let sentinelElement = $state<HTMLElement | null>(null);

  // Modals & Notifications
  let deployingTemplate = $state<Template | null>(null);
  let notification = $state<{ type: 'success' | 'error'; message: string; stackName?: string } | null>(
    null
  );

  async function loadTemplates(isRefresh = false) {
    if (isRefresh) {
      isRefreshing = true;
    } else {
      isLoading = true;
    }
    error = null;

    try {
      const res = await templateApi.list();
      templates = res.templates || [];
      categories = res.categories || [];
      categoryCounts = res.category_counts || {};
      sources = res.sources || [];
    } catch (err: unknown) {
      const e = err as { message?: string };
      error = e?.message || 'Failed to load application templates.';
    } finally {
      isLoading = false;
      isRefreshing = false;
    }
  }

  onMount(() => {
    loadTemplates();
  });

  // Derived filtered templates
  const filteredTemplates = $derived(
    templates.filter((tmpl) => {
      // Source match
      if (selectedSource !== 'All Sources') {
        const tmplSrc = tmpl.source || 'curated';
        if (tmplSrc.toLowerCase() !== selectedSource.toLowerCase()) return false;
      }

      // Category match
      if (selectedCategory !== 'All') {
        const matchesCategory = tmpl.categories?.some(
          (c) => c.toLowerCase() === selectedCategory.toLowerCase()
        );
        if (!matchesCategory) return false;
      }

      // Search match
      if (searchQuery.trim()) {
        const q = searchQuery.toLowerCase().trim();
        const inTitle = tmpl.title?.toLowerCase().includes(q);
        const inDesc = tmpl.description?.toLowerCase().includes(q);
        const inId = tmpl.id?.toLowerCase().includes(q);
        const inCat = tmpl.categories?.some((c) => c.toLowerCase().includes(q));
        const inSrc = tmpl.source?.toLowerCase().includes(q);
        if (!inTitle && !inDesc && !inId && !inCat && !inSrc) return false;
      }

      return true;
    })
  );

  // Derived pagination & display metrics
  const totalFiltered = $derived(filteredTemplates.length);
  const totalPages = $derived(Math.max(1, Math.ceil(totalFiltered / pageSize)));

  // Reset pagination when filters change
  $effect(() => {
    // Read dependencies to trigger on changes
    const _q = searchQuery;
    const _c = selectedCategory;
    const _s = selectedSource;
    const _ps = pageSize;
    currentPage = 1;
    infiniteLimit = pageSize;
    if (appsBoxElement) {
      appsBoxElement.scrollTo({ top: 0, behavior: 'instant' });
    }
  });

  const startIndex = $derived((currentPage - 1) * pageSize);
  const endIndex = $derived(Math.min(startIndex + pageSize, totalFiltered));

  const displayedTemplates = $derived(
    viewMode === 'paged'
      ? filteredTemplates.slice(startIndex, endIndex)
      : filteredTemplates.slice(0, infiniteLimit)
  );

  const hasMoreInfinite = $derived(infiniteLimit < totalFiltered);

  // Smart page numbers array e.g. [1, '...', 4, 5, 6, '...', 12]
  const pageNumbers = $derived.by(() => {
    if (totalPages <= 7) {
      return Array.from({ length: totalPages }, (_, i) => i + 1);
    }
    const pages: (number | string)[] = [];
    pages.push(1);

    const start = Math.max(2, currentPage - 1);
    const end = Math.min(totalPages - 1, currentPage + 1);

    if (start > 2) {
      pages.push('...');
    }

    for (let i = start; i <= end; i++) {
      pages.push(i);
    }

    if (end < totalPages - 1) {
      pages.push('...');
    }

    pages.push(totalPages);
    return pages;
  });

  function goToPage(page: number) {
    if (page < 1 || page > totalPages) return;
    currentPage = page;
    if (appsBoxElement) {
      appsBoxElement.scrollTo({ top: 0, behavior: 'smooth' });
    }
  }

  function loadMoreInfinite() {
    if (isLoadingMore || !hasMoreInfinite) return;
    isLoadingMore = true;
    setTimeout(() => {
      infiniteLimit = Math.min(infiniteLimit + pageSize, totalFiltered);
      isLoadingMore = false;
    }, 150);
  }

  function handleBoxScroll(e: Event) {
    if (viewMode !== 'infinite' || isLoadingMore || !hasMoreInfinite) return;
    const target = e.currentTarget as HTMLElement;
    if (!target) return;
    if (target.scrollTop + target.clientHeight >= target.scrollHeight - 350) {
      loadMoreInfinite();
    }
  }

  $effect(() => {
    if (viewMode !== 'infinite' || !sentinelElement || !appsBoxElement) return;

    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0]?.isIntersecting && hasMoreInfinite && !isLoadingMore) {
          loadMoreInfinite();
        }
      },
      { root: appsBoxElement, rootMargin: '250px' }
    );

    observer.observe(sentinelElement);
    return () => observer.disconnect();
  });

  async function handleOpenDeploy(tmpl: Template) {
    deployingTemplate = tmpl;
    try {
      const full = await templateApi.get(tmpl.id);
      if (full && deployingTemplate && deployingTemplate.id === tmpl.id) {
        deployingTemplate = full;
      }
    } catch {
      // Continue with provided template
    }
  }

  function handleCloseDeploy() {
    deployingTemplate = null;
  }

  function handleDeploySuccess(stackName: string) {
    deployingTemplate = null;
    notification = {
      type: 'success',
      message: `Deployment started for "${stackName}". Track live progress on Dashboard!`,
      stackName,
    };

    setTimeout(() => {
      if (onDeploySuccess) {
        onDeploySuccess(stackName);
      } else if (onNavigate) {
        onNavigate('dashboard');
      }
    }, 1000);
  }

  function clearFilters() {
    searchQuery = '';
    selectedCategory = 'All';
    selectedSource = 'All Sources';
    currentPage = 1;
    infiniteLimit = pageSize;
  }
</script>

<div class="flex-1 min-h-0 flex flex-col space-y-3 overflow-hidden animate-in fade-in duration-200 h-full">
  <!-- Top Banner / Header (Compact & Static) -->
  <div
    class="relative rounded-2xl bg-[#12131c] border border-white/[0.08] px-5 py-3.5 overflow-hidden shadow-xl shrink-0"
  >
    <!-- Background glow & grid accents -->
    <div
      class="absolute -right-16 -top-16 w-64 h-64 bg-violet-600/10 rounded-full blur-3xl pointer-events-none"
    ></div>
    <div
      class="absolute -left-16 -bottom-16 w-64 h-64 bg-cyan-600/10 rounded-full blur-3xl pointer-events-none"
    ></div>

    <div class="relative z-10 flex flex-col md:flex-row md:items-center justify-between gap-3">
      <div class="space-y-0.5">
        <div class="flex items-center gap-2.5">
          <div
            class="w-7 h-7 rounded-lg bg-violet-600/20 border border-violet-500/30 flex items-center justify-center text-violet-400 shadow-[0_0_12px_rgba(124,58,237,0.3)]"
          >
            <Boxes class="w-4 h-4" />
          </div>
          <h1 class="text-lg font-bold tracking-tight text-white flex items-center gap-2">
            <span>App Store & Templates</span>
            <span
              class="px-2 py-0.5 rounded-full text-[10px] font-mono font-medium bg-emerald-500/10 text-emerald-400 border border-emerald-500/30"
            >
              1-Click Deploy
            </span>
          </h1>
        </div>
        <p class="text-[11px] text-slate-400 max-w-xl">
          Curated self-hosted software ready to launch with single-click Docker Compose configurations.
        </p>
      </div>

      <div class="flex items-center gap-2.5">
        <!-- Count Badge -->
        <div
          class="flex items-center gap-1.5 px-2.5 py-1 rounded-lg bg-[#0a0b10] border border-white/[0.08] text-xs font-mono text-slate-300"
        >
          <Sparkles class="w-3.5 h-3.5 text-violet-400" />
          <span>{templates.length} templates</span>
        </div>

        <!-- Manage Catalogs Button -->
        {#if onNavigate}
          <button
            type="button"
            onclick={() => onNavigate('settings')}
            class="flex items-center gap-1.5 px-2.5 py-1 rounded-lg bg-[#181a26] border border-white/[0.08] hover:border-violet-500/40 text-slate-300 hover:text-white transition text-xs font-medium cursor-pointer"
            title="Configure custom template repositories in Settings"
          >
            <Layers class="w-3.5 h-3.5 text-violet-400" />
            <span>Manage Catalogs</span>
          </button>
        {/if}

        <!-- Refresh Button -->
        <button
          type="button"
          onclick={() => loadTemplates(true)}
          disabled={isLoading || isRefreshing}
          class="p-1.5 rounded-lg bg-[#181a26] border border-white/[0.08] hover:border-violet-500/40 text-slate-300 hover:text-white transition disabled:opacity-50 cursor-pointer"
          title="Refresh catalog"
        >
          <RefreshCw class="w-3.5 h-3.5 {isRefreshing ? 'animate-spin text-violet-400' : ''}" />
        </button>
      </div>
    </div>
  </div>

  <!-- Notification Banner -->
  {#if notification}
    <div
      class="shrink-0 flex items-center justify-between gap-3 p-3.5 rounded-xl border animate-in slide-in-from-top-2 duration-150 {notification.type === 'success'
        ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-300'
        : 'bg-red-500/10 border-red-500/30 text-red-300'}"
    >
      <div class="flex items-center gap-2.5 text-xs font-medium">
        {#if notification.type === 'success'}
          <CheckCircle2 class="w-4 h-4 text-emerald-400 shrink-0" />
        {:else}
          <AlertCircle class="w-4 h-4 text-red-400 shrink-0" />
        {/if}
        <span>{notification.message}</span>
      </div>

      <div class="flex items-center gap-2">
        {#if notification.stackName && onNavigate}
          <button
            type="button"
            onclick={() => onNavigate('stacks')}
            class="px-2.5 py-1 rounded-lg text-xs font-semibold bg-emerald-500/20 hover:bg-emerald-500/30 text-emerald-300 transition cursor-pointer"
          >
            View in Stacks &rarr;
          </button>
        {/if}
        <button
          type="button"
          onclick={() => (notification = null)}
          class="p-1 rounded text-slate-400 hover:text-white transition cursor-pointer"
        >
          <X class="w-4 h-4" />
        </button>
      </div>
    </div>
  {/if}

  <!-- Error Banner -->
  {#if error}
    <div
      class="shrink-0 flex items-center justify-between p-3.5 rounded-xl bg-red-500/10 border border-red-500/30 text-red-400 text-xs"
    >
      <div class="flex items-center gap-2">
        <AlertCircle class="w-4 h-4 shrink-0" />
        <span>{error}</span>
      </div>
      <button
        type="button"
        onclick={() => loadTemplates()}
        class="underline hover:text-white cursor-pointer"
      >
        Retry
      </button>
    </div>
  {/if}

  <!-- Search & Category Filters (Static Controls) -->
  <div class="space-y-2.5 shrink-0">
    <!-- Search Bar & Source Selector Row -->
    <div class="flex flex-col sm:flex-row gap-2.5">
      <!-- Search Input -->
      <div class="relative flex-1">
        <Search
          class="w-4 h-4 absolute left-3.5 top-1/2 -translate-y-1/2 text-slate-500 pointer-events-none"
        />
        <input
          type="text"
          bind:value={searchQuery}
          placeholder="Search templates by name, category, or description (e.g. postgres, jellyfin, n8n)..."
          class="w-full pl-10 pr-10 py-2.5 rounded-xl bg-[#12131c] border border-white/[0.08] focus:border-violet-500 focus:outline-none focus:ring-1 focus:ring-violet-500 text-sm text-white placeholder-slate-500 transition shadow-inner"
        />
        {#if searchQuery}
          <button
            type="button"
            onclick={() => (searchQuery = '')}
            class="absolute right-3.5 top-1/2 -translate-y-1/2 p-1 text-slate-500 hover:text-slate-200 transition cursor-pointer"
            title="Clear search"
          >
            <X class="w-3.5 h-3.5" />
          </button>
        {/if}
      </div>

      <!-- Source / Catalog Filter Dropdown -->
      {#if sources.length > 0}
        <div class="relative sm:w-56 shrink-0">
          <select
            bind:value={selectedSource}
            class="w-full pl-3 pr-8 py-2.5 rounded-xl bg-[#12131c] border border-white/[0.08] focus:border-violet-500 focus:outline-none focus:ring-1 focus:ring-violet-500 text-xs text-white transition appearance-none cursor-pointer"
          >
            <option value="All Sources">All Catalogs ({templates.length})</option>
            {#each sources as src}
              <option value={src}>
                {src === 'curated' ? 'Curated (Built-in)' : src}
              </option>
            {/each}
          </select>
          <div class="absolute right-3 top-1/2 -translate-y-1/2 pointer-events-none text-slate-400">
            <Filter class="w-3.5 h-3.5" />
          </div>
        </div>
      {/if}
    </div>

    <!-- Category Filter Pills -->
    <div class="flex items-center gap-1.5 overflow-x-auto pb-1 scrollbar-none">
      <!-- 'All' chip -->
      <button
        type="button"
        onclick={() => (selectedCategory = 'All')}
        class="shrink-0 flex items-center gap-2 px-3 py-1.5 rounded-xl text-xs font-medium transition cursor-pointer {selectedCategory ===
        'All'
          ? 'bg-violet-600 text-white shadow-[0_0_15px_rgba(124,58,237,0.4)] border border-violet-400/30'
          : 'bg-[#12131c] text-slate-400 hover:text-slate-200 hover:bg-[#181a26] border border-white/[0.08]'}"
      >
        <span>All</span>
        <span
          class="px-1.5 py-0.2 rounded-full text-[10px] font-mono {selectedCategory === 'All'
            ? 'bg-white/20 text-white'
            : 'bg-white/[0.05] text-slate-500'}"
        >
          {templates.length}
        </span>
      </button>

      <!-- Category Chips -->
      {#each categories as cat}
        {@const count = categoryCounts[cat] ?? 0}
        {@const isSelected = selectedCategory === cat}
        <button
          type="button"
          onclick={() => (selectedCategory = cat)}
          class="shrink-0 flex items-center gap-2 px-3 py-1.5 rounded-xl text-xs font-medium transition cursor-pointer {isSelected
            ? 'bg-violet-600 text-white shadow-[0_0_15px_rgba(124,58,237,0.4)] border border-violet-400/30'
            : 'bg-[#12131c] text-slate-400 hover:text-slate-200 hover:bg-[#181a26] border border-white/[0.08]'}"
        >
          <span>{cat}</span>
          <span
            class="px-1.5 py-0.2 rounded-full text-[10px] font-mono {isSelected
              ? 'bg-white/20 text-white'
              : 'bg-white/[0.05] text-slate-500'}"
          >
            {count}
          </span>
        </button>
      {/each}
    </div>
  </div>

  <!-- Loading State -->
  {#if isLoading}
    <div class="flex-1 flex flex-col items-center justify-center gap-3 py-16">
      <Loader2 class="w-8 h-8 text-violet-400 animate-spin" />
      <span class="text-xs font-mono text-slate-400 animate-pulse">Loading application templates...</span>
    </div>
  {:else if filteredTemplates.length === 0}
    <!-- Empty State -->
    <div
      class="flex-1 flex flex-col items-center justify-center py-16 px-6 rounded-2xl bg-[#12131c] border border-white/[0.08] text-center space-y-4 shadow-inner"
    >
      <div
        class="w-14 h-14 mx-auto rounded-2xl bg-violet-600/10 border border-violet-500/20 flex items-center justify-center text-violet-400 shadow-[0_0_20px_rgba(124,58,237,0.2)]"
      >
        <Boxes class="w-7 h-7" />
      </div>

      <div class="space-y-1">
        <h3 class="text-base font-bold text-white">No templates found</h3>
        <p class="text-xs text-slate-400 max-w-sm mx-auto">
          No application templates match your current filters. Try refining your search query or selecting a different category.
        </p>
      </div>

      <div class="pt-2">
        <button
          type="button"
          onclick={clearFilters}
          class="px-4 py-2 rounded-xl bg-[#181a26] hover:bg-white/[0.08] border border-white/[0.08] text-xs font-medium text-slate-300 hover:text-white transition cursor-pointer"
        >
          Clear Filters
        </button>
      </div>
    </div>
  {:else}
    <!-- Pagination & View Controls Toolbar (Static) -->
    <div
      class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pt-0.5 pb-0.5 shrink-0"
    >
      <!-- Results summary counter -->
      <div class="text-xs font-mono text-slate-400">
        {#if viewMode === 'paged'}
          Showing <span class="font-semibold text-white">{totalFiltered > 0 ? startIndex + 1 : 0}–{endIndex}</span> of <span class="font-semibold text-white">{totalFiltered}</span> templates
        {:else}
          Showing <span class="font-semibold text-white">{Math.min(infiniteLimit, totalFiltered)}</span> of <span class="font-semibold text-white">{totalFiltered}</span> templates
        {/if}
      </div>

      <!-- Controls: Page Size & View Mode Toggle -->
      <div class="flex items-center gap-3">
        <!-- Page size selector -->
        <div class="flex items-center gap-2 text-xs font-mono text-slate-400">
          <span>Per page:</span>
          <div class="flex items-center rounded-xl bg-[#12131c] border border-white/[0.08] p-0.5">
            {#each pageSizeOptions as size}
              <button
                type="button"
                onclick={() => (pageSize = size)}
                class="px-2 py-1 rounded-lg text-xs font-mono transition cursor-pointer {pageSize === size
                  ? 'bg-violet-600 text-white font-bold shadow-[0_0_10px_rgba(124,58,237,0.4)]'
                  : 'text-slate-400 hover:text-white'}"
              >
                {size}
              </button>
            {/each}
          </div>
        </div>

        <!-- Mode Toggle: Paged vs Infinite Scroll -->
        <div class="flex items-center rounded-xl bg-[#12131c] border border-white/[0.08] p-0.5">
          <button
            type="button"
            onclick={() => (viewMode = 'paged')}
            class="flex items-center gap-1.5 px-2.5 py-1 rounded-lg text-xs font-medium transition cursor-pointer {viewMode === 'paged'
              ? 'bg-violet-600 text-white shadow-[0_0_10px_rgba(124,58,237,0.4)]'
              : 'text-slate-400 hover:text-white'}"
            title="Standard pagination with pages"
          >
            <LayoutGrid class="w-3.5 h-3.5" />
            <span class="hidden md:inline">Pages</span>
          </button>
          <button
            type="button"
            onclick={() => (viewMode = 'infinite')}
            class="flex items-center gap-1.5 px-2.5 py-1 rounded-lg text-xs font-medium transition cursor-pointer {viewMode === 'infinite'
              ? 'bg-violet-600 text-white shadow-[0_0_10px_rgba(124,58,237,0.4)]'
              : 'text-slate-400 hover:text-white'}"
            title="Smooth infinite scroll"
          >
            <InfinityIcon class="w-3.5 h-3.5" />
            <span class="hidden md:inline">Infinite</span>
          </button>
        </div>
      </div>
    </div>

    <!-- Dedicated Scrollable Box of Apps -->
    <div
      bind:this={appsBoxElement}
      onscroll={handleBoxScroll}
      class="flex-1 min-h-0 overflow-y-auto rounded-2xl bg-[#0c0d14]/75 border border-white/[0.08] p-4 shadow-inner space-y-4 scrollbar-thin scrollbar-thumb-violet-500/20 hover:scrollbar-thumb-violet-500/40"
    >
      <!-- Template Grid -->
      <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
        {#each displayedTemplates as template, idx (template.id + '-' + (template.source || '') + '-' + idx)}
          <TemplateCard {template} onDeploy={handleOpenDeploy} />
        {/each}
      </div>

      <!-- Infinite Scroll Sentinel & Loader (Infinite Mode) inside the box -->
      {#if viewMode === 'infinite'}
        <div class="py-4">
          <!-- Sentinel element for IntersectionObserver -->
          <div bind:this={sentinelElement} class="h-6 w-full pointer-events-none"></div>

          {#if isLoadingMore}
            <div class="py-4 flex items-center justify-center gap-2.5 text-xs font-mono text-violet-400 animate-pulse">
              <Loader2 class="w-4 h-4 animate-spin text-violet-400" />
              <span>Loading more application templates...</span>
            </div>
          {:else if hasMoreInfinite}
            <div class="pt-2 flex justify-center">
              <button
                type="button"
                onclick={loadMoreInfinite}
                class="flex items-center gap-2 px-5 py-2 rounded-xl bg-[#181a26] hover:bg-violet-600/20 border border-white/[0.08] hover:border-violet-500/40 text-xs font-semibold text-slate-300 hover:text-white transition shadow-lg cursor-pointer active:scale-95"
              >
                <span>Load More Templates</span>
                <span class="px-2 py-0.5 rounded-full text-[10px] font-mono bg-violet-600/30 text-violet-300">
                  +{Math.min(pageSize, totalFiltered - infiniteLimit)}
                </span>
              </button>
            </div>
          {:else if totalFiltered > 0}
            <div class="pt-4 pb-2 text-center">
              <span class="text-xs font-mono text-slate-500">
                You've reached the end of the catalog ({totalFiltered} templates)
              </span>
            </div>
          {/if}
        </div>
      {/if}
    </div>

    <!-- Pinned Static Pagination Controls (Paged Mode) -->
    {#if viewMode === 'paged' && totalPages > 1}
      <div
        class="shrink-0 flex flex-col sm:flex-row items-center justify-between gap-3 pt-1 px-1"
      >
        <div class="text-xs font-mono text-slate-400">
          Page <span class="font-semibold text-white">{currentPage}</span> of <span class="font-semibold text-white">{totalPages}</span>
        </div>

        <div class="flex items-center gap-1.5 flex-wrap justify-center">
          <!-- Jump to First -->
          <button
            type="button"
            onclick={() => goToPage(1)}
            disabled={currentPage === 1}
            class="p-1.5 rounded-xl bg-[#12131c] border border-white/[0.08] hover:border-violet-500/40 text-slate-400 hover:text-white transition disabled:opacity-30 disabled:pointer-events-none cursor-pointer"
            title="First page"
          >
            <ChevronsLeft class="w-4 h-4" />
          </button>

          <!-- Previous Page -->
          <button
            type="button"
            onclick={() => goToPage(currentPage - 1)}
            disabled={currentPage === 1}
            class="flex items-center gap-1 px-2.5 py-1.5 rounded-xl bg-[#12131c] border border-white/[0.08] hover:border-violet-500/40 text-xs font-medium text-slate-400 hover:text-white transition disabled:opacity-30 disabled:pointer-events-none cursor-pointer"
            title="Previous page"
          >
            <ChevronLeft class="w-4 h-4" />
            <span class="hidden sm:inline">Prev</span>
          </button>

          <!-- Page Number Chips -->
          {#each pageNumbers as p}
            {#if p === '...'}
              <span class="px-2 py-1 text-xs font-mono text-slate-600 select-none">...</span>
            {:else}
              <button
                type="button"
                onclick={() => goToPage(Number(p))}
                class="min-w-[32px] h-8 px-2 rounded-xl text-xs font-mono font-medium transition cursor-pointer {currentPage === p
                  ? 'bg-violet-600 text-white font-bold shadow-[0_0_15px_rgba(124,58,237,0.4)] border border-violet-400/30'
                  : 'bg-[#12131c] text-slate-400 hover:text-white hover:bg-[#181a26] border border-white/[0.08]'}"
              >
                {p}
              </button>
            {/if}
          {/each}

          <!-- Next Page -->
          <button
            type="button"
            onclick={() => goToPage(currentPage + 1)}
            disabled={currentPage === totalPages}
            class="flex items-center gap-1 px-2.5 py-1.5 rounded-xl bg-[#12131c] border border-white/[0.08] hover:border-violet-500/40 text-xs font-medium text-slate-400 hover:text-white transition disabled:opacity-30 disabled:pointer-events-none cursor-pointer"
            title="Next page"
          >
            <span class="hidden sm:inline">Next</span>
            <ChevronRight class="w-4 h-4" />
          </button>

          <!-- Jump to Last -->
          <button
            type="button"
            onclick={() => goToPage(totalPages)}
            disabled={currentPage === totalPages}
            class="p-1.5 rounded-xl bg-[#12131c] border border-white/[0.08] hover:border-violet-500/40 text-slate-400 hover:text-white transition disabled:opacity-30 disabled:pointer-events-none cursor-pointer"
            title="Last page"
          >
            <ChevronsRight class="w-4 h-4" />
          </button>
        </div>
      </div>
    {/if}
  {/if}
</div>

<!-- Deploy Modal -->
{#if deployingTemplate}
  <TemplateDeployModal
    template={deployingTemplate}
    onClose={handleCloseDeploy}
    onSuccess={handleDeploySuccess}
  />
{/if}
