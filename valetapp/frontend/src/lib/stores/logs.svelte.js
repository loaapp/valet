import { GetLogs, GetLogsSince } from '../../../wailsjs/go/api/LogsService.js';

let logs = $state([]);
let lastTs = $state(0);
let pollTimer = null;

export function getLogs() {
  return logs;
}

export async function refreshLogs() {
  try {
    const newLogs = lastTs > 0 ? await GetLogsSince(lastTs) : await GetLogs(200);
    if (newLogs && newLogs.length > 0) {
      logs = [...logs, ...newLogs].slice(-500);
      lastTs = newLogs[newLogs.length - 1].ts;
    }
  } catch {
    // silently ignore fetch errors
  }
}

export function startPolling() {
  refreshLogs();
  pollTimer = setInterval(refreshLogs, 2000);
}

export function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer);
    pollTimer = null;
  }
}

export function clearLogs() {
  logs = [];
  lastTs = 0;
}
