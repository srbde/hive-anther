package client

import (
	"fmt"
)

// GetRankedPosts retrieves ranked posts from the Hivemind bridge API.
func (c *Client) GetRankedPosts(sort string, startAuthor string, startPermlink string, limit uint32, tag string) ([]map[string]any, error) {
	if limit > 100 {
		return nil, fmt.Errorf("limit cannot exceed 100")
	}

	params := map[string]any{
		"sort":  sort,
		"limit": limit,
	}
	if startAuthor != "" {
		params["start_author"] = startAuthor
	}
	if startPermlink != "" {
		params["start_permlink"] = startPermlink
	}
	if tag != "" {
		params["tag"] = tag
	}

	resp, err := c.Call("bridge", "get_ranked_posts", params)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return []map[string]any{}, nil
	}

	sliceVal, ok := resp.([]any)
	if !ok {
		return nil, fmt.Errorf("unexpected get_ranked_posts response type: %T", resp)
	}

	result := make([]map[string]any, len(sliceVal))
	for i, v := range sliceVal {
		m, ok := v.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("unexpected post element type: %T", v)
		}
		result[i] = m
	}
	return result, nil
}

// GetAccountPosts retrieves posts created by or associated with a specific account.
func (c *Client) GetAccountPosts(sort string, account string, limit uint32, startAuthor string, startPermlink string) ([]map[string]any, error) {
	if account == "" {
		return nil, fmt.Errorf("account name cannot be empty")
	}
	if limit > 100 {
		return nil, fmt.Errorf("limit cannot exceed 100")
	}

	params := map[string]any{
		"sort":    sort,
		"account": account,
		"limit":   limit,
	}
	if startAuthor != "" {
		params["start_author"] = startAuthor
	}
	if startPermlink != "" {
		params["start_permlink"] = startPermlink
	}

	resp, err := c.Call("bridge", "get_account_posts", params)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return []map[string]any{}, nil
	}

	sliceVal, ok := resp.([]any)
	if !ok {
		return nil, fmt.Errorf("unexpected get_account_posts response type: %T", resp)
	}

	result := make([]map[string]any, len(sliceVal))
	for i, v := range sliceVal {
		m, ok := v.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("unexpected post element type: %T", v)
		}
		result[i] = m
	}
	return result, nil
}

// GetCommunity retrieves details about a specific community.
func (c *Client) GetCommunity(name string) (map[string]any, error) {
	if name == "" {
		return nil, fmt.Errorf("community name cannot be empty")
	}

	params := map[string]any{
		"name": name,
	}

	resp, err := c.Call("bridge", "get_community", params)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, fmt.Errorf("community %s not found", name)
	}

	resMap, ok := resp.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected get_community response type: %T", resp)
	}
	return resMap, nil
}

// ListCommunities retrieves a list of communities.
func (c *Client) ListCommunities(last string, limit uint32, query string) ([]map[string]any, error) {
	if limit > 100 {
		return nil, fmt.Errorf("limit cannot exceed 100")
	}

	params := map[string]any{
		"limit": limit,
	}
	if last != "" {
		params["last"] = last
	}
	if query != "" {
		params["query"] = query
	}

	resp, err := c.Call("bridge", "list_communities", params)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return []map[string]any{}, nil
	}

	sliceVal, ok := resp.([]any)
	if !ok {
		return nil, fmt.Errorf("unexpected list_communities response type: %T", resp)
	}

	result := make([]map[string]any, len(sliceVal))
	for i, v := range sliceVal {
		m, ok := v.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("unexpected community element type: %T", v)
		}
		result[i] = m
	}
	return result, nil
}

// GetAccountNotifications retrieves notifications for a specific account.
func (c *Client) GetAccountNotifications(account string, limit uint32) ([]map[string]any, error) {
	if account == "" {
		return nil, fmt.Errorf("account name cannot be empty")
	}
	if limit > 100 {
		return nil, fmt.Errorf("limit cannot exceed 100")
	}

	params := map[string]any{
		"account": account,
		"limit":   limit,
	}

	resp, err := c.Call("bridge", "account_notifications", params)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return []map[string]any{}, nil
	}

	sliceVal, ok := resp.([]any)
	if !ok {
		return nil, fmt.Errorf("unexpected account_notifications response type: %T", resp)
	}

	result := make([]map[string]any, len(sliceVal))
	for i, v := range sliceVal {
		m, ok := v.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("unexpected notification element type: %T", v)
		}
		result[i] = m
	}
	return result, nil
}
