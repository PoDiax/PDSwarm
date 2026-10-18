import { environmentApi } from '../api';
import type {
  Environment,
  CreateEnvironmentRequest,
  UpdateEnvironmentRequest,
  PingEnvironmentResponse,
} from '../types';

export class EnvironmentStore {
  environments = $state<Environment[]>([]);
  activeId = $state<string | null>(
    typeof window !== 'undefined' ? localStorage.getItem('pdswarm_active_env') : null
  );
  loading = $state<boolean>(false);
  error = $state<string | null>(null);

  get activeEnvironment(): Environment | null {
    if (!this.environments || this.environments.length === 0) return null;
    if (this.activeId) {
      const found = this.environments.find((e) => e.id === this.activeId);
      if (found) return found;
    }
    return this.environments.find((e) => e.is_default) ?? this.environments[0] ?? null;
  }

  async fetchEnvironments(): Promise<Environment[]> {
    this.loading = true;
    this.error = null;
    try {
      const envs = await environmentApi.list();
      this.environments = envs;

      // Validate or establish activeId
      const matched = this.activeId ? envs.find((e) => e.id === this.activeId) : null;
      if (!matched && envs.length > 0) {
        const defaultEnv = envs.find((e) => e.is_default) ?? envs[0];
        this.activeId = defaultEnv.id;
        if (typeof window !== 'undefined') {
          localStorage.setItem('pdswarm_active_env', defaultEnv.id);
        }
      } else if (matched && typeof window !== 'undefined') {
        localStorage.setItem('pdswarm_active_env', matched.id);
      }

      return envs;
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to fetch environments';
      this.error = msg;
      return [];
    } finally {
      this.loading = false;
    }
  }

  switchEnvironment(id: string): void {
    if (this.activeId === id) return;
    this.activeId = id;
    if (typeof window !== 'undefined') {
      localStorage.setItem('pdswarm_active_env', id);
      window.dispatchEvent(
        new CustomEvent('pdswarm:environment-changed', { detail: { id } })
      );
    }
  }

  async addEnvironment(req: CreateEnvironmentRequest): Promise<Environment> {
    this.loading = true;
    this.error = null;
    try {
      const created = await environmentApi.create(req);
      if (created.is_default) {
        this.environments = this.environments.map((e) => ({ ...e, is_default: false }));
      }
      this.environments = [...this.environments.filter((e) => e.id !== created.id), created];
      this.switchEnvironment(created.id);
      return created;
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to create environment';
      this.error = msg;
      throw err;
    } finally {
      this.loading = false;
    }
  }

  async updateEnvironment(id: string, req: UpdateEnvironmentRequest): Promise<Environment> {
    this.loading = true;
    this.error = null;
    try {
      const updated = await environmentApi.update(id, req);
      if (updated.is_default) {
        this.environments = this.environments.map((e) =>
          e.id === id ? updated : { ...e, is_default: false }
        );
      } else {
        this.environments = this.environments.map((e) => (e.id === id ? updated : e));
      }
      return updated;
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to update environment';
      this.error = msg;
      throw err;
    } finally {
      this.loading = false;
    }
  }

  async deleteEnvironment(id: string): Promise<void> {
    this.loading = true;
    this.error = null;
    try {
      await environmentApi.delete(id);
      this.environments = this.environments.filter((e) => e.id !== id);
      if (this.activeId === id) {
        const fallback = this.environments.find((e) => e.is_default) ?? this.environments[0];
        if (fallback) {
          this.switchEnvironment(fallback.id);
        } else {
          this.activeId = null;
          if (typeof window !== 'undefined') {
            localStorage.removeItem('pdswarm_active_env');
          }
        }
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to delete environment';
      this.error = msg;
      throw err;
    } finally {
      this.loading = false;
    }
  }

  async ping(id: string): Promise<PingEnvironmentResponse> {
    try {
      const res = await environmentApi.ping(id);
      this.environments = this.environments.map((e) => {
        if (e.id === id) {
          return {
            ...e,
            status: res.connected ? 'connected' : 'disconnected',
            error: res.error,
          };
        }
        return e;
      });
      return res;
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Ping failed';
      this.environments = this.environments.map((e) => {
        if (e.id === id) {
          return {
            ...e,
            status: 'disconnected',
            error: msg,
          };
        }
        return e;
      });
      return {
        status: 'disconnected',
        connected: false,
        error: msg,
      };
    }
  }
}

export const environmentStore = new EnvironmentStore();
export const environment = environmentStore;
