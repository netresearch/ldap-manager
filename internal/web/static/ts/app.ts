/**
 * Main application entry point.
 * Initializes all client-side functionality after DOM is ready.
 */

import { initThemeToggle, initDensityToggle } from "./toggles.js";

/**
 * Initialize all application functionality.
 */
function init(): void {
  initThemeToggle();
  initDensityToggle();
}

// Wait for DOM to be ready
if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", init);
} else {
  // DOM is already ready
  init();
}
