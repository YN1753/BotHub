/**
 * Top-layer exit transition detection & IME-safe submit helpers
 * Aligned with modern-web-guidance best practices.
 */

let supportsDisplayTransition: boolean | undefined

export function canTransitionDisplay(): boolean {
  if (supportsDisplayTransition !== undefined) return supportsDisplayTransition
  if (typeof window === 'undefined' || !window.CSS?.supports?.('transition-behavior', 'allow-discrete') || !document.body) {
    return false
  }
  const probe = document.createElement('div')
  probe.style.cssText = 'transition: display 1s allow-discrete !important; display: block;'
  document.body.appendChild(probe)
  getComputedStyle(probe).display
  probe.style.display = 'none'
  supportsDisplayTransition = getComputedStyle(probe).display === 'block'
  probe.remove()
  return supportsDisplayTransition
}

export function supportsTopLayerExit(): boolean {
  return typeof window !== 'undefined' && Boolean(window.CSS?.supports?.('overlay', 'auto')) && canTransitionDisplay()
}

export async function closeDialogWithTransition(dialog: HTMLDialogElement): Promise<void> {
  if (!dialog.open) return
  if (!supportsTopLayerExit()) {
    dialog.setAttribute('data-closing', '')
    const animations = dialog.getAnimations ? dialog.getAnimations({ subtree: true }) : []
    if (animations.length > 0) {
      await Promise.race([
        Promise.allSettled(animations.map((a) => a.finished)),
        new Promise((resolve) => setTimeout(resolve, 600)),
      ])
    }
    if (!dialog.hasAttribute('data-closing')) return
    dialog.removeAttribute('data-closing')
  }
  if (dialog.open) {
    dialog.close()
  }
}

export function isPointOutsideRect(clientX: number, clientY: number, rect: DOMRect): boolean {
  return clientX < rect.left || clientX > rect.right || clientY < rect.top || clientY > rect.bottom
}

export function isImeComposingEnter(event: KeyboardEvent, lastCompositionEndAt: number | null): boolean {
  if (event.isComposing || event.keyCode === 229) return true
  if (lastCompositionEndAt !== null && Math.abs(event.timeStamp - lastCompositionEndAt) < 50) {
    return true
  }
  return false
}
