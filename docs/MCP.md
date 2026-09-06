# Azula — MCP Integration Guide

**Model Context Protocol (MCP)** gives Azula agents read access to project files, repositories, and data sources during investigations.

---

## MVP: Filesystem Connector (Required)

### Purpose

Allow agents to read uploaded project files during Fast, Deep, and Council stages.

### Supported file types

`.log`, `.yaml`, `.yml`, `.py`, `.json`, `.jsonl`, `.csv`, `.txt`

### Storage layout

```
{MCP_FILE_ROOT}/
  └── {projectId}/
        ├── training.log
        ├── config.yaml
        ├── pipeline.py
        └── dataset.jsonl
```

Default `MCP_FILE_ROOT=./uploads`

### Sample pipeline

Onboarding copies files from [`samples/broken-pipeline/`](../samples/broken-pipeline/) into the sample project directory. For a single-cause missing-value case, upload [`samples/broken-nan-impute/`](../samples/broken-nan-impute/) into a new project.

### Cursor MCP config (this repo)

Project file: [`.cursor/mcp.json`](../.cursor/mcp.json). It starts **cursor-security** (`@cursor-security/mcp` from [gurkanfikretgunak/cursor-security](https://github.com/gurkanfikretgunak/cursor-security)) so Agent can run `security_scan_full`, `security_score`, secrets/client/backend/agent scanners, and SARIF export.

First-time setup (already done if `tools/cursor-security-mcp/dist/index.js` exists):

```bash
cd tools/cursor-security-mcp
npm install
npm run build
```

Then **Cursor → Settings → MCP** and enable `cursor-security` (approve the prompt). Ask: “Run a full security scan on this repo.”

Optional extra (sample pipeline files only):

```json
{
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": [
        "-y",
        "@modelcontextprotocol/server-filesystem",
        "C:/Users/aysnu/Projects/azula/samples/broken-pipeline"
      ]
    }
  }
}
```

### Security rules

- Agents read only within `{MCP_FILE_ROOT}/{projectId}/`
- Reject paths containing `..` (path traversal)
- Max file size: 50MB per upload
- Git clone: HTTPS only; no credentials in the URL; no private/loopback/link-local hosts (SSRF)
- File bodies sent to models are redacted and marked untrusted — never treated as instructions
- MCP reads are audit-logged (`mcp.read`)

See [AGENTIC_SECURITY.md](AGENTIC_SECURITY.md).

---

## MVP+1: Git Connector

### Purpose

Clone repositories, run blame/diff, and trace code changes linked to pipeline failures.

### Implementation

`internal/mcp/git.go` — all git subprocesses stay inside the MCP package. Resolvers never call `git` directly.

- Clone HTTPS URL + branch into `{MCP_FILE_ROOT}/{projectId}/.repo`
- Copy allowed file types (including nested paths, flattened as `dir__file.ext`) into the project directory and register them on the project record
- Persist clone URL / branch / HEAD on the project
- `git blame`, `git diff`, `git log` against the clone (nested paths allowed)
- Reject `file://`, SSH, and path traversal in refs

### GraphQL

`connectGitRepo`, `gitRepo`, `gitBlame`, `gitDiff`, `gitLog` — Pro-gated.

---

## Post-MVP: GitHub Connector

### Purpose

Pull PR, issue, and CI failure context into investigations.

### Use cases

- "Why did CI fail on this PR?"
- Link investigation to GitHub issue
- Fetch workflow run logs

---

## Post-MVP: MongoDB Connector (Optional)

### Purpose

Let agents query investigation history and analytics within a workspace.

### Use cases

- "Have we seen this root cause before?"
- Compare current incident to past investigations

---

## Agent access pattern

All MCP access goes through `MCPConnector` — never direct `fs` or shell calls from resolvers.

```typescript
// Correct
const content = await mcpConnector.readFile(projectId, 'training.log');

// Wrong
const content = fs.readFileSync('./uploads/...');
```

---

## Connector priority

| Connector | Phase | Priority |
|-----------|-------|----------|
| Filesystem | MVP | Required |
| Git | MVP+1 | High |
| GitHub | Post-MVP | Medium |
| MongoDB | Post-MVP | Nice-to-have |

---

## Related documents

- [ARCHITECTURE.md](ARCHITECTURE.md) — `MCPConnector` interface
- [MVP.md](MVP.md) — US-2 file upload requirements
- [ONBOARDING.md](ONBOARDING.md) — sample pipeline files
- `.cursor/skills/azula-investigation/SKILL.md` — dev workflow
