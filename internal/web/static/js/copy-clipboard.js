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
    // Prevent duplicate initialization
    if (element.hasAttribute("data-copy-initialized")) {
        return;
    }
    element.setAttribute("data-copy-initialized", "true");

    const button = element.querySelector("[data-copy-button]");
    const textElement = element.querySelector("[data-copy-text]");
    const copyIcon = element.querySelector("[data-copy-icon]");
    const checkIcon = element.querySelector("[data-check-icon]");

    if (!button || !textElement) return;

    // Store timeout ID for cleanup on rapid clicks
    let resetTimeout = null;

    button.addEventListener("click", async () => {
        const text = textElement.textContent?.trim() || "";

        // Clear any pending timeout from previous click
        if (resetTimeout) {
            clearTimeout(resetTimeout);
            resetTimeout = null;
        }

        try {
            await navigator.clipboard.writeText(text);
            showSuccessFeedback(copyIcon, checkIcon, (timeoutId) => {
                resetTimeout = timeoutId;
            });
        } catch (err) {
            console.error("Failed to copy text:", err);
            // Fallback for older browsers
            const success = fallbackCopy(text);
            if (success) {
                showSuccessFeedback(copyIcon, checkIcon, (timeoutId) => {
                    resetTimeout = timeoutId;
                });
            }
        }
    });
}

/**
 * Show success feedback by toggling icons.
 * @param {HTMLElement|null} copyIcon - The copy icon element
 * @param {HTMLElement|null} checkIcon - The check icon element
 * @param {function} onTimeout - Callback to store the timeout ID
 */
function showSuccessFeedback(copyIcon, checkIcon, onTimeout) {
    if (copyIcon && checkIcon) {
        copyIcon.classList.add("hidden");
        checkIcon.classList.remove("hidden");

        // Reset after 2 seconds
        const timeoutId = setTimeout(() => {
            copyIcon.classList.remove("hidden");
            checkIcon.classList.add("hidden");
        }, 2000);

        if (onTimeout) {
            onTimeout(timeoutId);
        }
    }
}

/**
 * Fallback copy method for browsers without Clipboard API.
 * @param {string} text - The text to copy
 * @returns {boolean} - Whether the copy was successful
 */
function fallbackCopy(text) {
    const textarea = document.createElement("textarea");
    textarea.value = text;
    textarea.style.position = "fixed";
    textarea.style.opacity = "0";
    document.body.appendChild(textarea);
    textarea.select();
    let success = false;
    try {
        success = document.execCommand("copy");
    } catch (err) {
        console.error("Fallback copy failed:", err);
    }
    document.body.removeChild(textarea);
    return success;
}
