<script lang="ts">
  import { untrack, onMount } from 'svelte';
  import {
    X,
    Rocket,
    AlertCircle,
    TriangleAlert,
    Loader2,
    Eye,
    EyeOff,
    Code,
    Copy,
    Check,
    ChevronDown,
    ChevronRight,
    Box,
    Layers,
    Info,
    Network,
    Plus,
    Trash,
    Sparkles,
    HardDrive,
    Pencil,
    RotateCcw,
    Sliders,
    Globe,
    Radio,
  } from '@lucide/svelte';
  import { systemApi, networkApi } from '../api';
  import { deploymentStore } from '../stores/deployments.svelte';
  import HostPathInput from './HostPathInput.svelte';
  import type { Template, TemplateEnvVar } from '../types';

  interface Props {
    template: Template;
    onClose: () => void;
    onSuccess: (stackName: string) => void;
  }

  let { template, onClose, onSuccess }: Props = $props();

  interface ConfiguredPort {
    id: string;
    hostPort: string;
    containerPort: string;
    protocol: string;
    inUse: boolean;
    conflictWith: string;
    suggestedPort: number;
    checking: boolean;
  }

  interface ConfiguredVolume {
    id: string;
    hostPath: string;
    containerPath: string;
    mode: 'rw' | 'ro';
  }

  interface ConfiguredNetwork {
    id: string;
    name: string;
    driver: string;
    isExternal: boolean;
  }

  // Active Modal Tab
  let activeTab = $state<'config' | 'yaml'>('config');

  let stackName = $state('');
  let envValues = $state<Record<string, string>>({});
  let effectiveEnvList = $state<TemplateEnvVar[]>([]);
  let showSecret = $state<Record<string, boolean>>({});
  let isEditingYaml = $state(false);
  let manualYamlText = $state('');
  let copiedYaml = $state(false);
  let isDeploying = $state(false);
  let error = $state<string | null>(null);
  let imgFailed = $state(false);

  // Accordion Sections Open/Closed States
  let isPortsOpen = $state(true);
  let isVolumesOpen = $state(true);
  let isNetworksOpen = $state(true);
  let isEnvOpen = $state(true);

  // Port Configuration & Conflict Monitoring
  let configuredPorts = $state<ConfiguredPort[]>([]);
  let checkingPorts = $state(false);

  // Volume Configuration
  let configuredVolumes = $state<ConfiguredVolume[]>([]);

  // Network Configuration
  let configuredNetworks = $state<ConfiguredNetwork[]>([]);
  let availableHostNetworks = $state<string[]>([]);

  let lastInitializedKey = '';
  let checkPortTimer: ReturnType<typeof setTimeout> | null = null;

  function safeStr(val: unknown): string {
    if (val === null || val === undefined) return '';
    return String(val);
  }

  // Bracket-aware port string splitting: splits on ':' only when NOT inside ${...}
  function splitPortString(raw: string): string[] {
    let insideBrackets = false;
    let splitIdx = -1;
    for (let i = 0; i < raw.length; i++) {
      if (raw[i] === '$' && i + 1 < raw.length && raw[i + 1] === '{') {
        insideBrackets = true;
        i++;
      } else if (insideBrackets && raw[i] === '}') {
        insideBrackets = false;
      } else if (!insideBrackets && raw[i] === ':') {
        splitIdx = i;
        break;
      }
    }
    if (splitIdx === -1) {
      return [raw];
    }
    return [raw.slice(0, splitIdx), raw.slice(splitIdx + 1)];
  }

  // Resolves default port from ${PORT:-3000} or returns val
  function resolvePortDefault(val: string): string {
    val = val.trim();
    const m = val.match(/^\$\{([^:-]+):-(.*)\}$/);
    if (m && m[2]) {
      return m[2].trim();
    }
    return val;
  }

  // Extracts environment variables from Compose YAML environment block
  function extractEnvFromYaml(yaml: string): Record<string, string> {
    const envMap: Record<string, string> = {};
    if (!yaml) return envMap;
    const lines = yaml.split('\n');
    let inEnvBlock = false;
    let baseIndent = -1;

    for (const line of lines) {
      const trimmed = line.trim();
      if (!trimmed || trimmed.startsWith('#')) continue;

      if (trimmed === 'environment:' || trimmed.startsWith('environment:')) {
        inEnvBlock = true;
        baseIndent = line.search(/\S/);
        continue;
      }

      if (inEnvBlock) {
        const currentIndent = line.search(/\S/);
        if (currentIndent <= baseIndent) {
          inEnvBlock = false;
          continue;
        }

        // List format: - KEY=VAL or - KEY
        const listMatch = trimmed.match(/^-\s*([A-Za-z_][A-Za-z0-9_]*)(?:=(.*))?$/);
        if (listMatch) {
          const key = listMatch[1];
          let val = listMatch[2] ?? '';
          val = val.replace(/^["']|["']$/g, '');
          const defMatch = val.match(/^\$\{[^:-]+:-(.*)\}$/);
          if (defMatch) val = defMatch[1];
          envMap[key] = val;
          continue;
        }

        // Map format: KEY: VAL or KEY:
        const mapMatch = trimmed.match(/^([A-Za-z_][A-Za-z0-9_]*):\s*(.*)$/);
        if (mapMatch) {
          const key = mapMatch[1];
          let val = mapMatch[2] ?? '';
          val = val.replace(/^["']|["']$/g, '');
          const defMatch = val.match(/^\$\{[^:-]+:-(.*)\}$/);
          if (defMatch) val = defMatch[1];
          envMap[key] = val;
          continue;
        }
      }
    }
    return envMap;
  }

  // Reactive effect tracking template changes (including async resolved compose_yaml / ports)
  $effect(() => {
    const tid = template?.id;
    const yamlLen = template?.compose_yaml ? template.compose_yaml.length : 0;
    const portsLen = template?.ports ? template.ports.length : 0;
    const volsLen = template?.volumes ? template.volumes.length : 0;
    const key = `${tid}::${yamlLen}::${portsLen}::${volsLen}`;
    if (tid && key !== lastInitializedKey) {
      lastInitializedKey = key;
      untrack(() => {
        initModalState();
      });
    }
  });

  const RESERVED_COMPOSE_KEYS = new Set([
    'version', 'services', 'volumes', 'networks', 'ports', 'labels',
    'deploy', 'restart', 'image', 'build', 'container_name', 'entrypoint',
    'command', 'depends_on', 'healthcheck', 'logging', 'devices',
    'cap_add', 'cap_drop', 'sysctls', 'ulimits', 'dns', 'expose',
    'extra_hosts', 'configs', 'secrets', 'environment', 'user',
    'working_dir', 'stdin_open', 'tty', 'privileged', 'hostname'
  ]);

  function isValidEnvName(name: string): boolean {
    if (!name || typeof name !== 'string') return false;
    const trimmed = name.trim();
    if (!/^[A-Za-z_][A-Za-z0-9_]*$/.test(trimmed)) return false;
    if (RESERVED_COMPOSE_KEYS.has(trimmed.toLowerCase())) return false;
    return true;
  }

  function initModalState() {
    const sanitizedId = (template.id || 'app').toLowerCase().replace(/[^a-z0-9_-]/g, '-');
    stackName = `${sanitizedId}-app`;
    const initialEnv: Record<string, string> = {};
    const envList: TemplateEnvVar[] = [];
    const seenKeys = new Set<string>();

    if (template.env && template.env.length > 0) {
      for (const ev of template.env) {
        if (!isValidEnvName(ev.name)) continue;
        envList.push(ev);
        seenKeys.add(ev.name);
        initialEnv[ev.name] = ev.default ?? '';
      }
    }

    if (template.compose_yaml) {
      const extracted = extractEnvFromYaml(template.compose_yaml);
      for (const [k, v] of Object.entries(extracted)) {
        if (!isValidEnvName(k)) continue;
        if (!seenKeys.has(k)) {
          seenKeys.add(k);
          envList.push({
            name: k,
            label: k.replace(/_/g, ' ').toLowerCase().replace(/\b\w/g, (c) => c.toUpperCase()),
            default: v,
            description: `Extracted from Compose environment`,
            required: false,
          });
          initialEnv[k] = v;
        }
      }
    }

    effectiveEnvList = envList;
    envValues = initialEnv;
    error = null;
    isDeploying = false;
    isEditingYaml = false;
    manualYamlText = '';
    copiedYaml = false;
    imgFailed = false;

    initPorts();
    initVolumes();
    initNetworks();
  }

  onMount(() => {
    fetchHostNetworks();
  });

  async function fetchHostNetworks() {
    try {
      const nets = await networkApi.list();
      if (nets && Array.isArray(nets)) {
        const names = nets.map((n) => n.Name).filter(Boolean);
        const set = new Set([...names, 'bridge', 'host', 'none']);
        availableHostNetworks = Array.from(set);
      } else {
        availableHostNetworks = ['bridge', 'host', 'none'];
      }
    } catch {
      availableHostNetworks = ['bridge', 'host', 'none'];
    }
  }

  function initPorts() {
    const rawPorts: string[] = [];
    if (template.ports && template.ports.length > 0) {
      rawPorts.push(...template.ports);
    } else if (template.compose_yaml) {
      const regex = /^\s*-\s*["']?(\S+?:\S+?)(?:\/(tcp|udp))?["']?\s*$/gm;
      let match;
      while ((match = regex.exec(template.compose_yaml)) !== null) {
        rawPorts.push(`${match[1]}${match[2] ? '/' + match[2] : ''}`);
      }
    }

    if (rawPorts.length > 0) {
      configuredPorts = rawPorts.map((raw, idx) => {
        let proto = 'tcp';
        let p = raw.trim();
        if (p.includes('/')) {
          const parts = p.split('/');
          p = parts[0];
          proto = (parts[1] || 'tcp').toLowerCase();
        }
        const splitted = splitPortString(p);
        const rawHp = splitted[0] || '';
        const rawCp = splitted[1] || splitted[0] || '';
        const hp = resolvePortDefault(rawHp);
        const cp = resolvePortDefault(rawCp);
        return {
          id: `p-${idx}-${Date.now()}-${Math.random().toString(36).substring(2, 6)}`,
          hostPort: hp,
          containerPort: cp,
          protocol: proto,
          inUse: false,
          conflictWith: '',
          suggestedPort: Number(hp) || 8080,
          checking: false,
        };
      });
    } else {
      configuredPorts = [];
    }

    triggerPortConflictCheck();
  }

  function initVolumes() {
    const rawVols: string[] = [];
    if (template.volumes && template.volumes.length > 0) {
      rawVols.push(...template.volumes);
    } else if (template.compose_yaml) {
      const regex = /^\s*-\s*["']?([^:\s"']+):([^:\s"']+)(?::(rw|ro))?["']?/gm;
      let match;
      while ((match = regex.exec(template.compose_yaml)) !== null) {
        const mode = match[3] || 'rw';
        rawVols.push(`${match[1]}:${match[2]}:${mode}`);
      }
    }

    if (rawVols.length > 0) {
      configuredVolumes = rawVols.map((raw, idx) => {
        let mode: 'rw' | 'ro' = 'rw';
        const parts = raw.split(':');
        const hp = parts[0] || '';
        const cp = parts[1] || '';
        if (parts.length > 2 && (parts[2] === 'ro' || parts[2] === 'rw')) {
          mode = parts[2] as 'rw' | 'ro';
        }
        return {
          id: `v-${idx}-${Date.now()}-${Math.random().toString(36).substring(2, 6)}`,
          hostPath: hp,
          containerPath: cp,
          mode,
        };
      });
    } else {
      configuredVolumes = [];
    }
  }

  function initNetworks() {
    const foundNetworks: ConfiguredNetwork[] = [];
    const seen = new Set<string>();

    if (template.compose_yaml) {
      const lines = template.compose_yaml.split('\n');
      let inRootNetworks = false;
      let rootIndent = -1;
      let currentNetName = '';
      let currentNetDriver = 'bridge';
      let currentNetExternal = false;

      const flushCurrentNet = () => {
        if (currentNetName && !seen.has(currentNetName)) {
          seen.add(currentNetName);
          foundNetworks.push({
            id: `n-${foundNetworks.length}-${Date.now()}-${Math.random().toString(36).substring(2, 6)}`,
            name: currentNetName,
            driver: currentNetDriver,
            isExternal: currentNetExternal,
          });
        }
        currentNetName = '';
        currentNetDriver = 'bridge';
        currentNetExternal = false;
      };

      for (const line of lines) {
        const trimmed = line.trim();
        if (!trimmed || trimmed.startsWith('#')) continue;

        if (/^networks:\s*$/.test(line)) {
          flushCurrentNet();
          inRootNetworks = true;
          rootIndent = 0;
          continue;
        }

        if (inRootNetworks) {
          const currentIndent = line.search(/\S/);
          if (currentIndent <= rootIndent) {
            flushCurrentNet();
            inRootNetworks = false;
            continue;
          }

          const netHeaderMatch = line.match(/^(\s{2,4})([A-Za-z0-9_-]+):\s*$/);
          if (netHeaderMatch) {
            flushCurrentNet();
            currentNetName = netHeaderMatch[2];
            continue;
          }

          if (currentNetName) {
            if (/driver:\s*([A-Za-z0-9_-]+)/i.test(trimmed)) {
              const dm = trimmed.match(/driver:\s*([A-Za-z0-9_-]+)/i);
              if (dm) currentNetDriver = dm[1].toLowerCase();
            }
            if (/external:\s*(true|false)/i.test(trimmed)) {
              const em = trimmed.match(/external:\s*(true|false)/i);
              if (em) currentNetExternal = em[1].toLowerCase() === 'true';
            }
          }
        }
      }
      flushCurrentNet();

      const serviceNetRegex = /^[ \t]*networks:\s*\n((?:[ \t]*-[ \t]*[A-Za-z0-9_-]+[^\n]*\n)+)/gm;
      let sMatch;
      while ((sMatch = serviceNetRegex.exec(template.compose_yaml)) !== null) {
        const itemLines = sMatch[1].split('\n');
        for (const iLine of itemLines) {
          const itemMatch = iLine.trim().match(/^-\s*([A-Za-z0-9_-]+)/);
          if (itemMatch) {
            const netName = itemMatch[1];
            if (!seen.has(netName) && !RESERVED_COMPOSE_KEYS.has(netName.toLowerCase())) {
              seen.add(netName);
              foundNetworks.push({
                id: `n-${foundNetworks.length}-${Date.now()}-${Math.random().toString(36).substring(2, 6)}`,
                name: netName,
                driver: 'bridge',
                isExternal: false,
              });
            }
          }
        }
      }
    }

    configuredNetworks = foundNetworks;
  }

  function triggerPortConflictCheck() {
    if (checkPortTimer) {
      clearTimeout(checkPortTimer);
    }
    checkPortTimer = setTimeout(() => {
      runPortConflictCheck();
    }, 250);
  }

  async function runPortConflictCheck() {
    if (configuredPorts.length === 0) {
      checkingPorts = false;
      return;
    }

    const portsToCheck = configuredPorts
      .filter((p) => safeStr(p.hostPort).trim() !== '')
      .map((p) => `${safeStr(p.hostPort).trim()}:${safeStr(p.containerPort).trim()}/${p.protocol}`);

    if (portsToCheck.length === 0) {
      checkingPorts = false;
      return;
    }

    checkingPorts = true;
    try {
      const res = await systemApi.checkPorts(portsToCheck);
      if (res && res.conflicts) {
        configuredPorts = configuredPorts.map((p) => {
          const conf = res.conflicts.find(
            (c) => c.host_port === Number(p.hostPort) && c.protocol === p.protocol
          );
          if (conf) {
            return {
              ...p,
              inUse: conf.in_use,
              conflictWith: conf.conflict_with || '',
              suggestedPort: conf.suggested_port || conf.host_port,
            };
          }
          return {
            ...p,
            inUse: false,
            conflictWith: '',
            suggestedPort: Number(p.hostPort) || 8080,
          };
        });
      }
    } catch {
      // Continue gracefully
    } finally {
      checkingPorts = false;
    }
  }

  function applySuggestedPort(idx: number) {
    if (configuredPorts[idx] && configuredPorts[idx].suggestedPort) {
      configuredPorts[idx].hostPort = String(configuredPorts[idx].suggestedPort);
      configuredPorts[idx].inUse = false;
      configuredPorts[idx].conflictWith = '';
      triggerPortConflictCheck();
    }
  }

  function addPortMapping() {
    isPortsOpen = true;
    const usedHostPorts = configuredPorts
      .map((p) => Number(p.hostPort))
      .filter((n) => !isNaN(n) && n > 0);
    const nextPort = usedHostPorts.length > 0 ? Math.max(...usedHostPorts) + 1 : 8080;

    configuredPorts = [
      ...configuredPorts,
      {
        id: `p-custom-${Date.now()}-${Math.random().toString(36).substring(2, 6)}`,
        hostPort: String(nextPort),
        containerPort: String(nextPort),
        protocol: 'tcp',
        inUse: false,
        conflictWith: '',
        suggestedPort: nextPort,
        checking: false,
      },
    ];
    triggerPortConflictCheck();
  }

  function removePortMapping(idx: number) {
    configuredPorts = configuredPorts.filter((_, i) => i !== idx);
    triggerPortConflictCheck();
  }

  function addVolumeMapping() {
    isVolumesOpen = true;
    const sName = stackName.trim() || 'app';
    const volNum = configuredVolumes.length + 1;
    configuredVolumes = [
      ...configuredVolumes,
      {
        id: `v-custom-${Date.now()}-${Math.random().toString(36).substring(2, 6)}`,
        hostPath: `${sName}_data_${volNum}`,
        containerPath: `/data${volNum > 1 ? volNum : ''}`,
        mode: 'rw',
      },
    ];
  }

  function removeVolumeMapping(idx: number) {
    configuredVolumes = configuredVolumes.filter((_, i) => i !== idx);
  }

  function addNetwork() {
    isNetworksOpen = true;
    const nextIdx = configuredNetworks.length + 1;
    const defaultName = configuredNetworks.length === 0 ? 'app_net' : `custom_net_${nextIdx}`;
    configuredNetworks = [
      ...configuredNetworks,
      {
        id: `net-${Date.now()}-${Math.random().toString(36).substring(2, 6)}`,
        name: defaultName,
        driver: 'bridge',
        isExternal: false,
      },
    ];
  }

  function removeNetwork(idx: number) {
    configuredNetworks = configuredNetworks.filter((_, i) => i !== idx);
  }

  function addCustomEnv() {
    isEnvOpen = true;
    const varNum = effectiveEnvList.length + 1;
    const varName = `CUSTOM_VAR_${varNum}`;
    effectiveEnvList = [
      ...effectiveEnvList,
      {
        name: varName,
        label: `Custom Variable ${varNum}`,
        default: '',
        description: 'User-defined environment variable',
        required: false,
      },
    ];
    envValues[varName] = '';
  }

  function removeCustomEnv(varName: string) {
    effectiveEnvList = effectiveEnvList.filter((ev) => ev.name !== varName);
    const updated = { ...envValues };
    delete updated[varName];
    envValues = updated;
  }

  function isSensitiveField(name: string, label: string): boolean {
    const combined = `${name} ${label}`.toLowerCase();
    return (
      combined.includes('password') ||
      combined.includes('secret') ||
      combined.includes('token') ||
      combined.includes('api_key') ||
      combined.includes('private')
    );
  }

  function toggleSecret(varName: string) {
    showSecret[varName] = !showSecret[varName];
  }

  // Dynamically rebuilt Compose YAML incorporating remapped ports, volumes, and networks
  const customComposeYaml = $derived.by(() => {
    let base = template.compose_yaml || '';

    // If base is empty or lacks services, construct working boilerplate
    if (!base.trim() || (!base.includes('services:') && !base.includes('image:'))) {
      const sanitizedApp = (template.id || 'app').toLowerCase().replace(/[^a-z0-9_-]/g, '-');
      const lines = [
        "version: '3.8'",
        'services:',
        `  ${sanitizedApp}:`,
        `    image: ${template.title.toLowerCase().replace(/[^a-z0-9]/g, '')}:latest`,
        '    restart: unless-stopped',
      ];
      base = lines.join('\n') + '\n';
    }

    let result = base;

    // Apply ports
    const validPorts = configuredPorts.filter(
      (p) => safeStr(p.hostPort).trim() !== '' && safeStr(p.containerPort).trim() !== ''
    );
    if (validPorts.length > 0) {
      const portsYamlLines = validPorts.map(
        (p) => `      - "${safeStr(p.hostPort).trim()}:${safeStr(p.containerPort).trim()}${p.protocol === 'udp' ? '/udp' : ''}"`
      );
      const portsBlock = `    ports:\n${portsYamlLines.join('\n')}\n`;
      if (result.includes('ports:')) {
        result = result.replace(/([ \t]*ports:\s*\n)(?:[ \t]*-[^\n]*\n)*/, portsBlock);
      } else if (result.includes('restart:')) {
        result = result.replace(/(restart:[^\n]*\n)/, `$1${portsBlock}`);
      } else if (result.includes('image:')) {
        result = result.replace(/(image:[^\n]*\n)/, `$1${portsBlock}`);
      } else {
        result = result + '\n' + portsBlock;
      }
    }

    // Apply volumes
    const validVolumes = configuredVolumes.filter(
      (v) => safeStr(v.hostPath).trim() !== '' && safeStr(v.containerPath).trim() !== ''
    );
    if (validVolumes.length > 0) {
      const volsYamlLines = validVolumes.map(
        (v) => `      - "${safeStr(v.hostPath).trim()}:${safeStr(v.containerPath).trim()}${v.mode === 'ro' ? ':ro' : ''}"`
      );
      const volsBlock = `    volumes:\n${volsYamlLines.join('\n')}\n`;
      if (result.includes('volumes:')) {
        result = result.replace(/([ \t]*volumes:\s*\n)(?:[ \t]*-[^\n]*\n)*/, volsBlock);
      } else if (result.includes('ports:')) {
        result = result.replace(/([ \t]*ports:\s*\n(?:[ \t]*-[^\n]*\n)*)/, `$1${volsBlock}`);
      } else if (result.includes('restart:')) {
        result = result.replace(/(restart:[^\n]*\n)/, `$1${volsBlock}`);
      } else if (result.includes('image:')) {
        result = result.replace(/(image:[^\n]*\n)/, `$1${volsBlock}`);
      } else {
        result = result + '\n' + volsBlock;
      }
    }

    // Apply networks
    const validNetworks = configuredNetworks.filter((n) => safeStr(n.name).trim() !== '');
    if (validNetworks.length > 0) {
      const netServiceLines = validNetworks.map((n) => `      - ${safeStr(n.name).trim()}`);
      const netServiceBlock = `    networks:\n${netServiceLines.join('\n')}\n`;

      if (/([ \t]+networks:\s*\n)(?:[ \t]+-[^\n]*\n)*/.test(result)) {
        result = result.replace(/([ \t]+networks:\s*\n)(?:[ \t]+-[^\n]*\n)*/, netServiceBlock);
      } else if (result.includes('volumes:')) {
        result = result.replace(/([ \t]*volumes:\s*\n(?:[ \t]*-[^\n]*\n)*)/, `$1${netServiceBlock}`);
      } else if (result.includes('ports:')) {
        result = result.replace(/([ \t]*ports:\s*\n(?:[ \t]*-[^\n]*\n)*)/, `$1${netServiceBlock}`);
      } else if (result.includes('restart:')) {
        result = result.replace(/(restart:[^\n]*\n)/, `$1${netServiceBlock}`);
      } else if (result.includes('image:')) {
        result = result.replace(/(image:[^\n]*\n)/, `$1${netServiceBlock}`);
      } else {
        result = result + '\n' + netServiceBlock;
      }

      // Root-level networks definition
      const rootNetLines: string[] = ['\nnetworks:'];
      for (const n of validNetworks) {
        const netName = safeStr(n.name).trim();
        rootNetLines.push(`  ${netName}:`);
        if (n.isExternal) {
          rootNetLines.push('    external: true');
        } else {
          rootNetLines.push(`    driver: ${n.driver || 'bridge'}`);
        }
      }
      const rootNetBlock = rootNetLines.join('\n') + '\n';

      if (/^networks:\s*\n(?:[ \t]+[^\n]*\n)*/m.test(result)) {
        result = result.replace(/^networks:\s*\n(?:[ \t]+[^\n]*\n)*/m, rootNetBlock.trimStart());
      } else {
        result = result.trimEnd() + '\n' + rootNetBlock;
      }
    }

    return result;
  });

  const finalEffectiveYaml = $derived(
    isEditingYaml && manualYamlText.trim() ? manualYamlText : customComposeYaml
  );

  const hasPortConflict = $derived(configuredPorts.some((p) => p.inUse));

  function toggleYamlEdit() {
    if (!isEditingYaml) {
      manualYamlText = customComposeYaml;
      isEditingYaml = true;
      activeTab = 'yaml';
    } else {
      isEditingYaml = false;
    }
  }

  function resetYamlFromForm() {
    manualYamlText = customComposeYaml;
    isEditingYaml = false;
  }

  async function copyYaml() {
    if (!finalEffectiveYaml) return;
    try {
      await navigator.clipboard.writeText(finalEffectiveYaml);
      copiedYaml = true;
      setTimeout(() => {
        copiedYaml = false;
      }, 2000);
    } catch {
      // Fallback
    }
  }

  function handleDeploy(e?: Event) {
    if (e) e.preventDefault();
    const trimmed = stackName.trim();
    if (!trimmed) {
      error = 'Stack name is required.';
      activeTab = 'config';
      return;
    }

    if (!/^[a-zA-Z0-9][a-zA-Z0-9_.-]*$/.test(trimmed)) {
      error = 'Stack name must contain only alphanumeric characters, underscores, hyphens, or dots.';
      activeTab = 'config';
      return;
    }

    if (effectiveEnvList.length > 0) {
      for (const ev of effectiveEnvList) {
        if (ev.required && (!envValues[ev.name] || !envValues[ev.name].trim())) {
          error = `Required environment variable "${ev.label || ev.name}" must be provided.`;
          activeTab = 'config';
          return;
        }
      }
    }

    if (hasPortConflict) {
      error = 'One or more host ports have active conflicts. Please resolve the conflict using the suggested port or choose another free port.';
      activeTab = 'config';
      return;
    }

    // Launch background deployment task in store
    deploymentStore.startDeployment(
      template,
      trimmed,
      envValues,
      finalEffectiveYaml
    );

    // Close modal and notify parent
    onSuccess(trimmed);
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape' && !isDeploying) {
      onClose();
    }
  }
</script>

<svelte:window onkeydown={handleKeydown} />

<!-- Modal Backdrop -->
<div
  class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-md animate-in fade-in duration-150"
>
  <!-- Modal Dialog Card -->
  <div
    class="relative w-full max-w-2xl bg-[#12131c] border border-white/[0.08] shadow-[0_0_50px_rgba(124,58,237,0.15)] rounded-2xl flex flex-col max-h-[92vh] overflow-hidden animate-in zoom-in-95 duration-150"
    role="dialog"
    aria-modal="true"
    aria-labelledby="template-modal-title"
  >
    <!-- Modal Header -->
    <div
      class="flex items-center justify-between p-5 border-b border-white/[0.08] bg-[#181a26]/50 shrink-0"
    >
      <div class="flex items-center gap-3.5 min-w-0">
        <!-- Logo Avatar -->
        <div class="relative shrink-0">
          {#if template.logo && !imgFailed}
            <div
              class="w-12 h-12 rounded-xl bg-[#0a0b10] border border-white/[0.08] p-1.5 flex items-center justify-center shadow-[0_0_15px_rgba(124,58,237,0.2)] overflow-hidden"
            >
              <img
                src={template.logo}
                alt={template.title}
                class="w-full h-full object-contain"
                onerror={() => {
                  imgFailed = true;
                }}
              />
            </div>
          {:else}
            <div
              class="w-12 h-12 rounded-xl bg-gradient-to-br from-violet-600/30 to-indigo-600/20 border border-violet-500/40 text-violet-300 font-mono font-bold text-lg flex items-center justify-center shadow-[0_0_20px_rgba(124,58,237,0.3)]"
            >
              {(template.title || template.id || '?').charAt(0).toUpperCase()}
            </div>
          {/if}
        </div>

        <div class="min-w-0">
          <div class="flex items-center gap-2">
            <h2 id="template-modal-title" class="text-base font-bold text-white truncate">
              {template.title}
            </h2>
            <span
              class="px-2 py-0.5 rounded-full text-[10px] font-mono uppercase tracking-wider bg-violet-500/10 text-violet-400 border border-violet-500/30 shrink-0"
            >
              1-Click Deploy
            </span>
          </div>
          <p class="text-xs text-slate-400 mt-0.5 max-w-md line-clamp-1">
            {template.description || 'Configurable template for PDSwarm'}
          </p>
        </div>
      </div>

      <button
        type="button"
        onclick={onClose}
        disabled={isDeploying}
        class="p-2 rounded-xl text-slate-400 hover:text-white hover:bg-white/5 transition disabled:opacity-50 cursor-pointer"
        aria-label="Close modal"
      >
        <X class="w-5 h-5" />
      </button>
    </div>

    <!-- Navigation Tabs Bar -->
    <div class="flex items-center px-5 border-b border-white/[0.06] bg-[#0d0e16] shrink-0 gap-1">
      <button
        type="button"
        onclick={() => (activeTab = 'config')}
        class="flex items-center gap-2 px-3.5 py-2.5 text-xs font-semibold border-b-2 transition cursor-pointer {activeTab ===
        'config'
          ? 'border-violet-500 text-violet-400 bg-violet-500/5'
          : 'border-transparent text-slate-400 hover:text-slate-200'}"
      >
        <Sliders class="w-3.5 h-3.5" />
        <span>Configuration</span>
      </button>

      <button
        type="button"
        onclick={() => (activeTab = 'yaml')}
        class="flex items-center gap-2 px-3.5 py-2.5 text-xs font-semibold border-b-2 transition cursor-pointer {activeTab ===
        'yaml'
          ? 'border-violet-500 text-violet-400 bg-violet-500/5'
          : 'border-transparent text-slate-400 hover:text-slate-200'}"
      >
        <Code class="w-3.5 h-3.5" />
        <span>Compose YAML</span>
        {#if isEditingYaml}
          <span
            class="px-1.5 py-0.2 rounded text-[9px] font-mono bg-amber-500/20 text-amber-300 border border-amber-500/30"
          >
            Custom
          </span>
        {/if}
      </button>
    </div>

    <!-- Modal Body Content -->
    <div class="flex-1 overflow-y-auto p-5">
      <!-- Error Message Banner -->
      {#if error}
        <div
          class="flex items-start gap-3 p-3.5 mb-4 rounded-xl bg-red-500/10 border border-red-500/30 text-red-400 text-xs animate-in fade-in"
        >
          <AlertCircle class="w-4 h-4 shrink-0 mt-0.5 text-red-400" />
          <div class="flex-1 font-mono break-all">{error}</div>
        </div>
      {/if}

      <!-- TAB 1: CONFIGURATION FORM -->
      {#if activeTab === 'config'}
        <form id="template-deploy-form" onsubmit={handleDeploy} class="space-y-5">
          <!-- Stack Name Section -->
          <div class="space-y-1.5">
            <label
              for="stack-name-input"
              class="text-xs font-medium text-slate-200 flex items-center gap-1.5"
            >
              <span>Stack Name</span>
              <span class="text-violet-400 font-mono text-[10px]">*</span>
            </label>
            <input
              id="stack-name-input"
              type="text"
              bind:value={stackName}
              required
              disabled={isDeploying}
              placeholder="e.g. my-app"
              class="w-full px-3.5 py-2 rounded-xl bg-[#0a0b10] border border-white/[0.08] focus:border-violet-500 focus:outline-none text-xs text-white font-mono placeholder-slate-600 transition"
            />
            <p class="text-[11px] text-slate-500">
              Name of the Docker Compose stack created for this deployment.
            </p>
          </div>

          <!-- 1. Port Mappings & Conflict Monitor Collapsible Card -->
          <div class="rounded-xl border border-white/[0.08] bg-[#0c0d14] overflow-hidden transition-all duration-200">
            <div
              role="button"
              tabindex="0"
              aria-expanded={isPortsOpen}
              class="flex items-center justify-between p-3.5 bg-[#12131c]/60 hover:bg-[#12131c] cursor-pointer transition select-none"
              onclick={() => (isPortsOpen = !isPortsOpen)}
              onkeydown={(e) => {
                if (e.key === 'Enter' || e.key === ' ') {
                  e.preventDefault();
                  isPortsOpen = !isPortsOpen;
                }
              }}
            >
              <div class="flex items-center gap-2.5">
                <Radio class="w-4 h-4 text-violet-400" />
                <h3 class="text-xs font-semibold text-slate-200 uppercase tracking-wider">
                  Port Mappings & Conflict Monitor
                </h3>
                <span
                  class="px-2 py-0.5 rounded-full text-[10px] font-mono bg-white/[0.05] text-slate-400 border border-white/[0.04]"
                >
                  {configuredPorts.length} {configuredPorts.length === 1 ? 'Port' : 'Ports'}
                </span>
                {#if hasPortConflict}
                  <span
                    class="px-2 py-0.5 rounded-full text-[10px] font-mono bg-amber-500/10 text-amber-400 border border-amber-500/30"
                  >
                    Conflict
                  </span>
                {/if}
              </div>

              <div class="flex items-center gap-2">
                <button
                  type="button"
                  onclick={(e) => {
                    e.stopPropagation();
                    addPortMapping();
                  }}
                  disabled={isDeploying}
                  class="flex items-center gap-1.5 px-2.5 py-1 rounded-lg text-xs font-medium bg-white/[0.05] hover:bg-white/[0.1] text-violet-300 border border-violet-500/20 transition cursor-pointer"
                >
                  <Plus class="w-3.5 h-3.5" />
                  <span>Add Port</span>
                </button>

                <div class="text-slate-400 hover:text-white p-1">
                  {#if isPortsOpen}
                    <ChevronDown class="w-4 h-4" />
                  {:else}
                    <ChevronRight class="w-4 h-4" />
                  {/if}
                </div>
              </div>
            </div>

            {#if isPortsOpen}
              <div class="p-4 border-t border-white/[0.06] space-y-3 bg-[#0a0b10]/40 animate-in fade-in duration-150">
                {#if configuredPorts.length === 0}
                  <div
                    class="p-4 rounded-xl border border-dashed border-white/[0.08] text-center text-xs text-slate-500"
                  >
                    No ports exposed by default. Click "Add Port" to publish a container port to the host.
                  </div>
                {:else}
                  <div class="space-y-2.5">
                    {#each configuredPorts as port, idx (port.id)}
                      <div
                        class="p-3 rounded-xl bg-[#0a0b10]/70 border transition space-y-2 {port.inUse
                          ? 'border-amber-500/40 bg-amber-950/10'
                          : 'border-white/[0.06] hover:border-white/[0.12]'}"
                      >
                        <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-2.5">
                          <div class="flex items-center gap-2 flex-1">
                            <div class="space-y-1 flex-1">
                              <span class="text-[10px] font-mono text-slate-400">Host Port</span>
                              <input
                                type="text"
                                inputmode="numeric"
                                pattern="[0-9]*"
                                value={safeStr(port.hostPort)}
                                oninput={(e) => {
                                  port.hostPort = e.currentTarget.value.replace(/[^0-9]/g, '');
                                  triggerPortConflictCheck();
                                }}
                                disabled={isDeploying}
                                placeholder="e.g. 5000"
                                class="w-full px-3 py-1.5 rounded-lg bg-[#12131c] border border-white/[0.08] focus:border-violet-500 focus:outline-none text-xs text-white font-mono"
                              />
                            </div>

                            <span class="text-slate-500 font-mono text-xs pt-4">&rarr;</span>

                            <div class="space-y-1 flex-1">
                              <span class="text-[10px] font-mono text-slate-400">Container Port</span>
                              <input
                                type="text"
                                inputmode="numeric"
                                pattern="[0-9]*"
                                value={safeStr(port.containerPort)}
                                oninput={(e) => {
                                  port.containerPort = e.currentTarget.value.replace(/[^0-9]/g, '');
                                }}
                                disabled={isDeploying}
                                placeholder="e.g. 5000"
                                class="w-full px-3 py-1.5 rounded-lg bg-[#12131c] border border-white/[0.08] focus:border-violet-500 focus:outline-none text-xs text-white font-mono"
                              />
                            </div>

                            <div class="space-y-1 w-20 shrink-0">
                              <span class="text-[10px] font-mono text-slate-400">Proto</span>
                              <select
                                bind:value={port.protocol}
                                onchange={() => triggerPortConflictCheck()}
                                disabled={isDeploying}
                                class="w-full px-2 py-1.5 rounded-lg bg-[#12131c] border border-white/[0.08] focus:border-violet-500 focus:outline-none text-xs text-white font-mono"
                              >
                                <option value="tcp">TCP</option>
                                <option value="udp">UDP</option>
                              </select>
                            </div>

                            <div class="pt-4">
                              <button
                                type="button"
                                onclick={() => removePortMapping(idx)}
                                disabled={isDeploying}
                                class="p-2 rounded-lg text-slate-500 hover:text-red-400 hover:bg-red-500/10 transition cursor-pointer"
                                title="Remove port mapping"
                              >
                                <Trash class="w-3.5 h-3.5" />
                              </button>
                            </div>
                          </div>
                        </div>

                        <!-- Status & Conflict resolution helper -->
                        <div class="flex items-center justify-between text-[11px] pt-1">
                          {#if port.inUse}
                            <div class="flex items-center gap-1.5 text-amber-400 font-medium">
                              <TriangleAlert class="w-3.5 h-3.5 shrink-0" />
                              <span>Port {port.hostPort} in use by {port.conflictWith || 'host service'}</span>
                            </div>

                            {#if port.suggestedPort && String(port.suggestedPort) !== port.hostPort}
                              <button
                                type="button"
                                onclick={() => applySuggestedPort(idx)}
                                class="flex items-center gap-1 px-2.5 py-1 rounded-md text-[11px] font-mono font-semibold bg-violet-600/30 hover:bg-violet-600/50 text-violet-300 border border-violet-500/30 transition cursor-pointer"
                                title="Auto-switch to next available free port"
                              >
                                <Sparkles class="w-3 h-3 text-violet-300" />
                                <span>Use Free Port {port.suggestedPort}</span>
                              </button>
                            {/if}
                          {:else if safeStr(port.hostPort).trim()}
                            <div class="flex items-center gap-1.5 text-emerald-400 font-medium">
                              <Check class="w-3.5 h-3.5 shrink-0" />
                              <span>Port {port.hostPort} is available</span>
                            </div>
                          {:else}
                            <span class="text-slate-500">Enter a host port number</span>
                          {/if}
                        </div>
                      </div>
                    {/each}
                  </div>
                {/if}
              </div>
            {/if}
          </div>

          <!-- 2. Volume & Storage Mappings Collapsible Card -->
          <div class="rounded-xl border border-white/[0.08] bg-[#0c0d14] overflow-hidden transition-all duration-200">
            <div
              role="button"
              tabindex="0"
              aria-expanded={isVolumesOpen}
              class="flex items-center justify-between p-3.5 bg-[#12131c]/60 hover:bg-[#12131c] cursor-pointer transition select-none"
              onclick={() => (isVolumesOpen = !isVolumesOpen)}
              onkeydown={(e) => {
                if (e.key === 'Enter' || e.key === ' ') {
                  e.preventDefault();
                  isVolumesOpen = !isVolumesOpen;
                }
              }}
            >
              <div class="flex items-center gap-2.5">
                <HardDrive class="w-4 h-4 text-violet-400" />
                <h3 class="text-xs font-semibold text-slate-200 uppercase tracking-wider">
                  Volume & Storage Mappings
                </h3>
                <span
                  class="px-2 py-0.5 rounded-full text-[10px] font-mono bg-white/[0.05] text-slate-400 border border-white/[0.04]"
                >
                  {configuredVolumes.length} {configuredVolumes.length === 1 ? 'Volume' : 'Volumes'}
                </span>
              </div>

              <div class="flex items-center gap-2">
                <button
                  type="button"
                  onclick={(e) => {
                    e.stopPropagation();
                    addVolumeMapping();
                  }}
                  disabled={isDeploying}
                  class="flex items-center gap-1.5 px-2.5 py-1 rounded-lg text-xs font-medium bg-white/[0.05] hover:bg-white/[0.1] text-violet-300 border border-violet-500/20 transition cursor-pointer"
                >
                  <Plus class="w-3.5 h-3.5" />
                  <span>Add Volume</span>
                </button>

                <div class="text-slate-400 hover:text-white p-1">
                  {#if isVolumesOpen}
                    <ChevronDown class="w-4 h-4" />
                  {:else}
                    <ChevronRight class="w-4 h-4" />
                  {/if}
                </div>
              </div>
            </div>

            {#if isVolumesOpen}
              <div class="p-4 border-t border-white/[0.06] space-y-3 bg-[#0a0b10]/40 animate-in fade-in duration-150">
                {#if configuredVolumes.length === 0}
                  <div
                    class="p-4 rounded-xl border border-dashed border-white/[0.08] text-center text-xs text-slate-500"
                  >
                    No persistent volumes configured. Click "Add Volume" to persist container data.
                  </div>
                {:else}
                  <div class="space-y-2.5">
                    {#each configuredVolumes as vol, idx (vol.id)}
                      <div
                        class="p-3 rounded-xl bg-[#0a0b10]/70 border border-white/[0.06] hover:border-white/[0.12] transition space-y-2"
                      >
                        <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-2.5">
                          <div class="flex items-center gap-2 flex-1">
                            <div class="space-y-1 flex-1">
                              <span class="text-[10px] font-mono text-slate-400">Host Path or Volume</span>
                              <HostPathInput
                                bind:value={vol.hostPath}
                                placeholder="e.g. /opt/appdata or my_data"
                                disabled={isDeploying}
                              />
                            </div>

                            <span class="text-slate-500 font-mono text-xs pt-4">&rarr;</span>

                            <div class="space-y-1 flex-1">
                              <span class="text-[10px] font-mono text-slate-400">Container Path</span>
                              <input
                                type="text"
                                bind:value={vol.containerPath}
                                disabled={isDeploying}
                                placeholder="e.g. /data"
                                class="w-full px-3 py-1.5 rounded-lg bg-[#12131c] border border-white/[0.08] focus:border-violet-500 focus:outline-none text-xs text-white font-mono"
                              />
                            </div>

                            <div class="space-y-1 w-20 shrink-0">
                              <span class="text-[10px] font-mono text-slate-400">Mode</span>
                              <select
                                bind:value={vol.mode}
                                disabled={isDeploying}
                                class="w-full px-2 py-1.5 rounded-lg bg-[#12131c] border border-white/[0.08] focus:border-violet-500 focus:outline-none text-xs text-white font-mono"
                              >
                                <option value="rw">RW</option>
                                <option value="ro">RO</option>
                              </select>
                            </div>

                            <div class="pt-4">
                              <button
                                type="button"
                                onclick={() => removeVolumeMapping(idx)}
                                disabled={isDeploying}
                                class="p-2 rounded-lg text-slate-500 hover:text-red-400 hover:bg-red-500/10 transition cursor-pointer"
                                title="Remove volume"
                              >
                                <Trash class="w-3.5 h-3.5" />
                              </button>
                            </div>
                          </div>
                        </div>
                      </div>
                    {/each}
                  </div>
                {/if}
              </div>
            {/if}
          </div>

          <!-- 3. Network Configuration Collapsible Card -->
          <div class="rounded-xl border border-white/[0.08] bg-[#0c0d14] overflow-hidden transition-all duration-200">
            <datalist id="host-network-options">
              {#each availableHostNetworks as hNet}
                <option value={hNet}></option>
              {/each}
            </datalist>

            <div
              role="button"
              tabindex="0"
              aria-expanded={isNetworksOpen}
              class="flex items-center justify-between p-3.5 bg-[#12131c]/60 hover:bg-[#12131c] cursor-pointer transition select-none"
              onclick={() => (isNetworksOpen = !isNetworksOpen)}
              onkeydown={(e) => {
                if (e.key === 'Enter' || e.key === ' ') {
                  e.preventDefault();
                  isNetworksOpen = !isNetworksOpen;
                }
              }}
            >
              <div class="flex items-center gap-2.5">
                <Globe class="w-4 h-4 text-violet-400" />
                <h3 class="text-xs font-semibold text-slate-200 uppercase tracking-wider">
                  Network Configuration
                </h3>
                <span
                  class="px-2 py-0.5 rounded-full text-[10px] font-mono bg-white/[0.05] text-slate-400 border border-white/[0.04]"
                >
                  {configuredNetworks.length} {configuredNetworks.length === 1 ? 'Network' : 'Networks'}
                </span>
              </div>

              <div class="flex items-center gap-2">
                <button
                  type="button"
                  onclick={(e) => {
                    e.stopPropagation();
                    addNetwork();
                  }}
                  disabled={isDeploying}
                  class="flex items-center gap-1.5 px-2.5 py-1 rounded-lg text-xs font-medium bg-white/[0.05] hover:bg-white/[0.1] text-violet-300 border border-violet-500/20 transition cursor-pointer"
                >
                  <Plus class="w-3.5 h-3.5" />
                  <span>Add Network</span>
                </button>

                <div class="text-slate-400 hover:text-white p-1">
                  {#if isNetworksOpen}
                    <ChevronDown class="w-4 h-4" />
                  {:else}
                    <ChevronRight class="w-4 h-4" />
                  {/if}
                </div>
              </div>
            </div>

            {#if isNetworksOpen}
              <div class="p-4 border-t border-white/[0.06] space-y-3 bg-[#0a0b10]/40 animate-in fade-in duration-150">
                {#if configuredNetworks.length === 0}
                  <div
                    class="p-4 rounded-xl border border-dashed border-white/[0.08] text-center text-xs text-slate-500"
                  >
                    No custom networks defined (default Docker bridge network will be used). Click "Add Network" to attach host or custom networks.
                  </div>
                {:else}
                  <div class="space-y-2.5">
                    {#each configuredNetworks as net, idx (net.id)}
                      {@const isHostMatch = availableHostNetworks.includes(net.name.trim())}
                      <div
                        class="p-3 rounded-xl bg-[#0a0b10]/70 border border-white/[0.06] hover:border-white/[0.12] transition space-y-2"
                      >
                        <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-2.5">
                          <div class="flex items-center gap-2 flex-1">
                            <div class="space-y-1 flex-1">
                              <div class="flex items-center justify-between">
                                <span class="text-[10px] font-mono text-slate-400">Network Name</span>
                                {#if isHostMatch}
                                  <span class="text-[9px] font-mono px-1.5 py-0.2 rounded bg-cyan-500/10 text-cyan-400 border border-cyan-500/30">
                                    Host Network
                                  </span>
                                {/if}
                              </div>
                              <input
                                type="text"
                                list="host-network-options"
                                bind:value={net.name}
                                disabled={isDeploying}
                                placeholder="e.g. bridge, host, or custom_net"
                                class="w-full px-3 py-1.5 rounded-lg bg-[#12131c] border border-white/[0.08] focus:border-violet-500 focus:outline-none text-xs text-white font-mono"
                              />
                            </div>

                            <div class="space-y-1 w-28 shrink-0">
                              <span class="text-[10px] font-mono text-slate-400">Driver</span>
                              <select
                                bind:value={net.driver}
                                disabled={isDeploying || net.isExternal}
                                class="w-full px-2 py-1.5 rounded-lg bg-[#12131c] border border-white/[0.08] focus:border-violet-500 focus:outline-none text-xs text-white font-mono disabled:opacity-40"
                              >
                                <option value="bridge">bridge</option>
                                <option value="host">host</option>
                                <option value="overlay">overlay</option>
                                <option value="macvlan">macvlan</option>
                                <option value="none">none</option>
                              </select>
                            </div>

                            <div class="pt-4 flex items-center">
                              <label
                                class="flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg bg-[#12131c] border border-white/[0.08] text-xs text-slate-300 cursor-pointer select-none hover:border-white/[0.15] transition"
                                title="Treat as pre-existing external network"
                              >
                                <input
                                  type="checkbox"
                                  bind:checked={net.isExternal}
                                  disabled={isDeploying}
                                  class="rounded bg-[#0a0b10] border-white/20 text-violet-600 focus:ring-0 cursor-pointer"
                                />
                                <span class="text-[11px] font-mono">External</span>
                              </label>
                            </div>

                            <div class="pt-4">
                              <button
                                type="button"
                                onclick={() => removeNetwork(idx)}
                                disabled={isDeploying}
                                class="p-2 rounded-lg text-slate-500 hover:text-red-400 hover:bg-red-500/10 transition cursor-pointer"
                                title="Remove network"
                              >
                                <Trash class="w-3.5 h-3.5" />
                              </button>
                            </div>
                          </div>
                        </div>
                      </div>
                    {/each}
                  </div>
                {/if}
              </div>
            {/if}
          </div>

          <!-- 4. Environment Variables Collapsible Card -->
          <div class="rounded-xl border border-white/[0.08] bg-[#0c0d14] overflow-hidden transition-all duration-200">
            <div
              role="button"
              tabindex="0"
              aria-expanded={isEnvOpen}
              class="flex items-center justify-between p-3.5 bg-[#12131c]/60 hover:bg-[#12131c] cursor-pointer transition select-none"
              onclick={() => (isEnvOpen = !isEnvOpen)}
              onkeydown={(e) => {
                if (e.key === 'Enter' || e.key === ' ') {
                  e.preventDefault();
                  isEnvOpen = !isEnvOpen;
                }
              }}
            >
              <div class="flex items-center gap-2.5">
                <Sliders class="w-4 h-4 text-violet-400" />
                <h3 class="text-xs font-semibold text-slate-200 uppercase tracking-wider">
                  Environment Configuration
                </h3>
                <span
                  class="px-2 py-0.5 rounded-full text-[10px] font-mono bg-white/[0.05] text-slate-400 border border-white/[0.04]"
                >
                  {effectiveEnvList.length} {effectiveEnvList.length === 1 ? 'Variable' : 'Variables'}
                </span>
              </div>

              <div class="flex items-center gap-2">
                <button
                  type="button"
                  onclick={(e) => {
                    e.stopPropagation();
                    addCustomEnv();
                  }}
                  disabled={isDeploying}
                  class="flex items-center gap-1.5 px-2.5 py-1 rounded-lg text-xs font-medium bg-white/[0.05] hover:bg-white/[0.1] text-violet-300 border border-violet-500/20 transition cursor-pointer"
                >
                  <Plus class="w-3.5 h-3.5" />
                  <span>Add Variable</span>
                </button>

                <div class="text-slate-400 hover:text-white p-1">
                  {#if isEnvOpen}
                    <ChevronDown class="w-4 h-4" />
                  {:else}
                    <ChevronRight class="w-4 h-4" />
                  {/if}
                </div>
              </div>
            </div>

            {#if isEnvOpen}
              <div class="p-4 border-t border-white/[0.06] space-y-3 bg-[#0a0b10]/40 animate-in fade-in duration-150">
                {#if effectiveEnvList.length === 0}
                  <div
                    class="p-4 rounded-xl border border-dashed border-white/[0.08] text-center text-xs text-slate-500"
                  >
                    No environment variables defined for this template. Click "Add Variable" to set custom container environment variables.
                  </div>
                {:else}
                  <div class="space-y-2.5">
                    {#each effectiveEnvList as ev}
                      {@const isSensitive = isSensitiveField(ev.name, ev.label)}
                      {@const isShown = showSecret[ev.name]}
                      <div
                        class="p-3 rounded-xl bg-[#0a0b10]/70 border border-white/[0.06] hover:border-white/[0.12] transition space-y-1.5"
                      >
                        <div class="flex items-center justify-between">
                          <label
                            for="env-{ev.name}"
                            class="text-xs font-medium text-slate-200 flex items-center gap-1.5"
                          >
                            <span>{ev.label || ev.name}</span>
                            {#if ev.required}
                              <span class="text-red-400 text-xs" title="Required">*</span>
                            {/if}
                          </label>
                          <div class="flex items-center gap-2">
                            <span
                              class="text-[10px] font-mono px-2 py-0.5 rounded bg-white/[0.04] text-violet-300 border border-white/[0.05]"
                            >
                              {ev.name}
                            </span>
                            {#if ev.name.startsWith('CUSTOM_VAR_')}
                              <button
                                type="button"
                                onclick={() => removeCustomEnv(ev.name)}
                                class="p-1 rounded text-slate-500 hover:text-red-400 hover:bg-red-500/10 transition cursor-pointer"
                                title="Remove custom variable"
                              >
                                <Trash class="w-3 h-3" />
                              </button>
                            {/if}
                          </div>
                        </div>

                        {#if ev.description}
                          <p class="text-[11px] text-slate-500 leading-tight">
                            {ev.description}
                          </p>
                        {/if}

                        <div class="relative mt-1">
                          {#if isSensitive}
                            <input
                              id="env-{ev.name}"
                              type={isShown ? 'text' : 'password'}
                              bind:value={envValues[ev.name]}
                              required={ev.required}
                              disabled={isDeploying}
                              placeholder={ev.default || 'Enter value...'}
                              class="w-full pl-3 pr-10 py-2 rounded-lg bg-[#12131c] border border-white/[0.08] focus:border-violet-500 focus:outline-none text-xs text-white font-mono placeholder-slate-600 transition"
                            />
                            <button
                              type="button"
                              onclick={() => toggleSecret(ev.name)}
                              tabindex="-1"
                              class="absolute right-2 top-1/2 -translate-y-1/2 p-1 text-slate-500 hover:text-slate-300 transition cursor-pointer"
                              title={isShown ? 'Hide secret' : 'Show secret'}
                            >
                              {#if isShown}
                                <EyeOff class="w-3.5 h-3.5" />
                              {:else}
                                <Eye class="w-3.5 h-3.5" />
                              {/if}
                            </button>
                          {:else}
                            <input
                              id="env-{ev.name}"
                              type="text"
                              bind:value={envValues[ev.name]}
                              required={ev.required}
                              disabled={isDeploying}
                              placeholder={ev.default || 'Enter value...'}
                              class="w-full px-3 py-2 rounded-lg bg-[#12131c] border border-white/[0.08] focus:border-violet-500 focus:outline-none text-xs text-white font-mono placeholder-slate-600 transition"
                            />
                          {/if}
                        </div>
                      </div>
                    {/each}
                  </div>
                {/if}
              </div>
            {/if}
          </div>

          <!-- Quick Jump to YAML Section -->
          <div class="pt-2">
            <button
              type="button"
              onclick={() => (activeTab = 'yaml')}
              class="w-full flex items-center justify-between p-3 rounded-xl border border-white/[0.06] bg-[#0c0d14] hover:bg-[#12131f] hover:border-violet-500/30 transition text-xs text-slate-300 cursor-pointer"
            >
              <div class="flex items-center gap-2">
                <Code class="w-4 h-4 text-violet-400" />
                <span>View & Edit Docker Compose YAML Specification</span>
              </div>
              <ChevronRight class="w-4 h-4 text-slate-500" />
            </button>
          </div>
        </form>
      {/if}

      <!-- TAB 2: FULL COMPOSE YAML VIEW & EDITOR -->
      {#if activeTab === 'yaml'}
        <div class="space-y-3 h-full flex flex-col animate-in fade-in duration-150">
          <div class="flex items-center justify-between bg-[#0c0d14] p-3 rounded-xl border border-white/[0.06]">
            <div class="flex items-center gap-2">
              <Code class="w-4 h-4 text-violet-400" />
              <span class="text-xs font-semibold text-white">Docker Compose Definition</span>
              {#if isEditingYaml}
                <span
                  class="text-[10px] font-mono px-2 py-0.5 rounded bg-amber-500/10 text-amber-300 border border-amber-500/30"
                >
                  Direct Edit Active
                </span>
              {/if}
            </div>

            <div class="flex items-center gap-2">
              {#if isEditingYaml}
                <button
                  type="button"
                  onclick={resetYamlFromForm}
                  class="flex items-center gap-1 px-2.5 py-1 rounded-lg text-xs font-mono bg-white/[0.05] hover:bg-white/[0.1] text-slate-300 transition cursor-pointer"
                  title="Discard manual edits and re-generate from form"
                >
                  <RotateCcw class="w-3.5 h-3.5 text-slate-400" />
                  <span>Sync Form</span>
                </button>
              {/if}

              <button
                type="button"
                onclick={toggleYamlEdit}
                class="flex items-center gap-1.5 px-3 py-1 rounded-lg text-xs font-mono transition cursor-pointer {isEditingYaml
                  ? 'bg-violet-600/30 text-violet-300 border border-violet-500/40'
                  : 'bg-white/[0.05] hover:bg-white/[0.1] text-slate-300'}"
              >
                <Pencil class="w-3.5 h-3.5" />
                <span>{isEditingYaml ? 'Lock YAML' : 'Edit YAML'}</span>
              </button>

              <button
                type="button"
                onclick={copyYaml}
                class="flex items-center gap-1.5 px-3 py-1 rounded-lg text-xs font-mono bg-white/[0.05] hover:bg-white/[0.1] text-slate-300 transition cursor-pointer"
              >
                {#if copiedYaml}
                  <Check class="w-3.5 h-3.5 text-emerald-400" />
                  <span class="text-emerald-400">Copied</span>
                {:else}
                  <Copy class="w-3.5 h-3.5 text-slate-400" />
                  <span>Copy</span>
                {/if}
              </button>
            </div>
          </div>

          <div class="flex-1 min-h-[360px] flex flex-col">
            {#if isEditingYaml}
              <textarea
                bind:value={manualYamlText}
                class="w-full flex-1 p-4 rounded-xl bg-[#090a10] border border-violet-500/40 text-xs font-mono text-violet-100 focus:outline-none focus:border-violet-400 leading-relaxed resize-none shadow-inner"
                rows="18"
                placeholder="Enter custom docker-compose.yml definition..."
              ></textarea>
              <p class="text-[11px] text-amber-400/80 mt-2">
                Custom YAML is active. Deployment will use this exact specification. Click "Sync Form" to discard manual edits.
              </p>
            {:else}
              <pre
                class="w-full flex-1 p-4 rounded-xl bg-[#090a10] border border-white/[0.08] text-xs font-mono text-slate-300 overflow-auto leading-relaxed select-all shadow-inner"><code>{finalEffectiveYaml}</code></pre>
            {/if}
          </div>
        </div>
      {/if}

    </div>

    <!-- Modal Footer Actions -->
    <div
      class="flex items-center justify-between p-4 px-5 border-t border-white/[0.08] bg-[#181a26]/40 shrink-0"
    >
      <div class="text-xs text-slate-500 font-mono">
        {#if hasPortConflict && activeTab === 'config'}
          <span class="text-amber-400">Host port conflict detected.</span>
        {/if}
      </div>

      <div class="flex items-center gap-3">
        <button
          type="button"
          onclick={onClose}
          class="px-4 py-2 rounded-xl text-xs font-semibold text-slate-400 hover:text-white hover:bg-white/5 transition cursor-pointer"
        >
          Cancel
        </button>

        <button
          type="button"
          onclick={handleDeploy}
          disabled={hasPortConflict}
          class="flex items-center gap-2 px-5 py-2 rounded-xl bg-violet-600 hover:bg-violet-500 disabled:bg-violet-600/40 text-white text-xs font-semibold shadow-[0_0_20px_rgba(124,58,237,0.4)] transition cursor-pointer active:scale-95 disabled:pointer-events-none"
        >
          <Rocket class="w-4 h-4" />
          <span>Deploy Application</span>
        </button>
      </div>
    </div>
  </div>
</div>
