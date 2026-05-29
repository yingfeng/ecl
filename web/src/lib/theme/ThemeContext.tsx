/**
 * ThemeContext — provides current theme and toggle function.
 * Theme variables are applied as inline styles on <html>.
 */
import { createContext, useContext, useState, useEffect, type ReactNode } from 'react';
import {
  THEMES,
  applyTheme,
  getSavedTheme,
  saveTheme,
  type ThemeName,
} from './themes';

interface ThemeContextValue {
  theme: ThemeName;
  setTheme: (name: ThemeName) => void;
  toggleTheme: () => void;
}

const ThemeContext = createContext<ThemeContextValue>({
  theme: 'dark',
  setTheme: () => {},
  toggleTheme: () => {},
});

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setThemeState] = useState<ThemeName>(() => {
    const saved = getSavedTheme();
    return saved in THEMES ? saved : 'dark';
  });

  // Apply theme on mount and when changed
  useEffect(() => {
    const vars = THEMES[theme] || THEMES.dark;
    applyTheme(vars);
    saveTheme(theme);
  }, [theme]);

  const setTheme = (name: ThemeName) => {
    if (name in THEMES) setThemeState(name);
  };

  const toggleTheme = () => {
    setThemeState(prev => (prev === 'dark' ? 'light' : 'dark'));
  };

  return (
    <ThemeContext.Provider value={{ theme, setTheme, toggleTheme }}>
      {children}
    </ThemeContext.Provider>
  );
}

export function useTheme(): ThemeContextValue {
  return useContext(ThemeContext);
}
