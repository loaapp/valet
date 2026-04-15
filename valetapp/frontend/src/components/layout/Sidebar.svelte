<script>
  import { isConnected } from '../../lib/stores/status.svelte.js';

  let { currentView, onNavigate, onOpenSettings } = $props();

  const navItems = [
    { id: 'dashboard', label: 'Dashboard', icon: 'grid' },
    { id: 'routes', label: 'Routes', icon: 'route' },
    { id: 'tlds', label: 'TLDs', icon: 'globe' },
    { id: 'assistant', label: 'Assistant', icon: 'chat' },
    { id: 'logs', label: 'Logs', icon: 'logs' },
  ];

  const icons = {
    grid: 'M4 4h6v6H4V4zm10 0h6v6h-6V4zM4 14h6v6H4v-6zm10 0h6v6h-6v-6z',
    route: 'M13 3v18M3 8l4-4 4 4M17 16l4 4-4 4',
    globe: 'M12 2a10 10 0 100 20 10 10 0 000-20zm0 2c1.66 0 3.14 2.69 3.5 6.5H8.5C8.86 6.69 10.34 4 12 4zm0 16c-1.66 0-3.14-2.69-3.5-6.5h7c-.36 3.81-1.84 6.5-3.5 6.5zM2.05 13h4.46c.1 1.38.35 2.66.72 3.78A10.03 10.03 0 012.05 13zm0-2a10.03 10.03 0 015.18-3.78A14.8 14.8 0 006.51 11H2.05zm19.9 0h-4.46c-.1-1.38-.35-2.66-.72-3.78A10.03 10.03 0 0121.95 11zm0 2a10.03 10.03 0 01-5.18 3.78c.37-1.12.62-2.4.72-3.78h4.46z',
    chat: 'M21 15a2 2 0 01-2 2H7l-4 4V5a2 2 0 012-2h14a2 2 0 012 2z',
    logs: 'M4 6h16M4 10h16M4 14h10M4 18h6',
  };
</script>

<aside class="sidebar">
  <div class="titlebar" style="--wails-draggable: drag;">
    <div class="app-title">
      <span class="dot" class:connected={isConnected()} class:disconnected={!isConnected()}></span>
      <span>Valet</span>
    </div>
  </div>
  <nav class="nav">
    {#each navItems as item}
      <button
        class="nav-item"
        class:active={currentView === item.id}
        onclick={() => onNavigate(item.id)}
      >
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
          <path d={icons[item.icon]} />
        </svg>
        <span>{item.label}</span>
      </button>
    {/each}
  </nav>
  <div class="sidebar-footer">
    <button class="nav-item" onclick={onOpenSettings}>
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
        <path d="M12.22 2h-.44a2 2 0 00-2 2v.18a2 2 0 01-1 1.73l-.43.25a2 2 0 01-2 0l-.15-.08a2 2 0 00-2.73.73l-.22.38a2 2 0 00.73 2.73l.15.1a2 2 0 011 1.72v.51a2 2 0 01-1 1.74l-.15.09a2 2 0 00-.73 2.73l.22.38a2 2 0 002.73.73l.15-.08a2 2 0 012 0l.43.25a2 2 0 011 1.73V20a2 2 0 002 2h.44a2 2 0 002-2v-.18a2 2 0 011-1.73l.43-.25a2 2 0 012 0l.15.08a2 2 0 002.73-.73l.22-.39a2 2 0 00-.73-2.73l-.15-.08a2 2 0 01-1-1.74v-.5a2 2 0 011-1.74l.15-.09a2 2 0 00.73-2.73l-.22-.38a2 2 0 00-2.73-.73l-.15.08a2 2 0 01-2 0l-.43-.25a2 2 0 01-1-1.73V4a2 2 0 00-2-2z" />
        <circle cx="12" cy="12" r="3" />
      </svg>
      <span>Settings</span>
    </button>
  </div>
</aside>

<style>
  .sidebar {
    width: 180px;
    height: 100%;
    background: var(--sidebar-bg);
    backdrop-filter: var(--sidebar-vibrancy);
    border-right: 1px solid var(--border);
    display: flex;
    flex-direction: column;
    flex-shrink: 0;
  }
  .titlebar {
    height: 52px;
    display: flex;
    align-items: center;
    padding: 0 16px;
    padding-top: 8px;
  }
  .app-title {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 13px;
    font-weight: 600;
    color: var(--text-primary);
  }
  .dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
  }
  .dot.connected {
    background: var(--success);
    box-shadow: 0 0 6px var(--success);
  }
  .dot.disconnected {
    background: var(--danger);
    box-shadow: 0 0 6px var(--danger);
  }
  .nav {
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: 8px;
  }
  .nav-item {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 7px 10px;
    border: none;
    border-radius: var(--radius-sm);
    background: transparent;
    color: var(--text-secondary);
    font-size: 12px;
    font-weight: 500;
    text-align: left;
    transition: background 0.15s, color 0.15s;
  }
  .nav-item:hover {
    background: var(--bg-hover);
    color: var(--text-primary);
  }
  .nav-item.active {
    background: var(--bg-active);
    color: var(--text-primary);
  }
  .sidebar-footer {
    margin-top: auto;
    padding: 8px;
    border-top: 1px solid var(--border-subtle);
  }
</style>
