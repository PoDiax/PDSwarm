<script lang="ts">
  import { Rocket, Box, Globe, ExternalLink } from '@lucide/svelte';
  import type { Template } from '../types';

  interface Props {
    template: Template;
    onDeploy?: (template: Template) => void;
  }

  let { template, onDeploy }: Props = $props();

  let imgFailed = $state(false);

  function getFallbackLetter(title: string, id: string): string {
    const text = title || id || '?';
    return text.charAt(0).toUpperCase();
  }

  function cleanPort(port: string): string {
    // Port might be "5432:5432", "80/tcp", "8080"
    if (port.includes(':')) {
      return port.split(':')[0];
    }
    return port.replace(/\/tcp|\/udp/i, '');
  }

  function handleDeployClick(e: MouseEvent) {
    e.stopPropagation();
    if (onDeploy) {
      onDeploy(template);
    }
  }
</script>

<div
  class="group relative flex flex-col justify-between rounded-2xl bg-[#12131c] border border-white/[0.08] hover:border-violet-500/50 hover:shadow-[0_0_25px_rgba(124,58,237,0.22)] transition-all duration-200 overflow-hidden p-5 min-w-0 w-full h-full"
>
  <!-- Card Header & Info -->
  <div class="space-y-3.5 min-w-0 w-full">
    <!-- Top Row: Logo & Category Badges -->
    <div class="flex items-start justify-between gap-3 min-w-0">
      <!-- App Logo with fallback -->
      <div class="relative shrink-0">
        {#if template.logo && !imgFailed}
          <div class="w-12 h-12 rounded-xl bg-[#181a26] border border-white/[0.06] p-2 flex items-center justify-center overflow-hidden shadow-inner group-hover:border-violet-500/30 transition-colors">
            <img
              src={template.logo}
              alt={template.title}
              class="w-full h-full object-contain"
              loading="lazy"
              onerror={() => { imgFailed = true; }}
            />
          </div>
        {:else}
          <div
            class="w-12 h-12 rounded-xl bg-gradient-to-br from-violet-600/30 via-indigo-600/20 to-purple-800/30 border border-violet-500/40 text-violet-300 font-mono font-bold text-lg flex items-center justify-center shadow-[0_0_15px_rgba(124,58,237,0.25)]"
          >
            {getFallbackLetter(template.title, template.id)}
          </div>
        {/if}
      </div>

      <!-- Categories & Source -->
      <div class="flex flex-wrap items-center justify-end gap-1.5 max-w-[65%] min-w-0">
        {#if template.source && template.source !== 'curated'}
          <span
            class="px-2 py-0.5 rounded-full text-[10px] font-mono font-semibold uppercase tracking-wider bg-cyan-500/10 text-cyan-400 border border-cyan-500/30 shrink-0 truncate max-w-[120px]"
            title={template.source}
          >
            {template.source}
          </span>
        {/if}

        {#if template.categories && template.categories.length > 0}
          {#each template.categories.slice(0, 2) as cat}
            <span
              class="px-2 py-0.5 rounded-full text-[10px] font-medium bg-white/[0.05] text-slate-300 border border-white/[0.08] truncate max-w-[100px]"
              title={cat}
            >
              {cat}
            </span>
          {/each}
          {#if template.categories.length > 2}
            <span
              class="px-1.5 py-0.5 rounded-full text-[9px] font-mono bg-white/[0.03] text-slate-500 border border-white/[0.05] shrink-0"
              title={template.categories.slice(2).join(', ')}
            >
              +{template.categories.length - 2}
            </span>
          {/if}
        {/if}
      </div>
    </div>

    <!-- Title and Clamped Description -->
    <div class="min-w-0">
      <h3 class="text-base font-bold text-white group-hover:text-violet-300 transition-colors flex items-center gap-2 min-w-0">
        <span class="truncate">{template.title}</span>
      </h3>
      <p class="text-xs text-slate-400 mt-1 line-clamp-2 leading-relaxed min-h-[2.5rem] break-words">
        {template.description || 'No description provided.'}
      </p>
    </div>
  </div>

  <!-- Card Footer: Ports & 1-Click Deploy -->
  <div class="pt-4 mt-4 border-t border-white/[0.06] flex items-center justify-between gap-3">
    <!-- Ports Badges -->
    <div class="flex items-center gap-1.5 overflow-hidden flex-1 min-w-0">
      {#if template.ports && template.ports.length > 0}
        <div class="flex items-center gap-1.5 flex-wrap">
          {#each template.ports.slice(0, 2) as port}
            <span
              class="px-2 py-0.5 rounded-md text-[11px] font-mono bg-[#181a26] text-slate-300 border border-white/[0.08]"
              title="Port {port}"
            >
              :{cleanPort(port)}
            </span>
          {/each}
          {#if template.ports.length > 2}
            <span
              class="text-[10px] font-mono text-slate-500"
              title={template.ports.slice(2).join(', ')}
            >
              +{template.ports.length - 2}
            </span>
          {/if}
        </div>
      {:else}
        <span class="text-[11px] font-mono text-slate-600 flex items-center gap-1">
          <Box class="w-3 h-3 text-slate-600" /> Internal
        </span>
      {/if}
    </div>

    <!-- 1-Click Deploy Button -->
    <button
      type="button"
      onclick={handleDeployClick}
      class="shrink-0 flex items-center gap-1.5 px-3 py-1.5 rounded-xl bg-violet-600/90 hover:bg-violet-500 text-white text-xs font-semibold shadow-[0_0_15px_rgba(124,58,237,0.35)] hover:shadow-[0_0_20px_rgba(124,58,237,0.55)] transition-all cursor-pointer active:scale-95"
    >
      <Rocket class="w-3.5 h-3.5" />
      <span>Deploy</span>
    </button>
  </div>
</div>
