import type { TreeNode } from '../types'

interface Props {
  file: TreeNode
  content: string
  onChange: (content: string) => void
  readOnly: boolean
}

export default function Editor({ file, content, onChange, readOnly }: Props) {
  return (
    <div className="editor-container">
      <div className="editor-header">
        <span className="editor-filename">📄 {file.name}</span>
        {readOnly && <span className="badge read-only">只读</span>}
      </div>
      <div className="editor-body">
        <textarea
          className="editor-textarea"
          value={content}
          onChange={e => onChange(e.target.value)}
          readOnly={readOnly}
          placeholder="在此输入 Markdown 内容..."
          spellCheck={false}
        />
        <div className="preview-pane">
          <div className="preview-header">预览</div>
          <div className="preview-content">
            {renderPreview(content)}
          </div>
        </div>
      </div>
    </div>
  )
}

function renderPreview(md: string): JSX.Element {
  // Simple markdown rendering
  const html = md
    .replace(/^### (.+)$/gm, '<h3>$1</h3>')
    .replace(/^## (.+)$/gm, '<h2>$1</h2>')
    .replace(/^# (.+)$/gm, '<h1>$1</h1>')
    .replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
    .replace(/\*(.+?)\*/g, '<em>$1</em>')
    .replace(/```(\w*)\n([\s\S]*?)```/g, '<pre><code>$2</code></pre>')
    .replace(/`(.+?)`/g, '<code>$1</code>')
    .replace(/^- (.+)$/gm, '<li>$1</li>')
    .replace(/^(\d+)\. (.+)$/gm, '<li>$2</li>')
    .replace(/\n\n/g, '</p><p>')
    .replace(/\n/g, '<br/>')

  return <div dangerouslySetInnerHTML={{ __html: '<p>' + html + '</p>' }} />
}
