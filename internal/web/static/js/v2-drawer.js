/*
 * Drawer helpers: record recents on htmx swap, handle drawer-close click.
 *
 * All DOM access is CSP-safe (no inline handlers, no innerHTML with dynamic
 * strings). Entity metadata arrives via data-recent-* attributes set by the
 * drawer fragment template.
 */
(function () {
  "use strict";

  function recordRecentFromHead(head) {
    if (!head) return;
    var type = head.getAttribute("data-recent-type");
    var dn = head.getAttribute("data-recent-dn");
    var cn = head.getAttribute("data-recent-cn");
    if (!type || !dn || !cn) return;
    if (typeof window.ldapManagerPushRecent === "function") {
      window.ldapManagerPushRecent({ type: type, dn: dn, cn: cn });
    }
  }

  // When htmx swaps the drawer target, read the freshly-inserted head.
  document.body.addEventListener("htmx:afterSwap", function (ev) {
    var target = ev.detail && ev.detail.target;
    if (!target) return;
    if (target.id !== "drawer") return;
    var head = target.querySelector("[data-recent-type]");
    recordRecentFromHead(head);
  });

  // On full-page detail view, also record on load.
  var head = document.querySelector(".drawer--full [data-recent-type]");
  if (head) recordRecentFromHead(head);
})();
