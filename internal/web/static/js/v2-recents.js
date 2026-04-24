/*
 * Recents — per-user localStorage ring buffer.
 *
 * Records: {type, dn, cn, lastSeenAt}
 * Key:     ldap-manager:recents:v1
 * Cap:     10, FIFO eviction.
 *
 * Icon rendering mirrors the palette (v2-palette.js) and the server-
 * rendered pinned_fragment so the home page presents a consistent
 * vocabulary — user / group / computer glyph + name, no text badge.
 */
(function () {
  "use strict";

  var KEY = "ldap-manager:recents:v1";
  var LIMIT = 10;
  var SVG_NS = "http://www.w3.org/2000/svg";

  // Static icon specs — literal attribute strings, CSP-safe. Mirrors
  // the shapes in internal/web/templates/icons.templ so list rows,
  // palette rows, and recents rows share one visual language.
  var ICON_SPECS = {
    user: [
      { tag: "circle", attrs: { cx: "8", cy: "5.5", r: "2.75" } },
      { tag: "path",   attrs: { d: "M2.5 13.5c0-2.5 2.5-4 5.5-4s5.5 1.5 5.5 4" } }
    ],
    group: [
      { tag: "circle", attrs: { cx: "6", cy: "6", r: "2.25" } },
      { tag: "circle", attrs: { cx: "11.5", cy: "5", r: "1.75" } },
      { tag: "path",   attrs: { d: "M1.5 13c0-2.2 2-3.5 4.5-3.5s4.5 1.3 4.5 3.5" } },
      { tag: "path",   attrs: { d: "M11 9.5c2 0 3.5 1 3.5 3" } }
    ],
    computer: [
      { tag: "rect", attrs: { x: "2", y: "3", width: "12", height: "8", rx: "1" } },
      { tag: "path", attrs: { d: "M6 14h4" } },
      { tag: "path", attrs: { d: "M8 11v3" } }
    ]
  };

  function buildIconSVG(kind) {
    var spec = ICON_SPECS[kind];
    if (!spec) return null;
    var svg = document.createElementNS(SVG_NS, "svg");
    svg.setAttribute("viewBox", "0 0 16 16");
    svg.setAttribute("fill", "none");
    svg.setAttribute("stroke", "currentColor");
    svg.setAttribute("stroke-width", "1.5");
    svg.setAttribute("stroke-linecap", "round");
    svg.setAttribute("stroke-linejoin", "round");
    svg.setAttribute("aria-hidden", "true");
    svg.setAttribute("focusable", "false");
    for (var i = 0; i < spec.length; i++) {
      var child = document.createElementNS(SVG_NS, spec[i].tag);
      var attrs = spec[i].attrs;
      for (var key in attrs) {
        if (Object.prototype.hasOwnProperty.call(attrs, key)) {
          child.setAttribute(key, attrs[key]);
        }
      }
      svg.appendChild(child);
    }
    return svg;
  }

  function read() {
    try { return JSON.parse(localStorage.getItem(KEY) || "[]"); }
    catch (_e) { return []; }
  }

  function write(arr) {
    try { localStorage.setItem(KEY, JSON.stringify(arr)); } catch (_e) {}
  }

  function push(entry) {
    if (!entry || !entry.dn || !entry.type || !entry.cn) return;
    var arr = read().filter(function (e) { return e.dn !== entry.dn; });
    arr.unshift({
      type: entry.type,
      dn: entry.dn,
      cn: entry.cn,
      lastSeenAt: new Date().toISOString()
    });
    if (arr.length > LIMIT) arr = arr.slice(0, LIMIT);
    write(arr);
  }

  function hrefFor(e) {
    var p = encodeURIComponent(e.dn);
    if (e.type === "user") return "/users/" + p;
    if (e.type === "group") return "/groups/" + p;
    if (e.type === "computer") return "/computers/" + p;
    return "/";
  }

  function render(container) {
    var list = container.querySelector("[data-recents-list]");
    var empty = container.querySelector("[data-recents-empty]");
    if (!list) return;

    var arr = read();
    if (arr.length === 0) return; // leave empty-message in place

    if (empty) empty.remove();
    while (list.firstChild) list.removeChild(list.firstChild);

    for (var i = 0; i < arr.length; i++) {
      var e = arr[i];
      var li = document.createElement("li");
      li.className = "home__list-item";

      // Prefer the entity icon; fall back to the text badge only if the
      // stored recent has an unknown type so the row never goes blank.
      var iconSVG = buildIconSVG(e.type);
      var type;
      if (iconSVG) {
        type = document.createElement("span");
        type.className = "home__list-type-icon";
        type.setAttribute("aria-hidden", "true");
        type.appendChild(iconSVG);
      } else {
        type = document.createElement("span");
        type.className = "home__list-type";
        type.textContent = e.type || "";
      }

      var a = document.createElement("a");
      a.className = "home__list-link";
      a.href = hrefFor(e);
      a.textContent = e.cn;

      li.appendChild(type);
      li.appendChild(a);
      list.appendChild(li);
    }
  }

  window.ldapManagerPushRecent = push;

  var container = document.querySelector("[data-recents]");
  if (container) render(container);
})();
