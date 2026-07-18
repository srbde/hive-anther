// Package types defines the core structures and types used across the Anther library for Hive blockchain data models and serialization.
package types

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
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
	Name          string    `json:"name"`
	VotingPower   float64   `json:"voting_power"`
	VotingManabar Manabar   `json:"voting_manabar"`
	LastVoteTime  Time      `json:"last_vote_time"`
	Balance       string    `json:"balance"`
	HbdBalance    string    `json:"hbd_balance"`
	VestingShares string    `json:"vesting_shares"`
	Created       Time      `json:"created"`
	Owner         Authority `json:"owner"`
	Active        Authority `json:"active"`
	Posting       Authority `json:"posting"`
	MemoKey       string    `json:"memo_key"`
}

// Authority represents Hive's weighted-key/account voting threshold model, used both for
// interpreting an account's owner/active/posting authorities (read side) and for building
// operations that set them, such as account_update (write side, see the transaction package).
type Authority struct {
	WeightThreshold uint32
	AccountAuths    map[string]uint16
	KeyAuths        map[string]uint16
}

// authorityWire mirrors Hive's actual JSON wire format for an Authority, where account_auths
// and key_auths are arrays of [name-or-key, weight] tuples rather than JSON objects.
type authorityWire struct {
	WeightThreshold uint32   `json:"weight_threshold"`
	AccountAuths    [][2]any `json:"account_auths"`
	KeyAuths        [][2]any `json:"key_auths"`
}

func tuplesToWeightMap(tuples [][2]any) (map[string]uint16, error) {
	m := make(map[string]uint16, len(tuples))
	for _, t := range tuples {
		key, ok := t[0].(string)
		if !ok {
			return nil, fmt.Errorf("unexpected authority tuple key type: %T", t[0])
		}
		weight, ok := t[1].(float64)
		if !ok {
			return nil, fmt.Errorf("unexpected authority tuple weight type: %T", t[1])
		}
		m[key] = uint16(weight)
	}
	return m, nil
}

func weightMapToTuples(m map[string]uint16) [][2]any {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	tuples := make([][2]any, 0, len(m))
	for _, k := range keys {
		tuples = append(tuples, [2]any{k, m[k]})
	}
	return tuples
}

// UnmarshalJSON customizes unmarshaling for Authority to parse Hive's tuple-array wire format
// for account_auths/key_auths into weight maps.
func (a *Authority) UnmarshalJSON(data []byte) error {
	var w authorityWire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	accountAuths, err := tuplesToWeightMap(w.AccountAuths)
	if err != nil {
		return fmt.Errorf("failed to parse account_auths: %w", err)
	}
	keyAuths, err := tuplesToWeightMap(w.KeyAuths)
	if err != nil {
		return fmt.Errorf("failed to parse key_auths: %w", err)
	}
	a.WeightThreshold = w.WeightThreshold
	a.AccountAuths = accountAuths
	a.KeyAuths = keyAuths
	return nil
}

// MarshalJSON customizes marshaling for Authority to emit Hive's tuple-array wire format for
// account_auths/key_auths instead of the default JSON object encoding of a Go map.
func (a Authority) MarshalJSON() ([]byte, error) {
	return json.Marshal(authorityWire{
		WeightThreshold: a.WeightThreshold,
		AccountAuths:    weightMapToTuples(a.AccountAuths),
		KeyAuths:        weightMapToTuples(a.KeyAuths),
	})
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

// Errors returned by typed operation helpers when a matching operation cannot
// be decoded without guessing at the Hive wire representation.
var (
	ErrMalformedOperationTuple = errors.New("malformed operation tuple")
	ErrMissingOperationField   = errors.New("missing operation field")
	ErrWrongOperationFieldType = errors.New("wrong operation field type")
	ErrMalformedAmount         = errors.New("malformed operation amount")
	ErrMalformedAuthArray      = errors.New("malformed authorization array")
)

// TransferOperation contains the Hive-generic fields of a transfer operation.
type TransferOperation struct {
	From   string
	To     string
	Amount string
	Memo   string
}

// CustomJSONOperation contains the Hive-generic fields of a custom_json operation.
type CustomJSONOperation struct {
	ID                   string
	RequiredAuths        []string
	RequiredPostingAuths []string
	JSON                 string
}

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

// CustomJSONID returns the id field of a custom_json operation tuple, if this tuple represents
// a custom_json operation with a string id. The second return value is false for any other
// operation type or if the id field is missing/non-string.
func (ot OperationTuple) CustomJSONID() (string, bool) {
	opType, data, ok := operationData(ot, "custom_json")
	if !ok || opType != "custom_json" {
		return "", false
	}
	id, ok := data["id"].(string)
	return id, ok
}

// Transfer extracts a typed transfer operation. It returns (false, nil) for
// unrelated operations and distinguishes malformed matching operations with an
// error.
func (ot OperationTuple) Transfer() (TransferOperation, bool, error) {
	var result TransferOperation
	_, data, ok, err := matchingOperationData(ot, "transfer")
	if err != nil || !ok {
		return result, ok, err
	}

	if result.From, err = requiredString(data, "from"); err != nil {
		return result, true, err
	}
	if result.To, err = requiredString(data, "to"); err != nil {
		return result, true, err
	}
	if result.Amount, err = requiredString(data, "amount"); err != nil {
		return result, true, err
	}
	if _, parseErr := ParseAmount(result.Amount); parseErr != nil {
		return result, true, fmt.Errorf("%w: %v", ErrMalformedAmount, parseErr)
	}
	if memo, present := data["memo"]; present {
		var memoOK bool
		result.Memo, memoOK = memo.(string)
		if !memoOK {
			return result, true, fmt.Errorf("%w: %q", ErrWrongOperationFieldType, "memo")
		}
	}
	return result, true, nil
}

// CustomJSON extracts a typed custom_json operation without interpreting its
// application-specific JSON payload.
func (ot OperationTuple) CustomJSON() (CustomJSONOperation, bool, error) {
	var result CustomJSONOperation
	_, data, ok, err := matchingOperationData(ot, "custom_json")
	if err != nil || !ok {
		return result, ok, err
	}
	if result.ID, err = requiredString(data, "id"); err != nil {
		return result, true, err
	}
	if result.JSON, err = requiredString(data, "json"); err != nil {
		return result, true, err
	}
	if result.RequiredAuths, err = stringArray(data, "required_auths"); err != nil {
		return result, true, err
	}
	if result.RequiredPostingAuths, err = stringArray(data, "required_posting_auths"); err != nil {
		return result, true, err
	}
	return result, true, nil
}

func operationData(ot OperationTuple, want string) (string, map[string]any, bool) {
	opType, data, ok, _ := matchingOperationData(ot, want)
	return opType, data, ok
}

func matchingOperationData(ot OperationTuple, want string) (string, map[string]any, bool, error) {
	if len(ot) != 2 {
		return "", nil, false, fmt.Errorf("%w: expected [type, value]", ErrMalformedOperationTuple)
	}
	opType, ok := ot[0].(string)
	if !ok {
		return "", nil, false, fmt.Errorf("%w: operation type", ErrWrongOperationFieldType)
	}
	if opType != want {
		return opType, nil, false, nil
	}
	data, ok := ot[1].(map[string]any)
	if !ok {
		return opType, nil, true, fmt.Errorf("%w: operation value", ErrMalformedOperationTuple)
	}
	return opType, data, true, nil
}

func requiredString(data map[string]any, field string) (string, error) {
	value, present := data[field]
	if !present {
		return "", fmt.Errorf("%w: %q", ErrMissingOperationField, field)
	}
	result, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrWrongOperationFieldType, field)
	}
	if result == "" {
		return "", fmt.Errorf("%w: %q", ErrMissingOperationField, field)
	}
	return result, nil
}

func stringArray(data map[string]any, field string) ([]string, error) {
	value, present := data[field]
	if !present {
		return nil, fmt.Errorf("%w: %q", ErrMissingOperationField, field)
	}
	array, ok := value.([]any)
	if !ok {
		if strings, ok := value.([]string); ok {
			return append([]string(nil), strings...), nil
		}
		return nil, fmt.Errorf("%w: %q", ErrMalformedAuthArray, field)
	}
	result := make([]string, len(array))
	for i, item := range array {
		var itemOK bool
		result[i], itemOK = item.(string)
		if !itemOK {
			return nil, fmt.Errorf("%w: %q[%d]", ErrMalformedAuthArray, field, i)
		}
	}
	return result, nil
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
