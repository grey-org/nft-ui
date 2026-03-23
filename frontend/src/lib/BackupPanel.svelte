<script>
  import { exportBackup, importBackup, readOnly } from './stores.js';

  let importing = $state(false);
  let lastSummary = $state(null);
  let fileInput = $state();

  function handleExport() {
    exportBackup();
  }

  function handleImport() {
    fileInput?.click();
  }

  async function handleFileSelect(event) {
    const file = event.target.files?.[0];
    if (!file) return;
    lastSummary = null;
    importing = true;
    try {
      const result = await importBackup(file);
      lastSummary = result?.summary ?? null;
    } catch {
      // error toast already shown by store
    } finally {
      importing = false;
      event.target.value = '';
    }
  }

  let totalSkipped = $derived(
    lastSummary
      ? (lastSummary.quotas_skipped ?? 0) +
        (lastSummary.forwarding_skipped ?? 0) +
        (lastSummary.ports_skipped ?? 0)
      : 0
  );
</script>

<section class="mb-4">
  <div class="flex justify-between items-center mb-2">
    <div class="flex items-center gap-2">
      <span class="dot-teal"></span>
      <span style="font-size: 9px; font-weight: 700; letter-spacing: 0.12em; text-transform: uppercase; color: var(--text-muted);">Backup / Restore</span>
    </div>
  </div>

  <div class="card overflow-hidden">
    <!-- Export row -->
    <div class="flex items-center justify-between px-3 py-3" style="border-bottom: 0.5px solid var(--border);">
      <div>
        <div style="font-size: 11px; font-weight: 600; color: var(--text-muted); letter-spacing: 0.04em;">export</div>
        <div style="font-size: 10px; color: var(--text-dim); margin-top: 2px;">download a JSON snapshot of all quotas, forwarding rules, and ports</div>
      </div>
      <button class="btn btn-sm btn-secondary" onclick={handleExport}>export</button>
    </div>

    <!-- Import row -->
    {#if !$readOnly}
      <div class="flex items-center justify-between px-3 py-3" style="border-bottom: {lastSummary ? '0.5px solid var(--border)' : 'none'};">
        <div>
          <div style="font-size: 11px; font-weight: 600; color: var(--text-muted); letter-spacing: 0.04em;">import</div>
          <div style="font-size: 10px; color: var(--text-dim); margin-top: 2px;">restore from a previously exported JSON backup file</div>
        </div>
        <button class="btn btn-sm btn-secondary" onclick={handleImport} disabled={importing}>
          {importing ? '…' : 'import'}
        </button>
      </div>
    {/if}

    <!-- Summary row (shown after a successful import) -->
    {#if lastSummary}
      <div class="px-3 py-2">
        <span style="font-size: 10px; color: var(--teal);">
          {lastSummary.quotas_added} quotas, {lastSummary.forwarding_added} forwarding, {lastSummary.ports_added} ports
        </span>
        {#if totalSkipped > 0}
          <span style="font-size: 10px; color: var(--amber);"> · {totalSkipped} skipped</span>
        {/if}
      </div>
    {/if}
  </div>

  <input type="file" accept=".json" bind:this={fileInput} onchange={handleFileSelect} style="display: none;" />
</section>
