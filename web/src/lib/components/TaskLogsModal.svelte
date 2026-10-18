<script lang="ts">
  import {
    X,
    Terminal,
    Copy,
    Check,
    Loader2,
    RefreshCw,
    Download,
    Trash2,
  } from '@lucide/svelte';
  import { portal } from '../actions/portal';
  import { taskApi } from '../api';
  import type { BackgroundTask } from '../types';

  interface Props {
    task: BackgroundTask | null;
    isOpen: boolean;
    onClose: () => void;
  }

  let { task = null, isOpen = false, onClose }: Props = $props();

  let liveTask = $state<BackgroundTask | null>(null);
  let isCopied = $state(false);
  let pollInterval: ReturnType<typeof setInterval> | null = null;
  let logBox = $state<HTMLDivElement | null>(null);

  $effect(() => {
    if (isOpen && task) {
      liveTask = task;
      refreshLogs();
      // Poll every 1s while modal is open and task is running
      pollInterval = setInterval(refreshLogs, 1000);
    } else {
      if (pollInterval) {
        clearInterval(pollInterval);
        pollInterval = null;
      }
    }

    return () => {
      if (pollInterval) {
        clearInterval(pollInterval);
        pollInterval = null;
      }
    };
  });

  async function refreshLogs() {
    if (!task) return;
    try {
      const updated = await taskApi.get(task.id);
      liveTask = updated;
      scrollToBottom();
    } catch {
      // ignore
    }
  }

  function scrollToBottom() {
    if (logBox) {
      setTimeout(() => {
        if (logBox) logBox.scrollTop = logBox.scrollHeight;
      }, 50);
    }
  }

  async function copyLogs() {
    if (!liveTask || !liveTask.logs) return;
    try {
      await navigator.clipboard.writeText(liveTask.logs.join('\n'));
      isCopied = true;
      setTimeout(() => (isCopied = false), 2000);
    } catch {
      // ignore
    }
  }

  function handleClose() {
    onClose();
  }
</script>

<svelte:window
  onkeydown={(e) => {
    if (isOpen && e.key === 'Escape') handleClose();
  }}
/>

{#if isOpen && liveTask}
  <div
    use:portal
    class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-md animate-in fade-in duration-200"
    role="dialog"
    tabindex="-1"
    aria-modal="true"
    onclick={(e) => {
      if (e.target === e.currentTarget) handleClose();
    }}
    onkeydown={(e) => {
      if (e.key === 'Escape') handleClose();
    }}
  >
    <div
      class="relative w-full max-w-3xl bg-[#0e0f17] border border-white/[0.08] rounded-2xl shadow-[0_25px_60px_rgba(0,0,0,0.85)] overflow-hidden flex flex-col h-[80vh]"
    >
      <!-- Header -->
      <div class="px-6 py-4 border-b border-white/[0.08] flex items-center justify-between bg-[#12131c] shrink-0">
        <div class="flex items-center gap-3 min-w-0">
          <div class="w-9 h-9 rounded-xl bg-violet-600/20 border border-violet-500/30 flex items-center justify-center shadow-[0_0_15px_rgba(124,58,237,0.2)] shrink-0">
            <Terminal class="w-4 h-4 text-violet-400" />
          </div>
          <div class="min-w-0">
            <div class="flex items-center gap-2">
              <h3 class="text-sm font-semibold text-white truncate max-w-md">
                {liveTask.title}
              </h3>
              <span
                class="text-[10px] font-mono px-2 py-0.5 rounded-full border {liveTask.status === 'running' ? 'bg-cyan-500/10 text-cyan-300 border-cyan-500/20 animate-pulse' : liveTask.status === 'completed' ? 'bg-emerald-500/10 text-emerald-300 border-emerald-500/20' : 'bg-rose-500/10 text-rose-300 border-rose-500/20'}"
              >
                {liveTask.status}
              </span>
            </div>
            <p class="text-xs text-slate-400 font-mono mt-0.5 truncate">
              {liveTask.current_stage || (liveTask.status === 'completed' ? 'Completed successfully' : 'In progress')}
            </p>
          </div>
        </div>

        <div class="flex items-center gap-2 shrink-0">
          <!-- Copy Logs -->
          <button
            onclick={copyLogs}
            class="px-2.5 py-1.5 rounded-lg bg-white/5 hover:bg-white/10 text-xs font-medium text-slate-300 hover:text-white transition flex items-center gap-1.5"
            title="Copy all logs"
          >
            {#if isCopied}
              <Check class="w-3.5 h-3.5 text-emerald-400" />
              <span class="text-emerald-400">Copied</span>
            {:else}
              <Copy class="w-3.5 h-3.5" />
              <span>Copy</span>
            {/if}
          </button>

          <!-- Close -->
          <button
            onclick={handleClose}
            class="p-2 rounded-lg text-slate-400 hover:text-white hover:bg-white/5 transition"
            aria-label="Close logs modal"
          >
            <X class="w-5 h-5" />
          </button>
        </div>
      </div>

      <!-- Log Output Box -->
      <div
        bind:this={logBox}
        class="flex-1 p-4 bg-[#08090d] overflow-y-auto font-mono text-xs text-slate-300 select-text leading-relaxed space-y-1"
      >
        {#if liveTask.logs && liveTask.logs.length > 0}
          {#each liveTask.logs as line, i}
            <div class="flex items-start gap-3 hover:bg-white/[0.02] px-1 rounded">
              <span class="text-slate-600 select-none text-[10px] w-8 text-right shrink-0">{i + 1}</span>
              <span class="break-all whitespace-pre-wrap {line.toLowerCase().includes('error') ? 'text-rose-400' : line.toLowerCase().includes('success') ? 'text-emerald-300' : 'text-slate-300'}">{line}</span>
            </div>
          {/each}
        {:else}
          <div class="h-full flex items-center justify-center text-slate-600 text-xs italic">
            Waiting for log output...
          </div>
        {/if}
      </div>

      <!-- Footer Bar -->
      <div class="px-6 py-2.5 border-t border-white/[0.08] bg-[#12131c]/90 flex items-center justify-between text-xs font-mono text-slate-500 shrink-0">
        <div class="flex items-center gap-2">
          {#if liveTask.status === 'running'}
            <span class="relative flex h-2 w-2">
              <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-cyan-400 opacity-75"></span>
              <span class="relative inline-flex rounded-full h-2 w-2 bg-cyan-500"></span>
            </span>
            <span class="text-slate-400">Streaming live updates ({liveTask.progress}%)</span>
          {:else}
            <span>Finished at {new Date(liveTask.finished_at || liveTask.created_at).toLocaleTimeString()}</span>
          {/if}
        </div>
        <div>
          <span>{liveTask.logs?.length || 0} lines</span>
        </div>
      </div>
    </div>
  </div>
{/if}
