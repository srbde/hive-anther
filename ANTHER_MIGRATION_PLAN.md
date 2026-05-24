# Project Anther: Migration & Modernization Plan

This document outlines the strategic roadmap for bringing **`anther`** to a feature-complete, production-ready v1.0.0 release.

---

## ✅ Phase 1: Rebranding & Identity (COMPLETED)

- [x] **Module Renaming**: Updated `go.mod` path from `github.com/thecrazygm/nectar-go` to `github.com/thecrazygm/anther`.
- [x] **Global Search & Replace**: Migrated all imports of `github.com/thecrazygm/nectar-go` to `github.com/thecrazygm/anther`.
- [x] **README & License Modernization**: Aligned documentation with the plant theme and updated references.

## ✅ Phase 2: Tooling & Package Hygiene (COMPLETED)

- [x] **Go Module Tidy**: Cleaned up dependencies and ensured Go 1.26+ compatibility.
- [x] **Linter & Formatter**: Configured standard Go formatting checks.
- [x] **Task Automation**: Provided clear tasks/Makefile for testing, linting, and running examples.

## ✅ Phase 3: Critical Bug Fixes & Refactoring (COMPLETED)

- [x] **HTTP Client Timeout**: Set the HTTP timeout parameter in the internal HTTP client.
- [x] **Connection & Body Leak Prevention**: Prevented socket and body leaks in JSON-RPC request cycles with node failover.
- [x] **Varint String Serialization**: Supported arbitrary length strings by using standard Protocol Varint (Uvarint) encoding.

## ✅ Phase 4: Local Serialization & Offline Signing (COMPLETED)

- [x] **Binary Serialization Writer**: Implemented a local byte serialization helper.
- [x] **Operation Serializers**: Implemented local wire serialization (`Bytes()`) for supported operations (`Vote`, `Comment`, `Transfer`, `CustomJSON`, `Follow`).
- [x] **Transaction Serialization**: Implemented `Bytes()` serialization for `Transaction` struct (incorporating chain ID).
- [x] **Offline Signing Integration**: Signed transactions without depending on RPC `get_transaction_hex`.

## ✅ Phase 5: Network Resilience & Failover (COMPLETED)

- [x] **Exponential Backoff**: Added robust retry policies with exponential backoff for RPC node queries.
- [x] **Node Health Tracker**: Handled automatic node failover when queries fail.

## ✅ Phase 6: Strict Data Integrity & Types (COMPLETED)

- [x] **Typed RPC Response Models**: Converted generic dynamic responses to strongly typed Go structs in `types`.
- [x] **Custom Structured Errors**: Refactored exceptions to use `AntherError` and added specific `RPCError`/`SerializationError` types.

## ✅ Phase 7: Knowledge Garden & Examples (COMPLETED)

- [x] **Offline Signing Example**: Added clean interactive examples of local transaction construction, signing, and broadcasting.
- [x] **Modern Documentation**: Cleaned, structured API documentation inside all code headers.

---

## ⏳ Phase 8: Comprehensive Operation Coverage (v1.0.0 Target)

Expand local serialization, types, and operation structures to cover the full dApp operation suite:
- [ ] **Social & Curation Operations**:
  - [ ] Implement `comment_options` operation (ID 19) structure, dict parsing, and local wire serialization.
  - [ ] Implement `delete_comment` operation (ID 17) structure and serialization.
- [ ] **Financial Operations**:
  - [ ] Implement `transfer_to_vesting` operation (ID 3) structure and serialization.
  - [ ] Implement `withdraw_vesting` operation (ID 4) structure and serialization.
  - [ ] Implement `delegate_vesting_shares` operation (ID 40) structure and serialization.
  - [ ] Implement `claim_reward_balance` operation (ID 39) structure and serialization.
  - [ ] Implement `recurrent_transfer` operation (ID 49) structure and serialization.
- [ ] **Account Management Operations**:
  - [ ] Implement `claim_account` operation (ID 22) structure and serialization.
  - [ ] Implement `create_claimed_account` operation (ID 23) structure and serialization.
  - [ ] Implement `account_update` operation (ID 10) structure and serialization.

## ⏳ Phase 9: Blockchain Streaming (v1.0.0 Target)

Build native, concurrent, channel-based blockchain streams:
- [ ] **Live Block Stream**:
  - [ ] Implement `StreamBlocks(ctx context.Context, startBlock uint32) (chan *types.Block, chan error)` method on Client.
  - [ ] Add automatic block height tracking, polling, and missing block backfilling.
  - [ ] Support context cancellation to cleanly stop streaming goroutines.
- [ ] **Live Operation Stream**:
  - [ ] Implement `StreamOperations(ctx context.Context, startBlock uint32, filterOps ...string) (chan types.Operation, chan error)` method on Client.
  - [ ] Implement automatic operation parsing and filtering based on operation names.

## ⏳ Phase 10: Private Memo Encryption & Decryption (v1.0.0 Target)

Implement key-based message security matching Pollen's memo implementation:
- [ ] **ECDH Shared Secret Derivation**: Derive shared secrets from secp256k1 private keys and recipient public keys.
- [ ] **AES-CBC-256 Encryption**: Secure payload encryption with PKCS#7 padding.
- [ ] **SHA-512 Hash Authentication**: Add double-hash verification checks on encrypted payloads.
- [ ] **Private Memo Formatter**: Support encrypting/decrypting memos prefixed with `#`.

## ⏳ Phase 11: Advanced Authority & Multi-Signature (v1.0.0 Target)

- [ ] **Multi-Signature Transactions**: Sign a single transaction with keys representing different roles/accounts.
- [ ] **Public Key Recovery**: Recover the public key from compact signatures.
- [ ] **Authority Verification**: Implement local check verifying if a transaction's signatures meet the consensus authority thresholds.

---

_Document updated on Sunday, May 24, 2026. Mapped out to v1.0.0._
