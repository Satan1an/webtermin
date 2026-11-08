import { useEffect, useState, type ComponentType } from "react";
import { Download } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { api } from "@/lib/api";
import { useIsAdmin } from "@/store/auth";
import { usePackageOp } from "@/components/PackageOpDialog";

interface Props {
  icon: ComponentType<{ className?: string }>;
  title: string;
  // Package name per manager; the backend picks apt or dnf at runtime.
  apt: string;
  dnf: string;
  onInstalled: () => void | Promise<void>;
}

// Placeholder for pages whose backing tool is missing. Admins get a one-click
// install (dry run + confirm, see usePackageOp); everyone else sees the command.
export function MissingTool({ icon: Icon, title, apt, dnf, onInstalled }: Props) {
  const isAdmin = useIsAdmin();
  const [manager, setManager] = useState("");
  const { start, busy, dialog } = usePackageOp();

  useEffect(() => {
    void api.get<{ manager: string }>("/api/packages/status").then((s) => setManager(s.manager)).catch(() => {});
  }, []);

  const pkg = manager === "dnf" ? dnf : apt;
  const cmd = manager === "dnf" ? `sudo dnf install ${dnf}` : `sudo apt install ${apt}`;

  return (
    <Card>
      <CardContent className="p-6 text-sm">
        <Icon className="h-6 w-6 text-muted-foreground mb-3" />
        <strong>{title}</strong>
        <p className="mt-2 text-muted-foreground">
          Install it with <code className="rounded bg-muted px-1.5 py-0.5">{cmd}</code> and refresh this page.
        </p>
        {isAdmin && manager && (
          <Button className="mt-4" onClick={() => void start("install", [pkg], { onDone: onInstalled })} disabled={!!busy}>
            <Download className="h-4 w-4" /> {busy ? `Working on ${pkg}…` : `Install ${pkg}`}
          </Button>
        )}
        {dialog}
      </CardContent>
    </Card>
  );
}
