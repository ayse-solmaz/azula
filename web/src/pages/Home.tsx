import { FormEvent, useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { Entitlements, gql, INV_FIELDS, Investigation, Project, uploadProjectFile, User, Workspace } from "../api";
import { useI18n } from "../i18n";
import { canEdit, EmptyState, FileDropzone, FREE_TIER_MAX_PROJECTS, fileKind, formatWhen, HowTo, isFreeTier, isProFeatureError, isRunningStatus, isTierLimitError, prettyStatus, statusTone, UpgradeBanner } from "../ui";

const WORKSPACES_QUERY = `query {
  me { id email orgRole tier }
  entitlements { demoUpgrade generate evaluate gitMcp maxInvestigationsPerMonth investigationsUsed }
  workspaces {
    id name
    projects {
      id workspaceId name isSample
      files { name mimeType uploadedAt }
      investigations { id status createdAt executionMode fastResult { incidentType confidence } }
    }
  }
}`;

function runsOf(p: Project): Investigation[] {
  return Array.isArray(p.investigations) ? p.investigations : [];
}

function normalizeSpaces(workspaces: Workspace[] | null | undefined): Workspace[] {
  return (workspaces || []).map((ws) => ({
    ...ws,
    projects: (ws.projects || []).map((p) => ({ ...p, investigations: runsOf(p) })),
  }));
}

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

function formatLoadError(e: unknown, apiDown: string, failed: string) {
  const msg = e instanceof Error ? e.message : String(e);
  if (/failed to fetch|cannot reach api|networkerror|load failed|econnrefused/i.test(msg)) {
    return apiDown;
  }
  return msg || failed;
}

const RECENT_PROJECTS = 4;

function projectRecency(p: Project) {
  const latest = runsOf(p)[0]?.createdAt || "";
  const file = p.files[0]?.uploadedAt || "";
  return latest > file ? latest : file;
}

function sortProjects(projects: Project[]) {
  return [...projects].sort((a, b) => {
    if (a.isSample !== b.isSample) return a.isSample ? -1 : 1;
    return projectRecency(b).localeCompare(projectRecency(a));
  });
}

export default function HomePage() {
  const { t, locale } = useI18n();
  const nav = useNavigate();
  const [me, setMe] = useState<User | null>(null);
  const [workspaces, setWorkspaces] = useState<Workspace[]>([]);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [newName, setNewName] = useState("");
  const [compare, setCompare] = useState<Compare | null>(null);
  const [ent, setEnt] = useState<Entitlements | null>(null);
  const [notice, setNotice] = useState("");

  const editable = canEdit(me?.orgRole);
  const bootstrapped = useRef(false);

  async function load() {
    const data = await gql<{ me: User; workspaces: Workspace[]; entitlements?: Entitlements }>(WORKSPACES_QUERY);
    if (!data.me) throw new Error(t("failed"));
    setMe(data.me);
    if (data.entitlements) setEnt(data.entitlements);
    let spaces = normalizeSpaces(data.workspaces);
    if (!spaces.length) {
      await gql(`mutation { createWorkspace(name: "My ML Lab") { id name } }`);
      const again = await gql<{ workspaces: Workspace[] }>(WORKSPACES_QUERY);
      spaces = normalizeSpaces(again.workspaces);
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
        setWorkspaces(normalizeSpaces(seeded.workspaces));
      } catch (e) {
        bootstrapped.current = false;
        throw e;
      }
    }
  }

  useEffect(() => {
    load().catch((e) => setError(formatLoadError(e, t("apiDown"), t("failed"))));
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
      setNotice(t("sampleReady"));
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
      setNotice(t("projectCreated"));
    } catch (err) {
      const msg = err instanceof Error ? err.message : "Failed";
      setError(msg);
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

  const projectCount = workspaces.reduce((n, ws) => n + (ws.projects?.length || 0), 0);
  const atLimit = isFreeTier(me?.tier) && projectCount >= FREE_TIER_MAX_PROJECTS;
  const limitError = error && isTierLimitError(error);

  function renderTile(p: Project) {
    const runs = runsOf(p);
    const latest = runs[0];
    const running = latest ? isRunningStatus(latest.status) : false;
    const complete = (latest?.status || "").toUpperCase() === "COMPLETED";
    const failed = (latest?.status || "").toUpperCase() === "FAILED";
    const noFiles = !p.files.length && !p.isSample;
    const latestLabel = latest ? prettyStatus(latest.status, t) : t("notStarted");
    return (
      <article key={p.id} className="project-card compact">
        <div className="project-header">
          <h3 className="project-title">{p.name}</h3>
          <div className="project-badges">
            {p.isSample && <span className="badge accent">{t("sampleBadge")}</span>}
            <span className="badge">{t("filesCount", { n: p.files.length })}</span>
          </div>
        </div>
        <p className="project-status">
          <span className={`badge ${statusTone(latest ? latest.status : "not started")}`}>{latestLabel}</span>
          {latest ? ` · ${formatWhen(latest.createdAt, locale)}` : ""}
        </p>
        <div className="project-actions">
          {running && latest && (
            <button type="button" className="primary" onClick={() => nav(`/investigation/${latest.id}`)}>
              {t("seeProgress")}
            </button>
          )}
          {complete && latest && (
            <button type="button" className="primary" onClick={() => nav(`/investigation/${latest.id}`)}>
              {t("seeResults")}
            </button>
          )}
          {complete && (
            <button type="button" onClick={() => nav(`/loop/${p.id}`)}>
              {t("afterRunGenerate")}
            </button>
          )}
          {editable && !running && (
            <button
              className={complete || failed ? undefined : "primary"}
              disabled={busy || noFiles}
              onClick={() => startInv(p.id)}
            >
              {complete || failed ? t("startAnother") : t("startInvestigation")}
            </button>
          )}
          {failed && latest && (
            <button type="button" onClick={() => nav(`/investigation/${latest.id}`)}>
              {t("openLatest")}
            </button>
          )}
        </div>
        {noFiles && editable && <p className="disabled-why">{t("noFilesHint")}</p>}
        <details className="archive-block">
          <summary>{t("projectMore")}</summary>
          {!!p.files.length && (
            <table className="data-table">
              <thead>
                <tr>
                  <th>{t("file")}</th>
                  <th>{t("type")}</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {p.files.map((f) => (
                  <tr key={f.name}>
                    <td>{f.name}</td>
                    <td className="file-kind">{fileKind(f.name)}</td>
                    <td>
                      <button type="button" className="linkish" disabled={busy} onClick={() => void openCompare(p.id, f.name)}>
                        {t("compare")}
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
          {editable && <FileDropzone disabled={busy} onFiles={(files) => void onUpload(p.id, files)} />}
          {!complete && (
            <button type="button" disabled={busy} onClick={() => nav(`/loop/${p.id}`)}>
              {t("afterRunGenerate")}
            </button>
          )}
          {!!runs.length && (
            <ul className="history">
              {runs.map((inv) => (
                <li key={inv.id}>
                  <button className="linkish" type="button" onClick={() => nav(`/investigation/${inv.id}`)}>
                    {prettyStatus(inv.status, t)} · {formatWhen(inv.createdAt, locale)}
                  </button>
                </li>
              ))}
            </ul>
          )}
        </details>
      </article>
    );
  }

  return (
    <div className="page">
      <HowTo
        title={t("homeHowTitle")}
        steps={[
          { n: "1", title: t("homeHow1t"), body: t("homeHow1b") },
          { n: "2", title: t("homeHow2t"), body: t("homeHow2b") },
          { n: "3", title: t("homeHow3t"), body: t("homeHow3b") },
        ]}
      />
      {workspaces.map((ws) => (
        <div key={ws.id}>
          {!editable && error && (
            <section className="panel">
              <p className="error">{error}</p>
            </section>
          )}
          <section className="panel">
            <div className="feed-head">
              <h2>{ws.name}</h2>
              <p className="feed-lead">{t("workspaceLead")}</p>
            </div>
            {notice && (
              <div className="notice">
                <p>{notice}</p>
              </div>
            )}
            {error && !limitError && !isProFeatureError(error) && <p className="error">{error}</p>}
            {isProFeatureError(error) && (
              <UpgradeBanner title={t("proRequired")} text={error} demo={!!ent?.demoUpgrade} />
            )}
            {editable && !ws.projects.some((p) => p.isSample) && ws.projects.length > 0 && (
              <div className="row-actions">
                <button type="button" className="primary" disabled={busy} onClick={() => void seedSample(ws.id)}>
                  {t("trySample")}
                </button>
              </div>
            )}
            {ws.projects.length === 0 && (
              <EmptyState
                title={t("noProjects")}
                text={t("noProjectsText")}
                action={
                  editable ? (
                    <button type="button" className="primary" disabled={busy} onClick={() => void seedSample(ws.id)}>
                      {t("trySample")}
                    </button>
                  ) : undefined
                }
              />
            )}
            {(() => {
              const sorted = sortProjects(ws.projects);
              const recent = sorted.slice(0, RECENT_PROJECTS);
              const older = sorted.slice(RECENT_PROJECTS);
              return (
                <>
                  <div className="project-list">{recent.map(renderTile)}</div>
                  {older.length > 0 && (
                    <details className="archive-block older-projects">
                      <summary>{t("olderProjects", { n: older.length })}</summary>
                      <div className="project-list">{older.map(renderTile)}</div>
                    </details>
                  )}
                </>
              );
            })()}
          </section>
          {editable && (
            <section className="panel">
              <h2>{t("addProject")}</h2>
              <p className="feed-lead">{t("addProjectLead")}</p>
              {me?.orgRole === "viewer" && <p className="hint">{t("viewerHint")}</p>}
              {atLimit || limitError ? (
                <UpgradeBanner
                  title={t("projectLimitTitle", { n: FREE_TIER_MAX_PROJECTS })}
                  text={t("projectLimitText")}
                  demo={!!ent?.demoUpgrade}
                />
              ) : (
                <form className="stack-form" onSubmit={(e) => createProject(e, ws.id)}>
                  <label>
                    {t("projectName")}
                    <input
                      value={newName}
                      onChange={(e) => setNewName(e.target.value)}
                      placeholder={t("projectNamePh")}
                      required
                    />
                  </label>
                  <div className="row-actions">
                    <button className="primary" type="submit" disabled={busy}>
                      {t("createProject")}
                    </button>
                    <button type="button" disabled={busy} onClick={() => seedSample(ws.id)}>
                      {t("addSampleAgain")}
                    </button>
                  </div>
                </form>
              )}
            </section>
          )}
        </div>
      ))}
      {workspaces.length === 0 && (
        <section className="panel">
          <h2>{t("navProjects")}</h2>
          {error ? <p className="error">{error}</p> : <p className="feed-lead">{t("loadingWorkspace")}</p>}
        </section>
      )}

      {compare && (
        <div className="modal-backdrop" role="dialog" aria-modal="true">
          <div className="panel modal-card">
            <div className="row bar">
              <h2>
                {t("versionsTitle", { name: compare.fileName })}
              </h2>
              <button type="button" onClick={() => setCompare(null)}>
                {t("close")}
              </button>
            </div>
            <p className="feed-lead">{t("compareLead")}</p>
            <div className="compare-grid">
              <div>
                <label>
                  {t("left")}
                  <select
                    value={compare.left === "current" ? "current" : String(compare.left)}
                    onChange={(e) => {
                      const v = e.target.value === "current" ? "current" : Number(e.target.value);
                      void loadSide(compare, "left", v);
                    }}
                  >
                    <option value="current">{t("currentActive")}</option>
                    {compare.versions.map((v) => (
                      <option key={`l-${v.version}`} value={v.version}>
                        {t("snapshotV", { v: v.version })}
                      </option>
                    ))}
                  </select>
                </label>
                <pre className="file-body">{compare.leftBody || t("empty")}</pre>
              </div>
              <div>
                <label>
                  {t("right")}
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
                        {t("snapshotV", { v: v.version })} · {v.uploadedAt.slice(0, 16).replace("T", " ")}
                      </option>
                    ))}
                  </select>
                </label>
                <pre className="file-body">{compare.rightBody || t("noSnapshot")}</pre>
              </div>
            </div>
            {editable && compare.right > 0 && (
              <button disabled={busy} type="button" onClick={() => void restoreVersion(compare, compare.right)}>
                {t("restoreRight")}
              </button>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
