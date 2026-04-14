<script>
  import { onMount } from 'svelte';
  import { getRoutes, loadRoutes, loadTemplates } from '../../lib/stores/routes.svelte.js';
  import RouteRow from './RouteRow.svelte';
  import RouteForm from './RouteForm.svelte';

  let showForm = $state(false);
  let editingRoute = $state(null);

  onMount(() => { loadRoutes(); loadTemplates(); });

  function handleAdd() {
    editingRoute = null;
    showForm = true;
  }

  function handleEdit(route) {
    editingRoute = route;
    showForm = true;
  }

  function handleClose() {
    showForm = false;
    editingRoute = null;
    loadRoutes();
  }
</script>

<div class="page">
  <div class="page-header">
    <h1>Routes</h1>
    <button class="btn btn-primary btn-sm" onclick={handleAdd}>Add Route</button>
  </div>

  <div class="table">
    <div class="table-header">
      <span>Domain</span>
      <span>Upstream</span>
      <span>Flags</span>
      <span>Actions</span>
    </div>
    {#each getRoutes() as route (route.id)}
      <RouteRow {route} onEdit={() => handleEdit(route)} />
    {:else}
      <div class="empty">No routes configured.</div>
    {/each}
  </div>
</div>

{#if showForm}
  <RouteForm route={editingRoute} onClose={handleClose} />
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
    grid-template-columns: 1.2fr 1fr 120px 80px;
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
  .empty {
    padding: 32px 14px;
    text-align: center;
    color: var(--text-muted);
    font-size: 12px;
  }
</style>
