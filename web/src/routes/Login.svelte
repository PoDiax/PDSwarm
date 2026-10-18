<script lang="ts">
  import {
    Lock,
    User as UserIcon,
    Eye,
    EyeOff,
    AlertCircle,
    RefreshCw,
    ArrowRight,
  } from '@lucide/svelte';
  import { auth } from '../lib/stores/auth.svelte';
  import AnimatedLogo from '../lib/components/AnimatedLogo.svelte';

  interface Props {
    onSuccess?: () => void;
  }

  let { onSuccess }: Props = $props();

  let username = $state('admin');
  let password = $state('');
  let showPassword = $state(false);
  let errorMessage = $state<string | null>(null);
  let isSubmitting = $state(false);

  let isFormValid = $derived(
    username.trim().length > 0 &&
    password.length > 0 &&
    !isSubmitting
  );

  async function handleLogin(e: SubmitEvent) {
    e.preventDefault();
    errorMessage = null;

    if (!username.trim() || !password) {
      errorMessage = 'Please provide both username and password.';
      return;
    }

    isSubmitting = true;

    try {
      await auth.login(username.trim(), password);
      if (onSuccess) {
        onSuccess();
      }
    } catch (err: unknown) {
      if (err instanceof Error) {
        errorMessage = err.message;
      } else if (typeof err === 'object' && err !== null && 'message' in err) {
        errorMessage = String((err as { message: unknown }).message);
      } else {
        errorMessage = 'Invalid username or password.';
      }
    } finally {
      isSubmitting = false;
    }
  }
</script>

<div class="relative min-h-screen flex items-center justify-center p-4 bg-[#0a0b10] overflow-hidden">
  <div class="absolute -top-32 -left-32 w-96 h-96 bg-violet-600/15 rounded-full blur-[100px] pointer-events-none"></div>
  <div class="absolute -bottom-32 -right-32 w-96 h-96 bg-cyan-500/10 rounded-full blur-[100px] pointer-events-none"></div>
  <div class="absolute inset-0 bg-[linear-gradient(to_right,rgba(255,255,255,0.02)_1px,transparent_1px),linear-gradient(to_bottom,rgba(255,255,255,0.02)_1px,transparent_1px)] bg-[size:3.5rem_3.5rem] pointer-events-none"></div>

  <div class="relative w-full max-w-md bg-[#12131c]/90 backdrop-blur-xl border border-white/[0.08] shadow-[0_0_50px_-10px_rgba(124,58,237,0.2)] rounded-2xl p-6 sm:p-8 z-10 animate-in fade-in zoom-in-95 duration-200">
    <div class="flex flex-col items-center text-center mb-6">
      <div class="inline-flex items-center justify-center p-3.5 rounded-2xl bg-[#0a0b10]/80 border border-violet-500/30 shadow-[0_0_30px_rgba(124,58,237,0.35)] mb-4">
        <AnimatedLogo class="h-8 w-auto" />
      </div>

      <h1 class="text-2xl font-bold tracking-tight text-white">Sign In</h1>
      <p class="text-sm text-slate-400 mt-1.5">
        Enter your credentials to continue.
      </p>
    </div>

    {#if errorMessage}
      <div class="mb-5 p-3.5 rounded-xl bg-rose-500/10 border border-rose-500/25 text-rose-300 text-sm flex items-start gap-2.5 animate-in fade-in duration-150">
        <AlertCircle class="w-4 h-4 mt-0.5 shrink-0 text-rose-400" />
        <span class="leading-snug">{errorMessage}</span>
      </div>
    {/if}

    <form onsubmit={handleLogin} class="space-y-4">
      <div>
        <label for="login-username" class="block text-xs font-semibold uppercase tracking-wider text-slate-300 mb-1.5">
          Username
        </label>
        <div class="relative">
          <div class="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none text-slate-400">
            <UserIcon class="w-4 h-4" />
          </div>
          <input
            id="login-username"
            type="text"
            bind:value={username}
            placeholder="admin"
            required
            autocomplete="username"
            class="w-full pl-10 pr-4 py-2.5 bg-[#0a0b10]/90 border border-white/[0.1] rounded-xl text-white placeholder-slate-500 text-sm focus:outline-none focus:border-violet-500 focus:ring-1 focus:ring-violet-500 transition-all shadow-inner"
          />
        </div>
      </div>

      <div>
        <div class="flex items-center justify-between mb-1.5">
          <label for="login-password" class="block text-xs font-semibold uppercase tracking-wider text-slate-300">
            Password
          </label>
        </div>
        <div class="relative">
          <div class="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none text-slate-400">
            <Lock class="w-4 h-4" />
          </div>
          <input
            id="login-password"
            type={showPassword ? 'text' : 'password'}
            bind:value={password}
            placeholder="••••••••"
            required
            autocomplete="current-password"
            class="w-full pl-10 pr-10 py-2.5 bg-[#0a0b10]/90 border border-white/[0.1] rounded-xl text-white placeholder-slate-500 text-sm focus:outline-none focus:border-violet-500 focus:ring-1 focus:ring-violet-500 transition-all shadow-inner"
          />
          <button
            type="button"
            onclick={() => showPassword = !showPassword}
            class="absolute inset-y-0 right-0 pr-3.5 flex items-center text-slate-400 hover:text-white transition"
            aria-label={showPassword ? 'Hide password' : 'Show password'}
          >
            {#if showPassword}
              <EyeOff class="w-4 h-4" />
            {:else}
              <Eye class="w-4 h-4" />
            {/if}
          </button>
        </div>
      </div>

      <div class="pt-2">
        <button
          type="submit"
          disabled={!isFormValid}
          class="w-full py-3 px-4 rounded-xl bg-violet-600 hover:bg-violet-500 active:bg-violet-700 text-white font-semibold text-sm shadow-[0_0_20px_-3px_rgba(124,58,237,0.5)] hover:shadow-[0_0_25px_-1px_rgba(124,58,237,0.7)] transition-all flex items-center justify-center gap-2 cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed disabled:shadow-none"
        >
          {#if isSubmitting}
            <RefreshCw class="w-4 h-4 animate-spin text-white" />
            <span>Authenticating...</span>
          {:else}
            <span>Sign In</span>
            <ArrowRight class="w-4 h-4" />
          {/if}
        </button>
      </div>
    </form>

    <div class="mt-6 text-center text-xs text-slate-500">
      Made by <a href="https://github.com/PoDiax/" target="_blank" rel="noopener noreferrer" class="text-slate-400 hover:text-cyan-400 transition font-medium">PoDiax</a>
    </div>
  </div>
</div>
