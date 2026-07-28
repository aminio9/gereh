import js from "@eslint/js";
import globals from "globals";
import jsxA11y from "eslint-plugin-jsx-a11y";
import reactHooks from "eslint-plugin-react-hooks";
import reactRefresh from "eslint-plugin-react-refresh";
import tseslint from "typescript-eslint";

export default tseslint.config(
  {
    ignores: ["coverage/**", "dist/**", "node_modules/**"],
  },

  js.configs.recommended,

  {
    files: ["**/*.{ts,tsx}"],

    extends: [...tseslint.configs.recommendedTypeChecked, ...tseslint.configs.stylisticTypeChecked],

    languageOptions: {
      ecmaVersion: "latest",

      parserOptions: {
        projectService: true,
        tsconfigRootDir: import.meta.dirname,
      },
    },

    rules: {
      "@typescript-eslint/consistent-type-imports": [
        "error",
        {
          fixStyle: "inline-type-imports",
          prefer: "type-imports",
        },
      ],

      "@typescript-eslint/no-floating-promises": "error",
      "@typescript-eslint/no-misused-promises": "error",
      "@typescript-eslint/prefer-nullish-coalescing": "error",
      "@typescript-eslint/prefer-optional-chain": "error",
    },
  },

  {
    files: ["src/**/*.{ts,tsx}"],

    languageOptions: {
      globals: globals.browser,
    },

    plugins: {
      "react-hooks": reactHooks,
    },

    rules: {
      ...reactHooks.configs.recommended.rules,
    },
  },

  {
    files: ["src/**/*.tsx"],

    plugins: {
      "jsx-a11y": jsxA11y,
      "react-refresh": reactRefresh,
    },

    rules: {
      ...jsxA11y.configs.recommended.rules,

      "react-refresh/only-export-components": [
        "warn",
        {
          allowConstantExport: true,
        },
      ],
    },
  },

  {
    files: [
      "**/*.config.{js,mjs,cjs,ts}",
      "eslint.config.mjs",
      "prettier.config.mjs",
      "vite.config.ts",
      "vitest.config.ts",
    ],

    languageOptions: {
      globals: globals.node,
    },
  },

  {
    files: ["**/*.{js,mjs,cjs}"],

    extends: [tseslint.configs.disableTypeChecked],
  },
);
