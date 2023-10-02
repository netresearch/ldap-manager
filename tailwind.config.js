import plugin from "tailwindcss/plugin";

/** @type {import('tailwindcss').Config} */
const config = {
  content: ["internal/web/layouts/*.html", "internal/web/views/*.html"],
  plugins: [
    plugin(({ addVariant }) => {
      addVariant("hocus", ["&:hover", "&:focus"]);
    })
  ],
  theme: {
    extend: {}
  },
  safelist: ["bg-gray-700"]
};

export default config;
