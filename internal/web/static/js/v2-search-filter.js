// v2-search-filter.js — client-side filter for list pages
// ---------------------------------------------------------------
// Wires each [data-search-input] on the page to the sibling
// [data-search-list] and hides [data-search-item] rows whose text
// content does not match the query (case-insensitive, debounced).
//
// CSP-safe: no inline handlers, no eval, no dependencies.

(function () {
    "use strict";

    function initFilter(input) {
        // The list target lives outside the filter container in V2
        // (filters row above, list-rows below). Fall back to the
        // document-wide lookup when a scoped one fails.
        var scope =
            input.closest("[data-search-scope]") ||
            input.ownerDocument;
        var list =
            scope.querySelector("[data-search-list]") ||
            document.querySelector("[data-search-list]");
        if (!list) return;

        var items = Array.prototype.slice.call(
            list.querySelectorAll("[data-search-item]")
        );
        // Keep count scoped consistently with `list` — if more than one
        // [data-search-filter] widget is ever rendered on a page the
        // document-wide lookup would point every filter at the first
        // count element. The document fallback is kept for legacy
        // single-widget pages that put the count outside the scope.
        var count =
            scope.querySelector("[data-search-count]") ||
            document.querySelector("[data-search-count]");
        var debounceTimer = null;

        input.setAttribute("role", "searchbox");

        function filter() {
            var query = (input.value || "").toLowerCase().trim();
            var visible = 0;
            for (var i = 0; i < items.length; i++) {
                var item = items[i];
                // Prefer data-search-text when present — rows use it to
                // include hidden fields like email, sAMAccountName, and
                // description so the filter matches attributes that
                // aren't displayed on the row itself. Falls back to the
                // row's textContent for rows without the attribute.
                var haystack = item.getAttribute("data-search-text") || item.textContent || "";
                haystack = haystack.toLowerCase();
                var matches = query === "" || haystack.indexOf(query) !== -1;
                item.hidden = !matches;
                if (matches) visible++;
            }
            if (count) {
                count.textContent = visible + " items";
            }
        }

        input.addEventListener("input", function () {
            if (debounceTimer) clearTimeout(debounceTimer);
            debounceTimer = setTimeout(filter, 80);
        });
        input.addEventListener("keydown", function (e) {
            if (e.key === "Escape") {
                input.value = "";
                filter();
            }
        });
    }

    function init() {
        var inputs = document.querySelectorAll("[data-search-input]");
        for (var i = 0; i < inputs.length; i++) initFilter(inputs[i]);
    }

    if (document.readyState === "loading") {
        document.addEventListener("DOMContentLoaded", init);
    } else {
        init();
    }
})();
