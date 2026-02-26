<script>
  import {
    readOnly,
    removeForwardingRule,
    enableForwardingRule,
    disableForwardingRule,
  } from './stores.js';
  import { formatProtocol } from './utils.js';
  import ConfirmDialog from './ConfirmDialog.svelte';
  import EditForwardingModal from './EditForwardingModal.svelte';

  let { rule } = $props();

  let expanded = $state(false);
  let showEditModal = $state(false);
  let showDeleteConfirm = $state(false);
  let processing = $state(false);

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

        {#if !$readOnly && rule.managed}
          <div class="flex gap-2 mt-4">
            <button
              class="btn btn-sm btn-secondary"
              onclick={handleToggleEnabled}
              disabled={processing}
            >
              {rule.enabled ? 'Disable' : 'Enable'}
            </button>
            <button
              class="btn btn-sm btn-secondary"
              onclick={() => showEditModal = true}
              disabled={processing}
            >
              Edit
            </button>
            <button
              class="btn btn-sm btn-danger"
              onclick={() => showDeleteConfirm = true}
              disabled={processing}
            >
              Delete
            </button>
          </div>
        {:else if !$readOnly && !rule.managed}
          {#if rule.enabled}
            <div class="text-sm p-3 rounded-lg mt-3" style="background-color: var(--surface-hover); color: var(--text-muted); border: 1px solid var(--border);">
              This rule was created externally and cannot be modified through nft-ui.
            </div>
          {:else}
            <div class="flex gap-2 mt-4">
              <button
                class="btn btn-sm btn-secondary"
                onclick={handleToggleEnabled}
                disabled={processing}
              >
                Enable
              </button>
              <button
                class="btn btn-sm btn-danger"
                onclick={() => showDeleteConfirm = true}
                disabled={processing}
              >
                Delete
              </button>
            </div>
          {/if}
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
