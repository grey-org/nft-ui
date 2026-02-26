<script>
  import { selectedIds, toggleSelection, readOnly, loadQuotas, success, errorNotify, allowedPorts } from './stores.js';
  import { resetQuota, deleteQuota } from './api.js';
  import { formatBytes, formatPercent, getProgressColor, getStatusColor } from './utils.js';
  import ConfirmDialog from './ConfirmDialog.svelte';
  import EditQuotaModal from './EditQuotaModal.svelte';

  let { quota } = $props();

  let expanded = $state(false);
  let showResetConfirm = $state(false);
  let showDeleteConfirm = $state(false);
  let showEditModal = $state(false);
  let processing = $state(false);
  let copiedToken = $state(false);
  let copiedUrl = $state(false);

  let isSelected = $derived($selectedIds.has(quota.id));
  let progressColor = $derived(getProgressColor(quota.usage_percent));
  let statusColor = $derived(getStatusColor(quota.status));
  let hasInbound = $derived($allowedPorts.some(p => p.port === quota.port));
  let ringPercent = $derived(Math.min(quota.usage_percent, 100));
  let queryUrl = $derived(quota.token ? `${window.location.origin}/query?token=${quota.token}` : '');

  function handleCheckbox(e) {
    e.stopPropagation();
    toggleSelection(quota.id);
  }

  function toggleExpand() {
    expanded = !expanded;
  }

  async function handleReset() {
    processing = true;
    try {
      await resetQuota(quota.id);
      success('Quota reset successfully');
      await loadQuotas();
    } catch (e) {
      errorNotify(`Failed to reset quota: ${e.message}`);
    } finally {
      processing = false;
      showResetConfirm = false;
    }
  }

  async function handleDelete() {
    processing = true;
    try {
      await deleteQuota(quota.id);
      success('Quota deleted successfully');
      await loadQuotas();
    } catch (e) {
      errorNotify(`Failed to delete quota: ${e.message}`);
    } finally {
      processing = false;
      showDeleteConfirm = false;
    }
  }

  function copyToken() {
    if (quota.token) {
      navigator.clipboard.writeText(quota.token);
      copiedToken = true;
      setTimeout(() => copiedToken = false, 2000);
    }
  }

  function copyQueryUrl() {
    if (queryUrl) {
      navigator.clipboard.writeText(queryUrl);
      copiedUrl = true;
      setTimeout(() => copiedUrl = false, 2000);
    }
  }
</script>

<tr class="data-row" class:selected={isSelected} onclick={toggleExpand}>
  <td class="w-10">
    <input
      type="checkbox"
      class="w-[18px] h-[18px] cursor-pointer accent-[var(--primary)]"
      checked={isSelected}
      onclick={handleCheckbox}
    />
  </td>
  <td>
    <div class="flex items-center gap-2">
      <span
        class="status-dot"
        class:status-dot-active={hasInbound}
        class:status-dot-warning={!hasInbound}
        title={hasInbound ? 'Inbound allowed' : 'No inbound rule'}
      ></span>
      <span class="font-semibold text-base" style="color: var(--text);">{quota.port}</span>
    </div>
  </td>
  <td class="hidden md:table-cell">
    <div class="flex items-center gap-1 text-sm">
      <span class="font-semibold" style="color: var(--text);">{formatBytes(quota.used_bytes)}</span>
      <span style="color: var(--text-muted);">/</span>
      <span style="color: var(--text-muted);">{formatBytes(quota.quota_bytes)}</span>
    </div>
  </td>
  <td>
    <div class="flex items-center gap-2">
      <div class="flex-1 h-2 rounded-full overflow-hidden" style="background-color: var(--border); min-width: 60px;">
        <div
          class="h-full rounded-full transition-all duration-300"
          style="width: {ringPercent}%; background-color: {progressColor};"
        ></div>
      </div>
      <span class="text-xs font-semibold min-w-[40px]" style="color: var(--text);">{formatPercent(quota.usage_percent)}</span>
    </div>
  </td>
  <td class="hidden md:table-cell">
    <div class="flex items-center gap-2">
      <span class="w-2 h-2 rounded-full" style="background-color: {statusColor}; box-shadow: 0 0 6px {statusColor};"></span>
      <span class="text-sm capitalize" style="color: var(--text);">{quota.status}</span>
    </div>
  </td>
  <td class="w-12 text-center">
    <span class="text-xl" style="color: var(--text-muted);">{expanded ? '−' : '+'}</span>
  </td>
</tr>

{#if expanded}
  <tr class="detail-row">
    <td colspan="6">
      <div class="px-4 pb-4 pt-2 animate-[slideDown_0.2s_ease]">
        {#if quota.comment}
          <div class="flex gap-2 mb-2 text-sm">
            <span style="color: var(--text-muted);">Comment:</span>
            <span style="color: var(--text);">{quota.comment}</span>
          </div>
        {/if}
        <div class="flex gap-2 mb-2 text-sm">
          <span style="color: var(--text-muted);">ID:</span>
          <span class="font-mono text-xs px-1.5 py-0.5 rounded" style="background-color: var(--bg); color: var(--text); border: 1px solid var(--border);">{quota.id}</span>
        </div>
        {#if quota.token}
          <div class="flex gap-2 mb-2 text-sm">
            <span style="color: var(--text-muted);">Query Token:</span>
            <span class="inline-flex items-center gap-2 font-mono text-xs px-1.5 py-0.5 rounded" style="background-color: var(--bg); color: var(--text); border: 1px solid var(--border);">
              {quota.token}
              <button
                class="px-2 py-0.5 text-[11px] rounded cursor-pointer transition-all"
                style="background-color: var(--surface-hover); border: 1px solid var(--border); color: var(--text-muted);"
                onmouseover={(e) => e.currentTarget.style.backgroundColor = 'var(--border)'}
                onmouseout={(e) => e.currentTarget.style.backgroundColor = 'var(--surface-hover)'}
                onclick={copyToken}
                title="Copy token"
              >
                {copiedToken ? 'Copied!' : 'Copy'}
              </button>
            </span>
          </div>
          <div class="flex gap-2 mb-2 text-sm">
            <span style="color: var(--text-muted);">Query URL:</span>
            <span class="inline-flex items-center gap-2 font-mono text-xs px-1.5 py-0.5 rounded" style="background-color: var(--bg); color: var(--text); border: 1px solid var(--border);">
              <a href={queryUrl} target="_blank" class="no-underline hover:underline" style="color: var(--primary);">
                /query?token={quota.token}
              </a>
              <button
                class="px-2 py-0.5 text-[11px] rounded cursor-pointer transition-all"
                style="background-color: var(--surface-hover); border: 1px solid var(--border); color: var(--text-muted);"
                onmouseover={(e) => e.currentTarget.style.backgroundColor = 'var(--border)'}
                onmouseout={(e) => e.currentTarget.style.backgroundColor = 'var(--surface-hover)'}
                onclick={copyQueryUrl}
                title="Copy URL"
              >
                {copiedUrl ? 'Copied!' : 'Copy'}
              </button>
            </span>
          </div>
        {/if}

        {#if !$readOnly}
          <div class="flex gap-2 mt-4">
            <button
              class="btn btn-sm btn-secondary"
              onclick={() => (showResetConfirm = true)}
              disabled={processing}
            >
              Reset
            </button>
            <button
              class="btn btn-sm btn-secondary"
              onclick={() => (showEditModal = true)}
              disabled={processing}
            >
              Edit
            </button>
            <button
              class="btn btn-sm btn-danger"
              onclick={() => (showDeleteConfirm = true)}
              disabled={processing}
            >
              Delete
            </button>
          </div>
        {/if}
      </div>
    </td>
  </tr>
{/if}

<!-- Reset confirm dialog -->
{#if showResetConfirm}
  <ConfirmDialog
    title="Reset Quota"
    message={`Are you sure you want to reset the quota for port ${quota.port}? This will set the used traffic to 0.`}
    confirmText="Reset"
    onconfirm={handleReset}
    oncancel={() => (showResetConfirm = false)}
  />
{/if}

<!-- Delete confirm dialog -->
{#if showDeleteConfirm}
  <ConfirmDialog
    title="Delete Quota"
    message={`Are you sure you want to delete the quota rule for port ${quota.port}? This action cannot be undone.`}
    confirmText="Delete"
    danger={true}
    onconfirm={handleDelete}
    oncancel={() => (showDeleteConfirm = false)}
  />
{/if}

<!-- Edit modal -->
{#if showEditModal}
  <EditQuotaModal {quota} onclose={() => (showEditModal = false)} />
{/if}

<style>
  .selected td {
    background-color: rgba(59, 130, 246, 0.1);
  }

  @keyframes slideDown {
    from {
      opacity: 0;
      transform: translateY(-10px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }
</style>
