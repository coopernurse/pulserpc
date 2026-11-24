package generator

import (
	"strings"
	"testing"

	"github.com/coopernurse/pulserpc/pkg/parser"
)

// Generated Go interfaces, clients, and transports must put a context first and
// always expose an error result.
func TestGoGeneratedContextSignatures(t *testing.T) {
	out := wave6Generate(t, map[string]string{
		"root.pulse": "namespace ctx\n\ninterface Svc {\n  Ping(msg string) string\n}\n",
	}, "root.pulse", false)

	server := wave6Read(t, out+"/ctx/server.go")
	client := wave6Read(t, out+"/ctx/client.go")

	wave6Want(t, server,
		"type Svc interface",
		"Ping(ctx context.Context, msg string) (string, error)",
		"type HTTPMiddleware func(w http.ResponseWriter, r *http.Request) (context.Context, bool)",
		"func (s *PulseRPCServer) UseHTTP(mw HTTPMiddleware)",
		"func (s *PulseRPCServer) UseRPC(mw pulserpc.Middleware)",
		"func (s *PulseRPCServer) Register(interfaceName string, implementation interface{}) error",
		"func (s *PulseRPCServer) CallWithContext(ctx context.Context, request map[string]interface{}) map[string]interface{}",
		"func (s *PulseRPCServer) Call(request map[string]interface{}) map[string]interface{}",
		// HTTP middleware runs before the method check.
		"ctx := r.Context()",
		"for _, mw := range s.httpMiddleware {",
		"if !cont {",
		"s.rpcServer.CallWithContext(ctx, reqMap)",
		"s.rpcServer.CallWithContext(memberCtx, reqMap)",
	)
	wave6Want(t, client,
		"Ping(ctx context.Context, msg string) (string, error)",
		"func (c *SvcClient) Ping(ctx context.Context, msg string) (string, error)",
		"Call(ctx context.Context, method string, params []interface{}) (map[string]interface{}, error)",
		"Notify(ctx context.Context, method string, params []interface{}) error",
		"func (c *SvcClient) Notify(ctx context.Context, method string, params []interface{}) error",
		"func (c *SvcClient) ensureBootstrapVerification(ctx context.Context)",
		"http.NewRequestWithContext(ctx, \"POST\", t.baseURL, bytes.NewBuffer(jsonData))",
		"c.transport.Call(ctx, methodName, params)",
	)

	wave6Go(t, out, "build", "./...")
}

// The void path (no declared return type) emits error as the only result even
// though the current IDL grammar requires a return type.
func TestGoVoidMethodSignatureReturnsErrorOnly(t *testing.T) {
	iface := &parser.Interface{
		Name:      "Svc",
		Namespace: "ctx",
		Methods: []*parser.Method{{
			Name: "Notify",
			Parameters: []*parser.Parameter{{
				Name: "msg",
				Type: &parser.Type{BuiltIn: "string"},
			}},
		}},
	}
	structMap := map[string]*parser.Struct{}
	enumMap := map[string]*parser.Enum{}
	nsImports := map[string]string{}

	var stub strings.Builder
	writeInterfaceStubGo(&stub, iface, structMap, enumMap, "ctx", nsImports, "example.com/rpc", nil)
	if got := stub.String(); !strings.Contains(got, "Notify(ctx context.Context, msg string) error") {
		t.Errorf("void interface method is not error-only:\n%s", got)
	}

	var client strings.Builder
	writeClientMethodGo(&client, iface, iface.Methods[0], structMap, enumMap, "ctx", nsImports, "example.com/rpc")
	if got := client.String(); !strings.Contains(got, "func (c *SvcClient) Notify(ctx context.Context, msg string) error") {
		t.Errorf("void client method is not error-only:\n%s", got)
	}
}

// Generated test files use the same context-first, error-returning shape.
func TestGoGeneratedTestFilesUseContext(t *testing.T) {
	out := wave6Generate(t, map[string]string{
		"root.pulse": "namespace ctx\n\ninterface Svc {\n  Ping(msg string) string\n}\n",
	}, "root.pulse", true)

	testServer := wave6Read(t, out+"/cmd/test_server/main.go")
	testClient := wave6Read(t, out+"/cmd/test_client/main.go")

	wave6Want(t, testServer,
		"func (i *SvcImpl) Ping(ctx context.Context, msg string) (string, error)",
	)
	wave6Want(t, testClient,
		"result, err := svcClient.Ping(context.Background(), \"test\")",
		"func (divergentIDLTransport) Call(ctx context.Context, method string, params []interface{}) (map[string]interface{}, error)",
	)

	wave6Go(t, out, "build", "./...")
}
