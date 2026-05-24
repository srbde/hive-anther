package crypto

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/btcutil/base58"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
)

const HiveChainID = "beeab0de00000000000000000000000000000000000000000000000000000000"

// SignTransactionBytes signs transaction bytes with a private WIF key and the default chain ID.
func SignTransactionBytes(txBytes []byte, wif string) (string, error) {
	return SignTransactionBytesWithChainID(txBytes, wif, HiveChainID)
}

// SignTransactionBytesWithChainID signs transaction bytes with a private WIF key and a custom chain ID.
func SignTransactionBytesWithChainID(txBytes []byte, wif string, chainID string) (string, error) {
	chainIDBytes, err := hex.DecodeString(chainID)
	if err != nil {
		return "", fmt.Errorf("invalid chain ID: %w", err)
	}

	message := append(chainIDBytes, txBytes...)
	digest := sha256.Sum256(message)

	N := new(big.Int)
	N.SetString("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141", 16)
	e := new(big.Int).SetBytes(digest[:])
	e.Mod(e, N)

	wifDecoded, err := btcutil.DecodeWIF(wif)
	if err != nil {
		return "", err
	}

	privKeyBytes := wifDecoded.PrivKey.Serialize()
	privKeySEC := secp256k1.PrivKeyFromBytes(privKeyBytes)

	compactSig := ecdsa.SignCompact(privKeySEC, digest[:], true)

	recoveryByte := compactSig[0]
	rBytes := compactSig[1:33]
	sBytes := compactSig[33:65]

	recoveryID := int(recoveryByte) - 31

	s := new(big.Int).SetBytes(sBytes)
	nDiv2 := new(big.Int)
	nDiv2.SetString("7FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF5D576E7357A4501DDFE92F46681B20A0", 16)

	if s.Cmp(nDiv2) > 0 {
		s = new(big.Int).Sub(N, s)
		sBytes = s.Bytes()
		if len(sBytes) < 32 {
			sBytes = append(make([]byte, 32-len(sBytes)), sBytes...)
		}

		recoveryID = recoveryID ^ 1
	}

	canonical := append(rBytes, sBytes...)
	finalSig := append([]byte{byte(27 + 4 + recoveryID)}, canonical...)

	return hex.EncodeToString(finalSig), nil
}

// SignTransactionHex signs a transaction hex string for the Hive blockchain using the
// default chain ID.
func SignTransactionHex(txHex string, wif string) (string, error) {
	return SignTransactionHexWithChainID(txHex, wif, HiveChainID)
}

// SignTransactionHexWithChainID signs a transaction hex string with the provided chain ID.
func SignTransactionHexWithChainID(txHex string, wif string, chainID string) (string, error) {
	trimmed := strings.TrimSpace(txHex)
	if trimmed == "" {
		return "", errors.New("empty transaction hex")
	}

	chainIDBytes, err := hex.DecodeString(chainID)
	if err != nil {
		return "", fmt.Errorf("invalid chain ID: %w", err)
	}

	txHexBytes, err := hex.DecodeString(trimmed)
	if err != nil {
		return "", fmt.Errorf("invalid transaction hex: %w", err)
	}

	if len(trimmed) > 2 {
		txHexBytes, err = hex.DecodeString(trimmed[:len(trimmed)-2])
		if err != nil {
			return "", fmt.Errorf("invalid transaction hex payload: %w", err)
		}
	}

	message := append(chainIDBytes, txHexBytes...)

	digest := sha256.Sum256(message)

	N := new(big.Int)
	N.SetString("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141", 16)
	e := new(big.Int).SetBytes(digest[:])
	e.Mod(e, N)

	wifDecoded, err := btcutil.DecodeWIF(wif)
	if err != nil {
		return "", err
	}

	privKeyBytes := wifDecoded.PrivKey.Serialize()
	privKeySEC := secp256k1.PrivKeyFromBytes(privKeyBytes)

	compactSig := ecdsa.SignCompact(privKeySEC, digest[:], true)

	recoveryByte := compactSig[0]
	rBytes := compactSig[1:33]
	sBytes := compactSig[33:65]

	recoveryID := int(recoveryByte) - 31

	s := new(big.Int).SetBytes(sBytes)
	nDiv2 := new(big.Int)
	nDiv2.SetString("7FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF5D576E7357A4501DDFE92F46681B20A0", 16)

	if s.Cmp(nDiv2) > 0 {
		s = new(big.Int).Sub(N, s)
		sBytes = s.Bytes()
		if len(sBytes) < 32 {
			sBytes = append(make([]byte, 32-len(sBytes)), sBytes...)
		}

		recoveryID = recoveryID ^ 1
	}

	canonical := append(rBytes, sBytes...)
	finalSig := append([]byte{byte(27 + 4 + recoveryID)}, canonical...)

	return hex.EncodeToString(finalSig), nil
}

// RecoverKeyFromSignature recovers the Hive public key string from a 65-byte hex-encoded signature
// and a 32-byte message digest.
func RecoverKeyFromSignature(signatureHex string, digest []byte) (string, error) {
	sigBytes, err := hex.DecodeString(signatureHex)
	if err != nil {
		return "", fmt.Errorf("failed to decode signature hex: %w", err)
	}
	if len(sigBytes) != 65 {
		return "", fmt.Errorf("invalid signature length: expected 65, got %d", len(sigBytes))
	}

	// Hive recovery byte has 31-34 or 27-30.
	// decred/secp256k1 ecdsa expects standard compact signature format:
	// recovery byte (27-30 or 31-34) + R (32 bytes) + S (32 bytes).
	// Normalize recovery byte to standard ecdsa compact signature format.
	recByte := sigBytes[0]
	if recByte >= 31 {
		// keep it >= 31 (indicates compressed)
	} else if recByte >= 27 {
		sigBytes[0] = recByte - 27 + 31
	}

	pub, _, err := ecdsa.RecoverCompact(sigBytes, digest)
	if err != nil {
		return "", fmt.Errorf("failed to recover public key: %w", err)
	}

	pubBytes := pub.SerializeCompressed()
	checksum := btcutil.Hash160(pubBytes)
	payload := append(pubBytes, checksum[:4]...)
	return "STM" + base58.Encode(payload), nil
}
