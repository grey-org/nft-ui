// Format bytes to human-readable string
export function formatBytes(bytes, decimals = 2) {
  if (bytes === 0) return '0 B';

  const k = 1000;
  const dm = decimals < 0 ? 0 : decimals;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];

  const i = Math.floor(Math.log(bytes) / Math.log(k));

  return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + ' ' + sizes[i];
}

// Parse human-readable bytes string to number
export function parseBytes(value, unit) {
  const units = {
    MB: 1000 * 1000,
    GB: 1000 * 1000 * 1000,
    TB: 1000 * 1000 * 1000 * 1000,
  };

  return value * (units[unit] || 1);
}

// Format percentage
export function formatPercent(value) {
  return value.toFixed(1) + '%';
}

// Get status color
export function getStatusColor(status) {
  switch (status) {
    case 'exceeded':
      return 'var(--color-danger)';
    case 'warning':
      return 'var(--color-warning)';
    default:
      return 'var(--color-ok)';
  }
}

// Get progress bar color based on percentage
export function getProgressColor(percent) {
  if (percent >= 90) return 'var(--color-danger)';
  if (percent >= 70) return 'var(--color-warning)';
  return 'var(--color-ok)';
}

// Validate IPv4 address
export function isValidIPv4(ip) {
  if (!ip) return false;
  const pattern =
    /^(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$/;
  return pattern.test(ip);
}

// Validate IPv6 address
export function isValidIPv6(ip) {
  if (!ip) return false;
  const parsed = ip.match(/^[0-9a-fA-F:]+$/);
  if (!parsed) return false;
  // Use a simple structural check: must contain at least one colon and no IPv4 octets
  if (!ip.includes(':')) return false;
  // Check for valid IPv6 format via a broader pattern
  const ipv6Pattern = /^([0-9a-fA-F]{0,4}:){2,7}[0-9a-fA-F]{0,4}$|^::([0-9a-fA-F]{0,4}:){0,6}[0-9a-fA-F]{0,4}$|^([0-9a-fA-F]{0,4}:){1,6}:[0-9a-fA-F]{0,4}$/;
  return ipv6Pattern.test(ip);
}

// Validate IP address by family ("ip" = IPv4, "ip6" = IPv6)
export function isValidIPForFamily(ip, family) {
  if (family === 'ip6') return isValidIPv6(ip);
  return isValidIPv4(ip);
}

// Format protocol for display
export function formatProtocol(protocol) {
  switch (protocol) {
    case 'tcp':
      return 'TCP';
    case 'udp':
      return 'UDP';
    case 'both':
      return 'TCP+UDP';
    default:
      return protocol?.toUpperCase() || 'Unknown';
  }
}
