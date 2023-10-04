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
    files: ["internal/web/layouts/*.html", "internal/web/views/*.html", "internal/web/templates.go"],
    extract: {
      go: goExtractor("Classes")
    }
  },
  plugins: [
    plugin(({ addVariant }) => {
      addVariant("hocus", ["&:hover", "&:focus"]);
    })
  ],
  theme: {
    extend: {}
  }
};

export default config;
