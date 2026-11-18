<script setup lang="ts">
import { computed, ref, useId, onMounted, onBeforeUnmount } from 'vue'
import { getColorPalette, parseAvatarStyle, type BotShape } from '../utils/botLook'
import { clampUnit, retainGlobalBotGaze } from '../utils/botGaze'

export type BotStuntKind = 'hop' | 'flip' | 'dance' | 'shy'

const STUNT_CYCLE: BotStuntKind[] = ['hop', 'flip', 'dance', 'shy']

const props = withDefaults(
  defineProps<{
    avatarStyle?: string
    state?: string
    size?: 'xs' | 'sm' | 'md' | 'lg' | 'xl'
    interactive?: boolean
    focusable?: boolean
  }>(),
  {
    avatarStyle: 'cyan',
    state: 'idle',
    size: 'md',
    interactive: true,
    focusable: false,
  }
)

// Every archetype is sculpted from ONE single C1-smooth continuous closed Bezier contour (zero separate stuck-on parts).
const MONOLITHIC_PATHS: Record<BotShape, string> = {
  // 1. Classic GrokBot plump mochi drop
  butler:
    'M 28 7.6 C 39.8 7.6, 48.8 18.2, 48.8 31.8 C 48.8 42.8, 40.8 48.6, 28 48.6 C 15.2 48.6, 7.2 42.8, 7.2 31.8 C 7.2 18.2, 16.2 7.6, 28 7.6 Z',
  // 2. Seamless one-piece molded neko/fox spirit (ears & cheeks flow from a single continuous curve)
  sentinel:
    'M 28 13.2 C 33 13.2, 36.4 7, 40.2 7 C 44.6 7, 48.6 18.6, 48.6 31.8 C 48.6 42.8, 40.6 48.6, 28 48.6 C 15.4 48.6, 7.4 42.8, 7.4 31.8 C 7.4 18.6, 11.4 7, 15.8 7 C 19.6 7, 23 13.2, 28 13.2 Z',
  // 3. Seamless liquid starlight teardrop spirit
  radar:
    'M 28 5.4 C 33.6 5.4, 48.6 18.8, 48.6 32.4 C 48.6 43, 40.4 48.6, 28 48.6 C 15.6 48.6, 7.4 43, 7.4 32.4 C 7.4 18.8, 22.4 5.4, 28 5.4 Z',
  // 4. Seamless twin-horned little imp spirit (outward-flared horn tips & chubby cheeks in one continuous contour)
  critic:
    'M 28 14.4 C 35.6 14.4, 40.8 5.8, 45 5.8 C 48.4 5.8, 46.2 16.4, 47.8 26.8 C 49.2 35.9, 43.4 48.6, 28 48.6 C 12.6 48.6, 6.8 35.9, 8.2 26.8 C 9.8 16.4, 7.6 5.8, 11 5.8 C 15.2 5.8, 20.4 14.4, 28 14.4 Z',
  // 5. Seamless fluffy cloud puffball spirit
  swarm:
    'M 28 6.4 C 34.4 6.4, 36.8 11.6, 41.2 13 C 46.8 14.8, 50.4 20.6, 50.4 28.4 C 50.4 36.2, 46.2 42.4, 40.4 45.4 C 36.4 47.5, 32.6 48.6, 28 48.6 C 23.4 48.6, 19.6 47.5, 15.6 45.4 C 9.8 42.4, 5.6 36.2, 5.6 28.4 C 5.6 20.6, 9.2 14.8, 14.8 13 C 19.2 11.6, 21.6 6.4, 28 6.4 Z',
  // 6. Seamless continuous superellipse jelly cube
  visor:
    'M 28 8.2 C 43.2 8.2, 47.8 12.8, 47.8 28 C 47.8 43.2, 43.2 47.8, 28 47.8 C 12.8 47.8, 8.2 43.2, 8.2 28 C 8.2 12.8, 12.8 8.2, 28 8.2 Z',
}

const uid = useId()
const isWinking = ref(false)
const activeStunt = ref<BotStuntKind | null>(null)
const stuntSeq = ref(0)
const localGaze = ref<{ x: number; y: number } | null>(null)

let winkTimer: number | null = null
let stuntTimer: number | null = null
let touchGazeTimer: number | null = null
let releaseGlobalGaze: (() => void) | null = null
let cachedBox = { w: 40, h: 40 }

// Deterministic phase offset per instance so multiple bots never move in robotic lockstep
const phaseDelaySec = computed(() => {
  let acc = 0
  const seed = `${uid}:${props.avatarStyle}`
  for (let i = 0; i < seed.length; i++) {
    acc = (acc * 31 + seed.charCodeAt(i)) % 97
  }
  return -((acc % 43) * 0.14).toFixed(2)
})

function playStunt(forced?: BotStuntKind) {
  const chosen = forced || STUNT_CYCLE[stuntSeq.value % STUNT_CYCLE.length]
  if (!forced) stuntSeq.value += 1
  isWinking.value = chosen !== 'shy'
  activeStunt.value = null
  requestAnimationFrame(() => {
    activeStunt.value = chosen
  })
  if (winkTimer !== null) window.clearTimeout(winkTimer)
  if (stuntTimer !== null) window.clearTimeout(stuntTimer)
  const durationMs = chosen === 'flip' ? 760 : chosen === 'dance' ? 820 : chosen === 'shy' ? 860 : 620
  winkTimer = window.setTimeout(() => {
    isWinking.value = false
  }, durationMs + 160)
  stuntTimer = window.setTimeout(() => {
    activeStunt.value = null
  }, durationMs)
}

function triggerPoke(e?: PointerEvent | KeyboardEvent) {
  if (!props.interactive) return
  if (e && 'pointerType' in e && e.pointerType === 'touch' && props.size !== 'xs') {
    const el = e.currentTarget as HTMLElement | null
    const w = Math.max(el?.clientWidth || cachedBox.w, 16)
    const h = Math.max(el?.clientHeight || cachedBox.h, 16)
    const nx = clampUnit((e.offsetX / w) * 2 - 1)
    const ny = clampUnit((e.offsetY / h) * 2 - 1)
    localGaze.value = { x: Number(nx.toFixed(3)), y: Number(ny.toFixed(3)) }
    if (touchGazeTimer !== null) window.clearTimeout(touchGazeTimer)
    touchGazeTimer = window.setTimeout(() => {
      localGaze.value = null
    }, 950)
  }
  playStunt()
}

defineExpose({ playStunt })

function handlePointerEnter(e: PointerEvent) {
  if (!props.interactive || props.size === 'xs') return
  const el = e.currentTarget as HTMLElement | null
  if (el) {
    cachedBox = {
      w: Math.max(el.clientWidth, 16),
      h: Math.max(el.clientHeight, 16),
    }
  }
}

function handlePointerMove(e: PointerEvent) {
  if (!props.interactive || props.size === 'xs' || e.pointerType === 'touch') return
  const nx = clampUnit((e.offsetX / cachedBox.w) * 2 - 1)
  const ny = clampUnit((e.offsetY / cachedBox.h) * 2 - 1)
  localGaze.value = {
    x: Number(nx.toFixed(3)),
    y: Number(ny.toFixed(3)),
  }
}

function handlePointerLeave() {
  localGaze.value = null
}

onMounted(() => {
  if (props.size === 'lg' || props.size === 'xl') {
    releaseGlobalGaze = retainGlobalBotGaze()
  }
})

onBeforeUnmount(() => {
  if (winkTimer !== null) window.clearTimeout(winkTimer)
  if (stuntTimer !== null) window.clearTimeout(stuntTimer)
  if (touchGazeTimer !== null) window.clearTimeout(touchGazeTimer)
  if (releaseGlobalGaze) {
    releaseGlobalGaze()
    releaseGlobalGaze = null
  }
})

const parsed = computed(() => parseAvatarStyle(props.avatarStyle))
const archetype = computed(() => parsed.value.shape)
const palette = computed(() => getColorPalette(parsed.value.color))
const isTiny = computed(() => props.size === 'xs')
const isDarkBody = computed(() => parsed.value.color === 'obsidian')
const isAmbientTracked = computed(() => props.size === 'lg' || props.size === 'xl')

const bodyPath = computed(() => MONOLITHIC_PATHS[archetype.value] || MONOLITHIC_PATHS.butler)
const effectiveState = computed(() => (isWinking.value ? 'done' : props.state || 'idle'))
const showSparkles = computed(() => !isTiny.value && (effectiveState.value === 'done' || activeStunt.value === 'hop' || activeStunt.value === 'flip' || activeStunt.value === 'dance'))
const showThoughtBubbles = computed(() => !isTiny.value && props.state === 'thinking' && !isWinking.value)
const showWorkArcs = computed(() => !isTiny.value && props.state === 'working' && !isWinking.value)
const showAlertMark = computed(() => !isTiny.value && props.state === 'blocked' && !isWinking.value)

const auroraColor = computed(() => {
  if (props.state === 'blocked' && !isWinking.value) return '#fb7185'
  if (effectiveState.value === 'done') return '#34d399'
  if (props.state === 'thinking') return '#fbbf24'
  return palette.value.aurora
})

const eyeFill = computed(() => {
  if (props.state === 'blocked' && !isWinking.value) {
    return isDarkBody.value ? '#fda4af' : '#be123c'
  }
  return palette.value.eyeInk
})

// Eye vertical anchor adjusts subtly so tall-crowned or horned silhouettes keep balanced GrokBot proportions
const eyeOffsetY = computed(() => {
  if (archetype.value === 'critic') return 1.8
  if (archetype.value === 'radar') return 1.2
  if (archetype.value === 'visor') return -0.6
  return 0.4
})

const sizeClass = computed(() => {
  if (props.size === 'xs') return 'w-4 h-4'
  if (props.size === 'sm') return 'w-8 h-8'
  if (props.size === 'lg') return 'w-14 h-14'
  if (props.size === 'xl') return 'w-28 h-28'
  return 'w-10 h-10'
})

const inlineVars = computed(() => {
  const vars: Record<string, string> = {
    '--bot-delay': `${phaseDelaySec.value}s`,
    '--bot-glow':
      props.state === 'blocked' && !isWinking.value
        ? 'rgba(244, 63, 94, 0.46)'
        : effectiveState.value === 'done'
          ? 'rgba(52, 211, 153, 0.42)'
          : palette.value.glow,
  }
  if (localGaze.value) {
    vars['--local-gx'] = String(localGaze.value.x)
    vars['--local-gy'] = String(localGaze.value.y)
  }
  return vars
})
</script>

<template>
  <div
    :style="inlineVars"
    :role="focusable ? 'button' : undefined"
    :tabindex="focusable ? 0 : undefined"
    :aria-label="focusable ? '点击互动切换 Bot 动作特技' : undefined"
    :aria-hidden="!focusable || undefined"
    @pointerenter="handlePointerEnter"
    @pointermove="handlePointerMove"
    @pointerleave="handlePointerLeave"
    @pointerdown="triggerPoke"
    @keydown.enter.prevent="triggerPoke"
    @keydown.space.prevent="triggerPoke"
    :class="[
      'bot-avatar-root shrink-0 relative inline-flex items-center justify-center select-none',
      sizeClass,
      `bot-avatar-${effectiveState}`,
      `bot-kind-${archetype}`,
      isAmbientTracked ? 'bot-gaze-global' : '',
      localGaze ? 'bot-gaze-local' : '',
      activeStunt ? `bot-stunt-${activeStunt}` : '',
      interactive ? 'bot-avatar-interactive cursor-pointer' : '',
    ]"
  >
    <!-- Soft outer ambient glow (unframed, zero hard border circle) -->
    <span
      v-if="!isTiny"
      aria-hidden="true"
      class="bot-aura pointer-events-none absolute -inset-1 rounded-full blur-md opacity-60"
    />

    <svg
      viewBox="0 0 56 56"
      fill="none"
      aria-hidden="true"
      class="bot-svg relative w-full h-full block overflow-visible"
    >
      <defs>
        <!-- Unified One-Piece Body Clip & Alpha Mask (Zero Seams) -->
        <clipPath :id="`${uid}-clip`">
          <path :d="bodyPath" />
        </clipPath>
        <mask :id="`${uid}-mask`" mask-type="alpha">
          <path :d="bodyPath" fill="#ffffff" />
        </mask>

        <!-- Continuous World-Space 3D Mochi/Porcelain Body Shader -->
        <radialGradient
          :id="`${uid}-body`"
          gradientUnits="userSpaceOnUse"
          cx="22"
          cy="17"
          r="34"
          fx="18"
          fy="13"
        >
          <stop offset="0%" :stop-color="palette.shellHi" />
          <stop offset="56%" :stop-color="palette.shellMid" />
          <stop offset="100%" :stop-color="palette.shellDeep" />
        </radialGradient>

        <!-- Living Internal Subsurface Aurora Light (Drifts Organically Inside Body) -->
        <radialGradient :id="`${uid}-aurora`" cx="50%" cy="50%" r="50%">
          <stop offset="0%" :stop-color="auroraColor" :stop-opacity="isDarkBody ? '0.52' : '0.42'" />
          <stop offset="62%" :stop-color="auroraColor" :stop-opacity="isDarkBody ? '0.18' : '0.12'" />
          <stop offset="100%" :stop-color="auroraColor" stop-opacity="0" />
        </radialGradient>

        <!-- Seamless Airbrushed Top Dome Lustre (Fades to 0% Opacity — No Hard Sticker Edge) -->
        <radialGradient :id="`${uid}-lustre`" cx="50%" cy="50%" r="50%">
          <stop offset="0%" stop-color="#ffffff" :stop-opacity="isDarkBody ? '0.32' : '0.82'" />
          <stop offset="55%" stop-color="#ffffff" :stop-opacity="isDarkBody ? '0.08' : '0.28'" />
          <stop offset="100%" stop-color="#ffffff" stop-opacity="0" />
        </radialGradient>

        <!-- Seamless Airbrushed Cheek Blush (Fades to 0% Opacity) -->
        <radialGradient :id="`${uid}-blush`" cx="50%" cy="50%" r="50%">
          <stop offset="0%" :stop-color="palette.blush" stop-opacity="0.62" />
          <stop offset="60%" :stop-color="palette.blush" stop-opacity="0.22" />
          <stop offset="100%" :stop-color="palette.blush" stop-opacity="0" />
        </radialGradient>

        <!-- Soft Rim Light Along Unified Silhouette Contour -->
        <linearGradient
          :id="`${uid}-rim`"
          gradientUnits="userSpaceOnUse"
          x1="12"
          y1="8"
          x2="44"
          y2="48"
        >
          <stop offset="0%" stop-color="#ffffff" :stop-opacity="isDarkBody ? '0.42' : '0.75'" />
          <stop offset="52%" :stop-color="auroraColor" stop-opacity="0.12" />
          <stop offset="100%" :stop-color="auroraColor" stop-opacity="0.55" />
        </linearGradient>
      </defs>

      <!-- Soft Floating Ground Shadow -->
      <ellipse
        v-if="!isTiny"
        cx="28"
        cy="51.8"
        rx="12.5"
        ry="2.2"
        class="bot-ground-shadow"
      />

      <!-- Unified Soft-Body Mochi Spirit Rig (3 Nested Compositor Layers: Float -> Personality Act -> Interactive Head) -->
      <g class="bot-float-rig">
        <g class="bot-act-rig">
          <g class="bot-head">
            <!-- Seamless Monolithic Interior (Clipped + Alpha-Masked to Single Silhouette Path) -->
            <g :clip-path="`url(#${uid}-clip)`" :mask="`url(#${uid}-mask)`">
              <!-- 1. Continuous Sculpted Base Material -->
              <path :d="bodyPath" :fill="`url(#${uid}-body)`" />

              <!-- 2. Living Subsurface Aurora Glow (Drifts & Pulses Inside the Spirit) -->
              <circle
                cx="35"
                cy="37"
                r="21"
                :fill="`url(#${uid}-aurora)`"
                class="bot-inner-aurora"
              />
              <circle
                v-if="state === 'working' || state === 'thinking'"
                cx="19"
                cy="20"
                r="18"
                :fill="`url(#${uid}-aurora)`"
                class="bot-inner-aurora-alt"
              />

              <!-- 3. Counter-Shifting Soft Dome Lustre -->
              <ellipse
                cx="21.5"
                cy="15.5"
                rx="13"
                ry="8.5"
                :fill="`url(#${uid}-lustre)`"
                class="bot-specular"
              />

              <!-- 4. Integrated Face Features: Airbrushed Cheek Blush + GrokBot Capsule Eyes -->
              <g :transform="`translate(0, ${eyeOffsetY})`">
                <g class="bot-face-rig">
                  <!-- Airbrushed Subsurface Cheek Glows -->
                  <g v-if="!isTiny" class="bot-blush">
                    <ellipse cx="15.4" cy="33.2" rx="6.8" ry="4.4" :fill="`url(#${uid}-blush)`" />
                    <ellipse cx="40.6" cy="33.2" rx="6.8" ry="4.4" :fill="`url(#${uid}-blush)`" />
                  </g>

                  <!-- Parallel GrokBot Capsule Eyes (Directly on the Seamless Face) -->
                  <g class="bot-eye-gaze">
                    <!-- Blocked / Alert Angled Capsules -->
                    <g v-if="state === 'blocked' && !isWinking">
                      <rect
                        x="18.6"
                        y="22.4"
                        width="5.2"
                        height="10.2"
                        rx="2.6"
                        transform="rotate(14 21.2 27.5)"
                        :fill="eyeFill"
                      />
                      <rect
                        x="32.2"
                        y="22.4"
                        width="5.2"
                        height="10.2"
                        rx="2.6"
                        transform="rotate(-14 34.8 27.5)"
                        :fill="eyeFill"
                      />
                    </g>

                    <!-- Shy Peek Squinted Horizontal Pill Eyes -->
                    <g v-else-if="activeStunt === 'shy'">
                      <rect x="17.2" y="27.2" width="7.2" height="3.6" rx="1.8" :fill="eyeFill" />
                      <rect x="31.6" y="27.2" width="7.2" height="3.6" rx="1.8" :fill="eyeFill" />
                    </g>

                    <!-- Done / Poke Happy Curved Crescent Pills -->
                    <g v-else-if="effectiveState === 'done'">
                      <path
                        d="M 16.8 28.8 C 17.6 23.4, 24.8 23.4, 25.6 28.8 C 25.8 30.1, 24.2 30.8, 23.4 29.7 C 22.3 27.6, 20.1 27.6, 19 29.7 C 18.2 30.8, 16.6 30.1, 16.8 28.8 Z"
                        :fill="eyeFill"
                      />
                      <path
                        d="M 30.4 28.8 C 31.2 23.4, 38.4 23.4, 39.2 28.8 C 39.4 30.1, 37.8 30.8, 37 29.7 C 35.9 27.6, 33.7 27.6, 32.6 29.7 C 31.8 30.8, 30.2 30.1, 30.4 28.8 Z"
                        :fill="eyeFill"
                      />
                    </g>

                    <!-- Signature GrokBot Vertical Parallel Capsule (Pill) Eyes -->
                    <g v-else class="bot-eye-pair">
                      <!-- Soft Optical Glow Behind Capsules on Dark Body or Active Working/Thinking -->
                      <template v-if="!isTiny && (isDarkBody || state === 'working' || state === 'thinking')">
                        <rect
                          x="17.8"
                          y="21"
                          width="7"
                          height="13.2"
                          rx="3.5"
                          :fill="auroraColor"
                          opacity="0.28"
                        />
                        <rect
                          x="31.2"
                          y="21"
                          width="7"
                          height="13.2"
                          rx="3.5"
                          :fill="auroraColor"
                          opacity="0.28"
                        />
                      </template>

                      <!-- Left Capsule Eye -->
                      <g class="bot-eye-l">
                        <rect
                          x="18.8"
                          y="22"
                          width="5"
                          height="11.2"
                          rx="2.5"
                          :fill="eyeFill"
                        />
                        <circle
                          v-if="!isTiny"
                          cx="20.5"
                          cy="24.7"
                          r="1.15"
                          fill="#ffffff"
                          :opacity="isDarkBody ? '0.96' : '0.88'"
                        />
                      </g>

                      <!-- Right Capsule Eye -->
                      <g class="bot-eye-r">
                        <rect
                          x="32.2"
                          y="22"
                          width="5"
                          height="11.2"
                          rx="2.5"
                          :fill="eyeFill"
                        />
                        <circle
                          v-if="!isTiny"
                          cx="33.9"
                          cy="24.7"
                          r="1.15"
                          fill="#ffffff"
                          :opacity="isDarkBody ? '0.96' : '0.88'"
                        />
                      </g>
                    </g>
                  </g>
                </g>
              </g>
            </g>

            <!-- Ultra-Subtle Continuous Rim Sheen on the Single Unified Contour -->
            <path
              :d="bodyPath"
              :stroke="`url(#${uid}-rim)`"
              stroke-width="0.9"
              fill="none"
            />
            <path
              :d="bodyPath"
              class="bot-light-outline"
              stroke-width="0.85"
              fill="none"
            />

            <!-- Expressive Geometric Vector Emotes (Outside ClipPath — Zero Unicode Emojis) -->
            <g pointer-events="none">
              <!-- Happy / Stunt Twinkling Starlight Motes -->
              <g v-if="showSparkles">
                <path
                  d="M 48.5 8.5 Q 48.5 12.2 52.2 12.2 Q 48.5 12.2 48.5 15.9 Q 48.5 12.2 44.8 12.2 Q 48.5 12.2 48.5 8.5 Z"
                  :fill="palette.ring"
                  class="bot-sparkle-a"
                />
                <path
                  d="M 7.5 13.5 Q 7.5 16 10 16 Q 7.5 16 7.5 18.5 Q 7.5 16 5 16 Q 7.5 16 7.5 13.5 Z"
                  fill="#ffffff"
                  opacity="0.9"
                  class="bot-sparkle-b"
                />
              </g>

              <!-- Thinking Rising Soft Thought Pearls -->
              <g v-if="showThoughtBubbles">
                <circle cx="44.5" cy="14.5" r="1.5" :fill="palette.shellHi" opacity="0.78" class="bot-thought-1" />
                <circle cx="48.5" cy="10.2" r="2.2" :fill="palette.shellHi" opacity="0.88" class="bot-thought-2" />
                <circle cx="52.2" cy="5.4" r="3" :fill="auroraColor" opacity="0.92" class="bot-thought-3" />
              </g>

              <!-- Working Kinetic Focus Arcs -->
              <g v-if="showWorkArcs">
                <path
                  d="M 5.2 23 A 24 24 0 0 1 11.5 9.5"
                  :stroke="auroraColor"
                  stroke-width="1.8"
                  stroke-linecap="round"
                  opacity="0.78"
                  class="bot-work-arc-l"
                />
                <path
                  d="M 50.8 33 A 24 24 0 0 1 44.5 46.5"
                  :stroke="palette.ring"
                  stroke-width="1.8"
                  stroke-linecap="round"
                  opacity="0.78"
                  class="bot-work-arc-r"
                />
              </g>

              <!-- Blocked / Alert Geometric Beacon -->
              <g v-if="showAlertMark" class="bot-emote-alert">
                <rect x="26.4" y="0.5" width="3.2" height="6.2" rx="1.6" fill="#fb7185" />
                <circle cx="28" cy="9.2" r="1.5" fill="#fb7185" />
              </g>
            </g>
          </g>
        </g>
      </g>
    </svg>
  </div>
</template>
