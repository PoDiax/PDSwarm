<script lang="ts">
  import {
    Activity,
    LoaderCircle,
    ChevronDown,
    Terminal,
    X,
    CircleCheck,
    Download,
    Cpu,
    Layers,
  } from '@lucide/svelte';
  import { tasksStore } from '../stores/tasks.svelte';
  import TaskLogsModal from './TaskLogsModal.svelte';
  import type { BackgroundTask } from '../types';

  let isOpen = $state(false);
  let selectedTaskForLogs = $state<BackgroundTask | null>(null);
  let isLogsModalOpen = $state(false);

  function toggleDropdown() {
    isOpen = !isOpen;
  }

  function closeDropdown() {
    isOpen = false;
  }

  function openLogs(task: BackgroundTask, e: MouseEvent) {
    e.stopPropagation();
    selectedTaskForLogs = task;
    isLogsModalOpen = true;
    isOpen = false;
  }

  function closeLogs() {
    isLogsModalOpen = false;
    selectedTaskForLogs = null;
  }

  let activeCount = $derived(tasksStore.activeCount);
  let activeTasks = $derived(tasksStore.activeTasks);
</script>

<svelte:window onclick={(e) => {
  const target = e.target as HTMLElement;
  if (!target.closest('#navbar-tasks-container')) {
    closeDropdown();
  }
}} />

{#if activeCount > 0}
  <div class="relative" id="navbar-tasks-container">
    <button
      onclick={toggleDropdown}
      type="button"
      class="flex items-center gap-2 px-2.5 py-1 rounded-full bg-cyan-950/40 hover:bg-cyan-950/60 border border-cyan-500/30 text-cyan-300 text-xs font-medium transition shadow-[0_0_12px_rgba(6,182,212,0.15)] group select-none animate-in fade-in duration-200"
      aria-expanded={isOpen}
      title="{activeCount} background task(s) active"
    >
      <LoaderCircle class="w-3.5 h-3.5 animate-spin text-cyan-400 shrink-0" />
      <span class="font-mono text-[11px] font-semibold">{activeCount} {activeCount === 1 ? 'task' : 'tasks'}</span>
      <ChevronDown class="w-3 h-3 text-cyan-400/70 group-hover:text-cyan-300 transition-transform duration-200 {isOpen ? 'rotate-180' : ''}" />
    </button>

    {#if isOpen}
      <div
        class="absolute right-0 mt-2 w-80 sm:w-96 rounded-2xl bg-[#0e0f17]/95 backdrop-blur-xl border border-white/[0.08] shadow-[0_15px_35px_rgba(0,0,0,0.7)] p-3 z-50 animate-in fade-in duration-150 space-y-2"
        role="menu"
      >
        <div class="flex items-center justify-between border-b border-white/[0.06] pb-2 px-1">
          <span class="text-[11px] font-semibold text-slate-400 uppercase tracking-wider flex items-center gap-1.5 font-mono">
            <Activity class="w-3.5 h-3.5 text-cyan-400" />
            Active Tasks ({activeCount})
          </span>
          <span class="text-[10px] text-slate-500 font-mono">running</span>
        </div>

        <div class="max-h-72 overflow-y-auto space-y-2 py-1">
          {#each activeTasks as task (task.id)}
            <div class="p-2.5 rounded-xl bg-[#12131c] border border-white/[0.06] hover:border-cyan-500/30 transition">
              <div class="flex items-start justify-between gap-2">
                <div class="min-w-0 flex-1">
                  <div class="text-xs font-semibold text-white truncate">
                    {task.title}
                  </div>
                  <div class="text-[11px] text-slate-400 font-mono truncate mt-0.5">
                    {task.current_stage || 'Working...'}
                  </div>
                </div>

                <button
                  onclick={(e) => openLogs(task, e)}
                  class="p-1 rounded-lg text-slate-400 hover:text-cyan-300 hover:bg-cyan-500/10 transition shrink-0"
                  title="View Logs"
                >
                  <Terminal class="w-3.5 h-3.5" />
                </button>
              </div>

              <!-- Progress bar -->
              <div class="mt-2 flex items-center gap-2">
                <div class="flex-1 bg-black/40 h-1.5 rounded-full overflow-hidden border border-white/[0.04]">
                  <div
                    class="h-full bg-gradient-to-r from-violet-500 to-cyan-400 rounded-full transition-all duration-300"
                    style="width: {Math.max(5, task.progress)}%"
                  ></div>
                </div>
                <span class="text-[10px] font-mono text-cyan-300 shrink-0 font-medium">{task.progress}%</span>
              </div>
            </div>
          {/each}
        </div>
      </div>
    {/if}
  </div>
{/if}

<!-- Logs Modal -->
<TaskLogsModal
  task={selectedTaskForLogs}
  isOpen={isLogsModalOpen}
  onClose={closeLogs}
/>
