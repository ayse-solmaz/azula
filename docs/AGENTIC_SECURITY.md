# Agentic security in Azula

Mapped from the [Agentic AI Security Manifest](https://github.com/gurkanfikretgunak/cursor-security/blob/main/MANIFEST.md).

| Principle | Azula enforcement |
| --- | --- |
| Least agency | MCP Files is project-scoped; Git is clone/blame/diff/log only; HTTPS + no private IPs |
| Identity before action | JWT in HttpOnly cookie (web) or OS `safeStorage` in the Electron main process (desktop never keeps the JWT in `localStorage`) |
| Tool trust zero default | No host shell for the agent; uploads allowlisted by extension |
| Prompt ≠ policy | RBAC, tier gates, and path checks in Go |
| Human control | Stop agent on the investigation page; `AZULA_KILL_SWITCH` |
| Observable | Audit: `agent.start`, `agent.cancel`, `mcp.read` |
| Memory is sensitive | Secrets redacted before LLM payloads |
| Blast radius | Worker slots, request timeouts, auth rate limits, complexity limit |
| Adversarial eval | Untrusted-file wrappers; gold-set tests |
| Ownership | Named investigation `userId`; org roles |

Threat notes and headers: [ARCHITECTURE.md](ARCHITECTURE.md) §11, [SECURITY.md](SECURITY.md).

Cursor MCP: [`.cursor/mcp.json`](../.cursor/mcp.json) runs `@cursor-security/mcp` from `tools/cursor-security-mcp`. Enable it under **Settings → MCP**, then ask for `security_scan_full` / `security_score`.
