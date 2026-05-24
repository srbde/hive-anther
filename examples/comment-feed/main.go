package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/thecrazygm/anther/client"
)

func main() {
	// Initialize client
	nodes := []string{
		"https://api.hive.blog",
		"https://api.hivecosystem.dev",
	}
	api := client.NewClient(nodes, 30)

	fmt.Println("=== Anther Go Library - Live Comment Feed ===")
	fmt.Println("Streaming new posts and comments from the blockchain...")
	fmt.Println()

	ctx := context.Background()
	// Stream only "comment" operations from the latest head
	ops, errs := api.StreamOperations(ctx, 0, client.Latest, []string{"comment"})

	for {
		select {
		case op := <-ops:
			if op == nil || len(op.Op) < 2 {
				continue
			}

			opData, ok := op.Op[1].(map[string]any)
			if !ok {
				continue
			}

			author, _ := opData["author"].(string)
			permlink, _ := opData["permlink"].(string)
			parentAuthor, _ := opData["parent_author"].(string)
			title, _ := opData["title"].(string)
			body, _ := opData["body"].(string)

			// Clean body snippet
			bodySnippet := body
			if len(bodySnippet) > 100 {
				bodySnippet = bodySnippet[:100] + "..."
			}
			bodySnippet = strings.ReplaceAll(bodySnippet, "\n", " ")

			if parentAuthor == "" {
				// This is a top-level post
				fmt.Printf("📝 [POST] By @%s | Title: %q\n", author, title)
				fmt.Printf("   Link: https://hive.blog/@%s/%s\n", author, permlink)
				fmt.Printf("   Excerpt: %s\n\n", bodySnippet)
			} else {
				// This is a reply/comment
				fmt.Printf("💬 [REPLY] By @%s to @%s\n", author, parentAuthor)
				fmt.Printf("   Excerpt: %s\n\n", bodySnippet)
			}

		case err := <-errs:
			if err != nil {
				log.Printf("Stream error: %v", err)
				time.Sleep(2 * time.Second)
			}
		}
	}
}
