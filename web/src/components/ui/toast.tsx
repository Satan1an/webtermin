// Lightweight toaster — a thin wrapper around a global event bus that pushes
// dismissable cards into a portal. Avoids pulling in @radix-ui/react-toast's
// full API surface for our small use case.
import * as React from "react";
import { motion, AnimatePresence } from "framer-motion";
import { cn } from "@/lib/utils";

type ToastKind = "default" | "success" | "error";
interface Toast {
  id: number;
  kind: ToastKind;
  title: string;
  description?: string;
}

let _nextId = 1;
const listeners = new Set<(t: Toast) => void>();

export function toast(title: string, description?: string, kind: ToastKind = "default") {
  const t: Toast = { id: _nextId++, kind, title, description };
  listeners.forEach((l) => l(t));
}
toast.success = (t: string, d?: string) => toast(t, d, "success");
toast.error = (t: string, d?: string) => toast(t, d, "error");

export function Toaster() {
  const [items, setItems] = React.useState<Toast[]>([]);
  React.useEffect(() => {
    const handler = (t: Toast) => {
      setItems((cur) => [...cur, t]);
      setTimeout(() => {
        setItems((cur) => cur.filter((x) => x.id !== t.id));
      }, 4500);
    };
    listeners.add(handler);
    return () => {
      listeners.delete(handler);
    };
  }, []);
  return (
    <div className="pointer-events-none fixed bottom-4 right-4 z-50 flex w-full max-w-sm flex-col gap-2">
      <AnimatePresence>
        {items.map((t) => (
          <motion.div
            key={t.id}
            initial={{ opacity: 0, y: 16, scale: 0.96 }}
            animate={{ opacity: 1, y: 0, scale: 1 }}
            exit={{ opacity: 0, y: 8, scale: 0.96 }}
            transition={{ duration: 0.18, ease: "easeOut" }}
            className={cn(
              "pointer-events-auto rounded-xl border bg-card/95 backdrop-blur-md shadow-xl p-3 text-sm",
              t.kind === "success" && "border-success/40",
              t.kind === "error" && "border-destructive/50",
            )}
          >
            <div className="font-medium">{t.title}</div>
            {t.description && (
              <div className="mt-0.5 text-muted-foreground">{t.description}</div>
            )}
          </motion.div>
        ))}
      </AnimatePresence>
    </div>
  );
}
