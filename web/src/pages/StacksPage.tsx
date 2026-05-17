import { useEffect, useMemo, useState, type FormEvent } from "react";
import Editor from "@monaco-editor/react";
import {
  Boxes,
  ChevronRight,
  Layers,
  Play,
  Plus,
  RotateCw,
  Square,
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
import { api, ApiError } from "@/lib/api";
import { toast } from "@/components/ui/toast";
import { useCanWrite } from "@/store/auth";

interface DContainer {
  Id: string;
  Names: string[];
  Image: string;
  State: string;
  Status: string;
  Labels?: Record<string, string>;
}

interface Stack {
  id: number;
  name: string;
  compose: string;
  created_at: string;
  updated_at: string;
  containers?: DContainer[];
  status?: "running" | "partial" | "stopped" | "empty";
  services?: string[];
}

const STARTER_COMPOSE = `version: "3.9"

services:
  web:
    image: nginx:1.27-alpine
    restart: unless-stopped
    ports:
      - "8080:80"
    volumes:
      - "html:/usr/share/nginx/html"
    networks:
      - web

networks:
  web:
    driver: bridge

volumes:
  html:
`;

export function StacksPage() {
  const canWrite = useCanWrite();
  const [stacks, setStacks] = useState<Stack[]>([]);
  const [selected, setSelected] = useState<Stack | null>(null);
  const [deployOpen, setDeployOpen] = useState(false);
  const [busy, setBusy] = useState<string | null>(null);

  const load = async () => {
    try {
      const list = await api.get<Stack[]>("/api/stacks");
      setStacks(list ?? []);
      if (selected) {
        const fresh = (list ?? []).find((s) => s.id === selected.id);
        if (fresh) setSelected(fresh);
        else setSelected(null);
      }
    } catch (e) {
      if (e instanceof ApiError) toast.error("Failed to load stacks", e.message);
    }
  };

  useEffect(() => {
    void load();
    const t = setInterval(load, 5000);
    return () => clearInterval(t);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const act = async (st: Stack, action: "start" | "stop") => {
    setBusy(st.id + action);
    try {
      await api.post(`/api/stacks/${st.id}/${action}`);
      toast.success(`stack ${action} ✓`, st.name);
      await load();
    } catch (e) {
      if (e instanceof ApiError) toast.error(`${action} failed`, e.message);
    } finally {
      setBusy(null);
    }
  };

  const remove = async (st: Stack) => {
    const removeData = confirm(
      `Remove stack "${st.name}"?\n\n` +
        `OK    = remove containers + delete its volumes (data is GONE)\n` +
        `Cancel = keep volumes (only abort if you want to abort entirely)`,
    );
    // Two-step prompt: cancel above means user changed their mind, abort.
    if (!removeData && !confirm(`Really? Remove stack "${st.name}" but KEEP its volumes?`)) return;
    try {
      await api.del(`/api/stacks/${st.id}${removeData ? "?remove_data=1" : ""}`);
      toast.success(`Stack ${st.name} removed`);
      setSelected(null);
      await load();
    } catch (e) {
      if (e instanceof ApiError) toast.error("Remove failed", e.message);
    }
  };

  if (selected) {
    return (
      <StackDetail
        stack={selected}
        onBack={() => setSelected(null)}
        onChanged={load}
        canWrite={canWrite}
      />
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-end justify-between gap-4 flex-wrap">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Stacks</h1>
          <p className="text-sm text-muted-foreground">
            Multi-container deployments managed via <code>docker-compose</code> v3.x YAML.
            {stacks.length > 0 && <> · {stacks.length} stacks</>}
          </p>
        </div>
        {canWrite && (
          <Button onClick={() => setDeployOpen(true)}>
            <Plus className="h-4 w-4" /> Deploy stack
          </Button>
        )}
      </div>

      <Card>
        <CardContent className="p-0">
          <div className="divide-y divide-border">
            <div className="grid grid-cols-12 gap-3 px-5 py-2.5 text-xs uppercase tracking-wider text-muted-foreground">
              <div className="col-span-3">Name</div>
              <div className="col-span-2">Status</div>
              <div className="col-span-2">Services</div>
              <div className="col-span-2">Containers</div>
              <div className="col-span-1">Updated</div>
              <div className="col-span-2 text-right">Actions</div>
            </div>
            {stacks.map((st) => (
              <div
                key={st.id}
                className="grid grid-cols-12 gap-3 items-center px-5 py-3 hover:bg-accent/30 transition-colors cursor-pointer"
                onClick={() => setSelected(st)}
              >
                <div className="col-span-3 font-medium flex items-center gap-2">
                  <Layers className="h-4 w-4 text-primary" />
                  {st.name}
                </div>
                <div className="col-span-2">
                  <StackStatusBadge status={st.status} />
                </div>
                <div className="col-span-2 text-sm text-muted-foreground truncate">
                  {st.services?.join(", ") || "—"}
                </div>
                <div className="col-span-2 text-sm tabular-nums">
                  {st.containers?.length ?? 0}
                </div>
                <div className="col-span-1 text-xs text-muted-foreground">
                  {new Date(st.updated_at).toLocaleDateString()}
                </div>
                <div
                  className="col-span-2 flex justify-end gap-1"
                  onClick={(e) => e.stopPropagation()}
                >
                  {canWrite && (st.status === "stopped" || st.status === "partial" || st.status === "empty") && (
                    <Button size="icon" variant="ghost" title="Start" disabled={!!busy} onClick={() => act(st, "start")}>
                      <Play className="h-4 w-4" />
                    </Button>
                  )}
                  {canWrite && (st.status === "running" || st.status === "partial") && (
                    <Button size="icon" variant="ghost" title="Stop" disabled={!!busy} onClick={() => act(st, "stop")}>
                      <Square className="h-4 w-4" />
                    </Button>
                  )}
                  {canWrite && (
                    <Button size="icon" variant="ghost" title="Remove" onClick={() => remove(st)}>
                      <Trash2 className="h-4 w-4 text-destructive" />
                    </Button>
                  )}
                  <ChevronRight className="h-4 w-4 text-muted-foreground self-center" />
                </div>
              </div>
            ))}
            {stacks.length === 0 && (
              <div className="p-12 text-center">
                <Boxes className="mx-auto h-8 w-8 text-muted-foreground mb-2" />
                <div className="text-sm text-muted-foreground">
                  No stacks yet. Click <strong>Deploy stack</strong> to get started.
                </div>
              </div>
            )}
          </div>
        </CardContent>
      </Card>

      <DeployDialog open={deployOpen} onClose={() => setDeployOpen(false)} onDeployed={load} />
    </div>
  );
}

function StackStatusBadge({ status }: { status?: string }) {
  if (status === "running") return <Badge variant="success">running</Badge>;
  if (status === "partial") return <Badge variant="warning">partial</Badge>;
  if (status === "stopped") return <Badge variant="muted">stopped</Badge>;
  if (status === "empty") return <Badge variant="muted">empty</Badge>;
  return <Badge variant="muted">{status ?? "unknown"}</Badge>;
}

function StackDetail({
  stack, onBack, onChanged, canWrite,
}: {
  stack: Stack;
  onBack: () => void;
  onChanged: () => Promise<void>;
  canWrite: boolean;
}) {
  const [yamlText, setYamlText] = useState(stack.compose);
  const [saving, setSaving] = useState(false);
  const dirty = yamlText !== stack.compose;

  const grouped = useMemo(() => {
    const out: Record<string, DContainer[]> = {};
    for (const c of stack.containers ?? []) {
      const svc = c.Labels?.["com.docker.compose.service"] || "?";
      (out[svc] ??= []).push(c);
    }
    return out;
  }, [stack.containers]);

  const saveAndRedeploy = async () => {
    setSaving(true);
    try {
      await api.put(`/api/stacks/${stack.id}`, { compose: yamlText });
      toast.success("Stack updated", stack.name);
      await onChanged();
    } catch (e) {
      if (e instanceof ApiError) toast.error("Update failed", e.message);
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-2 text-sm">
        <Button variant="ghost" size="sm" onClick={onBack}>← Back to stacks</Button>
        <ChevronRight className="h-3 w-3 text-muted-foreground" />
        <span className="font-medium">{stack.name}</span>
        <StackStatusBadge status={stack.status} />
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Layers className="h-4 w-4 text-primary" /> Services
          </CardTitle>
        </CardHeader>
        <CardContent className="p-0">
          <div className="divide-y divide-border">
            {Object.entries(grouped).map(([service, cs]) => (
              <div key={service} className="px-5 py-3">
                <div className="flex items-center justify-between">
                  <div className="font-medium">{service}</div>
                  <div className="text-xs text-muted-foreground">
                    {cs.filter((c) => c.State === "running").length}/{cs.length} running
                  </div>
                </div>
                <div className="mt-2 space-y-1">
                  {cs.map((c) => (
                    <div
                      key={c.Id}
                      className="flex items-center gap-3 text-xs font-mono text-muted-foreground"
                    >
                      <span className="w-4 h-4 inline-grid place-items-center">
                        <span
                          className={`w-2 h-2 rounded-full ${
                            c.State === "running"
                              ? "bg-success"
                              : c.State === "paused"
                              ? "bg-warning"
                              : "bg-muted-foreground/50"
                          }`}
                        />
                      </span>
                      <span className="truncate flex-1">{c.Names[0]?.replace(/^\//, "")}</span>
                      <span className="text-[10px]">{c.Image}</span>
                      <span className="text-[10px]">{c.Status}</span>
                    </div>
                  ))}
                </div>
              </div>
            ))}
            {Object.keys(grouped).length === 0 && (
              <div className="p-8 text-center text-sm text-muted-foreground">
                No containers running for this stack. Hit <strong>Start</strong> on the list view to bring it up.
              </div>
            )}
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0">
          <CardTitle>docker-compose.yml</CardTitle>
          {canWrite && (
            <Button onClick={saveAndRedeploy} disabled={!dirty || saving}>
              <RotateCw className={`h-4 w-4 ${saving ? "animate-spin" : ""}`} />
              {saving ? "Redeploying…" : dirty ? "Save & redeploy" : "Saved"}
            </Button>
          )}
        </CardHeader>
        <CardContent>
          <div className="h-[60vh] rounded-md overflow-hidden border">
            <Editor
              theme="vs-dark"
              language="yaml"
              value={yamlText}
              onChange={(v) => setYamlText(v ?? "")}
              options={{
                fontSize: 13,
                minimap: { enabled: false },
                scrollBeyondLastLine: false,
                wordWrap: "on",
                tabSize: 2,
                readOnly: !canWrite,
              }}
            />
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

function DeployDialog({
  open, onClose, onDeployed,
}: {
  open: boolean;
  onClose: () => void;
  onDeployed: () => Promise<void>;
}) {
  const [name, setName] = useState("");
  const [yamlText, setYamlText] = useState(STARTER_COMPOSE);
  const [busy, setBusy] = useState(false);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    try {
      await api.post("/api/stacks", { name, compose: yamlText });
      toast.success("Stack deployed", name);
      setName(""); setYamlText(STARTER_COMPOSE);
      onClose();
      await onDeployed();
    } catch (err) {
      if (err instanceof ApiError) toast.error("Deploy failed", err.message);
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-w-4xl">
        <DialogHeader>
          <DialogTitle>Deploy a new stack</DialogTitle>
          <DialogDescription>
            Paste a <code>docker-compose.yml</code> below. Container names will be
            prefixed with the stack name; declared networks and volumes get
            <code> stack_name_</code> prefix.
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-3">
          <div className="space-y-1.5">
            <Label>Stack name</Label>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="my-app"
              pattern="[a-z0-9][a-z0-9_-]*"
              maxLength={32}
              required
              autoFocus
            />
          </div>
          <div className="space-y-1.5">
            <Label>docker-compose.yml</Label>
            <div className="h-[55vh] rounded-md overflow-hidden border">
              <Editor
                theme="vs-dark"
                language="yaml"
                value={yamlText}
                onChange={(v) => setYamlText(v ?? "")}
                options={{
                  fontSize: 13,
                  minimap: { enabled: false },
                  scrollBeyondLastLine: false,
                  wordWrap: "on",
                  tabSize: 2,
                }}
              />
            </div>
          </div>
          <div className="flex justify-end gap-2">
            <Button type="button" variant="ghost" onClick={onClose}>Cancel</Button>
            <Button type="submit" disabled={busy}>{busy ? "Deploying…" : "Deploy"}</Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}
