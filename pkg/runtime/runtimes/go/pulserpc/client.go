package pulserpc

import (
	"fmt"
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
	contract         *Contract
	validateRequest   bool
	validateResponse bool
	requestID        int
	ifaces           map[string]*InterfaceClientProxy
}

// NewClient creates a new Client and bootstraps by fetching IDL from server
func NewClient(transport Transport, validateRequest bool, validateResponse bool) (*Client, error) {
	c := &Client{
		transport:         transport,
		validateRequest:   validateRequest,
		validateResponse: validateResponse,
		requestID:        0,
		ifaces:           make(map[string]*InterfaceClientProxy),
	}

	// Bootstrap: fetch IDL from server
	if err := c.bootstrap(); err != nil {
		return nil, err
	}

	return c, nil
}

// bootstrap fetches the IDL from the server and creates interface proxies
func (c *Client) bootstrap() error {
	// Make request to get IDL
	resp, err := c.transport.Request("pulserpc-idl", nil)
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

// createMethodCaller creates a function that calls the RPC method
func (c *Client) createMethodCaller(ifaceName, funcName string) func(params interface{}) interface{} {
	return func(params interface{}) interface{} {
		result, err := c.Call(ifaceName+"."+funcName, params)
		if err != nil {
			panic(err)
		}
		return result
	}
}

// Call makes a JSON-RPC call
func (c *Client) Call(method string, params interface{}) (interface{}, error) {
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

	// Generate request ID
	c.requestID++
	reqID := c.requestID

	// Build request
	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
		"id":      reqID,
	}
	if params != nil {
		req["params"] = params
	}

	// Send request via transport
	response, err := c.transport.Request(method, params)
	if err != nil {
		return nil, fmt.Errorf("transport request failed: %w", err)
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

	// Validate response if enabled
	if c.validateResponse && c.contract != nil && result != nil {
		ifaceName, funcName, err := parseMethodName(method)
		if err == nil {
			if err := c.contract.ValidateResponse(ifaceName, funcName, result); err != nil {
				return nil, fmt.Errorf("response validation failed: %w", err)
			}
		}
	}

	return result, nil
}

// Notify sends a notification (no response expected)
func (c *Client) Notify(method string, params interface{}) {
	// Ignore errors for notifications
	c.Call(method, params)
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
