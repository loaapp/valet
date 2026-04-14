<script>
  import { onMount, onDestroy } from 'svelte';
  import { loadSettings } from './lib/stores/settings.svelte.js';
  import { startPolling, stopPolling } from './lib/stores/status.svelte.js';
  import Shell from './components/layout/Shell.svelte';

  onMount(() => {
    loadSettings();
    startPolling(5000);
  });

  onDestroy(() => {
    stopPolling();
  });
</script>

<Shell />
