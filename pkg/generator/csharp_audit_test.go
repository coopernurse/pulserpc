package generator

import (
	"strings"
	"testing"
)

// The generated C# server adapter must suppress every response for a
// notification, including error envelopes (invariant M4). The core dispatcher
// keeps returning error envelopes; the wrapper drops them when there was no id.
func TestGeneratedCSharpServerSuppressesNotificationErrors(t *testing.T) {
	idl := parseTestIDL(t, "namespace test\ninterface Svc { Ping() string }\n")
	paths := NewCSharpNamespacePaths("", "")
	code := generateServerCs(idl, map[string]*NamespaceTypes{}, nil, nil, "{}", paths)

	for _, want := range []string{
		"private async Task<Dictionary<string, object?>?> HandleSingleRequest(",
		"private async Task<Dictionary<string, object?>?> HandleSingleRequestCore(",
		"bool isNotification = !requestJson.TryGetValue(\"id\", out var notificationId) || notificationId == null;",
		"if (isNotification && response != null && response.ContainsKey(\"error\"))",
	} {
		if !strings.Contains(code, want) {
			t.Errorf("generated C# server missing notification suppression: %q", want)
		}
	}
}
