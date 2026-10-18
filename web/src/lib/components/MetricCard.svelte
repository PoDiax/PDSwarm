<script lang="ts">
  import type { Component, Snippet } from 'svelte';

  interface Props {
    title: string;
    value: string | number;
    subtitle?: string;
    icon?: Component<any>;
    variant?: 'violet' | 'cyan' | 'emerald' | 'rose' | 'amber' | 'slate';
    badge?: string;
    badgeColor?: string;
    trend?: {
      label: string;
      positive?: boolean;
    };
    clickable?: boolean;
    class?: string;
    onclick?: () => void;
    children?: Snippet;
  }

  let {
    title,
    value,
    subtitle,
    icon,
    variant = 'violet',
    badge,
    badgeColor,
    trend,
    clickable = false,
    class: className = '',
    onclick,
    children,
  }: Props = $props();

  const Icon = $derived(icon);

  const variantStyles = {
    violet: {
      iconBg: 'bg-violet-500/10 border-violet-500/25 text-violet-400',
      borderHover: 'hover:border-violet-500/40',
      bottomLine: 'bg-gradient-to-r from-violet-600 via-indigo-500 to-cyan-500',
      glow: 'group-hover:shadow-[0_0_20px_rgba(124,58,237,0.18)]',
      badge: 'bg-violet-500/15 text-violet-300 border-violet-500/30',
    },
    cyan: {
      iconBg: 'bg-cyan-500/10 border-cyan-500/25 text-cyan-400',
      borderHover: 'hover:border-cyan-500/40',
      bottomLine: 'bg-gradient-to-r from-cyan-500 via-teal-400 to-blue-500',
      glow: 'group-hover:shadow-[0_0_20px_rgba(6,182,212,0.18)]',
      badge: 'bg-cyan-500/15 text-cyan-300 border-cyan-500/30',
    },
    emerald: {
      iconBg: 'bg-emerald-500/10 border-emerald-500/25 text-emerald-400',
      borderHover: 'hover:border-emerald-500/40',
      bottomLine: 'bg-gradient-to-r from-emerald-500 via-teal-400 to-cyan-500',
      glow: 'group-hover:shadow-[0_0_20px_rgba(16,185,129,0.18)]',
      badge: 'bg-emerald-500/15 text-emerald-300 border-emerald-500/30',
    },
    rose: {
      iconBg: 'bg-rose-500/10 border-rose-500/25 text-rose-400',
      borderHover: 'hover:border-rose-500/40',
      bottomLine: 'bg-gradient-to-r from-rose-500 via-pink-500 to-amber-500',
      glow: 'group-hover:shadow-[0_0_20px_rgba(244,63,94,0.18)]',
      badge: 'bg-rose-500/15 text-rose-300 border-rose-500/30',
    },
    amber: {
      iconBg: 'bg-amber-500/10 border-amber-500/25 text-amber-400',
      borderHover: 'hover:border-amber-500/40',
      bottomLine: 'bg-gradient-to-r from-amber-500 via-yellow-400 to-orange-500',
      glow: 'group-hover:shadow-[0_0_20px_rgba(245,158,11,0.18)]',
      badge: 'bg-amber-500/15 text-amber-300 border-amber-500/30',
    },
    slate: {
      iconBg: 'bg-slate-500/10 border-slate-500/25 text-slate-300',
      borderHover: 'hover:border-slate-400/30',
      bottomLine: 'bg-gradient-to-r from-slate-600 via-slate-400 to-slate-500',
      glow: 'group-hover:shadow-[0_0_20px_rgba(148,163,184,0.1)]',
      badge: 'bg-slate-500/15 text-slate-300 border-slate-500/30',
    },
  };

  const style = $derived(variantStyles[variant] || variantStyles.violet);

  function handleClick() {
    if (onclick) {
      onclick();
    }
  }
</script>

{#snippet cardContent()}
  <!-- Top header row -->
  <div class="flex items-center justify-between gap-3 text-slate-400">
    <span class="text-xs font-semibold tracking-wider uppercase text-slate-400 truncate">
      {title}
    </span>
    {#if Icon}
      <div class="w-8 h-8 rounded-xl flex items-center justify-center border {style.iconBg} shadow-sm shrink-0 transition-transform duration-200 group-hover:scale-105">
        <Icon class="w-4 h-4" />
      </div>
    {/if}
  </div>

  <!-- Primary Metric Value -->
  <div class="mt-3 flex items-baseline gap-2.5">
    <span class="text-3xl font-extrabold text-white tracking-tight font-mono">
      {value}
    </span>
    {#if badge}
      <span class="inline-flex items-center px-2 py-0.5 rounded-full text-[11px] font-mono font-medium border {badgeColor || style.badge}">
        {badge}
      </span>
    {/if}
    {#if trend}
      <span class="inline-flex items-center gap-1 text-[11px] font-mono font-medium {trend.positive ? 'text-emerald-400' : 'text-rose-400'}">
        <span>{trend.positive ? '↑' : '↓'}</span>
        <span>{trend.label}</span>
      </span>
    {/if}
  </div>

  <!-- Subtitle description if present -->
  {#if subtitle}
    <p class="mt-2 text-xs text-slate-400 leading-relaxed truncate">
      {subtitle}
    </p>
  {/if}

  <!-- Children custom snippet if provided -->
  {#if children}
    <div class="mt-3">
      {@render children()}
    </div>
  {/if}

  <!-- Bottom neon accent border -->
  <div class="absolute bottom-0 left-0 right-0 h-0.5 {style.bottomLine} opacity-50 group-hover:opacity-100 transition-opacity duration-300"></div>
{/snippet}

{#if onclick || clickable}
  <button
    type="button"
    class="text-left w-full p-5 rounded-2xl bg-[#12131c] border border-white/[0.08] relative overflow-hidden backdrop-blur-md transition-all duration-200 group {style.borderHover} {style.glow} cursor-pointer hover:-translate-y-0.5 select-none focus:outline-none focus:ring-1 focus:ring-violet-500/50 {className}"
    onclick={handleClick}
  >
    {@render cardContent()}
  </button>
{:else}
  <div
    class="p-5 rounded-2xl bg-[#12131c] border border-white/[0.08] relative overflow-hidden backdrop-blur-md transition-all duration-200 group {style.borderHover} {style.glow} {className}"
  >
    {@render cardContent()}
  </div>
{/if}
