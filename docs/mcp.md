# AI agents over MCP

webtermin speaks the [Model Context Protocol](https://modelcontextprotocol.io), so an AI agent —
Claude Code, Claude Desktop, Cursor, or any MCP client — can look after the host **through the
same typed, role-checked operations as the UI, never a shell**.

> **Status: read-only.** This version lets an agent diagnose (status, services, containers, logs,
> network, pending updates) and *dry-run* package changes. Nothing it calls changes the host.
> Changes will arrive with an approval queue the agent cannot answer itself — the plan goes to
> you in Telegram or the panel, and only your "yes" runs it.

## Connect

1. **Create a token** in *API tokens*. Its role decides which tools the agent even sees, so start
   with `viewer` (or `operator` if it should read logs). The reveal dialog shows the exact command.
2. **Add the server** to your client. Endpoint: `https://<host>:8443/api/mcp` (streamable HTTP).

Claude Code:

```bash
claude mcp add --transport http webtermin https://pi.lan:8443/api/mcp \
  --header 'Authorization: Bearer wt_…'
```

Cursor / Claude Desktop / other clients (`mcp.json`-style config):

```json
{
  "mcpServers": {
    "webtermin": {
      "type": "http",
      "url": "https://pi.lan:8443/api/mcp",
      "headers": { "Authorization": "Bearer wt_…" }
    }
  }
}
```

### Self-signed certificate

With the default auto-generated certificate, Node-based clients refuse the connection. Trust the
panel's own certificate instead of turning verification off:

```bash
scp root@pi.lan:/var/lib/webtermin/tls/cert.pem ~/.config/webtermin-pi.pem
export NODE_EXTRA_CA_CERTS=~/.config/webtermin-pi.pem   # then start the client
```

Or point `server.tls_cert` / `tls_key` at a CA-issued certificate.

## Tools

| Tool | Min role | What it returns |
|---|---|---|
| `host_status` | viewer | OS, kernel, uptime, CPU/memory/disk, reboot required (and why), pending updates |
| `list_services` | viewer | systemd services; `state` = failed / active / inactive / all, optional `query` |
| `list_containers` | viewer | Docker containers with image, state and compose stack |
| `list_updates` | viewer | Pending package updates, security fixes flagged |
| `service_logs` | operator | Last N journal lines of a unit |
| `container_logs` | operator | Last N log lines of a container (name or id) |
| `network_interfaces` | admin | Interfaces, addresses, gateway, DNS, managing stack |
| `package_plan` | admin | Dry run of install / remove / upgrade: what changes, warnings, what the safety checks block |
| `recent_audit` | admin | Latest audit-log entries |

The roles mirror the REST API: logs can carry secrets, so they need `operator`; package plans,
network and the audit log need `admin`. A tool the token's role doesn't allow is not listed, and
calling it anyway fails with "unknown tool".

## Security model

- **API tokens only.** Browser sessions are refused at `/api/mcp`, so a page can't ride your
  cookie into it. Revoke the token and the agent is out.
- **Typed operations.** Inputs are validated (unit names, container ids, package names) exactly
  like the REST handlers — there is no command string to smuggle anything through.
- **Everything is audited** as `mcp.<tool>` with the token name, next to what humans did.
- **Logs are untrusted.** The server tells the client that journal and container output is data,
  not instructions; still, prompt injection through logs has no general fix, which is why
  changes will need an approval the agent can't give itself.
