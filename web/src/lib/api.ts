// HTTP client for PDSwarm REST API

import type {
  Container,
  Image,
  Volume,
  Network,
  Stack,
  SystemInfo,
  User,
  AuthStatus,
  CreateContainerRequest,
  CreateVolumeRequest,
  CreateNetworkRequest,
  ContainerInspectInfo,
  DockerPullProgress,
  Environment,
  CreateEnvironmentRequest,
  UpdateEnvironmentRequest,
  PingEnvironmentResponse,
  ContainerFileInfo,
  ContainerFileContent,
  Registry,
  CreateRegistryRequest,
  UpdateRegistryRequest,
  PingRegistryResponse,
  TestRegistryRequest,
  Template,
  DeployTemplateRequest,
  TemplateListResponse,
  DeployTemplateResponse,
  APIToken,
  ChangePasswordRequest,
  CreateUserRequest,
  CreateTokenRequest,
  CreateTokenResponse,
  TemplateRepo,
  TemplateSettings,
  CreateTemplateRepoRequest,
  PortAllocation,
  PortConflict,
  CheckPortsResponse,
  AllocatedPortsResponse,
  HostPathResponse,
  DeployProgressEvent,
  BackgroundTask,
  ImageUpdateStatus,
  CheckImageUpdatesBatchResponse,
  LoggingConfig,
  UpdateLoggingConfigPayload,
  UpdateStatus,
  ApplyUpdateResponse,
  AuditLogInfo,
} from './types';

export class ApiError extends Error {
  public status: number;
  public statusText: string;
  public data: unknown;

  constructor(status: number, statusText: string, message: string, data?: unknown) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.statusText = statusText;
    this.data = data;
  }
}

const API_BASE = '/api';

export async function apiFetch<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
  const url = endpoint.startsWith('http') || endpoint.startsWith('/api')
    ? endpoint
    : `${API_BASE}${endpoint.startsWith('/') ? '' : '/'}${endpoint}`;

  const headers = new Headers(options.headers || {});
  
  if (options.body && !(options.body instanceof FormData) && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json');
  }

  if (typeof window !== 'undefined') {
    const token = localStorage.getItem('pdswarm_token');
    if (token && !headers.has('Authorization')) {
      headers.set('Authorization', `Bearer ${token}`);
    }
    const envId = localStorage.getItem('pdswarm_active_env');
    if (envId && !headers.has('X-Environment-ID')) {
      headers.set('X-Environment-ID', envId);
    }
  }

  const config: RequestInit = {
    ...options,
    headers,
    credentials: 'include',
  };

  const response = await fetch(url, config);

  if (!response.ok) {
    let errorMessage = `HTTP error ${response.status}: ${response.statusText}`;
    let errorData: unknown;
    try {
      const json = await response.json();
      errorData = json;
      if (json && typeof json.error === 'string') {
        errorMessage = json.error;
      } else if (json && typeof json.message === 'string') {
        errorMessage = json.message;
      }
    } catch {
      try {
        const text = await response.text();
        if (text) errorMessage = text;
      } catch {
        // use default HTTP error
      }
    }
    throw new ApiError(response.status, response.statusText, errorMessage, errorData);
  }

  // 204 No Content or empty responses
  if (response.status === 204) {
    return {} as T;
  }

  const contentType = response.headers.get('content-type');
  if (contentType && contentType.includes('application/json')) {
    return await response.json() as T;
  }

  const text = await response.text();
  try {
    return JSON.parse(text) as T;
  } catch {
    return text as unknown as T;
  }
}

export const api = {
  get: <T>(url: string, options?: RequestInit) =>
    apiFetch<T>(url, { ...options, method: 'GET' }),
  post: <T>(url: string, body?: unknown, options?: RequestInit) =>
    apiFetch<T>(url, {
      ...options,
      method: 'POST',
      body: body !== undefined ? (body instanceof FormData ? body : JSON.stringify(body)) : undefined,
    }),
  put: <T>(url: string, body?: unknown, options?: RequestInit) =>
    apiFetch<T>(url, {
      ...options,
      method: 'PUT',
      body: body !== undefined ? (body instanceof FormData ? body : JSON.stringify(body)) : undefined,
    }),
  delete: <T>(url: string, options?: RequestInit) =>
    apiFetch<T>(url, { ...options, method: 'DELETE' }),
};

// Domain-specific API helpers

export const authApi = {
  getStatus: () => api.get<AuthStatus>('/auth/status'),
  setup: (usernameOrPassword: string, maybePassword?: string) => {
    const username = maybePassword !== undefined ? usernameOrPassword : 'admin';
    const password = maybePassword !== undefined ? maybePassword : usernameOrPassword;
    return api.post<{ token: string; user: User }>('/auth/setup', { username, password });
  },
  login: (usernameOrPassword: string, maybePassword?: string) => {
    const username = maybePassword !== undefined ? usernameOrPassword : 'admin';
    const password = maybePassword !== undefined ? maybePassword : usernameOrPassword;
    return api.post<{ token: string; user: User }>('/auth/login', { username, password });
  },
  logout: () => api.post<{ message: string }>('/auth/logout'),
  getMe: () => api.get<User>('/auth/me'),
  changePassword: (req: ChangePasswordRequest) =>
    api.put<{ message: string }>('/auth/password', req),
};

export const userApi = {
  list: () => api.get<User[]>('/users'),
  create: (req: CreateUserRequest) => api.post<User>('/users', req),
  delete: (username: string) =>
    api.delete<{ message: string }>(`/users/${encodeURIComponent(username)}`),
};

export const tokenApi = {
  list: () => api.get<APIToken[]>('/tokens'),
  create: (req: CreateTokenRequest) => api.post<CreateTokenResponse>('/tokens', req),
  revoke: (id: string) =>
    api.delete<{ message: string }>(`/tokens/${encodeURIComponent(id)}`),
};

export const systemApi = {
  ping: () => api.get<{ status: string; version?: string }>('/system/ping'),
  getVersion: () => api.get<{ version: string; name?: string }>('/system/version'),
  getInfo: () => api.get<SystemInfo>('/system/info'),
  getPorts: () => api.get<AllocatedPortsResponse>('/system/ports'),
  checkPorts: (ports: string[]) => api.post<CheckPortsResponse>('/system/ports/check', { ports }),
  getHostPaths: (query = '') =>
    api.get<HostPathResponse>(`/system/host-paths?query=${encodeURIComponent(query)}`),
  getLoggingConfig: () => api.get<LoggingConfig>('/system/logging'),
  updateLoggingConfig: (payload: UpdateLoggingConfigPayload) =>
    api.put<LoggingConfig>('/system/logging', payload),
  getUpdateStatus: (force = false) =>
    api.get<UpdateStatus>(`/system/update${force ? '?check=true' : ''}`),
  checkUpdate: () => api.post<UpdateStatus>('/system/update/check', {}),
  setAutoUpdate: (enabled: boolean) =>
    api.put<UpdateStatus>('/system/update/auto', { enabled }),
  applyUpdate: () => api.post<ApplyUpdateResponse>('/system/update/apply', {}),
  getAuditLogs: () => api.get<AuditLogInfo>('/system/audit-logs'),
};

export const containerApi = {
  list: (all = true) => api.get<Container[]>(`/containers?all=${all}`),
  inspect: (id: string) => api.get<ContainerInspectInfo>(`/containers/${id}`),
  create: (req: CreateContainerRequest) => api.post<{ id: string }>('/containers', req),
  recreate: (id: string, req: CreateContainerRequest) =>
    api.post<{ task_id: string; status: string; message: string }>(`/containers/${id}/recreate`, req),
  start: (id: string) => api.post<{ status: string }>(`/containers/${id}/start`),
  stop: (id: string, timeout = 10) => api.post<{ status: string }>(`/containers/${id}/stop?timeout=${timeout}`),
  restart: (id: string, timeout = 10) => api.post<{ status: string }>(`/containers/${id}/restart?timeout=${timeout}`),
  pause: (id: string) => api.post<{ status: string }>(`/containers/${id}/pause`),
  unpause: (id: string) => api.post<{ status: string }>(`/containers/${id}/unpause`),
  kill: (id: string, signal = 'SIGKILL') => api.post<{ status: string }>(`/containers/${id}/kill?signal=${signal}`),
  updateImage: (id: string) =>
    api.post<{ task_id: string; status: string; message: string }>(`/containers/${id}/update-image`),
  remove: (id: string, force = false, removeVolumes = false) =>
    api.delete<{ status: string }>(`/containers/${id}?force=${force}&volumes=${removeVolumes}`),
  prune: () => api.post<{ ContainersDeleted: string[]; SpaceReclaimed: number }>('/containers/prune'),
};

export const imageApi = {
  list: () => api.get<Image[]>('/images'),
  pull: (image: string, registryId?: string, async = true) =>
    api.post<{ status: string; task_id?: string; title?: string; message?: string }>('/images/pull', {
      image,
      registry_id: registryId,
      async,
    }),
  pullStream: async (
    image: string,
    onProgress: (event: DockerPullProgress) => void,
    signal?: AbortSignal,
    registryId?: string
  ): Promise<void> => {
    const token = typeof window !== 'undefined' ? localStorage.getItem('pdswarm_token') : null;
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
    };
    if (token) {
      headers['Authorization'] = `Bearer ${token}`;
    }
    const envId = typeof window !== 'undefined' ? localStorage.getItem('pdswarm_active_env') : null;
    if (envId && !headers['X-Environment-ID']) {
      headers['X-Environment-ID'] = envId;
    }

    const query = new URLSearchParams({ stream: 'true' });
    if (registryId) {
      query.set('registry_id', registryId);
    }

    const response = await fetch(`/api/images/pull?${query.toString()}`, {
      method: 'POST',
      headers,
      body: JSON.stringify({ image, registry_id: registryId }),
      signal,
    });

    if (!response.ok) {
      let msg = `HTTP error ${response.status}: ${response.statusText}`;
      try {
        const json = await response.json();
        if (json && typeof json.error === 'string') {
          msg = json.error;
        } else if (json && typeof json.message === 'string') {
          msg = json.message;
        }
      } catch {
        // ignore
      }
      throw new Error(msg);
    }

    if (!response.body) {
      return;
    }

    const reader = response.body.getReader();
    const decoder = new TextDecoder('utf-8');
    let buffer = '';

    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });
      const lines = buffer.split('\n');
      buffer = lines.pop() || '';

      for (const line of lines) {
        const trimmed = line.trim();
        if (!trimmed) continue;
        try {
          const parsed = JSON.parse(trimmed) as DockerPullProgress;
          if (parsed.error) {
            throw new Error(parsed.error);
          }
          if (parsed.errorDetail?.message) {
            throw new Error(parsed.errorDetail.message);
          }
          onProgress(parsed);
        } catch (parseErr) {
          if (parseErr instanceof Error && !parseErr.message.includes('JSON')) {
            throw parseErr;
          }
        }
      }
    }

    if (buffer.trim()) {
      try {
        const parsed = JSON.parse(buffer.trim()) as DockerPullProgress;
        if (parsed.error) throw new Error(parsed.error);
        if (parsed.errorDetail?.message) throw new Error(parsed.errorDetail.message);
        onProgress(parsed);
      } catch (parseErr) {
        if (parseErr instanceof Error && !parseErr.message.includes('JSON')) {
          throw parseErr;
        }
      }
    }
  },
  build: async (
    tag: string,
    dockerfile: string,
    onProgress: (chunk: string) => void,
    signal?: AbortSignal,
    contextDir?: string
  ): Promise<void> => {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
    };
    const token = typeof window !== 'undefined' ? localStorage.getItem('pdswarm_token') : null;
    if (token) {
      headers['Authorization'] = `Bearer ${token}`;
    }
    const envId = typeof window !== 'undefined' ? localStorage.getItem('pdswarm_active_env') : null;
    if (envId && !headers['X-Environment-ID']) {
      headers['X-Environment-ID'] = envId;
    }

    const payload: { tag: string; dockerfile: string; context_dir?: string } = { tag, dockerfile };
    if (contextDir && contextDir.trim()) {
      payload.context_dir = contextDir.trim();
    }

    const response = await fetch('/api/images/build', {
      method: 'POST',
      headers,
      body: JSON.stringify(payload),
      signal,
    });

    if (!response.ok) {
      let msg = `HTTP error ${response.status}: ${response.statusText}`;
      try {
        const json = await response.json();
        if (json && typeof json.error === 'string') {
          msg = json.error;
        } else if (json && typeof json.message === 'string') {
          msg = json.message;
        }
      } catch {
        // ignore
      }
      throw new Error(msg);
    }

    if (!response.body) {
      return;
    }

    const reader = response.body.getReader();
    const decoder = new TextDecoder('utf-8');
    let buffer = '';

    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });
      const lines = buffer.split('\n');
      buffer = lines.pop() || '';

      for (const line of lines) {
        const trimmed = line.trim();
        if (!trimmed) continue;
        try {
          const parsed = JSON.parse(trimmed) as { stream?: string; error?: string; errorDetail?: { message?: string } };
          if (parsed.error) {
            throw new Error(parsed.error);
          }
          if (parsed.errorDetail?.message) {
            throw new Error(parsed.errorDetail.message);
          }
          if (typeof parsed.stream === 'string') {
            onProgress(parsed.stream);
          } else {
            onProgress(trimmed + '\n');
          }
        } catch (parseErr) {
          if (parseErr instanceof Error && !parseErr.message.includes('JSON')) {
            throw parseErr;
          }
          onProgress(line + '\n');
        }
      }
    }

    if (buffer.trim()) {
      try {
        const parsed = JSON.parse(buffer.trim()) as { stream?: string; error?: string; errorDetail?: { message?: string } };
        if (parsed.error) throw new Error(parsed.error);
        if (parsed.errorDetail?.message) throw new Error(parsed.errorDetail.message);
        if (typeof parsed.stream === 'string') {
          onProgress(parsed.stream);
        } else {
          onProgress(buffer);
        }
      } catch (parseErr) {
        if (parseErr instanceof Error && !parseErr.message.includes('JSON')) {
          throw parseErr;
        }
        onProgress(buffer);
      }
    }
  },
  remove: (id: string, force = false) => api.delete<{ status: string }>(`/images/${id}?force=${force}`),
  prune: () => api.post<{ ImagesDeleted: unknown[]; SpaceReclaimed: number }>('/images/prune'),
  checkUpdate: (image: string, registryId?: string) =>
    api.post<ImageUpdateStatus>('/images/check-update', { image, registry_id: registryId }),
  checkBatch: (images: string[], registryId?: string) =>
    api.post<CheckImageUpdatesBatchResponse>('/images/check-updates', { images, registry_id: registryId }),
};

export const volumeApi = {
  list: () => api.get<{ Volumes: Volume[]; Warnings?: string[] }>('/volumes'),
  create: (req: CreateVolumeRequest) =>
    api.post<Volume>('/volumes', {
      name: req.name || req.Name,
      driver: req.driver || req.Driver || 'local',
      labels: req.labels || req.Labels,
    }),
  remove: (name: string, force = false) => api.delete<{ status: string }>(`/volumes/${name}?force=${force}`),
  prune: () => api.post<{ VolumesDeleted: string[]; SpaceReclaimed: number }>('/volumes/prune'),
};

export const networkApi = {
  list: () => api.get<Network[]>('/networks'),
  create: (req: CreateNetworkRequest) =>
    api.post<{ id: string } & Network>('/networks', {
      name: req.name || req.Name,
      driver: req.driver || req.Driver || 'bridge',
    }),
  remove: (id: string) => api.delete<{ status: string }>(`/networks/${id}`),
  prune: () => api.post<{ NetworksDeleted: string[] }>('/networks/prune'),
};

export const stackApi = {
  list: () => api.get<Stack[]>('/stacks'),
  get: (name: string) => api.get<Stack & { containers?: Container[] }>(`/stacks/${name}`),
  create: (name: string, compose_yaml: string, deploy = true, env = '') =>
    api.post<Stack>('/stacks', { name, compose_yaml, deploy, env }),
  update: (name: string, compose_yaml: string, deploy = true, env = '') =>
    api.put<Stack>(`/stacks/${name}`, { compose_yaml, deploy, env }),
  adopt: (name: string, data?: { compose_yaml?: string; working_dir?: string }) =>
    api.post<Stack>(`/stacks/${name}/adopt`, data || {}),
  stop: (name: string) => api.post<{ status: string }>(`/stacks/${name}/stop`),
  down: (name: string, removeVolumes = false) =>
    api.post<{ status: string }>(`/stacks/${name}/down?volumes=${removeVolumes}`),
  delete: (name: string) => api.delete<{ status?: string; message?: string }>(`/stacks/${name}`),
};

export const environmentApi = {
  list: () => api.get<Environment[]>('/environments'),
  get: (id: string) => api.get<Environment>(`/environments/${id}`),
  create: (req: CreateEnvironmentRequest) => api.post<Environment>('/environments', req),
  update: (id: string, req: UpdateEnvironmentRequest) => api.put<Environment>(`/environments/${id}`, req),
  delete: (id: string) => api.delete<{ message?: string }>(`/environments/${id}`),
  ping: (id: string) => api.post<PingEnvironmentResponse>(`/environments/${id}/ping`),
  test: (req: CreateEnvironmentRequest) =>
    api.post<{ success: boolean; latency_ms?: number; containers?: number; message?: string; error?: string }>(
      '/environments/test',
      req
    ),
};

export const fileApi = {
  list: (containerId: string, path: string = '/'): Promise<ContainerFileInfo[]> =>
    api.get<ContainerFileInfo[]>(`/containers/${containerId}/files/list?path=${encodeURIComponent(path)}`),
  read: (containerId: string, path: string, maxBytes?: number): Promise<ContainerFileContent> =>
    api.get<ContainerFileContent>(
      `/containers/${containerId}/files/read?path=${encodeURIComponent(path)}${maxBytes ? `&max=${maxBytes}` : ''}`
    ),
  getDownloadUrl: (containerId: string, path: string): string => {
    const params = new URLSearchParams();
    params.set('path', path);
    if (typeof window !== 'undefined') {
      const token = localStorage.getItem('pdswarm_token');
      if (token) params.set('token', token);
      const envId = localStorage.getItem('pdswarm_active_env');
      if (envId) params.set('env', envId);
    }
    return `/api/containers/${containerId}/files/download?${params.toString()}`;
  },
  upload: (containerId: string, destDir: string, file: File): Promise<void> => {
    const formData = new FormData();
    formData.append('file', file);
    return api.post<void>(
      `/containers/${containerId}/files/upload?path=${encodeURIComponent(destDir)}`,
      formData
    );
  },
  delete: (containerId: string, path: string): Promise<void> =>
    api.delete<void>(`/containers/${containerId}/files?path=${encodeURIComponent(path)}`),
};

export const registryApi = {
  list: () => api.get<Registry[]>('/registries'),
  get: (id: string) => api.get<Registry>(`/registries/${id}`),
  create: (data: CreateRegistryRequest) => api.post<Registry>('/registries', data),
  update: (id: string, data: UpdateRegistryRequest) => api.put<Registry>(`/registries/${id}`, data),
  delete: (id: string) => api.delete<{ message: string; id: string }>(`/registries/${id}`),
  test: (data: Partial<TestRegistryRequest>) => api.post<PingRegistryResponse>('/registries/test', data),
  ping: (id: string) => api.post<PingRegistryResponse>(`/registries/${id}/ping`),
};

export const templateApi = {
  list: (params?: { category?: string; q?: string; source?: string }) => {
    const searchParams = new URLSearchParams();
    if (params?.category && params.category !== 'all' && params.category !== 'All') {
      searchParams.set('category', params.category);
    }
    if (params?.q) searchParams.set('q', params.q);
    if (params?.source) searchParams.set('source', params.source);
    const qs = searchParams.toString();
    return api.get<TemplateListResponse>(`/templates${qs ? `?${qs}` : ''}`);
  },
  getCategories: () =>
    api.get<{ categories: string[]; counts: Record<string, number> }>('/templates/categories'),
  get: (id: string) => api.get<Template>(`/templates/${id}`),
  deploy: (req: DeployTemplateRequest) =>
    api.post<DeployTemplateResponse>('/templates/deploy', req),
  deployStream: async (
    req: DeployTemplateRequest,
    onProgress: (event: DeployProgressEvent) => void
  ): Promise<DeployTemplateResponse> => {
    const url = `${API_BASE}/templates/deploy?stream=true`;
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
    };
    if (typeof window !== 'undefined') {
      const token = localStorage.getItem('pdswarm_token');
      if (token) headers['Authorization'] = `Bearer ${token}`;
      const envId = localStorage.getItem('pdswarm_active_env');
      if (envId) headers['X-Environment-ID'] = envId;
    }

    const resp = await fetch(url, {
      method: 'POST',
      headers,
      body: JSON.stringify(req),
    });

    if (!resp.ok) {
      let msg = `HTTP error ${resp.status}: ${resp.statusText}`;
      try {
        const j = await resp.json();
        msg = j.error || j.message || msg;
      } catch {}
      throw new Error(msg);
    }

    if (!resp.body) {
      throw new Error('No response stream received from server');
    }

    const reader = resp.body.getReader();
    const decoder = new TextDecoder();
    let buffer = '';

    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });
      const lines = buffer.split('\n');
      buffer = lines.pop() || '';
      for (const line of lines) {
        const trimmed = line.trim();
        if (!trimmed) continue;
        try {
          const evt = JSON.parse(trimmed) as DeployProgressEvent;
          onProgress(evt);
          if (evt.step === 'error') {
            throw new Error(evt.detail || evt.status || 'Deployment failed');
          }
        } catch (e) {
          if (e instanceof Error && !e.message.includes('Unexpected end of JSON')) {
            throw e;
          }
        }
      }
    }

    return {
      status: 'deployed',
      stack_name: req.stack_name,
    };
  },
};

export const templateRepoApi = {
  list: () => api.get<TemplateRepo[]>('/templates/repos'),
  create: (req: CreateTemplateRepoRequest) => api.post<TemplateRepo>('/templates/repos', req),
  update: (id: string, req: Partial<CreateTemplateRepoRequest>) =>
    api.put<TemplateRepo>(`/templates/repos/${id}`, req),
  delete: (id: string) => api.delete<{ status: string; id: string }>(`/templates/repos/${id}`),
  sync: (id?: string) =>
    id
      ? api.post<TemplateRepo>(`/templates/repos/${id}/sync`, {})
      : api.post<{ status: string; total_repos: number; total_apps: number; repositories: TemplateRepo[] }>(
          '/templates/repos/sync',
          {}
        ),
  getSettings: () => api.get<TemplateSettings>('/templates/settings'),
  updateSettings: (settings: TemplateSettings) =>
    api.put<TemplateSettings>('/templates/settings', settings),
};

export const taskApi = {
  list: () => api.get<BackgroundTask[]>('/tasks'),
  get: (id: string) => api.get<BackgroundTask>(`/tasks/${id}`),
  cancel: (id: string) => api.post<{ message: string }>(`/tasks/${id}/cancel`),
  delete: (id: string) => api.delete<{ message: string }>(`/tasks/${id}`),
};

export function getWebSocketUrl(
  path: string,
  params: Record<string, string | number | boolean | undefined> = {}
): string {
  if (typeof window === 'undefined') return '';
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const host = window.location.host;
  const cleanPath = path.startsWith('/') ? path : `/${path}`;
  const url = new URL(`${protocol}//${host}${cleanPath}`);

  const token = localStorage.getItem('pdswarm_token');
  if (token) {
    url.searchParams.set('token', token);
  }
  const envId = localStorage.getItem('pdswarm_active_env');
  if (envId && !url.searchParams.has('env')) {
    url.searchParams.set('env', envId);
  }
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== null && v !== '') {
      url.searchParams.set(k, String(v));
    }
  }
  return url.toString();
}
