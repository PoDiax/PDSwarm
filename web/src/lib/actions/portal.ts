/**
 * Svelte action to teleport a DOM element to document.body (or specified target).
 * This ensures modals, tooltips, and overlays escape ancestor containing blocks
 * created by CSS properties like backdrop-filter, transform, filter, or overflow.
 */
export function portal(node: HTMLElement, target: HTMLElement | string = 'body') {
  function mount() {
    if (typeof document === 'undefined') return;
    const targetEl = typeof target === 'string' ? document.querySelector(target) : target;
    if (targetEl && node.parentNode !== targetEl) {
      targetEl.appendChild(node);
    }
  }

  mount();

  return {
    update(newTarget: HTMLElement | string) {
      target = newTarget;
      mount();
    },
    destroy() {
      if (node.parentNode) {
        node.parentNode.removeChild(node);
      }
    },
  };
}
