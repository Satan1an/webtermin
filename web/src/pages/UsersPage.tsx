import { useEffect, useState, type FormEvent } from "react";
import {
  Key,
  Plus,
  ShieldCheck,
  Trash2,
  Users as UsersIcon,
} from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Badge } from "@/components/ui/badge";
import { api, ApiError } from "@/lib/api";
import { toast } from "@/components/ui/toast";

interface LinuxUser {
  name: string;
  uid: number;
  gid: number;
  gecos: string;
  home: string;
  shell: string;
  is_system: boolean;
}

interface SSHKey {
  type: string;
  fingerprint: string;
  comment: string;
  raw: string;
}

export function UsersPage() {
  const [users, setUsers] = useState<LinuxUser[]>([]);
  const [createOpen, setCreateOpen] = useState(false);
  const [keysFor, setKeysFor] = useState<string | null>(null);

  const load = async () => {
    try {
      const u = await api.get<LinuxUser[]>("/api/linux/users");
      setUsers(u ?? []);
    } catch (e) {
      if (e instanceof ApiError) toast.error("Failed to load users", e.message);
    }
  };

  useEffect(() => {
    void load();
  }, []);

  const remove = async (name: string) => {
    if (!confirm(`Delete Linux user "${name}"? Their home will be removed.`)) return;
    try {
      await api.del(`/api/linux/users/${encodeURIComponent(name)}?remove_home=1`);
      toast.success(`Deleted ${name}`);
      await load();
    } catch (e) {
      if (e instanceof ApiError) toast.error("Delete failed", e.message);
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-end justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Linux users</h1>
          <p className="text-sm text-muted-foreground">{users.length} non-system accounts</p>
        </div>
        <Button onClick={() => setCreateOpen(true)}>
          <Plus className="h-4 w-4" /> New user
        </Button>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <UsersIcon className="h-4 w-4" /> Users
          </CardTitle>
        </CardHeader>
        <CardContent className="p-0">
          <div className="divide-y divide-border">
            <div className="grid grid-cols-12 gap-3 px-5 py-2.5 text-xs uppercase tracking-wider text-muted-foreground">
              <div className="col-span-3">Name</div>
              <div className="col-span-1">UID</div>
              <div className="col-span-3">Home</div>
              <div className="col-span-3">Shell</div>
              <div className="col-span-2 text-right">Actions</div>
            </div>
            {users.map((u) => (
              <div
                key={u.name}
                className="grid grid-cols-12 gap-3 items-center px-5 py-3 hover:bg-accent/30 transition-colors"
              >
                <div className="col-span-3 min-w-0">
                  <div className="font-medium truncate">{u.name}</div>
                  <div className="text-xs text-muted-foreground truncate">{u.gecos || "—"}</div>
                </div>
                <div className="col-span-1 tabular-nums">{u.uid}</div>
                <div className="col-span-3 text-sm text-muted-foreground truncate font-mono">
                  {u.home}
                </div>
                <div className="col-span-3 text-sm text-muted-foreground truncate font-mono">
                  {u.shell}
                </div>
                <div className="col-span-2 flex justify-end gap-1.5">
                  <Button
                    variant="ghost"
                    size="icon"
                    title="SSH keys"
                    onClick={() => setKeysFor(u.name)}
                  >
                    <Key className="h-4 w-4" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon"
                    title="Delete"
                    onClick={() => remove(u.name)}
                  >
                    <Trash2 className="h-4 w-4 text-destructive" />
                  </Button>
                </div>
              </div>
            ))}
            {users.length === 0 && (
              <div className="p-8 text-center text-sm text-muted-foreground">No users.</div>
            )}
          </div>
        </CardContent>
      </Card>

      <CreateUserDialog open={createOpen} onClose={() => setCreateOpen(false)} onCreated={load} />
      <KeysDialog open={!!keysFor} username={keysFor ?? ""} onClose={() => setKeysFor(null)} />
    </div>
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
  const [gecos, setGecos] = useState("");
  const [pw, setPw] = useState("");
  const [busy, setBusy] = useState(false);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    try {
      await api.post("/api/linux/users", { name, gecos, password: pw });
      toast.success(`User "${name}" created`);
      await onCreated();
      onClose();
      setName(""); setGecos(""); setPw("");
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
          <DialogTitle>New Linux user</DialogTitle>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-4">
          <div className="space-y-1.5">
            <Label htmlFor="n">Username</Label>
            <Input id="n" value={name} onChange={(e) => setName(e.target.value)} required />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="g">Full name</Label>
            <Input id="g" value={gecos} onChange={(e) => setGecos(e.target.value)} />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="p">Initial password</Label>
            <Input
              id="p"
              type="password"
              value={pw}
              onChange={(e) => setPw(e.target.value)}
              required
              minLength={6}
            />
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

function KeysDialog({
  open, username, onClose,
}: {
  open: boolean;
  username: string;
  onClose: () => void;
}) {
  const [keys, setKeys] = useState<SSHKey[]>([]);
  const [newKey, setNewKey] = useState("");
  const [busy, setBusy] = useState(false);

  const load = async () => {
    if (!username) return;
    try {
      setKeys(await api.get<SSHKey[]>(`/api/linux/users/${encodeURIComponent(username)}/keys`));
    } catch (e) {
      if (e instanceof ApiError) toast.error("Failed to load keys", e.message);
    }
  };

  useEffect(() => {
    if (open) void load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, username]);

  const add = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    try {
      await api.post(`/api/linux/users/${encodeURIComponent(username)}/keys`, { key: newKey });
      setNewKey("");
      await load();
      toast.success("Key added");
    } catch (err) {
      if (err instanceof ApiError) toast.error("Add failed", err.message);
    } finally {
      setBusy(false);
    }
  };

  const remove = async (fp: string) => {
    try {
      await api.del(
        `/api/linux/users/${encodeURIComponent(username)}/keys/${encodeURIComponent(fp)}`,
      );
      await load();
      toast.success("Key removed");
    } catch (e) {
      if (e instanceof ApiError) toast.error("Remove failed", e.message);
    }
  };

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <ShieldCheck className="h-4 w-4 text-success" /> SSH keys — {username}
          </DialogTitle>
        </DialogHeader>
        <div className="space-y-3">
          {keys.length === 0 && (
            <div className="text-sm text-muted-foreground">No SSH keys yet.</div>
          )}
          {keys.map((k) => (
            <div key={k.fingerprint} className="flex items-center justify-between rounded-md border px-3 py-2">
              <div className="min-w-0">
                <div className="flex items-center gap-2 text-sm">
                  <Badge variant="muted">{k.type}</Badge>
                  <span className="font-mono text-xs truncate">{k.fingerprint}</span>
                </div>
                {k.comment && <div className="text-xs text-muted-foreground">{k.comment}</div>}
              </div>
              <Button variant="ghost" size="icon" onClick={() => remove(k.fingerprint)}>
                <Trash2 className="h-4 w-4 text-destructive" />
              </Button>
            </div>
          ))}
        </div>
        <form onSubmit={add} className="space-y-2 pt-2 border-t border-border">
          <Label htmlFor="newk">Paste a new authorized key</Label>
          <textarea
            id="newk"
            value={newKey}
            onChange={(e) => setNewKey(e.target.value)}
            placeholder="ssh-ed25519 AAAA…  user@host"
            className="w-full h-24 rounded-md border bg-background p-2 text-xs font-mono"
            required
          />
          <div className="flex justify-end">
            <Button type="submit" disabled={busy || !newKey.trim()}>
              {busy ? "Adding…" : "Add key"}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}
