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
  } from '@lucide/svelte';
  import YamlEditor from "../lib/components/YamlEditor.svelte";
  import { stackApi } from "../lib/api";
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
  let isYamlValid = $state(true);
  let yamlError = $state<string | null>(null);

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

  // Save or Deploy Action
  async function handleDeploy(shouldDeploy = true) {
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
        await stackApi.update(trimmedName, composeYaml, shouldDeploy);
      } else {
        await stackApi.create(trimmedName, composeYaml, shouldDeploy);
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
      handleDeploy(true);
    }
  }

  // Quick insertion helpers
  function insertSnippet(snippet: string) {
    composeYaml =
      composeYaml + (composeYaml.endsWith("\n") ? "" : "\n") + snippet + "\n";
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
        onclick={() => handleDeploy(true)}
        disabled={isSaving ||
          isDeploying ||
          !isNameValid ||
          !composeYaml.trim() ||
          !isYamlValid}
        class="flex items-center gap-2 px-5 py-2 rounded-xl bg-gradient-to-r from-violet-600 to-indigo-600 hover:from-violet-500 hover:to-indigo-500 text-white text-xs font-semibold shadow-[0_0_20px_rgba(124,58,237,0.4)] border border-violet-400/30 transition disabled:opacity-50 disabled:cursor-not-allowed"
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
      {#if isLoadingStack}
        <div
          class="h-96 rounded-2xl bg-[#12131c] border border-white/[0.08] flex items-center justify-center text-slate-400 gap-3"
        >
          <LoaderCircle class="w-6 h-6 animate-spin text-violet-400" />
          <span class="text-xs font-mono">Loading stack compose file...</span>
        </div>
      {:else}
        <YamlEditor
          bind:value={composeYaml}
          minHeight="540px"
          onValidate={(valid, err) => {
            isYamlValid = valid;
            yamlError = err;
          }}
        />
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
        <span class="text-slate-500 font-mono">PDSwarm v1.0 Compose Studio</span
        >
      </div>
    </div>

    <!-- Right Side: Real-time Service Inspector & Blueprint Helpers -->
    <div class="lg:col-span-4 space-y-4">
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
            class="px-2.5 py-1.5 rounded-lg bg-white/[0.04] hover:bg-white/[0.08] border border-white/[0.06] text-left text-slate-300 hover:text-white transition font-mono text-[11px] truncate"
          >
            + restart policy
          </button>
          <button
            onclick={() => insertSnippet('    ports:\n      - "8080:80"')}
            class="px-2.5 py-1.5 rounded-lg bg-white/[0.04] hover:bg-white/[0.08] border border-white/[0.06] text-left text-slate-300 hover:text-white transition font-mono text-[11px] truncate"
          >
            + ports mapping
          </button>
          <button
            onclick={() =>
              insertSnippet("    environment:\n      - NODE_ENV=production")}
            class="px-2.5 py-1.5 rounded-lg bg-white/[0.04] hover:bg-white/[0.08] border border-white/[0.06] text-left text-slate-300 hover:text-white transition font-mono text-[11px] truncate"
          >
            + environment
          </button>
          <button
            onclick={() =>
              insertSnippet("    volumes:\n      - app-data:/data")}
            class="px-2.5 py-1.5 rounded-lg bg-white/[0.04] hover:bg-white/[0.08] border border-white/[0.06] text-left text-slate-300 hover:text-white transition font-mono text-[11px] truncate"
          >
            + volume bind
          </button>
        </div>
      </div>
    </div>
  </div>
</div>
