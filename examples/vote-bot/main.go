package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/thecrazygm/anther/client"
	"github.com/thecrazygm/anther/transaction"
)

func main() {
	// Bot configuration from environment variables
	botUser := os.Getenv("BOT_USER")
	postingKey := os.Getenv("POSTING_KEY")
	followUser := os.Getenv("FOLLOW_USER")
	voteWeightStr := os.Getenv("VOTE_WEIGHT")

	if botUser == "" {
		botUser = "your-bot-name"
	}
	if followUser == "" {
		followUser = "thecrazygm"
	}

	voteWeight := int16(10000) // Default 100%
	if voteWeightStr != "" {
		parsedWeight, err := strconv.Atoi(voteWeightStr)
		if err == nil {
			voteWeight = int16(parsedWeight)
		}
	}

	if postingKey == "" {
		fmt.Println("Example ready. Set POSTING_KEY environment variable to start voting.")
		fmt.Printf("Default config: Bot %q following %q with weight %.2f%%\n\n", botUser, followUser, float64(voteWeight)/100.0)
		fmt.Println("To run, set env variables:")
		fmt.Println("  export POSTING_KEY=\"5Jxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx\"")
		fmt.Println("  export BOT_USER=\"mybotaccount\"")
		fmt.Println("  export FOLLOW_USER=\"thecrazygm\"")
		fmt.Println("  go run examples/vote-bot/main.go")
		return
	}

	// Initialize Client
	nodes := []string{
		"https://api.hive.blog",
		"https://api.hivecosystem.dev",
	}
	api := client.NewClient(nodes, 30)

	fmt.Printf("🌿 Anther Vote Bot active. Following %q with %.2f%% weight\n", followUser, float64(voteWeight)/100.0)

	ctx := context.Background()
	// Stream only "vote" operations
	ops, errs := api.StreamOperations(ctx, 0, client.Latest, []string{"vote"})

	for {
		select {
		case op := <-ops:
			if op == nil || len(op.Op) < 2 {
				continue
			}

			// op.Op is [string, map[string]any]
			opData, ok := op.Op[1].(map[string]any)
			if !ok {
				continue
			}

			voter, _ := opData["voter"].(string)
			author, _ := opData["author"].(string)
			permlink, _ := opData["permlink"].(string)
			weightVal, _ := opData["weight"].(float64)

			if voter == followUser {
				fmt.Printf("Detected vote by %s on @%s/%s. Mirroring vote...\n", voter, author, permlink)

				// Determine mirrored weight
				targetWeight := voteWeight
				if weightVal < 0 {
					targetWeight = -voteWeight
				}

				// Build mirror vote transaction
				tx := transaction.NewTransaction(api)
				tx.AppendOp(&transaction.Vote{
					Voter:    botUser,
					Author:   author,
					Permlink: permlink,
					Weight:   targetWeight,
				})

				// Sign with posting key
				if err := tx.Sign(postingKey); err != nil {
					log.Printf("❌ Failed to sign mirror vote: %v", err)
					continue
				}

				// Broadcast
				res, err := tx.Broadcast()
				if err != nil {
					log.Printf("❌ Failed to broadcast mirror vote: %v", err)
				} else {
					fmt.Printf("✅ Successfully voted for @%s/%s! Result: %v\n", author, permlink, res)
				}
			}

		case err := <-errs:
			if err != nil {
				log.Printf("Stream error: %v", err)
				time.Sleep(2 * time.Second) // backoff before continuation
			}
		}
	}
}
