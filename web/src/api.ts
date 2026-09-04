const TOKEN_KEY = "azula_token";
const DEVICE_KEY = "azula_device";

function desktop() {
  return window.azulaDesktop;
}

export function graphqlUrl() {
  return desktop()?.graphqlUrl || "/graphql";
}

export function getToken() {
  return localStorage.getItem(TOKEN_KEY);
}

export function setToken(token: string | null) {
  if (token) localStorage.setItem(TOKEN_KEY, token);
  else localStorage.removeItem(TOKEN_KEY);
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
  const plat = navigator.platform || "web";
  return `browser · ${plat}`;
}

/** Drops this browser's device id so the next login is treated as a new device. */
export function forgetLocalDevice() {
  localStorage.removeItem(DEVICE_KEY);
  localStorage.removeItem(TOKEN_KEY);
}

type GqlError = { message: string };

export async function gql<T>(
  query: string,
  variables?: Record<string, unknown>
): Promise<T> {
  const token = getToken();
  const res = await fetch(graphqlUrl(), {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: JSON.stringify({ query, variables }),
  });
  const json = await res.json();
  if (json.errors?.length) {
    throw new Error((json.errors as GqlError[]).map((e) => e.message).join("; "));
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
  const res = await fetch(graphqlUrl(), {
    method: "POST",
    headers: token ? { Authorization: `Bearer ${token}` } : {},
    body: form,
  });
  const json = await res.json();
  if (json.errors?.length) {
    throw new Error((json.errors as GqlError[]).map((e: GqlError) => e.message).join("; "));
  }
  return json.data.uploadFile as ProjectFile;
}

export type TrustedDevice = { id: string; name: string; createdAt: string; lastSeenAt: string };
export type ConsentRecord = { purpose: string; accepted: boolean; createdAt: string };
export type User = {
  id: string;
  email: string;
  tier: string;
  mfaEnabled: boolean;
  orgId?: string | null;
  orgName?: string | null;
  orgRole?: string | null;
  trustedDevices?: TrustedDevice[];
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
export type CouncilModel = { role: string; hypothesis: string; confidence: number; evidence: Evidence[] };
export type Disagreement = { topic: string; investigator: string; challenger: string };
export type CouncilResult = {
  models: CouncilModel[];
  agreements: string[];
  disagreements: Disagreement[];
  finalJudgment: { mostLikelyCause: string; confidence: number; recommendedAction: string };
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
  createdAt: string;
};
export type ModelConfig = {
  workspaceId: string;
  modelAProvider: string;
  modelAName: string;
  modelBProvider: string;
  modelBName: string;
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

export const INV_FIELDS = `
  id projectId prompt status createdAt errorMessage filesAccessed modelAName modelBName
  plan { order description status }
  fastResult { summary incidentType confidence }
  deepResult { rootCause confidence suggestedFix evidence { file lines excerpt } }
  councilResult {
    agreements
    models { role hypothesis confidence evidence { file lines excerpt } }
    disagreements { topic investigator challenger }
    finalJudgment { mostLikelyCause confidence recommendedAction }
  }
`;
