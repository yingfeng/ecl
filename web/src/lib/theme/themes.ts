/**
 * Nimbalyst theme definitions for llmwiki.
 * Each theme maps --nim-* CSS variable names to their values.
 * Applied as inline styles on document.documentElement at runtime.
 */

/** Dark theme — ported from Nimbalyst registry.ts darkThemeColors */
export const DARK: Record<string, string> = {
  '--nim-bg': '#2d2d2d',
  '--nim-bg-secondary': '#1a1a1a',
  '--nim-bg-tertiary': '#3a3a3a',
  '--nim-bg-hover': 'rgba(255, 255, 255, 0.05)',
  '--nim-bg-selected': 'rgba(96, 165, 250, 0.15)',
  '--nim-bg-active': '#4a4a4a',
  '--nim-text': '#ffffff',
  '--nim-text-muted': '#b3b3b3',
  '--nim-text-faint': '#808080',
  '--nim-text-disabled': '#666666',
  '--nim-border': '#4a4a4a',
  '--nim-border-focus': '#60a5fa',
  '--nim-primary': '#60a5fa',
  '--nim-primary-hover': '#3b82f6',
  '--nim-link': '#60a5fa',
  '--nim-link-hover': '#93c5fd',
  '--nim-error': '#ef4444',
  '--nim-success': '#4ade80',
  '--nim-warning': '#fbbf24',
  '--nim-info': '#60a5fa',
  '--nim-purple': '#a78bfa',
  '--nim-code-bg': '#1e1e1e',
  '--nim-code-text': '#d4d4d4',
  '--nim-code-border': '#4a4a4a',
  '--nim-code-gutter': '#2a2a2a',
  '--nim-table-border': '#4a4a4a',
  '--nim-table-header': '#3a3a3a',
  '--nim-table-cell': '#2d2d2d',
  '--nim-table-stripe': '#363636',
  '--nim-toolbar-bg': '#2d2d2d',
  '--nim-toolbar-border': '#4a4a4a',
  '--nim-toolbar-hover': '#3a3a3a',
  '--nim-toolbar-active': 'rgba(96, 165, 250, 0.2)',
  '--nim-highlight-bg': 'rgba(255, 212, 0, 0.2)',
  '--nim-highlight-border': 'rgba(255, 212, 0, 0.4)',
  '--nim-quote-text': '#b3b3b3',
  '--nim-quote-border': '#4a4a4a',
  '--nim-scrollbar-thumb': '#4a4a4a',
  '--nim-scrollbar-thumb-hover': '#5a5a5a',
  '--nim-scrollbar-track': 'transparent',
  '--nim-diff-add-bg': 'rgba(40, 167, 69, 0.15)',
  '--nim-diff-add-border': 'rgba(40, 167, 69, 0.4)',
  '--nim-diff-remove-bg': 'rgba(220, 53, 69, 0.15)',
  '--nim-diff-remove-border': 'rgba(220, 53, 69, 0.4)',
  '--nim-code-comment': '#6a9955',
  '--nim-code-punctuation': '#cccccc',
  '--nim-code-property': '#9cdcfe',
  '--nim-code-selector': '#d7ba7d',
  '--nim-code-operator': '#d4d4d4',
  '--nim-code-attr': '#92c5f8',
  '--nim-code-variable': '#4fc1ff',
  '--nim-code-function': '#dcdcaa',
  '--max-text-width': '800px',
  '--nim-accent-subtle': 'rgba(96, 165, 250, 0.1)',
};

/** Light theme — ported from Nimbalyst NimbalystTheme.css defaults (getBaseThemeColors(false)) */
export const LIGHT: Record<string, string> = {
  '--nim-bg': '#ffffff',
  '--nim-bg-secondary': '#f9fafb',
  '--nim-bg-tertiary': '#f3f4f6',
  '--nim-bg-hover': 'rgba(0, 0, 0, 0.05)',
  '--nim-bg-selected': 'rgba(59, 130, 246, 0.1)',
  '--nim-bg-active': 'rgba(59, 130, 246, 0.2)',
  '--nim-text': '#111827',
  '--nim-text-muted': '#6b7280',
  '--nim-text-faint': '#9ca3af',
  '--nim-text-disabled': '#d1d5db',
  '--nim-border': '#e5e7eb',
  '--nim-border-focus': '#3b82f6',
  '--nim-primary': '#3b82f6',
  '--nim-primary-hover': '#2563eb',
  '--nim-link': '#216fd9',
  '--nim-link-hover': '#216fd9',
  '--nim-error': '#ef4444',
  '--nim-success': '#10b981',
  '--nim-warning': '#f59e0b',
  '--nim-info': '#3b82f6',
  '--nim-purple': '#7c3aed',
  '--nim-code-bg': '#f0f2f5',
  '--nim-code-text': '#111827',
  '--nim-code-border': '#ccc',
  '--nim-code-gutter': '#eee',
  '--nim-table-border': '#bbb',
  '--nim-table-header': '#f2f3f5',
  '--nim-table-cell': '#ffffff',
  '--nim-table-stripe': '#f2f5fb',
  '--nim-toolbar-bg': '#ffffff',
  '--nim-toolbar-border': '#e5e7eb',
  '--nim-toolbar-hover': '#f3f4f6',
  '--nim-toolbar-active': 'rgba(59, 130, 246, 0.2)',
  '--nim-highlight-bg': 'rgba(255, 212, 0, 0.14)',
  '--nim-highlight-border': 'rgba(255, 212, 0, 0.3)',
  '--nim-quote-text': '#65676b',
  '--nim-quote-border': '#ced0d4',
  '--nim-scrollbar-thumb': '#d1d5db',
  '--nim-scrollbar-thumb-hover': '#9ca3af',
  '--nim-scrollbar-track': 'transparent',
  '--nim-diff-add-bg': '#e6ffed',
  '--nim-diff-add-border': '#e6ffed',
  '--nim-diff-remove-bg': '#ffebe9',
  '--nim-diff-remove-border': '#ffebe9',
  '--nim-code-comment': 'slategray',
  '--nim-code-punctuation': '#999',
  '--nim-code-property': '#905',
  '--nim-code-selector': '#690',
  '--nim-code-operator': '#9a6e3a',
  '--nim-code-attr': '#07a',
  '--nim-code-variable': '#e90',
  '--nim-code-function': '#dd4a68',
  '--max-text-width': '800px',
  '--nim-accent-subtle': 'rgba(59, 130, 246, 0.1)',
};

/** Map of theme name to theme variables */
export const THEMES: Record<string, Record<string, string>> = {
  dark: DARK,
  light: LIGHT,
};

export type ThemeName = keyof typeof THEMES;

/** Apply a theme's variables as inline styles on document.documentElement */
export function applyTheme(theme: Record<string, string>): void {
  const root = document.documentElement;
  for (const [key, value] of Object.entries(theme)) {
    root.style.setProperty(key, value);
  }
}

const STORAGE_KEY = 'llmwiki-theme';

export function getSavedTheme(): ThemeName {
  try {
    const saved = localStorage.getItem(STORAGE_KEY);
    if (saved && saved in THEMES) return saved as ThemeName;
  } catch {}
  return 'dark';
}

export function saveTheme(name: ThemeName): void {
  try {
    localStorage.setItem(STORAGE_KEY, name);
  } catch {}
}
