<script>
  import {
    bypassConfig,
    bypassApplied,
    bypassIPRule,
    bypassLoading,
    loadBypass,
    updateBypass,
    readOnly,
  } from './stores.js';
  import { onMount } from 'svelte';

  onMount(() => {
    loadBypass();
  });

  // Local edit state
  let markInput = $state('0x1');
  let priorityInput = $state('8990');
  let dirty = $state(false);

  // Sync local inputs when config loads (only if not dirty)
  $effect(() => {
    if (!dirty) {
      const cfg = $bypassConfig;
      markInput = cfg.mark ? `0x${cfg.mark.toString(16)}` : '0x1';
      priorityInput = String(cfg.priority || 8990);
    }
  });

  function parseHex(val) {
    const s = val.trim().toLowerCase();
    if (s.startsWith('0x')) return parseInt(s, 16);
    return parseInt(s, 10);
  }

  function isValidMark(val) {
    const n = parseHex(val);
    return !isNaN(n) && n > 0 && n <= 0xffffffff;
  }

  function isValidPriority(val) {
    const n = parseInt(val, 10);
    return !isNaN(n) && n > 0 && n < 32767;
  }

  async function handleToggle() {
    const mark = parseHex(markInput);
    const priority = parseInt(priorityInput, 10);
    if (isNaN(mark) || mark <= 0) return;
    if (isNaN(priority) || priority <= 0) return;
    dirty = false;
    await updateBypass(!$bypassConfig.enabled, mark, priority);
  }

  async function handleApply() {
    const mark = parseHex(markInput);
    const priority = parseInt(priorityInput, 10);
    if (!isValidMark(markInput) || !isValidPriority(priorityInput)) return;
    dirty = false;
    await updateBypass($bypassConfig.enabled, mark, priority);
  }

  let markError = $derived(!isValidMark(markInput) ? 'invalid mark value' : '');
  let priorityError = $derived(!isValidPriority(priorityInput) ? 'must be 1–32766' : '');
</script>

<section class="mb-4">
  <div class="flex justify-between items-center mb-2">
    <div class="flex items-center gap-2">
      <span class="dot-teal"></span>
      <span style="font-size: 9px; font-weight: 700; letter-spacing: 0.12em; text-transform: uppercase; color: var(--text-muted);">Forward Bypass</span>
      <!-- live status indicator -->
      <span style="font-size: 9px; letter-spacing: 0.06em; text-transform: uppercase; color: {$bypassConfig.enabled && $bypassApplied ? 'var(--teal)' : $bypassConfig.enabled && !$bypassApplied ? 'var(--amber)' : 'var(--text-dim)'};">
        {#if $bypassConfig.enabled && $bypassApplied}
          ● active
        {:else if $bypassConfig.enabled && !$bypassApplied}
          ◌ pending
        {:else}
          ○ inactive
        {/if}
      </span>
    </div>
    {#if !$readOnly}
      <button
        class="btn btn-sm {$bypassConfig.enabled ? 'btn-danger' : 'btn-primary'}"
        onclick={handleToggle}
        disabled={$bypassLoading || !!markError || !!priorityError}
      >
        {$bypassLoading ? '…' : $bypassConfig.enabled ? 'disable' : 'enable'}
      </button>
    {/if}
  </div>

  <div class="card overflow-hidden">
    <!-- Config row -->
    <div class="flex flex-wrap items-start gap-4 px-3 py-3" style="border-bottom: 0.5px solid var(--border);">
      <!-- mark -->
      <div style="min-width: 120px;">
        <div class="label" style="margin-bottom: 3px;">fwmark</div>
        <input
          type="text"
          class="input"
          class:input-error={!!markError}
          bind:value={markInput}
          oninput={() => dirty = true}
          placeholder="0x1"
          style="width: 100px; font-size: 12px;"
          disabled={$readOnly}
        />
        {#if markError}
          <span style="font-size: 9px; color: var(--danger); display: block; margin-top: 2px;">{markError}</span>
        {/if}
      </div>
      <!-- priority -->
      <div style="min-width: 120px;">
        <div class="label" style="margin-bottom: 3px;">ip rule priority</div>
        <input
          type="number"
          class="input"
          class:input-error={!!priorityError}
          bind:value={priorityInput}
          oninput={() => dirty = true}
          placeholder="8990"
          min="1"
          max="32766"
          style="width: 100px; font-size: 12px;"
          disabled={$readOnly}
        />
        {#if priorityError}
          <span style="font-size: 9px; color: var(--danger); display: block; margin-top: 2px;">{priorityError}</span>
        {/if}
      </div>
      <!-- apply button (only shown when dirty and enabled) -->
      {#if dirty && !$readOnly}
        <div style="display: flex; align-items: flex-end; padding-bottom: 1px;">
          <button
            class="btn btn-sm btn-secondary"
            onclick={handleApply}
            disabled={$bypassLoading || !!markError || !!priorityError}
          >apply changes</button>
        </div>
      {/if}
    </div>

    <!-- Status row -->
    <div class="px-3 py-2">
      <div style="font-size: 10px; color: var(--text-muted); font-family: inherit;">
        {#if $bypassConfig.enabled}
          <span style="color: var(--text-dim);">ip rule → </span>
          <span style="color: {$bypassApplied ? 'var(--teal)' : 'var(--amber)'};">{$bypassIPRule || '—'}</span>
        {:else}
          <span style="color: var(--text-dim);">disabled — forwarded packets use default routing table</span>
        {/if}
      </div>
      <div style="font-size: 10px; color: var(--text-dim); margin-top: 4px; line-height: 1.6;">
        when enabled, marks forwarded packets with fwmark and adds an ip rule to keep them on the main routing table, bypassing TUN/VPN policy routing.
      </div>
    </div>
  </div>
</section>
