import { create } from "zustand";

interface Me {
  user: string;
  is_admin: boolean;
  csrf_token: string;
  has_2fa: boolean;
}

interface AuthState {
  user: string | null;
  isAdmin: boolean;
  csrfToken: string | null;
  has2FA: boolean;
  needsSetup: boolean | null;
  set: (me: Me) => void;
  setNeedsSetup: (b: boolean) => void;
  clear: () => void;
}

export const useAuth = create<AuthState>((set) => ({
  user: null,
  isAdmin: false,
  csrfToken: null,
  has2FA: false,
  needsSetup: null,
  set: (me) =>
    set({
      user: me.user,
      isAdmin: me.is_admin,
      csrfToken: me.csrf_token,
      has2FA: me.has_2fa,
      needsSetup: false,
    }),
  setNeedsSetup: (b) => set({ needsSetup: b }),
  clear: () =>
    set({ user: null, isAdmin: false, csrfToken: null, has2FA: false }),
}));
