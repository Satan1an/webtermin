import { useEffect, useMemo, useState } from "react";
import { Boxes, Download, Search, Trash2 } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { api, ApiError } from "@/lib/api";
import { toast } from "@/components/ui/toast";
import { useIsAdmin } from "@/store/auth";

interface Pkg {
  name: string;
  version?: string;
  description?: string;
  installed?: boolean;
}

export function PackagesPage() {
  const isAdmin = useIsAdmin();
  const [manager, setManager] = useState<string>("");
  const [tab, setTab] = useState<"installed" | "search">("installed");
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<Pkg[]>([]);
  const [installed, setInstalled] = useState<Pkg[]>([]);
  const [busy, setBusy] = useState<string | null>(null);

  useEffect(() => {
    void api.get<{ manager: string }>("/api/packages/status").then((s) => setManager(s.manager));
  }, []);

  const loadInstalled = async () => {
    try {
      setInstalled((await api.get<Pkg[]>("/api/packages/installed")) ?? []);
    } catch (e) {
      if (e instanceof ApiError) toast.error("Failed", e.message);
    }
  };
  useEffect(() => { if (tab === "installed") void loadInstalled(); }, [tab]);

  const search = async () => {
    if (!query) return;
    try {
      setResults((await api.get<Pkg[]>(`/api/packages/search?q=${encodeURIComponent(query)}`)) ?? []);
      setTab("search");
    } catch (e) {
      if (e instanceof ApiError) toast.error("Search failed", e.message);
    }
  };

  const op = async (name: string, action: "install" | "remove" | "upgrade") => {
    if (!confirm(`${action} package "${name}"?`)) return;
    setBusy(name + action);
    try {
      await api.post(`/api/packages/${action}`, { names: [name] });
      toast.success(`${action} ✓`, name);
      await loadInstalled();
      if (tab === "search") await search();
    } catch (e) {
      if (e instanceof ApiError) toast.error(`${action} failed`, e.message);
    } finally {
      setBusy(null);
    }
  };

  if (!manager) {
    return (
      <Card>
        <CardContent className="p-6">
          <Boxes className="h-6 w-6 text-muted-foreground mb-3" />
          <strong>No supported package manager found.</strong>
          <p className="mt-2 text-sm text-muted-foreground">
            webtermin supports <code>apt-get</code> (Debian/Ubuntu) and <code>dnf</code> (Fedora/RHEL/Alma) at v0.8.
          </p>
        </CardContent>
      </Card>
    );
  }

  const display = tab === "search" ? results : installed;

  return (
    <div className="space-y-6">
      <div className="flex items-end justify-between gap-4 flex-wrap">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Packages</h1>
          <p className="text-sm text-muted-foreground">
            Managed via <code>{manager}</code>. Installs run non-interactively.
          </p>
        </div>
        <form
          onSubmit={(e) => { e.preventDefault(); void search(); }}
          className="flex gap-2"
        >
          <Input
            placeholder="Search package…"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            className="w-64"
          />
          <Button type="submit">
            <Search className="h-4 w-4" /> Search
          </Button>
        </form>
      </div>

      <div className="rounded-md border border-border p-0.5 flex gap-0.5 w-fit">
        {(["installed", "search"] as const).map((t) => (
          <button
            key={t}
            onClick={() => setTab(t)}
            className={`px-3 py-1.5 text-sm rounded-sm transition-colors ${
              tab === t ? "bg-primary/10 text-primary" : "text-muted-foreground hover:text-foreground"
            }`}
          >
            {t === "installed" ? `Installed (${installed.length})` : `Search (${results.length})`}
          </button>
        ))}
      </div>

      <Card>
        <CardContent className="p-0">
          <div className="divide-y divide-border">
            {display.map((p) => (
              <div key={p.name} className="grid grid-cols-12 gap-3 items-center px-5 py-3 hover:bg-accent/30">
                <div className="col-span-3 font-mono text-sm truncate">{p.name}</div>
                <div className="col-span-2 text-xs text-muted-foreground tabular-nums truncate">{p.version}</div>
                <div className="col-span-5 text-sm text-muted-foreground truncate">{p.description}</div>
                <div className="col-span-1">
                  {p.installed && <Badge variant="success">installed</Badge>}
                </div>
                <div className="col-span-1 flex justify-end gap-1">
                  {isAdmin && !p.installed && (
                    <Button size="icon" variant="ghost" title="Install" disabled={!!busy} onClick={() => op(p.name, "install")}>
                      <Download className="h-4 w-4" />
                    </Button>
                  )}
                  {isAdmin && p.installed && (
                    <Button size="icon" variant="ghost" title="Remove" disabled={!!busy} onClick={() => op(p.name, "remove")}>
                      <Trash2 className="h-4 w-4 text-destructive" />
                    </Button>
                  )}
                </div>
              </div>
            ))}
            {display.length === 0 && (
              <div className="p-8 text-center text-sm text-muted-foreground">
                {tab === "search" ? "Type a query and press Enter." : "No packages."}
              </div>
            )}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
