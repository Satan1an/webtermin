import { test, expect } from "@playwright/test";
import { ADMIN, OPERATOR, apiLogin } from "./helpers";

test.describe("package safety", () => {
  test("dry-run plan for installing htop lists it under install", async ({ request }) => {
    const headers = await apiLogin(request, ADMIN);
    const res = await request.post("/api/packages/plan", {
      headers,
      data: { op: "install", names: ["htop"] },
      timeout: 120_000, // may refresh stale apt lists first
    });
    expect(res.status(), await res.text()).toBe(200);
    const plan = await res.json();
    expect(plan.install).toContain("htop");
    expect(plan.remove ?? []).toEqual([]);
    expect(plan.blocked ?? []).toEqual([]);

    // A plan is only a dry run: htop is still not installed.
    const installed = await request.get("/api/packages/installed");
    expect(installed.status()).toBe(200);
    const names = ((await installed.json()) as { name: string }[]).map((p) => p.name);
    expect(names).toContain("openssh-server");
    expect(names).not.toContain("htop");
  });

  test("removing openssh-server is blocked even when confirmed", async ({ request }) => {
    const headers = await apiLogin(request, ADMIN);
    const res = await request.post("/api/packages/remove", {
      headers,
      data: { names: ["openssh-server"], confirm_remove: ["openssh-server"] },
      timeout: 120_000,
    });
    expect(res.status(), await res.text()).toBe(403);
    const body = await res.json();
    expect(body.error).toMatch(/blocked/i);
    expect(body.plan.blocked.map((b: { name: string }) => b.name)).toContain("openssh-server");
  });
});

test.describe("RBAC", () => {
  test("operator gets 403 on the file manager", async ({ request }) => {
    await apiLogin(request, OPERATOR);
    const me = await request.get("/api/auth/me");
    expect((await me.json()).role).toBe("operator");

    const res = await request.get("/api/files/list?path=/etc");
    expect(res.status()).toBe(403);
  });

  test("admin can list files (control for the operator check)", async ({ request }) => {
    await apiLogin(request, ADMIN);
    const res = await request.get("/api/files/list?path=/etc");
    expect(res.status(), await res.text()).toBe(200);
  });
});

test.describe("host status", () => {
  test("status reports reboot and updates; reboot needs admin and an exact confirm", async ({ request }) => {
    const admin = await apiLogin(request, ADMIN);
    const res = await request.get("/api/system/status");
    expect(res.status()).toBe(200);
    const st = await res.json();
    expect(typeof st.reboot.required).toBe("boolean");
    expect(st.reboot.running_kernel).toBeTruthy();
    expect(typeof st.updates.total).toBe("number");

    // Never send the real confirm here: it would reboot the stand mid-suite.
    const bad = await request.post("/api/system/reboot", { headers: admin, data: { confirm: "yes" } });
    expect(bad.status()).toBe(400);
  });

  test("operator cannot reboot", async ({ request }) => {
    const op = await apiLogin(request, OPERATOR);
    const res = await request.post("/api/system/reboot", { headers: op, data: { confirm: "reboot" } });
    expect(res.status()).toBe(403);
  });
});
