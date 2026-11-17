<script setup lang="ts">
import { computed } from 'vue'
import type { SkillParamField } from '../../types/bothub'
import CustomSelect from '../CustomSelect.vue'

export interface HeaderRowItem {
  key: string
  value: string
}

const props = defineProps<{
  httpMethod: string
  httpUrl: string
  instruction: string
  paramRows: SkillParamField[]
  headerRows: HeaderRowItem[]
}>()

const emit = defineEmits<{
  'update:httpMethod': [val: string]
  'update:httpUrl': [val: string]
  'update:instruction': [val: string]
  'update:paramRows': [val: SkillParamField[]]
  'update:headerRows': [val: HeaderRowItem[]]
  applyTemplate: [mode: 'get' | 'post']
}>()

const METHOD_OPTIONS = [
  { value: 'GET', label: 'GET' },
  { value: 'POST', label: 'POST' },
  { value: 'PUT', label: 'PUT' },
  { value: 'PATCH', label: 'PATCH' },
  { value: 'DELETE', label: 'DELETE' },
]

const PARAM_TYPE_OPTIONS = [
  { value: 'string', label: '文本 (string)' },
  { value: 'integer', label: '整数 (integer)' },
  { value: 'boolean', label: '布尔 (boolean)' },
]

const methodModel = computed({
  get: () => props.httpMethod,
  set: (v: string) => emit('update:httpMethod', v),
})

const urlModel = computed({
  get: () => props.httpUrl,
  set: (v: string) => emit('update:httpUrl', v),
})

const instructionModel = computed({
  get: () => props.instruction,
  set: (v: string) => emit('update:instruction', v),
})

const declaredParamKeys = computed(() =>
  props.paramRows.map((p) => p.key.trim()).filter(Boolean)
)

const undeclaredTokens = computed(() => {
  const declared = new Set(declaredParamKeys.value)
  const found = new Set<string>()
  const combined = `${props.httpUrl}\n${props.instruction}`
  for (const m of combined.matchAll(/\{\{\s*([a-zA-Z0-9_.-]+)\s*\}\}/g)) {
    const token = m[1]?.trim()
    if (token && !declared.has(token)) {
      found.add(token)
    }
  }
  return Array.from(found)
})

const urlSafetyWarning = computed(() => {
  const raw = props.httpUrl.trim()
  if (!raw) return ''
  const probe = raw.replace(/\{\{[^}]+\}\}/g, 'sample')
  try {
    const u = new URL(probe)
    if (u.protocol !== 'https:' && u.protocol !== 'http:') {
      return '仅支持 http:// 或 https:// 协议'
    }
    const host = u.hostname.toLowerCase()
    if (
      host === 'localhost' ||
      host.endsWith('.local') ||
      host.endsWith('.ts.net') ||
      /^127\./.test(host) ||
      /^10\./.test(host) ||
      /^192\.168\./.test(host) ||
      /^100\.(6[4-9]|[7-9]\d|1[01]\d|12[0-7])\./.test(host)
    ) {
      return '安全提示：出站 SSRF 防护会拦截本机回环或内网私网地址，请填写公网可达的 API 地址'
    }
  } catch {
    return 'URL 格式尚不完整（需包含 https:// 或 http://）'
  }
  return ''
})

const bodyJsonWarning = computed(() => {
  if (props.httpMethod === 'GET') return ''
  const raw = props.instruction.trim()
  if (!raw || (!raw.startsWith('{') && !raw.startsWith('['))) return ''
  // Replace unquoted {{token}} with 1 and quoted "{{token}}" safely to test structural JSON syntax
  const normalized = raw
    .replace(/"\{\{\s*[a-zA-Z0-9_.-]+\s*\}\}"/g, '"__placeholder__"')
    .replace(/\{\{\s*[a-zA-Z0-9_.-]+\s*\}\}/g, '1')
  try {
    JSON.parse(normalized)
    return ''
  } catch {
    return 'Body 模板疑似 JSON 但存在语法不闭合（如多余逗号或缺少双引号）'
  }
})

function addMissingParams() {
  const next = [...props.paramRows]
  for (const token of undeclaredTokens.value) {
    next.push({
      key: token,
      type: 'string',
      description: `插值参数 ${token}`,
      required: true,
      default: '',
    })
  }
  emit('update:paramRows', next)
}

function addParamRow() {
  emit('update:paramRows', [
    ...props.paramRows,
    { key: '', type: 'string', description: '', required: false, default: '' },
  ])
}

function removeParamRow(idx: number) {
  emit(
    'update:paramRows',
    props.paramRows.filter((_, i) => i !== idx)
  )
}

function addHeaderRow() {
  emit('update:headerRows', [...props.headerRows, { key: '', value: '' }])
}

function removeHeaderRow(idx: number) {
  emit(
    'update:headerRows',
    props.headerRows.filter((_, i) => i !== idx)
  )
}
</script>

<template>
  <div class="flex flex-col gap-3">
    <div class="flex items-center gap-1.5 flex-wrap text-[11px]">
      <span class="text-slate-500">快速脚手架:</span>
      <button
        type="button"
        @click="emit('applyTemplate', 'get')"
        class="px-2 py-0.5 rounded-md border border-[#fff8ee]/12 bg-[#050811] text-slate-300 hover:border-cyan-400/45 hover:text-cyan-200 cursor-pointer"
      >
        GET 查询模板
      </button>
      <button
        type="button"
        @click="emit('applyTemplate', 'post')"
        class="px-2 py-0.5 rounded-md border border-[#fff8ee]/12 bg-[#050811] text-slate-300 hover:border-[#f2b880]/45 hover:text-[#f2b880] cursor-pointer"
      >
        POST JSON 模板
      </button>
    </div>

    <div class="grid grid-cols-1 sm:grid-cols-3 gap-2">
      <div class="flex flex-col gap-1 text-xs text-slate-300">
        <span>请求方法</span>
        <CustomSelect v-model="methodModel" :options="METHOD_OPTIONS" aria-label="请求方法" />
      </div>
      <label class="sm:col-span-2 flex flex-col gap-1 text-xs text-slate-300">
        <span>Endpoint URL (支持 <code class="text-[#f2b880]">&#123;&#123;key&#125;&#125;</code> 参数插值)</span>
        <input
          v-model="urlModel"
          type="url"
          placeholder="https://api.example.com/v1/item?q={{query}}"
          required
          class="min-h-[38px] rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-1.5 font-mono text-xs text-white focus:border-[#f2b880]/60"
        />
        <p v-if="urlSafetyWarning" class="text-[10.5px] text-amber-300/90 leading-snug">
          {{ urlSafetyWarning }}
        </p>
        <div v-if="declaredParamKeys.length > 0" class="flex items-center gap-1 flex-wrap text-[10.5px]">
          <span class="text-slate-500">插入变量:</span>
          <button
            v-for="pk in declaredParamKeys"
            :key="pk"
            type="button"
            @click="urlModel = `${urlModel}{{${pk}}}`"
            class="px-1.5 py-0.5 rounded border border-[#fff8ee]/12 bg-[#050811] font-mono text-[#f2b880] hover:border-[#f2b880]/45 cursor-pointer"
          >
            &#123;&#123;{{ pk }}&#125;&#125;
          </button>
        </div>
      </label>
    </div>

    <label class="flex flex-col gap-1 text-xs text-slate-300">
      <span v-if="httpMethod === 'GET'">补充调用说明 (可选)</span>
      <span v-else>Body 请求模板 (支持 <code class="text-[#f2b880]">&#123;&#123;key&#125;&#125;</code> 插值或留空自动传 JSON)</span>
      <textarea
        v-model="instructionModel"
        rows="3"
        :placeholder="
          httpMethod === 'GET'
            ? '给模型的参数提示，例如：city 建议传英文城市名'
            : '{&quot;target&quot;: &quot;{{target}}&quot;, &quot;note&quot;: &quot;{{note}}&quot;}'
        "
        class="auto-size-textarea rounded-xl border border-[#fff8ee]/12 bg-[#050811] px-3 py-2 font-mono text-[11.5px] text-white focus:border-[#f2b880]/60"
      />
      <p v-if="bodyJsonWarning" class="text-[10.5px] text-amber-300/90 leading-snug">
        {{ bodyJsonWarning }}
      </p>
    </label>

    <!-- Undeclared interpolation variable quick-fix banner -->
    <div
      v-if="undeclaredTokens.length > 0"
      class="rounded-xl border border-cyan-400/35 bg-cyan-500/10 px-3 py-2 flex items-center justify-between gap-2 flex-wrap text-[11px] text-cyan-200"
    >
      <span>
        检测到未声明的插值变量：<code class="font-mono text-cyan-100">{{ undeclaredTokens.join(', ') }}</code>
      </span>
      <button
        type="button"
        @click="addMissingParams"
        class="px-2.5 py-0.5 rounded-lg border border-cyan-400/45 bg-cyan-500/20 hover:bg-cyan-500/30 font-medium text-cyan-100 cursor-pointer shrink-0"
      >
        + 一键补齐参数声明
      </button>
    </div>

    <!-- Dynamic Parameter Schema -->
    <div class="rounded-xl border border-[#fff8ee]/10 bg-[#050811]/70 p-2.5 flex flex-col gap-2">
      <div class="flex items-center justify-between">
        <span class="text-[11px] font-medium text-slate-300">输入参数定义 (编译为 Function Calling Schema)</span>
        <button
          type="button"
          @click="addParamRow"
          class="text-[11px] text-[#f2b880] hover:underline cursor-pointer"
        >
          + 添加参数
        </button>
      </div>
      <div v-if="paramRows.length === 0" class="text-[11px] text-slate-500">
        无入参；若 URL/Body 中包含插值变量，请在这里声明参数名。
      </div>
      <div
        v-for="(p, idx) in paramRows"
        :key="idx"
        class="grid grid-cols-1 sm:grid-cols-12 gap-1.5 items-center"
      >
        <input
          v-model="p.key"
          type="text"
          placeholder="参数名 key"
          class="sm:col-span-3 min-h-[32px] rounded-lg border border-[#fff8ee]/10 bg-[#070a13] px-2 font-mono text-[11px] text-white"
        />
        <div class="sm:col-span-2">
          <CustomSelect v-model="p.type" :options="PARAM_TYPE_OPTIONS" aria-label="参数类型" />
        </div>
        <input
          v-model="p.description"
          type="text"
          placeholder="参数说明"
          class="sm:col-span-3 min-h-[32px] rounded-lg border border-[#fff8ee]/10 bg-[#070a13] px-2 text-[11px] text-white"
        />
        <input
          v-model="p.default"
          type="text"
          placeholder="默认值(可选)"
          class="sm:col-span-2 min-h-[32px] rounded-lg border border-[#fff8ee]/10 bg-[#070a13] px-2 font-mono text-[11px] text-white"
        />
        <div class="sm:col-span-2 flex items-center justify-between gap-1">
          <label class="inline-flex items-center gap-1 text-[10.5px] text-slate-400 cursor-pointer">
            <input v-model="p.required" type="checkbox" class="accent-[#f2b880]" />
            <span>必填</span>
          </label>
          <button
            type="button"
            @click="removeParamRow(idx)"
            class="text-[11px] text-rose-400 hover:text-rose-300 cursor-pointer"
          >
            删
          </button>
        </div>
      </div>
      <div v-if="paramRows.length > 0" class="hidden sm:grid sm:grid-cols-12 gap-1.5 text-[10px] text-slate-500 px-0.5">
        <span class="col-span-3">参数键名</span>
        <span class="col-span-2">类型</span>
        <span class="col-span-3">语义描述</span>
        <span class="col-span-2">默认值</span>
        <span class="col-span-2">操作</span>
      </div>
    </div>

    <!-- Custom Headers -->
    <div class="rounded-xl border border-[#fff8ee]/10 bg-[#050811]/70 p-2.5 flex flex-col gap-2">
      <div class="flex items-center justify-between">
        <span class="text-[11px] font-medium text-slate-300">请求头 Headers (可选)</span>
        <button
          type="button"
          @click="addHeaderRow"
          class="text-[11px] text-[#f2b880] hover:underline cursor-pointer"
        >
          + 添加请求头
        </button>
      </div>
      <div
        v-for="(h, idx) in headerRows"
        :key="idx"
        class="grid grid-cols-12 gap-1.5 items-center"
      >
        <input
          v-model="h.key"
          type="text"
          placeholder="Header 名 (如 Authorization)"
          class="col-span-5 min-h-[32px] rounded-lg border border-[#fff8ee]/10 bg-[#070a13] px-2 font-mono text-[11px] text-white"
        />
        <input
          v-model="h.value"
          type="text"
          placeholder="Header 值 (如 Bearer xxx)"
          class="col-span-6 min-h-[32px] rounded-lg border border-[#fff8ee]/10 bg-[#070a13] px-2 font-mono text-[11px] text-white"
        />
        <button
          type="button"
          @click="removeHeaderRow(idx)"
          class="col-span-1 text-[11px] text-rose-400 hover:text-rose-300 cursor-pointer text-right"
        >
          删
        </button>
      </div>
    </div>
  </div>
</template>
