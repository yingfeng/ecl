/**
 * RawMarkdownEditor — raw markdown editing using Monaco Editor.
 * Theme follows the app's Nimbalyst theme system.
 */

import type { JSX } from 'react';
import { useEffect, useRef } from 'react';
import Editor from '@monaco-editor/react';
import type { OnMount, OnChange } from '@monaco-editor/react';
import type { editor as monacoEditor } from 'monaco-editor';
import { useTheme } from '../theme/ThemeContext';

interface Props {
  content: string;
  onChange?: (value: string) => void;
  readOnly?: boolean;
  onToggleSource?: () => void;
}

function defineTheme(monaco: typeof import('monaco-editor'), isDark: boolean) {
  const name = isDark ? 'llmwiki-dark' : 'llmwiki-light';
  monaco.editor.defineTheme(name, {
    base: isDark ? 'vs-dark' : 'vs',
    inherit: true,
    rules: [],
    colors: isDark ? {
      'editor.background': '#2d2d2d',
      'editor.foreground': '#ffffff',
      'editor.lineHighlightBackground': '#3a3a3a',
      'editor.selectionBackground': 'rgba(96,165,250,0.2)',
      'editorCursor.foreground': '#60a5fa',
      'editorLineNumber.foreground': '#808080',
      'editorLineNumber.activeForeground': '#b3b3b3',
      'editor.selectionHighlightBackground': 'rgba(96,165,250,0.1)',
    } : {
      'editor.background': '#ffffff',
      'editor.foreground': '#111827',
      'editor.lineHighlightBackground': '#f3f4f6',
      'editor.selectionBackground': 'rgba(59,130,246,0.2)',
      'editorCursor.foreground': '#3b82f6',
      'editorLineNumber.foreground': '#9ca3af',
      'editorLineNumber.activeForeground': '#6b7280',
      'editor.selectionHighlightBackground': 'rgba(59,130,246,0.1)',
    },
  });
  return name;
}

export default function RawMarkdownEditor({ content, onChange, readOnly = false, onToggleSource }: Props): JSX.Element {
  const { theme } = useTheme();
  const editorRef = useRef<monacoEditor.IStandaloneCodeEditor | null>(null);
  const monacoRef = useRef<typeof import('monaco-editor') | null>(null);
  const themeApplied = useRef<string>('');

  const handleMount: OnMount = (editor, monaco) => {
    editorRef.current = editor;
    monacoRef.current = monaco;
    const isDark = theme === 'dark';
    const name = defineTheme(monaco, isDark);
    monaco.editor.setTheme(name);
    themeApplied.current = name;
  };

  // Switch Monaco theme when app theme changes
  useEffect(() => {
    if (!monacoRef.current) return;
    const isDark = theme === 'dark';
    const expected = isDark ? 'llmwiki-dark' : 'llmwiki-light';
    if (themeApplied.current === expected) return;
    const name = defineTheme(monacoRef.current, isDark);
    monacoRef.current.editor.setTheme(name);
    themeApplied.current = name;
  }, [theme]);

  const handleChange: OnChange = (value) => {
    if (onChange && value !== undefined) {
      onChange(value);
    }
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', flex: 1, minHeight: 0 }}>
      <div style={{
        display: 'flex', alignItems: 'center', gap: 6,
        padding: '4px 12px', borderBottom: '1px solid var(--nim-border)',
        background: 'var(--nim-toolbar-bg)', flexShrink: 0,
      }}>
        <button
          onClick={onToggleSource}
          className="nim-mode-btn"
          style={{
            padding: '4px 10px', border: '1px solid var(--nim-primary)',
            borderRadius: 4, cursor: 'pointer', fontSize: 12, fontWeight: 500,
            background: 'var(--nim-bg-selected)',
            color: 'var(--nim-primary)',
            fontFamily: 'inherit',
          }}
        >
          ✏ WYSIWYG
        </button>
      </div>
      <div style={{ flex: 1, minHeight: 0 }}>
        <Editor
          height="100%"
          defaultLanguage="markdown"
          value={content}
          onChange={handleChange}
          onMount={handleMount}
          options={{
            readOnly,
            minimap: { enabled: false },
            fontSize: 14,
            fontFamily: "'SF Mono', 'JetBrains Mono', 'Fira Code', monospace",
            lineNumbers: 'on',
            scrollBeyondLastLine: false,
            wordWrap: 'on',
            tabSize: 2,
            renderLineHighlight: 'line',
            cursorBlinking: 'smooth',
            smoothScrolling: true,
            padding: { top: 16 },
            automaticLayout: true,
          }}
        />
      </div>
    </div>
  );
}
