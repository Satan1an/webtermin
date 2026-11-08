import { useEffect, useRef, useState } from "react";
import { AlertTriangle, Ban, CheckCircle2, Loader2, XCircle } from "lucide-react";
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

export type PkgOp = "install" | "remove" | "upgrade";

interface Plan {
  install: string[] | null;
  upgrade: string[] | null;
  remove: string[] | null;
  blocked: { name: string; reason: string }[] | null;
  warnings: string[] | null;
}

interface Pending {
  op: PkgOp;
  names: string[];
  plan: Plan;
  onDone?: () => void | Promise<void>;
  // Set once confirmed: the dialog stays open and streams apt/dnf output.
  phase?: "running" | "ok" | "failed";
  error?: string;
}

interface Progress {
  running: boolean;
  lines: string[] | null;
}

// usePackageOp dry-runs a package operation on the server, shows what it will
// install/upgrade/remove plus any safety warnings, and only then runs it. The
// server re-checks everything on execute; this dialog is the human half.
export function usePackageOp() {
  const [pending, setPending] = useState<Pending | null>(null);
  const [busy, setBusy] = useState<string | null>(null);
  const [lines, setLines] = useState<string[]>([]);
  const logRef = useRef<HTMLPreElement>(null);
  const running = pending?.phase === "running";

  // Poll the server-side output buffer while the POST is in flight.
  useEffect(() => {
    if (!running) return;
    const tick = () =>
      api.get<Progress>("/api/packages/progress").then((p) => setLines(p.lines ?? [])).catch(() => {});
    void tick();
    const id = setInterval(tick, 1000);
    return () => clearInterval(id);
  }, [running]);

  useEffect(() => {
    logRef.current?.scrollTo({ top: logRef.current.scrollHeight });
  }, [lines]);

  const start = async (op: PkgOp, names: string[], opts: { onDone?: () => void | Promise<void> } = {}) => {
    setBusy(names.join(",") + op);
    try {
      const plan = await api.post<Plan>("/api/packages/plan", { op, names });
      setPending({ op, names, plan, ...opts });
    } catch (e) {
      if (e instanceof ApiError) toast.error("Dry run failed", e.message);
    } finally {
      setBusy(null);
    }
  };

  const confirm = async () => {
    if (!pending) return;
    const { op, names, plan, onDone } = pending;
    const label = names.length ? names.join(", ") : "all packages";
    setLines([]);
    setPending({ ...pending, phase: "running" });
    setBusy(names.join(",") + op);
    try {
      await api.post(`/api/packages/${op}`, { names, confirm_remove: plan.remove ?? [] });
      setPending((p) => p && { ...p, phase: "ok" });
      toast.success(`${op} ✓`, label);
      await onDone?.();
    } catch (e) {
      const msg = e instanceof ApiError ? e.message : String(e);
      setPending((p) => p && { ...p, phase: "failed", error: msg });
      toast.error(`${op} failed`, msg);
    } finally {
      // One last fetch so the log ends with apt's final lines.
      await api.get<Progress>("/api/packages/progress").then((p) => setLines(p.lines ?? [])).catch(() => {});
      setBusy(null);
    }
  };

  const plan = pending?.plan;
  const blocked = plan?.blocked ?? [];
  const warnings = plan?.warnings ?? [];

  const dialog = (
    <Dialog open={!!pending} onOpenChange={(o) => !o && !running && setPending(null)}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            {pending?.phase === "running" && <Loader2 className="h-4 w-4 animate-spin text-primary" />}
            {pending?.phase === "ok" && <CheckCircle2 className="h-4 w-4 text-success" />}
            {pending?.phase === "failed" && <XCircle className="h-4 w-4 text-destructive" />}
            {pending?.op} {pending?.names.length ? pending.names.join(", ") : "all packages"}
          </DialogTitle>
          <DialogDescription>
            {pending?.phase
              ? running ? "Running — you can keep this open or close the tab; the server finishes the job." : "Finished."
              : "Dry run of what the package manager will do."}
          </DialogDescription>
        </DialogHeader>
        {pending?.phase && (
          <pre
            ref={logRef}
            className="max-h-72 overflow-auto rounded-md bg-muted/50 p-3 text-xs font-mono leading-relaxed whitespace-pre-wrap"
          >
            {lines.length ? lines.join("\n") : "Starting…"}
            {pending.error && `\n\n${pending.error}`}
          </pre>
        )}
        {plan && !pending?.phase && (
          <div className="space-y-3 text-sm">
            <PkgList label="Install" variant="success" names={plan.install} />
            <PkgList label="Upgrade" variant="default" names={plan.upgrade} />
            <PkgList label="Remove" variant="destructive" names={plan.remove} />
            {!plan.install?.length && !plan.upgrade?.length && !plan.remove?.length && (
              <p className="text-muted-foreground">Nothing to do.</p>
            )}
            {blocked.map((b, i) => (
              <p key={i} className="flex gap-2 text-destructive">
                <Ban className="h-4 w-4 shrink-0 mt-0.5" />
                <span>
                  {b.name && <code>{b.name}</code>} {b.reason}. Blocked; use the terminal if you really mean it.
                </span>
              </p>
            ))}
            {warnings.map((w, i) => (
              <p key={i} className="flex gap-2 text-warning">
                <AlertTriangle className="h-4 w-4 shrink-0 mt-0.5" />
                <span>{w}</span>
              </p>
            ))}
          </div>
        )}
        <DialogFooter>
          {pending?.phase ? (
            <Button variant="ghost" disabled={running} onClick={() => setPending(null)}>Close</Button>
          ) : (
            <>
              <Button variant="ghost" onClick={() => setPending(null)}>Cancel</Button>
              <Button
                variant={plan?.remove?.length ? "destructive" : "default"}
                disabled={blocked.length > 0}
                onClick={() => void confirm()}
              >
                Confirm {pending?.op}
              </Button>
            </>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );

  return { start, busy, dialog };
}

function PkgList({ label, variant, names }: { label: string; variant: "success" | "default" | "destructive"; names: string[] | null }) {
  if (!names?.length) return null;
  return (
    <div>
      <div className="mb-1 font-medium">
        {label} ({names.length})
      </div>
      <div className="flex flex-wrap gap-1 max-h-32 overflow-y-auto">
        {names.map((n) => (
          <Badge key={n} variant={variant} className="font-mono">{n}</Badge>
        ))}
      </div>
    </div>
  );
}
