<script>
  import { allowedPorts, readOnly, removeAllowedPort } from './stores.js';
  import AddPortModal from './AddPortModal.svelte';
  import ConfirmDialog from './ConfirmDialog.svelte';

  let showAddModal = $state(false);
  let portToDelete = $state(null);

  let sortedPorts = $derived(
    [...$allowedPorts].sort((a, b) => a.port - b.port)
  );

  function handleAddClick() {
    showAddModal = true;
  }

  function handleDeleteClick(port) {
    portToDelete = port;
  }

  async function confirmDelete() {
    if (portToDelete) {
      try {
        await removeAllowedPort(portToDelete.handle);
      } catch (e) {
        // Error already shown by store
      }
    }
    portToDelete = null;
  }

  function cancelDelete() {
    portToDelete = null;
  }
</script>

<section class="mb-4">
  <div class="flex justify-between items-center mb-2">
    <div class="flex items-center gap-2">
      <span class="dot-amber"></span>
      <span style="font-size: 9px; font-weight: 700; letter-spacing: 0.12em; text-transform: uppercase; color: var(--text-muted);">Allowed Inbound Ports</span>
    </div>
    {#if !$readOnly}
      <button class="btn btn-sm btn-primary" onclick={handleAddClick}>+ add port</button>
    {/if}
  </div>

  {#if $allowedPorts.length === 0}
    <p style="font-size: 11px; color: var(--text-muted); padding: 12px 0;">no allowed port rules found</p>
  {:else}
    <div class="flex flex-wrap gap-1.5">
      {#each sortedPorts as port}
        <div
          class="flex items-center gap-1.5"
          style="padding: 3px 7px; border: 0.5px solid var(--border); border-radius: 2px; background: transparent; font-size: 11px;"
        >
          <span style="color: {port.managed ? 'var(--teal)' : 'var(--text)'};">{port.port}</span>
          <span style="font-size: 9px; color: var(--text-dim); letter-spacing: 0.05em;">{port.protocol || 'tcp'}</span>
          {#if port.comment && port.comment !== 'nft-ui managed'}
            <span style="font-size: 10px; color: var(--text-muted);">{port.comment}</span>
          {/if}
          {#if port.managed && !$readOnly}
            <button
              style="background: transparent; border: none; color: var(--text-dim); font-size: 14px; line-height: 1; padding: 0 1px; cursor: pointer;"
              onmouseover={(e) => e.currentTarget.style.color = 'var(--danger)'}
              onmouseout={(e) => e.currentTarget.style.color = 'var(--text-dim)'}
              onclick={() => handleDeleteClick(port)}
              title="Delete port"
            >×</button>
          {/if}
        </div>
      {/each}
    </div>
  {/if}
</section>

{#if showAddModal}
  <AddPortModal onclose={() => showAddModal = false} />
{/if}

{#if portToDelete}
  <ConfirmDialog
    title="Delete Port"
    message={`Delete port ${portToDelete.port}?`}
    confirmText="Delete"
    danger={true}
    onconfirm={confirmDelete}
    oncancel={cancelDelete}
  />
{/if}
