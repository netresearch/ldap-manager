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
    files: ["internal/web/templates/*.templ"],
    extract: {
      go: goExtractor("Classes")
    }
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
