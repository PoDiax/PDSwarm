<script lang="ts">
  import {
    onMount } from "svelte";
  import {
    ArrowLeft,
    Play,
    Save,
    RotateCw,
    Layers,
    Box,
    CircleCheck,
    CircleAlert,
    LoaderCircle,
    Sparkles,
    Code,
    ChevronDown,
    ShieldAlert,
    TriangleAlert,
    RefreshCw,
    FileCode,
    FileKey,
    Copy,
    Trash,
    Check,
    Key,
  } from '@lucide/svelte';
  import YamlEditor from "../lib/components/YamlEditor.svelte";
  import { stackApi } from "../lib/api";
  import { tasksStore } from "../lib/stores/tasks.svelte";
  import type { Stack } from "../lib/types";
  import {
    detectRelativeVolumes,
    resolveRelativeVolumesInYaml,
    getStackComposeDir,
  } from "../lib/utils/compose";

  interface Props {
    stackName?: string | null;
    onClose: () => void;
    onDeployed?: (name: string) => void;
  }

  let { stackName = null, onClose, onDeployed }: Props = $props();

  let isEditing = $derived(!!stackName);

  // Form State
  let name = $state("");
  let composeYaml = $state("");
  let stackEnv = $state("");
  let activeEditorTab = $state<"compose" | "env">("compose");
  let isYamlValid = $state(true);
  let yamlError = $state<string | null>(null);

  let parsedEnvMap = $derived.by<Record<string, string>>(() => {
    const map: Record<string, string> = {};
    const lines = stackEnv.split("\n");
    for (const rawLine of lines) {
      const line = rawLine.trim();
      if (!line || line.startsWith("#")) continue;
      const clean = line.startsWith("export ") ? line.slice(7).trim() : line;
      const idx = clean.indexOf("=");
      if (idx !== -1) {
        const k = clean.slice(0, idx).trim();
        const v = clean.slice(idx + 1).trim();
        if (k) map[k] = v;
      }
    }
    return map;
  });

  let envVarCount = $derived(Object.keys(parsedEnvMap).length);
  let envVarEntries = $derived(Object.entries(parsedEnvMap));

  $effect(() => {
    if (stackName) {
      name = stackName;
    }
  });

  // Loading & Submission State
  let loadedStack = $state<Stack | null>(null);
  let isLoadingStack = $state(false);
  let isDeploying = $state(false);
  let isSaving = $state(false);
  let isExternalStack = $state(false);
  let submitError = $state<string | null>(null);
  let successNotice = $state<string | null>(null);

  let detectedRelPaths = $derived(detectRelativeVolumes(composeYaml));
  let composeDir = $derived(getStackComposeDir(loadedStack));

  // Studio inspector panel
  let isInspectorOpen = $state(true);

  // Starter Compose Templates
  interface Template {
    id: string;
    name: string;
    description: string;
    icon: string;
    yaml: string;
  }

  const templates: Template[] = [
    {
      id: "blank",
      name: "Blank Template",
      description: "Minimal starter compose skeleton",
      icon: "Layers",
      yaml: `services:
  app:
    image: alpine:latest
    command: ["sh", "-c", "echo 'PDSwarm ready' && sleep 3600"]
    restart: unless-stopped
`,
    },
    {
      id: "nginx",
      name: "Nginx Web Server",
      description: "High-performance HTTP & reverse proxy",
      icon: "Box",
      yaml: `services:
  web:
    image: nginx:alpine
    ports:
      - "80:80"
    restart: unless-stopped
    environment:
      - NGINX_PORT=80
`,
    },
    {
      id: "redis",
      name: "Redis Cache",
      description: "In-memory key-value data store",
      icon: "Cpu",
      yaml: `services:
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    command: redis-server --appendonly yes
    restart: unless-stopped
`,
    },
    {
      id: "postgres",
      name: "PostgreSQL Database",
      description: "PostgreSQL 16 relational database",
      icon: "HardDrive",
      yaml: `services:
  db:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: pdswarm
      POSTGRES_PASSWORD: pdswarm_secure_password
      POSTGRES_DB: pdswarm_db
    ports:
      - "5432:5432"
    restart: unless-stopped
`,
    },
    {
      id: "node-mongo",
      name: "Node.js + MongoDB",
      description: "Full-stack Node.js microservice with Mongo",
      icon: "Layers",
      yaml: `services:
  api:
    image: node:20-alpine
    command: sh -c "echo 'Node API started on port 3000' && sleep infinity"
    ports:
      - "3000:3000"
    environment:
      - MONGO_URI=mongodb://db:27017/app
    depends_on:
      - db
    restart: unless-stopped

  db:
    image: mongo:7.0
    ports:
      - "27017:27017"
    restart: unless-stopped
`,
    },
    {
      id: "lemp",
      name: "LEMP Web Stack",
      description: "Nginx + PHP 8.3 FPM + MariaDB",
      icon: "Box",
      yaml: `services:
  nginx:
    image: nginx:alpine
    ports:
      - "80:80"
    depends_on:
      - php
    restart: unless-stopped

  php:
    image: php:8.3-fpm-alpine
    restart: unless-stopped

  db:
    image: mariadb:11
    environment:
      MARIADB_ROOT_PASSWORD: secret_root_pass
      MARIADB_DATABASE: lemp_app
    ports:
      - "3306:3306"
    restart: unless-stopped
`,
    },
    {
      id: "wordpress",
      name: "WordPress + MySQL",
      description: "WordPress CMS with MySQL database",
      icon: "ExternalLink",
      yaml: `services:
  wordpress:
    image: wordpress:latest
    ports:
      - "8080:80"
    environment:
      WORDPRESS_DB_HOST: db
      WORDPRESS_DB_USER: wordpress
      WORDPRESS_DB_PASSWORD: wordpress_password
      WORDPRESS_DB_NAME: wordpress
    depends_on:
      - db
    restart: unless-stopped

  db:
    image: mysql:8.0
    environment:
      MYSQL_DATABASE: wordpress
      MYSQL_USER: wordpress
      MYSQL_PASSWORD: wordpress_password
      MYSQL_RANDOM_ROOT_PASSWORD: "1"
    restart: unless-stopped
`,
    },
  ];

  let selectedTemplateId = $state("blank");

  function applyTemplate(tplId: string) {
    const tpl = templates.find((t) => t.id === tplId);
    if (!tpl) return;
    selectedTemplateId = tplId;
    composeYaml = tpl.yaml;
    submitError = null;
  }

  // Parse Compose YAML on the fly for Inspector Panel
  interface ParsedService {
    name: string;
    image: string;
    ports: string[];
    restart?: string;
  }

  let parsedServices = $derived.by<ParsedService[]>(() => {
    if (!composeYaml.trim()) return [];
    const results: ParsedService[] = [];
    const lines = composeYaml.split("\n");

    let inServices = false;
    let currentService: ParsedService | null = null;
    let inPorts = false;

    for (let i = 0; i < lines.length; i++) {
      const line = lines[i];
      const trimmed = line.trim();

      if (!trimmed || trimmed.startsWith("#")) continue;

      // Detect "services:"
      if (/^services:\s*$/.test(line)) {
        inServices = true;
        currentService = null;
        inPorts = false;
        continue;
      }

      // Exit services if other top-level block starts
      if (
        inServices &&
        /^[a-zA-Z0-9_\-]+:\s*$/.test(line) &&
        !line.startsWith(" ")
      ) {
        inServices = false;
        if (currentService) results.push(currentService);
        currentService = null;
        inPorts = false;
        continue;
      }

      if (inServices) {
        // Detect service name (indented by 2 spaces)
        const serviceMatch = line.match(/^  ([a-zA-Z0-9_\-\.]+):\s*$/);
        if (serviceMatch) {
          if (currentService) results.push(currentService);
          currentService = {
            name: serviceMatch[1],
            image: "unspecified",
            ports: [],
          };
          inPorts = false;
          continue;
        }

        if (currentService) {
          // Detect image
          const imageMatch = line.match(/^\s+image:\s*(["']?)([^"'\s]+)\1/);
          if (imageMatch) {
            currentService.image = imageMatch[2];
            inPorts = false;
            continue;
          }

          // Detect ports section
          if (/^\s+ports:\s*$/.test(line)) {
            inPorts = true;
            continue;
          }

          if (inPorts) {
            const portItemMatch = line.match(/^\s+-\s*(["']?)([^"'\s]+)\1/);
            if (portItemMatch) {
              currentService.ports.push(portItemMatch[2]);
              continue;
            } else if (!line.startsWith("    -")) {
              inPorts = false;
            }
          }

          // Detect restart
          const restartMatch = line.match(/^\s+restart:\s*([a-zA-Z0-9_\-]+)/);
          if (restartMatch) {
            currentService.restart = restartMatch[1];
          }
        }
      }
    }

    if (currentService) results.push(currentService);
    return results;
  });

  // Name validation
  let isNameValid = $derived.by(() => {
    const trimmed = name.trim();
    if (!trimmed) return false;
    // Lowercase letters, numbers, hyphens and underscores
    return /^[a-z0-9][a-z0-9_.-]*$/i.test(trimmed);
  });

  onMount(async () => {
    if (stackName) {
      isLoadingStack = true;
      try {
        const stack = await stackApi.get(stackName);
        if (stack) {
          loadedStack = stack;
          name = stack.name;
          composeYaml = stack.compose_yaml;
          stackEnv = stack.env || "";
          isExternalStack = !!stack.is_external;
        }
      } catch (err: unknown) {
        submitError =
          err instanceof Error ? err.message : "Failed to load stack details.";
      } finally {
        isLoadingStack = false;
      }
    } else {
      // Default to Blank template
      applyTemplate("blank");
    }
  });

  // Modal State for Redeploy Confirmation & Image Pulling
  let isRedeployModalOpen = $state(false);
  let pullLatestImages = $state(true);

  function onRedeployClick() {
    if (isEditing) {
      isRedeployModalOpen = true;
    } else {
      handleDeploy(true, false);
    }
  }

  // Save or Deploy Action
  async function handleDeploy(shouldDeploy = true, pullImages = false) {
    const trimmedName = name.trim();
    if (!trimmedName) {
      submitError = "Stack name is required.";
      return;
    }

    if (!isNameValid) {
      submitError =
        "Stack name must contain only alphanumeric characters, hyphens, or underscores.";
      return;
    }

    if (!composeYaml.trim()) {
      submitError = "Compose YAML cannot be empty.";
      return;
    }

    if (!isYamlValid) {
      submitError = `Please fix YAML syntax issues before deploying: ${yamlError || "Invalid YAML"}`;
      return;
    }

    submitError = null;
    successNotice = null;

    if (shouldDeploy) {
      isDeploying = true;
    } else {
      isSaving = true;
    }

    try {
      if (isEditing) {
        await stackApi.update(trimmedName, composeYaml, shouldDeploy, stackEnv, pullImages);
      } else {
        await stackApi.create(trimmedName, composeYaml, shouldDeploy, stackEnv, pullImages);
      }

      if (shouldDeploy) {
        tasksStore.poll();
      }

      successNotice = shouldDeploy
        ? `Stack "${trimmedName}" successfully deployed!`
        : `Stack "${trimmedName}" saved as draft.`;

      setTimeout(() => {
        if (onDeployed) {
          onDeployed(trimmedName);
        } else {
          onClose();
        }
      }, 700);
    } catch (err: unknown) {
      if (err instanceof Error) {
        submitError = err.message;
      } else {
        submitError = "Failed to process stack request.";
      }
    } finally {
      isDeploying = false;
      isSaving = false;
    }
  }

  // Keyboard shortcut: Ctrl+S or Cmd+S to deploy
  function handleGlobalKeyDown(e: KeyboardEvent) {
    if ((e.ctrlKey || e.metaKey) && e.key === "s") {
      e.preventDefault();
      onRedeployClick();
    }
  }

  // Quick insertion helpers
  function insertSnippet(snippet: string) {
    composeYaml =
      composeYaml + (composeYaml.endsWith("\n") ? "" : "\n") + snippet + "\n";
  }

  function insertEnvSnippet(snippet: string) {
    stackEnv =
      stackEnv + (stackEnv.endsWith("\n") || !stackEnv ? "" : "\n") + snippet + "\n";
  }
</script>

<svelte:window onkeydown={handleGlobalKeyDown} />

<div class="space-y-5 animate-in fade-in duration-200">
  <!-- Top Breadcrumb & Action Bar -->
  <div
    class="flex flex-wrap items-center justify-between gap-4 p-4 rounded-2xl bg-[#12131c] border border-white/[0.08] shadow-lg"
  >
    <div class="flex items-center gap-3">
      <button
        onclick={onClose}
        class="p-2 rounded-xl bg-white/[0.05] hover:bg-white/[0.1] text-slate-300 hover:text-white border border-white/[0.08] transition"
        title="Return to Stacks list"
      >
        <ArrowLeft class="w-5 h-5" />
      </button>

      <div class="flex items-center gap-2.5">
        <div
          class="w-9 h-9 rounded-xl bg-violet-600/20 border border-violet-500/30 flex items-center justify-center text-violet-400 shadow-[0_0_15px_rgba(124,58,237,0.3)]"
        >
          <Layers class="w-5 h-5" />
        </div>
        <div>
          <div class="flex items-center gap-2">
            <h1 class="text-base font-bold text-white tracking-wide">
              {isEditing ? `Edit Stack: ${stackName}` : "Compose Studio"}
            </h1>
            <span
              class="px-2 py-0.5 text-[10px] font-mono uppercase tracking-wider rounded bg-cyan-500/10 text-cyan-400 border border-cyan-500/20"
            >
              {isEditing ? "UPDATE MODE" : "CREATE MODE"}
            </span>
          </div>
          <p class="text-xs text-slate-400">
            {isEditing
              ? "Modify Compose definition and redeploy container services"
              : "Design and deploy multi-container Docker applications"}
          </p>
        </div>
      </div>
    </div>

    <!-- Action Buttons -->
    <div class="flex items-center gap-2.5">
      <button
        onclick={onClose}
        class="px-4 py-2 rounded-xl bg-white/[0.05] hover:bg-white/[0.1] text-slate-300 hover:text-white border border-white/[0.08] text-xs font-semibold transition"
      >
        Cancel
      </button>

      <button
        onclick={() => handleDeploy(false)}
        disabled={isSaving ||
          isDeploying ||
          !isNameValid ||
          !composeYaml.trim()}
        class="flex items-center gap-1.5 px-4 py-2 rounded-xl bg-white/[0.08] hover:bg-white/[0.14] text-slate-200 border border-white/[0.1] text-xs font-semibold transition disabled:opacity-50 disabled:cursor-not-allowed"
        title="Save configuration without deploying containers"
      >
        {#if isSaving}
          <LoaderCircle class="w-3.5 h-3.5 animate-spin text-cyan-400" />
          <span>Saving...</span>
        {:else}
          <Save class="w-3.5 h-3.5 text-cyan-400" />
          <span>Save Draft</span>
        {/if}
      </button>

      <button
        onclick={onRedeployClick}
        disabled={isSaving ||
          isDeploying ||
          !isNameValid ||
          !composeYaml.trim() ||
          !isYamlValid}
        class="flex items-center gap-2 px-5 py-2 rounded-xl bg-gradient-to-r from-violet-600 to-indigo-600 hover:from-violet-500 hover:to-indigo-500 text-white text-xs font-semibold shadow-[0_0_20px_rgba(124,58,237,0.4)] border border-violet-400/30 transition disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
      >
        {#if isDeploying}
          <LoaderCircle class="w-4 h-4 animate-spin" />
          <span>Deploying Stack...</span>
        {:else if isEditing}
          <RotateCw class="w-4 h-4 text-cyan-300" />
          <span>Redeploy Stack</span>
        {:else}
          <Play class="w-4 h-4 text-cyan-300 fill-cyan-300" />
          <span>Deploy Stack</span>
        {/if}
      </button>
    </div>
  </div>

  <!-- Alerts / Notices -->
  {#if submitError}
    <div
      class="p-3.5 rounded-xl bg-rose-500/15 border border-rose-500/30 text-rose-300 flex items-start gap-3 text-xs animate-in fade-in"
    >
      <CircleAlert class="w-4 h-4 text-rose-400 shrink-0 mt-0.5" />
      <div class="flex-1">
        <span class="font-bold">Deploy Error: </span>
        <span>{submitError}</span>
      </div>
      <button
        onclick={() => (submitError = null)}
        class="text-rose-400 hover:text-white text-sm font-bold">&times;</button
      >
    </div>
  {/if}

  {#if successNotice}
    <div
      class="p-3.5 rounded-xl bg-emerald-500/15 border border-emerald-500/30 text-emerald-300 flex items-center gap-2.5 text-xs animate-in fade-in"
    >
      <CircleCheck class="w-4 h-4 text-emerald-400 shrink-0" />
      <span>{successNotice}</span>
    </div>
  {/if}

  {#if isExternalStack}
    <div
      class="p-4 rounded-xl bg-amber-500/10 border border-amber-500/30 text-amber-300 flex items-start gap-3 text-xs animate-in fade-in"
    >
      <ShieldAlert class="w-5 h-5 text-amber-400 shrink-0 mt-0.5" />
      <div class="space-y-0.5">
        <p class="font-bold text-amber-200">External Compose Stack</p>
        <p class="text-slate-300">
          This stack was originally deployed outside PDSwarm. Saving or
          redeploying this stack in Compose Studio will automatically adopt it
          into full PDSwarm management, preserving all data and running
          containers.
        </p>
      </div>
    </div>
  {/if}

  {#if detectedRelPaths.length > 0}
    <div
      class="p-4 rounded-xl bg-amber-500/10 border border-amber-500/30 text-amber-300 flex flex-col sm:flex-row sm:items-center justify-between gap-3 text-xs animate-in fade-in"
    >
      <div class="flex items-start gap-3">
        <TriangleAlert class="w-5 h-5 text-amber-400 shrink-0 mt-0.5" />
        <div class="space-y-0.5">
          <p class="font-bold text-amber-200">
            Relative Volume Mounts Detected ({detectedRelPaths.length})
          </p>
          <p class="text-slate-300 leading-relaxed">
            This Compose file contains relative host paths: <code
              class="font-mono text-amber-300"
              >{detectedRelPaths.join(", ")}</code
            >.
            {#if composeDir}
              PDSwarm can resolve them to absolute host paths based on <code
                class="font-mono text-cyan-300">{composeDir}</code
              > so volume mounts won't fail when deployed.
            {:else}
              If deployed, Docker requires an absolute host path or a configured
              working directory to prevent volume creation errors.
            {/if}
          </p>
        </div>
      </div>
      {#if composeDir}
        <button
          type="button"
          onclick={() => {
            const count = detectedRelPaths.length;
            composeYaml = resolveRelativeVolumesInYaml(composeYaml, composeDir);
            successNotice = `Converted ${count} relative volume path(s) to absolute paths under ${composeDir}`;
          }}
          class="shrink-0 px-3.5 py-1.5 rounded-lg bg-amber-500/20 hover:bg-amber-500/30 text-amber-200 border border-amber-500/40 text-xs font-semibold flex items-center gap-1.5 transition cursor-pointer"
        >
          <RefreshCw class="w-3.5 h-3.5" />
          <span>Convert to Absolute</span>
        </button>
      {/if}
    </div>
  {/if}

  <!-- Stack Name & Template Selection Configuration Bar -->
  <div
    class="grid grid-cols-1 md:grid-cols-12 gap-4 p-4 rounded-2xl bg-[#12131c] border border-white/[0.08]"
  >
    <!-- Stack Name Input -->
    <div class="md:col-span-6 space-y-1.5">
      <label
        for="stack-name-input"
        class="flex items-center justify-between text-xs font-medium text-slate-300"
      >
        <span class="flex items-center gap-1.5">
          <Layers class="w-3.5 h-3.5 text-cyan-400" />
          <span>Stack Identifier</span>
        </span>
        {#if name}
          <span
            class="font-mono text-[10px] {isNameValid
              ? 'text-emerald-400'
              : 'text-rose-400'}"
          >
            {isNameValid ? "Valid name" : "Lowercase, 0-9, -, _ only"}
          </span>
        {/if}
      </label>
      <input
        id="stack-name-input"
        type="text"
        bind:value={name}
        disabled={isEditing}
        placeholder="e.g. web-app, monitoring-stack, redis-cluster"
        class="w-full px-3.5 py-2.5 rounded-xl bg-[#090a10] border {name &&
        !isNameValid
          ? 'border-rose-500/50 focus:border-rose-500'
          : 'border-white/[0.1] focus:border-violet-500'} text-white font-mono text-xs focus:outline-none transition disabled:opacity-60 disabled:cursor-not-allowed"
      />
    </div>

    <!-- Template Dropdown -->
    <div class="md:col-span-6 space-y-1.5">
      <label
        for="template-select"
        class="flex items-center justify-between text-xs font-medium text-slate-300"
      >
        <span class="flex items-center gap-1.5">
          <Sparkles class="w-3.5 h-3.5 text-violet-400" />
          <span>Load Compose Starter Template</span>
        </span>
        <span class="text-[10px] text-slate-500">Preset Blueprints</span>
      </label>
      <div class="relative">
        <select
          id="template-select"
          bind:value={selectedTemplateId}
          onchange={(e) => applyTemplate((e.target as HTMLSelectElement).value)}
          class="w-full appearance-none px-3.5 py-2.5 rounded-xl bg-[#090a10] border border-white/[0.1] text-white text-xs focus:outline-none focus:border-violet-500 transition pr-9"
        >
          {#each templates as tpl}
            <option value={tpl.id} class="bg-[#12131c] text-white">
              {tpl.name} &mdash; {tpl.description}
            </option>
          {/each}
        </select>
        <ChevronDown
          class="w-4 h-4 text-slate-400 absolute right-3 top-1/2 -translate-y-1/2 pointer-events-none"
        />
      </div>
    </div>
  </div>

  <!-- Studio Workspace: Code Editor + Real-time Inspector -->
  <div class="grid grid-cols-1 lg:grid-cols-12 gap-5">
    <!-- Main Code Editor Column -->
    <div class="lg:col-span-8 flex flex-col space-y-3">
      <!-- File Switcher Tabs: Compose YAML vs .env -->
      <div
        class="flex flex-wrap items-center justify-between gap-2 p-1.5 rounded-xl bg-[#12131c] border border-white/[0.08]"
      >
        <div class="flex items-center gap-1.5">
          <button
            type="button"
            onclick={() => (activeEditorTab = "compose")}
            class="flex items-center gap-2 px-3.5 py-1.5 rounded-lg text-xs transition cursor-pointer {activeEditorTab ===
            'compose'
              ? 'bg-cyan-500/15 text-cyan-300 border border-cyan-500/30 font-semibold shadow-sm'
              : 'text-slate-400 hover:text-slate-200 hover:bg-white/[0.04]'}"
          >
            <FileCode class="w-3.5 h-3.5 text-cyan-400" />
            <span>docker-compose.yml</span>
          </button>

          <button
            type="button"
            onclick={() => (activeEditorTab = "env")}
            class="flex items-center gap-2 px-3.5 py-1.5 rounded-lg text-xs transition cursor-pointer {activeEditorTab ===
            'env'
              ? 'bg-amber-500/15 text-amber-300 border border-amber-500/30 font-semibold shadow-sm'
              : 'text-slate-400 hover:text-slate-200 hover:bg-white/[0.04]'}"
          >
            <FileKey class="w-3.5 h-3.5 text-amber-400" />
            <span>.env (Environment Variables)</span>
            {#if envVarCount > 0}
              <span
                class="px-1.5 py-0.5 rounded-full text-[10px] font-mono bg-amber-500/25 text-amber-300 border border-amber-500/40 font-bold"
              >
                {envVarCount}
              </span>
            {/if}
          </button>
        </div>

        <div class="text-[11px] font-mono text-slate-500 pr-2 hidden sm:block">
          {#if activeEditorTab === "compose"}
            <span>Variables like <code>${"{VAR:-default}"}</code> are interpolated from .env</span>
          {:else}
            <span>Define values for <code>${"{VAR}"}</code> references in compose</span>
          {/if}
        </div>
      </div>

      {#if isLoadingStack}
        <div
          class="h-96 rounded-2xl bg-[#12131c] border border-white/[0.08] flex items-center justify-center text-slate-400 gap-3"
        >
          <LoaderCircle class="w-6 h-6 animate-spin text-violet-400" />
          <span class="text-xs font-mono">Loading stack compose file...</span>
        </div>
      {:else if activeEditorTab === "compose"}
        <YamlEditor
          bind:value={composeYaml}
          minHeight="540px"
          onValidate={(valid, err) => {
            isYamlValid = valid;
            yamlError = err;
          }}
        />
      {:else}
        <!-- .env Editor Card -->
        <div
          class="flex flex-col rounded-2xl bg-[#0e1017] border border-white/[0.08] shadow-2xl overflow-hidden"
        >
          <div
            class="flex flex-wrap items-center justify-between gap-3 px-4 py-2.5 bg-[#090a10] border-b border-white/[0.08] text-xs select-none"
          >
            <div class="flex items-center gap-3">
              <div class="flex items-center gap-1.5 text-slate-300 font-medium">
                <FileKey class="w-4 h-4 text-amber-400" />
                <span class="font-mono text-amber-300 font-semibold tracking-wide"
                  >Stack Environment (.env)</span
                >
              </div>
              <span class="w-1 h-3 rounded-full bg-white/10"></span>
              <div
                class="flex items-center gap-2 font-mono text-[11px] text-slate-400"
              >
                <span>{envVarCount} {envVarCount === 1 ? "variable" : "variables"} defined</span>
              </div>
            </div>

            <div class="flex items-center gap-2">
              <button
                type="button"
                onclick={() => {
                  navigator.clipboard.writeText(stackEnv);
                }}
                class="flex items-center gap-1 px-2.5 py-1 rounded-lg bg-white/[0.04] hover:bg-white/[0.08] text-slate-300 hover:text-white border border-white/[0.06] transition text-[11px] cursor-pointer"
              >
                <Copy class="w-3 h-3" />
                <span>Copy .env</span>
              </button>
              <button
                type="button"
                onclick={() => {
                  stackEnv = "";
                }}
                class="p-1 rounded-lg bg-white/[0.04] hover:bg-rose-500/20 text-slate-400 hover:text-rose-300 border border-white/[0.06] transition cursor-pointer"
                title="Clear environment variables"
              >
                <Trash class="w-3.5 h-3.5" />
              </button>
            </div>
          </div>

          <div class="relative flex flex-1 p-3 bg-[#0e1017]">
            <textarea
              bind:value={stackEnv}
              placeholder="# Define stack environment variables (.env) here&#10;# Syntax: KEY=VALUE&#10;# Example:&#10;PUID=1000&#10;PGID=1000&#10;DB_PASSWORD=secretpassword&#10;DB_HOSTNAME=database&#10;UPLOAD_LOCATION=/var/lib/immich&#10;IMMICH_VERSION=release"
              class="w-full font-mono text-xs text-amber-100 bg-transparent resize-y outline-none border-none p-3 leading-relaxed selection:bg-amber-500/30 placeholder:text-slate-600 focus:ring-0"
              style="min-height: 480px;"
              spellcheck="false"
            ></textarea>
          </div>
        </div>
      {/if}

      <!-- Keyboard Shortcuts & Hints -->
      <div
        class="flex flex-wrap items-center justify-between gap-3 px-4 py-2 rounded-xl bg-[#12131c] border border-white/[0.06] text-[11px] text-slate-400"
      >
        <div class="flex items-center gap-3">
          <span
            ><kbd
              class="px-1.5 py-0.5 rounded bg-white/10 font-mono text-slate-300"
              >Tab</kbd
            > Indent 2sp</span
          >
          <span
            ><kbd
              class="px-1.5 py-0.5 rounded bg-white/10 font-mono text-slate-300"
              >Shift+Tab</kbd
            > Outdent</span
          >
          <span
            ><kbd
              class="px-1.5 py-0.5 rounded bg-white/10 font-mono text-slate-300"
              >Ctrl+S</kbd
            > Deploy</span
          >
        </div>
        <span class="text-slate-500 font-mono">PDSwarm Compose Studio</span>
      </div>
    </div>

    <!-- Right Side: Real-time Service Inspector & Blueprint Helpers -->
    <div class="lg:col-span-4 space-y-4">
      {#if activeEditorTab === "compose"}
        <!-- Live Service Inspector Card -->
        <div
          class="p-4 rounded-2xl bg-[#12131c] border border-white/[0.08] shadow-lg space-y-3"
        >
          <div
            class="flex items-center justify-between border-b border-white/[0.08] pb-2.5"
          >
            <div class="flex items-center gap-2">
              <Box class="w-4 h-4 text-cyan-400" />
              <span class="text-xs font-bold text-white tracking-wide"
                >Live Services Inspector</span
              >
            </div>
            <span
              class="px-2 py-0.5 rounded-full text-[10px] font-mono bg-violet-500/20 text-violet-300 border border-violet-500/30"
            >
              {parsedServices.length}
              {parsedServices.length === 1 ? "service" : "services"}
            </span>
          </div>

          {#if parsedServices.length === 0}
            <div class="py-6 text-center text-slate-500 text-xs">
              <Code class="w-8 h-8 mx-auto text-slate-600 mb-2" />
              <p>No services detected in compose file.</p>
              <p class="text-[11px] text-slate-600 mt-0.5">
                Define a <code class="text-violet-400">services:</code> block to view
                inspector.
              </p>
            </div>
          {:else}
            <div class="space-y-2.5 max-h-[380px] overflow-y-auto pr-1">
              {#each parsedServices as svc}
                <div
                  class="p-3 rounded-xl bg-[#090a10] border border-white/[0.06] space-y-2"
                >
                  <div class="flex items-center justify-between">
                    <span class="font-mono text-xs font-bold text-cyan-300"
                      >{svc.name}</span
                    >
                    {#if svc.restart}
                      <span
                        class="text-[10px] font-mono text-slate-400 bg-white/[0.04] px-1.5 py-0.5 rounded"
                      >
                        {svc.restart}
                      </span>
                    {/if}
                  </div>

                  <div
                    class="text-[11px] font-mono text-slate-400 truncate flex items-center gap-1.5"
                  >
                    <span class="text-slate-500">Image:</span>
                    <span class="text-white truncate">{svc.image}</span>
                  </div>

                  {#if svc.ports.length > 0}
                    <div class="flex flex-wrap gap-1.5 pt-1">
                      {#each svc.ports as port}
                        <span
                          class="px-1.5 py-0.5 rounded text-[10px] font-mono bg-cyan-500/10 text-cyan-300 border border-cyan-500/20"
                        >
                          {port}
                        </span>
                      {/each}
                    </div>
                  {/if}
                </div>
              {/each}
            </div>
          {/if}
        </div>

        <!-- Quick Snippets Insertion Card -->
        <div
          class="p-4 rounded-2xl bg-[#12131c] border border-white/[0.08] shadow-lg space-y-3"
        >
          <div
            class="flex items-center justify-between border-b border-white/[0.08] pb-2.5"
          >
            <div class="flex items-center gap-2">
              <Code class="w-4 h-4 text-violet-400" />
              <span class="text-xs font-bold text-white tracking-wide"
                >Quick Snippets</span
              >
            </div>
            <span class="text-[10px] text-slate-500">Click to append</span>
          </div>

          <div class="grid grid-cols-2 gap-2 text-xs">
            <button
              onclick={() => insertSnippet("    restart: unless-stopped")}
              class="px-2.5 py-1.5 rounded-lg bg-white/[0.04] hover:bg-white/[0.08] border border-white/[0.06] text-left text-slate-300 hover:text-white transition font-mono text-[11px] truncate cursor-pointer"
            >
              + restart policy
            </button>
            <button
              onclick={() => insertSnippet('    ports:\n      - "8080:80"')}
              class="px-2.5 py-1.5 rounded-lg bg-white/[0.04] hover:bg-white/[0.08] border border-white/[0.06] text-left text-slate-300 hover:text-white transition font-mono text-[11px] truncate cursor-pointer"
            >
              + ports mapping
            </button>
            <button
              onclick={() =>
                insertSnippet("    environment:\n      - NODE_ENV=production")}
              class="px-2.5 py-1.5 rounded-lg bg-white/[0.04] hover:bg-white/[0.08] border border-white/[0.06] text-left text-slate-300 hover:text-white transition font-mono text-[11px] truncate cursor-pointer"
            >
              + environment
            </button>
            <button
              onclick={() =>
                insertSnippet("    volumes:\n      - app-data:/data")}
              class="px-2.5 py-1.5 rounded-lg bg-white/[0.04] hover:bg-white/[0.08] border border-white/[0.06] text-left text-slate-300 hover:text-white transition font-mono text-[11px] truncate cursor-pointer"
            >
              + volume bind
            </button>
          </div>
        </div>
      {:else}
        <!-- Environment Variables Summary Card -->
        <div
          class="p-4 rounded-2xl bg-[#12131c] border border-white/[0.08] shadow-lg space-y-3"
        >
          <div
            class="flex items-center justify-between border-b border-white/[0.08] pb-2.5"
          >
            <div class="flex items-center gap-2">
              <Key class="w-4 h-4 text-amber-400" />
              <span class="text-xs font-bold text-white tracking-wide"
                >Active Stack Variables</span
              >
            </div>
            <span
              class="px-2 py-0.5 rounded-full text-[10px] font-mono bg-amber-500/20 text-amber-300 border border-amber-500/30"
            >
              {envVarCount} {envVarCount === 1 ? "var" : "vars"}
            </span>
          </div>

          {#if envVarCount === 0}
            <div class="py-6 text-center text-slate-500 text-xs">
              <FileKey class="w-8 h-8 mx-auto text-slate-600 mb-2" />
              <p>No variables defined yet.</p>
              <p class="text-[11px] text-slate-600 mt-1 leading-relaxed">
                Add <code>KEY=VALUE</code> lines in the .env editor to supply values for Compose variables.
              </p>
            </div>
          {:else}
            <div class="space-y-1.5 max-h-[380px] overflow-y-auto pr-1">
              {#each envVarEntries as [key, val]}
                <div
                  class="flex items-center justify-between gap-2 p-2 rounded-lg bg-[#090a10] border border-white/[0.06] text-xs font-mono"
                >
                  <span class="text-amber-300 font-semibold truncate">{key}</span>
                  <span
                    class="text-slate-400 truncate max-w-[140px] text-right text-[11px]"
                    >{val || '""'}</span
                  >
                </div>
              {/each}
            </div>
          {/if}
        </div>

        <!-- Quick Env Snippets Card -->
        <div
          class="p-4 rounded-2xl bg-[#12131c] border border-white/[0.08] shadow-lg space-y-3"
        >
          <div
            class="flex items-center justify-between border-b border-white/[0.08] pb-2.5"
          >
            <div class="flex items-center gap-2">
              <Sparkles class="w-4 h-4 text-amber-400" />
              <span class="text-xs font-bold text-white tracking-wide"
                >Common Env Presets</span
              >
            </div>
            <span class="text-[10px] text-slate-500">Click to append</span>
          </div>

          <div class="grid grid-cols-2 gap-2 text-xs">
            <button
              onclick={() => insertEnvSnippet("PUID=1000\nPGID=1000")}
              class="px-2.5 py-1.5 rounded-lg bg-white/[0.04] hover:bg-white/[0.08] border border-white/[0.06] text-left text-slate-300 hover:text-white transition font-mono text-[11px] truncate cursor-pointer"
            >
              + PUID &amp; PGID
            </button>
            <button
              onclick={() => insertEnvSnippet("TZ=UTC")}
              class="px-2.5 py-1.5 rounded-lg bg-white/[0.04] hover:bg-white/[0.08] border border-white/[0.06] text-left text-slate-300 hover:text-white transition font-mono text-[11px] truncate cursor-pointer"
            >
              + Timezone (TZ)
            </button>
            <button
              onclick={() => insertEnvSnippet("PORT=8080")}
              class="px-2.5 py-1.5 rounded-lg bg-white/[0.04] hover:bg-white/[0.08] border border-white/[0.06] text-left text-slate-300 hover:text-white transition font-mono text-[11px] truncate cursor-pointer"
            >
              + PORT=8080
            </button>
            <button
              onclick={() =>
                insertEnvSnippet("DATABASE_URL=postgres://user:pass@db:5432/app")}
              class="px-2.5 py-1.5 rounded-lg bg-white/[0.04] hover:bg-white/[0.08] border border-white/[0.06] text-left text-slate-300 hover:text-white transition font-mono text-[11px] truncate cursor-pointer"
            >
              + DATABASE_URL
            </button>
          </div>
        </div>
      {/if}
    </div>
  </div>

  <!-- Redeploy Confirmation Modal (Asks to re-pull images) -->
  {#if isRedeployModalOpen}
    <div
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-md animate-in fade-in"
    >
      <div
        class="w-full max-w-lg rounded-2xl bg-[#12131c] border border-white/[0.1] p-6 space-y-5 shadow-2xl animate-in zoom-in-95 duration-150"
      >
        <div class="flex items-center gap-3">
          <div
            class="w-10 h-10 rounded-xl bg-violet-500/15 border border-violet-500/30 flex items-center justify-center text-violet-400 shrink-0"
          >
            <RotateCw class="w-5 h-5 text-violet-400" />
          </div>
          <div>
            <h3 class="text-base font-bold text-white">Redeploy Stack: {name}</h3>
            <p class="text-xs text-slate-400">
              Recreate container services with your updated Compose &amp; environment configuration.
            </p>
          </div>
        </div>

        <!-- Services list preview -->
        {#if parsedServices.length > 0}
          <div class="space-y-1.5">
            <div
              class="text-[11px] font-semibold text-slate-400 uppercase tracking-wider"
            >
              Affected Services ({parsedServices.length})
            </div>
            <div
              class="p-2.5 rounded-xl bg-[#090a10] border border-white/[0.06] max-h-36 overflow-y-auto space-y-1.5"
            >
              {#each parsedServices as svc}
                <div
                  class="flex items-center justify-between text-xs font-mono"
                >
                  <span class="text-cyan-300 font-semibold">{svc.name}</span>
                  <span
                    class="text-slate-400 text-[11px] truncate max-w-[220px]"
                    >{svc.image}</span
                  >
                </div>
              {/each}
            </div>
          </div>
        {/if}

        <!-- Pull Latest Images Toggle Checkbox -->
        <label
          class="flex items-start gap-3 p-3.5 rounded-xl bg-violet-500/10 border border-violet-500/25 hover:border-violet-500/40 transition cursor-pointer select-none"
        >
          <input
            type="checkbox"
            bind:checked={pullLatestImages}
            class="mt-0.5 rounded border-violet-500/40 bg-black/40 text-violet-500 focus:ring-0 cursor-pointer"
          />
          <div class="text-xs space-y-0.5">
            <div class="font-semibold text-violet-200 flex items-center gap-1.5">
              <span>Re-pull latest images before redeploying</span>
              <span
                class="px-1.5 py-0.2 rounded text-[9px] font-mono bg-violet-500/25 text-violet-300 border border-violet-500/40"
                >Recommended</span
              >
            </div>
            <p class="text-[11px] text-slate-400 leading-relaxed">
              Pulls the newest image tags from remote registries before restarting containers. If unchecked, uses existing cached local images for a faster restart.
            </p>
          </div>
        </label>

        <!-- Modal Actions -->
        <div class="flex items-center justify-end gap-2.5 pt-2">
          <button
            type="button"
            onclick={() => (isRedeployModalOpen = false)}
            disabled={isDeploying}
            class="px-4 py-2 rounded-xl bg-white/[0.05] hover:bg-white/[0.1] text-slate-300 text-xs font-semibold transition cursor-pointer"
          >
            Cancel
          </button>

          <button
            type="button"
            onclick={() => {
              isRedeployModalOpen = false;
              handleDeploy(true, pullLatestImages);
            }}
            disabled={isDeploying}
            class="flex items-center gap-2 px-5 py-2 rounded-xl bg-gradient-to-r from-violet-600 to-indigo-600 hover:from-violet-500 hover:to-indigo-500 text-white text-xs font-semibold shadow-[0_0_20px_rgba(124,58,237,0.4)] border border-violet-400/30 transition cursor-pointer"
          >
            <RotateCw class="w-3.5 h-3.5 text-cyan-300" />
            <span
              >{pullLatestImages
                ? "Pull Images & Redeploy"
                : "Redeploy with Existing Images"}</span
            >
          </button>
        </div>
      </div>
    </div>
  {/if}
</div>
