import { useEffect, useState, type FormEvent } from "react";
import { Archive, Download, Plus, Trash2 } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { api, ApiError } from "@/lib/api";
import { toast } from "@/components/ui/toast";
import { useIsAdmin } from "@/store/auth";
import { formatBytes } from "@/lib/utils";

interface Backup {
  id: number;
  name: string;
  path: string;
  size_bytes: number;
  paths: string[];
  created_at: string;
}

const DEFAULT_PATHS = ["/etc/webtermin", "/var/lib/webtermin"];

export function BackupPage() {
  const isAdmin = useIsAdmin();
  const [items, setItems] = useState<Backup[]>([]);
  const [open, setOpen] = useState(false);

  const load = async () => {
    try {
      setItems((await api.get<Backup[]>("/api/backups")) ?? []);
    } catch (e) {
      if (e instanceof ApiError) toast.error("Failed", e.message);
    }
  };
  useEffect(() => { void load(); }, []);

  const remove = async (b: Backup) => {
    if (!confirm(`Delete backup "${b.name}"? The .tar.gz file is removed too.`)) return;
    try {
      await api.del(`/api/backups/${b.id}`);
      toast.success("Removed", b.name);
      await load();
    } catch (e) {
      if (e instanceof ApiError) toast.error("Remove failed", e.message);
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-end justify-between gap-4 flex-wrap">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Backups</h1>
          <p className="text-sm text-muted-foreground">
            Tar.gz snapshots of <code>/etc/webtermin</code>, <code>/var/lib/webtermin</code>,
            and any extra paths you list. Download them, copy off-box, restore with
            <code> sudo tar xzf &lt;file&gt; -C /</code>.
          </p>
        </div>
        {isAdmin && (
          <Button onClick={() => setOpen(true)}>
            <Plus className="h-4 w-4" /> Create backup
          </Button>
        )}
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Archive className="h-4 w-4 text-primary" /> Snapshots
          </CardTitle>
        </CardHeader>
        <CardContent className="p-0">
          <div className="divide-y divide-border">
            <div className="grid grid-cols-12 gap-3 px-5 py-2.5 text-xs uppercase tracking-wider text-muted-foreground">
              <div className="col-span-3">Name</div>
              <div className="col-span-5">Paths</div>
              <div className="col-span-2 text-right">Size</div>
              <div className="col-span-1">Created</div>
              <div className="col-span-1 text-right">Actions</div>
            </div>
            {items.map((b) => (
              <div key={b.id} className="grid grid-cols-12 gap-3 items-center px-5 py-3 hover:bg-accent/30 transition-colors">
                <div className="col-span-3 font-medium truncate">{b.name}</div>
                <div className="col-span-5 text-xs text-muted-foreground font-mono truncate">
                  {b.paths.join(", ")}
                </div>
                <div className="col-span-2 text-right tabular-nums text-sm">
                  {formatBytes(b.size_bytes)}
                </div>
                <div className="col-span-1 text-xs text-muted-foreground">
                  {new Date(b.created_at).toLocaleDateString()}
                </div>
                <div className="col-span-1 flex justify-end gap-1">
                  <a
                    href={`/api/backups/${b.id}/download`}
                    title="Download"
                    className="inline-grid h-9 w-9 place-items-center rounded-md hover:bg-accent"
                  >
                    <Download className="h-4 w-4" />
                  </a>
                  {isAdmin && (
                    <Button variant="ghost" size="icon" onClick={() => remove(b)}>
                      <Trash2 className="h-4 w-4 text-destructive" />
                    </Button>
                  )}
                </div>
              </div>
            ))}
            {items.length === 0 && (
              <div className="p-8 text-center text-sm text-muted-foreground">
                No backups yet.
              </div>
            )}
          </div>
        </CardContent>
      </Card>

      <CreateDialog open={open} onClose={() => setOpen(false)} onCreated={load} />
    </div>
  );
}

function CreateDialog({ open, onClose, onCreated }: { open: boolean; onClose: () => void; onCreated: () => Promise<void> }) {
  const [name, setName] = useState("pre-upgrade");
  const [paths, setPaths] = useState(DEFAULT_PATHS.join("\n"));
  const [busy, setBusy] = useState(false);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    try {
      const list = paths.split("\n").map((s) => s.trim()).filter(Boolean);
      await api.post("/api/backups", { name, paths: list });
      toast.success("Backup created", name);
      onClose();
      await onCreated();
    } catch (err) {
      if (err instanceof ApiError) toast.error("Failed", err.message);
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Create backup</DialogTitle>
          <DialogDescription>One absolute path per line. Defaults cover the panel state.</DialogDescription>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-3">
          <div className="space-y-1.5">
            <Label>Name</Label>
            <Input value={name} onChange={(e) => setName(e.target.value)} required pattern="[a-zA-Z0-9][a-zA-Z0-9._-]*" maxLength={64} />
          </div>
          <div className="space-y-1.5">
            <Label>Paths</Label>
            <textarea
              value={paths}
              onChange={(e) => setPaths(e.target.value)}
              className="w-full h-32 rounded-md border bg-background p-2 text-xs font-mono"
            />
          </div>
          <div className="flex justify-end gap-2">
            <Button type="button" variant="ghost" onClick={onClose}>Cancel</Button>
            <Button type="submit" disabled={busy}>{busy ? "Archiving…" : "Create"}</Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}
