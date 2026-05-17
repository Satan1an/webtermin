import { useEffect, useState, type FormEvent } from "react";
import { Clock, Plus, Trash2 } from "lucide-react";
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
import { useCanWrite } from "@/store/auth";

interface CronEntry {
  line: number;
  schedule: string;
  command: string;
  comment?: string;
}

const PRESETS = [
  { label: "Every minute", value: "* * * * *" },
  { label: "Every 5 min", value: "*/5 * * * *" },
  { label: "Hourly", value: "@hourly" },
  { label: "Daily 03:00", value: "0 3 * * *" },
  { label: "Weekly (Sun)", value: "0 4 * * 0" },
  { label: "Monthly (1st)", value: "0 4 1 * *" },
  { label: "On reboot", value: "@reboot" },
];

export function CronPage() {
  const [user, setUser] = useState("root");
  const [entries, setEntries] = useState<CronEntry[]>([]);
  const [open, setOpen] = useState(false);
  const canWrite = useCanWrite();

  const load = async (u: string) => {
    try {
      setEntries(await api.get<CronEntry[]>(`/api/cron/${encodeURIComponent(u)}`));
    } catch (e) {
      if (e instanceof ApiError) toast.error("Failed to load cron", e.message);
    }
  };

  useEffect(() => {
    void load(user);
  }, [user]);

  const remove = async (e: CronEntry) => {
    if (!confirm(`Delete cron entry on line ${e.line}?\n\n${e.schedule} ${e.command}`)) return;
    try {
      await api.del(`/api/cron/${encodeURIComponent(user)}/${e.line}`);
      toast.success("Deleted");
      await load(user);
    } catch (err) {
      if (err instanceof ApiError) toast.error("Delete failed", err.message);
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-end justify-between gap-4 flex-wrap">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Cron</h1>
          <p className="text-sm text-muted-foreground">
            Scheduled jobs from the selected user's crontab.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Label htmlFor="cu" className="text-xs uppercase text-muted-foreground tracking-wider">
            User
          </Label>
          <Input
            id="cu"
            className="w-40"
            value={user}
            onChange={(e) => setUser(e.target.value)}
            placeholder="root"
          />
          {canWrite && (
            <Button onClick={() => setOpen(true)} disabled={!user}>
              <Plus className="h-4 w-4" /> Add entry
            </Button>
          )}
        </div>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Clock className="h-4 w-4 text-primary" /> Entries for <code>{user}</code>
          </CardTitle>
        </CardHeader>
        <CardContent className="p-0">
          <div className="divide-y divide-border">
            <div className="grid grid-cols-12 gap-3 px-5 py-2.5 text-xs uppercase tracking-wider text-muted-foreground">
              <div className="col-span-3">Schedule</div>
              <div className="col-span-7">Command</div>
              <div className="col-span-1">Line</div>
              <div className="col-span-1 text-right">Actions</div>
            </div>
            {entries.map((e) => (
              <div
                key={e.line}
                className="grid grid-cols-12 gap-3 items-center px-5 py-3 hover:bg-accent/30 transition-colors"
              >
                <div className="col-span-3 font-mono text-sm">{e.schedule}</div>
                <div className="col-span-7 min-w-0">
                  <div className="font-mono text-sm truncate">{e.command}</div>
                  {e.comment && (
                    <div className="text-xs text-muted-foreground truncate"># {e.comment}</div>
                  )}
                </div>
                <div className="col-span-1 text-xs text-muted-foreground tabular-nums">
                  {e.line}
                </div>
                <div className="col-span-1 flex justify-end">
                  {canWrite && (
                    <Button variant="ghost" size="icon" onClick={() => remove(e)} title="Delete">
                      <Trash2 className="h-4 w-4 text-destructive" />
                    </Button>
                  )}
                </div>
              </div>
            ))}
            {entries.length === 0 && (
              <div className="p-8 text-center text-sm text-muted-foreground">
                No cron entries for <code>{user}</code>.
              </div>
            )}
          </div>
        </CardContent>
      </Card>

      <AddDialog
        open={open}
        onClose={() => setOpen(false)}
        onAdded={() => load(user)}
        user={user}
      />
    </div>
  );
}

function AddDialog({
  open, onClose, onAdded, user,
}: {
  open: boolean;
  onClose: () => void;
  onAdded: () => Promise<void>;
  user: string;
}) {
  const [schedule, setSchedule] = useState("0 3 * * *");
  const [command, setCommand] = useState("");
  const [comment, setComment] = useState("");
  const [busy, setBusy] = useState(false);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    try {
      await api.post(`/api/cron/${encodeURIComponent(user)}`, { schedule, command, comment });
      toast.success("Cron entry added");
      setCommand(""); setComment("");
      onClose();
      await onAdded();
    } catch (err) {
      if (err instanceof ApiError) toast.error("Add failed", err.message);
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>New cron entry for {user}</DialogTitle>
          <DialogDescription>
            Standard 5-field syntax (<code>min hour dom mon dow</code>) or an alias like{" "}
            <code>@daily</code>.
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-4">
          <div className="space-y-1.5">
            <Label htmlFor="sched">Schedule</Label>
            <Input
              id="sched"
              value={schedule}
              onChange={(e) => setSchedule(e.target.value)}
              className="font-mono"
              required
            />
            <div className="flex gap-1.5 flex-wrap">
              {PRESETS.map((p) => (
                <button
                  key={p.value}
                  type="button"
                  onClick={() => setSchedule(p.value)}
                  className="rounded-md border border-input px-2 py-0.5 text-xs hover:bg-accent transition-colors"
                >
                  {p.label}
                </button>
              ))}
            </div>
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="cmd">Command</Label>
            <Input
              id="cmd"
              value={command}
              onChange={(e) => setCommand(e.target.value)}
              className="font-mono"
              placeholder="/usr/local/bin/backup.sh"
              required
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="cmt">Comment (optional)</Label>
            <Input
              id="cmt"
              value={comment}
              onChange={(e) => setComment(e.target.value)}
              placeholder="nightly backup"
            />
          </div>
          <div className="flex justify-end gap-2">
            <Button type="button" variant="ghost" onClick={onClose}>Cancel</Button>
            <Button type="submit" disabled={busy}>{busy ? "Adding…" : "Add"}</Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}
