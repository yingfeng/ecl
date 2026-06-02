/**
 * Export utilities — download files for Obsidian.
 */
import JSZip from 'jszip';
import * as api from '../api';
import type { TreeNode } from '../types';

/** Recursively collect all .md files from a tree node. */
function collectMdFiles(node: TreeNode): { id: string; name: string; path: string }[] {
  const results: { id: string; name: string; path: string }[] = [];
  const walk = (n: TreeNode, prefix: string) => {
    if (n.type === 'file' && n.name.endsWith('.md')) {
      results.push({ id: n.id, name: n.name, path: prefix + n.name });
    }
    if (n.children) {
      for (const child of n.children) {
        walk(child, n.type === 'folder' ? prefix + n.name + '/' : prefix);
      }
    }
  };
  walk(node, '');
  return results;
}

/** Download a single file from a commit as .md. */
export async function downloadCommitFile(commitId: string, fileId: string, fileName: string) {
  const content = await api.getCommitFileContent(commitId, fileId);
  const blob = new Blob([content], { type: 'text/markdown' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = fileName.replace(/\.md$/i, '') + '.md';
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}

/** Download all .md files from a commit as a ZIP archive. */
export async function downloadAllCommitFiles(commitId: string, label: string) {
  const tree = await api.getCommitTree(commitId);
  const files = collectMdFiles(tree);

  if (files.length === 0) {
    alert('No markdown files found in this commit.');
    return;
  }

  const zip = new JSZip();
  let loaded = 0;

  for (const f of files) {
    try {
      const content = await api.getCommitFileContent(commitId, f.id);
      zip.file(f.path, content);
      loaded++;
    } catch {
      console.warn('[Export] Failed to fetch:', f.path);
    }
  }

  if (loaded === 0) {
    alert('Failed to load any files.');
    return;
  }

  const blob = await zip.generateAsync({ type: 'blob' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `${label || 'llmwiki-export'}-${commitId.slice(0, 8)}.zip`;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}
