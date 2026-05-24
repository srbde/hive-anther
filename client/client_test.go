package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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

func TestStreamBlocksAndOps(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		method := payload["method"].(string)
		response := map[string]any{"jsonrpc": "2.0", "id": payload["id"]}

		switch method {
		case "condenser_api.get_dynamic_global_properties":
			response["result"] = map[string]any{
				"head_block_number":           10.0,
				"last_irreversible_block_num": 8.0,
				"time":                        "2026-05-24T12:00:00",
			}
		case "condenser_api.get_block":
			params := payload["params"].([]any)
			blockNum := params[0].(float64)
			response["result"] = map[string]any{
				"block_id":  fmt.Sprintf("block-id-%d", int(blockNum)),
				"previous":  "prev-hash",
				"timestamp": "2026-05-24T12:00:00",
				"witness":   "initminer",
			}
		case "condenser_api.get_ops_in_block":
			params := payload["params"].([]any)
			blockNum := params[0].(float64)
			response["result"] = []any{
				map[string]any{
					"trx_id": "tx-123",
					"block":  blockNum,
					"op": []any{
						"transfer",
						map[string]any{
							"from":   "alice",
							"to":     "bob",
							"amount": "1.000 HIVE",
						},
					},
				},
				map[string]any{
					"trx_id": "tx-456",
					"block":  blockNum,
					"op": []any{
						"vote",
						map[string]any{
							"voter": "charlie",
						},
					},
				},
			}
		default:
			response["error"] = map[string]any{"message": "unknown method"}
		}

		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	c := NewClient([]string{server.URL}, 5)

	t.Run("StreamBlocks", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		blocks, errs := c.StreamBlocks(ctx, 8, Irreversible)

		select {
		case block := <-blocks:
			if block == nil || block.BlockID != "block-id-8" {
				t.Fatalf("expected block 8, got %#v", block)
			}
		case err := <-errs:
			t.Fatalf("unexpected error: %v", err)
		case <-ctx.Done():
			t.Fatalf("timeout waiting for block")
		}
	})

	t.Run("StreamOperations", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		ops, errs := c.StreamOperations(ctx, 8, Irreversible, []string{"transfer"})

		select {
		case op := <-ops:
			if op == nil || op.TrxID != "tx-123" {
				t.Fatalf("expected op trx_id tx-123, got %#v", op)
			}
			if len(op.Op) == 0 || op.Op[0] != "transfer" {
				t.Fatalf("expected transfer op, got %#v", op.Op)
			}
		case err := <-errs:
			t.Fatalf("unexpected error: %v", err)
		case <-ctx.Done():
			t.Fatalf("timeout waiting for operation")
		}
	})
}
