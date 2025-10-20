package transaction

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/thecrazygm/nectar-go/client"
)

type dummyOp struct {
	name string
	data map[string]any
}

func (d dummyOp) ToDict() (string, map[string]any) {
	return d.name, d.data
}

func generateTestWIF(t *testing.T) string {
	t.Helper()
	priv := [32]byte{}
	for i := range priv {
		priv[i] = byte(i + 1)
	}
	key, _ := btcec.PrivKeyFromBytes(priv[:])
	wif, err := btcutil.NewWIF(key, &chaincfg.MainNetParams, false)
	if err != nil {
		t.Fatalf("failed to create test wif: %v", err)
	}
	return wif.String()
}

func TestAppendOpAndToDict(t *testing.T) {
	tx := NewTransaction(nil)
	op := dummyOp{name: "custom", data: map[string]any{"foo": "bar"}}
	tx.AppendOp(op)

	if len(tx.Operations) != 1 {
		t.Fatalf("expected one operation, got %d", len(tx.Operations))
	}

	dict := tx.toDict()
	ops, ok := dict["operations"].([]any)
	if !ok || len(ops) != 1 {
		t.Fatalf("unexpected operations payload: %#v", dict["operations"])
	}
	entry, ok := ops[0].([]any)
	if !ok || len(entry) != 2 {
		t.Fatalf("unexpected operation entry: %#v", ops[0])
	}
	if entry[0] != "custom" {
		t.Fatalf("unexpected operation name: %v", entry[0])
	}
	params, ok := entry[1].(map[string]any)
	if !ok || params["foo"] != "bar" {
		t.Fatalf("unexpected operation params: %#v", entry[1])
	}
}

func TestSignAndBroadcast(t *testing.T) {
	var broadcastCalled int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		method, _ := payload["method"].(string)
		response := map[string]any{"jsonrpc": "2.0", "id": payload["id"]}

		switch method {
		case "condenser_api.get_dynamic_global_properties":
			response["result"] = map[string]any{
				"head_block_number": 12345.0,
				"time":              "2025-01-01T00:00:00",
			}
		case "block_api.get_block":
			response["result"] = map[string]any{
				"block": map[string]any{
					"previous": "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20",
				},
			}
		case "condenser_api.get_transaction_hex":
			response["result"] = map[string]any{
				"hex": "deadbeefcafebabe",
			}
		case "condenser_api.broadcast_transaction_synchronous":
			atomic.AddInt64(&broadcastCalled, 1)
			params, _ := payload["params"].([]any)
			if len(params) != 1 {
				http.Error(w, "unexpected params", http.StatusBadRequest)
				return
			}
			txPayload, _ := params[0].(map[string]any)
			signatures, _ := txPayload["signatures"].([]any)
			if len(signatures) != 1 {
				http.Error(w, "missing signature", http.StatusBadRequest)
				return
			}
			response["result"] = map[string]any{"status": "ok"}
		default:
			http.Error(w, "unexpected method", http.StatusBadRequest)
			return
		}

		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	api := client.NewClient([]string{server.URL}, 30)
	tx := NewTransaction(api)
	tx.AppendOp(dummyOp{name: "custom", data: map[string]any{"foo": "bar"}})

	if err := tx.Sign(generateTestWIF(t)); err != nil {
		t.Fatalf("unexpected sign error: %v", err)
	}
	if len(tx.Signatures) != 1 {
		t.Fatalf("expected one signature, got %d", len(tx.Signatures))
	}

	result, err := tx.Broadcast()
	if err != nil {
		t.Fatalf("unexpected broadcast error: %v", err)
	}
	resMap, ok := result.(map[string]any)
	if !ok || resMap["status"] != "ok" {
		t.Fatalf("unexpected broadcast result: %#v", result)
	}

	if atomic.LoadInt64(&broadcastCalled) != 1 {
		t.Fatalf("expected broadcast to be called once, got %d", broadcastCalled)
	}
}

func TestBroadcastWithoutSignature(t *testing.T) {
	tx := NewTransaction(nil)
	if _, err := tx.Broadcast(); err == nil {
		t.Fatalf("expected error when broadcasting without signature")
	}
}
