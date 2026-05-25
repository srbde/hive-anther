package main

import (
	"fmt"
	"log"
	"os"

	"github.com/thecrazygm/anther/client"
	"github.com/thecrazygm/anther/crypto"
)

func main() {
	wif := os.Getenv("ACTIVE_WIF")
	if wif == "" {
		log.Fatal("ACTIVE_WIF environment variable is not set")
	}

	// Derive public key from WIF using our library
	derivedPubKey, err := crypto.WIFToPublicKey(wif)
	if err != nil {
		log.Fatalf("Error deriving public key from WIF: %v", err)
	}

	fmt.Println("=== Anther Key Verification ===")
	fmt.Printf("Derived Active Public Key from WIF: %s\n\n", derivedPubKey)

	// Fetch actual account data from the blockchain
	nodes := []string{"https://api.hive.blog"}
	api := client.NewClient(nodes, 30)

	fmt.Println("Looking up account names for the derived public key...")
	refs, err := api.GetKeyReferences([]string{derivedPubKey})
	if err != nil {
		log.Fatalf("Error looking up key references: %v", err)
	}

	accountName := "thecrazygm" // fallback
	if len(refs) > 0 {
		accountName = refs[0]
		fmt.Printf("✓ Public key is registered to account: @%s\n\n", accountName)
	} else {
		fmt.Printf("⚠️ Public key is not registered to any account. Falling back to @%s\n\n", accountName)
	}

	fmt.Printf("Querying blockchain for @%s...\n", accountName)
	resp, err := api.Call("condenser_api", "get_accounts", []any{[]string{accountName}})
	if err != nil {
		log.Fatalf("Error querying account: %v", err)
	}

	accounts, ok := resp.([]any)
	if !ok || len(accounts) == 0 {
		log.Fatal("Account not found")
	}

	acc, ok := accounts[0].(map[string]any)
	if !ok {
		log.Fatal("Invalid account response format")
	}

	fmt.Printf("\nRegistered Public Keys on @%s:\n", accountName)

	// Print active authority keys
	if active, ok := acc["active"].(map[string]any); ok {
		if keyAuths, ok := active["key_auths"].([]any); ok {
			fmt.Println("Active Authority Keys:")
			for _, ka := range keyAuths {
				if pair, ok := ka.([]any); ok && len(pair) > 0 {
					fmt.Printf("  - %v (weight: %v)\n", pair[0], pair[1])
				}
			}
		}
	}

	// Print posting authority keys
	if posting, ok := acc["posting"].(map[string]any); ok {
		if keyAuths, ok := posting["key_auths"].([]any); ok {
			fmt.Println("Posting Authority Keys:")
			for _, ka := range keyAuths {
				if pair, ok := ka.([]any); ok && len(pair) > 0 {
					fmt.Printf("  - %v (weight: %v)\n", pair[0], pair[1])
				}
			}
		}
	}

	// Print owner authority keys
	if owner, ok := acc["owner"].(map[string]any); ok {
		if keyAuths, ok := owner["key_auths"].([]any); ok {
			fmt.Println("Owner Authority Keys:")
			for _, ka := range keyAuths {
				if pair, ok := ka.([]any); ok && len(pair) > 0 {
					fmt.Printf("  - %v (weight: %v)\n", pair[0], pair[1])
				}
			}
		}
	}

	// Print memo key
	if memoKey, ok := acc["memo_key"].(string); ok {
		fmt.Printf("Memo Public Key:\n  - %s\n", memoKey)
	}

	fmt.Println("\n==============================")
	// Check if derived matches active
	matched := false
	if active, ok := acc["active"].(map[string]any); ok {
		if keyAuths, ok := active["key_auths"].([]any); ok {
			for _, ka := range keyAuths {
				if pair, ok := ka.([]any); ok && len(pair) > 0 {
					if pair[0] == derivedPubKey {
						matched = true
					}
				}
			}
		}
	}
	if matched {
		fmt.Println("🎉 MATCH SUCCESSFUL! The derived public key matches a registered Active Key on the blockchain.")
	} else {
		fmt.Println("⚠️ MATCH FAILED! The derived public key does not match any registered Active Key.")
	}
}
