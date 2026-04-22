<script>
  import { addRoute, updateRoute, getRoutes, getTemplates, previewRoute } from '../../lib/stores/routes.svelte.js';
  import { getTlds } from '../../lib/stores/tlds.svelte.js';
  import { validateRoute, validateField } from '../../lib/validation.js';
  import { PickDirectory } from '../../../wailsjs/go/api/DialogService.js';

  let { route = null, onClose } = $props();

  const _route = $state.snapshot(route);
  const editId = _route?.id ?? null;
  const isEdit = !!editId;

  // --- State ---
  let domain = $state(_route?.domain ?? '');
  let upstream = $state(_route?.upstream ?? '');
  let description = $state(_route?.description ?? '');
  let tlsUpstream = $state(_route?.tlsUpstream ?? false);
  const originalTemplate = _route?.template || 'simple';
  let selectedTemplate = $state(originalTemplate);
  let templateParams = $state(inferTemplateParams(_route));

  // Advanced options
  function parseJson(str) {
    if (!str) return null;
    try { return JSON.parse(str); } catch { return null; }
  }
  const parsedMatch = parseJson(_route?.matchConfig);
  const parsedHandler = parseJson(_route?.handlerConfig);
  const hasExistingAdvanced = !!(_route?.matchConfig || _route?.handlerConfig);
  let showAdvanced = $state(hasExistingAdvanced);
  let pathRules = $state(parsedMatch?.path ?? ['']);
  let corsEnabled = $state(!!parsedHandler?.cors);
  let corsAllowOrigin = $state(parsedHandler?.cors?.allowOrigin ?? '*');
  let compressionEnabled = $state(!!parsedHandler?.compression);
  let stripPathPrefix = $state(parsedHandler?.stripPathPrefix ?? '');
  let customHeaders = $state(parsedHandler?.responseHeaders ?? [{ key: '', value: '' }]);

  let previewJson = $state('');
  let showPreview = $state(false);
  let copied = $state(false);
  let saving = $state(false);
  let error = $state('');
  let fieldErrors = $state({});

  // Reconstruct template params from stored handlerConfig/matchConfig on edit.
  function inferTemplateParams(route) {
    if (!route) return {};
    if (route.templateParams && Object.keys(route.templateParams).length > 0) {
      return { ...route.templateParams };
    }

    const handler = parseJson(route.handlerConfig);
    const match = parseJson(route.matchConfig);
    const params = {};

    if (route.template === 'static' && Array.isArray(handler)) {
      const fs = handler.find(h => h.handler === 'file_server');
      if (fs) {
        params.root = fs.root || '';
        params.browse = fs.browse != null ? 'true' : 'false';
      }
    } else if (route.template === 'custom') {
      params.handlerJson = route.handlerConfig || '';
      params.matchJson = route.matchConfig || '';
    } else if (route.template === 'spa-api' && Array.isArray(handler)) {
      const subroute = handler.find(h => h.handler === 'subroute');
      if (subroute?.routes) {
        const apiRoute = subroute.routes.find(r => r.match);
        if (apiRoute?.handle?.[0]?.upstreams?.[0]?.dial) {
          params.apiUpstream = apiRoute.handle[0].upstreams[0].dial;
        }
        if (apiRoute?.match?.[0]?.path?.[0]) {
          params.apiPath = apiRoute.match[0].path[0];
        }
        const fallback = subroute.routes.find(r => !r.match);
        if (fallback?.handle?.[0]?.upstreams?.[0]?.dial) {
          params.frontendUpstream = fallback.handle[0].upstreams[0].dial;
        }
      }
    } else if (route.template === 'cors-proxy' && Array.isArray(handler)) {
      const headers = handler.find(h => h.handler === 'headers');
      if (headers?.response?.set?.['Access-Control-Allow-Origin']?.[0]) {
        params.allowOrigin = headers.response.set['Access-Control-Allow-Origin'][0];
      }
    } else if (route.template === 'multi-upstream' && Array.isArray(handler)) {
      const rp = handler.find(h => h.handler === 'reverse_proxy');
      if (rp?.upstreams) {
        params.upstreams = rp.upstreams.map(u => u.dial).join(',');
      }
    }

    return params;
  }

  // --- Domain autocomplete ---
  let showSuggestions = $state(false);
  let domainFocused = $state(false);

  function getDomainSuggestions() {
    const typed = domain.trim().toLowerCase();
    if (!typed) return [];

    const tlds = getTlds();
    const existingDomains = new Set(getRoutes().map(r => r.domain));

    // If user already typed a dot, don't suggest
    if (typed.includes('.')) return [];

    return tlds
      .map(t => `${typed}.${t.tld}`)
      .filter(d => !existingDomains.has(d));
  }

  function selectSuggestion(suggestion) {
    domain = suggestion;
    showSuggestions = false;
  }

  function handleDomainInput() {
    clearFieldError('domain');
    showSuggestions = domainFocused && getDomainSuggestions().length > 0;
  }

  function handleDomainFocus() {
    domainFocused = true;
    if (domain.trim() && !domain.includes('.')) {
      showSuggestions = getDomainSuggestions().length > 0;
    }
  }

  function handleDomainBlur() {
    // Delay to allow click on suggestion
    setTimeout(() => {
      domainFocused = false;
      showSuggestions = false;
      validateOnBlur('domain', domain.trim());
    }, 150);
  }

  // --- Template logic ---
  function getSelectedTemplateObj() {
    return getTemplates().find(t => t.slug === selectedTemplate);
  }

  const noUpstreamTemplates = new Set(['static', 'custom']);

  function needsUpstream() {
    return !noUpstreamTemplates.has(selectedTemplate);
  }

  function onTemplateChange() {
    templateParams = {};
    const tpl = getSelectedTemplateObj();
    if (tpl?.params) {
      tpl.params.forEach(p => { templateParams[p.key] = ''; });
    }
  }

  async function pickDirectory(paramKey) {
    try {
      const dir = await PickDirectory('Select directory');
      if (dir) {
        templateParams = { ...templateParams, [paramKey]: dir };
      }
    } catch (e) {
      // User cancelled
    }
  }

  // --- Validation ---
  function clearFieldError(field) {
    if (fieldErrors[field]) {
      fieldErrors = { ...fieldErrors, [field]: undefined };
    }
  }

  function validateOnBlur(field, value) {
    const err = validateField(field, value, !needsUpstream());
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

  // --- Build & Submit ---
  function buildRequest() {
    const req = {
      domain: domain.trim(),
      upstream: needsUpstream() ? upstream.trim() : '',
      tls: true,
      tlsUpstream,
      description: description.trim(),
    };

    const templateIsNew = !isEdit || selectedTemplate !== originalTemplate;
    if (selectedTemplate && selectedTemplate !== 'simple' && templateIsNew) {
      // Only send template on create or when switching templates.
      // On edit with the same template, the config is already stored.
      req.template = selectedTemplate;
      req.templateParams = { ...templateParams };
    }

    const filteredPaths = pathRules.filter(p => p.trim());
    if (filteredPaths.length > 0) {
      req.matchConfig = { path: filteredPaths };
    }

    const hc = {};
    let hasHandler = false;
    if (corsEnabled) { hc.cors = { allowOrigin: corsAllowOrigin.trim() || '*' }; hasHandler = true; }
    if (compressionEnabled) { hc.compression = { encodings: ['gzip', 'zstd'] }; hasHandler = true; }
    if (stripPathPrefix.trim()) { hc.stripPathPrefix = stripPathPrefix.trim(); hasHandler = true; }
    const filteredHeaders = customHeaders.filter(h => h.key.trim());
    if (filteredHeaders.length > 0) {
      hc.responseHeaders = filteredHeaders.map(h => ({ key: h.key.trim(), value: h.value.trim() }));
      hasHandler = true;
    }
    if (hasHandler) req.handlerConfig = hc;

    return req;
  }

  async function handleSubmit(e) {
    e.preventDefault();
    fieldErrors = {};
    error = '';

    const useTemplate = !needsUpstream();
    const { valid, errors } = validateRoute(
      { domain: domain.trim(), upstream: upstream.trim(), description: description.trim() },
      useTemplate
    );

    if (!valid) {
      fieldErrors = errors;
      return;
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

  function addPathRule() { pathRules = [...pathRules, '']; }
  function removePathRule(i) { pathRules = pathRules.filter((_, idx) => idx !== i); }
  function addHeader() { customHeaders = [...customHeaders, { key: '', value: '' }]; }
  function removeHeader(i) { customHeaders = customHeaders.filter((_, idx) => idx !== i); }
</script>

<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="overlay" onclick={onClose}>
  <div class="modal" onclick={(e) => e.stopPropagation()}>
    <h2>{isEdit ? 'Edit Route' : 'Add Route'}</h2>
    <form onsubmit={handleSubmit}>

      <!-- Domain with autocomplete -->
      <div class="field domain-field">
        <span class="label">Domain</span>
        <input
          type="text"
          bind:value={domain}
          placeholder="myapp"
          class:has-error={fieldErrors.domain}
          oninput={handleDomainInput}
          onfocus={handleDomainFocus}
          onblur={handleDomainBlur}
        />
        {#if showSuggestions}
          <div class="suggestions">
            {#each getDomainSuggestions() as suggestion}
              <button type="button" class="suggestion" onmousedown={() => selectSuggestion(suggestion)}>
                {suggestion}
              </button>
            {/each}
          </div>
        {/if}
        {#if fieldErrors.domain}<span class="field-error">{fieldErrors.domain}</span>{/if}
      </div>

      <!-- Description -->
      <label class="field">
        <span class="label">Description</span>
        <input type="text" bind:value={description} placeholder="What this route is for" />
      </label>

      <!-- Template type selector -->
      <div class="field">
        <span class="label">Type</span>
        <div class="template-selector">
          {#each getTemplates() as tpl}
            <button
              type="button"
              class="template-chip"
              class:active={selectedTemplate === tpl.slug}
              onclick={() => { selectedTemplate = tpl.slug; onTemplateChange(); }}
            >
              {tpl.name}
            </button>
          {/each}
        </div>
      </div>

      <!-- Template-specific fields -->
      {#if needsUpstream()}
        <label class="field">
          <span class="label">Upstream</span>
          <input type="text" bind:value={upstream} placeholder="localhost:3000" class:has-error={fieldErrors.upstream} oninput={() => clearFieldError('upstream')} onblur={handleUpstreamBlur} />
          {#if fieldErrors.upstream}<span class="field-error">{fieldErrors.upstream}</span>{/if}
        </label>
        <label class="field checkbox-field">
          <input type="checkbox" bind:checked={tlsUpstream} />
          <span>Backend uses SSL</span>
        </label>
      {/if}

      {#if getSelectedTemplateObj()?.params?.length}
        <div class="template-params">
          {#each getSelectedTemplateObj().params as param}
            <label class="field">
              <span class="label">{param.label}{#if param.required} *{/if}</span>
              {#if param.key === 'root'}
                <div class="input-with-button">
                  <input type="text" bind:value={templateParams[param.key]} placeholder={param.placeholder ?? ''} />
                  <button type="button" class="btn btn-sm" onclick={() => pickDirectory(param.key)}>Browse</button>
                </div>
              {:else if param.key === 'handlerJson' || param.key === 'matchJson'}
                <textarea
                  class="json-editor"
                  bind:value={templateParams[param.key]}
                  placeholder={param.placeholder ?? ''}
                  rows="4"
                  spellcheck="false"
                ></textarea>
              {:else}
                <input type="text" bind:value={templateParams[param.key]} placeholder={param.placeholder ?? ''} />
              {/if}
            </label>
          {/each}
        </div>
      {/if}

      <!-- Advanced options -->
      <div class="advanced-section">
        <button type="button" class="advanced-toggle" onclick={() => showAdvanced = !showAdvanced}>
          {showAdvanced ? 'Hide' : 'Show'} advanced options
        </button>
        {#if showAdvanced}
          <div class="advanced-body">
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
            <div class="advanced-group">
              <label class="field checkbox-field">
                <input type="checkbox" bind:checked={compressionEnabled} />
                <span>Enable Compression (gzip + zstd)</span>
              </label>
            </div>
            <div class="advanced-group">
              <label class="field">
                <span class="label">Strip Path Prefix</span>
                <input type="text" bind:value={stripPathPrefix} placeholder="/api" />
              </label>
            </div>
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

  /* Domain autocomplete */
  .domain-field {
    position: relative;
  }
  .suggestions {
    position: absolute;
    top: 100%;
    left: 0;
    right: 0;
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-top: none;
    border-radius: 0 0 var(--radius-sm) var(--radius-sm);
    z-index: 10;
    box-shadow: 0 4px 12px rgba(0,0,0,0.2);
    max-height: 150px;
    overflow-y: auto;
  }
  .suggestion {
    display: block;
    width: 100%;
    padding: 7px 10px;
    border: none;
    background: none;
    text-align: left;
    font-size: 12px;
    color: var(--text-primary);
    cursor: pointer;
    font-family: var(--font-mono);
  }
  .suggestion:hover {
    background: var(--bg-hover);
  }

  /* Template selector chips */
  .template-selector {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }
  .template-chip {
    padding: 5px 12px;
    border: 1px solid var(--border);
    border-radius: 14px;
    background: transparent;
    color: var(--text-secondary);
    font-size: 11px;
    font-weight: 500;
    cursor: pointer;
    transition: all 0.15s;
    font-family: var(--font-sans);
  }
  .template-chip:hover {
    border-color: var(--accent);
    color: var(--text-primary);
  }
  .template-chip.active {
    background: var(--accent);
    border-color: var(--accent);
    color: #fff;
  }

  /* Template params */
  .template-params {
    padding: 12px;
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    background: var(--bg-tertiary);
    margin-bottom: 12px;
  }
  .template-params .field:last-child {
    margin-bottom: 0;
  }
  .input-with-button {
    display: flex;
    gap: 6px;
    align-items: center;
  }
  .input-with-button input {
    flex: 1;
  }
  .json-editor {
    font-family: var(--font-mono);
    font-size: 11px;
    padding: 8px 10px;
    border-radius: var(--radius-sm);
    border: 1px solid var(--border);
    background: var(--bg-input);
    color: var(--text-primary);
    outline: none;
    resize: vertical;
    min-height: 60px;
    line-height: 1.4;
  }
  .json-editor:focus {
    border-color: var(--accent);
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

  .field-error {
    font-size: 11px;
    color: var(--danger);
    margin-top: 2px;
  }
  .has-error {
    border-color: var(--danger) !important;
  }
  .btn-xs {
    font-size: 10px;
    padding: 2px 6px;
  }
</style>
