package pulserpc

import (
	"sync"
	"testing"
)

// concurrentTransport records request ids safely so many goroutines can call
// Client.Call at once.
type concurrentTransport struct {
	mu  sync.Mutex
	ids []interface{}
}

func (t *concurrentTransport) Request(method string, params interface{}) (map[string]interface{}, error) {
	return t.RequestWithID(method, params, 1)
}

func (t *concurrentTransport) RequestWithID(method string, params interface{}, id interface{}) (map[string]interface{}, error) {
	t.mu.Lock()
	t.ids = append(t.ids, id)
	t.mu.Unlock()

	response := map[string]interface{}{"jsonrpc": "2.0", "result": nil}
	if id != nil {
		response["id"] = id
	}
	return response, nil
}

// R20: concurrent client calls must use distinct ids. The runtime client used a
// plain requestID++ before this change, which races and reuses ids.
func TestConcurrentCallsUseDistinctIDs(t *testing.T) {
	transport := &concurrentTransport{}
	client := &Client{transport: transport}

	const calls = 100
	var wg sync.WaitGroup
	errs := make(chan error, calls)
	for i := 0; i < calls; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := client.Call("A.add", []interface{}{1, 2}); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent call failed: %v", err)
	}

	if len(transport.ids) != calls {
		t.Fatalf("recorded %d ids, want %d", len(transport.ids), calls)
	}
	seen := make(map[uint64]bool, calls)
	for _, id := range transport.ids {
		numeric, ok := id.(uint64)
		if !ok {
			t.Fatalf("request id is not uint64: %#v", id)
		}
		if seen[numeric] {
			t.Fatalf("duplicate request id %d", numeric)
		}
		seen[numeric] = true
	}
}
