<script>
  import { onMount } from 'svelte';
  import { addForwardingRule, pauseRefresh, resumeRefresh } from './stores.js';
  import { isValidIPv4 } from './utils.js';

  let { onclose } = $props();

  onMount(() => {
    pauseRefresh();
    return () => resumeRefresh();
  });

  let srcPort = $state('');
  let dstIP = $state('');
  let dstPort = $state('');
  let protocol = $state('both');
  let comment = $state('');
  let limitMbps = $state('0');
  let mssMode = $state('pmtu');
  let submitting = $state(false);
  let errors = $state({});

  function validate() {
    const newErrors = {};

    const srcPortNum = parseInt(srcPort, 10);
    if (isNaN(srcPortNum) || srcPortNum < 1 || srcPortNum > 65535) {
      newErrors.srcPort = 'Source port must be between 1 and 65535';
    }

    if (!isValidIPv4(dstIP)) {
      newErrors.dstIP = 'Please enter a valid IPv4 address';
    }

    const dstPortNum = parseInt(dstPort, 10);
    if (isNaN(dstPortNum) || dstPortNum < 1 || dstPortNum > 65535) {
      newErrors.dstPort = 'Destination port must be between 1 and 65535';
    }

    const limitNum = parseInt(limitMbps, 10);
    if (isNaN(limitNum) || limitNum < 0) {
      newErrors.limitMbps = 'Limit must be 0 or positive (0 = no limit)';
    }

    errors = newErrors;
    return Object.keys(newErrors).length === 0;
  }

  async function handleSubmit() {
    if (!validate()) return;

    submitting = true;
    try {
      await addForwardingRule(
        parseInt(srcPort, 10),
        dstIP,
        parseInt(dstPort, 10),
        protocol,
        comment,
        parseInt(limitMbps, 10),
        mssMode
      );
      onclose?.();
    } catch (e) {
      // Error notification handled by store
    } finally {
      submitting = false;
    }
  }

  function handleKeydown(e) {
    if (e.key === 'Escape') {
      onclose?.();
    }
  }
</script>

<svelte:window onkeydown={handleKeydown} />

<div
  class="modal-backdrop"
  onclick={onclose}
  role="presentation"
>
  <div
    class="modal w-full max-w-[420px] max-h-[90vh] overflow-y-auto"
    onclick={(e) => e.stopPropagation()}
    role="dialog"
    aria-modal="true"
  >
    <div class="flex justify-between items-center mb-5">
      <h3 class="text-lg font-semibold" style="color: var(--text);">Add Forwarding Rule</h3>
      <button
        class="bg-transparent border-none text-2xl cursor-pointer p-0 leading-none transition-colors"
        style="color: var(--text-muted);"
        onmouseover={(e) => e.currentTarget.style.color = 'var(--text)'}
        onmouseout={(e) => e.currentTarget.style.color = 'var(--text-muted)'}
        onclick={onclose}
      >
        &times;
      </button>
    </div>

    <form onsubmit={(e) => { e.preventDefault(); handleSubmit(); }}>
      <div class="mb-4">
        <label for="srcPort" class="label">
          <span>Source Port</span>
        </label>
        <input
          type="number"
          id="srcPort"
          class="input"
          class:input-error={errors.srcPort}
          bind:value={srcPort}
          placeholder="e.g. 12103"
          min="1"
          max="65535"
        />
        {#if errors.srcPort}
          <span class="text-xs mt-1 block" style="color: var(--danger);">{errors.srcPort}</span>
        {/if}
        <span class="text-xs mt-1 block" style="color: var(--text-muted);">The local port to forward from</span>
      </div>

      <div class="mb-4">
        <label for="dstIP" class="label">
          <span>Destination IP</span>
        </label>
        <input
          type="text"
          id="dstIP"
          class="input"
          class:input-error={errors.dstIP}
          bind:value={dstIP}
          placeholder="e.g. 192.168.1.100"
        />
        {#if errors.dstIP}
          <span class="text-xs mt-1 block" style="color: var(--danger);">{errors.dstIP}</span>
        {/if}
        <span class="text-xs mt-1 block" style="color: var(--text-muted);">The target IP address</span>
      </div>

      <div class="mb-4">
        <label for="dstPort" class="label">
          <span>Destination Port</span>
        </label>
        <input
          type="number"
          id="dstPort"
          class="input"
          class:input-error={errors.dstPort}
          bind:value={dstPort}
          placeholder="e.g. 22"
          min="1"
          max="65535"
        />
        {#if errors.dstPort}
          <span class="text-xs mt-1 block" style="color: var(--danger);">{errors.dstPort}</span>
        {/if}
        <span class="text-xs mt-1 block" style="color: var(--text-muted);">The target port</span>
      </div>

      <div class="mb-4">
        <label for="protocol" class="label">
          <span>Protocol</span>
        </label>
        <select id="protocol" class="select" bind:value={protocol}>
          <option value="both">TCP + UDP</option>
          <option value="tcp">TCP only</option>
          <option value="udp">UDP only</option>
        </select>
      </div>

      <div class="mb-4">
        <label for="comment" class="label">
          <span>Comment (optional)</span>
        </label>
        <input
          type="text"
          id="comment"
          class="input"
          bind:value={comment}
          placeholder="e.g. SSH tunnel"
          maxlength="100"
        />
      </div>

      <div class="mb-4">
        <label for="mssMode" class="label">
          <span>TCP MSS Handling</span>
        </label>
        <select id="mssMode" class="select" bind:value={mssMode}>
          <option value="pmtu">Auto clamp to PMTU (recommended)</option>
          <option value="fixed1452">Fixed MSS 1452 (legacy compatibility)</option>
          <option value="disabled">Disabled</option>
        </select>
        <span class="text-xs mt-1 block" style="color: var(--text-muted);">Applies to TCP SYN/SYN-ACK only. PMTU mode maps to <code>tcp option maxseg size set rt mtu</code>.</span>
      </div>

      <div class="mb-6">
        <label for="limitMbps" class="label">
          <span>Bandwidth Limit (Mbps)</span>
        </label>
        <input
          type="number"
          id="limitMbps"
          class="input"
          class:input-error={errors.limitMbps}
          bind:value={limitMbps}
          placeholder="0"
          min="0"
        />
        {#if errors.limitMbps}
          <span class="text-xs mt-1 block" style="color: var(--danger);">{errors.limitMbps}</span>
        {/if}
        <span class="text-xs mt-1 block" style="color: var(--text-muted);">0 = no limit, or set max Mbps (e.g. 10, 100)</span>
      </div>

      <div class="flex justify-end gap-3">
        <button type="button" class="btn btn-secondary" onclick={onclose}>
          Cancel
        </button>
        <button type="submit" class="btn btn-primary" disabled={submitting}>
          {submitting ? 'Adding...' : 'Add Rule'}
        </button>
      </div>
    </form>
  </div>
</div>
