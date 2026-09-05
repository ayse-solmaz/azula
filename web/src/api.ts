const TOKEN_KEY = "azula_token";
const DEVICE_KEY = "azula_device";

function desktop() {
  return window.azulaDesktop;
}

function migrateLegacyDesktopToken() {
  const d = desktop();
  if (!d?.setSession) return;
  const leftover = localStorage.getItem("azula" + "_token");
  if (!leftover) return;
  d.setSession(leftover);
  localStorage.removeItem(TOKEN_KEY);
}

export function formatApiError(e: unknown, t: (key: "apiDown" | "failed", vars?: Record<string, string | number>) => string) {
  const msg = e instanceof Error ? e.message : String(e || "");
  if (/failed to fetch|cannot reach api|networkerror|load failed|econnrefused|failed to fetc/i.test(msg)) {
    return t("apiDown");
  }
  return msg || t("failed");
}

export function graphqlUrl() {
  const d = desktop();
  const raw = d?.graphqlUrl;
  const fromDesktop = typeof raw === "function" ? raw() : raw;
  const url = fromDesktop || "/graphql";
  return url.replace("://localhost", "://127.0.0.1");
}

export function getToken() {
  const d = desktop();
  if (d?.getToken) {
    const t = d.getToken();
    return t ? t : null;
  }
  return null;
}

export function hasSession() {
  migrateLegacyDesktopToken();
  const d = desktop();
  if (d?.hasSession) return d.hasSession();
  if (typeof document !== "undefined" && document.cookie.split(";").some((c) => c.trim().startsWith("azula_ui="))) {
    return true;
  }
  return false;
}

export function setToken(token: string | null) {
  const d = desktop();
  if (d?.setSession) {
    d.setSession(token);
    localStorage.removeItem(TOKEN_KEY);
    return;
  }
  if (!token) localStorage.removeItem(TOKEN_KEY);
}

export function getDeviceId() {
  const fromDesktop = desktop()?.deviceId;
  if (fromDesktop) return fromDesktop;
  let id = localStorage.getItem(DEVICE_KEY);
  if (!id) {
    id = crypto.randomUUID();
    localStorage.setItem(DEVICE_KEY, id);
  }
  return id;
}

export function getDeviceName() {
  const named = desktop()?.deviceName;
  if (named) return named;
  const ua = navigator.userAgent;
  let browser = "Browser";
  if (/Edg\//.test(ua)) browser = "Edge";
  else if (/Chrome\//.test(ua) && !/Edg\//.test(ua)) browser = "Chrome";
  else if (/Firefox\//.test(ua)) browser = "Firefox";
  else if (/Safari\//.test(ua)) browser = "Safari";
  let os = "Unknown OS";
  if (/Windows/i.test(ua)) os = "Windows";
  else if (/Mac OS X|Macintosh/i.test(ua)) os = "macOS";
  else if (/Linux/i.test(ua)) os = "Linux";
  else if (/Android/i.test(ua)) os = "Android";
  else if (/iPhone|iPad/i.test(ua)) os = "iOS";
  return `${browser} on ${os}`;
}

export function deviceShortLabel(name: string) {
  if (!name) return "Unknown device";
  if (/^device:/i.test(name) || /^[0-9a-f-]{36}$/i.test(name)) return "Trusted device";
  return name;
}

/** Drops this browser's device id so the next login is treated as a new device. */
export function forgetLocalDevice() {
  setToken(null);
  localStorage.removeItem(DEVICE_KEY);
  localStorage.removeItem(TOKEN_KEY);
}

type GqlError = { message: string };

function graphqlOpts(token: string | null, extraHeaders?: Record<string, string>): RequestInit {
  const url = graphqlUrl();
  const cross = /^https?:\/\//i.test(url);
  return {
    method: "POST",
    credentials: cross ? "omit" : "include",
    headers: {
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...extraHeaders,
    },
  };
}

async function readGraphQL(res: Response): Promise<{ data?: unknown; errors?: GqlError[] }> {
  const text = await res.text();
  if (!text) {
    throw new Error(res.ok ? "Empty response from API" : `API ${res.status}`);
  }
  try {
    return JSON.parse(text) as { data?: unknown; errors?: GqlError[] };
  } catch {
    throw new Error(res.ok ? "API did not return JSON" : `API ${res.status}: ${text.slice(0, 180)}`);
  }
}

export async function gql<T>(
  query: string,
  variables?: Record<string, unknown>
): Promise<T> {
  const token = getToken();
  let res: Response;
  try {
    res = await fetch(graphqlUrl(), {
      ...graphqlOpts(token, { "Content-Type": "application/json" }),
      body: JSON.stringify({ query, variables }),
    });
  } catch {
    throw new Error("Cannot reach API at " + graphqlUrl() + ". Start it with: go run ./cmd/api");
  }
  const json = await readGraphQL(res);
  if (json.errors?.length) {
    const msg = json.errors.map((e) => e.message).join("; ");
    if (!onLoginPage() && /unauthorized/i.test(msg)) {
      setToken(null);
      goLogin();
    }
    const data = json.data as Record<string, unknown> | null | undefined;
    if (data && Object.values(data).some((v) => v != null)) {
      return data as T;
    }
    throw new Error(msg);
  }
  if (!json.data) {
    throw new Error(res.ok ? "Empty GraphQL data" : `API ${res.status}`);
  }
  return json.data as T;
}

export async function uploadProjectFile(projectId: string, file: File): Promise<ProjectFile> {
  const token = getToken();
  const operations = JSON.stringify({
    query: `mutation ($projectId: ID!, $file: Upload!) {
      uploadFile(projectId: $projectId, file: $file) { name mimeType uploadedAt }
    }`,
    variables: { projectId, file: null },
  });
  const form = new FormData();
  form.append("operations", operations);
  form.append("map", JSON.stringify({ "0": ["variables.file"] }));
  form.append("0", file);
  let res: Response;
  try {
    res = await fetch(graphqlUrl(), {
      ...graphqlOpts(token),
      body: form,
    });
  } catch {
    throw new Error("Cannot reach API at " + graphqlUrl() + ". Start it with: go run ./cmd/api");
  }
  const json = await readGraphQL(res);
  if (json.errors?.length) {
    throw new Error(json.errors.map((e) => e.message).join("; "));
  }
  const data = json.data as { uploadFile?: ProjectFile } | undefined;
  if (!data?.uploadFile) throw new Error("Upload failed");
  return data.uploadFile;
}

export type TrustedDevice = { id: string; name: string; createdAt: string; lastSeenAt: string };
export type ConsentRecord = { purpose: string; accepted: boolean; createdAt: string };
export type User = {
  id: string;
  email: string;
  displayName?: string;
  tier: string;
  mfaEnabled: boolean;
  orgId?: string | null;
  orgName?: string | null;
  orgRole?: string | null;
  trustedDevices?: TrustedDevice[];
  ssoLinked?: boolean;
  disabled?: boolean;
  createdAt?: string;
  notifyEmail?: boolean;
  notifyInvestigations?: boolean;
  notifyMarketing?: boolean;
  shareUsage?: boolean;
};
export type OrgMember = { email: string; role: string; userId?: string | null };
export type Organization = { id: string; name: string; members: OrgMember[] };
export type ProjectFile = { name: string; mimeType: string; uploadedAt: string };
export type Project = {
  id: string;
  workspaceId: string;
  name: string;
  isSample: boolean;
  files: ProjectFile[];
  investigations?: Investigation[];
};
export type Workspace = { id: string; name: string; projects: Project[] };
export type PlanStep = { order: number; description: string; status: string };
export type Evidence = { file: string; lines: string; excerpt: string };
export type FastResult = { summary: string; incidentType: string; confidence: number };
export type DeepResult = { rootCause: string; confidence: number; evidence: Evidence[]; suggestedFix: string };
export type CouncilModel = { role: string; hypothesis: string; confidence: number; evidence: Evidence[]; model?: string | null };
export type Disagreement = { topic: string; investigator: string; challenger: string };
export type CouncilResult = {
  models: CouncilModel[];
  agreements: string[];
  disagreements: Disagreement[];
  finalJudgment: { mostLikelyCause: string; confidence: number; recommendedAction: string };
  aggregation: string;
  needsReview: boolean;
  aggregationNote: string;
};
export type Investigation = {
  id: string;
  projectId: string;
  prompt: string;
  status: string;
  plan: PlanStep[];
  filesAccessed?: string[];
  fastResult: FastResult | null;
  deepResult: DeepResult | null;
  councilResult: CouncilResult | null;
  errorMessage: string | null;
  modelAName?: string | null;
  modelBName?: string | null;
  modelCName?: string | null;
  escalationReason?: string | null;
  executionMode?: string | null;
  fallbackStages?: string[];
  createdAt: string;
};
export type ModelConfig = {
  workspaceId: string;
  modelAProvider: string;
  modelAName: string;
  modelBProvider: string;
  modelBName: string;
  modelCProvider: string;
  modelCName: string;
  temperature: number;
  maxTokens: number;
  investigatorPrompt: string;
  challengerPrompt: string;
  judgePrompt: string;
  activeSlot: string;
};
export type LLMOpsMetrics = {
  totalInvestigations: number;
  completed: number;
  failed: number;
  avgConfidence: number;
  avgDurationSec: number;
  workerSlots: number;
  busySlots: number;
  modelAName: string;
  modelBName: string;
  ollamaReachable: boolean;
  ollamaModels: string[];
  incidentModelReady: boolean;
  adapterOnDisk: boolean;
  topCauses: string[];
};
export type FineTuneJob = {
  id: string;
  workspaceId: string;
  status: string;
  adapterPath: string;
  error?: string | null;
  createdAt: string;
};
export type AuditLog = { id: string; action: string; resource: string; createdAt: string };
export type Entitlements = {
  tier: string;
  maxProjects: number;
  maxInvestigationsPerMonth: number;
  investigationsUsed: number;
  deepAnalysis: boolean;
  council: boolean;
  generate: boolean;
  evaluate: boolean;
  gitMcp: boolean;
  modelSelection: boolean;
  teamWorkspace: boolean;
  billingConfigured: boolean;
  ssoEnabled: boolean;
  demoUpgrade: boolean;
};
export type AuthFeatures = { ssoEnabled: boolean; billingEnabled: boolean; demoUpgrade: boolean };
export type GitRepo = { url: string; branch: string; head: string; connected: boolean };
export type GitBlameLine = { line: number; sha: string; author: string; summary: string };
export type GitCommit = { sha: string; author: string; date: string; message: string };
export type MetricDelta = { name: string; before: number; after: number; delta: number };
export type Generation = {
  id: string;
  projectId: string;
  investigationId?: string | null;
  prompt: string;
  fileName: string;
  rowCount: number;
  schemaNote: string;
  qualityNotes: string;
  confidence: number;
  status: string;
  error?: string | null;
  createdAt: string;
};
export type Evaluation = {
  id: string;
  projectId: string;
  investigationId?: string | null;
  generationId?: string | null;
  summary: string;
  recommendation: string;
  confidence: number;
  metrics: MetricDelta[];
  status: string;
  error?: string | null;
  createdAt: string;
};

export function apiOrigin() {
  const g = graphqlUrl();
  if (g.startsWith("http")) return g.replace(/\/graphql\/?$/, "");
  return "";
}

export function onLoginPage() {
  const hash = window.location.hash || "";
  if (hash.includes("/login")) return true;
  return window.location.pathname.startsWith("/login");
}

export function isDesktopShell() {
  return Boolean(desktop());
}

export function goLogin() {
  if (isDesktopShell() || window.location.protocol === "file:") {
    window.location.hash = "#/login";
    return;
  }
  window.location.replace("/login");
}

export const GEN_FIELDS = `
  id projectId investigationId prompt fileName rowCount schemaNote qualityNotes confidence status error createdAt
`;
export const EVAL_FIELDS = `
  id projectId investigationId generationId summary recommendation confidence status error createdAt
  metrics { name before after delta }
`;

/** Investigation GraphQL selection used by /investigation/:id polling. Includes Council aggregation fields — no parallel badge/state enums. */
export const INV_FIELDS = `
  id projectId prompt status createdAt errorMessage filesAccessed modelAName modelBName modelCName
  escalationReason executionMode fallbackStages
  plan { order description status }
  fastResult { summary incidentType confidence }
  deepResult { rootCause confidence suggestedFix evidence { file lines excerpt } }
  councilResult {
    aggregation needsReview aggregationNote
    agreements
    models { role hypothesis confidence model evidence { file lines excerpt } }
    disagreements { topic investigator challenger }
    finalJudgment { mostLikelyCause confidence recommendedAction }
  }
`;
