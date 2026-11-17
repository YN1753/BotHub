<script setup lang="ts">
import { computed, ref } from 'vue'
import {
  BOT_COLORS,
  BOT_SHAPES,
  composeAvatarStyle,
  parseAvatarStyle,
  type BotColor,
  type BotShape,
} from '../utils/botLook'
import { trackSpotlight } from '../utils/topLayer'
import BotAvatar, { type BotStuntKind } from './BotAvatar.vue'

const props = defineProps<{
  modelValue: string
  previewName?: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

const PREVIEW_STATES = [
  { id: 'idle', label: '待机' },
  { id: 'thinking', label: '思考' },
  { id: 'working', label: '执行' },
  { id: 'done', label: '愉悦' },
  { id: 'blocked', label: '告警' },
] as const

const STUNT_BUTTONS: Array<{ id: BotStuntKind; label: string }> = [
  { id: 'hop', label: '果冻跳' },
  { id: 'flip', label: '空翻' },
  { id: 'dance', label: '摇摆舞' },
  { id: 'shy', label: '害羞趴' },
]

const previewState = ref<string>('idle')
const previewAvatarRef = ref<InstanceType<typeof BotAvatar> | null>(null)
const parsed = computed(() => parseAvatarStyle(props.modelValue))

function pickShape(shape: BotShape) {
  emit('update:modelValue', composeAvatarStyle(shape, parsed.value.color))
}

function pickColor(color: BotColor) {
  emit('update:modelValue', composeAvatarStyle(parsed.value.shape, color))
}

function randomizeLook() {
  const nextShape = BOT_SHAPES[Math.floor(Math.random() * BOT_SHAPES.length)].id
  const nextColor = BOT_COLORS[Math.floor(Math.random() * BOT_COLORS.length)].id
  emit('update:modelValue', composeAvatarStyle(nextShape, nextColor))
}

function triggerPreviewStunt(kind: BotStuntKind) {
  previewAvatarRef.value?.playStunt(kind)
}
</script>

<template>
  <div class="modal-section flex flex-col gap-3.5">
    <div class="flex items-center justify-between gap-3 flex-wrap">
      <div class="flex items-center gap-3.5 min-w-0">
        <div class="well-recessed relative flex items-center justify-center p-2 rounded-2xl">
          <BotAvatar ref="previewAvatarRef" :avatar-style="modelValue" :state="previewState" size="lg" focusable />
        </div>
        <div class="min-w-0">
          <div class="text-xs font-semibold text-white truncate">
            {{ previewName || '自定义灵动伙伴预览' }}
          </div>
          <div class="text-[11px] text-slate-400 mt-0.5">
            6 种性格待机律动 + 4 套互动动作特技 · 点击 Bot 循环触发果冻跳 / 空翻 / 摇摆 / 害羞趴
          </div>
          <!-- Live State & Stunt Action Playground -->
          <div class="flex items-center gap-1 mt-1.5 flex-wrap" role="group" aria-label="预览动态状态与特技动作">
            <button
              v-for="st in PREVIEW_STATES"
              :key="st.id"
              type="button"
              :aria-pressed="previewState === st.id"
              @click="previewState = st.id"
              :class="[
                'px-2 py-0.5 rounded-full font-mono text-[10px] border cursor-pointer transition-colors',
                previewState === st.id
                  ? 'btn-tactile-gold'
                  : 'border-[#fff8ee]/10 bg-[#050811]/60 text-slate-400 hover:text-slate-200',
              ]"
            >
              {{ st.label }}
            </button>
            <span class="mx-0.5 h-3 w-px bg-[#fff8ee]/12" aria-hidden="true" />
            <button
              v-for="act in STUNT_BUTTONS"
              :key="act.id"
              type="button"
              @click="triggerPreviewStunt(act.id)"
              class="btn-tactile px-2 py-0.5 rounded-full font-mono text-[10px] cursor-pointer"
            >
              {{ act.label }}
            </button>
          </div>
        </div>
      </div>

      <!-- Color Swatch Dots + Randomizer -->
      <div class="flex items-center gap-1 shrink-0 flex-wrap justify-end" role="group" aria-label="材质釉色">
        <button
          v-for="c in BOT_COLORS"
          :key="c.id"
          type="button"
          :title="c.label"
          :aria-label="c.label"
          :aria-pressed="parsed.color === c.id"
          @click="pickColor(c.id)"
          class="min-w-[30px] min-h-[30px] inline-flex items-center justify-center rounded-full focus-visible:outline focus-visible:outline-2 focus-visible:outline-[#f2b880] cursor-pointer"
        >
          <span
            :style="{
              background: `radial-gradient(circle at 32% 28%, ${c.shellHi} 0%, ${c.shellMid} 55%, ${c.ring} 100%)`,
            }"
            :class="[
              'w-5 h-5 rounded-full transition-transform duration-200 shadow-inner',
              parsed.color === c.id
                ? 'scale-110 ring-2 ring-white/90 ring-offset-2 ring-offset-[#070a16]'
                : 'opacity-75 hover:opacity-100 hover:scale-105',
            ]"
          />
        </button>
        <button
          type="button"
          @click="randomizeLook"
          class="btn-tactile ml-1 px-2.5 py-1 rounded-lg text-[10.5px] cursor-pointer"
        >
          随机搭配
        </button>
      </div>
    </div>

    <!-- Interactive Living 3D Archetype Grid -->
    <div class="grid grid-cols-3 sm:grid-cols-6 gap-2" role="group" aria-label="伙伴造型">
      <button
        v-for="sh in BOT_SHAPES"
        :key="sh.id"
        type="button"
        :title="`${sh.label} · ${sh.hint}`"
        :aria-label="sh.label"
        @pointermove="trackSpotlight"
        @click="pickShape(sh.id)"
        :aria-pressed="parsed.shape === sh.id"
        :class="[
          'group luxury-card rounded-xl p-2.5 flex flex-col items-center gap-1.5 cursor-pointer text-center focus-visible:outline focus-visible:outline-2 focus-visible:outline-[#f2b880]',
          parsed.shape === sh.id
            ? 'border-[#f2b880]/70 bg-[#f2b880]/14'
            : 'border-[#fff8ee]/10 opacity-88 hover:opacity-100',
        ]"
      >
        <BotAvatar :avatar-style="composeAvatarStyle(sh.id, parsed.color)" :state="previewState" size="md" :interactive="false" />
        <span :class="['text-[11px] leading-tight font-medium', parsed.shape === sh.id ? 'text-[#f7cba0]' : 'text-slate-200']">
          {{ sh.label }}
        </span>
        <span class="text-[9.5px] text-slate-500 leading-none">
          {{ sh.hint }}
        </span>
      </button>
    </div>
  </div>
</template>
