import { expect, type APIRequestContext, type Page } from "@playwright/test";

export const ADMIN = { username: "e2eadmin", password: "e2e-admin-Passw0rd!" };
export const OPERATOR = { username: "e2eoperator", password: "e2e-operator-Passw0rd!" };

// Name of the container run.sh starts inside the stand's own dockerd.
export const DOCKER_FIXTURE = process.env.WT_DOCKER_FIXTURE ?? "e2e-hello";

/** Logs in through the API and returns headers for mutating calls. */
export async function apiLogin(
  request: APIRequestContext,
  creds: { username: string; password: string },
): Promise<Record<string, string>> {
  const res = await request.post("/api/auth/login", { data: creds });
  expect(res.status(), await res.text()).toBe(200);
  const body = await res.json();
  expect(body.csrf_token).toBeTruthy();
  return { "X-CSRF-Token": body.csrf_token };
}

/** Logs in through the real login form. */
export async function uiLogin(page: Page, creds: { username: string; password: string }) {
  await page.goto("/login");
  await page.locator("#user").fill(creds.username);
  await page.locator("#pw").fill(creds.password);
  await page.locator('button[type="submit"]').click();
  await expect(page).toHaveURL(/\/dashboard$/);
}
