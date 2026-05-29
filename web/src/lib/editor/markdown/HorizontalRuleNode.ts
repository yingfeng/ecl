/**
 * Custom HorizontalRuleNode for Lexical.
 * Uses ElementNode (no decorative rendering needed).
 */

import {
  $applyNodeReplacement,
  type EditorConfig,
  type LexicalNode,
  type SerializedElementNode,
  type SerializedLexicalNode,
  type ElementFormatType,
  type Spread,
  ElementNode,
} from 'lexical';

export type SerializedHorizontalRuleNode = Spread<
  {
    children: [];
    direction: 'ltr' | 'rtl' | null;
    format: ElementFormatType;
    indent: number;
  },
  SerializedLexicalNode
>;

export class HorizontalRuleNode extends ElementNode {
  static getType(): string {
    return 'horizontalrule';
  }

  static clone(node: HorizontalRuleNode): HorizontalRuleNode {
    return new HorizontalRuleNode(node.__key);
  }

  static importJSON(_serializedNode: SerializedHorizontalRuleNode): HorizontalRuleNode {
    return $createHorizontalRuleNode();
  }

  exportJSON(): SerializedHorizontalRuleNode {
    return {
      children: [],
      direction: 'ltr',
      format: '',
      indent: 0,
      type: 'horizontalrule',
      version: 1,
    };
  }

  createDOM(_config: EditorConfig, _editor: any): HTMLElement {
    const element = document.createElement('hr');
    return element;
  }

  updateDOM(): boolean {
    return false;
  }

  getTextContent(): string {
    return '\n';
  }

  isInline(): boolean {
    return false;
  }

  canBeEmpty(): boolean {
    return true;
  }
}

export function $createHorizontalRuleNode(): HorizontalRuleNode {
  return $applyNodeReplacement(new HorizontalRuleNode());
}

export function $isHorizontalRuleNode(node: LexicalNode | null | undefined): node is HorizontalRuleNode {
  return node instanceof HorizontalRuleNode;
}
