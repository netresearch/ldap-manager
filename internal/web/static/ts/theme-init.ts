/**
 * Theme initialization - runs before page renders to prevent flash of wrong theme.
 * This script should be loaded early in the document head.
 *
 * Theme modes:
 * - "auto": Follows system preference (prefers-color-scheme)
 * - "light": Forces light theme
 * - "dark": Forces dark theme
 */

(function () {
  type ThemeMode = "auto" | "light" | "dark";

  const STORAGE_KEY = "theme";

  function shouldUseDarkMode(storedTheme: ThemeMode | null): boolean {
    if (storedTheme === "dark") return true;
    if (storedTheme === "light") return false;
    return window.matchMedia("(prefers-color-scheme: dark)").matches;
  }

  function applyTheme(isDark: boolean): void {
    if (isDark) {
      document.documentElement.classList.add("dark");
    } else {
      document.documentElement.classList.remove("dark");
    }
  }

  function initTheme(): void {
    const storedTheme = localStorage.getItem(STORAGE_KEY) as ThemeMode | null;
    const isDark = shouldUseDarkMode(storedTheme);
    applyTheme(isDark);
  }

  // Run immediately
  initTheme();

  // Listen for system preference changes when in auto mode
  window.matchMedia("(prefers-color-scheme: dark)").addEventListener("change", (e) => {
    const storedTheme = localStorage.getItem(STORAGE_KEY) as ThemeMode | null;
    if (!storedTheme || storedTheme === "auto") {
      applyTheme(e.matches);
    }
  });
})();
