import { FormEvent, useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import {
  EVAL_FIELDS,
  Entitlements,
  Evaluation,
  GEN_FIELDS,
  Generation,
  GitBlameLine,
  GitCommit,
  GitRepo,
  gql,
  Project,
} from "../api";
import { ConfBar, EmptyState, formatWhen, isProFeatureError, UpgradeBanner } from "../ui";
import { useI18n } from "../i18n";

const GIT_FIELDS = `url branch head connected`;

export default function LoopPage() {
  const { t, locale } = useI18n();
  const { projectId } = useParams();
  const nav = useNavigate();
  const [project, setProject] = useState<Project | null>(null);
  const [ent, setEnt] = useState<Entitlements | null>(null);
  const [gens, setGens] = useState<Generation[]>([]);
  const [evals, setEvals] = useState<Evaluation[]>([]);
  const [git, setGit] = useState<GitRepo | null>(null);
  const [commits, setCommits] = useState<GitCommit[]>([]);
  const [blame, setBlame] = useState<GitBlameLine[]>([]);
  const [diff, setDiff] = useState("");
  const [gitUrl, setGitUrl] = useState("");
  const [branch, setBranch] = useState("main");
  const [blamePath, setBlamePath] = useState("pipeline.py");
  const [prompt, setPrompt] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const latestInv = project?.investigations?.[0];

  async function load() {
    if (!projectId) return;
    const data = await gql<{
      project: Project;
      entitlements: Entitlements;
      generations: Generation[];
      evaluations: Evaluation[];
      gitRepo: GitRepo;
    }>(
      `query ($id: ID!) {
        entitlements {
          tier maxProjects maxInvestigationsPerMonth investigationsUsed
          deepAnalysis council generate evaluate gitMcp modelSelection
          teamWorkspace billingConfigured ssoEnabled demoUpgrade
        }
        project(id: $id) { id workspaceId name isSample files { name mimeType uploadedAt } investigations { id } }
        generations(projectId: $id) { ${GEN_FIELDS} }
        evaluations(projectId: $id) { ${EVAL_FIELDS} }
        gitRepo(projectId: $id) { ${GIT_FIELDS} }
      }`,
      { id: projectId }
    );
    setProject(data.project);
    setEnt(data.entitlements);
    setGens(data.generations || []);
    setEvals(data.evaluations || []);
    setGit(data.gitRepo);
  }

  useEffect(() => {
    load().catch((e) => setError(e.message));
  }, [projectId]);

  async function generate(e: FormEvent) {
    e.preventDefault();
    if (!projectId) return;
    setBusy(true);
    setError("");
    try {
      await gql(
        `mutation ($id: ID!, $prompt: String, $investigationId: ID) {
          generateDataset(projectId: $id, prompt: $prompt, investigationId: $investigationId) { ${GEN_FIELDS} }
        }`,
        { id: projectId, prompt: prompt || null, investigationId: latestInv?.id || null }
      );
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : t("genFailed"));
    } finally {
      setBusy(false);
    }
  }

  async function evaluate(generationId?: string) {
    if (!projectId) return;
    setBusy(true);
    setError("");
    try {
      await gql(
        `mutation ($id: ID!, $generationId: ID, $investigationId: ID) {
          evaluateFix(projectId: $id, generationId: $generationId, investigationId: $investigationId) { ${EVAL_FIELDS} }
        }`,
        { id: projectId, generationId: generationId || null, investigationId: latestInv?.id || null }
      );
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : t("evalFailed"));
    } finally {
      setBusy(false);
    }
  }

  async function connectGit(e: FormEvent) {
    e.preventDefault();
    if (!projectId) return;
    setBusy(true);
    setError("");
    try {
      const data = await gql<{ connectGitRepo: GitRepo }>(
        `mutation ($id: ID!, $url: String!, $branch: String) {
          connectGitRepo(projectId: $id, url: $url, branch: $branch) { ${GIT_FIELDS} }
        }`,
        { id: projectId, url: gitUrl, branch }
      );
      setGit(data.connectGitRepo);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : t("gitFailed"));
    } finally {
      setBusy(false);
    }
  }

  async function loadGitExtras() {
    if (!projectId || !git?.connected) return;
    setBusy(true);
    setError("");
    try {
      const data = await gql<{ gitLog: GitCommit[]; gitBlame: GitBlameLine[]; gitDiff: string }>(
        `query ($id: ID!, $path: String!) {
          gitLog(projectId: $id, limit: 10) { sha author date message }
          gitBlame(projectId: $id, path: $path) { line sha author summary }
          gitDiff(projectId: $id)
        }`,
        { id: projectId, path: blamePath }
      );
      setCommits(data.gitLog || []);
      setBlame(data.gitBlame || []);
      setDiff(data.gitDiff || "");
    } catch (err) {
      setError(err instanceof Error ? err.message : t("gitInspectFailed"));
    } finally {
      setBusy(false);
    }
  }

  const lockedGenerate = ent && !ent.generate;
  const lockedGit = ent && !ent.gitMcp;

  return (
    <div className="page">
      <section className="panel">
        <div className="feed-head">
          <h2>{t("project")}</h2>
          <p className="feed-lead">{project ? project.name : t("loading")}</p>
        </div>
        <button type="button" className="linkish" onClick={() => nav("/")}>
          {t("backProjects")}
        </button>
        {error && !isProFeatureError(error) && <p className="error">{error}</p>}
        {isProFeatureError(error) && (
          <UpgradeBanner
            title={t("proFeature")}
            text={error}
            demo={!!ent?.demoUpgrade}
          />
        )}
      </section>

      <section className="panel">
        <h2>{t("genTitle")}</h2>
        <p className="feed-lead">{t("genLead")}</p>
        {lockedGenerate ? (
          <UpgradeBanner
            title={t("genLockedTitle")}
            text={t("genLockedText")}
            demo={!!ent?.demoUpgrade}
          />
        ) : (
          <form className="stack-form" onSubmit={generate}>
            <label>
              {t("prompt")}
              <input
                value={prompt}
                onChange={(e) => setPrompt(e.target.value)}
                placeholder={t("genPlaceholder")}
              />
            </label>
            <div className="row-actions">
              <button className="primary" disabled={busy} type="submit">
                {busy ? t("working") : t("generateDataset")}
              </button>
              {latestInv && (
                <span className="muted">{t("usesLatest")}</span>
              )}
            </div>
          </form>
        )}
        {gens.length === 0 ? (
          <EmptyState title={t("noGens")} text={t("noGensText")} />
        ) : (
          <ul className="history">
            {gens.map((g) => (
              <li key={g.id}>
                <strong>{g.fileName}</strong> · {g.rowCount} rows · {Math.round(g.confidence * 100)}% ·{" "}
                {formatWhen(g.createdAt, locale)}
                <p className="muted">{g.schemaNote}</p>
                {ent?.evaluate && (
                  <button type="button" className="linkish" disabled={busy} onClick={() => void evaluate(g.id)}>
                    {t("evalThis")}
                  </button>
                )}
              </li>
            ))}
          </ul>
        )}
      </section>

      <section className="panel">
        <h2>{t("evalTitle")}</h2>
        <p className="feed-lead">{t("evalLead")}</p>
        {ent && !ent.evaluate ? (
          <UpgradeBanner
            title={t("evalLockedTitle")}
            text={t("evalLockedText")}
            demo={!!ent.demoUpgrade}
          />
        ) : (
          <button type="button" disabled={busy} onClick={() => void evaluate(gens[0]?.id)}>
            {t("evalLatest")}
          </button>
        )}
        {evals.map((ev) => (
          <article key={ev.id} className="project-card" style={{ marginTop: 16 }}>
            <div className="project-header">
              <h3 className="project-title">{ev.recommendation}</h3>
              <span className="badge ok">{Math.round(ev.confidence * 100)}%</span>
            </div>
            <p>{ev.summary}</p>
            <ConfBar value={ev.confidence} />
            <table className="data-table">
              <thead>
                <tr>
                  <th>{t("metric")}</th>
                  <th>{t("before")}</th>
                  <th>{t("after")}</th>
                  <th>Δ</th>
                </tr>
              </thead>
              <tbody>
                {ev.metrics.map((m) => (
                  <tr key={m.name}>
                    <td>{m.name}</td>
                    <td>{m.before}</td>
                    <td>{m.after}</td>
                    <td>{m.delta > 0 ? `+${m.delta}` : m.delta}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </article>
        ))}
      </section>

      <section className="panel">
        <h2>{t("gitTitle")}</h2>
        <p className="feed-lead">{t("gitLead")}</p>
        {lockedGit ? (
          <UpgradeBanner
            title={t("gitLockedTitle")}
            text={t("gitLockedText")}
            demo={!!ent?.demoUpgrade}
          />
        ) : (
          <>
            {git?.connected ? (
              <p className="muted">
                {t("connected", { url: git.url, branch: git.branch, head: git.head.slice(0, 8) })}
              </p>
            ) : (
              <form className="stack-form" onSubmit={connectGit}>
                <label>
                  {t("gitUrl")}
                  <input
                    value={gitUrl}
                    onChange={(e) => setGitUrl(e.target.value)}
                    placeholder="https://github.com/org/repo.git"
                    required
                  />
                </label>
                <label>
                  {t("branch")}
                  <input value={branch} onChange={(e) => setBranch(e.target.value)} />
                </label>
                <button className="primary" disabled={busy} type="submit">
                  {t("clone")}
                </button>
              </form>
            )}
            {git?.connected && (
              <div className="stack-form" style={{ marginTop: 16 }}>
                <label>
                  {t("blamePath")}
                  <input value={blamePath} onChange={(e) => setBlamePath(e.target.value)} />
                </label>
                <button type="button" disabled={busy} onClick={() => void loadGitExtras()}>
                  {t("loadGit")}
                </button>
                {!!commits.length && (
                  <ul className="history">
                    {commits.map((c) => (
                      <li key={c.sha}>
                        {c.sha.slice(0, 8)} · {c.author} · {c.message}
                      </li>
                    ))}
                  </ul>
                )}
                {!!blame.length && (
                  <pre className="file-body">
                    {blame
                      .slice(0, 40)
                      .map((l) => `${l.line}\t${l.author}\t${l.summary}`)
                      .join("\n")}
                  </pre>
                )}
                {diff && <pre className="file-body">{diff}</pre>}
              </div>
            )}
          </>
        )}
      </section>
    </div>
  );
}
