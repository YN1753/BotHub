import { defineAsyncComponent, h, type AsyncComponentLoader, type Component } from 'vue'

const PRELOAD_RELOAD_KEY = 'bothub_preload_reload_at'
let memoryLastReloadAt = 0

export function shouldReloadOnPreloadError(nowMs: number, lastReloadMs: number, minIntervalMs = 10_000): boolean {
  return nowMs - lastReloadMs > minIntervalMs
}

export function installPreloadErrorRecovery(targetWindow: Window = window): void {
  targetWindow.addEventListener('vite:preloadError', (event) => {
    event.preventDefault()
    const now = Date.now()
    let last = memoryLastReloadAt
    try {
      const stored = Number(targetWindow.sessionStorage?.getItem(PRELOAD_RELOAD_KEY) || '0')
      if (Number.isFinite(stored) && stored > last) {
        last = stored
      }
    } catch {
      // sessionStorage may throw in strict Private Browsing / sandboxed contexts
    }
    if (!shouldReloadOnPreloadError(now, last)) return
    memoryLastReloadAt = now
    try {
      targetWindow.sessionStorage?.setItem(PRELOAD_RELOAD_KEY, String(now))
    } catch {
      // ignore storage quota / privacy errors
    }
    targetWindow.location.reload()
  })
}

const OverlayLoadingFallback: Component = {
  render: () =>
    h(
      'div',
      {
        class:
          'fixed bottom-5 right-5 z-[70] inline-flex items-center gap-2 rounded-full border border-[#f2b880]/40 bg-[#0a0f1f]/95 px-3.5 py-2 text-xs text-[#f7cba0] shadow-2xl backdrop-blur-xl',
        role: 'status',
      },
      [
        h('span', { class: 'w-2 h-2 rounded-full bg-[#f2b880] animate-pulse' }),
        h('span', '正在加载模块...'),
      ]
    ),
}

const OverlayErrorFallback: Component = {
  render: () =>
    h(
      'div',
      {
        class:
          'fixed bottom-5 right-5 z-[70] inline-flex items-center gap-2.5 rounded-xl border border-rose-400/50 bg-[#120912]/95 px-3.5 py-2 text-xs text-rose-200 shadow-2xl backdrop-blur-xl',
        role: 'alert',
      },
      [
        h('span', '模块加载失败，可能是网络波动或站点已发布新版本'),
        h(
          'button',
          {
            type: 'button',
            class: 'px-2.5 py-0.5 rounded bg-rose-500/25 border border-rose-400/45 text-rose-100 hover:bg-rose-500/35 cursor-pointer',
            onClick: () => window.location.reload(),
          },
          '刷新恢复'
        ),
      ]
    ),
}

const PanelLoadingFallback: Component = {
  render: () =>
    h(
      'div',
      {
        class: 'py-8 px-4 flex items-center justify-center gap-2 text-xs font-mono text-slate-400',
        role: 'status',
      },
      [
        h('span', { class: 'w-1.5 h-1.5 rounded-full bg-[#f2b880] animate-pulse' }),
        h('span', '正在加载子视图...'),
      ]
    ),
}

const PanelErrorFallback: Component = {
  render: () =>
    h(
      'div',
      {
        class:
          'my-4 mx-auto max-w-md rounded-xl border border-rose-400/40 bg-rose-500/10 p-4 flex items-center justify-between gap-3 text-xs text-rose-200',
        role: 'alert',
      },
      [
        h('span', '子面板资源加载失败，可能是网络波动或版本已更新'),
        h(
          'button',
          {
            type: 'button',
            class: 'px-3 py-1.5 rounded-lg border border-rose-400/45 bg-rose-500/20 text-rose-100 hover:bg-rose-500/30 cursor-pointer shrink-0',
            onClick: () => window.location.reload(),
          },
          '刷新页面'
        ),
      ]
    ),
}

function retryOnceOnTransientError(
  _error: Error,
  retry: () => void,
  fail: () => void,
  attempts: number
) {
  const online = typeof navigator === 'undefined' || navigator.onLine !== false
  if (attempts <= 1 && online) {
    window.setTimeout(retry, 350)
    return
  }
  fail()
}

export function lazyOverlayChunk<T extends Component = Component>(loader: AsyncComponentLoader<T>) {
  return defineAsyncComponent({
    loader,
    delay: 180,
    timeout: 15_000,
    loadingComponent: OverlayLoadingFallback,
    errorComponent: OverlayErrorFallback,
    onError: retryOnceOnTransientError,
  })
}

export function lazyPanelChunk<T extends Component = Component>(loader: AsyncComponentLoader<T>) {
  return defineAsyncComponent({
    loader,
    delay: 180,
    timeout: 15_000,
    loadingComponent: PanelLoadingFallback,
    errorComponent: PanelErrorFallback,
    onError: retryOnceOnTransientError,
  })
}
