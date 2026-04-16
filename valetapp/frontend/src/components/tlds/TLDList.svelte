<script>
  import { onMount } from 'svelte';
  import { getTlds, getEntries, loadAll, removeEntry } from '../../lib/stores/dns.svelte.js';
  import { GetCertInfo } from '../../../wailsjs/go/api/StatusService.js';
  import SubdomainForm from './SubdomainForm.svelte';

  let showFormForTLD = $state(null);
  let certInfo = $state(null);

  onMount(() => {
    loadAll();
    loadCertInfo();
  });

  async function loadCertInfo() {
    try { certInfo = await GetCertInfo(); } catch { certInfo = null; }
  }

  function entriesForTLD(tld) {
    return getEntries().filter(e => e.tld === tld);
  }

  async function handleDeleteEntry(domain) {
    await removeEntry(domain);
  }
</script>

<div class="page">
  <div class="page-header">
    <h1>TLDs & DNS Entries</h1>
  </div>

  {#each getTlds() as tld (tld.tld)}
    {@const tldEntries = entriesForTLD(tld.tld)}
    <div class="tld-section">
      <div class="tld-header">
        <div class="tld-info">
          <span class="tld-name">.{tld.tld}</span>
          <span class="badge" class:badge-on={tld.resolverInstalled}>
            {tld.resolverInstalled ? 'Installed' : 'Missing'}
          </span>
        </div>
        <button class="btn btn-ghost btn-sm" onclick={() => showFormForTLD = tld.tld}>
          Register Subdomain
        </button>
      </div>

      {#if !tld.resolverInstalled}
        <div class="resolver-hint">
          Resolver missing. Run: <code>sudo valetd tld add --tld {tld.tld}</code>
        </div>
      {/if}

      {#if tldEntries.length > 0}
        <div class="entries-table">
          <div class="entries-header">
            <span>Domain</span>
            <span>Target</span>
            <span>Actions</span>
          </div>
          {#each tldEntries as entry (entry.domain)}
            <div class="entry-row">
              <span class="entry-domain">{entry.domain}</span>
              <span class="entry-target">
                <code>{entry.target}</code>
              </span>
              <span class="actions">
                <button class="btn btn-ghost btn-sm text-danger" onclick={() => handleDeleteEntry(entry.domain)}>Delete</button>
              </span>
            </div>
          {/each}
        </div>
      {:else}
        <div class="no-entries">No subdomains registered.</div>
      {/if}
    </div>
  {:else}
    <div class="empty">
      No TLDs configured. Add one via CLI: <code>sudo valetd tld add --tld example</code>
    </div>
  {/each}

  {#if certInfo?.exists}
    <div class="cert-section">
      <div class="cert-header">
        <h3>TLS Certificate</h3>
        <span class="cert-meta">Issued by {certInfo.issuer} · Expires {certInfo.notAfter}</span>
      </div>
      <div class="cert-domains">
        {#each certInfo.domains as domain}
          <span class="cert-domain">{domain}</span>
        {/each}
      </div>
    </div>
  {/if}
</div>

{#if showFormForTLD}
  <SubdomainForm tld={showFormForTLD} onClose={() => showFormForTLD = null} />
{/if}

<style>
  .page {
    max-width: 700px;
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
  .tld-section {
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--bg-card);
    margin-bottom: 16px;
    overflow: hidden;
  }
  .tld-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 10px 14px;
    background: var(--bg-tertiary);
    border-bottom: 1px solid var(--border-subtle);
  }
  .tld-info {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .tld-name {
    font-size: 14px;
    font-weight: 600;
    color: var(--text-accent);
  }
  .badge {
    display: inline-block;
    padding: 2px 6px;
    border-radius: 3px;
    font-size: 10px;
    font-weight: 600;
    background: var(--badge-bg);
    color: var(--badge-text);
  }
  .badge-on {
    background: rgba(48,209,88,0.15);
    color: var(--success);
  }
  .resolver-hint {
    padding: 8px 14px;
    background: rgba(255,159,10,0.08);
    border-bottom: 1px solid var(--border-subtle);
    font-size: 11px;
    color: var(--text-secondary);
  }
  .resolver-hint code {
    font-family: var(--font-mono);
    background: var(--badge-bg);
    padding: 1px 4px;
    border-radius: 3px;
    font-size: 10px;
  }
  .entries-table {
    /* nested table */
  }
  .entries-header {
    display: grid;
    grid-template-columns: 1fr 1fr 60px;
    gap: 8px;
    padding: 6px 14px;
    border-bottom: 1px solid var(--border-subtle);
    font-size: 10px;
    font-weight: 600;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }
  .entry-row {
    display: grid;
    grid-template-columns: 1fr 1fr 60px;
    gap: 8px;
    padding: 6px 14px;
    align-items: center;
    border-bottom: 1px solid var(--border-subtle);
    font-size: 12px;
  }
  .entry-row:last-child {
    border-bottom: none;
  }
  .entry-domain {
    color: var(--text-primary);
    font-weight: 500;
  }
  .entry-target code {
    font-family: var(--font-mono);
    font-size: 11px;
    color: var(--text-secondary);
    background: var(--badge-bg);
    padding: 1px 4px;
    border-radius: 3px;
  }
  .actions {
    display: flex;
    gap: 2px;
  }
  .text-danger {
    color: var(--danger);
  }
  .no-entries {
    padding: 16px 14px;
    text-align: center;
    color: var(--text-muted);
    font-size: 11px;
  }
  .empty {
    padding: 32px 14px;
    text-align: center;
    color: var(--text-muted);
    font-size: 12px;
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--bg-card);
  }
  .empty code {
    font-family: var(--font-mono);
    background: var(--badge-bg);
    padding: 1px 4px;
    border-radius: 3px;
    font-size: 10px;
  }
  .cert-section {
    margin-top: 24px;
    padding: 14px;
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--bg-card);
  }
  .cert-header {
    display: flex;
    align-items: baseline;
    gap: 12px;
    margin-bottom: 10px;
  }
  .cert-header h3 {
    font-size: 13px;
    font-weight: 600;
    margin: 0;
  }
  .cert-meta {
    font-size: 11px;
    color: var(--text-muted);
  }
  .cert-domains {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }
  .cert-domain {
    display: inline-block;
    padding: 2px 8px;
    background: rgba(48,209,88,0.1);
    color: var(--success);
    border-radius: 3px;
    font-family: var(--font-mono);
    font-size: 11px;
  }
</style>
