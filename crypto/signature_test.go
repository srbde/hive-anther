package crypto

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/btcutil/base58"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
)

func generateTestWIF(t *testing.T) string {
	t.Helper()
	priv := [32]byte{}
	for i := range priv {
		priv[i] = byte(i + 1)
	}
	key, _ := btcec.PrivKeyFromBytes(priv[:])
	wif, err := btcutil.NewWIF(key, &chaincfg.MainNetParams, true)
	if err != nil {
		t.Fatalf("failed to create test wif: %v", err)
	}
	return wif.String()
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
	wifDecoded, err := btcutil.DecodeWIF(wif)
	if err != nil {
		t.Fatalf("failed to decode WIF: %v", err)
	}
	privKey := wifDecoded.PrivKey
	expectedPub := privKey.PubKey()

	digest := sha256.Sum256([]byte("hello world"))
	privKeySEC := secp256k1.PrivKeyFromBytes(privKey.Serialize())
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
	wifDecoded, err := btcutil.DecodeWIF(wif)
	if err != nil {
		t.Fatalf("failed to decode WIF: %v", err)
	}
	privKey := wifDecoded.PrivKey

	// Format expected pubkey
	pubBytes := privKey.PubKey().SerializeCompressed()
	checksum := btcutil.Hash160(pubBytes)
	payload := append(pubBytes, checksum[:4]...)
	expectedPubKeyStr := "STM" + base58.Encode(payload)

	digest := sha256.Sum256([]byte("hello world"))
	sigHex, err := SignTransactionBytesWithChainID(digest[:], wif, HiveChainID)
	if err != nil {
		t.Fatalf("failed to sign: %v", err)
	}

	// SignTransactionBytes prepend chain ID, but here digest was signed.
	// Actually SignTransactionBytesWithChainID appends chainID to the message, and hashes that.
	// Wait! Let's check SignTransactionBytesWithChainID:
	// message := append(chainIDBytes, txBytes...)
	// digest := sha256.Sum256(message)
	// So sigHex is the signature of sha256(chainID + digest).
	// Let's compute the correct digest:
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
