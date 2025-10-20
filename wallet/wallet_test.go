package wallet

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/thecrazygm/nectar-go/client"
	"github.com/thecrazygm/nectar-go/transaction"
)

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

func TestWalletAddAndGetKey(t *testing.T) {
	w := NewWallet()
	testWIF := generateTestWIF(t)

	if err := w.AddKey("alice", "posting", testWIF); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !w.HasKey("alice", "posting") {
		t.Fatalf("expected key to be present")
	}
	key, err := w.GetKey("alice", "posting")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != testWIF {
		t.Fatalf("unexpected key returned")
	}
}

func TestWalletAddKeyValidation(t *testing.T) {
	w := NewWallet()
	testWIF := generateTestWIF(t)

	t.Run("invalid role", func(t *testing.T) {
		if err := w.AddKey("alice", "owner", testWIF); err == nil {
			t.Fatalf("expected error for invalid role")
		}
	})

	t.Run("empty account", func(t *testing.T) {
		if err := w.AddKey("", "posting", testWIF); err == nil {
			t.Fatalf("expected error for empty account")
		}
	})

	t.Run("invalid wif prefix", func(t *testing.T) {
		if err := w.AddKey("alice", "posting", "abc"); err == nil {
			t.Fatalf("expected error for invalid wif prefix")
		}
	})

	t.Run("invalid wif decoding", func(t *testing.T) {
		if err := w.AddKey("alice", "posting", "5aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"); err == nil {
			t.Fatalf("expected error for invalid wif value")
		}
	})
}

func TestWalletSign(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		method, _ := req["method"].(string)

		response := map[string]any{"jsonrpc": "2.0", "id": req["id"]}

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
		default:
			http.Error(w, "unexpected method", http.StatusBadRequest)
			return
		}

		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	api := client.NewClient([]string{server.URL}, 30)
	tx := transaction.NewTransaction(api)

	w := NewWallet()
	if err := w.AddKey("alice", "active", generateTestWIF(t)); err != nil {
		t.Fatalf("setup error: %v", err)
	}

	if err := w.Sign(tx, "alice", "active"); err != nil {
		t.Fatalf("unexpected sign error: %v", err)
	}

	if len(tx.Signatures) != 1 {
		t.Fatalf("expected one signature, got %d", len(tx.Signatures))
	}
}
