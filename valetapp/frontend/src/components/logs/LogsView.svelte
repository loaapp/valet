<script>
  import { onMount } from 'svelte';
  import { getLogs, startPolling, stopPolling, clearLogs } from '../../lib/stores/logs.svelte.js';
  import { getDnsLogs, startPolling as startDnsPolling, stopPolling as stopDnsPolling, clearDnsLogs } from '../../lib/stores/dnsLogs.svelte.js';

  let tab = $state('http');
  let autoScroll = $state(true);
  let tableBody;

  function formatTime(ts) {
    if (!ts) return '--:--:--';
    const d = new Date(ts * 1000);
    const h = String(d.getHours()).padStart(2, '0');
    const m = String(d.getMinutes()).padStart(2, '0');
    const s = String(d.getSeconds()).padStart(2, '0');
    const ms = String(d.getMilliseconds()).padStart(3, '0');
    return `${h}:${m}:${s}.${ms}`;
  }

  function methodClass(method) {
    switch (method) {
      case 'GET': return 'method-get';
      case 'POST': return 'method-post';
      case 'PUT': return 'method-put';
      case 'DELETE': return 'method-delete';
      default: return '';
    }
  }

  function statusClass(code) {
    if (code >= 200 && code < 300) return 'status-2xx';
    if (code >= 400 && code < 500) return 'status-4xx';
    if (code >= 500) return 'status-5xx';
    return '';
  }

  function actionClass(action) {
    return action === 'local' ? 'action-local' : 'action-forwarded';
  }

  $effect(() => {
    const entries = tab === 'http' ? getLogs() : getDnsLogs();
    if (autoScroll && tableBody && entries.length > 0) {
      requestAnimationFrame(() => {
        tableBody.scrollTop = tableBody.scrollHeight;
      });
    }
  });

  async function handleClear() {
    if (tab === 'http') {
      stopPolling();
      await clearLogs();
      startPolling();
    } else {
      stopDnsPolling();
      await clearDnsLogs();
      startDnsPolling();
    }
  }

  onMount(() => {
    startPolling();
    startDnsPolling();
    return () => { stopPolling(); stopDnsPolling(); };
  });
</script>

<div class="logs-page">
  <div class="logs-header">
    <div class="tabs">
      <button class:active={tab === 'http'} onclick={() => tab = 'http'}>HTTP</button>
      <button class:active={tab === 'dns'} onclick={() => tab = 'dns'}>DNS</button>
    </div>
    <div class="logs-actions">
      <label class="auto-scroll-toggle">
        <input type="checkbox" bind:checked={autoScroll} />
        <span>Auto-scroll</span>
      </label>
      <button class="btn btn-sm" onclick={handleClear}>Clear</button>
    </div>
  </div>

  <div class="logs-table-wrapper" bind:this={tableBody}>
    {#if tab === 'http'}
      <table class="logs-table">
        <thead>
          <tr>
            <th class="col-time">Time</th>
            <th class="col-method">Method</th>
            <th class="col-host">Host</th>
            <th class="col-uri">URI</th>
            <th class="col-status">Status</th>
            <th class="col-duration">Duration</th>
          </tr>
        </thead>
        <tbody>
          {#each getLogs() as entry}
            <tr>
              <td class="col-time mono">{formatTime(entry.ts)}</td>
              <td class="col-method"><span class="method-badge {methodClass(entry.method)}">{entry.method ?? '--'}</span></td>
              <td class="col-host">{entry.host ?? '--'}</td>
              <td class="col-uri mono">{entry.uri ?? '--'}</td>
              <td class="col-status {statusClass(entry.status)}">{entry.status ?? '--'}</td>
              <td class="col-duration mono">{entry.duration != null ? Math.round(entry.duration * 1000) + 'ms' : '--'}</td>
            </tr>
          {/each}
          {#if getLogs().length === 0}
            <tr><td colspan="6" class="empty">No HTTP log entries yet</td></tr>
          {/if}
        </tbody>
      </table>
    {:else}
      <table class="logs-table">
        <thead>
          <tr>
            <th class="col-time">Time</th>
            <th class="col-domain">Domain</th>
            <th class="col-type">Type</th>
            <th class="col-action">Action</th>
            <th class="col-result">Result</th>
          </tr>
        </thead>
        <tbody>
          {#each getDnsLogs() as entry}
            <tr>
              <td class="col-time mono">{formatTime(entry.ts)}</td>
              <td class="col-domain">{entry.domain ?? '--'}</td>
              <td class="col-type mono">{entry.type ?? '--'}</td>
              <td class="col-action"><span class="action-badge {actionClass(entry.action)}">{entry.action ?? '--'}</span></td>
              <td class="col-result mono">{entry.result ?? '--'}</td>
            </tr>
          {/each}
          {#if getDnsLogs().length === 0}
            <tr><td colspan="5" class="empty">No DNS queries logged yet</td></tr>
          {/if}
        </tbody>
      </table>
    {/if}
  </div>
</div>

<style>
  .logs-page {
    display: flex;
    flex-direction: column;
    height: 100%;
    padding: 24px;
    overflow: hidden;
  }
  .logs-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 16px;
    flex-shrink: 0;
  }
  .tabs {
    display: flex;
    gap: 0;
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    overflow: hidden;
  }
  .tabs button {
    background: transparent;
    border: none;
    padding: 6px 16px;
    font-size: 12px;
    font-weight: 500;
    color: var(--text-secondary);
    cursor: pointer;
    font-family: var(--font-sans);
  }
  .tabs button:hover { background: var(--bg-hover); }
  .tabs button.active { background: var(--accent); color: #fff; }
  .tabs button + button { border-left: 1px solid var(--border); }
  .logs-actions {
    display: flex;
    align-items: center;
    gap: 12px;
  }
  .auto-scroll-toggle {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 12px;
    color: var(--text-secondary);
    cursor: pointer;
  }
  .auto-scroll-toggle input { margin: 0; }
  .logs-table-wrapper {
    flex: 1;
    overflow-y: auto;
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--bg-card);
  }
  .logs-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 11px;
  }
  .logs-table thead {
    position: sticky;
    top: 0;
    z-index: 1;
    background: var(--bg-card);
  }
  .logs-table th {
    text-align: left;
    padding: 8px 10px;
    font-weight: 500;
    color: var(--text-muted);
    border-bottom: 1px solid var(--border);
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }
  .logs-table td {
    padding: 5px 10px;
    border-bottom: 1px solid var(--border-subtle);
    color: var(--text-secondary);
    white-space: nowrap;
  }
  .mono {
    font-family: var(--font-mono);
    font-size: 11px;
  }
  .col-time { width: 100px; }
  .col-method { width: 70px; }
  .col-host { width: 150px; }
  .col-domain { min-width: 200px; }
  .col-type { width: 50px; }
  .col-action { width: 90px; }
  .col-result { min-width: 120px; }
  .col-uri { flex: 1; }
  .col-status { width: 60px; }
  .col-duration { width: 80px; }
  .method-badge, .action-badge {
    display: inline-block;
    padding: 1px 6px;
    border-radius: 3px;
    font-size: 10px;
    font-weight: 600;
  }
  .method-get { background: rgba(0,122,255,0.15); color: #007AFF; }
  .method-post { background: rgba(52,199,89,0.15); color: #34C759; }
  .method-put { background: rgba(255,159,10,0.15); color: #FF9F0A; }
  .method-delete { background: rgba(255,69,58,0.15); color: #FF453A; }
  .action-local { background: rgba(52,199,89,0.15); color: #34C759; }
  .action-forwarded { background: rgba(0,122,255,0.15); color: #007AFF; }
  .status-2xx { color: #34C759; }
  .status-4xx { color: #FF9F0A; }
  .status-5xx { color: #FF453A; }
  .empty {
    text-align: center;
    padding: 40px 10px !important;
    color: var(--text-muted);
  }
</style>
