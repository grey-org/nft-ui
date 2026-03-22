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
      style="width: 12px; height: 12px; cursor: pointer; accent-color: var(--amber);"
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
      <span style="font-size: 12px; color: var(--text);">{quota.port}</span>
    </div>
  </td>
  <td class="hidden md:table-cell">
    <div class="flex items-center gap-1" style="font-size: 11px;">
      <span style="color: var(--text);">{formatBytes(quota.used_bytes)}</span>
      <span style="color: var(--text-dim);">/</span>
      <span style="color: var(--text-muted);">{formatBytes(quota.quota_bytes)}</span>
    </div>
  </td>
  <td>
    <div class="flex items-center gap-2">
      <div style="flex: 1; height: 2px; border-radius: 1px; overflow: hidden; background-color: var(--border); min-width: 60px;">
        <div
          style="height: 100%; border-radius: 1px; transition: width 0.3s; width: {ringPercent}%; background-color: {progressColor};"
        ></div>
      </div>
      <span style="font-size: 10px; color: var(--text-muted); min-width: 36px;">{formatPercent(quota.usage_percent)}</span>
    </div>
  </td>
  <td class="hidden md:table-cell">
    <div class="flex items-center gap-2">
      <span style="width: 5px; height: 5px; border-radius: 50%; flex-shrink: 0; background-color: {statusColor};"></span>
      <span style="font-size: 11px; color: var(--text-muted);">{quota.status}</span>
    </div>
  </td>
  <td class="w-12 text-center">
    <span style="font-size: 12px; color: var(--text-dim);">{expanded ? '−' : '+'}</span>
  </td>
</tr>

{#if expanded}
  <tr class="detail-row">
    <td colspan="6">
      <div style="padding: 10px 12px 12px; animation: slideDown 0.15s ease;">
        {#if quota.comment}
          <div class="flex gap-2 mb-2" style="font-size: 11px;">
            <span style="color: var(--text-muted);">comment:</span>
            <span style="color: var(--text);">{quota.comment}</span>
          </div>
        {/if}
        <div class="flex gap-2 mb-2" style="font-size: 11px;">
          <span style="color: var(--text-muted);">id:</span>
          <span style="font-size: 10px; padding: 1px 5px; border-radius: 2px; background-color: var(--bg); color: var(--text); border: 0.5px solid var(--border);">{quota.id}</span>
        </div>
        {#if quota.token}
          <div class="flex gap-2 mb-2 items-center" style="font-size: 11px;">
            <span style="color: var(--text-muted);">token:</span>
            <span style="display: inline-flex; align-items: center; gap: 6px; font-size: 10px; padding: 1px 5px; border-radius: 2px; background-color: var(--bg); color: var(--text); border: 0.5px solid var(--border);">
              {quota.token}
              <button
                style="font-size: 10px; padding: 1px 5px; border-radius: 2px; background: var(--surface-hover); border: 0.5px solid var(--border); color: var(--text-muted); cursor: pointer; text-transform: uppercase; letter-spacing: 0.04em;"
                onclick={copyToken}
              >{copiedToken ? 'copied' : 'copy'}</button>
            </span>
          </div>
          <div class="flex gap-2 mb-2 items-center" style="font-size: 11px;">
            <span style="color: var(--text-muted);">url:</span>
            <span style="display: inline-flex; align-items: center; gap: 6px; font-size: 10px; padding: 1px 5px; border-radius: 2px; background-color: var(--bg); color: var(--text); border: 0.5px solid var(--border);">
              <a href={queryUrl} target="_blank" style="color: var(--teal); text-decoration: none;">/query?token={quota.token}</a>
              <button
                style="font-size: 10px; padding: 1px 5px; border-radius: 2px; background: var(--surface-hover); border: 0.5px solid var(--border); color: var(--text-muted); cursor: pointer; text-transform: uppercase; letter-spacing: 0.04em;"
                onclick={copyQueryUrl}
              >{copiedUrl ? 'copied' : 'copy'}</button>
            </span>
          </div>
        {/if}

        {#if !$readOnly}
          <div class="flex gap-2 mt-3">
            <button
              class="btn btn-sm btn-secondary"
              onclick={() => (showResetConfirm = true)}
              disabled={processing}
            >reset</button>
            <button
              class="btn btn-sm btn-secondary"
              onclick={() => (showEditModal = true)}
              disabled={processing}
            >edit</button>
            <button
              class="btn btn-sm btn-danger"
              onclick={() => (showDeleteConfirm = true)}
              disabled={processing}
            >delete</button>
          </div>
        {/if}
      </div>
    </td>
  </tr>
{/if}

{#if showResetConfirm}
  <ConfirmDialog
    title="Reset Quota"
    message={`Reset quota for port ${quota.port}? Sets used traffic to 0.`}
    confirmText="Reset"
    onconfirm={handleReset}
    oncancel={() => (showResetConfirm = false)}
  />
{/if}

{#if showDeleteConfirm}
  <ConfirmDialog
    title="Delete Quota"
    message={`Delete quota rule for port ${quota.port}? This cannot be undone.`}
    confirmText="Delete"
    danger={true}
    onconfirm={handleDelete}
    oncancel={() => (showDeleteConfirm = false)}
  />
{/if}

{#if showEditModal}
  <EditQuotaModal {quota} onclose={() => (showEditModal = false)} />
{/if}

<style>
  .selected td {
    background-color: var(--amber-dim);
  }

  @keyframes slideDown {
    from { opacity: 0; transform: translateY(-6px); }
    to   { opacity: 1; transform: translateY(0); }
  }
</style>
