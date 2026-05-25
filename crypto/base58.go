package crypto

import (
	"math/big"
	"strings"
)

const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

// Base58Encode encodes a byte slice into a Base58 string.
func Base58Encode(input []byte) string {
	x := new(big.Int).SetBytes(input)
	base := big.NewInt(58)
	zero := big.NewInt(0)
	mod := new(big.Int)
	var result []byte
	for x.Cmp(zero) > 0 {
		x.DivMod(x, base, mod)
		result = append(result, base58Alphabet[mod.Int64()])
	}
	// Leading zeros
	for _, b := range input {
		if b == 0x00 {
			result = append(result, base58Alphabet[0])
		} else {
			break
		}
	}
	// Reverse result
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return string(result)
}

// Base58Decode decodes a Base58 string into a byte slice.
func Base58Decode(input string) []byte {
	result := big.NewInt(0)
	base := big.NewInt(58)
	for i := 0; i < len(input); i++ {
		char := input[i]
		idx := strings.IndexByte(base58Alphabet, char)
		if idx < 0 {
			return nil
		}
		result.Mul(result, base)
		result.Add(result, big.NewInt(int64(idx)))
	}
	decoded := result.Bytes()
	// Leading zeros
	var leadingZeros []byte
	for i := 0; i < len(input); i++ {
		if input[i] == base58Alphabet[0] {
			leadingZeros = append(leadingZeros, 0x00)
		} else {
			break
		}
	}
	return append(leadingZeros, decoded...)
}
