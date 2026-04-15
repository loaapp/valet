<script>
  import { addEntry } from '../../lib/stores/dns.svelte.js';

  let { tld, onClose } = $props();

  let subdomain = $state('');
  let target = $state('');
  let saving = $state(false);
  let error = $state('');
  let fieldError = $state('');

  function validateSubdomain(value) {
    if (!value) {
      return 'Subdomain is required.';
    }
    if (value.includes('.')) {
      return 'Only a single subdomain level is supported (e.g., "app", not "app.api"). This is a macOS resolver limitation.';
    }
    if (!/^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?$/.test(value)) {
      return 'Must be alphanumeric (hyphens allowed, not at start/end).';
    }
    return '';
  }

  async function handleSubmit(e) {
    e.preventDefault();
    const sub = subdomain.trim().toLowerCase();
    fieldError = validateSubdomain(sub);
    if (fieldError) {
      return;
    }
    saving = true;
    error = '';
    try {
      const domain = sub + '.' + tld;
      await addEntry(domain, tld, target.trim() || '127.0.0.1');
      onClose();
    } catch (err) {
      error = err?.message ?? 'Failed to register subdomain.';
    } finally {
      saving = false;
    }
  }
</script>

<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="overlay" onclick={onClose}>
  <div class="modal" onclick={(e) => e.stopPropagation()}>
    <h2>Register Subdomain</h2>
    <form onsubmit={handleSubmit}>
      <label class="field">
        <span class="label">Subdomain</span>
        <div class="domain-input" class:has-error={fieldError}>
          <input type="text" bind:value={subdomain} placeholder="myapp" oninput={() => { fieldError = ''; }} />
          <span class="suffix">.{tld}</span>
        </div>
        {#if fieldError}<span class="field-error">{fieldError}</span>{/if}
      </label>
      <label class="field">
        <span class="label">Target</span>
        <input type="text" class="input" bind:value={target} placeholder="127.0.0.1 or hostname" />
        <span class="field-hint">IP address for A record, or hostname for CNAME. Defaults to 127.0.0.1</span>
      </label>
      {#if error}
        <div class="error">{error}</div>
      {/if}
      <div class="actions">
        <button type="button" class="btn" onclick={onClose}>Cancel</button>
        <button type="submit" class="btn btn-primary" disabled={saving}>
          {saving ? 'Saving...' : 'Save'}
        </button>
      </div>
    </form>
  </div>
</div>

<style>
  .overlay {
    position: fixed;
    inset: 0;
    background: rgba(0,0,0,0.5);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 100;
  }
  .modal {
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    padding: 24px;
    width: 380px;
    box-shadow: 0 8px 32px rgba(0,0,0,0.4);
  }
  h2 {
    font-size: 15px;
    font-weight: 600;
    margin-bottom: 16px;
    color: var(--text-primary);
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: 4px;
    margin-bottom: 12px;
  }
  .label {
    font-size: 11px;
    font-weight: 500;
    color: var(--text-secondary);
  }
  .domain-input {
    display: flex;
    align-items: center;
    border-radius: var(--radius-sm);
    border: 1px solid var(--border);
    background: var(--bg-input);
    overflow: hidden;
  }
  .domain-input:focus-within {
    border-color: var(--accent);
  }
  .domain-input input {
    flex: 1;
    padding: 7px 4px 7px 10px;
    border: none;
    background: transparent;
    color: var(--text-primary);
    outline: none;
  }
  .suffix {
    padding: 7px 10px 7px 0;
    color: var(--text-muted);
    font-weight: 600;
    font-size: 13px;
  }
  .input {
    padding: 7px 10px;
    border-radius: var(--radius-sm);
    border: 1px solid var(--border);
    background: var(--bg-input);
    color: var(--text-primary);
    outline: none;
  }
  .input:focus {
    border-color: var(--accent);
  }
  .field-hint {
    font-size: 10px;
    color: var(--text-muted);
  }
  .field-error {
    font-size: 11px;
    color: var(--danger);
    margin-top: 2px;
  }
  .has-error {
    border-color: var(--danger) !important;
  }
  .error {
    color: var(--danger);
    font-size: 11px;
    margin-bottom: 12px;
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 16px;
  }
</style>
