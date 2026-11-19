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
  summarizeAssetsYearlyCost,
  inferRecommendedRenewMonths,
  scoreOmnisearchMatch,
} from '../src/utils/format.ts'
import {
  formatToolDisplayName,
  formatToolHumanName,
  groupAvailableTools,
  summarizeToolStep,
} from '../src/utils/toolMeta.ts'
import {
  cleanProviderBaseUrl,
  formatTokenCount,
  buildRoutePreview,
} from '../src/utils/modelRouting.ts'
import {
  parseAvatarStyle,
  composeAvatarStyle,
  getColorPalette,
  resolveGroupMemberBots,
  pickDefaultWorkerBot,
  resolveScopedMemories,
} from '../src/utils/botLook.ts'
import {
  addMonthsPlain,
  buildCalendarMonthGrid,
  parsePlainDateParts,
  parsePlainDateTimeParts,
} from '../src/utils/calendar.ts'
import {
  mergeExtensibleCategories,
  resolveCategoryMeta,
} from '../src/utils/assetCategories.ts'
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
import { extractStructuredToolCards, parseNezhaHostInfo } from '../src/utils/toolCards.ts'
import { migrateLegacyTheme } from '../src/utils/theme.ts'

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
  assert.equal(
    summarizeAssetsYearlyCost([
      { status: '生效中', price: '$12 / 年' },
      { status: '生效中', price: '$5 / 月' },
      { status: '生效中', price: '¥30 / 月' },
      { status: '已停用', price: '¥999 / 月' },
    ]),
    '$72/年 + ¥360/年'
  )
  assert.equal(inferRecommendedRenewMonths('$5 / 月'), 1)
  assert.equal(inferRecommendedRenewMonths('$15 / 季'), 3)
  assert.equal(inferRecommendedRenewMonths('$21.6 / 年'), 12)
  assert.equal(inferRecommendedRenewMonths(''), 12)
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

  assert.equal(botStateDotClass('thinking'), 'bg-warn animate-pulse')
  assert.equal(botStateDotClass('working'), 'bg-accent animate-pulse')
  assert.equal(botStateDotClass('done'), 'bg-ok')
  assert.equal(botStateDotClass('blocked'), 'bg-danger')
  assert.equal(botStateDotClass('idle'), 'bg-fg-muted')
  assert.equal(botStateDotClass(undefined), 'bg-fg-muted')
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

test('location-agnostic calendar helpers handle month grids, leap clamping, and wall-clock datetimes', () => {
  assert.deepEqual(parsePlainDateParts('2026-11-18'), { year: 2026, month: 11, day: 18 })
  assert.equal(parsePlainDateParts('2026-02-30'), null)
  assert.equal(addMonthsPlain('2024-01-31', 1), '2024-02-29')
  assert.equal(addMonthsPlain('2026-01-31', 1), '2026-02-28')
  assert.equal(addMonthsPlain('2026-11-18', 12), '2027-11-18')

  const dt = parsePlainDateTimeParts('2026-11-18 09:30')
  assert.deepEqual(dt, { year: 2026, month: 11, day: 18, hour: 9, minute: 30 })

  const grid = buildCalendarMonthGrid(2026, 11) // Nov 1 2026 is Sunday -> 6 leading days (Oct 26..31)
  assert.equal(grid.length, 42)
  assert.equal(grid[0].dateStr, '2026-10-26')
  assert.equal(grid[0].inMonth, false)
  assert.equal(grid[6].dateStr, '2026-11-01')
  assert.equal(grid[6].inMonth, true)
})

test('extensible asset category resolver merges custom definitions and asset categories without hardcoded lock-in', () => {
  const emptyMerged = mergeExtensibleCategories([], [])
  assert.equal(emptyMerged.length, 0)

  const saved = [
    { id: 'insurance', label: '商业保险', icon: 'shield' },
    { id: 'software', label: '开发软件', icon: 'key' },
  ]
  const assets = [
    { id: 'a1', category: 'insurance' },
    { id: 'a2', category: 'custom_card' },
  ] as any[]

  const merged = mergeExtensibleCategories(saved, assets)
  assert.deepEqual(
    merged.map((c) => c.id),
    ['insurance', 'software', 'custom_card']
  )
  assert.equal(resolveCategoryMeta('insurance', saved).label, '商业保险')
  assert.equal(resolveCategoryMeta('insurance', saved).icon, 'shield')
  assert.equal(resolveCategoryMeta('custom_card', saved).label, 'custom_card')
})

test('manage_bots tool metadata, step summaries and butler slash commands work cleanly', () => {
  const step = summarizeToolStep({
    name: 'manage_bots',
    arguments: JSON.stringify({ action: 'create_group', name: '研发协同群' }),
    result: JSON.stringify({
      ok: true,
      name: '研发协同群',
      member_names: ['大管家', '代码审查官'],
    }),
    done: true,
  })
  assert.equal(step.title, '组建群聊 · 研发协同群')
  assert.equal(step.subtitle, '成员：大管家、代码审查官')
  assert.equal(step.statusLabel, '已完成')

  const butlerBot = { id: 'concierge', allowed_tools: '["manage_bots","manage_models","manage_skills"]' } as any
  const cmds = buildSlashCommands(butlerBot, [])
  assert.equal(cmds.some((c) => c.cmd === '/newbot'), true)
  assert.equal(cmds.some((c) => c.cmd === '/group'), true)
  assert.equal(cmds.some((c) => c.cmd === '/models'), true)
  assert.equal(cmds.some((c) => c.cmd === '/newskill'), true)
  assert.equal(formatToolHumanName('manage_models', []), 'AI 模型池巡检调优')
  assert.equal(formatToolHumanName('manage_skills', []), '自定义技能工坊')
})

test('modelRouting normalizes URLs, formats tokens, and previews dual-pool routes accurately', () => {
  assert.equal(
    cleanProviderBaseUrl('https://openrouter.ai/api/v1/chat/completions/'),
    'https://openrouter.ai/api/v1'
  )
  assert.equal(cleanProviderBaseUrl('https://api.deepseek.com/v1/models'), 'https://api.deepseek.com/v1')
  assert.equal(formatTokenCount(850), '850')
  assert.equal(formatTokenCount(12_500), '12.5K')
  assert.equal(formatTokenCount(2_340_000), '2.34M')

  const providers = [
    {
      id: 'or_1',
      name: 'OpenRouter',
      protocol: 'openai',
      base_url: 'https://openrouter.ai/api/v1',
      api_key: 'sk-****1234',
      enabled: true,
    },
  ]
  const pooledModels = [
    { provider_id: 'or_1', model_id: 'qwen/qwen3:free', tier: 'free' as const, enabled: true, priority: 10 },
    { provider_id: 'or_1', model_id: 'anthropic/claude-sonnet-4', tier: 'paid' as const, enabled: true, priority: 10 },
  ]

  const freeOnly = buildRoutePreview({
    selectedModel: 'auto:free',
    allowPaidFallback: false,
    providers,
    pooledModels,
    strategies: { free: 'round_robin', paid: 'priority' },
    needTools: true,
  })
  assert.equal(freeOnly.tone, 'free')
  assert.equal(freeOnly.headline.includes('轮询均摊'), true)
  assert.equal(freeOnly.headline.includes('严禁触发付费调用'), true)

  const conflictTask = buildRoutePreview({
    selectedModel: 'auto:paid',
    allowPaidFallback: false,
    providers,
    pooledModels,
    isTask: true,
  })
  assert.equal(conflictTask.tone, 'warn')
  assert.equal(conflictTask.headline.includes('冲突提醒'), true)

  const paidWithQuota = buildRoutePreview({
    selectedModel: 'auto:paid',
    allowPaidFallback: true,
    maxPaidCallsPerDay: 5,
    providers,
    pooledModels,
    isTask: true,
  })
  assert.equal(paidWithQuota.tone, 'paid')
  assert.equal(paidWithQuota.headline.includes('5 次/日'), true)
})

test('extractStructuredToolCards parses all structured tool payloads, deduplicates, and ignores partial JSON', () => {
  assert.deepEqual(extractStructuredToolCards(undefined), [])
  assert.deepEqual(extractStructuredToolCards([]), [])

  // Malformed / partial streaming JSON & error payloads should be safely ignored
  assert.deepEqual(
    extractStructuredToolCards([
      { id: 't1', name: 'manage_assets', arguments: '{}', result: '{"count":2,"assets":[{"id":', done: false },
      { id: 't2', name: 'manage_todos', arguments: '{}', result: '{"ok":false,"error":"not found"}', status: 'error', done: true },
      { id: 't3', name: 'web_search', arguments: '{}', result: '{"count":1,"results":[]}', done: true },
    ]),
    []
  )

  // manage_assets (list + expiring + summary + upsert) & deduplication by id
  const assetCards = extractStructuredToolCards([
    {
      id: 'a_sum',
      name: 'manage_assets',
      arguments: '{"action":"summary"}',
      result: JSON.stringify({
        active_count: 5,
        inactive_count: 1,
        expiring_30d_count: 2,
        by_category: { compute: 3, domain: 2 },
        cost_by_currency: [
          { currency: 'USD', monthly_estimated: 18.5, annualized_estimated: 222, priced_items: 4 },
        ],
      }),
      done: true,
    },
    {
      id: 'a_list',
      name: 'manage_assets',
      arguments: '{"action":"list"}',
      result: JSON.stringify({
        count: 2,
        assets: [
          { id: 'vps-jp', name: 'Tokyo VPS', category: 'compute', spec: '2C2G', price: '$5/月', status: '生效中', due_date: '2026-11-22', days_left: 3, auto_renew: true },
          { id: 'dom-1', name: 'bothub.dev', category: 'domain', spec: 'Cloudflare', price: '$12/年', status: '生效中', due_date: '2027-06-01', days_left: 190, auto_renew: false },
        ],
      }),
      done: true,
    },
    {
      id: 'a_exp',
      name: 'manage_assets',
      arguments: '{"action":"expiring"}',
      result: JSON.stringify({
        within_days: 30,
        count: 1,
        expiring: [
          { id: 'vps-jp', name: 'Tokyo VPS', category: 'compute', spec: '2C2G', price: '$5/月', status: '生效中', due_date: '2026-11-22', days_left: 3, auto_renew: true },
        ],
      }),
      done: true,
    },
    {
      id: 'a_upsert',
      name: 'manage_assets',
      arguments: '{"action":"upsert"}',
      result: JSON.stringify({
        ok: true,
        asset: { id: 'sub-1', name: 'Cursor Pro', category: 'subscription', spec: 'Yearly', price: '$20/月', status: '生效中', due_date: '2026-12-10', days_left: 21, auto_renew: true },
      }),
      done: true,
    },
  ])
  assert.equal(assetCards.length, 4) // 1 summary + 3 unique assets
  assert.equal(assetCards[0].kind, 'asset_summary')
  assert.equal(assetCards[1].kind, 'asset')
  if (assetCards[1].kind === 'asset') {
    assert.equal(assetCards[1].id, 'vps-jp')
    assert.equal(assetCards[1].isExpiring, true)
    assert.equal(assetCards[1].autoRenew, true)
  }

  // query_nezha_nodes + deduplication with compute_assets + host_info parsing
  assert.equal(
    parseNezhaHostInfo(
      JSON.stringify({
        Platform: 'Debian',
        PlatformVersion: '12',
        CPU: ['AMD EPYC 7763'],
        MemTotal: 2 * 1024 * 1024 * 1024,
        Arch: 'x86_64',
        CountryCode: 'jp',
      })
    ),
    'Debian 12 · AMD EPYC 7763 · RAM 2 GB · x86_64 · JP'
  )
  const nezhaCards = extractStructuredToolCards([
    {
      id: 'nz_1',
      name: 'query_nezha_nodes',
      arguments: '{}',
      result: JSON.stringify({
        source: 'nezha_sqlite',
        nezha_nodes: [
          {
            id: 'nezha-1',
            name: 'JP-Edge-01',
            host_info: '{"Platform":"Ubuntu","PlatformVersion":"24.04","CPU":["Intel Xeon","Intel Xeon"],"MemTotal":4294967296}',
            last_active: '2026-11-19 20:00:00',
            status: 'online',
          },
        ],
        compute_assets: [
          { id: 'vps-jp', name: 'Tokyo VPS', category: 'compute', spec: '2C2G', price: '$5/月', status: '生效中', due_date: '2026-11-22', days_left: 3, auto_renew: false },
        ],
      }),
      done: true,
    },
  ])
  assert.equal(nezhaCards.length, 2)
  assert.equal(nezhaCards[0].kind, 'nezha_node')
  if (nezhaCards[0].kind === 'nezha_node') {
    assert.equal(nezhaCards[0].hostSummary.includes('Ubuntu 24.04'), true)
    assert.equal(nezhaCards[0].hostSummary.includes('2C · Intel Xeon'), true)
    assert.equal(nezhaCards[0].hostSummary.includes('RAM 4 GB'), true)
  }

  // manage_todos (list + batch_add + complete updating existing item without duplication)
  const todoCards = extractStructuredToolCards([
    {
      id: 'td_list',
      name: 'manage_todos',
      arguments: '{"action":"list"}',
      result: JSON.stringify({
        filter: 'open',
        count: 1,
        overdue_count: 0,
        todos: [{ id: 'todo_1', title: 'Renew Tokyo VPS', status: 'pending', priority: 'high', due_at: '2026-11-21' }],
      }),
      done: true,
    },
    {
      id: 'td_complete',
      name: 'manage_todos',
      arguments: '{"action":"complete"}',
      result: JSON.stringify({
        ok: true,
        todo: { id: 'todo_1', title: 'Renew Tokyo VPS', status: 'done', priority: 'high', due_at: '2026-11-21' },
      }),
      done: true,
    },
    {
      id: 'td_batch',
      name: 'manage_todos',
      arguments: '{"action":"batch_add"}',
      result: JSON.stringify({
        ok: true,
        count: 2,
        todos: [
          { id: 'todo_2', title: 'Audit DNS', status: 'pending', priority: 'normal' },
          { id: 'todo_3', title: 'Backup SQLite', status: 'pending', priority: 'low' },
        ],
      }),
      done: true,
    },
  ])
  assert.equal(todoCards.length, 3)
  if (todoCards[0].kind === 'todo') {
    assert.equal(todoCards[0].status, 'done')
  }

  // manage_tasks (list + upsert)
  const taskCards = extractStructuredToolCards([
    {
      id: 'tk_list',
      name: 'manage_tasks',
      arguments: '{"action":"list"}',
      result: JSON.stringify({
        count: 1,
        tasks: [
          {
            id: 'task_daily',
            name: 'Morning Digest',
            type: 'bot_prompt',
            bot_id: 'butler',
            cron_expr: '0 8 * * *',
            enabled: true,
            last_run: { status: 'ok', summary: 'Pushed 3 items' },
          },
        ],
      }),
      done: true,
    },
    {
      id: 'tk_upsert',
      name: 'manage_tasks',
      arguments: '{"action":"upsert"}',
      result: JSON.stringify({
        ok: true,
        task: { id: 'task_once', name: 'Call Mom', type: 'bot_prompt', bot_id: 'butler', run_at: '2026-11-20 19:00', enabled: true },
      }),
      done: true,
    },
  ])
  assert.equal(taskCards.length, 2)
  assert.equal(taskCards[0].kind, 'task')
  if (taskCards[0].kind === 'task') {
    assert.equal(taskCards[0].lastSummary, 'Pushed 3 items')
  }

  // workspace_briefing
  const briefCards = extractStructuredToolCards([
    {
      id: 'wb_1',
      name: 'workspace_briefing',
      arguments: '{}',
      result: JSON.stringify({
        today: '2026-11-19',
        weekday: '周四',
        open_todo_count: 3,
        overdue_todos: [{ id: 'todo_ov', title: 'Overdue Task', status: 'pending', priority: 'high', due_at: '2026-11-17' }],
        due_today_todos: [{ id: 'todo_ov', title: 'Overdue Task', status: 'pending', priority: 'high', due_at: '2026-11-17' }],
        high_priority_todos: [],
        expiring_assets: [{ id: 'vps-jp', name: 'Tokyo VPS', category: 'compute', status: '生效中', due_date: '2026-11-22', days_left: 3, auto_renew: false }],
        unread_notifications: [{ id: 1, title: 'Alert' }],
        active_tasks: [{ id: 'task_daily', name: 'Morning Digest' }],
        summary_text: 'Focus on overdue items',
      }),
      done: true,
    },
  ])
  assert.equal(briefCards.length, 3) // 1 briefing + 1 deduped todo + 1 asset
  assert.equal(briefCards[0].kind, 'briefing')
  if (briefCards[0].kind === 'briefing') {
    assert.equal(briefCards[0].overdueCount, 1)
    assert.equal(briefCards[0].expiringAssetCount, 1)
    assert.equal(briefCards[0].unreadCount, 1)
    assert.equal(briefCards[0].activeTaskCount, 1)
  }
})

test('migrateLegacyTheme maps all legacy themes to decoupled mode and accent pairs', () => {
  assert.deepEqual(migrateLegacyTheme('obsidian'), { mode: 'dark', accent: 'amber' })
  assert.deepEqual(migrateLegacyTheme('ocean'), { mode: 'dark', accent: 'cyan' })
  assert.deepEqual(migrateLegacyTheme('emerald'), { mode: 'dark', accent: 'emerald' })
  assert.deepEqual(migrateLegacyTheme('amethyst'), { mode: 'dark', accent: 'violet' })
  assert.deepEqual(migrateLegacyTheme('daylight'), { mode: 'light', accent: 'blue' })
  assert.deepEqual(migrateLegacyTheme('parchment'), { mode: 'light', accent: 'amber' })
  assert.deepEqual(migrateLegacyTheme('system'), { mode: 'system', accent: 'blue' })
  assert.deepEqual(migrateLegacyTheme('unknown'), { mode: 'dark', accent: 'blue' })
  assert.deepEqual(migrateLegacyTheme(null), { mode: 'dark', accent: 'blue' })
  assert.deepEqual(migrateLegacyTheme(undefined), { mode: 'dark', accent: 'blue' })
})
