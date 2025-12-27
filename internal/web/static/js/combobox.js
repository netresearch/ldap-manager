/**
 * Accessible searchable combobox component.
 * WCAG 2.2 AAA compliant with full keyboard navigation.
 */
/**
 * Initialize all combobox elements on the page.
 */
export function initComboboxes() {
    document.querySelectorAll("[data-combobox]").forEach((container) => {
        new Combobox(container);
    });
}
/**
 * Combobox class for searchable dropdown functionality.
 */
class Combobox {
    constructor(container) {
        this.container = container;
        this.input = container.querySelector("[data-combobox-input]");
        this.hiddenInput = container.querySelector("[data-combobox-value]");
        this.listbox = container.querySelector("[data-combobox-listbox]");
        this.options = Array.from(this.listbox.querySelectorAll("[data-combobox-option]"));
        this.activeIndex = -1;
        this.isOpen = false;
        // Store original options for filtering
        this.allOptions = this.options.map((opt) => ({
            element: opt,
            value: opt.dataset["value"] || "",
            label: opt.textContent?.trim() || "",
            searchText: (opt.textContent?.trim() || "").toLowerCase(),
        }));
        this.init();
    }
    init() {
        // Set up ARIA attributes
        this.input.setAttribute("role", "combobox");
        this.input.setAttribute("aria-autocomplete", "list");
        this.input.setAttribute("aria-expanded", "false");
        this.input.setAttribute("aria-controls", this.listbox.id);
        this.listbox.setAttribute("role", "listbox");
        // Event listeners
        this.input.addEventListener("input", this.handleInput.bind(this));
        this.input.addEventListener("keydown", this.handleKeydown.bind(this));
        this.input.addEventListener("focus", this.handleFocus.bind(this));
        this.input.addEventListener("blur", this.handleBlur.bind(this));
        this.listbox.addEventListener("mousedown", this.handleOptionClick.bind(this));
        // Initially hide listbox
        this.close();
    }
    handleInput(event) {
        const target = event.target;
        const query = target.value.toLowerCase();
        this.filter(query);
        this.open();
    }
    handleFocus() {
        if (this.input.value === "" || this.getVisibleOptions().length > 0) {
            this.open();
        }
    }
    handleBlur() {
        // Delay close to allow click events on options
        setTimeout(() => {
            this.close();
        }, 150);
    }
    handleKeydown(event) {
        const visibleOptions = this.getVisibleOptions();
        switch (event.key) {
            case "ArrowDown":
                event.preventDefault();
                if (!this.isOpen) {
                    this.open();
                }
                else {
                    this.moveActive(1);
                }
                break;
            case "ArrowUp":
                event.preventDefault();
                if (this.isOpen) {
                    this.moveActive(-1);
                }
                break;
            case "Enter":
                event.preventDefault();
                if (this.isOpen && this.activeIndex >= 0) {
                    this.selectOption(visibleOptions[this.activeIndex]);
                }
                break;
            case "Escape":
                event.preventDefault();
                this.close();
                this.input.value = "";
                this.hiddenInput.value = "";
                break;
            case "Tab":
                this.close();
                break;
        }
    }
    handleOptionClick(event) {
        const target = event.target;
        const option = target.closest("[data-combobox-option]");
        if (option && !option.classList.contains("hidden")) {
            event.preventDefault();
            this.selectOption(option);
        }
    }
    filter(query) {
        this.activeIndex = -1;
        let hasVisible = false;
        this.allOptions.forEach(({ element, searchText }) => {
            const matches = query === "" || searchText.includes(query);
            element.classList.toggle("hidden", !matches);
            if (matches) {
                hasVisible = true;
            }
        });
        // Update active descendant
        this.updateActiveDescendant();
        // Select first visible if there's a query
        if (hasVisible && query !== "") {
            this.activeIndex = 0;
            this.updateActiveDescendant();
        }
    }
    getVisibleOptions() {
        return this.options.filter((opt) => !opt.classList.contains("hidden"));
    }
    moveActive(delta) {
        const visibleOptions = this.getVisibleOptions();
        if (visibleOptions.length === 0)
            return;
        this.activeIndex += delta;
        if (this.activeIndex < 0) {
            this.activeIndex = visibleOptions.length - 1;
        }
        else if (this.activeIndex >= visibleOptions.length) {
            this.activeIndex = 0;
        }
        this.updateActiveDescendant();
        visibleOptions[this.activeIndex]?.scrollIntoView({ block: "nearest" });
    }
    updateActiveDescendant() {
        const visibleOptions = this.getVisibleOptions();
        // Remove active state from all options
        this.options.forEach((opt) => {
            opt.classList.remove("bg-surface-hover");
            opt.removeAttribute("aria-selected");
        });
        // Add active state to current option
        if (this.activeIndex >= 0 && visibleOptions[this.activeIndex]) {
            const activeOption = visibleOptions[this.activeIndex];
            activeOption.classList.add("bg-surface-hover");
            activeOption.setAttribute("aria-selected", "true");
            this.input.setAttribute("aria-activedescendant", activeOption.id);
        }
        else {
            this.input.removeAttribute("aria-activedescendant");
        }
    }
    selectOption(option) {
        const value = option.dataset["value"] || "";
        const label = option.textContent?.trim() || "";
        this.input.value = label;
        this.hiddenInput.value = value;
        this.close();
        // Dispatch change event for form validation
        this.hiddenInput.dispatchEvent(new Event("change", { bubbles: true }));
    }
    open() {
        if (this.getVisibleOptions().length === 0)
            return;
        this.isOpen = true;
        this.listbox.classList.remove("hidden");
        this.input.setAttribute("aria-expanded", "true");
    }
    close() {
        this.isOpen = false;
        this.activeIndex = -1;
        this.listbox.classList.add("hidden");
        this.input.setAttribute("aria-expanded", "false");
        this.updateActiveDescendant();
    }
}
