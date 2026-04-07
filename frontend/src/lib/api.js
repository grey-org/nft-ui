const API_BASE = '/api/v1';

async function request(path, options = {}) {
  const response = await fetch(`${API_BASE}${path}`, {
    headers: {
      'Content-Type': 'application/json',
      ...options.headers,
    },
    ...options,
  });

  const data = await response.json();

  if (!response.ok) {
    throw new Error(data.error || `HTTP ${response.status}`);
  }

  return data;
}

export async function fetchQuotas() {
  return request('/quotas');
}

export async function resetQuota(id) {
  return request(`/quotas/${encodeURIComponent(id)}/reset`, {
    method: 'POST',
  });
}

export async function batchResetQuotas(ids) {
  return request('/quotas/batch-reset', {
    method: 'POST',
    body: JSON.stringify({ ids }),
  });
}

export async function modifyQuota(id, bytes) {
  return request(`/quotas/${encodeURIComponent(id)}`, {
    method: 'PUT',
    body: JSON.stringify({ bytes }),
  });
}

export async function addQuota(port, bytes, comment) {
  return request('/quotas', {
    method: 'POST',
    body: JSON.stringify({ port, bytes, comment }),
  });
}

export async function deleteQuota(id) {
  return request(`/quotas/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  });
}

export async function addPort(port, protocol = 'tcp') {
  return request('/ports', {
    method: 'POST',
    body: JSON.stringify({ port, protocol }),
  });
}

export async function deletePort(handle) {
  return request(`/ports/${handle}`, {
    method: 'DELETE',
  });
}

// Forwarding API functions
export async function fetchForwardingRules() {
  return request('/forwarding');
}

export async function testForwardingTarget(dstIP, dstPort, protocol, timeoutMs = 1500) {
  const params = new URLSearchParams({
    dst_ip: dstIP,
    dst_port: String(dstPort),
    protocol,
    timeout_ms: String(timeoutMs),
  });

  return request(`/forwarding/test?${params.toString()}`);
}

export async function addForwardingRule(srcPort, dstIP, dstPort, protocol, comment, limitMbps, mssMode, sourceNATMode, snatAddress, addrFamily) {
  return request('/forwarding', {
    method: 'POST',
    body: JSON.stringify({
      src_port: srcPort,
      dst_ip: dstIP,
      dst_port: dstPort,
      protocol,
      addr_family: addrFamily || 'ip',
      comment,
      limit_mbps: limitMbps || 0,
      mss_mode: mssMode || 'pmtu',
      source_nat_mode: sourceNATMode || 'masquerade',
      snat_address: sourceNATMode === 'snat' ? (snatAddress || '') : '',
    }),
  });
}

export async function editForwardingRule(id, dstIP, dstPort, protocol, comment, limitMbps, mssMode, sourceNATMode, snatAddress, addrFamily) {
  return request(`/forwarding/${encodeURIComponent(id)}`, {
    method: 'PUT',
    body: JSON.stringify({
      dst_ip: dstIP,
      dst_port: dstPort,
      protocol,
      addr_family: addrFamily || 'ip',
      comment,
      limit_mbps: limitMbps || 0,
      mss_mode: mssMode || 'pmtu',
      source_nat_mode: sourceNATMode || 'masquerade',
      snat_address: sourceNATMode === 'snat' ? (snatAddress || '') : '',
    }),
  });
}

export async function deleteForwardingRule(id) {
  return request(`/forwarding/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  });
}

export async function enableForwardingRule(id) {
  return request(`/forwarding/${encodeURIComponent(id)}/enable`, {
    method: 'POST',
  });
}

export async function disableForwardingRule(id) {
  return request(`/forwarding/${encodeURIComponent(id)}/disable`, {
    method: 'POST',
  });
}

export async function fetchRawRuleset() {
  return request('/raw-ruleset');
}

// Backup/restore functions
export async function exportBackup() {
  // This needs to trigger a download, so use fetch+blob
  const response = await fetch(`${API_BASE}/backup`);
  if (!response.ok) throw new Error('Export failed');
  const blob = await response.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = response.headers.get('content-disposition')?.split('filename=')[1] || 'nft-ui-backup.json';
  a.click();
  URL.revokeObjectURL(url);
}

export async function importBackup(file) {
  const text = await file.text();
  const data = JSON.parse(text);
  return request('/backup', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

// Interface Forwarding API
export async function fetchIfaceForwardingRules() {
  return request('/iface-forwarding');
}

export async function addIfaceForwardingRule(iifName, addrFamily, natAddrFamily, dstAddr, natTo, protocol, comment) {
  return request('/iface-forwarding', {
    method: 'POST',
    body: JSON.stringify({
      iif_name: iifName,
      addr_family: addrFamily,
      nat_addr_family: natAddrFamily || '',
      dst_addr: dstAddr,
      nat_to: natTo,
      protocol,
      comment,
    }),
  });
}

export async function editIfaceForwardingRule(id, iifName, addrFamily, natAddrFamily, dstAddr, natTo, protocol, comment) {
  return request(`/iface-forwarding/${encodeURIComponent(id)}`, {
    method: 'PUT',
    body: JSON.stringify({
      iif_name: iifName,
      addr_family: addrFamily,
      nat_addr_family: natAddrFamily || '',
      dst_addr: dstAddr,
      nat_to: natTo,
      protocol,
      comment,
    }),
  });
}

export async function deleteIfaceForwardingRule(id) {
  return request(`/iface-forwarding/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  });
}

export async function enableIfaceForwardingRule(id) {
  return request(`/iface-forwarding/${encodeURIComponent(id)}/enable`, {
    method: 'POST',
  });
}

export async function disableIfaceForwardingRule(id) {
  return request(`/iface-forwarding/${encodeURIComponent(id)}/disable`, {
    method: 'POST',
  });
}

// Bypass API
export async function fetchBypass() {
  return request('/bypass');
}

export async function setBypass(enabled, mark, priority) {
  return request('/bypass', {
    method: 'POST',
    body: JSON.stringify({ enabled, mark, priority }),
  });
}
