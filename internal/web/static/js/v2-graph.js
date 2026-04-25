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

  // labelHint composes the keyboard-affordance suffix for an aria-label.
  // Mirrors the suffix logic in activateNodes + renderNode so all nodes
  // carry consistent SR text. `expandable` is "true" for `+`, "expanded"
  // for `-`, anything else for "no badge".
  function labelHint(expandable) {
    var hint = " Press Enter to open.";
    if (expandable === "true") hint += " Press + to expand more relations.";
    if (expandable === "expanded") hint += " Press - to collapse.";
    return hint;
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
      // Strip any prior hint so re-activations don't pile suffixes.
      var cut = label.indexOf(" Press Enter");
      if (cut !== -1) label = label.substring(0, cut);
      label += labelHint(n.getAttribute("data-expandable"));
      n.setAttribute("aria-label", label);
    });
  }

  // wireHoverHighlight: hovering a node highlights its incident edges
  // by toggling a .graph-edge--hover-related class on every edge whose
  // data-source or data-target equals the node's DN.
  function wireHoverHighlight(svg) {
    function setHighlight(dn, on) {
      if (!dn) return;
      var sel =
        '.graph-edge[data-source="' +
        CSS.escape(dn) +
        '"], .graph-edge[data-target="' +
        CSS.escape(dn) +
        '"]';
      svg.querySelectorAll(sel).forEach(function (line) {
        line.classList.toggle("graph-edge--hover-related", on);
      });
    }
    svg.addEventListener("mouseover", function (e) {
      var node = e.target.closest(".graph-node");
      if (!node) return;
      setHighlight(node.getAttribute("data-dn"), true);
    });
    svg.addEventListener("mouseout", function (e) {
      var node = e.target.closest(".graph-node");
      if (!node) return;
      // mouseout fires when leaving inner SVG elements too; only un-set
      // the highlight when the relatedTarget isn't still inside the same
      // node group.
      var to = e.relatedTarget;
      if (to && node.contains(to)) return;
      setHighlight(node.getAttribute("data-dn"), false);
    });
    svg.addEventListener("focusin", function (e) {
      var node = e.target.closest(".graph-node");
      if (!node) return;
      setHighlight(node.getAttribute("data-dn"), true);
    });
    svg.addEventListener("focusout", function (e) {
      var node = e.target.closest(".graph-node");
      if (!node) return;
      setHighlight(node.getAttribute("data-dn"), false);
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
    wireHoverHighlight(svg);
    wireDepthSlider();
  });

  // userSpacePoint converts a screen-coord MouseEvent into the SVG's
  // own user-space coordinate system (the one the inner viewport <g>
  // is positioned in). Without this, cursor-anchored zoom math would
  // mix screen pixels with viewBox units and drift relative to the
  // pointer.
  function userSpacePoint(svg, e) {
    var pt = svg.createSVGPoint();
    pt.x = e.clientX;
    pt.y = e.clientY;
    var ctm = svg.getScreenCTM();
    if (!ctm) return { x: 0, y: 0 };
    var p = pt.matrixTransform(ctm.inverse());
    return { x: p.x, y: p.y };
  }

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
    // page scroll behaviour is preserved. Anchor zoom around the
    // pointer's USER-SPACE position so the world-point under the cursor
    // stays put across the scale change. (Earlier versions mixed screen
    // pixels with viewBox units and drifted noticeably.)
    svg.addEventListener(
      "wheel",
      function (e) {
        if (!(e.ctrlKey || e.metaKey)) return;
        e.preventDefault();
        var oldScale = scale;
        var delta = -e.deltaY * 0.001;
        scale = Math.min(3, Math.max(0.3, scale * (1 + delta)));
        if (oldScale !== scale) {
          var u = userSpacePoint(svg, e);
          tx = u.x - ((u.x - tx) * scale) / oldScale;
          ty = u.y - ((u.y - ty) * scale) / oldScale;
        }
        apply();
      },
      { passive: false },
    );

    // Arrow-key pan and +/- zoom via keyboard. Translation step is in
    // user-space units (matches the viewBox) so behaviour scales the
    // same regardless of the SVG's pixel size on screen.
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
      }
    });
  }

  function wireNodeClicks(svg, state) {
    svg.addEventListener("click", function (e) {
      var node = e.target.closest(".graph-node");
      if (!node) return;
      var dn = node.getAttribute("data-dn");
      var type = node.getAttribute("data-type");
      var expandable = node.getAttribute("data-expandable");
      var clickedBadge = !!e.target.closest(".graph-node__expand-badge");

      if (clickedBadge && expandable === "true") {
        expandNode(dn, state, svg);
      } else if (clickedBadge && expandable === "expanded") {
        collapseNode(dn, state, svg);
      } else {
        pivotToDrawer(dn, type);
      }
    });
    // Keyboard parity with mouse: Enter pivots (matches the SSR
    // aria-label "Press Enter to open"), and +/= expands when the node
    // is expandable, -/_ collapses when already expanded. Space is
    // intentionally not bound — convention on non-form elements is
    // page-down, not activation.
    svg.addEventListener("keydown", function (e) {
      var node = e.target.closest(".graph-node");
      if (!node) return;
      var dn = node.getAttribute("data-dn");
      var type = node.getAttribute("data-type");
      var expandable = node.getAttribute("data-expandable");

      if (e.key === "Enter") {
        e.preventDefault();
        pivotToDrawer(dn, type);
      } else if (
        (e.key === "+" || e.key === "=") &&
        expandable === "true"
      ) {
        e.preventDefault();
        expandNode(dn, state, svg);
      } else if (
        (e.key === "-" || e.key === "_") &&
        expandable === "expanded"
      ) {
        e.preventDefault();
        collapseNode(dn, state, svg);
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

  function announce(msg) {
    var el = document.getElementById("graph-announce");
    if (el) {
      el.textContent = "";
      setTimeout(function () {
        el.textContent = msg;
      }, 10);
    }
  }

  // setBadge replaces a node's expand-badge between "+" (expandable),
  // "-" (expanded — collapse possible), and removed (terminal node).
  // Returns the new badge element or null.
  function setBadge(nodeEl, kind) {
    var existing = nodeEl.querySelector(".graph-node__expand-badge");
    if (existing) existing.remove();
    if (kind !== "+" && kind !== "-") return null;
    var ns = "http://www.w3.org/2000/svg";
    var badge = document.createElementNS(ns, "g");
    badge.setAttribute("class", "graph-node__expand-badge");
    badge.setAttribute("transform", "translate(18,-18)");
    badge.setAttribute("aria-hidden", "true");
    var bg = document.createElementNS(ns, "circle");
    bg.setAttribute("r", "8");
    bg.setAttribute("class", "graph-node__expand-badge-bg");
    badge.appendChild(bg);
    var mark = document.createElementNS(ns, "text");
    mark.setAttribute("text-anchor", "middle");
    mark.setAttribute("y", "3");
    mark.setAttribute("class", "graph-node__expand-badge-mark");
    mark.textContent = kind;
    badge.appendChild(mark);
    nodeEl.appendChild(badge);
    return badge;
  }

  // refreshLabel rebuilds the aria-label for a node based on the
  // current data-expandable state ("true"/"expanded"/"false").
  function refreshLabel(nodeEl) {
    var label = nodeEl.getAttribute("aria-label") || "";
    var cut = label.indexOf(" Press Enter");
    if (cut !== -1) label = label.substring(0, cut);
    label += labelHint(nodeEl.getAttribute("data-expandable"));
    nodeEl.setAttribute("aria-label", label);
  }

  function expandNode(dn, state, svg) {
    var url = "/api/graph.json?entity=" + encodeURIComponent(dn) + "&depth=1";
    fetch(url, { credentials: "same-origin" })
      .then(function (r) {
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
        var parent = state.nodes.find(function (x) {
          return x.dn === dn;
        });
        var parentRing = parent ? parent.ring : 1;
        data.nodes.forEach(function (n) {
          if (existingDNs[n.dn]) return;
          n.ring = parentRing + 1;
          n.addedBy = dn;
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
            e.addedBy = dn;
            state.edges.push(e);
            renderEdge(svg, e, state.nodes);
          }
        });
        Object.keys(affectedRings).forEach(function (r) {
          reLayoutRing(svg, state, parseInt(r, 10));
        });
        recomputeDegrees(state, svg);
        // Switch the parent's badge to "-" (collapse) and update its
        // aria-label so SR users hear the new affordance.
        var el = svg.querySelector(
          '.graph-node[data-dn="' + CSS.escape(dn) + '"]',
        );
        if (el) {
          el.setAttribute("data-expandable", "expanded");
          setBadge(el, "-");
          refreshLabel(el);
        }
        announce("Expanded " + dn + ": added " + added + " nodes.");
      })
      .catch(function (err) {
        console.error("graph expand failed", err);
        announce("Expand failed.");
      });
  }

  // collapseNode removes every node and edge that was added when this
  // parent was expanded, restores the "+" badge so the user can
  // re-expand, and re-lays out any rings that lost members.
  function collapseNode(dn, state, svg) {
    var removedDNs = {};
    var affectedRings = {};
    // Collect descendants: a node is in the collapse set if it (or one
    // of its ancestors transitively) was addedBy the clicked dn.
    var queue = [dn];
    var ancestors = {};
    ancestors[dn] = true;
    while (queue.length > 0) {
      var head = queue.shift();
      state.nodes.forEach(function (n) {
        if (n.addedBy === head && !ancestors[n.dn]) {
          ancestors[n.dn] = true;
          removedDNs[n.dn] = true;
          affectedRings[n.ring] = true;
          queue.push(n.dn);
        }
      });
    }
    // Drop nodes from state + DOM.
    state.nodes = state.nodes.filter(function (n) {
      if (!removedDNs[n.dn]) return true;
      var el = svg.querySelector(
        '.graph-node[data-dn="' + CSS.escape(n.dn) + '"]',
      );
      if (el) el.remove();
      return false;
    });
    // Drop edges whose endpoints are removed, OR whose addedBy points
    // to the collapsed parent or any of its transitive descendants.
    // The transitive case covers edges introduced by a sub-expansion
    // that connect two nodes that survive the collapse — without the
    // ancestors check those edges would orphan after collapse.
    state.edges = state.edges.filter(function (e) {
      var keep =
        !removedDNs[e.source] &&
        !removedDNs[e.target] &&
        e.addedBy !== dn &&
        !ancestors[e.addedBy];
      if (keep) return true;
      var line = svg.querySelector(
        'line[data-source="' +
          CSS.escape(e.source) +
          '"][data-target="' +
          CSS.escape(e.target) +
          '"]',
      );
      if (line) line.remove();
      return false;
    });
    Object.keys(affectedRings).forEach(function (r) {
      reLayoutRing(svg, state, parseInt(r, 10));
    });
    recomputeDegrees(state, svg);
    // Restore the parent's "+" badge and aria-label.
    var parentEl = svg.querySelector(
      '.graph-node[data-dn="' + CSS.escape(dn) + '"]',
    );
    if (parentEl) {
      parentEl.setAttribute("data-expandable", "true");
      setBadge(parentEl, "+");
      refreshLabel(parentEl);
    }
    announce(
      "Collapsed " +
        dn +
        ": removed " +
        Object.keys(removedDNs).length +
        " nodes.",
    );
  }

  // reLayoutRing redistributes nodes on a ring evenly around the circle
  // and re-routes any edges touching them. Called after expandNode
  // merges new nodes onto an existing ring or collapseNode removes
  // some.
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

  // discRadius scales node disc size with degree so high-connectivity
  // nodes (e.g. groups with many members) read as visually weightier.
  // Capped so the max never exceeds ~1.4x the base — beyond that
  // overlapping discs become unreadable.
  //
  // Math.round mirrors the Go-side discRadius() in graph_v2.templ
  // (which truncates to int) so SSR-rendered and JS-rendered nodes
  // share the same radius for the same degree.
  function discRadius(degree) {
    var base = 22;
    var step = 1.5;
    var max = 32;
    return Math.min(max, Math.round(base + (degree || 0) * step));
  }

  // recomputeDegrees walks state.edges, refreshes each node's degree
  // count, and updates the rendered <circle r="..."> attribute so disc
  // sizing stays in sync with the actual edge set after expand or
  // collapse. Without this, a hub that gains/loses memberships keeps
  // its old radius and the weighted layout misrepresents reality.
  function recomputeDegrees(state, svg) {
    var deg = {};
    state.edges.forEach(function (e) {
      deg[e.source] = (deg[e.source] || 0) + 1;
      deg[e.target] = (deg[e.target] || 0) + 1;
    });
    state.nodes.forEach(function (n) {
      var newDegree = deg[n.dn] || 0;
      if (n.degree === newDegree) return;
      n.degree = newDegree;
      var el = svg.querySelector(
        '.graph-node[data-dn="' + CSS.escape(n.dn) + '"] .graph-node__disc',
      );
      if (el) el.setAttribute("r", String(discRadius(newDegree)));
    });
  }

  function renderNode(svg, n) {
    var ns = "http://www.w3.org/2000/svg";
    var viewport = svg.querySelector(".graph-viewport");
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
    g.setAttribute("data-expandable", n.expandable ? "true" : "false");
    var base = nodeLabel(n);
    g.setAttribute(
      "aria-label",
      base + labelHint(n.expandable ? "true" : "false"),
    );
    var circ = document.createElementNS(ns, "circle");
    circ.setAttribute("r", String(discRadius(n.degree)));
    circ.setAttribute("class", "graph-node__disc");
    g.appendChild(circ);
    var text = document.createElementNS(ns, "text");
    text.setAttribute("text-anchor", "middle");
    text.setAttribute("y", "4");
    text.setAttribute("class", "graph-node__label");
    text.textContent = n.label;
    g.appendChild(text);
    if (n.expandable) setBadge(g, "+");
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
    viewport.insertBefore(line, viewport.firstChild);
  }

  function wireDepthSlider() {
    var slider = document.querySelector("[data-graph-slider]");
    if (!slider) return;
    var out = document.querySelector(".graph-slider__value");
    slider.addEventListener("input", function () {
      if (out) out.textContent = slider.value;
    });
    slider.addEventListener("change", function () {
      var form = slider.form;
      if (form) form.submit();
    });
  }
})();
