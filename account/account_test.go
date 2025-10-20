package account

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/thecrazygm/nectar-go/client"
	"github.com/thecrazygm/nectar-go/haf"
)

func TestRefresh(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if payload["method"] != "condenser_api.get_accounts" {
			http.Error(w, "unexpected method", http.StatusBadRequest)
			return
		}

		response := map[string]any{
			"jsonrpc": "2.0",
			"id":      payload["id"],
			"result": []any{
				map[string]any{
					"balance": "10.000 HIVE",
				},
			},
		}

		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	api := client.NewClient([]string{server.URL}, 30)
	acc := NewAccount("alice", api)

	if err := acc.Refresh(); err != nil {
		t.Fatalf("unexpected refresh error: %v", err)
	}

	balance, ok := acc.Data["balance"].(string)
	if !ok || balance != "10.000 HIVE" {
		t.Fatalf("unexpected balance data: %#v", acc.Data["balance"])
	}
}

func TestRefreshWithoutAPI(t *testing.T) {
	acc := NewAccount("alice", nil)
	if err := acc.Refresh(); err == nil {
		t.Fatalf("expected error when API is nil")
	}
}

func TestGetReputationCaching(t *testing.T) {
	var calls int32
	hafServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&calls, 1)
		value := "123"
		if count > 1 {
			value = "456"
		}
		response := map[string]any{
			"account":    "alice",
			"reputation": value,
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer hafServer.Close()

	hafClient, err := haf.NewClient(hafServer.URL, time.Second)
	if err != nil {
		t.Fatalf("failed to create haf client: %v", err)
	}

	acc := NewAccount("alice", nil)
	acc.SetHAFClient(hafClient)

	rep1, err := acc.Reputation()
	if err != nil {
		t.Fatalf("unexpected reputation error: %v", err)
	}
	if rep1 != 123 {
		t.Fatalf("unexpected first reputation: %d", rep1)
	}

	rep2, err := acc.Reputation()
	if err != nil {
		t.Fatalf("unexpected cached reputation error: %v", err)
	}
	if rep2 != rep1 {
		t.Fatalf("expected cached reputation to match initial value: %d vs %d", rep1, rep2)
	}

	rep3, err := acc.GetReputation(true)
	if err != nil {
		t.Fatalf("unexpected refresh reputation error: %v", err)
	}
	if rep3 != 456 {
		t.Fatalf("unexpected refreshed reputation: %d", rep3)
	}

	if atomic.LoadInt32(&calls) != 2 {
		t.Fatalf("expected two HTTP calls, got %d", atomic.LoadInt32(&calls))
	}
}
