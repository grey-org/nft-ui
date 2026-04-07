<script>
  import {
    readOnly,
    removeIfaceForwardingRule,
    enableIfaceForwardingRule,
    disableIfaceForwardingRule,
  } from './stores.js';
  import ConfirmDialog from './ConfirmDialog.svelte';
  import EditIfaceForwardingModal from './EditIfaceForwardingModal.svelte';

  let { rule } = $props();

  let expanded = $state(false);
  let showEditModal = $state(false);
  let showDeleteConfirm = $state(false);
  let processing = $state(false);

  async function handleToggleEnabled() {
    processing = true;
    try {
      if (rule.enabled) {
        await disableIfaceForwardingRule(rule.id);
      } else {
        await enableIfaceForwardingRule(rule.id);
      }
    } finally {
      processing = false;
    }
  }

  async function handleDelete() {
    processing = true;
    try {
      await removeIfaceForwardingRule(rule.id);
    } finally {
      processing = false;
      showDeleteConfirm = false;
    }
  }

  function formatProto(p) {
    if (p === 'all') return 'ALL';
    return p?.toUpperCase() || 'ALL';
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
      <span style="font-size: 12px; color: var(--text);">{rule.iif_name}</span>
      <span style="font-size: 9px; color: var(--text-dim);">{rule.addr_family}{#if rule.nat_addr_family && rule.nat_addr_family !== rule.addr_family}→{rule.nat_addr_family}{/if}</span>
    </div>
  </td>
  <td class="hidden md:table-cell">
    <span style="font-size: 11px; color: var(--teal);">{rule.dst_addr}</span>
  </td>
  <td class="hidden md:table-cell">
    <span style="font-size: 11px; color: var(--text);">{rule.nat_to}</span>
  </td>
  <td class="hidden md:table-cell">
    <span class="badge" style="font-size: 9px;">{formatProto(rule.protocol)}</span>
  </td>
  <td class="w-12 text-center">
    <span style="font-size: 12px; color: var(--text-dim);">{expanded ? '−' : '+'}</span>
  </td>
</tr>

{#if expanded}
  <tr class="detail-row">
    <td colspan="6">
      <div style="padding: 10px 12px 12px; animation: slideDown 0.15s ease;">
        <!-- Mobile: dst + nat_to + protocol -->
        <div class="md:hidden mb-2">
          <div class="flex gap-2" style="font-size: 11px;">
            <span style="color: var(--text-muted);">dst addr:</span>
            <span style="color: var(--teal);">{rule.dst_addr}</span>
          </div>
          <div class="flex gap-2 mt-1" style="font-size: 11px;">
            <span style="color: var(--text-muted);">nat to:</span>
            <span style="color: var(--text);">{rule.nat_to}</span>
          </div>
          <div class="flex gap-2 mt-1" style="font-size: 11px;">
            <span style="color: var(--text-muted);">protocol:</span>
            <span style="color: var(--text);">{formatProto(rule.protocol)}</span>
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
          <span style="color: var(--text-muted);">addr family:</span>
          <span style="color: var(--text);">{rule.addr_family}{#if rule.nat_addr_family && rule.nat_addr_family !== rule.addr_family} → {rule.nat_addr_family}{/if}</span>
        </div>
        <div class="flex gap-2 mb-1" style="font-size: 11px;">
          <span style="color: var(--text-muted);">status:</span>
          <span style="color: var(--text);">{rule.enabled ? 'enabled' : 'disabled'}</span>
        </div>

        <div class="flex flex-wrap gap-2 mt-3">
          {#if !$readOnly && rule.managed}
            <button class="btn btn-sm btn-secondary" onclick={handleToggleEnabled} disabled={processing}>
              {rule.enabled ? 'disable' : 'enable'}
            </button>
            <button class="btn btn-sm btn-secondary" onclick={() => showEditModal = true} disabled={processing}>edit</button>
            <button class="btn btn-sm btn-danger" onclick={() => showDeleteConfirm = true} disabled={processing}>delete</button>
          {/if}
        </div>
      </div>
    </td>
  </tr>
{/if}

{#if showDeleteConfirm}
  <ConfirmDialog
    title="Delete Interface Forwarding Rule"
    message={`Delete iface forwarding rule for ${rule.iif_name} → ${rule.dst_addr}?`}
    confirmText="Delete"
    danger={true}
    onconfirm={handleDelete}
    oncancel={() => showDeleteConfirm = false}
  />
{/if}

{#if showEditModal}
  <EditIfaceForwardingModal {rule} onclose={() => showEditModal = false} />
{/if}

<style>
  @keyframes slideDown {
    from { opacity: 0; transform: translateY(-6px); }
    to   { opacity: 1; transform: translateY(0); }
  }
</style>
