<script>
  let {
    title = 'Confirm',
    message = 'Are you sure?',
    confirmText = 'Confirm',
    cancelText = 'Cancel',
    danger = false,
    onconfirm,
    oncancel,
  } = $props();

  function handleConfirm() {
    onconfirm?.();
  }

  function handleCancel() {
    oncancel?.();
  }

  function handleKeydown(e) {
    if (e.key === 'Escape') {
      handleCancel();
    }
  }
</script>

<svelte:window onkeydown={handleKeydown} />

<div
  class="modal-backdrop"
  onclick={handleCancel}
  role="presentation"
>
  <div
    class="modal min-w-[340px] max-w-[90%]"
    onclick={(e) => e.stopPropagation()}
    role="dialog"
    aria-modal="true"
  >
    <div class="flex items-center gap-2 mb-4" style="border-bottom: 0.5px solid var(--border); padding-bottom: 10px;">
      <span style="font-size: 10px; font-weight: 700; letter-spacing: 0.12em; text-transform: uppercase; color: var(--text-muted);">{title}</span>
    </div>
    <p style="font-size: 12px; color: var(--text-muted); margin: 0 0 16px; line-height: 1.6;">{message}</p>
    <div class="flex justify-end gap-2">
      <button class="btn btn-sm btn-secondary" onclick={handleCancel}>{cancelText}</button>
      <button
        class="btn btn-sm {danger ? 'btn-danger' : 'btn-primary'}"
        onclick={handleConfirm}
      >
        {confirmText}
      </button>
    </div>
  </div>
</div>
