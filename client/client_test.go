package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
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

func TestClientStickyFailover(t *testing.T) {
	c := NewClient([]string{"http://node1", "http://node2"}, 30)

	// Initially, it should return the first node as current (index 0)
	node1 := c.GetCurrentNode()
	if node1 != "http://node1" {
		t.Fatalf("expected http://node1, got %s", node1)
	}

	// Repeated calls to GetCurrentNode should remain sticky (still node 1)
	node2 := c.GetCurrentNode()
	if node2 != "http://node1" {
		t.Fatalf("expected http://node1, got %s", node2)
	}

	// Rotating should advance to the next node (node 2)
	next := c.RotateNode()
	if next != "http://node2" {
		t.Fatalf("expected http://node2, got %s", next)
	}

	// Now GetCurrentNode should return node 2
	curr := c.GetCurrentNode()
	if curr != "http://node2" {
		t.Fatalf("expected http://node2, got %s", curr)
	}

	// Rotating again should cycle back to node 1
	next2 := c.RotateNode()
	if next2 != "http://node1" {
		t.Fatalf("expected http://node1, got %s", next2)
	}
}

func TestCallStickyBehavior(t *testing.T) {
	var callCount1, callCount2 int

	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount1++
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"result":  map[string]any{"ok": true},
		})
	}))
	defer server1.Close()

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount2++
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"result":  map[string]any{"ok": true},
		})
	}))
	defer server2.Close()

	c := NewClient([]string{server1.URL, server2.URL}, 30)

	// First call should hit server1
	_, err := c.Call("test", "method", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Second call should ALSO hit server1 because of sticky logic
	_, err = c.Call("test", "method", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if callCount1 != 2 || callCount2 != 0 {
		t.Fatalf("expected 2 calls to server1 and 0 to server2, got %d and %d", callCount1, callCount2)
	}
}

func TestCallStickyFailover(t *testing.T) {
	var callCount1, callCount2 int

	// Server1 will fail (return 500 error / connection closed / transport error)
	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount1++
		// Close connection immediately to simulate transport error
		hj, ok := w.(http.Hijacker)
		if ok {
			conn, _, _ := hj.Hijack()
			_ = conn.Close()
			return
		}
		http.Error(w, "error", http.StatusInternalServerError)
	}))
	defer server1.Close()

	// Server2 will succeed
	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount2++
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"result":  map[string]any{"ok": true},
		})
	}))
	defer server2.Close()

	c := NewClient([]string{server1.URL, server2.URL}, 30)

	// Call should first try server1, fail, rotate to server2, and succeed.
	_, err := c.Call("test", "method", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if callCount1 != 1 || callCount2 != 1 {
		t.Fatalf("expected 1 call to server1 and 1 to server2, got %d and %d", callCount1, callCount2)
	}

	// Subsequent call should immediately hit server2 (since it is sticky now)
	_, err = c.Call("test", "method", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if callCount1 != 1 || callCount2 != 2 {
		t.Fatalf("expected server2 to be sticky, got call counts: server1=%d, server2=%d", callCount1, callCount2)
	}
}

func TestGetBlockRange(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		method := payload["method"].(string)
		response := map[string]any{"jsonrpc": "2.0", "id": payload["id"]}

		if method == "block_api.get_block_range" {
			params := payload["params"].(map[string]any)
			start := params["starting_block_num"].(float64)
			count := params["count"].(float64)

			blocks := []any{}
			for i := 0; i < int(count); i++ {
				blocks = append(blocks, map[string]any{
					"block_id":  fmt.Sprintf("block-id-%d", int(start)+i),
					"previous":  "prev-hash",
					"timestamp": "2026-05-24T12:00:00",
					"witness":   "initminer",
				})
			}
			response["result"] = map[string]any{
				"blocks": blocks,
			}
		} else {
			response["error"] = map[string]any{"message": "unknown method"}
		}

		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	c := NewClient([]string{server.URL}, 30)

	blocks, err := c.GetBlockRange(100, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(blocks) != 3 {
		t.Fatalf("expected 3 blocks, got %d", len(blocks))
	}

	if blocks[0].BlockID != "block-id-100" || blocks[1].BlockID != "block-id-101" || blocks[2].BlockID != "block-id-102" {
		t.Fatalf("unexpected block range response content")
	}

	// Test count = 0 validation
	if _, err := c.GetBlockRange(100, 0); err == nil || err.Error() != "block range count must be greater than 0" {
		t.Fatalf("expected error for count = 0, got: %v", err)
	}

	// Test count > 1000 validation
	if _, err := c.GetBlockRange(100, 1001); err == nil || err.Error() != "block range count cannot exceed 1000" {
		t.Fatalf("expected error for count > 1000, got: %v", err)
	}
}

func TestGetOpsInBlockRange(t *testing.T) {
	var concurrentCalls, maxConcurrentCalls int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cur := atomic.AddInt64(&concurrentCalls, 1)
		defer atomic.AddInt64(&concurrentCalls, -1)
		for {
			maxSoFar := atomic.LoadInt64(&maxConcurrentCalls)
			if cur <= maxSoFar || atomic.CompareAndSwapInt64(&maxConcurrentCalls, maxSoFar, cur) {
				break
			}
		}

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		method := payload["method"].(string)
		response := map[string]any{"jsonrpc": "2.0", "id": payload["id"]}

		if method == "condenser_api.get_ops_in_block" {
			params := payload["params"].([]any)
			blockNum := int(params[0].(float64))
			response["result"] = []any{
				map[string]any{
					"trx_id":       fmt.Sprintf("trx-%d", blockNum),
					"block":        blockNum,
					"trx_in_block": 0,
					"op_in_trx":    0,
					"virtual_op":   false,
					"op":           []any{"vote", map[string]any{"voter": "alice"}},
				},
			}
		} else {
			response["error"] = map[string]any{"message": "unknown method"}
		}

		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	c := NewClient([]string{server.URL}, 30)

	ops, err := c.GetOpsInBlockRange(100, 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ops) != 50 {
		t.Fatalf("expected 50 ops, got %d", len(ops))
	}
	for i, op := range ops {
		expectedBlock := uint32(100 + i)
		if op.Block != expectedBlock {
			t.Fatalf("expected ops in block order, got op[%d].Block = %d, want %d", i, op.Block, expectedBlock)
		}
	}

	if atomic.LoadInt64(&maxConcurrentCalls) <= 1 {
		t.Fatalf("expected concurrent calls, max observed concurrency was %d", maxConcurrentCalls)
	}
	if atomic.LoadInt64(&maxConcurrentCalls) > opsInBlockRangeConcurrency {
		t.Fatalf("exceeded concurrency cap: max observed concurrency was %d", maxConcurrentCalls)
	}

	// Test count = 0 validation
	if _, err := c.GetOpsInBlockRange(100, 0); err == nil || err.Error() != "block range count must be greater than 0" {
		t.Fatalf("expected error for count = 0, got: %v", err)
	}

	// Test count > 1000 validation
	if _, err := c.GetOpsInBlockRange(100, 1001); err == nil || err.Error() != "block range count cannot exceed 1000" {
		t.Fatalf("expected error for count > 1000, got: %v", err)
	}
}

func TestClientFailoverHttpStatusAndJsonRpcCode(t *testing.T) {
	var callCount1, callCount2 int

	// Server 1 returns 502 Bad Gateway
	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount1++
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("502 Bad Gateway"))
	}))
	defer server1.Close()

	// Server 2 returns JSON-RPC -32603 Internal Error first time, succeeds second time
	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount2++
		if callCount2 == 1 {
			response := map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"error": map[string]any{
					"code":    -32603,
					"message": "Internal error",
				},
			}
			_ = json.NewEncoder(w).Encode(response)
			return
		}

		response := map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"result":  "success",
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server2.Close()

	c := NewClient([]string{server1.URL, server2.URL}, 30)

	res, err := c.Call("test", "method", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res != "success" {
		t.Fatalf("expected result 'success', got %v", res)
	}

	if callCount1 != 2 {
		t.Fatalf("expected 2 calls to server1, got %d", callCount1)
	}

	if callCount2 != 2 {
		t.Fatalf("expected 2 calls to server2, got %d", callCount2)
	}
}
