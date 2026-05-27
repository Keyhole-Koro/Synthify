import js from "@eslint/js";
import reactHooks from "eslint-plugin-react-hooks";
import reactRefresh from "eslint-plugin-react-refresh";
import globals from "globals";
import tseslint from "typescript-eslint";

export default tseslint.config(
  {
    ignores: [
      ".next",
      "out",
      "playwright-report",
      "test-results",
      "src/gen/proto/**",
      "vender/**"
    ]
  },
  {
    files: ["**/*.{ts,tsx}"],
    extends: [js.configs.recommended, ...tseslint.configs.recommended],
    languageOptions: {
      ecmaVersion: 2023,
      globals: {
        ...globals.browser,
        ...globals.node
      }
    },
    plugins: {
      "react-hooks": reactHooks,
      "react-refresh": reactRefresh
    },
    rules: {
      ...reactHooks.configs.recommended.rules,
      "react-refresh/only-export-components": [
        "warn",
        { allowConstantExport: true, allowExportNames: ["metadata", "ROOT_ID", "STATIC_PAPERS"] }
      ],
      "no-restricted-imports": [
        "error",
        {
          paths: [
            {
              name: "firebase/firestore",
              importNames: ["onSnapshot"],
              message: "Use useAuthedDoc / useAuthedCollection from @/lib/firestore so listeners are gated on Firebase Auth."
            }
          ]
        }
      ]
    }
  },
  {
    files: ["src/lib/firestore/**/*.{ts,tsx}"],
    rules: {
      "no-restricted-imports": "off"
    }
  }
);
