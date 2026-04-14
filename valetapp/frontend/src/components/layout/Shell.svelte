<script>
  import Sidebar from './Sidebar.svelte';
  import Dashboard from '../dashboard/Dashboard.svelte';
  import RouteList from '../routes/RouteList.svelte';
  import TLDList from '../tlds/TLDList.svelte';
  import SettingsModal from '../settings/SettingsModal.svelte';

  let currentView = $state('dashboard');
  let settingsOpen = $state(false);

  function navigate(view) {
    currentView = view;
  }
</script>

<div class="shell">
  <Sidebar {currentView} onNavigate={navigate} onOpenSettings={() => settingsOpen = true} />
  <main class="content">
    {#if currentView === 'dashboard'}
      <Dashboard />
    {:else if currentView === 'routes'}
      <RouteList />
    {:else if currentView === 'tlds'}
      <TLDList />
    {/if}
  </main>
</div>

<SettingsModal bind:open={settingsOpen} />

<style>
  .shell {
    display: flex;
    height: 100%;
  }
  .content {
    flex: 1;
    overflow-y: auto;
    padding: 24px;
  }
</style>
