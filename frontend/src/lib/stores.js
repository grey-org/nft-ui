import { writable, derived, get } from 'svelte/store';
import {
  fetchQuotas,
  addPort,
  deletePort,
  fetchForwardingRules,
  addForwardingRule as apiAddForwarding,
  editForwardingRule as apiEditForwarding,
  deleteForwardingRule as apiDeleteForwarding,
  enableForwardingRule as apiEnableForwarding,
  disableForwardingRule as apiDisableForwarding,
  exportBackup as apiExportBackup,
  importBackup as apiImportBackup,
} from './api.js';

// Core state
export const quotas = writable([]);
export const allowedPorts = writable([]);
export const loading = writable(false);
export const error = writable(null);
export const selectedIds = writable(new Set());
export const readOnly = writable(false);
export const refreshInterval = writable(20);

// Modal state - when true, auto-refresh is paused
export const isEditingModal = writable(false);

// Helper to pause refresh while editing
export function pauseRefresh() {
  isEditingModal.set(true);
}

export function resumeRefresh() {
  isEditingModal.set(false);
}

// Derived stores
export const sortedQuotas = derived(quotas, ($quotas) =>
  [...$quotas].sort((a, b) => {
    // Sort by status (exceeded first), then by usage %
    if (a.status === 'exceeded' && b.status !== 'exceeded') return -1;
    if (b.status === 'exceeded' && a.status !== 'exceeded') return 1;
    return b.usage_percent - a.usage_percent;
  })
);

export const hasSelection = derived(selectedIds, ($ids) => $ids.size > 0);

export const selectedCount = derived(selectedIds, ($ids) => $ids.size);

// Actions
export async function loadQuotas() {
  loading.set(true);
  error.set(null);

  try {
    const data = await fetchQuotas();
    quotas.set(data.quotas || []);
    allowedPorts.set(data.allowed_ports || []);
    readOnly.set(data.read_only);
    refreshInterval.set(data.refresh_interval);
  } catch (e) {
    error.set(e.message);
  } finally {
    loading.set(false);
  }
}

export function toggleSelection(id) {
  selectedIds.update((ids) => {
    const newIds = new Set(ids);
    if (newIds.has(id)) {
      newIds.delete(id);
    } else {
      newIds.add(id);
    }
    return newIds;
  });
}

export function clearSelection() {
  selectedIds.set(new Set());
}

export function selectAll() {
  const allIds = new Set(get(quotas).map((q) => q.id));
  selectedIds.set(allIds);
}

// Notifications
export const notifications = writable([]);
let notificationId = 0;

export function addNotification(message, type = 'info', duration = 3000) {
  const id = ++notificationId;
  notifications.update((n) => [...n, { id, message, type }]);

  if (duration > 0) {
    setTimeout(() => {
      removeNotification(id);
    }, duration);
  }

  return id;
}

export function removeNotification(id) {
  notifications.update((n) => n.filter((item) => item.id !== id));
}

export const success = (msg) => addNotification(msg, 'success');
export const errorNotify = (msg) => addNotification(msg, 'error', 5000);
export const warning = (msg) => addNotification(msg, 'warning');

// Port management actions
export async function addAllowedPort(port) {
  try {
    await addPort(port);
    success('Port added successfully');
    await loadQuotas();
  } catch (e) {
    errorNotify(`Failed to add port: ${e.message}`);
    throw e;
  }
}

export async function removeAllowedPort(handle) {
  try {
    await deletePort(handle);
    success('Port deleted successfully');
    await loadQuotas();
  } catch (e) {
    errorNotify(`Failed to delete port: ${e.message}`);
    throw e;
  }
}

// Forwarding state
export const forwardingRules = writable([]);
export const forwardingLoading = writable(false);

// Sorted forwarding rules: enabled first, then by source port
export const sortedForwardingRules = derived(forwardingRules, ($rules) =>
  [...$rules].sort((a, b) => {
    // Enabled rules first
    if (a.enabled !== b.enabled) return b.enabled - a.enabled;
    // Then by source port
    return a.src_port - b.src_port;
  })
);

// Load forwarding rules
export async function loadForwardingRules() {
  forwardingLoading.set(true);
  try {
    const data = await fetchForwardingRules();
    forwardingRules.set(data.rules || []);
  } catch (e) {
    errorNotify(`Failed to load forwarding rules: ${e.message}`);
  } finally {
    forwardingLoading.set(false);
  }
}

// Add forwarding rule
export async function addForwardingRule(srcPort, dstIP, dstPort, protocol, comment, limitMbps, mssMode, sourceNATMode, snatAddress) {
  try {
    await apiAddForwarding(srcPort, dstIP, dstPort, protocol, comment, limitMbps, mssMode, sourceNATMode, snatAddress);
    success('Forwarding rule added');
    await loadForwardingRules();
  } catch (e) {
    errorNotify(`Failed to add forwarding rule: ${e.message}`);
    throw e;
  }
}

// Edit forwarding rule
export async function editForwardingRule(id, dstIP, dstPort, protocol, comment, limitMbps, mssMode, sourceNATMode, snatAddress) {
  try {
    await apiEditForwarding(id, dstIP, dstPort, protocol, comment, limitMbps, mssMode, sourceNATMode, snatAddress);
    success('Forwarding rule updated');
    await loadForwardingRules();
  } catch (e) {
    errorNotify(`Failed to update forwarding rule: ${e.message}`);
    throw e;
  }
}

// Delete forwarding rule
export async function removeForwardingRule(id) {
  try {
    await apiDeleteForwarding(id);
    success('Forwarding rule deleted');
    await loadForwardingRules();
  } catch (e) {
    errorNotify(`Failed to delete forwarding rule: ${e.message}`);
    throw e;
  }
}

// Enable forwarding rule
export async function enableForwardingRule(id) {
  try {
    await apiEnableForwarding(id);
    success('Forwarding rule enabled');
    await loadForwardingRules();
  } catch (e) {
    errorNotify(`Failed to enable forwarding rule: ${e.message}`);
    throw e;
  }
}

// Disable forwarding rule
export async function disableForwardingRule(id) {
  try {
    await apiDisableForwarding(id);
    success('Forwarding rule disabled');
    await loadForwardingRules();
  } catch (e) {
    errorNotify(`Failed to disable forwarding rule: ${e.message}`);
    throw e;
  }
}

// Export backup
export async function exportBackup() {
  try {
    await apiExportBackup();
    success('Backup exported successfully');
  } catch (e) {
    errorNotify(`Failed to export backup: ${e.message}`);
    throw e;
  }
}

// Import backup
export async function importBackup(file) {
  try {
    const result = await apiImportBackup(file);
    const summary = result.summary;
    const msg = `Imported: ${summary.quotas_added} quotas, ${summary.forwarding_added} forwarding rules, ${summary.ports_added} ports`;
    success(msg);
    // Reload all data
    await loadQuotas();
    await loadForwardingRules();
  } catch (e) {
    errorNotify(`Failed to import backup: ${e.message}`);
    throw e;
  }
}
