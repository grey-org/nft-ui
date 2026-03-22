<script>
  import { onMount } from 'svelte';
  import { addAllowedPort, pauseRefresh, resumeRefresh } from './stores.js';

  let { onclose } = $props();

  onMount(() => {
    pauseRefresh();
    return () => resumeRefresh();
  });

  let port = $state('');
  let submitting = $state(false);
  let error = $state('');

  function validate() {
    const portNum = parseInt(port, 10);
    if (isNaN(portNum) || portNum < 1 || portNum > 65535) {
      error = 'port must be between 1 and 65535';
      return false;
    }
    error = '';
    return true;
  }

  async function handleSubmit() {
    if (!validate()) return;
    submitting = true;
    try {
      await addAllowedPort(parseInt(port, 10));
      onclose?.();
    } catch (e) {
      // Error already shown by store
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
      <span style="font-size: 10px; font-weight: 700; letter-spacing: 0.12em; text-transform: uppercase; color: var(--text-muted);">Add Allowed Port</span>
    </div>

    <form onsubmit={(e) => { e.preventDefault(); handleSubmit(); }}>
      <div class="mb-5">
        <label for="port" class="label">Port Number</label>
        <input
          id="port"
          type="number"
          class="input"
          class:input-error={error}
          bind:value={port}
          placeholder="8080"
          min="1"
          max="65535"
          autofocus
        />
        {#if error}
          <span style="font-size: 10px; color: var(--danger); display: block; margin-top: 3px;">{error}</span>
        {/if}
        <span style="font-size: 10px; color: var(--text-muted); display: block; margin-top: 5px;">
          adds: <code>tcp dport &lt;port&gt; accept</code>
        </span>
      </div>

      <div class="flex justify-end gap-2">
        <button type="button" class="btn btn-sm btn-secondary" onclick={handleCancel}>cancel</button>
        <button type="submit" class="btn btn-sm btn-primary" disabled={submitting}>
          {submitting ? 'adding…' : 'add port'}
        </button>
      </div>
    </form>
  </div>
</div>
