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
  <div class="rounded-xl border border-info-border bg-canvas p-4 flex flex-col gap-2.5">
    <div class="flex items-center justify-between gap-2">
      <h4 class="text-xs font-semibold text-info truncate">
        实时调试工具: skill_{{ skill.slug }}
      </h4>
      <button
        type="button"
        @click="emit('close')"
        class="text-xs text-fg-muted hover:text-fg cursor-pointer shrink-0"
      >
        收起
      </button>
    </div>

    <div v-if="parsedSchema.length > 0" class="flex flex-col gap-2">
      <label
        v-for="field in parsedSchema"
        :key="field.key"
        class="flex flex-col gap-1 text-xs text-fg-sub"
      >
        <span class="flex items-center justify-between gap-2">
          <span class="font-mono text-info">
            {{ field.key }}
            <span v-if="field.required" class="text-danger">*</span>
            <span class="text-fg-muted text-xs ml-1">({{ field.type || 'string' }})</span>
          </span>
          <span v-if="field.description" class="text-xs text-fg-muted truncate">{{ field.description }}</span>
        </span>
        <input
          v-model="testArgs[field.key]"
          :type="field.type === 'integer' ? 'number' : 'text'"
          :placeholder="field.type === 'boolean' ? 'true 或 false' : '输入测试参数值'"
          class="min-h-[34px] rounded-lg border border-line bg-canvas px-2.5 text-xs text-fg focus:border-info focus:outline-none"
        />
      </label>
    </div>
    <div v-else class="text-xs text-fg-muted">
      该 Webhook 工具无需输入参数，可直接发送测试请求。
    </div>

    <button
      type="button"
      :disabled="testing"
      @click="runLiveTest"
      class="min-h-[36px] px-3 py-1.5 rounded-xl bg-info-soft border border-info-border text-xs font-medium text-info hover:bg-info-soft cursor-pointer disabled:opacity-50"
    >
      {{ testing ? '正在请求远端接口...' : '发送测试请求' }}
    </button>

    <div v-if="testResult" class="rounded-lg bg-canvas border border-line p-2.5 text-xs font-mono">
      <div v-if="testResult.error" class="text-danger">{{ testResult.error }}</div>
      <template v-else>
        <div class="flex items-center justify-between gap-2 mb-1">
          <span :class="(testResult.statusCode || 200) >= 400 ? 'text-warn' : 'text-ok'">
            HTTP {{ testResult.statusCode || 200 }} · {{ testResult.latencyMs }}ms
          </span>
          <button
            type="button"
            @click="copyTestOutput(testResult.output)"
            class="px-2 py-0.5 rounded border border-line text-xs text-fg-sub hover:text-accent-fg cursor-pointer"
          >
            复制响应
          </button>
        </div>
        <pre class="whitespace-pre-wrap break-all text-fg-sub max-h-48 overflow-y-auto">{{ formatTestOutput(testResult.output) }}</pre>
      </template>
    </div>
  </div>
</template>
