/**
 * Theme initialization - runs before page renders to prevent flash of wrong theme.
 * This script should be loaded early in the document head.
 *
 * Theme modes:
 * - "auto": Follows system preference (prefers-color-scheme)
 * - "light": Forces light theme
 * - "dark": Forces dark theme
 */
const STORAGE_KEY = "theme";
/**
 * Determines if dark mode should be active based on stored preference and system settings.
 */
function shouldUseDarkMode(storedTheme) {
    if (storedTheme === "dark")
        return true;
    if (storedTheme === "light")
        return false;
    // Auto mode: follow system preference
    return window.matchMedia("(prefers-color-scheme: dark)").matches;
}
/**
 * Applies the theme to the document.
 */
function applyTheme(isDark) {
    if (isDark) {
        document.documentElement.classList.add("dark");
    }
    else {
        document.documentElement.classList.remove("dark");
    }
}
/**
 * Initialize theme immediately to prevent flash of wrong theme.
 */
function initTheme() {
    const storedTheme = localStorage.getItem(STORAGE_KEY);
    const isDark = shouldUseDarkMode(storedTheme);
    applyTheme(isDark);
}
// Run immediately (not in DOMContentLoaded)
initTheme();
// Listen for system preference changes when in auto mode
window
    .matchMedia("(prefers-color-scheme: dark)")
    .addEventListener("change", (e) => {
    const storedTheme = localStorage.getItem(STORAGE_KEY);
    if (!storedTheme || storedTheme === "auto") {
        applyTheme(e.matches);
    }
});
export {};
