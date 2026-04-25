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

  // nodeLabel mirrors graphNodeLabel() in graph_v2.templ so screen-reader
  // labels for nodes added via expandNode() carry the same type/state
  // context as SSR-rendered nodes.
  function nodeLabel(n) {
    switch (n.type) {
      case "group":
        var count = typeof n.memberCount === "number" ? n.memberCount : 0;
        return "Group " + n.label + " (" + count + " members).";
      case "user":
        var state = n.enabled === false ? "disabled" : "enabled";
        return "User " + n.label + " (" + state + ").";
      case "computer":
        return "Computer " + n.label + ".";
      case "ou":
        return "Organisational unit " + n.label + ".";
    }
    return n.label;
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
        label = label + " Press Enter to open.";
      }
      if (
        n.getAttribute("data-expandable") === "true" &&
        label.indexOf("Press + to expand") === -1
      ) {
        label = label + " Press + to expand more relations.";
      }
      n.setAttribute("aria-label", label);
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
      // Allow pan-start anywhere except on a node — clicks on edges or
      // empty canvas both pan. Dense graphs leave little empty space, so
      // restricting pan to the SVG/viewport background made it hard to
      // grab.
      if (e.target.closest(".graph-node")) return;
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
    // page scroll behaviour is preserved. Zoom around the cursor so the
    // world-space point under the pointer stays put after the scale
    // change (standard map/graph UX).
    svg.addEventListener(
      "wheel",
      function (e) {
        if (!(e.ctrlKey || e.metaKey)) return;
        e.preventDefault();
        var rect = svg.getBoundingClientRect();
        var cx = e.clientX - rect.left;
        var cy = e.clientY - rect.top;
        var oldScale = scale;
        var delta = -e.deltaY * 0.001;
        scale = Math.min(3, Math.max(0.3, scale * (1 + delta)));
        if (oldScale !== scale) {
          tx = cx - ((cx - tx) * scale) / oldScale;
          ty = cy - ((cy - ty) * scale) / oldScale;
        }
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
    // Keyboard parity with mouse: Enter pivots (matches the SSR
    // aria-label "Press Enter to open"), and +/= expands when the node
    // is expandable. Space is intentionally not bound — convention on
    // non-form elements is page-down, not activation.
    svg.addEventListener("keydown", function (e) {
      var node = e.target.closest(".graph-node");
      if (!node) return;
      var dn = node.getAttribute("data-dn");
      var type = node.getAttribute("data-type");
      var expandable = node.getAttribute("data-expandable") === "true";

      if (e.key === "Enter") {
        e.preventDefault();
        pivotToDrawer(dn, type);
      } else if (expandable && (e.key === "+" || e.key === "=")) {
        e.preventDefault();
        expandNode(dn, state, svg);
      }
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
        // Surface non-2xx (401 redirect to login, 404 missing entity,
        // 500 server error) as thrown errors so the .catch below can
        // announce them — otherwise r.json() would parse the HTML
        // error body and throw a confusing SyntaxError instead.
        if (!r.ok) throw new Error("graph expand HTTP " + r.status);
        return r.json();
      })
      .then(function (data) {
        var added = 0;
        var existingDNs = {};
        var affectedRings = {};
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
          affectedRings[n.ring] = true;
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
        // Re-distribute angles on each ring that gained a node. Without
        // this, expanded nodes keep the angles the backend assigned in
        // the depth=1 ego graph (centred on the clicked node) — which
        // were computed for a different node set and overlap or cluster
        // oddly when merged into the original layout.
        Object.keys(affectedRings).forEach(function (r) {
          reLayoutRing(svg, state, parseInt(r, 10));
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
      })
      .catch(function (err) {
        // Without this the badge stays clickable forever and SR users
        // hear nothing. Trace + announce so the operator can retry.
        console.error("graph expand failed", err);
        announce("Expand failed.");
      });
  }

  // reLayoutRing redistributes nodes on a ring evenly around the circle
  // and re-routes any edges touching them. Called after expandNode
  // merges new nodes onto an existing ring.
  function reLayoutRing(svg, state, ring) {
    var ringNodes = state.nodes.filter(function (n) {
      return n.ring === ring;
    });
    ringNodes.sort(function (a, b) {
      if (a.type !== b.type) return a.type < b.type ? -1 : 1;
      if (a.label !== b.label) return a.label < b.label ? -1 : 1;
      return a.dn < b.dn ? -1 : 1;
    });
    var count = ringNodes.length;
    if (count === 0) return;
    ringNodes.forEach(function (n, i) {
      n.angle = (i * 2 * Math.PI) / count;
      var r = ring * 150;
      var x = r * Math.cos(n.angle);
      var y = r * Math.sin(n.angle);
      var el = svg.querySelector(
        '.graph-node[data-dn="' + CSS.escape(n.dn) + '"]',
      );
      if (el) el.setAttribute("transform", "translate(" + x + "," + y + ")");
    });
    // Re-route edges touching any moved node on this ring.
    state.edges.forEach(function (e) {
      var src = state.nodes.find(function (x) {
        return x.dn === e.source;
      });
      var tgt = state.nodes.find(function (x) {
        return x.dn === e.target;
      });
      if (!src || !tgt) return;
      if (src.ring !== ring && tgt.ring !== ring) return;
      var line = svg.querySelector(
        'line[data-source="' +
          CSS.escape(e.source) +
          '"][data-target="' +
          CSS.escape(e.target) +
          '"]',
      );
      if (!line) return;
      var sx = src.ring * 150 * Math.cos(src.angle);
      var sy = src.ring * 150 * Math.sin(src.angle);
      var tx = tgt.ring * 150 * Math.cos(tgt.angle);
      var ty = tgt.ring * 150 * Math.sin(tgt.angle);
      line.setAttribute("x1", sx);
      line.setAttribute("y1", sy);
      line.setAttribute("x2", tx);
      line.setAttribute("y2", ty);
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
    // Mirror activateNodes(): build the same "<typed label>. Press
    // Enter to open. [Press + to expand more relations.]" hint stack
    // so newly inserted nodes carry full screen-reader context.
    var base = nodeLabel(n);
    var hint = " Press Enter to open.";
    if (n.expandable) hint += " Press + to expand more relations.";
    g.setAttribute("aria-label", base + hint);
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
    // Mirror the SSR template's expand-badge (graph_v2.templ graphNode):
    // without it, freshly-fetched expandable children can be pivoted but
    // never further expanded by mouse — wireNodeClicks requires a click
    // on .graph-node__expand-badge to trigger expansion.
    if (n.expandable) {
      var badge = document.createElementNS(ns, "g");
      badge.setAttribute("class", "graph-node__expand-badge");
      badge.setAttribute("transform", "translate(18,-18)");
      badge.setAttribute("aria-hidden", "true");
      var bbg = document.createElementNS(ns, "circle");
      bbg.setAttribute("r", "8");
      bbg.setAttribute("class", "graph-node__expand-badge-bg");
      badge.appendChild(bbg);
      var bmark = document.createElementNS(ns, "text");
      bmark.setAttribute("text-anchor", "middle");
      bmark.setAttribute("y", "3");
      bmark.setAttribute("class", "graph-node__expand-badge-mark");
      bmark.textContent = "+";
      badge.appendChild(bmark);
      g.appendChild(badge);
    }
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
