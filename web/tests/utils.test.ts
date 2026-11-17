import test from 'node:test'
import assert from 'node:assert/strict'
import {
  formatDurationMs,
  describeCron,
  generateSlug,
  shortVersion,
  escapeHtml,
  groupModeLabel,
  summarizeAssetsMonthlyCost,
} from '../src/utils/format.ts'
import { formatToolDisplayName, groupAvailableTools } from '../src/utils/toolMeta.ts'
import { parseAvatarStyle, composeAvatarStyle, resolveGroupMemberBots } from '../src/utils/botLook.ts'

test('format utilities produce expected strings', () => {
  assert.equal(formatDurationMs(420), '420ms')
  assert.equal(formatDurationMs(2450), '2.5s')
  assert.equal(formatDurationMs(125000), '2m5s')
  assert.equal(describeCron('0 8 * * *'), '每天 早间 08:00 执行')
  assert.equal(describeCron('*/15 * * * *'), '每隔 15 分钟轮询一次')
  assert.equal(groupModeLabel('parallel'), '全员并发')
  assert.equal(groupModeLabel('smart'), '智能协同')
  assert.equal(generateSlug('My Check_Skill!'), 'my_check_skill')
  assert.equal(shortVersion('v2.22.0-abcdef12345678'), 'v2.22.0')
  assert.equal(escapeHtml('<script>"a&b"'), '&lt;script&gt;&quot;a&amp;b&quot;')
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

test('toolMeta groups and formats builtin & skill names cleanly', () => {
  assert.equal(formatToolDisplayName('manage_todos', []), '个人待办管理 (manage_todos)')
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
  assert.deepEqual(parseAvatarStyle('cyan'), { shape: 'butler', color: 'gold' })
  assert.equal(composeAvatarStyle('radar', 'emerald'), 'radar:emerald')

  const bots = [
    { id: 'b1', name: 'Alice', is_group: false },
    { id: 'b2', name: 'Bob', is_group: false },
    { id: 'g1', name: 'Team', is_group: true, member_ids: '["b2","missing"]' },
  ] as any[]
  const resolved = resolveGroupMemberBots(bots[2], bots)
  assert.equal(resolved.length, 1)
  assert.equal(resolved[0].id, 'b2')
})
