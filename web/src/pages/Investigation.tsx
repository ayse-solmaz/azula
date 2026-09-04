import { useEffect, useState } from "react";
import { useParams } from "react-router-dom";
import { Evidence, gql, INV_FIELDS, Investigation } from "../api";

function Conf({ value }: { value: number }) {
  const pct = Math.round(value * 100);
  return <span className="conf">{pct}%</span>;
}

function EvidenceList({ items, onOpen }: { items: Evidence[]; onOpen?: (file: string) => void }) {
  if (!items?.length) return <p className="flag">No evidence linked to this claim.</p>;
  return (
    <ul className="evidence">
      {items.map((e, i) => (
        <li key={i}>
          <button type="button" className="linkish evidence-file" onClick={() => onOpen?.(e.file)}>
            <strong>{e.file}</strong>
          </button>{" "}
          <span className="muted">L{e.lines}</span>
          <pre>{e.excerpt}</pre>
        </li>
      ))}
    </ul>
  );
}

export default function InvestigationPage() {
  const { id } = useParams();
  const [inv, setInv] = useState<Investigation | null>(null);
  const [error, setError] = useState("");
  const [fileName, setFileName] = useState("");
  const [fileBody, setFileBody] = useState("");

  useEffect(() => {
    let stop = false;
    async function tick() {
      try {
        const data = await gql<{ investigation: Investigation }>(
          `query ($id: ID!) { investigation(id: $id) { ${INV_FIELDS} } }`,
          { id }
        );
        if (!stop) setInv(data.investigation);
        const st = data.investigation.status;
        if (st !== "COMPLETED" && st !== "FAILED") {
          setTimeout(tick, 1200);
        }
      } catch (e) {
        if (!stop) setError(e instanceof Error ? e.message : "Failed");
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
    setFileBody("Loading…");
    try {
      const data = await gql<{ fileContent: string }>(
        `query ($projectId: ID!, $name: String!) { fileContent(projectId: $projectId, name: $name) }`,
        { projectId: inv.projectId, name }
      );
      setFileBody(data.fileContent);
    } catch (e) {
      setFileBody(e instanceof Error ? e.message : "Could not read file");
    }
  }

  if (error) return <p className="page error">{error}</p>;
  if (!inv) return <p className="page">loading investigation…</p>;

  const running = !["COMPLETED", "FAILED"].includes(inv.status);

  return (
    <div className="page inv-layout">
      <aside className="panel plan">
        <h2>agent plan</h2>
        <ol>
          {inv.plan.map((s) => (
            <li key={s.order} className={s.status.toLowerCase()}>
              <span className="mark">{s.status === "DONE" ? "✓" : s.status === "RUNNING" ? "●" : "○"}</span>
              {s.description}
            </li>
          ))}
        </ol>
        <p className="hint">status: {inv.status.replaceAll("_", " ")}</p>
        {running && <p className="pulse">executing…</p>}
        {inv.errorMessage && <p className="error">{inv.errorMessage}</p>}
        {(inv.modelAName || inv.modelBName) && (
          <p className="hint">
            models: {inv.modelAName || "a"} → fast · {inv.modelBName || "b"} → deep
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
            export json
          </button>
        )}
        {!!inv.filesAccessed?.length && (
          <p className="hint">mcp: {inv.filesAccessed.join(", ")}</p>
        )}
      </aside>
      <section className="stack">
        {!inv.fastResult && running && (
          <section className="panel">
            <h2>investigation</h2>
            <p className="feed-lead">fast → deep → council. results appear in this column as each stage finishes.</p>
          </section>
        )}
        {inv.fastResult && (
          <article className="panel">
            <h2>fast classification</h2>
            <p>
              <span className="badge">{inv.fastResult.incidentType}</span> <Conf value={inv.fastResult.confidence} />
            </p>
            <p className="feed-lead">{inv.fastResult.summary}</p>
          </article>
        )}
        {inv.deepResult && (
          <article className="panel">
            <h2>deep analysis</h2>
            {inv.modelBName && <p className="hint">model b · {inv.modelBName}</p>}
            <p>
              <strong>{inv.deepResult.rootCause}</strong> <Conf value={inv.deepResult.confidence} />
            </p>
            <p className="feed-lead">{inv.deepResult.suggestedFix}</p>
            <EvidenceList items={inv.deepResult.evidence} onOpen={openFile} />
          </article>
        )}
        {inv.councilResult && (
          <article className="panel">
            <h2>ai council</h2>
            <div className="council">
              {inv.councilResult.models.map((m) => (
                <div key={m.role} className="project-card">
                  <h3>{m.role}</h3>
                  <p>
                    {m.hypothesis} <Conf value={m.confidence} />
                  </p>
                  <EvidenceList items={m.evidence} onOpen={openFile} />
                </div>
              ))}
            </div>
            <h3>agreements</h3>
            <ul className="files">
              {inv.councilResult.agreements.map((a, i) => (
                <li key={i}>{a}</li>
              ))}
            </ul>
            <h3>disagreements</h3>
            <ul className="files">
              {inv.councilResult.disagreements.map((d, i) => (
                <li key={i}>
                  <strong>{d.topic}:</strong> investigator — {d.investigator}; challenger — {d.challenger}
                </li>
              ))}
            </ul>
            <div className="judgment">
              <h3>final judgment</h3>
              <p>
                {inv.councilResult.finalJudgment.mostLikelyCause}{" "}
                <Conf value={inv.councilResult.finalJudgment.confidence} />
              </p>
              <p>{inv.councilResult.finalJudgment.recommendedAction}</p>
            </div>
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
