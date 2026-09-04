import { FormEvent, useEffect, useState } from "react";
import { FineTuneJob, gql, LLMOpsMetrics, ModelConfig, Workspace } from "../api";

const METRICS_FIELDS = `
  totalInvestigations completed failed avgConfidence avgDurationSec
  workerSlots busySlots modelAName modelBName
  ollamaReachable ollamaModels incidentModelReady adapterOnDisk topCauses
`;

const CFG_FIELDS = `
  workspaceId modelAProvider modelAName modelBProvider modelBName
  temperature maxTokens investigatorPrompt challengerPrompt judgePrompt activeSlot
`;

function jobActive(status: string) {
  return ["queued", "running", "training", "merging"].includes(status);
}

export default function DashboardPage() {
  const [wsId, setWsId] = useState("");
  const [cfg, setCfg] = useState<ModelConfig | null>(null);
  const [metrics, setMetrics] = useState<LLMOpsMetrics | null>(null);
  const [jobs, setJobs] = useState<FineTuneJob[]>([]);
  const [error, setError] = useState("");
  const [saved, setSaved] = useState("");
  const [viewer, setViewer] = useState(false);
  const [loading, setLoading] = useState(true);

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
      setViewer(data.me.orgRole === "viewer");
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

  async function onSave(e: FormEvent) {
    e.preventDefault();
    if (!cfg) return;
    setError("");
    setSaved("");
    try {
      const data = await gql<{ updateModelConfig: ModelConfig }>(
        `mutation ($input: ModelConfigInput!) {
          updateModelConfig(input: $input) { ${CFG_FIELDS} }
        }`,
        {
          input: {
            workspaceId: wsId,
            modelAProvider: cfg.modelAProvider,
            modelAName: cfg.modelAName,
            modelBProvider: cfg.modelBProvider,
            modelBName: cfg.modelBName,
            temperature: cfg.temperature,
            maxTokens: cfg.maxTokens,
            investigatorPrompt: cfg.investigatorPrompt,
            challengerPrompt: cfg.challengerPrompt,
            judgePrompt: cfg.judgePrompt,
            activeSlot: cfg.activeSlot,
          },
        }
      );
      setCfg(data.updateModelConfig);
      setSaved("saved. next investigation uses this config.");
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
      setSaved("model b attached to azula-incident. deep analysis uses the qlora merge.");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed");
    }
  }

  if (loading) return <p className="page">loading llm dashboard…</p>;
  if (!cfg) {
    return (
      <div className="page">
        <section className="panel">
          <h2>llm dashboard</h2>
          {error ? <p className="error">{error}</p> : (
            <div className="empty-state">
              <p className="empty-title">no workspace yet</p>
              <p className="empty-text">create a project on investigate first, then configure models here.</p>
            </div>
          )}
        </section>
      </div>
    );
  }

  const ollamaNames = metrics?.ollamaModels ?? [];

  return (
    <div className="page">
      <section className="panel">
        <div className="feed-head">
          <h2>llm dashboard</h2>
          <p className="feed-lead">
            model a is fast classify. model b is deep + council investigator/challenger. attach the local qlora merge as model b.
          </p>
        </div>
        {metrics && (
          <div className="metrics">
            <div className="metric">
              <span>investigations</span>
              <strong>{metrics.totalInvestigations}</strong>
            </div>
            <div className="metric">
              <span>completed / failed</span>
              <strong>
                {metrics.completed} / {metrics.failed}
              </strong>
            </div>
            <div className="metric">
              <span>avg confidence</span>
              <strong>{Math.round(metrics.avgConfidence * 100)}%</strong>
            </div>
            <div className="metric">
              <span>avg time</span>
              <strong>{Math.max(0, Math.round(metrics.avgDurationSec))}s</strong>
            </div>
            <div className="metric">
              <span>workers</span>
              <strong>
                {metrics.busySlots}/{metrics.workerSlots}
              </strong>
            </div>
          </div>
        )}
        {metrics && (
          <>
            <p className="hint">
              ollama {metrics.ollamaReachable ? "reachable" : "offline"} · adapter{" "}
              {metrics.adapterOnDisk ? "on disk" : "missing"} · azula-incident{" "}
              {metrics.incidentModelReady ? "loaded" : "not in ollama"} · routing {metrics.modelAName} / {metrics.modelBName}
            </p>
            {metrics.topCauses.length > 0 && (
              <p className="hint">top causes: {metrics.topCauses.join(" · ")}</p>
            )}
            {!metrics.incidentModelReady && metrics.adapterOnDisk && (
              <p className="error">
                weights exist at adapters/azula-incident/merged-fp16 but ollama does not list azula-incident. run scripts/import-azula-incident.ps1.
              </p>
            )}
          </>
        )}
      </section>

      <form className="panel form-grid" onSubmit={onSave}>
        <h2 className="wide">models</h2>
        <label>
          model a provider
          <input value={cfg.modelAProvider} onChange={(e) => setCfg({ ...cfg, modelAProvider: e.target.value })} />
        </label>
        <label>
          model a name (fast)
          <input
            list="ollama-models"
            value={cfg.modelAName}
            onChange={(e) => setCfg({ ...cfg, modelAName: e.target.value })}
          />
        </label>
        <label>
          model b provider
          <input value={cfg.modelBProvider} onChange={(e) => setCfg({ ...cfg, modelBProvider: e.target.value })} />
        </label>
        <label>
          model b name (deep)
          <input
            list="ollama-models"
            value={cfg.modelBName}
            onChange={(e) => setCfg({ ...cfg, modelBName: e.target.value })}
            placeholder="azula-incident"
          />
        </label>
        <datalist id="ollama-models">
          {ollamaNames.map((n) => (
            <option key={n} value={n} />
          ))}
        </datalist>
        <p className="hint wide">
          model b should be ollama <code>azula-incident</code> (qlora merge). model a stays <code>qwen2.5:1.5b</code>.
        </p>
        <label>
          temperature
          <input
            type="number"
            step="0.1"
            min="0"
            max="2"
            value={cfg.temperature}
            onChange={(e) => setCfg({ ...cfg, temperature: Number(e.target.value) })}
          />
        </label>
        <label>
          max tokens
          <input
            type="number"
            value={cfg.maxTokens}
            onChange={(e) => setCfg({ ...cfg, maxTokens: Number(e.target.value) })}
          />
        </label>
        <label>
          active slot
          <select value={cfg.activeSlot} onChange={(e) => setCfg({ ...cfg, activeSlot: e.target.value })}>
            <option value="A">a — fast</option>
            <option value="B">b — deep</option>
          </select>
        </label>
        <label className="wide">
          investigator prompt
          <textarea rows={3} value={cfg.investigatorPrompt} onChange={(e) => setCfg({ ...cfg, investigatorPrompt: e.target.value })} />
        </label>
        <label className="wide">
          challenger prompt
          <textarea rows={3} value={cfg.challengerPrompt} onChange={(e) => setCfg({ ...cfg, challengerPrompt: e.target.value })} />
        </label>
        <label className="wide">
          judge prompt
          <textarea rows={3} value={cfg.judgePrompt} onChange={(e) => setCfg({ ...cfg, judgePrompt: e.target.value })} />
        </label>
        {error && <p className="error wide">{error}</p>}
        {saved && <p className="ok wide">{saved}</p>}
        <div className="wide project-actions">
          <button type="submit" disabled={viewer}>
            {viewer ? "viewers cannot change model config" : "save model config"}
          </button>
          <button type="button" disabled={viewer} onClick={attachB}>
            attach azula-incident to model b
          </button>
        </div>
      </form>

      <section className="panel">
        <h2>fine-tune (qlora)</h2>
        <p className="feed-lead">
          if merged weights are already on disk, start job marks them ready and points model b at azula-incident.
          otherwise python train.py runs and writes adapters/&lt;jobId&gt;.
        </p>
        <div className="project-actions">
          <button
            type="button"
            disabled={!wsId || viewer}
            onClick={async () => {
              setError("");
              try {
                const data = await gql<{ startFineTuneJob: FineTuneJob }>(
                  `mutation ($id: ID!) { startFineTuneJob(workspaceId: $id) { id status adapterPath error createdAt } }`,
                  { id: wsId }
                );
                setSaved(`fine-tune job ${data.startFineTuneJob.id} → ${data.startFineTuneJob.status}`);
                await loadJobs(wsId);
                await loadMetrics(wsId);
              } catch (err) {
                setError(err instanceof Error ? err.message : "Failed");
              }
            }}
          >
            start fine-tune job
          </button>
        </div>
        {jobs.length === 0 ? (
          <div className="empty-state" style={{ marginTop: 16 }}>
            <p className="empty-title">no jobs yet</p>
            <p className="empty-text">start a run to register the connected adapter and attach model b.</p>
          </div>
        ) : (
          <div className="project-list" style={{ marginTop: 16 }}>
            {jobs.map((job) => (
              <article key={job.id} className="project-card">
                <div className="project-header">
                  <h3 className="project-title">{job.id}</h3>
                  <span className="badge">{job.status}</span>
                </div>
                <dl className="project-meta">
                  <div>
                    <dt>adapter</dt>
                    <dd>{job.adapterPath || "—"}</dd>
                  </div>
                  <div>
                    <dt>created</dt>
                    <dd>{job.createdAt.slice(0, 19).replace("T", " ")}</dd>
                  </div>
                </dl>
                {job.error && <p className="error">{job.error}</p>}
              </article>
            ))}
          </div>
        )}
      </section>
    </div>
  );
}
