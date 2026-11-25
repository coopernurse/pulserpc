package pulserpc

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"
)

// InterfaceClientProxy represents a dynamic proxy for an interface
type InterfaceClientProxy struct {
	client    *Client
	iface     *Interface
	ifaceName string
	methods   map[string]func(params interface{}) interface{}
}

// Client is a JSON-RPC 2.0 client with automatic interface discovery
type Client struct {
	transport         Transport
	contract          *Contract
	localIDL          interface{}
	validateRequest   bool
	validateResponse  bool
	requestID         uint64
	ifaces            map[string]*InterfaceClientProxy
	auditor           ContractAuditor
	verifyOnBootstrap bool
}

// NewClient creates a new Client and bootstraps by fetching IDL from server.
// Bootstrap uses context.Background(); per-call contexts are supplied to Call
// and Notify.
func NewClient(transport Transport, validateRequest bool, validateResponse bool, opts ...ClientOption) (*Client, error) {
	c := &Client{
		transport:        transport,
		validateRequest:  validateRequest,
		validateResponse: validateResponse,
		requestID:        0,
		ifaces:           make(map[string]*InterfaceClientProxy),
	}

	for _, opt := range opts {
		opt(c)
	}

	// Bootstrap: fetch IDL from server
	if err := c.bootstrap(context.Background()); err != nil {
		return nil, err
	}

	// If VerifyOnBootstrap is set, perform verification immediately after bootstrap
	if c.verifyOnBootstrap {
		c.VerifyCompatibility(context.Background())
	}

	return c, nil
}

// bootstrap fetches the IDL from the server and creates interface proxies
func (c *Client) bootstrap(ctx context.Context) error {
	// Make request to get IDL
	resp, err := c.transport.Request(ctx, "pulserpc-idl", nil)
	if err != nil {
		return fmt.Errorf("failed to fetch IDL: %w", err)
	}

	// Check for error
	if errObj, ok := resp["error"].(map[string]interface{}); ok {
		return fmt.Errorf("server returned error: %v", errObj["message"])
	}

	// Get IDL result
	idlJSON, ok := resp["result"]
	if !ok || idlJSON == nil {
		return fmt.Errorf("server returned empty IDL")
	}

	// Create contract
	c.contract = NewContract(idlJSON)

	// Create interface proxies
	for name, iface := range c.contract.Interfaces {
		proxy := &InterfaceClientProxy{
			client:    c,
			iface:     iface,
			ifaceName: name,
			methods:   make(map[string]func(params interface{}) interface{}),
		}

		// Create method callables for each function
		for funcName := range iface.Functions {
			proxy.methods[funcName] = c.createMethodCaller(name, funcName)
		}

		c.ifaces[name] = proxy
	}

	return nil
}

// createMethodCaller creates a function that calls the RPC method. The
// reflection-based proxy has no caller context, so it uses
// context.Background(); callers that need cancellation should use Client.Call
// or a generated typed client.
func (c *Client) createMethodCaller(ifaceName, funcName string) func(params interface{}) interface{} {
	return func(params interface{}) interface{} {
		result, err := c.Call(context.Background(), ifaceName+"."+funcName, params)
		if err != nil {
			panic(err)
		}
		return result
	}
}

// Call makes a JSON-RPC call with the supplied context. A nil context is
// treated as context.Background().
func (c *Client) Call(ctx context.Context, method string, params interface{}) (interface{}, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	// Validate request if enabled
	if c.validateRequest && c.contract != nil {
		ifaceName, funcName, err := parseMethodName(method)
		if err == nil {
			var paramList []interface{}
			if paramsMap, ok := params.(map[string]interface{}); ok {
				paramList = c.namedToPositional(ifaceName, funcName, paramsMap)
			} else if paramsList, ok := params.([]interface{}); ok {
				paramList = paramsList
			} else if params != nil {
				paramList = []interface{}{params}
			}

			if paramList != nil {
				if err := c.contract.ValidateRequest(ifaceName, funcName, paramList); err != nil {
					return nil, fmt.Errorf("request validation failed: %w", err)
				}
			}
		}
	}

	// Generate a unique request ID and hand it to the transport so the
	// serialized request and the expected response envelope agree. Atomic so
	// concurrent calls never share an id (R20).
	reqID := atomic.AddUint64(&c.requestID, 1)

	response, err := c.sendRequest(ctx, reqID, method, params)
	if err != nil {
		return nil, fmt.Errorf("transport request failed: %w", err)
	}

	// Always enforce the response-envelope contract here. Transports that
	// cannot carry a caller-supplied id are checked without the id comparison
	// (their request id is outside the client's control).
	var checkID interface{} = reqID
	if _, ok := c.transport.(RequestWithID); !ok {
		checkID = nil
	}
	if err := validateEnvelope(response, checkID); err != nil {
		return nil, err
	}

	// Check for error response
	if errObj, ok := response["error"].(map[string]interface{}); ok {
		code, _ := errObj["code"].(float64)
		message, _ := errObj["message"].(string)
		var data interface{}
		if d, ok := errObj["data"].(interface{}); ok {
			data = d
		}
		return nil, NewRPCErrorWithData(int(code), message, data)
	}

	// Get result
	result, ok := response["result"]
	if !ok {
		result = nil
	}

	// Validate response if enabled. A nil result is not an excuse to skip
	// validation: required return types must reject null.
	if c.validateResponse && c.contract != nil {
		ifaceName, funcName, err := parseMethodName(method)
		if err == nil {
			if err := c.contract.ValidateResponse(ifaceName, funcName, result); err != nil {
				return nil, fmt.Errorf("response validation failed: %w", err)
			}
		}
	}

	return result, nil
}

// sendRequest routes a request through a transport that can carry the caller's
// id (and omit it for notifications), falling back to the legacy Request
// method for custom transports.
func (c *Client) sendRequest(ctx context.Context, id interface{}, method string, params interface{}) (map[string]interface{}, error) {
	if transport, ok := c.transport.(RequestWithID); ok {
		return transport.RequestWithID(ctx, method, params, id)
	}
	return c.transport.Request(ctx, method, params)
}

// Notify sends a JSON-RPC notification: a request with no id and no response.
// A nil context is treated as context.Background().
func (c *Client) Notify(ctx context.Context, method string, params interface{}) {
	if ctx == nil {
		ctx = context.Background()
	}
	// Notifications have no id and no caller-visible result; transport errors
	// are intentionally ignored.
	_, _ = c.sendRequest(ctx, nil, method, params)
}

// GetInterface returns an interface proxy by name
func (c *Client) GetInterface(ifaceName string) *InterfaceClientProxy {
	if proxy, ok := c.ifaces[ifaceName]; ok {
		return proxy
	}
	return nil
}

// namedToPositional converts named params to positional using IDL signature
func (c *Client) namedToPositional(ifaceName, funcName string, namedParams map[string]interface{}) []interface{} {
	if c.contract == nil {
		return nil
	}

	iface := c.contract.GetInterface(ifaceName)
	if iface == nil {
		return nil
	}

	fn := iface.GetFunction(funcName)
	if fn == nil {
		return nil
	}

	// Get parameter definitions
	paramDefs, ok := fn["parameters"].([]interface{})
	if !ok {
		return nil
	}

	// Build positional list
	positional := make([]interface{}, len(paramDefs))
	for i, paramDef := range paramDefs {
		if pd, ok := paramDef.(map[string]interface{}); ok {
			if name, ok := pd["name"].(string); ok {
				if v, exists := namedParams[name]; exists {
					positional[i] = v
				}
			}
		}
	}

	return positional
}

// GetContract returns the contract (for advanced use)
func (c *Client) GetContract() *Contract {
	return c.contract
}

// CallMethod calls a method on an interface proxy
// This is a helper that uses reflection to invoke the method
func (p *InterfaceClientProxy) CallMethod(funcName string, params interface{}) interface{} {
	if method, ok := p.methods[funcName]; ok {
		return method(params)
	}
	panic(fmt.Sprintf("method not found: %s", funcName))
}

// GetMethod returns a callable function for the given method name
func (p *InterfaceClientProxy) GetMethod(funcName string) func(interface{}) interface{} {
	if method, ok := p.methods[funcName]; ok {
		return method
	}
	return nil
}

// ListMethods returns all available method names for this interface
func (p *InterfaceClientProxy) ListMethods() []string {
	methods := make([]string, 0, len(p.methods))
	for name := range p.methods {
		methods = append(methods, name)
	}
	return methods
}

// Call is a convenience method that calls a method directly using reflection
// It accepts positional arguments that are passed to the RPC method
func (p *InterfaceClientProxy) Call(funcName string, args ...interface{}) interface{} {
	method := p.GetMethod(funcName)
	if method == nil {
		panic(fmt.Sprintf("method not found: %s", funcName))
	}

	var params interface{}
	if len(args) == 0 {
		params = nil
	} else if len(args) == 1 {
		params = args[0]
	} else {
		params = args
	}

	return method(params)
}

// Invoke uses reflection to call a method by name with positional arguments
// This provides a more natural calling convention
func (p *InterfaceClientProxy) Invoke(funcName string, args ...interface{}) (interface{}, error) {
	defer func() {
		if r := recover(); r != nil {
			// Method not found, return error instead of panicking
		}
	}()

	return p.Call(funcName, args...), nil
}

type ClientOption func(*Client)

func WithAuditor(auditor ContractAuditor) ClientOption {
	return func(c *Client) {
		c.auditor = auditor
	}
}

func VerifyOnBootstrap() ClientOption {
	return func(c *Client) {
		c.verifyOnBootstrap = true
	}
}

func (c *Client) SetLocalIDL(idlJSON string) error {
	var idlParsed interface{}
	if err := json.Unmarshal([]byte(idlJSON), &idlParsed); err != nil {
		return fmt.Errorf("failed to parse local IDL JSON: %w", err)
	}
	c.localIDL = idlParsed
	return nil
}

func (c *Client) VerifyCompatibility(ctx context.Context) *VerificationResult {
	if ctx == nil {
		ctx = context.Background()
	}
	var clientIDL interface{}
	if c.localIDL != nil {
		clientIDL = c.localIDL
	} else {
		clientIDL = c.contract.idlParsed
	}

	serverIDL := c.contract.idlParsed

	var clientChecksum string
	if dict, ok := clientIDL.(map[string]interface{}); ok {
		clientChecksum, _ = dict["checksum"].(string)
	}

	var serverChecksum string
	if dict, ok := serverIDL.(map[string]interface{}); ok {
		serverChecksum, _ = dict["checksum"].(string)
	}

	deltas := DiffIDL(clientIDL, serverIDL)

	compatible := true
	for _, delta := range deltas {
		if delta.Severity == SeverityError {
			compatible = false
			break
		}
	}

	result := &VerificationResult{
		Compatible:     compatible,
		ServerChecksum: serverChecksum,
		ClientChecksum: clientChecksum,
		Deltas:         deltas,
		Timestamp:      time.Now(),
	}

	if c.auditor != nil {
		c.auditor.Audit(ctx, result)
	}

	return result
}
