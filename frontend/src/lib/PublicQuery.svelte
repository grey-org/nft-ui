<script>
  import { onMount } from 'svelte';
  import { formatBytes, formatPercent, getProgressColor, getStatusColor } from './utils.js';

  let token = $state('');
  let result = $state(null);
  let error = $state(null);
  let loading = $state(false);

  onMount(() => {
    const params = new URLSearchParams(window.location.search);
    const urlToken = params.get('token');
    if (urlToken) {
      token = urlToken;
      handleQuery();
    }
  });

  async function handleQuery() {
    if (!token || token.length !== 8) {
      error = 'Please enter a valid 8-character token';
      return;
    }

    loading = true;
    error = null;
    result = null;

    try {
      const response = await fetch(`/api/v1/public/query/${encodeURIComponent(token)}`);
      const data = await response.json();

      if (!response.ok) {
        throw new Error(data.error || 'Query failed');
      }

      result = data;
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  function handleSubmit(e) {
    e.preventDefault();
    handleQuery();
  }

  let ringPercent = $derived(result ? Math.min(result.usage_percent, 100) : 0);
  let progressColor = $derived(result ? getProgressColor(result.usage_percent) : 'var(--primary)');

  function getStatusLabel(status) {
    switch (status) {
      case 'exceeded': return 'Exceeded';
      case 'warning': return 'Warning';
      default: return 'Normal';
    }
  }

  function getStatusIcon(status) {
    switch (status) {
      case 'exceeded': return '✕';
      case 'warning': return '!';
      default: return '✓';
    }
  }

  let remainingBytes = $derived(result ? Math.max(0, result.quota_bytes - result.used_bytes) : 0);
</script>

<div class="query-page">
  <div class="query-container">
    <!-- Header -->
    <div class="query-header">
      <div class="header-icon">
        <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
          <path d="M12 2L2 7l10 5 10-5-10-5z"/>
          <path d="M2 17l10 5 10-5"/>
          <path d="M2 12l10 5 10-5"/>
        </svg>
      </div>
      <h1>Bandwidth Query</h1>
      <p class="subtitle">Check your port traffic usage</p>
    </div>

    <!-- Search -->
    <form onsubmit={handleSubmit} class="search-form">
      <div class="search-box">
        <svg class="search-icon" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="11" cy="11" r="8"/>
          <path d="M21 21l-4.35-4.35"/>
        </svg>
        <input
          type="text"
          bind:value={token}
          placeholder="Enter your token"
          maxlength="8"
          autocomplete="off"
          spellcheck="false"
        />
        <button type="submit" disabled={loading}>
          {#if loading}
            <span class="spinner"></span>
          {:else}
            Query
          {/if}
        </button>
      </div>
    </form>

    {#if error}
      <div class="error-msg">
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><path d="M12 8v4m0 4h.01"/></svg>
        {error}
      </div>
    {/if}

    {#if result}
      <div class="result-card" class:status-warning={result.status === 'warning'} class:status-exceeded={result.status === 'exceeded'}>
        <!-- Top row: port + status -->
        <div class="result-top">
          <div class="port-label">
            <span class="port-num">Port {result.port}</span>
            {#if result.comment}
              <span class="port-comment">{result.comment}</span>
            {/if}
          </div>
          <div class="status-pill status-{result.status}">
            <span class="status-dot">{getStatusIcon(result.status)}</span>
            {getStatusLabel(result.status)}
          </div>
        </div>

        <!-- Gauge -->
        <div class="gauge-section">
          <div class="gauge-ring">
            <svg viewBox="0 0 120 120">
              <circle cx="60" cy="60" r="52" fill="none" stroke="var(--border)" stroke-width="8"/>
              <circle
                cx="60" cy="60" r="52"
                fill="none"
                stroke={progressColor}
                stroke-width="8"
                stroke-linecap="round"
                stroke-dasharray="{326.7}"
                stroke-dashoffset="{326.7 - (326.7 * ringPercent / 100)}"
                transform="rotate(-90 60 60)"
                class="gauge-progress"
              />
            </svg>
            <div class="gauge-center">
              <span class="gauge-value">{formatPercent(result.usage_percent)}</span>
              <span class="gauge-label">used</span>
            </div>
          </div>
        </div>

        <!-- Stats grid -->
        <div class="stats-grid">
          <div class="stat-item">
            <span class="stat-value">{formatBytes(result.used_bytes)}</span>
            <span class="stat-label">Used</span>
          </div>
          <div class="stat-divider"></div>
          <div class="stat-item">
            <span class="stat-value">{formatBytes(result.quota_bytes)}</span>
            <span class="stat-label">Quota</span>
          </div>
          <div class="stat-divider"></div>
          <div class="stat-item">
            <span class="stat-value">{formatBytes(remainingBytes)}</span>
            <span class="stat-label">Remaining</span>
          </div>
        </div>

        <!-- Info note -->
        <div class="info-note">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><path d="M12 16v-4m0-4h.01"/></svg>
          Traffic is metered on outbound (upload) direction only
        </div>
      </div>
    {/if}
  </div>
</div>

<style>
  .query-page {
    min-height: 100vh;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 20px;
    background: var(--bg);
  }

  .query-container {
    max-width: 440px;
    width: 100%;
  }

  /* Header */
  .query-header {
    text-align: center;
    margin-bottom: 32px;
  }

  .header-icon {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 56px;
    height: 56px;
    border-radius: 16px;
    background: linear-gradient(135deg, var(--primary), var(--accent));
    color: white;
    margin-bottom: 16px;
  }

  .query-header h1 {
    font-size: 24px;
    font-weight: 700;
    color: var(--text);
    margin: 0 0 6px;
    letter-spacing: -0.5px;
  }

  .subtitle {
    color: var(--text-muted);
    font-size: 14px;
    margin: 0;
  }

  /* Search */
  .search-form {
    margin-bottom: 24px;
  }

  .search-box {
    display: flex;
    align-items: center;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 12px;
    padding: 4px 4px 4px 14px;
    transition: border-color 0.2s;
  }

  .search-box:focus-within {
    border-color: var(--primary);
  }

  .search-icon {
    color: var(--text-muted);
    flex-shrink: 0;
  }

  .search-box input {
    flex: 1;
    background: none;
    border: none;
    outline: none;
    color: var(--text);
    font-size: 15px;
    font-family: ui-monospace, 'SF Mono', 'Cascadia Code', monospace;
    letter-spacing: 2px;
    text-transform: uppercase;
    padding: 10px 12px;
  }

  .search-box input::placeholder {
    text-transform: none;
    letter-spacing: normal;
    font-family: inherit;
    color: var(--text-muted);
    opacity: 0.6;
  }

  .search-box button {
    background: var(--primary);
    color: white;
    border: none;
    border-radius: 8px;
    padding: 10px 20px;
    font-size: 14px;
    font-weight: 600;
    cursor: pointer;
    transition: background 0.2s;
    white-space: nowrap;
  }

  .search-box button:hover:not(:disabled) {
    background: var(--primary-hover);
  }

  .search-box button:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  .spinner {
    display: inline-block;
    width: 16px;
    height: 16px;
    border: 2px solid rgba(255,255,255,0.3);
    border-top-color: white;
    border-radius: 50%;
    animation: spin 0.6s linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  /* Error */
  .error-msg {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 12px 16px;
    background: rgba(239, 68, 68, 0.1);
    border: 1px solid rgba(239, 68, 68, 0.2);
    border-radius: 10px;
    color: var(--danger);
    font-size: 14px;
    margin-bottom: 24px;
  }

  /* Result Card */
  .result-card {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 16px;
    padding: 28px;
    animation: slideUp 0.3s ease;
  }

  @keyframes slideUp {
    from { opacity: 0; transform: translateY(12px); }
    to { opacity: 1; transform: translateY(0); }
  }

  .result-top {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    margin-bottom: 24px;
  }

  .port-label {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .port-num {
    font-size: 20px;
    font-weight: 700;
    color: var(--text);
    letter-spacing: -0.3px;
  }

  .port-comment {
    font-size: 13px;
    color: var(--text-muted);
  }

  /* Status pill */
  .status-pill {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 5px 12px;
    border-radius: 20px;
    font-size: 13px;
    font-weight: 600;
    white-space: nowrap;
  }

  .status-dot {
    font-size: 11px;
    font-weight: 700;
  }

  .status-pill.status-ok {
    background: rgba(16, 185, 129, 0.12);
    color: var(--success);
  }

  .status-pill.status-warning {
    background: rgba(245, 158, 11, 0.12);
    color: var(--warning);
  }

  .status-pill.status-exceeded {
    background: rgba(239, 68, 68, 0.12);
    color: var(--danger);
  }

  /* Gauge */
  .gauge-section {
    display: flex;
    justify-content: center;
    margin-bottom: 24px;
  }

  .gauge-ring {
    position: relative;
    width: 140px;
    height: 140px;
  }

  .gauge-ring svg {
    width: 100%;
    height: 100%;
  }

  .gauge-progress {
    transition: stroke-dashoffset 0.8s ease;
  }

  .gauge-center {
    position: absolute;
    inset: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
  }

  .gauge-value {
    font-size: 22px;
    font-weight: 700;
    color: var(--text);
    letter-spacing: -0.5px;
  }

  .gauge-label {
    font-size: 12px;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 1px;
  }

  /* Stats grid */
  .stats-grid {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 0;
    padding: 16px 0;
    border-top: 1px solid var(--border);
    border-bottom: 1px solid var(--border);
    margin-bottom: 16px;
  }

  .stat-item {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 4px;
  }

  .stat-value {
    font-size: 15px;
    font-weight: 600;
    color: var(--text);
  }

  .stat-label {
    font-size: 11px;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }

  .stat-divider {
    width: 1px;
    height: 32px;
    background: var(--border);
  }

  /* Info note */
  .info-note {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 12px;
    color: var(--text-muted);
    opacity: 0.7;
  }

  .info-note svg {
    flex-shrink: 0;
  }
</style>
