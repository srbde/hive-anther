# Session Brief: Batched Operation Catch-Up

Prep doc for a dedicated Anther session. Written from the cambium-cosmos
side, surfaced while thinking through what a real Hive watcher (run by each
Cambium validator, per `x/hiveidentity/decisions.md` #0e) actually needs
from Anther's streaming API.

## Context

`x/hiveidentity`'s design has every Cambium validator run its own
long-lived watcher process against Hive, independently observing
`custom_json` registration ops and voting on what it saw (see
`x/hiveidentity/watcher` and `decisions.md` #0d/#0e in cambium-cosmos).
That depends entirely on Anther's `Client.StreamOperations`/`StreamBlocks`.

Checked what's already there before assuming anything was missing:

- `StreamBlocks`/`StreamOperations` (`client/client.go`) already support an
  `Irreversible` `StreamingMode` — exactly the trust boundary this design
  needs, since validators should only attest to Hive state that can't be
  rolled back by a fork.
- Both accept a `startBlock` parameter, so a watcher can persist its own
  last-processed height and resume from there after a restart. The resume
  mechanism itself is already there.
- `GetBlockRange` (batch, up to 1000 blocks) and `GetOpsInBlock` (single
  block) both already exist.

So this is **not** a "streaming doesn't exist" gap — Phase 9 already
covered that, and covered it well.

## The actual gap

`StreamOperations`'s internal loop (`client/client.go`, the `for seen <=
current` loop) calls `c.GetOpsInBlock(seen, false)` **one block at a
time**, sequentially, even though `GetBlockRange` proves the node can
return up to 1000 blocks in a single RPC round-trip.

For a validator continuously live-tailing Hive, one-block-at-a-time is
fine — new blocks arrive every ~3 seconds anyway, so there's no backlog to
clear. But if a validator's watcher process is offline for a while
(maintenance, crash, redeploy) and comes back needing to catch up a large
backlog of blocks before it can resume voting, the current per-block
round-trip pattern means catch-up time scales with backlog size at
one-RPC-call-per-block, which is slower than it needs to be.

## Suggested session scope

1. Add a batched historical fetch path — something like
   `GetOpsInBlockRange(startBlock, count uint32) ([]*types.AppliedOperation, error)`
   built on `get_ops_in_block` calls batched via the node's batch JSON-RPC
   support (if the node supports it) or otherwise pipelined more
   aggressively than the current one-at-a-time loop, respecting the same
   1000-block ceiling `GetBlockRange` already enforces.
2. Consider whether `StreamOperations` itself should detect "I'm far
   behind head/irreversible" and switch to the batched path automatically
   until caught up, then fall back to the current live one-block-at-a-time
   behavior — or whether that orchestration belongs in the caller
   (cambium-cosmos's watcher) instead, using the new batched method as a
   building block. Lean toward keeping this decision in Anther's scope
   only if it's a small, low-risk addition; otherwise leave the
   catch-up-vs-live switch to the watcher.
3. Add a benchmark or rough timing test comparing current per-block
   catch-up vs. the batched path over a realistic backlog (e.g. a few
   thousand blocks) to confirm this is actually worth the added surface
   area before committing to it.

## Not yet a confirmed need

This is speculative until an actual cambium-cosmos watcher exists and
hits this bottleneck in practice — flagging it now so the idea isn't lost,
not because it's urgent. No action needed on the cambium-cosmos side until
this lands; nothing there currently depends on it.

## Result (landed 2026-07-13)

Went with the low-risk option: **bounded-concurrency pipelining**, not
true JSON-RPC batch requests. Hive has no `get_ops_in_block_range` RPC
and no confirmed batch-array support across all node implementations
(hived vs. jussi-fronted public nodes), so item 1's "batched via the
node's batch JSON-RPC support" half was dropped in favor of the "otherwise
pipelined more aggressively" half.

Added `client.GetOpsInBlockRange(startingBlockNum, count uint32) ([]*types.AppliedOperation, error)`
(client/client.go) — fans out `count` concurrent `GetOpsInBlock` calls
through a worker pool bounded by `opsInBlockRangeConcurrency` (20),
preserves block order in the returned slice regardless of completion
order, and enforces the same 1000-block ceiling as `GetBlockRange`.
`StreamOperations` itself was left untouched — no auto-switch between
catch-up and live modes, per item 2's own lean toward keeping that
decision out of Anther unless trivial. Orchestrating "detect I'm behind,
call `GetOpsInBlockRange` in a loop until caught up, then switch to
`StreamOperations`" is left to the cambium-cosmos watcher.

**Item 3 (benchmark) — done against real Hive nodes**, not synthetic
timing. Used `github.com/thecrazygm/nectarflower-go` to pull a live
healthy-node list from the `nectarflower` account's on-chain metadata,
pointed a real `client.Client` at those nodes, and compared 200 real
blocks (7,236 ops) fetched sequentially vs. via the new
`GetOpsInBlockRange`:

| Path | Time | Rate |
|---|---|---|
| Sequential (old behavior) | 55.7s | 3.6 blocks/sec |
| Concurrent (`GetOpsInBlockRange`) | 9.4s | 21.3 blocks/sec |

**~5.9x speedup**, op counts identical between both paths (correctness
check). Confirms the bottleneck is real and the fix is worth the added
surface area. For a validator catching up a multi-thousand-block backlog
after downtime, this is the difference between minutes and closer to
half a minute per thousand blocks.

The benchmark script itself was a throwaway (used `nectarflower-go` as a
dev-only tool, not added as an Anther dependency) and was not committed —
this table is the durable record of the result. If a permanent in-repo
benchmark is wanted later, `client/client_test.go`'s new
`TestGetOpsInBlockRange` (mocked, deterministic) is the place to extend,
but it doesn't replace the real-node numbers above.

Tests: `client/client_test.go`'s `TestGetOpsInBlockRange` covers block
ordering, the concurrency cap, and the 0/1000+ validation errors, against
a mock server. `go build ./...`, `go vet ./...`, `go test ./...` all pass.
