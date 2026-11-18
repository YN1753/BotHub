import test from 'node:test'
import assert from 'node:assert/strict'
import {
  formatDurationMs,
  formatUptime,
  formatLocalDate,
  parseJsonStringArray,
  parseJsonRecord,
  botStateDotClass,
  truncateText,
  describeCron,
  generateSlug,
  escapeHtml,
  csvEscapeCell,
  groupModeLabel,
  summarizeAssetsMonthlyCost,
  scoreOmnisearchMatch,
} from '../src/utils/format.ts'
import {
  formatToolDisplayName,
  formatToolHumanName,
  groupAvailableTools,
  summarizeToolStep,
} from '../src/utils/toolMeta.ts'
import {
  parseAvatarStyle,
  composeAvatarStyle,
  getColorPalette,
  resolveGroupMemberBots,
  pickDefaultWorkerBot,
  resolveScopedMemories,
} from '../src/utils/botLook.ts'
import { clampUnit, computeNormalizedGaze } from '../src/utils/botGaze.ts'
import { renderSafeMarkdown } from '../src/utils/markdown.ts'
import { shouldReloadOnPreloadError, installPreloadErrorRecovery } from '../src/utils/lazyComponent.ts'
import {
  isImeComposingEnter,
  isPointOutsideRect,
  closePopoverWithTransition,
  clearSearchOnEscape,
} from '../src/utils/topLayer.ts'
import { resolveTabSlideTransition } from '../src/utils/motion.ts'
import { mapServerMessages } from '../src/utils/chatMapper.ts'
import { buildSlashCommands, filterSlashCommands } from '../src/utils/slashCommands.ts'
import { isValidBase64Url } from '../src/utils/passkey.ts'

test('format utilities produce expected strings', () => {
  assert.equal(formatDurationMs(420), '420ms')
  assert.equal(formatDurationMs(2450), '2.5s')
  assert.equal(formatDurationMs(125000), '2m5s')
  assert.equal(formatUptime(45), '45s')
  assert.equal(formatUptime(3720), '1h 2m')
  assert.equal(formatUptime(90000), '1d 1h')
  assert.equal(truncateText('abcdefg', 4), 'abcd...')
  assert.equal(describeCron('0 8 * * *'), '每天 早间 08:00 执行')
  assert.equal(describeCron('*/15 * * * *'), '每隔 15 分钟轮询一次')
  assert.equal(groupModeLabel('parallel'), '全员并发')
  assert.equal(groupModeLabel('smart'), '智能协同')
  assert.equal(groupModeLabel('sequential'), '全员接力')
  assert.equal(groupModeLabel('mention_only'), '仅 @点名')
  assert.equal(generateSlug('My Check_Skill!'), 'my_check_skill')
  assert.equal(escapeHtml('<script>"a&b"'), '&lt;script&gt;&quot;a&amp;b&quot;')
  assert.equal(csvEscapeCell('=CMD("calc")'), '"\'=CMD(""calc"")"')
  assert.equal(csvEscapeCell('+SUM(A1:A2)'), '"\'+SUM(A1:A2)"')
  assert.equal(csvEscapeCell('-3'), '"-3"')
  assert.equal(csvEscapeCell('@evil'), '"\'@evil"')
  assert.equal(csvEscapeCell('Tokyo "VPS"'), '"Tokyo ""VPS"""')
  assert.equal(
    summarizeAssetsMonthlyCost([
      { status: '生效中', price: '$12 / 年' },
      { status: '生效中', price: '$5 / 月' },
      { status: '生效中', price: '¥30 / 月' },
      { status: '已停用', price: '¥999 / 月' },
    ]),
    '$6/月 + ¥30/月'
  )
})

test('parseJsonStringArray, parseJsonRecord, formatLocalDate and botStateDotClass handle edge cases', () => {
  assert.deepEqual(parseJsonStringArray('["manage_todos", "web_search"]'), ['manage_todos', 'web_search'])
  assert.deepEqual(parseJsonStringArray([1, 'two']), ['1', 'two'])
  assert.deepEqual(parseJsonStringArray('{"not":"array"}'), [])
  assert.deepEqual(parseJsonStringArray('malformed-json'), [])
  assert.deepEqual(parseJsonStringArray(''), [])
  assert.deepEqual(parseJsonStringArray(undefined), [])

  assert.deepEqual(parseJsonRecord('{"X-Token":"abc","port":8080}'), { 'X-Token': 'abc', port: '8080' })
  assert.deepEqual(parseJsonRecord('["array"]'), {})
  assert.deepEqual(parseJsonRecord('bad-json'), {})
  assert.deepEqual(parseJsonRecord(null), {})

  // Local midnight date formatting must use local calendar fields rather than UTC slice
  const localMidnight = new Date(2026, 0, 5, 0, 15, 0)
  assert.equal(formatLocalDate(localMidnight), '2026-01-05')
  const localLateNight = new Date(2026, 10, 18, 23, 59, 59)
  assert.equal(formatLocalDate(localLateNight), '2026-11-18')

  assert.equal(botStateDotClass('thinking'), 'bg-amber-400 animate-pulse')
  assert.equal(botStateDotClass('working'), 'bg-[#f2b880] animate-pulse')
  assert.equal(botStateDotClass('done'), 'bg-emerald-400')
  assert.equal(botStateDotClass('blocked'), 'bg-rose-400')
  assert.equal(botStateDotClass('idle'), 'bg-slate-500')
  assert.equal(botStateDotClass(undefined), 'bg-slate-500')
})

test('scoreOmnisearchMatch ranks exact, prefix, title, subtitle and body hits', () => {
  const exact = scoreOmnisearchMatch('VPS', 'Asset', '资产', '', 'vps')
  const prefix = scoreOmnisearchMatch('VPS HongKong', 'Asset', '资产', '', 'vps')
  const subTitle = scoreOmnisearchMatch('My Server', 'VPS HongKong', '资产', '', 'vps')
  const bodyOnly = scoreOmnisearchMatch('Quarterly Note', 'deliverable', '文档', 'Renew VPS next week', 'vps')
  const none = scoreOmnisearchMatch('Quarterly Note', 'deliverable', '文档', 'Nothing here', 'vps')

  assert.equal(exact > prefix, true)
  assert.equal(prefix > subTitle, true)
  assert.equal(subTitle > bodyOnly, true)
  assert.equal(bodyOnly > 0, true)
  assert.equal(none, 0)
})

test('toolMeta groups and formats builtin & skill names cleanly', () => {
  assert.equal(formatToolDisplayName('manage_todos', []), '个人待办管理 (manage_todos)')
  assert.equal(formatToolDisplayName('manage_stash', []), '收藏与闪念匣 (manage_stash)')
  assert.equal(
    formatToolDisplayName('skill_weather', [{ id: '1', slug: 'weather', name: '天气助手' } as any]),
    '天气助手 (skill_weather)'
  )
  const grouped = groupAvailableTools([
    { id: 'manage_todos', description: 'todos' },
    { id: 'custom_unknown', description: 'extra' },
  ])
  assert.equal(grouped.length, 2)
  assert.equal(grouped[0].id, 'assistant')
  assert.equal(grouped[1].id, 'extra')

  assert.equal(formatToolHumanName('read_url', []), '阅读网页')
  assert.equal(formatToolHumanName('web_search', []), '联网检索')

  const readSummary = summarizeToolStep({
    name: 'read_url',
    arguments: '{"url":"https://github.com/trending"}',
    result: JSON.stringify({
      ok: true,
      url: 'https://github.com/trending',
      title: 'Trending repositories on GitHub',
      content: 'Trending repositories on GitHub today Skip to content Sign in...',
    }),
    done: true,
  })
  assert.equal(readSummary.title, '阅读网页 · github.com/trending')
  assert.equal(readSummary.statusLabel, '已读取')
  assert.equal(readSummary.subtitle, 'Trending repositories on GitHub')
  assert.equal(JSON.stringify(readSummary).includes('Skip to content'), false)

  const searchSummary = summarizeToolStep({
    name: 'web_search',
    arguments: '{"query":"open source AI agent project trending 2026"}',
    result: JSON.stringify({
      count: 2,
      query: 'open source AI agent project trending 2026',
      results: [
        { title: 'Whiteboard (YC W26)', url: 'https://github.com/devdotfast/whiteboard' },
      ],
    }),
    done: true,
  })
  assert.equal(searchSummary.statusLabel, '找到 2 条结果')
  assert.equal(searchSummary.sources.length, 1)
  assert.equal(searchSummary.sources[0].domain, 'github.com')
})

test('botLook parses styles, resolves group members, picks worker bots and scopes memories', () => {
  assert.deepEqual(parseAvatarStyle('visor:indigo'), { shape: 'visor', color: 'indigo' })
  assert.deepEqual(parseAvatarStyle('sentinel:obsidian'), { shape: 'sentinel', color: 'obsidian' })
  assert.deepEqual(parseAvatarStyle('cyan'), { shape: 'butler', color: 'gold' })
  assert.equal(composeAvatarStyle('radar', 'emerald'), 'radar:emerald')
  assert.equal(Boolean(getColorPalette('obsidian').shellHi), true)

  assert.deepEqual(computeNormalizedGaze(500, 250, 1000, 500), { gx: 0, gy: 0 })
  assert.deepEqual(computeNormalizedGaze(1000, 0, 1000, 500), { gx: 1, gy: -1 })
  assert.equal(clampUnit(2.5), 1)
  assert.equal(clampUnit(-3), -1)

  const bots = [
    { id: 'b1', name: 'Companion', is_group: false, bot_mode: 'companion' },
    { id: 'b2', name: 'Worker', is_group: false, bot_mode: 'worker' },
    { id: 'g1', name: 'Team', is_group: true, member_ids: '["b2","missing"]' },
  ] as any[]
  const resolved = resolveGroupMemberBots(bots[2], bots)
  assert.equal(resolved.length, 1)
  assert.equal(resolved[0].id, 'b2')

  assert.equal(pickDefaultWorkerBot(bots, null), 'b2')
  assert.equal(pickDefaultWorkerBot(bots, bots[0]), 'b1')
  assert.equal(pickDefaultWorkerBot(bots, bots[0], 'b2'), 'b2')
  assert.equal(pickDefaultWorkerBot(bots, null, 'nonexistent'), 'b2')

  const allMems = [
    { id: 1, scope: 'global', fact_key: 'tz' },
    { id: 2, scope: 'bot:b1', fact_key: 'nick' },
    { id: 3, scope: 'bot:b2', fact_key: 'ops' },
  ] as any[]
  assert.equal(resolveScopedMemories({ id: 'b1', memory_policy: 'isolated' } as any, [], allMems).length, 1)
  assert.equal(resolveScopedMemories({ id: 'b1', memory_policy: 'standard' } as any, [], allMems).length, 2)
  assert.equal(resolveScopedMemories({ id: 'b1', memory_policy: 'isolated' } as any, [allMems[1]], allMems).length, 1)
})

test('mapServerMessages joins tool outputs and merges consecutive tool rounds per sender', () => {
  const bots = [
    { id: 'ops', name: 'Ops Bot', role_title: 'SRE', avatar_style: 'visor:obsidian', model: 'deepseek-chat' },
    { id: 'res', name: 'Researcher', role_title: 'Analyst', avatar_style: 'radar:indigo', model: 'deepseek-reasoner' },
  ] as any[]

  const rawMsgs = [
    { id: 1, bot_id: 'ops', role: 'user', content: 'Check nodes', created_at: '2026-11-18T01:00:00Z' },
    {
      id: 2,
      bot_id: 'ops',
      sender_bot_id: 'ops',
      role: 'assistant',
      reasoning_content: 'Querying nezha...',
      content: '',
      tool_calls: JSON.stringify([{ id: 'call_1', function: { name: 'query_nezha_nodes', arguments: '{}' } }]),
      created_at: '2026-11-18T01:00:01Z',
    },
    {
      id: 3,
      bot_id: 'ops',
      role: 'tool',
      tool_call_id: 'call_1',
      content: '{"online":4}',
      created_at: '2026-11-18T01:00:02Z',
    },
    {
      id: 4,
      bot_id: 'ops',
      sender_bot_id: 'ops',
      role: 'assistant',
      reasoning_content: 'Summarizing result...',
      content: 'All 4 nodes are online.',
      created_at: '2026-11-18T01:00:03Z',
    },
    {
      id: 5,
      bot_id: 'ops',
      sender_bot_id: 'res',
      role: 'assistant',
      content: 'Traffic looks normal.',
      created_at: '2026-11-18T01:00:04Z',
    },
    {
      id: 6,
      bot_id: 'ops',
      role: 'assistant',
      msg_kind: 'routine',
      content: 'Morning digest ready.',
      created_at: '2026-11-18T08:00:00Z',
    },
  ] as any[]

  const timeline = mapServerMessages(rawMsgs, bots[0], { bots, defaultModel: 'primary' })
  assert.equal(timeline.length, 4)
  assert.equal(timeline[0].role, 'user')

  // Msg 2 + Msg 3 + Msg 4 merged into one clean assistant bubble for 'ops'
  assert.equal(timeline[1].serverId, 4)
  assert.equal(timeline[1].senderBotId, 'ops')
  assert.equal(timeline[1].content, 'All 4 nodes are online.')
  assert.equal(timeline[1].tools.length, 1)
  assert.equal(timeline[1].tools[0].name, 'query_nezha_nodes')
  assert.equal(timeline[1].tools[0].result, '{"online":4}')
  assert.match(timeline[1].reasoning, /Querying nezha/)
  assert.match(timeline[1].reasoning, /Summarizing result/)

  // Distinct sender 'res' stays a separate turn
  assert.equal(timeline[2].senderBotId, 'res')
  assert.equal(timeline[2].senderName, 'Researcher')

  // Routine message preserves msgKind & statusText
  assert.equal(timeline[3].msgKind, 'routine')
  assert.equal(timeline[3].statusText, '例程主动汇报')
})

test('slashCommands builds tool-gated commands, bound skills and filters cleanly', () => {
  const bot = {
    id: 'b1',
    allowed_tools: '["manage_todos","web_search"]',
    skill_ids: '["sk1","sk_disabled"]',
  } as any
  const skills = [
    { id: 'sk1', slug: 'weather', name: '天气查询', kind: 'webhook', enabled: true },
    { id: 'sk_disabled', slug: 'off', name: '已停用', kind: 'prompt', enabled: false },
  ] as any[]

  const cmds = buildSlashCommands(bot, skills)
  assert.equal(cmds[0].cmd, '/weather')
  assert.equal(cmds.some((c) => c.cmd === '/off'), false)
  assert.equal(cmds.some((c) => c.cmd === '/todo'), true)
  assert.equal(cmds.some((c) => c.cmd === '/search'), true)
  assert.equal(cmds.some((c) => c.cmd === '/nezha'), false)
  assert.equal(cmds.some((c) => c.cmd === '/clear'), true)

  assert.deepEqual(filterSlashCommands(cmds, 'hello'), [])
  assert.equal(filterSlashCommands(cmds, '/wea').length, 1)
  assert.equal(filterSlashCommands(cmds, '/待办').some((c) => c.cmd === '/todo'), true)
})

test('isValidBase64Url validates WebAuthn credential & user identifiers strictly', () => {
  assert.equal(isValidBase64Url('AbCdEfGhIjKlMnOpQrStUvWxYz0123456789-_'), true)
  assert.equal(isValidBase64Url('YWI'), true)
  assert.equal(isValidBase64Url('YWJj'), true)
  assert.equal(isValidBase64Url('YWJjZA'), true)
  assert.equal(isValidBase64Url(''), false)
  assert.equal(isValidBase64Url('a'), false) // length % 4 === 1 is invalid Base64URL
  assert.equal(isValidBase64Url('YWJjZ'), false) // length 5 (% 4 === 1)
  assert.equal(isValidBase64Url('ab+cd'), false)
  assert.equal(isValidBase64Url('ab/cd'), false)
  assert.equal(isValidBase64Url('YWJj='), false)
  assert.equal(isValidBase64Url(null), false)
  assert.equal(isValidBase64Url(12345), false)
})

test('renderSafeMarkdown sanitizes XSS and renders structured blocks', () => {
  const xss = renderSafeMarkdown('<script>alert(1)</script> [bad](javascript:alert(2)) [ok](https://example.com)')
  assert.equal(xss.includes('<script>'), false)
  assert.equal(xss.includes('href="javascript:'), false)
  assert.equal(xss.includes('href="https://example.com"'), true)

  const rich = renderSafeMarkdown([
    '## Ops Summary',
    '- [x] SSL verified',
    '- [ ] Renew domain',
    '',
    '| Node | Load |',
    '| --- | --- |',
    '| hk-1 | 0.21 |',
    '',
    '> check logs',
    '',
    '```sh',
    'uptime',
    '```',
  ].join('\n'))
  assert.match(rich, /<h2>Ops Summary<\/h2>/)
  assert.match(rich, /md-task-item is-done/)
  assert.match(rich, /<table>/)
  assert.match(rich, /<blockquote>/)
  assert.match(rich, /md-copy-code-btn/)

  // Verify LRU cache returns identical HTML output on repeated calls
  const repeat = renderSafeMarkdown('**cached** `token`')
  assert.equal(repeat, renderSafeMarkdown('**cached** `token`'))
})

test('preload error recovery guards against reload loops and storage exceptions', () => {
  assert.equal(shouldReloadOnPreloadError(25_000, 10_000, 10_000), true)
  assert.equal(shouldReloadOnPreloadError(18_000, 10_000, 10_000), false)

  let reloadCount = 0
  let listener: ((ev: any) => void) | null = null
  const mockWin = {
    addEventListener(type: string, fn: (ev: any) => void) {
      if (type === 'vite:preloadError') listener = fn
    },
    sessionStorage: {
      getItem() {
        throw new Error('Private browsing SecurityError')
      },
      setItem() {
        throw new Error('Private browsing SecurityError')
      },
    },
    location: {
      reload() {
        reloadCount += 1
      },
    },
  } as unknown as Window

  installPreloadErrorRecovery(mockWin)
  assert.equal(typeof listener, 'function')

  let prevented = 0
  listener!({ preventDefault: () => { prevented += 1 } })
  listener!({ preventDefault: () => { prevented += 1 } })

  assert.equal(prevented, 2)
  assert.equal(reloadCount, 1)
})

test('topLayer IME, Escape clear and hit-test helpers handle edge cases accurately', () => {
  assert.equal(isImeComposingEnter({ isComposing: true, keyCode: 13, timeStamp: 100 } as KeyboardEvent, null), true)
  assert.equal(isImeComposingEnter({ isComposing: false, keyCode: 229, timeStamp: 100 } as KeyboardEvent, null), true)
  assert.equal(isImeComposingEnter({ isComposing: false, keyCode: 13, timeStamp: 120 } as KeyboardEvent, 95), true)
  assert.equal(isImeComposingEnter({ isComposing: false, keyCode: 13, timeStamp: 200 } as KeyboardEvent, 95), false)

  const rect = { left: 10, top: 20, right: 110, bottom: 120 } as DOMRect
  assert.equal(isPointOutsideRect(50, 50, rect), false)
  assert.equal(isPointOutsideRect(5, 50, rect), true)
  assert.equal(isPointOutsideRect(50, 125, rect), true)

  let cleared = 0
  let stopped = 0
  clearSearchOnEscape(
    { preventDefault: () => {}, stopPropagation: () => { stopped += 1 } } as unknown as KeyboardEvent,
    '',
    () => { cleared += 1 }
  )
  assert.equal(cleared, 0)
  assert.equal(stopped, 0)

  clearSearchOnEscape(
    { preventDefault: () => {}, stopPropagation: () => { stopped += 1 } } as unknown as KeyboardEvent,
    'query',
    () => { cleared += 1 }
  )
  assert.equal(cleared, 1)
  assert.equal(stopped, 1)
})

test('directional motion and popover exit helpers degrade gracefully', async () => {
  const tabs = ['stream', 'routines', 'config', 'activity'] as const
  assert.equal(resolveTabSlideTransition('config', 'stream', tabs), 'vesper-slide-forward')
  assert.equal(resolveTabSlideTransition('routines', 'activity', tabs), 'vesper-slide-backward')
  assert.equal(resolveTabSlideTransition('stream', 'stream', tabs), 'vesper-slide-forward')

  let hidden = false
  const attrs = new Set<string>()
  const mockPopover = {
    matches: (sel: string) => sel === ':popover-open' && !hidden,
    setAttribute: (k: string) => attrs.add(k),
    hasAttribute: (k: string) => attrs.has(k),
    removeAttribute: (k: string) => attrs.delete(k),
    getAnimations: () => [{ finished: Promise.resolve() }],
    hidePopover: () => {
      hidden = true
    },
  } as unknown as HTMLElement

  await closePopoverWithTransition(mockPopover)
  assert.equal(hidden, true)
  assert.equal(attrs.has('data-closing'), false)
})
