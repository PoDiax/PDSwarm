<script lang="ts">
  import {
    Activity,
    LoaderCircle,
    ChevronDown,
    ChevronUp,
    Terminal,
    X,
    CircleCheck,
    CircleAlert,
    Maximize2,
    Minimize2,
  } from '@lucide/svelte';
  import { tasksStore } from '../stores/tasks.svelte';
  import TaskLogsModal from './TaskLogsModal.svelte';
  import type { BackgroundTask } from '../types';

  let isCollapsed = $state(false);
  let selectedTaskForLogs = $state<BackgroundTask | null>(null);
  let isLogsModalOpen = $state(false);

  let activeCount = $derived(tasksStore.activeCount);
  let activeTasks = $derived(tasksStore.activeTasks);
  let recentTasks = $derived(tasksStore.recentTasks);

  // Automatically expand if a new task begins running
  let prevActiveCount = 0;
  $effect(() => {
    if (activeCount > prevActiveCount) {
      isCollapsed = false;
    }
    prevActiveCount = activeCount;
  });

  function openLogs(task: BackgroundTask, e: MouseEvent) {
    e.stopPropagation();
    selectedTaskForLogs = task;
    isLogsModalOpen = true;
  }

  function closeLogs() {
    isLogsModalOpen = false;
    selectedTaskForLogs = null;
  }

  async function handleCancel(taskId: string, e: MouseEvent) {
    e.stopPropagation();
    try {
      await tasksStore.cancelTask(taskId);
    } catch {
      // ignore
    }
  }

  async function handleDismiss(taskId: string, e: MouseEvent) {
    e.stopPropagation();
    await tasksStore.dismissTask(taskId);
  }
</script>

{#if activeCount > 0 || recentTasks.length > 0}
  <aside
    aria-label="Background Tasks"
    class="fixed top-20 right-4 sm:right-6 z-40 max-w-[360px] sm:max-w-[400px] w-full pointer-events-auto select-none transition-all duration-300 ease-out"
  >
    {#if isCollapsed}
      <!-- Collapsed Compact Pill -->
      <div class="flex justify-end">
        <button
          onclick={() => (isCollapsed = false)}
          type="button"
          class="flex items-center gap-2.5 px-3.5 py-2 rounded-full bg-[#0d0e17]/95 hover:bg-[#12131f] border border-cyan-500/40 hover:border-cyan-400 text-white shadow-[0_10px_30px_rgba(6,182,212,0.25)] backdrop-blur-xl transition group text-left max-w-full"
          title="Click to expand background tasks"
        >
          {#if activeCount > 0}
            <div class="relative flex items-center justify-center shrink-0">
              <LoaderCircle class="w-4 h-4 animate-spin text-cyan-400" />
              <span class="absolute -top-1 -right-1 w-2 h-2 rounded-full bg-cyan-400 animate-ping"></span>
            </div>
            <div class="min-w-0 flex-1 truncate text-xs">
              <span class="font-bold text-cyan-300 font-mono">
                {activeCount} {activeCount === 1 ? 'task' : 'tasks'}
              </span>
              {#if activeTasks[0]}
                <span class="text-slate-300 font-medium ml-1 truncate">
                  • {activeTasks[0].title} ({activeTasks[0].progress}%)
                </span>
              {/if}
            </div>
          {:else if recentTasks.length > 0}
            {#if recentTasks[0].status === 'completed'}
              <CircleCheck class="w-4 h-4 text-emerald-400 shrink-0" />
              <span class="text-xs font-semibold text-emerald-300 truncate">
                {recentTasks[0].title} finished
              </span>
            {:else}
              <CircleAlert class="w-4 h-4 text-rose-400 shrink-0" />
              <span class="text-xs font-semibold text-rose-300 truncate">
                {recentTasks[0].title} failed
              </span>
            {/if}
          {/if}

          <ChevronDown class="w-3.5 h-3.5 text-cyan-400/80 group-hover:text-cyan-300 transition-transform shrink-0" />
        </button>
      </div>
    {:else}
      <!-- Expanded Full Notification Box -->
      <div
        class="rounded-2xl bg-[#0e0f17]/95 backdrop-blur-2xl border border-white/[0.12] shadow-[0_20px_50px_rgba(0,0,0,0.8)] overflow-hidden transition-all duration-200 animate-in fade-in slide-in-from-top-2"
      >
        <!-- Neon top accent line -->
        <div class="h-1 w-full bg-gradient-to-r from-violet-500 via-cyan-400 to-emerald-400"></div>

        <!-- Header -->
        <div class="flex items-center justify-between px-4 py-2.5 border-b border-white/[0.08] bg-white/[0.02]">
          <div class="flex items-center gap-2">
            {#if activeCount > 0}
              <LoaderCircle class="w-4 h-4 animate-spin text-cyan-400" />
            {:else}
              <Activity class="w-4 h-4 text-violet-400" />
            {/if}
            <span class="text-xs font-bold text-white uppercase tracking-wider font-mono">
              Background Tasks
            </span>
            {#if activeCount > 0}
              <span class="px-2 py-0.5 rounded-full text-[10px] font-mono font-bold bg-cyan-500/20 text-cyan-300 border border-cyan-500/30">
                {activeCount} running
              </span>
            {/if}
          </div>

          <div class="flex items-center gap-1">
            <button
              onclick={() => (isCollapsed = true)}
              class="p-1 rounded-lg text-slate-400 hover:text-white hover:bg-white/[0.06] transition"
              title="Collapse to compact badge"
            >
              <ChevronUp class="w-4 h-4" />
            </button>
          </div>
        </div>

        <!-- Tasks Body List -->
        <div class="p-3 space-y-2.5 max-h-[380px] overflow-y-auto">
          <!-- 1. Active Tasks -->
          {#each activeTasks as task (task.id)}
            <div class="p-3 rounded-xl bg-[#12131c] border border-white/[0.06] hover:border-cyan-500/30 transition space-y-2">
              <div class="flex items-start justify-between gap-2">
                <div class="min-w-0 flex-1">
                  <div class="text-xs font-semibold text-white truncate" title={task.title}>
                    {task.title}
                  </div>
                  <div class="text-[11px] text-cyan-300 font-mono truncate mt-0.5 flex items-center gap-1.5">
                    <span class="w-1.5 h-1.5 rounded-full bg-cyan-400 animate-pulse"></span>
                    <span>{task.current_stage || 'Processing...'}</span>
                  </div>
                </div>

                <div class="flex items-center gap-1 shrink-0">
                  <button
                    onclick={(e) => openLogs(task, e)}
                    class="p-1.5 rounded-lg text-slate-400 hover:text-cyan-300 hover:bg-cyan-500/10 transition"
                    title="View Live Logs"
                  >
                    <Terminal class="w-3.5 h-3.5" />
                  </button>
                  <button
                    onclick={(e) => handleCancel(task.id, e)}
                    class="p-1.5 rounded-lg text-slate-400 hover:text-rose-400 hover:bg-rose-500/10 transition"
                    title="Cancel Task"
                  >
                    <X class="w-3.5 h-3.5" />
                  </button>
                </div>
              </div>

              <!-- Progress bar -->
              <div class="space-y-1">
                <div class="h-1.5 w-full bg-white/[0.06] rounded-full overflow-hidden">
                  <div
                    class="h-full bg-gradient-to-r from-violet-500 to-cyan-400 rounded-full transition-all duration-300 ease-out"
                    style="width: {Math.max(task.progress, 5)}%"
                  ></div>
                </div>
                <div class="flex justify-between text-[10px] font-mono text-slate-500">
                  <span>Progress</span>
                  <span class="text-cyan-400 font-semibold">{task.progress}%</span>
                </div>
              </div>
            </div>
          {/each}

          <!-- 2. Recent Completed / Failed Tasks -->
          {#each recentTasks as task (task.id)}
            <div
              class="p-2.5 rounded-xl border transition flex items-center justify-between gap-2 {task.status === 'completed'
                ? 'bg-emerald-950/20 border-emerald-500/30 text-emerald-300'
                : 'bg-rose-950/20 border-rose-500/30 text-rose-300'}"
            >
              <div class="flex items-center gap-2 min-w-0 flex-1">
                {#if task.status === 'completed'}
                  <CircleCheck class="w-4 h-4 text-emerald-400 shrink-0" />
                {:else}
                  <CircleAlert class="w-4 h-4 text-rose-400 shrink-0" />
                {/if}
                <div class="min-w-0 flex-1 truncate">
                  <div class="text-xs font-semibold truncate text-white">
                    {task.title}
                  </div>
                  <div class="text-[10px] font-mono opacity-80 truncate">
                    {task.status === 'completed' ? 'Completed successfully' : (task.error || 'Operation failed')}
                  </div>
                </div>
              </div>

              <div class="flex items-center gap-1 shrink-0">
                <button
                  onclick={(e) => openLogs(task, e)}
                  class="p-1 rounded-md text-slate-400 hover:text-white transition"
                  title="View Logs"
                >
                  <Terminal class="w-3.5 h-3.5" />
                </button>
                <button
                  onclick={(e) => handleDismiss(task.id, e)}
                  class="p-1 rounded-md text-slate-400 hover:text-white transition"
                  title="Dismiss"
                >
                  <X class="w-3.5 h-3.5" />
                </button>
              </div>
            </div>
          {/each}
        </div>
      </div>
    {/if}
  </aside>
{/if}

<!-- Live Logs Modal -->
<TaskLogsModal
  isOpen={isLogsModalOpen}
  task={selectedTaskForLogs}
  onClose={closeLogs}
/>
