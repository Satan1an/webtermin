import { useState, type FormEvent } from "react";
import { useNavigate } from "react-router-dom";
import { motion } from "framer-motion";
import { Lock, ShieldCheck, TerminalSquare } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { api, ApiError } from "@/lib/api";
import { useAuth, type Role } from "@/store/auth";
import { toast } from "@/components/ui/toast";

export function LoginPage() {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [totp, setTotp] = useState("");
  const [needTotp, setNeedTotp] = useState(false);
  const [loading, setLoading] = useState(false);
  const auth = useAuth();
  const nav = useNavigate();

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setLoading(true);
    try {
      const me = await api.post<{
        user: string;
        is_admin: boolean;
        role: Role;
        csrf_token: string;
        has_2fa: boolean;
      }>("/api/auth/login", { username, password, totp });
      auth.set(me);
      toast.success("Welcome", `Signed in as ${me.user}`);
      nav("/dashboard");
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.message.includes("2FA") || err.message.includes("totp")) {
          setNeedTotp(true);
          toast("Enter your 6-digit code");
        } else {
          toast.error("Sign-in failed", err.message);
        }
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="grid h-full w-full place-items-center bg-background grid-bg">
      <motion.div
        initial={{ opacity: 0, y: 12 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.35, ease: "easeOut" }}
        className="w-full max-w-sm"
      >
        <Card className="border-border/80 shadow-2xl shadow-black/40 glass">
          <CardHeader className="space-y-3 text-center">
            <div className="mx-auto grid h-12 w-12 place-items-center rounded-xl bg-primary text-primary-foreground shadow-md">
              <TerminalSquare className="h-6 w-6" />
            </div>
            <CardTitle className="text-xl">webtermin</CardTitle>
            <CardDescription>Sign in to manage your server</CardDescription>
          </CardHeader>
          <CardContent>
            <form onSubmit={submit} className="space-y-4">
              <div className="space-y-1.5">
                <Label htmlFor="user">Username</Label>
                <Input
                  id="user"
                  autoFocus
                  autoComplete="username"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  required
                />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="pw">Password</Label>
                <Input
                  id="pw"
                  type="password"
                  autoComplete="current-password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required
                />
              </div>
              {needTotp && (
                <motion.div
                  initial={{ opacity: 0, height: 0 }}
                  animate={{ opacity: 1, height: "auto" }}
                  className="space-y-1.5"
                >
                  <Label htmlFor="totp">2FA code</Label>
                  <Input
                    id="totp"
                    inputMode="numeric"
                    pattern="\d{6}"
                    maxLength={6}
                    value={totp}
                    onChange={(e) => setTotp(e.target.value.replace(/\D/g, ""))}
                    placeholder="••••••"
                  />
                </motion.div>
              )}
              <Button type="submit" disabled={loading} className="w-full">
                {loading ? (
                  "Signing in…"
                ) : (
                  <>
                    <Lock className="h-4 w-4" />
                    Sign in
                  </>
                )}
              </Button>
            </form>
            <div className="mt-6 flex items-center justify-center gap-2 text-xs text-muted-foreground">
              <ShieldCheck className="h-3.5 w-3.5 text-success" />
              TLS · Argon2id · CSRF-protected
            </div>
          </CardContent>
        </Card>
      </motion.div>
    </div>
  );
}
