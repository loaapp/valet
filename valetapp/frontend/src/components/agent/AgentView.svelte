<script>
  import { onMount, tick } from 'svelte';
  import { getMessages, getIsGenerating, getStreamingText, loadModelConfig, clearMessages } from '../../lib/stores/agent.svelte.js';
  import { renderMarkdown } from '../../lib/markdown.js';
  import AgentInput from './AgentInput.svelte';

  let messagesEl;

  onMount(() => {
    loadModelConfig();
  });

  async function scrollToBottom() {
    await tick();
    if (messagesEl) {
      messagesEl.scrollTop = messagesEl.scrollHeight;
    }
  }

  // Auto-scroll when messages or streaming text change
  $effect(() => {
    getMessages();
    getStreamingText();
    scrollToBottom();
  });

  let toolExpanded = $state({});

  function toggleTool(index) {
    toolExpanded = { ...toolExpanded, [index]: !toolExpanded[index] };
  }
</script>

<div class="agent-view">
  {#if getMessages().length > 0}
    <div class="agent-header">
      <span class="agent-title">Assistant</span>
      <button class="btn btn-ghost btn-sm" onclick={clearMessages}>Clear</button>
    </div>
  {/if}
  <div class="messages" bind:this={messagesEl}>
    {#if getMessages().length === 0 && !getIsGenerating()}
      <div class="empty-state">
        <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" opacity="0.3">
          <path d="M21 15a2 2 0 01-2 2H7l-4 4V5a2 2 0 012-2h14a2 2 0 012 2z" />
        </svg>
        <p>Ask Valet to manage routes, TLDs, or certificates.</p>
      </div>
    {/if}

    {#each getMessages() as msg, i}
      {#if msg.role === 'user'}
        <div class="message user-message">
          <div class="bubble user-bubble">{msg.content}</div>
        </div>
      {:else if msg.role === 'assistant'}
        <div class="message assistant-message">
          <div class="bubble assistant-bubble markdown">{@html renderMarkdown(msg.content)}</div>
        </div>
      {:else if msg.role === 'toolcall'}
        <div class="message tool-message">
          <button class="tool-header" onclick={() => toggleTool(i)}>
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class:rotated={toolExpanded[i]}>
              <path d="M9 18l6-6-6-6" />
            </svg>
            <span class="tool-name">{msg.toolName}</span>
            {#if msg.toolResult !== null}
              <span class="tool-done">done</span>
            {:else}
              <span class="tool-running">running...</span>
            {/if}
          </button>
          {#if toolExpanded[i]}
            <div class="tool-detail">
              {#if msg.toolArgs}
                <div class="tool-section">
                  <div class="tool-label">Arguments</div>
                  <pre class="tool-code">{msg.toolArgs}</pre>
                </div>
              {/if}
              {#if msg.toolResult !== null}
                <div class="tool-section">
                  <div class="tool-label">Result</div>
                  <pre class="tool-code">{msg.toolResult}</pre>
                </div>
              {/if}
            </div>
          {/if}
        </div>
      {:else if msg.role === 'error'}
        <div class="message error-message">
          <div class="bubble error-bubble">{msg.content}</div>
        </div>
      {/if}
    {/each}

    {#if getStreamingText()}
      <div class="message assistant-message">
        <div class="bubble assistant-bubble streaming markdown">{@html renderMarkdown(getStreamingText())}</div>
      </div>
    {/if}
  </div>

  <AgentInput />
</div>

<style>
  .agent-view {
    display: flex;
    flex-direction: column;
    height: 100%;
  }
  .agent-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 8px 16px;
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
  }
  .agent-title {
    font-size: 12px;
    font-weight: 500;
    color: var(--text-secondary);
  }
  .messages {
    flex: 1;
    overflow-y: auto;
    padding: 16px;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .empty-state {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 8px;
    color: var(--text-tertiary);
    font-size: 13px;
  }
  .message {
    display: flex;
  }
  .user-message {
    justify-content: flex-end;
  }
  .assistant-message {
    justify-content: flex-start;
  }
  .bubble {
    max-width: 80%;
    padding: 8px 12px;
    border-radius: var(--radius-sm);
    font-size: 13px;
    line-height: 1.5;
    white-space: pre-wrap;
    word-break: break-word;
  }
  .user-bubble {
    background: var(--accent);
    color: white;
    border-bottom-right-radius: 2px;
  }
  .assistant-bubble {
    background: var(--bg-hover);
    color: var(--text-primary);
    border-bottom-left-radius: 2px;
  }
  .streaming {
    opacity: 0.9;
  }
  .markdown :global(p) { margin: 0 0 0.5em; }
  .markdown :global(p:last-child) { margin-bottom: 0; }
  .markdown :global(code) { font-family: var(--font-mono); font-size: 0.9em; background: rgba(0,0,0,0.15); padding: 1px 4px; border-radius: 3px; }
  .markdown :global(pre) { background: rgba(0,0,0,0.2); padding: 8px 10px; border-radius: var(--radius-sm); overflow-x: auto; margin: 0.5em 0; }
  .markdown :global(pre code) { background: none; padding: 0; font-size: 0.85em; }
  .markdown :global(ul), .markdown :global(ol) { margin: 0.5em 0; padding-left: 1.5em; }
  .markdown :global(li) { margin: 0.2em 0; }
  .markdown :global(h1), .markdown :global(h2), .markdown :global(h3) { margin: 0.5em 0 0.3em; font-size: 1em; font-weight: 600; }
  .markdown :global(strong) { font-weight: 600; }
  .markdown :global(a) { color: var(--text-accent); text-decoration: none; }
  .markdown :global(a:hover) { text-decoration: underline; }
  .error-bubble {
    background: var(--danger-bg, rgba(239, 68, 68, 0.1));
    color: var(--danger);
    border: 1px solid var(--danger);
  }
  .tool-message {
    flex-direction: column;
    width: 100%;
  }
  .tool-header {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 4px 8px;
    background: transparent;
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-sm);
    color: var(--text-secondary);
    font-size: 12px;
    cursor: pointer;
    width: fit-content;
  }
  .tool-header:hover {
    background: var(--bg-hover);
  }
  .tool-header svg {
    transition: transform 0.15s;
  }
  .tool-header svg.rotated {
    transform: rotate(90deg);
  }
  .tool-name {
    font-weight: 500;
    font-family: var(--font-mono, monospace);
  }
  .tool-done {
    color: var(--success);
    font-size: 11px;
  }
  .tool-running {
    color: var(--warning, #f59e0b);
    font-size: 11px;
  }
  .tool-detail {
    margin-top: 4px;
    margin-left: 20px;
  }
  .tool-section {
    margin-bottom: 4px;
  }
  .tool-label {
    font-size: 11px;
    color: var(--text-tertiary);
    margin-bottom: 2px;
  }
  .tool-code {
    background: var(--bg-tertiary, var(--bg-secondary));
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-sm);
    padding: 6px 8px;
    font-size: 11px;
    font-family: var(--font-mono, monospace);
    overflow-x: auto;
    max-height: 150px;
    overflow-y: auto;
    margin: 0;
    white-space: pre-wrap;
    word-break: break-all;
  }
</style>
