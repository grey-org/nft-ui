<script>
  import {
    readOnly,
    removeForwardingRule,
    enableForwardingRule,
    disableForwardingRule,
    testForwardingTarget as probeForwardingTarget,
  } from './stores.js';
  import { formatProtocol } from './utils.js';
  import ConfirmDialog from './ConfirmDialog.svelte';
  import EditForwardingModal from './EditForwardingModal.svelte';

  let { rule } = $props();

  let expanded = $state(false);
  let showEditModal = $state(false);
  let showDeleteConfirm = $state(false);
  let processing = $state(false);
  let testing = $state(false);
  let lastTest = $state(null);

  async function handleToggleEnabled() {
    processing = true;
    try {
      if (rule.enabled) {
        await disableForwardingRule(rule.id);
      } else {
        await enableForwardingRule(rule.id);
      }
    } finally {
      processing = false;
    }
  }

  async function handleDelete() {
    processing = true;
    try {
      await removeForwardingRule(rule.id);
    } finally {
      processing = false;
      showDeleteConfirm = false;
    }
  }

  async function handleTestConnection() {
    testing = true;
    try {
      lastTest = await probeForwardingTarget(rule.dst_ip, rule.dst_port, rule.protocol);
    } finally {
      testing = false;
    }
  }

  function formatTestTime(value) {
    if (!value) return '';
    return new Date(value).toLocaleString();
  }

  function getProbeBadgeClass(status) {
    switch (status) {
      case 'reachable':
        return 'badge-success';
      case 'unreachable':
        return 'badge-danger';
      default:
        return 'badge-warning';
    }
  }

  function getProbeStatusLabel(status) {
    switch (status) {
      case 'reachable':
        return 'Reachable';
      case 'unreachable':
        return 'Unreachable';
      default:
        return 'Inconclusive';
    }
  }

  function getProbeAccent(status) {
    switch (status) {
      case 'reachable':
        return 'var(--success)';
      case 'unreachable':
        return 'var(--danger)';
      default:
        return 'var(--warning)';
    }
  }
</script>

<tr
  class="data-row"
  class:opacity-60={!rule.enabled}
  class:opacity-85={!rule.managed}
  onclick={() => expanded = !expanded}
>
  <td class="w-20">
    <div class="flex items-center justify-center">
      <span
        class="status-dot"
        class:status-dot-active={rule.enabled && rule.managed}
        class:status-dot-warning={!rule.managed}
        class:status-dot-inactive={!rule.enabled && rule.managed}
        title={!rule.managed ? 'Unmanaged (external)' : rule.enabled ? 'Enabled' : 'Disabled'}
      ></span>
    </div>
  </td>
  <td>
    <div class="flex items-center gap-1.5">
      <span class="font-semibold text-base font-mono" style="color: var(--text);">{rule.src_port}</span>
      {#if !rule.managed}
        <span class="text-[10px] px-1 py-0 rounded font-medium uppercase" style="background-color: var(--warning); color: #000;">ext</span>
      {/if}
    </div>
  </td>
  <td class="hidden md:table-cell">
    <div class="flex items-center gap-0.5 font-mono">
      <span style="color: var(--primary);">{rule.dst_ip}</span>
      <span style="color: var(--text-muted);">:</span>
      <span class="font-semibold" style="color: var(--text);">{rule.dst_port}</span>
    </div>
  </td>
  <td class="hidden md:table-cell">
    <span class="badge text-xs px-2 py-0.5">{formatProtocol(rule.protocol)}</span>
  </td>
  <td class="w-12 text-center">
    <span class="text-xl" style="color: var(--text-muted);">{expanded ? '−' : '+'}</span>
  </td>
</tr>

{#if expanded}
  <tr class="detail-row">
    <td colspan="5">
      <div class="px-4 pb-4 pt-2 animate-[slideDown_0.2s_ease]">
        <!-- Mobile: show destination + protocol -->
        <div class="md:hidden mb-2">
          <div class="flex gap-2 text-sm">
            <span style="color: var(--text-muted);">Destination:</span>
            <span class="font-mono" style="color: var(--text);">{rule.dst_ip}:{rule.dst_port}</span>
          </div>
          <div class="flex gap-2 text-sm mt-1">
            <span style="color: var(--text-muted);">Protocol:</span>
            <span style="color: var(--text);">{formatProtocol(rule.protocol)}</span>
          </div>
        </div>

        {#if rule.comment}
          <div class="flex gap-2 mb-2 text-sm">
            <span style="color: var(--text-muted);">Comment:</span>
            <span style="color: var(--text);">{rule.comment}</span>
          </div>
        {/if}
        <div class="flex gap-2 mb-2 text-sm">
          <span style="color: var(--text-muted);">ID:</span>
          <span class="font-mono text-xs px-1.5 py-0.5 rounded" style="background-color: var(--bg); color: var(--text); border: 1px solid var(--border);">{rule.id}</span>
        </div>
        <div class="flex gap-2 mb-2 text-sm">
          <span style="color: var(--text-muted);">Status:</span>
          <span style="color: var(--text);">{rule.enabled ? 'Enabled' : 'Disabled'}</span>
        </div>
        <div class="flex gap-2 mb-2 text-sm">
          <span style="color: var(--text-muted);">Managed:</span>
          <span style="color: var(--text);">{rule.managed ? 'Yes (nft-ui)' : 'No (external)'}</span>
        </div>
        {#if rule.limit_mbps > 0}
          <div class="flex gap-2 mb-2 text-sm">
            <span style="color: var(--text-muted);">Bandwidth Limit:</span>
            <span style="color: var(--text);">{rule.limit_mbps} Mbps</span>
          </div>
        {/if}
        <div class="flex gap-2 mb-2 text-sm">
          <span style="color: var(--text-muted);">Source NAT:</span>
          <span style="color: var(--text);">
            {#if rule.source_nat_mode === 'snat'}
              Fixed SNAT{#if rule.snat_address} → {rule.snat_address}{/if}
            {:else}
              MASQUERADE
            {/if}
          </span>
        </div>

        <div class="flex gap-2 mb-2 text-sm">
          <span style="color: var(--text-muted);">TCP MSS:</span>
          <span style="color: var(--text);">
            {#if rule.mss_mode === 'pmtu'}
              Auto PMTU clamp
            {:else if rule.mss_mode === 'disabled'}
              Disabled
            {:else}
              Fixed 1452
            {/if}
          </span>
        </div>

        <div class="flex flex-wrap gap-2 mt-4">
          <button
            class="btn btn-sm btn-secondary"
            onclick={handleTestConnection}
            disabled={testing || processing}
          >
            {testing ? 'Testing...' : `Test ${formatProtocol(rule.protocol)}`}
          </button>

          {#if !$readOnly && rule.managed}
            <button
              class="btn btn-sm btn-secondary"
              onclick={handleToggleEnabled}
              disabled={processing || testing}
            >
              {rule.enabled ? 'Disable' : 'Enable'}
            </button>
            <button
              class="btn btn-sm btn-secondary"
              onclick={() => showEditModal = true}
              disabled={processing || testing}
            >
              Edit
            </button>
            <button
              class="btn btn-sm btn-danger"
              onclick={() => showDeleteConfirm = true}
              disabled={processing || testing}
            >
              Delete
            </button>
          {:else if !$readOnly && !rule.managed && !rule.enabled}
            <button
              class="btn btn-sm btn-secondary"
              onclick={handleToggleEnabled}
              disabled={processing || testing}
            >
              Enable
            </button>
            <button
              class="btn btn-sm btn-danger"
              onclick={() => showDeleteConfirm = true}
              disabled={processing || testing}
            >
              Delete
            </button>
          {/if}
        </div>

        {#if !$readOnly && !rule.managed && rule.enabled}
          <div class="text-sm p-3 rounded-lg mt-3" style="background-color: var(--surface-hover); color: var(--text-muted); border: 1px solid var(--border);">
            This rule was created externally and cannot be modified through nft-ui.
          </div>
        {/if}

        {#if lastTest}
          <div
            class="mt-4 p-3 rounded-lg"
            style={`background-color: var(--surface-hover); border: 1px solid var(--border); border-left: 3px solid ${getProbeAccent(lastTest.overall_status)};`}
          >
            <div class="flex flex-wrap items-center justify-between gap-2 mb-3">
              <div class="text-sm font-medium" style="color: var(--text);">Last connectivity test</div>
              <div class="text-xs" style="color: var(--text-muted);">{formatTestTime(lastTest.tested_at)}</div>
            </div>

            <div class="flex flex-col gap-2">
              {#each lastTest.results as result}
                <div class="rounded-md p-2" style="background-color: var(--surface); border: 1px solid var(--border);">
                  <div class="flex flex-wrap items-center justify-between gap-2 text-sm">
                    <div class="flex items-center gap-2">
                      <span class={`badge ${getProbeBadgeClass(result.status)}`}>{result.protocol.toUpperCase()}</span>
                      <span style="color: var(--text);">{getProbeStatusLabel(result.status)}</span>
                    </div>
                    <span class="font-mono text-xs" style="color: var(--text-muted);">{result.duration_ms} ms</span>
                  </div>
                  <div class="text-xs mt-2" style="color: var(--text-muted);">{result.message}</div>
                </div>
              {/each}
            </div>
          </div>
        {/if}
      </div>
    </td>
  </tr>
{/if}

{#if showDeleteConfirm}
  <ConfirmDialog
    title="Delete Forwarding Rule"
    message={`Are you sure you want to delete the forwarding rule for port ${rule.src_port}?`}
    confirmText="Delete"
    danger={true}
    onconfirm={handleDelete}
    oncancel={() => showDeleteConfirm = false}
  />
{/if}

{#if showEditModal}
  <EditForwardingModal {rule} onclose={() => showEditModal = false} />
{/if}

<style>
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
