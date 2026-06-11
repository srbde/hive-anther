package client

import (
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/srbde/hive-anther/crypto"
)

// Helper to generate a test WIF key
func generateTestWIF(t *testing.T) string {
	t.Helper()
	priv := make([]byte, 32)
	for i := range priv {
		priv[i] = byte(i + 1)
	}
	payload := append([]byte{0x80}, priv...)
	payload = append(payload, 0x01) // Compressed
	h1 := sha256.Sum256(payload)
	h2 := sha256.Sum256(h1[:])
	wifBytes := append(payload, h2[:4]...)
	return crypto.Base58Encode(wifBytes)
}

func TestClientExtensions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		method := payload["method"].(string)
		response := map[string]any{"jsonrpc": "2.0", "id": payload["id"]}

		switch method {
		case "database_api.get_config", "condenser_api.get_config":
			response["result"] = map[string]any{"HIVE_CHAIN_ID": "beeab0de00000000000000000000000000000000000000000000000000000000"}
		case "condenser_api.get_chain_properties":
			response["result"] = map[string]any{
				"account_creation_fee": "3.000 HIVE",
				"maximum_block_size":   131072,
				"hbd_interest_rate":    0,
			}
		case "condenser_api.get_current_median_history_price":
			response["result"] = map[string]any{
				"base":  "0.500 HBD",
				"quote": "1.000 HIVE",
			}
		case "condenser_api.get_accounts":
			response["result"] = []any{
				map[string]any{
					"name":         "alice",
					"voting_power": 10000.0,
					"voting_manabar": map[string]any{
						"current_mana":     "10000",
						"last_update_time": float64(time.Now().Add(-10 * time.Hour).Unix()),
					},
					"last_vote_time": "2026-05-25T00:00:00",
					"balance":        "100.000 HIVE",
					"hbd_balance":    "50.000 HBD",
					"vesting_shares": "10000.000000 VESTS",
					"created":        "2020-01-01T00:00:00",
				},
			}
		case "condenser_api.get_account_history":
			response["result"] = []any{
				[]any{
					0,
					map[string]any{
						"trx_id":       "tx123",
						"block":        100,
						"trx_in_block": 0,
						"op_in_trx":    0,
						"virtual_op":   false,
						"op": []any{
							"transfer",
							map[string]any{
								"from":   "alice",
								"to":     "bob",
								"amount": "1.000 HIVE",
								"memo":   "test",
							},
						},
					},
				},
			}
		case "condenser_api.get_vesting_delegations":
			response["result"] = []any{
				map[string]any{
					"id":                  1,
					"delegator":           "alice",
					"delegatee":           "bob",
					"vesting_shares":      "100.000000 VESTS",
					"min_delegation_time": "2026-05-25T10:00:00",
				},
			}
		case "condenser_api.get_block_header":
			response["result"] = map[string]any{
				"previous":                "0000006400000000000000000000000000000000",
				"timestamp":               "2026-05-25T10:00:00",
				"witness":                 "initminer",
				"transaction_merkle_root": "0000000000000000000000000000000000000000",
			}
		case "rc_api.get_rc_resource_params":
			response["result"] = map[string]any{"resource_names": []any{"history", "market"}}
		case "rc_api.get_rc_resource_pool":
			response["result"] = map[string]any{"pool": map[string]any{}}
		case "rc_api.find_rc_accounts":
			response["result"] = map[string]any{
				"rc_accounts": []any{
					map[string]any{
						"max_rc": 100000.0,
						"rc_manabar": map[string]any{
							"current_mana":     80000.0,
							"last_update_time": float64(time.Now().Add(-10 * time.Hour).Unix()),
						},
					},
				},
			}
		case "bridge.get_ranked_posts":
			response["result"] = []any{
				map[string]any{
					"author":   "alice",
					"permlink": "test-post",
					"title":    "Ranked Post",
				},
			}
		case "bridge.get_account_posts":
			response["result"] = []any{
				map[string]any{
					"author":   "alice",
					"permlink": "my-post",
					"title":    "Account Post",
				},
			}
		case "bridge.get_community":
			response["result"] = map[string]any{
				"name":  "hive-123456",
				"title": "Hive Community",
			}
		case "bridge.list_communities":
			response["result"] = []any{
				map[string]any{
					"name":  "hive-123456",
					"title": "Hive Community",
				},
			}
		case "bridge.account_notifications":
			response["result"] = []any{
				map[string]any{
					"type":   "reply",
					"author": "bob",
				},
			}
		case "bridge.unread_notifications":
			response["result"] = map[string]any{
				"unread": 1.0,
			}
		case "condenser_api.get_content_replies":
			response["result"] = []any{
				map[string]any{
					"author":   "bob",
					"permlink": "reply-permlink",
					"body":     "This is a reply.",
				},
			}
		case "bridge.get_discussion":
			response["result"] = map[string]any{
				"alice/test-post": map[string]any{
					"author":   "alice",
					"permlink": "test-post",
					"body":     "Original post.",
				},
				"bob/reply-permlink": map[string]any{
					"author":   "bob",
					"permlink": "reply-permlink",
					"body":     "This is a reply.",
				},
			}
		case "condenser_api.get_dynamic_global_properties":
			response["result"] = map[string]any{
				"head_block_number": 100.0,
				"head_block_id":     "0000006400000000000000000000000000000000",
				"time":              "2026-05-25T10:00:00",
			}
		case "block_api.get_block":
			response["result"] = map[string]any{
				"block": map[string]any{
					"previous": "0000006400000000000000000000000000000000",
				},
			}
		case "condenser_api.broadcast_transaction":
			response["result"] = map[string]any{"status": "ok"}
		default:
			http.Error(w, "unexpected method: "+method, http.StatusBadRequest)
			return
		}

		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	c := NewClient([]string{server.URL}, 30)

	// 1. Test Database API
	t.Run("GetConfig", func(t *testing.T) {
		cfg, err := c.GetConfig()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg["HIVE_CHAIN_ID"] != "beeab0de00000000000000000000000000000000000000000000000000000000" {
			t.Fatalf("unexpected HIVE_CHAIN_ID: %v", cfg["HIVE_CHAIN_ID"])
		}
	})

	t.Run("GetChainProperties", func(t *testing.T) {
		props, err := c.GetChainProperties()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if props.AccountCreationFee != "3.000 HIVE" || props.MaximumBlockSize != 131072 {
			t.Fatalf("unexpected properties: %+v", props)
		}
	})

	t.Run("GetCurrentMedianHistoryPrice", func(t *testing.T) {
		price, err := c.GetCurrentMedianHistoryPrice()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if price.Base != "0.500 HBD" || price.Quote != "1.000 HIVE" {
			t.Fatalf("unexpected price: %+v", price)
		}
	})

	t.Run("GetAccounts", func(t *testing.T) {
		accounts, err := c.GetAccounts([]string{"alice"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(accounts) != 1 || accounts[0].Name != "alice" {
			t.Fatalf("unexpected accounts data: %+v", accounts)
		}
	})

	t.Run("GetAccountHistory", func(t *testing.T) {
		history, err := c.GetAccountHistory("alice", -1, 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(history) != 1 || history[0].Seq != 0 || history[0].Op.TrxID != "tx123" {
			t.Fatalf("unexpected history: %+v", history)
		}

		// Validation error test
		_, err = c.GetAccountHistory("alice", -1, 1001)
		if err == nil {
			t.Fatalf("expected error for limit > 1000")
		}
	})

	t.Run("GetVestingDelegations", func(t *testing.T) {
		delegations, err := c.GetVestingDelegations("alice", "", 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(delegations) != 1 || delegations[0].Delegator != "alice" {
			t.Fatalf("unexpected delegations: %+v", delegations)
		}
	})

	t.Run("GetBlockHeader", func(t *testing.T) {
		header, err := c.GetBlockHeader(100)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if header.Witness != "initminer" || header.Previous != "0000006400000000000000000000000000000000" {
			t.Fatalf("unexpected header: %+v", header)
		}
	})

	// 2. Test RC API
	t.Run("GetRCParams", func(t *testing.T) {
		params, err := c.GetRCParams()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(params) == 0 {
			t.Fatalf("expected params to be non-empty")
		}
	})

	t.Run("GetRCPool", func(t *testing.T) {
		pool, err := c.GetRCPool()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if pool["pool"] == nil {
			t.Fatalf("expected pool field to be present")
		}
	})

	t.Run("GetRCMana", func(t *testing.T) {
		mana, err := c.GetRCMana("alice")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if mana.MaxMana != 100000 || mana.CurrentMana < 80000 {
			t.Fatalf("unexpected RC mana info: %+v", mana)
		}
	})

	t.Run("CalculateRCMana", func(t *testing.T) {
		accounts, err := c.GetAccounts([]string{"alice"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		rc := c.CalculateRCMana(accounts[0])
		if rc <= 0 {
			t.Fatalf("expected positive RC percentage: %f", rc)
		}
	})

	t.Run("CalculateVPMana", func(t *testing.T) {
		accounts, err := c.GetAccounts([]string{"alice"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		vp := c.CalculateVPMana(accounts[0])
		if vp != 100.0 {
			t.Fatalf("expected 100%% voting power, got %f", vp)
		}
	})

	// 3. Test Social API
	t.Run("GetRankedPosts", func(t *testing.T) {
		posts, err := c.GetRankedPosts("trending", "", "", 10, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(posts) != 1 || posts[0]["author"] != "alice" {
			t.Fatalf("unexpected posts: %+v", posts)
		}
	})

	t.Run("GetAccountPosts", func(t *testing.T) {
		posts, err := c.GetAccountPosts("posts", "alice", 10, "", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(posts) != 1 || posts[0]["author"] != "alice" {
			t.Fatalf("unexpected posts: %+v", posts)
		}
	})

	t.Run("GetCommunity", func(t *testing.T) {
		comm, err := c.GetCommunity("hive-123456")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if comm["name"] != "hive-123456" {
			t.Fatalf("unexpected community: %+v", comm)
		}
	})

	t.Run("ListCommunities", func(t *testing.T) {
		comms, err := c.ListCommunities("", 10, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(comms) != 1 || comms[0]["name"] != "hive-123456" {
			t.Fatalf("unexpected communities list: %+v", comms)
		}
	})

	t.Run("GetAccountNotifications", func(t *testing.T) {
		notifs, err := c.GetAccountNotifications("alice", 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(notifs) != 1 || notifs[0]["type"] != "reply" {
			t.Fatalf("unexpected notifications: %+v", notifs)
		}
	})

	t.Run("GetUnreadNotificationsCount", func(t *testing.T) {
		unread, err := c.GetUnreadNotificationsCount("alice")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if unread != 1 {
			t.Fatalf("expected 1 unread notification, got %d", unread)
		}
	})

	t.Run("GetUnreadNotifications", func(t *testing.T) {
		notifs, err := c.GetUnreadNotifications("alice", 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(notifs) != 1 || notifs[0]["type"] != "reply" {
			t.Fatalf("unexpected notifications: %+v", notifs)
		}
	})

	t.Run("GetContentReplies", func(t *testing.T) {
		replies, err := c.GetContentReplies("alice", "test-post")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(replies) != 1 || replies[0]["author"] != "bob" {
			t.Fatalf("unexpected replies: %+v", replies)
		}
	})

	t.Run("GetDiscussion", func(t *testing.T) {
		disc, err := c.GetDiscussion("alice", "test-post")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(disc) != 2 || disc["alice/test-post"] == nil || disc["bob/reply-permlink"] == nil {
			t.Fatalf("unexpected discussion: %+v", disc)
		}
	})

	// 4. Test Broadcast helpers
	testWIF := generateTestWIF(t)

	t.Run("BroadcastVote", func(t *testing.T) {
		resp, err := c.BroadcastVote("voter", "author", "permlink", 10000, testWIF)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		respMap, ok := resp.(map[string]any)
		if !ok || respMap["status"] != "ok" {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("BroadcastTransfer", func(t *testing.T) {
		resp, err := c.BroadcastTransfer("alice", "bob", "1.000 HIVE", "memo", testWIF)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		respMap, ok := resp.(map[string]any)
		if !ok || respMap["status"] != "ok" {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("BroadcastComment", func(t *testing.T) {
		resp, err := c.BroadcastComment("alice", "permlink", "parentAuth", "parentPerm", "title", "body", "{}", testWIF)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		respMap, ok := resp.(map[string]any)
		if !ok || respMap["status"] != "ok" {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("BroadcastCustomJSON", func(t *testing.T) {
		resp, err := c.BroadcastCustomJSON("follow", `["follow",{"follower":"alice"}]`, []string{"alice"}, testWIF)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		respMap, ok := resp.(map[string]any)
		if !ok || respMap["status"] != "ok" {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})
}
