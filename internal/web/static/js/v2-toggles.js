/*
 * Theme + density toggle wiring.
 *
 * Buttons declare their action via `data-toggle="theme"` or
 * `data-toggle="density"` — no inline handlers (CSP-safe).
 */
(function () {
  "use strict";

  var root = document.documentElement;

  function cycleTheme() {
    var next = root.getAttribute("data-theme") === "dark" ? "light" : "dark";
    root.setAttribute("data-theme", next);
    try { localStorage.setItem("theme", next); } catch (_e) {}
  }

  function cycleDensity() {
    var next = root.getAttribute("data-density") === "comfortable" ? "compact" : "comfortable";
    root.setAttribute("data-density", next);
    try { localStorage.setItem("density", next); } catch (_e) {}
  }

  document.addEventListener("click", function (ev) {
    var btn = ev.target instanceof Element ? ev.target.closest("[data-toggle]") : null;
    if (!btn) return;
    var kind = btn.getAttribute("data-toggle");
    if (kind === "theme") cycleTheme();
    else if (kind === "density") cycleDensity();
  });
})();
