package types

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"strings"
)

// Amount represents a Hive asset (e.g., "100.000 HIVE").
type Amount struct {
	Value  float64
	Symbol string
}

// Wire symbol aliases for legacy STEEM compatibility
// HIVE was forked from STEEM, so wire format uses legacy names for signing
var WireSymbolAliases = map[string]string{
	"HIVE": "STEEM",
	"HBD":  "SBD",
}

// Asset metadata for serialization
var AssetMetadata = map[string]map[string]any{
	"HIVE":  {"precision": int64(3)},
	"HBD":   {"precision": int64(3)},
	"VESTS": {"precision": int64(6)},
}

// NewAmount creates a new Amount from a value and symbol.
func NewAmount(value float64, symbol string) *Amount {
	return &Amount{
		Value:  value,
		Symbol: symbol,
	}
}

// ParseAmount parses a string like "100.000 HIVE" into an Amount.
func ParseAmount(s string) (*Amount, error) {
	parts := strings.Fields(s)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid amount format: %s", s)
	}

	value, err := parseFloat(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid amount value: %s", parts[0])
	}

	return &Amount{
		Value:  value,
		Symbol: parts[1],
	}, nil
}

// String returns the string representation of the amount.
func (a *Amount) String() string {
	return fmt.Sprintf("%.3f %s", a.Value, a.Symbol)
}

// Bytes returns the binary representation of the amount for wire serialization.
// This handles the HIVE->STEEM and HBD->SBD conversion for signing.
func (a *Amount) Bytes() ([]byte, error) {
	// Get asset metadata
	meta, ok := AssetMetadata[a.Symbol]
	if !ok {
		return nil, fmt.Errorf("unknown asset symbol: %s", a.Symbol)
	}

	precision := meta["precision"].(int64)

	// Convert amount to satoshis (smallest unit)
	amountSatoshis := int64(math.Round(a.Value * math.Pow(10, float64(precision))))

	// Get wire symbol (for legacy STEEM compatibility)
	wireSymbol := WireSymbolAliases[a.Symbol]
	if wireSymbol == "" {
		wireSymbol = a.Symbol
	}

	// Build binary representation
	var buf bytes.Buffer

	// Write amount as little-endian int64
	if err := binary.Write(&buf, binary.LittleEndian, amountSatoshis); err != nil {
		return nil, err
	}

	// Write precision as uint8
	buf.WriteByte(byte(precision))

	// Write symbol (ASCII encoded, padded to 7 bytes)
	symbolBytes := []byte(wireSymbol)
	if len(symbolBytes) > 7 {
		return nil, fmt.Errorf("asset symbol must be 7 characters or fewer")
	}
	buf.Write(symbolBytes)
	// Pad with null bytes
	for i := len(symbolBytes); i < 7; i++ {
		buf.WriteByte(0)
	}

	return buf.Bytes(), nil
}

// parseFloat parses a string to float64.
func parseFloat(s string) (float64, error) {
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}
