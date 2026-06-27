# Project Anther: Migration & Modernization Plan

This document outlines the strategic roadmap for bringing **`anther`** to a feature-complete, production-ready v1.0.0 release.

---

## ✅ Phase 1: Rebranding & Identity (COMPLETED)

- [x] **Module Renaming**: Updated `go.mod` path from `github.com/thecrazygm/nectar-go` to `github.com/srbde/hive-anther`.
- [x] **Global Search & Replace**: Migrated all imports of `github.com/thecrazygm/nectar-go` to `github.com/srbde/hive-anther`.
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

## ✅ Phase 8: Comprehensive Operation Coverage (COMPLETED)

- [x] **Social & Curation Operations**:
  - [x] Implemented `comment_options` (ID 19) operation structure, dictionary mapping, and local wire serialization.
  - [x] Implemented `delete_comment` (ID 17) operation structure and serialization.
- [x] **Financial Operations**:
  - [x] Implemented `transfer_to_vesting` (ID 3) structure and serialization.
  - [x] Implemented `withdraw_vesting` (ID 4) structure and serialization.
  - [x] Implemented `delegate_vesting_shares` (ID 40) structure and serialization.
  - [x] Implemented `claim_reward_balance` (ID 39) structure and serialization.
  - [x] Implemented `recurrent_transfer` (ID 49) structure and serialization.
- [x] **Account Management Operations**:
  - [x] Implemented `claim_account` (ID 22) structure and serialization.
  - [x] Implemented `create_claimed_account` (ID 23) structure and serialization.
  - [x] Implemented `account_update` (ID 10) structure and serialization.

## ✅ Phase 9: Blockchain Streaming (COMPLETED)

- [x] **Live Block Stream**:
  - [x] Implemented `StreamBlocks` method on Client using concurrent goroutines and channels.
  - [x] Handled automatic block height polling, polling latency, backfilling missing blocks, and node consensus latency.
- [x] **Live Operation Stream**:
  - [x] Implemented `StreamOperations` method on Client.
  - [x] Supported client-side type-based filtering of incoming operations.

## ✅ Phase 10: Private Memo Encryption & Decryption (COMPLETED)

- [x] **ECDH Shared Secret**: Derived shared secrets offline using secp256k1 private keys and recipient public keys.
- [x] **AES-256-CBC Payload Encryption**: Implemented AES-256-CBC cipher block chaining with PKCS#7 padding.
- [x] **SHA-512 Verification**: Added message authentication checks to prevent packet tampering.
- [x] **Base58 Formatting**: Handled `#` prefixing and standard Hive-compatible Base58 memo envelopes.

## ✅ Phase 11: Advanced Authority & Multi-Signature (COMPLETED)

- [x] **Multi-Sig Transactions**: Allowed signing a transaction with multiple WIF keys sequentially via `SignMany`.
- [x] **Public Key Recovery**: Recovered public keys from compact signatures via `RecoverKeyFromSignature`.
- [x] **Authority Threshold Checking**: Implemented local `VerifyAuthority` checking threshold limits for account and key authorities.

## ⏳ Phase 12: High-Level API Expansion & Parity (v1.0.0 Target)

To reach full v1.0.0 functionality and bring Anther to parity with Pollen and Nectar, we need to surface the remaining database, Resource Credit (RC), Hivemind, and simplified broadcast wrappers.

### 1. Database API Completeness

Surface direct methods on `Client` to query secondary blockchain properties:

- [ ] **`GetConfig`**: Query compiler configurations from the node.
- [ ] **`GetChainProperties`**: Fetch current chain limits (minimum delegation fees, HBD interest rates).
- [ ] **`GetCurrentMedianHistoryPrice`**: Retrieve the median conversion price for HIVE/HBD.
- [ ] **`GetAccounts`**: Query profile details for multiple accounts in a single call.
- [ ] **`GetAccountHistory`**: Query transaction history for a user, supporting pagination.
- [ ] **`GetVestingDelegations`**: Fetch active delegation logs.
- [ ] **`GetBlockHeader`**: Fetch only block headers (witness, timestamp, previous) to save bandwidth.

### 2. Resource Credit (RC) Sub-API

Surface Resource Credit monitoring APIs in `Client` matching Pollen's `rc` helper:

- [ ] **`GetRCParams`**: Retrieve global RC cost parameters.
- [ ] **`GetRCPool`**: Retrieve current global resource availability pools.
- [ ] **`GetRCMana`**: Fetch a user's current Resource Credit manabar details.
- [ ] **Mana Math & Calculations**:
  - [ ] Implement `CalculateRCMana` (compute real-time regenerated RC).
  - [ ] Implement `CalculateVPMana` (compute real-time regenerated Voting Power).

### 3. Social & Content (Hivemind API)

Surface methods to browse social posts, communities, and notifications matching Pollen's `hivemind` helper:

- [ ] **`GetRankedPosts`**: Fetch post feeds based on rank (trending, hot, created, promoted).
- [ ] **`GetAccountPosts`**: Fetch post feeds authored by, replied to, or voted on by a user.
- [ ] **`GetCommunity`**: Retrieve profile details for a community.
- [ ] **`ListCommunities`**: Query communities registry with filtering and sorting.
- [ ] **`GetAccountNotifications`**: Fetch user notification feeds (mentions, reblogs, replies).

### 4. Simplified Quick Broadcast Helpers

Add high-level helper functions on `Client` to sign and broadcast common operations in a single line (avoiding manual Transaction assembly):

- [ ] **`client.BroadcastVote(voter, author, permlink, weight, wif)`**
- [ ] **`client.BroadcastTransfer(from, to, amount, memo, wif)`**
- [ ] **`client.BroadcastComment(author, permlink, parentAuthor, parentPermlink, title, body, jsonMetadata, wif)`**
- [ ] **`client.BroadcastCustomJSON(id, jsonString, requiredPostingAuths, wif)`**

---

_Document updated on Sunday, May 24, 2026. Roadmap finalized for v1.0.0._
