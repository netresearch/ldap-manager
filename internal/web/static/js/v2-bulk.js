/*
 * Bulk-select toolbar for the /users list (Phase 3).
 *
 * Wires up:
 *   - data-bulk checkboxes → maintain a Set of selected DNs.
 *   - Floating .bulk-bar → shows count + Add-to-group + Cancel.
 *   - Add-to-group → prompts for a group DN, then submits a hidden form
 *     carrying target_dn[] + group_dn to POST /users/bulk?action=add-to-group.
 *
 * CSP-clean: no inline scripts, no innerHTML with dynamic strings. All DOM
 * construction uses createElement + textContent so arbitrary DNs can never
 * smuggle markup in.
 */
(function () {
  "use strict";

  var selected = new Set();
  var bar = null;

  function ensureBar() {
    if (bar) return bar;

    bar = document.createElement("div");
    bar.className = "bulk-bar";
    bar.setAttribute("role", "region");
    bar.setAttribute("aria-label", "Bulk actions");
    bar.hidden = true;

    var count = document.createElement("span");
    count.className = "bulk-bar__count";
    bar.appendChild(count);

    var addBtn = document.createElement("button");
    addBtn.type = "button";
    addBtn.className = "bulk-bar__action";
    addBtn.textContent = "Add to group\u2026";
    addBtn.addEventListener("click", openAddToGroup);
    bar.appendChild(addBtn);

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
      countEl.textContent = n + (n === 1 ? " user selected" : " users selected");
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

  function openAddToGroup() {
    if (selected.size === 0) return;

    var g = window.prompt(
      "Group DN to add the selected users to:\n" +
        "(e.g. cn=admins,ou=groups,dc=example,dc=com)"
    );
    if (g === null) return;
    g = g.trim();
    if (g === "") return;

    var form = document.createElement("form");
    form.method = "post";
    form.action = "/users/bulk?action=add-to-group";
    form.style.display = "none";

    var groupInput = document.createElement("input");
    groupInput.type = "hidden";
    groupInput.name = "group_dn";
    groupInput.value = g;
    form.appendChild(groupInput);

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

  document.addEventListener("change", function (ev) {
    var t = ev.target;
    if (!t || !t.hasAttribute || !t.hasAttribute("data-bulk")) return;
    if (t.checked) {
      selected.add(t.value);
    } else {
      selected.delete(t.value);
    }
    updateBar();
  });
})();
