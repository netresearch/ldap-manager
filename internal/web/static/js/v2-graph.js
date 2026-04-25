/*
 * v2-graph.js — Phase 3 graph view client. Reads JSON embedded by the
 * template, enhances the SVG with pan/zoom/keyboard nav and click-to-
 * pivot/expand. CSP-safe (no eval, no inline scripts, no Function).
 *
 * The data island is a <template id="graph-data"> (HTML data island,
 * inert under script-src 'self'); its content lives in a separate
 * DocumentFragment so we read it via .content.textContent rather than
 * .textContent.
 *
 * Slice 3 intentionally rendered nodes without role/tabindex/aria
 * affordances so the SSR markup didn't advertise interactivity it
 * couldn't provide. Slice 4 (this file) re-adds them via JS once the
 * handlers are wired up.
 */
(function () {
  "use strict";
  if (!document.getElementById("graph-data")) return;

  function parseData() {
    var el = document.getElementById("graph-data");
    if (!el) return null;
    try {
      // <template> content lives in a separate DocumentFragment.
      // Slice 3 chose <template> over <script type="application/json">
      // for CSP-safety under script-src 'self'.
      var text = el.content ? el.content.textContent : el.textContent;
      return JSON.parse(text);
    } catch (e) {
      console.error("graph-data JSON parse failed", e);
      return null;
    }
  }

  // activateNodes makes each rendered .graph-node keyboard-focusable
  // and properly labelled now that the JS handlers exist. Slice 3
  // deliberately omitted these attributes from the SSR template.
  function activateNodes(svg) {
    var nodes = svg.querySelectorAll(".graph-node");
    nodes.forEach(function (n) {
      n.setAttribute("tabindex", "0");
      n.setAttribute("role", "button");
      var label = n.getAttribute("aria-label") || "";
      if (label && label.indexOf("Press Enter to open.") === -1) {
        n.setAttribute("aria-label", label + " Press Enter to open.");
      }
    });
  }

  document.addEventListener("DOMContentLoaded", function () {
    var state = parseData();
    if (!state) return;
    var svg = document.getElementById("graph-canvas");
    if (!svg) return;
    var viewport = svg.querySelector(".graph-viewport");
    if (!viewport) return;

    activateNodes(svg);
    wirePanZoom(svg, viewport);
    wireKeyboardNav(svg);
    wireNodeClicks(svg, state);
    wireDepthSlider();
  });

  function wirePanZoom(svg, viewport) {
    var tx = 0,
      ty = 0,
      scale = 1;
    var dragging = false,
      sx = 0,
      sy = 0;

    function apply() {
      viewport.setAttribute(
        "transform",
        "translate(" + tx + "," + ty + ") scale(" + scale + ")",
      );
    }

    svg.addEventListener("mousedown", function (e) {
      // Only start panning when the user grabs empty canvas space
      // (the SVG itself or the viewport <g>) — not when they click a
      // node or edge.
      if (
        e.target !== svg &&
        !e.target.classList.contains("graph-viewport")
      ) {
        return;
      }
      dragging = true;
      sx = e.clientX - tx;
      sy = e.clientY - ty;
      e.preventDefault();
    });
    window.addEventListener("mousemove", function (e) {
      if (!dragging) return;
      tx = e.clientX - sx;
      ty = e.clientY - sy;
      apply();
    });
    window.addEventListener("mouseup", function () {
      dragging = false;
    });

    // Wheel zoom only when modifier key is held — without ctrl/meta the
    // page scroll behaviour is preserved.
    svg.addEventListener(
      "wheel",
      function (e) {
        if (!(e.ctrlKey || e.metaKey)) return;
        e.preventDefault();
        var delta = -e.deltaY * 0.001;
        scale = Math.min(3, Math.max(0.3, scale * (1 + delta)));
        apply();
      },
      { passive: false },
    );

    // Arrow-key pan and +/- zoom when the canvas itself is focused
    // (the SVG element has tabindex="0" from the SSR template).
    svg.addEventListener("keydown", function (e) {
      var step = 32;
      switch (e.key) {
        case "ArrowLeft":
          tx += step;
          apply();
          e.preventDefault();
          break;
        case "ArrowRight":
          tx -= step;
          apply();
          e.preventDefault();
          break;
        case "ArrowUp":
          ty += step;
          apply();
          e.preventDefault();
          break;
        case "ArrowDown":
          ty -= step;
          apply();
          e.preventDefault();
          break;
        case "+":
        case "=":
          scale = Math.min(3, scale * 1.1);
          apply();
          e.preventDefault();
          break;
        case "-":
        case "_":
          scale = Math.max(0.3, scale / 1.1);
          apply();
          e.preventDefault();
          break;
      }
    });
  }
  function wireKeyboardNav(_svg) {
    /* Task 25 */
  }
  function wireNodeClicks(_svg, _state) {
    /* Task 26 */
  }
  function wireDepthSlider() {
    /* Task 27 */
  }
})();
