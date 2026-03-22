<script>
  import { onMount } from 'svelte';

  let { message = '', type = 'info', onclose } = $props();

  let visible = $state(false);

  onMount(() => {
    requestAnimationFrame(() => {
      visible = true;
    });
  });

  function handleClose() {
    visible = false;
    setTimeout(() => {
      onclose?.();
    }, 150);
  }

  const colorMap = {
    info:    'var(--teal)',
    success: 'var(--success)',
    error:   'var(--danger)',
    warning: 'var(--amber)',
  };

  let accentColor = $derived(colorMap[type] || colorMap.info);
</script>

<div
  class="flex items-center gap-3 min-w-[240px] max-w-[380px] transition-all duration-150"
  class:opacity-0={!visible}
  class:translate-x-4={!visible}
  class:opacity-100={visible}
  class:translate-x-0={visible}
  style="background-color: var(--surface); border: 0.5px solid var(--border); border-left: 2px solid {accentColor}; border-radius: 2px; padding: 8px 12px;"
>
  <span style="flex: 1; font-size: 11px; color: var(--text);">{message}</span>
  <button
    style="background: transparent; border: none; font-size: 16px; line-height: 1; padding: 0; cursor: pointer; color: var(--text-muted);"
    onmouseover={(e) => e.currentTarget.style.color = 'var(--text)'}
    onmouseout={(e) => e.currentTarget.style.color = 'var(--text-muted)'}
    onclick={handleClose}
  >×</button>
</div>
