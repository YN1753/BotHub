import { ref, computed, type Ref } from 'vue'
import { BotHubApi } from '../api/client'
import { mapServerMessages, type ChatMessageUI } from '../utils/chatMapper'
import { formatToolHumanName } from '../utils/toolMeta'
import type { useAppStore } from '../stores/useAppStore'
import type { RelayStepItem } from '../components/workspace/BotRelayStrip.vue'

export interface ReplyQuoteTarget {
  serverId: number
  senderName: string
  preview: string
}

export function useBotChatStream(options: {
  store: ReturnType<typeof useAppStore>
  inputPrompt: Ref<string>
  selectedModel: Ref<string>
  enableThinking: Ref<boolean>
  replyTarget: Ref<ReplyQuoteTarget | null>
  scrollToBottom: (force?: boolean) => void
  onClosePopovers?: () => void
}) {
  const {
    store,
    inputPrompt,
    selectedModel,
    enableThinking,
    replyTarget,
    scrollToBottom,
    onClosePopovers,
  } = options

  const streamStatus = ref('就绪')
  const lastPrompt = ref('')
  const relaySteps = ref<RelayStepItem[]>([])
  let abortCtrl: AbortController | null = null

  const activeSpeakerBotId = computed(() => {
    const activeStep = relaySteps.value.find((s) => s.status === 'active')
    if (activeStep?.id) return activeStep.id
    const lastMsg = store.state.timeline[store.state.timeline.length - 1]
    if (lastMsg?.streaming && lastMsg.senderBotId) return lastMsg.senderBotId
    return ''
  })

  function stopStream() {
    abortCtrl?.abort()
  }

  async function sendPrompt(
    overrideText?: string,
    turnOpts?: {
      overrideModel?: string
      allowPaidOnce?: boolean
      replaceAssistantMsg?: ChatMessageUI
    }
  ) {
    if (store.state.isStreaming) return
    const msg = (overrideText ?? inputPrompt.value).trim()
    if (!msg) return

    const curBot = store.activeBot.value
    if (!curBot) {
      store.notify('请先创建一个 Bot 再开始对话')
      return
    }
    const botId = curBot.id
    const activeThreadSnapshot = store.state.activeThreadId || ''
    const quote = replyTarget.value
    const effectiveModel = (turnOpts?.overrideModel || selectedModel.value || 'auto:free').trim()
    lastPrompt.value = msg
    if (!overrideText) {
      inputPrompt.value = ''
      store.setBotDraft(botId, '', activeThreadSnapshot)
    }
    onClosePopovers?.()
    replyTarget.value = null
    relaySteps.value = []

    // In-place replacement when retrying with another model: remove the old assistant reply and its paired user turn
    if (turnOpts?.replaceAssistantMsg) {
      const targetMsg = turnOpts.replaceAssistantMsg
      const aIdx = store.state.timeline.findIndex((m) => m.localId === targetMsg.localId)
      if (aIdx >= 0) {
        let uIdx = -1
        for (let i = aIdx - 1; i >= 0; i--) {
          if (store.state.timeline[i]?.role === 'user') {
            uIdx = i
            break
          }
        }
        const userMsg = uIdx >= 0 ? store.state.timeline[uIdx] : null
        if (targetMsg.serverId) {
          await BotHubApi.deleteBotMessage(botId, targetMsg.serverId).catch(() => {})
        }
        if (userMsg?.serverId) {
          await BotHubApi.deleteBotMessage(botId, userMsg.serverId).catch(() => {})
        }
        store.state.timeline = store.state.timeline.filter(
          (m) => m.localId !== targetMsg.localId && (!userMsg || m.localId !== userMsg.localId)
        )
      }
    }

    store.state.isStreaming = true
    abortCtrl = new AbortController()
    streamStatus.value = `连线 [${curBot.name || botId}]...`

    store.state.timeline.push({
      localId: `u_${Date.now()}`,
      role: 'user',
      senderBotId: 'user',
      senderName: '管理员',
      avatarStyle: 'cyan',
      model: effectiveModel,
      msgKind: 'chat',
      createdAt: new Date().toISOString(),
      content: msg,
      reasoning: '',
      tools: [],
      replyToId: quote?.serverId,
      replyPreview: quote?.preview,
      statusText: '已发送',
      isLiveNew: true,
    })

    const createTurn = (
      bId: string,
      bName: string,
      style: string,
      roleTitle = '',
      isTempGuest = false
    ): ChatMessageUI => {
      const turn: ChatMessageUI = {
        localId: `a_${Date.now()}_${Math.random().toString(36).slice(2, 6)}`,
        role: 'assistant',
        senderBotId: bId,
        senderName: bName,
        roleTitle,
        avatarStyle: style,
        model: effectiveModel,
        msgKind: 'chat',
        createdAt: new Date().toISOString(),
        content: '',
        reasoning: '',
        tools: [],
        isTempGuest,
        statusText: '思考与生成中...',
        streaming: true,
        isLiveNew: true,
      }
      store.state.timeline.push(turn)
      return store.state.timeline[store.state.timeline.length - 1]
    }

    let activeTurn = createTurn(
      botId,
      curBot.name || 'BotHub',
      curBot.avatar_style || 'cyan',
      curBot.role_title || ''
    )
    const turnStartedAt = Date.now()
    scrollToBottom(true)

    let pendingReasoningDelta = ''
    let pendingContentDelta = ''
    let rafHandle: number | null = null

    const flushPendingDeltas = () => {
      if (rafHandle !== null && typeof window !== 'undefined') {
        window.cancelAnimationFrame(rafHandle)
        rafHandle = null
      }
      if (!pendingReasoningDelta && !pendingContentDelta) return
      if (pendingReasoningDelta) {
        activeTurn.reasoning += pendingReasoningDelta
        pendingReasoningDelta = ''
      }
      if (pendingContentDelta) {
        activeTurn.content += pendingContentDelta
        pendingContentDelta = ''
      }
      scrollToBottom()
    }

    const scheduleDeltaFlush = () => {
      if (typeof window === 'undefined' || typeof window.requestAnimationFrame !== 'function') {
        flushPendingDeltas()
        return
      }
      if (rafHandle !== null) return
      rafHandle = window.requestAnimationFrame(() => {
        rafHandle = null
        flushPendingDeltas()
      })
    }

    try {
      const res = await fetch('/api/agent/chat', {
        method: 'POST',
        credentials: 'same-origin',
        signal: abortCtrl.signal,
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          bot_id: botId,
          thread_id: store.state.activeThreadId || '',
          message: msg,
          model: effectiveModel,
          enable_thinking: enableThinking.value,
          ...(turnOpts?.allowPaidOnce ? { allow_paid_fallback: true } : {}),
          reply_to_id: quote?.serverId || 0,
          reply_preview: quote?.preview || '',
        }),
      })
      if (!res.ok) {
        const errData = await res.json().catch(() => ({}))
        throw new Error(errData.error || `HTTP ${res.status}`)
      }

      const reader = res.body!.getReader()
      const decoder = new TextDecoder('utf-8')
      let buffer = ''
      let currentEvent = 'message'
      let modelSwitchedInTurn = false

      while (true) {
        const { value, done } = await reader.read()
        if (done) break
        buffer += decoder.decode(value, { stream: true })
        const lines = buffer.split('\n')
        buffer = lines.pop() || ''

        for (const rawLine of lines) {
          const line = rawLine.trim()
          if (!line) continue
          if (line.startsWith('event:')) {
            currentEvent = line.slice(6).trim()
          } else if (line.startsWith('data:')) {
            let payload: any = {}
            try {
              payload = JSON.parse(line.slice(5).trim())
            } catch {
              continue
            }

            if (currentEvent === 'system_notice') {
              flushPendingDeltas()
              const idx = store.state.timeline.indexOf(activeTurn)
              const sysItem: ChatMessageUI = {
                localId: `srv_${payload.id || Date.now()}`,
                serverId: payload.id,
                role: 'system',
                senderBotId: botId,
                senderName: '系统动态',
                avatarStyle: curBot.avatar_style || 'cyan',
                model: selectedModel.value,
                msgKind: 'system',
                createdAt: payload.created_at || new Date().toISOString(),
                content: payload.content || '',
                reasoning: '',
                tools: [],
                statusText: '',
                isLiveNew: true,
              }
              if (idx >= 0) store.state.timeline.splice(idx, 0, sysItem)
              else store.state.timeline.push(sysItem)
            } else if (currentEvent === 'bot_turn') {
              flushPendingDeltas()
              relaySteps.value.forEach((s) => {
                s.status = 'done'
              })
              relaySteps.value.push({
                id: payload.sender_bot_id,
                name: payload.sender_name,
                status: 'active',
                guest: Boolean(payload.is_temp_guest),
              })
              if (Number(payload.step) > 1) {
                activeTurn.statusText = '已完成'
                activeTurn.streaming = false
                activeTurn = createTurn(
                  payload.sender_bot_id,
                  payload.sender_name,
                  payload.avatar_style || 'cyan',
                  payload.role_title || '',
                  Boolean(payload.is_temp_guest)
                )
              } else {
                activeTurn.senderBotId = payload.sender_bot_id
                activeTurn.senderName = payload.sender_name
                activeTurn.roleTitle = payload.role_title || ''
                activeTurn.avatarStyle = payload.avatar_style || 'cyan'
                activeTurn.isTempGuest = Boolean(payload.is_temp_guest)
              }
              streamStatus.value = `协同接力 (${payload.step}/${payload.total_steps}): ${payload.sender_name}`
            } else if (currentEvent === 'reasoning') {
              streamStatus.value = '深度思考中...'
              pendingReasoningDelta += payload.delta || ''
              scheduleDeltaFlush()
            } else if (currentEvent === 'tool_start') {
              flushPendingDeltas()
              const humanTool = formatToolHumanName(String(payload.name || ''), store.state.skills || [])
              streamStatus.value = `正在${humanTool}...`
              activeTurn.tools.push({
                id: payload.id,
                name: payload.name,
                arguments: payload.arguments,
                done: false,
              })
              scrollToBottom()
            } else if (currentEvent === 'tool_result') {
              const found = activeTurn.tools.find((t) => t.id === payload.id)
              if (found) {
                found.done = true
                found.result = payload.result
                if (payload.duration_ms !== undefined) found.durationMs = Number(payload.duration_ms)
                if (payload.status) found.status = String(payload.status)
              }
            } else if (currentEvent === 'content') {
              streamStatus.value = '正在生成回复...'
              pendingContentDelta += payload.delta || ''
              scheduleDeltaFlush()
            } else if (currentEvent === 'model_switch') {
              modelSwitchedInTurn = true
              const fromLabel = payload.from_label || '原选模型'
              const toLabel = payload.to_label || payload.to_model || '备用模型'
              const reason = String(payload.reason || '').trim().slice(0, 60)
              const notice = reason
                ? `${fromLabel} 不可用（${reason}），已自动切换至 ${toLabel}`
                : `${fromLabel} 限流或异常，已自动切换至 ${toLabel}`
              streamStatus.value = notice
              if (payload.to_model) activeTurn.model = `${payload.to_model} (已回退)`
              store.notify(notice)
            } else if (currentEvent === 'error') {
              flushPendingDeltas()
              activeTurn.statusText = '异常中断'
              activeTurn.errorText = payload.error || '未知流式错误'
              activeTurn.streaming = false
            } else if (currentEvent === 'done') {
              flushPendingDeltas()
              activeTurn.streaming = false
              if (payload.model) {
                activeTurn.model = modelSwitchedInTurn && !String(payload.model).includes('已回退')
                  ? `${payload.model} (已回退)`
                  : payload.model
              }
              const elapsedSec = ((Date.now() - turnStartedAt) / 1000).toFixed(1)
              activeTurn.statusText = modelSwitchedInTurn ? `已回退模型 · 用时 ${elapsedSec}s` : `用时 ${elapsedSec}s`
            }
          }
        }
      }

      flushPendingDeltas()
      activeTurn.streaming = false
      // Unlock composer & bot switching immediately in 0ms instead of blocking 3-4s on post-turn sync!
      store.state.isStreaming = false
      abortCtrl = null

      relaySteps.value.forEach((s) => {
        s.status = 'done'
      })
      window.setTimeout(() => {
        if (!store.state.isStreaming) relaySteps.value = []
      }, 1600)
      if (activeTurn.statusText === '思考与生成中...') activeTurn.statusText = '已完成'
      streamStatus.value = '就绪'
      store.playSubtleChime()

      void (async () => {
        await store.refreshWorkspaceSlices(['bots', 'memories', 'todos', 'notes', 'model_health'])
        if (curBot.id === store.state.activeBotId && !store.state.isStreaming) {
          const synced = await BotHubApi.getBotMessages(curBot.id, activeThreadSnapshot).catch(() => null)
          if (synced?.threads) store.state.botThreads = synced.threads
          if (synced?.memories) store.state.botMemories = synced.memories
          if (synced?.executions) store.state.botExecutions = synced.executions
          if (synced?.messages && !store.state.isStreaming && curBot.id === store.state.activeBotId) {
            const finalTurnStatus = activeTurn.statusText
            const mappedSynced = mapServerMessages(synced.messages, curBot, {
              bots: store.state.bots,
              defaultModel: store.state.configs.ai_model,
            })
            if (mappedSynced.length >= store.state.timeline.length) {
              if (finalTurnStatus && mappedSynced.length > 0 && mappedSynced[mappedSynced.length - 1].role === 'assistant') {
                mappedSynced[mappedSynced.length - 1].statusText = finalTurnStatus
              }
              store.state.timeline = mappedSynced
            }
          }
        }
      })()
    } catch (err: any) {
      flushPendingDeltas()
      activeTurn.streaming = false
      if (err.name === 'AbortError') {
        activeTurn.statusText = '已手动停止'
        streamStatus.value = '已手动停止'
        store.notify('已停止生成')
      } else {
        activeTurn.statusText = '异常中断'
        activeTurn.errorText = err.message
        streamStatus.value = '发生错误'
        store.notify(err.message)
      }
    } finally {
      store.state.isStreaming = false
      abortCtrl = null
    }
  }

  return {
    streamStatus,
    lastPrompt,
    relaySteps,
    activeSpeakerBotId,
    sendPrompt,
    stopStream,
  }
}
