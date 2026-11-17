import { createApp } from 'vue'
import './style.css'
import App from './App.vue'

// Auto-recover when an already-open tab requests a chunk whose hash changed after a new deployment
window.addEventListener('vite:preloadError', (event) => {
  event.preventDefault()
  const now = Date.now()
  const last = Number(sessionStorage.getItem('bothub_preload_reload_at') || '0')
  if (now - last > 10_000) {
    sessionStorage.setItem('bothub_preload_reload_at', String(now))
    window.location.reload()
  }
})

createApp(App).mount('#app')
