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

  // Datalist combobox helper: the drawer "Add to group" / "Add user"
  // forms pair a text input (user types a CN) with a hidden input
  // (the backend wants a DN). The <option> elements in the datalist
  // carry the DN in a data-dn attribute; when the user picks or types
  // a match, mirror that DN into the hidden input so the form POSTs
  // correctly. Works purely on standards-track DOM APIs — no extra
  // framework, CSP-safe.
  function syncDatalistForm(input) {
    if (!input) return;
    var form = input.closest("form");
    if (!form) return;
    var hidden = form.querySelector("[data-drawer-datalist-value]");
    if (!hidden) return;
    var listId = input.getAttribute("list");
    if (!listId) return;
    var list = document.getElementById(listId);
    if (!list) return;

    function resolve() {
      var v = input.value;
      if (!v) { hidden.value = ""; return; }
      var opts = list.querySelectorAll("option");
      for (var i = 0; i < opts.length; i++) {
        if (opts[i].value === v) {
          hidden.value = opts[i].getAttribute("data-dn") || "";
          return;
        }
      }
      // No CN match — clear the hidden DN so the backend doesn't
      // receive a stale value from a previous pick.
      hidden.value = "";
    }

    input.addEventListener("input", resolve);
    input.addEventListener("change", resolve);
    form.addEventListener("submit", function (ev) {
      resolve();
      if (!hidden.value) {
        // No valid pick — prevent a useless POST and re-focus the
        // input so the operator can correct the typo.
        ev.preventDefault();
        input.focus();
        input.setAttribute("aria-invalid", "true");
      } else {
        input.removeAttribute("aria-invalid");
      }
    });
  }

  function wireDatalistForms(root) {
    if (!root || !root.querySelectorAll) return;
    var inputs = root.querySelectorAll("[data-drawer-datalist-input]");
    for (var i = 0; i < inputs.length; i++) syncDatalistForm(inputs[i]);
  }

  wireDatalistForms(document);

  // Wire up the combobox again whenever htmx swaps a new drawer body.
  document.body.addEventListener("htmx:afterSwap", function (ev) {
    if (ev.detail && ev.detail.target) wireDatalistForms(ev.detail.target);
  });
})();
