// Package account manages Hive account profiles, reputations, voting power, Resource Credit details, and helper functions to build account operations.
package account

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/srbde/hive-anther/client"
	"github.com/srbde/hive-anther/haf"
	"github.com/srbde/hive-anther/transaction"
)

const (
	VotingManaRegenerationSeconds = 5 * 24 * 60 * 60 // 5 days in seconds
	RCManaRegenerationSeconds     = 5 * 24 * 60 * 60 // 5 days in seconds
)

// Account represents a Hive account.
type Account struct {
	Name       string
	API        *client.Client
	Data       map[string]any
	rcInfo     map[string]any
	hafClient  *haf.Client
	reputation *float64
}

// NewAccount creates a new Account.
func NewAccount(name string, api *client.Client) *Account {
	return &Account{
		Name: name,
		API:  api,
		Data: make(map[string]any),
	}
}

// Refresh fetches the account data from the blockchain.
func (a *Account) Refresh() error {
	if a.API == nil {
		return fmt.Errorf("API not configured")
	}

	resp, err := a.API.Call("condenser_api", "get_accounts", [][]string{{a.Name}})
	if err != nil {
		return err
	}

	accounts, ok := resp.([]any)
	if !ok || len(accounts) == 0 {
		return fmt.Errorf("account '%s' not found", a.Name)
	}

	a.Data, _ = accounts[0].(map[string]any)
	a.reputation = nil
	return nil
}

// SetHAFClient allows injecting a custom HAF client for reputation and balance lookups.
func (a *Account) SetHAFClient(client *haf.Client) {
	a.hafClient = client
}

// GetReputation fetches the account reputation using HAF, falling back to condenser API. When refresh is false and a
// cached value exists it will be returned directly.
func (a *Account) GetReputation(refresh bool) (float64, error) {
	if a.reputation != nil && !refresh {
		return *a.reputation, nil
	}

	// 1. Try HAF reputation using the injected Client first
	client := a.hafClient

	// 2. Try HAF client with configured API node if available
	if client == nil && a.API != nil && len(a.API.Nodes) > 0 {
		if c, err := haf.NewClient(a.API.Nodes[0], 10*time.Second); err == nil {
			client = c
		}
	}

	// 3. Try default client
	if client == nil {
		if defaultClient, err := haf.DefaultClient(); err == nil {
			client = defaultClient
		}
	}

	// Query HAF if client is available
	if client != nil {
		if result, err := client.Reputation(a.Name); err == nil && result != nil {
			rep := float64(result.Reputation)
			a.reputation = &rep
			return rep, nil
		}
	}

	// 4. Fallback to condenser API cached reputation
	if len(a.Data) == 0 && a.API != nil {
		_ = a.Refresh() // Try to refresh account data to fetch 'reputation' field
	}

	if len(a.Data) > 0 {
		var rawRep float64
		if repStr, ok := a.Data["reputation"].(string); ok {
			if parsed, err := strconv.ParseFloat(repStr, 64); err == nil {
				rawRep = parsed
			}
		} else if repFloat, ok := a.Data["reputation"].(float64); ok {
			rawRep = repFloat
		}
		if rawRep != 0 {
			rep := calculateReputation(rawRep)
			a.reputation = &rep
			return rep, nil
		}
	}

	// Fallback to default score if all else fails
	defaultRep := 25.0
	a.reputation = &defaultRep
	return defaultRep, nil
}

// Reputation returns the cached reputation value or fetches it when unavailable.
func (a *Account) Reputation() (float64, error) {
	return a.GetReputation(false)
}

// Rep is a shorthand alias for Reputation.
func (a *Account) Rep() (float64, error) {
	return a.GetReputation(false)
}

func calculateReputation(rawRep float64) float64 {
	if rawRep == 0 {
		return 25.0
	}
	sign := 1.0
	if rawRep < 0 {
		sign = -1.0
		rawRep = -rawRep
	}
	logVal := math.Log10(rawRep)
	rep := (logVal-9.0)*9.0 + 25.0
	return sign * rep
}

// Follow creates a follow transaction for another account.
func (a *Account) Follow(accountToFollow string) (*transaction.Transaction, error) {
	if a.API == nil {
		return nil, fmt.Errorf("API not configured")
	}

	tx := transaction.NewTransaction(a.API)
	follow := &transaction.Follow{
		Follower:  a.Name,
		Following: accountToFollow,
		What:      []string{"blog"},
	}
	tx.AppendOp(follow)
	return tx, nil
}

// Unfollow creates an unfollow transaction for an account.
func (a *Account) Unfollow(accountToUnfollow string) (*transaction.Transaction, error) {
	if a.API == nil {
		return nil, fmt.Errorf("API not configured")
	}

	tx := transaction.NewTransaction(a.API)
	follow := &transaction.Follow{
		Follower:  a.Name,
		Following: accountToUnfollow,
		What:      []string{}, // Empty list means unfollow
	}
	tx.AppendOp(follow)
	return tx, nil
}

// Ignore creates an ignore/mute transaction for another account.
func (a *Account) Ignore(accountToIgnore string) (*transaction.Transaction, error) {
	if a.API == nil {
		return nil, fmt.Errorf("API not configured")
	}

	tx := transaction.NewTransaction(a.API)
	follow := &transaction.Follow{
		Follower:  a.Name,
		Following: accountToIgnore,
		What:      []string{"ignore"},
	}
	tx.AppendOp(follow)
	return tx, nil
}

// Unignore creates an unignore transaction (same as unfollow).
func (a *Account) Unignore(accountToUnignore string) (*transaction.Transaction, error) {
	return a.Unfollow(accountToUnignore)
}

// GetVotingPower calculates the current voting power percentage.
func (a *Account) GetVotingPower(refresh bool) (float64, error) {
	if refresh || len(a.Data) == 0 {
		if a.API == nil {
			return 0, fmt.Errorf("API not configured")
		}
		if err := a.Refresh(); err != nil {
			return 0, err
		}
	}

	// Get manabar data
	manabar, ok := a.Data["voting_manabar"].(map[string]any)
	if !ok {
		manabar = make(map[string]any)
	}

	var currentMana float64
	var lastUpdateTime time.Time
	var useManabarTime bool = false

	// Priority: use voting_power from account data
	// The manabar.current_mana is in raw format and needs to be calculated differently
	// For now, use voting_power which is already a value out of 10000
	if votingPowerVal, ok := a.Data["voting_power"].(float64); ok {
		currentMana = votingPowerVal
	} else if currentManaVal, ok := manabar["current_mana"].(float64); ok {
		// Fallback to manabar current_mana if voting_power not available
		// This needs special handling as it's a raw value
		currentMana = currentManaVal
	} else {
		currentMana = 0
	}

	// Try to get last_update_time - prefer manabar timestamp for mana regeneration calculation
	if lastUpdateTimeVal, ok := manabar["last_update_time"].(float64); ok && lastUpdateTimeVal > 0 {
		lastUpdateTime = time.Unix(int64(lastUpdateTimeVal), 0)
		useManabarTime = true
	} else if lastVoteTimeStr, ok := a.Data["last_vote_time"].(string); ok {
		if t, err := time.Parse("2006-01-02T15:04:05", lastVoteTimeStr); err == nil {
			lastUpdateTime = t
		}
	}

	// max_mana defaults to 10000
	var maxMana float64 = 10000
	if maxManaVal, ok := manabar["max_mana"].(float64); ok {
		maxMana = maxManaVal
	}

	// Calculate mana regeneration if we have a valid timestamp
	if !lastUpdateTime.IsZero() && useManabarTime {
		diff := time.Since(lastUpdateTime).Seconds()
		regenerated := diff * maxMana / float64(VotingManaRegenerationSeconds)
		currentMana = minFloat64(maxMana, currentMana+regenerated)
	}

	if maxMana <= 0 {
		return 0.0, nil
	}

	return (currentMana / maxMana) * 100, nil
}

// VotingPower returns the current voting power percentage.
func (a *Account) VotingPower() (float64, error) {
	return a.GetVotingPower(false)
}

// VP returns the current voting power percentage (shorthand).
func (a *Account) VP() (float64, error) {
	return a.GetVotingPower(false)
}

// GetRCInfo fetches and caches Resource Credit information.
func (a *Account) GetRCInfo(refresh bool) (map[string]any, error) {
	if a.rcInfo != nil && !refresh {
		return a.rcInfo, nil
	}

	if a.API == nil {
		return nil, fmt.Errorf("API not configured")
	}

	resp, err := a.API.Call("rc_api", "find_rc_accounts", map[string]any{"accounts": []string{a.Name}})
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
		return nil, fmt.Errorf("no RC data found for account %s", a.Name)
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
	}

	lastMana := int64(0)
	if lastVal, ok := manabar["current_mana"].(float64); ok {
		lastMana = int64(lastVal)
	}

	var lastUpdateTime time.Time
	if lastUpdateVal, ok := manabar["last_update_time"].(float64); ok {
		lastUpdateTime = time.Unix(int64(lastUpdateVal), 0)
	}

	currentMana := lastMana
	if !lastUpdateTime.IsZero() && maxRC > 0 {
		diff := time.Since(lastUpdateTime).Seconds()
		regenerated := diff * float64(maxRC) / float64(RCManaRegenerationSeconds)
		currentMana = minInt64(maxRC, lastMana+int64(regenerated))
	}

	lastPercent := 0.0
	if maxRC > 0 {
		lastPercent = (float64(lastMana) / float64(maxRC)) * 100
	}

	currentPercent := 0.0
	if maxRC > 0 {
		currentPercent = (float64(currentMana) / float64(maxRC)) * 100
	}

	info := map[string]any{
		"last_mana":        lastMana,
		"current_mana":     currentMana,
		"max_mana":         maxRC,
		"last_update_time": lastUpdateTime,
		"last_percent":     lastPercent,
		"current_percent":  currentPercent,
	}

	a.rcInfo = info
	return info, nil
}

// RCInfo returns the RC info property.
func (a *Account) RCInfo() (map[string]any, error) {
	return a.GetRCInfo(false)
}

// RC returns the current RC percentage.
func (a *Account) RC() (float64, error) {
	info, err := a.GetRCInfo(false)
	if err != nil || info == nil {
		return 0, err
	}
	if currentPercent, ok := info["current_percent"].(float64); ok {
		return currentPercent, nil
	}
	return 0, nil
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func minFloat64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
