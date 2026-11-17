import { createApp } from 'vue'
import './style.css'
import App from './App.vue'
import { installPreloadErrorRecovery } from './utils/lazyComponent'

installPreloadErrorRecovery()

createApp(App).mount('#app')
