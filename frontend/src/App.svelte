<script>
  import { onMount } from 'svelte';
  import {
    loadQuotas,
    loadForwardingRules,
    loading,
    error,
    readOnly,
    refreshInterval,
    notifications,
    removeNotification,
    isEditingModal,
    exportBackup,
    importBackup,
  } from './lib/stores.js';
  import QuotaList from './lib/QuotaList.svelte';
  import PortList from './lib/PortList.svelte';
  import ForwardingList from './lib/ForwardingList.svelte';
  import RawRuleset from './lib/RawRuleset.svelte';
  import Toast from './lib/Toast.svelte';
  import PublicQuery from './lib/PublicQuery.svelte';

  // Detect route immediately at script initialization
  function getInitialRoute() {
    if (typeof window !== 'undefined') {
      const path = window.location.pathname;
      if (path === '/query' || path.startsWith('/query')) {
        return 'query';
      }
    }
    return 'admin';
  }

  let refreshTimer = $state(null);
  let currentRoute = $state(getInitialRoute());

  // Premium theme management
  const THEMES = [
    { id: 'midnight', name: 'Midnight' },
    { id: 'obsidian', name: 'Obsidian' },
    { id: 'nord', name: 'Nord' },
    { id: 'sunset', name: 'Sunset' }
  ];
  let currentTheme = $state('midnight');

  onMount(() => {
    // Load theme from localStorage
    const savedTheme = localStorage.getItem('theme') || 'midnight';
    currentTheme = savedTheme;
    applyTheme(savedTheme);

    // Only load data for admin route
    if (currentRoute === 'admin') {
      loadQuotas();
      loadForwardingRules();
      startAutoRefresh();
    }

    return () => stopAutoRefresh();
  });

  function applyTheme(theme) {
    if (typeof document !== 'undefined') {
      document.body.dataset.theme = theme;
    }
  }

  function handleThemeChange(event) {
    const newTheme = event.target.value;
    currentTheme = newTheme;
    applyTheme(newTheme);
    localStorage.setItem('theme', newTheme);
  }

  function startAutoRefresh() {
    stopAutoRefresh();
    const interval = $refreshInterval;
    if (interval > 0) {
      refreshTimer = setInterval(() => {
        // Skip refresh if user is editing in a modal
        if ($isEditingModal) return;
        loadQuotas();
        loadForwardingRules();
      }, interval * 1000);
    }
  }

  function stopAutoRefresh() {
    if (refreshTimer) {
      clearInterval(refreshTimer);
      refreshTimer = null;
    }
  }

  function handleRefresh() {
    loadQuotas();
    loadForwardingRules();
  }

  let fileInput = $state();

  function handleExport() {
    exportBackup();
  }

  function handleImport() {
    fileInput?.click();
  }

  async function handleFileSelect(event) {
    const file = event.target.files?.[0];
    if (file) {
      await importBackup(file);
      // Clear the input so the same file can be imported again
      event.target.value = '';
    }
  }
</script>

{#if currentRoute === 'query'}
  <PublicQuery />
{:else}
  <div class="min-h-screen" style="background-color: var(--bg);">
    <!-- Header with subtle border and frosted glass effect -->
    <header style="background-color: var(--surface); border-bottom: 1px solid var(--border); backdrop-filter: blur(8px);">
      <div class="container mx-auto px-5 py-4">
        <div class="flex justify-between items-center">
          <h1 class="text-2xl font-semibold" style="color: var(--text);">
            <span style="color: var(--primary);">nft</span>-ui
          </h1>
          <div class="flex items-center gap-3">
            <!-- Premium theme switcher -->
            <select
              class="select"
              bind:value={currentTheme}
              onchange={handleThemeChange}
            >
              {#each THEMES as theme}
                <option value={theme.id}>{theme.name}</option>
              {/each}
            </select>
            
            {#if $readOnly}
              <span class="badge badge-warning">Read Only</span>
            {/if}
            
            <button
              class="btn btn-secondary"
              onclick={handleExport}
              disabled={$loading}
            >
              Export
            </button>

            <button
              class="btn btn-secondary"
              onclick={handleImport}
              disabled={$loading || $readOnly}
            >
              Import
            </button>

            <button
              class="btn btn-secondary"
              onclick={handleRefresh}
              disabled={$loading}
            >
              {$loading ? 'Refreshing...' : 'Refresh'}
            </button>

            <!-- Hidden file input for import -->
            <input
              type="file"
              accept=".json"
              bind:this={fileInput}
              onchange={handleFileSelect}
              style="display: none;"
            />
          </div>
        </div>
      </div>
    </header>

    <main class="container mx-auto px-5 py-6">
      {#if $error}
        <div class="alert alert-error mb-5">
          <div>
            <strong>Error:</strong> {$error}
          </div>
          <button class="btn btn-sm btn-danger" onclick={handleRefresh}>Retry</button>
        </div>
      {/if}

      <QuotaList />
      <PortList />
      <ForwardingList />
      <RawRuleset />
    </main>

    <!-- Toast notifications -->
    <div class="fixed bottom-5 right-5 flex flex-col gap-2 z-[2000]">
      {#each $notifications as notification (notification.id)}
        <Toast
          message={notification.message}
          type={notification.type}
          onclose={() => removeNotification(notification.id)}
        />
      {/each}
    </div>
  </div>
{/if}
