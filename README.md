# bydb

A distributed, replicated document database written in Go. Built as a learning project to understand how consensus algorithms, on-disk state machines, and distributed storage fit together in practice.

## What it is

bydb is a partitioned document store with full-text search, replicated across a cluster using the RAFT consensus protocol. Writes are linearisable — a write is only acknowledged once it has been committed to a quorum of nodes. Reads are served locally after checking a pending-write queue for uncommitted entries.

The storage model is simple: documents live in named partitions, are identified by an ID, and carry an indexed set of fields for search alongside a private set of fields that are stored but not indexed.

## Architecture

```
Client (gRPC)
     │
     ▼
  API layer
     │
     ├── Reads  → SyncRead()  → Lookup()  (local, non-blocking)
     └── Writes → SyncPropose() → Update() → Sync() (committed to quorum)
                                     │
                              RAFT log (Dragonboat)
                                     │
                              On-disk state machine
                                     │
                    ┌────────────────┴────────────────┐
                    │          Shard map (36 shards)   │
                    │    keyed by first char of partID │
                    │                                  │
                    │  BoltDB (document storage)       │
                    │  Bleve  (full-text search index) │
                    └──────────────────────────────────┘
```

**Consensus:** [Dragonboat](https://github.com/lni/dragonboat) handles the RAFT protocol — leader election, log replication, and quorum writes. bydb implements the `IOnDiskStateMachine` interface, which means the state machine survives process restarts and handles its own snapshotting.

**Storage:** Each shard owns a BoltDB instance (document storage) and a Bleve index (full-text search). Sharding is by the first character of the partition ID, giving 36 shards across the alphabet and digits.

**Snapshots:** The state machine snapshots by tarring the entire data directory and streaming it via the Dragonboat snapshot interface, enabling new nodes to bootstrap from a snapshot rather than replaying the full log.

**Pending reads:** During the window between a write being proposed and being committed to disk via `Sync()`, reads check an in-memory pending queue first. This avoids a stale read immediately after a successful write.

## Command protocol

Commands are text strings passed over gRPC. The full set:

| Command | Format | Description |
|---|---|---|
| `POST` | `POST <json>` | Insert a new document (409 if ID exists) |
| `PUT` | `PUT <ts> <json>` | Update an existing document (409 on timestamp mismatch) |
| `DEL` | `DEL <part> <id> <ts>` | Delete a document (409 on timestamp mismatch) |
| `GET` | `GET <part> <id>` | Fetch a document by ID |
| `SEARCH` | `SEARCH <part> <query>` | Full-text search within a partition |
| `CREATE_PART` | `CREATE_PART <name>` | Create a partition (409 if exists) |
| `DELETE_PART` | `DELETE_PART <name>` | Delete a partition and all its documents |
| `JOIN_NODE` | `JOIN_NODE <address>` | Add a new node to the cluster |
| `REMOVE_NODE` | `REMOVE_NODE <id>` | Remove a node from the cluster |

PUT and DEL use timestamp-based conflict detection — the timestamp in the command must match the document's current `Updated` field, similar to an optimistic lock.

## Document format

```yaml
---
part: "users"
id: "abc123"
type: "user"
index:
  name: "Alice"
  role: "admin"
private:
  password_hash: "..."
```

Fields under `index` are stored in the Bleve full-text search index. Fields under `private` are stored in BoltDB only and excluded from search.

## Building

```bash
make
```

Produces three binaries: `server`, `client`, and `preseed`.

## Running a local 3-node cluster

The test config bootstraps three nodes on localhost. Run each in a separate terminal:

```bash
./server -config test-run-config.yml -replicaid 1 -gaddr localhost:64001
./server -config test-run-config.yml -replicaid 2 -gaddr localhost:64002
./server -config test-run-config.yml -replicaid 3 -gaddr localhost:64003
```

Then use the client to interact:

```bash
./client -addr localhost:64001
```

## Known limitations

- Config file path defaults to `test-run-config.yml` — not suitable for production deployment as-is
- The listen address and advertise address must match; this causes issues when joining nodes behind NAT
- Single shard group — no cross-shard transactions
- No authentication on the gRPC API

## Stack

- [Dragonboat](https://github.com/lni/dragonboat) — RAFT consensus
- [BoltDB](https://github.com/boltdb/bolt) — embedded key-value storage
- [Bleve](https://github.com/blevesearch/bleve) — full-text search
- [gRPC](https://grpc.io/) + Protocol Buffers — API transport
