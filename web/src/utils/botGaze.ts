// Lightweight zero-layout-thrash pointer gaze & typing-attention coordinator for GrokBot avatars.
// Uses viewport-normalized coordinates updated at most once per animation frame (no getBoundingClientRect calls).

export function clampUnit(value: number): number {
  if (!Number.isFinite(value)) return 0
  return Math.max(-1, Math.min(1, value))
}

export function computeNormalizedGaze(
  clientX: number,
  clientY: number,
  viewportWidth: number,
  viewportHeight: number
): { gx: number; gy: number } {
  if (viewportWidth <= 0 || viewportHeight <= 0) {
    return { gx: 0, gy: 0 }
  }
  const nx = (clientX / viewportWidth) * 2 - 1
  const ny = (clientY / viewportHeight) * 2 - 1
  return {
    gx: Number(clampUnit(nx).toFixed(3)),
    gy: Number(clampUnit(ny).toFixed(3)),
  }
}

let subscribers = 0
let rafId: number | null = null
let pendingX = 0
let pendingY = 0
let listeningTimer: number | null = null

function flushPointerGaze() {
  rafId = null
  if (typeof window === 'undefined' || typeof document === 'undefined') return
  const root = document.documentElement
  if (root.dataset.ambientPaused === 'true') return
  const { gx, gy } = computeNormalizedGaze(
    pendingX,
    pendingY,
    window.innerWidth,
    window.innerHeight
  )
  root.style.setProperty('--bot-global-gx', String(gx))
  root.style.setProperty('--bot-global-gy', String(gy))
}

function onWindowPointerMove(e: PointerEvent) {
  if (e.pointerType === 'touch') return
  pendingX = e.clientX
  pendingY = e.clientY
  if (rafId === null && typeof window !== 'undefined') {
    rafId = window.requestAnimationFrame(flushPointerGaze)
  }
}

function onWindowPointerLeave() {
  if (typeof document === 'undefined') return
  const root = document.documentElement
  root.style.setProperty('--bot-global-gx', '0')
  root.style.setProperty('--bot-global-gy', '0')
}

export function retainGlobalBotGaze(): () => void {
  if (typeof window === 'undefined') return () => {}
  if (window.matchMedia?.('(prefers-reduced-motion: reduce)').matches) {
    return () => {}
  }
  subscribers += 1
  if (subscribers === 1) {
    window.addEventListener('pointermove', onWindowPointerMove, { passive: true })
    document.addEventListener('mouseleave', onWindowPointerLeave, { passive: true })
  }
  return () => {
    subscribers = Math.max(0, subscribers - 1)
    if (subscribers === 0) {
      window.removeEventListener('pointermove', onWindowPointerMove)
      document.removeEventListener('mouseleave', onWindowPointerLeave)
      if (rafId !== null) {
        window.cancelAnimationFrame(rafId)
        rafId = null
      }
    }
  }
}

// Signals active GrokBot avatars to perk up attentively while the user types in the chat composer.
export function pulseBotListening(durationMs = 900) {
  if (typeof window === 'undefined' || typeof document === 'undefined') return
  const root = document.documentElement
  root.dataset.botListening = 'true'
  if (listeningTimer !== null) {
    window.clearTimeout(listeningTimer)
  }
  listeningTimer = window.setTimeout(() => {
    delete root.dataset.botListening
    listeningTimer = null
  }, durationMs)
}
