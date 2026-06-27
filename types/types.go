// Package types defines the core structures and types used across the Anther library for Hive blockchain data models and serialization.
package types

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"
)

// Time represents a timezone-naive UTC timestamp returned by the Hive API (format: "YYYY-MM-DDTHH:MM:SS").
type Time time.Time

// UnmarshalJSON customizes unmarshaling for Time to parse the Hive datetime format.
func (ht *Time) UnmarshalJSON(b []byte) error {
	s := string(b)
	// Strip quotes
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	if s == "" || s == "null" {
		*ht = Time(time.Time{})
		return nil
	}
	t, err := time.Parse("2006-01-02T15:04:05", s)
	if err != nil {
		return err
	}
	*ht = Time(t)
	return nil
}

// MarshalJSON customizes marshaling for Time to format as the Hive datetime string.
func (ht Time) MarshalJSON() ([]byte, error) {
	t := time.Time(ht)
	if t.IsZero() {
		return []byte("null"), nil
	}
	return fmt.Appendf(nil, `"%s"`, t.Format("2006-01-02T15:04:05")), nil
}

// Time converts types.Time back to the standard time.Time.
func (ht Time) Time() time.Time {
	return time.Time(ht)
}

// String returns the string representation of types.Time.
func (ht Time) String() string {
	return time.Time(ht).Format("2006-01-02T15:04:05")
}

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

// DisplaySymbolAliases maps wire symbols back to display symbols
var DisplaySymbolAliases = map[string]string{
	"STEEM": "HIVE",
	"SBD":   "HBD",
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

// AmountFromBytes deserializes an Amount from binary wire format.
// Format: int64 LE (satoshis) + uint8 (precision) + 7 bytes (symbol, null-padded).
func AmountFromBytes(r *bytes.Reader) (*Amount, error) {
	var satoshis int64
	if err := binary.Read(r, binary.LittleEndian, &satoshis); err != nil {
		return nil, fmt.Errorf("reading satoshis: %w", err)
	}

	var precision uint8
	if err := binary.Read(r, binary.LittleEndian, &precision); err != nil {
		return nil, fmt.Errorf("reading precision: %w", err)
	}

	symbolBuf := make([]byte, 7)
	if _, err := r.Read(symbolBuf); err != nil {
		return nil, fmt.Errorf("reading symbol: %w", err)
	}

	wireSymbol := strings.TrimRight(string(symbolBuf), "\x00")
	displaySymbol := wireSymbol
	if alias, ok := DisplaySymbolAliases[wireSymbol]; ok {
		displaySymbol = alias
	}

	value := float64(satoshis) / math.Pow(10, float64(precision))
	return &Amount{
		Value:  value,
		Symbol: displaySymbol,
	}, nil
}

// DynamicGlobalProperties represents the dynamic global properties of the Hive blockchain.
type DynamicGlobalProperties struct {
	HeadBlockNumber          uint32 `json:"head_block_number"`
	HeadBlockID              string `json:"head_block_id"`
	Time                     Time   `json:"time"`
	LastIrreversibleBlockNum uint32 `json:"last_irreversible_block_num"`
	TotalVestingFundHive     string `json:"total_vesting_fund_hive"`
	TotalVestingShares       string `json:"total_vesting_shares"`
}

// Manabar represents a player's voting or RC mana bar.
type Manabar struct {
	CurrentMana    float64 `json:"current_mana"`
	LastUpdateTime int64   `json:"last_update_time"`
}

// UnmarshalJSON customizes unmarshaling for Manabar to support both string and numeric current_mana.
func (m *Manabar) UnmarshalJSON(data []byte) error {
	type Alias Manabar
	aux := &struct {
		CurrentMana any `json:"current_mana"`
		*Alias
	}{
		Alias: (*Alias)(m),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if aux.CurrentMana != nil {
		switch v := aux.CurrentMana.(type) {
		case float64:
			m.CurrentMana = v
		case string:
			var f float64
			if _, err := fmt.Sscanf(v, "%f", &f); err != nil {
				return fmt.Errorf("invalid current_mana string: %w", err)
			}
			m.CurrentMana = f
		default:
			return fmt.Errorf("unexpected type for current_mana: %T", v)
		}
	}
	return nil
}

// AccountData represents Hive account query results.
type AccountData struct {
	Name          string  `json:"name"`
	VotingPower   float64 `json:"voting_power"`
	VotingManabar Manabar `json:"voting_manabar"`
	LastVoteTime  Time    `json:"last_vote_time"`
	Balance       string  `json:"balance"`
	HbdBalance    string  `json:"hbd_balance"`
	VestingShares string  `json:"vesting_shares"`
	Created       Time    `json:"created"`
}

// RCInfo represents a player's Resource Credit information.
type RCInfo struct {
	LastMana       int64     `json:"last_mana"`
	CurrentMana    int64     `json:"current_mana"`
	MaxMana        int64     `json:"max_mana"`
	LastUpdateTime time.Time `json:"last_update_time"`
	LastPercent    float64   `json:"last_percent"`
	CurrentPercent float64   `json:"current_percent"`
}

// BlockHeader represents the header of a Hive block.
type BlockHeader struct {
	Previous              string `json:"previous"`
	Timestamp             Time   `json:"timestamp"`
	Witness               string `json:"witness"`
	TransactionMerkleRoot string `json:"transaction_merkle_root"`
	Extensions            []any  `json:"extensions"`
}

// OperationTuple represents an operation inside a block transaction: [op_name, op_data]
type OperationTuple []any

// UnmarshalJSON customizes unmarshaling for OperationTuple to support both legacy array-based format [type, value]
// and Block API object-based format {"type": "...", "value": ...}.
func (ot *OperationTuple) UnmarshalJSON(data []byte) error {
	// 1. Try to unmarshal as an array (standard Condenser API style: [op_name, op_data])
	var arrayVal []any
	if err := json.Unmarshal(data, &arrayVal); err == nil {
		*ot = OperationTuple(arrayVal)
		return nil
	}

	// 2. Try to unmarshal as an object (Block API style: {"type": op_name, "value": op_data})
	var objVal struct {
		Type  string `json:"type"`
		Value any    `json:"value"`
	}
	if err := json.Unmarshal(data, &objVal); err == nil {
		*ot = OperationTuple{objVal.Type, objVal.Value}
		return nil
	}

	return fmt.Errorf("failed to unmarshal OperationTuple: invalid format")
}

// TransactionInBlock represents a transaction inside a block.
type TransactionInBlock struct {
	RefBlockNum    uint16           `json:"ref_block_num"`
	RefBlockPrefix uint32           `json:"ref_block_prefix"`
	Expiration     Time             `json:"expiration"`
	Operations     []OperationTuple `json:"operations"`
	Extensions     []any            `json:"extensions"`
	Signatures     []string         `json:"signatures"`
}

// Block represents a full signed Hive block.
type Block struct {
	BlockID               string               `json:"block_id"`
	Previous              string               `json:"previous"`
	Timestamp             Time                 `json:"timestamp"`
	Witness               string               `json:"witness"`
	TransactionMerkleRoot string               `json:"transaction_merkle_root"`
	Extensions            []any                `json:"extensions"`
	WitnessSignature      string               `json:"witness_signature"`
	Transactions          []TransactionInBlock `json:"transactions"`
	TransactionIDs        []string             `json:"transaction_ids"`
	SigningKey            string               `json:"signing_key"`
}

// AppliedOperation represents an operation applied to the blockchain.
type AppliedOperation struct {
	TrxID      string         `json:"trx_id"`
	Block      uint32         `json:"block"`
	TrxInBlock uint32         `json:"trx_in_block"`
	OpInTrx    uint32         `json:"op_in_trx"`
	VirtualOp  bool           `json:"virtual_op"`
	Op         OperationTuple `json:"op"`
}

// ChainProperties represents the blockchain configuration properties.
type ChainProperties struct {
	AccountCreationFee string `json:"account_creation_fee"`
	MaximumBlockSize   uint32 `json:"maximum_block_size"`
	HbdInterestRate    uint16 `json:"hbd_interest_rate"`
}

// Price represents Hive base/quote asset ratio.
type Price struct {
	Base  string `json:"base"`
	Quote string `json:"quote"`
}

// HistoryItem represents an entry in the account history.
type HistoryItem struct {
	Seq uint64
	Op  AppliedOperation
}

// UnmarshalJSON customizes unmarshaling for HistoryItem to parse the [seq, op] array format.
func (h *HistoryItem) UnmarshalJSON(data []byte) error {
	var arr []json.RawMessage
	if err := json.Unmarshal(data, &arr); err != nil {
		return err
	}
	if len(arr) != 2 {
		return fmt.Errorf("invalid history item format: expected 2 elements, got %d", len(arr))
	}
	if err := json.Unmarshal(arr[0], &h.Seq); err != nil {
		return err
	}
	if err := json.Unmarshal(arr[1], &h.Op); err != nil {
		return err
	}
	return nil
}

// VestingDelegation represents a vesting delegation on the Hive blockchain.
type VestingDelegation struct {
	ID                uint64 `json:"id"`
	Delegator         string `json:"delegator"`
	Delegatee         string `json:"delegatee"`
	VestingShares     string `json:"vesting_shares"`
	MinDelegationTime Time   `json:"min_delegation_time"`
}
