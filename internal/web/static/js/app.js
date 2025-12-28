/**
 * Main application entry point.
 * Initializes all client-side functionality after DOM is ready.
 */
import { initThemeToggle, initDensityToggle } from "./toggles.js";
import { initComboboxes } from "./combobox.js";
import { initSearchFilters } from "./search-filter.js";
import { initCopyButtons } from "./copy-clipboard.js";
/**
 * Initialize all application functionality.
 */
function init() {
    initThemeToggle();
    initDensityToggle();
    initComboboxes();
    initSearchFilters();
    initCopyButtons();
}
// Wait for DOM to be ready
if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
}
else {
    // DOM is already ready
    init();
}
