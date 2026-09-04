import { FormEvent, useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { gql, INV_FIELDS, Investigation, uploadProjectFile, User, Workspace } from "../api";

const WORKSPACES_QUERY = `query {
  me { id email orgRole }
  workspaces {
    id name
    projects {
      id workspaceId name isSample
      files { name mimeType uploadedAt }
      investigations { id status createdAt fastResult { incidentType confidence } }
    }
  }
}`;

type FileVer = { version: number; uploadedAt: string };

type Compare = {
  projectId: string;
  fileName: string;
  versions: FileVer[];
  current: string;
  left: "current" | number;
  right: number;
  leftBody: string;
  rightBody: string;
};

function canEdit(role: string | null | undefined) {
  return role !== "viewer";
}

export default function HomePage() {
  const nav = useNavigate();
  const [me, setMe] = useState<User | null>(null);
  const [workspaces, setWorkspaces] = useState<Workspace[]>([]);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [newName, setNewName] = useState("");
  const [compare, setCompare] = useState<Compare | null>(null);

  const editable = canEdit(me?.orgRole);
  const bootstrapped = useRef(false);

  async function load() {
    const data = await gql<{ me: User; workspaces: Workspace[] }>(WORKSPACES_QUERY);
    setMe(data.me);
    let spaces = data.workspaces;
    if (!spaces.length) {
      await gql(`mutation { createWorkspace(name: "My ML Lab") { id name } }`);
      const again = await gql<{ workspaces: Workspace[] }>(WORKSPACES_QUERY);
      spaces = again.workspaces;
    }
    setWorkspaces(spaces);
    const first = spaces[0];
    if (first && first.projects.length === 0 && canEdit(data.me.orgRole) && !bootstrapped.current) {
      bootstrapped.current = true;
      try {
        await gql(
          `mutation ($workspaceId: ID!) {
            createProject(workspaceId: $workspaceId, name: "sample-broken-pipeline", isSample: true) { id }
          }`,
          { workspaceId: first.id }
        );
        const seeded = await gql<{ me: User; workspaces: Workspace[] }>(WORKSPACES_QUERY);
        setMe(seeded.me);
        setWorkspaces(seeded.workspaces);
      } catch (e) {
        bootstrapped.current = false;
        throw e;
      }
    }
  }

  useEffect(() => {
    load().catch((e) => setError(e.message));
  }, []);

  async function seedSample(workspaceId: string) {
    setBusy(true);
    setError("");
    try {
      await gql(
        `mutation ($workspaceId: ID!) {
          createProject(workspaceId: $workspaceId, name: "sample-broken-pipeline", isSample: true) { id }
        }`,
        { workspaceId }
      );
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed");
    } finally {
      setBusy(false);
    }
  }

  async function createProject(e: FormEvent, workspaceId: string) {
    e.preventDefault();
    if (!newName.trim()) return;
    setBusy(true);
    setError("");
    try {
      await gql(
        `mutation ($workspaceId: ID!, $name: String!) {
          createProject(workspaceId: $workspaceId, name: $name) { id }
        }`,
        { workspaceId, name: newName.trim() }
      );
      setNewName("");
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed");
    } finally {
      setBusy(false);
    }
  }

  async function onUpload(projectId: string, fileList: FileList | null) {
    if (!fileList?.length) return;
    setBusy(true);
    setError("");
    try {
      for (const file of Array.from(fileList)) {
        await uploadProjectFile(projectId, file);
      }
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Upload failed");
    } finally {
      setBusy(false);
    }
  }

  async function startInv(projectId: string) {
    setBusy(true);
    setError("");
    try {
      const data = await gql<{ startInvestigation: Investigation }>(
        `mutation ($projectId: ID!) {
          startInvestigation(projectId: $projectId) { ${INV_FIELDS} }
        }`,
        { projectId }
      );
      nav(`/investigation/${data.startInvestigation.id}`);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed");
    } finally {
      setBusy(false);
    }
  }

  async function openCompare(projectId: string, fileName: string) {
    setBusy(true);
    setError("");
    try {
      const data = await gql<{
        fileVersions: FileVer[];
        fileContent: string;
      }>(
        `query ($id: ID!, $name: String!) {
          fileVersions(projectId: $id, fileName: $name) { version uploadedAt }
          fileContent(projectId: $id, name: $name)
        }`,
        { id: projectId, name: fileName }
      );
      const versions = [...data.fileVersions].sort((a, b) => b.version - a.version);
      const right = versions[0]?.version ?? 0;
      let rightBody = data.fileContent;
      if (right) {
        rightBody = await gql<{ fileVersionContent: string }>(
          `query ($id: ID!, $name: String!, $v: Int!) {
            fileVersionContent(projectId: $id, fileName: $name, version: $v)
          }`,
          { id: projectId, name: fileName, v: right }
        ).then((d) => d.fileVersionContent);
      }
      setCompare({
        projectId,
        fileName,
        versions,
        current: data.fileContent,
        left: "current",
        right: right || 0,
        leftBody: data.fileContent,
        rightBody,
      });
    } catch (e) {
      setError(e instanceof Error ? e.message : "Compare failed");
    } finally {
      setBusy(false);
    }
  }

  async function loadSide(c: Compare, side: "left" | "right", value: "current" | number) {
    setBusy(true);
    setError("");
    try {
      let body = c.current;
      if (value !== "current") {
        const data = await gql<{ fileVersionContent: string }>(
          `query ($id: ID!, $name: String!, $v: Int!) {
            fileVersionContent(projectId: $id, fileName: $name, version: $v)
          }`,
          { id: c.projectId, name: c.fileName, v: value }
        );
        body = data.fileVersionContent;
      }
      setCompare({
        ...c,
        [side]: value,
        [side === "left" ? "leftBody" : "rightBody"]: body,
      });
    } catch (e) {
      setError(e instanceof Error ? e.message : "Load failed");
    } finally {
      setBusy(false);
    }
  }

  async function restoreVersion(c: Compare, version: number) {
    if (!editable || !version) return;
    setBusy(true);
    setError("");
    try {
      await gql(
        `mutation ($id: ID!, $name: String!, $v: Int!) {
          swapFileVersion(projectId: $id, fileName: $name, version: $v) { name uploadedAt }
        }`,
        { id: c.projectId, name: c.fileName, v: version }
      );
      setCompare(null);
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Restore failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="page">
      {workspaces.map((ws) => (
        <div key={ws.id}>
          {editable && (
            <section className="panel">
              <h2>quick project</h2>
              <p className="feed-lead">
                your pipeline failed. let&apos;s find out why. the sample project is loaded so you can start an investigation immediately.
              </p>
              {me?.orgRole === "viewer" && (
                <p className="hint">viewer access: inspect files and history only.</p>
              )}
              {error && <p className="error">{error}</p>}
              <form className="stack-form" onSubmit={(e) => createProject(e, ws.id)}>
                <label>
                  project name
                  <input
                    value={newName}
                    onChange={(e) => setNewName(e.target.value)}
                    placeholder="new project name"
                    required
                  />
                </label>
                <div className="row-actions">
                  <button type="submit" disabled={busy}>
                    create project
                  </button>
                  <button type="button" disabled={busy} onClick={() => seedSample(ws.id)}>
                    load sample pipeline
                  </button>
                </div>
              </form>
            </section>
          )}
          {!editable && error && (
            <section className="panel">
              <p className="error">{error}</p>
            </section>
          )}
          <section className="panel">
            <div className="feed-head">
              <h2>{ws.name}</h2>
              <p className="feed-lead">projects, files, and investigation history for this workspace.</p>
            </div>
            {ws.projects.length === 0 && (
              <div className="empty-state">
                <p className="empty-title">no projects yet</p>
                <p className="empty-text">create one or load the sample pipeline to start an investigation.</p>
              </div>
            )}
            <div className="project-list">
              {ws.projects.map((p) => (
                <article key={p.id} className="project-card">
                  <div className="project-header">
                    <h3 className="project-title">{p.name}</h3>
                    <div className="project-badges">
                      {p.isSample && <span className="badge">sample</span>}
                      <span className="badge">{p.files.length} files</span>
                      <span className="badge">{p.investigations?.length || 0} runs</span>
                    </div>
                  </div>
                  <dl className="project-meta">
                    <div>
                      <dt>files</dt>
                      <dd>{p.files.length ? p.files.map((f) => f.name).join(", ") : "none yet"}</dd>
                    </div>
                    <div>
                      <dt>latest run</dt>
                      <dd>
                        {p.investigations?.[0]
                          ? `${p.investigations[0].fastResult?.incidentType || p.investigations[0].status} · ${p.investigations[0].createdAt.slice(0, 16).replace("T", " ")}`
                          : "not started"}
                      </dd>
                    </div>
                  </dl>
                  {!!p.files.length && (
                    <ul className="files">
                      {p.files.map((f) => (
                        <li key={f.name}>
                          <span>{f.name}</span>
                          <button
                            type="button"
                            className="linkish"
                            disabled={busy}
                            onClick={() => void openCompare(p.id, f.name)}
                          >
                            compare versions
                          </button>
                        </li>
                      ))}
                    </ul>
                  )}
                  {editable && (
                    <label className="upload">
                      upload files
                      <input
                        type="file"
                        multiple
                        disabled={busy}
                        accept=".log,.yaml,.yml,.py,.json,.jsonl,.csv,.txt"
                        onChange={(e) => {
                          void onUpload(p.id, e.target.files);
                          e.target.value = "";
                        }}
                      />
                    </label>
                  )}
                  <div className="project-actions">
                    {editable && (
                      <button disabled={busy} onClick={() => startInv(p.id)}>
                        start investigation
                      </button>
                    )}
                    {p.investigations?.[0] && (
                      <button type="button" onClick={() => nav(`/investigation/${p.investigations![0].id}`)}>
                        open latest run
                      </button>
                    )}
                  </div>
                  {!!p.investigations?.length && (
                    <details className="archive-block">
                      <summary>history · {p.investigations.length}</summary>
                      <ul className="history">
                        {p.investigations.map((inv) => (
                          <li key={inv.id}>
                            <button className="linkish" type="button" onClick={() => nav(`/investigation/${inv.id}`)}>
                              {inv.fastResult?.incidentType || inv.status} · {inv.createdAt.slice(0, 16).replace("T", " ")}
                            </button>
                          </li>
                        ))}
                      </ul>
                    </details>
                  )}
                </article>
              ))}
            </div>
          </section>
        </div>
      ))}
      {workspaces.length === 0 && (
        <section className="panel">
          <h2>investigate</h2>
          {error ? <p className="error">{error}</p> : <p className="feed-lead">loading workspace…</p>}
        </section>
      )}

      {compare && (
        <div className="modal-backdrop" role="dialog" aria-modal="true">
          <div className="panel modal-card">
            <div className="row bar">
              <h2>
                {compare.fileName} — versions
              </h2>
              <button type="button" onClick={() => setCompare(null)}>
                close
              </button>
            </div>
            <p className="feed-lead">active file vs a stored snapshot. restore writes the chosen snapshot back as current.</p>
            <div className="compare-grid">
              <div>
                <label>
                  left
                  <select
                    value={compare.left === "current" ? "current" : String(compare.left)}
                    onChange={(e) => {
                      const v = e.target.value === "current" ? "current" : Number(e.target.value);
                      void loadSide(compare, "left", v);
                    }}
                  >
                    <option value="current">current (active)</option>
                    {compare.versions.map((v) => (
                      <option key={`l-${v.version}`} value={v.version}>
                        snapshot v{v.version}
                      </option>
                    ))}
                  </select>
                </label>
                <pre className="file-body">{compare.leftBody || "(empty)"}</pre>
              </div>
              <div>
                <label>
                  right
                  <select
                    value={compare.right || ""}
                    onChange={(e) => {
                      const n = Number(e.target.value);
                      if (Number.isNaN(n)) return;
                      void loadSide(compare, "right", n);
                    }}
                  >
                    {compare.versions.map((v) => (
                      <option key={`r-${v.version}`} value={v.version}>
                        snapshot v{v.version} · {v.uploadedAt.slice(0, 16).replace("T", " ")}
                      </option>
                    ))}
                  </select>
                </label>
                <pre className="file-body">{compare.rightBody || "(no snapshot)"}</pre>
              </div>
            </div>
            {editable && compare.right > 0 && (
              <button disabled={busy} type="button" onClick={() => void restoreVersion(compare, compare.right)}>
                restore right snapshot as current
              </button>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
