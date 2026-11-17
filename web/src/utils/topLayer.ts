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

export async function closePopoverWithTransition(popover: HTMLElement | null): Promise<void> {
  if (!popover || typeof popover.hidePopover !== 'function') return
  const isOpen = typeof popover.matches === 'function' && popover.matches(':popover-open')
  if (!isOpen) return
  if (!supportsTopLayerExit()) {
    popover.setAttribute('data-closing', '')
    const animations = popover.getAnimations ? popover.getAnimations({ subtree: true }) : []
    if (animations.length > 0) {
      await Promise.race([
        Promise.allSettled(animations.map((a) => a.finished)),
        new Promise((resolve) => setTimeout(resolve, 400)),
      ])
    }
    if (!popover.hasAttribute('data-closing')) return
    popover.removeAttribute('data-closing')
  }
  if (typeof popover.matches === 'function' && popover.matches(':popover-open')) {
    popover.hidePopover()
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

export function applySiblingStaggerFallback(container: HTMLElement | null): void {
  if (!container || typeof window === 'undefined') return
  if (window.CSS?.supports?.('animation-delay', 'calc(sibling-index() * 0.1s)')) return
  const children = Array.from(container.children) as HTMLElement[]
  children.forEach((el, idx) => {
    el.style.setProperty('--sibling-index', String(idx))
  })
}

export function trackSpotlight(e: PointerEvent): void {
  if (e.pointerType !== 'mouse') return
  const el = e.currentTarget as HTMLElement | null
  if (!el) return
  const rect = el.getBoundingClientRect()
  el.style.setProperty('--mx', `${e.clientX - rect.left}px`)
  el.style.setProperty('--my', `${e.clientY - rect.top}px`)
}
