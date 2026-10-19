<script lang="ts">
  import {
    onMount } from "svelte";
  import {
    Settings as SettingsIcon,
    SlidersHorizontal,
    Users,
    Key,
    Globe,
    Boxes,
    Lock,
    Trash,
    Plus,
    RefreshCw,
    Check,
    Copy,
    CircleAlert,
    CircleCheck,
    LoaderCircle,
    Eye,
    EyeOff,
    Server,
    Clock,
    Cpu,
    HardDrive,
    ExternalLink,
    X,
    Link,
    FolderArchive,
    Power,
    FileText,
    Terminal,
    CircleArrowUp,
    Download,
    ShieldAlert,
  } from '@lucide/svelte';
  import RegistriesManager from "../lib/components/RegistriesManager.svelte";
  import { auth } from "../lib/stores/auth.svelte";
  import {
    systemApi,
    authApi,
    userApi,
    tokenApi,
    templateApi,
    templateRepoApi,
  } from "../lib/api";
  import type {
    SystemInfo,
    User,
    APIToken,
    TemplateListResponse,
    TemplateRepo,
    TemplateSettings,
    LoggingConfig,
    UpdateStatus,
    AuditLogInfo,
  } from "../lib/types";
  import {
    getPortLinkSettings,
    savePortLinkSettings,
    type PortLinkSettings,
  } from "../lib/utils/portLink";
  import {
    imageUpdateStore,
    IMAGE_UPDATE_INTERVALS,
  } from "../lib/stores/imageUpdates.svelte";

  interface Props {
    onNavigate?: (tab: string) => void;
  }

  let { onNavigate }: Props = $props();

  type TabType = "general" | "users" | "tokens" | "registries" | "catalogs";
  let activeTab = $state<TabType>("general");

  // --- General Tab State ---
  let systemInfo = $state<SystemInfo | null>(null);
  let isPingOk = $state(true);
  let refreshInterval = $state<string>(
    typeof window !== "undefined"
      ? localStorage.getItem("pdswarm_refresh_rate") || "5000"
      : "5000",
  );
  let selectedTheme = $state<string>(
    typeof window !== "undefined"
      ? localStorage.getItem("pdswarm_theme") || "obsidian-violet"
      : "obsidian-violet",
  );
  let selectedShell = $state<string>(
    typeof window !== "undefined"
      ? localStorage.getItem("pdswarm_default_shell") || "/bin/sh"
      : "/bin/sh",
  );
  let imageUpdateInterval = $state<number>(imageUpdateStore.checkIntervalMs);
  let isSavingPrefs = $state(false);
  let prefsFeedback = $state<string | null>(null);

  // --- Container Web Links & Port Redirection State ---
  const initialPortSettings = getPortLinkSettings();
  let portCustomHost = $state<string>(initialPortSettings.customHost);
  let portProtocol = $state<"auto" | "http" | "https">(
    initialPortSettings.protocol,
  );
  let isSavingPortSettings = $state(false);
  let portSettingsFeedback = $state<string | null>(null);
  let copiedSnippetKey = $state<string | null>(null);

  let currentBrowserHost = $derived(
    typeof window !== "undefined" ? window.location.hostname : "127.0.0.1",
  );

  let previewResolvedHost = $derived.by(() => {
    if (portCustomHost.trim()) {
      return portCustomHost.trim();
    }
    return currentBrowserHost;
  });

  let previewResolvedProto = $derived.by(() => {
    if (portProtocol === "http") return "http";
    if (portProtocol === "https") return "https";
    if (typeof window !== "undefined" && window.location.protocol) {
      return window.location.protocol.replace(":", "");
    }
    return "http";
  });

  let previewPortUrl = $derived(
    `${previewResolvedProto}://${previewResolvedHost}:8080`,
  );

  function handleSavePortSettings() {
    isSavingPortSettings = true;
    savePortLinkSettings({
      mode: portCustomHost.trim() ? "custom" : "auto",
      customHost: portCustomHost.trim(),
      protocol: portProtocol,
    });
    setTimeout(() => {
      isSavingPortSettings = false;
      portSettingsFeedback = "Global Server IP saved";
      setTimeout(() => (portSettingsFeedback = null), 3000);
    }, 200);
  }

  function copyCodeSnippet(text: string, key: string) {
    if (typeof navigator !== "undefined" && navigator.clipboard) {
      navigator.clipboard.writeText(text);
      copiedSnippetKey = key;
      setTimeout(() => {
        if (copiedSnippetKey === key) copiedSnippetKey = null;
      }, 2000);
    }
  }

  // --- Container Logging & Log Rotation State ---
  let loggingConfig = $state<LoggingConfig | null>(null);
  let isLoadingLogging = $state(false);
  let isSavingLogging = $state(false);
  let loggingFeedback = $state<string | null>(null);
  let loggingError = $state<string | null>(null);
  let copiedSnippet = $state(false);
  let copiedCommand = $state(false);

  let logDriver = $state("json-file");
  let logMaxSize = $state("10m");
  let logMaxFile = $state("3");
  let logCompress = $state(true);

  // --- System Updates & Auto-Update State ---
  let updateStatus = $state<UpdateStatus | null>(null);
  let isLoadingUpdate = $state(false);
  let isCheckingUpdate = $state(false);
  let isTogglingAutoUpdate = $state(false);
  let isApplyingUpdate = $state(false);
  let updateFeedback = $state<string | null>(null);
  let updateError = $state<string | null>(null);

  async function loadUpdateStatus(force = false) {
    isLoadingUpdate = true;
    try {
      updateStatus = await systemApi.getUpdateStatus(force);
    } catch (err: unknown) {
      console.error("Failed to load update status", err);
    } finally {
      isLoadingUpdate = false;
    }
  }

  async function handleCheckForUpdates() {
    isCheckingUpdate = true;
    updateFeedback = null;
    updateError = null;
    try {
      updateStatus = await systemApi.checkUpdate();
      if (updateStatus.update_available) {
        updateFeedback = `New version ${updateStatus.latest_version} is available!`;
      } else {
        updateFeedback = `PDSwarm is up to date (${updateStatus.current_version}).`;
      }
      setTimeout(() => (updateFeedback = null), 6000);
    } catch (err: unknown) {
      updateError =
        err instanceof Error
          ? err.message
          : "Failed to query GitHub for updates";
      setTimeout(() => (updateError = null), 6000);
    } finally {
      isCheckingUpdate = false;
    }
  }

  async function handleToggleAutoUpdate() {
    if (!updateStatus) return;
    isTogglingAutoUpdate = true;
    updateFeedback = null;
    updateError = null;
    try {
      const newTarget = !updateStatus.auto_update_enabled;
      const res = await systemApi.setAutoUpdate(newTarget);
      updateStatus = res;
      updateFeedback = newTarget
        ? "Daily auto-updates enabled (scheduled daily at 04:00 AM)."
        : "Daily auto-updates disabled.";
      setTimeout(() => (updateFeedback = null), 5000);
    } catch (err: unknown) {
      updateError =
        err instanceof Error
          ? err.message
          : "Failed to toggle auto-update timer";
      setTimeout(() => (updateError = null), 5000);
    } finally {
      isTogglingAutoUpdate = false;
    }
  }

  async function handleApplyUpdate() {
    if (
      !confirm(
        "Initiate update now? PDSwarm will download the latest release and restart the service.",
      )
    ) {
      return;
    }
    isApplyingUpdate = true;
    updateFeedback = null;
    updateError = null;
    try {
      const res = await systemApi.applyUpdate();
      updateFeedback = res.message;
      setTimeout(() => {
        loadUpdateStatus();
      }, 5000);
    } catch (err: unknown) {
      updateError =
        err instanceof Error ? err.message : "Failed to initiate update";
      setTimeout(() => (updateError = null), 6000);
    } finally {
      isApplyingUpdate = false;
    }
  }

  function handleThemeChange() {
    if (typeof window !== "undefined") {
      localStorage.setItem("pdswarm_theme", selectedTheme);
      document.documentElement.setAttribute("data-theme", selectedTheme);
    }
  }

  function handleShellChange() {
    if (typeof window !== "undefined") {
      localStorage.setItem("pdswarm_default_shell", selectedShell);
    }
  }

  async function loadLoggingConfig() {
    isLoadingLogging = true;
    loggingError = null;
    try {
      const cfg = await systemApi.getLoggingConfig();
      loggingConfig = cfg;
      logDriver = cfg.log_driver || "json-file";
      logMaxSize = cfg.max_size || "10m";
      logMaxFile = cfg.max_file || "3";
      logCompress = cfg.compress ?? true;
    } catch (err: unknown) {
      loggingError =
        err instanceof Error
          ? err.message
          : "Failed to load logging configuration";
    } finally {
      isLoadingLogging = false;
    }
  }

  async function handleSaveLogging(writeDaemon = false) {
    isSavingLogging = true;
    loggingFeedback = null;
    loggingError = null;
    try {
      const updated = await systemApi.updateLoggingConfig({
        log_driver: logDriver,
        max_size: logMaxSize,
        max_file: logMaxFile,
        compress: logCompress,
        write_daemon_file: writeDaemon,
      });
      loggingConfig = updated;
      loggingFeedback =
        writeDaemon && updated.config_writable
          ? "Settings saved and applied directly to daemon configuration file!"
          : "PDSwarm container logging policy saved successfully!";
      setTimeout(() => (loggingFeedback = null), 4000);
    } catch (err: unknown) {
      loggingError =
        err instanceof Error ? err.message : "Failed to save logging settings";
    } finally {
      isSavingLogging = false;
    }
  }

  function copyToClipboard(text: string, type: "snippet" | "command") {
    if (!text) return;
    navigator.clipboard.writeText(text);
    if (type === "snippet") {
      copiedSnippet = true;
      setTimeout(() => (copiedSnippet = false), 2500);
    } else {
      copiedCommand = true;
      setTimeout(() => (copiedCommand = false), 2500);
    }
  }

  // --- Users Tab State ---
  let users = $state<User[]>([]);
  let isLoadingUsers = $state(false);
  let userError = $state<string | null>(null);

  // --- Security Audit & Fail2ban Logging State ---
  let auditLogInfo = $state<AuditLogInfo | null>(null);
  let isLoadingAuditLogs = $state(false);
  let auditCopied = $state<'filter' | 'jail' | 'path' | null>(null);

  async function loadAuditLogs() {
    isLoadingAuditLogs = true;
    try {
      auditLogInfo = await systemApi.getAuditLogs();
    } catch {
      // ignore
    } finally {
      isLoadingAuditLogs = false;
    }
  }

  function copyAuditSnippet(type: 'filter' | 'jail' | 'path', text: string) {
    if (!text) return;
    navigator.clipboard.writeText(text);
    auditCopied = type;
    setTimeout(() => {
      if (auditCopied === type) auditCopied = null;
    }, 2500);
  }

  // Change Password
  let currentPassword = $state("");
  let newPassword = $state("");
  let confirmPassword = $state("");
  let showCurrentPassword = $state(false);
  let showNewPassword = $state(false);
  let isChangingPassword = $state(false);
  let passwordFeedback = $state<{
    type: "success" | "error";
    message: string;
  } | null>(null);

  // Add User Modal
  let isAddUserModalOpen = $state(false);
  let newUsername = $state("");
  let newUserPassword = $state("");
  let newUserRole = $state<"admin" | "operator" | "viewer">("operator");
  let isCreatingUser = $state(false);
  let addUserError = $state<string | null>(null);

  // Delete User Modal
  let deletingUser = $state<User | null>(null);
  let isDeletingUser = $state(false);

  // --- Tokens Tab State ---
  let tokens = $state<APIToken[]>([]);
  let isLoadingTokens = $state(false);
  let tokenError = $state<string | null>(null);

  // Create Token Modal
  let isCreateTokenModalOpen = $state(false);
  let tokenName = $state("");
  let tokenExpiresDays = $state<number>(30);
  let isGeneratingToken = $state(false);
  let createTokenError = $state<string | null>(null);
  let generatedRawToken = $state<string | null>(null);
  let isCopied = $state(false);

  // Revoke Token Modal
  let revokingToken = $state<APIToken | null>(null);
  let isRevokingToken = $state(false);

  // --- Catalogs Tab State ---
  let catalogData = $state<TemplateListResponse | null>(null);
  let templateSettings = $state<TemplateSettings>({
    disable_default_templates: false,
  });
  let templateRepos = $state<TemplateRepo[]>([]);
  let isLoadingCatalogs = $state(false);
  let isSyncingCatalogs = $state(false);
  let isTogglingDefaults = $state(false);
  let catalogFeedback = $state<string | null>(null);
  let syncingRepoId = $state<string | null>(null);

  // Add Custom Catalog Modal
  let isAddRepoModalOpen = $state(false);
  let newRepoName = $state("");
  let newRepoUrl = $state("");
  let newRepoType = $state<"auto" | "portainer" | "casaos_zip">("auto");
  let newRepoEnabled = $state(true);
  let isCreatingRepo = $state(false);
  let createRepoError = $state<string | null>(null);

  // Delete Custom Catalog Modal
  let deletingRepo = $state<TemplateRepo | null>(null);
  let isDeletingRepo = $state(false);

  async function loadGeneralData() {
    try {
      const ping = await systemApi.ping();
      isPingOk = ping && (ping.status === "OK" || ping.status === "ok");
      systemInfo = await systemApi.getInfo();
    } catch {
      isPingOk = false;
    }
    await loadLoggingConfig();
    await loadUpdateStatus();
  }

  async function loadUsers() {
    isLoadingUsers = true;
    userError = null;
    try {
      users = await userApi.list();
    } catch (err: unknown) {
      userError = err instanceof Error ? err.message : "Failed to load users";
    } finally {
      isLoadingUsers = false;
    }
  }

  async function loadTokens() {
    isLoadingTokens = true;
    tokenError = null;
    try {
      tokens = await tokenApi.list();
    } catch (err: unknown) {
      tokenError =
        err instanceof Error ? err.message : "Failed to load API tokens";
    } finally {
      isLoadingTokens = false;
    }
  }

  async function loadCatalogs() {
    isLoadingCatalogs = true;
    try {
      const [settings, repos, data] = await Promise.all([
        templateRepoApi.getSettings(),
        templateRepoApi.list(),
        templateApi.list(),
      ]);
      templateSettings = settings;
      templateRepos = repos || [];
      catalogData = data;
    } catch {
      // ignore
    } finally {
      isLoadingCatalogs = false;
    }
  }

  function handleSavePreferences() {
    isSavingPrefs = true;
    if (typeof window !== "undefined") {
      localStorage.setItem("pdswarm_refresh_rate", refreshInterval);
      localStorage.setItem("pdswarm_theme", selectedTheme);
      localStorage.setItem("pdswarm_default_shell", selectedShell);
      document.documentElement.setAttribute("data-theme", selectedTheme);
      imageUpdateStore.setInterval(Number(imageUpdateInterval));
      savePortLinkSettings({
        mode: portCustomHost.trim() ? "custom" : "auto",
        customHost: portCustomHost.trim(),
        protocol: portProtocol,
      });
    }
    setTimeout(() => {
      isSavingPrefs = false;
      prefsFeedback = "Web UI preferences saved";
      setTimeout(() => (prefsFeedback = null), 3000);
    }, 300);
  }

  async function handleChangePassword(e: SubmitEvent) {
    e.preventDefault();
    passwordFeedback = null;

    if (!currentPassword) {
      passwordFeedback = {
        type: "error",
        message: "Current password is required",
      };
      return;
    }
    if (!newPassword) {
      passwordFeedback = { type: "error", message: "New password is required" };
      return;
    }
    if (newPassword !== confirmPassword) {
      passwordFeedback = {
        type: "error",
        message: "New passwords do not match",
      };
      return;
    }

    isChangingPassword = true;
    try {
      await authApi.changePassword({
        old_password: currentPassword,
        new_password: newPassword,
      });
      passwordFeedback = {
        type: "success",
        message: "Password changed successfully",
      };
      currentPassword = "";
      newPassword = "";
      confirmPassword = "";
    } catch (err: unknown) {
      passwordFeedback = {
        type: "error",
        message:
          err instanceof Error ? err.message : "Failed to change password",
      };
    } finally {
      isChangingPassword = false;
    }
  }

  async function handleCreateUser(e: SubmitEvent) {
    e.preventDefault();
    addUserError = null;

    if (!newUsername.trim()) {
      addUserError = "Username is required";
      return;
    }
    if (!newUserPassword) {
      addUserError = "Password is required";
      return;
    }

    isCreatingUser = true;
    try {
      await userApi.create({
        username: newUsername.trim(),
        password: newUserPassword,
        role: newUserRole,
      });
      isAddUserModalOpen = false;
      newUsername = "";
      newUserPassword = "";
      newUserRole = "operator";
      await loadUsers();
    } catch (err: unknown) {
      addUserError =
        err instanceof Error ? err.message : "Failed to create user";
    } finally {
      isCreatingUser = false;
    }
  }

  async function handleDeleteUser() {
    if (!deletingUser) return;
    isDeletingUser = true;
    try {
      await userApi.delete(deletingUser.username);
      deletingUser = null;
      await loadUsers();
    } catch (err: unknown) {
      userError = err instanceof Error ? err.message : "Failed to delete user";
    } finally {
      isDeletingUser = false;
    }
  }

  async function handleCreateToken(e: SubmitEvent) {
    e.preventDefault();
    createTokenError = null;

    if (!tokenName.trim()) {
      createTokenError = "Token name is required";
      return;
    }

    isGeneratingToken = true;
    try {
      const res = await tokenApi.create({
        name: tokenName.trim(),
        expires_days: Number(tokenExpiresDays),
      });
      generatedRawToken = res.token;
      await loadTokens();
    } catch (err: unknown) {
      createTokenError =
        err instanceof Error ? err.message : "Failed to generate token";
    } finally {
      isGeneratingToken = false;
    }
  }

  async function copyTokenToClipboard() {
    if (!generatedRawToken) return;
    try {
      await navigator.clipboard.writeText(generatedRawToken);
      isCopied = true;
      setTimeout(() => (isCopied = false), 2500);
    } catch {
      // fallback
    }
  }

  function handleCloseTokenModal() {
    isCreateTokenModalOpen = false;
    generatedRawToken = null;
    tokenName = "";
    tokenExpiresDays = 30;
    createTokenError = null;
    isCopied = false;
  }

  async function handleRevokeToken() {
    if (!revokingToken) return;
    isRevokingToken = true;
    try {
      await tokenApi.revoke(revokingToken.id);
      revokingToken = null;
      await loadTokens();
    } catch (err: unknown) {
      tokenError =
        err instanceof Error ? err.message : "Failed to revoke token";
    } finally {
      isRevokingToken = false;
    }
  }

  async function handleToggleDefaultTemplates() {
    isTogglingDefaults = true;
    catalogFeedback = null;
    try {
      const updated = await templateRepoApi.updateSettings({
        disable_default_templates: !templateSettings.disable_default_templates,
      });
      templateSettings = updated;
      catalogData = await templateApi.list();
      catalogFeedback = updated.disable_default_templates
        ? "Default curated templates disabled. Only custom catalogs will be shown in the App Store."
        : "Default curated templates enabled (35+ apps).";
      setTimeout(() => (catalogFeedback = null), 4000);
    } catch {
      catalogFeedback = "Failed to update template settings";
    } finally {
      isTogglingDefaults = false;
    }
  }

  async function handleToggleRepo(repo: TemplateRepo) {
    try {
      await templateRepoApi.update(repo.id, { enabled: !repo.enabled });
      await loadCatalogs();
    } catch {
      catalogFeedback = "Failed to update repository";
    }
  }

  async function handleSyncSingleRepo(repoId: string) {
    syncingRepoId = repoId;
    catalogFeedback = null;
    try {
      await templateRepoApi.sync(repoId);
      await loadCatalogs();
      catalogFeedback = "Repository successfully synchronized!";
      setTimeout(() => (catalogFeedback = null), 4000);
    } catch (err: unknown) {
      catalogFeedback =
        "Failed to sync repository: " +
        (err instanceof Error ? err.message : String(err));
    } finally {
      syncingRepoId = null;
    }
  }

  async function handleSyncCatalogs() {
    isSyncingCatalogs = true;
    catalogFeedback = null;
    try {
      await templateRepoApi.sync();
      await loadCatalogs();
      catalogFeedback = "All template catalogs successfully synchronized!";
      setTimeout(() => (catalogFeedback = null), 4000);
    } catch {
      catalogFeedback = "Failed to sync catalogs";
    } finally {
      isSyncingCatalogs = false;
    }
  }

  function handleOpenAddRepoModal() {
    newRepoName = "";
    newRepoUrl = "";
    newRepoType = "auto";
    newRepoEnabled = true;
    createRepoError = null;
    isAddRepoModalOpen = true;
  }

  function applyRepoPreset(
    name: string,
    url: string,
    type: "auto" | "portainer" | "casaos_zip",
  ) {
    newRepoName = name;
    newRepoUrl = url;
    newRepoType = type;
  }

  async function handleCreateRepo(e: SubmitEvent) {
    e.preventDefault();
    createRepoError = null;
    if (!newRepoName.trim()) {
      createRepoError = "Repository name is required";
      return;
    }
    if (!newRepoUrl.trim()) {
      createRepoError = "Repository URL is required";
      return;
    }

    isCreatingRepo = true;
    try {
      await templateRepoApi.create({
        name: newRepoName.trim(),
        url: newRepoUrl.trim(),
        type: newRepoType,
        enabled: newRepoEnabled,
      });
      isAddRepoModalOpen = false;
      await loadCatalogs();
      catalogFeedback = `Added catalog "${newRepoName}" and triggered initial sync.`;
      setTimeout(() => (catalogFeedback = null), 4000);
    } catch (err: unknown) {
      createRepoError =
        err instanceof Error
          ? err.message
          : "Failed to add template repository";
    } finally {
      isCreatingRepo = false;
    }
  }

  async function handleDeleteRepo() {
    if (!deletingRepo) return;
    isDeletingRepo = true;
    try {
      await templateRepoApi.delete(deletingRepo.id);
      deletingRepo = null;
      await loadCatalogs();
      catalogFeedback = "Repository deleted successfully";
      setTimeout(() => (catalogFeedback = null), 4000);
    } catch {
      catalogFeedback = "Failed to delete repository";
    } finally {
      isDeletingRepo = false;
    }
  }

  function formatDate(d?: string | null): string {
    if (!d) return "Never";
    try {
      return new Date(d).toLocaleDateString(undefined, {
        year: "numeric",
        month: "short",
        day: "numeric",
        hour: "2-digit",
        minute: "2-digit",
      });
    } catch {
      return d;
    }
  }

  function formatMemory(bytes?: number): string {
    if (!bytes) return "N/A";
    const gb = bytes / (1024 * 1024 * 1024);
    return `${gb.toFixed(1)} GB`;
  }

  onMount(() => {
    loadGeneralData();
    loadUsers();
    loadTokens();
    loadCatalogs();
    loadAuditLogs();
  });
</script>

<div class="space-y-6 pb-12 animate-in fade-in duration-200">
  <!-- Header with Glow -->
  <div
    class="flex flex-col md:flex-row md:items-center justify-between gap-4 pb-4 border-b border-white/[0.08]"
  >
    <div class="flex items-center gap-3">
      <div
        class="w-10 h-10 rounded-xl bg-violet-600/20 border border-violet-500/30 flex items-center justify-center text-violet-400 shadow-[0_0_20px_rgba(124,58,237,0.3)]"
      >
        <SettingsIcon class="w-5 h-5" />
      </div>
      <div>
        <div class="flex items-center gap-2">
          <h1 class="text-xl font-bold tracking-tight text-white">
            System Settings & Access
          </h1>
        </div>
        <p class="text-xs text-slate-400 mt-0.5">
          Manage system preferences, authentication, API access tokens,
          registries, and app store sources
        </p>
      </div>
    </div>
  </div>

  <!-- Tabs Navigation -->
  <div
    class="flex items-center gap-2 border-b border-white/[0.08] overflow-x-auto pb-px"
  >
    <button
      onclick={() => (activeTab = "general")}
      class="flex items-center gap-2 px-4 py-2.5 text-xs font-semibold rounded-t-xl transition-all border-b-2 {activeTab ===
      'general'
        ? 'bg-[#161724] text-violet-400 border-violet-500 shadow-[0_0_15px_rgba(124,58,237,0.2)]'
        : 'text-slate-400 hover:text-slate-200 hover:bg-white/[0.03] border-transparent'}"
    >
      <SlidersHorizontal class="w-4 h-4" />
      <span>General</span>
    </button>

    <button
      onclick={() => (activeTab = "users")}
      class="flex items-center gap-2 px-4 py-2.5 text-xs font-semibold rounded-t-xl transition-all border-b-2 {activeTab ===
      'users'
        ? 'bg-[#161724] text-violet-400 border-violet-500 shadow-[0_0_15px_rgba(124,58,237,0.2)]'
        : 'text-slate-400 hover:text-slate-200 hover:bg-white/[0.03] border-transparent'}"
    >
      <Users class="w-4 h-4" />
      <span>Users & Password</span>
      {#if users.length > 0}
        <span
          class="ml-1 px-1.5 py-0.2 rounded-full text-[10px] font-mono bg-white/10 text-slate-300"
        >
          {users.length}
        </span>
      {/if}
    </button>

    <button
      onclick={() => (activeTab = "tokens")}
      class="flex items-center gap-2 px-4 py-2.5 text-xs font-semibold rounded-t-xl transition-all border-b-2 {activeTab ===
      'tokens'
        ? 'bg-[#161724] text-violet-400 border-violet-500 shadow-[0_0_15px_rgba(124,58,237,0.2)]'
        : 'text-slate-400 hover:text-slate-200 hover:bg-white/[0.03] border-transparent'}"
    >
      <Key class="w-4 h-4" />
      <span>API Access Tokens</span>
      {#if tokens.length > 0}
        <span
          class="ml-1 px-1.5 py-0.2 rounded-full text-[10px] font-mono bg-white/10 text-slate-300"
        >
          {tokens.length}
        </span>
      {/if}
    </button>

    <button
      onclick={() => (activeTab = "registries")}
      class="flex items-center gap-2 px-4 py-2.5 text-xs font-semibold rounded-t-xl transition-all border-b-2 {activeTab ===
      'registries'
        ? 'bg-[#161724] text-violet-400 border-violet-500 shadow-[0_0_15px_rgba(124,58,237,0.2)]'
        : 'text-slate-400 hover:text-slate-200 hover:bg-white/[0.03] border-transparent'}"
    >
      <Globe class="w-4 h-4" />
      <span>Registries</span>
    </button>

    <button
      onclick={() => (activeTab = "catalogs")}
      class="flex items-center gap-2 px-4 py-2.5 text-xs font-semibold rounded-t-xl transition-all border-b-2 {activeTab ===
      'catalogs'
        ? 'bg-[#161724] text-violet-400 border-violet-500 shadow-[0_0_15px_rgba(124,58,237,0.2)]'
        : 'text-slate-400 hover:text-slate-200 hover:bg-white/[0.03] border-transparent'}"
    >
      <Boxes class="w-4 h-4" />
      <span>App Catalogs</span>
      {#if catalogData?.total}
        <span
          class="ml-1 px-1.5 py-0.2 rounded-full text-[10px] font-mono bg-cyan-500/20 text-cyan-300"
        >
          {catalogData.total}
        </span>
      {/if}
    </button>
  </div>

  <!-- TAB CONTENT -->

  <!-- 1. GENERAL TAB -->
  {#if activeTab === "general"}
    <div class="space-y-6">
      <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
        <!-- System Telemetry Card -->
        <div
          class="p-6 rounded-2xl bg-[#12131c] border border-white/[0.08] shadow-xl space-y-5"
        >
          <div
            class="flex items-center justify-between border-b border-white/[0.08] pb-3"
          >
            <div class="flex items-center gap-2.5">
              <Server class="w-4 h-4 text-violet-400" />
              <h2 class="text-sm font-bold text-white uppercase tracking-wider">
                Engine & System Telemetry
              </h2>
            </div>
            <div class="flex items-center gap-2">
              <span class="relative flex h-2 w-2">
                <span
                  class="animate-ping absolute inline-flex h-full w-full rounded-full {isPingOk
                    ? 'bg-emerald-400'
                    : 'bg-rose-400'} opacity-75"
                ></span>
                <span
                  class="relative inline-flex rounded-full h-2 w-2 {isPingOk
                    ? 'bg-emerald-500 shadow-[0_0_8px_#10b981]'
                    : 'bg-rose-500'}"
                ></span>
              </span>
              <span
                class="text-xs font-mono {isPingOk
                  ? 'text-emerald-400'
                  : 'text-rose-400'}"
              >
                {isPingOk ? "Engine Connected" : "Engine Offline"}
              </span>
            </div>
          </div>

          <div class="grid grid-cols-2 gap-4">
            <div
              class="p-3.5 rounded-xl bg-[#0a0b10] border border-white/[0.04]"
            >
              <span class="text-[11px] text-slate-500 font-mono"
                >Container Engine</span
              >
              <div
                class="text-xs font-bold text-white mt-1 font-mono flex items-center gap-1.5 truncate"
              >
                {#if (systemInfo?.engine_type || "").toLowerCase() === "podman" || (systemInfo?.ServerVersion || "")
                    .toLowerCase()
                    .includes("podman") || (systemInfo?.server_version || "")
                    .toLowerCase()
                    .includes("podman")}
                  <span class="w-2 h-2 rounded-full bg-purple-400 shrink-0"
                  ></span>
                  <span class="text-purple-300"
                    >Podman {systemInfo?.engine_version ||
                      systemInfo?.server_version ||
                      ""}</span
                  >
                {:else}
                  <span class="w-2 h-2 rounded-full bg-cyan-400 shrink-0"
                  ></span>
                  <span class="text-cyan-300"
                    >{systemInfo?.server_version ||
                      systemInfo?.ServerVersion ||
                      "Docker Engine"}</span
                  >
                {/if}
              </div>
            </div>

            <div
              class="p-3.5 rounded-xl bg-[#0a0b10] border border-white/[0.04]"
            >
              <span class="text-[11px] text-slate-500 font-mono"
                >OS / Architecture</span
              >
              <div
                class="text-xs font-bold text-white mt-1 truncate"
                title={systemInfo?.os || systemInfo?.OperatingSystem || "Linux"}
              >
                {systemInfo?.os || systemInfo?.OperatingSystem || "Linux"}
              </div>
            </div>

            <div
              class="p-3.5 rounded-xl bg-[#0a0b10] border border-white/[0.04]"
            >
              <span class="text-[11px] text-slate-500 font-mono">Host CPUs</span
              >
              <div
                class="text-xs font-bold text-cyan-400 mt-1 flex items-center gap-1.5 font-mono"
              >
                <Cpu class="w-3.5 h-3.5" />
                <span>{systemInfo?.cpus || systemInfo?.NCPU || 1} Cores</span>
              </div>
            </div>

            <div
              class="p-3.5 rounded-xl bg-[#0a0b10] border border-white/[0.04]"
            >
              <span class="text-[11px] text-slate-500 font-mono"
                >Total Memory</span
              >
              <div
                class="text-xs font-bold text-violet-400 mt-1 flex items-center gap-1.5 font-mono"
              >
                <HardDrive class="w-3.5 h-3.5" />
                <span
                  >{formatMemory(
                    systemInfo?.memory_bytes || systemInfo?.MemTotal,
                  )}</span
                >
              </div>
            </div>
          </div>

          <!-- Socket Details -->
          <div
            class="p-4 rounded-xl bg-[#0a0b10] border border-white/[0.06] space-y-2"
          >
            <div class="flex items-center justify-between text-xs">
              <span class="text-slate-400">Daemon Socket</span>
              <span class="font-mono text-slate-300">/var/run/docker.sock</span>
            </div>
            <div class="flex items-center justify-between text-xs">
              <span class="text-slate-400">Total Containers</span>
              <span class="font-mono text-white"
                >{systemInfo?.containers ?? systemInfo?.Containers ?? 0} ({systemInfo?.containers_running ??
                  systemInfo?.ContainersRunning ??
                  0} active)</span
              >
            </div>
            <div class="flex items-center justify-between text-xs">
              <span class="text-slate-400">Stored Images</span>
              <span class="font-mono text-white"
                >{systemInfo?.images ?? systemInfo?.Images ?? 0}</span
              >
            </div>
          </div>
        </div>

        <!-- Web UI Preferences Card -->
        <div
          class="p-6 rounded-2xl bg-[#12131c] border border-white/[0.08] shadow-xl space-y-5"
        >
          <div
            class="flex items-center justify-between border-b border-white/[0.08] pb-3"
          >
            <div class="flex items-center gap-2.5">
              <SlidersHorizontal class="w-4 h-4 text-cyan-400" />
              <h2 class="text-sm font-bold text-white uppercase tracking-wider">
                Web UI Preferences
              </h2>
            </div>
            {#if prefsFeedback}
              <span
                class="text-xs text-emerald-400 font-medium flex items-center gap-1"
              >
                <Check class="w-3.5 h-3.5" />
                {prefsFeedback}
              </span>
            {/if}
          </div>

          <div class="space-y-4">
            <div>
              <label
                for="refresh-interval-select"
                class="block text-xs font-medium text-slate-300 mb-1.5"
              >
                Live Telemetry Auto-Refresh Interval
              </label>
              <select
                id="refresh-interval-select"
                bind:value={refreshInterval}
                class="w-full px-3.5 py-2.5 rounded-xl bg-[#0a0b10] border border-white/[0.08] text-white text-xs focus:outline-none focus:border-violet-500 transition"
              >
                <option value="2000">2 Seconds (High frequency)</option>
                <option value="5000">5 Seconds (Recommended default)</option>
                <option value="10000">10 Seconds (Low overhead)</option>
                <option value="30000">30 Seconds</option>
                <option value="0">Disabled (Manual refresh only)</option>
              </select>
              <p class="text-[11px] text-slate-500 mt-1.5">
                Controls the polling frequency of container stats, metrics, and
                health indicators.
              </p>
            </div>

            <div>
              <label
                for="image-update-interval-select"
                class="block text-xs font-medium text-slate-300 mb-1.5"
              >
                Image Updates Auto-Check Frequency
              </label>
              <select
                id="image-update-interval-select"
                bind:value={imageUpdateInterval}
                class="w-full px-3.5 py-2.5 rounded-xl bg-[#0a0b10] border border-white/[0.08] text-white text-xs focus:outline-none focus:border-violet-500 transition"
              >
                {#each IMAGE_UPDATE_INTERVALS as opt}
                  <option value={opt.value}>{opt.label}</option>
                {/each}
              </select>
              <div class="flex items-center justify-between text-[11px] text-slate-500 mt-1.5">
                <span>
                  Automatically queries remote registries in the background using fast batch requests to detect new image versions.
                </span>
                {#if imageUpdateStore.lastCheckedTimestamp > 0}
                  <span class="text-violet-400 font-mono shrink-0 ml-2">
                    Checked {Math.max(1, Math.round((Date.now() - imageUpdateStore.lastCheckedTimestamp) / 60000))}m ago
                  </span>
                {/if}
              </div>
            </div>

            <div
              class="p-4 rounded-xl bg-[#0a0b10] border border-white/[0.06] space-y-3"
            >
              <div class="flex items-center justify-between">
                <div>
                  <div class="text-xs font-semibold text-slate-200">
                    Terminal Shell Preset
                  </div>
                  <div class="text-[11px] text-slate-500">
                    Default shell used when attaching to container exec
                  </div>
                </div>
                <select
                  bind:value={selectedShell}
                  onchange={handleShellChange}
                  class="bg-[#0d0e17] border border-white/[0.12] focus:border-violet-500/70 rounded-lg px-2.5 py-1 text-xs font-mono text-violet-300 outline-none transition cursor-pointer"
                >
                  <option value="/bin/sh">/bin/sh (Standard)</option>
                  <option value="/bin/bash">/bin/bash (Bash)</option>
                  <option value="/bin/zsh">/bin/zsh (Zsh)</option>
                  <option value="sh">sh</option>
                  <option value="bash">bash</option>
                </select>
              </div>

              <div
                class="flex items-center justify-between border-t border-white/[0.04] pt-2.5"
              >
                <div>
                  <div class="text-xs font-semibold text-slate-200">
                    Theme Preset
                  </div>
                </div>
                <select
                  bind:value={selectedTheme}
                  onchange={handleThemeChange}
                  class="bg-[#0d0e17] border border-white/[0.12] focus:border-violet-500/70 rounded-lg px-2.5 py-1 text-xs font-mono text-cyan-300 outline-none transition cursor-pointer"
                >
                  <option value="obsidian-violet"
                    >Obsidian Violet (Default)</option
                  >
                  <option value="midnight-dark">Midnight Dark</option>
                  <option value="oled-black">OLED Pure Black</option>
                  <option value="nordic-slate">Nordic Slate</option>
                  <option value="emerald-matrix">Emerald Matrix</option>
                  <option value="cyber-cyan">Cyber Cyan</option>
                </select>
              </div>
            </div>

            <div class="pt-2 flex justify-end">
              <button
                onclick={handleSavePreferences}
                disabled={isSavingPrefs}
                class="flex items-center gap-2 px-4 py-2 rounded-xl bg-violet-600 hover:bg-violet-500 text-white text-xs font-semibold shadow-[0_0_15px_rgba(124,58,237,0.3)] transition disabled:opacity-50"
              >
                {#if isSavingPrefs}
                  <LoaderCircle class="w-3.5 h-3.5 animate-spin" />
                  <span>Saving...</span>
                {:else}
                  <Check class="w-3.5 h-3.5" />
                  <span>Save Preferences</span>
                {/if}
              </button>
            </div>
          </div>
        </div>
      </div>

      <!-- Container Web Links & Port Redirection Card -->
      <div
        class="p-6 rounded-2xl bg-[#12131c] border border-white/[0.08] shadow-xl space-y-6"
      >
        <div
          class="flex flex-col sm:flex-row sm:items-center justify-between pb-3 border-b border-white/[0.08] gap-3"
        >
          <div class="flex items-center gap-2.5">
            <div
              class="w-8 h-8 rounded-xl bg-cyan-500/10 border border-cyan-500/25 flex items-center justify-center text-cyan-400"
            >
              <Globe class="w-4 h-4" />
            </div>
            <div>
              <h2 class="text-sm font-bold text-white uppercase tracking-wider">
                Container Web Links & Port Redirection
              </h2>
              <p class="text-[11px] text-slate-400">
                Configure default IP/host when clicking exposed ports, and
                assign custom web URLs per app
              </p>
            </div>
          </div>
          {#if portSettingsFeedback}
            <span
              class="text-xs text-emerald-400 font-medium flex items-center gap-1.5 bg-emerald-500/10 px-3 py-1 rounded-lg border border-emerald-500/20"
            >
              <Check class="w-3.5 h-3.5" />
              {portSettingsFeedback}
            </span>
          {/if}
        </div>

        <div class="grid grid-cols-1 lg:grid-cols-12 gap-6">
          <!-- Left Column: Global Server IP / Host (5 cols) -->
          <div class="lg:col-span-5 space-y-4">
            <div>
              <label
                for="global-server-ip-input"
                class="block text-xs font-semibold text-slate-200 mb-1"
              >
                Global Server IP / Hostname
              </label>
              <p class="text-[11px] text-slate-400 mb-3 leading-relaxed">
                Enter your server's LAN IP or domain. When clicking any
                container's exposed port badge (e.g. <span
                  class="font-mono text-cyan-300">8080</span
                >), PDSwarm opens this address.
              </p>

              <div class="space-y-2">
                <input
                  id="global-server-ip-input"
                  type="text"
                  bind:value={portCustomHost}
                  placeholder="e.g. [IP_ADDRESS] or server.lan (empty = use browser host)"
                  class="w-full px-3.5 py-2.5 rounded-xl bg-[#0a0b10] border border-white/[0.12] text-white text-xs font-mono placeholder:text-slate-600 focus:outline-none focus:border-cyan-500 transition"
                />

                <div class="flex flex-wrap gap-2 pt-0.5">
                  <button
                    type="button"
                    onclick={() => (portCustomHost = currentBrowserHost)}
                    class="px-2 py-1 rounded-lg bg-cyan-500/10 hover:bg-cyan-500/20 text-cyan-300 border border-cyan-500/25 text-[11px] font-mono transition"
                  >
                    Use Browser IP: {currentBrowserHost}
                  </button>
                  <button
                    type="button"
                    onclick={() => (portCustomHost = "localhost")}
                    class="px-2 py-1 rounded-lg bg-white/[0.05] hover:bg-white/[0.1] text-slate-300 border border-white/[0.08] text-[11px] font-mono transition"
                  >
                    localhost
                  </button>
                  {#if portCustomHost.trim()}
                    <button
                      type="button"
                      onclick={() => (portCustomHost = "")}
                      class="px-2 py-1 rounded-lg hover:bg-rose-500/10 text-slate-500 hover:text-rose-400 text-[11px] font-mono transition"
                    >
                      Reset to Auto
                    </button>
                  {/if}
                </div>
              </div>
            </div>

            <!-- Protocol Selection -->
            <div
              class="p-3.5 rounded-xl bg-[#0a0b10] border border-white/[0.06] flex items-center justify-between"
            >
              <div>
                <div class="text-xs font-semibold text-slate-200">
                  Default Protocol
                </div>
                <div class="text-[11px] text-slate-500">
                  Scheme used for port links
                </div>
              </div>
              <select
                bind:value={portProtocol}
                class="bg-[#0d0e17] border border-white/[0.12] focus:border-cyan-500/70 rounded-lg px-2.5 py-1 text-xs font-mono text-cyan-300 outline-none transition cursor-pointer"
              >
                <option value="auto"
                  >Auto ({typeof window !== "undefined"
                    ? window.location.protocol.replace(":", "")
                    : "http"})</option
                >
                <option value="http">HTTP (http://)</option>
                <option value="https">HTTPS (https://)</option>
              </select>
            </div>

            <!-- Live URL Preview -->
            <div
              class="p-3.5 rounded-xl bg-gradient-to-r from-cyan-950/30 to-violet-950/20 border border-cyan-500/20 space-y-1.5"
            >
              <div class="text-[11px] font-medium text-slate-400">
                Clicking port 8080 will open:
              </div>
              <div class="flex items-center justify-between gap-2">
                <a
                  href={previewPortUrl}
                  target="_blank"
                  rel="noreferrer"
                  class="font-mono text-xs text-cyan-300 hover:text-cyan-200 underline decoration-cyan-500/40 hover:decoration-cyan-400 flex items-center gap-1.5 truncate"
                >
                  <ExternalLink class="w-3.5 h-3.5 shrink-0" />
                  <span class="truncate">{previewPortUrl}</span>
                </a>
                <span class="text-[10px] text-slate-500 font-mono shrink-0"
                  >Click to test</span
                >
              </div>
            </div>

            <div class="flex justify-end pt-1">
              <button
                onclick={handleSavePortSettings}
                disabled={isSavingPortSettings}
                class="flex items-center gap-2 px-4 py-2 rounded-xl bg-cyan-600 hover:bg-cyan-500 text-white text-xs font-semibold shadow-[0_0_15px_rgba(6,182,212,0.3)] transition disabled:opacity-50"
              >
                {#if isSavingPortSettings}
                  <LoaderCircle class="w-3.5 h-3.5 animate-spin" />
                  <span>Saving...</span>
                {:else}
                  <Check class="w-3.5 h-3.5" />
                  <span>Save Link Settings</span>
                {/if}
              </button>
            </div>
          </div>

          <!-- Right Column: Container Labels Guide & Examples (7 cols) -->
          <div class="lg:col-span-7 space-y-4">
            <div
              class="p-4 rounded-xl bg-[#0a0b10] border border-white/[0.06] space-y-3.5"
            >
              <div class="flex items-center justify-between">
                <div class="flex items-center gap-2">
                  <Link class="w-4 h-4 text-violet-400" />
                  <span
                    class="text-xs font-bold text-white uppercase tracking-wider"
                    >Per-Container Custom URLs & Labels</span
                  >
                </div>
                <span
                  class="text-[10px] font-mono px-2 py-0.5 rounded-full bg-violet-500/10 text-violet-300 border border-violet-500/20"
                >
                  Container Labels
                </span>
              </div>
              <p class="text-[11px] text-slate-400 leading-relaxed">
                Add Docker labels to your containers to assign custom web
                addresses or override the IP. PDSwarm will display an instant <span
                  class="text-cyan-300 font-mono">🌐 Web App</span
                > button in the container list and detail view!
              </p>

              <!-- Supported Labels Cards -->
              <div class="space-y-2.5 text-xs">
                <div
                  class="p-3 rounded-lg bg-[#12131c] border border-white/[0.04] space-y-1"
                >
                  <div class="flex items-center justify-between font-mono">
                    <span class="text-cyan-400 font-semibold">pdswarm.url</span>
                    <span class="text-[11px] text-slate-500"
                      >Direct App URL</span
                    >
                  </div>
                  <p class="text-[11px] text-slate-400">
                    Directly opens this exact URL when clicking the app badge.
                    Works even if the container publishes <strong
                      >no host ports</strong
                    > (e.g. reverse-proxied behind Traefik, Nginx Proxy Manager,
                    or Cloudflare Tunnel).
                  </p>
                  <div class="text-[11px] font-mono text-emerald-400/90 pt-0.5">
                    Example: <span class="text-slate-300"
                      >https://vaultwarden.myhomelab.org</span
                    >
                    or
                    <span class="text-slate-300">http://192.168.1.50:8080</span>
                  </div>
                </div>

                <div
                  class="p-3 rounded-lg bg-[#12131c] border border-white/[0.04] space-y-1"
                >
                  <div class="flex items-center justify-between font-mono">
                    <span class="text-violet-400 font-semibold"
                      >pdswarm.host</span
                    >
                    <span class="text-[11px] text-slate-500"
                      >Per-Container Host Override</span
                    >
                  </div>
                  <p class="text-[11px] text-slate-400">
                    Overrides the host IP or domain used for this container's
                    exposed ports (e.g. for containers bound to a macvlan IP or
                    secondary interface).
                  </p>
                  <div class="text-[11px] font-mono text-emerald-400/90 pt-0.5">
                    Example: <span class="text-slate-300">192.168.50.99</span>
                    or <span class="text-slate-300">pihole.local</span>
                  </div>
                </div>

                <div
                  class="p-3 rounded-lg bg-[#12131c] border border-white/[0.04] space-y-1"
                >
                  <div class="flex items-center justify-between font-mono">
                    <span class="text-amber-400 font-semibold"
                      >traefik.http.routers.*.rule</span
                    >
                    <span
                      class="text-[10px] font-sans px-1.5 py-0.2 rounded bg-amber-500/10 text-amber-300"
                      >Auto-Detected</span
                    >
                  </div>
                  <p class="text-[11px] text-slate-400">
                    If you use Traefik with <span
                      class="font-mono text-slate-300"
                      >Host(`sub.domain.com`)</span
                    >, PDSwarm automatically detects the rule and creates a
                    clickable HTTPS link without any extra labels needed!
                  </p>
                </div>
              </div>

              <!-- Ready-to-copy code snippets -->
              <div class="pt-2 space-y-2">
                <div
                  class="flex items-center justify-between text-[11px] text-slate-400"
                >
                  <span class="font-semibold text-slate-300"
                    >Example: docker-compose.yml</span
                  >
                  <button
                    onclick={() =>
                      copyCodeSnippet(
                        `services:\n  my-web-app:\n    image: nginx:alpine\n    ports:\n      - "8080:80"\n    labels:\n      - "pdswarm.url=https://my-app.local.home"`,
                        "compose",
                      )}
                    class="text-[11px] font-mono flex items-center gap-1 text-cyan-400 hover:text-cyan-300 transition"
                  >
                    {#if copiedSnippetKey === "compose"}
                      <Check class="w-3 h-3 text-emerald-400" />
                      <span class="text-emerald-400">Copied!</span>
                    {:else}
                      <Copy class="w-3 h-3" />
                      <span>Copy snippet</span>
                    {/if}
                  </button>
                </div>
                <pre
                  class="p-3 rounded-xl bg-[#07080c] border border-white/[0.08] text-[11px] font-mono text-slate-300 overflow-x-auto leading-relaxed"><code
                    >services:
  my-web-app:
    image: nginx:alpine
    ports:
      - "8080:80"
    labels:
      - "pdswarm.url=https://my-app.local.home"</code
                  ></pre>
              </div>

              <div class="space-y-2">
                <div
                  class="flex items-center justify-between text-[11px] text-slate-400"
                >
                  <span class="font-semibold text-slate-300"
                    >Example: docker run</span
                  >
                  <button
                    onclick={() =>
                      copyCodeSnippet(
                        `docker run -d --name my-app \\\n  --label "pdswarm.url=https://my-app.local.home" \\\n  -p 8080:80 nginx:alpine`,
                        "docker-run",
                      )}
                    class="text-[11px] font-mono flex items-center gap-1 text-cyan-400 hover:text-cyan-300 transition"
                  >
                    {#if copiedSnippetKey === "docker-run"}
                      <Check class="w-3 h-3 text-emerald-400" />
                      <span class="text-emerald-400">Copied!</span>
                    {:else}
                      <Copy class="w-3 h-3" />
                      <span>Copy snippet</span>
                    {/if}
                  </button>
                </div>
                <pre
                  class="p-3 rounded-xl bg-[#07080c] border border-white/[0.08] text-[11px] font-mono text-slate-300 overflow-x-auto leading-relaxed"><code
                    >docker run -d --name my-app \
  --label "pdswarm.url=https://my-app.local.home" \
  -p 8080:80 nginx:alpine</code
                  ></pre>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- System Updates & Version Management Card -->
      <div
        class="p-6 rounded-2xl bg-[#12131c] border border-white/[0.08] shadow-xl space-y-5"
      >
        <div
          class="flex flex-col sm:flex-row sm:items-center justify-between pb-3 border-b border-white/[0.08] gap-3"
        >
          <div class="flex items-center gap-2.5">
            <div
              class="w-8 h-8 rounded-xl bg-violet-500/10 border border-violet-500/25 flex items-center justify-center text-violet-400"
            >
              <CircleArrowUp class="w-4 h-4" />
            </div>
            <div>
              <h2 class="text-sm font-bold text-white uppercase tracking-wider">
                System Updates & Version Management
              </h2>
              <p class="text-[11px] text-slate-400">
                Release tracking, manual upgrades, and automated daily
                background updates
              </p>
            </div>
          </div>
          <div class="flex items-center gap-2">
            {#if isLoadingUpdate || isCheckingUpdate}
              <span
                class="text-xs text-slate-400 flex items-center gap-1 font-mono"
              >
                <LoaderCircle class="w-3.5 h-3.5 animate-spin text-violet-400" />
                Checking releases...
              </span>
            {:else if updateStatus?.update_available}
              <span
                class="inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-semibold bg-amber-500/10 text-amber-300 border border-amber-500/30 shadow-[0_0_12px_rgba(245,158,11,0.2)]"
              >
                <span class="w-2 h-2 rounded-full bg-amber-400 animate-pulse"
                ></span>
                Update Available ({updateStatus.latest_version})
              </span>
            {:else}
              <span
                class="inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-semibold bg-emerald-500/10 text-emerald-400 border border-emerald-500/25"
              >
                <span class="w-2 h-2 rounded-full bg-emerald-400"></span>
                Up to Date ({updateStatus?.current_version || "v0.2.0"})
              </span>
            {/if}
          </div>
        </div>

        {#if updateFeedback}
          <div
            class="p-3.5 rounded-xl border border-emerald-500/30 bg-emerald-500/10 text-emerald-300 text-xs flex items-center justify-between"
          >
            <div class="flex items-center gap-2">
              <CircleCheck class="w-4 h-4 shrink-0 text-emerald-400" />
              <span>{updateFeedback}</span>
            </div>
            <button
              onclick={() => (updateFeedback = null)}
              class="text-emerald-400 hover:text-white text-xs">&times;</button
            >
          </div>
        {/if}

        {#if updateError}
          <div
            class="p-3.5 rounded-xl border border-rose-500/30 bg-rose-500/10 text-rose-300 text-xs flex items-center justify-between"
          >
            <div class="flex items-center gap-2">
              <CircleAlert class="w-4 h-4 shrink-0 text-rose-400" />
              <span>{updateError}</span>
            </div>
            <button
              onclick={() => (updateError = null)}
              class="text-rose-400 hover:text-white text-xs">&times;</button
            >
          </div>
        {/if}

        <div class="grid grid-cols-1 lg:grid-cols-2 gap-5">
          <!-- Left Column: Version & Release Details -->
          <div
            class="p-4 rounded-xl bg-[#0a0b10] border border-white/[0.06] flex flex-col justify-between space-y-4"
          >
            <div class="space-y-3">
              <div
                class="flex items-center justify-between border-b border-white/[0.04] pb-2.5"
              >
                <div>
                  <span class="text-[11px] text-slate-500 font-mono"
                    >Installed Version</span
                  >
                  <div
                    class="text-sm font-bold text-white font-mono mt-0.5 flex items-center gap-2"
                  >
                    <span>{updateStatus?.current_version || "v0.2.0"}</span>
                    <span
                      class="px-2 py-0.5 rounded text-[10px] font-mono font-semibold bg-violet-500/20 text-violet-300 border border-violet-500/30"
                    >
                      GNU AGPLv3
                    </span>
                  </div>
                </div>
                <div class="text-right">
                  <span class="text-[11px] text-slate-500 font-mono"
                    >Latest Available</span
                  >
                  <div
                    class="text-sm font-bold {updateStatus?.update_available
                      ? 'text-amber-400'
                      : 'text-emerald-400'} font-mono mt-0.5"
                  >
                    {updateStatus?.latest_version ||
                      updateStatus?.current_version ||
                      "v0.2.0"}
                  </div>
                </div>
              </div>

              <div class="flex items-center justify-between text-xs">
                <span class="text-slate-400">Deployment Type</span>
                <span class="font-mono text-slate-200 capitalize">
                  {#if updateStatus?.install_type === "docker"}
                    Docker Container
                  {:else if updateStatus?.install_type === "systemd"}
                    Linux Service (systemd)
                  {:else}
                    Standalone Binary
                  {/if}
                </span>
              </div>

              {#if updateStatus?.published_at}
                <div class="flex items-center justify-between text-xs">
                  <span class="text-slate-400">Release Published</span>
                  <span class="font-mono text-slate-400"
                    >{formatDate(updateStatus.published_at)}</span
                  >
                </div>
              {/if}

              {#if updateStatus?.release_url}
                <div class="pt-1">
                  <a
                    href={updateStatus.release_url}
                    target="_blank"
                    rel="noopener noreferrer"
                    class="inline-flex items-center gap-1.5 text-xs text-violet-400 hover:text-violet-300 transition"
                  >
                    <span>View GitHub Release Notes</span>
                    <ExternalLink class="w-3.5 h-3.5" />
                  </a>
                </div>
              {/if}
            </div>

            <!-- Action buttons -->
            <div
              class="pt-3 border-t border-white/[0.04] flex flex-wrap items-center gap-2.5"
            >
              <button
                type="button"
                onclick={handleCheckForUpdates}
                disabled={isCheckingUpdate}
                class="flex items-center gap-2 px-3.5 py-2 rounded-xl bg-white/5 hover:bg-white/10 text-white text-xs font-semibold border border-white/10 transition disabled:opacity-50"
              >
                {#if isCheckingUpdate}
                  <LoaderCircle class="w-3.5 h-3.5 animate-spin text-violet-400" />
                  <span>Checking GitHub...</span>
                {:else}
                  <RefreshCw class="w-3.5 h-3.5 text-slate-300" />
                  <span>Check for Updates</span>
                {/if}
              </button>

              {#if updateStatus?.install_type !== "docker"}
                <button
                  type="button"
                  onclick={handleApplyUpdate}
                  disabled={isApplyingUpdate}
                  class="flex items-center gap-2 px-4 py-2 rounded-xl text-xs font-semibold transition disabled:opacity-50 {updateStatus?.update_available
                    ? 'bg-gradient-to-r from-amber-500 to-amber-600 hover:from-amber-400 hover:to-amber-500 text-black font-bold shadow-[0_0_15px_rgba(245,158,11,0.3)]'
                    : 'bg-violet-600 hover:bg-violet-500 text-white shadow-[0_0_15px_rgba(124,58,237,0.3)]'}"
                >
                  {#if isApplyingUpdate}
                    <LoaderCircle class="w-3.5 h-3.5 animate-spin" />
                    <span>Updating...</span>
                  {:else}
                    <Download class="w-3.5 h-3.5" />
                    <span
                      >{updateStatus?.update_available
                        ? `Upgrade to ${updateStatus.latest_version}`
                        : "Reinstall / Update Now"}</span
                    >
                  {/if}
                </button>
              {/if}
            </div>
          </div>

          <!-- Right Column: Automated Daily Updates (systemd timer) -->
          <div
            class="p-4 rounded-xl bg-[#0a0b10] border border-white/[0.06] flex flex-col justify-between space-y-4"
          >
            <div class="space-y-3">
              <div
                class="flex items-center justify-between border-b border-white/[0.04] pb-2.5"
              >
                <div class="flex items-center gap-2">
                  <Clock class="w-4 h-4 text-cyan-400" />
                  <span
                    class="text-xs font-bold text-white uppercase tracking-wider"
                    >Automated Daily Updates</span
                  >
                </div>
                <div>
                  {#if updateStatus?.install_type === "docker"}
                    <span
                      class="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[11px] font-mono bg-cyan-500/10 text-cyan-400 border border-cyan-500/25"
                    >
                      Container Managed
                    </span>
                  {:else if updateStatus?.auto_update_enabled}
                    <span
                      class="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[11px] font-mono bg-emerald-500/10 text-emerald-400 border border-emerald-500/25"
                    >
                      <span class="w-1.5 h-1.5 rounded-full bg-emerald-400"
                      ></span>
                      Timer Active (04:00 AM)
                    </span>
                  {:else}
                    <span
                      class="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[11px] font-mono bg-amber-500/10 text-amber-400 border border-amber-500/25"
                    >
                      <span class="w-1.5 h-1.5 rounded-full bg-amber-400"
                      ></span>
                      Auto-Updates Disabled
                    </span>
                  {/if}
                </div>
              </div>

              {#if updateStatus?.install_type === "docker"}
                <div class="space-y-2 text-[11px] text-slate-300">
                  <p class="leading-relaxed">
                    PDSwarm is running inside a Docker container. In container
                    environments, auto-updates are managed at the container
                    engine layer rather than via host systemd timers.
                  </p>
                  <p class="text-slate-400">
                    To upgrade your container to the latest version, run in your
                    terminal:
                  </p>
                  <div
                    class="p-2.5 rounded-lg bg-black/60 border border-white/5 font-mono text-[10px] text-cyan-300 flex items-center justify-between"
                  >
                    <code>docker compose pull && docker compose up -d</code>
                    <button
                      type="button"
                      onclick={() =>
                        copyToClipboard(
                          "docker compose pull && docker compose up -d",
                          "command",
                        )}
                      class="ml-2 text-slate-400 hover:text-white"
                      title="Copy command"
                    >
                      {#if copiedCommand}
                        <Check class="w-3.5 h-3.5 text-emerald-400" />
                      {:else}
                        <Copy class="w-3.5 h-3.5" />
                      {/if}
                    </button>
                  </div>
                </div>
              {:else}
                <div class="space-y-2 text-xs text-slate-300">
                  <p class="text-[11px] text-slate-400 leading-relaxed">
                    When enabled, systemd runs <code
                      class="text-cyan-400 font-mono">pdswarm-update.timer</code
                    >
                    every morning at <strong>04:00 AM</strong> (with randomized 30m
                    delay) to check GitHub Releases and seamlessly restart PDSwarm
                    if an update is published.
                  </p>

                  <div
                    class="p-3 rounded-xl bg-white/[0.02] border border-white/[0.04] space-y-1.5"
                  >
                    <div class="flex items-center justify-between text-[11px]">
                      <span class="text-slate-400">Systemd Timer Unit:</span>
                      <span class="font-mono text-slate-300"
                        >pdswarm-update.timer</span
                      >
                    </div>
                    <div class="flex items-center justify-between text-[11px]">
                      <span class="text-slate-400">Target Executable:</span>
                      <span class="font-mono text-slate-300"
                        >/usr/local/bin/pdswarm-update</span
                      >
                    </div>
                  </div>
                </div>
              {/if}
            </div>

            {#if updateStatus?.install_type !== "docker"}
              <div
                class="pt-3 border-t border-white/[0.04] flex items-center justify-between"
              >
                <span class="text-[11px] text-slate-400">
                  {updateStatus?.auto_update_enabled
                    ? "Disable background update timer"
                    : "Enable daily background updates"}
                </span>
                <button
                  type="button"
                  onclick={handleToggleAutoUpdate}
                  disabled={isTogglingAutoUpdate}
                  class="flex items-center gap-2 px-3.5 py-2 rounded-xl text-xs font-semibold transition disabled:opacity-50 {updateStatus?.auto_update_enabled
                    ? 'bg-white/5 hover:bg-rose-500/10 text-slate-300 hover:text-rose-300 border border-white/10 hover:border-rose-500/30'
                    : 'bg-emerald-600 hover:bg-emerald-500 text-white shadow-[0_0_15px_rgba(16,185,129,0.3)]'}"
                >
                  {#if isTogglingAutoUpdate}
                    <LoaderCircle class="w-3.5 h-3.5 animate-spin" />
                    <span>Saving...</span>
                  {:else if updateStatus?.auto_update_enabled}
                    <Power class="w-3.5 h-3.5" />
                    <span>Disable Auto-Updates</span>
                  {:else}
                    <Check class="w-3.5 h-3.5" />
                    <span>Enable Daily Auto-Updates</span>
                  {/if}
                </button>
              </div>
            {/if}
          </div>
        </div>
      </div>

      <!-- Container Logging & Log Rotation Card -->
      <div
        class="p-6 rounded-2xl bg-[#12131c] border border-white/[0.08] shadow-xl space-y-5"
      >
        <div
          class="flex flex-col sm:flex-row sm:items-center justify-between pb-3 border-b border-white/[0.08] gap-3"
        >
          <div class="flex items-center gap-2.5">
            <div
              class="w-8 h-8 rounded-xl bg-emerald-500/10 border border-emerald-500/25 flex items-center justify-center text-emerald-400"
            >
              <FileText class="w-4 h-4" />
            </div>
            <div>
              <h2 class="text-sm font-bold text-white uppercase tracking-wider">
                Container Logging & Log Rotation
              </h2>
              <p class="text-[11px] text-slate-400">
                Configure max log size, file retention count, and host engine
                log rotation
              </p>
            </div>
          </div>
          <div class="flex items-center gap-2">
            {#if isLoadingLogging}
              <span
                class="text-xs text-slate-400 flex items-center gap-1 font-mono"
              >
                <LoaderCircle class="w-3.5 h-3.5 animate-spin" />
                Checking daemon...
              </span>
            {:else if loggingConfig?.daemon_configured}
              <span
                class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-[11px] font-mono bg-emerald-500/10 text-emerald-400 border border-emerald-500/25"
              >
                <span class="w-1.5 h-1.5 rounded-full bg-emerald-400"></span>
                Daemon Rotation: Active
              </span>
            {:else}
              <span
                class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-[11px] font-mono bg-amber-500/10 text-amber-400 border border-amber-500/25"
                title="Daemon logging is unbounded by default in Docker. Logs will grow indefinitely unless configured."
              >
                <span class="w-1.5 h-1.5 rounded-full bg-amber-400"></span>
                Daemon Rotation: Unbounded (No limits)
              </span>
            {/if}
          </div>
        </div>

        {#if loggingFeedback}
          <div
            class="p-3 rounded-xl border border-emerald-500/30 bg-emerald-500/10 text-emerald-300 text-xs flex items-center gap-2"
          >
            <CircleCheck class="w-4 h-4 shrink-0" />
            <span>{loggingFeedback}</span>
          </div>
        {/if}

        {#if loggingError}
          <div
            class="p-3 rounded-xl border border-rose-500/30 bg-rose-500/10 text-rose-300 text-xs flex items-center gap-2"
          >
            <CircleAlert class="w-4 h-4 shrink-0" />
            <span>{loggingError}</span>
          </div>
        {/if}

        <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
          <!-- Left Column: Form Controls -->
          <div class="space-y-4">
            <div>
              <label
                for="logging-driver-select"
                class="block text-xs font-medium text-slate-300 mb-1.5"
              >
                Default Logging Driver
              </label>
              <select
                id="logging-driver-select"
                bind:value={logDriver}
                class="w-full px-3.5 py-2.5 rounded-xl bg-[#0a0b10] border border-white/[0.08] text-white text-xs focus:outline-none focus:border-emerald-500 transition"
              >
                <option value="json-file"
                  >json-file (Standard default for Docker & PDSwarm)</option
                >
                <option value="local"
                  >local (High performance binary log format)</option
                >
                <option value="journald"
                  >journald (Forward logs to host systemd journal)</option
                >
                <option value="syslog"
                  >syslog (Forward logs to host syslog daemon)</option
                >
                <option value="none"
                  >none (Disable container log collection)</option
                >
              </select>
              <p class="text-[11px] text-slate-500 mt-1">
                Driver used to capture stdout and stderr from running
                containers.
              </p>
            </div>

            <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div>
                <label
                  for="max-log-size-input"
                  class="block text-xs font-medium text-slate-300 mb-1.5"
                >
                  Max File Size (<code class="text-emerald-400 font-mono"
                    >max-size</code
                  >)
                </label>
                <input
                  id="max-log-size-input"
                  type="text"
                  bind:value={logMaxSize}
                  placeholder="10m"
                  class="w-full px-3.5 py-2 rounded-xl bg-[#0a0b10] border border-white/[0.08] text-white text-xs font-mono focus:outline-none focus:border-emerald-500 transition"
                />
                <div class="flex flex-wrap gap-1 mt-1.5">
                  {#each ["5m", "10m", "25m", "50m", "100m"] as preset}
                    <button
                      type="button"
                      onclick={() => (logMaxSize = preset)}
                      class="px-2 py-0.5 rounded text-[10px] font-mono transition {logMaxSize ===
                      preset
                        ? 'bg-emerald-500/20 text-emerald-300 border border-emerald-500/40'
                        : 'bg-white/5 text-slate-400 hover:text-white'}"
                    >
                      {preset}
                    </button>
                  {/each}
                </div>
                <p class="text-[11px] text-slate-500 mt-1">
                  Max size before rotating log file.
                </p>
              </div>

              <div>
                <label
                  for="max-log-files-input"
                  class="block text-xs font-medium text-slate-300 mb-1.5"
                >
                  Max Retained Files (<code class="text-emerald-400 font-mono"
                    >max-file</code
                  >)
                </label>
                <input
                  id="max-log-files-input"
                  type="number"
                  min="1"
                  max="50"
                  bind:value={logMaxFile}
                  placeholder="3"
                  class="w-full px-3.5 py-2 rounded-xl bg-[#0a0b10] border border-white/[0.08] text-white text-xs font-mono focus:outline-none focus:border-emerald-500 transition"
                />
                <div class="flex flex-wrap gap-1 mt-1.5">
                  {#each ["1", "3", "5", "10"] as preset}
                    <button
                      type="button"
                      onclick={() => (logMaxFile = preset)}
                      class="px-2 py-0.5 rounded text-[10px] font-mono transition {logMaxFile ===
                      preset
                        ? 'bg-emerald-500/20 text-emerald-300 border border-emerald-500/40'
                        : 'bg-white/5 text-slate-400 hover:text-white'}"
                    >
                      {preset}
                      {preset === "1" ? "file" : "files"}
                    </button>
                  {/each}
                </div>
                <p class="text-[11px] text-slate-500 mt-1">
                  Number of rotated files to keep.
                </p>
              </div>
            </div>

            <!-- Gzip Compression Toggle -->
            <div
              class="p-3.5 rounded-xl bg-[#0a0b10] border border-white/[0.06] flex items-center justify-between"
            >
              <div>
                <div class="text-xs font-semibold text-slate-200">
                  Compress Rotated Logs
                </div>
                <div class="text-[11px] text-slate-500">
                  Enable gzip compression on rotated files (<code
                    class="text-emerald-400 font-mono">compress: true</code
                  >)
                </div>
              </div>
              <input
                type="checkbox"
                bind:checked={logCompress}
                class="w-4 h-4 rounded bg-slate-900 border-white/20 text-emerald-500 focus:ring-emerald-500 cursor-pointer"
              />
            </div>

            <!-- Save Action Buttons -->
            <div class="flex flex-wrap items-center gap-3 pt-2">
              <button
                type="button"
                onclick={() => handleSaveLogging(false)}
                disabled={isSavingLogging}
                class="flex items-center gap-2 px-4 py-2.5 rounded-xl bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-semibold shadow-[0_0_15px_rgba(16,185,129,0.3)] transition disabled:opacity-50"
              >
                {#if isSavingLogging}
                  <LoaderCircle class="w-3.5 h-3.5 animate-spin" />
                  <span>Saving Policy...</span>
                {:else}
                  <Check class="w-3.5 h-3.5" />
                  <span>Save PDSwarm Policy</span>
                {/if}
              </button>

              {#if loggingConfig?.config_writable}
                <button
                  type="button"
                  onclick={() => handleSaveLogging(true)}
                  disabled={isSavingLogging}
                  class="flex items-center gap-2 px-4 py-2.5 rounded-xl bg-violet-600 hover:bg-violet-500 text-white text-xs font-semibold shadow-[0_0_15px_rgba(124,58,237,0.3)] transition disabled:opacity-50"
                >
                  <RefreshCw class="w-3.5 h-3.5" />
                  <span>Apply Directly to Host Config</span>
                </button>
              {/if}
            </div>
            <p class="text-[11px] text-slate-500 italic">
              * "Save PDSwarm Policy" sets default log limits for all new
              containers deployed via PDSwarm.
            </p>
          </div>

          <!-- Right Column: Host Daemon File & Copy Command -->
          <div
            class="p-4 rounded-xl bg-[#0a0b10] border border-white/[0.06] flex flex-col justify-between space-y-4"
          >
            <div>
              <div
                class="flex items-center justify-between pb-2.5 border-b border-white/[0.06]"
              >
                <div class="flex items-center gap-2">
                  <Terminal class="w-4 h-4 text-cyan-400" />
                  <span class="text-xs font-bold text-white font-mono"
                    >{loggingConfig?.config_file ||
                      "/etc/docker/daemon.json"}</span
                  >
                </div>
                <button
                  type="button"
                  onclick={() =>
                    copyToClipboard(
                      loggingConfig?.generated_snippet || "",
                      "snippet",
                    )}
                  class="flex items-center gap-1.5 px-2.5 py-1 rounded-lg bg-white/5 hover:bg-white/10 text-slate-300 text-[11px] font-mono transition"
                >
                  {#if copiedSnippet}
                    <Check class="w-3 h-3 text-emerald-400" />
                    <span class="text-emerald-400">Copied JSON!</span>
                  {:else}
                    <Copy class="w-3 h-3" />
                    <span>Copy JSON</span>
                  {/if}
                </button>
              </div>

              <p class="text-[11px] text-slate-400 mt-2.5 leading-relaxed">
                To enforce log limits host-wide on <strong>all</strong>
                containers (including Docker CLI and Compose), add
                <code class="text-emerald-400 font-mono">log-opts</code> to your
                daemon configuration:
              </p>

              <div
                class="mt-2.5 rounded-lg bg-[#07080c] border border-white/[0.08] p-3 font-mono text-[11px] text-emerald-300 overflow-x-auto max-h-44"
              >
                <pre>{loggingConfig?.generated_snippet ||
                    '{\n  "log-driver": "json-file",\n  "log-opts": {\n    "max-size": "10m",\n    "max-file": "3"\n  }\n}'}</pre>
              </div>
            </div>

            <!-- Terminal One-Liner Command -->
            <div class="pt-3 border-t border-white/[0.04]">
              <div class="flex items-center justify-between mb-1.5">
                <span class="text-[11px] font-semibold text-slate-300"
                  >Apply to Host Daemon via Terminal:</span
                >
                <button
                  type="button"
                  onclick={() =>
                    copyToClipboard(
                      loggingConfig?.apply_command || "",
                      "command",
                    )}
                  class="flex items-center gap-1 px-2 py-0.5 rounded bg-cyan-500/10 text-cyan-300 hover:bg-cyan-500/20 text-[10px] font-mono transition"
                >
                  {#if copiedCommand}
                    <Check class="w-3 h-3 text-emerald-400" />
                    <span class="text-emerald-400">Copied Command!</span>
                  {:else}
                    <Copy class="w-3 h-3" />
                    <span>Copy Command</span>
                  {/if}
                </button>
              </div>
              <div
                class="p-2.5 rounded-lg bg-black/60 border border-white/5 font-mono text-[10px] text-slate-300 overflow-x-auto select-all leading-relaxed"
              >
                <code
                  >{loggingConfig?.apply_command ||
                    "sudo systemctl reload docker"}</code
                >
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Security Audit & Fail2ban Jail Card -->
      <div class="p-6 rounded-2xl bg-[#12131c] border border-white/[0.08] shadow-xl space-y-5">
        <div class="flex flex-col sm:flex-row sm:items-center justify-between pb-3 border-b border-white/[0.08] gap-3">
          <div class="flex items-center gap-2.5">
            <div class="w-8 h-8 rounded-xl bg-rose-500/10 border border-rose-500/25 flex items-center justify-center text-rose-400">
              <ShieldAlert class="w-4 h-4" />
            </div>
            <div>
              <h2 class="text-sm font-bold text-white uppercase tracking-wider">
                Security Audit & Jail (Fail2ban) Logging
              </h2>
              <p class="text-[11px] text-slate-400">
                Dedicated security log file for intrusion detection, brute-force protection, and Fail2ban/CrowdSec jails
              </p>
            </div>
          </div>
          <button
            type="button"
            onclick={loadAuditLogs}
            disabled={isLoadingAuditLogs}
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-xl bg-white/[0.04] hover:bg-white/[0.08] text-slate-300 hover:text-white text-xs font-mono border border-white/[0.08] transition"
          >
            <RefreshCw class="w-3.5 h-3.5 {isLoadingAuditLogs ? 'animate-spin text-cyan-400' : ''}" />
            <span>Refresh</span>
          </button>
        </div>

        <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
          <!-- Active Log File Path -->
          <div class="p-4 rounded-xl bg-[#0a0b10] border border-white/[0.06] space-y-2">
            <div class="flex items-center justify-between text-xs">
              <span class="text-slate-400 font-medium">Audit Log File Path</span>
              <button
                type="button"
                onclick={() => copyAuditSnippet('path', auditLogInfo?.path || '')}
                class="text-[11px] text-cyan-400 hover:text-cyan-300 flex items-center gap-1 font-mono"
              >
                {#if auditCopied === 'path'}
                  <Check class="w-3 h-3 text-emerald-400" />
                  <span>Copied</span>
                {:else}
                  <Copy class="w-3 h-3" />
                  <span>Copy Path</span>
                {/if}
              </button>
            </div>
            <div class="p-2.5 rounded-lg bg-[#0d0e17] border border-white/[0.08] font-mono text-xs text-emerald-400 truncate select-all">
              {auditLogInfo?.path || '/var/lib/pdswarm/auth.log'}
            </div>
            <p class="text-[11px] text-slate-500">
              Configurable via <code class="text-slate-300">-auth-log &lt;path&gt;</code> CLI flag or <code class="text-slate-300">PDSWARM_AUTH_LOG</code>.
            </p>
          </div>

          <!-- Log Level & Filters -->
          <div class="p-4 rounded-xl bg-[#0a0b10] border border-white/[0.06] space-y-2">
            <div class="flex items-center justify-between text-xs">
              <span class="text-slate-400 font-medium">Log Filtering Level</span>
              <span class="px-2 py-0.5 rounded text-[10px] font-mono bg-violet-500/10 text-violet-300 border border-violet-500/20 uppercase font-semibold">
                {auditLogInfo?.level || 'auth'}
              </span>
            </div>
            <div class="p-2.5 rounded-lg bg-[#0d0e17] border border-white/[0.08] text-xs text-slate-300">
              {#if auditLogInfo?.level === 'failures_only'}
                Only logs failed authentications and unauthorized attempts (strictest).
              {:else if auditLogInfo?.level === 'off'}
                File logging disabled.
              {:else}
                Logs all authentication events (login successes/failures, password changes, token checks). <strong class="text-white">Zero noisy routine API calls.</strong>
              {/if}
            </div>
            <p class="text-[11px] text-slate-500">
              Configurable via <code class="text-slate-300">-log-level auth|failures_only|off</code> or <code class="text-slate-300">PDSWARM_LOG_LEVEL</code>.
            </p>
          </div>
        </div>

        <!-- Recent Security Audit Entries -->
        <div class="space-y-2">
          <div class="flex items-center justify-between text-xs">
            <span class="font-semibold text-slate-300">Recent Security & Auth Events</span>
            <span class="text-[11px] text-slate-500">
              Showing last {auditLogInfo?.entries?.length || 0} event(s)
            </span>
          </div>

          {#if !auditLogInfo?.entries || auditLogInfo.entries.length === 0}
            <div class="p-4 rounded-xl bg-[#0a0b10] border border-white/[0.04] text-center text-xs text-slate-500 font-mono italic">
              No authentication events logged yet.
            </div>
          {:else}
            <div class="p-3 rounded-xl bg-[#0a0b10] border border-white/[0.06] max-h-48 overflow-y-auto space-y-1 font-mono text-[11px]">
              {#each auditLogInfo.entries as entry}
                <div class="leading-relaxed truncate {entry.includes('[FAIL]') ? 'text-rose-400 font-medium' : entry.includes('[SUCCESS]') ? 'text-emerald-400' : 'text-slate-400'}">
                  {entry}
                </div>
              {/each}
            </div>
          {/if}
        </div>

        <!-- Fail2ban Ready Configurations -->
        <div class="p-4 rounded-xl bg-[#0a0b10] border border-white/[0.06] space-y-3">
          <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
            <div class="flex items-center gap-2">
              <ShieldAlert class="w-4 h-4 text-cyan-400 shrink-0" />
              <span class="text-xs font-semibold text-white">Fail2ban Jail Setup (1-Click Copy)</span>
            </div>
            <div class="flex items-center gap-2 flex-wrap">
              <button
                type="button"
                onclick={() => copyAuditSnippet('filter', auditLogInfo?.fail2ban_filter || '')}
                class="px-2.5 py-1 rounded-lg bg-white/[0.04] hover:bg-white/[0.08] text-xs font-mono text-cyan-300 border border-white/[0.08] transition flex items-center gap-1"
              >
                {#if auditCopied === 'filter'}
                  <Check class="w-3 h-3 text-emerald-400" />
                  <span>Copied Filter</span>
                {:else}
                  <Copy class="w-3 h-3" />
                  <span>Copy Filter (/etc/fail2ban/filter.d/pdswarm.conf)</span>
                {/if}
              </button>
              <button
                type="button"
                onclick={() => copyAuditSnippet('jail', auditLogInfo?.fail2ban_jail || '')}
                class="px-2.5 py-1 rounded-lg bg-violet-600/30 hover:bg-violet-600/50 text-xs font-mono text-violet-200 border border-violet-500/30 transition flex items-center gap-1"
              >
                {#if auditCopied === 'jail'}
                  <Check class="w-3 h-3 text-emerald-400" />
                  <span>Copied Jail</span>
                {:else}
                  <Copy class="w-3 h-3" />
                  <span>Copy Jail (/etc/fail2ban/jail.d/pdswarm.conf)</span>
                {/if}
              </button>
            </div>
          </div>
          <p class="text-[11px] text-slate-400">
            Paste the filter to <code class="text-slate-300">/etc/fail2ban/filter.d/pdswarm.conf</code> and the jail to <code class="text-slate-300">/etc/fail2ban/jail.d/pdswarm.conf</code>, then execute <code class="text-cyan-300 font-mono">sudo fail2ban-client reload</code>.
          </p>
        </div>
      </div>
    </div>

    <!-- 2. USERS & PASSWORD TAB -->
  {:else if activeTab === "users"}
    <div class="space-y-6">
      <div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <!-- Change Password Card (1 column) -->
        <div
          class="p-6 rounded-2xl bg-[#12131c] border border-white/[0.08] shadow-xl space-y-5 lg:col-span-1"
        >
          <div
            class="flex items-center gap-2.5 border-b border-white/[0.08] pb-3"
          >
            <Lock class="w-4 h-4 text-violet-400" />
            <h2 class="text-sm font-bold text-white uppercase tracking-wider">
              Change Password
            </h2>
          </div>

          {#if passwordFeedback}
            <div
              class="p-3 rounded-xl border text-xs flex items-center gap-2 {passwordFeedback.type ===
              'success'
                ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-300'
                : 'bg-rose-500/10 border-rose-500/30 text-rose-300'}"
            >
              {#if passwordFeedback.type === "success"}
                <CircleCheck class="w-4 h-4 shrink-0" />
              {:else}
                <CircleAlert class="w-4 h-4 shrink-0" />
              {/if}
              <span>{passwordFeedback.message}</span>
            </div>
          {/if}

          <form onsubmit={handleChangePassword} class="space-y-4">
            <div>
              <label
                for="current-password-input"
                class="block text-xs font-medium text-slate-300 mb-1.5"
                >Current Password</label
              >
              <div class="relative">
                <input
                  id="current-password-input"
                  type={showCurrentPassword ? "text" : "password"}
                  bind:value={currentPassword}
                  placeholder="&bull;&bull;&bull;&bull;&bull;&bull;&bull;&bull;"
                  class="w-full px-3.5 py-2 rounded-xl bg-[#0a0b10] border border-white/[0.08] text-white text-xs placeholder-slate-600 focus:outline-none focus:border-violet-500 pr-10"
                />
                <button
                  type="button"
                  onclick={() => (showCurrentPassword = !showCurrentPassword)}
                  class="absolute right-3 top-1/2 -translate-y-1/2 text-slate-500 hover:text-slate-300"
                >
                  {#if showCurrentPassword}<EyeOff
                      class="w-3.5 h-3.5"
                    />{:else}<Eye class="w-3.5 h-3.5" />{/if}
                </button>
              </div>
            </div>

            <div>
              <label
                for="new-password-input"
                class="block text-xs font-medium text-slate-300 mb-1.5"
                >New Password</label
              >
              <div class="relative">
                <input
                  id="new-password-input"
                  type={showNewPassword ? "text" : "password"}
                  bind:value={newPassword}
                  placeholder="&bull;&bull;&bull;&bull;&bull;&bull;&bull;&bull;"
                  class="w-full px-3.5 py-2 rounded-xl bg-[#0a0b10] border border-white/[0.08] text-white text-xs placeholder-slate-600 focus:outline-none focus:border-violet-500 pr-10"
                />
                <button
                  type="button"
                  onclick={() => (showNewPassword = !showNewPassword)}
                  class="absolute right-3 top-1/2 -translate-y-1/2 text-slate-500 hover:text-slate-300"
                >
                  {#if showNewPassword}<EyeOff class="w-3.5 h-3.5" />{:else}<Eye
                      class="w-3.5 h-3.5"
                    />{/if}
                </button>
              </div>
            </div>

            <div>
              <label
                for="confirm-password-input"
                class="block text-xs font-medium text-slate-300 mb-1.5"
                >Confirm New Password</label
              >
              <input
                id="confirm-password-input"
                type="password"
                bind:value={confirmPassword}
                placeholder="&bull;&bull;&bull;&bull;&bull;&bull;&bull;&bull;"
                class="w-full px-3.5 py-2 rounded-xl bg-[#0a0b10] border border-white/[0.08] text-white text-xs placeholder-slate-600 focus:outline-none focus:border-violet-500"
              />
            </div>

            <div class="pt-2">
              <button
                type="submit"
                disabled={isChangingPassword}
                class="w-full flex items-center justify-center gap-2 px-4 py-2.5 rounded-xl bg-violet-600 hover:bg-violet-500 text-white text-xs font-semibold shadow-[0_0_15px_rgba(124,58,237,0.3)] transition disabled:opacity-50"
              >
                {#if isChangingPassword}
                  <LoaderCircle class="w-3.5 h-3.5 animate-spin" />
                  <span>Updating Password...</span>
                {:else}
                  <Lock class="w-3.5 h-3.5" />
                  <span>Update Password</span>
                {/if}
              </button>
            </div>
          </form>
        </div>

        <!-- User Management Card (2 columns) -->
        <div
          class="p-6 rounded-2xl bg-[#12131c] border border-white/[0.08] shadow-xl space-y-5 lg:col-span-2"
        >
          <div
            class="flex items-center justify-between border-b border-white/[0.08] pb-3"
          >
            <div class="flex items-center gap-2.5">
              <Users class="w-4 h-4 text-cyan-400" />
              <h2 class="text-sm font-bold text-white uppercase tracking-wider">
                User Accounts
              </h2>
            </div>
            <button
              onclick={() => (isAddUserModalOpen = true)}
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-xl bg-violet-600/30 hover:bg-violet-600 text-violet-300 hover:text-white border border-violet-500/40 text-xs font-semibold shadow-[0_0_10px_rgba(124,58,237,0.2)] transition"
            >
              <Plus class="w-3.5 h-3.5" />
              <span>Add User</span>
            </button>
          </div>

          {#if userError}
            <div
              class="p-3 rounded-xl bg-rose-500/10 border border-rose-500/30 text-rose-300 text-xs flex items-center gap-2"
            >
              <CircleAlert class="w-4 h-4 shrink-0" />
              <span>{userError}</span>
            </div>
          {/if}

          <!-- Users Table -->
          <div
            class="overflow-x-auto rounded-xl border border-white/[0.06] bg-[#0a0b10]"
          >
            <table class="w-full text-left text-xs">
              <thead
                class="bg-[#161724] border-b border-white/[0.06] text-slate-400 uppercase text-[10px] tracking-wider font-mono"
              >
                <tr>
                  <th class="py-3 px-4">User</th>
                  <th class="py-3 px-4">Role</th>
                  <th class="py-3 px-4">Created Date</th>
                  <th class="py-3 px-4 text-right">Actions</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-white/[0.04]">
                {#if isLoadingUsers}
                  <tr>
                    <td colspan="4" class="py-8 text-center text-slate-500">
                      <LoaderCircle
                        class="w-5 h-5 animate-spin mx-auto text-violet-400"
                      />
                      <span class="mt-2 block text-xs"
                        >Loading user accounts...</span
                      >
                    </td>
                  </tr>
                {:else if users.length === 0}
                  <tr>
                    <td colspan="4" class="py-8 text-center text-slate-500">
                      No user accounts found.
                    </td>
                  </tr>
                {:else}
                  {#each users as u}
                    {@const isSelf = auth.user?.username === u.username}
                    <tr class="hover:bg-white/[0.02] transition">
                      <td class="py-3 px-4">
                        <div class="flex items-center gap-2.5">
                          <div
                            class="w-7 h-7 rounded-lg bg-violet-600/20 border border-violet-500/30 flex items-center justify-center text-violet-300 font-bold text-xs uppercase"
                          >
                            {u.username.slice(0, 2)}
                          </div>
                          <div>
                            <span class="font-semibold text-white"
                              >{u.username}</span
                            >
                            {#if isSelf}
                              <span
                                class="ml-1.5 px-1.5 py-0.5 text-[9px] font-mono rounded bg-emerald-500/20 text-emerald-300 border border-emerald-500/30"
                              >
                                You
                              </span>
                            {/if}
                          </div>
                        </div>
                      </td>
                      <td class="py-3 px-4">
                        {#if u.role === "admin"}
                          <span
                            class="px-2 py-0.5 rounded-full text-[10px] font-mono bg-violet-500/20 text-violet-300 border border-violet-500/40"
                          >
                            admin
                          </span>
                        {:else if u.role === "operator"}
                          <span
                            class="px-2 py-0.5 rounded-full text-[10px] font-mono bg-cyan-500/20 text-cyan-300 border border-cyan-500/40"
                          >
                            operator
                          </span>
                        {:else}
                          <span
                            class="px-2 py-0.5 rounded-full text-[10px] font-mono bg-slate-500/20 text-slate-300 border border-slate-500/40"
                          >
                            viewer
                          </span>
                        {/if}
                      </td>
                      <td
                        class="py-3 px-4 text-slate-400 font-mono text-[11px]"
                      >
                        {formatDate(u.created_at)}
                      </td>
                      <td class="py-3 px-4 text-right">
                        {#if !isSelf}
                          <button
                            onclick={() => (deletingUser = u)}
                            class="p-1.5 rounded-lg text-slate-400 hover:text-rose-400 hover:bg-rose-500/10 transition"
                            title="Delete user"
                          >
                            <Trash class="w-4 h-4" />
                          </button>
                        {:else}
                          <span
                            class="text-[11px] text-slate-600 font-mono italic"
                            >Current user</span
                          >
                        {/if}
                      </td>
                    </tr>
                  {/each}
                {/if}
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>

    <!-- 3. API ACCESS TOKENS TAB -->
  {:else if activeTab === "tokens"}
    <div class="space-y-6">
      <div
        class="p-6 rounded-2xl bg-[#12131c] border border-white/[0.08] shadow-xl space-y-5"
      >
        <div
          class="flex flex-col sm:flex-row sm:items-center justify-between gap-4 border-b border-white/[0.08] pb-4"
        >
          <div>
            <div class="flex items-center gap-2">
              <Key class="w-4 h-4 text-violet-400" />
              <h2 class="text-sm font-bold text-white uppercase tracking-wider">
                Programmatic Access Tokens
              </h2>
            </div>
            <p class="text-xs text-slate-400 mt-0.5">
              API tokens allow CLI scripts, CI/CD runners, and webhooks to
              interact with PDSwarm via Bearer authentication (<code
                class="text-violet-300">pds_...</code
              >)
            </p>
          </div>

          <button
            onclick={() => (isCreateTokenModalOpen = true)}
            class="flex items-center gap-2 px-3.5 py-2 rounded-xl bg-violet-600 hover:bg-violet-500 text-white text-xs font-semibold shadow-[0_0_15px_rgba(124,58,237,0.3)] transition shrink-0"
          >
            <Plus class="w-4 h-4" />
            <span>Generate New Token</span>
          </button>
        </div>

        {#if tokenError}
          <div
            class="p-3 rounded-xl bg-rose-500/10 border border-rose-500/30 text-rose-300 text-xs flex items-center gap-2"
          >
            <CircleAlert class="w-4 h-4 shrink-0" />
            <span>{tokenError}</span>
          </div>
        {/if}

        <!-- Tokens Table -->
        <div
          class="overflow-x-auto rounded-xl border border-white/[0.06] bg-[#0a0b10]"
        >
          <table class="w-full text-left text-xs">
            <thead
              class="bg-[#161724] border-b border-white/[0.06] text-slate-400 uppercase text-[10px] tracking-wider font-mono"
            >
              <tr>
                <th class="py-3 px-4">Name</th>
                <th class="py-3 px-4">Prefix</th>
                <th class="py-3 px-4">Created By</th>
                <th class="py-3 px-4">Created Date</th>
                <th class="py-3 px-4">Expires</th>
                <th class="py-3 px-4">Last Used</th>
                <th class="py-3 px-4 text-right">Revoke</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-white/[0.04]">
              {#if isLoadingTokens}
                <tr>
                  <td colspan="7" class="py-8 text-center text-slate-500">
                    <LoaderCircle
                      class="w-5 h-5 animate-spin mx-auto text-violet-400"
                    />
                    <span class="mt-2 block text-xs"
                      >Loading access tokens...</span
                    >
                  </td>
                </tr>
              {:else if tokens.length === 0}
                <tr>
                  <td colspan="7" class="py-8 text-center text-slate-500">
                    No active API tokens found. Click <span
                      class="text-violet-400 font-semibold"
                      >"Generate New Token"</span
                    > to create one.
                  </td>
                </tr>
              {:else}
                {#each tokens as t}
                  <tr class="hover:bg-white/[0.02] transition">
                    <td class="py-3 px-4 font-semibold text-white">
                      {t.name}
                    </td>
                    <td class="py-3 px-4">
                      <span
                        class="px-2 py-0.5 rounded font-mono text-[11px] bg-white/[0.06] text-cyan-300 border border-white/[0.08]"
                      >
                        {t.prefix}
                      </span>
                    </td>
                    <td class="py-3 px-4 text-slate-300">
                      {t.created_by}
                    </td>
                    <td class="py-3 px-4 text-slate-400 font-mono text-[11px]">
                      {formatDate(t.created_at)}
                    </td>
                    <td class="py-3 px-4 text-slate-400 font-mono text-[11px]">
                      {#if t.expires_at}
                        <span>{formatDate(t.expires_at)}</span>
                      {:else}
                        <span class="text-slate-500">Never</span>
                      {/if}
                    </td>
                    <td class="py-3 px-4 font-mono text-[11px]">
                      {#if t.last_used_at}
                        <span class="text-emerald-400"
                          >{formatDate(t.last_used_at)}</span
                        >
                      {:else}
                        <span class="text-slate-500">Never</span>
                      {/if}
                    </td>
                    <td class="py-3 px-4 text-right">
                      <button
                        onclick={() => (revokingToken = t)}
                        class="p-1.5 rounded-lg text-slate-400 hover:text-rose-400 hover:bg-rose-500/10 transition"
                        title="Revoke token"
                      >
                        <Trash class="w-4 h-4" />
                      </button>
                    </td>
                  </tr>
                {/each}
              {/if}
            </tbody>
          </table>
        </div>
      </div>
    </div>

    <!-- 4. REGISTRIES TAB -->
  {:else if activeTab === "registries"}
    <div class="space-y-6">
      <RegistriesManager isModal={false} />
    </div>

    <!-- 5. APP CATALOGS TAB -->
  {:else if activeTab === "catalogs"}
    <div class="space-y-6">
      <!-- Top Action Bar -->
      <div
        class="flex flex-col sm:flex-row sm:items-center justify-between gap-4"
      >
        <div>
          <div class="flex items-center gap-2">
            <Boxes class="w-5 h-5 text-cyan-400" />
            <h2 class="text-base font-bold text-white uppercase tracking-wider">
              Template Catalogs & Repositories
            </h2>
          </div>
          <p class="text-xs text-slate-400 mt-1">
            Configure application catalogs, import Portainer feeds or CasaOS
            appstore archives, and control curated defaults.
          </p>
        </div>

        <div class="flex items-center gap-2.5">
          <button
            onclick={handleSyncCatalogs}
            disabled={isSyncingCatalogs}
            class="flex items-center gap-2 px-3.5 py-2 rounded-xl bg-cyan-600/20 hover:bg-cyan-600/30 text-cyan-300 border border-cyan-500/30 text-xs font-semibold shadow-[0_0_15px_rgba(6,182,212,0.2)] transition disabled:opacity-50"
          >
            <RefreshCw
              class="w-3.5 h-3.5 {isSyncingCatalogs ? 'animate-spin' : ''}"
            />
            <span>{isSyncingCatalogs ? "Syncing..." : "Sync All"}</span>
          </button>

          <button
            onclick={handleOpenAddRepoModal}
            class="flex items-center gap-2 px-3.5 py-2 rounded-xl bg-violet-600 hover:bg-violet-500 text-white text-xs font-semibold shadow-[0_0_15px_rgba(124,58,237,0.3)] transition"
          >
            <Plus class="w-3.5 h-3.5" />
            <span>Add Catalog</span>
          </button>

          {#if onNavigate}
            <button
              onclick={() => onNavigate?.("templates")}
              class="flex items-center gap-1.5 px-3 py-2 rounded-xl bg-white/[0.05] hover:bg-white/[0.1] text-slate-300 hover:text-white border border-white/[0.08] text-xs font-semibold transition"
              title="Open App Store"
            >
              <span>App Store</span>
              <ExternalLink class="w-3.5 h-3.5" />
            </button>
          {/if}
        </div>
      </div>

      {#if catalogFeedback}
        <div
          class="p-3.5 rounded-xl bg-cyan-500/10 border border-cyan-500/30 text-cyan-300 text-xs flex items-center gap-2 animate-in fade-in duration-150"
        >
          <CircleCheck class="w-4 h-4 shrink-0" />
          <span>{catalogFeedback}</span>
        </div>
      {/if}

      <!-- 1. BUILT-IN TEMPLATES CONTROL CARD -->
      <div
        class="p-5 rounded-2xl bg-[#12131c] border border-white/[0.08] shadow-xl"
      >
        <div
          class="flex flex-col sm:flex-row sm:items-center justify-between gap-4"
        >
          <div class="flex items-start gap-3.5">
            <div
              class="p-2.5 rounded-xl bg-violet-600/20 border border-violet-500/30 text-violet-400 shrink-0"
            >
              <Boxes class="w-5 h-5" />
            </div>
            <div>
              <div class="flex items-center gap-2">
                <h3 class="text-sm font-bold text-white">
                  PDSwarm Built-in Curated Templates
                </h3>
                {#if templateSettings.disable_default_templates}
                  <span
                    class="px-2 py-0.5 rounded-full text-[10px] font-mono font-semibold bg-slate-700/50 text-slate-400 border border-slate-600/50"
                  >
                    Disabled
                  </span>
                {/if}
              </div>
              <p class="text-xs text-slate-400 mt-1 max-w-2xl">
                Pre-packaged, zero-bloat Docker Compose stacks (PostgreSQL,
                MySQL, Redis, Nginx, Jellyfin, Nextcloud, Vaultwarden,
                Prometheus, etc.) with pre-configured variables and volume
                definitions.
              </p>
            </div>
          </div>

          <div class="flex items-center gap-3 shrink-0">
            <button
              type="button"
              onclick={handleToggleDefaultTemplates}
              disabled={isTogglingDefaults}
              class="relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out {!templateSettings.disable_default_templates
                ? 'bg-violet-600'
                : 'bg-slate-700'} disabled:opacity-50"
              role="switch"
              aria-checked={!templateSettings.disable_default_templates}
              title="Toggle built-in curated templates"
            >
              <span
                class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow-lg ring-0 transition duration-200 ease-in-out {!templateSettings.disable_default_templates
                  ? 'translate-x-5'
                  : 'translate-x-0'}"
              ></span>
            </button>
            <span class="text-xs font-medium text-slate-300">
              {!templateSettings.disable_default_templates
                ? "Enabled"
                : "Disabled"}
            </span>
          </div>
        </div>
      </div>

      <!-- 2. CUSTOM CATALOGS TABLE -->
      <div
        class="p-6 rounded-2xl bg-[#12131c] border border-white/[0.08] shadow-xl space-y-4"
      >
        <div class="flex items-center justify-between">
          <div>
            <h3 class="text-sm font-bold text-white uppercase tracking-wider">
              Custom Catalogs & Feeds
            </h3>
            <p class="text-xs text-slate-400 mt-0.5">
              Add external Portainer JSON catalog feeds or CasaOS AppStore Zip
              archives to expand your App Store.
            </p>
          </div>
          <span
            class="px-2.5 py-1 rounded-full text-xs font-mono bg-white/[0.05] text-slate-300 border border-white/[0.08]"
          >
            {templateRepos.length} configured
          </span>
        </div>

        {#if templateRepos.length === 0}
          <div
            class="py-10 px-6 rounded-xl bg-[#0a0b10] border border-white/[0.04] text-center space-y-4"
          >
            <div
              class="w-12 h-12 mx-auto rounded-xl bg-cyan-600/10 border border-cyan-500/20 text-cyan-400 flex items-center justify-center"
            >
              <FolderArchive class="w-6 h-6" />
            </div>
            <div class="max-w-md mx-auto space-y-1">
              <h4 class="text-xs font-bold text-white uppercase tracking-wider">
                No Custom Catalogs Added
              </h4>
              <p class="text-xs text-slate-400">
                You can add community template lists like Lissy93 Portainer
                Templates or the CasaOS LinuxServer AppStore Zip.
              </p>
            </div>

            <!-- Quick Add Presets -->
            <div class="pt-2 flex flex-wrap items-center justify-center gap-2">
              <button
                onclick={() => {
                  handleOpenAddRepoModal();
                  applyRepoPreset(
                    "Lissy93 Portainer Templates",
                    "https://raw.githubusercontent.com/Lissy93/portainer-templates/main/templates.json",
                    "portainer",
                  );
                }}
                class="px-3 py-1.5 rounded-lg bg-violet-600/20 hover:bg-violet-600/30 text-violet-300 border border-violet-500/30 text-[11px] font-semibold transition"
              >
                + Preset: Lissy93 (Portainer JSON)
              </button>
              <button
                onclick={() => {
                  handleOpenAddRepoModal();
                  applyRepoPreset(
                    "CasaOS LinuxServer AppStore",
                    "https://casaos-appstore.paodayag.dev/linuxserver.zip",
                    "casaos_zip",
                  );
                }}
                class="px-3 py-1.5 rounded-lg bg-cyan-600/20 hover:bg-cyan-600/30 text-cyan-300 border border-cyan-500/30 text-[11px] font-semibold transition"
              >
                + Preset: LinuxServer (CasaOS Zip)
              </button>
              <button
                onclick={() => {
                  handleOpenAddRepoModal();
                  applyRepoPreset(
                    "Official Portainer Templates",
                    "https://raw.githubusercontent.com/portainer/templates/master/templates-2.0.json",
                    "portainer",
                  );
                }}
                class="px-3 py-1.5 rounded-lg bg-emerald-600/20 hover:bg-emerald-600/30 text-emerald-300 border border-emerald-500/30 text-[11px] font-semibold transition"
              >
                + Preset: Portainer 2.0 (Official)
              </button>
            </div>
          </div>
        {:else}
          <div class="overflow-x-auto rounded-xl border border-white/[0.06]">
            <table class="w-full text-left text-xs">
              <thead
                class="bg-[#0a0b10] border-b border-white/[0.06] text-slate-400 font-mono text-[11px] uppercase tracking-wider"
              >
                <tr>
                  <th class="py-3 px-4">Catalog Name / URL</th>
                  <th class="py-3 px-4">Type</th>
                  <th class="py-3 px-4">Apps</th>
                  <th class="py-3 px-4">Status</th>
                  <th class="py-3 px-4">Last Synced</th>
                  <th class="py-3 px-4">Enabled</th>
                  <th class="py-3 px-4 text-right">Actions</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-white/[0.04]">
                {#each templateRepos as repo}
                  <tr class="hover:bg-white/[0.02] transition">
                    <td class="py-3 px-4">
                      <div class="font-bold text-white">{repo.name}</div>
                      <div
                        class="text-[11px] font-mono text-slate-500 truncate max-w-sm"
                        title={repo.url}
                      >
                        {repo.url}
                      </div>
                    </td>
                    <td class="py-3 px-4">
                      <span
                        class="px-2 py-0.5 rounded text-[10px] font-mono uppercase bg-white/[0.06] text-cyan-300 border border-white/[0.08]"
                      >
                        {repo.type === "casaos_zip"
                          ? "CasaOS Zip"
                          : repo.type === "portainer"
                            ? "Portainer JSON"
                            : repo.type}
                      </span>
                    </td>
                    <td class="py-3 px-4 font-mono font-semibold text-white">
                      {repo.item_count} apps
                    </td>
                    <td class="py-3 px-4">
                      {#if repo.sync_error}
                        <span
                          class="inline-flex items-center gap-1 px-2 py-0.5 rounded text-[10px] font-mono bg-rose-500/20 text-rose-300 border border-rose-500/30"
                          title={repo.sync_error}
                        >
                          <CircleAlert class="w-3 h-3" /> Error
                        </span>
                      {:else if repo.enabled}
                        <span
                          class="inline-flex items-center gap-1 px-2 py-0.5 rounded text-[10px] font-mono bg-emerald-500/20 text-emerald-300 border border-emerald-500/30"
                        >
                          <Check class="w-3 h-3" /> Synced
                        </span>
                      {:else}
                        <span
                          class="px-2 py-0.5 rounded text-[10px] font-mono bg-slate-700/40 text-slate-400"
                        >
                          Disabled
                        </span>
                      {/if}
                    </td>
                    <td class="py-3 px-4 text-slate-400 font-mono text-[11px]">
                      {formatDate(repo.last_sync_at)}
                    </td>
                    <td class="py-3 px-4">
                      <button
                        onclick={() => handleToggleRepo(repo)}
                        class="p-1 rounded text-slate-400 hover:text-white transition"
                        title={repo.enabled
                          ? "Click to disable"
                          : "Click to enable"}
                      >
                        {#if repo.enabled}
                          <span
                            class="px-2 py-0.5 rounded-full text-[10px] font-bold bg-violet-600/30 text-violet-300 border border-violet-500/40"
                          >
                            ON
                          </span>
                        {:else}
                          <span
                            class="px-2 py-0.5 rounded-full text-[10px] font-bold bg-slate-800 text-slate-500 border border-slate-700"
                          >
                            OFF
                          </span>
                        {/if}
                      </button>
                    </td>
                    <td class="py-3 px-4 text-right">
                      <div class="flex items-center justify-end gap-1.5">
                        <button
                          onclick={() => handleSyncSingleRepo(repo.id)}
                          disabled={syncingRepoId === repo.id}
                          class="p-1.5 rounded-lg text-slate-400 hover:text-cyan-400 hover:bg-cyan-500/10 transition disabled:opacity-50"
                          title="Sync catalog now"
                        >
                          <RefreshCw
                            class="w-4 h-4 {syncingRepoId === repo.id
                              ? 'animate-spin text-cyan-400'
                              : ''}"
                          />
                        </button>
                        <button
                          onclick={() => (deletingRepo = repo)}
                          class="p-1.5 rounded-lg text-slate-400 hover:text-rose-400 hover:bg-rose-500/10 transition"
                          title="Delete catalog"
                        >
                          <Trash class="w-4 h-4" />
                        </button>
                      </div>
                    </td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        {/if}

        <!-- Summary Stats Footer -->
        <div
          class="p-4 rounded-xl bg-[#0a0b10] border border-white/[0.04] flex flex-wrap items-center justify-between gap-4"
        >
          <div class="flex items-center gap-6 text-xs">
            <div>
              <span class="text-slate-500">Total Active Templates:</span>
              <span class="ml-1.5 font-bold font-mono text-white"
                >{catalogData?.total ?? 0}</span
              >
            </div>
            <div>
              <span class="text-slate-500">Categories:</span>
              <span class="ml-1.5 font-bold font-mono text-cyan-400"
                >{catalogData?.categories?.length ?? 0}</span
              >
            </div>
            <div>
              <span class="text-slate-500">Connected Sources:</span>
              <span class="ml-1.5 font-bold font-mono text-violet-400"
                >{catalogData?.sources?.length ?? 1}</span
              >
            </div>
          </div>
          <div class="text-[11px] text-slate-500 font-mono">
            Catalogs sync automatically upon changes &bull; Pure Go archive
            engine
          </div>
        </div>
      </div>
    </div>
  {/if}
</div>

<!-- ================= MODALS ================= -->

<!-- ADD USER MODAL -->
{#if isAddUserModalOpen}
  <div
    class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-sm animate-in fade-in duration-150"
  >
    <div
      class="w-full max-w-md p-6 rounded-2xl bg-[#12131c] border border-white/[0.1] shadow-2xl space-y-5"
    >
      <div
        class="flex items-center justify-between border-b border-white/[0.08] pb-3"
      >
        <div class="flex items-center gap-2">
          <Users class="w-4 h-4 text-violet-400" />
          <h3 class="text-sm font-bold text-white uppercase tracking-wider">
            Add User Account
          </h3>
        </div>
        <button
          onclick={() => (isAddUserModalOpen = false)}
          class="p-1 rounded-lg text-slate-400 hover:text-white hover:bg-white/5 transition"
        >
          <X class="w-4 h-4" />
        </button>
      </div>

      {#if addUserError}
        <div
          class="p-3 rounded-xl bg-rose-500/10 border border-rose-500/30 text-rose-300 text-xs flex items-center gap-2"
        >
          <CircleAlert class="w-4 h-4 shrink-0" />
          <span>{addUserError}</span>
        </div>
      {/if}

      <form onsubmit={handleCreateUser} class="space-y-4">
        <div>
          <label
            for="new-username-input"
            class="block text-xs font-medium text-slate-300 mb-1.5"
            >Username</label
          >
          <input
            id="new-username-input"
            type="text"
            bind:value={newUsername}
            placeholder="e.g. devops-lead"
            class="w-full px-3.5 py-2 rounded-xl bg-[#0a0b10] border border-white/[0.08] text-white text-xs placeholder-slate-600 focus:outline-none focus:border-violet-500"
          />
        </div>

        <div>
          <label
            for="new-user-password-input"
            class="block text-xs font-medium text-slate-300 mb-1.5"
            >Password</label
          >
          <input
            id="new-user-password-input"
            type="password"
            bind:value={newUserPassword}
            placeholder="&bull;&bull;&bull;&bull;&bull;&bull;&bull;&bull;"
            class="w-full px-3.5 py-2 rounded-xl bg-[#0a0b10] border border-white/[0.08] text-white text-xs placeholder-slate-600 focus:outline-none focus:border-violet-500"
          />
        </div>

        <div>
          <label
            for="new-user-role-select"
            class="block text-xs font-medium text-slate-300 mb-1.5"
            >Role & Privileges</label
          >
          <select
            id="new-user-role-select"
            bind:value={newUserRole}
            class="w-full px-3.5 py-2.5 rounded-xl bg-[#0a0b10] border border-white/[0.08] text-white text-xs focus:outline-none focus:border-violet-500"
          >
            <option value="admin">Admin (Full administrative control)</option>
            <option value="operator"
              >Operator (Deploy stacks, start/stop containers)</option
            >
            <option value="viewer">Viewer (Read-only access)</option>
          </select>
        </div>

        <div
          class="pt-3 flex items-center justify-end gap-3 border-t border-white/[0.08]"
        >
          <button
            type="button"
            onclick={() => (isAddUserModalOpen = false)}
            class="px-4 py-2 rounded-xl text-xs font-medium text-slate-400 hover:text-white transition"
          >
            Cancel
          </button>
          <button
            type="submit"
            disabled={isCreatingUser}
            class="flex items-center gap-2 px-4 py-2 rounded-xl bg-violet-600 hover:bg-violet-500 text-white text-xs font-semibold shadow-[0_0_15px_rgba(124,58,237,0.3)] transition disabled:opacity-50"
          >
            {#if isCreatingUser}
              <LoaderCircle class="w-3.5 h-3.5 animate-spin" />
              <span>Creating...</span>
            {:else}
              <Plus class="w-3.5 h-3.5" />
              <span>Create User</span>
            {/if}
          </button>
        </div>
      </form>
    </div>
  </div>
{/if}

<!-- DELETE USER CONFIRMATION MODAL -->
{#if deletingUser}
  <div
    class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-sm animate-in fade-in duration-150"
  >
    <div
      class="w-full max-w-sm p-6 rounded-2xl bg-[#12131c] border border-white/[0.1] shadow-2xl space-y-4"
    >
      <div class="flex items-center gap-2.5 text-rose-400">
        <Trash class="w-5 h-5" />
        <h3 class="text-sm font-bold uppercase tracking-wider text-white">
          Delete User Account
        </h3>
      </div>

      <p class="text-xs text-slate-300">
        Are you sure you want to permanently delete user <span
          class="font-bold text-white font-mono">{deletingUser.username}</span
        >? This action cannot be undone.
      </p>

      <div class="pt-2 flex items-center justify-end gap-3">
        <button
          onclick={() => (deletingUser = null)}
          class="px-4 py-2 rounded-xl text-xs font-medium text-slate-400 hover:text-white transition"
        >
          Cancel
        </button>
        <button
          onclick={handleDeleteUser}
          disabled={isDeletingUser}
          class="flex items-center gap-1.5 px-4 py-2 rounded-xl bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold shadow-[0_0_15px_rgba(244,63,94,0.3)] transition disabled:opacity-50"
        >
          {#if isDeletingUser}
            <LoaderCircle class="w-3.5 h-3.5 animate-spin" />
            <span>Deleting...</span>
          {:else}
            <Trash class="w-3.5 h-3.5" />
            <span>Delete User</span>
          {/if}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- GENERATE TOKEN MODAL -->
{#if isCreateTokenModalOpen}
  <div
    class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-sm animate-in fade-in duration-150"
  >
    <div
      class="w-full max-w-lg p-6 rounded-2xl bg-[#12131c] border border-white/[0.1] shadow-2xl space-y-5"
    >
      <div
        class="flex items-center justify-between border-b border-white/[0.08] pb-3"
      >
        <div class="flex items-center gap-2">
          <Key class="w-4 h-4 text-violet-400" />
          <h3 class="text-sm font-bold text-white uppercase tracking-wider">
            {generatedRawToken
              ? "Token Created Successfully"
              : "Generate API Access Token"}
          </h3>
        </div>
        <button
          onclick={handleCloseTokenModal}
          class="p-1 rounded-lg text-slate-400 hover:text-white hover:bg-white/5 transition"
        >
          <X class="w-4 h-4" />
        </button>
      </div>

      {#if generatedRawToken}
        <!-- Step 2: Show raw token once -->
        <div class="space-y-4">
          <div
            class="p-3.5 rounded-xl bg-amber-500/10 border border-amber-500/30 text-amber-300 text-xs flex items-start gap-2.5"
          >
            <CircleAlert class="w-4 h-4 shrink-0 mt-0.5" />
            <div>
              <span class="font-bold">Copy your token now.</span>
              <p class="mt-0.5 text-amber-200/90 text-[11px]">
                For security reasons, this token will never be displayed again.
                If you lose it, you will need to generate a new one.
              </p>
            </div>
          </div>

          <div>
            <label
              for="generated-token-field"
              class="block text-xs font-medium text-slate-400 mb-1.5"
              >Raw API Bearer Token</label
            >
            <div class="relative">
              <input
                id="generated-token-field"
                type="text"
                readonly
                value={generatedRawToken}
                class="w-full px-3.5 py-2.5 rounded-xl bg-[#0a0b10] border border-violet-500/40 text-violet-300 font-mono text-xs pr-24 shadow-inner select-all"
              />
              <button
                type="button"
                onclick={copyTokenToClipboard}
                class="absolute right-2 top-1/2 -translate-y-1/2 flex items-center gap-1.5 px-2.5 py-1 rounded-lg bg-violet-600 hover:bg-violet-500 text-white text-[11px] font-medium transition"
              >
                {#if isCopied}
                  <Check class="w-3 h-3 text-emerald-300" />
                  <span class="text-emerald-300">Copied</span>
                {:else}
                  <Copy class="w-3 h-3" />
                  <span>Copy</span>
                {/if}
              </button>
            </div>
          </div>

          <div
            class="p-3.5 rounded-xl bg-[#0a0b10] border border-white/[0.04] space-y-1 text-xs"
          >
            <span class="text-slate-400 font-medium">HTTP Header Example:</span>
            <div
              class="font-mono text-[11px] text-slate-300 bg-black/40 p-2 rounded-lg truncate"
            >
              Authorization: Bearer {generatedRawToken}
            </div>
          </div>

          <div class="pt-2 flex justify-end">
            <button
              type="button"
              onclick={handleCloseTokenModal}
              class="px-5 py-2 rounded-xl bg-violet-600 hover:bg-violet-500 text-white text-xs font-semibold shadow-[0_0_15px_rgba(124,58,237,0.3)] transition"
            >
              Done & Return
            </button>
          </div>
        </div>
      {:else}
        <!-- Step 1: Token Generation Form -->
        {#if createTokenError}
          <div
            class="p-3 rounded-xl bg-rose-500/10 border border-rose-500/30 text-rose-300 text-xs flex items-center gap-2"
          >
            <CircleAlert class="w-4 h-4 shrink-0" />
            <span>{createTokenError}</span>
          </div>
        {/if}

        <form onsubmit={handleCreateToken} class="space-y-4">
          <div>
            <label
              for="token-name-input"
              class="block text-xs font-medium text-slate-300 mb-1.5"
              >Token Name / Description</label
            >
            <input
              id="token-name-input"
              type="text"
              bind:value={tokenName}
              placeholder="e.g. GitHub Actions CI, Swarm Backup Daemon"
              class="w-full px-3.5 py-2 rounded-xl bg-[#0a0b10] border border-white/[0.08] text-white text-xs placeholder-slate-600 focus:outline-none focus:border-violet-500"
            />
          </div>

          <div>
            <label
              for="token-expiration-select"
              class="block text-xs font-medium text-slate-300 mb-1.5"
              >Expiration Period</label
            >
            <select
              id="token-expiration-select"
              bind:value={tokenExpiresDays}
              class="w-full px-3.5 py-2.5 rounded-xl bg-[#0a0b10] border border-white/[0.08] text-white text-xs focus:outline-none focus:border-violet-500"
            >
              <option value={7}>7 Days</option>
              <option value={30}>30 Days (Recommended)</option>
              <option value={90}>90 Days</option>
              <option value={365}>1 Year</option>
              <option value={0}>Never Expires</option>
            </select>
          </div>

          <div
            class="pt-3 flex items-center justify-end gap-3 border-t border-white/[0.08]"
          >
            <button
              type="button"
              onclick={handleCloseTokenModal}
              class="px-4 py-2 rounded-xl text-xs font-medium text-slate-400 hover:text-white transition"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={isGeneratingToken}
              class="flex items-center gap-2 px-4 py-2 rounded-xl bg-violet-600 hover:bg-violet-500 text-white text-xs font-semibold shadow-[0_0_15px_rgba(124,58,237,0.3)] transition disabled:opacity-50"
            >
              {#if isGeneratingToken}
                <LoaderCircle class="w-3.5 h-3.5 animate-spin" />
                <span>Generating...</span>
              {:else}
                <Key class="w-3.5 h-3.5" />
                <span>Generate Token</span>
              {/if}
            </button>
          </div>
        </form>
      {/if}
    </div>
  </div>
{/if}

<!-- REVOKE TOKEN CONFIRMATION MODAL -->
{#if revokingToken}
  <div
    class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-sm animate-in fade-in duration-150"
  >
    <div
      class="w-full max-w-sm p-6 rounded-2xl bg-[#12131c] border border-white/[0.1] shadow-2xl space-y-4"
    >
      <div class="flex items-center gap-2.5 text-rose-400">
        <Trash class="w-5 h-5" />
        <h3 class="text-sm font-bold uppercase tracking-wider text-white">
          Revoke Access Token
        </h3>
      </div>

      <p class="text-xs text-slate-300">
        Are you sure you want to revoke token <span
          class="font-bold text-white font-mono">{revokingToken.name}</span
        >
        (<code class="text-cyan-300">{revokingToken.prefix}</code>)? Any scripts
        using this token will be immediately denied access.
      </p>

      <div class="pt-2 flex items-center justify-end gap-3">
        <button
          onclick={() => (revokingToken = null)}
          class="px-4 py-2 rounded-xl text-xs font-medium text-slate-400 hover:text-white transition"
        >
          Cancel
        </button>
        <button
          onclick={handleRevokeToken}
          disabled={isRevokingToken}
          class="flex items-center gap-1.5 px-4 py-2 rounded-xl bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold shadow-[0_0_15px_rgba(244,63,94,0.3)] transition disabled:opacity-50"
        >
          {#if isRevokingToken}
            <LoaderCircle class="w-3.5 h-3.5 animate-spin" />
            <span>Revoking...</span>
          {:else}
            <Trash class="w-3.5 h-3.5" />
            <span>Revoke Token</span>
          {/if}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- ADD CUSTOM CATALOG MODAL -->
{#if isAddRepoModalOpen}
  <div
    class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-sm animate-in fade-in duration-150"
  >
    <div
      class="w-full max-w-lg p-6 rounded-2xl bg-[#12131c] border border-white/[0.1] shadow-2xl space-y-5"
    >
      <div
        class="flex items-center justify-between border-b border-white/[0.08] pb-3"
      >
        <div class="flex items-center gap-2">
          <Boxes class="w-4 h-4 text-cyan-400" />
          <h3 class="text-sm font-bold text-white uppercase tracking-wider">
            Add Template Catalog
          </h3>
        </div>
        <button
          onclick={() => (isAddRepoModalOpen = false)}
          class="p-1 rounded-lg text-slate-400 hover:text-white hover:bg-white/5 transition"
        >
          <X class="w-4 h-4" />
        </button>
      </div>

      <!-- Quick Preset Buttons -->
      <div class="space-y-1.5">
        <span
          class="text-[11px] font-medium text-slate-400 uppercase tracking-wider"
          >Quick Presets</span
        >
        <div class="flex flex-wrap gap-1.5">
          <button
            type="button"
            onclick={() =>
              applyRepoPreset(
                "Lissy93 Portainer Templates",
                "https://raw.githubusercontent.com/Lissy93/portainer-templates/main/templates.json",
                "portainer",
              )}
            class="px-2.5 py-1 rounded-lg bg-violet-600/20 hover:bg-violet-600/30 text-violet-300 border border-violet-500/30 text-[11px] font-medium transition"
          >
            Lissy93 Portainer JSON
          </button>
          <button
            type="button"
            onclick={() =>
              applyRepoPreset(
                "CasaOS LinuxServer AppStore",
                "https://casaos-appstore.paodayag.dev/linuxserver.zip",
                "casaos_zip",
              )}
            class="px-2.5 py-1 rounded-lg bg-cyan-600/20 hover:bg-cyan-600/30 text-cyan-300 border border-cyan-500/30 text-[11px] font-medium transition"
          >
            LinuxServer CasaOS Zip
          </button>
          <button
            type="button"
            onclick={() =>
              applyRepoPreset(
                "Official Portainer Templates",
                "https://raw.githubusercontent.com/portainer/templates/master/templates-2.0.json",
                "portainer",
              )}
            class="px-2.5 py-1 rounded-lg bg-emerald-600/20 hover:bg-emerald-600/30 text-emerald-300 border border-emerald-500/30 text-[11px] font-medium transition"
          >
            Portainer 2.0 Spec
          </button>
        </div>
      </div>

      {#if createRepoError}
        <div
          class="p-3 rounded-xl bg-rose-500/10 border border-rose-500/30 text-rose-300 text-xs flex items-center gap-2"
        >
          <CircleAlert class="w-4 h-4 shrink-0" />
          <span>{createRepoError}</span>
        </div>
      {/if}

      <form onsubmit={handleCreateRepo} class="space-y-4">
        <div>
          <label
            for="catalog-name-input"
            class="block text-xs font-medium text-slate-300 mb-1.5"
            >Catalog Name</label
          >
          <input
            id="catalog-name-input"
            type="text"
            bind:value={newRepoName}
            placeholder="e.g. Lissy93 Portainer Templates"
            class="w-full px-3.5 py-2 rounded-xl bg-[#0a0b10] border border-white/[0.08] text-white text-xs placeholder-slate-600 focus:outline-none focus:border-cyan-500"
          />
        </div>

        <div>
          <label
            for="catalog-url-input"
            class="block text-xs font-medium text-slate-300 mb-1.5"
            >Feed URL (.json or .zip)</label
          >
          <input
            id="catalog-url-input"
            type="url"
            bind:value={newRepoUrl}
            placeholder="https://.../templates.json or https://.../store.zip"
            class="w-full px-3.5 py-2 rounded-xl bg-[#0a0b10] border border-white/[0.08] text-white text-xs font-mono placeholder-slate-600 focus:outline-none focus:border-cyan-500"
          />
        </div>

        <div class="grid grid-cols-2 gap-3">
          <div>
            <label
              for="catalog-type-select"
              class="block text-xs font-medium text-slate-300 mb-1.5"
              >Catalog Type</label
            >
            <select
              id="catalog-type-select"
              bind:value={newRepoType}
              class="w-full px-3.5 py-2 rounded-xl bg-[#0a0b10] border border-white/[0.08] text-white text-xs focus:outline-none focus:border-cyan-500"
            >
              <option value="auto">Auto-Detect</option>
              <option value="portainer">Portainer JSON Feed</option>
              <option value="casaos_zip">CasaOS Zip Archive</option>
            </select>
          </div>

          <div class="flex items-center pt-5">
            <label
              class="flex items-center gap-2 text-xs text-slate-300 cursor-pointer select-none"
            >
              <input
                type="checkbox"
                bind:checked={newRepoEnabled}
                class="rounded bg-[#0a0b10] border-white/20 text-cyan-500 focus:ring-0 focus:ring-offset-0"
              />
              <span>Enable immediately</span>
            </label>
          </div>
        </div>

        <div
          class="pt-3 flex items-center justify-end gap-3 border-t border-white/[0.08]"
        >
          <button
            type="button"
            onclick={() => (isAddRepoModalOpen = false)}
            class="px-4 py-2 rounded-xl text-xs font-medium text-slate-400 hover:text-white transition"
          >
            Cancel
          </button>
          <button
            type="submit"
            disabled={isCreatingRepo}
            class="flex items-center gap-2 px-4 py-2 rounded-xl bg-cyan-600 hover:bg-cyan-500 text-white text-xs font-semibold shadow-[0_0_15px_rgba(6,182,212,0.3)] transition disabled:opacity-50"
          >
            {#if isCreatingRepo}
              <LoaderCircle class="w-3.5 h-3.5 animate-spin" />
              <span>Fetching & Syncing...</span>
            {:else}
              <Plus class="w-3.5 h-3.5" />
              <span>Add & Sync Catalog</span>
            {/if}
          </button>
        </div>
      </form>
    </div>
  </div>
{/if}

<!-- DELETE CUSTOM CATALOG CONFIRMATION MODAL -->
{#if deletingRepo}
  <div
    class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-sm animate-in fade-in duration-150"
  >
    <div
      class="w-full max-w-sm p-6 rounded-2xl bg-[#12131c] border border-white/[0.1] shadow-2xl space-y-4"
    >
      <div class="flex items-center gap-2.5 text-rose-400">
        <Trash class="w-5 h-5" />
        <h3 class="text-sm font-bold uppercase tracking-wider text-white">
          Delete Catalog
        </h3>
      </div>

      <p class="text-xs text-slate-300">
        Are you sure you want to remove catalog <span
          class="font-bold text-white font-mono">{deletingRepo.name}</span
        >? All {deletingRepo.item_count} applications from this source will be removed
        from your App Store.
      </p>

      <div class="pt-2 flex items-center justify-end gap-3">
        <button
          onclick={() => (deletingRepo = null)}
          class="px-4 py-2 rounded-xl text-xs font-medium text-slate-400 hover:text-white transition"
        >
          Cancel
        </button>
        <button
          onclick={handleDeleteRepo}
          disabled={isDeletingRepo}
          class="flex items-center gap-1.5 px-4 py-2 rounded-xl bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold shadow-[0_0_15px_rgba(244,63,94,0.3)] transition disabled:opacity-50"
        >
          {#if isDeletingRepo}
            <LoaderCircle class="w-3.5 h-3.5 animate-spin" />
            <span>Deleting...</span>
          {:else}
            <Trash class="w-3.5 h-3.5" />
            <span>Delete Catalog</span>
          {/if}
        </button>
      </div>
    </div>
  </div>
{/if}
