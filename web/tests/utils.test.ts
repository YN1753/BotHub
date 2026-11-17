import test from 'node:test'
import assert from 'node:assert/strict'
import {
  formatDurationMs,
  formatUptime,
  truncateText,
  describeCron,
  generateSlug,
  shortVersion,
  escapeHtml,
  csvEscapeCell,
  groupModeLabel,
  summarizeAssetsMonthlyCost,
  scoreOmnisearchMatch,
} from '../src/utils/format.ts'
import { formatToolDisplayName, groupAvailableTools } from '../src/utils/toolMeta.ts'
import { parseAvatarStyle, composeAvatarStyle, getColorPalette, resolveGroupMemberBots } from '../src/utils/botLook.ts'
import { clampUnit, computeNormalizedGaze } from '../src/utils/botGaze.ts'
import { renderSafeMarkdown } from '../src/utils/markdown.ts'
import { shouldReloadOnPreloadError, installPreloadErrorRecovery } from '../src/utils/lazyComponent.ts'
import { isImeComposingEnter, isPointOutsideRect, closePopoverWithTransition } from '../src/utils/topLayer.ts'
import { resolveTabSlideTransition, runViewTransition } from '../src/utils/motion.ts'

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
  assert.equal(generateSlug('My Check_Skill!'), 'my_check_skill')
  assert.equal(shortVersion('v2.22.0-abcdef12345678'), 'v2.22.0')
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
})

test('botLook parses styles and resolves group members cleanly', () => {
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
    { id: 'b1', name: 'Alice', is_group: false },
    { id: 'b2', name: 'Bob', is_group: false },
    { id: 'g1', name: 'Team', is_group: true, member_ids: '["b2","missing"]' },
  ] as any[]
  const resolved = resolveGroupMemberBots(bots[2], bots)
  assert.equal(resolved.length, 1)
  assert.equal(resolved[0].id, 'b2')
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

test('topLayer IME and hit-test helpers handle edge cases accurately', () => {
  assert.equal(isImeComposingEnter({ isComposing: true, keyCode: 13, timeStamp: 100 } as KeyboardEvent, null), true)
  assert.equal(isImeComposingEnter({ isComposing: false, keyCode: 229, timeStamp: 100 } as KeyboardEvent, null), true)
  assert.equal(isImeComposingEnter({ isComposing: false, keyCode: 13, timeStamp: 120 } as KeyboardEvent, 95), true)
  assert.equal(isImeComposingEnter({ isComposing: false, keyCode: 13, timeStamp: 200 } as KeyboardEvent, 95), false)

  const rect = { left: 10, top: 20, right: 110, bottom: 120 } as DOMRect
  assert.equal(isPointOutsideRect(50, 50, rect), false)
  assert.equal(isPointOutsideRect(5, 50, rect), true)
  assert.equal(isPointOutsideRect(50, 125, rect), true)
})

test('directional motion and popover exit helpers degrade gracefully', async () => {
  const tabs = ['stream', 'routines', 'config', 'activity'] as const
  assert.equal(resolveTabSlideTransition('config', 'stream', tabs), 'vesper-slide-forward')
  assert.equal(resolveTabSlideTransition('routines', 'activity', tabs), 'vesper-slide-backward')
  assert.equal(resolveTabSlideTransition('stream', 'stream', tabs), 'vesper-slide-forward')

  let updated = false
  runViewTransition(() => {
    updated = true
  }, 'forward')
  assert.equal(updated, true)

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
