// Editor utilities (kept from original markdown.ts)
// Markdown rendering is now handled by the Lexical-based editor

// ─── Extract headings for outline ─────────────────────────────────
export function extractHeadings(md: string): { level: number; text: string; id: string }[] {
  const headings: { level: number; text: string; id: string }[] = []
  let idCounter = 0
  for (const line of md.split('\n')) {
    const m = line.match(/^(#{1,6})\s+(.+)/)
    if (m) {
      const text = m[2].replace(/\*\*(.+?)\*\*/g, '$1').replace(/\*(.+?)\*/g, '$1')
      headings.push({ level: m[1].length, text, id: `h-${idCounter++}` })
    }
  }
  return headings
}

// ─── Editor utilities ─────────────────────────────────────────────
export function wrapSelection(text: string, selStart: number, selEnd: number, wrapper: { before: string; after: string; placeholder?: string }): { text: string; cursor: number } | null {
  const selected = text.slice(selStart, selEnd)
  if (!selected && selStart === selEnd) return null
  const content = selected || wrapper.placeholder || ''
  const newText = text.slice(0, selStart) + wrapper.before + content + wrapper.after + text.slice(selEnd)
  return { text: newText, cursor: selStart + wrapper.before.length + content.length + wrapper.after.length }
}

export function toggleLineWrapper(text: string, selStart: number, selEnd: number, prefix: string): { text: string; cursor: number } | null {
  const lineStart = text.lastIndexOf('\n', selStart - 1) + 1
  const beforeLine = text.slice(0, lineStart)
  const afterLine = text.slice(lineStart)
  const newText = beforeLine + prefix + afterLine
  return { text: newText, cursor: selStart + prefix.length }
}

// Markdown quick conversion
export function applyQuickConversion(text: string, selStart: number, selEnd: number, key: string): { text: string; cursor: number } | null {
  if (key !== ' ') return null
  const lineStart = text.lastIndexOf('\n', selStart - 1) + 1
  const line = text.slice(lineStart, selStart)
  if (line === '#') return { text: text.slice(0, lineStart) + '# ' + text.slice(selStart), cursor: selStart + 1 }
  if (line === '##') return { text: text.slice(0, lineStart) + '## ' + text.slice(selStart), cursor: selStart + 2 }
  if (line === '###') return { text: text.slice(0, lineStart) + '### ' + text.slice(selStart), cursor: selStart + 3 }
  if (line === '####') return { text: text.slice(0, lineStart) + '#### ' + text.slice(selStart), cursor: selStart + 4 }
  if (line === '#####') return { text: text.slice(0, lineStart) + '##### ' + text.slice(selStart), cursor: selStart + 5 }
  if (line === '######') return { text: text.slice(0, lineStart) + '###### ' + text.slice(selStart), cursor: selStart + 6 }
  if (line === '-') return { text: text.slice(0, lineStart) + '- ' + text.slice(selStart), cursor: selStart + 1 }
  if (line === '*') return { text: text.slice(0, lineStart) + '* ' + text.slice(selStart), cursor: selStart + 1 }
  if (line === '>') return { text: text.slice(0, lineStart) + '> ' + text.slice(selStart), cursor: selStart + 1 }
  if (/^\d+$/.test(line)) return { text: text.slice(0, lineStart) + line + '. ' + text.slice(selStart), cursor: selStart + 1 }
  return null
}
