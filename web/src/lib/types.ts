// TypeScript type definitions for PDSwarm

export type ContainerState = 'running' | 'exited' | 'paused' | 'restarting' | 'dead' | 'created';

export interface PortMapping {
  IP?: string;
  PrivatePort: number;
  PublicPort?: number;
  Type: string;
}

export interface ContainerMount {
  Type: string;
  Name?: string;
  Source: string;
  Destination: string;
  Driver?: string;
  Mode: string;
  RW: boolean;
  Propagation: string;
}

export interface Container {
  Id: string;
  Names: string[];
  Image: string;
  ImageID: string;
  Command: string;
  Created: number;
  State: ContainerState | string;
  Status: string;
  Ports: PortMapping[];
  Labels: Record<string, string>;
  SizeRw?: number;
  SizeRootFs?: number;
  Mounts?: ContainerMount[];
}

export interface Image {
  Id: string;
  ParentId?: string;
  RepoTags: string[];
  RepoDigests?: string[];
  Created: number;
  Size: number;
  SharedSize?: number;
  VirtualSize?: number;
  Labels?: Record<string, string>;
  Containers?: number;
}

export interface Volume {
  Name: string;
  Driver: string;
  Mountpoint: string;
  CreatedAt?: string;
  Status?: Record<string, unknown>;
  Labels?: Record<string, string>;
  Scope?: string;
  Options?: Record<string, string>;
  UsageData?: {
    Size: number;
    RefCount: number;
  };
}

export interface NetworkIPAMConfig {
  Subnet?: string;
  IPRange?: string;
  Gateway?: string;
}

export interface NetworkIPAM {
  Driver: string;
  Config?: NetworkIPAMConfig[];
  Options?: Record<string, string>;
}

export interface Network {
  Id: string;
  Name: string;
  Created?: string;
  Scope: string;
  Driver: string;
  EnableIPv6: boolean;
  IPAM?: NetworkIPAM;
  Internal: boolean;
  Attachable: boolean;
  Ingress: boolean;
  Containers?: Record<string, {
    Name: string;
    EndpointID: string;
    MacAddress: string;
    IPv4Address: string;
    IPv6Address: string;
  }>;
  Options?: Record<string, string>;
  Labels?: Record<string, string>;
}

export interface Stack {
  name: string;
  compose_yaml: string;
  status: string;
  is_external?: boolean;
  working_dir?: string;
  config_file?: string;
  created_at: string;
  updated_at: string;
}

export interface SystemInfo {
  ID?: string;
  Containers?: number;
  ContainersRunning?: number;
  ContainersPaused?: number;
  ContainersStopped?: number;
  Images?: number;
  Driver?: string;
  MemoryLimit?: boolean;
  SwapLimit?: boolean;
  KernelVersion?: string;
  OperatingSystem?: string;
  OSVersion?: string;
  OSType?: string;
  Architecture?: string;
  NCPU?: number;
  MemTotal?: number;
  DockerRootDir?: string;
  ServerVersion?: string;
  CgroupVersion?: string;
  CgroupDriver?: string;
  // DTO fields from /api/system/info
  os?: string;
  kernel_version?: string;
  containers?: number;
  containers_running?: number;
  containers_paused?: number;
  containers_stopped?: number;
  images?: number;
  cpus?: number;
  memory_bytes?: number;
  cpu_percent?: number;
  memory_used_bytes?: number;
  memory_percent?: number;
  server_version?: string;
  engine_type?: 'docker' | 'podman' | string;
  engine_name?: string;
  engine_version?: string;
  driver?: string;
  memory_limit?: boolean;
  swap_limit?: boolean;
  docker_root_dir?: string;
  cgroup_version?: string;
  cgroup_driver?: string;
  version?: string;
  app_version?: string;
  pdswarm_version?: string;
}

export interface User {
  username: string;
  role: string;
  created_at?: string;
}

export interface AuthStatus {
  setup_required?: boolean;
  needs_setup?: boolean;
  auth_enabled?: boolean;
  no_auth?: boolean;
  version?: string;
}

export interface CreateContainerRequest {
  name?: string;
  image: string;
  env?: string[];
  ports?: {
    host_port: string;
    container_port: string;
    protocol?: string;
  }[];
  volumes?: {
    host_path: string;
    container_path: string;
    read_only?: boolean;
  }[];
  restart_policy?: string;
  memory_mb?: number;
  cpu_cores?: number;
  labels?: Record<string, string>;
}

export interface CreateVolumeRequest {
  Name?: string;
  name?: string;
  Driver?: string;
  driver?: string;
  Labels?: Record<string, string>;
  labels?: Record<string, string>;
  DriverOpts?: Record<string, string>;
}

export interface CreateNetworkRequest {
  Name?: string;
  name?: string;
  Driver?: string;
  driver?: string;
  Internal?: boolean;
  Attachable?: boolean;
  Labels?: Record<string, string>;
  labels?: Record<string, string>;
  Subnet?: string;
  Gateway?: string;
}

export interface DockerPullProgress {
  id?: string;
  status: string;
  progressDetail?: {
    current?: number;
    total?: number;
  };
  progress?: string;
  error?: string;
  errorDetail?: {
    message: string;
  };
}

export interface ContainerStateDetail {
  Status: string;
  Running: boolean;
  Paused: boolean;
  Restarting: boolean;
  OOMKilled: boolean;
  Dead: boolean;
  Pid: number;
  ExitCode: number;
  Error: string;
  StartedAt: string;
  FinishedAt: string;
  Health?: {
    Status: string;
    FailingStreak: number;
    Log?: Array<{
      Start: string;
      End: string;
      ExitCode: number;
      Output: string;
    }>;
  };
}

export interface ContainerConfigDetail {
  Hostname?: string;
  Domainname?: string;
  User?: string;
  AttachStdin?: boolean;
  AttachStdout?: boolean;
  AttachStderr?: boolean;
  Tty?: boolean;
  OpenStdin?: boolean;
  StdinOnce?: boolean;
  Env?: string[];
  Cmd?: string[];
  Image?: string;
  WorkingDir?: string;
  Entrypoint?: string[];
  Labels?: Record<string, string>;
  StopSignal?: string;
}

export interface ContainerHostConfigDetail {
  Binds?: string[];
  NetworkMode?: string;
  PortBindings?: Record<string, Array<{ HostIp: string; HostPort: string }> | null>;
  RestartPolicy?: {
    Name: string;
    MaximumRetryCount?: number;
  };
  AutoRemove?: boolean;
  Memory?: number;
  NanoCPUs?: number;
  CpuShares?: number;
  Privileged?: boolean;
}

export interface ContainerNetworkSettingsDetail {
  IPAddress?: string;
  Gateway?: string;
  MacAddress?: string;
  Ports?: Record<string, Array<{ HostIp: string; HostPort: string }> | null>;
  Networks?: Record<string, {
    IPAddress: string;
    Gateway: string;
    MacAddress: string;
    NetworkID: string;
    Aliases?: string[];
  }>;
}

export interface ContainerInspectInfo {
  Id: string;
  Created: string;
  Path?: string;
  Args?: string[];
  State: ContainerStateDetail;
  Image: string;
  ResolvConfPath?: string;
  HostnamePath?: string;
  HostsPath?: string;
  LogPath?: string;
  Name: string;
  RestartCount?: number;
  Driver?: string;
  Platform?: string;
  MountLabel?: string;
  ProcessLabel?: string;
  AppArmorProfile?: string;
  HostConfig?: ContainerHostConfigDetail;
  Config?: ContainerConfigDetail;
  NetworkSettings?: ContainerNetworkSettingsDetail;
  Mounts?: ContainerMount[];
  SizeRw?: number;
  SizeRootFs?: number;
  ImageEnv?: string[];
  UserEnv?: string[];
  [key: string]: unknown;
}

export interface ContainerStatsPayload {
  cpu_percent: number;
  memory_usage: number;
  memory_limit: number;
  memory_mb: number;
  memory_limit_mb: number;
  memory_percent: number;
  network_rx: number;
  network_tx: number;
  block_read: number;
  block_write: number;
  pids: number;
}

export type EnvironmentType = 'local' | 'socket' | 'tcp' | 'tls' | string;
export type EnvironmentStatus = 'connected' | 'disconnected' | 'connecting' | string;

export interface Environment {
  id: string;
  name: string;
  type: EnvironmentType;
  url: string;
  tls_cert?: string;
  tls_key?: string;
  tls_ca?: string;
  is_default: boolean;
  created_at: string;
  updated_at: string;
  status: EnvironmentStatus;
  containers_count: number;
  error?: string;
}

export interface CreateEnvironmentRequest {
  name: string;
  type: string;
  url: string;
  tls_cert?: string;
  tls_key?: string;
  tls_ca?: string;
  is_default?: boolean;
}

export interface UpdateEnvironmentRequest {
  name?: string;
  type?: string;
  url?: string;
  tls_cert?: string;
  tls_key?: string;
  tls_ca?: string;
  is_default?: boolean;
}

export interface PingEnvironmentResponse {
  status: string;
  connected: boolean;
  latency_ms?: number;
  message?: string;
  error?: string;
}

export interface ContainerFileInfo {
  name: string;
  path: string;
  size: number;
  mode: string;
  mod_time: string;
  is_dir: boolean;
  link_target?: string;
}

export interface ContainerFileContent {
  name: string;
  path: string;
  size: number;
  content: string;
}

export interface Registry {
  id: string;
  name: string;
  url: string;
  username: string;
  password?: string;
  has_password?: boolean;
  created_at: string;
  updated_at: string;
}

export interface CreateRegistryRequest {
  name: string;
  url: string;
  username: string;
  password?: string;
}

export interface UpdateRegistryRequest {
  name?: string;
  url?: string;
  username?: string;
  password?: string;
}

export interface PingRegistryResponse {
  status: string;
  message?: string;
  error?: string;
}

export interface TestRegistryRequest {
  id?: string;
  url: string;
  username: string;
  password?: string;
}

export interface TemplateEnvVar {
  name: string;
  label: string;
  default: string;
  description: string;
  required: boolean;
}

export interface Template {
  id: string;
  title: string;
  description: string;
  categories: string[];
  logo: string;
  platform: string;
  type: string;
  compose_yaml: string;
  env?: TemplateEnvVar[];
  volumes?: string[];
  ports?: string[];
  source: string;
  repository?: {
    url: string;
    stackfile: string;
  };
}

export interface DeployTemplateRequest {
  template_id: string;
  stack_name: string;
  env: Record<string, string>;
  compose_yaml?: string;
}

export interface PortAllocation {
  host_ip: string;
  host_port: number;
  container_port: number;
  protocol: string;
  container_id: string;
  container_name: string;
  state: string;
}

export interface PortConflict {
  host_port: number;
  container_port: number;
  protocol: string;
  in_use: boolean;
  conflict_with?: string;
  suggested_port?: number;
}

export interface CheckPortsResponse {
  has_conflict: boolean;
  conflicts: PortConflict[];
}

export interface AllocatedPortsResponse {
  allocated: PortAllocation[];
  used_ports: number[];
}

export interface HostPathSuggestion {
  path: string;
  name: string;
  type: 'dir' | 'volume';
  is_dir: boolean;
}

export interface HostPathResponse {
  current_dir: string;
  suggestions: HostPathSuggestion[];
}

export interface TemplateListResponse {
  templates: Template[];
  total: number;
  categories: string[];
  sources: string[];
  category_counts: Record<string, number>;
}
export interface DeployTemplateResponse {
  status: string;
  stack_name: string;
  stack?: Stack;
}

export interface DeployProgressEvent {
  step: 'init' | 'network' | 'pull' | 'create' | 'start' | 'done' | 'error';
  service?: string;
  image?: string;
  status: string;
  progress?: string;
  detail?: string;
  error?: string;
  timestamp?: number;
}

export interface APIToken {
  id: string;
  name: string;
  prefix: string;
  created_by: string;
  expires_at?: string;
  created_at: string;
  last_used_at?: string;
}

export interface ChangePasswordRequest {
  old_password: string;
  new_password: string;
}

export interface CreateUserRequest {
  username: string;
  password: string;
  role: 'admin' | 'operator' | 'viewer';
}

export interface CreateTokenRequest {
  name: string;
  expires_days: number;
}

export interface CreateTokenResponse {
  token: string;
  token_info: APIToken;
}

export interface TemplateRepo {
  id: string;
  name: string;
  url: string;
  type: 'auto' | 'portainer' | 'casaos_zip' | 'casaos_compose';
  enabled: boolean;
  last_sync_at?: string | null;
  sync_error?: string | null;
  item_count: number;
  created_at: string;
  updated_at?: string;
}

export interface TemplateSettings {
  disable_default_templates: boolean;
}

export interface CreateTemplateRepoRequest {
  name: string;
  url: string;
  type?: string;
  enabled?: boolean;
}

export type TaskStatus = 'pending' | 'running' | 'completed' | 'failed' | 'cancelled';
export type TaskType = 'image_pull' | 'image_build' | 'container_update' | 'stack_deploy' | 'stack_down' | 'template_deploy' | 'prune' | 'general';

export interface BackgroundTask {
  id: string;
  type: TaskType;
  title: string;
  target?: string;
  status: TaskStatus;
  progress: number;
  current_stage?: string;
  logs: string[];
  error?: string;
  created_at: string;
  started_at?: string;
  finished_at?: string;
}

export interface ImageUpdateStatus {
  image_ref: string;
  status: 'up_to_date' | 'update_available' | 'not_found' | 'unknown';
  up_to_date: boolean;
  local_digest?: string;
  remote_digest?: string;
  message: string;
  error?: string;
}

export interface BuildImageRequest {
  tag: string;
  dockerfile: string;
  context_dir?: string;
}

export interface LoggingConfig {
  log_driver: string;
  max_size: string;
  max_file: string;
  compress: boolean;
  config_file: string;
  config_writable: boolean;
  daemon_configured: boolean;
  generated_snippet: string;
  apply_command: string;
  engine_type: string;
}

export interface UpdateLoggingConfigPayload {
  log_driver: string;
  max_size: string;
  max_file: string;
  compress: boolean;
  write_daemon_file?: boolean;
}
