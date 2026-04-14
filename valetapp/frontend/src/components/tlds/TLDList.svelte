<script>
  import { onMount } from 'svelte';
  import { getTlds, loadTlds, deleteTld } from '../../lib/stores/tlds.svelte.js';
  import TLDForm from './TLDForm.svelte';

  let showForm = $state(false);

  onMount(() => { loadTlds(); });

  async function handleDelete(tld) {
    await deleteTld(tld);
  }
</script>

<div class="page">
  <div class="page-header">
    <h1>TLDs</h1>
    <button class="btn btn-primary btn-sm" onclick={() => showForm = true}>Add TLD</button>
  </div>

  <div class="table">
    <div class="table-header">
      <span>TLD</span>
      <span>Resolver</span>
      <span>Actions</span>
    </div>
    {#each getTlds() as tld (tld.tld)}
      <div class="row">
        <span class="tld-name">.{tld.tld}</span>
        <span>
          <span class="badge" class:badge-on={tld.resolverInstalled}>
            {tld.resolverInstalled ? 'Installed' : 'Missing'}
          </span>
        </span>
        <span class="actions">
          <button class="btn btn-ghost btn-sm text-danger" onclick={() => handleDelete(tld.tld)}>Delete</button>
        </span>
      </div>
    {:else}
      <div class="empty">No TLDs configured.</div>
    {/each}
  </div>
</div>

{#if showForm}
  <TLDForm onClose={() => showForm = false} />
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
  .table {
    border: 1px solid var(--border);
    border-radius: var(--radius);
    overflow: hidden;
    background: var(--bg-card);
  }
  .table-header {
    display: grid;
    grid-template-columns: 1fr 1fr 60px;
    gap: 8px;
    padding: 8px 14px;
    background: var(--bg-tertiary);
    border-bottom: 1px solid var(--border);
    font-size: 11px;
    font-weight: 600;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }
  .row {
    display: grid;
    grid-template-columns: 1fr 1fr 60px;
    gap: 8px;
    padding: 8px 14px;
    align-items: center;
    border-bottom: 1px solid var(--border-subtle);
    font-size: 12px;
  }
  .row:last-child {
    border-bottom: none;
  }
  .tld-name {
    color: var(--text-accent);
    font-weight: 500;
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
  .actions {
    display: flex;
    gap: 2px;
  }
  .text-danger {
    color: var(--danger);
  }
  .empty {
    padding: 32px 14px;
    text-align: center;
    color: var(--text-muted);
    font-size: 12px;
  }
</style>
