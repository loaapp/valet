import { SendMessage, StopGeneration, ClearHistory, GetHistory } from '../../../wailsjs/go/api/AgentService.js';
import { EventsOn } from '../../../wailsjs/runtime/runtime.js';
import { getSettings, setSetting } from './settings.svelte.js';

let messages = $state([]);
let isGenerating = $state(false);
let streamingText = $state('');

// Model config defaults
let modelConfig = $state({
  baseURL: 'http://localhost:11434/v1',
  modelID: 'llama3.1',
  apiKey: '',
});

// Set up event listeners once
EventsOn('agent:token', (data) => {
  streamingText += data.text;
});

EventsOn('agent:toolcall', (data) => {
  messages = [...messages, {
    role: 'toolcall',
    toolName: data.name,
    toolArgs: data.args,
    toolResult: null,
    content: '',
  }];
});

EventsOn('agent:toolresult', (data) => {
  // Find the last tool call with this name and attach the result
  const updated = [...messages];
  for (let i = updated.length - 1; i >= 0; i--) {
    if (updated[i].role === 'toolcall' && updated[i].toolName === data.name && updated[i].toolResult === null) {
      updated[i] = { ...updated[i], toolResult: data.result };
      break;
    }
  }
  messages = updated;
});

EventsOn('agent:done', (data) => {
  const finalContent = data.content || streamingText;
  if (finalContent) {
    messages = [...messages, { role: 'assistant', content: finalContent }];
  }
  streamingText = '';
  isGenerating = false;
});

EventsOn('agent:error', (data) => {
  messages = [...messages, { role: 'error', content: data.error }];
  streamingText = '';
  isGenerating = false;
});

export function getMessages() { return messages; }
export function getIsGenerating() { return isGenerating; }
export function getStreamingText() { return streamingText; }
export function getModelConfig() { return modelConfig; }

export async function loadHistory() {
  try {
    const history = await GetHistory();
    if (history && history.length > 0) {
      messages = history.map(m => ({
        role: m.role,
        content: m.content || '',
        toolName: m.toolName || '',
        toolArgs: m.toolArgs || '',
        toolResult: m.toolResult || null,
      }));
    }
  } catch {
    // Ignore — history might not be available yet
  }
}

export function loadModelConfig() {
  const s = getSettings();
  modelConfig = {
    baseURL: s.model_base_url || 'http://localhost:11434/v1',
    modelID: s.model_id || 'llama3.1',
    apiKey: s.model_api_key || '',
  };
}

export async function setModelConfig(baseURL, modelID, apiKey) {
  modelConfig = { baseURL, modelID, apiKey };
  await setSetting('model_base_url', baseURL);
  await setSetting('model_id', modelID);
  await setSetting('model_api_key', apiKey);
}

function currentModelConfig() {
  // Always read fresh from settings in case they changed
  const s = getSettings();
  return {
    baseURL: s.model_base_url || modelConfig.baseURL,
    modelID: s.model_id || modelConfig.modelID,
    apiKey: s.model_api_key || modelConfig.apiKey,
  };
}

export async function sendMessage(text) {
  if (!text.trim() || isGenerating) return;

  messages = [...messages, { role: 'user', content: text }];
  isGenerating = true;
  streamingText = '';

  const cfg = currentModelConfig();
  try {
    await SendMessage(cfg.baseURL, cfg.modelID, cfg.apiKey, text);
  } catch (e) {
    messages = [...messages, { role: 'error', content: String(e) }];
    isGenerating = false;
  }
}

export function clearMessages() {
  messages = [];
  streamingText = '';
  try { ClearHistory(); } catch {}
}

export async function stopGeneration() {
  try {
    await StopGeneration();
  } catch (e) {
    console.error('Failed to stop generation:', e);
  }
}
