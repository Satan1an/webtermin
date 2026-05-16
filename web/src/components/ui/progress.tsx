import * as React from "react";
import { cn } from "@/lib/utils";

interface ProgressProps extends React.HTMLAttributes<HTMLDivElement> {
  value: number;
  max?: number;
  tone?: "default" | "warning" | "danger";
}

export const Progress = React.forwardRef<HTMLDivElement, ProgressProps>(
  ({ className, value, max = 100, tone = "default", ...props }, ref) => {
    const pct = Math.max(0, Math.min(100, (value / max) * 100));
    const colour =
      tone === "danger"
        ? "bg-destructive"
        : tone === "warning"
        ? "bg-warning"
        : "bg-primary";
    return (
      <div
        ref={ref}
        className={cn("h-2 w-full overflow-hidden rounded-full bg-secondary", className)}
        {...props}
      >
        <div
          className={cn("h-full transition-all duration-500", colour)}
          style={{ width: `${pct}%` }}
        />
      </div>
    );
  },
);
Progress.displayName = "Progress";
