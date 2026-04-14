<script>
  import { getStatus, isConnected, refreshStatus } from '../../lib/stores/status.svelte.js';
  import { Trust } from '../../../wailsjs/go/api/StatusService.js';

  async function handleTrust() {
    try {
      await Trust();
      await refreshStatus();
    } catch (err) {
      console.error('Trust failed:', err);
    }
  }
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
      <div class="card-label">Routes</div>
      <div class="card-value">{getStatus()?.routes ?? 0}</div>
    </div>
    <div class="card">
      <div class="card-label">TLDs</div>
      <div class="card-value">{getStatus()?.tlds ?? 0}</div>
    </div>
    <div class="card">
      <div class="card-label">mkcert</div>
      <div class="card-value" class:text-success={getStatus()?.mkcert} class:text-danger={!getStatus()?.mkcert}>
        {getStatus()?.mkcert ? 'Installed' : 'Not Found'}
      </div>
    </div>
  </div>
</div>

<style>
  .page {
    max-width: 640px;
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
</style>
