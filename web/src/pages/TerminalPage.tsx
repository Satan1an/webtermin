import { useEffect, useRef } from "react";
import { Terminal as XTerm } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { TerminalSquare } from "lucide-react";
import { wsURL } from "@/lib/api";

export function TerminalPage() {
  const ref = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!ref.current) return;
    const term = new XTerm({
      fontFamily: '"JetBrains Mono", ui-monospace, monospace',
      fontSize: 13,
      lineHeight: 1.25,
      cursorBlink: true,
      theme: {
        background: "#0a1020",
        foreground: "#e6edf3",
        cursor: "#00DC82",
        selectionBackground: "#1f4cff55",
        black: "#0a1020",
        red: "#ff6b6b",
        green: "#00dc82",
        yellow: "#ffd166",
        blue: "#60a5fa",
        magenta: "#c084fc",
        cyan: "#22d3ee",
        white: "#e6edf3",
        brightBlack: "#475569",
        brightRed: "#fb7185",
        brightGreen: "#34d399",
        brightYellow: "#fbbf24",
        brightBlue: "#93c5fd",
        brightMagenta: "#d8b4fe",
        brightCyan: "#67e8f9",
        brightWhite: "#f1f5f9",
      },
    });
    const fit = new FitAddon();
    term.loadAddon(fit);
    term.open(ref.current);
    fit.fit();

    const ws = new WebSocket(
      wsURL(`/api/terminal/ws?rows=${term.rows}&cols=${term.cols}`),
    );
    ws.binaryType = "arraybuffer";

    ws.onopen = () => {
      term.focus();
    };
    ws.onmessage = (ev) => {
      if (ev.data instanceof ArrayBuffer) {
        term.write(new Uint8Array(ev.data));
      } else if (typeof ev.data === "string") {
        try {
          const msg = JSON.parse(ev.data);
          if (msg.type === "closed") term.write(`\r\n\x1b[31m${msg.detail || "session closed"}\x1b[0m\r\n`);
        } catch {
          term.write(ev.data);
        }
      }
    };
    ws.onclose = () => {
      term.write("\r\n\x1b[33m[disconnected]\x1b[0m\r\n");
    };

    const onData = term.onData((d) => {
      if (ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify({ type: "data", data: d }));
    });
    const onResize = term.onResize(({ rows, cols }) => {
      if (ws.readyState === WebSocket.OPEN)
        ws.send(JSON.stringify({ type: "resize", rows, cols }));
    });

    const fitObserver = new ResizeObserver(() => fit.fit());
    fitObserver.observe(ref.current);

    return () => {
      onData.dispose();
      onResize.dispose();
      fitObserver.disconnect();
      ws.close();
      term.dispose();
    };
  }, []);

  return (
    <div className="space-y-4">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Terminal</h1>
        <p className="text-sm text-muted-foreground">
          Interactive shell — same privileges as the panel process owner.
        </p>
      </div>
      <Card className="overflow-hidden">
        <CardHeader className="bg-card/80 border-b border-border py-3">
          <CardTitle className="flex items-center gap-2 text-sm">
            <TerminalSquare className="h-4 w-4 text-primary" />
            <span className="font-mono">/dev/pts</span>
          </CardTitle>
        </CardHeader>
        <CardContent className="p-0">
          <div ref={ref} className="h-[70vh] bg-[#0a1020] p-2" />
        </CardContent>
      </Card>
    </div>
  );
}
