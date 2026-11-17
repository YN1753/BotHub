<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { BotHubApi } from '../../api/client'
import { useAppStore } from '../../stores/useAppStore'
import { copyToClipboard } from '../../utils/clipboard'
import type { Skill, SkillParamField } from '../../types/bothub'

const props = defineProps<{
  skill: Skill
}>()

const emit = defineEmits<{
  close: []
}>()

const store = useAppStore()
const testArgs = ref<Record<string, string>>({})
const lastTestArgsBySkill = ref<Record<string, Record<string, string>>>({})
const testing = ref(false)
const testResult = ref<{ statusCode?: number; latencyMs?: number; output: string; error?: string } | null>(null)

const parsedSchema = computed<SkillParamField[]>(() => {
  try {
    const arr = JSON.parse(props.skill.params_schema || '[]')
    return Array.isArray(arr) ? arr.filter((f: any) => Boolean(f?.key)) : []
  } catch {
    return []
  }
})

watch(
  () => props.skill,
  (sk, prev) => {
    if (prev) {
      lastTestArgsBySkill.value[prev.id] = { ...testArgs.value }
    }
    testResult.value = null
    const initArgs: Record<string, string> = {}
    const prevSaved = lastTestArgsBySkill.value[sk.id] || {}
    for (const f of parsedSchema.value) {
      const k = String(f.key)
      initArgs[k] = k in prevSaved ? prevSaved[k] : String(f.default || '')
    }
    testArgs.value = initArgs
  },
  { immediate: true }
)

function formatTestOutput(raw: string): string {
  try {
    return JSON.stringify(JSON.parse(raw), null, 2)
  } catch {
    return raw
  }
}

async function copyTestOutput(raw: string) {
  const ok = await copyToClipboard(formatTestOutput(raw))
  store.notify(ok ? '已复制响应内容' : '复制失败')
}

async function runLiveTest() {
  if (testing.value) return
  testing.value = true
  testResult.value = null
  try {
    const typedArgs: Record<string, any> = {}
    const typeMap = new Map(parsedSchema.value.map((f) => [f.key, f.type]))
    for (const [k, rawVal] of Object.entries(testArgs.value)) {
      const ftype = typeMap.get(k) || 'string'
      if (ftype === 'integer') {
        typedArgs[k] = Number.parseInt(rawVal || '0', 10) || 0
      } else if (ftype === 'boolean') {
        typedArgs[k] = rawVal.trim().toLowerCase() === 'true' || rawVal.trim() === '1'
      } else {
        typedArgs[k] = rawVal
      }
    }
    const res = await BotHubApi.testSkill(props.skill.id, typedArgs)
    testResult.value = {
      statusCode: res.status_code,
      latencyMs: res.latency_ms,
      output: typeof res.result === 'string' ? res.result : JSON.stringify(res.result, null, 2),
    }
  } catch (err: any) {
    testResult.value = {
      output: '',
      error: err.message || '调用测试失败',
    }
  } finally {
    testing.value = false
  }
}
</script>

<template>
  <div class="rounded-xl border border-cyan-400/30 bg-[#060914] p-4 flex flex-col gap-2.5">
    <div class="flex items-center justify-between gap-2">
      <h4 class="text-xs font-semibold text-cyan-200 truncate">
        实时调试工具: skill_{{ skill.slug }}
      </h4>
      <button
        type="button"
        @click="emit('close')"
        class="text-[11px] text-slate-400 hover:text-white cursor-pointer shrink-0"
      >
        收起
      </button>
    </div>

    <div v-if="parsedSchema.length > 0" class="flex flex-col gap-2">
      <label
        v-for="field in parsedSchema"
        :key="field.key"
        class="flex flex-col gap-1 text-[11px] text-slate-300"
      >
        <span class="flex items-center justify-between gap-2">
          <span class="font-mono text-cyan-300">
            {{ field.key }}
            <span v-if="field.required" class="text-rose-300">*</span>
            <span class="text-slate-500 text-[10px] ml-1">({{ field.type || 'string' }})</span>
          </span>
          <span v-if="field.description" class="text-[10.5px] text-slate-500 truncate">{{ field.description }}</span>
        </span>
        <input
          v-model="testArgs[field.key]"
          :type="field.type === 'integer' ? 'number' : 'text'"
          :placeholder="field.type === 'boolean' ? 'true 或 false' : '输入测试参数值'"
          class="min-h-[34px] rounded-lg border border-[#fff8ee]/12 bg-[#050811] px-2.5 text-xs text-white focus:border-cyan-400/55 focus:outline-none"
        />
      </label>
    </div>
    <div v-else class="text-[11px] text-slate-500">
      该 Webhook 工具无需输入参数，可直接发送测试请求。
    </div>

    <button
      type="button"
      :disabled="testing"
      @click="runLiveTest"
      class="min-h-[36px] px-3 py-1.5 rounded-xl bg-cyan-500/20 border border-cyan-400/40 text-xs font-medium text-cyan-200 hover:bg-cyan-500/30 cursor-pointer disabled:opacity-50"
    >
      {{ testing ? '正在请求远端接口...' : '发送测试请求' }}
    </button>

    <div v-if="testResult" class="rounded-lg bg-[#050811] border border-[#fff8ee]/10 p-2.5 text-[11px] font-mono">
      <div v-if="testResult.error" class="text-rose-300">{{ testResult.error }}</div>
      <template v-else>
        <div class="flex items-center justify-between gap-2 mb-1">
          <span :class="(testResult.statusCode || 200) >= 400 ? 'text-amber-300' : 'text-emerald-300'">
            HTTP {{ testResult.statusCode || 200 }} · {{ testResult.latencyMs }}ms
          </span>
          <button
            type="button"
            @click="copyTestOutput(testResult.output)"
            class="px-2 py-0.5 rounded border border-[#fff8ee]/12 text-[10px] text-slate-300 hover:text-[#f2b880] cursor-pointer"
          >
            复制响应
          </button>
        </div>
        <pre class="whitespace-pre-wrap break-all text-slate-300 max-h-48 overflow-y-auto">{{ formatTestOutput(testResult.output) }}</pre>
      </template>
    </div>
  </div>
</template>
