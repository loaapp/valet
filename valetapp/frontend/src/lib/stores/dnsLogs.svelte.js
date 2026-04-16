import { GetDNSLogs, ClearDNSLogs } from '../../../wailsjs/go/api/LogsService.js';

let logs = $state([]);
let timer = null;

export function getDnsLogs() { return logs; }

export async function refreshDnsLogs() {
  try {
    logs = await GetDNSLogs(200);
  } catch {
    // service may not be available
  }
}

export function startPolling() {
  refreshDnsLogs();
  timer = setInterval(refreshDnsLogs, 2000);
}

export function stopPolling() {
  if (timer) { clearInterval(timer); timer = null; }
}

export async function clearDnsLogs() {
  try { await ClearDNSLogs(); } catch {}
  logs = [];
}
