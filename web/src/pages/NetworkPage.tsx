import { useEffect, useState, type FormEvent } from "react";
import { Globe, Network as NetIcon, Pencil, Save } from "lucide-react";
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

interface Iface {
  name: string;
  type: string;
  state: string;
  connection: string;
  mac?: string;
  ipv4?: string[];
  ipv4_gateway?: string;
  ipv6?: string[];
  dns?: string[];
  ipv4_method?: string;
}

export function NetworkPage() {
  const [available, setAvailable] = useState<boolean | null>(null);
  const [ifaces, setIfaces] = useState<Iface[]>([]);
  const [hostname, setHostname] = useState("");
  const [editingHostname, setEditingHostname] = useState(false);
  const [newHostname, setNewHostname] = useState("");
  const [editFor, setEditFor] = useState<Iface | null>(null);

  const load = async () => {
    try {
      const r = await api.get<{ available: boolean; interfaces: Iface[] }>("/api/network/interfaces");
      setAvailable(r.available);
      setIfaces(r.interfaces ?? []);
      const h = await api.get<{ hostname: string }>("/api/network/hostname");
      setHostname(h.hostname);
    } catch (e) {
      if (e instanceof ApiError) toast.error("Failed", e.message);
    }
  };
  useEffect(() => { void load(); }, []);

  const saveHostname = async (e: FormEvent) => {
    e.preventDefault();
    try {
      await api.post("/api/network/hostname", { hostname: newHostname });
      toast.success("Hostname updated", newHostname);
      setHostname(newHostname);
      setEditingHostname(false);
    } catch (err) {
      if (err instanceof ApiError) toast.error("Failed", err.message);
    }
  };

  if (available === null) return <div className="text-sm text-muted-foreground">Loading…</div>;
  if (!available) {
    return (
      <Card>
        <CardContent className="p-6">
          <NetIcon className="h-6 w-6 text-muted-foreground mb-3" />
          <strong>NetworkManager not installed.</strong>
          <p className="mt-2 text-sm text-muted-foreground">
            Install with <code>sudo apt install network-manager</code> (or <code>NetworkManager</code> on Fedora/RHEL) and restart webtermin.
          </p>
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Network</h1>
        <p className="text-sm text-muted-foreground">
          Managed via <code>nmcli</code>. Changes apply immediately AND persist — a bad gateway can lock you out, so double-check before saving.
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Globe className="h-4 w-4 text-primary" /> Hostname
          </CardTitle>
        </CardHeader>
        <CardContent>
          {editingHostname ? (
            <form onSubmit={saveHostname} className="flex gap-2">
              <Input
                value={newHostname}
                onChange={(e) => setNewHostname(e.target.value)}
                className="font-mono"
                required
                autoFocus
              />
              <Button type="submit"><Save className="h-4 w-4" /> Save</Button>
              <Button type="button" variant="ghost" onClick={() => setEditingHostname(false)}>Cancel</Button>
            </form>
          ) : (
            <div className="flex items-center gap-3">
              <code className="text-lg">{hostname}</code>
              <Button size="sm" variant="ghost" onClick={() => { setNewHostname(hostname); setEditingHostname(true); }}>
                <Pencil className="h-4 w-4" /> Edit
              </Button>
            </div>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <NetIcon className="h-4 w-4 text-primary" /> Interfaces ({ifaces.length})
          </CardTitle>
        </CardHeader>
        <CardContent className="p-0">
          <div className="divide-y divide-border">
            {ifaces.map((i) => (
              <div key={i.name} className="grid grid-cols-12 gap-3 items-center px-5 py-3">
                <div className="col-span-2">
                  <div className="font-medium">{i.name}</div>
                  <div className="text-xs text-muted-foreground">{i.type}</div>
                </div>
                <div className="col-span-2">
                  <StateBadge state={i.state} />
                </div>
                <div className="col-span-3">
                  <div className="text-xs uppercase tracking-wider text-muted-foreground">IPv4</div>
                  <div className="font-mono text-xs">
                    {i.ipv4?.length ? i.ipv4.join(", ") : "—"}
                  </div>
                  {i.ipv4_gateway && (
                    <div className="text-xs text-muted-foreground font-mono">gw {i.ipv4_gateway}</div>
                  )}
                </div>
                <div className="col-span-3">
                  <div className="text-xs uppercase tracking-wider text-muted-foreground">DNS</div>
                  <div className="font-mono text-xs">
                    {i.dns?.length ? i.dns.join(", ") : "—"}
                  </div>
                </div>
                <div className="col-span-1">
                  <Badge variant="muted">{i.ipv4_method || "auto"}</Badge>
                </div>
                <div className="col-span-1 flex justify-end">
                  <Button size="icon" variant="ghost" title="Edit" onClick={() => setEditFor(i)}>
                    <Pencil className="h-4 w-4" />
                  </Button>
                </div>
              </div>
            ))}
            {ifaces.length === 0 && (
              <div className="p-8 text-center text-sm text-muted-foreground">No interfaces.</div>
            )}
          </div>
        </CardContent>
      </Card>

      <EditDialog target={editFor} onClose={() => setEditFor(null)} onSaved={load} />
    </div>
  );
}

function StateBadge({ state }: { state: string }) {
  const s = state.toLowerCase();
  if (s === "connected" || s === "100") return <Badge variant="success">connected</Badge>;
  if (s.includes("disconnect")) return <Badge variant="muted">disconnected</Badge>;
  if (s === "unavailable") return <Badge variant="destructive">unavailable</Badge>;
  return <Badge variant="warning">{state}</Badge>;
}

function EditDialog({ target, onClose, onSaved }: { target: Iface | null; onClose: () => void; onSaved: () => Promise<void> }) {
  const [mode, setMode] = useState<"static" | "dhcp">("dhcp");
  const [address, setAddress] = useState("");
  const [gateway, setGateway] = useState("");
  const [dns, setDns] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (target) {
      setMode(target.ipv4_method === "manual" ? "static" : "dhcp");
      setAddress(target.ipv4?.[0] || "");
      setGateway(target.ipv4_gateway || "");
      setDns((target.dns || []).join(", "));
    }
  }, [target]);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (!target) return;
    if (!confirm(
      mode === "static"
        ? `Set ${target.name} static to ${address}? A wrong address or gateway can lock you out of the panel.`
        : `Switch ${target.name} to DHCP? Current static settings will be cleared.`,
    )) return;
    setBusy(true);
    try {
      if (mode === "static") {
        const dnsList = dns.split(/[,\s]+/).map((d) => d.trim()).filter(Boolean);
        await api.post(`/api/network/interfaces/${target.name}/static`, {
          address, gateway, dns: dnsList,
        });
      } else {
        await api.post(`/api/network/interfaces/${target.name}/dhcp`);
      }
      toast.success(`${target.name}: ${mode === "static" ? "static set" : "DHCP enabled"}`);
      onClose();
      await onSaved();
    } catch (err) {
      if (err instanceof ApiError) toast.error("Failed", err.message);
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog open={!!target} onOpenChange={(o) => !o && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Edit {target?.name}</DialogTitle>
          <DialogDescription>
            Changes apply immediately. Make sure you're not editing the
            interface you're connected through, or you'll need physical access
            to recover.
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-3">
          <div className="space-y-1.5">
            <Label>Mode</Label>
            <div className="flex gap-2">
              {(["dhcp", "static"] as const).map((m) => (
                <button
                  key={m}
                  type="button"
                  onClick={() => setMode(m)}
                  className={`flex-1 rounded-md border px-3 py-2 text-sm transition-colors ${
                    mode === m
                      ? "border-primary bg-primary/10 text-primary"
                      : "border-input hover:bg-accent"
                  }`}
                >
                  {m === "dhcp" ? "Automatic (DHCP)" : "Static"}
                </button>
              ))}
            </div>
          </div>
          {mode === "static" && (
            <>
              <div className="space-y-1.5">
                <Label>Address (x.x.x.x/yy)</Label>
                <Input value={address} onChange={(e) => setAddress(e.target.value)} className="font-mono" required />
              </div>
              <div className="space-y-1.5">
                <Label>Gateway</Label>
                <Input value={gateway} onChange={(e) => setGateway(e.target.value)} className="font-mono" />
              </div>
              <div className="space-y-1.5">
                <Label>DNS (comma or space separated)</Label>
                <Input value={dns} onChange={(e) => setDns(e.target.value)} className="font-mono" placeholder="1.1.1.1, 1.0.0.1" />
              </div>
            </>
          )}
          <div className="flex justify-end gap-2">
            <Button type="button" variant="ghost" onClick={onClose}>Cancel</Button>
            <Button type="submit" disabled={busy}>{busy ? "Saving…" : "Apply"}</Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}
