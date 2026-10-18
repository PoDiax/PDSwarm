<script lang="ts">
  import {
    X,
    Server,
    Shield,
    Terminal,
    Globe,
    Lock,
    Upload,
    CheckCircle2,
    AlertCircle,
    Loader2,
    Zap,
    FileText,
  } from '@lucide/svelte';
  import { environmentStore } from '../stores/environment.svelte';
  import { environmentApi } from '../api';
  import { portal } from '../actions/portal';
  import type { Environment, CreateEnvironmentRequest } from '../types';

  interface Props {
    isOpen: boolean;
    onClose: () => void;
    onCreated?: (env: Environment) => void;
  }

  let {
    isOpen = false,
    onClose,
    onCreated,
  }: Props = $props();

  // Form states
  let name = $state('');
  let envType = $state<'local' | 'tcp' | 'tls'>('local');
  let url = $state('unix:///var/run/docker.sock');
  let isDefault = $state(false);

  // TLS inputs
  let tlsCa = $state('');
  let tlsCert = $state('');
  let tlsKey = $state('');
  let tlsInputMode = $state<'text' | 'file'>('text');

  // Status states
  let isSubmitting = $state(false);
  let isTesting = $state(false);
  let testResult = $state<{ success: boolean; message: string } | null>(null);
  let error = $state<string | null>(null);

  function handleTypeChange(type: 'local' | 'tcp' | 'tls') {
    envType = type;
    testResult = null;
    error = null;
    if (type === 'local' && (!url || url.startsWith('tcp://'))) {
      url = 'unix:///var/run/docker.sock';
    } else if (type === 'tcp' && (!url || url.startsWith('unix://'))) {
      url = 'tcp://192.168.1.100:2375';
    } else if (type === 'tls' && (!url || url.startsWith('unix://') || url.endsWith(':2375'))) {
      url = 'tcp://192.168.1.100:2376';
    }
  }

  function resetForm() {
    name = '';
    envType = 'local';
    url = 'unix:///var/run/docker.sock';
    isDefault = false;
    tlsCa = '';
    tlsCert = '';
    tlsKey = '';
    tlsInputMode = 'text';
    isSubmitting = false;
    isTesting = false;
    testResult = null;
    error = null;
  }

  async function handleFileUpload(field: 'ca' | 'cert' | 'key', e: Event) {
    const input = e.target as HTMLInputElement;
    if (!input.files || input.files.length === 0) return;
    const file = input.files[0];
    try {
      const text = await file.text();
      if (field === 'ca') tlsCa = text;
      if (field === 'cert') tlsCert = text;
      if (field === 'key') tlsKey = text;
    } catch {
      error = `Failed to read file ${file.name}`;
    }
  }

  function validateInput(): string | null {
    const trimmedName = name.trim();
    if (!trimmedName) return 'Environment name is required.';
    const trimmedUrl = url.trim();
    if (!trimmedUrl) return 'Daemon URL is required.';

    if (envType === 'local') {
      if (!trimmedUrl.startsWith('unix://') && !trimmedUrl.startsWith('/')) {
        return 'Local socket URL must start with "unix://" or be an absolute socket path.';
      }
    } else if (envType === 'tcp' || envType === 'tls') {
      if (!trimmedUrl.startsWith('tcp://') && !trimmedUrl.startsWith('http://') && !trimmedUrl.startsWith('https://')) {
        return 'Remote daemon URL should start with "tcp://", "http://", or "https://".';
      }
    }

    if (envType === 'tls') {
      if (!tlsCert.trim() && !tlsKey.trim() && !tlsCa.trim()) {
        return 'TLS environment requires CA or client certificates to be configured.';
      }
    }

    return null;
  }

  async function handleTestConnection() {
    const validationError = validateInput();
    if (validationError) {
      error = validationError;
      testResult = null;
      return;
    }

    isTesting = true;
    error = null;
    testResult = null;

    try {
      const payload: CreateEnvironmentRequest = {
        name: name.trim(),
        type: envType,
        url: url.trim(),
        is_default: isDefault,
        tls_ca: envType === 'tls' && tlsCa.trim() ? tlsCa.trim() : undefined,
        tls_cert: envType === 'tls' && tlsCert.trim() ? tlsCert.trim() : undefined,
        tls_key: envType === 'tls' && tlsKey.trim() ? tlsKey.trim() : undefined,
      };

      const res = await environmentApi.test(payload);
      if (res.success) {
        testResult = {
          success: true,
          message: res.message || 'Connection verified successfully.',
        };
      } else {
        testResult = {
          success: false,
          message: res.error || 'Connection probe failed.',
        };
      }
    } catch (err: unknown) {
      testResult = {
        success: false,
        message: err instanceof Error ? err.message : 'Connection test failed',
      };
    } finally {
      isTesting = false;
    }
  }

  async function handleSubmit(e: SubmitEvent) {
    e.preventDefault();
    const validationError = validateInput();
    if (validationError) {
      error = validationError;
      return;
    }

    isSubmitting = true;
    error = null;
    testResult = null;

    try {
      const payload: CreateEnvironmentRequest = {
        name: name.trim(),
        type: envType,
        url: url.trim(),
        is_default: isDefault,
        tls_ca: envType === 'tls' && tlsCa.trim() ? tlsCa.trim() : undefined,
        tls_cert: envType === 'tls' && tlsCert.trim() ? tlsCert.trim() : undefined,
        tls_key: envType === 'tls' && tlsKey.trim() ? tlsKey.trim() : undefined,
      };

      const created = await environmentStore.addEnvironment(payload);
      resetForm();
      if (onCreated) {
        onCreated(created);
      }
      onClose();
    } catch (err: unknown) {
      if (err instanceof Error) {
        error = err.message;
      } else {
        error = 'Failed to connect and register Docker environment.';
      }
    } finally {
      isSubmitting = false;
    }
  }

  function handleClose() {
    resetForm();
    onClose();
  }
</script>

<svelte:window
  onkeydown={(e) => {
    if (isOpen && e.key === 'Escape') {
      handleClose();
    }
  }}
/>

{#if isOpen}
  <div
    use:portal
    class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-md animate-in fade-in duration-200"
    role="dialog"
    tabindex="-1"
    aria-modal="true"
    onclick={(e) => {
      if (e.target === e.currentTarget) handleClose();
    }}
    onkeydown={(e) => {
      if (e.key === 'Escape') handleClose();
    }}
  >
    <div
      class="relative w-full max-w-2xl bg-[#0e0f17] border border-white/[0.08] rounded-2xl shadow-[0_25px_60px_rgba(0,0,0,0.85)] overflow-hidden flex flex-col max-h-[90vh]"
    >
      <!-- Header with glowing accent (Pinned) -->
      <div class="px-6 py-4 border-b border-white/[0.08] flex items-center justify-between bg-[#12131c] shrink-0">
        <div class="flex items-center gap-3">
          <div class="w-10 h-10 rounded-xl bg-gradient-to-br from-violet-600/30 to-cyan-500/20 border border-violet-500/30 flex items-center justify-center shadow-[0_0_15px_rgba(124,58,237,0.25)]">
            <Server class="w-5 h-5 text-violet-400" />
          </div>
          <div>
            <h2 class="text-base font-semibold text-white tracking-wide flex items-center gap-2">
              Connect Docker Host
              <span class="text-[10px] font-mono font-medium px-2 py-0.5 rounded-full bg-violet-500/10 text-violet-300 border border-violet-500/20">
                Multi-Node
              </span>
            </h2>
            <p class="text-xs text-slate-400 mt-0.5">
              Add a local socket, unencrypted TCP daemon, or secured TLS Docker engine
            </p>
          </div>
        </div>

        <button
          onclick={handleClose}
          type="button"
          class="p-2 rounded-lg text-slate-400 hover:text-white hover:bg-white/5 transition"
          aria-label="Close modal"
        >
          <X class="w-5 h-5" />
        </button>
      </div>

      <!-- Form container wrapping scrollable inputs and pinned footer -->
      <form onsubmit={handleSubmit} class="flex flex-col flex-1 min-h-0 overflow-hidden">
        <!-- Scrollable Form Body -->
        <div class="flex-1 overflow-y-auto p-6 space-y-5">
        <!-- Error Banner -->
        {#if error}
          <div class="p-3.5 rounded-xl bg-rose-500/10 border border-rose-500/30 flex items-start gap-3 text-rose-300 text-xs animate-in fade-in duration-150">
            <AlertCircle class="w-4 h-4 text-rose-400 shrink-0 mt-0.5" />
            <div class="flex-1 leading-relaxed">
              <span class="font-semibold text-rose-200">Connection Error:</span> {error}
            </div>
          </div>
        {/if}

        <!-- Test Result Banner -->
        {#if testResult}
          <div class="p-3.5 rounded-xl {testResult.success ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-300' : 'bg-amber-500/10 border-amber-500/30 text-amber-300'} border flex items-start gap-3 text-xs animate-in fade-in duration-150">
            {#if testResult.success}
              <CheckCircle2 class="w-4 h-4 text-emerald-400 shrink-0 mt-0.5" />
            {:else}
              <AlertCircle class="w-4 h-4 text-amber-400 shrink-0 mt-0.5" />
            {/if}
            <div class="flex-1 leading-relaxed">
              {testResult.message}
            </div>
          </div>
        {/if}

        <!-- Environment Name -->
        <div class="space-y-1.5">
          <label for="env-name" class="block text-xs font-medium text-slate-300">
            Environment Name <span class="text-violet-400">*</span>
          </label>
          <input
            id="env-name"
            type="text"
            bind:value={name}
            placeholder="e.g. Production Swarm, Edge Node 01, Worker Daemon"
            class="w-full px-3.5 py-2.5 rounded-xl bg-[#12131c] border border-white/[0.08] focus:border-violet-500 focus:ring-1 focus:ring-violet-500 text-sm text-white placeholder-slate-500 outline-none transition"
            required
          />
        </div>

        <!-- Connection Type Selector -->
        <div class="space-y-2">
          <span class="block text-xs font-medium text-slate-300">
            Connection Type <span class="text-violet-400">*</span>
          </span>
          <div class="grid grid-cols-3 gap-2.5">
            <!-- Local Socket -->
            <button
              type="button"
              onclick={() => handleTypeChange('local')}
              class="flex flex-col items-center justify-center p-3 rounded-xl border text-center transition-all duration-150 {envType === 'local' ? 'bg-violet-600/15 border-violet-500 text-white shadow-[0_0_12px_rgba(124,58,237,0.2)]' : 'bg-[#12131c] border-white/[0.06] text-slate-400 hover:text-slate-200 hover:bg-[#181a26]'}"
            >
              <Terminal class="w-4 h-4 mb-1.5 {envType === 'local' ? 'text-violet-400' : 'text-slate-400'}" />
              <span class="text-xs font-semibold">Unix Socket</span>
              <span class="text-[10px] text-slate-500 mt-0.5">Local Daemon</span>
            </button>

            <!-- Remote TCP -->
            <button
              type="button"
              onclick={() => handleTypeChange('tcp')}
              class="flex flex-col items-center justify-center p-3 rounded-xl border text-center transition-all duration-150 {envType === 'tcp' ? 'bg-cyan-600/15 border-cyan-500 text-white shadow-[0_0_12px_rgba(6,182,212,0.2)]' : 'bg-[#12131c] border-white/[0.06] text-slate-400 hover:text-slate-200 hover:bg-[#181a26]'}"
            >
              <Globe class="w-4 h-4 mb-1.5 {envType === 'tcp' ? 'text-cyan-400' : 'text-slate-400'}" />
              <span class="text-xs font-semibold">Remote TCP</span>
              <span class="text-[10px] text-slate-500 mt-0.5">Plain Socket</span>
            </button>

            <!-- Secured TLS -->
            <button
              type="button"
              onclick={() => handleTypeChange('tls')}
              class="flex flex-col items-center justify-center p-3 rounded-xl border text-center transition-all duration-150 {envType === 'tls' ? 'bg-emerald-600/15 border-emerald-500 text-white shadow-[0_0_12px_rgba(16,185,129,0.2)]' : 'bg-[#12131c] border-white/[0.06] text-slate-400 hover:text-slate-200 hover:bg-[#181a26]'}"
            >
              <Lock class="w-4 h-4 mb-1.5 {envType === 'tls' ? 'text-emerald-400' : 'text-slate-400'}" />
              <span class="text-xs font-semibold">TLS Encrypted</span>
              <span class="text-[10px] text-slate-500 mt-0.5">Cert Auth</span>
            </button>
          </div>
        </div>

        <!-- Daemon URL -->
        <div class="space-y-1.5">
          <label for="env-url" class="block text-xs font-medium text-slate-300">
            Docker Daemon URL <span class="text-violet-400">*</span>
          </label>
          <input
            id="env-url"
            type="text"
            bind:value={url}
            placeholder={envType === 'local' ? 'unix:///var/run/docker.sock' : 'tcp://192.168.1.100:2376'}
            class="w-full px-3.5 py-2.5 rounded-xl bg-[#12131c] border border-white/[0.08] focus:border-violet-500 focus:ring-1 focus:ring-violet-500 text-sm font-mono text-cyan-300 placeholder-slate-600 outline-none transition"
            required
          />
          <p class="text-[11px] text-slate-500">
            {#if envType === 'local'}
              Default Unix socket path on Linux/macOS host.
            {:else if envType === 'tcp'}
              Standard remote Docker daemon port is typically 2375.
            {:else}
              Standard TLS-secured Docker port is 2376.
            {/if}
          </p>
        </div>

        <!-- TLS Certificates Section (Conditional) -->
        {#if envType === 'tls'}
          <div class="p-4 rounded-xl bg-[#12131c]/60 border border-emerald-500/20 space-y-4 animate-in fade-in duration-200">
            <div class="flex items-center justify-between border-b border-white/[0.06] pb-3">
              <div class="flex items-center gap-2">
                <Shield class="w-4 h-4 text-emerald-400" />
                <span class="text-xs font-semibold text-white">TLS Certificates & Keys</span>
              </div>
              <div class="flex items-center gap-1 bg-[#181a26] p-1 rounded-lg border border-white/[0.06]">
                <button
                  type="button"
                  onclick={() => (tlsInputMode = 'text')}
                  class="px-2.5 py-1 rounded text-[11px] font-medium transition {tlsInputMode === 'text' ? 'bg-violet-600 text-white' : 'text-slate-400 hover:text-white'}"
                >
                  <FileText class="w-3 h-3 inline mr-1" /> Text Paste
                </button>
                <button
                  type="button"
                  onclick={() => (tlsInputMode = 'file')}
                  class="px-2.5 py-1 rounded text-[11px] font-medium transition {tlsInputMode === 'file' ? 'bg-violet-600 text-white' : 'text-slate-400 hover:text-white'}"
                >
                  <Upload class="w-3 h-3 inline mr-1" /> Upload Files
                </button>
              </div>
            </div>

            <!-- CA Certificate -->
            <div class="space-y-1.5">
              <label for="tls-ca" class="block text-xs font-medium text-slate-300">
                CA Certificate (ca.pem)
              </label>
              {#if tlsInputMode === 'text'}
                <textarea
                  id="tls-ca"
                  bind:value={tlsCa}
                  rows={2}
                  placeholder="-----BEGIN CERTIFICATE-----&#10;...&#10;-----END CERTIFICATE-----"
                  class="w-full px-3 py-2 rounded-lg bg-[#0a0b10] border border-white/[0.08] focus:border-emerald-500 text-xs font-mono text-emerald-300 placeholder-slate-600 outline-none resize-none"
                ></textarea>
              {:else}
                <div class="flex items-center gap-3">
                  <input
                    type="file"
                    accept=".pem,.crt,.cer"
                    onchange={(e) => handleFileUpload('ca', e)}
                    class="block w-full text-xs text-slate-400 file:mr-3 file:py-1.5 file:px-3 file:rounded-lg file:border-0 file:text-xs file:font-medium file:bg-white/5 file:text-slate-300 hover:file:bg-white/10 cursor-pointer"
                  />
                  {#if tlsCa}
                    <span class="text-xs text-emerald-400 font-mono">Loaded ({tlsCa.length} B)</span>
                  {/if}
                </div>
              {/if}
            </div>

            <!-- Client Certificate -->
            <div class="space-y-1.5">
              <label for="tls-cert" class="block text-xs font-medium text-slate-300">
                Client Certificate (cert.pem)
              </label>
              {#if tlsInputMode === 'text'}
                <textarea
                  id="tls-cert"
                  bind:value={tlsCert}
                  rows={2}
                  placeholder="-----BEGIN CERTIFICATE-----&#10;...&#10;-----END CERTIFICATE-----"
                  class="w-full px-3 py-2 rounded-lg bg-[#0a0b10] border border-white/[0.08] focus:border-emerald-500 text-xs font-mono text-emerald-300 placeholder-slate-600 outline-none resize-none"
                ></textarea>
              {:else}
                <div class="flex items-center gap-3">
                  <input
                    type="file"
                    accept=".pem,.crt,.cer"
                    onchange={(e) => handleFileUpload('cert', e)}
                    class="block w-full text-xs text-slate-400 file:mr-3 file:py-1.5 file:px-3 file:rounded-lg file:border-0 file:text-xs file:font-medium file:bg-white/5 file:text-slate-300 hover:file:bg-white/10 cursor-pointer"
                  />
                  {#if tlsCert}
                    <span class="text-xs text-emerald-400 font-mono">Loaded ({tlsCert.length} B)</span>
                  {/if}
                </div>
              {/if}
            </div>

            <!-- Client Key -->
            <div class="space-y-1.5">
              <label for="tls-key" class="block text-xs font-medium text-slate-300">
                Client Key (key.pem)
              </label>
              {#if tlsInputMode === 'text'}
                <textarea
                  id="tls-key"
                  bind:value={tlsKey}
                  rows={2}
                  placeholder="-----BEGIN RSA PRIVATE KEY-----&#10;...&#10;-----END RSA PRIVATE KEY-----"
                  class="w-full px-3 py-2 rounded-lg bg-[#0a0b10] border border-white/[0.08] focus:border-emerald-500 text-xs font-mono text-emerald-300 placeholder-slate-600 outline-none resize-none"
                ></textarea>
              {:else}
                <div class="flex items-center gap-3">
                  <input
                    type="file"
                    accept=".pem,.key"
                    onchange={(e) => handleFileUpload('key', e)}
                    class="block w-full text-xs text-slate-400 file:mr-3 file:py-1.5 file:px-3 file:rounded-lg file:border-0 file:text-xs file:font-medium file:bg-white/5 file:text-slate-300 hover:file:bg-white/10 cursor-pointer"
                  />
                  {#if tlsKey}
                    <span class="text-xs text-emerald-400 font-mono">Loaded ({tlsKey.length} B)</span>
                  {/if}
                </div>
              {/if}
            </div>
          </div>
        {/if}

        <!-- Is Default Checkbox -->
        <div class="flex items-center gap-3 pt-1">
          <input
            id="env-default"
            type="checkbox"
            bind:checked={isDefault}
            class="w-4 h-4 rounded bg-[#12131c] border-white/[0.2] text-violet-600 focus:ring-violet-500 focus:ring-offset-0 focus:ring-offset-transparent cursor-pointer"
          />
          <label for="env-default" class="text-xs text-slate-300 select-none cursor-pointer">
            Set as Default Environment for new sessions
          </label>
        </div>
      </div>

      <!-- Footer Actions (Pinned) -->
      <div class="px-6 py-4 border-t border-white/[0.08] bg-[#12131c]/80 backdrop-blur-sm flex items-center justify-between shrink-0">
          <button
            type="button"
            onclick={handleTestConnection}
            disabled={isTesting || isSubmitting}
            class="px-4 py-2 rounded-xl bg-white/[0.05] hover:bg-white/[0.1] border border-white/[0.08] text-xs font-medium text-slate-300 hover:text-white transition flex items-center gap-2 disabled:opacity-50"
          >
            {#if isTesting}
              <Loader2 class="w-3.5 h-3.5 animate-spin text-cyan-400" />
              <span>Checking...</span>
            {:else}
              <Zap class="w-3.5 h-3.5 text-cyan-400" />
              <span>Pre-flight Check</span>
            {/if}
          </button>

          <div class="flex items-center gap-2.5">
            <button
              type="button"
              onclick={handleClose}
              disabled={isSubmitting}
              class="px-4 py-2 rounded-xl text-xs font-medium text-slate-400 hover:text-slate-200 transition"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={isSubmitting}
              class="px-5 py-2 rounded-xl bg-gradient-to-r from-violet-600 to-cyan-600 hover:from-violet-500 hover:to-cyan-500 text-xs font-semibold text-white shadow-[0_0_15px_rgba(124,58,237,0.3)] transition flex items-center gap-2 disabled:opacity-50"
            >
              {#if isSubmitting}
                <Loader2 class="w-3.5 h-3.5 animate-spin" />
                <span>Connecting...</span>
              {:else}
                <CheckCircle2 class="w-3.5 h-3.5" />
                <span>Save & Connect</span>
              {/if}
            </button>
          </div>
        </div>
      </form>
    </div>
  </div>
{/if}
