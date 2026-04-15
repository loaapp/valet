<script>
  import { getIsGenerating, sendMessage, stopGeneration } from '../../lib/stores/agent.svelte.js';

  let text = $state('');

  function handleSubmit(e) {
    e.preventDefault();
    if (getIsGenerating() || !text.trim()) return;
    sendMessage(text);
    text = '';
  }

  function handleKeydown(e) {
    if (e.key === 'Enter' && !e.shiftKey) {
      handleSubmit(e);
    }
  }
</script>

<form class="agent-input" onsubmit={handleSubmit}>
  <textarea
    bind:value={text}
    placeholder="Ask Valet..."
    rows="1"
    disabled={getIsGenerating()}
    onkeydown={handleKeydown}
  ></textarea>
  {#if getIsGenerating()}
    <button type="button" class="btn btn-stop" onclick={stopGeneration}>
      <svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor">
        <rect x="6" y="6" width="12" height="12" rx="2" />
      </svg>
      Stop
    </button>
  {:else}
    <button type="submit" class="btn btn-send" disabled={!text.trim()}>
      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <path d="M22 2L11 13M22 2l-7 20-4-9-9-4 20-7z" />
      </svg>
      Send
    </button>
  {/if}
</form>

<style>
  .agent-input {
    display: flex;
    gap: 8px;
    padding: 12px 16px;
    border-top: 1px solid var(--border);
    background: var(--bg-secondary);
    align-items: flex-end;
  }
  textarea {
    flex: 1;
    resize: none;
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    background: var(--bg-primary);
    color: var(--text-primary);
    padding: 8px 12px;
    font-size: 13px;
    font-family: inherit;
    line-height: 1.4;
    min-height: 36px;
    max-height: 120px;
  }
  textarea:focus {
    outline: none;
    border-color: var(--accent);
  }
  textarea:disabled {
    opacity: 0.5;
  }
  .btn {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 8px 12px;
    border: none;
    border-radius: var(--radius-sm);
    font-size: 12px;
    font-weight: 500;
    cursor: pointer;
    white-space: nowrap;
  }
  .btn-send {
    background: var(--accent);
    color: white;
  }
  .btn-send:disabled {
    opacity: 0.4;
    cursor: default;
  }
  .btn-stop {
    background: var(--danger);
    color: white;
  }
</style>
