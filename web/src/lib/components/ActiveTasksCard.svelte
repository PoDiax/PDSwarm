<script lang="ts">
  import {
    Activity,
    Download,
    Cpu,
    Layers,
    CircleCheck,
    CircleAlert,
    XCircle,
    Clock,
    Terminal,
    X,
    LoaderCircle,
  } from '@lucide/svelte';
  import { tasksStore } from '../stores/tasks.svelte';
  import TaskLogsModal from './TaskLogsModal.svelte';
  import type { BackgroundTask } from '../types';

  let selectedTaskForLogs = $state<BackgroundTask | null>(null);
  let isLogsModalOpen = $state(false);

  function openLogs(task: BackgroundTask) {
    selectedTaskForLogs = task;
    isLogsModalOpen = true;
  }

  function closeLogs() {
    isLogsModalOpen = false;
    selectedTaskForLogs = null;
  }

  function getTaskIcon(type: string) {
    switch (type) {
      case 'image_pull':
        return Download;
      case 'image_build':
        return Cpu;
      case 'stack_deploy':
      case 'template_deploy':
        return Layers;
      default:
        return Activity;
    }
  }

  function formatDuration(startStr?: string, endStr?: string): string {
    if (!startStr) return '0s';
    const start = new Date(startStr).getTime();
    const end = endStr ? new Date(endStr).getTime() : Date.now();
    const sec = Math.max(0, Math.floor((end - start) / 1000));
    if (sec < 60) return `${sec}s`;
    const min = Math.floor(sec / 60);
    const remSec = sec % 60;
    return `${min}m ${remSec}s`;
  }
</script>

{#if tasksStore.activeTasks.length > 0 || tasksStore.recentTasks.length > 0}
  <div class="rounded-2xl bg-[#12131c]/90 border border-violet-500/20 shadow-xl overflow-hidden mb-6 backdrop-blur-md">
    <!-- Header -->
    <div class="px-5 py-3.5 border-b border-white/[0.08] flex items-center justify-between bg-gradient-to-r from-violet-600/10 via-transparent to-cyan-500/10">
      <div class="flex items-center gap-2.5">
        <div class="relative flex h-2.5 w-2.5">
          {#if tasksStore.activeCount > 0}
            <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-cyan-400 opacity-75"></span>
            <span class="relative inline-flex rounded-full h-2.5 w-2.5 bg-cyan-500 shadow-[0_0_10px_#06b6d4]"></span>
          {:else}
            <span class="relative inline-flex rounded-full h-2.5 w-2.5 bg-emerald-500"></span>
          {/if}
        </div>
        <h3 class="text-sm font-semibold text-white tracking-wide flex items-center gap-2">
          Operations & Background Tasks
          {#if tasksStore.activeCount > 0}
            <span class="text-[11px] font-mono px-2 py-0.5 rounded-full bg-cyan-500/15 text-cyan-300 border border-cyan-500/30">
              {tasksStore.activeCount} Running
            </span>
          {/if}
        </h3>
      </div>
    </div>

    <!-- Active Tasks List -->
    <div class="divide-y divide-white/[0.06]">
      {#each tasksStore.activeTasks as task (task.id)}
        {@const Icon = getTaskIcon(task.type)}
        <div class="p-4 sm:p-5 bg-violet-600/[0.02] hover:bg-violet-600/[0.04] transition">
          <div class="flex items-start justify-between gap-4">
            <div class="flex items-start gap-3 min-w-0 flex-1">
              <div class="w-9 h-9 rounded-xl bg-cyan-500/10 border border-cyan-500/20 text-cyan-400 flex items-center justify-center shrink-0 shadow-[0_0_12px_rgba(6,182,212,0.15)] mt-0.5">
                <Icon class="w-4 h-4 animate-bounce" />
              </div>

              <div class="min-w-0 flex-1">
                <div class="flex items-center gap-2 flex-wrap">
                  <h4 class="text-sm font-semibold text-white truncate max-w-lg">
                    {task.title}
                  </h4>
                  <span class="text-[10px] font-mono px-2 py-0.5 rounded-full bg-cyan-500/15 text-cyan-300 border border-cyan-500/30 flex items-center gap-1">
                    <LoaderCircle class="w-3 h-3 animate-spin" />
                    Running ({task.progress}%)
                  </span>
                </div>

                <p class="text-xs text-slate-400 font-mono mt-1 truncate">
                  {task.current_stage || 'Processing operation...'}
                </p>

                <!-- Live Progress Bar -->
                <div class="mt-3 w-full bg-black/40 h-2 rounded-full overflow-hidden border border-white/[0.06]">
                  <div
                    class="h-full bg-gradient-to-r from-violet-600 to-cyan-400 transition-all duration-300 rounded-full shadow-[0_0_10px_rgba(6,182,212,0.5)]"
                    style="width: {Math.max(5, task.progress)}%"
                  ></div>
                </div>
              </div>
            </div>

            <!-- Actions -->
            <div class="flex items-center gap-2 shrink-0">
              <button
                onclick={() => openLogs(task)}
                class="px-3 py-1.5 rounded-xl bg-white/[0.06] hover:bg-white/[0.1] border border-white/[0.08] text-xs font-medium text-slate-300 hover:text-white transition flex items-center gap-1.5"
                title="View Live Task Logs"
              >
                <Terminal class="w-3.5 h-3.5 text-cyan-400" />
                <span class="hidden sm:inline">View Logs</span>
              </button>

              <button
                onclick={() => tasksStore.cancelTask(task.id)}
                class="px-2.5 py-1.5 rounded-xl text-xs font-medium text-slate-400 hover:text-rose-400 hover:bg-rose-500/10 transition"
                title="Cancel Task"
              >
                Cancel
              </button>
            </div>
          </div>
        </div>
      {/each}

      <!-- Recent Completed / Failed Tasks (If any) -->
      {#if tasksStore.recentTasks.length > 0}
        <div class="px-5 py-2.5 bg-[#0e0f17]/60 text-[11px] font-semibold text-slate-400 uppercase tracking-wider flex items-center justify-between border-t border-white/[0.04]">
          <span>Recent Operation</span>
        </div>

        {#each tasksStore.recentTasks.slice(0, 1) as task (task.id)}
          <div class="px-5 py-3 flex items-center justify-between gap-3 text-xs hover:bg-white/[0.02] transition">
            <div class="flex items-center gap-3 min-w-0">
              {#if task.status === 'completed'}
                <CircleCheck class="w-4 h-4 text-emerald-400 shrink-0" />
              {:else if task.status === 'failed'}
                <CircleAlert class="w-4 h-4 text-rose-400 shrink-0" />
              {:else}
                <XCircle class="w-4 h-4 text-slate-500 shrink-0" />
              {/if}

              <div class="min-w-0">
                <span class="font-medium text-slate-200 truncate block max-w-md">
                  {task.title}
                </span>
                <span class="text-[10px] text-slate-500 font-mono">
                  {task.status === 'completed' ? 'Completed successfully' : task.error || task.status} • {formatDuration(task.started_at, task.finished_at)}
                </span>
              </div>
            </div>

            <div class="flex items-center gap-2 shrink-0">
              <button
                onclick={() => openLogs(task)}
                class="p-1.5 rounded-lg text-slate-400 hover:text-white hover:bg-white/5 transition"
                title="View Logs"
              >
                <Terminal class="w-3.5 h-3.5" />
              </button>
              <button
                onclick={() => tasksStore.dismissTask(task.id)}
                class="p-1.5 rounded-lg text-slate-500 hover:text-slate-300 hover:bg-white/5 transition"
                title="Dismiss"
              >
                <X class="w-3.5 h-3.5" />
              </button>
            </div>
          </div>
        {/each}
      {/if}
    </div>
  </div>
{/if}

<!-- Logs Modal -->
<TaskLogsModal
  task={selectedTaskForLogs}
  isOpen={isLogsModalOpen}
  onClose={closeLogs}
/>
