import { templateApi } from '../api';
import type { Template, DeployProgressEvent } from '../types';

export interface DeploymentLogItem {
  id: string;
  time: string;
  step: string;
  status: string;
  progress?: string;
  detail?: string;
  isError?: boolean;
}

export interface DeploymentTask {
  id: string;
  stackName: string;
  templateTitle: string;
  templateLogo?: string;
  status: 'deploying' | 'success' | 'error';
  step: string;
  currentStatus: string;
  progress?: string;
  detail?: string;
  error?: string;
  startedAt: Date;
  finishedAt?: Date;
  logs: DeploymentLogItem[];
}

class DeploymentStore {
  tasks = $state<DeploymentTask[]>([]);

  get activeCount(): number {
    return this.tasks.filter((t) => t.status === 'deploying').length;
  }

  get allTasks(): DeploymentTask[] {
    return this.tasks;
  }

  startDeployment(
    template: Template,
    stackName: string,
    envValues: Record<string, string>,
    composeYaml: string
  ): string {
    const id = `deploy-${Date.now()}-${Math.random().toString(36).substring(2, 7)}`;
    const now = new Date();
    const timeStr = now.toLocaleTimeString([], { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' });

    const newTask: DeploymentTask = {
      id,
      stackName,
      templateTitle: template.title || stackName,
      templateLogo: template.logo,
      status: 'deploying',
      step: 'init',
      currentStatus: `Initializing deployment for "${stackName}"...`,
      startedAt: now,
      logs: [
        {
          id: `log-${Date.now()}-1`,
          time: timeStr,
          step: 'init',
          status: `Deployment task started for "${stackName}"`,
        },
      ],
    };

    // Prepend to list so newest is first
    this.tasks = [newTask, ...this.tasks];

    // Launch streaming deploy in background
    (async () => {
      try {
        await templateApi.deployStream(
          {
            template_id: template.id,
            stack_name: stackName,
            env: envValues,
            compose_yaml: composeYaml,
          },
          (evt: DeployProgressEvent) => {
            this.handleProgress(id, evt);
          }
        );

        this.handleComplete(id, stackName);
      } catch (err: unknown) {
        const e = err as { message?: string };
        const msg = e?.message || 'Deployment failed due to unexpected error.';
        this.handleError(id, msg);
      }
    })();

    return id;
  }

  private handleProgress(taskId: string, evt: DeployProgressEvent) {
    const taskIndex = this.tasks.findIndex((t) => t.id === taskId);
    if (taskIndex === -1) return;

    const task = this.tasks[taskIndex];
    const now = new Date();
    const timeStr = now.toLocaleTimeString([], { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' });

    const newLog: DeploymentLogItem = {
      id: `log-${Date.now()}-${Math.random().toString(36).substring(2, 6)}`,
      time: timeStr,
      step: evt.step,
      status: evt.status,
      progress: evt.progress,
      detail: evt.detail,
      isError: evt.step === 'error',
    };

    this.tasks[taskIndex] = {
      ...task,
      step: evt.step,
      currentStatus: evt.status,
      progress: evt.progress,
      detail: evt.detail,
      logs: [...task.logs, newLog],
    };
  }

  private handleComplete(taskId: string, stackName: string) {
    const taskIndex = this.tasks.findIndex((t) => t.id === taskId);
    if (taskIndex === -1) return;

    const task = this.tasks[taskIndex];
    const now = new Date();
    const timeStr = now.toLocaleTimeString([], { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' });

    const doneLog: DeploymentLogItem = {
      id: `log-${Date.now()}-done`,
      time: timeStr,
      step: 'done',
      status: `Stack "${stackName}" deployed and running!`,
    };

    this.tasks[taskIndex] = {
      ...task,
      status: 'success',
      step: 'done',
      currentStatus: `Application "${stackName}" deployed successfully!`,
      finishedAt: now,
      logs: [...task.logs, doneLog],
    };
  }

  private handleError(taskId: string, errorMsg: string) {
    const taskIndex = this.tasks.findIndex((t) => t.id === taskId);
    if (taskIndex === -1) return;

    const task = this.tasks[taskIndex];
    const now = new Date();
    const timeStr = now.toLocaleTimeString([], { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' });

    const errLog: DeploymentLogItem = {
      id: `log-${Date.now()}-err`,
      time: timeStr,
      step: 'error',
      status: 'Deployment failed',
      detail: errorMsg,
      isError: true,
    };

    this.tasks[taskIndex] = {
      ...task,
      status: 'error',
      step: 'error',
      currentStatus: 'Deployment failed',
      error: errorMsg,
      finishedAt: now,
      logs: [...task.logs, errLog],
    };
  }

  dismiss(taskId: string) {
    this.tasks = this.tasks.filter((t) => t.id !== taskId);
  }

  clearFinished() {
    this.tasks = this.tasks.filter((t) => t.status === 'deploying');
  }
}

export const deploymentStore = new DeploymentStore();
