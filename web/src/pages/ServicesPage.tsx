import { useEffect, useMemo, useRef, useState } from "react";
import {
  Activity,
  CircleAlert,
  CircleCheck,
  CircleDashed,
  Play,
  RotateCw,
  Search,
  Square,
  Terminal,
} from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { api, ApiError, wsURL } from "@/lib/api";
import { toast } from "@/components/ui/toast";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

interface Unit {
  name: string;
  description: string;
  load_state: string;
  active_state: string;
  sub_state: string;
}

export function ServicesPage() {
  const [units, setUnits] = useState<Unit[]>([]);
  const [filter, setFilter] = useState("");
  const [busy, setBusy] = useState<string | null>(null);
  const [journal, setJournal] = useState<string | null>(null);

  const load = async () => {
    try {
      const list = await api.get<Unit[]>("/api/services");
      setUnits(list);
    } catch (e) {
      if (e instanceof ApiError) toast.error("Failed to load services", e.message);
    }
  };

  useEffect(() => {
    void load();
    const t = setInterval(load, 7000);
    return () => clearInterval(t);
  }, []);

  const filtered = useMemo(() => {
    const q = filter.trim().toLowerCase();
    if (!q) return units;
    return units.filter(
      (u) => u.name.toLowerCase().includes(q) || u.description.toLowerCase().includes(q),
    );
  }, [units, filter]);

  const act = async (name: string, action: string) => {
    setBusy(name + action);
    try {
      await api.post(`/api/services/${encodeURIComponent(name)}/action`, { action });
      toast.success(`${action} ✓`, name);
      await load();
    } catch (e) {
      if (e instanceof ApiError) toast.error(`${action} failed`, e.message);
    } finally {
      setBusy(null);
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-end justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Services</h1>
          <p className="text-sm text-muted-foreground">
            {units.length} systemd units · {units.filter((u) => u.active_state === "active").length}{" "}
            active
          </p>
        </div>
        <div className="relative w-72 max-w-full">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input
            placeholder="Filter units…"
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            className="pl-9"
          />
        </div>
      </div>

      <Card>
        <CardContent className="p-0">
          <div className="divide-y divide-border">
            <div className="grid grid-cols-12 gap-3 px-5 py-2.5 text-xs uppercase tracking-wider text-muted-foreground">
              <div className="col-span-5">Unit</div>
              <div className="col-span-4">Description</div>
              <div className="col-span-1">State</div>
              <div className="col-span-2 text-right">Actions</div>
            </div>
            {filtered.map((u) => (
              <div
                key={u.name}
                className="grid grid-cols-12 gap-3 items-center px-5 py-3 hover:bg-accent/30 transition-colors"
              >
                <div className="col-span-5 min-w-0">
                  <div className="flex items-center gap-2 font-medium truncate">
                    <StateIcon active={u.active_state} sub={u.sub_state} />
                    <span className="truncate">{u.name}</span>
                  </div>
                  <div className="text-xs text-muted-foreground">
                    load: {u.load_state} · sub: {u.sub_state}
                  </div>
                </div>
                <div className="col-span-4 text-sm text-muted-foreground truncate">
                  {u.description || "—"}
                </div>
                <div className="col-span-1">
                  <StateBadge active={u.active_state} />
                </div>
                <div className="col-span-2 flex justify-end gap-1.5">
                  <Button
                    size="icon"
                    variant="ghost"
                    title="Logs"
                    onClick={() => setJournal(u.name)}
                  >
                    <Terminal className="h-4 w-4" />
                  </Button>
                  <Button
                    size="icon"
                    variant="ghost"
                    title="Start"
                    disabled={!!busy}
                    onClick={() => act(u.name, "start")}
                  >
                    <Play className="h-4 w-4" />
                  </Button>
                  <Button
                    size="icon"
                    variant="ghost"
                    title="Restart"
                    disabled={!!busy}
                    onClick={() => act(u.name, "restart")}
                  >
                    <RotateCw className="h-4 w-4" />
                  </Button>
                  <Button
                    size="icon"
                    variant="ghost"
                    title="Stop"
                    disabled={!!busy}
                    onClick={() => act(u.name, "stop")}
                  >
                    <Square className="h-4 w-4" />
                  </Button>
                </div>
              </div>
            ))}
            {filtered.length === 0 && (
              <div className="px-5 py-10 text-center text-sm text-muted-foreground">
                No units match the filter.
              </div>
            )}
          </div>
        </CardContent>
      </Card>

      <Dialog open={!!journal} onOpenChange={(o) => !o && setJournal(null)}>
        <DialogContent className="max-w-3xl">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <Activity className="h-4 w-4 text-primary" />
              {journal} — journal
            </DialogTitle>
          </DialogHeader>
          {journal && <JournalView unit={journal} />}
        </DialogContent>
      </Dialog>
    </div>
  );
}

function StateIcon({ active, sub }: { active: string; sub: string }) {
  if (active === "active" && sub === "running") {
    return <CircleCheck className="h-4 w-4 text-success" />;
  }
  if (active === "failed") return <CircleAlert className="h-4 w-4 text-destructive" />;
  if (active === "inactive") return <CircleDashed className="h-4 w-4 text-muted-foreground" />;
  return <CircleDashed className="h-4 w-4 text-warning" />;
}

function StateBadge({ active }: { active: string }) {
  if (active === "active") return <Badge variant="success">active</Badge>;
  if (active === "failed") return <Badge variant="destructive">failed</Badge>;
  if (active === "inactive") return <Badge variant="muted">inactive</Badge>;
  return <Badge variant="warning">{active}</Badge>;
}

function JournalView({ unit }: { unit: string }) {
  const [lines, setLines] = useState<string[]>([]);
  const ref = useRef<HTMLDivElement | null>(null);
  useEffect(() => {
    const ws = new WebSocket(
      wsURL(`/api/services/${encodeURIComponent(unit)}/journal/stream`),
    );
    ws.onmessage = (e) => {
      try {
        const m = JSON.parse(e.data as string) as { line?: string; error?: string };
        if (m.line) setLines((cur) => [...cur.slice(-500), m.line!]);
        if (m.error) setLines((cur) => [...cur, `[error] ${m.error}`]);
      } catch {
        /* ignore */
      }
    };
    return () => ws.close();
  }, [unit]);

  useEffect(() => {
    ref.current?.scrollTo({ top: ref.current.scrollHeight });
  }, [lines]);

  return (
    <div
      ref={ref}
      className="h-[60vh] overflow-y-auto rounded-md border bg-black/60 p-3 font-mono text-xs leading-relaxed text-green-300/90"
    >
      {lines.length === 0 ? (
        <span className="text-muted-foreground">Waiting for journal output…</span>
      ) : (
        lines.map((l, i) => <div key={i}>{l}</div>)
      )}
    </div>
  );
}
