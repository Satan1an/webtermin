import { useEffect, useState } from "react";
import { CheckCircle2, Download, Loader2, Power, RefreshCw, ShieldAlert } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { api, ApiError } from "@/lib/api";
import { toast } from "@/components/ui/toast";
import { useIsAdmin } from "@/store/auth";
import { usePackageOp } from "@/components/PackageOpDialog";

interface Status {
  reboot: { required: boolean; reasons: string[] | null; running_kernel: string };
  updates: {
    total: number;
    security: number;
    packages: { name: string; current: string; candidate: string; security: boolean }[] | null;
    checked_at: string;
    checking: boolean;
    error?: string;
  };
}

// HostStatus is the dashboard's "needs attention" row: a pending reboot and
// pending updates, with the admin actions to resolve them.
export function HostStatus() {
  const isAdmin = useIsAdmin();
  const [st, setSt] = useState<Status | null>(null);
  const [confirmReboot, setConfirmReboot] = useState(false);
  const [rebooting, setRebooting] = useState(false);
  const pkgOp = usePackageOp();

  const load = () => api.get<Status>("/api/system/status").then(setSt).catch(() => {});

  // Poll fast while the server is still running `apt-get update` in the
  // background, then settle to once a minute.
  useEffect(() => {
    void load();
    const id = setInterval(() => void load(), st?.updates.checking ? 4000 : 60000);
    return () => clearInterval(id);
  }, [st?.updates.checking]);

  const reboot = async () => {
    setConfirmReboot(false);
    try {
      await api.post("/api/system/reboot", { confirm: "reboot" });
      setRebooting(true);
      toast.success("Rebooting", "The page reconnects when the host is back.");
      // Wait for the host to go down, then for the panel to answer again.
      await new Promise((r) => setTimeout(r, 15000));
      for (;;) {
        try {
          await api.get("/api/system/info");
          window.location.reload();
          return;
        } catch {
          await new Promise((r) => setTimeout(r, 3000));
        }
      }
    } catch (e) {
      if (e instanceof ApiError) toast.error("Reboot failed", e.message);
    }
  };

  if (!st) return null;
  const { reboot: rb, updates: up } = st;
  const pkgs = up.packages ?? [];
  const sorted = [...pkgs].sort((a, b) => Number(b.security) - Number(a.security) || a.name.localeCompare(b.name));

  if (!rb.required && up.total === 0 && !up.checking) {
    return (
      <div className="flex items-center gap-2 text-sm text-muted-foreground">
        <CheckCircle2 className="h-4 w-4 text-success" />
        Up to date · no reboot needed
        {up.checked_at && !up.checked_at.startsWith("0001") && (
          <span className="text-xs">· checked {new Date(up.checked_at).toLocaleTimeString()}</span>
        )}
      </div>
    );
  }

  return (
    <div className="grid gap-4 md:grid-cols-2">
      {rb.required && (
        <Card className="border-warning/40">
          <CardContent className="p-5 space-y-3">
            <div className="flex items-start justify-between gap-3">
              <div className="flex items-center gap-2 font-medium">
                <Power className="h-4 w-4 text-warning" /> Reboot required
              </div>
              {isAdmin && (
                <Button size="sm" variant="outline" disabled={rebooting} onClick={() => setConfirmReboot(true)}>
                  {rebooting ? <Loader2 className="h-4 w-4 animate-spin" /> : <Power className="h-4 w-4" />}
                  {rebooting ? "Rebooting…" : "Reboot"}
                </Button>
              )}
            </div>
            <ul className="text-sm text-muted-foreground space-y-1">
              {(rb.reasons ?? []).map((r) => <li key={r}>{r}</li>)}
            </ul>
          </CardContent>
        </Card>
      )}

      {(up.total > 0 || up.checking) && (
        <Card className={up.security > 0 ? "border-destructive/40" : undefined}>
          <CardContent className="p-5 space-y-3">
            <div className="flex items-start justify-between gap-3">
              <div className="flex items-center gap-2 font-medium">
                {up.security > 0 ? <ShieldAlert className="h-4 w-4 text-destructive" /> : <Download className="h-4 w-4 text-primary" />}
                {up.checking && up.total === 0 ? "Checking for updates…" : `${up.total} update${up.total === 1 ? "" : "s"} available`}
                {up.security > 0 && <Badge variant="destructive">{up.security} security</Badge>}
                {up.checking && <RefreshCw className="h-3.5 w-3.5 animate-spin text-muted-foreground" />}
              </div>
              {isAdmin && up.total > 0 && (
                <Button size="sm" disabled={!!pkgOp.busy} onClick={() => void pkgOp.start("upgrade", [], { onDone: load })}>
                  <Download className="h-4 w-4" /> Review &amp; upgrade
                </Button>
              )}
            </div>
            {sorted.length > 0 && (
              <div className="flex flex-wrap gap-1 max-h-24 overflow-y-auto">
                {sorted.map((p) => (
                  <Badge
                    key={p.name}
                    variant={p.security ? "destructive" : "muted"}
                    className="font-mono"
                    title={`${p.current || "?"} → ${p.candidate}`}
                  >
                    {p.name}
                  </Badge>
                ))}
              </div>
            )}
            {up.error && <p className="text-xs text-destructive">{up.error}</p>}
          </CardContent>
        </Card>
      )}

      <Dialog open={confirmReboot} onOpenChange={setConfirmReboot}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Reboot the host?</DialogTitle>
            <DialogDescription>
              Every service and container restarts, and open terminals drop. If the new kernel fails to boot
              you will need console access.
            </DialogDescription>
          </DialogHeader>
          <ul className="text-sm text-muted-foreground list-disc pl-5">
            {(rb.reasons ?? []).map((r) => <li key={r}>{r}</li>)}
          </ul>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setConfirmReboot(false)}>Cancel</Button>
            <Button variant="destructive" onClick={() => void reboot()}>
              <Power className="h-4 w-4" /> Reboot now
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
      {pkgOp.dialog}
    </div>
  );
}
