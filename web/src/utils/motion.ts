/**
 * Directional spatial tab & panel transition helper.
 * Keeps motion strictly on compositor properties (opacity, transform).
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
