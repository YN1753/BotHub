<script setup lang="ts">
import { ref, watch, onMounted, onBeforeUnmount } from 'vue'
import { useAppStore } from '../stores/useAppStore'

const store = useAppStore()
const canvasRef = ref<HTMLCanvasElement | null>(null)
let rafId = 0

onMounted(() => {
  const canvas = canvasRef.value
  if (!canvas) return
  const ctx = canvas.getContext('2d')
  if (!ctx) return

  interface Star {
    x: number
    y: number
    r: number
    baseA: number
    phase: number
    vx: number
    vy: number
  }
  const stars: Star[] = []
  let w = 0
  let h = 0
  let hidden = document.hidden
  let lastFrameAt = 0
  const mql =
    typeof window.matchMedia === 'function'
      ? window.matchMedia('(prefers-reduced-motion: reduce)')
      : null
  let reducedMotion = Boolean(mql?.matches)

  const shouldAnimate = () =>
    !hidden &&
    !reducedMotion &&
    !store.state.ambientMotionPaused &&
    !document.querySelector('dialog[open]')

  const resize = () => {
    const dpr = Math.min(window.devicePixelRatio || 1, 2)
    w = window.innerWidth
    h = window.innerHeight
    canvas.width = Math.floor(w * dpr)
    canvas.height = Math.floor(h * dpr)
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
  }

  const initStars = () => {
    stars.length = 0
    for (let i = 0; i < 78; i++) {
      stars.push({
        x: Math.random() * w,
        y: Math.random() * h,
        r: Math.random() * 1.35 + 0.3,
        baseA: Math.random() * 0.48 + 0.14,
        phase: Math.random() * Math.PI * 2,
        vx: (Math.random() - 0.5) * 4.5,
        vy: (Math.random() - 0.5) * 4.5,
      })
    }
  }

  const renderFrame = (dtSec: number, nowMs: number) => {
    ctx.clearRect(0, 0, w, h)
    for (const s of stars) {
      if (dtSec > 0) {
        s.x += s.vx * dtSec
        s.y += s.vy * dtSec
        if (s.x < 0) s.x = w
        if (s.x > w) s.x = 0
        if (s.y < 0) s.y = h
        if (s.y > h) s.y = 0
      }
      const twinkle = dtSec > 0 ? Math.sin(nowMs * 0.0009 + s.phase) * 0.09 : 0
      const alpha = Math.max(0.08, Math.min(0.75, s.baseA + twinkle))
      ctx.beginPath()
      ctx.arc(s.x, s.y, s.r, 0, Math.PI * 2)
      ctx.fillStyle = `rgba(255, 238, 214, ${alpha})`
      ctx.fill()
    }
  }

  // Throttled ~24fps delta-timed drift cuts background GPU wakeups by >60%
  const FRAME_INTERVAL_MS = 42

  const draw = (nowMs: number) => {
    if (!shouldAnimate()) {
      rafId = 0
      lastFrameAt = 0
      return
    }
    if (!lastFrameAt) {
      lastFrameAt = nowMs
      renderFrame(0.042, nowMs)
    } else if (nowMs - lastFrameAt >= FRAME_INTERVAL_MS) {
      const dtSec = Math.min((nowMs - lastFrameAt) / 1000, 0.1)
      lastFrameAt = nowMs
      renderFrame(dtSec, nowMs)
    }
    rafId = window.requestAnimationFrame(draw)
  }

  const syncLoop = () => {
    if (shouldAnimate()) {
      if (!rafId) {
        lastFrameAt = 0
        rafId = window.requestAnimationFrame(draw)
      }
    } else {
      if (rafId) {
        window.cancelAnimationFrame(rafId)
        rafId = 0
      }
      renderFrame(0, 0)
    }
  }

  const onVisibilityChange = () => {
    hidden = document.hidden
    syncLoop()
  }

  const onMotionPrefChange = (e: MediaQueryListEvent) => {
    reducedMotion = e.matches
    syncLoop()
  }

  const onResize = () => {
    resize()
    renderFrame(0, 0)
  }

  const stopWatch = watch(
    () => store.state.ambientMotionPaused,
    () => syncLoop()
  )

  resize()
  initStars()
  renderFrame(0, 0)
  syncLoop()

  window.addEventListener('resize', onResize)
  document.addEventListener('visibilitychange', onVisibilityChange)
  mql?.addEventListener?.('change', onMotionPrefChange)

  onBeforeUnmount(() => {
    stopWatch()
    window.cancelAnimationFrame(rafId)
    window.removeEventListener('resize', onResize)
    document.removeEventListener('visibilitychange', onVisibilityChange)
    mql?.removeEventListener?.('change', onMotionPrefChange)
  })
})
</script>

<template>
  <canvas ref="canvasRef" aria-hidden="true" class="fixed inset-0 z-0 pointer-events-none opacity-80" />
</template>
