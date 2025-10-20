package haf

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClientValidation(t *testing.T) {
	if _, err := NewClient("ftp://invalid", time.Second); err == nil {
		t.Fatalf("expected error for invalid scheme")
	}

	client, err := NewClient("http://example.com", -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.httpClient.Timeout != 30*time.Second {
		t.Fatalf("expected default timeout, got %v", client.httpClient.Timeout)
	}
}

func TestReputationRequest(t *testing.T) {
	var requestedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		response := map[string]any{
			"account":    "alice",
			"reputation": json.Number("123"),
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, time.Second)
	if err != nil {
		t.Fatalf("failed to create haf client: %v", err)
	}
	client.httpClient = server.Client()
	client.baseURL = server.URL

	result, err := client.Reputation("  alice  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Account != "alice" || result.Reputation != 123 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if requestedPath != "/reputation-api/accounts/alice/reputation" {
		t.Fatalf("unexpected request path: %s", requestedPath)
	}
}

func TestAccountBalances(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]any{
			"HIVE": "1.000 HIVE",
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, time.Second)
	if err != nil {
		t.Fatalf("failed to create haf client: %v", err)
	}
	client.httpClient = server.Client()
	client.baseURL = server.URL

	balances, err := client.AccountBalances("bob")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if balances["HIVE"].(string) != "1.000 HIVE" {
		t.Fatalf("unexpected balances: %#v", balances)
	}
}

func TestExtractInt64(t *testing.T) {
	cases := []struct {
		name   string
		input  any
		expect int64
		err    bool
	}{
		{"json number", json.Number("12"), 12, false},
		{"float", float64(34), 34, false},
		{"int", int(56), 56, false},
		{"string", "78", 78, false},
		{"nil", nil, 0, true},
		{"bad string", "zz", 0, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			value, err := extractInt64(tc.input)
			if tc.err {
				if err == nil {
					t.Fatalf("expected error but got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if value != tc.expect {
				t.Fatalf("unexpected value: %d", value)
			}
		})
	}
}

func TestRequestNilClient(t *testing.T) {
	var c *Client
	if _, err := c.request("path"); err == nil {
		t.Fatalf("expected error when client is nil")
	}
}

func TestRequestHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusTeapot)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, time.Second)
	if err != nil {
		t.Fatalf("failed to create haf client: %v", err)
	}
	client.httpClient = server.Client()
	client.baseURL = server.URL

	if _, err := client.request("test"); err == nil || err.Error() != fmt.Sprintf("haf request failed (%d): boom", http.StatusTeapot) {
		t.Fatalf("unexpected error: %v", err)
	}
}
