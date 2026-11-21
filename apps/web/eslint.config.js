import js from "@eslint/js";
import reactHooks from "eslint-plugin-react-hooks";
import reactRefresh from "eslint-plugin-react-refresh";
import globals from "globals";
import tseslint from "typescript-eslint";

export default tseslint.config(
  {
    ignores: [
      "out",
      "playwright-report",
      "test-results",
      "src/gen/proto/**",
      "src/client/routeTree.gen.ts",
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
      "@typescript-eslint/no-unused-vars": ["warn", { "argsIgnorePattern": "^_" }],
      "react-refresh/only-export-components": [
        "warn",
        { allowConstantExport: true, allowExportNames: ["ROOT_ID", "STATIC_PAPERS"] }
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
  // アーキテクチャ境界の強制: src/shared は client, features, components に依存しない
  {
    files: ["src/shared/**/*.{ts,tsx}"],
    rules: {
      "no-restricted-imports": [
        "error",
        {
          patterns: [
            {
              group: ["@/client", "@/client/*", "@/features", "@/features/*", "@/components", "@/components/*"],
              message: "src/shared は純粋な基盤モジュールであり、UIや特定のfeatureに依存してはならない。"
            }
          ]
        }
      ]
    }
  },
  // アーキテクチャ境界の強制: src/features は client (ルーティング具象) に依存しない
  {
    files: ["src/features/**/*.{ts,tsx}"],
    rules: {
      "no-restricted-imports": [
        "error",
        {
          patterns: [
            {
              group: ["@/client", "@/client/*"],
              message: "src/features はドメイン機能モジュールであり、client (ルーティング具象) に依存してはならない。"
            }
          ]
        }
      ]
    }
  },
  // アーキテクチャ境界の強制: src/components (共通UI) は個別 features に依存しない
  {
    files: ["src/components/**/*.{ts,tsx}"],
    rules: {
      "no-restricted-imports": [
        "error",
        {
          patterns: [
            {
              group: ["@/features", "@/features/*"],
              message: "src/components は共通UIモジュールであり、個別 feature に依存してはならない。"
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
