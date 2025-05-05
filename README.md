# ETH TX Parser

`ethtxparser` is a **simple Ethereum transaction indexer** and REST API
showcase written in Go. It connects to an Ethereum JSON‑RPC endpoint (mainnet or any
testnet), watches new blocks, filters transactions for **subscribed
addresses**, and exposes the results over HTTP together with Prometheus
metrics.

---

## What it does

1. **Polls an Ethereum node** every *poll‑interval* (default 10s).
2. Keeps a ring‑buffer of the last *N* blocks (default 3) to absorb normal
   1–2‑block re‑orgs.
3. Flushes a block to the store only when it is *N*‑deep
   (confirmation depth).
4. While indexing each flushed block it:
    * Iterates every transaction
    * Normalises `from` and `to` addresses (lower‑case)
    * Checks if either address is in the **subscription store**
    * Saves matching txs in memory.
5. Exposes REST endpoints for current block, transactions per address,
   and subscription management.
6. Publishes custom Prometheus metrics (`/metrics`).

---

## Run it

```bash
go run ./cmd/ethtxparser \
  --server-addr    localhost:8080 \
  --node-addr      https://ethereum-rpc.publicnode.com \
  --poll-interval  10s \
  --reorg-confirmation-depth 3 \
  -v
```

---

## REST API

| Verb    | Path                              | Description                                  |
|---------|-----------------------------------|----------------------------------------------|
| **GET** | `/api/v1/blocks/current`          | Return the last confirmed block number.      |
| **GET** | `/api/v1/transactions/{address}`  | List all indexed txs involving `{address}`.  |
| **PUT** | `/api/v1/subscriptions/{address}` | Subscribe to an address (idempotent).        |
| **GET** | `/api/v1/subscriptions/`          | List all current subscriptions.              |
| **GET** | `/metrics`                        | Prometheus metrics (only custom collectors). |

All addresses can be with or without the `0x` prefix and checksum; they are
stored lower‑case internally.

---

## Internals

```text
              ┌──────────┐  (polls)         ┌──────────────┐   (confirmed)   ┌─────────────┐  (matched txs)  ┌────────────────┐
 ETH-RPC ───▶ │ eth.Cli  │ ────────────────▶│ ReorgFilter  │ ──────────────▶ │  Indexer    │ ───────────────▶│ memdb.TxStore  │
              └──────────┘  stream of blocks└──────────────┘  stream of safe └─────────────┘                 └────────────────┘
                                                                  blocks            │ addr look‑ups                  ▲
                                                                                    │                                │
                                                                               ┌─────────────────────┐               │
                                                                               │ memdb.Subscription  │               │
                                                                               └─────────────────────┘               │
                                                                                         ▲                           │
                                                                                         │                           │ 
                                                                                   REST server ──────────────────────┘
```

### Component flow

1. **eth.Client**  
   *Actively polls* the configured JSON‑RPC endpoint at the given `--poll-interval`,  
   emitting a stream (Go channel) of the *latest* blocks.

2. **ReorgFilter**  
   Maintains a ring buffer of the last *N* blocks (default 3).  
   If a block’s `parentHash` doesn’t link, it pops the forked tip(s) and only
   forwards blocks that are **N‑deep** — effectively “confirmed”.

3. **Indexer**  
   Consumes confirmed blocks.  
   For every transaction it lower‑cases `from` and `to` and checks both
   addresses against **memdb.SubscriptionStore** (constant‑time map hits).  
   Matches are written to **memdb.TxStore**.

> **Note on look‑ups:** for simplicity each tx does two direct map look‑ups.
> Production‑scale options:
> - **Batch address look‑ups per block**  
    Build a de‑duplicated set of all `from`/`to` addresses in the block and query the store once, instead of two map hits per transaction.
> - **Bloom filter**  
    Insert a fast in‑memory check before hitting the on‑disk subscription table when the user base becomes very large.
> - **In‑memory store (e.g. Redis)**  
    Can be persistent across restarts and accessible by multiple parser instances.

---

## Metrics

| Metric name                                  | Meaning                                                                   |
|----------------------------------------------|---------------------------------------------------------------------------|
| `ethtxparser_block_retrievals_total`         | Number of **successful** full‑block RPC retrievals                        |
| `ethtxparser_failed_block_retrievals_total`  | Number of **failed** full‑block RPC retrieval attempts                    |
| `ethtxparser_blocks_processed_total`         | Total number of blocks **consumed** by the indexer (before any filtering) |
| `ethtxparser_blocks_failed_processing_total` | Blocks that **failed during processing**                                  |
| `ethtxparser_indexed_transactions_total`     | Total transactions **successfully stored** for subscribed addresses       |
| `ethtxparser_reorg_dropped_blocks_total`     | Blocks **dropped** from the ring buffer because of chain re‑organizations |

---

## Tests
### Tests
Coverage is intentionally minimal; the included cases just illustrate the project's table‑driven unit testing style.

---
