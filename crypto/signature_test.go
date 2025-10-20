package crypto

import (
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
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
