/**
 * Density initialization - runs before page renders to prevent layout shift.
 * This script should be loaded early in the document head.
 *
 * Density modes:
 * - "auto": Automatically selects based on device (touch -> comfortable, desktop -> compact)
 * - "comfortable": Spacious layout, better for touch devices and accessibility
 * - "compact": Space-efficient layout for desktop users
 */

export type DensityMode = "auto" | "comfortable" | "compact";
export type ActualDensity = "comfortable" | "compact";

const STORAGE_KEY = "densityMode";

/**
 * Determines the actual density to use based on stored preference and device characteristics.
 */
function determineActualDensity(storedMode: DensityMode | null): ActualDensity {
  // If explicitly set, use that
  if (storedMode === "comfortable") return "comfortable";
  if (storedMode === "compact") return "compact";

  // Auto mode: detect device characteristics
  const isTouch = window.matchMedia("(pointer: coarse)").matches;
  const prefersMoreContrast = window.matchMedia(
    "(prefers-contrast: more)"
  ).matches;

  // Touch devices and high contrast preference get comfortable mode
  if (isTouch || prefersMoreContrast) {
    return "comfortable";
  }

  // Desktop defaults to compact
  return "compact";
}

/**
 * Applies the density to the document.
 */
function applyDensity(density: ActualDensity): void {
  document.documentElement.dataset["density"] = density;
}

/**
 * Initialize density immediately to prevent layout shift.
 */
function initDensity(): void {
  const storedMode = localStorage.getItem(STORAGE_KEY) as DensityMode | null;
  const actualDensity = determineActualDensity(storedMode);
  applyDensity(actualDensity);
}

// Run immediately (not in DOMContentLoaded)
initDensity();

// Listen for pointer type changes (e.g., docking/undocking tablet)
window.matchMedia("(pointer: coarse)").addEventListener("change", () => {
  const storedMode = localStorage.getItem(STORAGE_KEY) as DensityMode | null;
  if (!storedMode || storedMode === "auto") {
    const actualDensity = determineActualDensity(storedMode);
    applyDensity(actualDensity);
  }
});
