package client

import (
	"github.com/thecrazygm/anther/transaction"
)

// BroadcastVote creates, signs, and broadcasts a vote transaction.
func (c *Client) BroadcastVote(voter, author, permlink string, weight int16, wif string) (any, error) {
	tx := transaction.NewTransaction(c)
	tx.AppendOp(&transaction.Vote{
		Voter:    voter,
		Author:   author,
		Permlink: permlink,
		Weight:   weight,
	})
	if err := tx.Sign(wif); err != nil {
		return nil, err
	}
	return tx.Broadcast()
}

// BroadcastTransfer creates, signs, and broadcasts a transfer transaction.
func (c *Client) BroadcastTransfer(from, to, amount, memo string, wif string) (any, error) {
	tx := transaction.NewTransaction(c)
	tx.AppendOp(&transaction.Transfer{
		From:   from,
		To:     to,
		Amount: amount,
		Memo:   memo,
	})
	if err := tx.Sign(wif); err != nil {
		return nil, err
	}
	return tx.Broadcast()
}

// BroadcastComment creates, signs, and broadcasts a comment (post or reply) transaction.
func (c *Client) BroadcastComment(author, permlink, parentAuthor, parentPermlink, title, body, jsonMetadata string, wif string) (any, error) {
	tx := transaction.NewTransaction(c)
	tx.AppendOp(&transaction.Comment{
		Author:         author,
		Permlink:       permlink,
		ParentAuthor:   parentAuthor,
		ParentPermlink: parentPermlink,
		Title:          title,
		Body:           body,
		JSONMetadata:   jsonMetadata,
	})
	if err := tx.Sign(wif); err != nil {
		return nil, err
	}
	return tx.Broadcast()
}

// BroadcastCustomJSON creates, signs, and broadcasts a custom JSON transaction using posting authority.
func (c *Client) BroadcastCustomJSON(id, jsonString string, requiredPostingAuths []string, wif string) (any, error) {
	tx := transaction.NewTransaction(c)
	tx.AppendOp(&transaction.CustomJSON{
		ID:                   id,
		JSON:                 jsonString,
		RequiredAuths:        []string{},
		RequiredPostingAuths: requiredPostingAuths,
	})
	if err := tx.Sign(wif); err != nil {
		return nil, err
	}
	return tx.Broadcast()
}
