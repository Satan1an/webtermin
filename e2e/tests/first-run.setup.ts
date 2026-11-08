import { test, expect } from "@playwright/test";
import { ADMIN, OPERATOR, apiLogin } from "./helpers";

// The stand is fresh: no panel users yet. run.sh pulls the one-time token
// from the webtermin journal and passes it in.
const token = process.env.WT_SETUP_TOKEN ?? "";

test.describe.configure({ mode: "serial" });

test("stand starts unconfigured", async ({ request }) => {
  const res = await request.get("/api/auth/status");
  expect(res.status()).toBe(200);
  expect((await res.json()).needs_setup).toBe(true);
});

test("first-run setup rejects a wrong setup token with 403", async ({ request }) => {
  const res = await request.post("/api/auth/setup", {
    data: { ...ADMIN, setup_token: "definitely-not-the-token" },
  });
  expect(res.status()).toBe(403);
  expect((await res.json()).error).toMatch(/setup token/i);

  // ...and no setup token at all is rejected the same way.
  const empty = await request.post("/api/auth/setup", { data: { ...ADMIN } });
  expect(empty.status()).toBe(403);

  // Still unconfigured afterwards.
  const status = await request.get("/api/auth/status");
  expect((await status.json()).needs_setup).toBe(true);
});

test("first-run setup with the journal token creates the admin", async ({ request }) => {
  expect(token, "WT_SETUP_TOKEN must be set (run.sh reads it from journalctl)").not.toBe("");
  const res = await request.post("/api/auth/setup", { data: { ...ADMIN, setup_token: token } });
  expect(res.status(), await res.text()).toBe(200);

  // The token is single-use: a second setup is refused.
  const again = await request.post("/api/auth/setup", { data: { ...ADMIN, setup_token: token } });
  expect(again.status()).toBe(409);
});

test("admin creates an operator panel user", async ({ request }) => {
  const headers = await apiLogin(request, ADMIN);
  const res = await request.post("/api/panel/users", {
    headers,
    data: { ...OPERATOR, role: "operator" },
  });
  expect(res.status(), await res.text()).toBeLessThan(300);
});
