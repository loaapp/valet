import { GetCurrent, GetHistory } from '../../../wailsjs/go/api/MetricsService.js';

let currentMetrics = $state({});
let historyData = $state(null);
let selectedRange = $state('5m');
let metricsTimer = null;
let historyTimer = null;

export function getCurrentMetrics() {
  return currentMetrics;
}

export function getHistoryData() {
  return historyData;
}

export function getSelectedRange() {
  return selectedRange;
}

export function setSelectedRange(r) {
  selectedRange = r;
  refreshHistory();
}

export async function refreshMetrics() {
  try {
    currentMetrics = await GetCurrent();
  } catch {
    currentMetrics = {};
  }
}

export async function refreshHistory() {
  try {
    historyData = await GetHistory(selectedRange);
  } catch {
    historyData = null;
  }
}

export function startPolling() {
  refreshMetrics();
  refreshHistory();
  metricsTimer = setInterval(refreshMetrics, 2000);
  historyTimer = setInterval(refreshHistory, 5000);
}

export function stopPolling() {
  if (metricsTimer) {
    clearInterval(metricsTimer);
    metricsTimer = null;
  }
  if (historyTimer) {
    clearInterval(historyTimer);
    historyTimer = null;
  }
}
