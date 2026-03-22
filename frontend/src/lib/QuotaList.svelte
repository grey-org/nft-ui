<script>
  import {
    sortedQuotas,
    loading,
    selectedIds,
    hasSelection,
    selectedCount,
    readOnly,
    clearSelection,
    selectAll,
    loadQuotas,
    success,
    errorNotify,
  } from './stores.js';
  import { batchResetQuotas } from './api.js';
  import QuotaItem from './QuotaItem.svelte';
  import AddQuotaModal from './AddQuotaModal.svelte';
  import ConfirmDialog from './ConfirmDialog.svelte';

  let showAddModal = $state(false);
  let showBatchResetConfirm = $state(false);
  let batchResetting = $state(false);

  async function handleBatchReset() {
    batchResetting = true;
    try {
      const ids = Array.from($selectedIds);
      await batchResetQuotas(ids);
      success(`Reset ${ids.length} quota(s) successfully`);
      clearSelection();
      await loadQuotas();
    } catch (e) {
      errorNotify(`Failed to reset quotas: ${e.message}`);
    } finally {
      batchResetting = false;
      showBatchResetConfirm = false;
    }
  }
</script>

<div class="card mb-4 overflow-hidden">
  <!-- Pane header -->
  <div class="flex justify-between items-center px-3 py-2" style="border-bottom: 0.5px solid var(--border);">
    <div class="flex items-center gap-2">
      <span class="dot-amber"></span>
      <span style="font-size: 9px; font-weight: 700; letter-spacing: 0.12em; text-transform: uppercase; color: var(--text-muted);">Quota Rules</span>
      {#if $hasSelection}
        <span style="font-size: 10px; color: var(--text-muted);">— {$selectedCount} selected</span>
      {/if}
    </div>
    <div class="flex items-center gap-2">
      {#if $hasSelection}
        <button class="btn btn-sm btn-secondary" onclick={clearSelection}>clear</button>
        {#if !$readOnly}
          <button
            class="btn btn-sm btn-danger"
            onclick={() => (showBatchResetConfirm = true)}
            disabled={batchResetting}
          >
            reset selected
          </button>
        {/if}
      {:else}
        <button class="btn btn-sm btn-secondary" onclick={selectAll}>select all</button>
      {/if}
      {#if !$readOnly}
        <button class="btn btn-sm btn-primary" onclick={() => (showAddModal = true)}>+ add rule</button>
      {/if}
    </div>
  </div>

  <!-- Table -->
  {#if $loading && $sortedQuotas.length === 0}
    <div class="py-8 text-center" style="color: var(--text-muted); font-size: 11px;">loading…</div>
  {:else if $sortedQuotas.length === 0}
    <div class="py-8 text-center" style="color: var(--text-muted); font-size: 11px;">no quota rules found</div>
  {:else}
    <table class="data-table">
      <thead>
        <tr>
          <th class="w-10"></th>
          <th>Port</th>
          <th class="hidden md:table-cell">Usage</th>
          <th>Progress</th>
          <th class="hidden md:table-cell">Status</th>
          <th class="w-12"></th>
        </tr>
      </thead>
      <tbody>
        {#each $sortedQuotas as quota (quota.id)}
          <QuotaItem {quota} />
        {/each}
      </tbody>
    </table>
  {/if}
</div>

{#if showAddModal}
  <AddQuotaModal onclose={() => (showAddModal = false)} />
{/if}

{#if showBatchResetConfirm}
  <ConfirmDialog
    title="Reset Quotas"
    message={`Reset ${$selectedCount} quota(s)? This sets used traffic to 0.`}
    confirmText="Reset"
    onconfirm={handleBatchReset}
    oncancel={() => (showBatchResetConfirm = false)}
  />
{/if}
