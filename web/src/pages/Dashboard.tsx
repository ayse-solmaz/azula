import { FormEvent, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { FineTuneJob, gql, LLMOpsMetrics, ModelConfig, Workspace } from "../api";
import { useI18n } from "../i18n";
import { EmptyState, Tabs, formatWhen, statusTone } from "../ui";

const METRICS_FIELDS = `
  totalInvestigations completed failed avgConfidence avgDurationSec
  workerSlots busySlots modelAName modelBName
  ollamaReachable ollamaModels incidentModelReady adapterOnDisk topCauses
`;

const CFG_FIELDS = `
  workspaceId modelAProvider modelAName modelBProvider modelBName
  modelCProvider modelCName
  temperature maxTokens investigatorPrompt challengerPrompt judgePrompt activeSlot
`;

function jobActive(status: string) {
  return ["queued", "running", "training", "merging"].includes(status);
}

export default function DashboardPage() {
  const { t, locale } = useI18n();
  const [wsId, setWsId] = useState("");
  const [cfg, setCfg] = useState<ModelConfig | null>(null);
  const [metrics, setMetrics] = useState<LLMOpsMetrics | null>(null);
  const [jobs, setJobs] = useState<FineTuneJob[]>([]);
  const [error, setError] = useState("");
  const [saved, setSaved] = useState("");
  const [viewer, setViewer] = useState(false);
  const [loading, setLoading] = useState(true);
  const [pageTab, setPageTab] = useState("basic");
  const [promptTab, setPromptTab] = useState("investigator");

  async function loadJobs(id: string) {
    const data = await gql<{ fineTuneJobs: FineTuneJob[] }>(
      `query ($id: ID!) {
        fineTuneJobs(workspaceId: $id) { id workspaceId status adapterPath error createdAt }
      }`,
      { id }
    );
    setJobs(data.fineTuneJobs);
    return data.fineTuneJobs;
  }

  async function loadMetrics(id: string) {
    const rest = await gql<{ modelConfig: ModelConfig; llmOpsMetrics: LLMOpsMetrics }>(
      `query ($id: ID!) {
        modelConfig(workspaceId: $id) { ${CFG_FIELDS} }
        llmOpsMetrics(workspaceId: $id) { ${METRICS_FIELDS} }
      }`,
      { id }
    );
    setCfg(rest.modelConfig);
    setMetrics(rest.llmOpsMetrics);
  }

  useEffect(() => {
    (async () => {
      const data = await gql<{ me: { orgRole?: string | null }; workspaces: Workspace[] }>(
        `query { me { orgRole } workspaces { id name } }`
      );
      setViewer(data.me.orgRole === "viewer" || data.me.orgRole === "engineer");
      const id = data.workspaces[0]?.id;
      if (!id) {
        setLoading(false);
        return;
      }
      setWsId(id);
      await loadMetrics(id);
      await loadJobs(id);
      setLoading(false);
    })().catch((e) => {
      setError(e.message);
      setLoading(false);
    });
  }, []);

  useEffect(() => {
    if (!wsId) return;
    const tick = window.setInterval(() => {
      loadJobs(wsId)
        .then((list) => {
          if (!list.some((j) => jobActive(j.status))) return;
          return loadMetrics(wsId);
        })
        .catch((e) => setError(e instanceof Error ? e.message : "Failed"));
    }, 2500);
    return () => window.clearInterval(tick);
  }, [wsId]);

  async function saveConfig(next: ModelConfig) {
    setError("");
    setSaved("");
    const data = await gql<{ updateModelConfig: ModelConfig }>(
      `mutation ($input: ModelConfigInput!) {
        updateModelConfig(input: $input) { ${CFG_FIELDS} }
      }`,
      {
        input: {
          workspaceId: wsId,
          modelAProvider: next.modelAProvider,
          modelAName: next.modelAName,
          modelBProvider: next.modelBProvider,
          modelBName: next.modelBName,
          modelCProvider: next.modelCProvider,
          modelCName: next.modelCName,
          temperature: next.temperature,
          maxTokens: next.maxTokens,
          investigatorPrompt: next.investigatorPrompt,
          challengerPrompt: next.challengerPrompt,
          judgePrompt: next.judgePrompt,
          activeSlot: next.activeSlot,
        },
      }
    );
    setCfg(data.updateModelConfig);
    setSaved(t("savedConfig"));
  }

  async function onSave(e: FormEvent) {
    e.preventDefault();
    if (!cfg) return;
    try {
      await saveConfig(cfg);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed");
    }
  }

  async function attachB() {
    setError("");
    try {
      const data = await gql<{ attachIncidentModel: ModelConfig }>(
        `mutation ($id: ID!) { attachIncidentModel(workspaceId: $id) { ${CFG_FIELDS} } }`,
        { id: wsId }
      );
      setCfg(data.attachIncidentModel);
      await loadMetrics(wsId);
      setSaved(t("savedAttach"));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed");
    }
  }

  if (loading) return <p className="page muted">{t("loadingModels")}</p>;
  if (!cfg) {
    return (
      <div className="page">
        <section className="panel">
          <h2>{t("thinkTitle")}</h2>
          <p className="feed-lead">{t("thinkLead")}</p>
          {error ? <p className="error">{error}</p> : (
            <EmptyState
              title={t("noWorkspace")}
              text={t("noWorkspaceText")}
              action={
                <Link className="primary" to="/">
                  {t("goStartInv")}
                </Link>
              }
            />
          )}
        </section>
      </div>
    );
  }

  const ollamaNames = metrics?.ollamaModels ?? [];
  const systemOk = Boolean(metrics?.ollamaReachable);
  const fastName = metrics?.modelAName || cfg.modelAName;
  const deepName = metrics?.modelBName || cfg.modelBName;

  return (
    <div className="page">
      <section className="panel think-hero">
        <p className="eyebrow">{t("navThink")}</p>
        <h2>{t("thinkTitle")}</h2>
        <p className="feed-lead">{t("thinkLead")}</p>
        <div className="role-grid">
          <article className="role-card">
            <h3>{t("cardFast")}</h3>
            <p>{t("cardFastBody")}</p>
            <p className="think-used">{t("thinkFastUses", { name: fastName || "—" })}</p>
          </article>
          <article className="role-card">
            <h3>{t("cardDeep")}</h3>
            <p>{t("cardDeepBody")}</p>
            <p className="think-used">{t("thinkDeepUses", { name: deepName || "—" })}</p>
          </article>
          <article className="role-card">
            <h3>{t("cardCouncil")}</h3>
            <p>{t("cardCouncilBody")}</p>
          </article>
        </div>
      </section>

      {metrics && (
        <section className={`system-banner ${systemOk ? "ok" : "warn"}`}>
          <h2>{systemOk ? t("systemReady") : t("systemNotReady")}</h2>
          <p>{systemOk ? t("systemReadyLead") : t("systemNotReadyLead")}</p>
          <p className="hint">{systemOk ? t("ollamaOk") : t("ollamaOff")}</p>
        </section>
      )}

      <Tabs
        tabs={[
          { id: "basic", label: t("basic") },
          { id: "advanced", label: t("advanced") },
          { id: "developer", label: t("developer") },
        ]}
        active={pageTab}
        onChange={setPageTab}
      />

      {pageTab === "basic" && (
        <section className="panel">
          <h2>{t("thinkUsed")}</h2>
          <p className="feed-lead">{t("tempHelp")}</p>
          <form className="stack-form" onSubmit={onSave}>
            <label>
              {t("temperature")}
              <input
                type="number"
                step="0.1"
                min="0"
                max="2"
                value={cfg.temperature}
                onChange={(e) => setCfg({ ...cfg, temperature: Number(e.target.value) })}
              />
            </label>
            {error && <p className="error">{error}</p>}
            {saved && <p className="ok">{saved}</p>}
            <div className="project-actions">
              <button className="primary" type="submit" disabled={viewer}>
                {viewer ? t("saveViewer") : t("saveTemp")}
              </button>
              <Link className="primary" to="/">
                {t("goStartInv")}
              </Link>
            </div>
          </form>
        </section>
      )}

      {pageTab === "advanced" && (
        <form className="panel form-grid" onSubmit={onSave}>
          <h2 className="wide">{t("optionalOllama")}</h2>
          <p className="feed-lead wide">{t("advancedOnly")}</p>
          <label>
            {t("fastModel")}
            <select
              value={cfg.modelAName}
              onChange={(e) => setCfg({ ...cfg, modelAName: e.target.value, modelAProvider: cfg.modelAProvider || "ollama" })}
            >
              {Array.from(new Set([cfg.modelAName, ...ollamaNames, "qwen2.5:1.5b"])).map((n) => (
                <option key={n} value={n}>{n}</option>
              ))}
            </select>
          </label>
          <label>
            {t("deepModel")}
            <select
              value={cfg.modelBName}
              onChange={(e) => setCfg({ ...cfg, modelBName: e.target.value, modelBProvider: cfg.modelBProvider || "ollama" })}
            >
              {Array.from(new Set([cfg.modelBName, ...ollamaNames, "azula-incident", "mistral:latest"])).map((n) => (
                <option key={n} value={n}>{n}</option>
              ))}
            </select>
          </label>
          <p className="hint wide">{t("modelsHint")}</p>
          <label>
            {t("modelAProvider")}
            <input value={cfg.modelAProvider} onChange={(e) => setCfg({ ...cfg, modelAProvider: e.target.value })} />
          </label>
          <label>
            {t("modelBProvider")}
            <input value={cfg.modelBProvider} onChange={(e) => setCfg({ ...cfg, modelBProvider: e.target.value })} />
          </label>
          <label>
            {t("modelCProvider")}
            <input
              value={cfg.modelCProvider || "openai"}
              onChange={(e) => setCfg({ ...cfg, modelCProvider: e.target.value })}
            />
          </label>
          <label>
            {t("modelCName")}
            <input
              value={cfg.modelCName || "gpt-4o-mini"}
              onChange={(e) => setCfg({ ...cfg, modelCName: e.target.value })}
            />
          </label>
          <label>
            {t("maxTokens")}
            <input
              type="number"
              value={cfg.maxTokens}
              onChange={(e) => setCfg({ ...cfg, maxTokens: Number(e.target.value) })}
            />
          </label>
          <label>
            {t("activeSlot")}
            <select value={cfg.activeSlot} onChange={(e) => setCfg({ ...cfg, activeSlot: e.target.value })}>
              <option value="A">{t("slotA")}</option>
              <option value="B">{t("slotB")}</option>
            </select>
          </label>
          <div className="wide">
            <Tabs
              tabs={[
                { id: "investigator", label: t("roleInvestigator") },
                { id: "challenger", label: t("roleChallenger") },
                { id: "judge", label: t("roleJudge") },
              ]}
              active={promptTab}
              onChange={setPromptTab}
            />
            {promptTab === "investigator" && (
              <label>
                {t("investigatorPrompt")}
                <textarea rows={8} value={cfg.investigatorPrompt} onChange={(e) => setCfg({ ...cfg, investigatorPrompt: e.target.value })} />
              </label>
            )}
            {promptTab === "challenger" && (
              <label>
                {t("challengerPrompt")}
                <textarea rows={8} value={cfg.challengerPrompt} onChange={(e) => setCfg({ ...cfg, challengerPrompt: e.target.value })} />
              </label>
            )}
            {promptTab === "judge" && (
              <label>
                {t("judgePrompt")}
                <textarea rows={8} value={cfg.judgePrompt} onChange={(e) => setCfg({ ...cfg, judgePrompt: e.target.value })} />
              </label>
            )}
          </div>
          {error && <p className="error wide">{error}</p>}
          {saved && <p className="ok wide">{saved}</p>}
          <div className="wide project-actions">
            <button className="primary" type="submit" disabled={viewer}>
              {viewer ? t("saveViewer") : t("saveConfig")}
            </button>
            <button type="button" disabled={viewer} onClick={attachB}>
              {t("attachB")}
            </button>
          </div>
        </form>
      )}

      {pageTab === "developer" && (
        <section className="panel">
          <h2>{t("finetuneTitle")}</h2>
          <p className="feed-lead">{t("developerOnly")}</p>
          <p className="feed-lead">{t("finetuneLead")}</p>
          <div className="project-actions">
            <button
              type="button"
              className="primary"
              disabled={!wsId || viewer}
              onClick={async () => {
                setError("");
                try {
                  const data = await gql<{ startFineTuneJob: FineTuneJob }>(
                    `mutation ($id: ID!) { startFineTuneJob(workspaceId: $id) { id status adapterPath error createdAt } }`,
                    { id: wsId }
                  );
                  setSaved(`Fine-tune job ${data.startFineTuneJob.id} → ${data.startFineTuneJob.status}`);
                  await loadJobs(wsId);
                  await loadMetrics(wsId);
                } catch (err) {
                  setError(err instanceof Error ? err.message : "Failed");
                }
              }}
            >
              {t("startFinetune")}
            </button>
          </div>
          {error && <p className="error">{error}</p>}
          {saved && <p className="ok">{saved}</p>}
          {jobs.length === 0 ? (
            <div style={{ marginTop: 16 }}>
              <EmptyState
                title={t("noJobs")}
                text={t("noJobsText")}
                action={
                  <button type="button" className="primary" disabled={!wsId || viewer} onClick={attachB}>
                    {t("attachIncident")}
                  </button>
                }
              />
            </div>
          ) : (
            <div className="project-list" style={{ marginTop: 16 }}>
              {jobs.map((job) => (
                <article key={job.id} className="project-card">
                  <div className="project-header">
                    <h3 className="project-title">{job.id.slice(0, 8)}…</h3>
                    <span className={`badge ${statusTone(job.status)}`}>{job.status}</span>
                  </div>
                  <dl className="project-meta">
                    <div>
                      <dt>{t("adapter")}</dt>
                      <dd>{job.adapterPath || "—"}</dd>
                    </div>
                    <div>
                      <dt>{t("created")}</dt>
                      <dd>{formatWhen(job.createdAt, locale)}</dd>
                    </div>
                  </dl>
                  {job.error && <p className="error">{job.error}</p>}
                </article>
              ))}
            </div>
          )}
        </section>
      )}
    </div>
  );
}
