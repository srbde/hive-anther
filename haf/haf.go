package haf

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

var defaultAPIs = []string{
	"https://api.hive.blog",
	"https://api.syncad.com",
}

const (
	defaultTimeout = 30 * time.Second
	userAgent      = "nectarlite-go/0.0.1"
)

// Client is an HTTP client for interacting with Hive Account Framework (HAF) endpoints.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient constructs a new HAF client using the provided base API URL and timeout.
// When api is empty the first default HAF endpoint is used. A non-positive timeout
// results in the library default timeout of 30 seconds being applied.
func NewClient(api string, timeout time.Duration) (*Client, error) {
	trimmed := strings.TrimSpace(api)
	if trimmed == "" {
		trimmed = defaultAPIs[0]
	}

	if !strings.HasPrefix(trimmed, "http://") && !strings.HasPrefix(trimmed, "https://") {
		return nil, fmt.Errorf("invalid HAF API URL: %s", trimmed)
	}

	if timeout <= 0 {
		timeout = defaultTimeout
	}

	return &Client{
		baseURL:    strings.TrimRight(trimmed, "/"),
		httpClient: &http.Client{Timeout: timeout},
	}, nil
}

var (
	defaultClient     *Client
	defaultClientOnce sync.Once
	defaultClientErr  error
)

// DefaultClient returns a singleton HAF client configured with the default endpoint list.
func DefaultClient() (*Client, error) {
	defaultClientOnce.Do(func() {
		defaultClient, defaultClientErr = NewClient("", 0)
	})
	return defaultClient, defaultClientErr
}

// ReputationResult represents the structured reputation payload returned by HAF.
type ReputationResult struct {
	Account    string
	Reputation int64
}

// Reputation fetches the reputation information for a single Hive account.
func (c *Client) Reputation(account string) (*ReputationResult, error) {
	account = strings.TrimSpace(account)
	if account == "" {
		return nil, fmt.Errorf("account name must be provided")
	}

	payload, err := c.request(fmt.Sprintf("reputation-api/accounts/%s/reputation", account))
	if err != nil {
		return nil, err
	}
	if payload == nil {
		return nil, fmt.Errorf("empty reputation response for account %s", account)
	}

	switch value := payload.(type) {
	case map[string]any:
		rep, err := extractInt64(value["reputation"])
		if err != nil {
			return nil, err
		}
		acct := account
		if rawAccount, ok := value["account"].(string); ok && rawAccount != "" {
			acct = rawAccount
		}
		return &ReputationResult{Account: acct, Reputation: rep}, nil
	case json.Number:
		rep, err := value.Int64()
		if err != nil {
			return nil, err
		}
		return &ReputationResult{Account: account, Reputation: rep}, nil
	case float64:
		return &ReputationResult{Account: account, Reputation: int64(value)}, nil
	case int64:
		return &ReputationResult{Account: account, Reputation: value}, nil
	default:
		return nil, fmt.Errorf("unexpected reputation response type: %T", value)
	}
}

// AccountBalances retrieves balance information for an account via the HAF balance API.
func (c *Client) AccountBalances(account string) (map[string]any, error) {
	account = strings.TrimSpace(account)
	if account == "" {
		return nil, fmt.Errorf("account name must be provided")
	}

	payload, err := c.request(fmt.Sprintf("balance-api/accounts/%s/balances", account))
	if err != nil {
		return nil, err
	}
	if payload == nil {
		return nil, fmt.Errorf("empty balances response for account %s", account)
	}

	balances, ok := payload.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected balances response type: %T", payload)
	}

	return balances, nil
}

func (c *Client) request(endpoint string) (any, error) {
	if c == nil {
		return nil, fmt.Errorf("haf client is nil")
	}

	url := fmt.Sprintf("%s/%s", c.baseURL, strings.TrimLeft(endpoint, "/"))

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("haf request failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	decoder := json.NewDecoder(resp.Body)
	decoder.UseNumber()

	var payload any
	if err := decoder.Decode(&payload); err != nil {
		if err == io.EOF {
			return nil, nil
		}
		return nil, err
	}

	return payload, nil
}

func extractInt64(value any) (int64, error) {
	switch v := value.(type) {
	case json.Number:
		return v.Int64()
	case float64:
		return int64(v), nil
	case int64:
		return v, nil
	case int32:
		return int64(v), nil
	case int:
		return int64(v), nil
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("unable to parse numeric string: %w", err)
		}
		return parsed, nil
	case nil:
		return 0, fmt.Errorf("numeric value missing")
	default:
		return 0, fmt.Errorf("unsupported numeric type: %T", value)
	}
}
