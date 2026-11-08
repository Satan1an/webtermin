import { test, expect } from "@playwright/test";
import { ADMIN, DOCKER_FIXTURE, uiLogin } from "./helpers";

test.beforeEach(async ({ page }) => {
  await uiLogin(page, ADMIN);
});

test("login lands on a rendered dashboard", async ({ page }) => {
  await expect(page.getByRole("heading", { name: "Dashboard" })).toBeVisible();
  // Metric cards are filled from /api/system/metrics.
  await expect(page.getByText("CPU", { exact: true }).first()).toBeVisible();
  await expect(page.getByText("Memory", { exact: true }).first()).toBeVisible();
});

test("services page lists webtermin.service", async ({ page }) => {
  await page.goto("/services");
  await expect(page.getByRole("heading", { name: "Services" })).toBeVisible();
  await page.getByPlaceholder("Filter units…").fill("webtermin");
  await expect(page.getByText("webtermin.service", { exact: true })).toBeVisible();
});

test("docker page lists the container started inside the stand", async ({ page }) => {
  await page.goto("/docker");
  await expect(page.getByRole("heading", { name: "Docker" })).toBeVisible();
  await expect(page.getByText(DOCKER_FIXTURE, { exact: true }).first()).toBeVisible();
});

test("stack editor (Monaco) loads from the panel itself, with no CSP violations", async ({ page }) => {
  // Regression: Monaco used to come from a CDN that the panel's CSP blocks,
  // so the editor never rendered.
  const errors: string[] = [];
  page.on("console", (m) => m.type() === "error" && errors.push(m.text()));
  await page.goto("/stacks");
  await page.getByRole("button", { name: /Deploy stack/ }).click();
  await expect(page.locator(".monaco-editor").first()).toBeVisible({ timeout: 20_000 });
  expect(errors.filter((e) => e.includes("Content Security Policy"))).toEqual([]);
});

test("network page falls back to a read-only view without NetworkManager", async ({ page }) => {
  await page.goto("/network");
  await expect(page.getByText(/shown read-only from/)).toBeVisible();
  await expect(page.getByRole("button", { name: /Install network-manager/ })).toBeVisible();
});
