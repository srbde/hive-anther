package main

import (
	"fmt"
	"log"

	"github.com/thecrazygm/anther/memo"
)

func main() {
	// These are sample/test WIF keys and public keys (do not use in real wallets)
	senderWIF := "5JdeC9P7Pbd1uGdFVEsJ41EkEnADbbHGq6p1BwFxm6txNBsQnsw"
	recipientPubKey := "STM8m5UgaFAAYQRuaNejYdS8FVLVp9Ss3K1qAVk5de6F8s3HnVbvA"

	// Plaintext message starting with "#" to trigger encryption
	plaintext := "#Hello Nectar and Pollen! This is a secret message sent from Anther Go! 🌿🔒"

	fmt.Println("=== Anther Go Library - ECIES Memo Encryption Demo ===")
	fmt.Println()
	fmt.Printf("Original message: %s\n\n", plaintext)

	// 1. Encrypt the memo
	fmt.Println("Encrypting memo using Diffie-Hellman + AES-256-CBC...")
	ciphertext, err := memo.Encode(senderWIF, recipientPubKey, plaintext)
	if err != nil {
		log.Fatalf("Encryption failed: %v", err)
	}

	fmt.Printf("✓ Encrypted payload (Base58): %s\n\n", ciphertext)

	// 2. Decrypt the memo using sender key (we can decrypt since we know recipient's key)
	fmt.Println("Decrypting memo using sender private key...")
	decryptedSender, err := memo.Decode(senderWIF, ciphertext)
	if err != nil {
		log.Fatalf("Decryption failed by sender: %v", err)
	}
	fmt.Printf("✓ Decrypted by sender: %s\n\n", decryptedSender)

	// Note: Since this is a demo, in real life the recipient would use their WIF private key
	// to decrypt the memo. Both parties can decrypt because ECDH computes the same shared secret.
	fmt.Println("=== Memo Encryption Demo Complete ===")
}
