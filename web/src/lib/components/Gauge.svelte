<script lang="ts">
  interface Props {
    value: number;
    max?: number;
    min?: number;
    label: string;
    sublabel?: string;
    size?: number;
    strokeWidth?: number;
    color?: 'cyan' | 'violet' | 'emerald' | 'amber' | 'rose';
    unit?: string;
    id?: string;
    class?: string;
  }

  let {
    value = 0,
    max = 100,
    min = 0,
    label,
    sublabel,
    size = 190,
    strokeWidth = 12,
    color = 'cyan',
    unit = '%',
    id,
    class: className = '',
  }: Props = $props();

  // Generate a stable unique ID for SVG gradient & filter definitions
  const autoId = Math.random().toString(36).slice(2, 8);
  const gaugeId = $derived(id || `gauge-${color}-${autoId}`);
  const gradId = $derived(`grad-${gaugeId}`);
  const glowId = $derived(`glow-${gaugeId}`);

  // Geometry calculations
  const cx = $derived(size / 2);
  const cy = $derived(size / 2);
  // Padding for filter glow & stroke width
  const r = $derived(Math.max(10, (size - strokeWidth * 2 - 16) / 2));
  const circumference = $derived(2 * Math.PI * r);
  // 240 degree arch
  const arcLength = $derived((240 / 360) * circumference);

  const clampedValue = $derived(Math.min(Math.max(value, min), max));
  const percentage = $derived(
    max > min ? ((clampedValue - min) / (max - min)) * 100 : 0
  );
  const activeLength = $derived((percentage / 100) * arcLength);

  // Neon color schemes
  const colorMap = {
    cyan: {
      gradientStart: '#00f5ff',
      gradientMid: '#06b6d4',
      gradientEnd: '#3b82f6',
      glowColor: 'rgba(0, 245, 255, 0.5)',
      textColor: 'text-[#00f5ff]',
      badgeBg: 'bg-cyan-500/15 border-cyan-500/30 text-cyan-300',
    },
    violet: {
      gradientStart: '#c084fc',
      gradientMid: '#8b5cf6',
      gradientEnd: '#6366f1',
      glowColor: 'rgba(139, 92, 246, 0.5)',
      textColor: 'text-violet-400',
      badgeBg: 'bg-violet-500/15 border-violet-500/30 text-violet-300',
    },
    emerald: {
      gradientStart: '#34d399',
      gradientMid: '#10b981',
      gradientEnd: '#059669',
      glowColor: 'rgba(16, 185, 129, 0.5)',
      textColor: 'text-emerald-400',
      badgeBg: 'bg-emerald-500/15 border-emerald-500/30 text-emerald-300',
    },
    amber: {
      gradientStart: '#fbbf24',
      gradientMid: '#f59e0b',
      gradientEnd: '#d97706',
      glowColor: 'rgba(245, 158, 11, 0.5)',
      textColor: 'text-amber-400',
      badgeBg: 'bg-amber-500/15 border-amber-500/30 text-amber-300',
    },
    rose: {
      gradientStart: '#fb7185',
      gradientMid: '#f43f5e',
      gradientEnd: '#e11d48',
      glowColor: 'rgba(244, 63, 94, 0.5)',
      textColor: 'text-rose-400',
      badgeBg: 'bg-rose-500/15 border-rose-500/30 text-rose-300',
    },
  };

  const scheme = $derived(colorMap[color] || colorMap.cyan);
</script>

<div class="flex flex-col items-center justify-center {className} relative select-none">
  <div class="relative flex items-center justify-center" style="width: {size}px; height: {size}px;">
    <svg
      width={size}
      height={size}
      viewBox="0 0 {size} {size}"
      class="overflow-visible"
    >
      <defs>
        <!-- Linear gradient for active arc -->
        <linearGradient id={gradId} x1="0%" y1="100%" x2="100%" y2="0%">
          <stop offset="0%" stop-color={scheme.gradientStart} />
          <stop offset="50%" stop-color={scheme.gradientMid} />
          <stop offset="100%" stop-color={scheme.gradientEnd} />
        </linearGradient>

        <!-- Neon glow filter -->
        <filter id={glowId} x="-30%" y="-30%" width="160%" height="160%">
          <feGaussianBlur stdDeviation="3" result="blur" />
          <feMerge>
            <feMergeNode in="blur" />
            <feMergeNode in="SourceGraphic" />
          </feMerge>
        </filter>
      </defs>

      <!-- Background track circle: 240 degree arc starting at 150 deg -->
      <circle
        {cx}
        {cy}
        {r}
        fill="none"
        stroke="rgba(255, 255, 255, 0.08)"
        stroke-width={strokeWidth}
        stroke-linecap="round"
        stroke-dasharray="{arcLength} {circumference}"
        transform="rotate(150, {cx}, {cy})"
      />

      <!-- Subtle inner tick guideline -->
      <circle
        {cx}
        {cy}
        r={r - strokeWidth * 0.9}
        fill="none"
        stroke="rgba(255, 255, 255, 0.03)"
        stroke-width="1"
        stroke-dasharray="2 6"
        transform="rotate(150, {cx}, {cy})"
      />

      <!-- Glowing Active Stroke -->
      {#if activeLength > 0}
        <circle
          {cx}
          {cy}
          {r}
          fill="none"
          stroke="url(#{gradId})"
          stroke-width={strokeWidth}
          stroke-linecap="round"
          stroke-dasharray="{activeLength} {circumference}"
          filter="url(#{glowId})"
          transform="rotate(150, {cx}, {cy})"
          class="transition-all duration-700 ease-out"
        />
      {/if}
    </svg>

    <!-- Center Readout -->
    <div class="absolute inset-0 flex flex-col items-center justify-center text-center px-4 pointer-events-none">
      <div class="flex items-baseline justify-center gap-0.5">
        <span class="text-3xl font-black font-mono tracking-tight text-white drop-shadow-[0_0_12px_{scheme.glowColor}]">
          {Math.round(clampedValue)}
        </span>
        <span class="text-sm font-bold font-mono {scheme.textColor}">
          {unit}
        </span>
      </div>

      <span class="text-xs font-semibold uppercase tracking-wider text-slate-300 mt-1 max-w-[120px] truncate">
        {label}
      </span>

      {#if sublabel}
        <span class="text-[11px] font-mono text-slate-400 mt-0.5 max-w-[120px] truncate">
          {sublabel}
        </span>
      {/if}
    </div>
  </div>

  <!-- Min and Max range markers at bottom arc endpoints -->
  <div class="w-full flex justify-between px-6 -mt-3 text-[10px] font-mono text-slate-400">
    <span>{min}{unit}</span>
    <span>{max}{unit}</span>
  </div>
</div>
