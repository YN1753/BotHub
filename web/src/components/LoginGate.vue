<script setup lang="ts">
import { ref, computed, useId, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { useAppStore } from '../stores/useAppStore'
import { BotHubApi } from '../api/client'
import { trackSpotlight } from '../utils/topLayer'
import { checkPasskeyCapabilities, loginWithPasskeyCeremony } from '../utils/passkey'
import BotAvatar from './BotAvatar.vue'
import VesperBrandIcon from './ui/VesperBrandIcon.vue'

const store = useAppStore()
const headingId = useId()
const passwordId = useId()
const fallbackFormId = useId()
const hostLabel = typeof window !== 'undefined' && window.location.host ? window.location.host : 'BotHub'
const password = ref('')
const showPassword = ref(false)
const rememberDevice = ref(true)
const capsLockOn = ref(false)
const loading = ref(false)
const oidcRedirecting = ref(false)
const passkeyLoading = ref(false)
const oidcEnabled = ref(Boolean(store.state.oidcEnabled))
const hasPasskeys = ref(false)
const passkeySupported = ref(false)
const showPasswordFallback = ref(!store.state.oidcEnabled)
const errorMsg = ref('')
const inputEl = ref<HTMLInputElement | null>(null)

const hasPasswordlessPrimary = computed(
  () => oidcEnabled.value || (hasPasskeys.value && passkeySupported.value)
)

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
  const [authRes, statusRes, caps] = await Promise.all([
    BotHubApi.authStatus().catch(() => ({
      authenticated: false,
      oidc_enabled: store.state.oidcEnabled,
      oidc_provider_name: store.state.oidcProviderName,
    })),
    BotHubApi.passkeyStatus().catch(() => ({ has_passkeys: false, count: 0, rp_id: '' })),
    checkPasskeyCapabilities(),
  ])
  oidcEnabled.value = Boolean(authRes.oidc_enabled)
  store.state.oidcEnabled = oidcEnabled.value
  if (authRes.oidc_provider_name) {
    store.state.oidcProviderName = authRes.oidc_provider_name
  }
  hasPasskeys.value = Boolean(statusRes.has_passkeys)
  passkeySupported.value = caps.platformSupported

  if (oidcEnabled.value) {
    showPasswordFallback.value = false
  } else if (hasPasskeys.value && caps.platformSupported) {
    showPasswordFallback.value = false
    void armConditionalAutofill(caps.conditionalSupported)
  } else {
    showPasswordFallback.value = true
    inputEl.value?.focus()
  }
})

onBeforeUnmount(() => {
  autofillAbort?.abort()
})

function handleOIDCLogin() {
  if (oidcRedirecting.value || loading.value || passkeyLoading.value) return
  autofillAbort?.abort()
  oidcRedirecting.value = true
  errorMsg.value = ''
  const params = new URLSearchParams(window.location.search)
  if (params.get('auto_pair_app') === '1') {
    window.location.assign('/api/auth/oidc/login?platform=android')
  } else {
    window.location.assign('/api/auth/oidc/login?platform=web')
  }
}

async function revealPasswordFallback() {
  showPasswordFallback.value = !showPasswordFallback.value
  if (showPasswordFallback.value) {
    await nextTick()
    inputEl.value?.focus()
  }
}

async function handlePasskeyUnlock() {
  if (passkeyLoading.value || loading.value || oidcRedirecting.value) return
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
  if (!password.value.trim() || loading.value || oidcRedirecting.value) return
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
          <span>BOTHUB</span>
        </div>
        <BotAvatar
          avatar-style="butler:gold"
          :state="loading || passkeyLoading || oidcRedirecting ? 'working' : password ? 'thinking' : 'idle'"
          size="lg"
          focusable
        />
      </div>
      <h1 :id="headingId" class="text-2xl font-semibold text-white mb-1 tracking-tight">BotHub 智能中枢</h1>
      <p class="font-mono text-xs text-slate-300 mb-6">
        {{ hostLabel }} ·
        {{
          oidcEnabled
            ? '支持 Vesper Auth 单点通行证免密登录'
            : hasPasskeys && passkeySupported
            ? '支持面容 / 指纹通行密钥解锁'
            : '身份验证'
        }}
      </p>

      <!-- Priority 1: Vesper Auth OIDC + PKCE S256 Single Sign-On -->
      <div v-if="oidcEnabled" class="flex flex-col gap-2.5 mb-3">
        <button
          type="button"
          :disabled="oidcRedirecting || passkeyLoading || loading"
          @click="handleOIDCLogin"
          class="btn-jewel w-full min-h-[48px] rounded-xl text-sm font-semibold inline-flex items-center justify-center gap-2 cursor-pointer disabled:opacity-50"
        >
          <svg viewBox="0 0 24 24" width="17" height="17" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
            <path d="m9 12 2 2 4-4" />
          </svg>
          <span>{{ oidcRedirecting ? '正在跳转 Vesper Auth...' : '使用 Vesper Auth 通行证登录' }}</span>
        </button>
      </div>

      <!-- Biometric Passkey Sign-In (Primary when OIDC disabled, Secondary when OIDC enabled) -->
      <div v-if="hasPasskeys && passkeySupported" class="flex flex-col gap-2.5 mb-3">
        <button
          type="button"
          :disabled="passkeyLoading || loading || oidcRedirecting"
          @click="handlePasskeyUnlock"
          :class="[
            'w-full min-h-[46px] rounded-xl text-xs sm:text-sm font-semibold inline-flex items-center justify-center gap-2 cursor-pointer disabled:opacity-50',
            oidcEnabled
              ? 'border border-[#f2b880]/40 bg-[#f2b880]/10 hover:bg-[#f2b880]/20 text-[#ffe2be]'
              : 'btn-jewel'
          ]"
        >
          <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
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
      </div>

      <form
        v-if="showPasswordFallback"
        :id="fallbackFormId"
        class="flex flex-col gap-3 mt-1"
        @submit.prevent="handleSubmit"
      >
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
          :class="[
            'w-full min-h-[44px] rounded-xl text-sm cursor-pointer disabled:opacity-50',
            hasPasswordlessPrimary
              ? 'border border-[#fff8ee]/18 bg-white/[0.06] hover:bg-white/[0.12] text-white font-medium'
              : 'btn-jewel'
          ]"
        >
          {{ loading ? '验证中...' : '解锁控制台' }}
        </button>
      </form>

      <div v-if="hasPasswordlessPrimary" class="mt-3 text-center">
        <button
          type="button"
          :aria-expanded="showPasswordFallback"
          :aria-controls="fallbackFormId"
          @click="revealPasswordFallback"
          class="text-xs text-slate-400 hover:text-[#f2b880] py-1.5 px-2 cursor-pointer"
        >
          {{ showPasswordFallback ? '收起访问密钥表单' : '改用访问密钥解锁' }}
        </button>
      </div>

      <p v-if="errorMsg" role="alert" class="text-xs text-rose-400 mt-2">{{ errorMsg }}</p>
    </section>
  </main>
</template>
