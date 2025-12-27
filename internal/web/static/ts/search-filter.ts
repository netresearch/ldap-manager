/**
 * Client-side search filter for list pages.
 * Filters list items based on text content without server requests.
 */

export function initSearchFilters(): void {
  document.querySelectorAll<HTMLElement>("[data-search-filter]").forEach((container) => {
    new SearchFilter(container);
  });
}

class SearchFilter {
  private input: HTMLInputElement;
  private listContainer: HTMLElement;
  private items: HTMLElement[] = [];
  private countDisplay: HTMLElement | null;

  constructor(container: HTMLElement) {
    const input = container.querySelector("[data-search-input]") as HTMLInputElement | null;
    const listContainer = container.querySelector("[data-search-list]") as HTMLElement | null;
    this.countDisplay = container.querySelector("[data-search-count]");

    if (!input || !listContainer) {
      this.input = null as unknown as HTMLInputElement;
      this.listContainer = null as unknown as HTMLElement;
      return;
    }

    this.input = input;
    this.listContainer = listContainer;
    this.items = Array.from(this.listContainer.querySelectorAll<HTMLElement>("[data-search-item]"));

    this.init();
  }

  private init(): void {
    // Set up ARIA attributes
    this.input.setAttribute("role", "searchbox");
    this.input.setAttribute("aria-label", "Filter list");

    // Event listener with debounce for performance
    let debounceTimer: ReturnType<typeof setTimeout>;
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

  private filter(): void {
    const query = this.input.value.toLowerCase().trim();
    let visibleCount = 0;

    this.items.forEach((item) => {
      const text = (item.textContent || "").toLowerCase();
      const matches = query === "" || text.includes(query);

      item.classList.toggle("hidden", !matches);
      if (matches) visibleCount++;
    });

    this.updateCount(visibleCount);
  }

  private updateCount(count: number): void {
    if (this.countDisplay) {
      const total = this.items.length;
      if (count === total) {
        this.countDisplay.textContent = `${total} items`;
      } else {
        this.countDisplay.textContent = `${count} of ${total} items`;
      }
    }
  }
}
