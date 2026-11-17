/**
 * Directional spatial transition & View Transitions API helpers.
 * Keeps motion strictly on compositor properties (opacity, transform) and
 * degrades cleanly when prefers-reduced-motion is active or unsupported.
 */

export type DirectionalTransitionName = 'vesper-slide-forward' | 'vesper-slide-backward'

export function resolveTabSlideTransition(
  nextId: string,
  prevId: string,
  orderedIds: readonly string[]
): DirectionalTransitionName {
  const nextIdx = orderedIds.indexOf(nextId)
  const prevIdx = orderedIds.indexOf(prevId)
  if (nextIdx >= 0 && prevIdx >= 0 && nextIdx < prevIdx) {
    return 'vesper-slide-backward'
  }
  return 'vesper-slide-forward'
}

export function prefersReducedMotion(): boolean {
  return (
    typeof window !== 'undefined' &&
    typeof window.matchMedia === 'function' &&
    window.matchMedia('(prefers-reduced-motion: reduce)').matches
  )
}

export function runViewTransition(updateDOM: () => void | Promise<void>, direction?: 'forward' | 'backward'): void {
  if (
    typeof document === 'undefined' ||
    typeof document.startViewTransition !== 'function' ||
    prefersReducedMotion()
  ) {
    void updateDOM()
    return
  }

  try {
    if (direction) {
      ;(document as any).startViewTransition({
        update: updateDOM,
        types: [direction],
      })
    } else {
      document.startViewTransition(() => updateDOM())
    }
  } catch {
    void updateDOM()
  }
}
