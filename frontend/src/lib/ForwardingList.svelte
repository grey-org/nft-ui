<script>
  import { sortedForwardingRules, forwardingLoading, readOnly } from './stores.js';
  import ForwardingItem from './ForwardingItem.svelte';
  import AddForwardingModal from './AddForwardingModal.svelte';

  let showAddModal = $state(false);
</script>

<section class="mt-8">
  <div class="flex justify-between items-center mb-4">
    <h2 class="text-lg font-semibold m-0" style="color: var(--text);">Port Forwarding</h2>
    {#if !$readOnly}
      <button class="btn btn-sm btn-primary" onclick={() => showAddModal = true}>
        + Add Rule
      </button>
    {/if}
  </div>

  {#if $forwardingLoading}
    <div class="text-center py-8" style="color: var(--text-muted);">Loading forwarding rules...</div>
  {:else if $sortedForwardingRules.length === 0}
    <div class="text-center py-8" style="color: var(--text-muted);">
      <p>No forwarding rules configured</p>
      {#if !$readOnly}
        <p class="text-sm mt-2">Click "Add Rule" to create a new port forwarding rule</p>
      {/if}
    </div>
  {:else}
    <div class="card overflow-hidden">
      <div class="table-header hidden md:grid grid-cols-[80px_120px_1fr_100px_80px] px-4">
        <div>Status</div>
        <div>Source Port</div>
        <div>Destination</div>
        <div>Protocol</div>
        <div></div>
      </div>
      {#each $sortedForwardingRules as rule (rule.id)}
        <ForwardingItem {rule} />
      {/each}
    </div>
  {/if}
</section>

{#if showAddModal}
  <AddForwardingModal onclose={() => showAddModal = false} />
{/if}
