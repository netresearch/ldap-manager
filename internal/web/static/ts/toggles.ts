/**
 * Toggle functionality for theme and density buttons.
 * These functions should be called after DOMContentLoaded.
 */

type ThemeMode = "auto" | "light" | "dark";
type DensityMode = "auto" | "comfortable" | "compact";
type ActualDensity = "comfortable" | "compact";

const THEME_STORAGE_KEY = "theme";
const DENSITY_STORAGE_KEY = "densityMode";

/**
 * Theme toggle: cycles through auto -> light -> dark -> auto
 */
export function initThemeToggle(): void {
  const button = document.getElementById("themeToggle");
  if (!button) return;

  // Set initial state from storage
  const currentMode =
    (localStorage.getItem(THEME_STORAGE_KEY) as ThemeMode) || "auto";
  updateThemeButtonState(button, currentMode);

  button.addEventListener("click", () => {
    const current =
      (localStorage.getItem(THEME_STORAGE_KEY) as ThemeMode) || "auto";
    const next = getNextThemeMode(current);

    localStorage.setItem(THEME_STORAGE_KEY, next);
    updateThemeButtonState(button, next);
    applyTheme(next);
  });
}

function getNextThemeMode(current: ThemeMode): ThemeMode {
  switch (current) {
    case "auto":
      return "light";
    case "light":
      return "dark";
    case "dark":
      return "auto";
    default:
      return "auto";
  }
}

function updateThemeButtonState(button: HTMLElement, mode: ThemeMode): void {
  button.dataset["theme"] = mode;

  // Update icon visibility
  const autoIcon = button.querySelector("#theme-auto");
  const lightIcon = button.querySelector("#theme-light");
  const darkIcon = button.querySelector("#theme-dark");

  autoIcon?.classList.toggle("hidden", mode !== "auto");
  lightIcon?.classList.toggle("hidden", mode !== "light");
  darkIcon?.classList.toggle("hidden", mode !== "dark");

  // Update aria-label
  const labels: Record<ThemeMode, string> = {
    auto: "Theme: Auto (click to switch to light)",
    light: "Theme: Light (click to switch to dark)",
    dark: "Theme: Dark (click to switch to auto)",
  };
  button.setAttribute("aria-label", labels[mode]);
}

function applyTheme(mode: ThemeMode): void {
  const isDark =
    mode === "dark" ||
    (mode === "auto" &&
      window.matchMedia("(prefers-color-scheme: dark)").matches);

  if (isDark) {
    document.documentElement.classList.add("dark");
  } else {
    document.documentElement.classList.remove("dark");
  }
}

/**
 * Density toggle: cycles through auto -> comfortable -> compact -> auto
 */
export function initDensityToggle(): void {
  const button = document.getElementById("densityToggle");
  if (!button) return;

  // Set initial state from storage
  const currentMode =
    (localStorage.getItem(DENSITY_STORAGE_KEY) as DensityMode) || "auto";
  updateDensityButtonState(button, currentMode);

  button.addEventListener("click", () => {
    const current =
      (localStorage.getItem(DENSITY_STORAGE_KEY) as DensityMode) || "auto";
    const next = getNextDensityMode(current);

    localStorage.setItem(DENSITY_STORAGE_KEY, next);
    updateDensityButtonState(button, next);
    applyDensity(next);
  });
}

function getNextDensityMode(current: DensityMode): DensityMode {
  switch (current) {
    case "auto":
      return "comfortable";
    case "comfortable":
      return "compact";
    case "compact":
      return "auto";
    default:
      return "auto";
  }
}

function updateDensityButtonState(
  button: HTMLElement,
  mode: DensityMode
): void {
  button.dataset["densityMode"] = mode;

  // Update icon visibility
  const autoIcon = button.querySelector("#density-auto");
  const comfortableIcon = button.querySelector("#density-comfortable");
  const compactIcon = button.querySelector("#density-compact");

  autoIcon?.classList.toggle("hidden", mode !== "auto");
  comfortableIcon?.classList.toggle("hidden", mode !== "comfortable");
  compactIcon?.classList.toggle("hidden", mode !== "compact");

  // Update aria-label
  const labels: Record<DensityMode, string> = {
    auto: "Density: Auto (click to switch to comfortable)",
    comfortable: "Density: Comfortable (click to switch to compact)",
    compact: "Density: Compact (click to switch to auto)",
  };
  button.setAttribute("aria-label", labels[mode]);
}

function determineActualDensity(mode: DensityMode): ActualDensity {
  if (mode === "comfortable") return "comfortable";
  if (mode === "compact") return "compact";

  // Auto mode
  const isTouch = window.matchMedia("(pointer: coarse)").matches;
  const prefersMoreContrast = window.matchMedia(
    "(prefers-contrast: more)"
  ).matches;

  return isTouch || prefersMoreContrast ? "comfortable" : "compact";
}

function applyDensity(mode: DensityMode): void {
  const actual = determineActualDensity(mode);
  document.documentElement.dataset["density"] = actual;
}
