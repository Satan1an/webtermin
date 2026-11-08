import { lazy, useEffect, useState } from "react";
import { Navigate, Route, Routes, useLocation, useNavigate } from "react-router";
import { api } from "@/lib/api";
import { useAuth, type Role } from "@/store/auth";
import { AppLayout } from "@/components/layout/AppLayout";
import { LoginPage } from "@/pages/LoginPage";
import { SetupPage } from "@/pages/SetupPage";
import { Toaster } from "@/components/ui/toast";
import { motion } from "framer-motion";

// Route pages are code-split so heavy deps (Monaco, xterm, recharts) load
// only when the page that needs them is opened.
const DashboardPage = lazy(() => import("@/pages/DashboardPage").then((m) => ({ default: m.DashboardPage })));
const ServicesPage = lazy(() => import("@/pages/ServicesPage").then((m) => ({ default: m.ServicesPage })));
const UsersPage = lazy(() => import("@/pages/UsersPage").then((m) => ({ default: m.UsersPage })));
const FilesPage = lazy(() => import("@/pages/FilesPage").then((m) => ({ default: m.FilesPage })));
const TerminalPage = lazy(() => import("@/pages/TerminalPage").then((m) => ({ default: m.TerminalPage })));
const AuditPage = lazy(() => import("@/pages/AuditPage").then((m) => ({ default: m.AuditPage })));
const PanelUsersPage = lazy(() => import("@/pages/PanelUsersPage").then((m) => ({ default: m.PanelUsersPage })));
const ApiTokensPage = lazy(() => import("@/pages/ApiTokensPage").then((m) => ({ default: m.ApiTokensPage })));
const CronPage = lazy(() => import("@/pages/CronPage").then((m) => ({ default: m.CronPage })));
const FirewallPage = lazy(() => import("@/pages/FirewallPage").then((m) => ({ default: m.FirewallPage })));
const DockerPage = lazy(() => import("@/pages/DockerPage").then((m) => ({ default: m.DockerPage })));
const StacksPage = lazy(() => import("@/pages/StacksPage").then((m) => ({ default: m.StacksPage })));
const BackupPage = lazy(() => import("@/pages/BackupPage").then((m) => ({ default: m.BackupPage })));
const PackagesPage = lazy(() => import("@/pages/PackagesPage").then((m) => ({ default: m.PackagesPage })));
const WireGuardPage = lazy(() => import("@/pages/WireGuardPage").then((m) => ({ default: m.WireGuardPage })));
const NetworkPage = lazy(() => import("@/pages/NetworkPage").then((m) => ({ default: m.NetworkPage })));

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
            <Route path="/api-tokens" element={<ApiTokensPage />} />
            <Route path="/cron" element={<CronPage />} />
            <Route path="/firewall" element={<FirewallPage />} />
            <Route path="/stacks" element={<StacksPage />} />
            <Route path="/docker" element={<DockerPage />} />
            <Route path="/backup" element={<BackupPage />} />
            <Route path="/packages" element={<PackagesPage />} />
            <Route path="/wireguard" element={<WireGuardPage />} />
            <Route path="/network" element={<NetworkPage />} />
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
