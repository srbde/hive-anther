package types

import (
	"encoding/binary"
	"encoding/json"
	"math"
	"testing"
	"time"
)

func TestParseAmount(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		amt, err := ParseAmount("123.456 HIVE")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if math.Abs(amt.Value-123.456) > 1e-9 {
			t.Fatalf("unexpected value: %v", amt.Value)
		}
		if amt.Symbol != "HIVE" {
			t.Fatalf("unexpected symbol: %s", amt.Symbol)
		}
	})

	t.Run("invalid format", func(t *testing.T) {
		if _, err := ParseAmount("1000HIVE"); err == nil {
			t.Fatalf("expected error for malformed amount")
		}
	})

	t.Run("invalid numeric value", func(t *testing.T) {
		if _, err := ParseAmount("abc HIVE"); err == nil {
			t.Fatalf("expected error for invalid numeric value")
		}
	})
}

func TestAmountBytes(t *testing.T) {
	t.Run("serializes hive amount", func(t *testing.T) {
		a := NewAmount(1.234, "HIVE")
		b, err := a.Bytes()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(b) != 16 {
			t.Fatalf("unexpected length: %d", len(b))
		}

		value := binary.LittleEndian.Uint64(b[:8])
		if value != 1234 {
			t.Fatalf("unexpected satoshi value: %d", value)
		}
		if b[8] != 0x03 {
			t.Fatalf("unexpected precision byte: %d", b[8])
		}
		expectedSymbol := []byte{'S', 'T', 'E', 'E', 'M', 0x00, 0x00}
		for i, v := range expectedSymbol {
			if b[9+i] != v {
				t.Fatalf("unexpected symbol byte at %d: got %d want %d", 9+i, b[9+i], v)
			}
		}
	})

	t.Run("unknown symbol", func(t *testing.T) {
		a := NewAmount(1, "TEST")
		if _, err := a.Bytes(); err == nil {
			t.Fatalf("expected error for unknown symbol")
		}
	})

	t.Run("symbol too long", func(t *testing.T) {
		a := NewAmount(1, "LONGSYMB")
		if _, err := a.Bytes(); err == nil {
			t.Fatalf("expected error for long symbol")
		}
	})
}

func TestAppliedOperationUnmarshal(t *testing.T) {
	jsonData := `{
		"trx_id": "0000000000000000000000000000000000000000",
		"block": 106666224,
		"trx_in_block": 4294967295,
		"op_in_trx": 4294967295,
		"virtual_op": true,
		"op": ["custom_json", {"id": "test"}]
	}`

	var op AppliedOperation
	if err := json.Unmarshal([]byte(jsonData), &op); err != nil {
		t.Fatalf("failed to unmarshal AppliedOperation: %v", err)
	}

	if op.TrxInBlock != 4294967295 {
		t.Errorf("expected TrxInBlock 4294967295, got %d", op.TrxInBlock)
	}
	if op.OpInTrx != 4294967295 {
		t.Errorf("expected OpInTrx 4294967295, got %d", op.OpInTrx)
	}
}

func TestOperationTupleUnmarshal(t *testing.T) {
	t.Run("array format", func(t *testing.T) {
		jsonData := `["transfer", {"from": "alice", "to": "bob", "amount": "1.000 HIVE"}]`
		var ot OperationTuple
		if err := json.Unmarshal([]byte(jsonData), &ot); err != nil {
			t.Fatalf("unexpected error unmarshaling array-based OperationTuple: %v", err)
		}
		if len(ot) != 2 || ot[0] != "transfer" {
			t.Fatalf("unexpected result: %#v", ot)
		}
	})

	t.Run("object format", func(t *testing.T) {
		jsonData := `{"type": "transfer", "value": {"from": "alice", "to": "bob", "amount": "1.000 HIVE"}}`
		var ot OperationTuple
		if err := json.Unmarshal([]byte(jsonData), &ot); err != nil {
			t.Fatalf("unexpected error unmarshaling object-based OperationTuple: %v", err)
		}
		if len(ot) != 2 || ot[0] != "transfer" {
			t.Fatalf("unexpected result: %#v", ot)
		}
	})

	t.Run("invalid format", func(t *testing.T) {
		jsonData := `"just a string"`
		var ot OperationTuple
		if err := json.Unmarshal([]byte(jsonData), &ot); err == nil {
			t.Fatalf("expected error for invalid OperationTuple format")
		}
	})
}

func TestOperationTupleCustomJSONID(t *testing.T) {
	t.Run("matching custom_json", func(t *testing.T) {
		jsonData := `["custom_json", {"id": "hiveidentity", "json": "{}", "required_posting_auths": ["alice"]}]`
		var ot OperationTuple
		if err := json.Unmarshal([]byte(jsonData), &ot); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		id, ok := ot.CustomJSONID()
		if !ok || id != "hiveidentity" {
			t.Fatalf("expected id %q, got %q (ok=%v)", "hiveidentity", id, ok)
		}
	})

	t.Run("non custom_json op", func(t *testing.T) {
		jsonData := `["transfer", {"from": "alice", "to": "bob", "amount": "1.000 HIVE"}]`
		var ot OperationTuple
		if err := json.Unmarshal([]byte(jsonData), &ot); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok := ot.CustomJSONID(); ok {
			t.Fatalf("expected ok=false for non custom_json op")
		}
	})

	t.Run("missing id field", func(t *testing.T) {
		jsonData := `["custom_json", {"json": "{}"}]`
		var ot OperationTuple
		if err := json.Unmarshal([]byte(jsonData), &ot); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok := ot.CustomJSONID(); ok {
			t.Fatalf("expected ok=false for missing id field")
		}
	})
}

func TestAuthorityJSON(t *testing.T) {
	t.Run("unmarshal tuple-array wire format", func(t *testing.T) {
		jsonData := `{
			"weight_threshold": 2,
			"account_auths": [["bob", 1]],
			"key_auths": [["STM5key1111111111111111111111111111111111111111111111", 2]]
		}`
		var a Authority
		if err := json.Unmarshal([]byte(jsonData), &a); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if a.WeightThreshold != 2 {
			t.Fatalf("expected weight_threshold 2, got %d", a.WeightThreshold)
		}
		if a.AccountAuths["bob"] != 1 {
			t.Fatalf("unexpected account_auths: %+v", a.AccountAuths)
		}
		if a.KeyAuths["STM5key1111111111111111111111111111111111111111111111"] != 2 {
			t.Fatalf("unexpected key_auths: %+v", a.KeyAuths)
		}
	})

	t.Run("round trip through marshal", func(t *testing.T) {
		original := Authority{
			WeightThreshold: 3,
			AccountAuths:    map[string]uint16{"bob": 1, "carol": 2},
			KeyAuths:        map[string]uint16{"STM5keyone": 1},
		}
		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("unexpected marshal error: %v", err)
		}

		var roundTripped Authority
		if err := json.Unmarshal(data, &roundTripped); err != nil {
			t.Fatalf("unexpected unmarshal error: %v", err)
		}

		if roundTripped.WeightThreshold != original.WeightThreshold {
			t.Fatalf("weight_threshold mismatch: %+v", roundTripped)
		}
		for k, v := range original.AccountAuths {
			if roundTripped.AccountAuths[k] != v {
				t.Fatalf("account_auths mismatch: %+v", roundTripped.AccountAuths)
			}
		}
		for k, v := range original.KeyAuths {
			if roundTripped.KeyAuths[k] != v {
				t.Fatalf("key_auths mismatch: %+v", roundTripped.KeyAuths)
			}
		}
	})
}

func TestTimeUnmarshal(t *testing.T) {
	t.Run("valid datetime", func(t *testing.T) {
		jsonData := `"2026-05-24T23:17:09"`
		var ht Time
		if err := json.Unmarshal([]byte(jsonData), &ht); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expectedTime, _ := time.Parse("2006-01-02T15:04:05", "2026-05-24T23:17:09")
		if ht.Time() != expectedTime {
			t.Fatalf("expected %v, got %v", expectedTime, ht.Time())
		}
		if ht.String() != "2026-05-24T23:17:09" {
			t.Fatalf("expected string 2026-05-24T23:17:09, got %s", ht.String())
		}
	})

	t.Run("null or empty", func(t *testing.T) {
		var ht Time
		if err := json.Unmarshal([]byte("null"), &ht); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ht.Time().IsZero() {
			t.Fatalf("expected zero time, got %v", ht.Time())
		}
	})

	t.Run("marshal time", func(t *testing.T) {
		parsed, _ := time.Parse("2006-01-02T15:04:05", "2026-05-24T23:17:09")
		ht := Time(parsed)
		b, err := json.Marshal(ht)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expectedJSON := `"2026-05-24T23:17:09"`
		if string(b) != expectedJSON {
			t.Fatalf("expected %s, got %s", expectedJSON, string(b))
		}
	})
}
