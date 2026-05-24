// Package transaction handles constructing, encoding, signing, verifying, and broadcasting Hive blockchain transactions.
package transaction

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/thecrazygm/anther/client"
	cryptoutil "github.com/thecrazygm/anther/crypto"
	"github.com/thecrazygm/anther/types"
)

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
	Bytes() ([]byte, error)
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

	// Serialize transaction to bytes
	txBytes, err := tx.Bytes()
	if err != nil {
		return fmt.Errorf("error serializing transaction: %w", err)
	}

	signature, err := cryptoutil.SignTransactionBytes(txBytes, wif)
	if err != nil {
		return fmt.Errorf("error signing transaction: %w", err)
	}

	tx.Signatures = append(tx.Signatures, signature)
	return nil
}

// SignMany signs the transaction with multiple WIF keys.
func (tx *Transaction) SignMany(wifKeys []string) error {
	for _, wif := range wifKeys {
		if err := tx.Sign(wif); err != nil {
			return err
		}
	}
	return nil
}

// VerifyAuthority verifies if the accumulated signatures satisfy the provided authority's threshold using direct key auths.
func (tx *Transaction) VerifyAuthority(auth *Authority, chainID string) (bool, error) {
	if len(tx.Signatures) == 0 {
		return false, errors.New("transaction has no signatures to verify")
	}

	txBytes, err := tx.Bytes()
	if err != nil {
		return false, fmt.Errorf("failed to serialize transaction: %w", err)
	}

	chainBytes, err := hex.DecodeString(chainID)
	if err != nil {
		return false, fmt.Errorf("invalid chain ID: %w", err)
	}

	message := append(chainBytes, txBytes...)
	digest := sha256.Sum256(message)

	recoveredKeys := make(map[string]bool)
	for _, sig := range tx.Signatures {
		pubKeyStr, err := cryptoutil.RecoverKeyFromSignature(sig, digest[:])
		if err != nil {
			return false, fmt.Errorf("failed to recover key from signature: %w", err)
		}
		recoveredKeys[pubKeyStr] = true
	}

	var totalWeight uint32
	for keyStr, weight := range auth.KeyAuths {
		if recoveredKeys[keyStr] {
			totalWeight += uint32(weight)
		}
	}

	return totalWeight >= auth.WeightThreshold, nil
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

// Bytes returns the serialized transaction bytes (for signing)
func (tx *Transaction) Bytes() ([]byte, error) {
	var buf bytes.Buffer

	// Write ref_block_num (uint16)
	if err := binary.Write(&buf, binary.LittleEndian, tx.RefBlockNum); err != nil {
		return nil, err
	}

	// Write ref_block_prefix (uint32)
	if err := binary.Write(&buf, binary.LittleEndian, tx.RefBlockPrefix); err != nil {
		return nil, err
	}

	// Write expiration (uint32 Unix timestamp)
	expirationSeconds := uint32(tx.Expiration.Unix())
	if err := binary.Write(&buf, binary.LittleEndian, expirationSeconds); err != nil {
		return nil, err
	}

	// Write operations array length
	opsLen := uint64(len(tx.Operations))
	varintBuf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(varintBuf, opsLen)
	buf.Write(varintBuf[:n])

	// Serialize each operation
	for _, op := range tx.Operations {
		opBytes, err := op.Bytes()
		if err != nil {
			return nil, err
		}
		buf.Write(opBytes)
	}

	// Write extensions array length (0)
	buf.WriteByte(0)

	return buf.Bytes(), nil
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

// Bytes returns the binary representation of the vote operation.
func (v *Vote) Bytes() ([]byte, error) {
	var buf bytes.Buffer
	// Write operation ID (0)
	varintBuf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(varintBuf, 0)
	buf.Write(varintBuf[:n])

	if err := serializeString(&buf, v.Voter); err != nil {
		return nil, fmt.Errorf("error serializing voter: %v", err)
	}
	if err := serializeString(&buf, v.Author); err != nil {
		return nil, fmt.Errorf("error serializing author: %v", err)
	}
	if err := serializeString(&buf, v.Permlink); err != nil {
		return nil, fmt.Errorf("error serializing permlink: %v", err)
	}
	if err := binary.Write(&buf, binary.LittleEndian, v.Weight); err != nil {
		return nil, fmt.Errorf("error serializing weight: %v", err)
	}
	return buf.Bytes(), nil
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
	// Write operation ID (2)
	varintBuf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(varintBuf, 2)
	buf.Write(varintBuf[:n])

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
	length := uint64(len(strBytes))

	var varintBuf [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(varintBuf[:], length)
	buf.Write(varintBuf[:n])

	buf.Write(strBytes)
	return nil
}

// Helper function to serialize an array of strings with varint length prefix
func serializeStringArray(buf *bytes.Buffer, arr []string) error {
	length := uint64(len(arr))
	varintBuf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(varintBuf, length)
	buf.Write(varintBuf[:n])

	for _, s := range arr {
		if err := serializeString(buf, s); err != nil {
			return err
		}
	}
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

// Bytes returns the binary representation of the comment operation.
func (c *Comment) Bytes() ([]byte, error) {
	var buf bytes.Buffer
	// Write operation ID (1)
	varintBuf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(varintBuf, 1)
	buf.Write(varintBuf[:n])

	if err := serializeString(&buf, c.ParentAuthor); err != nil {
		return nil, fmt.Errorf("error serializing parent author: %v", err)
	}
	if err := serializeString(&buf, c.ParentPermlink); err != nil {
		return nil, fmt.Errorf("error serializing parent permlink: %v", err)
	}
	if err := serializeString(&buf, c.Author); err != nil {
		return nil, fmt.Errorf("error serializing author: %v", err)
	}
	if err := serializeString(&buf, c.Permlink); err != nil {
		return nil, fmt.Errorf("error serializing permlink: %v", err)
	}
	if err := serializeString(&buf, c.Title); err != nil {
		return nil, fmt.Errorf("error serializing title: %v", err)
	}
	if err := serializeString(&buf, c.Body); err != nil {
		return nil, fmt.Errorf("error serializing body: %v", err)
	}
	if err := serializeString(&buf, c.JSONMetadata); err != nil {
		return nil, fmt.Errorf("error serializing json metadata: %v", err)
	}
	return buf.Bytes(), nil
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

// Bytes returns the binary representation of the custom json operation.
func (cj *CustomJSON) Bytes() ([]byte, error) {
	var buf bytes.Buffer
	// Write operation ID (18)
	varintBuf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(varintBuf, 18)
	buf.Write(varintBuf[:n])

	if err := serializeStringArray(&buf, cj.RequiredAuths); err != nil {
		return nil, fmt.Errorf("error serializing required auths: %v", err)
	}
	if err := serializeStringArray(&buf, cj.RequiredPostingAuths); err != nil {
		return nil, fmt.Errorf("error serializing required posting auths: %v", err)
	}
	if err := serializeString(&buf, cj.ID); err != nil {
		return nil, fmt.Errorf("error serializing id: %v", err)
	}
	if err := serializeString(&buf, cj.JSON); err != nil {
		return nil, fmt.Errorf("error serializing json: %v", err)
	}
	return buf.Bytes(), nil
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

// Bytes returns the binary representation of the follow operation.
func (f *Follow) Bytes() ([]byte, error) {
	followJSON := map[string]any{
		"follower":  f.Follower,
		"following": f.Following,
		"what":      f.What,
	}
	jsonBytes, err := json.Marshal([]any{"follow", followJSON})
	if err != nil {
		return nil, fmt.Errorf("error marshaling follow JSON: %v", err)
	}
	jsonStr := string(jsonBytes)

	cj := &CustomJSON{
		ID:                   "follow",
		JSON:                 jsonStr,
		RequiredAuths:        []string{},
		RequiredPostingAuths: []string{f.Follower},
	}
	return cj.Bytes()
}
