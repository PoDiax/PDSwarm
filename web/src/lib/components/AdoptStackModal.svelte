<script lang="ts">
  import {
    ShieldCheck,
    ShieldAlert,
    X,
    LoaderCircle,
    Copy,
    Check,
    FileText,
    Folder,
    Info,
    Sparkles,
    RefreshCw,
    CircleCheck,
  } from '@lucide/svelte';
  import { stackApi } from '../api';
  import type { Stack } from '../types';
  import {
    detectRelativeVolumes,
    resolveRelativeVolumesInYaml,
    getStackComposeDir,
  } from '../utils/compose';

  interface Props {
    stack: Stack | null;
    isOpen: boolean;
    onClose: () => void;
    onAdopted: (adopted: Stack) => void;
  }

  let { stack, isOpen, onClose, onAdopted }: Props = $props();

  let isSubmitting = $state(false);
  let error = $state<string | null>(null);
  let copied = $state(false);

  // Editable compose YAML and working directory
  let editableYaml = $state('');
  let editableWorkingDir = $state('');

  let detectedRelPaths = $derived(detectRelativeVolumes(editableYaml));
  let detectedComposeDir = $derived(editableWorkingDir?.trim() || getStackComposeDir(stack) || '');
  let autoResolvePaths = $state(true);
  let pathsConverted = $state(false);

  $effect(() => {
    if (stack && isOpen) {
      editableYaml = stack.compose_yaml || '';
      editableWorkingDir = stack.working_dir || getStackComposeDir(stack) || '';
      error = null;
      pathsConverted = false;
      autoResolvePaths = true;

      // If compose_yaml is not yet loaded on client, fetch from backend (recovers from Portainer or synthesizes from containers)
      if (!editableYaml) {
        stackApi.get(stack.name).then(fullStack => {
          if (fullStack && fullStack.compose_yaml) {
            editableYaml = fullStack.compose_yaml;
            if (!editableWorkingDir) {
              editableWorkingDir = fullStack.working_dir || getStackComposeDir(fullStack) || '';
            }
          }
        }).catch(() => {});
      }
    }
  });

  function handleConvertRelativePaths() {
    if (!detectedComposeDir) return;
    editableYaml = resolveRelativeVolumesInYaml(editableYaml, detectedComposeDir);
    if (!editableWorkingDir) {
      editableWorkingDir = detectedComposeDir;
    }
    pathsConverted = true;
  }

  async function handleCopyYaml() {
    if (!editableYaml) return;
    try {
      await navigator.clipboard.writeText(editableYaml);
      copied = true;
      setTimeout(() => (copied = false), 2000);
    } catch {
      // ignore
    }
  }

  async function handleAdopt() {
    if (!stack) return;
    isSubmitting = true;
    error = null;

    try {
      let finalYaml = editableYaml.trim();
      let finalDir = editableWorkingDir.trim() || detectedComposeDir || undefined;

      if (autoResolvePaths && finalDir && detectedRelPaths.length > 0) {
        finalYaml = resolveRelativeVolumesInYaml(finalYaml, finalDir);
        editableYaml = finalYaml;
      }

      const adopted = await stackApi.adopt(stack.name, {
        compose_yaml: finalYaml || undefined,
        working_dir: finalDir,
      });
      onAdopted(adopted);
      onClose();
    } catch (err: unknown) {
      error = err instanceof Error ? err.message : 'Failed to adopt stack';
    } finally {
      isSubmitting = false;
    }
  }

  function handleKeyDown(e: KeyboardEvent) {
    if (e.key === 'Escape' && isOpen && !isSubmitting) {
      onClose();
    }
  }
</script>

<svelte:window onkeydown={handleKeyDown} />

{#if isOpen && stack}
  <div
    class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-md animate-in fade-in duration-200"
    role="dialog"
    aria-modal="true"
    aria-labelledby="adopt-modal-title"
  >
    <div
      class="relative w-full max-w-2xl max-h-[90vh] flex flex-col rounded-2xl bg-[#12131c] border border-cyan-500/30 shadow-[0_0_50px_rgba(6,182,212,0.15)] overflow-hidden"
    >
      <!-- Header -->
      <div class="flex items-center justify-between p-5 border-b border-white/[0.08] bg-[#151724]">
        <div class="flex items-center gap-3">
          <div class="w-10 h-10 rounded-xl bg-gradient-to-br from-violet-600/30 to-cyan-500/30 border border-cyan-500/40 flex items-center justify-center text-cyan-400 shadow-[0_0_15px_rgba(6,182,212,0.3)]">
            <ShieldCheck class="w-5 h-5" />
          </div>
          <div>
            <h2 id="adopt-modal-title" class="text-base font-bold text-white flex items-center gap-2">
              <span>Take Full Control:</span>
              <span class="font-mono text-cyan-400">{stack.name}</span>
            </h2>
            <p class="text-xs text-slate-400">
              Adopt this external Docker Compose stack into native PDSwarm management
            </p>
          </div>
        </div>

        <button
          onclick={onClose}
          disabled={isSubmitting}
          class="p-2 rounded-xl text-slate-400 hover:text-white hover:bg-white/[0.06] transition disabled:opacity-50"
          title="Close modal"
        >
          <X class="w-5 h-5" />
        </button>
      </div>

      <!-- Body / Scrollable Content -->
      <div class="p-6 overflow-y-auto space-y-5 flex-1">
        <!-- Portainer Comparison / Feature Banner -->
        <div class="p-4 rounded-xl bg-gradient-to-r from-violet-950/40 via-cyan-950/30 to-indigo-950/40 border border-cyan-500/30 space-y-2">
          <div class="flex items-center gap-2 text-cyan-300 font-semibold text-xs">
            <Sparkles class="w-4 h-4 text-cyan-400" />
            <span>Why PDSwarm Adoption Succeeds Where Portainer Fails</span>
          </div>
          <p class="text-xs text-slate-300 leading-relaxed">
            Portainer marks external stacks as <span class="text-amber-400 font-mono">"Limited control"</span> and disables editing or redeploying because it lacks Compose context.
            PDSwarm automatically extracts the host Compose YAML, aligns container names and named volume prefixes (<span class="font-mono text-violet-300">{stack.name}_*</span>), and grants you <strong>full control</strong> with <strong>zero downtime</strong>.
          </p>
        </div>

        <!-- Detected Context Metadata -->
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3 text-xs">
          <div class="p-3 rounded-xl bg-[#090a10] border border-white/[0.08] space-y-1">
            <span class="text-[11px] text-slate-500 font-medium flex items-center gap-1.5">
              <FileText class="w-3.5 h-3.5 text-cyan-400" />
              <span>Compose Config File</span>
            </span>
            <p class="font-mono text-[11px] text-slate-200 truncate" title={stack.config_file || 'Discovered from labels'}>
              {stack.config_file || 'Discovered from container labels'}
            </p>
          </div>

          <div class="p-3 rounded-xl bg-[#090a10] border border-white/[0.08] space-y-1">
            <span class="text-[11px] text-slate-500 font-medium flex items-center gap-1.5">
              <Folder class="w-3.5 h-3.5 text-violet-400" />
              <span>Host Working Directory</span>
            </span>
            <input
              type="text"
              bind:value={editableWorkingDir}
              placeholder="e.g. /home/user/project"
              class="w-full bg-transparent font-mono text-[11px] text-slate-200 focus:outline-none focus:text-white"
            />
          </div>
        </div>

        <!-- Relative Volumes Warning & Conversion -->
        {#if detectedRelPaths.length > 0}
          <div class="p-4 rounded-xl bg-amber-500/10 border border-amber-500/30 space-y-3">
            <div class="flex items-start justify-between gap-3">
              <div class="flex items-start gap-2.5">
                <ShieldAlert class="w-4 h-4 text-amber-400 shrink-0 mt-0.5" />
                <div class="space-y-1">
                  <h4 class="text-xs font-bold text-amber-300">
                    Relative Volume Mounts Detected ({detectedRelPaths.length})
                  </h4>
                  <p class="text-xs text-slate-300 leading-relaxed">
                    This Compose file mounts relative host directories:
                    {#each detectedRelPaths as path, i}
                      <code class="px-1.5 py-0.5 rounded bg-black/40 text-amber-300 font-mono text-[11px] border border-amber-500/20">{path}</code>{i < detectedRelPaths.length - 1 ? ', ' : ''}
                    {/each}.
                    In Docker Compose, relative paths break if executed from a different directory or recreated without context.
                  </p>
                  {#if detectedComposeDir}
                    <p class="text-[11px] text-slate-400 pt-0.5">
                      Detected host Compose folder: <span class="text-cyan-300 font-mono font-medium">{detectedComposeDir}</span>
                    </p>
                  {:else}
                    <p class="text-[11px] text-amber-400/90 pt-0.5">
                      Enter the <strong>Host Working Directory</strong> above to automatically resolve these paths to their absolute location.
                    </p>
                  {/if}
                </div>
              </div>

              {#if detectedComposeDir}
                <button
                  type="button"
                  onclick={handleConvertRelativePaths}
                  class="shrink-0 px-3 py-1.5 rounded-lg bg-amber-500/20 hover:bg-amber-500/30 text-amber-200 border border-amber-500/40 text-xs font-semibold flex items-center gap-1.5 transition cursor-pointer"
                  title="Convert relative paths to absolute host paths immediately"
                >
                  <RefreshCw class="w-3.5 h-3.5" />
                  <span>Convert Now</span>
                </button>
              {/if}
            </div>

            {#if detectedComposeDir}
              <label class="flex items-center gap-2 pt-2 border-t border-amber-500/20 text-xs text-slate-300 cursor-pointer select-none">
                <input
                  type="checkbox"
                  bind:checked={autoResolvePaths}
                  class="rounded bg-[#090a10] border-amber-500/40 text-cyan-500 focus:ring-0 focus:ring-offset-0 cursor-pointer"
                />
                <span>Automatically convert relative volumes to <span class="font-mono text-cyan-300 text-[11px]">{detectedComposeDir}/...</span> on adoption</span>
              </label>
            {/if}
          </div>
        {:else if pathsConverted}
          <div class="p-3 rounded-xl bg-emerald-500/10 border border-emerald-500/30 flex items-center gap-2 text-xs text-emerald-300">
            <CircleCheck class="w-4 h-4 text-emerald-400 shrink-0" />
            <span>All relative volume paths converted to absolute paths in <strong class="font-mono">{detectedComposeDir}</strong>.</span>
          </div>
        {/if}

        <!-- Compose YAML Code Preview / Editor -->
        <div class="space-y-2">
          <div class="flex items-center justify-between">
            <label for="adopt-yaml-editor" class="text-xs font-semibold text-slate-300 flex items-center gap-1.5">
              <FileText class="w-3.5 h-3.5 text-slate-400" />
              <span>Compose Definition ({editableYaml ? 'Detected on Host' : 'Manual Input Required'})</span>
            </label>
            {#if editableYaml}
              <button
                onclick={handleCopyYaml}
                class="flex items-center gap-1 text-[11px] font-mono text-slate-400 hover:text-white transition"
              >
                {#if copied}
                  <Check class="w-3.5 h-3.5 text-emerald-400" />
                  <span class="text-emerald-400">Copied!</span>
                {:else}
                  <Copy class="w-3.5 h-3.5" />
                  <span>Copy YAML</span>
                {/if}
              </button>
            {/if}
          </div>

          <div class="relative rounded-xl border border-white/[0.08] bg-[#090a10] overflow-hidden focus-within:border-cyan-500/50 transition">
            <textarea
              id="adopt-yaml-editor"
              bind:value={editableYaml}
              rows={12}
              placeholder="services:&#10;  app:&#10;    image: my-image:latest&#10;    ports:&#10;      - '8080:80'"
              class="w-full p-3.5 font-mono text-xs text-slate-200 bg-transparent resize-y focus:outline-none leading-relaxed"
              spellcheck="false"
            ></textarea>
          </div>
          <p class="text-[11px] text-slate-500">
            You can review or adjust this Compose YAML before adopting. The running containers will keep running without interruption.
          </p>
        </div>

        {#if error}
          <div class="p-3.5 rounded-xl bg-rose-500/15 border border-rose-500/30 flex items-start gap-2.5 text-xs text-rose-300">
            <ShieldAlert class="w-4 h-4 text-rose-400 shrink-0 mt-0.5" />
            <span>{error}</span>
          </div>
        {/if}
      </div>

      <!-- Footer Actions -->
      <div class="flex items-center justify-end gap-3 p-4 border-t border-white/[0.08] bg-[#151724]">
        <button
          onclick={onClose}
          disabled={isSubmitting}
          class="px-4 py-2 rounded-xl bg-white/[0.05] hover:bg-white/[0.1] text-xs font-semibold text-slate-300 hover:text-white transition disabled:opacity-50"
        >
          Cancel
        </button>

        <button
          onclick={handleAdopt}
          disabled={isSubmitting || !editableYaml.trim()}
          class="inline-flex items-center gap-2 px-5 py-2.5 rounded-xl bg-gradient-to-r from-violet-600 to-cyan-600 hover:from-violet-500 hover:to-cyan-500 text-white text-xs font-semibold shadow-[0_0_20px_rgba(6,182,212,0.35)] border border-cyan-400/30 transition disabled:opacity-50 cursor-pointer"
        >
          {#if isSubmitting}
            <LoaderCircle class="w-4 h-4 animate-spin text-cyan-200" />
            <span>Adopting Stack...</span>
          {:else}
            <ShieldCheck class="w-4 h-4 text-cyan-300" />
            <span>Take Full Control &amp; Adopt</span>
          {/if}
        </button>
      </div>
    </div>
  </div>
{/if}
