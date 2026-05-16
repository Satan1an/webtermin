import { useEffect, useState } from "react";
import { ScrollText } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { api } from "@/lib/api";

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
