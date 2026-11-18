import { createApp } from 'vue'
import './style.css'
import App from './App.vue'
import { installPreloadErrorRecovery } from './utils/lazyComponent'
import { initThemeSync } from './utils/theme'

initThemeSync()
installPreloadErrorRecovery()

createApp(App).mount('#app')
