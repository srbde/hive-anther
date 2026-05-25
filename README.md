# 🌿 Anther

**The modern, secure, and production-ready Go SDK for the Hive blockchain. Built for 2026 and beyond.**

Go is built for high-performance concurrency, network resilience, and robust system-level tooling. **Anther** brings those strengths to the Hive blockchain. It is designed from the ground up to provide secure, offline transaction serialization and signing, multi-signature authority validation, and native goroutine/channel-based blockchain streaming.

If you are building high-throughput backend services, bots, or block indexers on Hive, Anther is your foundation.

---

**Secured and Native:** Anther uses the battle-tested **secp256k1** and **btcutil** packages, and implements a native binary serialization engine. It has zero external dependencies for protocol-level serialization and signing.

---

## Why Anther?

The Hive ecosystem deserves infrastructure that is safe and fast by default.

### 🔒 Audited Cryptography & Local Serialization

Anther strips out deprecated RPC-based serialization (`get_transaction_hex`). In its place:

- **[btcutil](https://github.com/btcsuite/btcd/tree/master/btcutil)**: Audited, robust package for handling WIF key formats and Base58 checksum encodings.
- **[secp256k1/ecdsa](https://github.com/decred/dcrd/tree/master/dcrec/secp256k1)**: Uses Decred's secp256k1 compact signature engine to generate canonical signatures (s ≤ N/2) and recovery IDs natively.
- **Offline Serialization**: Encodes operations (`Vote`, `Comment`, `Transfer`, `CommentOptions`, `CreateClaimedAccount`, `AccountUpdate`, etc.) into exact consensus-compatible wire bytes locally.

### ⚡ Concurrency & Blockchain Streaming

Anther leverages Go's concurrency primitives to provide native blockchain feeds:

- **Goroutine & Channel Streams**: Streams blocks (`StreamBlocks`) or applied/virtual operations (`StreamOperations`) natively via Go channels.
- **Node Failover & Retry Backoff**: Transparently retries failed requests and falls over to alternative nodes with exponential backoff.
- **Context-Aware API**: Fully supports Go's `context.Context` for deadlines and cancellation across all network operations.

### 🔒 Private Memo Encryption (ECIES)

Anther features built-in support for ECIES memo encryption and decryption matching Pollen and Nectar implementations:

- **secp256k1 ECDH**: Derives shared secrets using Elliptic Curve Diffie-Hellman (`btcec.GenerateSharedSecret`).
- **AES-CBC-256 PKCS#7**: Secure encryption/decryption with in-memory padding validation.
- **Fallback Decryption**: Transparently falls back to raw strings for legacy/unpadded memos.

### 🔌 Ecosystem Alignment

Anther is the Go counterpart to **[Pollen](https://github.com/srbde/pollen)** (TypeScript) and **[Nectar](https://github.com/srbde/hive-nectar)** (Python). Together, they form a unified, secure foundation for building cross-platform Hive applications under the **SRBDE** umbrella.

---

## 🚀 Quick Start

Requires Go >= 1.20.

```bash
go get github.com/thecrazygm/anther
```

### Read account data (Go)

```go
package main

import (
	"fmt"
	"log"

	"github.com/thecrazygm/anther/account"
	"github.com/thecrazygm/anther/client"
)

func main() {
	// Initialize client
	api := client.NewClient([]string{"https://api.hive.blog"}, 30)

	// Fetch account
	acc := account.NewAccount("thecrazygm", api)
	if err := acc.Refresh(); err != nil {
		log.Fatalf("failed to fetch account: %v", err)
	}

	fmt.Printf("Account:      %s\n", acc.Name)
	fmt.Printf("HIVE Balance: %v\n", acc.Data["balance"])
	fmt.Printf("HBD Balance:  %v\n", acc.Data["hbd_balance"])
}
```

### Sign and broadcast a transaction

```go
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/thecrazygm/anther/client"
	"github.com/thecrazygm/anther/transaction"
)

func main() {
	api := client.NewClient([]string{"https://api.hive.blog"}, 30)
	tx := transaction.NewTransaction(api)

	// Add operations
	tx.AppendOp(&transaction.Transfer{
		From:   "youraccount",
		To:     "recipient",
		Amount: "0.001 HIVE",
		Memo:   "Sent with Anther 🌿",
	})

	// Sign transaction offline
	wif := os.Getenv("ACTIVE_WIF")
	if err := tx.Sign(wif); err != nil {
		log.Fatalf("failed to sign transaction: %v", err)
	}

	// Broadcast
	result, err := tx.Broadcast()
	if err != nil {
		log.Fatalf("failed to broadcast: %v", err)
	}
	fmt.Printf("Transaction broadcasted: %v\n", result)
}
```

### Stream operations live

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/thecrazygm/anther/client"
)

func main() {
	api := client.NewClient([]string{"https://api.hive.blog"}, 30)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Stream irreversible transfer operations
	ops, errs := api.StreamOperations(ctx, 0, client.Irreversible, []string{"transfer"})

	for {
		select {
		case op := <-ops:
			if op != nil {
				fmt.Printf("Transfer [Block %d]: %v\n", op.Block, op.Op[1])
			}
		case err := <-errs:
			if err != nil {
				log.Printf("Streaming error: %v", err)
			}
		case <-ctx.Done():
			fmt.Println("Streaming stopped.")
			return
		}
	}
}
```

---

## 🛠️ Building and Testing

Anther uses standard Go tooling alongside a structured Makefile.

```bash
# Auto-format codebase
make fmt

# Run all unit tests with the race detector
make test

# Compile all modules
make build
```

---

## 📜 Standing on Shoulders

Anther is a completely original Go library designed from the ground up to bring Hive development to the Go ecosystem. It was built using the TAPOS headers, transaction signatures, and cryptographic standards verified by the SRBDE team to ensure 100% mathematical consensus compatibility with the Hive blockchain.

---

## 🌐 Built by SRBDE

**Anther** is developed and maintained by the **Sustainable Resource and Business Development Enterprise (SRBDE)** — an open-source infrastructure organization building tools and platforms for communities that build things together.

We apply the logic of agricultural sustainability to software: the goal is always to return more to the ecosystem than we extract.

- **Open source is our value, not just our business model.**
- **Our commercial products fund our open-source core. The open work is the mission.**

### Explore the Ecosystem

| Project                                               | Description                       |
| ----------------------------------------------------- | --------------------------------- |
| [Pollen](https://github.com/srbde/pollen)             | The modern Hive TypeScript SDK    |
| [Anther](https://github.com/thecrazygm/anther)        | The modern Hive Go SDK            |
| [Xylem](https://github.com/srbde/xylem)               | The modern Hive Rust SDK          |
| [Hive-Nectar](https://github.com/srbde/hive-nectar)   | The modern Hive Python SDK        |
| [nectarengine](https://github.com/srbde/nectarengine) | The Hive-Engine sidechain library |
| [ecoinstats.net](https://ecoinstats.net)              | SRBDE corporate hub               |
| [thecrazygm.com](https://thecrazygm.com)              | Open gaming tools & TTRPGs        |

---

## 🤝 Contributing

Audits, forks, and pull requests are welcome. **Anther** is built to last for the decade, not the quarter. If you find a security issue, please open a private advisory rather than a public issue.
