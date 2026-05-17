import { useEffect, useMemo, useRef, useState, type FormEvent } from "react";
import { Terminal as XTerm } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";
import {
  Activity,
  Boxes,
  Container as ContainerIcon,
  Download,
  HardDrive,
  Network as NetworkIcon,
  Pause,
  Play,
  Plus,
  RotateCw,
  Square,
  Terminal,
  Trash2,
} from "lucide-react";
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
import { api, ApiError, wsURL } from "@/lib/api";
import { toast } from "@/components/ui/toast";
import { useCanWrite, useIsAdmin } from "@/store/auth";
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

interface DNetwork {
  Id: string;
  Name: string;
  Driver: string;
  Scope: string;
  Internal: boolean;
  Attachable: boolean;
}

interface DVolume {
  Name: string;
  Driver: string;
  Mountpoint: string;
  CreatedAt: string;
}

interface DInfo {
  Containers: number;
  ContainersRunning: number;
  ContainersStopped: number;
  Images: number;
  Driver: string;
  KernelVersion: string;
  OperatingSystem: string;
  Architecture: string;
  NCPU: number;
  MemTotal: number;
  ServerVersion: string;
  DockerRootDir: string;
}

type Tab = "containers" | "images" | "networks" | "volumes" | "system";

export function DockerPage() {
  const [tab, setTab] = useState<Tab>("containers");
  const [unavailable, setUnavailable] = useState(false);
  const [info, setInfo] = useState<DInfo | null>(null);

  useEffect(() => {
    void api.get<DInfo>("/api/docker/info")
      .then(setInfo)
      .catch((e) => {
        if (e instanceof ApiError && e.status === 503) setUnavailable(true);
      });
  }, []);

  if (unavailable) {
    return (
      <Card>
        <CardContent className="p-6 text-sm">
          <ContainerIcon className="h-6 w-6 text-muted-foreground mb-3" />
          <strong>Docker is not running.</strong>
          <p className="mt-2 text-muted-foreground">
            The socket at <code>/var/run/docker.sock</code> isn't reachable.
            Install Docker and start the daemon, then refresh.
          </p>
        </CardContent>
      </Card>
    );
  }

  const tabs: { id: Tab; label: string; icon: typeof ContainerIcon }[] = [
    { id: "containers", label: "Containers", icon: ContainerIcon },
    { id: "images", label: "Images", icon: Boxes },
    { id: "networks", label: "Networks", icon: NetworkIcon },
    { id: "volumes", label: "Volumes", icon: HardDrive },
    { id: "system", label: "System", icon: Activity },
  ];

  return (
    <div className="space-y-6">
      <div className="flex items-end justify-between gap-4 flex-wrap">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Docker</h1>
          {info && (
            <p className="text-sm text-muted-foreground">
              {info.ServerVersion} · {info.Driver} · {info.NCPU} cores · {formatBytes(info.MemTotal)} · {info.ContainersRunning}/{info.Containers} containers running
            </p>
          )}
        </div>
        <div className="rounded-md border border-border p-0.5 flex gap-0.5">
          {tabs.map((t) => (
            <button
              key={t.id}
              onClick={() => setTab(t.id)}
              className={`px-3 py-1.5 text-sm rounded-sm transition-colors flex items-center gap-1.5 ${
                tab === t.id ? "bg-primary/10 text-primary" : "text-muted-foreground hover:text-foreground"
              }`}
            >
              <t.icon className="h-3.5 w-3.5" />
              {t.label}
            </button>
          ))}
        </div>
      </div>

      {tab === "containers" && <ContainersTab />}
      {tab === "images" && <ImagesTab />}
      {tab === "networks" && <NetworksTab />}
      {tab === "volumes" && <VolumesTab />}
      {tab === "system" && <SystemTab info={info} />}
    </div>
  );
}

function ContainersTab() {
  const canWrite = useCanWrite();
  const [containers, setContainers] = useState<DContainer[]>([]);
  const [filter, setFilter] = useState("");
  const [busy, setBusy] = useState<string | null>(null);
  const [logsFor, setLogsFor] = useState<DContainer | null>(null);
  const [statsFor, setStatsFor] = useState<DContainer | null>(null);
  const [execFor, setExecFor] = useState<DContainer | null>(null);
  const [createOpen, setCreateOpen] = useState(false);

  const load = async () => {
    try {
      setContainers(await api.get<DContainer[]>("/api/docker/containers"));
    } catch (e) {
      if (e instanceof ApiError) toast.error("Failed", e.message);
    }
  };
  useEffect(() => {
    void load();
    const t = setInterval(load, 5000);
    return () => clearInterval(t);
  }, []);

  const filtered = useMemo(() => {
    const q = filter.trim().toLowerCase();
    if (!q) return containers;
    return containers.filter(
      (c) => c.Names.some((n) => n.toLowerCase().includes(q)) || c.Image.toLowerCase().includes(q),
    );
  }, [containers, filter]);

  const act = async (c: DContainer, action: string) => {
    setBusy(c.Id + action);
    try {
      await api.post(`/api/docker/containers/${c.Id}/action`, { action });
      toast.success(`${action} ✓`, c.Names[0] ?? c.Id.slice(0, 12));
      await load();
    } catch (e) {
      if (e instanceof ApiError) toast.error(`${action} failed`, e.message);
    } finally {
      setBusy(null);
    }
  };

  const remove = async (c: DContainer) => {
    const name = c.Names[0]?.replace(/^\//, "") ?? c.Id.slice(0, 12);
    const force = c.State === "running";
    if (!confirm(`Remove container "${name}"?${force ? "\n\nIt's running — will be force-killed." : ""}`)) return;
    try {
      await api.del(`/api/docker/containers/${c.Id}${force ? "?force=1" : ""}`);
      toast.success("Removed", name);
      await load();
    } catch (e) {
      if (e instanceof ApiError) toast.error("Remove failed", e.message);
    }
  };

  return (
    <>
      <div className="flex justify-between items-center gap-4 flex-wrap">
        <Input
          placeholder="Filter by name or image…"
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          className="w-64"
        />
        {canWrite && (
          <Button onClick={() => setCreateOpen(true)}>
            <Plus className="h-4 w-4" /> New container
          </Button>
        )}
      </div>

      <Card>
        <CardContent className="p-0">
          <div className="divide-y divide-border">
            <div className="grid grid-cols-12 gap-3 px-5 py-2.5 text-xs uppercase tracking-wider text-muted-foreground">
              <div className="col-span-3">Name</div>
              <div className="col-span-3">Image</div>
              <div className="col-span-1">State</div>
              <div className="col-span-2">Ports</div>
              <div className="col-span-3 text-right">Actions</div>
            </div>
            {filtered.map((c) => (
              <div
                key={c.Id}
                className="grid grid-cols-12 gap-3 items-center px-5 py-3 hover:bg-accent/30 transition-colors"
              >
                <div className="col-span-3 min-w-0">
                  <div className="font-medium truncate">{c.Names[0]?.replace(/^\//, "") ?? c.Id.slice(0, 12)}</div>
                  <div className="text-xs text-muted-foreground font-mono truncate">{c.Id.slice(0, 12)}</div>
                </div>
                <div className="col-span-3 text-sm text-muted-foreground truncate font-mono">{c.Image}</div>
                <div className="col-span-1">
                  <StateBadge state={c.State} status={c.Status} />
                </div>
                <div className="col-span-2 text-xs text-muted-foreground font-mono truncate">{portList(c)}</div>
                <div className="col-span-3 flex justify-end gap-0.5">
                  <Button size="icon" variant="ghost" title="Logs" onClick={() => setLogsFor(c)}>
                    <Terminal className="h-4 w-4" />
                  </Button>
                  <Button size="icon" variant="ghost" title="Stats" onClick={() => setStatsFor(c)} disabled={c.State !== "running"}>
                    <Activity className="h-4 w-4" />
                  </Button>
                  {canWrite && (
                    <Button size="icon" variant="ghost" title="Console (exec)" onClick={() => setExecFor(c)} disabled={c.State !== "running"}>
                      <ContainerIcon className="h-4 w-4" />
                    </Button>
                  )}
                  <Button size="icon" variant="ghost" title="Start" disabled={!!busy || !canWrite || c.State === "running"} onClick={() => act(c, "start")}>
                    <Play className="h-4 w-4" />
                  </Button>
                  <Button size="icon" variant="ghost" title="Restart" disabled={!!busy || !canWrite} onClick={() => act(c, "restart")}>
                    <RotateCw className="h-4 w-4" />
                  </Button>
                  {c.State === "running" && (
                    <Button size="icon" variant="ghost" title="Stop" disabled={!!busy || !canWrite} onClick={() => act(c, "stop")}>
                      <Square className="h-4 w-4" />
                    </Button>
                  )}
                  {c.State === "paused" ? (
                    <Button size="icon" variant="ghost" title="Unpause" disabled={!!busy || !canWrite} onClick={() => act(c, "unpause")}>
                      <Play className="h-4 w-4" />
                    </Button>
                  ) : c.State === "running" ? (
                    <Button size="icon" variant="ghost" title="Pause" disabled={!!busy || !canWrite} onClick={() => act(c, "pause")}>
                      <Pause className="h-4 w-4" />
                    </Button>
                  ) : null}
                  {canWrite && (
                    <Button size="icon" variant="ghost" title="Remove" onClick={() => remove(c)}>
                      <Trash2 className="h-4 w-4 text-destructive" />
                    </Button>
                  )}
                </div>
              </div>
            ))}
            {filtered.length === 0 && (
              <div className="p-8 text-center text-sm text-muted-foreground">No containers match.</div>
            )}
          </div>
        </CardContent>
      </Card>

      <LogsDialog target={logsFor} onClose={() => setLogsFor(null)} />
      <StatsDialog target={statsFor} onClose={() => setStatsFor(null)} />
      <ExecDialog target={execFor} onClose={() => setExecFor(null)} />
      <CreateContainerDialog open={createOpen} onClose={() => setCreateOpen(false)} onCreated={load} />
    </>
  );
}

function ImagesTab() {
  const canWrite = useCanWrite();
  const [images, setImages] = useState<DImage[]>([]);
  const [pullOpen, setPullOpen] = useState(false);

  const load = async () => {
    try {
      setImages(await api.get<DImage[]>("/api/docker/images"));
    } catch (e) {
      if (e instanceof ApiError) toast.error("Failed", e.message);
    }
  };
  useEffect(() => { void load(); }, []);

  const remove = async (im: DImage) => {
    const ref = im.RepoTags?.[0] ?? im.Id;
    if (!confirm(`Remove image ${ref}?`)) return;
    try {
      await api.del(`/api/docker/images/${encodeURIComponent(ref)}`);
      toast.success("Removed", ref);
      await load();
    } catch (e) {
      if (e instanceof ApiError) toast.error("Remove failed", e.message);
    }
  };

  return (
    <>
      <div className="flex justify-end">
        {canWrite && (
          <Button onClick={() => setPullOpen(true)}>
            <Download className="h-4 w-4" /> Pull image
          </Button>
        )}
      </div>
      <Card>
        <CardContent className="p-0">
          <div className="divide-y divide-border">
            <div className="grid grid-cols-12 gap-3 px-5 py-2.5 text-xs uppercase tracking-wider text-muted-foreground">
              <div className="col-span-5">Repo:Tag</div>
              <div className="col-span-3">ID</div>
              <div className="col-span-2 text-right">Size</div>
              <div className="col-span-1">Created</div>
              <div className="col-span-1 text-right">Actions</div>
            </div>
            {images.map((im) => (
              <div key={im.Id} className="grid grid-cols-12 gap-3 items-center px-5 py-3 hover:bg-accent/30 transition-colors">
                <div className="col-span-5 font-mono text-sm truncate">{im.RepoTags?.[0] ?? "<none>"}</div>
                <div className="col-span-3 font-mono text-xs text-muted-foreground truncate">
                  {im.Id.replace(/^sha256:/, "").slice(0, 12)}
                </div>
                <div className="col-span-2 text-right tabular-nums text-sm">{formatBytes(im.Size)}</div>
                <div className="col-span-1 text-xs text-muted-foreground">
                  {new Date(im.Created * 1000).toLocaleDateString()}
                </div>
                <div className="col-span-1 flex justify-end">
                  {canWrite && (
                    <Button variant="ghost" size="icon" onClick={() => remove(im)}>
                      <Trash2 className="h-4 w-4 text-destructive" />
                    </Button>
                  )}
                </div>
              </div>
            ))}
            {images.length === 0 && (
              <div className="p-8 text-center text-sm text-muted-foreground">No images.</div>
            )}
          </div>
        </CardContent>
      </Card>
      <PullImageDialog open={pullOpen} onClose={() => setPullOpen(false)} onPulled={load} />
    </>
  );
}

function NetworksTab() {
  const canWrite = useCanWrite();
  const [networks, setNetworks] = useState<DNetwork[]>([]);
  const [open, setOpen] = useState(false);

  const load = async () => {
    try {
      setNetworks(await api.get<DNetwork[]>("/api/docker/networks"));
    } catch (e) {
      if (e instanceof ApiError) toast.error("Failed", e.message);
    }
  };
  useEffect(() => { void load(); }, []);

  const remove = async (n: DNetwork) => {
    if (!confirm(`Remove network "${n.Name}"?`)) return;
    try {
      await api.del(`/api/docker/networks/${encodeURIComponent(n.Id)}`);
      toast.success("Removed", n.Name);
      await load();
    } catch (e) {
      if (e instanceof ApiError) toast.error("Remove failed", e.message);
    }
  };

  return (
    <>
      <div className="flex justify-end">
        {canWrite && (
          <Button onClick={() => setOpen(true)}>
            <Plus className="h-4 w-4" /> New network
          </Button>
        )}
      </div>
      <Card>
        <CardContent className="p-0">
          <div className="divide-y divide-border">
            <div className="grid grid-cols-12 gap-3 px-5 py-2.5 text-xs uppercase tracking-wider text-muted-foreground">
              <div className="col-span-4">Name</div>
              <div className="col-span-2">Driver</div>
              <div className="col-span-2">Scope</div>
              <div className="col-span-3 font-mono">ID</div>
              <div className="col-span-1 text-right">Actions</div>
            </div>
            {networks.map((n) => (
              <div key={n.Id} className="grid grid-cols-12 gap-3 items-center px-5 py-3 hover:bg-accent/30 transition-colors">
                <div className="col-span-4 font-medium truncate">{n.Name}</div>
                <div className="col-span-2">
                  <Badge variant="muted">{n.Driver}</Badge>
                </div>
                <div className="col-span-2 text-sm text-muted-foreground">{n.Scope}</div>
                <div className="col-span-3 font-mono text-xs text-muted-foreground truncate">{n.Id.slice(0, 12)}</div>
                <div className="col-span-1 flex justify-end">
                  {canWrite && !["host", "bridge", "none"].includes(n.Name) && (
                    <Button variant="ghost" size="icon" onClick={() => remove(n)}>
                      <Trash2 className="h-4 w-4 text-destructive" />
                    </Button>
                  )}
                </div>
              </div>
            ))}
            {networks.length === 0 && (
              <div className="p-8 text-center text-sm text-muted-foreground">No networks.</div>
            )}
          </div>
        </CardContent>
      </Card>
      <CreateNetworkDialog open={open} onClose={() => setOpen(false)} onCreated={load} />
    </>
  );
}

function VolumesTab() {
  const canWrite = useCanWrite();
  const [volumes, setVolumes] = useState<DVolume[]>([]);
  const [open, setOpen] = useState(false);

  const load = async () => {
    try {
      setVolumes((await api.get<DVolume[]>("/api/docker/volumes")) ?? []);
    } catch (e) {
      if (e instanceof ApiError) toast.error("Failed", e.message);
    }
  };
  useEffect(() => { void load(); }, []);

  const remove = async (v: DVolume) => {
    if (!confirm(`Remove volume "${v.Name}"? Its data on the host will be deleted.`)) return;
    try {
      await api.del(`/api/docker/volumes/${encodeURIComponent(v.Name)}`);
      toast.success("Removed", v.Name);
      await load();
    } catch (e) {
      if (e instanceof ApiError) toast.error("Remove failed", e.message);
    }
  };

  return (
    <>
      <div className="flex justify-end">
        {canWrite && (
          <Button onClick={() => setOpen(true)}>
            <Plus className="h-4 w-4" /> New volume
          </Button>
        )}
      </div>
      <Card>
        <CardContent className="p-0">
          <div className="divide-y divide-border">
            <div className="grid grid-cols-12 gap-3 px-5 py-2.5 text-xs uppercase tracking-wider text-muted-foreground">
              <div className="col-span-3">Name</div>
              <div className="col-span-2">Driver</div>
              <div className="col-span-5 font-mono">Mountpoint</div>
              <div className="col-span-1">Created</div>
              <div className="col-span-1 text-right">Actions</div>
            </div>
            {volumes.map((v) => (
              <div key={v.Name} className="grid grid-cols-12 gap-3 items-center px-5 py-3 hover:bg-accent/30 transition-colors">
                <div className="col-span-3 font-medium truncate">{v.Name}</div>
                <div className="col-span-2">
                  <Badge variant="muted">{v.Driver}</Badge>
                </div>
                <div className="col-span-5 font-mono text-xs text-muted-foreground truncate">{v.Mountpoint}</div>
                <div className="col-span-1 text-xs text-muted-foreground">{v.CreatedAt ? new Date(v.CreatedAt).toLocaleDateString() : "—"}</div>
                <div className="col-span-1 flex justify-end">
                  {canWrite && (
                    <Button variant="ghost" size="icon" onClick={() => remove(v)}>
                      <Trash2 className="h-4 w-4 text-destructive" />
                    </Button>
                  )}
                </div>
              </div>
            ))}
            {volumes.length === 0 && (
              <div className="p-8 text-center text-sm text-muted-foreground">No volumes.</div>
            )}
          </div>
        </CardContent>
      </Card>
      <CreateVolumeDialog open={open} onClose={() => setOpen(false)} onCreated={load} />
    </>
  );
}

function SystemTab({ info }: { info: DInfo | null }) {
  const isAdmin = useIsAdmin();
  const [df, setDF] = useState<{
    Containers?: any[];
    Images?: any[];
    Volumes?: any[];
    BuildCache?: any[];
    LayersSize?: number;
  } | null>(null);

  const load = async () => {
    try {
      setDF(await api.get("/api/docker/df"));
    } catch (e) {
      if (e instanceof ApiError) toast.error("Failed", e.message);
    }
  };
  useEffect(() => { void load(); }, []);

  const prune = async (target: string) => {
    if (!confirm(`Prune unused ${target}? This deletes anything not currently in use.`)) return;
    try {
      await api.post("/api/docker/prune", { target });
      toast.success(`Pruned ${target}`);
      await load();
    } catch (e) {
      if (e instanceof ApiError) toast.error("Prune failed", e.message);
    }
  };

  const sizeOf = (arr?: any[]) =>
    (arr ?? []).reduce((s, x) => s + (x.Size ?? x.SharedSize ?? 0), 0);

  return (
    <div className="grid gap-4 lg:grid-cols-2">
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Activity className="h-4 w-4 text-primary" /> Engine
          </CardTitle>
        </CardHeader>
        <CardContent>
          <dl className="grid grid-cols-2 gap-x-6 gap-y-2 text-sm">
            <KV k="Version" v={info?.ServerVersion} />
            <KV k="Storage driver" v={info?.Driver} />
            <KV k="Kernel" v={info?.KernelVersion} />
            <KV k="OS" v={info?.OperatingSystem} />
            <KV k="Architecture" v={info?.Architecture} />
            <KV k="CPUs" v={info?.NCPU?.toString()} />
            <KV k="Memory" v={info ? formatBytes(info.MemTotal) : undefined} />
            <KV k="Root dir" v={info?.DockerRootDir} />
          </dl>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <HardDrive className="h-4 w-4 text-primary" /> Disk usage
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <DFRow label="Containers" size={sizeOf(df?.Containers)} count={df?.Containers?.length} onPrune={isAdmin ? () => prune("containers") : undefined} />
          <DFRow label="Images" size={sizeOf(df?.Images)} count={df?.Images?.length} onPrune={isAdmin ? () => prune("images") : undefined} />
          <DFRow label="Volumes" size={sizeOf(df?.Volumes)} count={df?.Volumes?.length} onPrune={isAdmin ? () => prune("volumes") : undefined} />
          <DFRow label="Networks (unused)" size={0} count={0} onPrune={isAdmin ? () => prune("networks") : undefined} pruneLabel="Prune networks" />
        </CardContent>
      </Card>
    </div>
  );
}

function DFRow({ label, size, count, onPrune, pruneLabel }: {
  label: string; size: number; count?: number; onPrune?: () => void; pruneLabel?: string;
}) {
  return (
    <div className="flex items-center justify-between gap-3 rounded-md border border-border px-3 py-2">
      <div>
        <div className="text-sm font-medium">{label}</div>
        <div className="text-xs text-muted-foreground">
          {count ?? 0} objects · {formatBytes(size)}
        </div>
      </div>
      {onPrune && (
        <Button size="sm" variant="outline" onClick={onPrune}>
          {pruneLabel ?? `Prune ${label.toLowerCase()}`}
        </Button>
      )}
    </div>
  );
}

function KV({ k, v }: { k: string; v?: string }) {
  return (
    <div>
      <dt className="text-xs uppercase tracking-wider text-muted-foreground">{k}</dt>
      <dd className="mt-0.5 truncate">{v || "—"}</dd>
    </div>
  );
}

function StateBadge({ state, status }: { state: string; status: string }) {
  if (state === "running") return <Badge variant="success" title={status}>running</Badge>;
  if (state === "exited") return <Badge variant="muted" title={status}>exited</Badge>;
  if (state === "paused") return <Badge variant="warning" title={status}>paused</Badge>;
  if (state === "created") return <Badge variant="muted" title={status}>created</Badge>;
  if (state === "dead" || state === "restarting") return <Badge variant="destructive" title={status}>{state}</Badge>;
  return <Badge variant="muted" title={status}>{state}</Badge>;
}

function portList(c: DContainer): string {
  if (!c.Ports?.length) return "—";
  return c.Ports.slice(0, 3)
    .map((p) => (p.PublicPort ? `${p.PublicPort}→${p.PrivatePort}/${p.Type}` : `${p.PrivatePort}/${p.Type}`))
    .join(", ");
}

function LogsDialog({ target, onClose }: { target: DContainer | null; onClose: () => void }) {
  return (
    <Dialog open={!!target} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-w-4xl">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Activity className="h-4 w-4 text-primary" />
            {target?.Names[0]?.replace(/^\//, "")} — logs
          </DialogTitle>
        </DialogHeader>
        {target && <LogsView id={target.Id} />}
      </DialogContent>
    </Dialog>
  );
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
      } catch { /* ignore */ }
    };
    return () => ws.close();
  }, [id]);
  useEffect(() => {
    ref.current?.scrollTo({ top: ref.current.scrollHeight });
  }, [lines]);
  return (
    <div ref={ref} className="h-[60vh] overflow-y-auto rounded-md border bg-black/60 p-3 font-mono text-xs leading-relaxed text-green-300/90">
      {lines.length === 0
        ? <span className="text-muted-foreground">Waiting…</span>
        : lines.map((l, i) => <div key={i}>{l}</div>)}
    </div>
  );
}

function StatsDialog({ target, onClose }: { target: DContainer | null; onClose: () => void }) {
  const [cpu, setCpu] = useState(0);
  const [memUsed, setMemUsed] = useState(0);
  const [memLimit, setMemLimit] = useState(0);
  const [netRx, setNetRx] = useState(0);
  const [netTx, setNetTx] = useState(0);
  const lastRef = useRef<{ cpu: number; sys: number } | null>(null);

  useEffect(() => {
    if (!target) return;
    const ws = new WebSocket(wsURL(`/api/docker/containers/${target.Id}/stats/stream`));
    ws.onmessage = (e) => {
      try {
        const raw = JSON.parse(e.data as string);
        if (raw.error) return;
        const cpuTotal = raw.cpu_stats?.cpu_usage?.total_usage ?? 0;
        const sysTotal = raw.cpu_stats?.system_cpu_usage ?? 0;
        const onlineCPUs = raw.cpu_stats?.online_cpus ?? 1;
        if (lastRef.current) {
          const cpuDelta = cpuTotal - lastRef.current.cpu;
          const sysDelta = sysTotal - lastRef.current.sys;
          if (sysDelta > 0 && cpuDelta > 0) {
            setCpu((cpuDelta / sysDelta) * onlineCPUs * 100);
          }
        }
        lastRef.current = { cpu: cpuTotal, sys: sysTotal };
        setMemUsed(raw.memory_stats?.usage ?? 0);
        setMemLimit(raw.memory_stats?.limit ?? 0);
        const nets = raw.networks ?? {};
        let rx = 0, tx = 0;
        Object.values<any>(nets).forEach((n) => { rx += n.rx_bytes ?? 0; tx += n.tx_bytes ?? 0; });
        setNetRx(rx);
        setNetTx(tx);
      } catch { /* ignore */ }
    };
    return () => ws.close();
  }, [target]);

  return (
    <Dialog open={!!target} onOpenChange={(o) => !o && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Activity className="h-4 w-4 text-primary" />
            {target?.Names[0]?.replace(/^\//, "")} — live stats
          </DialogTitle>
        </DialogHeader>
        <div className="grid grid-cols-2 gap-4">
          <StatTile label="CPU" value={`${cpu.toFixed(1)}%`} />
          <StatTile label="Memory" value={`${formatBytes(memUsed)} / ${formatBytes(memLimit)}`} />
          <StatTile label="Net RX" value={formatBytes(netRx)} />
          <StatTile label="Net TX" value={formatBytes(netTx)} />
        </div>
      </DialogContent>
    </Dialog>
  );
}

function StatTile({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-md border border-border p-4">
      <div className="text-xs uppercase tracking-wider text-muted-foreground">{label}</div>
      <div className="mt-1 text-2xl font-semibold tabular-nums">{value}</div>
    </div>
  );
}

function ExecDialog({ target, onClose }: { target: DContainer | null; onClose: () => void }) {
  const ref = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!target || !ref.current) return;
    const term = new XTerm({
      fontFamily: '"JetBrains Mono", ui-monospace, monospace',
      fontSize: 13,
      cursorBlink: true,
      theme: {
        background: "#0a1020",
        foreground: "#e6edf3",
        cursor: "#00DC82",
        selectionBackground: "#1f4cff55",
      },
    });
    const fit = new FitAddon();
    term.loadAddon(fit);
    term.open(ref.current);
    fit.fit();

    const ws = new WebSocket(wsURL(`/api/docker/containers/${target.Id}/exec/ws?shell=/bin/sh`));
    ws.binaryType = "arraybuffer";
    ws.onopen = () => term.focus();
    ws.onmessage = (ev) => {
      if (ev.data instanceof ArrayBuffer) {
        term.write(new Uint8Array(ev.data));
      } else {
        try {
          const m = JSON.parse(ev.data);
          if (m.type === "closed") term.write(`\r\n\x1b[31m${m.detail || "session closed"}\x1b[0m\r\n`);
        } catch { term.write(ev.data); }
      }
    };
    ws.onclose = () => term.write("\r\n\x1b[33m[disconnected]\x1b[0m\r\n");

    const onData = term.onData((d) => {
      if (ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify({ type: "data", data: d }));
    });
    const onResize = term.onResize(({ rows, cols }) => {
      if (ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify({ type: "resize", rows, cols }));
    });
    const ro = new ResizeObserver(() => fit.fit());
    ro.observe(ref.current);
    return () => {
      onData.dispose(); onResize.dispose(); ro.disconnect(); ws.close(); term.dispose();
    };
  }, [target]);

  return (
    <Dialog open={!!target} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-w-5xl">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <ContainerIcon className="h-4 w-4 text-primary" />
            {target?.Names[0]?.replace(/^\//, "")} — console (/bin/sh)
          </DialogTitle>
        </DialogHeader>
        <div ref={ref} className="h-[60vh] bg-[#0a1020] p-2 rounded-md" />
      </DialogContent>
    </Dialog>
  );
}

function CreateContainerDialog({
  open, onClose, onCreated,
}: { open: boolean; onClose: () => void; onCreated: () => Promise<void> }) {
  const [image, setImage] = useState("nginx:latest");
  const [name, setName] = useState("");
  const [restart, setRestart] = useState("unless-stopped");
  const [ports, setPorts] = useState<{ host: string; container: string; proto: string }[]>([]);
  const [envs, setEnvs] = useState<{ key: string; value: string }[]>([]);
  const [mounts, setMounts] = useState<{ type: string; source: string; target: string; ro: boolean }[]>([]);
  const [autoStart, setAutoStart] = useState(true);
  const [busy, setBusy] = useState(false);

  const reset = () => {
    setImage("nginx:latest"); setName(""); setRestart("unless-stopped");
    setPorts([]); setEnvs([]); setMounts([]); setAutoStart(true);
  };

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    try {
      await api.post("/api/docker/containers", {
        image, name, restart_policy: restart, auto_start: autoStart,
        port_bindings: ports.map((p) => ({
          host_port: p.host, container_port: Number(p.container), protocol: p.proto,
        })),
        env: envs.filter((e) => e.key).map((e) => `${e.key}=${e.value}`),
        mounts: mounts.filter((m) => m.target).map((m) => ({
          type: m.type, source: m.source, target: m.target, read_only: m.ro,
        })),
      });
      toast.success("Container created", name || image);
      reset(); onClose(); await onCreated();
    } catch (err) {
      if (err instanceof ApiError) toast.error("Create failed", err.message);
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>New container</DialogTitle>
          <DialogDescription>
            The image is pulled automatically if it isn't already on the host.
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-4 max-h-[70vh] overflow-y-auto pr-2">
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5 col-span-2">
              <Label>Image</Label>
              <Input value={image} onChange={(e) => setImage(e.target.value)} className="font-mono" required />
            </div>
            <div className="space-y-1.5">
              <Label>Name (optional)</Label>
              <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="my-app" />
            </div>
            <div className="space-y-1.5">
              <Label>Restart policy</Label>
              <select value={restart} onChange={(e) => setRestart(e.target.value)}
                className="flex h-9 w-full rounded-md border border-input bg-background px-3 text-sm">
                <option value="no">no</option>
                <option value="always">always</option>
                <option value="unless-stopped">unless-stopped</option>
                <option value="on-failure">on-failure</option>
              </select>
            </div>
          </div>

          <RowList label="Port bindings" items={ports}
            add={() => setPorts([...ports, { host: "", container: "", proto: "tcp" }])}
            remove={(i) => setPorts(ports.filter((_, j) => j !== i))}
            render={(p, i) => (
              <>
                <Input placeholder="Host" className="w-24 font-mono" value={p.host} onChange={(e) => update(ports, setPorts, i, { ...p, host: e.target.value })} />
                <span>→</span>
                <Input placeholder="Container" className="w-24 font-mono" value={p.container} onChange={(e) => update(ports, setPorts, i, { ...p, container: e.target.value })} />
                <select value={p.proto} onChange={(e) => update(ports, setPorts, i, { ...p, proto: e.target.value })}
                  className="h-9 rounded-md border border-input bg-background px-2 text-sm">
                  <option value="tcp">tcp</option>
                  <option value="udp">udp</option>
                </select>
              </>
            )}
          />

          <RowList label="Environment variables" items={envs}
            add={() => setEnvs([...envs, { key: "", value: "" }])}
            remove={(i) => setEnvs(envs.filter((_, j) => j !== i))}
            render={(e, i) => (
              <>
                <Input placeholder="KEY" className="w-40 font-mono uppercase" value={e.key} onChange={(ev) => update(envs, setEnvs, i, { ...e, key: ev.target.value })} />
                <span>=</span>
                <Input placeholder="value" className="flex-1 font-mono" value={e.value} onChange={(ev) => update(envs, setEnvs, i, { ...e, value: ev.target.value })} />
              </>
            )}
          />

          <RowList label="Volumes & bind mounts" items={mounts}
            add={() => setMounts([...mounts, { type: "volume", source: "", target: "", ro: false }])}
            remove={(i) => setMounts(mounts.filter((_, j) => j !== i))}
            render={(m, i) => (
              <>
                <select value={m.type} onChange={(e) => update(mounts, setMounts, i, { ...m, type: e.target.value })}
                  className="h-9 rounded-md border border-input bg-background px-2 text-sm">
                  <option value="volume">volume</option>
                  <option value="bind">bind</option>
                </select>
                <Input placeholder={m.type === "bind" ? "/host/path" : "volume-name"} className="flex-1 font-mono" value={m.source} onChange={(e) => update(mounts, setMounts, i, { ...m, source: e.target.value })} />
                <span>→</span>
                <Input placeholder="/container/path" className="flex-1 font-mono" value={m.target} onChange={(e) => update(mounts, setMounts, i, { ...m, target: e.target.value })} />
                <label className="flex items-center gap-1 text-xs">
                  <input type="checkbox" checked={m.ro} onChange={(e) => update(mounts, setMounts, i, { ...m, ro: e.target.checked })} />
                  ro
                </label>
              </>
            )}
          />

          <label className="flex items-center gap-2 text-sm">
            <input type="checkbox" checked={autoStart} onChange={(e) => setAutoStart(e.target.checked)} />
            Start container after creation
          </label>

          <div className="flex justify-end gap-2 sticky bottom-0 bg-card py-2 -mx-2 px-2 border-t">
            <Button type="button" variant="ghost" onClick={onClose}>Cancel</Button>
            <Button type="submit" disabled={busy}>{busy ? "Creating…" : "Create"}</Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function update<T>(arr: T[], set: (a: T[]) => void, i: number, v: T) {
  const next = [...arr];
  next[i] = v;
  set(next);
}

function RowList<T>({
  label, items, add, remove, render,
}: {
  label: string;
  items: T[];
  add: () => void;
  remove: (i: number) => void;
  render: (item: T, i: number) => React.ReactNode;
}) {
  return (
    <div className="space-y-1.5">
      <div className="flex justify-between items-center">
        <Label>{label}</Label>
        <Button type="button" variant="ghost" size="sm" onClick={add}>
          <Plus className="h-3 w-3" /> Add
        </Button>
      </div>
      {items.length === 0 ? (
        <div className="text-xs text-muted-foreground">None</div>
      ) : (
        <div className="space-y-1.5">
          {items.map((it, i) => (
            <div key={i} className="flex items-center gap-1.5">
              {render(it, i)}
              <Button type="button" variant="ghost" size="icon" onClick={() => remove(i)}>
                <Trash2 className="h-3.5 w-3.5 text-destructive" />
              </Button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function PullImageDialog({ open, onClose, onPulled }: { open: boolean; onClose: () => void; onPulled: () => Promise<void> }) {
  const [ref, setRef] = useState("nginx:latest");
  const [lines, setLines] = useState<string[]>([]);
  const [pulling, setPulling] = useState(false);
  const wsRef = useRef<WebSocket | null>(null);

  const start = (e: FormEvent) => {
    e.preventDefault();
    setLines([]); setPulling(true);
    const ws = new WebSocket(wsURL("/api/docker/images/pull"));
    wsRef.current = ws;
    ws.onopen = () => ws.send(JSON.stringify({ image: ref }));
    ws.onmessage = (e) => {
      try {
        const msg = JSON.parse(e.data as string);
        if (msg.error) setLines((cur) => [...cur, `[error] ${msg.error}`]);
        else if (msg.type === "done") {
          setLines((cur) => [...cur, "✓ done"]);
          setPulling(false);
          void onPulled();
        } else if (msg.status) {
          const id = msg.id ? ` ${msg.id}` : "";
          const progress = msg.progress ? ` ${msg.progress}` : "";
          setLines((cur) => [...cur.slice(-200), `${msg.status}${id}${progress}`]);
        }
      } catch { /* ignore */ }
    };
    ws.onclose = () => setPulling(false);
  };

  useEffect(() => {
    if (!open) {
      wsRef.current?.close();
      setLines([]); setPulling(false);
    }
  }, [open]);

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>Pull image</DialogTitle>
          <DialogDescription>Image reference like <code>nginx:latest</code> or <code>ghcr.io/owner/repo:tag</code>.</DialogDescription>
        </DialogHeader>
        <form onSubmit={start} className="space-y-3">
          <div className="flex gap-2">
            <Input value={ref} onChange={(e) => setRef(e.target.value)} className="font-mono" disabled={pulling} required />
            <Button type="submit" disabled={pulling}>{pulling ? "Pulling…" : "Pull"}</Button>
          </div>
        </form>
        <div className="h-64 overflow-y-auto rounded-md border bg-black/60 p-3 font-mono text-xs leading-relaxed text-green-300/90">
          {lines.length === 0 ? <span className="text-muted-foreground">Ready.</span> : lines.map((l, i) => <div key={i}>{l}</div>)}
        </div>
      </DialogContent>
    </Dialog>
  );
}

function CreateNetworkDialog({ open, onClose, onCreated }: { open: boolean; onClose: () => void; onCreated: () => Promise<void> }) {
  const [name, setName] = useState("");
  const [driver, setDriver] = useState("bridge");
  const [subnet, setSubnet] = useState("");
  const [busy, setBusy] = useState(false);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    try {
      await api.post("/api/docker/networks", { name, driver, subnet });
      toast.success("Network created", name);
      setName(""); setSubnet("");
      onClose(); await onCreated();
    } catch (err) {
      if (err instanceof ApiError) toast.error("Create failed", err.message);
    } finally { setBusy(false); }
  };

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>New network</DialogTitle>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-3">
          <div className="space-y-1.5">
            <Label>Name</Label>
            <Input value={name} onChange={(e) => setName(e.target.value)} required />
          </div>
          <div className="space-y-1.5">
            <Label>Driver</Label>
            <select value={driver} onChange={(e) => setDriver(e.target.value)}
              className="flex h-9 w-full rounded-md border border-input bg-background px-3 text-sm">
              <option value="bridge">bridge</option>
              <option value="overlay">overlay</option>
              <option value="macvlan">macvlan</option>
            </select>
          </div>
          <div className="space-y-1.5">
            <Label>Subnet (optional)</Label>
            <Input value={subnet} onChange={(e) => setSubnet(e.target.value)} placeholder="172.20.0.0/16" className="font-mono" />
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

function CreateVolumeDialog({ open, onClose, onCreated }: { open: boolean; onClose: () => void; onCreated: () => Promise<void> }) {
  const [name, setName] = useState("");
  const [driver, setDriver] = useState("local");
  const [busy, setBusy] = useState(false);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    try {
      await api.post("/api/docker/volumes", { name, driver });
      toast.success("Volume created", name);
      setName("");
      onClose(); await onCreated();
    } catch (err) {
      if (err instanceof ApiError) toast.error("Create failed", err.message);
    } finally { setBusy(false); }
  };

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>New volume</DialogTitle>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-3">
          <div className="space-y-1.5">
            <Label>Name</Label>
            <Input value={name} onChange={(e) => setName(e.target.value)} required />
          </div>
          <div className="space-y-1.5">
            <Label>Driver</Label>
            <Input value={driver} onChange={(e) => setDriver(e.target.value)} />
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
