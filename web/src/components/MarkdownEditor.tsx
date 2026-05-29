/**
 * MarkdownEditor — supports WYSIWYG (Lexical) and Raw (Monaco) modes.
 * Mode switching is triggered by the editor toolbar's "Toggle Source" button.
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

export default function MarkdownEditor({ content, onChange, readOnly, fileName }: Props) {
  const [showSource, setShowSource] = useState(false)
  const [rawContent, setRawContent] = useState(content)
  const contentRef = useRef(content)

  useEffect(() => {
    contentRef.current = content
    if (showSource) {
      setRawContent(content)
    }
  }, [showSource, content])

  const handleWysiwygChange = useCallback((md: string) => {
    contentRef.current = md
    onChange(md)
  }, [onChange])

  const handleRawChange = useCallback((value: string) => {
    setRawContent(value)
    contentRef.current = value
    onChange(value)
  }, [onChange])

  const toggleSource = useCallback(() => {
    if (!showSource) {
      setRawContent(contentRef.current)
    }
    setShowSource(prev => !prev)
  }, [showSource])

  return (
    <div style={{ display: 'flex', flexDirection: 'column', flex: 1, minHeight: 0 }}>
      <div className="nim-editor-container">
        {showSource ? (
          <RawMarkdownEditor
            content={rawContent}
            onChange={handleRawChange}
            readOnly={readOnly}
            onToggleSource={toggleSource}
          />
        ) : (
          <LexicalEditor
            content={content}
            onChange={handleWysiwygChange}
            readOnly={readOnly}
            placeholder={readOnly ? '' : 'Start writing...'}
            onToggleSource={toggleSource}
            showSource={showSource}
          />
        )}
      </div>
    </div>
  )
}
