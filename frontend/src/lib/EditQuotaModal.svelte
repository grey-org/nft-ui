<script>
  import { onMount } from 'svelte';
  import { modifyQuota } from './api.js';
  import { loadQuotas, success, errorNotify, pauseRefresh, resumeRefresh } from './stores.js';
  import { formatBytes, parseBytes } from './utils.js';

  let { quota, onclose } = $props();

  onMount(() => {
    pauseRefresh();
    return () => resumeRefresh();
  });

  let quotaValue = $state('');
  let quotaUnit = $state('GB');
  let submitting = $state(false);
  let error = $state('');

  $effect(() => {
    const bytes = quota.quota_bytes;
    if (bytes >= 1000 * 1000 * 1000 * 1000) {
      quotaValue = (bytes / (1000 * 1000 * 1000 * 1000)).toString();
      quotaUnit = 'TB';
    } else if (bytes >= 1000 * 1000 * 1000) {
      quotaValue = (bytes / (1000 * 1000 * 1000)).toString();
      quotaUnit = 'GB';
    } else {
      quotaValue = (bytes / (1000 * 1000)).toString();
      quotaUnit = 'MB';
    }
  });

  function validate() {
    const value = parseFloat(quotaValue);
    if (isNaN(value) || value <= 0) {
      error = 'quota must be a positive number';
      return false;
    }
    error = '';
    return true;
  }

  async function handleSubmit() {
    if (!validate()) return;
    submitting = true;
    try {
      const bytes = parseBytes(parseFloat(quotaValue), quotaUnit);
      await modifyQuota(quota.id, bytes);
      success('Quota modified successfully');
      await loadQuotas();
      onclose?.();
    } catch (e) {
      errorNotify(`Failed to modify quota: ${e.message}`);
    } finally {
      submitting = false;
    }
  }

  function handleCancel() { onclose?.(); }

  function handleKeydown(e) {
    if (e.key === 'Escape') handleCancel();
  }
</script>

<svelte:window onkeydown={handleKeydown} />

<div class="modal-backdrop" onclick={handleCancel} role="presentation">
  <div class="modal" onclick={(e) => e.stopPropagation()} role="dialog" aria-modal="true">
    <div class="flex items-center gap-2 mb-4" style="border-bottom: 0.5px solid var(--border); padding-bottom: 10px;">
      <span class="dot-amber"></span>
      <span style="font-size: 10px; font-weight: 700; letter-spacing: 0.12em; text-transform: uppercase; color: var(--text-muted);">Edit Quota Rule</span>
    </div>

    <form onsubmit={(e) => { e.preventDefault(); handleSubmit(); }}>
      <div class="mb-4">
        <label for="port-display" class="label">Port</label>
        <input id="port-display" type="text" class="input" value={quota.port} disabled />
      </div>

      <div class="mb-4">
        <label for="usage-display" class="label">Current Usage</label>
        <input id="usage-display" type="text" class="input" value={formatBytes(quota.used_bytes)} disabled />
      </div>

      <div class="mb-5">
        <label for="quota" class="label">New Quota Limit</label>
        <div class="flex gap-2">
          <input
            id="quota"
            type="number"
            class="input flex-1 min-w-0"
            class:input-error={error}
            bind:value={quotaValue}
            placeholder="100"
            min="1"
            step="any"
          />
          <select class="select shrink-0 w-20" bind:value={quotaUnit}>
            <option value="MB">MB</option>
            <option value="GB">GB</option>
            <option value="TB">TB</option>
          </select>
        </div>
        {#if error}
          <span style="font-size: 10px; color: var(--danger); display: block; margin-top: 3px;">{error}</span>
        {/if}
        <span style="font-size: 10px; color: var(--warning); display: block; margin-top: 5px;">
          note: modifying quota resets used traffic to 0.
        </span>
      </div>

      <div class="flex justify-end gap-2">
        <button type="button" class="btn btn-sm btn-secondary" onclick={handleCancel}>cancel</button>
        <button type="submit" class="btn btn-sm btn-primary" disabled={submitting}>
          {submitting ? 'saving…' : 'save'}
        </button>
      </div>
    </form>
  </div>
</div>
