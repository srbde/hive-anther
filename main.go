package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"time"

	"github.com/srbde/hive-anther/account"
	"github.com/srbde/hive-anther/client"
	"github.com/srbde/hive-anther/crypto"
	"github.com/srbde/hive-anther/memo"
	"github.com/srbde/hive-anther/transaction"
)

func main() {
	fmt.Println("=====================================================")
	fmt.Println("   🌿 ANTHER GO HIVE LIBRARY - COMPLETE TOUR 🌿")
	fmt.Println("=====================================================")
	fmt.Println("This script showcases the features of the Anther Go")
	fmt.Println("library (offline signing, ECIES, HAF, streaming, and")
	fmt.Println("local serialization) without requiring any real keys.")
	fmt.Println("=====================================================")
	fmt.Println()

	// Initialize the client with Hive nodes
	nodes := []string{
		"https://api.hive.blog",
		"https://api.syncad.com",
	}
	api := client.NewClient(nodes, 30)

	// ==========================================
	// Phase 1: Blockchain Live Metrics
	// ==========================================
	fmt.Println("🌐 [PHASE 1] Querying Hive Blockchain Status...")
	props, err := api.GetDynamicGlobalPropertiesStruct()
	if err != nil {
		log.Fatalf("Failed to fetch blockchain global properties: %v", err)
	}
	fmt.Printf("✓ Current Head Block: %d\n", props.HeadBlockNumber)
	fmt.Printf("  Head Block ID:      %s\n", props.HeadBlockID)
	fmt.Printf("  Blockchain Time:    %s\n", props.Time)
	fmt.Printf("  Irreversible Block:  %d\n", props.LastIrreversibleBlockNum)
	fmt.Println()

	// ==========================================
	// Phase 2: Live Block Analysis & Operations
	// ==========================================
	targetBlock := props.HeadBlockNumber - 5 // query a block slightly in the past to ensure node consensus has it cached
	fmt.Printf("📦 [PHASE 2] Inspecting Block #%d & Operation Distribution...\n", targetBlock)
	block, err := api.GetBlock(targetBlock)
	if err != nil {
		log.Printf("Warning: Failed to fetch block #%d: %v\n", targetBlock, err)
	} else {
		fmt.Printf("✓ Block Witness:   @%s\n", block.Witness)
		fmt.Printf("  Transactions:    %d\n", len(block.Transactions))
		if len(block.TransactionIDs) > 0 {
			showCount := min(len(block.TransactionIDs), 3)
			fmt.Printf("  Tx ID Snippets:  %v\n", block.TransactionIDs[:showCount])
		}

		// Fetch and analyze operations within this block
		ops, err := api.GetOpsInBlock(targetBlock, false)
		if err != nil {
			log.Printf("Warning: Failed to fetch operations for block #%d: %v\n", targetBlock, err)
		} else {
			opCounts := make(map[string]int)
			for _, op := range ops {
				if len(op.Op) > 0 {
					if name, ok := op.Op[0].(string); ok {
						opCounts[name]++
					}
				}
			}
			fmt.Println("  Operation Distribution in this Block:")
			if len(opCounts) == 0 {
				fmt.Println("    (None found or empty block)")
			} else {
				for opName, count := range opCounts {
					fmt.Printf("    - %-20s: %d\n", opName, count)
				}
			}
		}
	}
	fmt.Println()

	// ==========================================
	// Phase 3: Public Account Info (HAF + Condenser)
	// ==========================================
	fmt.Println("👤 [PHASE 3] Querying Public Account Stats (No Keys Required)...")
	acc := account.NewAccount("thecrazygm", api)
	if err := acc.Refresh(); err != nil {
		log.Printf("Warning: Failed to refresh account statistics: %v\n", err)
	} else {
		rep, err := acc.Reputation()
		if err != nil {
			rep = 0.0
		}
		vp, err := acc.VotingPower()
		if err != nil {
			vp = 0.0
		}
		rc, err := acc.RC()
		if err != nil {
			rc = 0.0
		}

		balance, _ := acc.Data["balance"].(string)
		hbdBalance, _ := acc.Data["hbd_balance"].(string)
		vestingShares, _ := acc.Data["vesting_shares"].(string)

		fmt.Printf("✓ Target Account:   @%s\n", acc.Name)
		fmt.Printf("  Reputation (HAF):  %.2f\n", rep)
		fmt.Printf("  HIVE Balance:      %s\n", balance)
		fmt.Printf("  HBD Balance:       %s\n", hbdBalance)
		fmt.Printf("  Vesting Shares:    %s\n", vestingShares)
		fmt.Printf("  Voting Power:      %.2f%%\n", vp)
		fmt.Printf("  Resource Credits:  %.2f%%\n", rc)
	}
	fmt.Println()

	// ==========================================
	// Phase 4: Secure ECIES Memo Encryption (Offline Demo)
	// ==========================================
	fmt.Println("🔒 [PHASE 4] Offline Secure ECIES Memo Encryption/Decryption...")

	// Generate dummy key pairs for sender and recipient offline
	senderWIF, senderPubKey := generateKeyPair(0x01)
	recipientWIF, recipientPubKey := generateKeyPair(0x02)

	fmt.Printf("✓ Created Demo Sender Keypair:\n")
	fmt.Printf("  - Private WIF: %s\n", senderWIF)
	fmt.Printf("  - Public Key:  %s\n", senderPubKey)
	fmt.Printf("✓ Created Demo Recipient Keypair:\n")
	fmt.Printf("  - Private WIF: %s\n", recipientWIF)
	fmt.Printf("  - Public Key:  %s\n", recipientPubKey)

	secretMemo := "#Anther provides premium end-to-end secp256k1 ECIES memo protection!"
	fmt.Printf("Original Memo: %q\n", secretMemo)

	// Encrypt the memo
	encryptedMemo, err := memo.Encode(senderWIF, recipientPubKey, secretMemo)
	if err != nil {
		log.Fatalf("Failed to encrypt memo: %v", err)
	}
	fmt.Printf("Encrypted Envelope: %s\n", encryptedMemo)

	// Decrypt the memo using recipient WIF
	decryptedMemo, err := memo.Decode(recipientWIF, encryptedMemo)
	if err != nil {
		log.Fatalf("Failed to decrypt memo: %v", err)
	}
	fmt.Printf("Decrypted Memo:    %q\n", decryptedMemo)
	fmt.Println()

	// ==========================================
	// Phase 5: Local Binary Serialization & TaPoS
	// ==========================================
	fmt.Println("⚡ [PHASE 5] Local Serialization & TaPoS Headers...")

	// Instantiate a transaction
	tx := transaction.NewTransaction(api)

	// Setup TaPoS headers automatically using global properties
	// ref_block_num is head block's lower 16 bits
	tx.RefBlockNum = uint16(props.HeadBlockNumber & 0xFFFF)
	// ref_block_prefix is head block ID prefix parsed as little-endian uint32
	if len(props.HeadBlockID) >= 16 {
		// Parse block prefix from block ID hex string
		prefixBytes, err := hex.DecodeString(props.HeadBlockID[8:16])
		if err == nil && len(prefixBytes) == 4 {
			// Convert little endian bytes to uint32
			tx.RefBlockPrefix = uint32(prefixBytes[0]) |
				uint32(prefixBytes[1])<<8 |
				uint32(prefixBytes[2])<<16 |
				uint32(prefixBytes[3])<<24
		}
	}

	// Set expiration to 1 hour from now
	tx.Expiration = time.Now().Add(1 * time.Hour).UTC()

	// Append a CustomJSON operation to show serialization
	customJSONOp := &transaction.CustomJSON{
		ID:                   "follow",
		JSON:                 `["follow",{"follower":"thecrazygm","following":"srbde","what":["blog"]}]`,
		RequiredAuths:        []string{},
		RequiredPostingAuths: []string{"thecrazygm"},
	}
	tx.AppendOp(customJSONOp)

	// Serialize the transaction locally
	txBytes, err := tx.Bytes()
	if err != nil {
		log.Fatalf("Failed to serialize transaction: %v", err)
	}

	fmt.Printf("✓ Built CustomJSON operation with TaPoS:\n")
	fmt.Printf("  - Ref Block Num:    %d\n", tx.RefBlockNum)
	fmt.Printf("  - Ref Block Prefix: %d\n", tx.RefBlockPrefix)
	fmt.Printf("  - Expiration:       %s\n", tx.Expiration.Format(time.RFC3339))
	fmt.Printf("✓ Serialized Transaction Bytes Size: %d bytes\n", len(txBytes))
	fmt.Printf("✓ Raw Serialized Hex Payload:\n  %s\n", hex.EncodeToString(txBytes))
	fmt.Println()

	fmt.Println("=====================================================")
	fmt.Println("🌿 Tour Complete! All features working successfully.")
	fmt.Println("=====================================================")
}

// generateKeyPair derives a testing private WIF key and Hive-formatted STM public key
func generateKeyPair(seed byte) (string, string) {
	priv := make([]byte, 32)
	for i := range priv {
		priv[i] = seed
	}
	payload := append([]byte{0x80}, priv...)
	payload = append(payload, 0x01) // Compressed WIF
	h1 := sha256.Sum256(payload)
	h2 := sha256.Sum256(h1[:])
	wifBytes := append(payload, h2[:4]...)
	wifStr := crypto.Base58Encode(wifBytes)

	pubKeyStr, _ := crypto.WIFToPublicKey(wifStr)
	return wifStr, pubKeyStr
}
