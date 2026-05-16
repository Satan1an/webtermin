import { useEffect, useState } from "react";
import { Navigate, Route, Routes, useLocation, useNavigate } from "react-router-dom";
import { api } from "@/lib/api";
import { useAuth, type Role } from "@/store/auth";
import { AppLayout } from "@/components/layout/AppLayout";
import { LoginPage } from "@/pages/LoginPage";
import { SetupPage } from "@/pages/SetupPage";
import { DashboardPage } from "@/pages/DashboardPage";
import { ServicesPage } from "@/pages/ServicesPage";
import { UsersPage } from "@/pages/UsersPage";
import { FilesPage } from "@/pages/FilesPage";
import { TerminalPage } from "@/pages/TerminalPage";
import { AuditPage } from "@/pages/AuditPage";
import { PanelUsersPage } from "@/pages/PanelUsersPage";
import { Toaster } from "@/components/ui/toast";
import { motion } from "framer-motion";

export default function App() {
  const auth = useAuth();
  const [bootstrapped, setBootstrapped] = useState(false);

  useEffect(() => {
    void (async () => {
      try {
        const status = await api.get<{
          needs_setup: boolean;
          user?: { user: string; is_admin: boolean; role: Role; csrf_token: string; has_2fa: boolean };
        }>("/api/auth/status");
        auth.setNeedsSetup(status.needs_setup);
        if (status.user) auth.set(status.user);
      } catch {
        // ignore
      } finally {
        setBootstrapped(true);
      }
    })();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  if (!bootstrapped) {
    return <BootSplash />;
  }

  return (
    <>
      <Routes>
        {auth.needsSetup ? (
          <>
            <Route path="/setup" element={<SetupPage />} />
            <Route path="*" element={<Navigate to="/setup" replace />} />
          </>
        ) : auth.user ? (
          <Route element={<AppLayout />}>
            <Route path="/" element={<Navigate to="/dashboard" replace />} />
            <Route path="/dashboard" element={<DashboardPage />} />
            <Route path="/services" element={<ServicesPage />} />
            <Route path="/users" element={<UsersPage />} />
            <Route path="/files" element={<FilesPage />} />
            <Route path="/terminal" element={<TerminalPage />} />
            <Route path="/audit" element={<AuditPage />} />
            <Route path="/panel-users" element={<PanelUsersPage />} />
            <Route path="*" element={<NavigateAfterLogin />} />
          </Route>
        ) : (
          <>
            <Route path="/login" element={<LoginPage />} />
            <Route path="*" element={<Navigate to="/login" replace />} />
          </>
        )}
      </Routes>
      <Toaster />
    </>
  );
}

function NavigateAfterLogin() {
  const loc = useLocation();
  const nav = useNavigate();
  useEffect(() => {
    nav("/dashboard", { replace: true });
  }, [loc, nav]);
  return null;
}

function BootSplash() {
  return (
    <div className="grid h-full w-full place-items-center bg-background">
      <motion.div
        initial={{ opacity: 0, y: 6 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.3 }}
        className="flex items-center gap-3 text-muted-foreground"
      >
        <div className="h-2 w-2 animate-pulse rounded-full bg-primary" />
        <span className="text-sm tracking-wide">starting webtermin…</span>
      </motion.div>
    </div>
  );
}
