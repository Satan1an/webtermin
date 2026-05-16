import { useEffect, useRef, useState } from "react";
import {
  Area,
  AreaChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip as RTooltip,
  XAxis,
  YAxis,
} from "recharts";
import {
  Cpu,
  HardDrive,
  Layers,
  MemoryStick,
  Network,
  Server,
  Timer,
} from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import { Badge } from "@/components/ui/badge";
import { api, wsURL } from "@/lib/api";
import { formatBytes, formatDuration } from "@/lib/utils";

interface SystemInfo {
  hostname: string;
  os: string;
  platform: string;
  platform_version: string;
  kernel_version: string;
  arch: string;
  cpu_model: string;
  cpu_cores: number;
  uptime_sec: number;
}

interface DiskMetric {
  device: string;
  mount: string;
  fstype: string;
  total: number;
  used: number;
  used_pct: number;
}

interface NetMetric {
  iface: string;
  bytes_sent: number;
  bytes_recv: number;
}

interface Metrics {
  time: string;
  cpu_pct: number;
  load: [number, number, number];
  mem: { total: number; used: number; used_pct: number };
  swap: { total: number; used: number; used_pct: number };
  disks: DiskMetric[];
  network: NetMetric[];
}

interface Sample {
  t: number;
  cpu: number;
  mem: number;
  netRx: number;
  netTx: number;
}

const MAX_SAMPLES = 60;

export function DashboardPage() {
  const [info, setInfo] = useState<SystemInfo | null>(null);
  const [m, setM] = useState<Metrics | null>(null);
  const [samples, setSamples] = useState<Sample[]>([]);
  const lastNet = useRef<{ rx: number; tx: number; t: number } | null>(null);

  useEffect(() => {
    void api.get<SystemInfo>("/api/system/info").then(setInfo).catch(() => {});
    const ws = new WebSocket(wsURL("/api/system/metrics/stream"));
    ws.onmessage = (ev) => {
      try {
        const data = JSON.parse(ev.data as string) as Metrics;
        setM(data);
        setSamples((cur) => {
          const now = Date.now();
          const totalRx = data.network.reduce((a, n) => a + n.bytes_recv, 0);
          const totalTx = data.network.reduce((a, n) => a + n.bytes_sent, 0);
          let netRx = 0;
          let netTx = 0;
          if (lastNet.current) {
            const dt = (now - lastNet.current.t) / 1000;
            if (dt > 0) {
              netRx = Math.max(0, (totalRx - lastNet.current.rx) / dt);
              netTx = Math.max(0, (totalTx - lastNet.current.tx) / dt);
            }
          }
          lastNet.current = { rx: totalRx, tx: totalTx, t: now };
          const next = [
            ...cur,
            {
              t: now,
              cpu: data.cpu_pct,
              mem: data.mem.used_pct,
              netRx,
              netTx,
            },
          ];
          return next.length > MAX_SAMPLES ? next.slice(-MAX_SAMPLES) : next;
        });
      } catch {
        // ignore
      }
    };
    return () => ws.close();
  }, []);

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Dashboard</h1>
        <p className="text-sm text-muted-foreground">
          {info?.hostname ?? "…"} · {info?.platform ?? ""} {info?.platform_version ?? ""} ·{" "}
          {info?.kernel_version ?? ""}
        </p>
      </div>

      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        <StatCard
          icon={<Cpu className="h-4 w-4" />}
          title="CPU"
          value={`${(m?.cpu_pct ?? 0).toFixed(1)}%`}
          tone={cpuTone(m?.cpu_pct ?? 0)}
          progress={m?.cpu_pct ?? 0}
          sub={
            m?.load
              ? `load ${m.load[0].toFixed(2)} / ${m.load[1].toFixed(2)} / ${m.load[2].toFixed(2)}`
              : ""
          }
        />
        <StatCard
          icon={<MemoryStick className="h-4 w-4" />}
          title="Memory"
          value={`${(m?.mem.used_pct ?? 0).toFixed(1)}%`}
          tone={cpuTone(m?.mem.used_pct ?? 0)}
          progress={m?.mem.used_pct ?? 0}
          sub={m ? `${formatBytes(m.mem.used)} / ${formatBytes(m.mem.total)}` : ""}
        />
        <StatCard
          icon={<Layers className="h-4 w-4" />}
          title="Swap"
          value={`${(m?.swap.used_pct ?? 0).toFixed(1)}%`}
          tone={cpuTone(m?.swap.used_pct ?? 0)}
          progress={m?.swap.used_pct ?? 0}
          sub={m ? `${formatBytes(m.swap.used)} / ${formatBytes(m.swap.total)}` : ""}
        />
        <StatCard
          icon={<Timer className="h-4 w-4" />}
          title="Uptime"
          value={info ? formatDuration(info.uptime_sec) : "—"}
          tone="default"
          sub={info ? `${info.cpu_cores} cores · ${info.arch}` : ""}
        />
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Cpu className="h-4 w-4" />
              CPU & Memory
            </CardTitle>
          </CardHeader>
          <CardContent className="h-64">
            <ResponsiveContainer width="100%" height="100%">
              <AreaChart data={samples}>
                <defs>
                  <linearGradient id="cpuG" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stopColor="hsl(var(--primary))" stopOpacity={0.45} />
                    <stop offset="100%" stopColor="hsl(var(--primary))" stopOpacity={0} />
                  </linearGradient>
                  <linearGradient id="memG" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stopColor="hsl(var(--warning))" stopOpacity={0.35} />
                    <stop offset="100%" stopColor="hsl(var(--warning))" stopOpacity={0} />
                  </linearGradient>
                </defs>
                <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" opacity={0.4} />
                <XAxis dataKey="t" hide />
                <YAxis domain={[0, 100]} tick={{ fontSize: 11, fill: "hsl(var(--muted-foreground))" }} width={30} />
                <RTooltip content={<TooltipBody />} />
                <Area
                  type="monotone"
                  dataKey="cpu"
                  stroke="hsl(var(--primary))"
                  fill="url(#cpuG)"
                  strokeWidth={2}
                  name="CPU %"
                  isAnimationActive={false}
                />
                <Area
                  type="monotone"
                  dataKey="mem"
                  stroke="hsl(var(--warning))"
                  fill="url(#memG)"
                  strokeWidth={2}
                  name="Memory %"
                  isAnimationActive={false}
                />
              </AreaChart>
            </ResponsiveContainer>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Network className="h-4 w-4" />
              Network throughput
            </CardTitle>
          </CardHeader>
          <CardContent className="h-64">
            <ResponsiveContainer width="100%" height="100%">
              <AreaChart data={samples}>
                <defs>
                  <linearGradient id="rxG" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stopColor="hsl(var(--primary))" stopOpacity={0.45} />
                    <stop offset="100%" stopColor="hsl(var(--primary))" stopOpacity={0} />
                  </linearGradient>
                  <linearGradient id="txG" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stopColor="hsl(280 80% 60%)" stopOpacity={0.45} />
                    <stop offset="100%" stopColor="hsl(280 80% 60%)" stopOpacity={0} />
                  </linearGradient>
                </defs>
                <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" opacity={0.4} />
                <XAxis dataKey="t" hide />
                <YAxis tickFormatter={(v) => formatBytes(v) + "/s"} tick={{ fontSize: 11, fill: "hsl(var(--muted-foreground))" }} width={70} />
                <RTooltip content={<TooltipBody isBytes />} />
                <Area
                  type="monotone"
                  dataKey="netRx"
                  stroke="hsl(var(--primary))"
                  fill="url(#rxG)"
                  strokeWidth={2}
                  name="RX"
                  isAnimationActive={false}
                />
                <Area
                  type="monotone"
                  dataKey="netTx"
                  stroke="hsl(280 80% 60%)"
                  fill="url(#txG)"
                  strokeWidth={2}
                  name="TX"
                  isAnimationActive={false}
                />
              </AreaChart>
            </ResponsiveContainer>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <HardDrive className="h-4 w-4" />
            Filesystems
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          {m?.disks?.length ? (
            m.disks.map((d) => (
              <div key={d.mount} className="space-y-1.5">
                <div className="flex items-baseline justify-between text-sm">
                  <div className="flex items-center gap-2">
                    <span className="font-medium">{d.mount}</span>
                    <Badge variant="muted">{d.fstype}</Badge>
                    <span className="text-xs text-muted-foreground">{d.device}</span>
                  </div>
                  <span className="text-xs text-muted-foreground">
                    {formatBytes(d.used)} / {formatBytes(d.total)} ({d.used_pct.toFixed(0)}%)
                  </span>
                </div>
                <Progress value={d.used_pct} tone={cpuTone(d.used_pct)} />
              </div>
            ))
          ) : (
            <div className="text-sm text-muted-foreground">No mounted filesystems</div>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Server className="h-4 w-4" />
            About this server
          </CardTitle>
        </CardHeader>
        <CardContent>
          <dl className="grid grid-cols-1 sm:grid-cols-2 gap-x-6 gap-y-2 text-sm">
            <KV k="Hostname" v={info?.hostname} />
            <KV k="OS" v={`${info?.platform ?? ""} ${info?.platform_version ?? ""}`} />
            <KV k="Kernel" v={info?.kernel_version} />
            <KV k="Architecture" v={info?.arch} />
            <KV k="CPU" v={info?.cpu_model} />
            <KV k="Cores" v={info?.cpu_cores?.toString()} />
          </dl>
        </CardContent>
      </Card>
    </div>
  );
}

function StatCard({
  icon, title, value, sub, progress, tone,
}: {
  icon: React.ReactNode;
  title: string;
  value: string;
  sub?: string;
  progress?: number;
  tone: "default" | "warning" | "danger";
}) {
  return (
    <Card>
      <CardHeader className="pb-2 flex flex-row items-center justify-between space-y-0">
        <CardTitle className="text-xs uppercase tracking-wider text-muted-foreground font-medium">
          {title}
        </CardTitle>
        <span className="text-muted-foreground">{icon}</span>
      </CardHeader>
      <CardContent className="space-y-2">
        <div className="text-2xl font-semibold tracking-tight tabular-nums">{value}</div>
        {progress !== undefined && <Progress value={progress} tone={tone} />}
        {sub && <div className="text-xs text-muted-foreground">{sub}</div>}
      </CardContent>
    </Card>
  );
}

function KV({ k, v }: { k: string; v?: string }) {
  return (
    <div className="flex justify-between gap-3">
      <dt className="text-muted-foreground">{k}</dt>
      <dd className="font-medium text-right truncate">{v || "—"}</dd>
    </div>
  );
}

function TooltipBody({ active, payload, isBytes }: any) {
  if (!active || !payload?.length) return null;
  return (
    <div className="rounded-md border bg-card p-2 text-xs shadow">
      {payload.map((p: any) => (
        <div key={p.dataKey} className="flex items-center gap-2">
          <span className="h-2 w-2 rounded-full" style={{ background: p.color }} />
          <span className="text-muted-foreground">{p.name}</span>
          <span className="font-medium tabular-nums">
            {isBytes ? formatBytes(p.value) + "/s" : `${p.value.toFixed(1)}%`}
          </span>
        </div>
      ))}
    </div>
  );
}

function cpuTone(pct: number): "default" | "warning" | "danger" {
  if (pct >= 90) return "danger";
  if (pct >= 70) return "warning";
  return "default";
}
