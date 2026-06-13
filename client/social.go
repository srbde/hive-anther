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

// GetUnreadNotificationsCount retrieves the unread notifications count for a specific account.
func (c *Client) GetUnreadNotificationsCount(account string) (int, error) {
	if account == "" {
		return 0, fmt.Errorf("account name cannot be empty")
	}

	resp, err := c.Call("bridge", "unread_notifications", map[string]any{"account": account})
	if err != nil {
		return 0, err
	}
	if resp == nil {
		return 0, nil
	}

	resMap, ok := resp.(map[string]any)
	if !ok {
		return 0, fmt.Errorf("unexpected unread_notifications response type: %T", resp)
	}

	unreadVal, _ := resMap["unread"].(float64)
	return int(unreadVal), nil
}

// GetUnreadNotifications retrieves only unread notifications for a specific account.
// It first fetches the unread count and then limits the retrieved notifications to that count.
func (c *Client) GetUnreadNotifications(account string, limit uint32) ([]map[string]any, error) {
	if account == "" {
		return nil, fmt.Errorf("account name cannot be empty")
	}
	if limit > 100 {
		return nil, fmt.Errorf("limit cannot exceed 100")
	}

	unread, err := c.GetUnreadNotificationsCount(account)
	if err != nil {
		return nil, err
	}

	if unread <= 0 {
		return []map[string]any{}, nil
	}

	fetchLimit := min(uint32(unread), limit)

	return c.GetAccountNotifications(account, fetchLimit)
}

// GetContentReplies retrieves direct replies for a specific post/comment.
func (c *Client) GetContentReplies(author string, permlink string) ([]map[string]any, error) {
	if author == "" {
		return nil, fmt.Errorf("author cannot be empty")
	}
	if permlink == "" {
		return nil, fmt.Errorf("permlink cannot be empty")
	}

	resp, err := c.Call("condenser_api", "get_content_replies", []string{author, permlink})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return []map[string]any{}, nil
	}

	sliceVal, ok := resp.([]any)
	if !ok {
		return nil, fmt.Errorf("unexpected get_content_replies response type: %T", resp)
	}

	result := make([]map[string]any, len(sliceVal))
	for i, v := range sliceVal {
		m, ok := v.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("unexpected reply element type: %T", v)
		}
		result[i] = m
	}
	return result, nil
}

// GetDiscussion retrieves the full discussion thread for a post.
func (c *Client) GetDiscussion(author string, permlink string) (map[string]any, error) {
	if author == "" {
		return nil, fmt.Errorf("author cannot be empty")
	}
	if permlink == "" {
		return nil, fmt.Errorf("permlink cannot be empty")
	}

	params := map[string]any{
		"author":   author,
		"permlink": permlink,
	}

	resp, err := c.Call("bridge", "get_discussion", params)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return map[string]any{}, nil
	}

	resMap, ok := resp.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected get_discussion response type: %T", resp)
	}
	return resMap, nil
}

// GetFollowCount retrieves the follower and following counts for an account.
func (c *Client) GetFollowCount(account string) (map[string]any, error) {
	if account == "" {
		return nil, fmt.Errorf("account name cannot be empty")
	}

	resp, err := c.Call("condenser_api", "get_follow_count", []string{account})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return map[string]any{}, nil
	}

	resMap, ok := resp.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected get_follow_count response type: %T", resp)
	}
	return resMap, nil
}
