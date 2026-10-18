import { taskApi } from '../api';
import type { BackgroundTask } from '../types';

class TasksStore {
  tasks = $state<BackgroundTask[]>([]);
  isLoading = $state(false);
  lastError = $state<string | null>(null);

  // Active running tasks
  activeTasks = $derived(
    this.tasks.filter((t) => t.status === 'running' || t.status === 'pending')
  );

  // Recent completed/failed tasks (limit to 1 recent operation)
  recentTasks = $derived(
    this.tasks
      .filter((t) => t.status !== 'running' && t.status !== 'pending')
      .slice(0, 1)
  );

  // Count of currently active operations
  activeCount = $derived(this.activeTasks.length);

  private pollTimer: ReturnType<typeof setTimeout> | null = null;
  private isPolling = false;

  constructor() {
    if (typeof window !== 'undefined') {
      this.startPolling();
    }
  }

  async fetchTasks(silent = true) {
    if (!silent) this.isLoading = true;
    try {
      const data = await taskApi.list();
      this.tasks = data || [];
      this.lastError = null;
    } catch (err: unknown) {
      this.lastError = err instanceof Error ? err.message : 'Failed to fetch tasks';
    } finally {
      this.isLoading = false;
    }
  }

  async cancelTask(id: string) {
    try {
      await taskApi.cancel(id);
      await this.fetchTasks(true);
    } catch (err: unknown) {
      this.lastError = err instanceof Error ? err.message : 'Failed to cancel task';
      throw err;
    }
  }

  async dismissTask(id: string) {
    try {
      await taskApi.delete(id);
      this.tasks = this.tasks.filter((t) => t.id !== id);
    } catch (err: unknown) {
      this.lastError = err instanceof Error ? err.message : 'Failed to dismiss task';
    }
  }

  async poll() {
    return this.fetchTasks(true);
  }

  startPolling() {
    if (this.isPolling) return;
    this.isPolling = true;

    const poll = async () => {
      if (!this.isPolling) return;
      await this.fetchTasks(true);

      // Fast polling if active tasks exist (1.5s), slower if idle (5s)
      const nextDelay = this.activeCount > 0 ? 1500 : 5000;
      this.pollTimer = setTimeout(poll, nextDelay);
    };

    poll();
  }

  stopPolling() {
    this.isPolling = false;
    if (this.pollTimer) {
      clearTimeout(this.pollTimer);
      this.pollTimer = null;
    }
  }
}

export const tasksStore = new TasksStore();
