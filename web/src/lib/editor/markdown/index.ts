/**
 * Public API for the markdown pipeline.
 */

import { CORE_TRANSFORMERS } from './MarkdownTransformers';

export { $convertFromEnhancedMarkdownString } from './EnhancedMarkdownImport';
export type { EnhancedImportOptions } from './EnhancedMarkdownImport';

export {
  $convertToEnhancedMarkdownString,
  $convertNodeToEnhancedMarkdownString,
} from './EnhancedMarkdownExport';
export type { EnhancedExportOptions } from './EnhancedMarkdownExport';

export {
  CORE_TRANSFORMERS,
  HEADING, QUOTE, CODE,
  UNORDERED_LIST, ORDERED_LIST, CHECK_LIST,
  INLINE_CODE, HIGHLIGHT,
  BOLD_STAR, BOLD_UNDERSCORE,
  BOLD_ITALIC_STAR, BOLD_ITALIC_UNDERSCORE,
  ITALIC_STAR, ITALIC_UNDERSCORE,
  STRIKETHROUGH, LINK,
  ELEMENT_TRANSFORMERS,
  MULTILINE_ELEMENT_TRANSFORMERS,
  TEXT_FORMAT_TRANSFORMERS,
  TEXT_MATCH_TRANSFORMERS,
  setMarkdownConfig,
  getMarkdownConfig,
  setListConfig,
  getListConfig,
  MERMAID_TRANSFORMER,
  TABLE_TRANSFORMER,
} from './MarkdownTransformers';
export type {
  MarkdownConfig,
  ListConfig,
} from './MarkdownTransformers';

// Re-export types from @lexical/markdown
export type {
  ElementTransformer,
  MultilineElementTransformer,
  TextFormatTransformer,
  TextMatchTransformer,
  Transformer,
} from '@lexical/markdown';

export { HorizontalRuleNode, $createHorizontalRuleNode, $isHorizontalRuleNode } from './HorizontalRuleNode';
export type { SerializedHorizontalRuleNode } from './HorizontalRuleNode';

/**
 * Returns the core set of transformers used for markdown import/export.
 * This function is used by plugins (e.g., TableTransformer) that need to
 * perform markdown conversion inside their own logic.
 */
export function getEditorTransformers(): Transformer[] {
  return CORE_TRANSFORMERS as Transformer[];
}
