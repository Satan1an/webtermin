import { useEffect, useState, type FormEvent } from "react";
import { Copy, Key, Plus, Trash2 } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { api, ApiError } from "@/lib/api";
import { toast } from "@/components/ui/toast";
import { useAuth, type Role } from "@/store/auth";

interface ApiToken {
  id: number;
  name: string;
  role: Role;
  owner_user_id: number;
  owner_name?: string;
  created_at: string;
  last_used_at?: string;
  expires_at?: string;
}

const ALL_ROLES: Role[] = ["viewer", "operator", "admin"];
const ROLE_RANK: Record<Role, number> = { viewer: 1, operator: 2, admin: 3 };

export function ApiTokensPage() {
  const myRole = useAuth((s) => s.role);
  const [tokens, setTokens] = useState<ApiToken[]>([]);
  const [createOpen, setCreateOpen] = useState(false);
  const [revealed, setRevealed] = useState<string | null>(null);

  const load = async () => {
    try {
      setTokens(await api.get<ApiToken[]>("/api/panel/tokens"));
    } catch (e) {
      if (e instanceof ApiError) toast.error("Failed to load tokens", e.message);
    }
  };

  useEffect(() => {
    void load();
  }, []);

  const revoke = async (t: ApiToken) => {
    if (!confirm(`Revoke token "${t.name}"? Any client using it stops working immediately.`)) return;
    try {
      await api.del(`/api/panel/tokens/${t.id}`);
      toast.success(`Revoked ${t.name}`);
      await load();
    } catch (e) {
      if (e instanceof ApiError) toast.error("Revoke failed", e.message);
    }
  };

  // Roles the current user is allowed to issue — capped at their own.
  const allowedRoles = myRole
    ? ALL_ROLES.filter((r) => ROLE_RANK[r] <= ROLE_RANK[myRole])
    : [];

  return (
    <div className="space-y-6">
      <div className="flex items-end justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">API tokens</h1>
          <p className="text-sm text-muted-foreground">
            Programmatic access tokens for scripts and automation. They authenticate via
            <code className="mx-1 rounded bg-muted px-1.5 py-0.5 font-mono text-xs">
              Authorization: Bearer wt_…
            </code>
            and bypass CSRF. Tokens are scoped at issue time — a viewer token can't perform
            operator actions even if its owner is later promoted.
          </p>
        </div>
        <Button onClick={() => setCreateOpen(true)}>
          <Plus className="h-4 w-4" /> New token
        </Button>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Key className="h-4 w-4 text-primary" /> Active tokens
          </CardTitle>
        </CardHeader>
        <CardContent className="p-0">
          <div className="divide-y divide-border">
            <div className="grid grid-cols-12 gap-3 px-5 py-2.5 text-xs uppercase tracking-wider text-muted-foreground">
              <div className="col-span-3">Name</div>
              <div className="col-span-2">Role</div>
              <div className="col-span-2">Owner</div>
              <div className="col-span-2">Last used</div>
              <div className="col-span-2">Expires</div>
              <div className="col-span-1 text-right">Actions</div>
            </div>
            {tokens.map((t) => (
              <div
                key={t.id}
                className="grid grid-cols-12 gap-3 items-center px-5 py-3 hover:bg-accent/30 transition-colors"
              >
                <div className="col-span-3 min-w-0 font-medium truncate">{t.name}</div>
                <div className="col-span-2">
                  <RoleBadge role={t.role} />
                </div>
                <div className="col-span-2 text-sm text-muted-foreground truncate">
                  {t.owner_name ?? "—"}
                </div>
                <div className="col-span-2 text-xs text-muted-foreground">
                  {t.last_used_at ? new Date(t.last_used_at).toLocaleString() : "never"}
                </div>
                <div className="col-span-2 text-xs text-muted-foreground">
                  {t.expires_at ? new Date(t.expires_at).toLocaleDateString() : "no expiry"}
                </div>
                <div className="col-span-1 flex justify-end">
                  <Button variant="ghost" size="icon" onClick={() => revoke(t)} title="Revoke">
                    <Trash2 className="h-4 w-4 text-destructive" />
                  </Button>
                </div>
              </div>
            ))}
            {tokens.length === 0 && (
              <div className="p-8 text-center text-sm text-muted-foreground">
                No tokens yet. Click <strong>New token</strong> to create one.
              </div>
            )}
          </div>
        </CardContent>
      </Card>

      <CreateTokenDialog
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        allowedRoles={allowedRoles}
        onCreated={async (plaintext) => {
          await load();
          setRevealed(plaintext);
        }}
      />

      <RevealDialog
        plaintext={revealed}
        onClose={() => setRevealed(null)}
      />
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

function CreateTokenDialog({
  open, onClose, allowedRoles, onCreated,
}: {
  open: boolean;
  onClose: () => void;
  allowedRoles: Role[];
  onCreated: (plaintext: string) => Promise<void> | void;
}) {
  const [name, setName] = useState("");
  const [role, setRole] = useState<Role>(allowedRoles[0] ?? "viewer");
  const [expiry, setExpiry] = useState(0); // 0 = no expiry
  const [busy, setBusy] = useState(false);

  // Sync role default when the dialog opens (in case user role changed).
  useEffect(() => {
    if (open && allowedRoles.length > 0) setRole(allowedRoles[0]);
  }, [open, allowedRoles]);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    try {
      const res = await api.post<{ plaintext: string }>("/api/panel/tokens", {
        name,
        role,
        expires_in_days: expiry,
      });
      setName(""); setExpiry(0);
      onClose();
      await onCreated(res.plaintext);
    } catch (e) {
      if (e instanceof ApiError) toast.error("Create failed", e.message);
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>New API token</DialogTitle>
          <DialogDescription>
            The token's plaintext is shown <strong>once</strong> after creation. Store it now —
            we can't recover it later, only revoke it.
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-4">
          <div className="space-y-1.5">
            <Label htmlFor="tn">Name</Label>
            <Input
              id="tn"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="ci-bot, backup-script, …"
              required
              maxLength={64}
              autoFocus
            />
          </div>
          <div className="space-y-1.5">
            <Label>Role</Label>
            <div className="flex gap-2">
              {allowedRoles.map((r) => (
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
            <p className="text-xs text-muted-foreground">
              Capped at your own role. The token will be scoped to this — even if your role
              changes later, the token's role stays put.
            </p>
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="exp">Expires after (days)</Label>
            <Input
              id="exp"
              type="number"
              min={0}
              max={1825}
              value={expiry}
              onChange={(e) => setExpiry(Number(e.target.value))}
            />
            <p className="text-xs text-muted-foreground">0 = no expiry. Max 1825 (5 years).</p>
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

function RevealDialog({ plaintext, onClose }: { plaintext: string | null; onClose: () => void }) {
  const copy = async () => {
    if (!plaintext) return;
    await navigator.clipboard.writeText(plaintext);
    toast.success("Copied", "Token is in your clipboard");
  };
  return (
    <Dialog open={!!plaintext} onOpenChange={(o) => !o && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Your new token</DialogTitle>
          <DialogDescription>
            Copy it now — this is the only time it'll be shown. We store only a hash.
          </DialogDescription>
        </DialogHeader>
        <div className="flex items-center gap-2 rounded-md border border-primary/40 bg-primary/5 p-3">
          <code className="flex-1 break-all font-mono text-sm">{plaintext}</code>
          <Button variant="outline" size="icon" onClick={copy} title="Copy">
            <Copy className="h-4 w-4" />
          </Button>
        </div>
        <div className="rounded-md bg-muted/40 p-3 text-xs text-muted-foreground">
          Use it like:
          <pre className="mt-1 font-mono text-[11px]">
            curl -H &apos;Authorization: Bearer {plaintext ?? "wt_…"}&apos; https://&lt;host&gt;:8443/api/auth/me
          </pre>
        </div>
        <div className="flex justify-end">
          <Button onClick={onClose}>Done</Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
