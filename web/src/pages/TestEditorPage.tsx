import { useState } from 'react'
import LexicalEditor from '../lib/editor/LexicalEditor'

const TEST_MD = `# Hello World

This is a **test** paragraph with *italic* and \`code\`.

## Level 2 Heading

- List item 1
- List item 2
- List item 3

> Blockquote text here

\`\`\`mermaid
graph TD
  A[Start] --> B[End]
\`\`\`

\`\`\`javascript
const greeting = "Hello";
console.log(greeting);
\`\`\`

---

| Col 1 | Col 2 |
|-------|-------|
| A     | B     |
`

export default function TestEditorPage() {
  const [md, setMd] = useState(TEST_MD)

  return (
    <div style={{ height: '100vh', display: 'flex', flexDirection: 'column' }}>
      <div style={{ padding: '8px 16px', background: '#1a1a2e', borderBottom: '1px solid #2a2a4a', display: 'flex', gap: 8, alignItems: 'center' }}>
        <span style={{ color: '#4fc3f7', fontWeight: 700 }}>Test Editor</span>
        <button onClick={() => console.log('Current markdown:', md)} style={{ marginLeft: 'auto', padding: '4px 12px', background: '#232340', color: '#e0e0f0', border: '1px solid #2a2a4a', borderRadius: 4, cursor: 'pointer' }}>
          Log Content
        </button>
      </div>
      <div style={{ flex: 1, overflow: 'hidden' }}>
        <LexicalEditor
          content={md}
          onChange={(newMd) => setMd(newMd)}
        />
      </div>
    </div>
  )
}
