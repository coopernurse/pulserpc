package pulserpc

// ErrorPhase identifies which stage of request handling produced an error that
// is passed to ServerConfig.MapError. It lets an application distinguish a
// malformed parameter payload from a value that decoded but failed contract
// validation.
type ErrorPhase string

const (
	// ErrorPhaseDecodeParams covers params-shape, parameter-count, named-param
	// conversion, and unknown-request-field failures.
	ErrorPhaseDecodeParams ErrorPhase = "decode_params"
	// ErrorPhaseValidateParams covers IDL contract validation failures, such as
	// a wrong value type, a missing required struct field, or a bad enum value.
	ErrorPhaseValidateParams ErrorPhase = "validate_params"
)

// ParamsMode selects which JSON-RPC params forms a server accepts.
type ParamsMode int

const (
	// ParamsModeAny accepts positional (array) and named (object) params.
	ParamsModeAny ParamsMode = iota
	// ParamsModePositionalOnly rejects object params with -32600. Positional
	// params, absent params, and null params are accepted.
	ParamsModePositionalOnly
)

// ServerConfig controls the JSON-RPC policy enforced by the Go runtime server
// and the generated HTTP adapter.
//
// The zero value is intentionally strict about batching: AllowBatch is false
// and MaxRequestBodyBytes is 0, meaning the legacy 1 MiB default and legacy
// oversize error. Callers should start from DefaultServerConfig() (or set every
// field explicitly) so the defaults are the lenient v0.4.0 behavior.
type ServerConfig struct {
	// AllowBatch accepts a JSON array request body. When false, batch bodies
	// are rejected with -32600 before dispatch.
	AllowBatch bool
	// RequireNonNullID rejects requests with an absent or null id with -32600.
	// Such requests are notifications by default and receive no response.
	RequireNonNullID bool
	// ParamsMode selects which params forms are accepted. The zero value
	// (ParamsModeAny) accepts both.
	ParamsMode ParamsMode
	// MaxRequestBodyBytes caps the request body. A value <= 0 keeps the legacy
	// 1 MiB cap and the legacy -32600 "Request body too large" response; a
	// positive value uses that cap and reports oversize bodies as
	// -32700 "Parse error".
	MaxRequestBodyBytes int
	// RejectInvalidUTF8 rejects a body that is not valid UTF-8 with
	// -32700 "Parse error". When false, encoding/json replaces invalid bytes
	// with U+FFFD, matching v0.4.0.
	RejectInvalidUTF8 bool
	// RejectUnknownRequestFields rejects unknown keys in struct-typed request
	// params, recursively through pointers, slices, arrays, maps, and embedded
	// structs. Unknown top-level JSON-RPC envelope members are still ignored.
	RejectUnknownRequestFields bool
	// MapError maps argument decode and validation failures to an RPCError.
	// Returning nil falls back to the normal JSON-RPC error. Envelope protocol
	// errors (invalid jsonrpc/method/id, named-param rejection) are never
	// mapped.
	MapError func(phase ErrorPhase, err error) *RPCError
}

// DefaultServerConfig returns the lenient v0.4.0 policy: batches allowed,
// notifications allowed, both params forms accepted, the legacy 1 MiB body cap
// with its legacy oversize error, no UTF-8 or unknown-field rejection, and no
// error mapping.
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		AllowBatch:          true,
		MaxRequestBodyBytes: 0,
	}
}
