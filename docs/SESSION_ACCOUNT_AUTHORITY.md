# Session Brief: Account Authority Support

Prep doc for a dedicated Anther session. Written from the cambium-cosmos
side, where the gap first showed up while building `x/hiveidentity`.

## Problem

`types.AccountData` (types/types.go) has no `Active`/`Owner`/`Posting`/`Memo`
authority fields. `client.GetAccounts` (client/database.go) calls the real
`get_accounts` RPC and gets a full response back from the node — Hive's
response genuinely includes each account's authorities — but Anther's
`json.Unmarshal` into `AccountData` silently drops them because they're not
declared on the struct. TS (dhive) and Python (beem/nectar) both model
these fields already; this isn't a Hive or Go limitation, just an
unimplemented mapping.

## What's missing

An `Authority` type (Hive's weighted-key model):

```go
type Authority struct {
    WeightThreshold uint32            `json:"weight_threshold"`
    AccountAuths    [][2]any          `json:"account_auths"` // [account_name, weight] pairs
    KeyAuths        map[string]uint16 `json:"key_auths"`
}
```

(`transaction.Authority` in Anther already has `WeightThreshold`/`KeyAuths`
in this shape for signing purposes — this would be the read-side
counterpart, and the two could plausibly be unified into one type in
`types` rather than kept separate in `transaction`.)

And on `AccountData`:

```go
Owner    Authority `json:"owner"`
Active   Authority `json:"active"`
Posting  Authority `json:"posting"`
MemoKey  string    `json:"memo_key"`
```

## Why this matters beyond just "complete the struct"

While scoping `x/hiveidentity` (cambium-cosmos), the original plan was:
client submits a claimed `KeyAuths`/`WeightThreshold` snapshot plus
signatures, chain verifies signatures recover to that claimed authority.
That's forgeable — nothing stops a submitter from claiming an authority
that isn't real, since the chain has no way to check it against Hive L1
itself (no light client, no Merkleized account-state root in Hive block
headers — genuinely not possible to prove trustlessly, not a tooling gap).

Looked at `vsc-eco/go-vsc-node` (a production Hive L2, own Go Hive SDK is
`github.com/vsc-eco/hivego`) for comparison. Their answer: don't verify a
submitted authority claim at all. Users broadcast a `custom_json` operation
*directly to Hive L1*, signed with their real key. By the time it's in a
block, Hive's own witnesses already verified the signature against the
account's real authority — that's just normal Hive consensus. VSC's job is
only "did this op really land in an irreversible Hive block," answered by
an elected/bonded witness committee reaching BLS-threshold consensus, not
by re-deriving authority from a client-submitted claim.

This points `x/hiveidentity` toward the same pattern: watch Hive for a
specific `custom_json` id from the target account, rather than accepting a
signature over a claimed authority from an untrusted submitter. If that's
where cambium-cosmos ends up, **`GetAccounts`/authority parsing becomes
useful for the watcher's own bookkeeping (e.g. resolving an account's memo
key, sanity-checking state, surfacing account info in tooling) rather than
being load-bearing for identity verification itself** — worth keeping that
in mind when scoping how deep to go here.

## Suggested session scope

1. Add `Authority` type + `Owner`/`Active`/`Posting`/`MemoKey` fields to
   `AccountData`.
2. Decide whether to unify with `transaction.Authority` or keep them
   separate (signing-side vs. read-side).
3. Add/extend a `GetAccounts` test fixture with a real `active`/`owner`
   authority payload to confirm round-trip parsing.
4. Separately, evaluate whether Anther needs `custom_json` **operation
   watching/filtering** support beyond what `StreamOperations` already
   offers (it takes a type filter — confirm `custom_json` with a specific
   `id` field can already be matched, or whether that needs its own
   helper).
5. Not in scope for this session: any witness-committee/quorum design for
   cambium-cosmos's own Hive watcher — that's a cambium-cosmos-side
   architecture decision to make once this data is available, not an
   Anther change. The watcher itself is a cambium-cosmos component, not an
   Anther one (same split VSC uses: their watcher lives in go-vsc-node,
   not in hivego) — this session should not attempt to build it.

## Report back to cambium-cosmos when done

When this session's work lands, produce a short report back for the
cambium-cosmos side (append to or reference from
`x/hiveidentity/decisions.md` there) covering:

- What `Authority`/`AccountData` shape actually landed (field names/types),
  since the watcher will need to consume it.
- Whether `StreamOperations`' existing type filter can already match a
  specific `custom_json` id cleanly, or whether a small generic filter
  helper was added — this determines how much op-matching logic the
  cambium-cosmos watcher has to hand-roll itself versus getting from
  Anther.
- Anything that changed in scope items 1-4 above during implementation.

## Result (landed 2026-07-13)

**Shape that landed** — `types.Authority` (types/types.go), not a separate
read/write pair:

```go
type Authority struct {
    WeightThreshold uint32
    AccountAuths    map[string]uint16
    KeyAuths        map[string]uint16
}
```

`transaction.Authority` is now `= types.Authority` (a type alias), so
`x/hiveidentity` or any Go caller building/signing operations and any
caller reading `GetAccounts` results use the exact same type. `Authority`
has custom `UnmarshalJSON`/`MarshalJSON` that convert to/from Hive's real
wire format — `account_auths`/`key_auths` as arrays of `[name-or-key,
weight]` tuples, not JSON objects. `types.AccountData` gained:

```go
Owner    Authority `json:"owner"`
Active   Authority `json:"active"`
Posting  Authority `json:"posting"`
MemoKey  string    `json:"memo_key"`
```

**Bonus fix, not originally scoped**: adding the JSON codec exposed that
`Authority`'s old JSON tags were never actually exercised — operations
like `create_claimed_account`/`account_update` broadcast their `Authority`
fields through `json.Marshal` (`transaction.Transaction.Broadcast`), which
would have serialized `account_auths`/`key_auths` as JSON objects instead
of Hive's expected tuple arrays. That direction is now also correct,
since the same `MarshalJSON` is used both ways.

**custom_json id matching (item 4)** — `StreamOperations` still only
filters by op-type string (unchanged). Added `types.OperationTuple.CustomJSONID() (string, bool)`,
so a watcher can do:

```go
if id, ok := op.Op.CustomJSONID(); ok && id == "hiveidentity" {
    // handle it
}
```
without hand-rolling the `op.Op[0]`/`op.Op[1]` type assertions itself.
No change to `StreamOperations`'s signature.

**Tests**: `types/types_test.go` (`TestAuthorityJSON`,
`TestOperationTupleCustomJSONID`) and an extended `GetAccounts` fixture/
assertions in `client/client_extension_test.go` cover round-trip parsing
of a realistic `owner`/`active`/`posting` payload. `go build ./...`,
`go vet ./...`, and `go test ./...` all pass.

**Not done**: `docs/API.md` was not regenerated — the installed
`gomarkdoc@latest` produces ~750 lines of unrelated diff against the
checked-in file (older gomarkdoc version, plus some pre-existing doc/code
drift like `GetReputation`'s return type). Regenerate separately with
whatever gomarkdoc version this repo normally pins, if desired.
