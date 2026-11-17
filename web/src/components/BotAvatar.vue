<script setup lang="ts">
import { computed, ref, onBeforeUnmount } from 'vue'
import { getColorPalette, parseAvatarStyle } from '../utils/botLook'

const props = withDefaults(
  defineProps<{
    avatarStyle?: string
    state?: string
    size?: 'xs' | 'sm' | 'md' | 'lg' | 'xl'
    interactive?: boolean
  }>(),
  {
    avatarStyle: 'cyan',
    state: 'idle',
    size: 'md',
    interactive: true,
  }
)

const isWinking = ref(false)
let winkTimer: number | null = null

function triggerWink() {
  if (!props.interactive) return
  isWinking.value = true
  if (winkTimer !== null) window.clearTimeout(winkTimer)
  winkTimer = window.setTimeout(() => {
    isWinking.value = false
  }, 850)
}

onBeforeUnmount(() => {
  if (winkTimer !== null) window.clearTimeout(winkTimer)
})

const parsed = computed(() => parseAvatarStyle(props.avatarStyle))
const archetype = computed(() => parsed.value.shape)
const palette = computed(() => getColorPalette(parsed.value.color))

const ringColor = computed(() => {
  if (props.state === 'blocked') return '#f87171'
  if (props.state === 'done') return '#34d399'
  if (props.state === 'thinking') return '#fbbf24'
  return palette.value.ring
})

const sizeClass = computed(() => {
  if (props.size === 'xs') return 'w-3.5 h-3.5'
  if (props.size === 'sm') return 'w-7 h-7'
  if (props.size === 'lg') return 'w-11 h-11'
  if (props.size === 'xl') return 'w-20 h-20'
  return 'w-9 h-9'
})
</script>

<template>
  <div
    :style="{ '--bot-glow': palette.glow }"
    :role="interactive ? 'button' : undefined"
    :tabindex="interactive ? 0 : undefined"
    :aria-label="interactive ? '点击触发互动眨眼' : undefined"
    :aria-hidden="!interactive || undefined"
    @click="triggerWink"
    @keydown.enter.prevent="triggerWink"
    @keydown.space.prevent="triggerWink"
    :class="[
      'bot-avatar-root shrink-0 relative inline-flex items-center justify-center select-none',
      sizeClass,
      `bot-avatar-${isWinking ? 'done' : state}`,
      `bot-kind-${archetype}`,
      interactive ? 'bot-avatar-interactive cursor-pointer' : ''
    ]"
  >
    <!-- Soft ambient aura behind hero/large avatars -->
    <span
      v-if="size === 'xl' || size === 'lg'"
      aria-hidden="true"
      class="bot-aura pointer-events-none absolute inset-0 rounded-full blur-md opacity-60"
    />

    <svg viewBox="0 0 48 48" fill="none" class="bot-svg relative w-full h-full block overflow-visible">
      <!-- Outer orbital aura ring -->
      <circle
        cx="24"
        cy="24"
        r="21"
        stroke="currentColor"
        stroke-width="0.9"
        stroke-dasharray="3 5"
        :style="{ color: ringColor }"
        class="bot-orbit opacity-45"
      />

      <!-- Top Archetype Antennas / Horns / Crown -->
      <g class="bot-crest" :stroke="ringColor" stroke-width="1.7" stroke-linecap="round">
        <!-- Butler: sleek celestial antenna + pearl -->
        <template v-if="archetype === 'butler'">
          <line x1="24" y1="6" x2="24" y2="2" />
          <circle cx="24" cy="1.8" r="1.4" :fill="ringColor" />
        </template>
        <!-- Sentinel: twin tactical sensor ears -->
        <template v-else-if="archetype === 'sentinel'">
          <path d="M11 12 L8 6" />
          <path d="M37 12 L40 6" />
          <circle cx="8" cy="5.5" r="1.1" :fill="ringColor" />
          <circle cx="40" cy="5.5" r="1.1" :fill="ringColor" />
        </template>
        <!-- Radar: directional parabolic dish -->
        <template v-else-if="archetype === 'radar'">
          <path d="M18 5.5 Q24 2 30 5.5" />
          <line x1="24" y1="6.5" x2="24" y2="3" />
        </template>
        <!-- Critic: sharp energetic horn fin -->
        <template v-else-if="archetype === 'critic'">
          <path d="M18 7 L21 2.5 L24 6.5 L27 2.5 L30 7" stroke-linejoin="round" />
        </template>
        <!-- Swarm: twin orbiting satellites -->
        <template v-else-if="archetype === 'swarm'">
          <circle cx="9" cy="9" r="1.6" :fill="ringColor" stroke="none" class="bot-sat-a" />
          <circle cx="39" cy="39" r="1.6" :fill="ringColor" stroke="none" class="bot-sat-b" />
        </template>
        <!-- Visor: sleek twin cyber side fins -->
        <template v-else>
          <path d="M5 20 L8 17 L8 27 L5 24 Z" :fill="ringColor" stroke="none" opacity="0.8" />
          <path d="M43 20 L40 17 L40 27 L43 24 Z" :fill="ringColor" stroke="none" opacity="0.8" />
        </template>
      </g>

      <!-- Main Chassis Head -->
      <g class="bot-head">
        <rect
          x="7.5"
          y="7.5"
          width="33"
          height="33"
          :rx="archetype === 'visor' ? 11 : 16.5"
          :fill="palette.fill"
          :stroke="ringColor"
          stroke-width="1.9"
          class="bot-ring"
        />

        <!-- Inner Visor Glass Plate -->
        <rect
          :x="archetype === 'visor' ? 10.5 : 12"
          :y="archetype === 'visor' ? 16.5 : 15"
          :width="archetype === 'visor' ? 27 : 24"
          :height="archetype === 'visor' ? 13 : 16.5"
          :rx="archetype === 'visor' ? 6.5 : 8"
          fill="rgba(6, 9, 19, 0.78)"
          :stroke="palette.visor"
          stroke-width="1"
        />

        <!-- Facial Expression Layer -->
        <g class="bot-eyes">
          <!-- Blocked / Alert expression -->
          <template v-if="state === 'blocked' && !isWinking">
            <path d="M16 19.5 L20 22.5 L16 25.5" :stroke="ringColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" />
            <path d="M32 19.5 L28 22.5 L32 25.5" :stroke="ringColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" />
            <path d="M19 29 Q24 26.5 29 29" :stroke="ringColor" stroke-width="1.6" stroke-linecap="round" />
          </template>

          <!-- Done / Wink Happy crescent expression -->
          <template v-else-if="state === 'done' || isWinking">
            <path d="M15.5 22.5 Q18.5 19 21.5 22.5" :stroke="palette.eye" stroke-width="2.2" stroke-linecap="round" />
            <path d="M26.5 22.5 Q29.5 19 32.5 22.5" :stroke="palette.eye" stroke-width="2.2" stroke-linecap="round" />
            <path d="M20 27.2 Q24 30 28 27.2" :stroke="ringColor" stroke-width="1.7" stroke-linecap="round" />
          </template>

          <!-- Visor Archetype Luminous LED Bar Eyes -->
          <template v-else-if="archetype === 'visor'">
            <g class="bot-eye-pair">
              <rect x="15" y="21" width="6.5" height="3.2" rx="1.6" :fill="palette.eye" />
              <rect x="26.5" y="21" width="6.5" height="3.2" rx="1.6" :fill="palette.eye" />
            </g>
            <line x1="21" y1="33" x2="27" y2="33" :stroke="ringColor" stroke-width="1.6" stroke-linecap="round" opacity="0.85" />
          </template>

          <!-- Standard Active Idle / Thinking / Working Expressive Blinking Eyes -->
          <template v-else>
            <g class="bot-eye-pair">
              <ellipse cx="18.5" cy="22" rx="2.4" ry="2.9" :fill="palette.eye" class="bot-eye-l" />
              <ellipse cx="29.5" cy="22" rx="2.4" ry="2.9" :fill="palette.eye" class="bot-eye-r" />
            </g>
            <path
              v-if="archetype === 'critic'"
              d="M19.5 27.5 Q24.5 29.8 29 26.6"
              :stroke="ringColor"
              stroke-width="1.65"
              stroke-linecap="round"
            />
            <line
              v-else-if="archetype === 'sentinel'"
              x1="20.5"
              y1="27.5"
              x2="27.5"
              y2="27.5"
              :stroke="ringColor"
              stroke-width="1.7"
              stroke-linecap="round"
              stroke-dasharray="2 2"
            />
            <path
              v-else
              d="M20 27.2 Q24 29.4 28 27.2"
              :stroke="ringColor"
              stroke-width="1.65"
              stroke-linecap="round"
              opacity="0.88"
            />
          </template>
        </g>
      </g>
    </svg>
  </div>
</template>
