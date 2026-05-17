import { NavLink, Outlet, useLocation, useNavigate } from "react-router-dom";
import { motion, AnimatePresence } from "framer-motion";
import {
  Activity,
  Archive,
  Boxes,
  CircuitBoard,
  Clock,
  Files,
  KeyRound,
  Layers,
  LogOut,
  Network as NetworkIcon,
  Package,
  Radio,
  ScrollText,
  Settings2,
  Shield,
  ShieldCheck,
  TerminalSquare,
  UserRound,
  Users,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Dropdown,
  DropdownContent,
  DropdownItem,
  DropdownSeparator,
  DropdownTrigger,
} from "@/components/ui/dropdown";
import { Separator } from "@/components/ui/separator";
import { useAuth, useIsAdmin } from "@/store/auth";
import { api } from "@/lib/api";
import { cn } from "@/lib/utils";
import { toast } from "@/components/ui/toast";

const navItems = [
  { to: "/dashboard", label: "Dashboard", icon: Activity, adminOnly: false },
  { to: "/services", label: "Services", icon: CircuitBoard, adminOnly: false },
  { to: "/docker", label: "Docker", icon: Boxes, adminOnly: false },
  { to: "/stacks", label: "Stacks", icon: Layers, adminOnly: false },
  { to: "/cron", label: "Cron", icon: Clock, adminOnly: false },
  { to: "/users", label: "Users", icon: Users, adminOnly: false },
  { to: "/files", label: "Files", icon: Files, adminOnly: false },
  { to: "/terminal", label: "Terminal", icon: TerminalSquare, adminOnly: false },
  { to: "/api-tokens", label: "API tokens", icon: KeyRound, adminOnly: false },
  { to: "/firewall", label: "Firewall", icon: ShieldCheck, adminOnly: true },
  { to: "/packages", label: "Packages", icon: Package, adminOnly: false },
  { to: "/network", label: "Network", icon: NetworkIcon, adminOnly: true },
  { to: "/wireguard", label: "WireGuard", icon: Radio, adminOnly: true },
  { to: "/backup", label: "Backup", icon: Archive, adminOnly: true },
  { to: "/panel-users", label: "Panel users", icon: Shield, adminOnly: true },
  { to: "/audit", label: "Audit log", icon: ScrollText, adminOnly: true },
];

export function AppLayout() {
  const auth = useAuth();
  const isAdmin = useIsAdmin();
  const nav = useNavigate();
  const loc = useLocation();

  const visibleNav = navItems.filter((it) => !it.adminOnly || isAdmin);

  const logout = async () => {
    try {
      await api.post("/api/auth/logout");
    } catch {
      /* ignore */
    }
    auth.clear();
    toast("Signed out");
    nav("/login");
  };

  return (
    <div className="flex h-full w-full">
      {/* Sidebar */}
      <aside className="hidden md:flex w-60 shrink-0 flex-col border-r border-border bg-card/60 backdrop-blur-md">
        <div className="flex h-16 items-center px-5">
          <Brand />
        </div>
        <Separator />
        <nav className="flex-1 px-3 py-4 space-y-1">
          {visibleNav.map((it) => (
            <NavLink
              key={it.to}
              to={it.to}
              className={({ isActive }) =>
                cn(
                  "flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors",
                  isActive
                    ? "bg-primary/10 text-primary"
                    : "text-muted-foreground hover:bg-accent hover:text-foreground",
                )
              }
            >
              <it.icon className="h-4 w-4" />
              <span>{it.label}</span>
            </NavLink>
          ))}
        </nav>
        <div className="p-3">
          <Dropdown>
            <DropdownTrigger asChild>
              <Button variant="ghost" className="w-full justify-start gap-2">
                <UserRound className="h-4 w-4" />
                <span className="truncate flex-1 text-left">{auth.user}</span>
                {auth.role && <RoleBadge role={auth.role} />}
              </Button>
            </DropdownTrigger>
            <DropdownContent align="start" className="w-52">
              <DropdownItem
                onClick={() => nav("/audit")}
                className="cursor-pointer"
              >
                <Settings2 className="mr-2 h-4 w-4" />
                Account settings
              </DropdownItem>
              <DropdownSeparator />
              <DropdownItem onClick={logout} className="cursor-pointer text-destructive">
                <LogOut className="mr-2 h-4 w-4" />
                Sign out
              </DropdownItem>
            </DropdownContent>
          </Dropdown>
        </div>
      </aside>

      {/* Main */}
      <main className="flex flex-1 min-w-0 flex-col">
        {/* Mobile top nav */}
        <header className="md:hidden flex items-center justify-between border-b border-border px-4 h-14">
          <Brand small />
          <Dropdown>
            <DropdownTrigger asChild>
              <Button variant="ghost" size="icon">
                <UserRound className="h-5 w-5" />
              </Button>
            </DropdownTrigger>
            <DropdownContent align="end">
              {visibleNav.map((it) => (
                <DropdownItem key={it.to} onClick={() => nav(it.to)}>
                  <it.icon className="mr-2 h-4 w-4" />
                  {it.label}
                </DropdownItem>
              ))}
              <DropdownSeparator />
              <DropdownItem onClick={logout} className="text-destructive">
                <LogOut className="mr-2 h-4 w-4" />
                Sign out
              </DropdownItem>
            </DropdownContent>
          </Dropdown>
        </header>

        <div className="flex-1 overflow-y-auto">
          <AnimatePresence mode="wait">
            <motion.div
              key={loc.pathname}
              initial={{ opacity: 0, y: 4 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -4 }}
              transition={{ duration: 0.18, ease: "easeOut" }}
              className="p-6 md:p-8"
            >
              <Outlet />
            </motion.div>
          </AnimatePresence>
        </div>
      </main>
    </div>
  );
}

function RoleBadge({ role }: { role: "viewer" | "operator" | "admin" }) {
  const variant =
    role === "admin" ? "default" : role === "operator" ? "secondary" : "muted";
  return (
    <Badge variant={variant} className="ml-auto text-[10px] uppercase tracking-wider">
      {role}
    </Badge>
  );
}

function Brand({ small }: { small?: boolean }) {
  return (
    <div className="flex items-center gap-2.5">
      <div className="grid h-8 w-8 place-items-center rounded-lg bg-primary text-primary-foreground shadow-sm">
        <TerminalSquare className="h-4.5 w-4.5" />
      </div>
      {!small && (
        <div className="leading-tight">
          <div className="text-sm font-semibold tracking-tight">webtermin</div>
          <div className="text-[10px] uppercase tracking-widest text-muted-foreground">
            server control
          </div>
        </div>
      )}
    </div>
  );
}
