<script setup lang="ts">
import { ref, useId } from 'vue'
import { useAppStore } from '../stores/useAppStore'
import { trackSpotlight } from '../utils/topLayer'
import BotAvatar from './BotAvatar.vue'

const store = useAppStore()
const passwordId = useId()
const password = ref('')
const loading = ref(false)
const errorMsg = ref('')

async function handleSubmit() {
  if (!password.value.trim() || loading.value) return
  loading.value = true
  errorMsg.value = ''
  try {
    await store.login(password.value)
    password.value = ''
  } catch (err: any) {
    errorMsg.value = err.message || '密钥验证失败'
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
        <BotAvatar avatar-style="cyan" state="idle" size="lg" />
      </div>
      <h1 class="text-2xl font-semibold text-white mb-1 tracking-tight">星穹 GrokBot 中枢</h1>
      <p class="font-mono text-xs text-slate-400 mb-6">bot.starryheavens.uk · 请输入管理密钥</p>

      <form class="flex flex-col gap-3" @submit.prevent="handleSubmit">
        <label :for="passwordId" class="sr-only">访问密钥</label>
        <input
          :id="passwordId"
          v-model="password"
          type="password"
          placeholder="输入访问密钥..."
          autocomplete="current-password"
          autofocus
          required
          class="w-full min-h-[44px] rounded-xl border border-[#fff8ee]/12 bg-[#070a14]/85 px-3.5 py-2.5 font-mono text-sm text-white placeholder-slate-500 focus:border-[#f2b880]/60 focus:outline-none"
        />
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
