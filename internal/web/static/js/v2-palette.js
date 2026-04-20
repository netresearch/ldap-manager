/*
 * Command palette — vanilla JS on top of <dialog>.
 *
 * Responsibilities:
 *   - Open via ⌘K / Ctrl+K / "/" / any [data-open-palette] click.
 *   - Close via Esc (dialog does this for free) or backdrop click.
 *   - Fetch /api/search-index.json on first open, cache in sessionStorage.
 *   - Fuzzy-match query against index on every keystroke (40ms debounced).
 *   - Keyboard: ↑/↓ change aria-selected, Enter navigates.
 *
 * CSP-safe — no inline code, no eval, no innerHTML with user content.
 */
(function () {
  "use strict";

  var dialog = document.getElementById("cmd-palette");
  if (!dialog) return;

  var input = dialog.querySelector("[data-palette-input]");
  var results = dialog.querySelector("[data-palette-results]");

  var INDEX_KEY = "ldap-manager:search-index:v1";
  var ETAG_KEY  = "ldap-manager:search-index-etag:v1";

  var index = null;
  var focused = -1;

  function openPalette() {
    if (dialog.open) return;
    if (typeof dialog.showModal === "function") dialog.showModal();
    else dialog.setAttribute("open", "");
    input.value = "";
    focused = -1;
    renderEmptyState("Type to search.");
    input.focus();
    loadIndex();
  }

  function closePalette() {
    if (!dialog.open) return;
    try { dialog.close(); } catch (_e) { dialog.removeAttribute("open"); }
  }

  function loadIndex() {
    if (index) return;
    var cachedIndex = null, cachedETag = null;
    try {
      cachedETag = sessionStorage.getItem(ETAG_KEY);
      var raw = sessionStorage.getItem(INDEX_KEY);
      if (raw) cachedIndex = JSON.parse(raw);
    } catch (_e) {}

    var headers = {};
    if (cachedETag) headers["If-None-Match"] = cachedETag;

    fetch("/api/search-index.json", { headers: headers, credentials: "same-origin" })
      .then(function (r) {
        if (r.status === 304 && cachedIndex) {
          index = cachedIndex;
          renderQuery(input.value);
          return;
        }
        if (!r.ok) throw new Error("search-index " + r.status);
        var etag = r.headers.get("ETag");
        return r.json().then(function (data) {
          index = data;
          try {
            sessionStorage.setItem(INDEX_KEY, JSON.stringify(data));
            if (etag) sessionStorage.setItem(ETAG_KEY, etag);
          } catch (_e) {}
          renderQuery(input.value);
        });
      })
      .catch(function (err) {
        console.error(err);
        renderEmptyState("Could not load search index.");
      });
  }

  // Score: lower = better; -1 = reject.
  function scoreEntry(q, entry) {
    if (!q) return 0;
    var qlc = q.toLowerCase();
    var name = (entry.cn || "").toLowerCase();
    var sam = (entry.sam || "").toLowerCase();
    var ou  = (entry.ou  || "").toLowerCase();

    if (name === qlc || sam === qlc) return 0;
    if (name.indexOf(qlc) === 0 || sam.indexOf(qlc) === 0) return 1;
    if (name.indexOf(qlc) >= 0 || sam.indexOf(qlc) >= 0) return 2;

    var initials = name.split(/\s+|[._-]/).map(function (w) { return w.charAt(0); }).join("");
    if (initials.indexOf(qlc) >= 0) return 3;

    if (ou.indexOf(qlc) >= 0) return 4;
    return -1;
  }

  function clearResults() {
    while (results.firstChild) results.removeChild(results.firstChild);
  }

  function renderEmptyState(message) {
    clearResults();
    var li = document.createElement("li");
    li.className = "palette__empty";
    li.textContent = message;
    results.appendChild(li);
    focused = -1;
  }

  // Build one result row using only safe DOM methods.
  function buildItem(entry, isFocused) {
    var li = document.createElement("li");
    li.className = "palette__item";
    li.setAttribute("role", "option");
    li.setAttribute("data-href", hrefFor(entry));
    li.setAttribute("aria-selected", isFocused ? "true" : "false");

    var type = document.createElement("span");
    type.className = "palette__type";
    type.textContent = entry.type;

    var name = document.createElement("span");
    var nameText = document.createElement("span");
    nameText.textContent = entry.cn;
    name.appendChild(nameText);
    if (entry.sam) {
      var sam = document.createElement("span");
      sam.className = "palette__ctx";
      sam.textContent = " (" + entry.sam + ")";
      name.appendChild(sam);
    }

    var ctx = document.createElement("span");
    ctx.className = "palette__ctx";
    ctx.textContent = entry.ou || "";

    li.appendChild(type);
    li.appendChild(name);
    li.appendChild(ctx);

    li.addEventListener("click", function () {
      var href = li.getAttribute("data-href");
      if (href) navigateTo(href);
    });
    return li;
  }

  function renderQuery(q) {
    if (!index) return;

    var matched = [];
    for (var i = 0; i < index.length; i++) {
      var s = scoreEntry(q, index[i]);
      if (s >= 0) matched.push({ s: s, e: index[i] });
    }
    matched.sort(function (a, b) {
      if (a.s !== b.s) return a.s - b.s;
      return a.e.cn.localeCompare(b.e.cn);
    });

    var top = matched.slice(0, 50);
    clearResults();

    if (top.length === 0) {
      renderEmptyState(q ? "No matches." : "Start typing.");
      return;
    }
    for (var j = 0; j < top.length; j++) {
      results.appendChild(buildItem(top[j].e, j === 0));
    }
    focused = 0;
  }

  function hrefFor(e) {
    var p = encodeURIComponent(e.dn);
    if (e.type === "user") return "/users/" + p;
    if (e.type === "group") return "/groups/" + p;
    if (e.type === "computer") return "/computers/" + p;
    return "/";
  }

  function navigateTo(href) {
    closePalette();
    window.location.href = href;
  }

  function moveFocus(delta) {
    var items = results.querySelectorAll("[role=option]");
    if (items.length === 0) return;
    focused = Math.max(0, Math.min(items.length - 1, focused + delta));
    for (var i = 0; i < items.length; i++) {
      items[i].setAttribute("aria-selected", i === focused ? "true" : "false");
    }
    items[focused].scrollIntoView({ block: "nearest" });
  }

  function enterFocused() {
    var items = results.querySelectorAll("[role=option]");
    if (focused < 0 || focused >= items.length) return;
    var href = items[focused].getAttribute("data-href");
    if (href) navigateTo(href);
  }

  // --- wire up ---
  document.addEventListener("click", function (ev) {
    var t = ev.target instanceof Element ? ev.target.closest("[data-open-palette]") : null;
    if (t) { ev.preventDefault(); openPalette(); }
  });

  document.addEventListener("keydown", function (ev) {
    var mod = ev.metaKey || ev.ctrlKey;
    if (mod && (ev.key === "k" || ev.key === "K" || ev.key === "/")) {
      ev.preventDefault();
      openPalette();
      return;
    }
    if (ev.key === "/" && !mod && !dialog.open) {
      var a = document.activeElement;
      var tag = a && a.tagName;
      if (tag !== "INPUT" && tag !== "TEXTAREA" && !(a && a.isContentEditable)) {
        ev.preventDefault();
        openPalette();
      }
    }
  });

  dialog.addEventListener("click", function (ev) {
    if (ev.target === dialog) closePalette();
  });

  var t = null;
  input.addEventListener("input", function () {
    if (t) clearTimeout(t);
    t = setTimeout(function () { renderQuery(input.value); }, 40);
  });

  input.addEventListener("keydown", function (ev) {
    if (ev.key === "ArrowDown") { ev.preventDefault(); moveFocus(1); return; }
    if (ev.key === "ArrowUp")   { ev.preventDefault(); moveFocus(-1); return; }
    if (ev.key === "Enter")     { ev.preventDefault(); enterFocused(); return; }
  });
})();
