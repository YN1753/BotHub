<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'
import type { Bot } from '../types/bothub'
import BotAvatar from './BotAvatar.vue'

const props = withDefaults(
  defineProps<{
    members: Bot[]
    fallbackStyle?: string
    state?: string
    size?: 'sm' | 'md' | 'lg' | 'xl'
    activeSpeakerId?: string
  }>(),
  {
    fallbackStyle: 'swarm:indigo',
    state: 'idle',
    size: 'md',
    activeSpeakerId: '',
  }
)

const pinnedLeadId = ref<string>('')
const touchPeeking = ref(false)
let touchPeekTimer: number | null = null
const displayMembers = computed(() => props.members.slice(0, 4))

onBeforeUnmount(() => {
  if (touchPeekTimer !== null) window.clearTimeout(touchPeekTimer)
})

// Determine who currently takes Front-Center C-Position (Slot 0):
// 1. Explicit activeSpeakerId during live streaming/relay
// 2. Member currently in working/thinking state
// 3. User-clicked spirit in interactive stage mode
// 4. First member in group
const currentLeadId = computed(() => {
  const list = displayMembers.value
  if (!list.length) return ''
  if (props.activeSpeakerId && list.some((m) => m.id === props.activeSpeakerId)) {
    return props.activeSpeakerId
  }
  const workingMember = list.find((m) => m.state === 'working' || m.state === 'thinking')
  if (workingMember) return workingMember.id
  if (pinnedLeadId.value && list.some((m) => m.id === pinnedLeadId.value)) {
    return pinnedLeadId.value
  }
  return list[0].id
})

const hasActiveSpeaker = computed(() => {
  const list = displayMembers.value
  if (props.activeSpeakerId && list.some((m) => m.id === props.activeSpeakerId)) return true
  return list.some((m) => m.state === 'working' || m.state === 'thinking')
})

// Stable DOM order (keyed by m.id) -> dynamic slot index (0 = Front Lead, 1..3 = Peeking Buddies)
const slotMap = computed(() => {
  const map = new Map<string, number>()
  const list = displayMembers.value
  const lead = currentLeadId.value
  let nextBackSlot = 1
  for (const m of list) {
    if (m.id === lead) {
      map.set(m.id, 0)
    } else {
      map.set(m.id, nextBackSlot++)
    }
  }
  return map
})

interface SlotPose {
  tx: string
  ty: string
  sc: number
  rot: string
  z: number
}

function resolvePose(slot: number, total: number): SlotPose {
  if (total === 2) {
    return slot === 0
      ? { tx: '15%', ty: '11%', sc: 0.77, rot: '6deg', z: 30 }
      : { tx: '-17%', ty: '-10%', sc: 0.69, rot: '-13deg', z: 10 }
  }
  if (total === 3) {
    if (slot === 0) return { tx: '0%', ty: '14%', sc: 0.73, rot: '0deg', z: 30 }
    if (slot === 1) return { tx: '-24%', ty: '-10%', sc: 0.63, rot: '-15deg', z: 15 }
    return { tx: '24%', ty: '-10%', sc: 0.63, rot: '15deg', z: 14 }
  }
  // 4+ members: snug Tsum-Tsum cuddle pyramid (zero central hole!)
  if (slot === 0) return { tx: '0%', ty: '15%', sc: 0.71, rot: '0deg', z: 30 }
  if (slot === 1) return { tx: '-25%', ty: '-1%', sc: 0.61, rot: '-16deg', z: 20 }
  if (slot === 2) return { tx: '25%', ty: '-1%', sc: 0.61, rot: '16deg', z: 19 }
  return { tx: '0%', ty: '-18%', sc: 0.58, rot: '3deg', z: 10 }
}

function memberVars(id: string): Record<string, string | number> {
  const slot = slotMap.value.get(id) ?? 0
  const pose = resolvePose(slot, displayMembers.value.length)
  const isSpeakingLead = slot === 0 && hasActiveSpeaker.value
  return {
    '--m-tx': pose.tx,
    '--m-ty': isSpeakingLead ? `calc(${pose.ty} - 4%)` : pose.ty,
    '--m-sc': isSpeakingLead ? Number((pose.sc * 1.08).toFixed(3)) : pose.sc,
    '--m-rot': pose.rot,
    zIndex: isSpeakingLead ? 40 : pose.z,
  }
}

function memberEffectiveState(m: Bot): string {
  if (props.activeSpeakerId && m.id === props.activeSpeakerId) return 'working'
  if (m.state && m.state !== 'idle') return m.state
  if (slotMap.value.get(m.id) === 0 && props.state && props.state !== 'idle') return props.state
  return 'idle'
}

function handleMemberClick(id: string) {
  touchPeeking.value = true
  if (touchPeekTimer !== null) window.clearTimeout(touchPeekTimer)
  touchPeekTimer = window.setTimeout(() => {
    touchPeeking.value = false
  }, 950)
  if (props.size !== 'xl' && props.size !== 'lg') return
  pinnedLeadId.value = id
}

const containerSizeClass = computed(() => {
  if (props.size === 'sm') return 'w-8 h-7'
  if (props.size === 'lg') return 'w-14 h-12'
  if (props.size === 'xl') return 'w-32 h-28 sm:w-36 sm:h-30'
  return 'w-10 h-9'
})

const isHeroInteractive = computed(() => props.size === 'xl' || props.size === 'lg')
</script>

<template>
  <BotAvatar
    v-if="displayMembers.length <= 1"
    :avatar-style="displayMembers[0]?.avatar_style || fallbackStyle"
    :state="state"
    :size="size"
    :focusable="size === 'xl'"
  />

  <!-- Unboxed Snuggled Mochi Cluster (叠叠乐贴贴团: Zero Box, Zero Circles, Live C-Position Speaker Swap) -->
  <div
    v-else
    :class="[
      'bot-mochi-cluster shrink-0 relative inline-block select-none overflow-visible',
      containerSizeClass,
      hasActiveSpeaker ? 'cluster-has-speaker' : '',
      touchPeeking ? 'is-touch-peeking' : '',
    ]"
  >
    <!-- Shared Soft Cozy Ground Shadow Under the Cuddle Pile -->
    <span
      aria-hidden="true"
      class="pointer-events-none absolute left-1/2 -bottom-0.5 -translate-x-1/2 w-[76%] h-[14%] rounded-full bg-black/55 blur-[3px]"
    />

    <div
      v-for="m in displayMembers"
      :key="m.id"
      :style="memberVars(m.id)"
      :title="isHeroInteractive ? `${m.name}（点击切至C位互动）` : undefined"
      @pointerdown="handleMemberClick(m.id)"
      :class="[
        'mochi-cluster-item absolute inset-0 flex items-center justify-center',
        (slotMap.get(m.id) ?? 0) === 0 ? 'is-front-lead' : 'is-peeking-buddy',
        (slotMap.get(m.id) ?? 0) === 0 && hasActiveSpeaker ? 'is-speaking-hop' : '',
      ]"
    >
      <BotAvatar
        :avatar-style="m.avatar_style"
        :state="memberEffectiveState(m)"
        :size="size === 'xl' ? 'lg' : 'sm'"
        :interactive="isHeroInteractive"
        :focusable="size === 'xl'"
        class="!w-full !h-full"
      />
    </div>

    <span
      v-if="members.length > 4"
      :class="[
        'btn-jewel pointer-events-none absolute -bottom-0.5 -right-1 z-50 rounded-full font-mono leading-none',
        size === 'xl' ? 'px-2 py-0.5 text-[10.5px]' : 'px-1 py-0.5 text-[8.5px]',
      ]"
    >
      +{{ members.length - 4 }}
    </span>
  </div>
</template>
