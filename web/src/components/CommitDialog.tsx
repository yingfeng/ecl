import { useState } from 'react'

interface PendingFile {
  id: string
  name: string
}

interface Props {
  files: PendingFile[]
  onConfirm: (message: string) => void
  onCancel: () => void
}

export default function CommitDialog({ files, onConfirm, onCancel }: Props) {
  const [message, setMessage] = useState('')

  return (
    <div className="dialog-overlay" onClick={onCancel}>
      <div className="dialog" onClick={e => e.stopPropagation()}>
        <h3>✏️ 提交修改</h3>

        <div className="form-group">
          <label>提交说明</label>
          <input
            type="text"
            value={message}
            onChange={e => setMessage(e.target.value)}
            placeholder="描述本次修改..."
            autoFocus
          />
        </div>

        <div className="form-group">
          <label>包含的文件</label>
          <ul className="file-list">
            {files.map(f => (
              <li key={f.id}>📄 {f.name}</li>
            ))}
          </ul>
        </div>

        <div className="dialog-actions">
          <button className="btn" onClick={onCancel}>取消</button>
          <button
            className="btn btn-primary"
            onClick={() => onConfirm(message)}
            disabled={!message.trim()}
          >
            提交
          </button>
        </div>
      </div>
    </div>
  )
}
