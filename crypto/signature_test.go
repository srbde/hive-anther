package crypto

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
)

func generateTestWIF(t *testing.T) string {
	t.Helper()
	priv := make([]byte, 32)
	for i := range priv {
		priv[i] = byte(i + 1)
	}
	payload := append([]byte{0x80}, priv...)
	payload = append(payload, 0x01) // Compressed WIF
	h1 := sha256.Sum256(payload)
	h2 := sha256.Sum256(h1[:])
	wifBytes := append(payload, h2[:4]...)
	return Base58Encode(wifBytes)
}

func TestSignTransactionHexWithChainID(t *testing.T) {
	sig, err := SignTransactionHexWithChainID("deadbeefcafebabe", generateTestWIF(t), HiveChainID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sig) != 130 {
		t.Fatalf("unexpected signature length: %d", len(sig))
	}
}

func TestSignTransactionHexWithChainIDValidation(t *testing.T) {
	t.Run("empty hex", func(t *testing.T) {
		if _, err := SignTransactionHex("   ", generateTestWIF(t)); err == nil {
			t.Fatalf("expected error for empty hex")
		}
	})

	t.Run("invalid chain id", func(t *testing.T) {
		if _, err := SignTransactionHexWithChainID("deadbeef", generateTestWIF(t), "invalid"); err == nil {
			t.Fatalf("expected error for invalid chain id")
		}
	})

	t.Run("invalid transaction hex", func(t *testing.T) {
		if _, err := SignTransactionHexWithChainID("zz", generateTestWIF(t), HiveChainID); err == nil {
			t.Fatalf("expected error for invalid transaction hex")
		}
	})

	t.Run("invalid wif", func(t *testing.T) {
		if _, err := SignTransactionHexWithChainID("deadbeef", "5aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", HiveChainID); err == nil {
			t.Fatalf("expected error for invalid wif")
		}
	})
}

func TestSignTransactionBytes(t *testing.T) {
	txBytes, err := hex.DecodeString("deadbeefcafeba")
	if err != nil {
		t.Fatalf("failed to decode hex: %v", err)
	}

	sig, err := SignTransactionBytes(txBytes, generateTestWIF(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sig) != 130 {
		t.Fatalf("unexpected signature length: %d", len(sig))
	}
}

func TestRecoverPublicKey(t *testing.T) {
	wif := generateTestWIF(t)
	privKeyBytes, err := DecodeWIF(wif)
	if err != nil {
		t.Fatalf("failed to decode WIF: %v", err)
	}
	privKeySEC := secp256k1.PrivKeyFromBytes(privKeyBytes)
	expectedPub := privKeySEC.PubKey()

	digest := sha256.Sum256([]byte("hello world"))
	compactSig := ecdsa.SignCompact(privKeySEC, digest[:], true)

	recoveredPub, compressed, err := ecdsa.RecoverCompact(compactSig, digest[:])
	if err != nil {
		t.Fatalf("RecoverCompact failed: %v", err)
	}
	if !compressed {
		t.Fatalf("expected compressed public key")
	}
	if !recoveredPub.IsEqual(expectedPub) {
		t.Fatalf("recovered public key does not match expected")
	}
}

func TestRecoverKeyFromSignature(t *testing.T) {
	wif := generateTestWIF(t)
	privKeyBytes, err := DecodeWIF(wif)
	if err != nil {
		t.Fatalf("failed to decode WIF: %v", err)
	}
	privKeySEC := secp256k1.PrivKeyFromBytes(privKeyBytes)

	// Format expected pubkey
	pubBytes := privKeySEC.PubKey().SerializeCompressed()
	checksum := ripemd160Checksum(pubBytes)
	payload := append(pubBytes, checksum...)
	expectedPubKeyStr := "STM" + Base58Encode(payload)

	digest := sha256.Sum256([]byte("hello world"))
	sigHex, err := SignTransactionBytesWithChainID(digest[:], wif, HiveChainID)
	if err != nil {
		t.Fatalf("failed to sign: %v", err)
	}

	chainIDBytes, _ := hex.DecodeString(HiveChainID)
	msg := append(chainIDBytes, digest[:]...)
	msgDigest := sha256.Sum256(msg)

	recoveredKeyStr, err := RecoverKeyFromSignature(sigHex, msgDigest[:])
	if err != nil {
		t.Fatalf("RecoverKeyFromSignature failed: %v", err)
	}

	if recoveredKeyStr != expectedPubKeyStr {
		t.Fatalf("expected key %s, got %s", expectedPubKeyStr, recoveredKeyStr)
	}
}

func TestWIFToPublicKey(t *testing.T) {
	wif := generateTestWIF(t)
	pubKeyStr, err := WIFToPublicKey(wif)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// It should start with "STM"
	if !strings.HasPrefix(pubKeyStr, "STM") {
		t.Errorf("expected public key to start with STM, got %s", pubKeyStr)
	}

	// Verify decoding an invalid WIF fails
	if _, err := WIFToPublicKey("invalid-wif"); err == nil {
		t.Errorf("expected error when decoding invalid WIF")
	}
}
