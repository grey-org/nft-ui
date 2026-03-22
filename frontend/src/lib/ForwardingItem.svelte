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
      case 'reachable':   return 'badge-success';
      case 'unreachable': return 'badge-danger';
      default:            return 'badge-warning';
    }
  }

  function getProbeStatusLabel(status) {
    switch (status) {
      case 'reachable':   return 'reachable';
      case 'unreachable': return 'unreachable';
      default:            return 'inconclusive';
    }
  }

  function getProbeAccent(status) {
    switch (status) {
      case 'reachable':   return 'var(--success)';
      case 'unreachable': return 'var(--danger)';
      default:            return 'var(--amber)';
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
    <div class="flex items-center gap-2">
      <span style="font-size: 12px; color: var(--text);">{rule.src_port}</span>
      {#if !rule.managed}
        <span style="font-size: 9px; padding: 1px 4px; border-radius: 2px; background-color: var(--amber); color: #000; text-transform: uppercase; letter-spacing: 0.06em;">ext</span>
      {/if}
    </div>
  </td>
  <td class="hidden md:table-cell">
    <div style="font-size: 11px;">
      <span style="color: var(--teal);">{rule.dst_ip}</span><span style="color: var(--text-dim);">:</span><span style="color: var(--text);">{rule.dst_port}</span>
    </div>
  </td>
  <td class="hidden md:table-cell">
    <span class="badge" style="font-size: 9px;">{formatProtocol(rule.protocol)}</span>
  </td>
  <td class="w-12 text-center">
    <span style="font-size: 12px; color: var(--text-dim);">{expanded ? '−' : '+'}</span>
  </td>
</tr>

{#if expanded}
  <tr class="detail-row">
    <td colspan="5">
      <div style="padding: 10px 12px 12px; animation: slideDown 0.15s ease;">
        <!-- Mobile: destination + protocol -->
        <div class="md:hidden mb-2">
          <div class="flex gap-2" style="font-size: 11px;">
            <span style="color: var(--text-muted);">destination:</span>
            <span style="color: var(--text);">{rule.dst_ip}:{rule.dst_port}</span>
          </div>
          <div class="flex gap-2 mt-1" style="font-size: 11px;">
            <span style="color: var(--text-muted);">protocol:</span>
            <span style="color: var(--text);">{formatProtocol(rule.protocol)}</span>
          </div>
        </div>

        {#if rule.comment}
          <div class="flex gap-2 mb-1" style="font-size: 11px;">
            <span style="color: var(--text-muted);">comment:</span>
            <span style="color: var(--text);">{rule.comment}</span>
          </div>
        {/if}
        <div class="flex gap-2 mb-1" style="font-size: 11px;">
          <span style="color: var(--text-muted);">id:</span>
          <span style="font-size: 10px; padding: 1px 5px; border-radius: 2px; background-color: var(--bg); color: var(--text); border: 0.5px solid var(--border);">{rule.id}</span>
        </div>
        <div class="flex gap-2 mb-1" style="font-size: 11px;">
          <span style="color: var(--text-muted);">status:</span>
          <span style="color: var(--text);">{rule.enabled ? 'enabled' : 'disabled'}</span>
        </div>
        <div class="flex gap-2 mb-1" style="font-size: 11px;">
          <span style="color: var(--text-muted);">managed:</span>
          <span style="color: var(--text);">{rule.managed ? 'yes (nft-ui)' : 'no (external)'}</span>
        </div>
        {#if rule.limit_mbps > 0}
          <div class="flex gap-2 mb-1" style="font-size: 11px;">
            <span style="color: var(--text-muted);">limit:</span>
            <span style="color: var(--text);">{rule.limit_mbps} Mbps</span>
          </div>
        {/if}
        <div class="flex gap-2 mb-1" style="font-size: 11px;">
          <span style="color: var(--text-muted);">snat:</span>
          <span style="color: var(--text);">
            {#if rule.source_nat_mode === 'snat'}
              snat{#if rule.snat_address} → {rule.snat_address}{/if}
            {:else}
              masquerade
            {/if}
          </span>
        </div>
        <div class="flex gap-2 mb-1" style="font-size: 11px;">
          <span style="color: var(--text-muted);">tcp mss:</span>
          <span style="color: var(--text);">
            {#if rule.mss_mode === 'pmtu'}auto pmtu
            {:else if rule.mss_mode === 'disabled'}disabled
            {:else}fixed 1452
            {/if}
          </span>
        </div>

        <div class="flex flex-wrap gap-2 mt-3">
          <button
            class="btn btn-sm btn-secondary"
            onclick={handleTestConnection}
            disabled={testing || processing}
          >{testing ? 'testing…' : `test ${formatProtocol(rule.protocol)}`}</button>

          {#if !$readOnly && rule.managed}
            <button class="btn btn-sm btn-secondary" onclick={handleToggleEnabled} disabled={processing || testing}>
              {rule.enabled ? 'disable' : 'enable'}
            </button>
            <button class="btn btn-sm btn-secondary" onclick={() => showEditModal = true} disabled={processing || testing}>edit</button>
            <button class="btn btn-sm btn-danger" onclick={() => showDeleteConfirm = true} disabled={processing || testing}>delete</button>
          {:else if !$readOnly && !rule.managed && !rule.enabled}
            <button class="btn btn-sm btn-secondary" onclick={handleToggleEnabled} disabled={processing || testing}>enable</button>
            <button class="btn btn-sm btn-danger" onclick={() => showDeleteConfirm = true} disabled={processing || testing}>delete</button>
          {/if}
        </div>

        {#if !$readOnly && !rule.managed && rule.enabled}
          <div style="font-size: 11px; padding: 8px 10px; border-radius: 2px; margin-top: 8px; background-color: var(--surface-hover); color: var(--text-muted); border: 0.5px solid var(--border);">
            this rule was created externally and cannot be modified through nft-ui.
          </div>
        {/if}

        {#if lastTest}
          <div
            style="margin-top: 12px; padding: 10px; border-radius: 2px; background-color: var(--surface-hover); border: 0.5px solid var(--border); border-left: 2px solid {getProbeAccent(lastTest.overall_status)};"
          >
            <div class="flex items-center justify-between mb-2">
              <span style="font-size: 10px; font-weight: 700; letter-spacing: 0.1em; text-transform: uppercase; color: var(--text-muted);">connectivity test</span>
              <span style="font-size: 10px; color: var(--text-dim);">{formatTestTime(lastTest.tested_at)}</span>
            </div>

            <div class="flex flex-col gap-1.5">
              {#each lastTest.results as result}
                <div style="padding: 6px 8px; border-radius: 2px; background-color: var(--surface); border: 0.5px solid var(--border);">
                  <div class="flex items-center justify-between gap-2" style="font-size: 11px;">
                    <div class="flex items-center gap-2">
                      <span class="badge {getProbeBadgeClass(result.status)}">{result.protocol.toUpperCase()}</span>
                      <span style="color: var(--text);">{getProbeStatusLabel(result.status)}</span>
                    </div>
                    <span style="font-size: 10px; color: var(--text-muted);">{result.duration_ms} ms</span>
                  </div>
                  <div style="font-size: 10px; margin-top: 4px; color: var(--text-muted);">{result.message}</div>
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
    message={`Delete forwarding rule for port ${rule.src_port}?`}
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
    from { opacity: 0; transform: translateY(-6px); }
    to   { opacity: 1; transform: translateY(0); }
  }
</style>
