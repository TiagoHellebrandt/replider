import react from "eslint-plugin-react";
import tseslint from "typescript-eslint";
import globals from "globals";

// Flat ESLint config focused on enforcing safe external links.
// `react/jsx-no-target-blank` flags any `target="_blank"` anchor that is
// missing `rel="noopener noreferrer"` (reverse-tabnabbing / window.opener leak).
export default [
  {
    ignores: ["dist/**", "node_modules/**", ".astro/**", "**/*.astro"],
  },
  {
    files: ["src/**/*.{js,jsx,ts,tsx}"],
    plugins: { react },
    languageOptions: {
      parser: tseslint.parser,
      ecmaVersion: "latest",
      sourceType: "module",
      parserOptions: { ecmaFeatures: { jsx: true } },
      globals: { ...globals.browser },
    },
    settings: { react: { version: "detect" } },
    rules: {
      // The core deliverable of US-140: never allow an unguarded _blank link.
      "react/jsx-no-target-blank": [
        "error",
        { allowReferrer: false, enforceDynamicLinks: "always" },
      ],
    },
  },
];
