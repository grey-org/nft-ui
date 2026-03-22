<script>
  import { onMount } from 'svelte';
  import { editForwardingRule, pauseRefresh, resumeRefresh } from './stores.js';
  import { isValidIPv4 } from './utils.js';

  let { rule, onclose } = $props();

  onMount(() => {
    pauseRefresh();
    return () => resumeRefresh();
  });

  let dstIP = $state(rule.dst_ip);
  let dstPort = $state(rule.dst_port.toString());
  let protocol = $state(rule.protocol);
  let comment = $state(rule.comment || '');
  let limitMbps = $state((rule.limit_mbps || 0).toString());
  let mssMode = $state(rule.mss_mode || 'fixed1452');
  let sourceNATMode = $state(rule.source_nat_mode || 'masquerade');
  let snatAddress = $state(rule.snat_address || '');
  let submitting = $state(false);
  let errors = $state({});

  function validate() {
    const newErrors = {};
    if (!isValidIPv4(dstIP)) {
      newErrors.dstIP = 'enter a valid IPv4 address';
    }
    const dstPortNum = parseInt(dstPort, 10);
    if (isNaN(dstPortNum) || dstPortNum < 1 || dstPortNum > 65535) {
      newErrors.dstPort = 'destination port must be between 1 and 65535';
    }
    const limitNum = parseInt(limitMbps, 10);
    if (isNaN(limitNum) || limitNum < 0) {
      newErrors.limitMbps = 'limit must be 0 or positive';
    }
    if (sourceNATMode === 'snat' && !isValidIPv4(snatAddress)) {
      newErrors.snatAddress = 'enter a valid IPv4 SNAT address';
    }
    errors = newErrors;
    return Object.keys(newErrors).length === 0;
  }

  async function handleSubmit() {
    if (!validate()) return;
    submitting = true;
    try {
      await editForwardingRule(
        rule.id,
        dstIP,
        parseInt(dstPort, 10),
        protocol,
        comment,
        parseInt(limitMbps, 10),
        mssMode,
        sourceNATMode,
        snatAddress
      );
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
        <span style="font-size: 10px; font-weight: 700; letter-spacing: 0.12em; text-transform: uppercase; color: var(--text-muted);">Edit Forwarding Rule</span>
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
        <label for="srcPortDisplay" class="label">Source Port</label>
        <input type="text" id="srcPortDisplay" class="input" value={rule.src_port} disabled />
        <span style="font-size: 10px; color: var(--text-muted); display: block; margin-top: 3px;">source port cannot be changed</span>
      </div>

      <div class="mb-4">
        <label for="dstIP" class="label">Destination IP</label>
        <input
          type="text" id="dstIP" class="input" class:input-error={errors.dstIP}
          bind:value={dstIP} placeholder="192.168.1.100"
        />
        {#if errors.dstIP}
          <span style="font-size: 10px; color: var(--danger); display: block; margin-top: 3px;">{errors.dstIP}</span>
        {/if}
      </div>

      <div class="mb-4">
        <label for="dstPort" class="label">Destination Port</label>
        <input
          type="number" id="dstPort" class="input" class:input-error={errors.dstPort}
          bind:value={dstPort} placeholder="22" min="1" max="65535"
        />
        {#if errors.dstPort}
          <span style="font-size: 10px; color: var(--danger); display: block; margin-top: 3px;">{errors.dstPort}</span>
        {/if}
      </div>

      <div class="mb-4">
        <label for="protocol" class="label">Protocol</label>
        <select id="protocol" class="select w-full" bind:value={protocol}>
          <option value="both">TCP + UDP</option>
          <option value="tcp">TCP only</option>
          <option value="udp">UDP only</option>
        </select>
      </div>

      <div class="mb-4">
        <label for="comment" class="label">Comment (optional)</label>
        <input type="text" id="comment" class="input" bind:value={comment} placeholder="SSH tunnel" maxlength="100" />
      </div>

      <div class="mb-4">
        <label for="sourceNATMode" class="label">Source NAT</label>
        <select id="sourceNATMode" class="select w-full" bind:value={sourceNATMode}>
          <option value="masquerade">MASQUERADE (dynamic egress IP)</option>
          <option value="snat">Fixed SNAT (static egress IP)</option>
        </select>
      </div>

      {#if sourceNATMode === 'snat'}
        <div class="mb-4">
          <label for="snatAddress" class="label">SNAT Address</label>
          <input
            type="text" id="snatAddress" class="input" class:input-error={errors.snatAddress}
            bind:value={snatAddress} placeholder="203.0.113.10"
          />
          {#if errors.snatAddress}
            <span style="font-size: 10px; color: var(--danger); display: block; margin-top: 3px;">{errors.snatAddress}</span>
          {/if}
          <span style="font-size: 10px; color: var(--text-muted); display: block; margin-top: 3px;">required for <code>snat to …</code></span>
        </div>
      {/if}

      <div class="mb-4">
        <label for="mssMode" class="label">TCP MSS Handling</label>
        <select id="mssMode" class="select w-full" bind:value={mssMode}>
          <option value="pmtu">Auto clamp to PMTU (recommended)</option>
          <option value="fixed1452">Fixed MSS 1452 (legacy)</option>
          <option value="disabled">Disabled</option>
        </select>
        <span style="font-size: 10px; color: var(--text-muted); display: block; margin-top: 3px;">existing older rules default to fixed 1452 until changed</span>
      </div>

      <div class="mb-4">
        <label for="limitMbps" class="label">Bandwidth Limit (Mbps)</label>
        <input
          type="number" id="limitMbps" class="input" class:input-error={errors.limitMbps}
          bind:value={limitMbps} placeholder="0" min="0"
        />
        {#if errors.limitMbps}
          <span style="font-size: 10px; color: var(--danger); display: block; margin-top: 3px;">{errors.limitMbps}</span>
        {/if}
        <span style="font-size: 10px; color: var(--text-muted); display: block; margin-top: 3px;">0 = no limit</span>
      </div>

      <div style="font-size: 11px; padding: 8px 10px; border-radius: 2px; margin-bottom: 16px; background-color: var(--surface-hover); color: var(--text-muted); border: 0.5px solid var(--border);">
        note: editing will briefly disable and re-enable the rule.
      </div>

      <div class="flex justify-end gap-2">
        <button type="button" class="btn btn-sm btn-secondary" onclick={onclose}>cancel</button>
        <button type="submit" class="btn btn-sm btn-primary" disabled={submitting}>
          {submitting ? 'saving…' : 'save'}
        </button>
      </div>
    </form>
  </div>
</div>
