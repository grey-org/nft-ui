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
      <table class="data-table">
        <thead>
          <tr>
            <th class="w-20">Status</th>
            <th>Source Port</th>
            <th class="hidden md:table-cell">Destination</th>
            <th class="hidden md:table-cell">Protocol</th>
            <th class="w-12"></th>
          </tr>
        </thead>
        <tbody>
          {#each $sortedForwardingRules as rule (rule.id)}
            <ForwardingItem {rule} />
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</section>

{#if showAddModal}
  <AddForwardingModal onclose={() => showAddModal = false} />
{/if}
