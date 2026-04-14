<script>
  import { addRoute, updateRoute, getTemplates, previewRoute } from '../../lib/stores/routes.svelte.js';
  import { validateRoute, validateField } from '../../lib/validation.js';

  let { route = null, onClose } = $props();

  // Form is remounted for each edit, so capturing initial values is intentional.
  const _route = $state.snapshot(route);
  const editId = _route?.id ?? null;
  const isEdit = !!editId;

  // Parse JSON config strings from the route (they come as strings from the API)
  function parseJson(str) {
    if (!str) return null;
    try { return JSON.parse(str); } catch { return null; }
  }
  const parsedMatch = parseJson(_route?.matchConfig);
  const parsedHandler = parseJson(_route?.handlerConfig);

  // Simple fields
  let domain = $state(_route?.domain ?? '');
  let upstream = $state(_route?.upstream ?? '');
  let tls = $state(_route?.tlsEnabled ?? false);
  let description = $state(_route?.description ?? '');

  // Mode: 'simple' | 'template' | (simple with advanced expanded)
  let selectedTemplate = $state(_route?.template ?? '');
  let templateParams = $state({});
  let showTemplatePicker = $state(false);

  // Advanced options — auto-expand if route has advanced config
  const hasExistingAdvanced = !!(_route?.matchConfig || _route?.handlerConfig);
  let showAdvanced = $state(hasExistingAdvanced);
  let pathRules = $state(parsedMatch?.path ?? ['']);
  let corsEnabled = $state(!!parsedHandler?.cors);
  let corsAllowOrigin = $state(parsedHandler?.cors?.allowOrigin ?? '*');
  let compressionEnabled = $state(!!parsedHandler?.compression);
  let stripPathPrefix = $state(parsedHandler?.stripPathPrefix ?? '');
  let customHeaders = $state(parsedHandler?.responseHeaders ?? [{ key: '', value: '' }]);

  // Preview
  let previewJson = $state('');
  let showPreview = $state(false);
  let copied = $state(false);

  let saving = $state(false);
  let error = $state('');
  let fieldErrors = $state({});

  // Initialize template params from existing route
  if (_route?.templateParams) {
    templateParams = { ..._route.templateParams };
  }

  function buildRequest() {
    const req = {
      domain: domain.trim(),
      upstream: upstream.trim(),
      tls,
      description: description.trim(),
    };

    if (selectedTemplate) {
      req.template = selectedTemplate;
      req.templateParams = { ...templateParams };
    }

    // Build matchConfig from advanced options
    const filteredPaths = pathRules.filter(p => p.trim());
    if (filteredPaths.length > 0) {
      req.matchConfig = { path: filteredPaths };
    }

    // Build handlerConfig from advanced options
    const hc = {};
    let hasHandler = false;

    if (corsEnabled) {
      hc.cors = { allowOrigin: corsAllowOrigin.trim() || '*' };
      hasHandler = true;
    }
    if (compressionEnabled) {
      hc.compression = { encodings: ['gzip', 'zstd'] };
      hasHandler = true;
    }
    if (stripPathPrefix.trim()) {
      hc.stripPathPrefix = stripPathPrefix.trim();
      hasHandler = true;
    }
    const filteredHeaders = customHeaders.filter(h => h.key.trim());
    if (filteredHeaders.length > 0) {
      hc.responseHeaders = filteredHeaders.map(h => ({ key: h.key.trim(), value: h.value.trim() }));
      hasHandler = true;
    }

    if (hasHandler) {
      req.handlerConfig = hc;
    }

    return req;
  }

  function clearFieldError(field) {
    if (fieldErrors[field]) {
      fieldErrors = { ...fieldErrors, [field]: undefined };
    }
  }

  function validateOnBlur(field, value) {
    const err = validateField(field, value, !!selectedTemplate);
    if (err) {
      fieldErrors = { ...fieldErrors, [field]: err };
    } else {
      clearFieldError(field);
    }
  }

  function handleUpstreamBlur() {
    upstream = upstream.replace(/^https?:\/\//i, '');
    validateOnBlur('upstream', upstream);
  }

  async function handleSubmit(e) {
    e.preventDefault();
    fieldErrors = {};
    error = '';

    const useTemplate = !!selectedTemplate;
    const { valid, data: validatedData, errors } = validateRoute(
      { domain: domain.trim(), upstream: upstream.trim(), description: description.trim() },
      useTemplate
    );

    if (!valid) {
      fieldErrors = errors;
      return;
    }

    // Apply normalized upstream from validation (protocol stripped)
    if (validatedData.upstream) {
      upstream = validatedData.upstream;
    }

    saving = true;
    try {
      const req = buildRequest();
      if (isEdit) {
        await updateRoute(editId, req);
      } else {
        await addRoute(req);
      }
      onClose();
    } catch (err) {
      error = typeof err === 'string' ? err : (err?.message ?? 'Failed to save route.');
    } finally {
      saving = false;
    }
  }

  function selectTemplate(tpl) {
    selectedTemplate = tpl.slug;
    templateParams = {};
    (tpl.params || []).forEach(p => {
      templateParams[p.key] = '';
    });
    showTemplatePicker = false;
  }

  function clearTemplate() {
    selectedTemplate = '';
    templateParams = {};
    showTemplatePicker = false;
  }

  function addPathRule() {
    pathRules = [...pathRules, ''];
  }

  function removePathRule(index) {
    pathRules = pathRules.filter((_, i) => i !== index);
  }

  function addHeader() {
    customHeaders = [...customHeaders, { key: '', value: '' }];
  }

  function removeHeader(index) {
    customHeaders = customHeaders.filter((_, i) => i !== index);
  }

  async function handlePreview() {
    try {
      const req = buildRequest();
      const json = await previewRoute(req);
      previewJson = JSON.stringify(JSON.parse(json), null, 2);
      showPreview = true;
    } catch (err) {
      previewJson = 'Error: ' + (err?.message ?? 'Failed to generate preview.');
      showPreview = true;
    }
  }

  function getSelectedTemplateObj() {
    return getTemplates().find(t => t.slug === selectedTemplate);
  }
</script>

<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="overlay" onclick={onClose}>
  <div class="modal" onclick={(e) => e.stopPropagation()}>
    <h2>{isEdit ? 'Edit Route' : 'Add Route'}</h2>
    <form onsubmit={handleSubmit}>
      <label class="field">
        <span class="label">Domain</span>
        <input type="text" bind:value={domain} placeholder="myapp.test" class:has-error={fieldErrors.domain} oninput={() => clearFieldError('domain')} onblur={() => validateOnBlur('domain', domain.trim())} />
        {#if fieldErrors.domain}<span class="field-error">{fieldErrors.domain}</span>{/if}
      </label>

      <label class="field">
        <span class="label">Upstream</span>
        <input type="text" bind:value={upstream} placeholder="localhost:3000" class:has-error={fieldErrors.upstream} oninput={() => clearFieldError('upstream')} onblur={handleUpstreamBlur} />
        {#if fieldErrors.upstream}<span class="field-error">{fieldErrors.upstream}</span>{/if}
      </label>

      <label class="field checkbox-field">
        <input type="checkbox" bind:checked={tls} />
        <span>Enable TLS</span>
      </label>

      <label class="field">
        <span class="label">Description</span>
        <input type="text" bind:value={description} placeholder="Short description of this route" />
      </label>

      <!-- Template section -->
      {#if selectedTemplate}
        {@const tplObj = getSelectedTemplateObj()}
        <div class="template-selected">
          <div class="template-selected-header">
            <span class="template-badge">{tplObj?.name ?? selectedTemplate}</span>
            <button type="button" class="btn btn-ghost btn-xs" onclick={clearTemplate}>Clear</button>
          </div>
          {#if tplObj?.params?.length}
            <div class="template-params">
              {#each tplObj.params as param}
                <label class="field">
                  <span class="label">{param.label}{#if param.required} *{/if}</span>
                  <input
                    type="text"
                    bind:value={templateParams[param.key]}
                    placeholder={param.placeholder ?? ''}
                  />
                </label>
              {/each}
            </div>
          {/if}
        </div>
      {:else if showTemplatePicker}
        <div class="template-picker">
          <div class="template-picker-header">
            <span class="label">Choose a Template</span>
            <button type="button" class="btn btn-ghost btn-xs" onclick={() => showTemplatePicker = false}>Cancel</button>
          </div>
          <div class="template-grid">
            {#each getTemplates() as tpl}
              <button type="button" class="template-card" onclick={() => selectTemplate(tpl)}>
                <span class="template-card-name">{tpl.name}</span>
                <span class="template-card-desc">{tpl.description}</span>
              </button>
            {/each}
            {#if getTemplates().length === 0}
              <div class="empty-templates">No templates available.</div>
            {/if}
          </div>
        </div>
      {:else}
        <button type="button" class="btn btn-ghost btn-sm use-template-btn" onclick={() => showTemplatePicker = true}>
          Use Template
        </button>
      {/if}

      <!-- Advanced options -->
      <div class="advanced-section">
        <button type="button" class="advanced-toggle" onclick={() => showAdvanced = !showAdvanced}>
          {showAdvanced ? 'Hide' : 'Show'} advanced options
        </button>
        {#if showAdvanced}
          <div class="advanced-body">
            <!-- Path rules -->
            <div class="advanced-group">
              <span class="label">Path Rules</span>
              {#each pathRules as _rule, i}
                <div class="row-input">
                  <input type="text" bind:value={pathRules[i]} placeholder="/api/*" />
                  <button type="button" class="btn btn-ghost btn-xs" onclick={() => removePathRule(i)}>x</button>
                </div>
              {/each}
              <button type="button" class="btn btn-ghost btn-xs" onclick={addPathRule}>+ Add path</button>
            </div>

            <!-- CORS -->
            <div class="advanced-group">
              <label class="field checkbox-field">
                <input type="checkbox" bind:checked={corsEnabled} />
                <span>Enable CORS</span>
              </label>
              {#if corsEnabled}
                <label class="field">
                  <span class="label">Allow Origin</span>
                  <input type="text" bind:value={corsAllowOrigin} placeholder="*" />
                </label>
              {/if}
            </div>

            <!-- Compression -->
            <div class="advanced-group">
              <label class="field checkbox-field">
                <input type="checkbox" bind:checked={compressionEnabled} />
                <span>Enable Compression (gzip + zstd)</span>
              </label>
            </div>

            <!-- Strip path prefix -->
            <div class="advanced-group">
              <label class="field">
                <span class="label">Strip Path Prefix</span>
                <input type="text" bind:value={stripPathPrefix} placeholder="/api" />
              </label>
            </div>

            <!-- Custom response headers -->
            <div class="advanced-group">
              <span class="label">Custom Response Headers</span>
              {#each customHeaders as _header, i}
                <div class="row-input header-row">
                  <input type="text" bind:value={customHeaders[i].key} placeholder="Header-Name" />
                  <input type="text" bind:value={customHeaders[i].value} placeholder="value" />
                  <button type="button" class="btn btn-ghost btn-xs" onclick={() => removeHeader(i)}>x</button>
                </div>
              {/each}
              <button type="button" class="btn btn-ghost btn-xs" onclick={addHeader}>+ Add header</button>
            </div>
          </div>
        {/if}
      </div>

      {#if error}
        <div class="error">{error}</div>
      {/if}

      <!-- Preview -->
      {#if showPreview}
        <div class="preview-section">
          <div class="preview-header">
            <span class="label">Caddy JSON Preview</span>
            <div class="preview-actions">
              <button type="button" class="btn btn-ghost btn-xs" onclick={() => {
                navigator.clipboard.writeText(previewJson);
                copied = true;
                setTimeout(() => copied = false, 2000);
              }}>{copied ? 'Copied!' : 'Copy'}</button>
              <button type="button" class="btn btn-ghost btn-xs" onclick={() => showPreview = false}>Close</button>
            </div>
          </div>
          <pre class="preview-code">{previewJson}</pre>
        </div>
      {/if}

      <div class="actions">
        <button type="button" class="btn btn-ghost btn-sm" onclick={handlePreview}>Preview</button>
        <div class="actions-right">
          <button type="button" class="btn" onclick={onClose}>Cancel</button>
          <button type="submit" class="btn btn-primary" disabled={saving}>
            {saving ? 'Saving...' : isEdit ? 'Update' : 'Create'}
          </button>
        </div>
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
    width: 520px;
    max-height: 85vh;
    overflow-y: auto;
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
  input[type="text"] {
    padding: 7px 10px;
    border-radius: var(--radius-sm);
    border: 1px solid var(--border);
    background: var(--bg-input);
    color: var(--text-primary);
    outline: none;
    font-size: 12px;
  }
  input[type="text"]:focus {
    border-color: var(--accent);
  }
  .checkbox-field {
    flex-direction: row;
    align-items: center;
    gap: 8px;
    font-size: 12px;
    color: var(--text-secondary);
  }
  .error {
    color: var(--danger);
    font-size: 11px;
    margin-bottom: 12px;
  }
  .actions {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 8px;
    margin-top: 16px;
  }
  .actions-right {
    display: flex;
    gap: 8px;
  }

  /* Template picker */
  .use-template-btn {
    margin-bottom: 12px;
    color: var(--accent);
  }
  .template-picker {
    margin-bottom: 12px;
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    padding: 12px;
    background: var(--bg-tertiary);
  }
  .template-picker-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 10px;
  }
  .template-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 8px;
  }
  .template-card {
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 10px;
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    background: var(--bg-card);
    cursor: pointer;
    text-align: left;
    transition: border-color 0.15s;
  }
  .template-card:hover {
    border-color: var(--accent);
  }
  .template-card-name {
    font-size: 12px;
    font-weight: 600;
    color: var(--text-primary);
  }
  .template-card-desc {
    font-size: 10px;
    color: var(--text-muted);
    line-height: 1.3;
  }
  .empty-templates {
    grid-column: 1 / -1;
    text-align: center;
    color: var(--text-muted);
    font-size: 11px;
    padding: 12px;
  }

  /* Template selected */
  .template-selected {
    margin-bottom: 12px;
    border: 1px solid var(--accent);
    border-radius: var(--radius-sm);
    padding: 12px;
    background: var(--bg-tertiary);
  }
  .template-selected-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 8px;
  }
  .template-badge {
    display: inline-block;
    padding: 2px 8px;
    border-radius: 3px;
    font-size: 11px;
    font-weight: 600;
    background: rgba(10, 132, 255, 0.15);
    color: var(--accent);
  }
  .template-params {
    padding-top: 4px;
  }

  /* Advanced section */
  .advanced-section {
    margin-top: 4px;
    margin-bottom: 8px;
  }
  .advanced-toggle {
    background: none;
    border: none;
    padding: 0;
    color: var(--text-muted);
    font-size: 11px;
    cursor: pointer;
    text-decoration: underline;
    text-underline-offset: 2px;
  }
  .advanced-toggle:hover {
    color: var(--text-secondary);
  }
  .advanced-body {
    margin-top: 12px;
    padding: 12px;
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    background: var(--bg-tertiary);
    animation: slideDown 0.15s ease-out;
  }
  @keyframes slideDown {
    from { opacity: 0; transform: translateY(-4px); }
    to { opacity: 1; transform: translateY(0); }
  }
  .advanced-group {
    margin-bottom: 14px;
  }
  .advanced-group:last-child {
    margin-bottom: 0;
  }
  .row-input {
    display: flex;
    gap: 6px;
    align-items: center;
    margin-bottom: 6px;
  }
  .row-input input[type="text"] {
    flex: 1;
  }
  .header-row input[type="text"]:first-child {
    flex: 0.8;
  }
  .header-row input[type="text"]:nth-child(2) {
    flex: 1.2;
  }

  /* Preview */
  .preview-section {
    margin-top: 12px;
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    background: var(--bg-tertiary);
    padding: 10px;
  }
  .preview-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 8px;
  }
  .preview-actions {
    display: flex;
    gap: 4px;
  }
  .preview-code {
    font-family: var(--font-mono);
    font-size: 10px;
    color: var(--text-secondary);
    max-height: 200px;
    overflow-y: auto;
    white-space: pre-wrap;
    word-break: break-all;
    margin: 0;
    line-height: 1.4;
  }

  /* Field validation errors */
  .field-error {
    font-size: 11px;
    color: var(--danger);
    margin-top: 2px;
  }
  .has-error {
    border-color: var(--danger) !important;
  }

  /* Utility */
  .btn-xs {
    font-size: 10px;
    padding: 2px 6px;
  }
</style>
