package main

import (
	"fmt"
	"log"
	"os"

	"github.com/thecrazygm/anther/client"
	"github.com/thecrazygm/anther/transaction"
)

func main() {
	api := client.NewClient([]string{"https://api.hive.blog"}, 30)
	tx := transaction.NewTransaction(api)

	// Add operations
	tx.AppendOp(&transaction.Transfer{
		From:   "thecrazygm",
		To:     "ecoinstant",
		Amount: "0.001 HIVE",
		Memo:   "Sent with Anther 🌿",
	})

	// Sign transaction offline
	wif := os.Getenv("ACTIVE_WIF")
	if err := tx.Sign(wif); err != nil {
		log.Fatalf("failed to sign transaction: %v", err)
	}

	// Broadcast
	result, err := tx.Broadcast()
	if err != nil {
		log.Fatalf("failed to broadcast: %v", err)
	}
	fmt.Printf("Transaction broadcasted: %v\n", result)
}
