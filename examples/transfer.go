package main

import (
	"fmt"
	"log"
	"os"

	"github.com/thecrazygm/nectar-go/account"
	"github.com/thecrazygm/nectar-go/client"
	"github.com/thecrazygm/nectar-go/transaction"
	"github.com/thecrazygm/nectar-go/wallet"
)

func main() {
	// Get the active key from environment
	activeWIF := os.Getenv("ACTIVE_WIF")
	if activeWIF == "" {
		log.Fatal("ACTIVE_WIF environment variable not set")
	}

	// Initialize the client with Hive nodes
	nodes := []string{
		"https://api.hive.blog",
		"https://api.hivecosystem.dev",
	}
	api := client.NewClient(nodes, 30)

	// Create account and wallet
	acc := account.NewAccount("thecrazygm", api)
	w := wallet.NewWallet()

	fmt.Println("=== Nectarlite Go Library - Transfer Example ===")
	fmt.Println()

	// Add the active key to the wallet
	fmt.Println("Adding active key to wallet...")
	if err := w.AddKey("thecrazygm", "active", activeWIF); err != nil {
		log.Fatalf("Error adding key to wallet: %v\n\nPlease provide a valid WIF private key via ACTIVE_WIF environment variable.\nExample: export ACTIVE_WIF=\"5Kxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx\"\n", err)
	}
	fmt.Println("✓ Key added successfully")
	fmt.Println()

	// Refresh account data
	fmt.Println("Fetching account data...")
	if err := acc.Refresh(); err != nil {
		log.Fatalf("Error refreshing account: %v", err)
	}
	fmt.Println("✓ Account data fetched successfully")
	fmt.Println()

	// Display account info
	fmt.Println("--- Account Information ---")
	fmt.Printf("Account: %s\n", acc.Name)
	if balance, ok := acc.Data["balance"].(string); ok {
		fmt.Printf("Current Balance: %s\n", balance)
	}

	// Create a transfer transaction
	fmt.Println()
	fmt.Println("--- Creating Transfer ---")
	tx := transaction.NewTransaction(api)

	transfer := &transaction.Transfer{
		From:   "thecrazygm",
		To:     "ecoinstant",
		Amount: "0.001 HIVE",
		Memo:   "Hello from golang!",
	}

	tx.AppendOp(transfer)
	fmt.Printf("Transfer prepared:\n")
	fmt.Printf("  From: %s\n", transfer.From)
	fmt.Printf("  To: %s\n", transfer.To)
	fmt.Printf("  Amount: %s\n", transfer.Amount)
	fmt.Printf("  Memo: %s\n\n", transfer.Memo)

	// Sign the transaction
	fmt.Println("--- Signing Transaction ---")
	fmt.Println("Signing with active key...")
	if err := w.Sign(tx, "thecrazygm", "active"); err != nil {
		log.Fatalf("Error signing transaction: %v\n\nCommon issues:\n  • get_transaction_hex API may not be available\n  • Network might be congested\n  • Node might be in maintenance\n  Try with a different node or wait a moment and retry.\n", err)
	}
	fmt.Printf("Transaction signed successfully\n")
	fmt.Printf("Signatures: %d\n\n", len(tx.Signatures))

	// Broadcast the transaction
	fmt.Println("--- Broadcasting Transaction ---")
	fmt.Println("Broadcasting to network...")
	result, err := tx.Broadcast()
	if err != nil {
		log.Fatalf("Error broadcasting transaction: %v", err)
	}

	fmt.Println("Transaction broadcast successfully!")
	fmt.Println()
	fmt.Printf("Result: %v\n", result)
	fmt.Println("\n=== Transfer Complete ===")
}
