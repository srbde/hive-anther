package transaction

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sort"

	"github.com/srbde/hive-anther/crypto"
	"github.com/srbde/hive-anther/types"
)

// Authority represents a cryptographic voting or transaction threshold authority.
type Authority struct {
	WeightThreshold uint32            `json:"weight_threshold"`
	AccountAuths    map[string]uint16 `json:"account_auths"`
	KeyAuths        map[string]uint16 `json:"key_auths"`
}

// Helper function to serialize an Authority
func serializeAuthority(buf *bytes.Buffer, auth *Authority) error {
	if err := binary.Write(buf, binary.LittleEndian, auth.WeightThreshold); err != nil {
		return err
	}

	// 1. Serialize AccountAuths (sorted alphabetically by account name)
	accNames := make([]string, 0, len(auth.AccountAuths))
	for name := range auth.AccountAuths {
		accNames = append(accNames, name)
	}
	sort.Strings(accNames)

	varintBuf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(varintBuf, uint64(len(accNames)))
	buf.Write(varintBuf[:n])

	for _, name := range accNames {
		if err := serializeString(buf, name); err != nil {
			return err
		}
		weight := auth.AccountAuths[name]
		if err := binary.Write(buf, binary.LittleEndian, weight); err != nil {
			return err
		}
	}

	// 2. Serialize KeyAuths (sorted alphabetically by key string)
	keyStrs := make([]string, 0, len(auth.KeyAuths))
	for key := range auth.KeyAuths {
		keyStrs = append(keyStrs, key)
	}
	sort.Strings(keyStrs)

	n = binary.PutUvarint(varintBuf, uint64(len(keyStrs)))
	buf.Write(varintBuf[:n])

	for _, key := range keyStrs {
		if err := serializePublicKey(buf, key); err != nil {
			return err
		}
		weight := auth.KeyAuths[key]
		if err := binary.Write(buf, binary.LittleEndian, weight); err != nil {
			return err
		}
	}

	return nil
}

// Helper function to serialize a public key (STM... or raw 33 bytes)
func serializePublicKey(buf *bytes.Buffer, pubKeyStr string) error {
	if pubKeyStr == "" {
		buf.Write(make([]byte, 33))
		return nil
	}
	trimmed := pubKeyStr
	if len(pubKeyStr) > 3 && (pubKeyStr[:3] == "STM" || pubKeyStr[:3] == "TST") {
		trimmed = pubKeyStr[3:]
	}
	decoded := crypto.Base58Decode(trimmed)
	if len(decoded) < 33 {
		return fmt.Errorf("invalid public key length: %d", len(decoded))
	}
	buf.Write(decoded[:33])
	return nil
}

// CommentOptions represents a comment_options operation.
type CommentOptions struct {
	Author               string             `json:"author"`
	Permlink             string             `json:"permlink"`
	MaxAcceptedPayout    string             `json:"max_accepted_payout"`
	PercentHBD           uint16             `json:"percent_hbd"`
	AllowVotes           bool               `json:"allow_votes"`
	AllowCurationRewards bool               `json:"allow_curation_rewards"`
	Extensions           []CommentExtension `json:"extensions"`
}

// CommentExtension is an interface for comment extensions.
type CommentExtension interface {
	VariantID() uint64
	Bytes() ([]byte, error)
}

// BeneficiaryRoute represents a single beneficiary and their reward share.
type BeneficiaryRoute struct {
	Account string `json:"account"`
	Weight  uint16 `json:"weight"`
}

// CommentPayoutBeneficiaries represents the beneficiaries comment extension.
type CommentPayoutBeneficiaries struct {
	Beneficiaries []BeneficiaryRoute `json:"beneficiaries"`
}

func (c *CommentPayoutBeneficiaries) VariantID() uint64 {
	return 0
}

func (c *CommentPayoutBeneficiaries) Bytes() ([]byte, error) {
	var buf bytes.Buffer
	length := uint64(len(c.Beneficiaries))
	varintBuf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(varintBuf, length)
	buf.Write(varintBuf[:n])

	for _, b := range c.Beneficiaries {
		if err := serializeString(&buf, b.Account); err != nil {
			return nil, err
		}
		if err := binary.Write(&buf, binary.LittleEndian, b.Weight); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

func (co *CommentOptions) ToDict() (string, map[string]any) {
	exts := []any{}
	for _, ext := range co.Extensions {
		if bExt, ok := ext.(*CommentPayoutBeneficiaries); ok {
			exts = append(exts, []any{0, map[string]any{"beneficiaries": bExt.Beneficiaries}})
		}
	}

	return "comment_options", map[string]any{
		"author":                 co.Author,
		"permlink":               co.Permlink,
		"max_accepted_payout":    co.MaxAcceptedPayout,
		"percent_hbd":            co.PercentHBD,
		"allow_votes":            co.AllowVotes,
		"allow_curation_rewards": co.AllowCurationRewards,
		"extensions":             exts,
	}
}

func (co *CommentOptions) Bytes() ([]byte, error) {
	var buf bytes.Buffer
	varintBuf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(varintBuf, 19) // ID 19
	buf.Write(varintBuf[:n])

	if err := serializeString(&buf, co.Author); err != nil {
		return nil, err
	}
	if err := serializeString(&buf, co.Permlink); err != nil {
		return nil, err
	}

	amt, err := types.ParseAmount(co.MaxAcceptedPayout)
	if err != nil {
		return nil, err
	}
	amtBytes, err := amt.Bytes()
	if err != nil {
		return nil, err
	}
	buf.Write(amtBytes)

	if err := binary.Write(&buf, binary.LittleEndian, co.PercentHBD); err != nil {
		return nil, err
	}

	if co.AllowVotes {
		buf.WriteByte(1)
	} else {
		buf.WriteByte(0)
	}

	if co.AllowCurationRewards {
		buf.WriteByte(1)
	} else {
		buf.WriteByte(0)
	}

	extLen := uint64(len(co.Extensions))
	n = binary.PutUvarint(varintBuf, extLen)
	buf.Write(varintBuf[:n])

	for _, ext := range co.Extensions {
		n = binary.PutUvarint(varintBuf, ext.VariantID())
		buf.Write(varintBuf[:n])

		extBytes, err := ext.Bytes()
		if err != nil {
			return nil, err
		}
		buf.Write(extBytes)
	}

	return buf.Bytes(), nil
}

// FromBytes deserializes CommentOptions from binary bytes.
func (co *CommentOptions) FromBytes(r *bytes.Reader) error {
	return fmt.Errorf("CommentOptions deserialization not implemented")
}

// DeleteComment represents a delete_comment operation.
type DeleteComment struct {
	Author   string `json:"author"`
	Permlink string `json:"permlink"`
}

func (dc *DeleteComment) ToDict() (string, map[string]any) {
	return "delete_comment", map[string]any{
		"author":   dc.Author,
		"permlink": dc.Permlink,
	}
}

func (dc *DeleteComment) Bytes() ([]byte, error) {
	var buf bytes.Buffer
	varintBuf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(varintBuf, 17) // ID 17
	buf.Write(varintBuf[:n])

	if err := serializeString(&buf, dc.Author); err != nil {
		return nil, err
	}
	if err := serializeString(&buf, dc.Permlink); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// FromBytes deserializes DeleteComment from binary bytes.
func (dc *DeleteComment) FromBytes(r *bytes.Reader) error {
	return fmt.Errorf("DeleteComment deserialization not implemented")
}

// TransferToVesting represents a transfer_to_vesting operation.
type TransferToVesting struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Amount string `json:"amount"`
}

func (tv *TransferToVesting) ToDict() (string, map[string]any) {
	return "transfer_to_vesting", map[string]any{
		"from":   tv.From,
		"to":     tv.To,
		"amount": tv.Amount,
	}
}

func (tv *TransferToVesting) Bytes() ([]byte, error) {
	var buf bytes.Buffer
	varintBuf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(varintBuf, 3) // ID 3
	buf.Write(varintBuf[:n])

	if err := serializeString(&buf, tv.From); err != nil {
		return nil, err
	}
	if err := serializeString(&buf, tv.To); err != nil {
		return nil, err
	}

	amt, err := types.ParseAmount(tv.Amount)
	if err != nil {
		return nil, err
	}
	amtBytes, err := amt.Bytes()
	if err != nil {
		return nil, err
	}
	buf.Write(amtBytes)

	return buf.Bytes(), nil
}

// FromBytes deserializes TransferToVesting from binary bytes.
func (tv *TransferToVesting) FromBytes(r *bytes.Reader) error {
	return fmt.Errorf("TransferToVesting deserialization not implemented")
}

// WithdrawVesting represents a withdraw_vesting operation.
type WithdrawVesting struct {
	Account       string `json:"account"`
	VestingShares string `json:"vesting_shares"`
}

func (wv *WithdrawVesting) ToDict() (string, map[string]any) {
	return "withdraw_vesting", map[string]any{
		"account":        wv.Account,
		"vesting_shares": wv.VestingShares,
	}
}

func (wv *WithdrawVesting) Bytes() ([]byte, error) {
	var buf bytes.Buffer
	varintBuf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(varintBuf, 4) // ID 4
	buf.Write(varintBuf[:n])

	if err := serializeString(&buf, wv.Account); err != nil {
		return nil, err
	}

	amt, err := types.ParseAmount(wv.VestingShares)
	if err != nil {
		return nil, err
	}
	amtBytes, err := amt.Bytes()
	if err != nil {
		return nil, err
	}
	buf.Write(amtBytes)

	return buf.Bytes(), nil
}

// FromBytes deserializes WithdrawVesting from binary bytes.
func (wv *WithdrawVesting) FromBytes(r *bytes.Reader) error {
	return fmt.Errorf("WithdrawVesting deserialization not implemented")
}

// DelegateVestingShares represents a delegate_vesting_shares operation.
type DelegateVestingShares struct {
	Delegator     string `json:"delegator"`
	Delegatee     string `json:"delegatee"`
	VestingShares string `json:"vesting_shares"`
}

func (dvs *DelegateVestingShares) ToDict() (string, map[string]any) {
	return "delegate_vesting_shares", map[string]any{
		"delegator":      dvs.Delegator,
		"delegatee":      dvs.Delegatee,
		"vesting_shares": dvs.VestingShares,
	}
}

func (dvs *DelegateVestingShares) Bytes() ([]byte, error) {
	var buf bytes.Buffer
	varintBuf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(varintBuf, 40) // ID 40
	buf.Write(varintBuf[:n])

	if err := serializeString(&buf, dvs.Delegator); err != nil {
		return nil, err
	}
	if err := serializeString(&buf, dvs.Delegatee); err != nil {
		return nil, err
	}

	amt, err := types.ParseAmount(dvs.VestingShares)
	if err != nil {
		return nil, err
	}
	amtBytes, err := amt.Bytes()
	if err != nil {
		return nil, err
	}
	buf.Write(amtBytes)

	return buf.Bytes(), nil
}

// FromBytes deserializes DelegateVestingShares from binary bytes.
func (dvs *DelegateVestingShares) FromBytes(r *bytes.Reader) error {
	return fmt.Errorf("DelegateVestingShares deserialization not implemented")
}

// ClaimRewardBalance represents a claim_reward_balance operation.
type ClaimRewardBalance struct {
	Account     string `json:"account"`
	RewardHive  string `json:"reward_hive"`
	RewardHBD   string `json:"reward_hbd"`
	RewardVests string `json:"reward_vests"`
}

func (crb *ClaimRewardBalance) ToDict() (string, map[string]any) {
	return "claim_reward_balance", map[string]any{
		"account":      crb.Account,
		"reward_hive":  crb.RewardHive,
		"reward_hbd":   crb.RewardHBD,
		"reward_vests": crb.RewardVests,
	}
}

func (crb *ClaimRewardBalance) Bytes() ([]byte, error) {
	var buf bytes.Buffer
	varintBuf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(varintBuf, 39) // ID 39
	buf.Write(varintBuf[:n])

	if err := serializeString(&buf, crb.Account); err != nil {
		return nil, err
	}

	amtHive, err := types.ParseAmount(crb.RewardHive)
	if err != nil {
		return nil, err
	}
	amtHiveBytes, err := amtHive.Bytes()
	if err != nil {
		return nil, err
	}
	buf.Write(amtHiveBytes)

	amtHbd, err := types.ParseAmount(crb.RewardHBD)
	if err != nil {
		return nil, err
	}
	amtHbdBytes, err := amtHbd.Bytes()
	if err != nil {
		return nil, err
	}
	buf.Write(amtHbdBytes)

	amtVests, err := types.ParseAmount(crb.RewardVests)
	if err != nil {
		return nil, err
	}
	amtVestsBytes, err := amtVests.Bytes()
	if err != nil {
		return nil, err
	}
	buf.Write(amtVestsBytes)

	return buf.Bytes(), nil
}

// FromBytes deserializes ClaimRewardBalance from binary bytes.
func (crb *ClaimRewardBalance) FromBytes(r *bytes.Reader) error {
	return fmt.Errorf("ClaimRewardBalance deserialization not implemented")
}

// RecurrentTransfer represents a recurrent_transfer operation.
type RecurrentTransfer struct {
	From       string `json:"from"`
	To         string `json:"to"`
	Amount     string `json:"amount"`
	Memo       string `json:"memo"`
	Recurrence uint16 `json:"recurrence"`
	Executions uint16 `json:"executions"`
}

func (rt *RecurrentTransfer) ToDict() (string, map[string]any) {
	return "recurrent_transfer", map[string]any{
		"from":       rt.From,
		"to":         rt.To,
		"amount":     rt.Amount,
		"memo":       rt.Memo,
		"recurrence": rt.Recurrence,
		"executions": rt.Executions,
		"extensions": []any{},
	}
}

func (rt *RecurrentTransfer) Bytes() ([]byte, error) {
	var buf bytes.Buffer
	varintBuf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(varintBuf, 49) // ID 49
	buf.Write(varintBuf[:n])

	if err := serializeString(&buf, rt.From); err != nil {
		return nil, err
	}
	if err := serializeString(&buf, rt.To); err != nil {
		return nil, err
	}

	amt, err := types.ParseAmount(rt.Amount)
	if err != nil {
		return nil, err
	}
	amtBytes, err := amt.Bytes()
	if err != nil {
		return nil, err
	}
	buf.Write(amtBytes)

	if err := serializeString(&buf, rt.Memo); err != nil {
		return nil, err
	}

	if err := binary.Write(&buf, binary.LittleEndian, rt.Recurrence); err != nil {
		return nil, err
	}

	if err := binary.Write(&buf, binary.LittleEndian, rt.Executions); err != nil {
		return nil, err
	}

	// Extensions: empty array -> 0
	buf.WriteByte(0)

	return buf.Bytes(), nil
}

// FromBytes deserializes RecurrentTransfer from binary bytes.
func (rt *RecurrentTransfer) FromBytes(r *bytes.Reader) error {
	return fmt.Errorf("RecurrentTransfer deserialization not implemented")
}

// ClaimAccount represents a claim_account operation.
type ClaimAccount struct {
	Creator string `json:"creator"`
	Fee     string `json:"fee"`
}

func (ca *ClaimAccount) ToDict() (string, map[string]any) {
	return "claim_account", map[string]any{
		"creator":    ca.Creator,
		"fee":        ca.Fee,
		"extensions": []any{},
	}
}

func (ca *ClaimAccount) Bytes() ([]byte, error) {
	var buf bytes.Buffer
	varintBuf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(varintBuf, 22) // ID 22
	buf.Write(varintBuf[:n])

	if err := serializeString(&buf, ca.Creator); err != nil {
		return nil, err
	}

	amt, err := types.ParseAmount(ca.Fee)
	if err != nil {
		return nil, err
	}
	amtBytes, err := amt.Bytes()
	if err != nil {
		return nil, err
	}
	buf.Write(amtBytes)

	// Extensions: empty array -> 0
	buf.WriteByte(0)

	return buf.Bytes(), nil
}

// FromBytes deserializes ClaimAccount from binary bytes.
func (ca *ClaimAccount) FromBytes(r *bytes.Reader) error {
	return fmt.Errorf("ClaimAccount deserialization not implemented")
}

// CreateClaimedAccount represents a create_claimed_account operation.
type CreateClaimedAccount struct {
	Creator        string     `json:"creator"`
	NewAccountName string     `json:"new_account_name"`
	Owner          *Authority `json:"owner"`
	Active         *Authority `json:"active"`
	Posting        *Authority `json:"posting"`
	MemoKey        string     `json:"memo_key"`
	JSONMetadata   string     `json:"json_metadata"`
}

func (cca *CreateClaimedAccount) ToDict() (string, map[string]any) {
	return "create_claimed_account", map[string]any{
		"creator":          cca.Creator,
		"new_account_name": cca.NewAccountName,
		"owner":            cca.Owner,
		"active":           cca.Active,
		"posting":          cca.Posting,
		"memo_key":         cca.MemoKey,
		"json_metadata":    cca.JSONMetadata,
		"extensions":       []any{},
	}
}

func (cca *CreateClaimedAccount) Bytes() ([]byte, error) {
	var buf bytes.Buffer
	varintBuf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(varintBuf, 23) // ID 23
	buf.Write(varintBuf[:n])

	if err := serializeString(&buf, cca.Creator); err != nil {
		return nil, err
	}
	if err := serializeString(&buf, cca.NewAccountName); err != nil {
		return nil, err
	}

	if err := serializeAuthority(&buf, cca.Owner); err != nil {
		return nil, err
	}
	if err := serializeAuthority(&buf, cca.Active); err != nil {
		return nil, err
	}
	if err := serializeAuthority(&buf, cca.Posting); err != nil {
		return nil, err
	}

	if err := serializePublicKey(&buf, cca.MemoKey); err != nil {
		return nil, err
	}

	if err := serializeString(&buf, cca.JSONMetadata); err != nil {
		return nil, err
	}

	// Extensions: empty array -> 0
	buf.WriteByte(0)

	return buf.Bytes(), nil
}

// FromBytes deserializes CreateClaimedAccount from binary bytes.
func (cca *CreateClaimedAccount) FromBytes(r *bytes.Reader) error {
	return fmt.Errorf("CreateClaimedAccount deserialization not implemented")
}

// AccountUpdate represents an account_update operation.
type AccountUpdate struct {
	Account      string     `json:"account"`
	Owner        *Authority `json:"owner,omitempty"`
	Active       *Authority `json:"active,omitempty"`
	Posting      *Authority `json:"posting,omitempty"`
	MemoKey      string     `json:"memo_key"`
	JSONMetadata string     `json:"json_metadata"`
}

func (au *AccountUpdate) ToDict() (string, map[string]any) {
	dict := map[string]any{
		"account":       au.Account,
		"memo_key":      au.MemoKey,
		"json_metadata": au.JSONMetadata,
	}
	if au.Owner != nil {
		dict["owner"] = au.Owner
	}
	if au.Active != nil {
		dict["active"] = au.Active
	}
	if au.Posting != nil {
		dict["posting"] = au.Posting
	}
	return "account_update", dict
}

func (au *AccountUpdate) Bytes() ([]byte, error) {
	var buf bytes.Buffer
	varintBuf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(varintBuf, 10) // ID 10
	buf.Write(varintBuf[:n])

	if err := serializeString(&buf, au.Account); err != nil {
		return nil, err
	}

	// Owner (optional)
	if au.Owner != nil {
		buf.WriteByte(1)
		if err := serializeAuthority(&buf, au.Owner); err != nil {
			return nil, err
		}
	} else {
		buf.WriteByte(0)
	}

	// Active (optional)
	if au.Active != nil {
		buf.WriteByte(1)
		if err := serializeAuthority(&buf, au.Active); err != nil {
			return nil, err
		}
	} else {
		buf.WriteByte(0)
	}

	// Posting (optional)
	if au.Posting != nil {
		buf.WriteByte(1)
		if err := serializeAuthority(&buf, au.Posting); err != nil {
			return nil, err
		}
	} else {
		buf.WriteByte(0)
	}

	if err := serializePublicKey(&buf, au.MemoKey); err != nil {
		return nil, err
	}

	if err := serializeString(&buf, au.JSONMetadata); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// FromBytes deserializes AccountUpdate from binary bytes.
func (au *AccountUpdate) FromBytes(r *bytes.Reader) error {
	return fmt.Errorf("AccountUpdate deserialization not implemented")
}
