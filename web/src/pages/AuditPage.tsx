import { useEffect, useState } from "react";
import { BellRing, ScrollText, Send } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { api, ApiError } from "@/lib/api";
import { toast } from "@/components/ui/toast";

interface Entry {
  id: number;
  time: string;
  username: string;
  ip: string;
  action: string;
  target: string;
  outcome: string;
  detail: string;
}

export function AuditPage() {
  const [items, setItems] = useState<Entry[]>([]);

  useEffect(() => {
    void api.get<Entry[]>("/api/auth/audit").then((d) => setItems(d ?? []));
  }, []);

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Audit log</h1>
        <p className="text-sm text-muted-foreground">Last 200 events</p>
      </div>
      <AlertsCard />
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <ScrollText className="h-4 w-4" /> Events
          </CardTitle>
        </CardHeader>
        <CardContent className="p-0">
          <div className="divide-y divide-border">
            {items.map((e) => (
              <div key={e.id} className="grid grid-cols-12 gap-2 items-center px-5 py-2.5 text-sm">
                <div className="col-span-2 text-xs text-muted-foreground tabular-nums">
                  {new Date(e.time).toLocaleString()}
                </div>
                <div className="col-span-2 truncate">{e.username || "—"}</div>
                <div className="col-span-1 text-xs text-muted-foreground">{e.ip}</div>
                <div className="col-span-2 font-medium truncate">{e.action}</div>
                <div className="col-span-2 text-muted-foreground truncate">{e.target}</div>
                <div className="col-span-1">
                  <OutcomeBadge o={e.outcome} />
                </div>
                <div className="col-span-2 text-xs text-muted-foreground truncate">{e.detail}</div>
              </div>
            ))}
            {items.length === 0 && (
              <div className="p-8 text-center text-sm text-muted-foreground">No events yet.</div>
            )}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

function OutcomeBadge({ o }: { o: string }) {
  if (o === "ok") return <Badge variant="success">ok</Badge>;
  if (o === "denied") return <Badge variant="warning">denied</Badge>;
  return <Badge variant="destructive">{o}</Badge>;
}

interface NotifyStatus {
  enabled: boolean;
  events: string[] | null;
  disk_pct: number;
}

// AlertsCard shows whether Telegram alerts are configured and lets the admin
// send a test message. Configuration lives in config.yaml (notify.telegram),
// not in the UI, so the bot token never round-trips through the browser.
function AlertsCard() {
  const [st, setSt] = useState<NotifyStatus | null>(null);
  const [sending, setSending] = useState(false);

  useEffect(() => {
    void api.get<NotifyStatus>("/api/notify/status").then(setSt).catch(() => {});
  }, []);

  const test = async () => {
    setSending(true);
    try {
      await api.post("/api/notify/test");
      toast.success("Test alert sent", "Check the Telegram chat.");
    } catch (e) {
      if (e instanceof ApiError) toast.error("Telegram failed", e.message);
    } finally {
      setSending(false);
    }
  };

  if (!st) return null;
  return (
    <Card>
      <CardContent className="p-5 flex flex-wrap items-center justify-between gap-3">
        <div className="space-y-1">
          <div className="flex items-center gap-2 font-medium">
            <BellRing className="h-4 w-4 text-primary" /> Telegram alerts
            <Badge variant={st.enabled ? "success" : "muted"}>{st.enabled ? "on" : "off"}</Badge>
          </div>
          <p className="text-sm text-muted-foreground">
            {st.enabled ? (
              <>Sending: {(st.events ?? []).join(", ")} · disk alert at {st.disk_pct}%</>
            ) : (
              <>Set <code>notify.telegram.bot_token</code> and <code>chat_id</code> in <code>/etc/webtermin/config.yaml</code>, then restart webtermin.</>
            )}
          </p>
        </div>
        {st.enabled && (
          <Button size="sm" variant="outline" disabled={sending} onClick={() => void test()}>
            <Send className="h-4 w-4" /> Send test
          </Button>
        )}
      </CardContent>
    </Card>
  );
}
