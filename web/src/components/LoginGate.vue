<script setup lang="ts">
import { ref, useId, onMounted } from 'vue'
import { useAppStore } from '../stores/useAppStore'
import { trackSpotlight } from '../utils/topLayer'
import BotAvatar from './BotAvatar.vue'

const store = useAppStore()
const passwordId = useId()
const hostLabel = typeof window !== 'undefined' && window.location.host ? window.location.host : 'Vesper BotHub'
const password = ref('')
const showPassword = ref(false)
const capsLockOn = ref(false)
const loading = ref(false)
const errorMsg = ref('')
const inputEl = ref<HTMLInputElement | null>(null)

function checkCapsLock(e: KeyboardEvent) {
  capsLockOn.value = typeof e.getModifierState === 'function' && e.getModifierState('CapsLock')
}

onMounted(() => {
  inputEl.value?.focus()
})

async function handleSubmit() {
  if (!password.value.trim() || loading.value) return
  loading.value = true
  errorMsg.value = ''
  try {
    await store.login(password.value)
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
      class="luxury-card w-full max-w-md rounded-2xl p-7 shadow-2xl"
    >
      <div class="flex items-center justify-between gap-3 mb-4">
        <div class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full border border-[#f2b880]/30 bg-[#f2b880]/10 text-[#f2b880] font-mono text-[11px] tracking-wider">
          <span class="w-1.5 h-1.5 rounded-full bg-[#f2b880]" aria-hidden="true" />
          <span>VESPER BOTHUB</span>
        </div>
        <BotAvatar avatar-style="butler:gold" :state="loading ? 'working' : password ? 'thinking' : 'idle'" size="lg" />
      </div>
      <h1 class="text-2xl font-semibold text-white mb-1 tracking-tight">星穹 GrokBot 中枢</h1>
      <p class="font-mono text-xs text-slate-400 mb-6">{{ hostLabel }} · 请输入管理密钥</p>

      <form class="flex flex-col gap-3" @submit.prevent="handleSubmit">
        <label :for="passwordId" class="sr-only">访问密钥</label>
        <div class="relative">
          <input
            ref="inputEl"
            :id="passwordId"
            v-model="password"
            :type="showPassword ? 'text' : 'password'"
            placeholder="输入访问密钥..."
            autocomplete="current-password"
            :aria-invalid="errorMsg ? 'true' : undefined"
            required
            @keydown="checkCapsLock"
            @keyup="checkCapsLock"
            class="w-full min-h-[44px] rounded-xl border border-[#fff8ee]/12 bg-[#070a14]/85 pl-3.5 pr-12 py-2.5 font-mono text-sm text-white placeholder-slate-500 focus:border-[#f2b880]/60"
          />
          <button
            type="button"
            @click="showPassword = !showPassword"
            :aria-label="showPassword ? '隐藏密钥' : '显示密钥'"
            class="absolute right-2 top-1/2 -translate-y-1/2 px-2 py-1 rounded-lg text-[11px] font-mono text-slate-400 hover:text-[#f2b880] cursor-pointer"
          >
            {{ showPassword ? '隐藏' : '显示' }}
          </button>
        </div>
        <p v-if="capsLockOn" class="text-[11px] font-mono text-amber-300/90">
          大写锁定 (Caps Lock) 已开启
        </p>
        <button
          type="submit"
          :disabled="loading"
          class="w-full min-h-[44px] rounded-xl bg-[#f2b880] hover:bg-[#f7cba0] text-[#070a13] font-semibold text-sm cursor-pointer disabled:opacity-50"
        >
          {{ loading ? '验证中...' : '解锁控制台' }}
        </button>
        <p v-if="errorMsg" role="alert" class="text-xs text-rose-400 mt-1">{{ errorMsg }}</p>
      </form>
    </section>
  </main>
</template>
