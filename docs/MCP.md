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

Onboarding copies files from [`samples/broken-pipeline/`](../samples/broken-pipeline/) into the sample project directory.

### Cursor MCP config (development)

Add to your Cursor MCP settings (`~/.cursor/mcp.json` or project-level):

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

Adjust the path to your local `samples/broken-pipeline` or `uploads/{projectId}` directory.

### Security rules

- Agents read only within `{MCP_FILE_ROOT}/{projectId}/`
- Reject paths containing `..` (path traversal)
- Max file size: 50MB per upload

---

## MVP+1: Git Connector

### Purpose

Clone repositories, run blame/diff, and trace code changes linked to pipeline failures.

### Planned capabilities

- Clone repo by URL + branch
- `git log`, `git blame`, `git diff` for changed files
- Link commits to investigation timeline

### Cursor MCP config (future)

```json
{
  "mcpServers": {
    "git": {
      "command": "uvx",
      "args": ["mcp-server-git", "--repository", "/path/to/repo"]
    }
  }
}
```

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
