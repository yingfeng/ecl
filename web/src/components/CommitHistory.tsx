import type { FileCommit } from '../types'

interface Props {
  commits: FileCommit[]
  selectedCommit: FileCommit | null
  onSelect: (commit: FileCommit) => void
  onClose: () => void
}

export default function CommitHistory({ commits, selectedCommit, onSelect, onClose }: Props) {
  return (
    <div className="commit-panel">
      <div className="commit-panel-header">
        <span>📋 提交历史</span>
        <button className="btn-sm" onClick={onClose}>✕</button>
      </div>
      {commits.length === 0 && (
        <div className="commit-empty">暂无提交记录</div>
      )}
      {commits.map(c => (
        <div
          key={c.id}
          className={`commit-item ${selectedCommit?.id === c.id ? 'active' : ''}`}
          onClick={() => onSelect(c)}
        >
          <div className="commit-message">{c.message || '(无描述)'}</div>
          <div className="commit-meta">
            {c.file_count} 个文件 · {formatTime(c.create_time)}
          </div>
        </div>
      ))}
    </div>
  )
}

function formatTime(ts?: number): string {
  if (!ts) return ''
  const d = new Date(ts)
  return d.toLocaleString('zh-CN')
}
