/**
 * Lexical nodes for the editor.
 * Each node type must be registered in the LexicalComposer config.
 */

import type { Klass, LexicalNode } from 'lexical';
import { CodeHighlightNode, CodeNode } from '@lexical/code';
import { HashtagNode } from '@lexical/hashtag';
import { HeadingNode, QuoteNode } from '@lexical/rich-text';
import { ListItemNode, ListNode } from '@lexical/list';
import { LinkNode, AutoLinkNode } from '@lexical/link';
import { TableNode, TableRowNode, TableCellNode } from '@lexical/table';
import { HorizontalRuleNode } from './markdown/HorizontalRuleNode';
import { MermaidNode } from './MermaidNode';

const EditorNodes: Array<Klass<LexicalNode>> = [
  HeadingNode,
  QuoteNode,
  CodeNode,
  CodeHighlightNode,
  ListItemNode,
  ListNode,
  LinkNode,
  AutoLinkNode,
  HashtagNode,
  HorizontalRuleNode,
  MermaidNode,
  TableNode,
  TableRowNode,
  TableCellNode,
];

export default EditorNodes;
