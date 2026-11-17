<script setup lang="ts">
import { ref, useId, onMounted, onBeforeUnmount } from 'vue'
import { useAppStore } from '../stores/useAppStore'
import { BotHubApi } from '../api/client'
import { trackSpotlight } from '../utils/topLayer'
import { checkPasskeyCapabilities, loginWithPasskeyCeremony } from '../utils/passkey'
import BotAvatar from './BotAvatar.vue'
import VesperBrandIcon from './ui/VesperBrandIcon.vue'

const store = useAppStore()
const headingId = useId()
const passwordId = useId()
const hostLabel = typeof window !== 'undefined' && window.location.host ? window.location.host : 'Vesper BotHub'
const password = ref('')
const showPassword = ref(false)
const rememberDevice = ref(true)
const capsLockOn = ref(false)
const loading = ref(false)
const passkeyLoading = ref(false)
const hasPasskeys = ref(false)
const passkeySupported = ref(false)
const showPasswordFallback = ref(true)
const errorMsg = ref('')
const inputEl = ref<HTMLInputElement | null>(null)

let autofillAbort: AbortController | null = null

function checkCapsLock(e: KeyboardEvent) {
  capsLockOn.value = typeof e.getModifierState === 'function' && e.getModifierState('CapsLock')
}

async function armConditionalAutofill(conditionalOk: boolean) {
  if (!conditionalOk || !hasPasskeys.value) return
  autofillAbort?.abort()
  autofillAbort = new AbortController()
  const res = await loginWithPasskeyCeremony({
    signal: autofillAbort.signal,
    conditional: true,
  })
  if (res.ok) {
    await store.completeSessionActivation()
  }
}

onMounted(async () => {
  const [statusRes, caps] = await Promise.all([
    BotHubApi.passkeyStatus().catch(() => ({ has_passkeys: false, count: 0, rp_id: '' })),
    checkPasskeyCapabilities(),
  ])
  hasPasskeys.value = Boolean(statusRes.has_passkeys)
  passkeySupported.value = caps.platformSupported

  if (hasPasskeys.value && caps.platformSupported) {
    showPasswordFallback.value = false
    void armConditionalAutofill(caps.conditionalSupported)
  } else {
    inputEl.value?.focus()
  }
})

onBeforeUnmount(() => {
  autofillAbort?.abort()
})

async function handlePasskeyUnlock() {
  if (passkeyLoading.value || loading.value) return
  autofillAbort?.abort()
  autofillAbort = new AbortController()
  passkeyLoading.value = true
  errorMsg.value = ''
  try {
    const res = await loginWithPasskeyCeremony({ signal: autofillAbort.signal })
    if (res.ok) {
      await store.completeSessionActivation()
      return
    }
    if (!res.cancelled && res.error) {
      errorMsg.value = res.error
      showPasswordFallback.value = true
    }
  } finally {
    passkeyLoading.value = false
  }
}

async function handleSubmit() {
  if (!password.value.trim() || loading.value) return
  autofillAbort?.abort()
  loading.value = true
  errorMsg.value = ''
  try {
    await store.login(password.value, rememberDevice.value)
    password.value = ''
  } catch (err: any) {
    errorMsg.value = err.message || '密钥验证失败'
    inputEl.value?.focus()
    inputEl.value?.select()
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <main class="relative z-10 min-h-dvh flex items-center justify-center p-4">
    <section
      @pointermove="trackSpotlight"
      :class="['luxury-card w-full max-w-md rounded-2xl p-7 shadow-2xl', errorMsg ? 'vesper-shake border-rose-400/35' : '']"
    >
      <div class="flex items-center justify-between gap-3 mb-4">
        <div class="btn-tactile-gold inline-flex items-center gap-2 pl-1.5 pr-3 py-1 rounded-full font-mono text-[11px] tracking-wider">
          <VesperBrandIcon class="w-4 h-4 rounded-[4px]" />
          <span>VESPER BOTHUB</span>
        </div>
        <BotAvatar
          avatar-style="butler:gold"
          :state="loading || passkeyLoading ? 'working' : password ? 'thinking' : 'idle'"
          size="lg"
          focusable
        />
      </div>
      <h1 :id="headingId" class="text-2xl font-semibold text-white mb-1 tracking-tight">星穹 GrokBot 中枢</h1>
      <p class="font-mono text-xs text-slate-300 mb-6">
        {{ hostLabel }} · {{ hasPasskeys && passkeySupported ? '支持面容 / 指纹通行密钥解锁' : '身份验证' }}
      </p>

      <!-- Biometric Passkey Primary Sign-In -->
      <div v-if="hasPasskeys && passkeySupported" class="flex flex-col gap-3 mb-4">
        <button
          type="button"
          :disabled="passkeyLoading || loading"
          @click="handlePasskeyUnlock"
          class="btn-jewel w-full min-h-[48px] rounded-xl text-sm font-semibold inline-flex items-center justify-center gap-2 cursor-pointer disabled:opacity-50"
        >
          <svg viewBox="0 0 24 24" width="17" height="17" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <path d="M12 10a2 2 0 0 0-2 2c0 1.02-.1 2.51-.26 4" />
            <path d="M14 13.12c0 2.38 0 6.38-1 8.88" />
            <path d="M17.29 21.02c.12-.6.43-2.3.5-3.02" />
            <path d="M2 12a10 10 0 0 1 18-6" />
            <path d="M2 16h.01" />
            <path d="M21.8 16c.2-2 .131-5.354 0-6" />
            <path d="M5 19.5C5.5 18 6 15 6 12a6 6 0 0 1 .34-2" />
            <path d="M8.65 22c.21-.66.45-1.32.57-2" />
            <path d="M9 6.8a6 6 0 0 1 9 5.2v2" />
          </svg>
          <span>{{ passkeyLoading ? '正在调起面容 / 指纹...' : '使用面容 / 指纹无感解锁' }}</span>
        </button>

        <button
          v-if="!showPasswordFallback"
          type="button"
          @click="showPasswordFallback = true"
          class="text-xs text-slate-400 hover:text-[#f2b880] py-1 cursor-pointer"
        >
          改用访问密码解锁
        </button>
      </div>

      <form v-if="showPasswordFallback" class="flex flex-col gap-3" @submit.prevent="handleSubmit">
        <input
          type="text"
          name="username"
          :aria-describedby="headingId"
          autocomplete="username webauthn"
          :value="hostLabel"
          readonly
          tabindex="-1"
          aria-hidden="true"
          class="sr-only"
        />
        <label :for="passwordId" class="sr-only">访问密钥</label>
        <div class="relative">
          <input
            ref="inputEl"
            :id="passwordId"
            v-model="password"
            name="password"
            :type="showPassword ? 'text' : 'password'"
            placeholder="输入访问密钥..."
            autocomplete="current-password"
            enterkeyhint="go"
            :aria-invalid="errorMsg ? 'true' : undefined"
            required
            @keydown="checkCapsLock"
            @keyup="checkCapsLock"
            class="w-full min-h-[44px] rounded-xl border border-[#fff8ee]/12 bg-[#070a14]/85 pl-3.5 pr-12 py-2.5 font-mono text-sm text-white placeholder-slate-500 focus:border-[#f2b880]/60"
          />
          <button
            type="button"
            @click="showPassword = !showPassword"
            :aria-pressed="showPassword"
            :aria-label="showPassword ? '隐藏密钥' : '显示密钥'"
            class="absolute right-2 top-1/2 -translate-y-1/2 px-2 py-1 rounded-lg text-[11px] font-mono text-slate-400 hover:text-[#f2b880] focus-visible:outline focus-visible:outline-2 focus-visible:outline-[#f2b880] cursor-pointer"
          >
            {{ showPassword ? '隐藏' : '显示' }}
          </button>
        </div>
        <p v-if="capsLockOn" class="text-[11px] font-mono text-amber-300/90">
          大写锁定 (Caps Lock) 已开启
        </p>
        <label class="inline-flex items-start gap-2 text-xs text-slate-300 cursor-pointer select-none">
          <input v-model="rememberDevice" type="checkbox" class="accent-[#f2b880] rounded mt-0.5 shrink-0" />
          <span>记住此设备（30 天免重输；公共设备请勿勾选）</span>
        </label>
        <button
          type="submit"
          :disabled="loading"
          class="btn-jewel w-full min-h-[44px] rounded-xl text-sm cursor-pointer disabled:opacity-50"
        >
          {{ loading ? '验证中...' : '解锁控制台' }}
        </button>
      </form>
      <p v-if="errorMsg" role="alert" class="text-xs text-rose-400 mt-2">{{ errorMsg }}</p>
    </section>
  </main>
</template>
