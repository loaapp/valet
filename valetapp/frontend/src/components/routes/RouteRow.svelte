<script>
  import { deleteRoute } from '../../lib/stores/routes.svelte.js';

  let { route, onEdit } = $props();

  async function handleDelete() {
    await deleteRoute(route.id);
  }

  function hasAdvancedConfig() {
    return (route.matchConfig && Object.keys(route.matchConfig).length > 0)
        || (route.handlerConfig && Object.keys(route.handlerConfig).length > 0);
  }
</script>

<div class="row">
  <div class="domain-cell">
    <span class="domain">{route.domain}</span>
    {#if route.description}
      <span class="description">{route.description}</span>
    {/if}
  </div>
  <span class="upstream">{route.upstream}</span>
  <span class="badges">
    {#if route.template}
      <span class="badge badge-template">{route.template}</span>
    {/if}
    {#if hasAdvancedConfig()}
      <span class="badge badge-advanced">Adv</span>
    {/if}
  </span>
  <span class="actions">
    <button class="btn btn-ghost btn-sm" onclick={onEdit}>Edit</button>
    <button class="btn btn-ghost btn-sm text-danger" onclick={handleDelete}>Del</button>
  </span>
</div>

<style>
  .row {
    display: grid;
    grid-template-columns: 1.2fr 1fr 120px 80px;
    gap: 8px;
    padding: 8px 14px;
    align-items: center;
    border-bottom: 1px solid var(--border-subtle);
    font-size: 12px;
  }
  .row:last-child {
    border-bottom: none;
  }
  .domain-cell {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }
  .domain {
    color: var(--text-accent);
    font-weight: 500;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .description {
    font-size: 10px;
    color: var(--text-muted);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .upstream {
    font-family: var(--font-mono);
    color: var(--text-secondary);
    font-size: 11px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .badges {
    display: flex;
    gap: 4px;
    flex-wrap: wrap;
  }
  .badge {
    display: inline-block;
    padding: 2px 6px;
    border-radius: 3px;
    font-size: 10px;
    font-weight: 600;
    background: var(--badge-bg);
    color: var(--badge-text);
    white-space: nowrap;
  }
  .badge-on {
    background: rgba(48,209,88,0.15);
    color: var(--success);
  }
  .badge-template {
    background: rgba(10,132,255,0.15);
    color: var(--accent);
  }
  .badge-advanced {
    background: rgba(255,159,10,0.15);
    color: #ff9f0a;
  }
  .actions {
    display: flex;
    gap: 2px;
  }
  .text-danger {
    color: var(--danger);
  }
</style>
