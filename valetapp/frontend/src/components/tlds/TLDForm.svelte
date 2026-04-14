<script>
  import { addTld } from '../../lib/stores/tlds.svelte.js';

  let { onClose } = $props();

  let tld = $state('');
  let saving = $state(false);
  let error = $state('');
  let fieldError = $state('');

  function validateTLD(value) {
    if (!value) {
      return 'TLD name is required.';
    }
    if (!/^[a-zA-Z0-9]+$/.test(value)) {
      return 'TLD must be alphanumeric only (no dots, spaces, or special characters).';
    }
    return '';
  }

  async function handleSubmit(e) {
    e.preventDefault();
    const value = tld.trim().replace(/^\./, '');
    fieldError = validateTLD(value);
    if (fieldError) {
      return;
    }
    saving = true;
    error = '';
    fieldError = '';
    try {
      await addTld(value);
      onClose();
    } catch (err) {
      error = err?.message ?? 'Failed to create TLD.';
    } finally {
      saving = false;
    }
  }
</script>

<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="overlay" onclick={onClose}>
  <div class="modal" onclick={(e) => e.stopPropagation()}>
    <h2>Add TLD</h2>
    <form onsubmit={handleSubmit}>
      <label class="field">
        <span class="label">TLD Name</span>
        <div class="tld-input" class:has-error={fieldError}>
          <span class="prefix">.</span>
          <input type="text" bind:value={tld} placeholder="test" oninput={() => { fieldError = ''; }} />
        </div>
        {#if fieldError}<span class="field-error">{fieldError}</span>{/if}
      </label>
      <p class="hint">This will resolve all <code>*.{tld || 'tld'}</code> domains locally.</p>
      {#if error}
        <div class="error">{error}</div>
      {/if}
      <div class="actions">
        <button type="button" class="btn" onclick={onClose}>Cancel</button>
        <button type="submit" class="btn btn-primary" disabled={saving}>
          {saving ? 'Creating...' : 'Create'}
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
    width: 340px;
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
    margin-bottom: 8px;
  }
  .label {
    font-size: 11px;
    font-weight: 500;
    color: var(--text-secondary);
  }
  .tld-input {
    display: flex;
    align-items: center;
    border-radius: var(--radius-sm);
    border: 1px solid var(--border);
    background: var(--bg-input);
    overflow: hidden;
  }
  .tld-input:focus-within {
    border-color: var(--accent);
  }
  .prefix {
    padding: 7px 0 7px 10px;
    color: var(--text-muted);
    font-weight: 600;
  }
  .tld-input input {
    flex: 1;
    padding: 7px 10px 7px 2px;
    border: none;
    background: transparent;
    color: var(--text-primary);
    outline: none;
  }
  .hint {
    font-size: 11px;
    color: var(--text-muted);
    margin-bottom: 12px;
  }
  .hint code {
    font-family: var(--font-mono);
    background: var(--badge-bg);
    padding: 1px 4px;
    border-radius: 3px;
    font-size: 10px;
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
