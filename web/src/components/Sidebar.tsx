import type { FileCommit, TreeNode } from '../types'
import CommitHistory from './CommitHistory'

interface Props {
  rootFolder: TreeNode | null
  currentFolder: TreeNode | null
  commits: FileCommit[]
  selectedCommit: FileCommit | null
  selectedFile: TreeNode | null
  viewMode: 'current' | 'commit'
  commitTree: TreeNode | null
  onSelectFolder: (folder: TreeNode) => void
  onSelectFile: (file: TreeNode) => void
  onCommit: () => void
  onViewCommit: (commit: FileCommit) => void
  onViewCommitFile: (file: TreeNode) => void
  onSwitchCurrent: () => void
  onToggleCommits: () => void
  showCommitsPanel: boolean
  dirty: boolean
}

export default function Sidebar({
  rootFolder,
  currentFolder,
  commits,
  selectedCommit,
  selectedFile,
  viewMode,
  commitTree,
  onSelectFolder,
  onSelectFile,
  onCommit,
  onViewCommit,
  onViewCommitFile,
  onSwitchCurrent,
  onToggleCommits,
  showCommitsPanel,
  dirty,
}: Props) {
  return (
    <div className="sidebar">
      <div className="sidebar-header">
        <h2>llmwiki</h2>
        <span className="badge">{viewMode === 'commit' ? '📜 历史' : '📝 当前'}</span>
      </div>

      {/* Level-1 folders */}
      {rootFolder?.children?.filter(c => c.type === 'folder').map(folder => (
        <div key={folder.id} className="tree-item-wrapper">
          <div
            className={`tree-item folder ${currentFolder?.id === folder.id ? 'active' : ''}`}
            onClick={() => onSelectFolder(folder)}
          >
            📁 {folder.name}
          </div>
        </div>
      ))}

      {/* Current folder files */}
      {currentFolder && viewMode === 'current' && (
        <>
          <div className="section-header">
            <span>📂 {currentFolder.name}</span>
            <div className="section-actions">
              <button className="btn-icon" onClick={onCommit} title="提交" disabled={!dirty}>
                ✏️
              </button>
              <button className="btn-icon" onClick={onToggleCommits} title="历史">
                📋
              </button>
            </div>
          </div>
          {currentFolder.children?.filter(c => c.type === 'file').map(file => (
            <div
              key={file.id}
              className={`tree-item file ${selectedFile?.id === file.id ? 'active' : ''}`}
              onClick={() => onSelectFile(file)}
            >
              📄 {file.name}
            </div>
          ))}
        </>
      )}

      {/* Commit version tree */}
      {commitTree && viewMode === 'commit' && (
        <>
          <div className="section-header">
            <span>📜 {selectedCommit?.message || '历史版本'}</span>
            <button className="btn-icon" onClick={onSwitchCurrent} title="返回当前">↩</button>
          </div>
          {renderCommitTreeNodes(commitTree, selectedFile, onViewCommitFile)}
        </>
      )}

      {/* Commit history */}
      {showCommitsPanel && (
        <CommitHistory
          commits={commits}
          selectedCommit={selectedCommit}
          onSelect={onViewCommit}
          onClose={onToggleCommits}
        />
      )}
    </div>
  )
}

function renderCommitTreeNodes(
  node: TreeNode,
  selectedFile: TreeNode | null,
  onClick: (f: TreeNode) => void
): JSX.Element {
  if (!node.children) return <></>
  return (
    <>
      {node.children.map(child =>
        child.type === 'folder' ? (
          <div key={child.id}>
            <div className="tree-item folder-label">📁 {child.name}</div>
            {renderCommitTreeNodes(child, selectedFile, onClick)}
          </div>
        ) : (
          <div
            key={child.id}
            className={`tree-item file ${selectedFile?.id === child.id ? 'active' : ''}`}
            onClick={() => onClick(child)}
          >
            📄 {child.name}
          </div>
        )
      )}
    </>
  )
}
