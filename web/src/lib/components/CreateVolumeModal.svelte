<script lang="ts">
  import {
    X,
    HardDrive,
    Plus,
    Trash2,
    CheckCircle2,
    AlertCircle,
    Loader2,
    Tag,
  } from '@lucide/svelte';
  import { volumeApi } from '../api';
  import type { Volume, CreateVolumeRequest } from '../types';

  interface Props {
    isOpen: boolean;
    onClose: () => void;
    onCreated?: (vol: Volume) => void;
  }

  let {
    isOpen = false,
    onClose,
    onCreated,
  }: Props = $props();

  // Form states
  let volumeName = $state('');
  let driver = $state('local');
  let labels = $state<Array<{ key: string; value: string }>>([]);

  let isSubmitting = $state(false);
  let error = $state<string | null>(null);

  function resetForm() {
    volumeName = '';
    driver = 'local';
    labels = [];
    error = null;
    isSubmitting = false;
  }

  function addLabel() {
    labels = [...labels, { key: '', value: '' }];
  }

  function removeLabel(index: number) {
    labels = labels.filter((_, i) => i !== index);
  }

  async function handleSubmit(e: SubmitEvent) {
    e.preventDefault();
    const trimmedName = volumeName.trim();
    if (!trimmedName) {
      error = 'Volume name is required.';
      return;
    }

    // Validate docker volume name format: [a-zA-Z0-9][a-zA-Z0-9_.-]*
    if (!/^[a-zA-Z0-9][a-zA-Z0-9_.-]*$/.test(trimmedName)) {
      error = 'Volume name may only contain alphanumeric characters, dots, dashes, and underscores, and must begin with an alphanumeric character.';
      return;
    }

    isSubmitting = true;
    error = null;

    try {
      const labelsMap: Record<string, string> = {};
      for (const item of labels) {
        if (item.key.trim()) {
          labelsMap[item.key.trim()] = item.value.trim();
        }
      }

      const req: CreateVolumeRequest = {
        name: trimmedName,
        Name: trimmedName,
        driver: driver.trim() || 'local',
        Driver: driver.trim() || 'local',
        labels: Object.keys(labelsMap).length > 0 ? labelsMap : undefined,
      };

      const createdVol = await volumeApi.create(req);
      resetForm();
      if (onCreated) {
        onCreated(createdVol);
      }
      onClose();
    } catch (err: unknown) {
      if (err instanceof Error) {
        error = err.message;
      } else {
        error = 'Failed to create Docker volume.';
      }
    } finally {
      isSubmitting = false;
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape' && isOpen && !isSubmitting) {
      handleClose();
    }
  }

  function handleClose() {
    resetForm();
    onClose();
  }
</script>

<svelte:window onkeydown={handleKeydown} />

{#if isOpen}
  <div
    class="fixed inset-0 z-50 flex items-center justify-center p-4"
    role="dialog"
    aria-modal="true"
    aria-labelledby="create-volume-modal-title"
    tabindex="-1"
  >
    <!-- Backdrop button -->
    <button
      type="button"
      class="fixed inset-0 bg-black/80 backdrop-blur-md cursor-default border-0"
      onclick={handleClose}
      aria-label="Close modal backdrop"
      tabindex="-1"
    ></button>

    <div
      class="relative w-full max-w-lg flex flex-col rounded-2xl bg-[#12131c] border border-white/[0.12] shadow-[0_10px_50px_rgba(0,0,0,0.8)] overflow-hidden animate-in fade-in zoom-in-95 duration-150"
    >
      <!-- Modal Header -->
      <div class="px-6 py-4 flex items-center justify-between border-b border-white/[0.08] bg-[#0c0d14]/80">
        <div class="flex items-center gap-3">
          <div class="w-8 h-8 rounded-xl bg-violet-600/20 border border-violet-500/30 flex items-center justify-center text-violet-400 shadow-[0_0_15px_rgba(124,58,237,0.3)]">
            <HardDrive class="w-4 h-4" />
          </div>
          <div>
            <h2 id="create-volume-modal-title" class="text-sm font-bold text-white tracking-wide">Create Docker Volume</h2>
            <p class="text-[11px] text-slate-400">Provision persistent storage managed by Docker daemon</p>
          </div>
        </div>

        <button
          type="button"
          onclick={handleClose}
          disabled={isSubmitting}
          class="p-1.5 rounded-lg hover:bg-white/10 text-slate-400 hover:text-white transition disabled:opacity-50"
          aria-label="Close modal"
        >
          <X class="w-4 h-4" />
        </button>
      </div>

      <!-- Modal Body -->
      <form onsubmit={handleSubmit} class="p-6 space-y-5 text-xs">
        {#if error}
          <div class="p-3 rounded-xl bg-rose-500/10 border border-rose-500/30 flex items-start gap-2.5 text-rose-300 animate-in fade-in">
            <AlertCircle class="w-4 h-4 shrink-0 mt-0.5 text-rose-400" />
            <div class="flex-1 leading-relaxed">
              <span class="font-semibold block">Volume Creation Error</span>
              <span class="text-[11px]">{error}</span>
            </div>
          </div>
        {/if}

        <!-- Volume Name -->
        <div class="space-y-1.5">
          <label for="volume-name" class="block font-medium text-slate-300">
            Volume Name <span class="text-violet-400">*</span>
          </label>
          <input
            id="volume-name"
            type="text"
            bind:value={volumeName}
            placeholder="e.g. pgdata, redis_storage, app_cache"
            required
            class="w-full px-3.5 py-2.5 rounded-xl bg-[#0c0d14] border border-white/10 text-white placeholder-slate-500 focus:outline-none focus:border-violet-500 focus:ring-1 focus:ring-violet-500 font-mono text-xs transition"
          />
          <span class="text-[10px] text-slate-500">
            Use lowercase alphanumeric characters, dashes, or underscores.
          </span>
        </div>

        <!-- Driver -->
        <div class="space-y-1.5">
          <label for="volume-driver" class="block font-medium text-slate-300">
            Volume Driver
          </label>
          <input
            id="volume-driver"
            type="text"
            bind:value={driver}
            placeholder="local"
            class="w-full px-3.5 py-2.5 rounded-xl bg-[#0c0d14] border border-white/10 text-white placeholder-slate-500 focus:outline-none focus:border-violet-500 font-mono text-xs transition"
          />
          <span class="text-[10px] text-slate-500">
            Defaults to <code class="text-violet-300">local</code> on standard Docker installations.
          </span>
        </div>

        <!-- Labels (Optional) -->
        <div class="space-y-2 pt-2 border-t border-white/[0.08]">
          <div class="flex items-center justify-between">
            <span class="font-medium text-slate-300 flex items-center gap-1.5">
              <Tag class="w-3.5 h-3.5 text-violet-400" />
              Labels (Optional)
            </span>
            <button
              type="button"
              onclick={addLabel}
              class="px-2 py-1 rounded-lg bg-white/5 hover:bg-white/10 text-slate-300 hover:text-white text-[11px] font-medium transition flex items-center gap-1"
            >
              <Plus class="w-3 h-3" />
              Add Label
            </button>
          </div>

          {#if labels.length > 0}
            <div class="space-y-2 max-h-36 overflow-y-auto pr-1">
              {#each labels as label, i}
                <div class="flex items-center gap-2">
                  <input
                    type="text"
                    bind:value={label.key}
                    placeholder="key (e.g. env)"
                    class="flex-1 px-3 py-1.5 rounded-lg bg-[#0c0d14] border border-white/10 text-white text-xs font-mono focus:outline-none focus:border-violet-500"
                  />
                  <input
                    type="text"
                    bind:value={label.value}
                    placeholder="value (e.g. production)"
                    class="flex-1 px-3 py-1.5 rounded-lg bg-[#0c0d14] border border-white/10 text-white text-xs font-mono focus:outline-none focus:border-violet-500"
                  />
                  <button
                    type="button"
                    onclick={() => removeLabel(i)}
                    class="p-1.5 rounded-lg hover:bg-rose-500/20 text-slate-500 hover:text-rose-400 transition"
                    aria-label="Remove label"
                  >
                    <Trash2 class="w-3.5 h-3.5" />
                  </button>
                </div>
              {/each}
            </div>
          {/if}
        </div>

        <!-- Footer Actions -->
        <div class="flex items-center justify-end gap-2 pt-4 border-t border-white/[0.08]">
          <button
            type="button"
            onclick={handleClose}
            disabled={isSubmitting}
            class="px-4 py-2 rounded-xl bg-white/5 hover:bg-white/10 text-slate-300 hover:text-white text-xs font-medium transition disabled:opacity-50"
          >
            Cancel
          </button>
          <button
            type="submit"
            disabled={isSubmitting || !volumeName.trim()}
            class="px-5 py-2 rounded-xl bg-violet-600 hover:bg-violet-500 text-white text-xs font-semibold shadow-[0_0_15px_rgba(124,58,237,0.35)] transition flex items-center gap-2 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {#if isSubmitting}
              <Loader2 class="w-3.5 h-3.5 animate-spin" />
              Creating Volume...
            {:else}
              <Plus class="w-3.5 h-3.5" />
              Create Volume
            {/if}
          </button>
        </div>
      </form>
    </div>
  </div>
{/if}
