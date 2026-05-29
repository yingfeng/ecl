/**
 * MermaidNode - A Lexical DecoratorNode for rendering Mermaid diagrams.
 * Ported from Nimbalyst.
 */

import type { JSX } from 'react';
import type {
  DOMConversionMap,
  DOMConversionOutput,
  DOMExportOutput,
  EditorConfig,
  LexicalEditor,
  LexicalNode,
  NodeKey,
  SerializedLexicalNode,
  Spread,
} from 'lexical';
import {
  $applyNodeReplacement,
  DecoratorNode,
} from 'lexical';
import { addClassNamesToElement } from '@lexical/utils';

export interface MermaidPayload {
  content: string;
  key?: NodeKey;
}

export type SerializedMermaidNode = Spread<
  { content: string },
  SerializedLexicalNode
>;

export class MermaidNode extends DecoratorNode<JSX.Element> {
  __content: string;

  constructor(content: string, key?: NodeKey) {
    super(key);
    this.__content = content;
  }

  static getType(): string {
    return 'mermaid';
  }

  static clone(node: MermaidNode): MermaidNode {
    return new MermaidNode(node.__content, node.__key);
  }

  static importJSON(serializedNode: SerializedMermaidNode): MermaidNode {
    return $createMermaidNode({ content: serializedNode.content });
  }

  exportJSON(): SerializedMermaidNode {
    return {
      content: this.__content,
      type: 'mermaid',
      version: 1,
    };
  }

  createDOM(_config: EditorConfig, _editor: LexicalEditor): HTMLElement {
    const div = document.createElement('div');
    addClassNamesToElement(div, 'mermaid-container');
    return div;
  }

  updateDOM(_prevNode: MermaidNode, _dom: HTMLElement): boolean {
    return _prevNode.__content !== this.__content;
  }

  exportDOM(_editor: LexicalEditor): DOMExportOutput {
    const element = document.createElement('div');
    element.classList.add('mermaid-container');
    const pre = document.createElement('pre');
    const code = document.createElement('code');
    code.classList.add('language-mermaid');
    code.textContent = this.__content;
    pre.appendChild(code);
    element.appendChild(pre);
    return { element };
  }

  static importDOM(): DOMConversionMap | null {
    return {
      div: (domNode: HTMLElement) => {
        if (!domNode.classList.contains('mermaid-container')) return null;
        return { conversion: convertMermaidElement, priority: 1 };
      },
    };
  }

  getContent(): string {
    return this.__content;
  }

  getTextContent(): string {
    return this.__content;
  }

  setContent(content: string): void {
    const writable = this.getWritable();
    writable.__content = content;
  }

  decorate(_editor: LexicalEditor, config: EditorConfig): JSX.Element {
    return <MermaidComponent content={this.__content} nodeKey={this.__key} />;
  }
}

function convertMermaidElement(domNode: HTMLElement): DOMConversionOutput | null {
  const codeElement = domNode.querySelector('code.language-mermaid');
  if (codeElement) {
    const node = $createMermaidNode({ content: codeElement.textContent || '' });
    return { node };
  }
  return null;
}

export function $createMermaidNode(payload?: MermaidPayload): MermaidNode {
  const content = payload?.content || 'graph TD\n  A[Start] --> B[End]';
  return $applyNodeReplacement(new MermaidNode(content, payload?.key));
}

export function $isMermaidNode(node: LexicalNode | null | undefined): node is MermaidNode {
  return node instanceof MermaidNode;
}

// ─── React rendering component ─────────────────────────────

import { useEffect, useRef, useState } from 'react';
import mermaid from 'mermaid';

// Initialize mermaid once at module level
let mermaidInitialized = false;
function ensureMermaidInit() {
  if (!mermaidInitialized) {
    mermaid.initialize({
      startOnLoad: false,
      theme: 'dark',
      securityLevel: 'antiscript',
      fontFamily: 'monospace',
    });
    mermaidInitialized = true;
  }
}

// Sequential render queue — avoids freezing with many diagrams
let renderQueue = Promise.resolve();
function queueRender(fn: () => Promise<void>): void {
  renderQueue = renderQueue.then(() => new Promise<void>(resolve => {
    setTimeout(async () => {
      await fn();
      resolve();
    }, 30); // 30ms gap between renders
  }));
}

function MermaidComponent({ content, nodeKey }: { content: string; nodeKey: NodeKey }) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [state, setState] = useState<'loading' | 'error' | 'done'>('loading');
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let mounted = true;
    ensureMermaidInit();

    queueRender(async () => {
      if (!mounted) return;
      if (!containerRef.current) return;

      try {
        const id = `mermaid_${nodeKey}`;
        const { svg } = await mermaid.render(id, content);
        if (!mounted || !containerRef.current) return;
        containerRef.current.innerHTML = svg;
        setState('done');
        setError(null);
      } catch (err: any) {
        if (mounted) {
          setState('error');
          setError(err?.message || String(err));
        }
      }
    });

    return () => { mounted = false; };
  }, [content, nodeKey]);

  if (state === 'error') {
    return (
      <div className="mermaid-block" style={{ margin: '12px 0', padding: 16, border: '1px solid var(--nim-error)', borderRadius: 8, background: 'color-mix(in srgb, var(--nim-error) 8%, transparent)' }}>
        <pre style={{ margin: 0, fontSize: 12, color: 'var(--nim-error)', whiteSpace: 'pre-wrap' }}>{content}</pre>
        <div style={{ fontSize: 11, color: 'var(--nim-text-faint)', marginTop: 8 }}>Mermaid error: {error}</div>
      </div>
    );
  }

  return (
    <div className="mermaid-block" style={{ margin: '12px 0', textAlign: 'center' }}>
      <div ref={containerRef} className="mermaid-render-container" style={{ overflowX: 'auto', minHeight: state === 'loading' ? 60 : 'auto' }} />
    </div>
  );
}
