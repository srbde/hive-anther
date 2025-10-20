package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientGetNextNode(t *testing.T) {
	c := NewClient([]string{"http://node1", "http://node2"}, 30)

	node1 := c.GetNextNode()
	node2 := c.GetNextNode()
	node3 := c.GetNextNode()

	if node1 != "http://node1" || node2 != "http://node2" || node3 != "http://node1" {
		t.Fatalf("unexpected node rotation: %s %s %s", node1, node2, node3)
	}
}

func TestBuildPayload(t *testing.T) {
	c := NewClient([]string{"http://node"}, 30)

	payload, err := c.BuildPayload("condenser_api", "get_version", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if payload["params"].([]any) == nil {
		t.Fatalf("expected params to default to empty slice")
	}

	payload, err = c.BuildPayload("api", "method", map[string]any{"foo": "bar"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	params, ok := payload["params"].(map[string]any)
	if !ok || params["foo"] != "bar" {
		t.Fatalf("unexpected params value: %#v", payload["params"])
	}
}

func TestCall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		method := payload["method"].(string)
		response := map[string]any{"jsonrpc": "2.0", "id": payload["id"]}

		if method == "condenser_api.get_dynamic_global_properties" {
			response["result"] = map[string]any{"head_block_number": 1}
		} else {
			response["error"] = map[string]any{"message": "boom"}
		}

		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	c := NewClient([]string{server.URL}, 30)

	result, err := c.Call("condenser_api", "get_dynamic_global_properties", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	respMap, ok := result.(map[string]any)
	if !ok || respMap["head_block_number"].(float64) != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}

	if _, err := c.Call("other", "method", nil); err == nil {
		t.Fatalf("expected error for rpc error payload")
	}
}
