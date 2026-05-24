package main

import (
	"fmt"
	"log"
	"os"
	"time"

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

	resultMap, ok := result.(map[string]any)
	if !ok {
		log.Fatalf("failed to parse broadcast result: %v", result)
	}

	trxID, _ := resultMap["id"].(string)
	fmt.Printf("Success! Transaction broadcasted.\n")
	fmt.Printf("Transaction ID: %s\n", trxID)

	fmt.Printf("Polling for block inclusion")
	var foundTx any
	for i := 0; i < 15; i++ {
		txData, err := api.GetTransaction(trxID)
		if err == nil && txData != nil {
			foundTx = txData
			break
		}
		fmt.Print(".")
		time.Sleep(3 * time.Second)
	}
	fmt.Println()

	if foundTx != nil {
		fmt.Printf("Transaction found in blockchain!\n")
		fmt.Printf("Full Result: %+v\n", foundTx)
	} else {
		fmt.Printf("Transaction not found within timeout.\n")
	}
}
