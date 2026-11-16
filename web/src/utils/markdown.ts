import { escapeHtml } from './format'

export function renderSafeMarkdown(raw: string): string {
  const src = String(raw || '').replace(/\r\n/g, '\n')
  if (!src.trim()) return ''

  const codeBlocks: string[] = []
  const withPlaceholders = src.replace(/```([a-zA-Z0-9_-]*)\n([\s\S]*?)```/g, (_, lang: string, code: string) => {
    const idx = codeBlocks.length
    const safeLang = escapeHtml(lang || 'text')
    const safeCode = escapeHtml(code.replace(/\n$/, ''))
    codeBlocks.push(
      `<div class="md-code-wrap"><div class="md-code-head"><span class="font-mono">${safeLang}</span><button type="button" class="md-copy-code-btn px-2 py-0.5 rounded border border-slate-600/40 text-slate-300 hover:text-white text-[11px] cursor-pointer">复制代码</button></div><pre><code>${safeCode}</code></pre></div>`
    )
    return `\n@@CODEBLOCK_${idx}@@\n`
  })

  const formatInline = (text: string): string => {
    let out = escapeHtml(text)
    out = out.replace(/`([^`\n]+)`/g, '<code class="md-inline-code">$1</code>')
    out = out.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
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
      html.push(`<table>${thead}${tbody}</table>`)
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
    const ulMatch = trimmed.match(/^[-*]\s+(.+)$/)
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
  return `<div class="md-body">${html.join('')}</div>`
}
