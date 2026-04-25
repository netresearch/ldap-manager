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
  function wireKeyboardNav(svg) {
    // Snapshot the node list at wire time. Nodes added later by
    // expandNode (Task 26) won't join this cycle until the user
    // navigates back to the graph; that's an acceptable tradeoff to
    // keep this handler stateless.
    var nodes = Array.prototype.slice.call(
      svg.querySelectorAll(".graph-node"),
    );
    var index = 0;
    function focusAt(i) {
      if (nodes.length === 0) return;
      index = ((i % nodes.length) + nodes.length) % nodes.length;
      nodes[index].focus();
    }
    svg.addEventListener("keydown", function (e) {
      if (!e.target.classList.contains("graph-node")) return;
      if (e.key === "Tab" && !e.shiftKey) {
        focusAt(index + 1);
        e.preventDefault();
      } else if (e.key === "Tab" && e.shiftKey) {
        focusAt(index - 1);
        e.preventDefault();
      }
    });
  }
  function wireNodeClicks(svg, state) {
    svg.addEventListener("click", function (e) {
      var node = e.target.closest(".graph-node");
      if (!node) return;
      var dn = node.getAttribute("data-dn");
      var type = node.getAttribute("data-type");
      var expandable = node.getAttribute("data-expandable") === "true";
      var clickedBadge = !!e.target.closest(".graph-node__expand-badge");

      if (expandable && clickedBadge) {
        expandNode(dn, state, svg);
      } else {
        pivotToDrawer(dn, type);
      }
    });
    svg.addEventListener("keydown", function (e) {
      if (e.key !== "Enter" && e.key !== " ") return;
      var node = e.target.closest(".graph-node");
      if (!node) return;
      var dn = node.getAttribute("data-dn");
      var type = node.getAttribute("data-type");
      var expandable = node.getAttribute("data-expandable") === "true";
      e.preventDefault();
      if (expandable) expandNode(dn, state, svg);
      else pivotToDrawer(dn, type);
    });
  }

  function pivotToDrawer(dn, type) {
    var base = {
      user: "/users/",
      group: "/groups/",
      computer: "/computers/",
      ou: "/users?ou=",
    }[type];
    if (!base) return;
    window.location.href = base + encodeURIComponent(dn);
  }

  // announce updates the off-screen aria-live region so SR users hear
  // expansion results. Clear-then-set forces re-announcement when the
  // same message fires twice.
  function announce(msg) {
    var el = document.getElementById("graph-announce");
    if (el) {
      el.textContent = "";
      setTimeout(function () {
        el.textContent = msg;
      }, 10);
    }
  }

  function expandNode(dn, state, svg) {
    var url =
      "/api/graph.json?entity=" + encodeURIComponent(dn) + "&depth=1";
    fetch(url, { credentials: "same-origin" })
      .then(function (r) {
        return r.json();
      })
      .then(function (data) {
        var added = 0;
        var existingDNs = {};
        state.nodes.forEach(function (n) {
          existingDNs[n.dn] = true;
        });
        data.nodes.forEach(function (n) {
          if (existingDNs[n.dn]) return;
          var parent = state.nodes.find(function (x) {
            return x.dn === dn;
          });
          n.ring = (parent && parent.ring + 1) || 2;
          state.nodes.push(n);
          renderNode(svg, n);
          added++;
        });
        data.edges.forEach(function (e) {
          var dup = state.edges.some(function (x) {
            return (
              x.source === e.source &&
              x.target === e.target &&
              x.kind === e.kind
            );
          });
          if (!dup) {
            state.edges.push(e);
            renderEdge(svg, e, state.nodes);
          }
        });
        // Mark clicked node as non-expandable so a subsequent click
        // pivots to the drawer instead of re-fetching.
        var el = svg.querySelector(
          '.graph-node[data-dn="' + CSS.escape(dn) + '"]',
        );
        if (el) {
          el.setAttribute("data-expandable", "false");
          var badge = el.querySelector(".graph-node__expand-badge");
          if (badge) badge.remove();
        }
        announce("Expanded " + dn + ": added " + added + " nodes.");
      });
  }

  function renderNode(svg, n) {
    var ns = "http://www.w3.org/2000/svg";
    var viewport = svg.querySelector(".graph-viewport");
    // Match the SSR concentricXY radius multiplier (graph_v2.templ
    // uses 150 so ring-3 nodes — including the 28-radius disc and
    // expand-badge — fit inside the ±500 viewBox).
    var r = n.ring * 150;
    var x = r * Math.cos(n.angle),
      y = r * Math.sin(n.angle);
    var g = document.createElementNS(ns, "g");
    g.setAttribute(
      "class",
      "graph-node graph-node--" + n.type + " graph-node--added",
    );
    g.setAttribute("transform", "translate(" + x + "," + y + ")");
    g.setAttribute("tabindex", "0");
    g.setAttribute("role", "button");
    g.setAttribute("data-dn", n.dn);
    g.setAttribute("data-type", n.type);
    g.setAttribute("data-expandable", String(!!n.expandable));
    // Match activateNodes() so newly inserted nodes get the same
    // accessible affordance the SSR-rendered ones receive.
    g.setAttribute(
      "aria-label",
      (n.label || n.dn) + ". Press Enter to open.",
    );
    var circ = document.createElementNS(ns, "circle");
    circ.setAttribute("r", "28");
    circ.setAttribute("class", "graph-node__disc");
    g.appendChild(circ);
    var text = document.createElementNS(ns, "text");
    text.setAttribute("text-anchor", "middle");
    text.setAttribute("y", "4");
    text.setAttribute("class", "graph-node__label");
    text.textContent = n.label;
    g.appendChild(text);
    viewport.appendChild(g);
  }

  function renderEdge(svg, e, nodes) {
    var ns = "http://www.w3.org/2000/svg";
    var viewport = svg.querySelector(".graph-viewport");
    function xy(dn) {
      var n = nodes.find(function (x) {
        return x.dn === dn;
      });
      if (!n) return [0, 0];
      var r = n.ring * 150;
      return [r * Math.cos(n.angle), r * Math.sin(n.angle)];
    }
    var s = xy(e.source),
      t = xy(e.target);
    var line = document.createElementNS(ns, "line");
    line.setAttribute("class", "graph-edge graph-edge--" + e.kind);
    line.setAttribute("x1", s[0]);
    line.setAttribute("y1", s[1]);
    line.setAttribute("x2", t[0]);
    line.setAttribute("y2", t[1]);
    line.setAttribute("data-source", e.source);
    line.setAttribute("data-target", e.target);
    // Insert at the front so the edge sits behind nodes in z-order.
    viewport.insertBefore(line, viewport.firstChild);
  }
  function wireDepthSlider() {
    var slider = document.querySelector("[data-graph-slider]");
    if (!slider) return;
    var out = document.querySelector(".graph-slider__value");
    // 'input' fires per-step while the user drags; 'change' fires on
    // release. Use 'input' for the live value display and 'change' to
    // submit the form, so a drag from 1→3 produces one navigation,
    // not three.
    slider.addEventListener("input", function () {
      if (out) out.textContent = slider.value;
    });
    slider.addEventListener("change", function () {
      var form = slider.form;
      if (form) form.submit();
    });
  }
})();
