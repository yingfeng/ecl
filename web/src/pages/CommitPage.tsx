import { useState, useEffect } from 'react'
import { useParams, Link } from 'react-router-dom'
import type { TreeNode } from '../types'
import * as api from '../api'
import MarkdownEditor from '../components/MarkdownEditor'

export default function CommitPage() {
  const { name, commitId } = useParams()
  const [tree, setTree] = useState<TreeNode | null>(null)
  const [curNode, setCurNode] = useState<TreeNode | null>(null)
  const [selFile, setSelFile] = useState<TreeNode | null>(null)
  const [content, setContent] = useState('')
  const [error, setError] = useState('')
  const [breadcrumb, setBreadcrumb] = useState<{ id: string; name: string }[]>([])

  useEffect(() => { loadCommit() }, [commitId])

  async function loadCommit() {
    if (!commitId) return
    try {
      const t = await api.getCommitTree(commitId)
      setTree(t)
      setCurNode(t)
      setBreadcrumb([{ id: t.id, name: t.name }])
    } catch (e: any) { setError('Failed to load commit: ' + e.message) }
  }

  function enterFolder(node: TreeNode) {
    setCurNode(node)
    setSelFile(null)
    setContent('')
    setBreadcrumb(prev => [...prev, { id: node.id, name: node.name }])
  }

  function goToBreadcrumb(idx: number) {
    if (!tree) return
    const target = findBreadcrumbNode(tree, breadcrumb, idx)
    if (!target) return
    setCurNode(target)
    setSelFile(null)
    setContent('')
    setBreadcrumb(breadcrumb.slice(0, idx + 1))
  }

  async function selectFile(file: TreeNode) {
    if (!commitId) return
    try {
      const text = await api.getCommitFileContent(commitId, file.id)
      setContent(text)
      setSelFile(file)
    } catch { setContent(''); setSelFile(file) }
  }

  const files = curNode?.children?.filter(c => c.type === 'file') || []
  const subfolders = curNode?.children?.filter(c => c.type === 'folder') || []

  return (
    <div className="app">
      <aside className="sidebar">
        <div className="sidebar-hdr">
          <span className="logo">llmwiki</span>
          <span className="badge badge-commit">HISTORY</span>
        </div>
        <div className="section-hdr" style={{padding:'12px 20px 6px'}}>
          <span style={{textTransform:'uppercase',letterSpacing:'1px',fontSize:'11px',color:'var(--fg3)'}}>
            Snapshot · {commitId?.slice(0,8)}
          </span>
          <Link to="/" className="btn-icon-sm" style={{textDecoration:'none'}}>↩</Link>
        </div>

        <div className="breadcrumb-bar">
          {breadcrumb.map((b, i) => (
            <span key={b.id}>
              {i > 0 && <span className="breadcrumb-sep">/</span>}
              <span className={`breadcrumb-item ${i === breadcrumb.length - 1 ? 'active' : ''}`}
                onClick={() => goToBreadcrumb(i)}>
                {b.name}
              </span>
            </span>
          ))}
        </div>

        <div className="files-section">
          {subfolders.map(f => (
            <div key={f.id} className="nav-item sub folder" onClick={() => enterFolder(f)}>
              <span>📁</span>
              <span className="nav-label" style={{marginLeft:8}}>{f.name}</span>
            </div>
          ))}
          {files.map(f => (
            <div key={f.id}
              className={`nav-item sub file ${selFile?.id === f.id ? 'active' : ''}`}
              onClick={() => selectFile(f)}>
              <span>📄</span>
              <span className="nav-label" style={{marginLeft:8}}>{f.name}</span>
            </div>
          ))}
          {subfolders.length === 0 && files.length === 0 && (
            <div className="hint" style={{padding:'12px 20px'}}>Empty folder</div>
          )}
        </div>
      </aside>

      <main className="main">
        {error && <div className="error-bar">{error}</div>}
        {selFile ? (
          <div className="editor-view" style={{display:'flex',flexDirection:'column',height:'100%'}}>
            <div className="editor-toolbar">
              <div className="editor-toolbar-left">
                <span><strong>{selFile.name}</strong></span>
                <span className="badge badge-commit" style={{marginLeft:8}}>Read-only</span>
              </div>
              <Link to="/" className="btn">Back to Home</Link>
            </div>
            <div style={{flex:1,overflow:'hidden'}}>
              <MarkdownEditor
                content={content}
                onChange={() => {}}
                readOnly
              />
            </div>
          </div>
        ) : (
          <div className="welcome">
            <h1>📜 Commit Snapshot</h1>
            <p>Browse the workspace tree in the sidebar</p>
            <p className="hint">Click folders to navigate · Click files to view content at this version</p>
            <Link to="/" className="btn-primary" style={{marginTop:16,display:'inline-flex',padding:'8px 18px',textDecoration:'none'}}>↩ Back to Home</Link>
          </div>
        )}
      </main>
    </div>
  )
}

function findBreadcrumbNode(tree: TreeNode, breadcrumb: { id: string; name: string }[], targetIdx: number): TreeNode | undefined {
  if (targetIdx === 0) return tree
  let result: TreeNode | undefined
  function walk(n: TreeNode, depth: number) {
    if (depth === targetIdx && n.id === breadcrumb[targetIdx]?.id) { result = n; return }
    if (result || depth >= targetIdx) return
    for (const c of n.children || []) walk(c, depth + 1)
  }
  walk(tree, 0)
  return result
}
