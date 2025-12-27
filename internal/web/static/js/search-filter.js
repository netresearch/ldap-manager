/**
 * Client-side search filter for list pages.
 * Filters list items based on text content without server requests.
 */
export function initSearchFilters() {
    document.querySelectorAll("[data-search-filter]").forEach((container) => {
        new SearchFilter(container);
    });
}
class SearchFilter {
    constructor(container) {
        this.items = [];
        const input = container.querySelector("[data-search-input]");
        const listContainer = container.querySelector("[data-search-list]");
        if (!input || !listContainer) {
            // Required elements not found - skip initialization
            this.input = null;
            this.listContainer = null;
            this.countDisplay = null;
            return;
        }
        this.input = input;
        this.listContainer = listContainer;
        this.countDisplay = container.querySelector("[data-search-count]");
        this.items = Array.from(this.listContainer.querySelectorAll("[data-search-item]"));
        this.init();
    }
    init() {
        // Set up ARIA role (preserve template-defined aria-label for specificity)
        this.input.setAttribute("role", "searchbox");
        // Event listener with debounce for performance
        let debounceTimer;
        this.input.addEventListener("input", () => {
            clearTimeout(debounceTimer);
            debounceTimer = setTimeout(() => this.filter(), 100);
        });
        // Clear on Escape
        this.input.addEventListener("keydown", (e) => {
            if (e.key === "Escape") {
                this.input.value = "";
                this.filter();
            }
        });
        // Update count on initial load
        this.updateCount(this.items.length);
    }
    filter() {
        const query = this.input.value.toLowerCase().trim();
        let visibleCount = 0;
        this.items.forEach((item) => {
            const text = (item.textContent || "").toLowerCase();
            const matches = query === "" || text.includes(query);
            item.classList.toggle("hidden", !matches);
            if (matches)
                visibleCount++;
        });
        this.updateCount(visibleCount);
    }
    updateCount(count) {
        if (this.countDisplay) {
            const total = this.items.length;
            if (count === total) {
                this.countDisplay.textContent = `${total} items`;
            }
            else {
                this.countDisplay.textContent = `${count} of ${total} items`;
            }
        }
    }
}
