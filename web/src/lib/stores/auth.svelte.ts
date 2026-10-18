import { authApi } from '../api';
import type { User } from '../types';

export class AuthStore {
  user = $state<User | null>(null);
  token = $state<string | null>(
    typeof window !== 'undefined' ? localStorage.getItem('pdswarm_token') : null
  );
  needsSetup = $state<boolean>(false);
  authEnabled = $state<boolean>(true);
  isLoading = $state<boolean>(true);
  initialized = $state<boolean>(false);

  get isAuthenticated(): boolean {
    if (!this.authEnabled) return true;
    return this.user !== null;
  }

  async checkAuth(): Promise<void> {
    this.isLoading = true;
    try {
      const status = await authApi.getStatus();
      this.needsSetup = Boolean(status.setup_required ?? status.needs_setup ?? false);
      this.authEnabled = Boolean(status.auth_enabled ?? (status.no_auth !== undefined ? !status.no_auth : true));

      if (this.needsSetup) {
        this.user = null;
        this.isLoading = false;
        this.initialized = true;
        return;
      }

      if (!this.authEnabled) {
        this.user = { username: 'anonymous', role: 'admin' };
        this.isLoading = false;
        this.initialized = true;
        return;
      }

      // Check current session with /api/auth/me
      try {
        const me = await authApi.getMe();
        if (me && me.username) {
          this.user = me;
        } else {
          this.user = null;
          this.token = null;
          if (typeof window !== 'undefined') {
            localStorage.removeItem('pdswarm_token');
          }
        }
      } catch {
        this.user = null;
        this.token = null;
        if (typeof window !== 'undefined') {
          localStorage.removeItem('pdswarm_token');
        }
      }
    } catch (err) {
      console.error('Failed to check auth status:', err);
    } finally {
      this.isLoading = false;
      this.initialized = true;
    }
  }

  async login(username: string, password: string): Promise<void> {
    const res = await authApi.login(username, password);
    if (res.token) {
      this.token = res.token;
      if (typeof window !== 'undefined') {
        localStorage.setItem('pdswarm_token', res.token);
      }
    }
    if (res.user) {
      this.user = res.user;
    } else {
      this.user = { username, role: 'admin' };
    }
    this.needsSetup = false;
  }

  async setup(username: string, password: string): Promise<void> {
    const res = await authApi.setup(username, password);
    if (res.token) {
      this.token = res.token;
      if (typeof window !== 'undefined') {
        localStorage.setItem('pdswarm_token', res.token);
      }
    }
    if (res.user) {
      this.user = res.user;
    } else {
      this.user = { username, role: 'admin' };
    }
    this.needsSetup = false;
  }

  async logout(): Promise<void> {
    try {
      await authApi.logout();
    } catch {
      // Ignore network errors on logout
    } finally {
      this.user = null;
      this.token = null;
      if (typeof window !== 'undefined') {
        localStorage.removeItem('pdswarm_token');
      }
    }
  }
}

export const auth = new AuthStore();
