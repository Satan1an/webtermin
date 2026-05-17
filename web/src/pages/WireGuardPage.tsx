import { useEffect, useState, type FormEvent } from "react";
import { Plus, Radio, Trash2 } from "lucide-react";
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
import { formatBytes } from "@/lib/utils";

interface Peer {
  public_key: string;
  endpoint?: string;
  allowed_ips?: string;
  latest_handshake?: number;
  rx_bytes?: number;
  tx_bytes?: number;
}

interface WGStatus {
  available: boolean;
  iface?: string;
  public_key?: string;
  port?: number;
  peers: Peer[];
}

export function WireGuardPage() {
  const [status, setStatus] = useState<WGStatus | null>(null);
  const [open, setOpen] = useState(false);

  const load = async () => {
    try {
      setStatus(await api.get<WGStatus>("/api/wireguard/status"));
    } catch (e) {
      if (e instanceof ApiError) toast.error("Failed", e.message);
    }
  };
  useEffect(() => { void load(); }, []);

  const remove = async (p: Peer) => {
    if (!confirm(`Remove peer ${p.public_key.slice(0, 16)}…?`)) return;
    try {
      await api.post("/api/wireguard/peers/remove", { public_key: p.public_key });
      toast.success("Peer removed");
      await load();
    } catch (e) {
      if (e instanceof ApiError) toast.error("Remove failed", e.message);
    }
  };

  if (!status) return <div className="text-sm text-muted-foreground">Loading…</div>;
  if (!status.available) {
    return (
      <Card>
        <CardContent className="p-6">
          <Radio className="h-6 w-6 text-muted-foreground mb-3" />
          <strong>WireGuard tools not installed.</strong>
          <p className="mt-2 text-sm text-muted-foreground">
            Install with <code>sudo apt install wireguard</code> (or your distro's equivalent) and refresh.
          </p>
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-end justify-between gap-4 flex-wrap">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">WireGuard</h1>
          <p className="text-sm text-muted-foreground">
            Interface <code>{status.iface}</code> · listening on port {status.port || "—"}
          </p>
        </div>
        <Button onClick={() => setOpen(true)}>
          <Plus className="h-4 w-4" /> Add peer
        </Button>
      </div>

      {status.public_key && (
        <Card>
          <CardHeader>
            <CardTitle>Server</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-xs text-muted-foreground">Public key (share with clients)</div>
            <code className="text-xs break-all">{status.public_key}</code>
          </CardContent>
        </Card>
      )}

      <Card>
        <CardHeader>
          <CardTitle>Peers ({status.peers.length})</CardTitle>
        </CardHeader>
        <CardContent className="p-0">
          <div className="divide-y divide-border">
            <div className="grid grid-cols-12 gap-3 px-5 py-2.5 text-xs uppercase tracking-wider text-muted-foreground">
              <div className="col-span-4">Public key</div>
              <div className="col-span-2">Allowed IPs</div>
              <div className="col-span-2">Endpoint</div>
              <div className="col-span-1">RX</div>
              <div className="col-span-1">TX</div>
              <div className="col-span-1">Handshake</div>
              <div className="col-span-1 text-right">Actions</div>
            </div>
            {status.peers.map((p) => (
              <div key={p.public_key} className="grid grid-cols-12 gap-3 items-center px-5 py-3">
                <div className="col-span-4 font-mono text-xs truncate" title={p.public_key}>
                  {p.public_key.slice(0, 20)}…
                </div>
                <div className="col-span-2 font-mono text-xs">{p.allowed_ips || "—"}</div>
                <div className="col-span-2 font-mono text-xs">{p.endpoint || "—"}</div>
                <div className="col-span-1 text-xs tabular-nums">{formatBytes(p.rx_bytes ?? 0)}</div>
                <div className="col-span-1 text-xs tabular-nums">{formatBytes(p.tx_bytes ?? 0)}</div>
                <div className="col-span-1">
                  {p.latest_handshake ? (
                    <Badge variant="success" title={new Date(p.latest_handshake * 1000).toLocaleString()}>
                      {timeSince(p.latest_handshake)}
                    </Badge>
                  ) : (
                    <Badge variant="muted">never</Badge>
                  )}
                </div>
                <div className="col-span-1 flex justify-end">
                  <Button variant="ghost" size="icon" onClick={() => remove(p)}>
                    <Trash2 className="h-4 w-4 text-destructive" />
                  </Button>
                </div>
              </div>
            ))}
            {status.peers.length === 0 && (
              <div className="p-8 text-center text-sm text-muted-foreground">No peers configured.</div>
            )}
          </div>
        </CardContent>
      </Card>

      <AddPeerDialog open={open} onClose={() => setOpen(false)} onAdded={load} />
    </div>
  );
}

function timeSince(unix: number): string {
  const s = Math.floor(Date.now() / 1000 - unix);
  if (s < 60) return `${s}s ago`;
  if (s < 3600) return `${Math.floor(s / 60)}m ago`;
  if (s < 86400) return `${Math.floor(s / 3600)}h ago`;
  return `${Math.floor(s / 86400)}d ago`;
}

function AddPeerDialog({ open, onClose, onAdded }: { open: boolean; onClose: () => void; onAdded: () => Promise<void> }) {
  const [comment, setComment] = useState("");
  const [allowedIPs, setAllowedIPs] = useState("10.0.0.2/32");
  const [endpoint, setEndpoint] = useState("");
  const [publicKey, setPublicKey] = useState("");
  const [busy, setBusy] = useState(false);
  const [result, setResult] = useState<{ public_key: string; private_key?: string } | null>(null);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    try {
      const res = await api.post<{ public_key: string; private_key?: string }>(
        "/api/wireguard/peers",
        { comment, allowed_ips: allowedIPs, endpoint, public_key: publicKey },
      );
      setResult(res);
      await onAdded();
    } catch (err) {
      if (err instanceof ApiError) toast.error("Add failed", err.message);
    } finally {
      setBusy(false);
    }
  };

  const reset = () => {
    setComment(""); setAllowedIPs("10.0.0.2/32"); setEndpoint("");
    setPublicKey(""); setResult(null);
    onClose();
  };

  return (
    <Dialog open={open} onOpenChange={(o) => !o && reset()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Add WireGuard peer</DialogTitle>
          <DialogDescription>
            Leave Public key empty to let the server generate a keypair for you —
            the private key is shown ONCE.
          </DialogDescription>
        </DialogHeader>
        {result ? (
          <div className="space-y-3">
            <div className="rounded-md border border-success/40 bg-success/5 p-3 text-sm">
              ✓ Peer added.
            </div>
            <div className="space-y-1">
              <Label>Peer public key</Label>
              <code className="block text-xs break-all">{result.public_key}</code>
            </div>
            {result.private_key && (
              <div className="space-y-1">
                <Label>Peer private key (give to the client, then forget)</Label>
                <code className="block text-xs break-all bg-warning/10 p-2 rounded">{result.private_key}</code>
              </div>
            )}
            <Button onClick={reset}>Done</Button>
          </div>
        ) : (
          <form onSubmit={submit} className="space-y-3">
            <div className="space-y-1.5">
              <Label>Comment / name</Label>
              <Input value={comment} onChange={(e) => setComment(e.target.value)} placeholder="phone, laptop, …" />
            </div>
            <div className="space-y-1.5">
              <Label>Allowed IPs</Label>
              <Input value={allowedIPs} onChange={(e) => setAllowedIPs(e.target.value)} className="font-mono" required />
            </div>
            <div className="space-y-1.5">
              <Label>Endpoint (optional)</Label>
              <Input value={endpoint} onChange={(e) => setEndpoint(e.target.value)} className="font-mono" placeholder="" />
            </div>
            <div className="space-y-1.5">
              <Label>Public key (leave empty to generate one)</Label>
              <Input value={publicKey} onChange={(e) => setPublicKey(e.target.value)} className="font-mono" placeholder="auto-generate" />
            </div>
            <div className="flex justify-end gap-2">
              <Button type="button" variant="ghost" onClick={reset}>Cancel</Button>
              <Button type="submit" disabled={busy}>{busy ? "Adding…" : "Add peer"}</Button>
            </div>
          </form>
        )}
      </DialogContent>
    </Dialog>
  );
}
