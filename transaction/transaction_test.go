package transaction

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/srbde/hive-anther/crypto"
)

type mockRPCClient struct {
	url string
}

func (m *mockRPCClient) Call(api string, method string, params any) (any, error) {
	payload := map[string]any{
		"jsonrpc": "2.0",
		"method":  api + "." + method,
		"params":  params,
		"id":      1,
	}
	body, _ := json.Marshal(payload)
	resp, err := http.Post(m.url, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result["result"], nil
}

func (m *mockRPCClient) GetDynamicGlobalProperties() (map[string]any, error) {
	resp, err := m.Call("condenser_api", "get_dynamic_global_properties", nil)
	if err != nil {
		return nil, err
	}
	return resp.(map[string]any), nil
}

type dummyOp struct {
	name string
	data map[string]any
}

func (d dummyOp) ToDict() (string, map[string]any) {
	return d.name, d.data
}

func (d dummyOp) Bytes() ([]byte, error) {
	return []byte{0x12, 0x34}, nil
}

func (d dummyOp) FromBytes(r *bytes.Reader) error {
	return nil
}

func generateTestWIF(t *testing.T) string {
	t.Helper()
	priv := make([]byte, 32)
	for i := range priv {
		priv[i] = byte(i + 1)
	}
	payload := append([]byte{0x80}, priv...)
	h1 := sha256.Sum256(payload)
	h2 := sha256.Sum256(h1[:])
	wifBytes := append(payload, h2[:4]...)
	return crypto.Base58Encode(wifBytes)
}

func TestAppendOpAndToDict(t *testing.T) {
	tx := NewTransaction(nil)
	op := dummyOp{name: "custom", data: map[string]any{"foo": "bar"}}
	tx.AppendOp(op)

	if len(tx.Operations) != 1 {
		t.Fatalf("expected one operation, got %d", len(tx.Operations))
	}

	dict := tx.toDict()
	ops, ok := dict["operations"].([]any)
	if !ok || len(ops) != 1 {
		t.Fatalf("unexpected operations payload: %#v", dict["operations"])
	}
	entry, ok := ops[0].([]any)
	if !ok || len(entry) != 2 {
		t.Fatalf("unexpected operation entry: %#v", ops[0])
	}
	if entry[0] != "custom" {
		t.Fatalf("unexpected operation name: %v", entry[0])
	}
	params, ok := entry[1].(map[string]any)
	if !ok || params["foo"] != "bar" {
		t.Fatalf("unexpected operation params: %#v", entry[1])
	}
}

func TestSignAndBroadcast(t *testing.T) {
	var broadcastCalled int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		method, _ := payload["method"].(string)
		response := map[string]any{"jsonrpc": "2.0", "id": payload["id"]}

		switch method {
		case "condenser_api.get_dynamic_global_properties":
			response["result"] = map[string]any{
				"head_block_number": 12345.0,
				"time":              "2025-01-01T00:00:00",
			}
		case "block_api.get_block":
			response["result"] = map[string]any{
				"block": map[string]any{
					"previous": "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20",
				},
			}
		case "condenser_api.get_transaction_hex":
			response["result"] = map[string]any{
				"hex": "deadbeefcafebabe",
			}
		case "condenser_api.broadcast_transaction":
			atomic.AddInt64(&broadcastCalled, 1)
			params, _ := payload["params"].([]any)
			if len(params) != 1 {
				http.Error(w, "unexpected params", http.StatusBadRequest)
				return
			}
			txPayload, _ := params[0].(map[string]any)
			signatures, _ := txPayload["signatures"].([]any)
			if len(signatures) != 1 {
				http.Error(w, "missing signature", http.StatusBadRequest)
				return
			}
			response["result"] = map[string]any{"status": "ok"}
		default:
			http.Error(w, "unexpected method", http.StatusBadRequest)
			return
		}

		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	api := &mockRPCClient{url: server.URL}
	tx := NewTransaction(api)
	tx.AppendOp(dummyOp{name: "custom", data: map[string]any{"foo": "bar"}})

	if err := tx.Sign(generateTestWIF(t)); err != nil {
		t.Fatalf("unexpected sign error: %v", err)
	}
	if len(tx.Signatures) != 1 {
		t.Fatalf("expected one signature, got %d", len(tx.Signatures))
	}

	result, err := tx.Broadcast()
	if err != nil {
		t.Fatalf("unexpected broadcast error: %v", err)
	}
	resMap, ok := result.(map[string]any)
	if !ok || resMap["status"] != "ok" {
		t.Fatalf("unexpected broadcast result: %#v", result)
	}

	if atomic.LoadInt64(&broadcastCalled) != 1 {
		t.Fatalf("expected broadcast to be called once, got %d", broadcastCalled)
	}
}

func TestBroadcastWithoutSignature(t *testing.T) {
	tx := NewTransaction(nil)
	if _, err := tx.Broadcast(); err == nil {
		t.Fatalf("expected error when broadcasting without signature")
	}
}

func TestSerializeString(t *testing.T) {
	t.Run("short string", func(t *testing.T) {
		var buf bytes.Buffer
		err := serializeString(&buf, "hello")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := append([]byte{0x05}, []byte("hello")...)
		if !bytes.Equal(buf.Bytes(), expected) {
			t.Fatalf("expected %v, got %v", expected, buf.Bytes())
		}
	})

	t.Run("long string", func(t *testing.T) {
		var buf bytes.Buffer
		longStr := make([]byte, 130)
		for i := range longStr {
			longStr[i] = 'a'
		}
		err := serializeString(&buf, string(longStr))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		res := buf.Bytes()
		if len(res) != 132 {
			t.Fatalf("expected length 132, got %d", len(res))
		}
		// 130 in LEB128 is 0x82, 0x01
		if res[0] != 0x82 || res[1] != 0x01 {
			t.Fatalf("expected varint prefix [0x82, 0x01], got [0x%02x, 0x%02x]", res[0], res[1])
		}
		if !bytes.Equal(res[2:], longStr) {
			t.Fatalf("unexpected payload suffix")
		}
	})
}

func TestTransactionBytes(t *testing.T) {
	tx := NewTransaction(nil)
	tx.RefBlockNum = 12345
	tx.RefBlockPrefix = 0x11223344
	tx.Expiration = time.Unix(1735689600, 0).UTC()

	tx.AppendOp(&Transfer{
		From:   "sender",
		To:     "receiver",
		Amount: "1.000 HIVE",
		Memo:   "hello",
	})

	txBytes, err := tx.Bytes()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []byte{
		// RefBlockNum (12345) -> 0x3039 in LE
		0x39, 0x30,
		// RefBlockPrefix (0x11223344) -> LE
		0x44, 0x33, 0x22, 0x11,
		// Expiration (1735689600) -> 0x67748580 in LE
		0x80, 0x85, 0x74, 0x67,
		// Ops length (1)
		0x01,
		// Op ID (2 for transfer)
		0x02,
		// From: 6, "sender"
		0x06, 0x73, 0x65, 0x6e, 0x64, 0x65, 0x72,
		// To: 8, "receiver"
		0x08, 0x72, 0x65, 0x63, 0x65, 0x69, 0x76, 0x65, 0x72,
		// Amount: satoshis (1000) -> LE int64
		0xe8, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		// Precision (3)
		0x03,
		// Symbol ("STEEM" padded to 7 bytes)
		0x53, 0x54, 0x45, 0x45, 0x4d, 0x00, 0x00,
		// Memo: 5, "hello"
		0x05, 0x68, 0x65, 0x6c, 0x6c, 0x6f,
		// Extensions length (0)
		0x00,
	}

	if !bytes.Equal(txBytes, expected) {
		t.Fatalf("local serialization does not match expected wire format.\nExpected:\n%x\nGot:\n%x", expected, txBytes)
	}
}

func TestDeleteCommentBytes(t *testing.T) {
	op := &DeleteComment{
		Author:   "author",
		Permlink: "permlink",
	}
	b, err := op.Bytes()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []byte{
		17, // ID
		6, 'a', 'u', 't', 'h', 'o', 'r',
		8, 'p', 'e', 'r', 'm', 'l', 'i', 'n', 'k',
	}
	if !bytes.Equal(b, expected) {
		t.Errorf("delete_comment: expected %x, got %x", expected, b)
	}
}

func TestCommentOptionsBytes(t *testing.T) {
	op := &CommentOptions{
		Author:               "author",
		Permlink:             "permlink",
		MaxAcceptedPayout:    "1000.000 HBD",
		PercentHBD:           10000,
		AllowVotes:           true,
		AllowCurationRewards: true,
		Extensions:           []CommentExtension{},
	}
	b, err := op.Bytes()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []byte{
		19, // ID
		6, 'a', 'u', 't', 'h', 'o', 'r',
		8, 'p', 'e', 'r', 'm', 'l', 'i', 'n', 'k',
		0x40, 0x42, 0x0f, 0x00, 0x00, 0x00, 0x00, 0x00,
		3,
		0x53, 0x42, 0x44, 0x00, 0x00, 0x00, 0x00,
		0x10, 0x27,
		1,
		1,
		0, // extensions length
	}
	if !bytes.Equal(b, expected) {
		t.Errorf("comment_options: expected %x, got %x", expected, b)
	}
}

func TestCommentOptionsWithBeneficiariesBytes(t *testing.T) {
	op := &CommentOptions{
		Author:               "author",
		Permlink:             "permlink",
		MaxAcceptedPayout:    "1000.000 HBD",
		PercentHBD:           10000,
		AllowVotes:           true,
		AllowCurationRewards: true,
		Extensions: []CommentExtension{
			&CommentPayoutBeneficiaries{
				Beneficiaries: []BeneficiaryRoute{
					{Account: "friend", Weight: 500},
				},
			},
		},
	}
	b, err := op.Bytes()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []byte{
		19, // ID
		6, 'a', 'u', 't', 'h', 'o', 'r',
		8, 'p', 'e', 'r', 'm', 'l', 'i', 'n', 'k',
		0x40, 0x42, 0x0f, 0x00, 0x00, 0x00, 0x00, 0x00,
		3,
		0x53, 0x42, 0x44, 0x00, 0x00, 0x00, 0x00,
		0x10, 0x27,
		1,
		1,
		1, // extensions length
		0, // variant ID
		1, // beneficiaries length
		6, 'f', 'r', 'i', 'e', 'n', 'd',
		0xf4, 0x01,
	}
	if !bytes.Equal(b, expected) {
		t.Errorf("comment_options with extensions: expected %x, got %x", expected, b)
	}
}

func TestTransferToVestingBytes(t *testing.T) {
	op := &TransferToVesting{
		From:   "sender",
		To:     "receiver",
		Amount: "10.000 HIVE",
	}
	b, err := op.Bytes()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []byte{
		3, // ID
		6, 's', 'e', 'n', 'd', 'e', 'r',
		8, 'r', 'e', 'c', 'e', 'i', 'v', 'e', 'r',
		0x10, 0x27, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		3,
		0x53, 0x54, 0x45, 0x45, 0x4d, 0x00, 0x00,
	}
	if !bytes.Equal(b, expected) {
		t.Errorf("transfer_to_vesting: expected %x, got %x", expected, b)
	}
}

func TestWithdrawVestingBytes(t *testing.T) {
	op := &WithdrawVesting{
		Account:       "account",
		VestingShares: "100.000000 VESTS",
	}
	b, err := op.Bytes()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []byte{
		4, // ID
		7, 'a', 'c', 'c', 'o', 'u', 'n', 't',
		0x00, 0xe1, 0xf5, 0x05, 0x00, 0x00, 0x00, 0x00,
		6,
		0x56, 0x45, 0x53, 0x54, 0x53, 0x00, 0x00,
	}
	if !bytes.Equal(b, expected) {
		t.Errorf("withdraw_vesting: expected %x, got %x", expected, b)
	}
}

func TestDelegateVestingSharesBytes(t *testing.T) {
	op := &DelegateVestingShares{
		Delegator:     "delegator",
		Delegatee:     "delegatee",
		VestingShares: "100.000000 VESTS",
	}
	b, err := op.Bytes()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []byte{
		40, // ID
		9, 'd', 'e', 'l', 'e', 'g', 'a', 't', 'o', 'r',
		9, 'd', 'e', 'l', 'e', 'g', 'a', 't', 'e', 'e',
		0x00, 0xe1, 0xf5, 0x05, 0x00, 0x00, 0x00, 0x00,
		6,
		0x56, 0x45, 0x53, 0x54, 0x53, 0x00, 0x00,
	}
	if !bytes.Equal(b, expected) {
		t.Errorf("delegate_vesting_shares: expected %x, got %x", expected, b)
	}
}

func TestClaimRewardBalanceBytes(t *testing.T) {
	op := &ClaimRewardBalance{
		Account:     "account",
		RewardHive:  "1.000 HIVE",
		RewardHBD:   "2.000 HBD",
		RewardVests: "3.000000 VESTS",
	}
	b, err := op.Bytes()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []byte{
		39, // ID
		7, 'a', 'c', 'c', 'o', 'u', 'n', 't',
		0xe8, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		3,
		0x53, 0x54, 0x45, 0x45, 0x4d, 0x00, 0x00,
		0xd0, 0x07, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		3,
		0x53, 0x42, 0x44, 0x00, 0x00, 0x00, 0x00,
		0xc0, 0xc6, 0x2d, 0x00, 0x00, 0x00, 0x00, 0x00,
		6,
		0x56, 0x45, 0x53, 0x54, 0x53, 0x00, 0x00,
	}
	if !bytes.Equal(b, expected) {
		t.Errorf("claim_reward_balance: expected %x, got %x", expected, b)
	}
}

func TestRecurrentTransferBytes(t *testing.T) {
	op := &RecurrentTransfer{
		From:       "sender",
		To:         "receiver",
		Amount:     "1.000 HIVE",
		Memo:       "memo",
		Recurrence: 24,
		Executions: 12,
	}
	b, err := op.Bytes()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []byte{
		49, // ID
		6, 's', 'e', 'n', 'd', 'e', 'r',
		8, 'r', 'e', 'c', 'e', 'i', 'v', 'e', 'r',
		0xe8, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		3,
		0x53, 0x54, 0x45, 0x45, 0x4d, 0x00, 0x00,
		4, 'm', 'e', 'm', 'o',
		0x18, 0x00,
		0x0c, 0x00,
		0,
	}
	if !bytes.Equal(b, expected) {
		t.Errorf("recurrent_transfer: expected %x, got %x", expected, b)
	}
}

func TestClaimAccountBytes(t *testing.T) {
	op := &ClaimAccount{
		Creator: "creator",
		Fee:     "0.000 HIVE",
	}
	b, err := op.Bytes()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []byte{
		22, // ID
		7, 'c', 'r', 'e', 'a', 't', 'o', 'r',
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		3,
		0x53, 0x54, 0x45, 0x45, 0x4d, 0x00, 0x00,
		0, // extensions
	}
	if !bytes.Equal(b, expected) {
		t.Errorf("claim_account: expected %x, got %x", expected, b)
	}
}

func TestCreateClaimedAccountBytes(t *testing.T) {
	pubKey := "STM8m5UgaFAAYQRuaNejYdS8FVLVp9Ss3K1qAVk5de6F8s3HnVbvA"
	auth := &Authority{
		WeightThreshold: 1,
		AccountAuths:    map[string]uint16{"friend": 1},
		KeyAuths:        map[string]uint16{pubKey: 1},
	}
	op := &CreateClaimedAccount{
		Creator:        "creator",
		NewAccountName: "newbie",
		Owner:          auth,
		Active:         auth,
		Posting:        auth,
		MemoKey:        pubKey,
		JSONMetadata:   "{}",
	}
	b, err := op.Bytes()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(b) == 0 {
		t.Fatalf("expected serialized bytes, got empty")
	}
}

func TestAccountUpdateBytes(t *testing.T) {
	pubKey := "STM8m5UgaFAAYQRuaNejYdS8FVLVp9Ss3K1qAVk5de6F8s3HnVbvA"
	auth := &Authority{
		WeightThreshold: 1,
		AccountAuths:    map[string]uint16{"friend": 1},
		KeyAuths:        map[string]uint16{pubKey: 1},
	}
	op := &AccountUpdate{
		Account:      "account",
		Owner:        auth,
		Active:       auth,
		Posting:      auth,
		MemoKey:      pubKey,
		JSONMetadata: "{}",
	}
	b, err := op.Bytes()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(b) == 0 {
		t.Fatalf("expected serialized bytes, got empty")
	}
}

func TestVerifyAuthority(t *testing.T) {
	wif1 := generateTestWIF(t)
	pubKeyStr1, err := crypto.WIFToPublicKey(wif1)
	if err != nil {
		t.Fatalf("failed to derive pubkey1: %v", err)
	}

	priv2Bytes := make([]byte, 32)
	priv2Bytes[0] = 0x99
	payload := append([]byte{0x80}, priv2Bytes...)
	h1 := sha256.Sum256(payload)
	h2 := sha256.Sum256(h1[:])
	wif2Bytes := append(payload, h2[:4]...)
	wif2 := crypto.Base58Encode(wif2Bytes)

	pubKeyStr2, err := crypto.WIFToPublicKey(wif2)
	if err != nil {
		t.Fatalf("failed to derive pubkey2: %v", err)
	}

	tx := NewTransaction(nil)
	tx.RefBlockNum = 12345
	tx.RefBlockPrefix = 0x11223344
	tx.Expiration = time.Unix(1735689600, 0).UTC()
	tx.AppendOp(&Vote{Voter: "voter", Author: "author", Permlink: "permlink", Weight: 1000})

	auth1 := &Authority{
		WeightThreshold: 1,
		KeyAuths:        map[string]uint16{pubKeyStr1: 1},
	}
	ok, err := tx.VerifyAuthority(auth1, crypto.HiveChainID)
	if err == nil || ok {
		t.Fatalf("expected error/false verifying unsigned transaction")
	}

	if err := tx.Sign(wif1); err != nil {
		t.Fatalf("sign failed: %v", err)
	}
	ok, err = tx.VerifyAuthority(auth1, crypto.HiveChainID)
	if err != nil || !ok {
		t.Fatalf("expected authority verified, got ok=%v, err=%v", ok, err)
	}

	auth2 := &Authority{
		WeightThreshold: 1,
		KeyAuths:        map[string]uint16{pubKeyStr2: 1},
	}
	ok, err = tx.VerifyAuthority(auth2, crypto.HiveChainID)
	if err != nil || ok {
		t.Fatalf("expected authority not verified, got ok=%v, err=%v", ok, err)
	}

	tx2 := NewTransaction(nil)
	tx2.RefBlockNum = 12345
	tx2.RefBlockPrefix = 0x11223344
	tx2.Expiration = time.Unix(1735689600, 0).UTC()
	tx2.AppendOp(&Vote{Voter: "voter", Author: "author", Permlink: "permlink", Weight: 1000})

	if err := tx2.SignMany([]string{wif1, wif2}); err != nil {
		t.Fatalf("SignMany failed: %v", err)
	}

	authMulti := &Authority{
		WeightThreshold: 2,
		KeyAuths:        map[string]uint16{pubKeyStr1: 1, pubKeyStr2: 1},
	}
	ok, err = tx2.VerifyAuthority(authMulti, crypto.HiveChainID)
	if err != nil || !ok {
		t.Fatalf("expected multi-sig verified, got ok=%v, err=%v", ok, err)
	}

	tx3 := NewTransaction(nil)
	tx3.RefBlockNum = 12345
	tx3.RefBlockPrefix = 0x11223344
	tx3.Expiration = time.Unix(1735689600, 0).UTC()
	tx3.AppendOp(&Vote{Voter: "voter", Author: "author", Permlink: "permlink", Weight: 1000})

	if err := tx3.Sign(wif1); err != nil {
		t.Fatalf("sign failed: %v", err)
	}
	ok, err = tx3.VerifyAuthority(authMulti, crypto.HiveChainID)
	if err != nil || ok {
		t.Fatalf("expected multi-sig not verified, got ok=%v, err=%v", ok, err)
	}
}

func TestTransferRoundtrip(t *testing.T) {
	tx := NewTransaction(nil)
	tx.RefBlockNum = 1234
	tx.RefBlockPrefix = 56789
	tx.Expiration = time.Unix(1735689600, 0).UTC()

	tx.AppendOp(&Transfer{
		From:   "alice",
		To:     "bob",
		Amount: "1.000 HIVE",
		Memo:   "test memo",
	})

	txBytes, err := tx.Bytes()
	if err != nil {
		t.Fatalf("Bytes() error: %v", err)
	}

	tx2, err := TransactionFromBytes(txBytes)
	if err != nil {
		t.Fatalf("TransactionFromBytes() error: %v", err)
	}

	if tx2.RefBlockNum != 1234 {
		t.Fatalf("RefBlockNum: got %d, want 1234", tx2.RefBlockNum)
	}
	if tx2.RefBlockPrefix != 56789 {
		t.Fatalf("RefBlockPrefix: got %d, want 56789", tx2.RefBlockPrefix)
	}
	if len(tx2.Operations) != 1 {
		t.Fatalf("Operations: got %d, want 1", len(tx2.Operations))
	}

	transfer, ok := tx2.Operations[0].(*Transfer)
	if !ok {
		t.Fatalf("expected *Transfer, got %T", tx2.Operations[0])
	}
	if transfer.From != "alice" {
		t.Fatalf("From: got %q, want %q", transfer.From, "alice")
	}
	if transfer.To != "bob" {
		t.Fatalf("To: got %q, want %q", transfer.To, "bob")
	}
	if transfer.Amount != "1.000 HIVE" {
		t.Fatalf("Amount: got %q, want %q", transfer.Amount, "1.000 HIVE")
	}
	if transfer.Memo != "test memo" {
		t.Fatalf("Memo: got %q, want %q", transfer.Memo, "test memo")
	}

	dict := tx2.toDict()
	ops := dict["operations"].([]any)
	entry := ops[0].([]any)
	if entry[0] != "transfer" {
		t.Fatalf("op name: got %v, want transfer", entry[0])
	}
}
