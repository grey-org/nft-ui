<script>
  import { fetchRawRuleset } from './api.js';

  let rawData = $state('');
  let loading = $state(false);
  let error = $state(null);
  let expanded = $state(false);
  let copied = $state(false);

  async function loadRawData() {
    loading = true;
    error = null;
    try {
      const response = await fetchRawRuleset();
      rawData = response.data;
    } catch (err) {
      error = err.message;
      rawData = '';
    } finally {
      loading = false;
    }
  }

  function toggleExpanded() {
    expanded = !expanded;
    if (expanded && !rawData && !loading) {
      loadRawData();
    }
  }

  async function copyToClipboard() {
    try {
      await navigator.clipboard.writeText(rawData);
      copied = true;
      setTimeout(() => { copied = false; }, 2000);
    } catch (err) {
      // fallback for older browsers
      const ta = document.createElement('textarea');
      ta.value = rawData;
      ta.style.position = 'fixed';
      ta.style.opacity = '0';
      document.body.appendChild(ta);
      ta.focus();
      ta.select();
      document.execCommand('copy');
      document.body.removeChild(ta);
      copied = true;
      setTimeout(() => { copied = false; }, 2000);
    }
  }
</script>

<section class="card mb-4 overflow-hidden">
  <div class="flex justify-between items-center px-3 py-2" style="border-bottom: {expanded ? '0.5px solid var(--border)' : 'none'};">
    <div class="flex items-center gap-2">
      <span style="font-size: 9px; font-weight: 700; letter-spacing: 0.12em; text-transform: uppercase; color: var(--text-muted);">Raw Ruleset</span>
    </div>
    <button class="btn btn-sm btn-secondary" onclick={toggleExpanded}>{expanded ? 'hide' : 'show'}</button>
  </div>

  {#if expanded}
    <div class="px-3 py-3">
      {#if loading}
        <div style="font-size: 11px; color: var(--text-muted); padding: 8px 0;">loading…</div>
      {:else if error}
        <div class="alert alert-error flex justify-between items-center">
          <span>error: {error}</span>
          <button class="btn btn-sm btn-danger" onclick={loadRawData}>retry</button>
        </div>
      {:else if rawData}
        <div class="flex justify-end mb-2">
          <button class="btn btn-sm btn-secondary" onclick={copyToClipboard}>
            {copied ? '✓ copied' : 'copy'}
          </button>
        </div>
        <pre class="code-block m-0 whitespace-pre-wrap break-words">{rawData}</pre>
      {:else}
        <div style="font-size: 11px; color: var(--text-muted); padding: 8px 0;">no data loaded</div>
      {/if}
    </div>
  {/if}
</section>
