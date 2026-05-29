/**
 * MarkdownEditor — mode-switching wrapper.
 *
 * Supports two modes:
 * - "wysiwyg": Lexical-based WYSIWYG editing (default)
 * - "raw": Monaco-based raw markdown editing
 */

import { useState, useEffect, useRef, useCallback } from 'react'
import LexicalEditor from '../lib/editor/LexicalEditor'
import RawMarkdownEditor from '../lib/editor/RawMarkdownEditor'

interface Props {
  content: string
  onChange: (content: string) => void
  readOnly?: boolean
  fileName?: string
}

type EditorMode = 'wysiwyg' | 'raw'

export default function MarkdownEditor({ content, onChange, readOnly, fileName }: Props) {
  const [mode, setMode] = useState<EditorMode>('wysiwyg')
  const [rawContent, setRawContent] = useState(content)
  const contentRef = useRef(content)

  useEffect(() => {
    contentRef.current = content
    if (mode === 'raw') {
      setRawContent(content)
    }
  }, [mode, content])

  const handleWysiwygChange = useCallback((md: string) => {
    contentRef.current = md
    onChange(md)
  }, [onChange])

  const handleRawChange = useCallback((value: string) => {
    setRawContent(value)
    contentRef.current = value
    onChange(value)
  }, [onChange])

  function switchMode(m: EditorMode) {
    if (m === 'raw') {
      setRawContent(contentRef.current)
    }
    setMode(m)
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', flex: 1, minHeight: 0 }}>
      {!readOnly && (
        <div className="nim-mode-bar">
          <span style={{ fontSize: 11, color: 'var(--nim-text-faint)', marginRight: 8, letterSpacing: 0.5 }}>
            MODE
          </span>
          <button
            className={`nim-mode-btn ${mode === 'wysiwyg' ? 'active' : ''}`}
            onClick={() => switchMode('wysiwyg')}
          >
            ✏ WYSIWYG
          </button>
          <button
            className={`nim-mode-btn ${mode === 'raw' ? 'active' : ''}`}
            onClick={() => switchMode('raw')}
          >
            # Raw
          </button>
          <div style={{ flex: 1 }} />
          {fileName && (
            <span style={{ fontSize: 11, color: 'var(--nim-text-faint)' }}>{fileName}</span>
          )}
        </div>
      )}

      <div className="nim-editor-container">
        {mode === 'wysiwyg' ? (
          <LexicalEditor
            content={content}
            onChange={handleWysiwygChange}
            readOnly={readOnly}
            placeholder={readOnly ? '' : 'Start writing...'}
          />
        ) : (
          <RawMarkdownEditor
            content={rawContent}
            onChange={handleRawChange}
            readOnly={readOnly}
          />
        )}
      </div>
    </div>
  )
}
