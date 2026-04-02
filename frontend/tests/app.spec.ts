import { expect, test } from "@playwright/test";

test("renders starter headline", async ({ page }) => {
  await page.goto("/");

  await expect(
    page.getByRole("heading", {
      name: "Next.js, Tailwind, and Playwright are ready to ship."
    })
  ).toBeVisible();
});
