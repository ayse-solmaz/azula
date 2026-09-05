import { ChangeEvent, DragEvent, ReactNode, useState } from "react";
import type { Investigation } from "./api";
import { useI18n } from "./i18n";

export type Translate = ReturnType<typeof useI18n>["t"];

export const FREE_TIER_MAX_PROJECTS = 3;

export function isFreeTier(tier?: string | null) {
  return !tier || tier.toUpperCase() === "FREE";
}

/** Personal workspaces and org engineers/admins can mutate; viewers cannot. */
export function canEdit(role?: string | null) {
  return role !== "viewer";
}

export function isTierLimitError(message: string) {
  return /project limit|tier allows|FREE_TIER_MAX_PROJECTS|investigations used this month/i.test(message);
}

export function isProFeatureError(message: string) {
  return /requires Pro|available on Pro|investigations used this month/i.test(message);
}

export function UpgradeBanner({
  title,
  text,
  demo,
}: {
  title: string;
  text: string;
  demo?: boolean;
}) {
  async function upgrade() {
    const { gql } = await import("./api");
    if (demo) {
      await gql(`mutation { activateProDemo { id tier } }`);
      window.location.reload();
      return;
    }
    const data = await gql<{ createCheckoutSession: { url: string } }>(
      `mutation { createCheckoutSession { url } }`
    );
    window.location.href = data.createCheckoutSession.url;
  }
  const { t } = useI18n();
  return (
    <div className="limit-banner">
      <p>
        <strong>{title}</strong>
      </p>
      <p>{text}</p>
      <button type="button" className="primary" onClick={() => void upgrade().catch(() => undefined)}>
        {demo ? t("activateProDemo") : t("upgradePro")}
      </button>
    </div>
  );
}

export function fileKind(name: string) {
  const ext = name.includes(".") ? name.slice(name.lastIndexOf(".") + 1).toLowerCase() : "";
  return ext || "file";
}

export function formatWhen(iso?: string | null, locale?: string) {
  if (!iso) return "—";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso.slice(0, 16).replace("T", " ");
  const tag = locale === "tr" ? "tr-TR" : locale === "en" ? "en-US" : undefined;
  return d.toLocaleString(tag, { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" });
}

export function statusTone(status?: string | null) {
  const s = (status || "").toLowerCase();
  if (["completed", "done", "enabled", "ok", "ready", "merged"].includes(s)) return "ok";
  if (["failed", "fail", "error", "config_error", "off"].includes(s)) return "fail";
  if (["pending", "queued", "not started", "warning", "running", "training", "merging", "fast_classify", "deep_analyze", "council"].includes(s)) return "warn";
  return "neutral";
}

export function isRunningStatus(status?: string | null) {
  const s = (status || "").toUpperCase();
  return ["PENDING", "FAST_CLASSIFY", "DEEP_ANALYZE", "COUNCIL"].includes(s);
}

export function prettyStatus(status?: string | null, t?: Translate) {
  if (!status) return t ? t("notStarted") : "Not started";
  switch (status.toUpperCase()) {
    case "PENDING":
      return t ? t("statusQueued") : "Queued";
    case "FAST_CLASSIFY":
      return t ? t("statusFast") : "Quick look";
    case "DEEP_ANALYZE":
      return t ? t("statusDeep") : "Deep look";
    case "COUNCIL":
      return t ? t("statusCouncil") : "Council";
    case "COMPLETED":
      return t ? t("statusComplete") : "Complete";
    case "FAILED":
      return t ? t("statusFailed") : "Failed";
    default:
      return status.replaceAll("_", " ").toLowerCase().replace(/^\w/, (c) => c.toUpperCase());
  }
}

export function roleLabel(role: string, t?: Translate) {
  switch ((role || "").toLowerCase()) {
    case "investigator":
      return t ? t("roleInvestigator") : "Investigator";
    case "challenger":
      return t ? t("roleChallenger") : "Challenger";
    case "judge":
      return t ? t("roleJudge") : "Judge";
    default:
      return role;
  }
}

export function executionTone(mode?: string | null) {
  const m = (mode || "").toLowerCase();
  if (m === "live") return "ok";
  if (m === "fallback" || m === "mixed") return "warn";
  return "neutral";
}

export function executionLabel(mode?: string | null, t?: Translate) {
  const m = (mode || "").toLowerCase();
  if (m === "live") return t ? t("liveModels") : "Live models";
  if (m === "fallback") return t ? t("fallbackCanned") : "Fallback (canned)";
  if (m === "mixed") return t ? t("mixedLive") : "Mixed live + fallback";
  return "";
}

export function aggregationTone(kind?: string | null) {
  const k = (kind || "").toLowerCase();
  if (k === "consensus") return "ok";
  if (k === "echo_chamber") return "warn";
  if (k === "disagreement") return "fail";
  return "neutral";
}

export function aggregationTitle(kind?: string | null, t?: Translate) {
  switch ((kind || "").toLowerCase()) {
    case "consensus":
      return t ? t("aggConsensus") : "Independent consensus";
    case "echo_chamber":
      return t ? t("aggEcho") : "Echo chamber";
    case "disagreement":
      return t ? t("aggSplit") : "Split judgment";
    default:
      return kind ? kind.replaceAll("_", " ") : t ? t("council") : "Council";
  }
}

export function aggregationBlurb(kind?: string | null, t?: Translate) {
  switch ((kind || "").toLowerCase()) {
    case "consensus":
      return t ? t("aggBlurbConsensus") : "Different model families named a similar cause. Confidence is boosted.";
    case "echo_chamber":
      return t ? t("aggBlurbEcho") : "Same family restated a similar cause. This is not independent agreement.";
    case "disagreement":
      return t ? t("aggBlurbSplit") : "Hypotheses diverge. Weighted vote picked a winner; review before closing.";
    default:
      return t ? t("aggBlurbDefault") : "Go scored Investigator vs Challenger after the Judge narrative.";
  }
}

export type CouncilViewState = "pending" | "complete" | "fallback" | "idle";

export type CouncilBadgeKey =
  | "fallback"
  | "echo_chamber"
  | "independent_consensus"
  | "split_judgment"
  | "needs_review";

export type CouncilBadge = { key: CouncilBadgeKey; label: string; tone: string };

function execMode(inv: Pick<Investigation, "executionMode">) {
  return (inv.executionMode || "").toLowerCase();
}

function invStatus(inv: Pick<Investigation, "status">) {
  return (inv.status || "").toLowerCase();
}

/** Derive Council screen state from Investigation fields — no parallel API enum. */
export function councilViewState(inv: Pick<Investigation, "status" | "councilResult" | "executionMode">): CouncilViewState {
  if (execMode(inv) === "fallback") return "fallback";
  if (inv.councilResult) return "complete";
  const st = invStatus(inv);
  if (["pending", "fast_classify", "deep_analyze", "council"].includes(st)) return "pending";
  return "idle";
}

export function isCannedFallback(inv: Pick<Investigation, "executionMode">) {
  return execMode(inv) === "fallback";
}

/** Full canned fallback: Fast summary only — do not render Judge / debate as live. */
export function showCouncilDebate(inv: Pick<Investigation, "councilResult" | "executionMode">) {
  return Boolean(inv.councilResult) && execMode(inv) !== "fallback";
}

export function councilBadges(inv: Pick<Investigation, "executionMode" | "councilResult">, t?: Translate): CouncilBadge[] {
  const out: CouncilBadge[] = [];
  if (execMode(inv) === "fallback") {
    out.push({
      key: "fallback",
      label: t ? t("badgeFallback") : "Canned fallback — not a live model debate",
      tone: "warn",
    });
  }
  const agg = (inv.councilResult?.aggregation || "").toLowerCase();
  if (agg === "echo_chamber") {
    out.push({ key: "echo_chamber", label: t ? t("badgeEcho") : "Echo chamber · same model family", tone: "warn" });
  } else if (agg === "consensus") {
    out.push({ key: "independent_consensus", label: t ? t("aggConsensus") : "Independent consensus", tone: "ok" });
  } else if (agg === "disagreement") {
    out.push({ key: "split_judgment", label: t ? t("aggSplit") : "Split judgment", tone: "fail" });
  }
  if (inv.councilResult?.needsReview) {
    out.push({ key: "needs_review", label: t ? t("badgeReview") : "Needs human review", tone: "fail" });
  }
  return out;
}

export function deepSkipped(inv: Pick<Investigation, "status" | "deepResult" | "councilResult">) {
  const st = (inv.status || "").toLowerCase();
  return st === "completed" && !inv.deepResult && !inv.councilResult;
}

export function stageCopy(status: string, t?: Translate) {
  switch (status) {
    case "PENDING":
      return t ? t("stagePending") : "Queuing workers…";
    case "FAST_CLASSIFY":
      return t ? t("stageFast") : "Investigator classifying the incident…";
    case "DEEP_ANALYZE":
      return t ? t("stageDeep") : "Deep analysis — reading files and tracing the failure…";
    case "COUNCIL":
      return t ? t("stageCouncil") : "Council in session — Investigator, Challenger, and Judge…";
    case "COMPLETED":
      return t ? t("stageDone") : "Investigation complete";
    case "FAILED":
      return t ? t("stageFail") : "Investigation failed";
    default:
      return status.replaceAll("_", " ");
  }
}

const UUID_RE = /[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/i;

export function friendlyResource(resource: string, t?: Translate): { label: string; detail: string } {
  if (!resource) return { label: "—", detail: "" };
  if (resource.startsWith("device:")) {
    const id = resource.slice("device:".length);
    return { label: t ? t("trustedDevice") : "Trusted device", detail: id };
  }
  const m = resource.match(UUID_RE);
  if (m) {
    return { label: resource.replace(m[0], "record").replace(/:+/g, " · "), detail: resource };
  }
  return { label: resource, detail: resource };
}

export function ConfBar({ value, label }: { value: number; label?: string }) {
  const { t } = useI18n();
  const pct = Math.max(0, Math.min(100, Math.round(value * 100)));
  const tone = pct >= 80 ? "ok" : pct >= 50 ? "warn" : "fail";
  return (
    <span className={`conf-bar tone-${tone}`} title={t("confTitle", { pct })}>
      <span className="conf-bar-fill" style={{ width: `${pct}%` }} />
      <span className="conf-bar-label">{label ? `${label} ${pct}%` : `${pct}%`}</span>
    </span>
  );
}

export function StatusDot({ ok, label }: { ok: boolean; label: string }) {
  return (
    <span className={`status-dot ${ok ? "on" : "off"}`}>
      <i />
      {label}
    </span>
  );
}

export function HowTo({
  title,
  steps,
}: {
  title: string;
  steps: { n: string; title: string; body: string }[];
}) {
  return (
    <section className="panel howto">
      <h2>{title}</h2>
      <ol className="howto-steps">
        {steps.map((s) => (
          <li key={s.n}>
            <span className="howto-n">{s.n}</span>
            <div>
              <strong>{s.title}</strong>
              <p>{s.body}</p>
            </div>
          </li>
        ))}
      </ol>
    </section>
  );
}

export function EmptyState({
  title,
  text,
  action,
}: {
  title: string;
  text: string;
  action?: ReactNode;
}) {
  return (
    <div className="empty-state">
      <div className="empty-mark" aria-hidden />
      <p className="empty-title">{title}</p>
      <p className="empty-text">{text}</p>
      {action}
    </div>
  );
}

export function FileDropzone({
  disabled,
  onFiles,
}: {
  disabled?: boolean;
  onFiles: (files: FileList) => void;
}) {
  const { t } = useI18n();
  const [over, setOver] = useState(false);

  function take(files: FileList | null) {
    if (files?.length) onFiles(files);
  }

  function onDrop(e: DragEvent) {
    e.preventDefault();
    setOver(false);
    take(e.dataTransfer.files);
  }

  function onChange(e: ChangeEvent<HTMLInputElement>) {
    take(e.target.files);
    e.target.value = "";
  }

  return (
    <label
      className={`dropzone ${over ? "over" : ""} ${disabled ? "disabled" : ""}`}
      onDragOver={(e) => {
        e.preventDefault();
        if (!disabled) setOver(true);
      }}
      onDragLeave={() => setOver(false)}
      onDrop={onDrop}
    >
      <input
        type="file"
        multiple
        disabled={disabled}
        accept=".log,.yaml,.yml,.py,.json,.jsonl,.csv,.txt"
        onChange={onChange}
      />
      <span className="dropzone-icon" aria-hidden>
        ↑
      </span>
      <span className="dropzone-title">{t("dropTitle")}</span>
      <span className="muted">{t("dropHint")}</span>
    </label>
  );
}

export function Tabs({
  tabs,
  active,
  onChange,
}: {
  tabs: { id: string; label: string }[];
  active: string;
  onChange: (id: string) => void;
}) {
  return (
    <div className="tabs" role="tablist">
      {tabs.map((t) => (
        <button
          key={t.id}
          type="button"
          role="tab"
          aria-selected={active === t.id}
          className={`tab ${active === t.id ? "active" : ""}`}
          onClick={() => onChange(t.id)}
        >
          {t.label}
        </button>
      ))}
    </div>
  );
}
