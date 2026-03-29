<script>
  import { ifaceForwardingRules, ifaceForwardingLoading, readOnly } from './stores.js';
  import IfaceForwardingItem from './IfaceForwardingItem.svelte';
  import AddIfaceForwardingModal from './AddIfaceForwardingModal.svelte';

  let showAddModal = $state(false);
</script>

<section class="mb-4">
  <div class="flex justify-between items-center mb-2">
    <div class="flex items-center gap-2">
      <span class="dot-teal"></span>
      <span style="font-size: 9px; font-weight: 700; letter-spacing: 0.12em; text-transform: uppercase; color: var(--text-muted);">Interface Forwarding</span>
    </div>
    {#if !$readOnly}
      <button class="btn btn-sm btn-primary" onclick={() => showAddModal = true}>+ add rule</button>
    {/if}
  </div>

  {#if $ifaceForwardingLoading}
    <div style="font-size: 11px; color: var(--text-muted); padding: 12px 0;">loading…</div>
  {:else if $ifaceForwardingRules.length === 0}
    <div style="font-size: 11px; color: var(--text-muted); padding: 12px 0;">
      no interface forwarding rules configured
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
            <th>Interface</th>
            <th class="hidden md:table-cell">Dst Address</th>
            <th class="hidden md:table-cell">NAT To</th>
            <th class="hidden md:table-cell">Protocol</th>
            <th class="w-12"></th>
          </tr>
        </thead>
        <tbody>
          {#each $ifaceForwardingRules as rule (rule.id)}
            <IfaceForwardingItem {rule} />
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</section>

{#if showAddModal}
  <AddIfaceForwardingModal onclose={() => showAddModal = false} />
{/if}
