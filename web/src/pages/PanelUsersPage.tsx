import { useEffect, useState, type FormEvent } from "react";
import { KeyRound, Plus, Shield, Trash2, UserCog } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Dropdown,
  DropdownContent,
  DropdownItem,
  DropdownTrigger,
} from "@/components/ui/dropdown";
import { api, ApiError } from "@/lib/api";
import { toast } from "@/components/ui/toast";
import { useAuth, type Role } from "@/store/auth";

interface PanelUser {
  id: number;
  username: string;
  role: Role;
  has_2fa: boolean;
  created_at: string;
  updated_at: string;
}

const ROLES: Role[] = ["viewer", "operator", "admin"];

export function PanelUsersPage() {
  const me = useAuth((s) => s.user);
  const [users, setUsers] = useState<PanelUser[]>([]);
  const [createOpen, setCreateOpen] = useState(false);
  const [pwTarget, setPwTarget] = useState<PanelUser | null>(null);

  const load = async () => {
    try {
      setUsers(await api.get<PanelUser[]>("/api/panel/users"));
    } catch (e) {
      if (e instanceof ApiError) toast.error("Failed to load", e.message);
    }
  };

  useEffect(() => {
    void load();
  }, []);

  const setRole = async (u: PanelUser, role: Role) => {
    try {
      await api.post(`/api/panel/users/${u.id}/role`, { role });
      toast.success(`${u.username} → ${role}`);
      await load();
    } catch (e) {
      if (e instanceof ApiError) toast.error("Change role failed", e.message);
    }
  };

  const remove = async (u: PanelUser) => {
    if (!confirm(`Delete panel user "${u.username}"?`)) return;
    try {
      await api.del(`/api/panel/users/${u.id}`);
      toast.success(`Deleted ${u.username}`);
      await load();
    } catch (e) {
      if (e instanceof ApiError) toast.error("Delete failed", e.message);
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-end justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Panel users</h1>
          <p className="text-sm text-muted-foreground">
            Accounts that can sign in to webtermin. <strong>Linux system users</strong> are managed
            on the <a href="/users" className="underline">Users</a> page instead.
          </p>
        </div>
        <Button onClick={() => setCreateOpen(true)}>
          <Plus className="h-4 w-4" /> New panel user
        </Button>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Shield className="h-4 w-4 text-primary" /> Accounts
          </CardTitle>
        </CardHeader>
        <CardContent className="p-0">
          <div className="divide-y divide-border">
            <div className="grid grid-cols-12 gap-3 px-5 py-2.5 text-xs uppercase tracking-wider text-muted-foreground">
              <div className="col-span-4">Username</div>
              <div className="col-span-2">Role</div>
              <div className="col-span-2">2FA</div>
              <div className="col-span-2">Created</div>
              <div className="col-span-2 text-right">Actions</div>
            </div>
            {users.map((u) => (
              <div
                key={u.id}
                className="grid grid-cols-12 gap-3 items-center px-5 py-3 hover:bg-accent/30 transition-colors"
              >
                <div className="col-span-4 min-w-0">
                  <div className="font-medium truncate">
                    {u.username}
                    {u.username === me && (
                      <span className="ml-2 text-xs text-muted-foreground">(you)</span>
                    )}
                  </div>
                </div>
                <div className="col-span-2">
                  <RoleBadge role={u.role} />
                </div>
                <div className="col-span-2">
                  {u.has_2fa ? (
                    <Badge variant="success">enabled</Badge>
                  ) : (
                    <Badge variant="muted">off</Badge>
                  )}
                </div>
                <div className="col-span-2 text-xs text-muted-foreground">
                  {new Date(u.created_at).toLocaleDateString()}
                </div>
                <div className="col-span-2 flex justify-end gap-1">
                  <Dropdown>
                    <DropdownTrigger asChild>
                      <Button variant="ghost" size="icon" title="Change role">
                        <UserCog className="h-4 w-4" />
                      </Button>
                    </DropdownTrigger>
                    <DropdownContent align="end">
                      {ROLES.map((r) => (
                        <DropdownItem
                          key={r}
                          onClick={() => setRole(u, r)}
                          className={u.role === r ? "font-semibold text-primary" : ""}
                        >
                          {r}
                        </DropdownItem>
                      ))}
                    </DropdownContent>
                  </Dropdown>
                  <Button
                    variant="ghost"
                    size="icon"
                    title="Reset password"
                    onClick={() => setPwTarget(u)}
                  >
                    <KeyRound className="h-4 w-4" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon"
                    title="Delete"
                    onClick={() => remove(u)}
                    disabled={u.username === me}
                  >
                    <Trash2 className="h-4 w-4 text-destructive" />
                  </Button>
                </div>
              </div>
            ))}
            {users.length === 0 && (
              <div className="p-8 text-center text-sm text-muted-foreground">
                No panel users.
              </div>
            )}
          </div>
        </CardContent>
      </Card>

      <CreateUserDialog open={createOpen} onClose={() => setCreateOpen(false)} onCreated={load} />
      <ResetPasswordDialog target={pwTarget} onClose={() => setPwTarget(null)} />
    </div>
  );
}

function RoleBadge({ role }: { role: Role }) {
  const variant =
    role === "admin" ? "default" : role === "operator" ? "secondary" : "muted";
  return (
    <Badge variant={variant} className="uppercase tracking-wider text-[10px]">
      {role}
    </Badge>
  );
}

function CreateUserDialog({
  open, onClose, onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: () => Promise<void>;
}) {
  const [name, setName] = useState("");
  const [pw, setPw] = useState("");
  const [role, setRole] = useState<Role>("viewer");
  const [busy, setBusy] = useState(false);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    try {
      await api.post("/api/panel/users", { username: name, password: pw, role });
      toast.success(`Created ${name}`, `role: ${role}`);
      await onCreated();
      setName(""); setPw(""); setRole("viewer");
      onClose();
    } catch (err) {
      if (err instanceof ApiError) toast.error("Create failed", err.message);
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>New panel user</DialogTitle>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-4">
          <div className="space-y-1.5">
            <Label htmlFor="pn">Username</Label>
            <Input id="pn" value={name} onChange={(e) => setName(e.target.value)} required />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="pp">Password (≥ 10 chars)</Label>
            <Input
              id="pp"
              type="password"
              value={pw}
              onChange={(e) => setPw(e.target.value)}
              minLength={10}
              required
            />
          </div>
          <div className="space-y-1.5">
            <Label>Role</Label>
            <div className="flex gap-2">
              {ROLES.map((r) => (
                <button
                  key={r}
                  type="button"
                  onClick={() => setRole(r)}
                  className={`flex-1 rounded-md border px-3 py-2 text-sm transition-colors ${
                    role === r
                      ? "border-primary bg-primary/10 text-primary"
                      : "border-input hover:bg-accent"
                  }`}
                >
                  {r}
                </button>
              ))}
            </div>
          </div>
          <div className="flex justify-end gap-2">
            <Button type="button" variant="ghost" onClick={onClose}>Cancel</Button>
            <Button type="submit" disabled={busy}>{busy ? "Creating…" : "Create"}</Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function ResetPasswordDialog({
  target, onClose,
}: { target: PanelUser | null; onClose: () => void }) {
  const [pw, setPw] = useState("");
  const [busy, setBusy] = useState(false);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (!target) return;
    setBusy(true);
    try {
      await api.post(`/api/panel/users/${target.id}/password`, { password: pw });
      toast.success(`Password reset for ${target.username}`);
      setPw("");
      onClose();
    } catch (err) {
      if (err instanceof ApiError) toast.error("Reset failed", err.message);
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog open={!!target} onOpenChange={(o) => !o && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Reset password — {target?.username}</DialogTitle>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-4">
          <div className="space-y-1.5">
            <Label htmlFor="np">New password (≥ 10 chars)</Label>
            <Input
              id="np"
              type="password"
              value={pw}
              onChange={(e) => setPw(e.target.value)}
              minLength={10}
              required
              autoFocus
            />
          </div>
          <div className="flex justify-end gap-2">
            <Button type="button" variant="ghost" onClick={onClose}>Cancel</Button>
            <Button type="submit" disabled={busy}>{busy ? "Saving…" : "Save"}</Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}
