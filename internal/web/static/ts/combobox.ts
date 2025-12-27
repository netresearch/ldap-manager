/**
 * Accessible searchable combobox component.
 * WCAG 2.2 AAA compliant with full keyboard navigation.
 */

interface ComboboxOption {
  element: HTMLElement;
  value: string;
  label: string;
  searchText: string;
}

/**
 * Initialize all combobox elements on the page.
 */
export function initComboboxes(): void {
  document.querySelectorAll<HTMLElement>("[data-combobox]").forEach((container) => {
    new Combobox(container);
  });
}

/**
 * Combobox class for searchable dropdown functionality.
 */
class Combobox {
  private container: HTMLElement;
  private input: HTMLInputElement;
  private hiddenInput: HTMLInputElement;
  private listbox: HTMLElement;
  private options: HTMLElement[];
  private allOptions: ComboboxOption[];
  private activeIndex: number;
  private isOpen: boolean;

  constructor(container: HTMLElement) {
    this.container = container;
    this.input = container.querySelector("[data-combobox-input]") as HTMLInputElement;
    this.hiddenInput = container.querySelector("[data-combobox-value]") as HTMLInputElement;
    this.listbox = container.querySelector("[data-combobox-listbox]") as HTMLElement;
    this.options = Array.from(this.listbox.querySelectorAll<HTMLElement>("[data-combobox-option]"));
    this.activeIndex = -1;
    this.isOpen = false;

    // Store original options for filtering
    this.allOptions = this.options.map((opt) => ({
      element: opt,
      value: opt.dataset["value"] || "",
      label: opt.textContent?.trim() || "",
      searchText: (opt.textContent?.trim() || "").toLowerCase()
    }));

    this.init();
  }

  private init(): void {
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

  private handleInput(event: Event): void {
    const target = event.target as HTMLInputElement;
    const query = target.value.toLowerCase();
    this.filter(query);
    this.open();
  }

  private handleFocus(): void {
    if (this.input.value === "" || this.getVisibleOptions().length > 0) {
      this.open();
    }
  }

  private handleBlur(): void {
    // Delay close to allow click events on options
    setTimeout(() => {
      this.close();
    }, 150);
  }

  private handleKeydown(event: KeyboardEvent): void {
    const visibleOptions = this.getVisibleOptions();

    switch (event.key) {
      case "ArrowDown":
        event.preventDefault();
        if (!this.isOpen) {
          this.open();
        } else {
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

  private handleOptionClick(event: MouseEvent): void {
    const target = event.target as HTMLElement;
    const option = target.closest<HTMLElement>("[data-combobox-option]");
    if (option && !option.classList.contains("hidden")) {
      event.preventDefault();
      this.selectOption(option);
    }
  }

  private filter(query: string): void {
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

  private getVisibleOptions(): HTMLElement[] {
    return this.options.filter((opt) => !opt.classList.contains("hidden"));
  }

  private moveActive(delta: number): void {
    const visibleOptions = this.getVisibleOptions();
    if (visibleOptions.length === 0) return;

    this.activeIndex += delta;

    if (this.activeIndex < 0) {
      this.activeIndex = visibleOptions.length - 1;
    } else if (this.activeIndex >= visibleOptions.length) {
      this.activeIndex = 0;
    }

    this.updateActiveDescendant();
    visibleOptions[this.activeIndex]?.scrollIntoView({ block: "nearest" });
  }

  private updateActiveDescendant(): void {
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
    } else {
      this.input.removeAttribute("aria-activedescendant");
    }
  }

  private selectOption(option: HTMLElement): void {
    const value = option.dataset["value"] || "";
    const label = option.textContent?.trim() || "";

    this.input.value = label;
    this.hiddenInput.value = value;
    this.close();

    // Dispatch change event for form validation
    this.hiddenInput.dispatchEvent(new Event("change", { bubbles: true }));
  }

  private open(): void {
    if (this.getVisibleOptions().length === 0) return;

    this.isOpen = true;
    this.listbox.classList.remove("hidden");
    this.input.setAttribute("aria-expanded", "true");
  }

  private close(): void {
    this.isOpen = false;
    this.activeIndex = -1;
    this.listbox.classList.add("hidden");
    this.input.setAttribute("aria-expanded", "false");
    this.updateActiveDescendant();
  }
}
