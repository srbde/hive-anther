package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync"
)

// Client is a JSON-RPC client for interacting with Hive nodes.
type Client struct {
	Nodes            []string
	Timeout          int
	CurrentNodeIndex int
	mutex            sync.Mutex
	httpClient       *http.Client
}

// NewClient creates a new Client.
func NewClient(nodes []string, timeout int) *Client {
	return &Client{
		Nodes:            nodes,
		Timeout:          timeout,
		CurrentNodeIndex: -1,
		httpClient:       &http.Client{},
	}
}

// GetNextNode gets the next available node from the list.
func (c *Client) GetNextNode() string {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.CurrentNodeIndex = (c.CurrentNodeIndex + 1) % len(c.Nodes)
	return c.Nodes[c.CurrentNodeIndex]
}

// BuildPayload builds the JSON-RPC payload.
func (c *Client) BuildPayload(api string, method string, params any) (map[string]any, error) {
	payload := map[string]any{
		"jsonrpc": "2.0",
		"method":  api + "." + method,
		"id":      1,
	}
	if params != nil {
		payload["params"] = params
	} else {
		payload["params"] = []any{}
	}
	return payload, nil
}

// Call makes a JSON-RPC call to a Hive node.
func (c *Client) Call(api string, method string, params any) (any, error) {
	for i := 0; i < len(c.Nodes); i++ {
		nodeURL := c.GetNextNode()
		payload, err := c.BuildPayload(api, method, params)
		if err != nil {
			return nil, err
		}

		jsonPayload, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}

		req, err := http.NewRequest("POST", nodeURL, bytes.NewBuffer(jsonPayload))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		var result map[string]any
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, err
		}

		if err, ok := result["error"]; ok {
			return nil, errors.New(err.(map[string]any)["message"].(string))
		}

		return result["result"], nil
	}

	return nil, errors.New("all nodes failed")
}

// GetDynamicGlobalProperties gets the dynamic global properties of the Hive blockchain.
func (c *Client) GetDynamicGlobalProperties() (map[string]any, error) {
	resp, err := c.Call("condenser_api", "get_dynamic_global_properties", nil)
	if err != nil {
		return nil, err
	}
	return resp.(map[string]any), nil
}
