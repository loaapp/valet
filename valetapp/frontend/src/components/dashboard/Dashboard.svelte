<script>
  import { onMount } from 'svelte';
  import { getStatus, isConnected, refreshStatus } from '../../lib/stores/status.svelte.js';
  import { getCurrentMetrics, getHistoryData, getSelectedRange, setSelectedRange, startPolling as startMetricsPolling, stopPolling as stopMetricsPolling } from '../../lib/stores/metrics.svelte.js';
  import { Trust } from '../../../wailsjs/go/api/StatusService.js';
  import { Chart, LineController, LineElement, PointElement, LinearScale, CategoryScale, Filler, Tooltip, Legend } from 'chart.js';

  Chart.register(LineController, LineElement, PointElement, LinearScale, CategoryScale, Filler, Tooltip, Legend);

  let chartCanvas;
  let chartInstance = null;
  let chartTimer = null;

  async function handleTrust() {
    try {
      await Trust();
      await refreshStatus();
    } catch (err) {
      console.error('Trust failed:', err);
    }
  }

  // Compute summary stats from history totals (cumulative over selected range)
  function computeTotalRequests() {
    const h = getHistoryData();
    if (!h || !Array.isArray(h.totals)) return 0;
    return h.totals.reduce((sum, p) => sum + (p.reqs || 0), 0);
  }

  function computeErrorRate() {
    const h = getHistoryData();
    if (!h || !Array.isArray(h.totals)) return '0%';
    let reqs = 0, errs = 0;
    for (const p of h.totals) { reqs += (p.reqs || 0); errs += (p.errs || 0); }
    if (reqs === 0) return '0%';
    return ((errs / reqs) * 100).toFixed(1) + '%';
  }

  function computeAvgLatency() {
    const h = getHistoryData();
    if (!h || !Array.isArray(h.totals)) return '0ms';
    let reqs = 0, weighted = 0;
    for (const p of h.totals) {
      const r = p.reqs || 0;
      reqs += r;
      weighted += r * (p.latMs || 0);
    }
    if (reqs === 0) return '0ms';
    return (weighted / reqs).toFixed(1) + 'ms';
  }

  // Aggregate per-route stats from history data
  function getRouteRows() {
    const h = getHistoryData();
    if (!h || !h.routes || typeof h.routes !== 'object') return [];
    return Object.entries(h.routes)
      .map(([domain, points]) => {
        const arr = Array.isArray(points) ? points : [];
        let reqs = 0, errs = 0, bytesOut = 0, weighted = 0;
        for (const p of arr) {
          reqs += (p.reqs || 0);
          errs += (p.errs || 0);
          bytesOut += (p.bytesOut || 0);
          weighted += (p.reqs || 0) * (p.latMs || 0);
        }
        const latMs = reqs > 0 ? weighted / reqs : 0;
        return { domain, reqs, errs, latMs, bytesOut };
      })
      .sort((a, b) => b.reqs - a.reqs);
  }

  function updateChart() {
    const h = getHistoryData();
    if (!chartCanvas || !h || !Array.isArray(h.totals) || h.totals.length < 2) return;

    const labels = h.totals.map(p => {
      const d = new Date(p.ts * 1000);
      return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
    });
    const data = h.totals.map(p => p.reqs || 0);

    if (chartInstance) {
      chartInstance.data.labels = labels;
      chartInstance.data.datasets[0].data = data;
      chartInstance.update('none');
      return;
    }

    chartInstance = new Chart(chartCanvas, {
      type: 'line',
      data: {
        labels,
        datasets: [{
          label: 'Requests',
          data,
          borderColor: '#007AFF',
          backgroundColor: 'rgba(0, 122, 255, 0.08)',
          fill: true,
          tension: 0.3,
          pointRadius: 0,
          borderWidth: 2,
        }],
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        animation: false,
        plugins: {
          tooltip: { mode: 'index', intersect: false },
          legend: { display: false },
        },
        scales: {
          x: {
            grid: { color: 'rgba(128,128,128,0.15)' },
            ticks: { color: '#888', maxTicksLimit: 8, font: { size: 10 } },
          },
          y: {
            beginAtZero: true,
            grid: { color: 'rgba(128,128,128,0.15)' },
            ticks: { color: '#888', font: { size: 10 } },
          },
        },
        interaction: { mode: 'nearest', axis: 'x', intersect: false },
      },
    });
  }

  function hasChartData() {
    const h = getHistoryData();
    return h && Array.isArray(h.totals) && h.totals.length >= 2;
  }

  // Force chart rebuild when range changes
  function handleRangeChange(range) {
    setSelectedRange(range);
    if (chartInstance) { chartInstance.destroy(); chartInstance = null; }
    setTimeout(updateChart, 500);
  }

  onMount(() => {
    startMetricsPolling();
    chartTimer = setInterval(updateChart, 3000);
    // Initial render after a short delay to let first poll complete
    setTimeout(updateChart, 500);
    return () => {
      stopMetricsPolling();
      if (chartTimer) clearInterval(chartTimer);
      if (chartInstance) { chartInstance.destroy(); chartInstance = null; }
    };
  });
</script>

<div class="page">
  <div class="page-header">
    <h1>Dashboard</h1>
    <button class="btn btn-primary btn-sm" onclick={handleTrust}>Trust Certificates</button>
  </div>

  {#if !isConnected()}
    <div class="banner banner-warning">
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/>
      </svg>
      <span>Daemon is not running. Start the Valet daemon to manage routes and TLDs.</span>
    </div>
  {/if}

  <div class="grid">
    <div class="card">
      <div class="card-label">Status</div>
      <div class="card-value" class:text-success={isConnected()} class:text-danger={!isConnected()}>
        {isConnected() ? (getStatus()?.status ?? 'Running') : 'Disconnected'}
      </div>
    </div>
    <div class="card">
      <div class="card-label">Total Requests</div>
      <div class="card-value">{computeTotalRequests()}</div>
    </div>
    <div class="card">
      <div class="card-label">Error Rate</div>
      <div class="card-value">{computeErrorRate()}</div>
    </div>
    <div class="card">
      <div class="card-label">Avg Latency</div>
      <div class="card-value">{computeAvgLatency()}</div>
    </div>
  </div>

  <div class="chart-section">
    <div class="chart-header">
      <h2>Requests Over Time</h2>
      <div class="range-buttons">
        {#each ['5m', '1h', '24h'] as range}
          <button
            class="btn btn-sm"
            class:btn-primary={getSelectedRange() === range}
            onclick={() => handleRangeChange(range)}
          >{range}</button>
        {/each}
      </div>
    </div>
    <div class="chart-container">
      <canvas bind:this={chartCanvas}></canvas>
      {#if !hasChartData()}
        <div class="chart-overlay">No data for this range</div>
      {/if}
    </div>
  </div>

  {#if getRouteRows().length > 0}
    <div class="table-section">
      <h2>Route Statistics</h2>
      <table class="table">
        <thead>
          <tr>
            <th>Domain</th>
            <th>Requests</th>
            <th>Errors</th>
            <th>Avg Latency</th>
            <th>Bytes</th>
          </tr>
        </thead>
        <tbody>
          {#each getRouteRows() as row}
            <tr>
              <td class="domain">{row.domain}</td>
              <td>{row.reqs}</td>
              <td>{row.errs}</td>
              <td>{row.latMs.toFixed(1)}ms</td>
              <td>{row.bytesOut}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</div>

<style>
  .page {
    max-width: 720px;
  }
  .page-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 20px;
  }
  h1 {
    font-size: 18px;
    font-weight: 600;
    color: var(--text-primary);
  }
  h2 {
    font-size: 13px;
    font-weight: 600;
    color: var(--text-primary);
  }
  .banner {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px 14px;
    border-radius: var(--radius);
    margin-bottom: 20px;
    font-size: 12px;
  }
  .banner-warning {
    background: var(--danger-bg);
    color: var(--danger);
    border: 1px solid rgba(255,69,58,0.2);
  }
  .grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 12px;
    margin-bottom: 20px;
  }
  .card {
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    padding: 16px;
    box-shadow: var(--shadow);
  }
  .card-label {
    font-size: 11px;
    font-weight: 500;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.5px;
    margin-bottom: 6px;
  }
  .card-value {
    font-size: 20px;
    font-weight: 600;
    color: var(--text-primary);
  }
  .text-success {
    color: var(--success);
  }
  .text-danger {
    color: var(--danger);
  }
  .chart-section {
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    padding: 16px;
    margin-bottom: 20px;
    box-shadow: var(--shadow);
  }
  .chart-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 12px;
  }
  .range-buttons {
    display: flex;
    gap: 4px;
  }
  .chart-container {
    width: 100%;
    height: 180px;
    position: relative;
  }
  .chart-overlay {
    position: absolute;
    inset: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--text-muted);
    font-size: 12px;
    pointer-events: none;
  }
  .table-section {
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    padding: 16px;
    box-shadow: var(--shadow);
  }
  .table-section h2 {
    margin-bottom: 12px;
  }
  .table {
    width: 100%;
    border-collapse: collapse;
    font-size: 12px;
  }
  .table th {
    text-align: left;
    padding: 8px 10px;
    font-weight: 500;
    color: var(--text-muted);
    border-bottom: 1px solid var(--border);
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }
  .table td {
    padding: 8px 10px;
    border-bottom: 1px solid var(--border-subtle);
    color: var(--text-secondary);
  }
  .table .domain {
    color: var(--text-primary);
    font-weight: 500;
  }
</style>
