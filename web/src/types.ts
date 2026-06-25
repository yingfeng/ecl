// === Types matching backend entity models ===

export interface FileCommit {
  id: string
  folder_id: string
  parent_id?: string
  message: string
  author_id: string
  file_count: number
  tree_state?: string
  create_time?: number
  create_date?: string
}

export interface FileCommitItem {
  id: string
  commit_id: string
  file_id: string
  operation: 'add' | 'modify' | 'delete' | 'rename'
  old_hash?: string
  new_hash?: string
  old_location?: string
  new_location?: string
  old_name?: string
  new_name?: string
}

export interface TreeNode {
  id: string
  name: string
  type: 'folder' | 'file'
  children?: TreeNode[]
  content_hash?: string
  location?: string
  size?: number
}

export interface Dataset {
  id: string
  tenant_id: string
  name: string
  embd_id: string
  created_by: string
  doc_num: number
  permission: string
  status?: string
  create_time?: number
}

export interface DocsFile {
  id: string
  parent_id: string
  tenant_id: string
  created_by: string
  name: string
  location?: string
  size: number
  type: string
  source_type: string
}

export interface Document {
  id: string
  kb_id: string
  name?: string
  type: string
  suffix: string
  created_by: string
  content_hash?: string
  size: number
}

// ===== Agent Compilation Types =====

export interface CompileTask {
  id: string
  status: 'pending' | 'running' | 'success' | 'failed'
  log: string
  created_at: string
  started_at?: string
  finished_at?: string
  result?: {
    commit_id: string
    files_created: number
  }
  error?: string
  log_preview?: string
}

// ===== Gbrain Cycle Types =====

export interface GbrainCycle {
  id: string
  status: 'pending' | 'running' | 'success' | 'failed' | 'skipped'
  log?: string
  agent_task_id?: string
  new_articles: number
  updated_articles: number
  facts_extracted: number
  takes_extracted: number
  patterns_found: number
  takes_consolidated: number
  created_at: string
  started_at?: string
  finished_at?: string
  error?: string
}

export interface GbrainCycleSummary {
  id: string
  status: string
  new_articles: number
  facts_extracted: number
  patterns_found: number
  created_at: string
  finished_at?: string
}
