import plugin from "tailwindcss/plugin";

const goExtractor = (postfix = "Classes") => {
  const regex = new RegExp(`^const .+${postfix} = "(.+)";?$`, "g");

  /**
   * @param {string} content
   * @returns {[]string}
   */
  return (content) => {
    const matches = regex.exec(content);
    if (!matches) return [];

    const rawClasses = matches[1];
    console.log(rawClasses);

    return rawClasses.split(" ");
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
  },
  safelist: ["bg-gray-700", "hocus:bg-gray-800"]
};

export default config;
