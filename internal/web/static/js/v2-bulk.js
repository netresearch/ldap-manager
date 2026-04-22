/*
 * Bulk-select toolbar for the V2 list pages (Phase 3 + 4).
 *
 * Wires up:
 *   - data-bulk checkboxes → maintain a Set of selected DNs.
 *   - Floating .bulk-bar → shows count + scope-appropriate action buttons.
 *   - Actions per scope:
 *       users:     Add to group…, Remove from group…, Delete, Cancel
 *       groups:    Add members…, Delete, Cancel
 *       computers: Disable, Delete, Cancel
 *
 * The scope comes from `[data-bulk-scope]` on <main> (emitted by the list
 * templates). If no scope is declared we fall back to the users action set —
 * that keeps the toolbar working on any legacy page that hasn't been
 * migrated.
 *
 * CSP-clean: no inline scripts, no innerHTML with dynamic strings. All DOM
 * construction uses createElement + textContent so arbitrary DNs can never
 * smuggle markup in.
 */
(function () {
  "use strict";

  var selected = new Set();
  var bar = null;
  var scope = null; // detected lazily, first time updateBar() runs

  // Per-scope action tables. Each entry is { label, onClick, dangerous }.
  // dangerous → rendered with the cancel-button color + a confirm() prompt.
  function detectScope() {
    var m = document.querySelector("main[data-bulk-scope]");
    if (m) return m.getAttribute("data-bulk-scope");
    return "users";
  }

  function entityNoun(s) {
    if (s === "groups") return "group";
    if (s === "computers") return "computer";
    return "user";
  }

  function buildActions(currentScope) {
    if (currentScope === "groups") {
      return [
        { label: "Add members\u2026", onClick: openAddMembers, danger: false },
        { label: "Delete groups", onClick: openDeleteGroups, danger: true },
      ];
    }
    if (currentScope === "computers") {
      return [
        { label: "Disable", onClick: openDisableComputers, danger: true },
        { label: "Delete", onClick: openDeleteComputers, danger: true },
      ];
    }
    return [
      { label: "Add to group\u2026", onClick: openAddToGroup, danger: false },
      { label: "Remove from group\u2026", onClick: openRemoveFromGroup, danger: false },
      { label: "Disable", onClick: openDisableUsers, danger: true },
      { label: "Delete", onClick: openDeleteUsers, danger: true },
    ];
  }

  function ensureBar() {
    if (bar) return bar;

    scope = detectScope();

    bar = document.createElement("div");
    bar.className = "bulk-bar";
    bar.setAttribute("role", "region");
    bar.setAttribute("aria-label", "Bulk actions");
    bar.hidden = true;

    var count = document.createElement("span");
    count.className = "bulk-bar__count";
    bar.appendChild(count);

    var actions = buildActions(scope);
    for (var i = 0; i < actions.length; i++) {
      var def = actions[i];
      var btn = document.createElement("button");
      btn.type = "button";
      btn.className = def.danger
        ? "bulk-bar__cancel bulk-bar__action"
        : "bulk-bar__action";
      btn.textContent = def.label;
      btn.addEventListener("click", def.onClick);
      bar.appendChild(btn);
    }

    var cancelBtn = document.createElement("button");
    cancelBtn.type = "button";
    cancelBtn.className = "bulk-bar__cancel";
    cancelBtn.textContent = "Cancel";
    cancelBtn.addEventListener("click", clearSelection);
    bar.appendChild(cancelBtn);

    document.body.appendChild(bar);

    return bar;
  }

  function updateBar() {
    ensureBar();
    var n = selected.size;
    bar.hidden = n === 0;
    var countEl = bar.querySelector(".bulk-bar__count");
    if (countEl) {
      var noun = entityNoun(scope);
      countEl.textContent =
        n + " " + (n === 1 ? noun : noun + "s") + " selected";
    }
  }

  function clearSelection() {
    selected.clear();
    var checks = document.querySelectorAll("[data-bulk]");
    for (var i = 0; i < checks.length; i++) {
      checks[i].checked = false;
    }
    updateBar();
  }

  // submitForm constructs a POST form with `target_dn[]` + any extra hidden
  // fields and submits it. Used by every scope's handler.
  function submitForm(action, extras) {
    if (selected.size === 0) return;

    var form = document.createElement("form");
    form.method = "post";
    form.action = action;
    form.style.display = "none";

    if (extras) {
      for (var name in extras) {
        if (!Object.prototype.hasOwnProperty.call(extras, name)) continue;
        var input = document.createElement("input");
        input.type = "hidden";
        input.name = name;
        input.value = extras[name];
        form.appendChild(input);
      }
    }

    selected.forEach(function (dn) {
      var i = document.createElement("input");
      i.type = "hidden";
      i.name = "target_dn";
      i.value = dn;
      form.appendChild(i);
    });

    document.body.appendChild(form);
    form.submit();
  }

  // ─── users actions ──────────────────────────────────────────────────

  function openAddToGroup() {
    if (selected.size === 0) return;

    var g = window.prompt(
      "Group DN to add the selected users to:\n" +
        "(e.g. cn=admins,ou=groups,dc=example,dc=com)"
    );
    if (g === null) return;
    g = g.trim();
    if (g === "") return;

    submitForm("/users/bulk?action=add-to-group", { group_dn: g });
  }

  function openRemoveFromGroup() {
    if (selected.size === 0) return;

    var g = window.prompt(
      "Group DN to remove the selected users from:\n" +
        "(e.g. cn=admins,ou=groups,dc=example,dc=com)"
    );
    if (g === null) return;
    g = g.trim();
    if (g === "") return;

    submitForm("/users/bulk?action=remove-from-group", { group_dn: g });
  }

  function openDisableUsers() {
    if (selected.size === 0) return;

    if (
      !window.confirm(
        "Disable " +
          selected.size +
          " user(s)? (Note: not yet implemented — will return 501)"
      )
    )
      return;

    submitForm("/users/bulk?action=disable", null);
  }

  function openDeleteUsers() {
    if (selected.size === 0) return;

    if (
      !window.confirm(
        "Delete " +
          selected.size +
          " user(s)? This cannot be undone."
      )
    )
      return;

    submitForm("/users/bulk?action=delete", null);
  }

  // ─── groups actions ─────────────────────────────────────────────────

  function openAddMembers() {
    if (selected.size === 0) return;

    var u = window.prompt(
      "User DN to add as a member to the selected groups:\n" +
        "(e.g. cn=alice,ou=users,dc=example,dc=com)"
    );
    if (u === null) return;
    u = u.trim();
    if (u === "") return;

    submitForm("/groups/bulk?action=add-members", { user_dn: u });
  }

  function openDeleteGroups() {
    if (selected.size === 0) return;

    if (
      !window.confirm(
        "Delete " +
          selected.size +
          " group(s)? (Note: not yet implemented — will return 501)"
      )
    )
      return;

    submitForm("/groups/bulk?action=delete", null);
  }

  // ─── computers actions ──────────────────────────────────────────────

  function openDisableComputers() {
    if (selected.size === 0) return;

    if (
      !window.confirm(
        "Disable " +
          selected.size +
          " computer(s)? (Note: not yet implemented — will return 501)"
      )
    )
      return;

    submitForm("/computers/bulk?action=disable", null);
  }

  function openDeleteComputers() {
    if (selected.size === 0) return;

    if (
      !window.confirm(
        "Delete " +
          selected.size +
          " computer(s)? (Note: not yet implemented — will return 501)"
      )
    )
      return;

    submitForm("/computers/bulk?action=delete", null);
  }

  document.addEventListener("change", function (ev) {
    var t = ev.target;
    if (!t || !t.hasAttribute) return;

    // Per-row checkbox
    if (t.hasAttribute("data-bulk")) {
      if (t.checked) {
        selected.add(t.value);
      } else {
        selected.delete(t.value);
      }
      updateBar();
      syncSelectAll();
      return;
    }

    // Select-all-visible master checkbox. Walks the list rows and
    // toggles every data-bulk checkbox whose row isn't `hidden` by
    // the free-text filter. This lets an operator narrow the list
    // with the search input, click the master, and bulk-act on the
    // filtered subset.
    if (t.hasAttribute("data-select-all-visible")) {
      var checks = document.querySelectorAll("[data-bulk]");
      for (var i = 0; i < checks.length; i++) {
        var ck = checks[i];
        var row = ck.closest("[data-search-item]") || ck.closest(".list-row");
        if (row && row.hidden) continue;
        ck.checked = t.checked;
        if (t.checked) {
          selected.add(ck.value);
        } else {
          selected.delete(ck.value);
        }
      }
      updateBar();
    }
  });

  // syncSelectAll flips the master checkbox between empty /
  // indeterminate / all depending on the current selection over the
  // visible rows.
  function syncSelectAll() {
    var master = document.querySelector("[data-select-all-visible]");
    if (!master) return;
    var checks = document.querySelectorAll("[data-bulk]");
    var visible = 0;
    var checked = 0;
    for (var i = 0; i < checks.length; i++) {
      var ck = checks[i];
      var row = ck.closest("[data-search-item]") || ck.closest(".list-row");
      if (row && row.hidden) continue;
      visible++;
      if (ck.checked) checked++;
    }
    master.indeterminate = checked > 0 && checked < visible;
    master.checked = visible > 0 && checked === visible;
  }

  // Re-sync the master whenever the filter hides or reveals rows.
  // v2-search-filter.js flips `hidden` directly; observe the list so
  // we don't couple the two modules.
  document.addEventListener("DOMContentLoaded", function () {
    var list = document.querySelector("[data-search-list]");
    if (!list || typeof MutationObserver === "undefined") return;
    new MutationObserver(syncSelectAll).observe(list, {
      subtree: true,
      attributes: true,
      attributeFilter: ["hidden"],
    });
  });
})();
