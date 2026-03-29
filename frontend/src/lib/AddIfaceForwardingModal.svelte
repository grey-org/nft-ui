<script>
  import { onMount } from 'svelte';
  import { addIfaceForwardingRule, pauseRefresh, resumeRefresh } from './stores.js';
  import { isValidIPv4, isValidIPv6 } from './utils.js';

  let { onclose } = $props();

  onMount(() => {
    pauseRefresh();
    return () => resumeRefresh();
  });

  let iifName = $state('');
  let addrFamily = $state('ip');
  let dstAddr = $state('');
  let natTo = $state('');
  let protocol = $state('udp');
  let comment = $state('');
  let submitting = $state(false);
  let errors = $state({});

  function validateAddr(addr) {
    if (addrFamily === 'ip') return isValidIPv4(addr);
    return isValidIPv6(addr);
  }

  function validate() {
    const newErrors = {};
    if (!iifName) {
      newErrors.iifName = 'interface name is required';
    } else if (iifName.length > 15) {
      newErrors.iifName = 'interface name must be at most 15 characters';
    } else if (!/^[a-zA-Z0-9._:-]+$/.test(iifName)) {
      newErrors.iifName = 'interface name contains invalid characters';
    }
    if (!validateAddr(dstAddr)) {
      newErrors.dstAddr = addrFamily === 'ip' ? 'enter a valid IPv4 address' : 'enter a valid IPv6 address';
    }
    if (!validateAddr(natTo)) {
      newErrors.natTo = addrFamily === 'ip' ? 'enter a valid IPv4 address' : 'enter a valid IPv6 address';
    }
    errors = newErrors;
    return Object.keys(newErrors).length === 0;
  }

  async function handleSubmit() {
    if (!validate()) return;
    submitting = true;
    try {
      await addIfaceForwardingRule(iifName, addrFamily, dstAddr, natTo, protocol, comment);
      onclose?.();
    } catch (e) {
      // Error notification handled by store
    } finally {
      submitting = false;
    }
  }

  function handleKeydown(e) {
    if (e.key === 'Escape') onclose?.();
  }
</script>

<svelte:window onkeydown={handleKeydown} />

<div class="modal-backdrop" onclick={onclose} role="presentation">
  <div
    class="modal w-full max-w-[420px] max-h-[90vh] overflow-y-auto"
    onclick={(e) => e.stopPropagation()}
    role="dialog"
    aria-modal="true"
  >
    <div class="flex items-center justify-between mb-4" style="border-bottom: 0.5px solid var(--border); padding-bottom: 10px;">
      <div class="flex items-center gap-2">
        <span class="dot-teal"></span>
        <span style="font-size: 10px; font-weight: 700; letter-spacing: 0.12em; text-transform: uppercase; color: var(--text-muted);">Add Interface Forwarding Rule</span>
      </div>
      <button
        style="background: transparent; border: none; font-size: 16px; line-height: 1; color: var(--text-muted); cursor: pointer; padding: 0;"
        onmouseover={(e) => e.currentTarget.style.color = 'var(--text)'}
        onmouseout={(e) => e.currentTarget.style.color = 'var(--text-muted)'}
        onclick={onclose}
      >×</button>
    </div>

    <form onsubmit={(e) => { e.preventDefault(); handleSubmit(); }}>
      <div class="mb-4">
        <label for="iifName" class="label">Ingress Interface</label>
        <input
          type="text" id="iifName" class="input" class:input-error={errors.iifName}
          bind:value={iifName} placeholder="eth0" maxlength="15"
        />
        {#if errors.iifName}
          <span style="font-size: 10px; color: var(--danger); display: block; margin-top: 3px;">{errors.iifName}</span>
        {/if}
        <span style="font-size: 10px; color: var(--text-muted); display: block; margin-top: 3px;">the NIC traffic arrives on</span>
      </div>

      <div class="mb-4">
        <label for="addrFamily" class="label">Address Family</label>
        <select id="addrFamily" class="select w-full" bind:value={addrFamily}>
          <option value="ip">IPv4</option>
          <option value="ip6">IPv6</option>
        </select>
      </div>

      <div class="mb-4">
        <label for="dstAddr" class="label">Destination Address (match)</label>
        <input
          type="text" id="dstAddr" class="input" class:input-error={errors.dstAddr}
          bind:value={dstAddr} placeholder={addrFamily === 'ip' ? '1.2.3.4' : '2001:db8::1'}
        />
        {#if errors.dstAddr}
          <span style="font-size: 10px; color: var(--danger); display: block; margin-top: 3px;">{errors.dstAddr}</span>
        {/if}
        <span style="font-size: 10px; color: var(--text-muted); display: block; margin-top: 3px;">IXP or BGP address to intercept</span>
      </div>

      <div class="mb-4">
        <label for="natTo" class="label">NAT To (DNAT target)</label>
        <input
          type="text" id="natTo" class="input" class:input-error={errors.natTo}
          bind:value={natTo} placeholder={addrFamily === 'ip' ? '5.6.7.8' : '2001:db8::2'}
        />
        {#if errors.natTo}
          <span style="font-size: 10px; color: var(--danger); display: block; margin-top: 3px;">{errors.natTo}</span>
        {/if}
        <span style="font-size: 10px; color: var(--text-muted); display: block; margin-top: 3px;">local service address to forward to</span>
      </div>

      <div class="mb-4">
        <label for="protocol" class="label">Protocol</label>
        <select id="protocol" class="select w-full" bind:value={protocol}>
          <option value="udp">UDP only</option>
          <option value="tcp">TCP only</option>
          <option value="all">All protocols</option>
        </select>
      </div>

      <div class="mb-5">
        <label for="comment" class="label">Comment (optional)</label>
        <input type="text" id="comment" class="input" bind:value={comment} placeholder="IXP route reflector" maxlength="100" />
      </div>

      <div class="flex justify-end gap-2">
        <button type="button" class="btn btn-sm btn-secondary" onclick={onclose}>cancel</button>
        <button type="submit" class="btn btn-sm btn-primary" disabled={submitting}>
          {submitting ? 'adding…' : 'add rule'}
        </button>
      </div>
    </form>
  </div>
</div>
