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
        this.countDisplay = container.querySelector("[data-search-count]");
        if (!input || !listContainer) {
            this.input = null;
            this.listContainer = null;
            return;
        }
        this.input = input;
        this.listContainer = listContainer;
        this.items = Array.from(this.listContainer.querySelectorAll("[data-search-item]"));
        this.init();
    }
    init() {
        // Set up ARIA attributes
        this.input.setAttribute("role", "searchbox");
        this.input.setAttribute("aria-label", "Filter list");
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
