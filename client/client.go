// Package client provides a JSON-RPC client to interact with Hive blockchain nodes, supporting node failover, automatic retries with exponential backoff, and live block and operation streaming.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/srbde/hive-anther/types"
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
		httpClient: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
	}
}

// GetNextNode gets the next available node from the list (legacy round-robin helper).
func (c *Client) GetNextNode() string {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if len(c.Nodes) == 0 {
		return ""
	}
	c.CurrentNodeIndex = (c.CurrentNodeIndex + 1) % len(c.Nodes)
	return c.Nodes[c.CurrentNodeIndex]
}

// GetCurrentNode returns the currently selected node URL.
// If no node has been selected yet, it selects the first one.
func (c *Client) GetCurrentNode() string {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if len(c.Nodes) == 0 {
		return ""
	}
	if c.CurrentNodeIndex < 0 {
		c.CurrentNodeIndex = 0
	}
	return c.Nodes[c.CurrentNodeIndex]
}

// RotateNode advances the client to the next node in the list and returns it.
func (c *Client) RotateNode() string {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if len(c.Nodes) == 0 {
		return ""
	}
	c.CurrentNodeIndex = (c.CurrentNodeIndex + 1) % len(c.Nodes)
	node := c.Nodes[c.CurrentNodeIndex]
	if enableLogging {
		logDebug("switching node to: %s", node)
	}
	return node
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

const (
	maxRetries    = 3
	baseBackoffMS = 100
)

// Call makes a JSON-RPC call to a Hive node with sticky node failover and exponential backoff retries.
func (c *Client) Call(api string, method string, params any) (any, error) {
	var lastErr error
	backoff := time.Duration(baseBackoffMS) * time.Millisecond

	for attempt := range maxRetries {
		for i := 0; i < len(c.Nodes); i++ {
			nodeURL := c.GetCurrentNode()
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

			var result map[string]any
			err = func() error {
				resp, err := c.httpClient.Do(req)
				if err != nil {
					return err
				}
				defer resp.Body.Close()

				if resp.StatusCode != http.StatusOK {
					return fmt.Errorf("HTTP status %d", resp.StatusCode)
				}

				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return err
				}

				return json.Unmarshal(body, &result)
			}()
			if err != nil {
				if enableLogging {
					logDebug("node %s call failed: %v", nodeURL, err)
				}
				lastErr = err
				c.RotateNode()
				continue
			}

			if errMsg, ok := result["error"]; ok {
				if errMap, ok := errMsg.(map[string]any); ok {
					codeVal, hasCode := errMap["code"]
					var rpcCode int64
					if hasCode {
						switch cv := codeVal.(type) {
						case float64:
							rpcCode = int64(cv)
						case int64:
							rpcCode = cv
						case int:
							rpcCode = int64(cv)
						}
					}

					// If it is a node-level error (Method not found or Internal error), failover!
					if hasCode && (rpcCode == -32601 || rpcCode == -32603) {
						if enableLogging {
							logDebug("node %s returned RPC error code %d; failing over", nodeURL, rpcCode)
						}
						lastErr = fmt.Errorf("node returned JSON-RPC error code %d", rpcCode)
						c.RotateNode()
						continue
					}

					if msg, ok := errMap["message"].(string); ok {
						return nil, errors.New(msg)
					}
				}
				return nil, errors.New("RPC error")
			}

			return result["result"], nil
		}

		if attempt < maxRetries-1 {
			if enableLogging {
				logDebug("all nodes failed on attempt %d, backing off for %v...", attempt+1, backoff)
			}
			time.Sleep(backoff)
			backoff *= 2
		}
	}

	if lastErr != nil {
		return nil, fmt.Errorf("all nodes failed after retries: %w", lastErr)
	}
	return nil, errors.New("all nodes failed after retries")
}

// GetDynamicGlobalProperties gets the dynamic global properties of the Hive blockchain.
func (c *Client) GetDynamicGlobalProperties() (map[string]any, error) {
	resp, err := c.Call("condenser_api", "get_dynamic_global_properties", nil)
	if err != nil {
		return nil, err
	}
	return resp.(map[string]any), nil
}

// StreamingMode controls whether head block or last irreversible block is followed
type StreamingMode string

const (
	Latest       StreamingMode = "latest"
	Irreversible StreamingMode = "irreversible"
)

// GetDynamicGlobalPropertiesStruct fetches the dynamic global properties of the Hive blockchain as a typed struct.
func (c *Client) GetDynamicGlobalPropertiesStruct() (*types.DynamicGlobalProperties, error) {
	resp, err := c.Call("condenser_api", "get_dynamic_global_properties", nil)
	if err != nil {
		return nil, err
	}
	bytesVal, err := json.Marshal(resp)
	if err != nil {
		return nil, err
	}
	var props types.DynamicGlobalProperties
	if err := json.Unmarshal(bytesVal, &props); err != nil {
		return nil, err
	}
	return &props, nil
}

// GetTransaction fetches a transaction by its transaction ID.
func (c *Client) GetTransaction(trxID string) (any, error) {
	return c.Call("condenser_api", "get_transaction", []any{trxID})
}

// GetBlock fetches a signed block by number.
func (c *Client) GetBlock(blockNum uint32) (*types.Block, error) {
	resp, err := c.Call("condenser_api", "get_block", []any{blockNum})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, fmt.Errorf("block %d not found", blockNum)
	}
	bytesVal, err := json.Marshal(resp)
	if err != nil {
		return nil, err
	}
	var block types.Block
	if err := json.Unmarshal(bytesVal, &block); err != nil {
		return nil, err
	}
	return &block, nil
}

// GetBlockRange fetches a range of blocks starting from startingBlockNum.
func (c *Client) GetBlockRange(startingBlockNum uint32, count uint32) ([]*types.Block, error) {
	if count == 0 {
		return nil, fmt.Errorf("block range count must be greater than 0")
	}
	if count > 1000 {
		return nil, fmt.Errorf("block range count cannot exceed 1000")
	}

	params := map[string]any{
		"starting_block_num": startingBlockNum,
		"count":              count,
	}
	resp, err := c.Call("block_api", "get_block_range", params)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, fmt.Errorf("block range starting at %d not found", startingBlockNum)
	}

	respMap, ok := resp.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected response type for get_block_range")
	}

	blocksVal, ok := respMap["blocks"]
	if !ok {
		return nil, fmt.Errorf("missing 'blocks' field in get_block_range response")
	}

	bytesVal, err := json.Marshal(blocksVal)
	if err != nil {
		return nil, err
	}

	var blocks []*types.Block
	if err := json.Unmarshal(bytesVal, &blocks); err != nil {
		return nil, err
	}

	return blocks, nil
}

// GetOpsInBlock fetches applied operations in a block.
func (c *Client) GetOpsInBlock(blockNum uint32, onlyVirtual bool) ([]*types.AppliedOperation, error) {
	resp, err := c.Call("condenser_api", "get_ops_in_block", []any{blockNum, onlyVirtual})
	if err != nil {
		return nil, err
	}
	bytesVal, err := json.Marshal(resp)
	if err != nil {
		return nil, err
	}
	var ops []*types.AppliedOperation
	if err := json.Unmarshal(bytesVal, &ops); err != nil {
		return nil, err
	}
	return ops, nil
}

// opsInBlockRangeConcurrency caps how many GetOpsInBlock calls GetOpsInBlockRange runs at once.
// Hive has no batch get_ops_in_block RPC, so this trades HTTP connections for wall-clock time
// instead of trading request count.
const opsInBlockRangeConcurrency = 20

// GetOpsInBlockRange fetches applied operations for a range of blocks starting at
// startingBlockNum, up to count blocks (same 1000-block ceiling as GetBlockRange). Blocks are
// fetched concurrently (bounded by opsInBlockRangeConcurrency) rather than one at a time, which
// meaningfully cuts wall-clock time when catching up a large backlog. Results preserve block
// order regardless of completion order.
func (c *Client) GetOpsInBlockRange(startingBlockNum uint32, count uint32) ([]*types.AppliedOperation, error) {
	if count == 0 {
		return nil, fmt.Errorf("block range count must be greater than 0")
	}
	if count > 1000 {
		return nil, fmt.Errorf("block range count cannot exceed 1000")
	}

	results := make([][]*types.AppliedOperation, count)
	errs := make([]error, count)

	sem := make(chan struct{}, opsInBlockRangeConcurrency)
	var wg sync.WaitGroup

	for i := uint32(0); i < count; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx uint32) {
			defer wg.Done()
			defer func() { <-sem }()
			results[idx], errs[idx] = c.GetOpsInBlock(startingBlockNum+idx, false)
		}(i)
	}
	wg.Wait()

	var allOps []*types.AppliedOperation
	for i := uint32(0); i < count; i++ {
		if errs[i] != nil {
			return nil, fmt.Errorf("failed to get ops for block %d: %w", startingBlockNum+i, errs[i])
		}
		allOps = append(allOps, results[i]...)
	}
	return allOps, nil
}

// StreamBlocks streams blocks starting from startBlock (or latest/irreversible if 0) indefinitely.
func (c *Client) StreamBlocks(ctx context.Context, startBlock uint32, mode StreamingMode) (<-chan *types.Block, <-chan error) {
	out := make(chan *types.Block, 10)
	errChan := make(chan error, 5)

	go func() {
		defer close(out)
		defer close(errChan)

		var current uint32
		props, err := c.GetDynamicGlobalPropertiesStruct()
		if err != nil {
			select {
			case errChan <- fmt.Errorf("failed to fetch properties: %w", err):
			case <-ctx.Done():
			}
			return
		}

		if mode == Irreversible {
			current = props.LastIrreversibleBlockNum
		} else {
			current = props.HeadBlockNumber
		}

		var seen uint32
		if startBlock > 0 {
			if startBlock > current {
				select {
				case errChan <- fmt.Errorf("start block %d cannot be in the future (current: %d)", startBlock, current):
				case <-ctx.Done():
				}
				return
			}
			seen = startBlock
		} else {
			seen = current
		}

		for {
			props, err := c.GetDynamicGlobalPropertiesStruct()
			if err != nil {
				select {
				case errChan <- fmt.Errorf("poll properties error: %w", err):
				case <-ctx.Done():
					return
				}
				select {
				case <-time.After(3 * time.Second):
				case <-ctx.Done():
					return
				}
				continue
			}

			if mode == Irreversible {
				current = props.LastIrreversibleBlockNum
			} else {
				current = props.HeadBlockNumber
			}

			for seen <= current {
				block, err := c.GetBlock(seen)
				if err != nil {
					select {
					case errChan <- fmt.Errorf("failed to get block %d: %w", seen, err):
					case <-ctx.Done():
						return
					}
					select {
					case <-time.After(1 * time.Second):
					case <-ctx.Done():
						return
					}
					continue
				}

				select {
				case out <- block:
					seen++
				case <-ctx.Done():
					return
				}
			}

			select {
			case <-time.After(3 * time.Second):
			case <-ctx.Done():
				return
			}
		}
	}()

	return out, errChan
}

// StreamOperations streams applied operations starting from startBlock (or latest/irreversible if 0), filtered by operation type.
func (c *Client) StreamOperations(ctx context.Context, startBlock uint32, mode StreamingMode, filter []string) (<-chan *types.AppliedOperation, <-chan error) {
	out := make(chan *types.AppliedOperation, 100)
	errChan := make(chan error, 5)

	filterMap := make(map[string]bool)
	for _, f := range filter {
		filterMap[f] = true
	}

	go func() {
		defer close(out)
		defer close(errChan)

		var current uint32
		props, err := c.GetDynamicGlobalPropertiesStruct()
		if err != nil {
			select {
			case errChan <- fmt.Errorf("failed to fetch properties: %w", err):
			case <-ctx.Done():
			}
			return
		}

		if mode == Irreversible {
			current = props.LastIrreversibleBlockNum
		} else {
			current = props.HeadBlockNumber
		}

		var seen uint32
		if startBlock > 0 {
			if startBlock > current {
				select {
				case errChan <- fmt.Errorf("start block %d cannot be in the future (current: %d)", startBlock, current):
				case <-ctx.Done():
				}
				return
			}
			seen = startBlock
		} else {
			seen = current
		}

		for {
			props, err := c.GetDynamicGlobalPropertiesStruct()
			if err != nil {
				select {
				case errChan <- fmt.Errorf("poll properties error: %w", err):
				case <-ctx.Done():
					return
				}
				select {
				case <-time.After(3 * time.Second):
				case <-ctx.Done():
					return
				}
				continue
			}

			if mode == Irreversible {
				current = props.LastIrreversibleBlockNum
			} else {
				current = props.HeadBlockNumber
			}

			for seen <= current {
				ops, err := c.GetOpsInBlock(seen, false)
				if err != nil {
					select {
					case errChan <- fmt.Errorf("failed to get operations for block %d: %w", seen, err):
					case <-ctx.Done():
						return
					}
					select {
					case <-time.After(1 * time.Second):
					case <-ctx.Done():
						return
					}
					continue
				}

				for _, op := range ops {
					if len(op.Op) > 0 {
						opType, ok := op.Op[0].(string)
						if ok {
							if len(filterMap) > 0 && !filterMap[opType] {
								continue
							}
						}
					}
					select {
					case out <- op:
					case <-ctx.Done():
						return
					}
				}
				seen++
			}

			select {
			case <-time.After(3 * time.Second):
			case <-ctx.Done():
				return
			}
		}
	}()

	return out, errChan
}
