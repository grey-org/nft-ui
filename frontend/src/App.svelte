<script>
  import { onMount } from 'svelte';
  import {
    loadQuotas,
    loadForwardingRules,
    loadIfaceForwardingRules,
    loading,
    error,
    readOnly,
    refreshInterval,
    notifications,
    removeNotification,
    isEditingModal,
    theme,
  } from './lib/stores.js';
  import QuotaList from './lib/QuotaList.svelte';
  import PortList from './lib/PortList.svelte';
  import ForwardingList from './lib/ForwardingList.svelte';
  import IfaceForwardingList from './lib/IfaceForwardingList.svelte';
  import RawRuleset from './lib/RawRuleset.svelte';
  import BypassPanel from './lib/BypassPanel.svelte';
  import BackupPanel from './lib/BackupPanel.svelte';
  import Toast from './lib/Toast.svelte';
  import PublicQuery from './lib/PublicQuery.svelte';

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

  onMount(() => {
    document.documentElement.setAttribute('data-theme', $theme);
    if (currentRoute === 'admin') {
      loadQuotas();
      loadForwardingRules();
      loadIfaceForwardingRules();
    }
    return () => stopAutoRefresh();
  });

  function startAutoRefresh() {
    stopAutoRefresh();
    const interval = $refreshInterval;
    if (interval > 0) {
      refreshTimer = setInterval(() => {
        if ($isEditingModal) return;
        loadQuotas();
        loadForwardingRules();
        loadIfaceForwardingRules();
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
    loadIfaceForwardingRules();
  }


</script>

{#if currentRoute === 'query'}
  <PublicQuery />
{:else}
  <div class="min-h-screen" style="background-color: var(--bg);">
    <header style="background-color: var(--surface); border-bottom: 0.5px solid var(--border);">
      <div class="container mx-auto px-4 py-2">
        <div class="flex justify-between items-center">
          <div class="flex items-center gap-3">
            <span style="font-size: 13px; color: var(--text-muted); letter-spacing: 0.05em; font-family: inherit;">nft-ui</span>
            <span style="font-size: 9px; color: var(--text-dim); letter-spacing: 0.1em; text-transform: uppercase;">firewall manager</span>
          </div>
          <div class="flex items-center gap-2">
            {#if $readOnly}
              <span class="badge badge-warning">read-only</span>
            {/if}
            <div class="flex items-center" style="border: 0.5px solid var(--border); border-radius: 2px;">
              {#each ['dark', 'auto', 'light'] as mode}
                <button
                  class="btn btn-sm"
                  class:btn-primary={$theme === mode}
                  style="border: none; border-radius: 0;"
                  onclick={() => theme.set(mode)}
                >{mode}</button>
              {/each}
            </div>
            <button class="btn btn-sm btn-secondary" onclick={handleRefresh} disabled={$loading}>
              {$loading ? 'loading…' : 'refresh'}
            </button>
          </div>
        </div>
      </div>
    </header>

    <main class="container mx-auto px-4 py-4">
      {#if $error}
        <div class="alert alert-error mb-4">
          <div>
            <strong>error:</strong> {$error}
          </div>
          <button class="btn btn-sm btn-danger" onclick={handleRefresh}>retry</button>
        </div>
      {/if}

      <QuotaList />
      <PortList />
      <ForwardingList />
      <IfaceForwardingList />
      <BypassPanel />
      <BackupPanel />
      <RawRuleset />
    </main>

    <div class="fixed bottom-4 right-4 flex flex-col gap-2 z-[2000]">
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
