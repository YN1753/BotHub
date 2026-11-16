<script setup lang="ts">
import { computed } from 'vue'
import {
  BOT_COLORS,
  BOT_SHAPES,
  composeAvatarStyle,
  parseAvatarStyle,
  type BotColor,
  type BotShape,
} from '../utils/botLook'
import { trackSpotlight } from '../utils/topLayer'
import BotAvatar from './BotAvatar.vue'

const props = defineProps<{
  modelValue: string
  previewName?: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

const parsed = computed(() => parseAvatarStyle(props.modelValue))

function pickShape(shape: BotShape) {
  emit('update:modelValue', composeAvatarStyle(shape, parsed.value.color))
}

function pickColor(color: BotColor) {
  emit('update:modelValue', composeAvatarStyle(parsed.value.shape, color))
}
</script>

<template>
  <div class="modal-section flex flex-col gap-3.5">
    <div class="flex items-center justify-between gap-3">
      <div class="flex items-center gap-3 min-w-0">
        <BotAvatar :avatar-style="modelValue" state="idle" size="lg" />
        <div class="min-w-0">
          <div class="text-xs font-semibold text-white truncate">
            {{ previewName || '自定义形象预览' }}
          </div>
          <div class="text-[11px] text-slate-400 mt-0.5">
            选择机甲造型与 Vesper 光学护盾配色
          </div>
        </div>
      </div>

      <!-- Color Swatch Dots -->
      <div class="flex items-center gap-1.5 shrink-0" role="group" aria-label="配色光效">
        <button
          v-for="c in BOT_COLORS"
          :key="c.id"
          type="button"
          :title="c.label"
          :aria-label="c.label"
          :aria-pressed="parsed.color === c.id"
          @click="pickColor(c.id)"
          :style="{ backgroundColor: c.ring }"
          :class="[
            'w-6 h-6 rounded-full cursor-pointer transition-transform duration-200',
            parsed.color === c.id
              ? 'scale-110 ring-2 ring-white/90 ring-offset-2 ring-offset-[#070a16]'
              : 'opacity-70 hover:opacity-100 hover:scale-105'
          ]"
        />
      </div>
    </div>

    <!-- Interactive Living Shape Grid -->
    <div class="grid grid-cols-3 sm:grid-cols-6 gap-2">
      <button
        v-for="sh in BOT_SHAPES"
        :key="sh.id"
        type="button"
        @pointermove="trackSpotlight"
        @click="pickShape(sh.id)"
        :aria-pressed="parsed.shape === sh.id"
        :class="[
          'group luxury-card rounded-xl p-2 flex flex-col items-center gap-1.5 cursor-pointer text-center',
          parsed.shape === sh.id
            ? 'border-[#f2b880]/70 bg-[#f2b880]/14'
            : 'border-[#fff8ee]/10 opacity-85 hover:opacity-100'
        ]"
      >
        <BotAvatar :avatar-style="composeAvatarStyle(sh.id, parsed.color)" state="idle" size="md" />
        <span :class="['text-[11px] leading-tight font-medium', parsed.shape === sh.id ? 'text-[#f7cba0]' : 'text-slate-300']">
          {{ sh.label }}
        </span>
      </button>
    </div>
  </div>
</template>
