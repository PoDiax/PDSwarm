<script lang="ts">
  import {
    X,
    Plus,
    Trash,
    Layers,
    Cpu,
    HardDrive,
    Network,
    Terminal,
    LoaderCircle,
    CircleCheck,
    CircleAlert,
    Play,
    Copy,
    RotateCw,
    SlidersHorizontal,
    AlertTriangle,
    Tag,
  } from '@lucide/svelte';
  import { containerApi, systemApi } from '../api';
  import { tasksStore } from '../stores/tasks.svelte';
  import HostPathInput from './HostPathInput.svelte';
  import type { CreateContainerRequest, ContainerInspectInfo, Container, PortConflict } from '../types';

  interface Props {
    isOpen: boolean;
    onClose: () => void;
    onCreated?: (containerId: string) => void;
    duplicateSource?: ContainerInspectInfo | Container | null;
    mode?: 'create' | 'duplicate' | 'edit';
    onNavigateToStack?: (stackName: string) => void;
  }

  let {
    isOpen = false,
    onClose,
    onCreated,
    duplicateSource = null,
    mode = 'create',
    onNavigateToStack,
  }: Props = $props();

  let composeProject = $derived.by(() => {
    if (!duplicateSource) return null;
    const inspect = duplicateSource as ContainerInspectInfo;
    const cont = duplicateSource as Container;
    const labels = (inspect.Config?.Labels || cont.Labels) as Record<string, string> | undefined;
    return labels?.['com.docker.compose.project'] || null;
  });

  let composeService = $derived.by(() => {
    if (!duplicateSource) return null;
    const inspect = duplicateSource as ContainerInspectInfo;
    const cont = duplicateSource as Container;
    const labels = (inspect.Config?.Labels || cont.Labels) as Record<string, string> | undefined;
    return labels?.['com.docker.compose.service'] || null;
  });

  // Form states
  let image = $state('');
  let name = $state('');
  let restartPolicy = $state('unless-stopped');
  let memoryMb = $state<number | undefined>(undefined);
  let cpuCores = $state<number | undefined>(undefined);
  let autoStart = $state(true);
  let replaceExisting = $state(false);

  // Dynamic lists
  let ports = $state<Array<{ host_port: string; container_port: string; protocol: 'tcp' | 'udp' }>>([]);
  let volumes = $state<Array<{ host_path: string; container_path: string; read_only: boolean }>>([]);
  let envVars = $state<Array<{ key: string; value: string }>>([]);
  let baseImageEnvs = $state<Array<{ key: string; value: string }>>([]);
  let showImageDefaults = $state(false);
  let labels = $state<Array<{ key: string; value: string }>>([]);

  // Port conflict detection state
  let portConflicts = $state<Record<number, PortConflict>>({});
  let checkingPorts = $state(false);

  let checkPortTimer: ReturnType<typeof setTimeout> | null = null;
  function triggerPortCheck() {
    if (checkPortTimer) clearTimeout(checkPortTimer);
    checkPortTimer = setTimeout(async () => {
      const candidates: string[] = [];
      for (const p of ports) {
        const hp = parseInt(p.host_port.trim(), 10);
        if (hp > 0 && hp <= 65535) {
          candidates.push(`${hp}:${p.container_port || hp}/${p.protocol || 'tcp'}`);
        }
      }
      if (candidates.length === 0) {
        portConflicts = {};
        return;
      }
      checkingPorts = true;
      try {
        const res = await systemApi.checkPorts(candidates);
        const map: Record<number, PortConflict> = {};
        for (const c of res.conflicts || []) {
          if (
            replaceExisting &&
            duplicateSource &&
            (c.conflict_with?.includes(originalName) ||
              (duplicateSource.Id && c.conflict_with?.includes(duplicateSource.Id.slice(0, 12))))
          ) {
            continue;
          }
          map[c.host_port] = c;
        }
        portConflicts = map;
      } catch (err) {
        console.error('Failed to check port conflicts:', err);
      } finally {
        checkingPorts = false;
      }
    }, 250);
  }

  // Submission & UI states
  let isSubmitting = $state(false);
  let error = $state<string | null>(null);

  let originalName = $derived.by(() => {
    if (!duplicateSource) return '';
    if ('Name' in duplicateSource && typeof duplicateSource.Name === 'string') {
      return duplicateSource.Name.replace(/^\//, '');
    }
    if ('Names' in duplicateSource && Array.isArray(duplicateSource.Names) && duplicateSource.Names.length > 0) {
      return duplicateSource.Names[0].replace(/^\//, '');
    }
    return duplicateSource.Id ? duplicateSource.Id.slice(0, 12) : '';
  });

  let modalTitle = $derived.by(() => {
    if (replaceExisting) return 'Edit & Recreate Container';
    if (duplicateSource || mode === 'duplicate') return 'Duplicate Container';
    return 'Create Container';
  });

  let modalSubtitle = $derived.by(() => {
    if (replaceExisting) return `Reconfigure and redeploy container '${originalName}'`;
    if (duplicateSource || mode === 'duplicate') return `Clone configuration from '${originalName}'`;
    return 'Configure and deploy a standalone Docker container';
  });

  function resetForm() {
    image = '';
    name = '';
    restartPolicy = 'unless-stopped';
    memoryMb = undefined;
    cpuCores = undefined;
    autoStart = true;
    replaceExisting = false;
    ports = [];
    volumes = [];
    envVars = [];
    labels = [];
    baseImageEnvs = [];
    showImageDefaults = false;
    portConflicts = {};
    error = null;
  }

  function initFromSource(source: ContainerInspectInfo | Container, currentMode: 'create' | 'duplicate' | 'edit') {
    const origName = (
      ('Name' in source && typeof source.Name === 'string')
        ? source.Name
        : ('Names' in source && Array.isArray(source.Names) && source.Names.length > 0)
          ? source.Names[0]
          : ''
    ).replace(/^\//, '');

    const shouldReplace = currentMode === 'edit';
    replaceExisting = shouldReplace;

    if (shouldReplace) {
      name = origName;
    } else if (currentMode === 'duplicate') {
      name = origName ? `${origName}_copy` : '';
    } else {
      name = origName;
    }

    const inspect = source as ContainerInspectInfo;
    image = inspect.Config?.Image || inspect.Image || '';
    restartPolicy = inspect.HostConfig?.RestartPolicy?.Name || 'unless-stopped';
    if (!restartPolicy) restartPolicy = 'unless-stopped';

    if (inspect.HostConfig?.Memory && inspect.HostConfig.Memory > 0) {
      memoryMb = Math.round(inspect.HostConfig.Memory / (1024 * 1024));
    } else {
      memoryMb = undefined;
    }

    if (inspect.HostConfig?.NanoCPUs && inspect.HostConfig.NanoCPUs > 0) {
      cpuCores = Number((inspect.HostConfig.NanoCPUs / 1e9).toFixed(2));
    } else {
      cpuCores = undefined;
    }

    autoStart = true;

    // Ports
    const newPorts: Array<{ host_port: string; container_port: string; protocol: 'tcp' | 'udp' }> = [];
    const seenPorts = new Set<string>();

    const addPortItem = (cPortStr: string, protoStr: string, hPortStr: string) => {
      const proto: 'tcp' | 'udp' = protoStr.toLowerCase() === 'udp' ? 'udp' : 'tcp';
      const key = `${hPortStr}:${cPortStr}/${proto}`;
      if (!seenPorts.has(key)) {
        seenPorts.add(key);
        newPorts.push({
          host_port: hPortStr,
          container_port: cPortStr,
          protocol: proto,
        });
      }
    };

    if (inspect.HostConfig?.PortBindings) {
      for (const [key, bindings] of Object.entries(inspect.HostConfig.PortBindings)) {
        const [cPort, proto = 'tcp'] = key.split('/');
        if (bindings && bindings.length > 0) {
          for (const b of bindings) {
            addPortItem(cPort, proto, b.HostPort || '');
          }
        } else {
          addPortItem(cPort, proto, '');
        }
      }
    }

    if (inspect.NetworkSettings?.Ports) {
      for (const [key, bindings] of Object.entries(inspect.NetworkSettings.Ports)) {
        const [cPort, proto = 'tcp'] = key.split('/');
        if (bindings && bindings.length > 0) {
          for (const b of bindings) {
            addPortItem(cPort, proto, b.HostPort || '');
          }
        } else {
          addPortItem(cPort, proto, '');
        }
      }
    }

    if (newPorts.length === 0 && 'Ports' in source && Array.isArray(source.Ports)) {
      for (const p of source.Ports) {
        if (p.PrivatePort) {
          addPortItem(
            String(p.PrivatePort),
            p.Type || 'tcp',
            p.PublicPort ? String(p.PublicPort) : ''
          );
        }
      }
    }
    ports = newPorts;

    // Volumes
    const newVolumes: Array<{ host_path: string; container_path: string; read_only: boolean }> = [];
    const seenVolumes = new Set<string>();

    if (inspect.HostConfig?.Binds) {
      for (const b of inspect.HostConfig.Binds) {
        const parts = b.split(':');
        if (parts.length >= 2) {
          const hp = parts[0];
          const cp = parts[1];
          const ro = parts.length >= 3 && parts[2].includes('ro');
          const key = `${hp}:${cp}`;
          if (!seenVolumes.has(key)) {
            seenVolumes.add(key);
            newVolumes.push({ host_path: hp, container_path: cp, read_only: ro });
          }
        }
      }
    }

    if (source.Mounts) {
      for (const m of source.Mounts) {
        const hp = m.Source;
        const cp = m.Destination;
        const ro = !m.RW || (m.Mode ? m.Mode.includes('ro') : false);
        const key = `${hp}:${cp}`;
        if (!seenVolumes.has(key)) {
          seenVolumes.add(key);
          newVolumes.push({ host_path: hp, container_path: cp, read_only: ro });
        }
      }
    }
    volumes = newVolumes;

    // Environment variables
    const newEnv: Array<{ key: string; value: string }> = [];
    const baseEnvs: Array<{ key: string; value: string }> = [];

    // Parse ImageEnv if provided
    const imgEnvList = inspect.ImageEnv || [];
    const imgEnvMap = new Map<string, string>();
    for (const e of imgEnvList) {
      const idx = e.indexOf('=');
      const k = idx !== -1 ? e.slice(0, idx) : e;
      const v = idx !== -1 ? e.slice(idx + 1) : '';
      imgEnvMap.set(k, v);
      baseEnvs.push({ key: k, value: v });
    }
    baseImageEnvs = baseEnvs;

    // Use UserEnv if provided by backend
    if (inspect.UserEnv && inspect.UserEnv.length > 0) {
      for (const e of inspect.UserEnv) {
        const idx = e.indexOf('=');
        if (idx !== -1) {
          newEnv.push({ key: e.slice(0, idx), value: e.slice(idx + 1) });
        } else {
          newEnv.push({ key: e, value: '' });
        }
      }
    } else if (inspect.Config?.Env) {
      // Fallback: if UserEnv was empty or not populated, filter against imgEnvMap
      for (const e of inspect.Config.Env) {
        const idx = e.indexOf('=');
        const k = idx !== -1 ? e.slice(0, idx) : e;
        const v = idx !== -1 ? e.slice(idx + 1) : '';
        if (imgEnvMap.has(k) && imgEnvMap.get(k) === v) {
          // Unchanged base image default, skip from user env!
          continue;
        }
        newEnv.push({ key: k, value: v });
      }
    }
    envVars = newEnv;

    // Container Labels
    const srcLabels = inspect.Config?.Labels || ('Labels' in source ? (source as Container).Labels : undefined) || {};
    labels = Object.entries(srcLabels).map(([key, value]) => ({ key, value: String(value) }));

    showImageDefaults = false;
    error = null;
    triggerPortCheck();
  }

  let prevOpen = $state(false);

  $effect(() => {
    if (isOpen && !prevOpen) {
      if (duplicateSource) {
        initFromSource(duplicateSource, mode);
      } else {
        resetForm();
      }
    }
    prevOpen = isOpen;
  });

  function handleReplaceToggle(checked: boolean) {
    replaceExisting = checked;
    if (!duplicateSource) return;
    const orig = originalName;
    if (checked) {
      if (name === `${orig}_copy`) {
        name = orig;
      }
    } else {
      if (name === orig) {
        name = orig ? `${orig}_copy` : '';
      }
    }
    triggerPortCheck();
  }

  function addPort() {
    ports = [...ports, { host_port: '', container_port: '', protocol: 'tcp' }];
    triggerPortCheck();
  }

  function removePort(index: number) {
    ports = ports.filter((_, i) => i !== index);
    triggerPortCheck();
  }

  function addVolume() {
    volumes = [...volumes, { host_path: '', container_path: '', read_only: false }];
  }

  function removeVolume(index: number) {
    volumes = volumes.filter((_, i) => i !== index);
  }

  function addEnv() {
    envVars = [...envVars, { key: '', value: '' }];
  }

  function removeEnv(index: number) {
    envVars = envVars.filter((_, i) => i !== index);
  }

  function overrideBaseEnv(baseEnv: { key: string; value: string }) {
    if (!envVars.some(e => e.key === baseEnv.key)) {
      envVars = [...envVars, { key: baseEnv.key, value: baseEnv.value }];
    }
  }

  function addLabel(key = '', value = '') {
    labels = [...labels, { key, value }];
  }

  function removeLabel(index: number) {
    labels = labels.filter((_, i) => i !== index);
  }

  function addPresetLabel(key: string, defaultValue = '') {
    const existing = labels.find(l => l.key === key);
    if (!existing) {
      labels = [...labels, { key, value: defaultValue }];
    }
  }

  async function handleSubmit(e: SubmitEvent) {
    e.preventDefault();
    if (!image.trim()) {
      error = 'Image name is required (e.g., "nginx:alpine" or "redis:latest").';
      return;
    }

    isSubmitting = true;
    error = null;

    try {
      // Build request
      const req: CreateContainerRequest = {
        image: image.trim(),
        restart_policy: restartPolicy,
      };

      if (name.trim()) {
        req.name = name.trim();
      }

      // Valid port mappings
      const validPorts = ports
        .filter(p => p.container_port.trim())
        .map(p => ({
          host_port: p.host_port.trim(),
          container_port: p.container_port.trim(),
          protocol: p.protocol || 'tcp',
        }));
      if (validPorts.length > 0) {
        req.ports = validPorts;
      }

      // Valid volume mounts
      const validVolumes = volumes
        .filter(v => v.host_path.trim() && v.container_path.trim())
        .map(v => ({
          host_path: v.host_path.trim(),
          container_path: v.container_path.trim(),
          read_only: v.read_only,
        }));
      if (validVolumes.length > 0) {
        req.volumes = validVolumes;
      }

      // Valid environment variables
      const validEnv = envVars
        .filter(e => e.key.trim())
        .map(e => `${e.key.trim()}=${e.value}`);
      if (validEnv.length > 0) {
        req.env = validEnv;
      }

      // Valid container labels
      const validLabels: Record<string, string> = {};
      for (const l of labels) {
        const k = l.key.trim();
        if (k) {
          validLabels[k] = l.value.trim();
        }
      }
      if (Object.keys(validLabels).length > 0) {
        req.labels = validLabels;
      }

      // Resource limits
      if (memoryMb && memoryMb > 0) {
        req.memory_mb = Number(memoryMb);
      }
      if (cpuCores && cpuCores > 0) {
        req.cpu_cores = Number(cpuCores);
      }
      req.auto_start = autoStart;

      // If replacing existing container, launch recreation in background task
      if (replaceExisting && duplicateSource?.Id) {
        await containerApi.recreate(duplicateSource.Id, req);
        tasksStore.poll();
        resetForm();
        if (onCreated) {
          onCreated(duplicateSource.Id);
        }
        onClose();
        return;
      }

      // Create container
      const res = await containerApi.create(req);
      const containerId = res.id;

      // Automatically start container if requested
      if (autoStart && containerId) {
        try {
          await containerApi.start(containerId);
        } catch (startErr) {
          console.warn('Container created but failed to start automatically:', startErr);
        }
      }

      resetForm();
      if (onCreated && containerId) {
        onCreated(containerId);
      }
      onClose();
    } catch (err: unknown) {
      if (err instanceof Error) {
        error = err.message;
      } else {
        error = 'Failed to create container. Check image name and port availability.';
      }
    } finally {
      isSubmitting = false;
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape' && isOpen && !isSubmitting) {
      onClose();
    }
  }
</script>

<svelte:window onkeydown={handleKeydown} />

{#if isOpen}
  <div
    class="fixed inset-0 z-50 flex items-center justify-center p-4"
    role="dialog"
    aria-modal="true"
    aria-labelledby="modal-title"
    tabindex="-1"
  >
    <!-- Backdrop button -->
    <button
      type="button"
      class="fixed inset-0 bg-black/75 backdrop-blur-md cursor-default border-0"
      onclick={() => {
        if (!isSubmitting) onClose();
      }}
      aria-label="Close modal backdrop"
      tabindex="-1"
    ></button>
    <div
      class="relative w-full max-w-2xl max-h-[90vh] flex flex-col rounded-2xl bg-[#12131c] border border-white/[0.12] shadow-[0_10px_50px_rgba(0,0,0,0.8)] overflow-hidden"
    >
      <!-- Modal Header -->
      <div class="px-6 py-4 flex items-center justify-between border-b border-white/[0.08] bg-[#0c0d14]/80">
        <div class="flex items-center gap-3">
          <div class="w-8 h-8 rounded-xl {replaceExisting ? 'bg-amber-500/20 border-amber-500/30 text-amber-400 shadow-[0_0_15px_rgba(245,158,11,0.3)]' : duplicateSource ? 'bg-cyan-500/20 border-cyan-500/30 text-cyan-400 shadow-[0_0_15px_rgba(6,182,212,0.3)]' : 'bg-violet-600/20 border-violet-500/30 text-violet-400 shadow-[0_0_15px_rgba(124,58,237,0.3)]'} border flex items-center justify-center">
            {#if replaceExisting}
              <SlidersHorizontal class="w-4 h-4" />
            {:else if duplicateSource || mode === 'duplicate'}
              <Copy class="w-4 h-4" />
            {:else}
              <Plus class="w-4 h-4" />
            {/if}
          </div>
          <div>
            <h2 id="modal-title" class="text-sm font-bold text-white tracking-wide">{modalTitle}</h2>
            <p class="text-[11px] text-slate-400">{modalSubtitle}</p>
          </div>
        </div>

        <button
          type="button"
          onclick={onClose}
          disabled={isSubmitting}
          class="p-1.5 rounded-lg hover:bg-white/10 text-slate-400 hover:text-white transition disabled:opacity-50"
        >
          <X class="w-4 h-4" />
        </button>
      </div>

      <!-- Modal Body (Scrollable) -->
      <form
        onsubmit={handleSubmit}
        class="flex-1 overflow-y-auto p-6 space-y-6 text-xs"
      >
        {#if error}
          <div class="p-3 rounded-xl bg-rose-500/10 border border-rose-500/30 flex items-start gap-2.5 text-rose-300">
            <CircleAlert class="w-4 h-4 shrink-0 mt-0.5 text-rose-400" />
            <div class="flex-1 leading-relaxed">
              <span class="font-semibold block">Execution Error</span>
              <span class="text-[11px]">{error}</span>
            </div>
          </div>
        {/if}

        <!-- Replace existing container toggle (when duplicateSource exists) -->
        {#if duplicateSource}
          <div class="p-4 rounded-2xl bg-[#0a0b10] border border-white/[0.08] space-y-3">
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-3">
                <div class="w-8 h-8 rounded-xl bg-amber-500/10 border border-amber-500/20 flex items-center justify-center text-amber-400">
                  <SlidersHorizontal class="w-4 h-4" />
                </div>
                <div>
                  <span class="text-xs font-bold text-white block">Replace existing container</span>
                  <span class="text-[11px] text-slate-400">Stop and remove '{originalName}' and recreate with updated settings</span>
                </div>
              </div>
              <label class="relative inline-flex items-center cursor-pointer">
                <input
                  type="checkbox"
                  checked={replaceExisting}
                  onchange={(e) => handleReplaceToggle(e.currentTarget.checked)}
                  class="sr-only peer"
                />
                <div class="w-10 h-5 bg-white/10 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:bg-amber-500"></div>
              </label>
            </div>

            {#if replaceExisting}
              <div class="p-3 rounded-xl bg-amber-500/10 border border-amber-500/30 flex items-start gap-2.5 text-amber-300 animate-in fade-in duration-150">
                <CircleAlert class="w-4 h-4 shrink-0 mt-0.5 text-amber-400" />
                <div class="text-xs leading-relaxed">
                  <span class="font-semibold block text-amber-200">Destructive Reconfiguration</span>
                  <span class="text-[11px] text-amber-200/90">
                    The existing container '{originalName}' will be stopped and deleted before starting the new configuration.
                  </span>
                </div>
              </div>
            {/if}
          </div>
        {/if}

        <!-- Stack-Managed Container Reminder Banner -->
        {#if composeProject}
          <div class="p-4 rounded-2xl bg-amber-500/10 border border-amber-500/30 flex items-start gap-3.5 text-amber-300 animate-in fade-in duration-150">
            <AlertTriangle class="w-5 h-5 text-amber-400 shrink-0 mt-0.5" />
            <div class="space-y-1.5 flex-1 text-xs">
              <div class="flex items-center gap-2 flex-wrap">
                <span class="font-bold text-amber-200">Notice: Container is part of a Compose Stack</span>
                <span class="px-2 py-0.5 rounded-full text-[10px] font-mono bg-amber-500/20 text-amber-300 border border-amber-500/30">
                  {composeProject}
                </span>
              </div>
              <p class="text-[11px] text-amber-200/90 leading-relaxed">
                This container was deployed as service <code class="text-cyan-300 font-mono font-semibold">{composeService || 'service'}</code> of the Compose stack <strong class="text-white font-mono">{composeProject}</strong>.
                Editing or recreating this container manually is not recommended: changes may be overwritten on the next stack deployment, and network/service dependencies may break.
              </p>
              {#if onNavigateToStack}
                <div class="pt-1">
                  <button
                    type="button"
                    onclick={() => {
                      onClose();
                      onNavigateToStack?.(composeProject);
                    }}
                    class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-xl bg-violet-600/30 hover:bg-violet-600/50 text-cyan-300 border border-violet-500/30 text-xs font-semibold transition cursor-pointer"
                  >
                    <Layers class="w-3.5 h-3.5" />
                    <span>Edit Stack '{composeProject}' in Compose Studio instead &rarr;</span>
                  </button>
                </div>
              {/if}
            </div>
          </div>
        {/if}

        <!-- Primary Image & Container Name -->
        <div class="space-y-4">
          <h3 class="text-[11px] font-mono uppercase tracking-wider text-violet-400 font-semibold flex items-center gap-1.5">
            <Layers class="w-3.5 h-3.5" />
            Basic Configuration
          </h3>

          <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
            <!-- Image Name -->
            <div class="space-y-1.5">
              <label for="c-image" class="block font-medium text-slate-300">
                Image Name <span class="text-rose-400">*</span>
              </label>
              <input
                id="c-image"
                type="text"
                bind:value={image}
                placeholder="e.g. nginx:alpine, redis:7"
                required
                class="w-full px-3 py-2 rounded-xl bg-[#0a0b10] border border-white/[0.1] text-white placeholder-slate-600 focus:outline-none focus:border-cyan-400 focus:ring-1 focus:ring-cyan-400/50 transition font-mono text-xs"
              />
              <span class="text-[10px] text-slate-500">Docker Hub or private registry image reference</span>
            </div>

            <!-- Container Name -->
            <div class="space-y-1.5">
              <label for="c-name" class="block font-medium text-slate-300">
                Container Name <span class="text-slate-500 font-normal">(Optional)</span>
              </label>
              <input
                id="c-name"
                type="text"
                bind:value={name}
                placeholder="e.g. my-app-web"
                class="w-full px-3 py-2 rounded-xl bg-[#0a0b10] border border-white/[0.1] text-white placeholder-slate-600 focus:outline-none focus:border-cyan-400 focus:ring-1 focus:ring-cyan-400/50 transition font-mono text-xs"
              />
              <span class="text-[10px] text-slate-500">Unique alphanumeric name; auto-generated if left blank</span>
            </div>
          </div>
        </div>

        <!-- Port Mappings -->
        <div class="space-y-3 pt-2 border-t border-white/[0.06]">
          <div class="flex items-center justify-between">
            <h3 class="text-[11px] font-mono uppercase tracking-wider text-cyan-400 font-semibold flex items-center gap-1.5">
              <Network class="w-3.5 h-3.5" />
              Port Mappings
            </h3>
            <button
              type="button"
              onclick={addPort}
              class="px-2.5 py-1 rounded-lg bg-cyan-500/10 hover:bg-cyan-500/20 text-cyan-400 border border-cyan-500/25 transition inline-flex items-center gap-1 font-mono text-[11px]"
            >
              <Plus class="w-3 h-3" />
              <span>Add Port</span>
            </button>
          </div>

          {#if ports.length === 0}
            <p class="text-slate-500 text-[11px] italic">No port forwardings configured.</p>
          {:else}
            <div class="space-y-2.5">
              {#each ports as port, index}
                {@const hostPortNum = parseInt(port.host_port.trim(), 10)}
                {@const conflict = hostPortNum ? portConflicts[hostPortNum] : undefined}
                <div class="space-y-1.5 p-2 rounded-xl bg-[#0a0b10]/60 border {conflict ? 'border-amber-500/40 bg-amber-500/[0.04]' : 'border-white/[0.06]'} transition">
                  <div class="flex items-center gap-2">
                    <div class="flex-1 grid grid-cols-5 gap-2">
                      <input
                        type="text"
                        bind:value={port.host_port}
                        oninput={triggerPortCheck}
                        placeholder="Host (e.g. 8080)"
                        class="col-span-2 px-2.5 py-1.5 rounded-lg bg-[#0a0b10] border {conflict ? 'border-amber-500/50 text-amber-300' : 'border-white/[0.1] text-white'} placeholder-slate-600 font-mono text-xs focus:outline-none focus:border-cyan-400"
                      />
                      <input
                        type="text"
                        bind:value={port.container_port}
                        placeholder="Container (e.g. 80)"
                        class="col-span-2 px-2.5 py-1.5 rounded-lg bg-[#0a0b10] border border-white/[0.1] text-white placeholder-slate-600 font-mono text-xs focus:outline-none focus:border-cyan-400"
                      />
                      <select
                        bind:value={port.protocol}
                        onchange={triggerPortCheck}
                        class="col-span-1 px-2 py-1.5 rounded-lg bg-[#0a0b10] border border-white/[0.1] text-white font-mono text-xs focus:outline-none focus:border-cyan-400"
                      >
                        <option value="tcp">TCP</option>
                        <option value="udp">UDP</option>
                      </select>
                    </div>
                    <button
                      type="button"
                      onclick={() => removePort(index)}
                      class="p-1.5 rounded-lg hover:bg-rose-500/20 text-slate-500 hover:text-rose-400 transition"
                      title="Remove port mapping"
                    >
                      <Trash class="w-3.5 h-3.5" />
                    </button>
                  </div>
                  {#if conflict}
                    <div class="flex items-center justify-between text-[11px] px-2 py-1 rounded-lg bg-amber-500/10 border border-amber-500/20 text-amber-300">
                      <div class="flex items-center gap-1.5 truncate">
                        <AlertTriangle class="w-3.5 h-3.5 text-amber-400 shrink-0" />
                        <span class="truncate">Host port {hostPortNum} in use ({conflict.conflict_with || 'in use'})</span>
                      </div>
                      {#if conflict.suggested_port}
                        <button
                          type="button"
                          onclick={() => {
                            port.host_port = String(conflict.suggested_port);
                            triggerPortCheck();
                          }}
                          class="shrink-0 ml-2 px-2 py-0.5 rounded bg-emerald-500/20 hover:bg-emerald-500/30 text-emerald-400 border border-emerald-500/30 font-semibold cursor-pointer transition text-[10px]"
                        >
                          Use {conflict.suggested_port}
                        </button>
                      {/if}
                    </div>
                  {/if}
                </div>
              {/each}
            </div>
          {/if}
        </div>

        <!-- Volume Mounts -->
        <div class="space-y-3 pt-2 border-t border-white/[0.06]">
          <div class="flex items-center justify-between">
            <h3 class="text-[11px] font-mono uppercase tracking-wider text-amber-400 font-semibold flex items-center gap-1.5">
              <HardDrive class="w-3.5 h-3.5" />
              Volume Mounts
            </h3>
            <button
              type="button"
              onclick={addVolume}
              class="px-2.5 py-1 rounded-lg bg-amber-500/10 hover:bg-amber-500/20 text-amber-400 border border-amber-500/25 transition inline-flex items-center gap-1 font-mono text-[11px]"
            >
              <Plus class="w-3 h-3" />
              <span>Add Mount</span>
            </button>
          </div>

          {#if volumes.length === 0}
            <p class="text-slate-500 text-[11px] italic">No storage volumes or bind mounts attached.</p>
          {:else}
            <div class="space-y-2">
              {#each volumes as volume, index}
                <div class="flex items-center gap-2">
                  <div class="flex-1 grid grid-cols-5 gap-2">
                    <div class="col-span-2">
                      <HostPathInput
                        bind:value={volume.host_path}
                        placeholder="Host Path / Volume Name"
                        class="w-full px-2.5 py-1.5 rounded-lg bg-[#0a0b10] border border-white/[0.1] text-white placeholder-slate-600 font-mono text-xs focus:outline-none focus:border-cyan-400"
                      />
                    </div>
                    <input
                      type="text"
                      bind:value={volume.container_path}
                      placeholder="Container Destination Path"
                      class="col-span-2 px-2.5 py-1.5 rounded-lg bg-[#0a0b10] border border-white/[0.1] text-white placeholder-slate-600 font-mono text-xs focus:outline-none focus:border-cyan-400"
                    />
                    <label class="col-span-1 flex items-center gap-1.5 px-2 py-1.5 rounded-lg bg-[#0a0b10] border border-white/[0.1] text-slate-300 font-mono text-[11px] cursor-pointer">
                      <input
                        type="checkbox"
                        bind:checked={volume.read_only}
                        class="rounded bg-[#181926] border-white/20 text-amber-500"
                      />
                      <span>RO</span>
                    </label>
                  </div>
                  <button
                    type="button"
                    onclick={() => removeVolume(index)}
                    class="p-1.5 rounded-lg hover:bg-rose-500/20 text-slate-500 hover:text-rose-400 transition"
                    title="Remove volume"
                  >
                    <Trash class="w-3.5 h-3.5" />
                  </button>
                </div>
              {/each}
            </div>
          {/if}
        </div>

        <!-- Environment Variables -->
        <div class="space-y-3 pt-2 border-t border-white/[0.06]">
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-2">
              <h3 class="text-[11px] font-mono uppercase tracking-wider text-emerald-400 font-semibold flex items-center gap-1.5">
                <Terminal class="w-3.5 h-3.5" />
                Environment Variables
              </h3>
              {#if duplicateSource && baseImageEnvs.length > 0}
                <span class="px-1.5 py-0.5 rounded text-[10px] font-mono bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
                  User Defined ({envVars.length})
                </span>
              {/if}
            </div>
            <button
              type="button"
              onclick={addEnv}
              class="px-2.5 py-1 rounded-lg bg-emerald-500/10 hover:bg-emerald-500/20 text-emerald-400 border border-emerald-500/25 transition inline-flex items-center gap-1 font-mono text-[11px]"
            >
              <Plus class="w-3 h-3" />
              <span>Add Env</span>
            </button>
          </div>

          {#if envVars.length === 0}
            <p class="text-slate-500 text-[11px] italic">No user-defined environment variables.</p>
          {:else}
            <div class="space-y-2">
              {#each envVars as env, index}
                <div class="flex items-center gap-2">
                  <div class="flex-1 grid grid-cols-2 gap-2">
                    <input
                      type="text"
                      bind:value={env.key}
                      placeholder="KEY (e.g. PORT)"
                      class="px-2.5 py-1.5 rounded-lg bg-[#0a0b10] border border-white/[0.1] text-white placeholder-slate-600 font-mono text-xs focus:outline-none focus:border-cyan-400"
                    />
                    <input
                      type="text"
                      bind:value={env.value}
                      placeholder="VALUE (e.g. 3000)"
                      class="px-2.5 py-1.5 rounded-lg bg-[#0a0b10] border border-white/[0.1] text-white placeholder-slate-600 font-mono text-xs focus:outline-none focus:border-cyan-400"
                    />
                  </div>
                  <button
                    type="button"
                    onclick={() => removeEnv(index)}
                    class="p-1.5 rounded-lg hover:bg-rose-500/20 text-slate-500 hover:text-rose-400 transition"
                    title="Remove variable"
                  >
                    <Trash class="w-3.5 h-3.5" />
                  </button>
                </div>
              {/each}
            </div>
          {/if}

          <!-- Base Image Defaults section (if cloning/editing container with baked env) -->
          {#if baseImageEnvs.length > 0}
            <div class="pt-2 border-t border-white/[0.04]">
              <div class="flex items-center justify-between">
                <button
                  type="button"
                  onclick={() => showImageDefaults = !showImageDefaults}
                  class="text-[11px] font-mono text-slate-400 hover:text-slate-200 transition flex items-center gap-1.5"
                >
                  <span class="text-slate-500 font-semibold text-[9px]">{showImageDefaults ? '▼' : '▶'}</span>
                  <span>Base Image Defaults ({baseImageEnvs.length})</span>
                  <span class="px-1.5 py-0.2 rounded text-[9px] bg-white/[0.05] text-slate-400 border border-white/[0.08]">
                    Filtered from overrides
                  </span>
                </button>
              </div>

              {#if showImageDefaults}
                <div class="mt-2 p-2.5 rounded-xl bg-[#0a0b10]/80 border border-white/[0.06] space-y-1.5 max-h-48 overflow-y-auto">
                  <p class="text-[10px] text-slate-400 font-mono">
                    These variables are baked into the upstream image Dockerfile. They will not be overridden unless explicitly added above.
                  </p>
                  {#each baseImageEnvs as baseEnv}
                    {@const isOverridden = envVars.some(e => e.key === baseEnv.key)}
                    <div class="flex items-center justify-between gap-2 p-1.5 rounded-lg bg-white/[0.02] text-xs font-mono">
                      <div class="flex items-center gap-2 truncate flex-1 min-w-0">
                        <span class="px-1.5 py-0.5 rounded text-[9px] font-mono bg-slate-700/50 text-slate-300 border border-slate-600/40 shrink-0">
                          IMAGE
                        </span>
                        <span class="text-slate-300 font-semibold truncate">{baseEnv.key}=</span>
                        <span class="text-slate-400 truncate text-[11px]">{baseEnv.value}</span>
                      </div>
                      {#if isOverridden}
                        <span class="text-[10px] text-cyan-400 font-semibold shrink-0">Overridden</span>
                      {:else}
                        <button
                          type="button"
                          onclick={() => overrideBaseEnv(baseEnv)}
                          class="px-2 py-0.5 rounded bg-cyan-500/10 hover:bg-cyan-500/20 text-cyan-300 border border-cyan-500/30 text-[10px] font-mono shrink-0 transition"
                        >
                          + Override
                        </button>
                      {/if}
                    </div>
                  {/each}
                </div>
              {/if}
            </div>
          {/if}
        </div>

        <!-- Container Labels -->
        <div class="space-y-3 pt-2 border-t border-white/[0.06]">
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-2">
              <h3 class="text-[11px] font-mono uppercase tracking-wider text-violet-400 font-semibold flex items-center gap-1.5">
                <Tag class="w-3.5 h-3.5" />
                Container Labels
              </h3>
              {#if labels.length > 0}
                <span class="px-1.5 py-0.5 rounded text-[10px] font-mono bg-violet-500/10 text-violet-400 border border-violet-500/20">
                  {labels.length}
                </span>
              {/if}
            </div>

            <div class="flex items-center gap-1.5">
              <button
                type="button"
                onclick={() => addPresetLabel('pdswarm.url', 'http://')}
                class="px-2 py-0.5 rounded-lg bg-cyan-500/10 hover:bg-cyan-500/20 text-cyan-300 border border-cyan-500/30 text-[10px] font-mono transition flex items-center gap-1"
                title="Add custom app web URL label (opens directly when clicking app badge)"
              >
                <Plus class="w-3 h-3" />
                <span>+ pdswarm.url</span>
              </button>
              <button
                type="button"
                onclick={() => addPresetLabel('pdswarm.host', '')}
                class="px-2 py-0.5 rounded-lg bg-violet-500/10 hover:bg-violet-500/20 text-violet-300 border border-violet-500/30 text-[10px] font-mono transition flex items-center gap-1"
                title="Add custom host IP override label for this container's ports"
              >
                <Plus class="w-3 h-3" />
                <span>+ pdswarm.host</span>
              </button>
              <button
                type="button"
                onclick={() => addLabel()}
                class="px-2.5 py-1 rounded-lg bg-white/[0.06] hover:bg-white/[0.1] text-slate-300 text-[11px] font-mono transition inline-flex items-center gap-1"
              >
                <Plus class="w-3 h-3" />
                <span>Add Label</span>
              </button>
            </div>
          </div>

          {#if labels.length === 0}
            <div class="p-3 rounded-xl bg-[#0a0b10] border border-white/[0.04] text-center space-y-1">
              <p class="text-xs text-slate-400 font-mono">No custom labels defined.</p>
              <p class="text-[11px] text-slate-500">
                Labels allow setting custom web addresses (<span class="font-mono text-cyan-300">pdswarm.url</span>), host IP overrides (<span class="font-mono text-violet-300">pdswarm.host</span>), or Traefik/reverse proxy rules.
              </p>
            </div>
          {:else}
            <div class="space-y-2">
              {#each labels as label, index}
                <div class="flex items-center gap-2">
                  <div class="flex-1 grid grid-cols-2 gap-2">
                    <input
                      type="text"
                      bind:value={label.key}
                      placeholder="KEY (e.g. pdswarm.url)"
                      class="px-2.5 py-1.5 rounded-lg bg-[#0a0b10] border border-white/[0.1] text-white placeholder-slate-600 font-mono text-xs focus:outline-none focus:border-violet-400"
                    />
                    <input
                      type="text"
                      bind:value={label.value}
                      placeholder="VALUE (e.g. https://my-app.home.arpa)"
                      class="px-2.5 py-1.5 rounded-lg bg-[#0a0b10] border border-white/[0.1] text-white placeholder-slate-600 font-mono text-xs focus:outline-none focus:border-violet-400"
                    />
                  </div>
                  <button
                    type="button"
                    onclick={() => removeLabel(index)}
                    class="p-1.5 rounded-lg hover:bg-rose-500/20 text-slate-500 hover:text-rose-400 transition"
                    title="Remove label"
                  >
                    <Trash class="w-3.5 h-3.5" />
                  </button>
                </div>
              {/each}
            </div>
          {/if}
        </div>

        <!-- Policy & Resource Limits -->
        <div class="space-y-4 pt-2 border-t border-white/[0.06]">
          <h3 class="text-[11px] font-mono uppercase tracking-wider text-indigo-400 font-semibold flex items-center gap-1.5">
            <Cpu class="w-3.5 h-3.5" />
            Execution Policy & Limits
          </h3>

          <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
            <!-- Restart Policy -->
            <div class="space-y-1.5">
              <label for="c-policy" class="block font-medium text-slate-300">Restart Policy</label>
              <select
                id="c-policy"
                bind:value={restartPolicy}
                class="w-full px-3 py-2 rounded-xl bg-[#0a0b10] border border-white/[0.1] text-white focus:outline-none focus:border-cyan-400 font-mono text-xs"
              >
                <option value="unless-stopped">Unless Stopped</option>
                <option value="always">Always</option>
                <option value="on-failure">On Failure</option>
                <option value="no">Do Not Restart (no)</option>
              </select>
            </div>

            <!-- Memory Limit -->
            <div class="space-y-1.5">
              <label for="c-mem" class="block font-medium text-slate-300">Memory Limit (MB)</label>
              <input
                id="c-mem"
                type="number"
                min="0"
                step="64"
                bind:value={memoryMb}
                placeholder="Unlimited (e.g. 512)"
                class="w-full px-3 py-2 rounded-xl bg-[#0a0b10] border border-white/[0.1] text-white placeholder-slate-600 focus:outline-none focus:border-cyan-400 font-mono text-xs"
              />
            </div>

            <!-- CPU Limit -->
            <div class="space-y-1.5">
              <label for="c-cpu" class="block font-medium text-slate-300">CPU Cores</label>
              <input
                id="c-cpu"
                type="number"
                min="0"
                step="0.25"
                bind:value={cpuCores}
                placeholder="Unlimited (e.g. 1.0)"
                class="w-full px-3 py-2 rounded-xl bg-[#0a0b10] border border-white/[0.1] text-white placeholder-slate-600 focus:outline-none focus:border-cyan-400 font-mono text-xs"
              />
            </div>
          </div>

          <!-- Auto-Start checkbox -->
          <label class="flex items-center gap-2.5 pt-2 cursor-pointer select-none">
            <input
              type="checkbox"
              bind:checked={autoStart}
              class="w-4 h-4 rounded border border-white/20 text-violet-600 focus:ring-0 focus:ring-offset-0 bg-[#0a0b10]"
            />
            <span class="text-slate-300 font-medium">Immediately start container after creation</span>
          </label>
        </div>

        <!-- Hidden submit button so Enter key triggers submit -->
        <button type="submit" class="hidden" aria-hidden="true"></button>
      </form>

      <!-- Modal Footer -->
      <div class="px-6 py-4 flex items-center justify-end gap-3 border-t border-white/[0.08] bg-[#0c0d14]/80">
        <button
          type="button"
          onclick={onClose}
          disabled={isSubmitting}
          class="px-4 py-2 rounded-xl hover:bg-white/10 text-slate-300 text-xs font-semibold transition disabled:opacity-50"
        >
          Cancel
        </button>

        <button
          type="button"
          onclick={(e) => {
            const form = (e.currentTarget as HTMLElement).closest('.relative')?.querySelector('form');
            if (form) form.requestSubmit();
          }}
          disabled={isSubmitting || !image.trim()}
          class="px-5 py-2 rounded-xl bg-gradient-to-r {replaceExisting ? 'from-amber-600 to-orange-600 hover:from-amber-500 hover:to-orange-500 shadow-[0_0_20px_rgba(245,158,11,0.4)]' : 'from-violet-600 to-indigo-600 hover:from-violet-500 hover:to-indigo-500 shadow-[0_0_20px_rgba(124,58,237,0.4)]'} text-white text-xs font-semibold transition disabled:opacity-50 disabled:cursor-not-allowed inline-flex items-center gap-2"
        >
          {#if isSubmitting}
            <LoaderCircle class="w-3.5 h-3.5 animate-spin" />
            <span>{replaceExisting ? 'Recreating...' : (duplicateSource ? 'Duplicating...' : 'Creating...')}</span>
          {:else if replaceExisting}
            <RotateCw class="w-3.5 h-3.5" />
            <span>Recreate Container</span>
          {:else if duplicateSource || mode === 'duplicate'}
            <Copy class="w-3.5 h-3.5" />
            <span>Duplicate Container</span>
          {:else}
            <Play class="w-3.5 h-3.5 fill-current" />
            <span>Create & Launch</span>
          {/if}
        </button>
      </div>
    </div>
  </div>
{/if}
