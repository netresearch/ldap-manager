/**
 * Copy-to-clipboard functionality for copyable text elements.
 * Uses the Clipboard API with visual feedback.
 */

/**
 * Initialize all copyable elements on the page.
 */
export function initCopyButtons() {
    const copyables = document.querySelectorAll("[data-copyable]");
    copyables.forEach(initCopyable);
}

/**
 * Initialize a single copyable element.
 * @param {HTMLElement} element - The copyable container element
 */
function initCopyable(element) {
    const button = element.querySelector("[data-copy-button]");
    const textElement = element.querySelector("[data-copy-text]");
    const copyIcon = element.querySelector("[data-copy-icon]");
    const checkIcon = element.querySelector("[data-check-icon]");

    if (!button || !textElement) return;

    button.addEventListener("click", async () => {
        const text = textElement.textContent?.trim() || "";

        try {
            await navigator.clipboard.writeText(text);

            // Show success feedback
            if (copyIcon && checkIcon) {
                copyIcon.classList.add("hidden");
                checkIcon.classList.remove("hidden");

                // Reset after 2 seconds
                setTimeout(() => {
                    copyIcon.classList.remove("hidden");
                    checkIcon.classList.add("hidden");
                }, 2000);
            }
        } catch (err) {
            console.error("Failed to copy text:", err);
            // Fallback for older browsers
            fallbackCopy(text);
        }
    });
}

/**
 * Fallback copy method for browsers without Clipboard API.
 * @param {string} text - The text to copy
 */
function fallbackCopy(text) {
    const textarea = document.createElement("textarea");
    textarea.value = text;
    textarea.style.position = "fixed";
    textarea.style.opacity = "0";
    document.body.appendChild(textarea);
    textarea.select();
    try {
        document.execCommand("copy");
    } catch (err) {
        console.error("Fallback copy failed:", err);
    }
    document.body.removeChild(textarea);
}
