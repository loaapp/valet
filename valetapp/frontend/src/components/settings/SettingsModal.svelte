<script>
  import { onMount } from 'svelte';
  import { getSettings, setSetting } from '../../lib/stores/settings.svelte.js';
  import { themes, fonts } from '../../lib/themes.js';
  import { GetDNSStatus } from '../../../wailsjs/go/api/StatusService.js';

  let { open = $bindable(false) } = $props();

  let tab = $state('appearance');
  const settings = $derived(getSettings());
  let dnsStatus = $state([]);

  function close() { open = false; }

  function saveModel(field, value) {
    setSetting(field, value);
  }

  async function loadDNSStatus() {
    try {
      dnsStatus = await GetDNSStatus();
    } catch {
      dnsStatus = [];
    }
  }

  $effect(() => {
    if (open && tab === 'dns') loadDNSStatus();
  });
</script>

{#if open}
  <div class="overlay" onclick={close} role="presentation">
    <div class="modal" onclick={(e) => e.stopPropagation()} role="dialog" tabindex="-1" onkeydown={(e) => e.key === 'Escape' && close()}>
      <div class="modal-header">
        <h2>Settings</h2>
        <button class="close-btn" onclick={close}>&times;</button>
      </div>

      <div class="modal-body">
        <nav class="tabs">
          <button class:active={tab === 'appearance'} onclick={() => tab = 'appearance'}>Appearance</button>
          <button class:active={tab === 'model'} onclick={() => tab = 'model'}>AI Model</button>
          <button class:active={tab === 'dns'} onclick={() => tab = 'dns'}>DNS</button>
        </nav>

        <div class="tab-content">
          {#if tab === 'appearance'}
          <div class="section">
            <h3>Theme</h3>
            <div class="theme-grid">
              {#each Object.entries(themes) as [id, theme] (id)}
                <button
                  class="theme-card"
                  class:active={(settings.theme || 'macos') === id}
                  onclick={() => setSetting('theme', id)}
                >
                  <div class="theme-preview" style="background: {theme.vars['--bg-primary']}; border-color: {theme.vars['--border']}">
                    <div class="preview-sidebar" style="background: {theme.vars['--bg-secondary']}"></div>
                    <div class="preview-content">
                      <div class="preview-bubble" style="background: {theme.vars['--user-bubble-bg']}"></div>
                      <div class="preview-bubble reply" style="background: {theme.vars['--assistant-bubble-bg']}"></div>
                    </div>
                  </div>
                  <span class="theme-name">{theme.name}</span>
                </button>
              {/each}
            </div>

            <h3>Font</h3>
            <div class="font-options">
              {#each Object.entries(fonts) as [id, font] (id)}
                <button
                  class="font-option"
                  class:active={(settings.font || 'system') === id}
                  onclick={() => setSetting('font', id)}
                >
                  <span style="font-family: {font.value}">{font.name}</span>
                </button>
              {/each}
            </div>
          </div>

          {:else if tab === 'model'}
          <div class="section">
            <h3>AI Model Configuration</h3>
            <p class="hint">Configure an OpenAI-compatible endpoint for the AI assistant. Works with Ollama, vLLM, LiteLLM, or any compatible API.</p>

            <div class="model-form">
              <label class="model-field">
                <span>Base URL</span>
                <input type="text"
                  value={settings.model_base_url || ''}
                  onchange={(e) => saveModel('model_base_url', e.target.value)}
                  placeholder="http://localhost:11434/v1"
                  spellcheck="false"
                />
              </label>
              <label class="model-field">
                <span>Model ID</span>
                <input type="text"
                  value={settings.model_id || ''}
                  onchange={(e) => saveModel('model_id', e.target.value)}
                  placeholder="llama3.1"
                  spellcheck="false"
                />
              </label>
              <label class="model-field">
                <span>API Key</span>
                <input type="password"
                  value={settings.model_api_key || ''}
                  onchange={(e) => saveModel('model_api_key', e.target.value)}
                  placeholder="Optional (not needed for Ollama)"
                  spellcheck="false"
                />
              </label>
            </div>

            <div class="presets">
              <span class="preset-label">Quick setup:</span>
              <button class="btn btn-sm" onclick={() => {
                saveModel('model_base_url', 'http://localhost:11434/v1');
                saveModel('model_id', 'llama3.1');
                saveModel('model_api_key', '');
              }}>Ollama (Llama 3.1)</button>
              <button class="btn btn-sm" onclick={() => {
                saveModel('model_base_url', 'http://localhost:11434/v1');
                saveModel('model_id', 'qwen2.5');
                saveModel('model_api_key', '');
              }}>Ollama (Qwen 2.5)</button>
            </div>
          </div>

          {:else if tab === 'dns'}
          <div class="section">
            <h3>DNS Resolver Status</h3>
            <p class="hint">Valet uses macOS resolver files (<code>/etc/resolver/</code>) to route DNS queries for managed TLDs to the local DNS server. This requires a one-time setup with sudo.</p>

            {#if dnsStatus.length === 0}
              <div class="dns-empty">
                <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" opacity="0.5">
                  <circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/>
                </svg>
                <p>No managed TLDs configured yet.</p>
                <p class="hint">Run <code>sudo valet tld add test</code> to register a TLD and install its DNS resolver in one step.</p>
              </div>
            {:else}
              <div class="dns-table">
                {#each dnsStatus as item}
                  <div class="dns-row">
                    <span class="dns-tld">.{item.tld}</span>
                    <span class="dns-status" class:dns-installed={item.installed}>
                      {item.installed ? 'Installed' : 'Not installed'}
                    </span>
                  </div>
                {/each}
              </div>
            {/if}

            <div class="dns-instructions">
              <h4>Managing TLDs</h4>
              <p class="hint">Add a TLD and install its DNS resolver in one step:</p>
              <code class="dns-command">sudo valet tld add test</code>
              <p class="hint" style="margin-top: 0.75rem">Remove a TLD and its resolver:</p>
              <code class="dns-command">sudo valet tld remove test</code>
              <p class="hint" style="margin-top: 0.75rem">Check status:</p>
              <code class="dns-command">valet tld list</code>
            </div>
          </div>
          {/if}
        </div>
      </div>
    </div>
  </div>
{/if}

<style>
  .overlay {
    position: fixed;
    inset: 0;
    background: rgba(0,0,0,0.6);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 100;
    backdrop-filter: blur(4px);
  }

  .modal {
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    width: min(640px, 90vw);
    max-height: 80vh;
    display: flex;
    flex-direction: column;
    box-shadow: 0 20px 60px rgba(0,0,0,0.3);
  }

  .modal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 1.25rem 1.5rem;
    border-bottom: 1px solid var(--border);
  }
  .modal-header h2 { margin: 0; font-size: 1.1rem; font-weight: 600; }
  .close-btn {
    background: none; border: none; color: var(--text-muted);
    font-size: 1.5rem; cursor: pointer; padding: 0; line-height: 1;
  }
  .close-btn:hover { color: var(--text-primary); }

  .modal-body { display: flex; flex-direction: column; overflow: hidden; }

  .tabs {
    display: flex;
    gap: 0;
    border-bottom: 1px solid var(--border);
    padding: 0 1.5rem;
  }
  .tabs button {
    background: none; border: none; border-bottom: 2px solid transparent;
    color: var(--text-muted); padding: 0.75rem 1rem; cursor: pointer;
    font-size: 0.85rem; font-weight: 500; font-family: var(--font-sans);
  }
  .tabs button:hover { color: var(--text-primary); }
  .tabs button.active { color: var(--accent); border-bottom-color: var(--accent); }

  .tab-content {
    padding: 1.25rem 1.5rem;
    overflow-y: auto;
    max-height: 55vh;
  }

  .section h3 { margin: 0 0 0.75rem; font-size: 0.95rem; font-weight: 600; color: var(--text-primary); }

  .theme-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: 0.75rem; margin-bottom: 1.5rem; }
  .theme-card {
    background: none; border: 2px solid var(--border); border-radius: var(--radius-sm);
    padding: 0.5rem; cursor: pointer; display: flex; flex-direction: column; align-items: center; gap: 0.4rem;
  }
  .theme-card:hover { border-color: var(--text-muted); }
  .theme-card.active { border-color: var(--accent); }
  .theme-preview {
    width: 100%; aspect-ratio: 16/10; border-radius: 4px; border: 1px solid;
    display: flex; overflow: hidden;
  }
  .preview-sidebar { width: 30%; }
  .preview-content { flex: 1; padding: 8%; display: flex; flex-direction: column; gap: 6%; }
  .preview-bubble { height: 20%; border-radius: 3px; width: 70%; align-self: flex-end; }
  .preview-bubble.reply { width: 80%; align-self: flex-start; }
  .theme-name { font-size: 0.75rem; color: var(--text-secondary); }

  .font-options { display: flex; gap: 0.5rem; }
  .font-option {
    background: var(--bg-tertiary); border: 2px solid var(--border); border-radius: var(--radius-sm);
    padding: 0.5rem 1rem; cursor: pointer; color: var(--text-primary); font-size: 0.85rem;
  }
  .font-option:hover { border-color: var(--text-muted); }
  .font-option.active { border-color: var(--accent); }

  .hint { font-size: 0.8rem; color: var(--text-muted); margin: 0 0 1rem; }

  .model-form { display: flex; flex-direction: column; gap: 0.75rem; margin-bottom: 1rem; }
  .model-field { display: flex; flex-direction: column; gap: 0.25rem; }
  .model-field span { font-size: 0.78rem; color: var(--text-secondary); font-weight: 500; }
  .model-field input {
    background: var(--bg-input); border: 1px solid var(--border); border-radius: var(--radius-sm);
    padding: 0.45rem 0.6rem; color: var(--text-primary); font-size: 0.85rem; outline: none;
  }
  .model-field input:focus { border-color: var(--accent); }

  .presets { display: flex; align-items: center; gap: 0.4rem; flex-wrap: wrap; }
  .preset-label { font-size: 0.78rem; color: var(--text-muted); }

  .dns-table { display: flex; flex-direction: column; gap: 0.4rem; margin-bottom: 1rem; }
  .dns-row { display: flex; align-items: center; justify-content: space-between; padding: 0.5rem 0.75rem; border: 1px solid var(--border-subtle); border-radius: var(--radius-sm); background: var(--bg-card); }
  .dns-tld { font-family: var(--font-mono); font-weight: 500; font-size: 0.85rem; }
  .dns-status { font-size: 0.78rem; color: var(--text-muted); }
  .dns-status.dns-installed { color: var(--success); }
  .dns-instructions { margin-top: 1rem; }
  .dns-instructions h4 { font-size: 0.85rem; font-weight: 600; margin: 0 0 0.5rem; color: var(--text-primary); }
  .dns-command { display: block; background: var(--bg-input); border: 1px solid var(--border); border-radius: var(--radius-sm); padding: 0.5rem 0.75rem; font-family: var(--font-mono); font-size: 0.8rem; color: var(--text-primary); user-select: all; }
  .dns-empty { text-align: center; padding: 1.5rem 1rem; border: 1px dashed var(--border); border-radius: var(--radius); margin-bottom: 1rem; }
  .dns-empty p { margin: 0.5rem 0 0; font-size: 0.85rem; color: var(--text-secondary); }
  .dns-empty code { background: var(--bg-input); padding: 1px 5px; border-radius: 3px; font-size: 0.8rem; }
</style>
