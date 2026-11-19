import { escapeHtml } from './format.ts'
import { copyToClipboard } from './clipboard.ts'

const MD_LRU_LIMIT = 240
const markdownHtmlCache = new Map<string, string>()

export function renderSafeMarkdown(raw: string): string {
  const src = String(raw || '').replace(/\r\n/g, '\n')
  if (!src.trim()) return ''

  if (src.length <= 16384) {
    const cached = markdownHtmlCache.get(src)
    if (cached !== undefined) {
      markdownHtmlCache.delete(src)
      markdownHtmlCache.set(src, cached)
      return cached
    }
  }

  const codeBlocks: string[] = []
  const withPlaceholders = src.replace(/```([a-zA-Z0-9_-]*)\n([\s\S]*?)```/g, (_, lang: string, code: string) => {
    const idx = codeBlocks.length
    const safeLang = escapeHtml(lang || 'text')
    const safeCode = escapeHtml(code.replace(/\n$/, ''))
    codeBlocks.push(
      `<div class="md-code-wrap"><div class="md-code-head"><span class="font-mono uppercase tracking-wider text-xs text-accent-fg">${safeLang}</span><button type="button" class="md-copy-code-btn min-h-[26px] px-2.5 py-0.5 rounded-lg border border-line bg-card text-fg-sub hover:border-line-strong hover:text-fg text-xs transition cursor-pointer">复制代码</button></div><pre><code>${safeCode}</code></pre></div>`
    )
    return `\n@@CODEBLOCK_${idx}@@\n`
  })

  const formatInline = (text: string): string => {
    let out = escapeHtml(text)
    out = out.replace(/`([^`\n]+)`/g, '<code class="md-inline-code">$1</code>')
    out = out.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
    out = out.replace(/==([^=\n]+)==/g, '<mark class="px-1 rounded-lg bg-accent-soft text-accent-fg">$1</mark>')
    out = out.replace(/~~([^~\n]+)~~/g, '<del class="md-del">$1</del>')
    out = out.replace(/(?<!\*)\*([^*\n]+)\*(?!\*)/g, '<em>$1</em>')
    out = out.replace(/!\[([^\]]*)\]\((https?:\/\/[^\s)]+)\)/g, '<a href="$2" target="_blank" rel="noopener noreferrer">[图片: $1]</a>')
    out = out.replace(/\[([^\]]+)\]\((https?:\/\/[^\s)]+)\)/g, '<a href="$2" target="_blank" rel="noopener noreferrer">$1</a>')
    return out
  }

  const lines = withPlaceholders.split('\n')
  const html: string[] = []
  let inList = false
  let listType: '' | 'ul' | 'ol' = ''

  const closeList = () => {
    if (inList) {
      html.push(listType === 'ol' ? '</ol>' : '</ul>')
      inList = false
      listType = ''
    }
  }

  const isTableRow = (l: string) => /^\|(.+)\|$/.test(l.trim())
  const isTableSep = (l: string) => /^\|[\s:|-]+\|$/.test(l.trim())

  for (let i = 0; i < lines.length; i++) {
    const trimmed = lines[i].trim()
    const cbMatch = trimmed.match(/^@@CODEBLOCK_(\d+)@@$/)
    if (cbMatch) {
      closeList()
      html.push(codeBlocks[Number(cbMatch[1])] || '')
      continue
    }
    if (!trimmed) {
      closeList()
      continue
    }
    if (isTableRow(trimmed) && i + 1 < lines.length && isTableSep(lines[i + 1])) {
      closeList()
      const splitCells = (row: string) => row.trim().slice(1, -1).split('|').map((c) => formatInline(c.trim()))
      const headers = splitCells(trimmed)
      i += 2
      const rows: string[][] = []
      while (i < lines.length && isTableRow(lines[i])) {
        rows.push(splitCells(lines[i]))
        i++
      }
      i--
      const thead = `<thead><tr>${headers.map((h) => `<th>${h}</th>`).join('')}</tr></thead>`
      const tbody = `<tbody>${rows.map((r) => `<tr>${r.map((c) => `<td>${c}</td>`).join('')}</tr>`).join('')}</tbody>`
      html.push(`<div class="md-table-wrap"><table>${thead}${tbody}</table></div>`)
      continue
    }
    if (/^---+$/.test(trimmed)) {
      closeList()
      html.push('<div class="md-hr"></div>')
      continue
    }
    if (trimmed.startsWith('> ')) {
      closeList()
      html.push(`<blockquote>${formatInline(trimmed.slice(2))}</blockquote>`)
      continue
    }
    const hMatch = trimmed.match(/^(#{1,4})\s+(.+)$/)
    if (hMatch) {
      closeList()
      const lvl = hMatch[1].length
      html.push(`<h${lvl}>${formatInline(hMatch[2])}</h${lvl}>`)
      continue
    }
    const taskMatch = trimmed.match(/^[-*]\s+\[([ xX])\]\s+(.+)$/)
    if (taskMatch) {
      if (!inList || listType !== 'ul') {
        closeList()
        html.push('<ul class="md-task-list">')
        inList = true
        listType = 'ul'
      }
      const checked = taskMatch[1].toLowerCase() === 'x'
      html.push(
        `<li class="md-task-item${checked ? ' is-done' : ''}"><span class="md-task-box" aria-hidden="true">${checked ? '&#10003;' : ''}</span><span>${formatInline(taskMatch[2])}</span></li>`
      )
      continue
    }
    const ulMatch = trimmed.match(/^[-*+]\s+(.+)$/)
    if (ulMatch) {
      if (!inList || listType !== 'ul') {
        closeList()
        html.push('<ul>')
        inList = true
        listType = 'ul'
      }
      html.push(`<li>${formatInline(ulMatch[1])}</li>`)
      continue
    }
    const olMatch = trimmed.match(/^\d+\.\s+(.+)$/)
    if (olMatch) {
      if (!inList || listType !== 'ol') {
        closeList()
        html.push('<ol>')
        inList = true
        listType = 'ol'
      }
      html.push(`<li>${formatInline(olMatch[1])}</li>`)
      continue
    }
    closeList()
    html.push(`<p>${formatInline(trimmed)}</p>`)
  }
  closeList()
  const rendered = `<div class="md-body">${html.join('')}</div>`
  if (src.length <= 16384) {
    if (markdownHtmlCache.size >= MD_LRU_LIMIT) {
      const oldestKey = markdownHtmlCache.keys().next().value
      if (oldestKey !== undefined) markdownHtmlCache.delete(oldestKey)
    }
    markdownHtmlCache.set(src, rendered)
  }
  return rendered
}

export function handleMarkdownCodeCopy(e: MouseEvent, onCopied?: () => void): void {
  const target = e.target as HTMLElement | null
  const btn = target?.closest('.md-copy-code-btn') as HTMLButtonElement | null
  if (!btn) return
  const code = btn.closest('.md-code-wrap')?.querySelector('pre code')?.textContent || ''
  void copyToClipboard(code).then((ok) => {
    if (!ok) return
    const prevTimer = Number(btn.dataset.copyTimer || 0)
    if (prevTimer) window.clearTimeout(prevTimer)
    btn.textContent = '已复制'
    btn.classList.remove('is-copied')
    void btn.offsetWidth
    btn.classList.add('is-copied', 'text-ok', 'border-ok-border', 'bg-ok-soft')
    onCopied?.()
    const timer = window.setTimeout(() => {
      btn.textContent = '复制代码'
      btn.classList.remove('is-copied', 'text-ok', 'border-ok-border', 'bg-ok-soft')
      delete btn.dataset.copyTimer
    }, 1500)
    btn.dataset.copyTimer = String(timer)
  })
}
