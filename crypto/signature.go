package crypto

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
	"golang.org/x/crypto/ripemd160"
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

	privKeyBytes, err := DecodeWIF(wif)
	if err != nil {
		return "", err
	}

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

	privKeyBytes, err := DecodeWIF(wif)
	if err != nil {
		return "", err
	}

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
	checksum := ripemd160Checksum(pubBytes)
	payload := append(pubBytes, checksum...)
	return "STM" + Base58Encode(payload), nil
}

// WIFToPublicKey derives the Hive-formatted public key ("STM...") from a private WIF key.
func WIFToPublicKey(wif string) (string, error) {
	privKeyBytes, err := DecodeWIF(wif)
	if err != nil {
		return "", fmt.Errorf("invalid WIF key: %w", err)
	}

	privKeySEC := secp256k1.PrivKeyFromBytes(privKeyBytes)
	pubBytes := privKeySEC.PubKey().SerializeCompressed()
	checksum := ripemd160Checksum(pubBytes)
	payload := append(pubBytes, checksum...)
	return "STM" + Base58Encode(payload), nil
}

// DecodeWIF decodes a private key in WIF format and returns the raw 32-byte private key.
func DecodeWIF(wif string) ([]byte, error) {
	decoded := Base58Decode(wif)
	if len(decoded) < 5 {
		return nil, fmt.Errorf("invalid WIF length")
	}

	// Double SHA256 checksum validation
	data := decoded[:len(decoded)-4]
	chk := decoded[len(decoded)-4:]

	h1 := sha256.Sum256(data)
	h2 := sha256.Sum256(h1[:])

	if !bytes.Equal(chk, h2[:4]) {
		return nil, fmt.Errorf("WIF checksum mismatch")
	}

	if data[0] != 0x80 {
		return nil, fmt.Errorf("invalid WIF version byte: expected 0x80, got 0x%02x", data[0])
	}

	// Private key bytes are after the version byte
	// If it is compressed, there is a trailing 0x01 byte at the end of the private key
	// payload, which we should exclude.
	privKeyBytes := data[1:]
	if len(privKeyBytes) == 33 && privKeyBytes[32] == 0x01 {
		privKeyBytes = privKeyBytes[:32]
	}
	if len(privKeyBytes) != 32 {
		return nil, fmt.Errorf("invalid private key length inside WIF: expected 32, got %d", len(privKeyBytes))
	}

	return privKeyBytes, nil
}

// ripemd160Checksum returns the first 4 bytes of the RIPEMD160 hash of the input data.
func ripemd160Checksum(data []byte) []byte {
	h := ripemd160.New()
	h.Write(data)
	return h.Sum(nil)[:4]
}
