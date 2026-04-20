/*
 * Recents — per-user localStorage ring buffer.
 *
 * Records: {type, dn, cn, lastSeenAt}
 * Key:     ldap-manager:recents:v1
 * Cap:     10, FIFO eviction.
 */
(function () {
  "use strict";

  var KEY = "ldap-manager:recents:v1";
  var LIMIT = 10;

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

      var type = document.createElement("span");
      type.className = "home__list-type";
      type.textContent = e.type;

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
