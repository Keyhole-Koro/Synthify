import type { InsightifyDesktopApi } from "@insightify/desktop-bridge";

declare global {
  interface Window {
    insightify: InsightifyDesktopApi;
  }
}

export {};
