/*
 * Pre-paint theme + density initialization.
 *
 * Loaded synchronously in <head> (no `defer`) so data-* attributes are set
 * before the first paint — avoids flash-of-wrong-theme.
 *
 * CSP-compatible (no inline code, no eval).
 */
(function () {
  "use strict";

  var root = document.documentElement;

  // Theme: explicit user preference > system preference > light.
  var storedTheme = null;
  try { storedTheme = localStorage.getItem("theme"); } catch (_e) { /* private mode */ }
  var prefersDark = window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches;
  var theme = storedTheme || (prefersDark ? "dark" : "light");
  root.setAttribute("data-theme", theme);

  // Density: explicit user preference > auto (coarse pointer OR narrow viewport
  // OR prefers-reduced-motion → comfortable; else compact).
  var storedDensity = null;
  try { storedDensity = localStorage.getItem("density"); } catch (_e) { /* private mode */ }
  var coarse = window.matchMedia && window.matchMedia("(pointer: coarse)").matches;
  var narrow = window.matchMedia && window.matchMedia("(max-width: 600px)").matches;
  var reduce = window.matchMedia && window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  var autoDensity = (coarse || narrow || reduce) ? "comfortable" : "compact";
  var density = storedDensity || autoDensity;
  root.setAttribute("data-density", density);
})();
