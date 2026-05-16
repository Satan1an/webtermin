import { create } from "zustand";

export type Role = "viewer" | "operator" | "admin";

interface Me {
  user: string;
  is_admin: boolean;
  role: Role;
  csrf_token: string;
  has_2fa: boolean;
}

interface AuthState {
  user: string | null;
  isAdmin: boolean;
  role: Role | null;
  csrfToken: string | null;
  has2FA: boolean;
  needsSetup: boolean | null;
  set: (me: Me) => void;
  setNeedsSetup: (b: boolean) => void;
  clear: () => void;
}

const roleRank: Record<Role, number> = { viewer: 1, operator: 2, admin: 3 };

export const useAuth = create<AuthState>((set) => ({
  user: null,
  isAdmin: false,
  role: null,
  csrfToken: null,
  has2FA: false,
  needsSetup: null,
  set: (me) =>
    set({
      user: me.user,
      isAdmin: me.is_admin,
      role: me.role,
      csrfToken: me.csrf_token,
      has2FA: me.has_2fa,
      needsSetup: false,
    }),
  setNeedsSetup: (b) => set({ needsSetup: b }),
  clear: () =>
    set({ user: null, isAdmin: false, role: null, csrfToken: null, has2FA: false }),
}));

/** True if the current user's role is `min` or higher. */
export function useAtLeast(min: Role): boolean {
  const role = useAuth((s) => s.role);
  if (!role) return false;
  return roleRank[role] >= roleRank[min];
}

export function useCanWrite(): boolean {
  return useAtLeast("operator");
}

export function useIsAdmin(): boolean {
  return useAtLeast("admin");
}
