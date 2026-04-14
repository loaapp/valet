import { GetStatus, IsDaemonRunning } from '../../../wailsjs/go/api/StatusService.js';

let status = $state(null);
let connected = $state(false);
let pollTimer = null;

async function refresh() {
  try {
    const running = await IsDaemonRunning();
    connected = running;
    if (running) {
      status = await GetStatus();
    } else {
      status = null;
    }
  } catch {
    connected = false;
    status = null;
  }
}

export function getStatus() {
  return status;
}

export function isConnected() {
  return connected;
}

export async function refreshStatus() {
  await refresh();
}

export function startPolling(ms = 5000) {
  refresh();
  pollTimer = setInterval(refresh, ms);
}

export function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer);
    pollTimer = null;
  }
}
