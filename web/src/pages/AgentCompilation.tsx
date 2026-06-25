import { useState, useEffect, useRef } from 'react'
import type { TreeNode, Dataset, CompileTask, GbrainCycle } from '../types'
import * as api from '../api'

export default function AgentCompilation() {
  const [tab, setTab] = useState<'compile' | 'gbrain' | 'history'>('compile')
  return (
    <div className="app">
      <nav className="sidebar">
        <div className="sidebar-hdr">
          <span className="logo">🧠 Agent Compiler</span>
        </div>
        <div className="sidebar-body">
          <div className="workspace-title">Compilation Mode</div>
          <div className={`nav-item ${tab === 'compile' ? 'active' : ''}`}
            onClick={() => setTab('compile')}>
            <span className="nav-folder-icon">▶</span>
            <span className="nav-label">Standard Pipeline</span>
          </div>
          <div className={`nav-item ${tab === 'gbrain' ? 'active' : ''}`}
            onClick={() => setTab('gbrain')}>
            <span className="nav-folder-icon">🔄</span>
            <span className="nav-label">Gbrain Cycle</span>
          </div>
          <div className={`nav-item ${tab === 'history' ? 'active' : ''}`}
            onClick={() => setTab('history')}>
            <span className="nav-folder-icon">📋</span>
            <span className="nav-label">Task History</span>
          </div>
        </div>
        <div style={{ padding: '8px 16px', borderTop: '1px solid var(--nim-border)' }}>
          <button className="btn-link" style={{ fontSize: 12, opacity: 0.6 }} onClick={() => window.location.href = '/'}>
            ← Back to Wiki
          </button>
          <button className="btn-link" style={{ fontSize: 12, opacity: 0.6, marginLeft: 8 }} onClick={() => window.location.href = '/orchestration'}>
            ⚙ Orchestration
          </button>
        </div>
      </nav>
      <main className="main" style={{ overflow: 'auto' }}>
        {tab === 'compile' && <CompilePanel />}
        {tab === 'gbrain' && <GbrainPanel />}
        {tab === 'history' && <HistoryPanel onView={(id) => {/* navigation handled by opening detail */}} />}
      </main>
    </div>
  )
}

// ===== Compile Panel =====

function CompilePanel() {
  const [datasets, setDatasets] = useState<Dataset[]>([])
  const [trees, setTrees] = useState<Record<string, TreeNode>>({})
  const [selectedDS, setSelectedDS] = useState('')
  const [workspaceID, setWorkspaceID] = useState('')
  const [workspaceName, setWorkspaceName] = useState('')
  const [instructions, setInstructions] = useState('')
  const [outputDir, setOutputDir] = useState('synthesis')
  const [commitMsg, setCommitMsg] = useState('Agent knowledge compilation')
  const [running, setRunning] = useState(false)
  const [activeTask, setActiveTask] = useState<string | null>(null)
  const [activeStatus, setActiveStatus] = useState('')
  const [log, setLog] = useState('')
  const logRef = useRef<HTMLPreElement>(null)

  useEffect(() => {
    api.listDatasets().then(ds => {
      setDatasets(ds)
      if (ds.length > 0) {
        setSelectedDS(ds[0].id)
        loadTree(ds[0].id)
      }
    })
  }, [])

  useEffect(() => {
    if (logRef.current) {
      logRef.current.scrollTop = logRef.current.scrollHeight
    }
  }, [log])

  useEffect(() => {
    if (!activeTask) return
    // Poll for status/log updates
    const id = setInterval(async () => {
      try {
        const task = await api.getCompileTask(activeTask)
        setActiveStatus(task.status)
        setLog(task.log || '')
        if (task.status === 'success' || task.status === 'failed') {
          setRunning(false)
          clearInterval(id)
        }
      } catch {}
    }, 1000)
    return () => clearInterval(id)
  }, [activeTask])

  async function loadTree(dsID: string) {
    if (trees[dsID]) return
    try {
      const tree = await api.getFolderTree(dsID)
      setTrees(t => ({ ...t, [dsID]: tree }))
    } catch {}
  }

  async function selectWorkspace(folder: TreeNode) {
    setWorkspaceID(folder.id)
    setWorkspaceName(folder.name)
  }

  async function handleRun() {
    if (!workspaceID) return
    setRunning(true)
    setLog('[SYSTEM] Starting compilation...\n')
    setActiveStatus('pending')

    try {
      const result = await api.startCompile({
        workspace_id: workspaceID,
        instructions: instructions || undefined,
        output_dir: outputDir || undefined,
        commit_message: commitMsg || undefined,
      })
      setActiveTask(result.task_id)
      setActiveStatus(result.status)
      setLog(prev => prev + `[SYSTEM] Task created: ${result.task_id}\n`)
    } catch (err: any) {
      setLog(prev => prev + `[ERROR] ${err.message}\n`)
      setRunning(false)
    }
  }

  const workspaceFolders = selectedDS
    ? (trees[selectedDS]?.children?.filter(c => c.type === 'folder') || [])
    : []

  return (
    <div className="welcome" style={{ textAlign: 'left', maxWidth: '100%' }}>
      <h1>🧠 Agent Compiler</h1>
      <p className="hint" style={{ marginBottom: 24 }}>
        Select a workspace and run the knowledge compilation agent.
      </p>

      <div style={{ background: 'var(--nim-bg-secondary)', borderRadius: 8, border: '1px solid var(--nim-border)', padding: 20, marginBottom: 20 }}>
        {/* Dataset Select */}
        <div className="form-field">
          <label className="form-label">Knowledge Base</label>
          <select className="rename-input" value={selectedDS} onChange={e => { setSelectedDS(e.target.value); loadTree(e.target.value) }}>
            {datasets.map(d => <option key={d.id} value={d.id}>{d.name}</option>)}
          </select>
        </div>

        {/* Workspace Select */}
        <div className="form-field">
          <label className="form-label">Data Source (Workspace)</label>
          {workspaceFolders.length === 0 ? (
            <p className="hint" style={{ marginTop: 4 }}>No workspaces found. Create a workspace first.</p>
          ) : (
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6, marginTop: 4 }}>
              {workspaceFolders.map(f => (
                <div key={f.id} className="file-card" style={{
                  cursor: 'pointer', padding: '8px 14px', flexDirection: 'row', gap: 6,
                  background: f.id === workspaceID ? 'var(--nim-bg-selected)' : 'var(--nim-bg-tertiary)',
                  border: f.id === workspaceID ? '1px solid var(--nim-primary)' : '1px solid var(--nim-border)',
                }} onClick={() => selectWorkspace(f)}>
                  <span>📁</span>
                  <span style={{ fontSize: 13 }}>{f.name}</span>
                </div>
              ))}
            </div>
          )}
          {workspaceName && <p style={{ fontSize: 12, color: 'var(--nim-primary)', marginTop: 4 }}>Selected: {workspaceName} ({workspaceID.slice(0, 8)}...)</p>}
        </div>

        {/* Output Directory */}
        <div className="form-field">
          <label className="form-label">Output Directory</label>
          <input className="rename-input" value={outputDir} onChange={e => setOutputDir(e.target.value)}
            placeholder="synthesis" style={{ width: '100%' }} />
        </div>

        {/* Instructions */}
        <div className="form-field">
          <label className="form-label">Instructions</label>
          <textarea className="rename-input" value={instructions} onChange={e => setInstructions(e.target.value)}
            rows={3} placeholder="Describe what the agent should do..." style={{ width: '100%', resize: 'vertical' }} />
        </div>

        {/* Commit Message */}
        <div className="form-field">
          <label className="form-label">Commit Message</label>
          <input className="rename-input" value={commitMsg} onChange={e => setCommitMsg(e.target.value)}
            style={{ width: '100%' }} />
        </div>

        {/* Run Button */}
        <div style={{ marginTop: 16 }}>
          <button className="btn-primary" onClick={handleRun} disabled={running || !workspaceID}
            style={{ opacity: (running || !workspaceID) ? 0.5 : 1 }}>
            {running ? '⏳ Running...' : '▶ Run Compilation'}
          </button>
        </div>
      </div>

      {/* Log Viewer */}
      {(log || running) && (
        <div style={{ marginTop: 16 }}>
          <h2 className="workspace-title" style={{ padding: '8px 0' }}>Agent Log</h2>
          <div style={{ display: 'flex', gap: 6, marginBottom: 8, fontSize: 12, color: 'var(--nim-text-muted)' }}>
            <span>Status: <strong style={{ color: activeStatus === 'success' ? '#2ECC71' : activeStatus === 'failed' ? '#E74C3C' : activeStatus === 'running' ? '#50C878' : '#FFB347' }}>{activeStatus || 'pending'}</strong></span>
          </div>
          <pre ref={logRef} style={{
            background: '#1a1a2e', color: '#e0e0e0', borderRadius: 8, padding: 16, fontSize: 12,
            lineHeight: 1.5, maxHeight: 600, overflow: 'auto',
            fontFamily: "'SF Mono','Fira Code','Consolas',monospace", whiteSpace: 'pre-wrap', wordBreak: 'break-all',
          }}>
            {log || 'Waiting...\n'}
            {running && <span style={{ animation: 'pulse 1s infinite', opacity: 0.5 }}>▌</span>}
          </pre>
          <style>{`@keyframes pulse { 50% { opacity: 1; } }`}</style>
        </div>
      )}
    </div>
  )
}

// ===== History Panel =====

function HistoryPanel({ onView }: { onView: (id: string) => void }) {
  const [tasks, setTasks] = useState<CompileTask[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    load()
    const id = setInterval(load, 5000)
    return () => clearInterval(id)
  }, [])

  async function load() {
    try {
      setTasks(await api.listCompileTasks())
    } catch {}
    setLoading(false)
  }

  return (
    <div className="welcome" style={{ textAlign: 'left' }}>
      <h1>📋 Task History</h1>
      <p className="hint" style={{ marginBottom: 16 }}>Recently executed compilation tasks</p>

      {loading ? <p>Loading...</p> : tasks.length === 0 ? (
        <p className="hint">No tasks yet.</p>
      ) : (
        <div style={{ display: 'grid', gap: 8 }}>
          {tasks.sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime()).map(t => (
            <div key={t.id} style={{
              background: 'var(--nim-bg-secondary)', borderRadius: 8, padding: '12px 16px',
              border: '1px solid var(--nim-border)', cursor: 'pointer',
            }} onClick={() => onView(t.id)}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 4 }}>
                <span style={{ fontSize: 12, fontFamily: 'monospace', color: 'var(--nim-text-muted)' }}>{t.id.slice(0, 12)}...</span>
                <StatusBadge status={t.status} />
              </div>
              <div style={{ fontSize: 12, color: 'var(--nim-text-muted)' }}>
                {new Date(t.created_at).toLocaleString()}
              </div>
              {t.log_preview && (
                <div style={{ fontSize: 11, color: 'var(--nim-text-muted)', marginTop: 4, maxHeight: 40, overflow: 'hidden' }}>
                  {t.log_preview}
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

function StatusBadge({ status }: { status: string }) {
  const colors: Record<string, string> = {
    pending: '#FFB347', running: '#50C878', success: '#2ECC71', failed: '#E74C3C',
  }
  return (
    <span style={{
      display: 'inline-block', padding: '2px 8px', borderRadius: 10, fontSize: 11, fontWeight: 600,
      background: `${colors[status] || '#95A5A6'}20`, color: colors[status] || '#95A5A6',
    }}>{status}</span>
  )
}

// ===== Gbrain Cycle Panel =====

function GbrainPanel() {
  const [datasets, setDatasets] = useState<Dataset[]>([])
  const [trees, setTrees] = useState<Record<string, TreeNode>>({})
  const [selectedDS, setSelectedDS] = useState('')
  const [workspaceID, setWorkspaceID] = useState('')
  const [workspaceName, setWorkspaceName] = useState('')
  const [instructions, setInstructions] = useState('')
  const [outputDir, setOutputDir] = useState('synthesis')
  const [running, setRunning] = useState(false)
  const [activeCycle, setActiveCycle] = useState<string | null>(null)
  const [activeStatus, setActiveStatus] = useState('')
  const [log, setLog] = useState('')
  const [result, setResult] = useState<GbrainCycle | null>(null)
  const logRef = useRef<HTMLPreElement>(null)

  useEffect(() => {
    api.listDatasets().then(ds => {
      setDatasets(ds)
      if (ds.length > 0) {
        setSelectedDS(ds[0].id)
        loadTree(ds[0].id)
      }
    })
  }, [])

  useEffect(() => {
    if (logRef.current) {
      logRef.current.scrollTop = logRef.current.scrollHeight
    }
  }, [log])

  useEffect(() => {
    if (!activeCycle) return
    const id = setInterval(async () => {
      try {
        const cycle = await api.getGbrainCycle(activeCycle)
        setActiveStatus(cycle.status)
        setLog(cycle.log || '')
        if (cycle.status === 'success' || cycle.status === 'failed') {
          setRunning(false)
          setResult(cycle)
          clearInterval(id)
        }
      } catch {}
    }, 1000)
    return () => clearInterval(id)
  }, [activeCycle])

  async function loadTree(dsID: string) {
    if (trees[dsID]) return
    try {
      const tree = await api.getFolderTree(dsID)
      setTrees(t => ({ ...t, [dsID]: tree }))
    } catch {}
  }

  async function selectWorkspace(folder: TreeNode) {
    setWorkspaceID(folder.id)
    setWorkspaceName(folder.name)
  }

  async function handleRun() {
    if (!workspaceID) return
    setRunning(true)
    setResult(null)
    setLog('[GBRAIN] Starting gbrain cycle...\n')
    setActiveStatus('pending')

    try {
      const result = await api.startGbrainCycle({
        workspace_id: workspaceID,
        instructions: instructions || undefined,
        output_dir: outputDir || undefined,
      })
      setActiveCycle(result.cycle_id)
      setActiveStatus(result.status)
      setLog(prev => prev + `[GBRAIN] Cycle created: ${result.cycle_id}\n`)
    } catch (err: any) {
      setLog(prev => prev + `[ERROR] ${err.message}\n`)
      setRunning(false)
    }
  }

  const workspaceFolders = selectedDS
    ? (trees[selectedDS]?.children?.filter(c => c.type === 'folder') || [])
    : []

  return (
    <div className="welcome" style={{ textAlign: 'left', maxWidth: '100%' }}>
      <h1>🔄 Gbrain Cycle Compilation</h1>
      <p className="hint" style={{ marginBottom: 16 }}>
        Gbrain-style multi-phase knowledge compilation. Runs standard pipeline then adds structured knowledge extraction (facts/takes), cross-session pattern discovery, and fact consolidation.
      </p>

      {/* Phase description card */}
      <div style={{ background: 'var(--nim-bg-secondary)', borderRadius: 8, border: '1px solid var(--nim-border)', padding: 16, marginBottom: 20, fontSize: 13 }}>
        <strong style={{ display: 'block', marginBottom: 8 }}>Gbrain Cycle Phases:</strong>
        <div style={{ display: 'grid', gridTemplateColumns: 'auto 1fr', gap: '4px 12px', opacity: 0.85 }}>
          <span>1-5.</span><span>Standard pipeline (detect → load → keywords → scan → compile)</span>
          <span>6.</span><span><strong>Extract Facts & Takes</strong> — LLM extracts structured knowledge from each article</span>
          <span>7.</span><span><strong>Discover Patterns</strong> — Cross-article theme detection (≥3 articles)</span>
          <span>8.</span><span><strong>Consolidate Facts → Takes</strong> — Group facts, promote to takes</span>
          <span>9.</span><span><strong>Write Enhanced Output</strong> — Articles include `## 知识快照` with facts/takes fences</span>
        </div>
      </div>

      <div style={{ background: 'var(--nim-bg-secondary)', borderRadius: 8, border: '1px solid var(--nim-border)', padding: 20, marginBottom: 20 }}>
        <div className="form-field">
          <label className="form-label">Knowledge Base</label>
          <select className="rename-input" value={selectedDS} onChange={e => { setSelectedDS(e.target.value); loadTree(e.target.value) }}>
            {datasets.map(d => <option key={d.id} value={d.id}>{d.name}</option>)}
          </select>
        </div>

        <div className="form-field">
          <label className="form-label">Data Source (Workspace)</label>
          {workspaceFolders.length === 0 ? (
            <p className="hint" style={{ marginTop: 4 }}>No workspaces found.</p>
          ) : (
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6, marginTop: 4 }}>
              {workspaceFolders.map(f => (
                <div key={f.id} className="file-card" style={{
                  cursor: 'pointer', padding: '8px 14px', flexDirection: 'row', gap: 6,
                  background: f.id === workspaceID ? 'var(--nim-bg-selected)' : 'var(--nim-bg-tertiary)',
                  border: f.id === workspaceID ? '1px solid var(--nim-primary)' : '1px solid var(--nim-border)',
                }} onClick={() => selectWorkspace(f)}>
                  <span>📁</span>
                  <span style={{ fontSize: 13 }}>{f.name}</span>
                </div>
              ))}
            </div>
          )}
          {workspaceName && <p style={{ fontSize: 12, color: 'var(--nim-primary)', marginTop: 4 }}>Selected: {workspaceName}</p>}
        </div>

        <div className="form-field">
          <label className="form-label">Output Directory</label>
          <input className="rename-input" value={outputDir} onChange={e => setOutputDir(e.target.value)}
            placeholder="synthesis" style={{ width: '100%' }} />
        </div>

        <div className="form-field">
          <label className="form-label">Instructions</label>
          <textarea className="rename-input" value={instructions} onChange={e => setInstructions(e.target.value)}
            rows={2} placeholder="Optional instructions..." style={{ width: '100%', resize: 'vertical' }} />
        </div>

        <div style={{ marginTop: 16 }}>
          <button className="btn-primary" onClick={handleRun} disabled={running || !workspaceID}
            style={{ opacity: (running || !workspaceID) ? 0.5 : 1 }}>
            {running ? '⏳ Running Cycle...' : '▶ Start Gbrain Cycle'}
          </button>
        </div>
      </div>

      {/* Result summary */}
      {result && result.status === 'success' && (
        <div style={{ background: '#1a3a2a', borderRadius: 8, border: '1px solid #2ECC71', padding: 16, marginBottom: 16 }}>
          <h3 style={{ margin: '0 0 12px', color: '#2ECC71', fontSize: 14 }}>✅ Gbrain Cycle Complete</h3>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: 12, fontSize: 13 }}>
            <div style={{ background: 'rgba(46,204,113,0.1)', borderRadius: 6, padding: 8, textAlign: 'center' }}>
              <div style={{ fontSize: 24, fontWeight: 700, color: '#2ECC71' }}>{result.new_articles}</div>
              <div style={{ fontSize: 11, opacity: 0.7 }}>Articles</div>
            </div>
            <div style={{ background: 'rgba(52,152,219,0.1)', borderRadius: 6, padding: 8, textAlign: 'center' }}>
              <div style={{ fontSize: 24, fontWeight: 700, color: '#3498DB' }}>{result.facts_extracted}</div>
              <div style={{ fontSize: 11, opacity: 0.7 }}>Facts Extracted</div>
            </div>
            <div style={{ background: 'rgba(155,89,182,0.1)', borderRadius: 6, padding: 8, textAlign: 'center' }}>
              <div style={{ fontSize: 24, fontWeight: 700, color: '#9B59B6' }}>{result.patterns_found}</div>
              <div style={{ fontSize: 11, opacity: 0.7 }}>Patterns</div>
            </div>
          </div>
          {result.updated_articles > 0 && (
            <p style={{ margin: '8px 0 0', fontSize: 12, opacity: 0.7 }}>
              {result.updated_articles} articles updated with facts/takes
            </p>
          )}
          {result.takes_consolidated > 0 && (
            <p style={{ margin: '4px 0 0', fontSize: 12, opacity: 0.7 }}>
              {result.takes_consolidated} facts consolidated into takes
            </p>
          )}
        </div>
      )}

      {/* Log */}
      {(log || running) && (
        <div style={{ marginTop: 16 }}>
          <h2 className="workspace-title" style={{ padding: '8px 0' }}>Cycle Log</h2>
          <div style={{ display: 'flex', gap: 6, marginBottom: 8, fontSize: 12, color: 'var(--nim-text-muted)' }}>
            Status: <strong style={{ color: activeStatus === 'success' ? '#2ECC71' : activeStatus === 'failed' ? '#E74C3C' : '#50C878' }}>{activeStatus}</strong>
          </div>
          <pre ref={logRef} style={{
            background: '#1a1a2e', color: '#e0e0e0', borderRadius: 8, padding: 16, fontSize: 12,
            lineHeight: 1.5, maxHeight: 500, overflow: 'auto',
            fontFamily: "'SF Mono','Fira Code','Consolas',monospace", whiteSpace: 'pre-wrap', wordBreak: 'break-all',
          }}>
            {log || 'Waiting...\n'}
            {running && <span style={{ animation: 'pulse 1s infinite', opacity: 0.5 }}>▌</span>}
          </pre>
        </div>
      )}
    </div>
  )
}
