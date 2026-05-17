import { useEffect, useState, type FormEvent } from "react";
import { Plus, Shield, ShieldOff, Trash2 } from "lucide-react";
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

interface Status {
  available: boolean;
  active: boolean;
  default_in?: string;
  default_out?: string;
  default_fwd?: string;
  logging?: string;
  rules: Rule[];
}

interface Rule {
  number: number;
  to: string;
  action: string;
  from: string;
}

const SPEC_PRESETS = [
  { label: "SSH (22/tcp)", value: "22/tcp" },
  { label: "HTTP (80/tcp)", value: "80/tcp" },
  { label: "HTTPS (443/tcp)", value: "443/tcp" },
  { label: "webtermin (8443/tcp)", value: "8443/tcp" },
];

export function FirewallPage() {
  const [status, setStatus] = useState<Status | null>(null);
  const [addOpen, setAddOpen] = useState(false);

  const load = async () => {
    try {
      setStatus(await api.get<Status>("/api/firewall/status"));
    } catch (e) {
      if (e instanceof ApiError) toast.error("Failed to load", e.message);
    }
  };

  useEffect(() => {
    void load();
  }, []);

  const toggle = async (next: boolean) => {
    if (!confirm(
      next
        ? "Enable ufw? Make sure SSH (22/tcp) is allowed or you may lock yourself out."
        : "Disable ufw? Your host will accept all incoming traffic.",
    )) return;
    try {
      await api.post("/api/firewall/toggle", { enabled: next });
      toast.success(next ? "Firewall enabled" : "Firewall disabled");
      await load();
    } catch (e) {
      if (e instanceof ApiError) toast.error("Toggle failed", e.message);
    }
  };

  const removeRule = async (r: Rule) => {
    if (!confirm(`Delete rule #${r.number}?\n${r.action}  ${r.to}  from ${r.from}`)) return;
    try {
      await api.del(`/api/firewall/rules/${r.number}`);
      toast.success(`Rule #${r.number} deleted`);
      await load();
    } catch (e) {
      if (e instanceof ApiError) toast.error("Delete failed", e.message);
    }
  };

  if (!status) {
    return <div className="text-sm text-muted-foreground">Loading firewall status…</div>;
  }

  if (!status.available) {
    return (
      <Card>
        <CardContent className="p-6 text-sm">
          <ShieldOff className="h-6 w-6 text-muted-foreground mb-3" />
          <strong>ufw is not installed.</strong>
          <p className="mt-2 text-muted-foreground">
            Install it with <code className="rounded bg-muted px-1.5 py-0.5">sudo apt install ufw</code> and refresh this page.
          </p>
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-end justify-between gap-4 flex-wrap">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Firewall</h1>
          <p className="text-sm text-muted-foreground">
            Managed via <code>ufw</code>. Adding rules is admin-only because a mistake can lock you out.
          </p>
        </div>
        <div className="flex items-center gap-2">
          {status.active ? (
            <Button variant="outline" onClick={() => toggle(false)}>
              <ShieldOff className="h-4 w-4" /> Disable firewall
            </Button>
          ) : (
            <Button onClick={() => toggle(true)}>
              <Shield className="h-4 w-4" /> Enable firewall
            </Button>
          )}
          <Button variant="outline" onClick={() => setAddOpen(true)}>
            <Plus className="h-4 w-4" /> Add rule
          </Button>
        </div>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Shield className="h-4 w-4 text-primary" /> Status
          </CardTitle>
        </CardHeader>
        <CardContent>
          <dl className="grid grid-cols-2 sm:grid-cols-4 gap-x-6 gap-y-2 text-sm">
            <div>
              <dt className="text-xs uppercase tracking-wider text-muted-foreground">State</dt>
              <dd className="mt-1">
                {status.active ? (
                  <Badge variant="success">active</Badge>
                ) : (
                  <Badge variant="warning">inactive</Badge>
                )}
              </dd>
            </div>
            <KV k="Default in" v={status.default_in} />
            <KV k="Default out" v={status.default_out} />
            <KV k="Logging" v={status.logging} />
          </dl>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Rules</CardTitle>
        </CardHeader>
        <CardContent className="p-0">
          <div className="divide-y divide-border">
            <div className="grid grid-cols-12 gap-3 px-5 py-2.5 text-xs uppercase tracking-wider text-muted-foreground">
              <div className="col-span-1">#</div>
              <div className="col-span-4">To</div>
              <div className="col-span-3">Action</div>
              <div className="col-span-3">From</div>
              <div className="col-span-1 text-right">Actions</div>
            </div>
            {status.rules.map((r) => (
              <div
                key={r.number}
                className="grid grid-cols-12 gap-3 items-center px-5 py-3 hover:bg-accent/30 transition-colors"
              >
                <div className="col-span-1 tabular-nums text-sm">{r.number}</div>
                <div className="col-span-4 font-mono text-sm">{r.to}</div>
                <div className="col-span-3">
                  <ActionBadge action={r.action} />
                </div>
                <div className="col-span-3 font-mono text-xs text-muted-foreground">{r.from}</div>
                <div className="col-span-1 flex justify-end">
                  <Button variant="ghost" size="icon" onClick={() => removeRule(r)}>
                    <Trash2 className="h-4 w-4 text-destructive" />
                  </Button>
                </div>
              </div>
            ))}
            {status.rules.length === 0 && (
              <div className="p-8 text-center text-sm text-muted-foreground">
                No rules configured.
              </div>
            )}
          </div>
        </CardContent>
      </Card>

      <AddRuleDialog open={addOpen} onClose={() => setAddOpen(false)} onAdded={load} />
    </div>
  );
}

function KV({ k, v }: { k: string; v?: string }) {
  return (
    <div>
      <dt className="text-xs uppercase tracking-wider text-muted-foreground">{k}</dt>
      <dd className="mt-1 font-mono">{v || "—"}</dd>
    </div>
  );
}

function ActionBadge({ action }: { action: string }) {
  if (action.startsWith("ALLOW")) return <Badge variant="success">{action}</Badge>;
  if (action.startsWith("DENY") || action.startsWith("REJECT")) return <Badge variant="destructive">{action}</Badge>;
  if (action.startsWith("LIMIT")) return <Badge variant="warning">{action}</Badge>;
  return <Badge variant="muted">{action}</Badge>;
}

function AddRuleDialog({
  open, onClose, onAdded,
}: {
  open: boolean;
  onClose: () => void;
  onAdded: () => Promise<void>;
}) {
  const [action, setAction] = useState<"allow" | "deny">("allow");
  const [spec, setSpec] = useState("22/tcp");
  const [busy, setBusy] = useState(false);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    try {
      await api.post("/api/firewall/rules", { action, spec });
      toast.success(`Rule added: ${action} ${spec}`);
      setSpec("22/tcp");
      onClose();
      await onAdded();
    } catch (err) {
      if (err instanceof ApiError) toast.error("Add failed", err.message);
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>New firewall rule</DialogTitle>
          <DialogDescription>
            Accepted: <code>22</code>, <code>443/tcp</code>, <code>8000:8010/tcp</code>,
            named services (<code>ssh</code>, <code>http</code>),
            <code>from 10.0.0.0/8</code>, or
            <code>from 1.2.3.4 to any port 22 proto tcp</code>.
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-4">
          <div className="space-y-1.5">
            <Label>Action</Label>
            <div className="flex gap-2">
              {(["allow", "deny"] as const).map((a) => (
                <button
                  key={a}
                  type="button"
                  onClick={() => setAction(a)}
                  className={`flex-1 rounded-md border px-3 py-2 text-sm transition-colors ${
                    action === a
                      ? a === "allow"
                        ? "border-success bg-success/10 text-success"
                        : "border-destructive bg-destructive/10 text-destructive"
                      : "border-input hover:bg-accent"
                  }`}
                >
                  {a}
                </button>
              ))}
            </div>
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="spec">Spec</Label>
            <Input
              id="spec"
              className="font-mono"
              value={spec}
              onChange={(e) => setSpec(e.target.value)}
              required
              autoFocus
            />
            <div className="flex gap-1.5 flex-wrap">
              {SPEC_PRESETS.map((p) => (
                <button
                  key={p.value}
                  type="button"
                  onClick={() => setSpec(p.value)}
                  className="rounded-md border border-input px-2 py-0.5 text-xs hover:bg-accent transition-colors"
                >
                  {p.label}
                </button>
              ))}
            </div>
          </div>
          <div className="flex justify-end gap-2">
            <Button type="button" variant="ghost" onClick={onClose}>Cancel</Button>
            <Button type="submit" disabled={busy}>{busy ? "Adding…" : "Add rule"}</Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}
