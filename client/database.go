package client

import (
	"encoding/json"
	"fmt"

	"github.com/thecrazygm/anther/types"
)

// GetConfig returns the node's configuration map.
func (c *Client) GetConfig() (map[string]any, error) {
	resp, err := c.Call("database_api", "get_config", nil)
	if err != nil {
		// Fallback to condenser_api
		resp, err = c.Call("condenser_api", "get_config", nil)
		if err != nil {
			return nil, err
		}
	}
	resMap, ok := resp.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected get_config response type: %T", resp)
	}
	return resMap, nil
}

// GetChainProperties returns the current chain properties.
func (c *Client) GetChainProperties() (*types.ChainProperties, error) {
	resp, err := c.Call("condenser_api", "get_chain_properties", nil)
	if err != nil {
		return nil, err
	}
	bytesVal, err := json.Marshal(resp)
	if err != nil {
		return nil, err
	}
	var props types.ChainProperties
	if err := json.Unmarshal(bytesVal, &props); err != nil {
		return nil, err
	}
	return &props, nil
}

// GetCurrentMedianHistoryPrice returns the current median history price for HIVE/HBD.
func (c *Client) GetCurrentMedianHistoryPrice() (*types.Price, error) {
	resp, err := c.Call("condenser_api", "get_current_median_history_price", nil)
	if err != nil {
		return nil, err
	}
	bytesVal, err := json.Marshal(resp)
	if err != nil {
		return nil, err
	}
	var price types.Price
	if err := json.Unmarshal(bytesVal, &price); err != nil {
		return nil, err
	}
	return &price, nil
}

// GetAccounts fetches details for a list of account names.
func (c *Client) GetAccounts(accounts []string) ([]*types.AccountData, error) {
	if len(accounts) == 0 {
		return nil, fmt.Errorf("accounts slice cannot be empty")
	}
	resp, err := c.Call("condenser_api", "get_accounts", []any{accounts})
	if err != nil {
		return nil, err
	}
	bytesVal, err := json.Marshal(resp)
	if err != nil {
		return nil, err
	}
	var accountsData []*types.AccountData
	if err := json.Unmarshal(bytesVal, &accountsData); err != nil {
		return nil, err
	}
	return accountsData, nil
}

// GetAccountHistory fetches the operation history of an account.
// The limit parameter cannot exceed 1000.
func (c *Client) GetAccountHistory(account string, start int64, limit uint32) ([]*types.HistoryItem, error) {
	if account == "" {
		return nil, fmt.Errorf("account name cannot be empty")
	}
	if limit > 1000 {
		return nil, fmt.Errorf("limit cannot exceed 1000")
	}
	resp, err := c.Call("condenser_api", "get_account_history", []any{account, start, limit})
	if err != nil {
		return nil, err
	}
	bytesVal, err := json.Marshal(resp)
	if err != nil {
		return nil, err
	}
	var history []*types.HistoryItem
	if err := json.Unmarshal(bytesVal, &history); err != nil {
		return nil, err
	}
	return history, nil
}

// GetVestingDelegations returns active vesting delegations for an account.
// The limit parameter cannot exceed 1000.
func (c *Client) GetVestingDelegations(delegator string, start string, limit uint32) ([]*types.VestingDelegation, error) {
	if delegator == "" {
		return nil, fmt.Errorf("delegator name cannot be empty")
	}
	if limit > 1000 {
		return nil, fmt.Errorf("limit cannot exceed 1000")
	}
	resp, err := c.Call("condenser_api", "get_vesting_delegations", []any{delegator, start, limit})
	if err != nil {
		return nil, err
	}
	bytesVal, err := json.Marshal(resp)
	if err != nil {
		return nil, err
	}
	var delegations []*types.VestingDelegation
	if err := json.Unmarshal(bytesVal, &delegations); err != nil {
		return nil, err
	}
	return delegations, nil
}

// GetBlockHeader returns the block header for a specific block.
func (c *Client) GetBlockHeader(blockNum uint32) (*types.BlockHeader, error) {
	resp, err := c.Call("condenser_api", "get_block_header", []any{blockNum})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, fmt.Errorf("block header for block %d not found", blockNum)
	}
	bytesVal, err := json.Marshal(resp)
	if err != nil {
		return nil, err
	}
	var header types.BlockHeader
	if err := json.Unmarshal(bytesVal, &header); err != nil {
		return nil, err
	}
	return &header, nil
}
