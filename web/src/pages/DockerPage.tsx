import { useEffect, useMemo, useRef, useState } from "react";
import {
  Activity,
  Boxes,
  Container,
  Pause,
  Play,
  RotateCw,
  Square,
  Terminal,
} from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { api, ApiError, wsURL } from "@/lib/api";
import { toast } from "@/components/ui/toast";
import { useCanWrite } from "@/store/auth";
import { formatBytes } from "@/lib/utils";

interface DContainer {
  Id: string;
  Names: string[];
  Image: string;
  Command: string;
  Created: number;
  State: string;
  Status: string;
  Ports?: { PrivatePort: number; PublicPort?: number; Type: string; IP?: string }[];
}

interface DImage {
  Id: string;
  RepoTags?: string[];
  Created: number;
  Size: number;
}

export function DockerPage() {
  const [tab, setTab] = useState<"containers" | "images">("containers");
  const [filter, setFilter] = useState("");
  const [containers, setContainers] = useState<DContainer[]>([]);
  const [images, setImages] = useState<DImage[]>([]);
  const [unavailable, setUnavailable] = useState(false);
  const [logsFor, setLogsFor] = useState<DContainer | null>(null);
  const [busy, setBusy] = useState<string | null>(null);
  const canWrite = useCanWrite();

  const load = async () => {
    try {
      const [c, i] = await Promise.all([
        api.get<DContainer[]>("/api/docker/containers"),
        api.get<DImage[]>("/api/docker/images"),
      ]);
      setContainers(c ?? []);
      setImages(i ?? []);
      setUnavailable(false);
    } catch (e) {
      if (e instanceof ApiError && e.status === 503) {
        setUnavailable(true);
        return;
      }
      if (e instanceof ApiError) toast.error("Failed to load docker", e.message);
    }
  };

  useEffect(() => {
    void load();
    const t = setInterval(load, 7000);
    return () => clearInterval(t);
  }, []);

  const filteredContainers = useMemo(() => {
    const q = filter.trim().toLowerCase();
    if (!q) return containers;
    return containers.filter(
      (c) =>
        c.Names.some((n) => n.toLowerCase().includes(q)) ||
        c.Image.toLowerCase().includes(q),
    );
  }, [containers, filter]);

  const act = async (c: DContainer, action: string) => {
    setBusy(c.Id + action);
    try {
      await api.post(`/api/docker/containers/${c.Id}/action`, { action });
      toast.success(`${action} ✓`, c.Names[0] ?? c.Id.slice(0, 12));
      await load();
    } catch (err) {
      if (err instanceof ApiError) toast.error(`${action} failed`, err.message);
    } finally {
      setBusy(null);
    }
  };

  if (unavailable) {
    return (
      <Card>
        <CardContent className="p-6 text-sm">
          <Container className="h-6 w-6 text-muted-foreground mb-3" />
          <strong>Docker is not running.</strong>
          <p className="mt-2 text-muted-foreground">
            The socket at <code>/var/run/docker.sock</code> isn't reachable. Install Docker and start the daemon, then refresh.
          </p>
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-end justify-between gap-4 flex-wrap">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Docker</h1>
          <p className="text-sm text-muted-foreground">
            {containers.length} containers · {containers.filter((c) => c.State === "running").length} running · {images.length} images
          </p>
        </div>
        <div className="flex gap-2">
          <div className="rounded-md border border-border p-0.5 flex gap-0.5">
            {(["containers", "images"] as const).map((t) => (
              <button
                key={t}
                onClick={() => setTab(t)}
                className={`px-3 py-1.5 text-sm rounded-sm transition-colors ${
                  tab === t ? "bg-primary/10 text-primary" : "text-muted-foreground hover:text-foreground"
                }`}
              >
                {t}
              </button>
            ))}
          </div>
          <Input
            placeholder="Filter…"
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            className="w-56"
          />
        </div>
      </div>

      {tab === "containers" ? (
        <Card>
          <CardContent className="p-0">
            <div className="divide-y divide-border">
              <div className="grid grid-cols-12 gap-3 px-5 py-2.5 text-xs uppercase tracking-wider text-muted-foreground">
                <div className="col-span-3">Name</div>
                <div className="col-span-3">Image</div>
                <div className="col-span-2">State</div>
                <div className="col-span-2">Ports</div>
                <div className="col-span-2 text-right">Actions</div>
              </div>
              {filteredContainers.map((c) => (
                <div
                  key={c.Id}
                  className="grid grid-cols-12 gap-3 items-center px-5 py-3 hover:bg-accent/30 transition-colors"
                >
                  <div className="col-span-3 min-w-0">
                    <div className="font-medium truncate">
                      {c.Names[0]?.replace(/^\//, "") ?? c.Id.slice(0, 12)}
                    </div>
                    <div className="text-xs text-muted-foreground font-mono truncate">
                      {c.Id.slice(0, 12)}
                    </div>
                  </div>
                  <div className="col-span-3 text-sm text-muted-foreground truncate font-mono">
                    {c.Image}
                  </div>
                  <div className="col-span-2">
                    <StateBadge state={c.State} status={c.Status} />
                  </div>
                  <div className="col-span-2 text-xs text-muted-foreground font-mono truncate">
                    {portList(c)}
                  </div>
                  <div className="col-span-2 flex justify-end gap-1">
                    <Button size="icon" variant="ghost" title="Logs" onClick={() => setLogsFor(c)}>
                      <Terminal className="h-4 w-4" />
                    </Button>
                    <Button
                      size="icon"
                      variant="ghost"
                      title={canWrite ? "Start" : "Read-only"}
                      disabled={!!busy || !canWrite || c.State === "running"}
                      onClick={() => act(c, "start")}
                    >
                      <Play className="h-4 w-4" />
                    </Button>
                    <Button
                      size="icon"
                      variant="ghost"
                      title={canWrite ? "Restart" : "Read-only"}
                      disabled={!!busy || !canWrite}
                      onClick={() => act(c, "restart")}
                    >
                      <RotateCw className="h-4 w-4" />
                    </Button>
                    {c.State === "running" ? (
                      <Button
                        size="icon"
                        variant="ghost"
                        title={canWrite ? "Stop" : "Read-only"}
                        disabled={!!busy || !canWrite}
                        onClick={() => act(c, "stop")}
                      >
                        <Square className="h-4 w-4" />
                      </Button>
                    ) : null}
                    {c.State === "paused" ? (
                      <Button
                        size="icon"
                        variant="ghost"
                        title="Unpause"
                        disabled={!!busy || !canWrite}
                        onClick={() => act(c, "unpause")}
                      >
                        <Play className="h-4 w-4" />
                      </Button>
                    ) : c.State === "running" ? (
                      <Button
                        size="icon"
                        variant="ghost"
                        title="Pause"
                        disabled={!!busy || !canWrite}
                        onClick={() => act(c, "pause")}
                      >
                        <Pause className="h-4 w-4" />
                      </Button>
                    ) : null}
                  </div>
                </div>
              ))}
              {filteredContainers.length === 0 && (
                <div className="p-8 text-center text-sm text-muted-foreground">
                  No containers match.
                </div>
              )}
            </div>
          </CardContent>
        </Card>
      ) : (
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Boxes className="h-4 w-4 text-primary" /> Images
            </CardTitle>
          </CardHeader>
          <CardContent className="p-0">
            <div className="divide-y divide-border">
              <div className="grid grid-cols-12 gap-3 px-5 py-2.5 text-xs uppercase tracking-wider text-muted-foreground">
                <div className="col-span-5">Repo:Tag</div>
                <div className="col-span-3">ID</div>
                <div className="col-span-2 text-right">Size</div>
                <div className="col-span-2">Created</div>
              </div>
              {images.map((im) => (
                <div
                  key={im.Id}
                  className="grid grid-cols-12 gap-3 items-center px-5 py-3 hover:bg-accent/30 transition-colors"
                >
                  <div className="col-span-5 font-mono text-sm truncate">
                    {im.RepoTags?.[0] ?? "<none>"}
                  </div>
                  <div className="col-span-3 font-mono text-xs text-muted-foreground truncate">
                    {im.Id.replace(/^sha256:/, "").slice(0, 12)}
                  </div>
                  <div className="col-span-2 text-right tabular-nums text-sm">
                    {formatBytes(im.Size)}
                  </div>
                  <div className="col-span-2 text-xs text-muted-foreground">
                    {new Date(im.Created * 1000).toLocaleDateString()}
                  </div>
                </div>
              ))}
              {images.length === 0 && (
                <div className="p-8 text-center text-sm text-muted-foreground">No images.</div>
              )}
            </div>
          </CardContent>
        </Card>
      )}

      <Dialog open={!!logsFor} onOpenChange={(o) => !o && setLogsFor(null)}>
        <DialogContent className="max-w-4xl">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <Activity className="h-4 w-4 text-primary" />
              {logsFor?.Names[0]?.replace(/^\//, "")} — logs
            </DialogTitle>
          </DialogHeader>
          {logsFor && <LogsView id={logsFor.Id} />}
        </DialogContent>
      </Dialog>
    </div>
  );
}

function StateBadge({ state, status }: { state: string; status: string }) {
  const title = status;
  if (state === "running") return <Badge variant="success" title={title}>running</Badge>;
  if (state === "exited") return <Badge variant="muted" title={title}>exited</Badge>;
  if (state === "paused") return <Badge variant="warning" title={title}>paused</Badge>;
  if (state === "created") return <Badge variant="muted" title={title}>created</Badge>;
  if (state === "dead" || state === "restarting") return <Badge variant="destructive" title={title}>{state}</Badge>;
  return <Badge variant="muted" title={title}>{state}</Badge>;
}

function portList(c: DContainer): string {
  if (!c.Ports?.length) return "—";
  return c.Ports.slice(0, 3)
    .map((p) =>
      p.PublicPort
        ? `${p.PublicPort}→${p.PrivatePort}/${p.Type}`
        : `${p.PrivatePort}/${p.Type}`,
    )
    .join(", ");
}

function LogsView({ id }: { id: string }) {
  const [lines, setLines] = useState<string[]>([]);
  const ref = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    const ws = new WebSocket(wsURL(`/api/docker/containers/${id}/logs/stream?tail=200`));
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
  }, [id]);

  useEffect(() => {
    ref.current?.scrollTo({ top: ref.current.scrollHeight });
  }, [lines]);

  return (
    <div
      ref={ref}
      className="h-[60vh] overflow-y-auto rounded-md border bg-black/60 p-3 font-mono text-xs leading-relaxed text-green-300/90"
    >
      {lines.length === 0 ? (
        <span className="text-muted-foreground">Waiting for log output…</span>
      ) : (
        lines.map((l, i) => <div key={i}>{l}</div>)
      )}
    </div>
  );
}
