package httpx

import (
	"strings"
	"testing"
)

func TestRestoreCamelFields(t *testing.T) {
	in := `query { me { trusteddevices { id lastseenat } } llmopsmetrics(workspaceid: $id) { avgdurationsec ollamareachable } }`
	got := restoreCamelFields(in)
	for _, part := range []string{"trustedDevices", "lastSeenAt", "llmOpsMetrics", "workspaceId", "avgDurationSec", "ollamaReachable"} {
		if !strings.Contains(got, part) {
			t.Fatalf("missing %s in %s", part, got)
		}
	}
}
