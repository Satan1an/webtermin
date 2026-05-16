// Minimal API client. CSRF token is read from the auth store and attached to
// every mutating request.

import { useAuth } from "@/store/auth";

const BASE = "";

class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message);
  }
}

async function request<T = unknown>(
  method: string,
  path: string,
  body?: unknown,
): Promise<T> {
  const headers: Record<string, string> = {};
  if (body !== undefined) headers["Content-Type"] = "application/json";
  if (method !== "GET" && method !== "HEAD") {
    const csrf = useAuth.getState().csrfToken;
    if (csrf) headers["X-CSRF-Token"] = csrf;
  }
  const res = await fetch(BASE + path, {
    method,
    headers,
    credentials: "same-origin",
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
  const ct = res.headers.get("content-type") ?? "";
  const isJson = ct.includes("application/json");
  if (!res.ok) {
    let msg = res.statusText;
    if (isJson) {
      try {
        const j = (await res.json()) as { error?: string };
        if (j?.error) msg = j.error;
      } catch {}
    }
    if (res.status === 401) {
      useAuth.getState().clear();
    }
    throw new ApiError(res.status, msg);
  }
  if (res.status === 204) return undefined as T;
  return isJson ? ((await res.json()) as T) : ((await res.text()) as T);
}

export const api = {
  get: <T = unknown>(p: string) => request<T>("GET", p),
  post: <T = unknown>(p: string, body?: unknown) => request<T>("POST", p, body),
  put: <T = unknown>(p: string, body?: unknown) => request<T>("PUT", p, body),
  del: <T = unknown>(p: string) => request<T>("DELETE", p),
};

export { ApiError };

export function wsURL(path: string): string {
  const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
  return `${proto}//${window.location.host}${path}`;
}
