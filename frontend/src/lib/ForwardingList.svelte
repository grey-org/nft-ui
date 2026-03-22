<script>
  import { sortedForwardingRules, forwardingLoading, readOnly } from './stores.js';
  import ForwardingItem from './ForwardingItem.svelte';
  import AddForwardingModal from './AddForwardingModal.svelte';

  let showAddModal = $state(false);
</script>

<section class="mb-4">
  <div class="flex justify-between items-center mb-2">
    <div class="flex items-center gap-2">
      <span class="dot-teal"></span>
      <span style="font-size: 9px; font-weight: 700; letter-spacing: 0.12em; text-transform: uppercase; color: var(--text-muted);">Port Forwarding</span>
    </div>
    {#if !$readOnly}
      <button class="btn btn-sm btn-primary" onclick={() => showAddModal = true}>+ add rule</button>
    {/if}
  </div>

  {#if $forwardingLoading}
    <div style="font-size: 11px; color: var(--text-muted); padding: 12px 0;">loading…</div>
  {:else if $sortedForwardingRules.length === 0}
    <div style="font-size: 11px; color: var(--text-muted); padding: 12px 0;">
      no forwarding rules configured
      {#if !$readOnly}
        <span style="color: var(--text-dim);"> — click "+ add rule" to create one</span>
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
