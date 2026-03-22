<script>
  import { onMount } from 'svelte';
  import { addQuota } from './api.js';
  import { loadQuotas, success, errorNotify, pauseRefresh, resumeRefresh } from './stores.js';
  import { parseBytes } from './utils.js';

  let { onclose } = $props();

  onMount(() => {
    pauseRefresh();
    return () => resumeRefresh();
  });

  let port = $state('');
  let quotaValue = $state('');
  let quotaUnit = $state('GB');
  let comment = $state('');
  let submitting = $state(false);
  let errors = $state({});

  function validate() {
    errors = {};
    const portNum = parseInt(port, 10);
    if (isNaN(portNum) || portNum < 1 || portNum > 65535) {
      errors.port = 'port must be between 1 and 65535';
    }
    const quota = parseFloat(quotaValue);
    if (isNaN(quota) || quota <= 0) {
      errors.quota = 'quota must be a positive number';
    }
    return Object.keys(errors).length === 0;
  }

  async function handleSubmit() {
    if (!validate()) return;
    submitting = true;
    try {
      const bytes = parseBytes(parseFloat(quotaValue), quotaUnit);
      await addQuota(parseInt(port, 10), bytes, comment);
      success('Quota rule added successfully');
      await loadQuotas();
      onclose?.();
    } catch (e) {
      errorNotify(`Failed to add quota: ${e.message}`);
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
      <span style="font-size: 10px; font-weight: 700; letter-spacing: 0.12em; text-transform: uppercase; color: var(--text-muted);">Add Quota Rule</span>
    </div>

    <form onsubmit={(e) => { e.preventDefault(); handleSubmit(); }}>
      <div class="mb-4">
        <label for="port" class="label">Port</label>
        <input
          id="port"
          type="number"
          class="input"
          class:input-error={errors.port}
          bind:value={port}
          placeholder="8080"
          min="1"
          max="65535"
        />
        {#if errors.port}
          <span style="font-size: 10px; color: var(--danger); display: block; margin-top: 3px;">{errors.port}</span>
        {/if}
      </div>

      <div class="mb-4">
        <label for="quota" class="label">Quota Limit</label>
        <div class="flex gap-2">
          <input
            id="quota"
            type="number"
            class="input flex-1 min-w-0"
            class:input-error={errors.quota}
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
        {#if errors.quota}
          <span style="font-size: 10px; color: var(--danger); display: block; margin-top: 3px;">{errors.quota}</span>
        {/if}
      </div>

      <div class="mb-5">
        <label for="comment" class="label">Comment (optional)</label>
        <input
          id="comment"
          type="text"
          class="input"
          bind:value={comment}
          placeholder="block port after limit"
        />
      </div>

      <div class="flex justify-end gap-2">
        <button type="button" class="btn btn-sm btn-secondary" onclick={handleCancel}>cancel</button>
        <button type="submit" class="btn btn-sm btn-primary" disabled={submitting}>
          {submitting ? 'adding…' : 'add rule'}
        </button>
      </div>
    </form>
  </div>
</div>
