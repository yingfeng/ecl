/**
 * RawMarkdownEditor — raw markdown editing using Monaco Editor.
 */

import type { JSX } from 'react';
import Editor from '@monaco-editor/react';
import type { OnMount, OnChange } from '@monaco-editor/react';

interface Props {
  content: string;
  onChange?: (value: string) => void;
  readOnly?: boolean;
}

export default function RawMarkdownEditor({ content, onChange, readOnly = false }: Props): JSX.Element {
  const handleMount: OnMount = (editor, monaco) => {
    monaco.editor.defineTheme('llmwiki-dark', {
      base: 'vs-dark',
      inherit: true,
      rules: [],
      colors: {
        'editor.background': '#2d2d2d',
        'editor.foreground': '#ffffff',
        'editor.lineHighlightBackground': '#3a3a3a',
        'editor.selectionBackground': 'rgba(96,165,250,0.2)',
        'editorCursor.foreground': '#60a5fa',
        'editorLineNumber.foreground': '#808080',
        'editorLineNumber.activeForeground': '#b3b3b3',
        'editor.selectionHighlightBackground': 'rgba(96,165,250,0.1)',
      },
    });
    monaco.editor.setTheme('llmwiki-dark');
  };

  const handleChange: OnChange = (value) => {
    if (onChange && value !== undefined) {
      onChange(value);
    }
  };

  return (
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
  );
}
