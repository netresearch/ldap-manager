/**
 * Density initialization - runs before page renders to prevent layout shift.
 * This script should be loaded early in the document head.
 *
 * Density modes:
 * - "auto": Automatically selects based on device (touch -> comfortable, desktop -> compact)
 * - "comfortable": Spacious layout, better for touch devices and accessibility
 * - "compact": Space-efficient layout for desktop users
 */

(function () {
  type DensityMode = "auto" | "comfortable" | "compact";
  type ActualDensity = "comfortable" | "compact";

  const STORAGE_KEY = "densityMode";

  function determineActualDensity(storedMode: DensityMode | null): ActualDensity {
    if (storedMode === "comfortable") return "comfortable";
    if (storedMode === "compact") return "compact";

    const isTouch = window.matchMedia("(pointer: coarse)").matches;
    const prefersMoreContrast = window.matchMedia("(prefers-contrast: more)").matches;

    return isTouch || prefersMoreContrast ? "comfortable" : "compact";
  }

  function applyDensity(density: ActualDensity): void {
    document.documentElement.dataset["density"] = density;
  }

  function initDensity(): void {
    const storedMode = localStorage.getItem(STORAGE_KEY) as DensityMode | null;
    const actualDensity = determineActualDensity(storedMode);
    applyDensity(actualDensity);
  }

  // Run immediately
  initDensity();

  // Listen for pointer type changes
  window.matchMedia("(pointer: coarse)").addEventListener("change", () => {
    const storedMode = localStorage.getItem(STORAGE_KEY) as DensityMode | null;
    if (!storedMode || storedMode === "auto") {
      const actualDensity = determineActualDensity(storedMode);
      applyDensity(actualDensity);
    }
  });
})();
