package client

import (
	"fmt"
	"time"

	"github.com/thecrazygm/anther/types"
)

// GetRCParams fetches the Resource Credit resource parameters.
func (c *Client) GetRCParams() (map[string]any, error) {
	resp, err := c.Call("rc_api", "get_rc_resource_params", nil)
	if err != nil {
		return nil, err
	}
	resMap, ok := resp.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected get_rc_resource_params response type: %T", resp)
	}
	return resMap, nil
}

// GetRCPool fetches the Resource Credit resource pool.
func (c *Client) GetRCPool() (map[string]any, error) {
	resp, err := c.Call("rc_api", "get_rc_resource_pool", nil)
	if err != nil {
		return nil, err
	}
	resMap, ok := resp.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected get_rc_resource_pool response type: %T", resp)
	}
	return resMap, nil
}

// GetRCMana retrieves and calculates Resource Credit details for a specific account.
func (c *Client) GetRCMana(account string) (*types.RCInfo, error) {
	if account == "" {
		return nil, fmt.Errorf("account name cannot be empty")
	}
	resp, err := c.Call("rc_api", "find_rc_accounts", map[string]any{"accounts": []string{account}})
	if err != nil {
		return nil, err
	}
	var rcAccounts []any
	if respMap, ok := resp.(map[string]any); ok {
		if accounts, ok := respMap["rc_accounts"].([]any); ok {
			rcAccounts = accounts
		} else if accounts, ok := respMap["result"].([]any); ok {
			rcAccounts = accounts
		}
	} else if accounts, ok := resp.([]any); ok {
		rcAccounts = accounts
	}

	if len(rcAccounts) == 0 {
		return nil, fmt.Errorf("no RC data found for account %s", account)
	}

	rcAccount, ok := rcAccounts[0].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid RC account data")
	}

	manabar, _ := rcAccount["rc_manabar"].(map[string]any)
	if manabar == nil {
		manabar = make(map[string]any)
	}

	maxRC := int64(0)
	if maxVal, ok := rcAccount["max_rc"].(float64); ok {
		maxRC = int64(maxVal)
	} else if maxValStr, ok := rcAccount["max_rc"].(string); ok {
		fmt.Sscanf(maxValStr, "%d", &maxRC)
	}

	lastMana := int64(0)
	if lastVal, ok := manabar["current_mana"].(float64); ok {
		lastMana = int64(lastVal)
	} else if lastValStr, ok := manabar["current_mana"].(string); ok {
		fmt.Sscanf(lastValStr, "%d", &lastMana)
	}

	var lastUpdateTime time.Time
	if lastUpdateVal, ok := manabar["last_update_time"].(float64); ok {
		lastUpdateTime = time.Unix(int64(lastUpdateVal), 0)
	}

	currentMana := lastMana
	if !lastUpdateTime.IsZero() && maxRC > 0 {
		diff := time.Since(lastUpdateTime).Seconds()
		regenerated := diff * float64(maxRC) / float64(5*24*60*60)
		currentMana = lastMana + int64(regenerated)
		if currentMana > maxRC {
			currentMana = maxRC
		}
	}

	lastPercent := 0.0
	if maxRC > 0 {
		lastPercent = (float64(lastMana) / float64(maxRC)) * 100
	}

	currentPercent := 0.0
	if maxRC > 0 {
		currentPercent = (float64(currentMana) / float64(maxRC)) * 100
	}

	return &types.RCInfo{
		LastMana:       lastMana,
		CurrentMana:    currentMana,
		MaxMana:        maxRC,
		LastUpdateTime: lastUpdateTime,
		LastPercent:    lastPercent,
		CurrentPercent: currentPercent,
	}, nil
}

// CalculateRCMana queries and calculates the current Resource Credit percentage.
func (c *Client) CalculateRCMana(accountData *types.AccountData) float64 {
	if accountData == nil {
		return 0.0
	}
	info, err := c.GetRCMana(accountData.Name)
	if err != nil {
		return 0.0
	}
	return info.CurrentPercent
}

// CalculateVPMana calculates the real-time Voting Power percentage of an account.
func (c *Client) CalculateVPMana(accountData *types.AccountData) float64 {
	if accountData == nil {
		return 0.0
	}

	maxMana := 10000.0
	currentMana := accountData.VotingPower

	if accountData.VotingManabar.CurrentMana > 0 {
		currentMana = accountData.VotingManabar.CurrentMana
	}

	if accountData.VotingManabar.LastUpdateTime > 0 {
		lastUpdateTime := time.Unix(accountData.VotingManabar.LastUpdateTime, 0)
		diff := time.Since(lastUpdateTime).Seconds()
		regenerated := diff * maxMana / float64(5*24*60*60)
		currentMana = currentMana + regenerated
		if currentMana > maxMana {
			currentMana = maxMana
		}
	}

	if maxMana <= 0 {
		return 0.0
	}
	return (currentMana / maxMana) * 100.0
}
