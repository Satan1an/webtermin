import { useState, type FormEvent } from "react";
import { motion } from "framer-motion";
import { Sparkles, TerminalSquare } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { api, ApiError } from "@/lib/api";
import { toast } from "@/components/ui/toast";
import { useAuth, type Role } from "@/store/auth";

export function SetupPage() {
  const [u, setU] = useState("");
  const [p, setP] = useState("");
  const [p2, setP2] = useState("");
  const [loading, setLoading] = useState(false);
  const auth = useAuth();

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (p !== p2) {
      toast.error("Passwords don't match");
      return;
    }
    setLoading(true);
    try {
      await api.post("/api/auth/setup", { username: u, password: p });
      // immediately log in
      const me = await api.post<{
        user: string;
        is_admin: boolean;
        role: Role;
        csrf_token: string;
        has_2fa: boolean;
      }>("/api/auth/login", { username: u, password: p });
      auth.set(me);
      auth.setNeedsSetup(false);
      toast.success("Welcome", `Admin account created for ${me.user}`);
    } catch (err) {
      if (err instanceof ApiError) toast.error("Setup failed", err.message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="grid h-full w-full place-items-center bg-background grid-bg">
      <motion.div
        initial={{ opacity: 0, y: 12 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.35 }}
        className="w-full max-w-md"
      >
        <Card className="border-border/80 shadow-2xl shadow-black/40 glass">
          <CardHeader className="space-y-3 text-center">
            <div className="mx-auto grid h-12 w-12 place-items-center rounded-xl bg-primary text-primary-foreground shadow-md">
              <Sparkles className="h-6 w-6" />
            </div>
            <CardTitle>First-run setup</CardTitle>
            <CardDescription>Create the first administrator</CardDescription>
          </CardHeader>
          <CardContent>
            <form onSubmit={submit} className="space-y-4">
              <div className="space-y-1.5">
                <Label htmlFor="u">Admin username</Label>
                <Input
                  id="u"
                  value={u}
                  onChange={(e) => setU(e.target.value)}
                  autoFocus
                  required
                />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="p">Password (≥ 10 chars)</Label>
                <Input
                  id="p"
                  type="password"
                  value={p}
                  onChange={(e) => setP(e.target.value)}
                  minLength={10}
                  required
                />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="p2">Confirm password</Label>
                <Input
                  id="p2"
                  type="password"
                  value={p2}
                  onChange={(e) => setP2(e.target.value)}
                  minLength={10}
                  required
                />
              </div>
              <Button type="submit" disabled={loading} className="w-full">
                {loading ? "Creating…" : (<><TerminalSquare className="h-4 w-4" /> Create admin</>)}
              </Button>
            </form>
          </CardContent>
        </Card>
      </motion.div>
    </div>
  );
}
