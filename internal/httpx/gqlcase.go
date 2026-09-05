package httpx

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var camelFields = func() [][2]string {
	names := []string{
		"trustedDevices", "avgDurationSec", "ollamaReachable", "ollamaModels",
		"incidentModelReady", "adapterOnDisk", "topCauses", "totalInvestigations",
		"avgConfidence", "workerSlots", "busySlots", "modelAName", "modelBName",
		"modelAProvider", "modelBProvider", "maxTokens", "investigatorPrompt",
		"challengerPrompt", "judgePrompt", "activeSlot", "workspaceId", "projectId",
		"deviceId", "deviceName", "deviceOtp", "mfaCode", "mfaEnabled", "fileName",
		"lastSeenAt", "createdAt", "uploadedAt", "mimeType", "isSample", "orgId",
		"orgName", "orgRole", "userId", "otpauthUrl", "mfaRequired", "newDevice",
		"ephemeralCode", "adapterPath", "incidentType", "rootCause", "suggestedFix",
		"mostLikelyCause", "recommendedAction", "finalJudgment", "filesAccessed",
		"fastResult", "deepResult", "councilResult", "errorMessage", "myOrganization",
		"myConsent", "auditLogs", "enrollMfa", "deleteAccount", "exportMyData",
		"llmOpsMetrics", "modelConfig", "fineTuneJobs", "startInvestigation",
		"cancelInvestigation", "logout",
		"revokeTrustedDevice", "attachIncidentModel", "startFineTuneJob",
		"updateModelConfig", "createOrganization", "inviteOrgMember", "recordConsent",
		"fileContent", "fileVersions", "fileVersionContent", "swapFileVersion",
		"enableMfa", "disableMfa",
	}
	sort.Slice(names, func(i, j int) bool { return len(names[i]) > len(names[j]) })
	out := make([][2]string, 0, len(names))
	for _, n := range names {
		low := strings.ToLower(n)
		if low == n {
			continue
		}
		out = append(out, [2]string{low, n})
	}
	return out
}()

var ident = regexp.MustCompile(`\b[a-z][a-z0-9]*\b`)

func restoreCamelFields(query string) string {
	return ident.ReplaceAllStringFunc(query, func(word string) string {
		for _, pair := range camelFields {
			if word == pair[0] {
				return pair[1]
			}
		}
		return word
	})
}

// RestoreGraphQLCamelCase rewrites lowercase GraphQL field names back to schema camelCase.
func RestoreGraphQLCamelCase(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.Contains(r.Header.Get("Content-Type"), "json") {
			raw, err := io.ReadAll(r.Body)
			_ = r.Body.Close()
			if err == nil && len(raw) > 0 {
				var payload map[string]any
				if json.Unmarshal(raw, &payload) == nil {
					if q, ok := payload["query"].(string); ok {
						payload["query"] = restoreCamelFields(q)
						if b, mErr := json.Marshal(payload); mErr == nil {
							raw = b
						}
					}
				}
			}
			r.Body = io.NopCloser(bytes.NewReader(raw))
			r.ContentLength = int64(len(raw))
			r.Header.Set("Content-Length", strconv.Itoa(len(raw)))
		}
		next.ServeHTTP(w, r)
	})
}
