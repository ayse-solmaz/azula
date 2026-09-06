import { useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { CouncilResult, Evidence, gql, INV_FIELDS, Investigation } from "../api";
import {
  ConfBar,
  aggregationBlurb,
  aggregationTone,
  councilBadges,
  councilViewState,
  deepSkipped,
  executionLabel,
  executionTone,
  prettyStatus,
  roleLabel,
  showCouncilDebate,
  stageCopy,
} from "../ui";
import { useI18n } from "../i18n";

function Conf({ value }: { value: number }) {
  return <ConfBar value={value} />;
}

const STAGES = [
  { id: "FAST_CLASSIFY", label: "Investigator classifying the incident…" },
  { id: "DEEP_ANALYZE", label: "Deep analysis — reading files and tracing the failure…" },
  { id: "COUNCIL", label: "Council — Investigator, Challenger, and Judge synthesizing…" },
];

function stageState(current: string, id: string, skipped: boolean) {
  const order = ["PENDING", "FAST_CLASSIFY", "DEEP_ANALYZE", "COUNCIL", "COMPLETED"];
  const cur = order.indexOf(current === "FAILED" ? "COMPLETED" : current);
  const idx = order.indexOf(id);
  if (skipped && (id === "DEEP_ANALYZE" || id === "COUNCIL")) return "skip";
  if (current === "FAILED") return "fail";
  if (current === "COMPLETED" || cur > idx) return "done";
  if (current === id || (current === "PENDING" && id === "FAST_CLASSIFY")) return "active";
  return "";
}

function EvidenceList({ items, onOpen, active }: { items: Evidence[]; onOpen?: (file: string) => void; active?: string }) {
  const { t } = useI18n();
  if (!items?.length) return <p className="flag">{t("noEvidence")}</p>;
  return (
    <ul className="evidence">
      {items.map((e, i) => (
        <li key={i}>
          <button
            type="button"
            className="linkish evidence-file"
            aria-current={active === e.file ? "true" : undefined}
            onClick={() => onOpen?.(e.file)}
            onKeyDown={(ev) => {
              if (ev.key === "Enter" || ev.key === " ") {
                ev.preventDefault();
                onOpen?.(e.file);
              }
            }}
          >
            <strong>{e.file}</strong>
          </button>{" "}
          <span className="muted">L{e.lines}</span>
          <pre>{e.excerpt}</pre>
        </li>
      ))}
    </ul>
  );
}

function modelOf(m: { role: string; model?: string | null }, inv: Investigation) {
  if (m.model) return m.model;
  if (m.role === "challenger") return inv.modelAName || "—";
  return inv.modelBName || "—";
}

function CouncilBoard({
  council,
  inv,
  onOpenFile,
  fileName,
  assembling,
}: {
  council: CouncilResult;
  inv: Investigation;
  onOpenFile: (file: string) => void;
  fileName: string;
  assembling?: boolean;
}) {
  const { t } = useI18n();
  const investigator = council.models.find((m) => m.role === "investigator") || council.models[0];
  const challenger = council.models.find((m) => m.role === "challenger") || council.models[1];
  const invScore = investigator?.confidence ?? 0;
  const chalScore = challenger?.confidence ?? 0;
  const total = invScore + chalScore || 1;
  const winnerRole =
    invScore === chalScore ? "" : invScore > chalScore ? investigator?.role : challenger?.role;
  const tone = aggregationTone(council.aggregation);
  const badges = councilBadges(inv, t);
  const echo = badges.some((b) => b.key === "echo_chamber");

  return (
    <article className="panel council-hero">
      <div className="council-head">
        <div>
          <h2>{t("councilResult")}</h2>
          <p className="council-note">{aggregationBlurb(council.aggregation, t)}</p>
        </div>
        <div className="council-tags">
          {badges.map((b) => (
            <span key={b.key} className={`badge ${b.tone}`}>
              {b.label}
            </span>
          ))}
          {inv.executionMode && (inv.executionMode || "").toLowerCase() !== "fallback" ? (
            <span className={`badge ${executionTone(inv.executionMode)}`}>{executionLabel(inv.executionMode, t)}</span>
          ) : null}
        </div>
      </div>

      {echo ? (
        <p className="flag">
          {t("echoFlag")}
        </p>
      ) : null}
      {council.aggregationNote ? <p className={`flag ${tone === "fail" ? "fail" : tone === "ok" ? "ok" : ""}`}>{council.aggregationNote}</p> : null}

      <div className="vote-meter" title="Stated confidence share (not the Go vote score)">
        <span>{t("roleInvestigator")} {Math.round(invScore * 100)}%</span>
        <div className="vote-track">
          <span className="vote-inv" style={{ width: `${(invScore / total) * 100}%` }} />
          <span className="vote-chal" style={{ width: `${(chalScore / total) * 100}%` }} />
        </div>
        <span>{t("roleChallenger")} {Math.round(chalScore * 100)}%</span>
      </div>

      <div className="debate">
        {investigator ? (
          <div className={`debate-card ${winnerRole === investigator.role ? "winner" : winnerRole ? "loser" : ""}`}>
            <div className="project-badges">
              <span className="badge accent">{roleLabel(investigator.role, t)}</span>
              <span className="badge">{modelOf(investigator, inv)}</span>
              {winnerRole === investigator.role ? <span className="badge ok">{t("voteLead")}</span> : null}
            </div>
            <h3>{t("hypothesis")}</h3>
            <p className="hypo">{investigator.hypothesis}</p>
            <Conf value={investigator.confidence} />
            <EvidenceList items={investigator.evidence} onOpen={onOpenFile} active={fileName} />
          </div>
        ) : assembling ? (
          <div className="debate-card">
            <span className="badge accent">{t("roleInvestigator")}</span>
            <p className="hint">{t("modelBThinking")}</p>
            <div className="skel" />
            <div className="skel" />
          </div>
        ) : null}
        <div className="debate-vs" aria-hidden>
          VS
        </div>
        {challenger ? (
          <div className={`debate-card ${winnerRole === challenger.role ? "winner" : winnerRole ? "loser" : ""}`}>
            <div className="project-badges">
              <span className="badge accent">{roleLabel(challenger.role, t)}</span>
              <span className="badge">{modelOf(challenger, inv)}</span>
              {winnerRole === challenger.role ? <span className="badge ok">{t("voteLead")}</span> : null}
            </div>
            <h3>{t("hypothesis")}</h3>
            <p className="hypo">{challenger.hypothesis}</p>
            <Conf value={challenger.confidence} />
            <EvidenceList items={challenger.evidence} onOpen={onOpenFile} active={fileName} />
          </div>
        ) : assembling ? (
          <div className="debate-card">
            <span className="badge accent">{t("roleChallenger")}</span>
            <p className="hint">{t("modelAThinking")}</p>
            <div className="skel" />
            <div className="skel" />
          </div>
        ) : null}
      </div>

      {assembling && !council.finalJudgment?.mostLikelyCause ? (
        <p className="council-note">{t("judgeWaiting")}</p>
      ) : (
        <>
          <h3>{t("whereAgree")}</h3>
          {council.agreements.length ? (
            <ul className="files">
              {council.agreements.map((a, i) => (
                <li key={i}>{a}</li>
              ))}
            </ul>
          ) : (
            <p className="hint">{t("noShared")}</p>
          )}

          <h3>{t("whereSplit")}</h3>
          {council.disagreements.length ? (
            <ul className="split-list">
              {council.disagreements.map((d, i) => (
                <li key={i}>
                  <span className="topic">{d.topic || t("rootCause")}</span>
                  <p className="side">
                    <span>{t("roleInvestigator")}</span>
                    {d.investigator}
                  </p>
                  <p className="side">
                    <span>{t("roleChallenger")}</span>
                    {d.challenger}
                  </p>
                </li>
              ))}
            </ul>
          ) : (
            <p className="hint">{t("noSplit")}</p>
          )}

          <div className="verdict">
            <h3>{t("finalJudgment")}</h3>
            <p className="verdict-cause">{council.finalJudgment.mostLikelyCause}</p>
            <Conf value={council.finalJudgment.confidence} />
            <p>{council.finalJudgment.recommendedAction}</p>
            {council.needsReview ? <p className="hint">{t("needsReviewHint")}</p> : null}
          </div>
        </>
      )}

      <details className="how-vote">
        <summary>{t("howVote")}</summary>
        <ol>
          <li>{t("howVote1")}</li>
          <li>{t("howVote2")}</li>
          <li>{t("howVote3")}</li>
          <li>{t("howVote4")}</li>
        </ol>
      </details>
    </article>
  );
}

export default function InvestigationPage() {
  const { t } = useI18n();
  const { id } = useParams();
  const nav = useNavigate();
  const [inv, setInv] = useState<Investigation | null>(null);
  const [error, setError] = useState("");
  const [fileName, setFileName] = useState("");
  const [fileBody, setFileBody] = useState("");

  useEffect(() => {
    let stop = false;
    async function tick() {
      try {
        const data = await gql<{ investigation: Investigation | null }>(
          `query ($id: ID!) { investigation(id: $id) { ${INV_FIELDS} } }`,
          { id }
        );
        if (!data.investigation) {
          if (!stop) setError(t("failed"));
          return;
        }
        if (!stop) setInv(data.investigation);
        const st = data.investigation.status;
        if (st !== "COMPLETED" && st !== "FAILED") {
          const delay = st === "COUNCIL" || st === "DEEP_ANALYZE" ? 600 : 1200;
          setTimeout(tick, delay);
        }
      } catch (e) {
        if (!stop) setError(e instanceof Error ? e.message : t("failed"));
      }
    }
    tick();
    return () => {
      stop = true;
    };
  }, [id]);

  async function openFile(name: string) {
    if (!inv) return;
    setFileName(name);
    setFileBody(t("loadingFile"));
    try {
      const data = await gql<{ fileContent: string }>(
        `query ($projectId: ID!, $name: String!) { fileContent(projectId: $projectId, name: $name) }`,
        { projectId: inv.projectId, name }
      );
      setFileBody(data.fileContent);
    } catch (e) {
      setFileBody(e instanceof Error ? e.message : t("readFileFail"));
    }
  }

  if (error) {
    return (
      <div className="page">
        <section className="panel">
          <h2>{t("invLoadFail")}</h2>
          <p className="error">{error}</p>
          <Link className="primary" to="/">
            {t("backHome")}
          </Link>
        </section>
      </div>
    );
  }
  if (!inv) return <p className="page muted">{t("loadingInv")}</p>;

  const running = !["COMPLETED", "FAILED"].includes(inv.status);
  const view = councilViewState(inv);
  const debate = showCouncilDebate(inv);
  const judged = Boolean(inv.councilResult?.finalJudgment?.mostLikelyCause);
  const hasCouncilModels = (inv.councilResult?.models?.length ?? 0) > 0;
  const councilFailed = inv.status === "FAILED" && !inv.councilResult;
  const skipped = deepSkipped(inv);
  const fastPct = Math.round((inv.fastResult?.confidence ?? 0) * 100);

  return (
    <div className="page inv-layout">
      <aside className="panel plan">
        <h2>{t("pipelineTitle")}</h2>
        <p className="feed-lead">{t("invLead")}</p>
        <div className="stage-track">
          {STAGES.map((st) => (
            <div key={st.id} className={`stage-row ${stageState(inv.status, st.id, skipped)}`}>
              <span className="stage-pip" />
              <span>
                {st.id === "FAST_CLASSIFY"
                  ? t("stageFastShort")
                  : st.id === "DEEP_ANALYZE"
                    ? skipped
                      ? t("stageDeepSkipped")
                      : t("stageDeepShort")
                    : skipped
                      ? t("stageCouncilSkipped")
                      : t("stageCouncilShort")}
              </span>
            </div>
          ))}
          {running ? <p className="pulse">{stageCopy(inv.status, t)}</p> : null}
        </div>
        {skipped ? (
          <div className="skip-note">
            <p>
              {inv.escalationReason ? (
                inv.escalationReason
              ) : (
                <>
                  <strong>{t("skipBanner", { pct: fastPct })}</strong>
                  {t("skipBannerBody")}
                </>
              )}
            </p>
          </div>
        ) : inv.escalationReason ? (
          <div className="skip-note">
            <p>{inv.escalationReason}</p>
          </div>
        ) : null}
        <p className="hint">{t("status", { s: prettyStatus(inv.status, t) })}</p>
        {running && (
          <div className="project-actions">
            <button
              type="button"
              className="danger"
              onClick={() => {
                void gql<{ cancelInvestigation: Investigation }>(
                  `mutation ($id: ID!) { cancelInvestigation(id: $id) { ${INV_FIELDS} } }`,
                  { id: inv.id }
                )
                  .then((d) => setInv(d.cancelInvestigation))
                  .catch((e) => setError(e instanceof Error ? e.message : t("cancelFail")));
              }}
            >
              {t("stopAgent")}
            </button>
          </div>
        )}
        {inv.executionMode && (inv.executionMode || "").toLowerCase() !== "live" ? (
          <p>
            <span className={`badge ${executionTone(inv.executionMode)}`}>{executionLabel(inv.executionMode, t)}</span>
          </p>
        ) : null}
        {inv.status === "COMPLETED" && (
          <button type="button" className="primary" onClick={() => nav(`/loop/${inv.projectId}`)}>
            {t("continueFix")}
          </button>
        )}
        {inv.errorMessage && <p className="error">{inv.errorMessage}</p>}
        <details className="archive-block">
          <summary>{t("techDetails")}</summary>
          {(inv.modelAName || inv.modelBName || inv.modelCName) && (
            <p className="hint">
              {t("stageFastShort")} {inv.modelAName || "—"} · {t("stageDeepShort")} {inv.modelBName || "—"}
              {inv.modelCName ? t("judgeApi", { name: inv.modelCName }) : t("judgeLocal")}
            </p>
          )}
          {inv.status === "COMPLETED" && (
            <button
              type="button"
              onClick={() => {
                const blob = new Blob([JSON.stringify(inv, null, 2)], { type: "application/json" });
                const url = URL.createObjectURL(blob);
                const a = document.createElement("a");
                a.href = url;
                a.download = `azula-investigation-${inv.id}.json`;
                a.click();
                URL.revokeObjectURL(url);
              }}
            >
              {t("exportJson")}
            </button>
          )}
          {!!inv.filesAccessed?.length && <p className="hint">{t("mcpFiles", { list: inv.filesAccessed.join(", ") })}</p>}
        </details>
        <p>
          <Link className="linkish" to="/">
            {t("backHome")}
          </Link>
        </p>
      </aside>
      <section className="stack">
        {inv.status === "COMPLETED" && (inv.councilResult?.finalJudgment || inv.deepResult) ? (
          <article className="panel findings-hero">
            <p className="eyebrow">{t("findingsEyebrow")}</p>
            <h2>{t("findingsCause")}</h2>
            <p className="verdict-cause">
              {inv.councilResult?.finalJudgment.mostLikelyCause || inv.deepResult?.rootCause}
            </p>
            <Conf
              value={inv.councilResult?.finalJudgment.confidence ?? inv.deepResult?.confidence ?? 0}
            />
            {(inv.councilResult?.finalJudgment.recommendedAction || inv.deepResult?.suggestedFix) && (
              <>
                <h3>{t("findingsFix")}</h3>
                <p className="feed-lead">
                  {inv.councilResult?.finalJudgment.recommendedAction || inv.deepResult?.suggestedFix}
                </p>
              </>
            )}
            <div className="project-actions">
              <button type="button" className="primary" onClick={() => nav(`/loop/${inv.projectId}`)}>
                {t("continueFix")}
              </button>
            </div>
          </article>
        ) : null}
        {debate && inv.councilResult ? (
          <CouncilBoard
            council={inv.councilResult}
            inv={inv}
            onOpenFile={openFile}
            fileName={fileName}
            assembling={running && !judged}
          />
        ) : null}
        {view === "fallback" && (
          <article className="panel council-hero">
            <div className="council-head">
              <div>
                <h2>{t("councilResult")}</h2>
                <p className="council-note">{t("councilNoteFallback")}</p>
              </div>
              <div className="council-tags">
                {councilBadges(inv, t).map((b) => (
                  <span key={b.key} className={`badge ${b.tone}`}>
                    {b.label}
                  </span>
                ))}
              </div>
            </div>
            <p className="flag">{t("fallbackFlag")}</p>
            {inv.fastResult ? (
              <p className="feed-lead">{inv.fastResult.summary}</p>
            ) : (
              <p className="hint">{t("noFastSummary")}</p>
            )}
          </article>
        )}
        {view === "pending" && !hasCouncilModels && (inv.status === "COUNCIL" || inv.status === "DEEP_ANALYZE") && (
          <article className="panel council-hero">
            <h2>{t("councilAssembling")}</h2>
            <p className="council-note">{t("councilWait")}</p>
            <div className="council-wait">
              <div className="debate-card">
                <span className="badge accent">{t("roleInvestigator")}</span>
                <p className="hint">{t("modelBThinking")}</p>
                <div className="skel" />
                <div className="skel" />
              </div>
              <div className="debate-card">
                <span className="badge accent">{t("roleChallenger")}</span>
                <p className="hint">{t("modelAThinking")}</p>
                <div className="skel" />
                <div className="skel" />
              </div>
            </div>
          </article>
        )}
        {councilFailed && (
          <article className="panel">
            <h2>{t("councilFailed")}</h2>
            <p className="flag fail">{inv.errorMessage || t("councilDidNotFinish")}</p>
            <div className="council-tags">
              {councilBadges(inv, t).map((b) => (
                <span key={b.key} className={`badge ${b.tone}`}>
                  {b.label}
                </span>
              ))}
            </div>
          </article>
        )}
        {!inv.fastResult && running && inv.status !== "COUNCIL" && inv.status !== "DEEP_ANALYZE" && (
          <section className="panel">
            <h2>{t("resultCard")}</h2>
            <p className="feed-lead">{stageCopy(inv.status, t)}</p>
          </section>
        )}
        {inv.fastResult && (
          <article className="panel">
            <h2>{t("resultCard")}</h2>
            <p>
              <span className="badge">{inv.fastResult.incidentType}</span> <Conf value={inv.fastResult.confidence} />
            </p>
            <p className="feed-lead">{inv.fastResult.summary}</p>
            {skipped ? <p className="hint">{t("skipBannerBody")}</p> : null}
          </article>
        )}
        {skipped && !inv.deepResult ? (
          <article className="panel">
            <h2>{t("skippedStage")}</h2>
            <p className="hint">{t("skipBanner", { pct: fastPct })}</p>
            <p className="feed-lead">{t("skipDeepHint")}</p>
          </article>
        ) : null}
        {inv.deepResult && view !== "fallback" && (
          <article className="panel">
            <h2>{t("deepAnalysis")}</h2>
            {inv.modelBName && <p className="hint">{t("modelB", { name: inv.modelBName })}</p>}
            <p>
              <strong>{inv.deepResult.rootCause}</strong> <Conf value={inv.deepResult.confidence} />
            </p>
            <p className="feed-lead">{inv.deepResult.suggestedFix}</p>
            <EvidenceList items={inv.deepResult.evidence} onOpen={openFile} active={fileName} />
          </article>
        )}
        {fileName && (
          <article className="panel">
            <h2>{fileName}</h2>
            <pre className="file-body">{fileBody}</pre>
          </article>
        )}
      </section>
    </div>
  );
}
