<script setup lang="ts">
import { ref } from 'vue'
import { useAppStore } from '../stores/useAppStore'

const store = useAppStore()
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
    <section class="aero-glass w-full max-w-md rounded-2xl p-7 shadow-2xl">
      <div class="inline-flex items-center px-2.5 py-1 rounded-full border border-[#f2b880]/30 bg-[#f2b880]/10 text-[#f2b880] font-mono text-[11px] tracking-wider mb-3">
        VESPER BOTHUB
      </div>
      <h1 class="text-2xl font-semibold text-white mb-1">星穹 GrokBot 中枢</h1>
      <p class="font-mono text-xs text-slate-400 mb-6">bot.starryheavens.uk · 请输入管理密钥</p>

      <form class="flex flex-col gap-3" @submit.prevent="handleSubmit">
        <input
          v-model="password"
          type="password"
          placeholder="输入访问密钥..."
          autocomplete="current-password"
          required
          class="w-full min-h-[44px] rounded-xl border border-[#fff8ee]/12 bg-[#070a14]/85 px-3.5 py-2.5 font-mono text-sm text-white placeholder-slate-500 focus:border-[#f2b880]/60 focus:outline-none focus:ring-2 focus:ring-[#f2b880]/30"
        />
        <button
          type="submit"
          :disabled="loading"
          class="w-full min-h-[44px] rounded-xl bg-[#f2b880] hover:bg-[#f7cba0] active:scale-[0.99] text-[#070a13] font-semibold text-sm transition cursor-pointer disabled:opacity-50"
        >
          {{ loading ? '验证中...' : '解锁控制台' }}
        </button>
        <p v-if="errorMsg" class="text-xs text-rose-400 mt-1">{{ errorMsg }}</p>
      </form>
    </section>
  </main>
</template>
