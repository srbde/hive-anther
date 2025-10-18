package transaction

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"

	"github.com/thecrazygm/nectar-go/client"
	"github.com/thecrazygm/nectar-go/types"
)

const HIVE_CHAIN_ID = "beeab0de00000000000000000000000000000000000000000000000000000000"

// OperationNames maps operation names to their numeric IDs
var OperationNames = map[string]int{
	"vote":            0,
	"comment":         1,
	"transfer":        2,
	"comment_options": 19,
	"custom_json":     18,
}

// Transaction represents a Hive transaction.
type Transaction struct {
	RefBlockNum    uint16
	RefBlockPrefix uint32
	Expiration     time.Time
	Operations     []Operation
	Signatures     []string
	API            *client.Client
}

// Operation is an interface for all Hive operations.
type Operation interface {
	ToDict() (string, map[string]any)
}

// NewTransaction creates a new Transaction.
func NewTransaction(api *client.Client) *Transaction {
	return &Transaction{
		API:        api,
		Operations: []Operation{},
		Signatures: []string{},
	}
}

// AppendOp appends an operation to the transaction.
func (tx *Transaction) AppendOp(op Operation) {
	tx.Operations = append(tx.Operations, op)
}

// Sign the transaction with a private key in WIF format.
func (tx *Transaction) Sign(wif string) error {
	if tx.RefBlockNum == 0 || tx.RefBlockPrefix == 0 {
		if tx.API == nil {
			return errors.New("API not configured to get block params")
		}
		if err := tx.setBlockParams(); err != nil {
			return err
		}
	}

	if tx.API == nil {
		return errors.New("API not configured to get transaction hex")
	}

	// Get the transaction hex from the API
	txDict := tx.toDict()
	txForHex := txDict
	if _, ok := txForHex["signatures"]; !ok {
		txForHex["signatures"] = []any{}
	}

	txHexResult, err := tx.API.Call("condenser_api", "get_transaction_hex", []any{txForHex})
	if err != nil {
		return fmt.Errorf("error calling get_transaction_hex: %v", err)
	}

	if txHexResult == nil {
		return errors.New("get_transaction_hex returned null")
	}

	// Handle the response - could be a string or wrapped in a dict
	var txHex string
	switch v := txHexResult.(type) {
	case string:
		txHex = v
	case map[string]any:
		if hex, ok := v["hex"].(string); ok {
			txHex = hex
		} else if hex, ok := v["transaction_hex"].(string); ok {
			txHex = hex
		}
		// If no hex field found, return the whole map as error info
		if txHex == "" {
			return fmt.Errorf("no hex field in response: %v", v)
		}
	default:
		return fmt.Errorf("unexpected response type from get_transaction_hex: %T (value: %v)", v, v)
	}

	// Strip whitespace
	txHex = strings.TrimSpace(txHex)

	if txHex == "" {
		return errors.New("empty transaction hex returned")
	}

	// Prepare the message to sign: chain ID + digest
	chainIDBytes, err := hex.DecodeString(HIVE_CHAIN_ID)
	if err != nil {
		return err
	}

	txHexBytes, err := hex.DecodeString(txHex)
	if err != nil {
		return err
	}

	// Remove the final 2 characters (signature suffix)
	if len(txHex) > 2 {
		txHexBytes, _ = hex.DecodeString(txHex[:len(txHex)-2])
	}

	message := append(chainIDBytes, txHexBytes...)

	// Following Python implementation exactly:
	// Line 83: digest = hashlib.sha256(message).digest()
	digest := sha256.Sum256(message)

	// Line 84: e = int.from_bytes(digest, "big") % N
	N := new(big.Int)
	N.SetString("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141", 16)
	e := new(big.Int).SetBytes(digest[:])
	e.Mod(e, N)

	wifDecoded, err := btcutil.DecodeWIF(wif)
	if err != nil {
		return err
	}

	// Convert WIF private key bytes to Decred secp256k1 private key
	privKeyBytes := wifDecoded.PrivKey.Serialize()
	privKeySEC := secp256k1.PrivKeyFromBytes(privKeyBytes)

	// Use Decred's SignCompact which produces compact signatures with embedded recovery info
	// This matches what Python's cryptography library does
	// SignCompact returns: [27 + recovery_id + (4 if compressed)][r: 32 bytes][s: 32 bytes]
	compactSig := ecdsa.SignCompact(privKeySEC, digest[:], true) // true = compressed key

	// Extract recovery byte and signature components
	recoveryByte := compactSig[0]
	rBytes := compactSig[1:33]
	sBytes := compactSig[33:65]

	// Extract recovery ID from recovery byte
	// Format: 27 + recovery_id + 4 (for compressed)
	// So: recovery_id = (recoveryByte - 27 - 4) = recoveryByte - 31
	recoveryID := int(recoveryByte) - 31

	// Line 94-95: Check if s needs canonicalization
	s := new(big.Int).SetBytes(sBytes)
	nDiv2 := new(big.Int)
	nDiv2.SetString("7FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF5D576E7357A4501DDFE92F46681B20A0", 16)

	if s.Cmp(nDiv2) > 0 {
		// Canonicalize: s = N - s
		s = new(big.Int).Sub(N, s)
		sBytes = s.Bytes()
		if len(sBytes) < 32 {
			sBytes = append(make([]byte, 32-len(sBytes)), sBytes...)
		}

		// When s is flipped, the recovery ID's y-parity bit changes
		recoveryID = recoveryID ^ 1
	}

	// Build the final canonical signature: [27 + 4 + recoveryID][r][canonical_s]
	canonical := append(rBytes, sBytes...)
	finalSig := append([]byte{byte(27 + 4 + recoveryID)}, canonical...)

	tx.Signatures = append(tx.Signatures, hex.EncodeToString(finalSig))
	return nil
}

// Broadcast the transaction to the network.
func (tx *Transaction) Broadcast() (any, error) {
	if len(tx.Signatures) == 0 {
		return nil, errors.New("transaction is not signed")
	}

	txDict := tx.toDict()
	return tx.API.Call("condenser_api", "broadcast_transaction_synchronous", []any{txDict})
}

// toDict converts the transaction to a dictionary.
func (tx *Transaction) toDict() map[string]any {
	ops := []any{}
	for _, op := range tx.Operations {
		name, params := op.ToDict()
		ops = append(ops, []any{name, params})
	}

	return map[string]any{
		"ref_block_num":    tx.RefBlockNum,
		"ref_block_prefix": tx.RefBlockPrefix,
		"expiration":       tx.Expiration.Format("2006-01-02T15:04:05"),
		"operations":       ops,
		"extensions":       []any{},
		"signatures":       tx.Signatures,
	}
}

// setBlockParams gets the reference block number and prefix from the blockchain.
func (tx *Transaction) setBlockParams() error {
	props, err := tx.API.GetDynamicGlobalProperties()
	if err != nil {
		return err
	}

	headBlockNumber, ok := props["head_block_number"].(float64)
	if !ok {
		return errors.New("invalid head_block_number")
	}

	tx.RefBlockNum = uint16(int(headBlockNumber-3) & 0xffff)

	blockNum := int(headBlockNumber) - 2
	blockResp, err := tx.API.Call("block_api", "get_block", map[string]any{"block_num": blockNum})
	if err != nil {
		return err
	}

	block, ok := blockResp.(map[string]any)["block"].(map[string]any)
	if !ok {
		return errors.New("invalid block response")
	}

	previous, ok := block["previous"].(string)
	if !ok {
		return errors.New("invalid previous block")
	}

	prevBytes, err := hex.DecodeString(previous)
	if err != nil {
		return err
	}

	tx.RefBlockPrefix = uint32(prevBytes[4]) | uint32(prevBytes[5])<<8 | uint32(prevBytes[6])<<16 | uint32(prevBytes[7])<<24

	headBlockTime, ok := props["time"].(string)
	if !ok {
		return errors.New("invalid time")
	}

	expiration, err := time.Parse("2006-01-02T15:04:05", headBlockTime)
	if err != nil {
		return err
	}
	tx.Expiration = expiration.Add(30 * time.Second)

	return nil
}

// Vote represents a vote operation.
type Vote struct {
	Voter    string
	Author   string
	Permlink string
	Weight   int16
}

// ToDict returns the operation as a dictionary.
func (v *Vote) ToDict() (string, map[string]any) {
	return "vote", map[string]any{
		"voter":    v.Voter,
		"author":   v.Author,
		"permlink": v.Permlink,
		"weight":   v.Weight,
	}
}

// Transfer represents a transfer operation.
type Transfer struct {
	To     string
	From   string
	Amount string // Format: "0.001 HIVE" or "1.000 HBD"
	Memo   string
}

// ToDict returns the operation as a dictionary.
func (t *Transfer) ToDict() (string, map[string]any) {
	return "transfer", map[string]any{
		"to":     t.To,
		"from":   t.From,
		"amount": t.Amount,
		"memo":   t.Memo,
	}
}

// Bytes returns the binary representation for wire protocol serialization
// This is used during transaction signing and handles HIVE->STEEM conversion
func (t *Transfer) Bytes() ([]byte, error) {
	var buf bytes.Buffer

	// Serialize strings with varint length prefix
	if err := serializeString(&buf, t.From); err != nil {
		return nil, fmt.Errorf("error serializing from: %v", err)
	}
	if err := serializeString(&buf, t.To); err != nil {
		return nil, fmt.Errorf("error serializing to: %v", err)
	}

	// Parse and serialize amount with wire symbol conversion
	amt, err := types.ParseAmount(t.Amount)
	if err != nil {
		return nil, fmt.Errorf("error parsing amount: %v", err)
	}
	amtBytes, err := amt.Bytes()
	if err != nil {
		return nil, fmt.Errorf("error serializing amount: %v", err)
	}
	buf.Write(amtBytes)

	// Serialize memo
	if err := serializeString(&buf, t.Memo); err != nil {
		return nil, fmt.Errorf("error serializing memo: %v", err)
	}

	return buf.Bytes(), nil
}

// Helper function to serialize a string with varint length prefix
func serializeString(buf *bytes.Buffer, s string) error {
	strBytes := []byte(s)
	length := len(strBytes)

	// Write varint length (simple implementation for < 128)
	if length < 128 {
		buf.WriteByte(byte(length))
	} else {
		// For lengths >= 128, we'd need proper varint encoding
		// For now, keep it simple as most strings will be < 128
		return fmt.Errorf("string too long for simple varint: %d", length)
	}

	buf.Write(strBytes)
	return nil
}

// Comment represents a comment (post) operation.
type Comment struct {
	ParentAuthor   string
	ParentPermlink string
	Author         string
	Permlink       string
	Title          string
	Body           string
	JSONMetadata   string
}

// ToDict returns the operation as a dictionary.
func (c *Comment) ToDict() (string, map[string]any) {
	return "comment", map[string]any{
		"parent_author":   c.ParentAuthor,
		"parent_permlink": c.ParentPermlink,
		"author":          c.Author,
		"permlink":        c.Permlink,
		"title":           c.Title,
		"body":            c.Body,
		"json_metadata":   c.JSONMetadata,
	}
}

// CustomJSON represents a custom JSON operation.
type CustomJSON struct {
	ID                   string
	JSON                 string
	RequiredAuths        []string
	RequiredPostingAuths []string
}

// ToDict returns the operation as a dictionary.
func (cj *CustomJSON) ToDict() (string, map[string]any) {
	return "custom_json", map[string]any{
		"id":                     cj.ID,
		"json":                   cj.JSON,
		"required_auths":         cj.RequiredAuths,
		"required_posting_auths": cj.RequiredPostingAuths,
	}
}

// Follow represents a follow operation via custom JSON.
type Follow struct {
	Follower  string
	Following string
	What      []string
}

// ToDict returns the operation as a dictionary.
func (f *Follow) ToDict() (string, map[string]any) {
	followJSON := map[string]any{
		"follower":  f.Follower,
		"following": f.Following,
		"what":      f.What,
	}
	jsonBytes, _ := json.Marshal([]any{"follow", followJSON})
	jsonStr := string(jsonBytes)

	return "custom_json", map[string]any{
		"id":                     "follow",
		"json":                   jsonStr,
		"required_auths":         []string{},
		"required_posting_auths": []string{f.Follower},
	}
}
