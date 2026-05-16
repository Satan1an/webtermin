import { useEffect, useMemo, useState } from "react";
import {
  ArrowUp,
  Download,
  File as FileIcon,
  FilePen,
  Folder,
  Home,
  Plus,
  Save,
  Trash2,
  Upload,
} from "lucide-react";
import Editor from "@monaco-editor/react";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { api, ApiError } from "@/lib/api";
import { toast } from "@/components/ui/toast";
import { formatBytes } from "@/lib/utils";
import { useAuth, useCanWrite } from "@/store/auth";

interface Entry {
  name: string;
  path: string;
  is_dir: boolean;
  is_link: boolean;
  size: number;
  mode_oct: string;
  uid: number;
  gid: number;
  mtime: string;
}

export function FilesPage() {
  const [cwd, setCwd] = useState("/root");
  const [entries, setEntries] = useState<Entry[]>([]);
  const [editing, setEditing] = useState<{ path: string; content: string } | null>(null);
  const [language, setLanguage] = useState("plaintext");
  const [busy, setBusy] = useState(false);
  const canWrite = useCanWrite();

  const load = async (path: string) => {
    try {
      const r = await api.get<{ path: string; entries: Entry[] }>(
        `/api/files/list?path=${encodeURIComponent(path)}`,
      );
      setCwd(r.path);
      setEntries(r.entries ?? []);
    } catch (e) {
      if (e instanceof ApiError) toast.error("Failed to list", e.message);
    }
  };

  useEffect(() => {
    void load(cwd);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const parts = useMemo(() => {
    const p = cwd.replace(/\/+$/, "") || "/";
    if (p === "/") return [{ name: "/", path: "/" }];
    const segs = p.split("/").filter(Boolean);
    let acc = "";
    return [{ name: "/", path: "/" }].concat(
      segs.map((s) => {
        acc += "/" + s;
        return { name: s, path: acc };
      }),
    );
  }, [cwd]);

  const open = async (e: Entry) => {
    if (e.is_dir) return load(e.path);
    try {
      const r = await api.get<{ path: string; content: string }>(
        `/api/files/read?path=${encodeURIComponent(e.path)}`,
      );
      setEditing({ path: r.path, content: r.content });
      setLanguage(guessLanguage(r.path));
    } catch (err) {
      if (err instanceof ApiError) toast.error("Cannot open", err.message);
    }
  };

  const save = async () => {
    if (!editing) return;
    setBusy(true);
    try {
      await api.post("/api/files/write", { path: editing.path, content: editing.content });
      toast.success("Saved", editing.path);
    } catch (e) {
      if (e instanceof ApiError) toast.error("Save failed", e.message);
    } finally {
      setBusy(false);
    }
  };

  const mkdir = async () => {
    const name = prompt("New directory name");
    if (!name) return;
    const path = cwd.replace(/\/+$/, "") + "/" + name;
    try {
      await api.post("/api/files/mkdir", { path });
      await load(cwd);
      toast.success("Created", path);
    } catch (e) {
      if (e instanceof ApiError) toast.error("mkdir failed", e.message);
    }
  };

  const remove = async (e: Entry) => {
    if (!confirm(`Delete ${e.path}?`)) return;
    try {
      await api.post("/api/files/delete", { path: e.path, recursive: e.is_dir });
      await load(cwd);
      toast.success("Deleted", e.path);
    } catch (err) {
      if (err instanceof ApiError) toast.error("Delete failed", err.message);
    }
  };

  const upload = async (file: File) => {
    const fd = new FormData();
    fd.append("dir", cwd);
    fd.append("file", file);
    try {
      const res = await fetch("/api/files/upload", {
        method: "POST",
        body: fd,
        credentials: "same-origin",
        headers: {
          "X-CSRF-Token": useAuth.getState().csrfToken ?? "",
        },
      });
      if (!res.ok) {
        const j = await res.json().catch(() => ({}));
        throw new Error(j.error || res.statusText);
      }
      await load(cwd);
      toast.success("Uploaded", file.name);
    } catch (e) {
      toast.error("Upload failed", (e as Error).message);
    }
  };

  return (
    <div className="space-y-4">
      <div className="flex items-end justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Files</h1>
          <div className="mt-1 flex items-center gap-1 text-sm">
            {parts.map((p, i) => (
              <span key={p.path} className="flex items-center gap-1">
                <button
                  onClick={() => load(p.path)}
                  className="text-muted-foreground hover:text-foreground transition-colors"
                >
                  {i === 0 ? <Home className="h-3.5 w-3.5" /> : p.name}
                </button>
                {i < parts.length - 1 && (
                  <span className="text-muted-foreground">/</span>
                )}
              </span>
            ))}
          </div>
        </div>
        <div className="flex gap-2">
          {canWrite && (
            <>
              <Button variant="outline" size="sm" onClick={mkdir}>
                <Plus className="h-4 w-4" /> Folder
              </Button>
              <label className="inline-block">
                <input
                  type="file"
                  className="hidden"
                  onChange={(e) => {
                    const f = e.target.files?.[0];
                    if (f) void upload(f);
                    e.target.value = "";
                  }}
                />
                <Button variant="outline" size="sm" asChild>
                  <span><Upload className="h-4 w-4" /> Upload</span>
                </Button>
              </label>
            </>
          )}
          <Button
            variant="ghost"
            size="sm"
            onClick={() => {
              const up = cwd.replace(/\/[^/]+\/?$/, "") || "/";
              void load(up);
            }}
          >
            <ArrowUp className="h-4 w-4" /> Up
          </Button>
        </div>
      </div>

      <Card>
        <CardContent className="p-0">
          <div className="divide-y divide-border">
            <div className="grid grid-cols-12 gap-3 px-5 py-2.5 text-xs uppercase tracking-wider text-muted-foreground">
              <div className="col-span-5">Name</div>
              <div className="col-span-2">Size</div>
              <div className="col-span-2">Mode</div>
              <div className="col-span-2">Modified</div>
              <div className="col-span-1 text-right">Actions</div>
            </div>
            {entries.map((e) => (
              <div
                key={e.path}
                className="grid grid-cols-12 gap-3 items-center px-5 py-2 hover:bg-accent/30 transition-colors cursor-pointer"
                onDoubleClick={() => open(e)}
              >
                <div className="col-span-5 min-w-0 flex items-center gap-2">
                  {e.is_dir ? (
                    <Folder className="h-4 w-4 text-primary" />
                  ) : (
                    <FileIcon className="h-4 w-4 text-muted-foreground" />
                  )}
                  <button
                    onClick={() => open(e)}
                    className="truncate hover:underline text-left"
                  >
                    {e.name}
                  </button>
                  {e.is_link && <span className="text-xs text-muted-foreground">→ link</span>}
                </div>
                <div className="col-span-2 text-sm text-muted-foreground tabular-nums">
                  {e.is_dir ? "—" : formatBytes(e.size)}
                </div>
                <div className="col-span-2 font-mono text-xs">{e.mode_oct}</div>
                <div className="col-span-2 text-xs text-muted-foreground">
                  {new Date(e.mtime).toLocaleString()}
                </div>
                <div className="col-span-1 flex justify-end gap-1">
                  {!e.is_dir && (
                    <a
                      href={`/api/files/download?path=${encodeURIComponent(e.path)}`}
                      title="Download"
                      className="inline-grid h-9 w-9 place-items-center rounded-md hover:bg-accent"
                    >
                      <Download className="h-4 w-4" />
                    </a>
                  )}
                  {canWrite && (
                    <Button variant="ghost" size="icon" onClick={() => remove(e)} title="Delete">
                      <Trash2 className="h-4 w-4 text-destructive" />
                    </Button>
                  )}
                </div>
              </div>
            ))}
            {entries.length === 0 && (
              <div className="p-8 text-center text-sm text-muted-foreground">Empty.</div>
            )}
          </div>
        </CardContent>
      </Card>

      <Dialog open={!!editing} onOpenChange={(o) => !o && setEditing(null)}>
        <DialogContent className="max-w-5xl">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <FilePen className="h-4 w-4 text-primary" />
              <span className="font-mono">{editing?.path}</span>
            </DialogTitle>
          </DialogHeader>
          <div className="h-[60vh]">
            <Editor
              theme="vs-dark"
              language={language}
              value={editing?.content ?? ""}
              onChange={(v) =>
                setEditing((cur) => (cur ? { ...cur, content: v ?? "" } : cur))
              }
              options={{
                fontSize: 13,
                minimap: { enabled: false },
                scrollBeyondLastLine: false,
                wordWrap: "on",
                tabSize: 2,
              }}
            />
          </div>
          <div className="flex justify-end">
            <Button onClick={save} disabled={busy || !canWrite} title={canWrite ? "" : "Read-only — your role can't modify files"}>
              <Save className="h-4 w-4" /> {busy ? "Saving…" : canWrite ? "Save" : "Read-only"}
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}

function guessLanguage(path: string): string {
  const lower = path.toLowerCase();
  if (lower.endsWith(".json")) return "json";
  if (lower.endsWith(".yaml") || lower.endsWith(".yml")) return "yaml";
  if (lower.endsWith(".toml")) return "ini";
  if (lower.endsWith(".sh") || lower.endsWith(".bash")) return "shell";
  if (lower.endsWith(".py")) return "python";
  if (lower.endsWith(".js") || lower.endsWith(".mjs")) return "javascript";
  if (lower.endsWith(".ts") || lower.endsWith(".tsx")) return "typescript";
  if (lower.endsWith(".go")) return "go";
  if (lower.endsWith(".rs")) return "rust";
  if (lower.endsWith(".md")) return "markdown";
  if (lower.endsWith(".conf") || lower.endsWith(".cfg") || lower.endsWith(".ini")) return "ini";
  if (lower.endsWith(".nginx") || lower.includes("nginx.conf")) return "ini";
  if (lower.endsWith(".service") || lower.endsWith(".socket")) return "ini";
  return "plaintext";
}
