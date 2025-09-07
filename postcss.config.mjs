const isProduction = process.env.NODE_ENV === "production";

const basePlugins = {
  "@tailwindcss/postcss": {},
  autoprefixer: {}
};

const productionPlugins = {
  cssnano: {
    preset: [
      "advanced",
      {
        // Aggressive optimizations for production
        reduceIdents: false, // Keep CSS custom properties intact
        zindex: false, // Don't optimize z-index values
        discardComments: { removeAll: true },
        normalizeWhitespace: true,
        colormin: true,
        mergeRules: true,
        mergeLonghand: true,
        minifyFontValues: true,
        minifyGradients: true,
        minifyParams: true,
        minifySelectors: true,
        normalizeCharset: true,
        normalizeDisplayValues: true,
        normalizePositions: true,
        normalizeRepeatStyle: true,
        normalizeString: true,
        normalizeTimingFunctions: true,
        normalizeUnicode: true,
        normalizeUrl: true,
        orderedValues: true,
        reduceInitial: true,
        reduceTransforms: true,
        svgo: {
          plugins: [
            { name: "removeViewBox", active: false },
            { name: "removeDimensions", active: true }
          ]
        },
        calc: { precision: 5 }
      }
    ]
  },
  "postcss-reporter": {
    clearReportedMessages: true,
    throwError: false
  }
};

const developmentPlugins = {
  "postcss-reporter": {
    clearReportedMessages: true,
    throwError: false
  }
};

export default {
  plugins: {
    ...basePlugins,
    ...(isProduction ? productionPlugins : developmentPlugins)
  }
};
