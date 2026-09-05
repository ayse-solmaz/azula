package httpx

import (
	"strings"
	"testing"
)

func TestRestoreCamelFields(t *testing.T) {
	in := `query { me { trusteddevices { id lastseenat } displayname } llmopsmetrics(workspaceid: $id) { avgdurationsec ollamareachable } investigation { executionmode fallbackstages } }`
	got := restoreCamelFields(in)
	for _, part := range []string{"trustedDevices", "lastSeenAt", "llmOpsMetrics", "workspaceId", "avgDurationSec", "ollamaReachable", "displayName", "executionMode", "fallbackStages"} {
		if !strings.Contains(got, part) {
			t.Fatalf("missing %s in %s", part, got)
		}
	}
}
