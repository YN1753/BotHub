<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount } from 'vue'

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
    a: number
    vx: number
    vy: number
  }
  const stars: Star[] = []
  let w = 0
  let h = 0

  const resize = () => {
    w = window.innerWidth
    h = window.innerHeight
    canvas.width = w
    canvas.height = h
  }

  const initStars = () => {
    stars.length = 0
    for (let i = 0; i < 85; i++) {
      stars.push({
        x: Math.random() * w,
        y: Math.random() * h,
        r: Math.random() * 1.4 + 0.3,
        a: Math.random() * 0.55 + 0.15,
        vx: (Math.random() - 0.5) * 0.08,
        vy: (Math.random() - 0.5) * 0.08,
      })
    }
  }

  const draw = () => {
    ctx.clearRect(0, 0, w, h)
    for (const s of stars) {
      s.x += s.vx
      s.y += s.vy
      if (s.x < 0) s.x = w
      if (s.x > w) s.x = 0
      if (s.y < 0) s.y = h
      if (s.y > h) s.y = 0
      ctx.beginPath()
      ctx.arc(s.x, s.y, s.r, 0, Math.PI * 2)
      ctx.fillStyle = `rgba(255, 238, 214, ${s.a})`
      ctx.fill()
    }
    rafId = window.requestAnimationFrame(draw)
  }

  resize()
  initStars()
  draw()
  window.addEventListener('resize', resize)

  onBeforeUnmount(() => {
    window.cancelAnimationFrame(rafId)
    window.removeEventListener('resize', resize)
  })
})
</script>

<template>
  <canvas ref="canvasRef" aria-hidden="true" class="fixed inset-0 z-0 pointer-events-none opacity-80" />
</template>
