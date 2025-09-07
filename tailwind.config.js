import forms from "@tailwindcss/forms";
import plugin from "tailwindcss/plugin";

const goExtractor = (postfix = "Classes") => {
  /**
   * @param {string} content
   * @returns {[]string}
   */
  return (content) => {
    const regex = new RegExp(`^const .*${postfix} = "(.+)";?`, "gm");

    const matches = regex.exec(content);
    if (!matches) return [];

    return matches[1].split(" ");
  };
};

/** @type {import('tailwindcss').Config} */
const config = {
  content: {
    files: [
      "internal/web/templates/*.templ",
      "internal/web/templates/**/*.templ",
      "internal/web/static/**/*.{js,ts}",
      "**/*.go"
    ],
    extract: {
      go: goExtractor("Classes"),
      templ: (content) => {
        // Enhanced extraction for templ files with class attributes
        const classRegex = /class="([^"]*)"/g;
        const classes = [];
        let match;
        while ((match = classRegex.exec(content)) !== null) {
          classes.push(...match[1].split(/\s+/).filter(Boolean));
        }
        return classes;
      }
    },
    // Dynamic class detection patterns
    safelist: [
      // Preserve dynamic classes that might not be detected
      "bg-red-500",
      "bg-green-500",
      "bg-blue-500",
      "border-red-500",
      "border-green-500",
      "border-blue-500",
      "text-red-500",
      "text-green-500",
      "text-blue-500"
    ]
  },
  plugins: [
    forms({ strategy: "class" }),
    plugin(({ addVariant }) => {
      addVariant("hocus", ["&:hover", "&:focus"]);
      addVariant("list-outer-hocus", ["&:has(a:focus)", "&:has(a:hover)"]);
    })
  ],
  theme: {
    extend: {}
  }
};

export default config;
