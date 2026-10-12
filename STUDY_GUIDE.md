# 🌱 Sprig-DB: Complete Technical Study Guide & Presentation Notes

> **Quick Links:**
> - Printable PDF: [SprigDB_Master_Presentation_and_Study_Guide.pdf](file:///home/mr_fool/golang/sprig-db/SprigDB_Master_Presentation_and_Study_Guide.pdf)
> - Styled HTML: [study_guide.html](file:///home/mr_fool/golang/sprig-db/study_guide.html)

---

## 📋 Table of Contents
1. [Executive Presentation Pitch & Novelty](#1-executive-presentation-pitch--novelty)
2. [Concepts Learned & The Learning Journey](#2-concepts-learned--the-learning-journey)
3. [System Architecture & Transaction Pipeline](#3-system-architecture--transaction-pipeline)
4. [Line-by-Line & Component-by-Component Breakdown](#4-line-by-line--component-by-component-breakdown)
   - [`sprig/sprig.go` (Engine & Lifecycle)](#a-sprigspriggo--the-engine--lifecycle)
   - [`sprig/index.go` (Secondary B-Tree Indexing)](#b-sprigindexgo--secondary-b-tree-indexing)
   - [`sprig/filter.go` (Query Builder & Big-Endian Storage)](#c-sprigfiltergo--query-builder--mutation-safety)
   - [`api/schema.go` (Schema Enforcement & Dry-Run)](#d-apischemago--dynamic-schemas--dry-run)
   - [`api/sql_query.go` (SQL-on-NoSQL Engine)](#e-apisql_querygo--sql-parsing--execution-engine)
   - [`api/auth.go` (Multi-Tenancy & SVG CAPTCHA)](#f-apiauthgo--multi-tenancy--svg-captcha)
   - [`cmd/loadtest/main.go` (Concurrency & Load Tests)](#g-cmdloadtestmaingo--high-concurrency-benchmarking)
5. [Minute-by-Minute Presentation Script](#5-minute-by-minute-presentation-script)
6. [Top Viva / Interview Defense Questions & Answers](#6-top-viva--interview-defense-qa)

---

## 1. Executive Presentation Pitch & Novelty

### The Problem Statement
Modern application development faces a dilemma:
- **Traditional Client-Server DBMSs (Postgres, Mongo):** Heavy, require Docker containers, external background processes, network overhead, connection pools, and maintenance. Overkill for desktop tools, CLI utilities, and embedded edge devices.
- **Raw Embedded Key-Value Stores (raw BoltDB, RocksDB):** Only store raw byte slices (`[]byte -> []byte`). They offer **zero** document awareness, **no** schema validation, **no** secondary indexing, and **no** query language.
- **SQLite:** Rigid and table-oriented; handling polymorphic nested JSON requires clunky JSON functions without native document collection abstractions.

### The Novelty of Sprig-DB
> **"Sprig-DB bridges the gap between raw embedded key-value storage and high-level document databases."**

1. **Embedded ACID Engine:** Zero external dependencies or daemons; single `.sprig` file backed by `bbolt`'s memory-mapped B+Tree.
2. **Automated Secondary B-Tree Indexing:** Extracts document fields into secondary B-Tree buckets using composite prefix keys (`value:id`), giving $O(\log N + K)$ lookups without scanning the entire collection. Sensitive fields (`password`, `token`, `secret`) are masked automatically.
3. **Pre-Flight Schema Dry-Run (`/schema/check`):** Unlike standard databases where altering a schema can corrupt or break live data, Sprig-DB evaluates a proposed schema against existing records in memory before committing, showing exact violating document IDs and error reasons.
4. **Dual Query Engine (SQL & NoSQL):** Developers can query documents either through a fluent Go API (`Coll("users").Eq(...).Find()`) or through an interactive SQL console (`SELECT ... WHERE ... ORDER BY ... LIMIT ...`).
5. **Storage-Level Multi-Tenancy:** Multi-user isolation (`_owner` partitioning) baked directly into the engine, allowing multiple tenants to safely share a single embedded database file.
6. **Glassmorphic Zero-Bundle Web Dashboard:** Full admin UI rendered via Echo and **HTMX**, delivering an SPA-like interactive experience without Node.js, Webpack, or client-side build tools.

---

## 2. Concepts Learned & The Learning Journey

| Concept Domain | What You Learned | The Practical Challenge & How You Solved It |
| :--- | :--- | :--- |
| **B+Tree Storage & ACID** | Single-writer, multi-reader transactions, mmap I/O, bucket hierarchies. | Learned how bbolt uses memory-mapped files and Copy-on-Write pages to guarantee ACID durability with zero data corruption. |
| **Binary Key Sorting** | Big-Endian vs. Little-Endian lexicographical sorting in B-Trees. | *Challenge:* Document IDs sorted lexicographically (ID `10` before `2`).<br>*Solution:* Encoded uint64 sequence IDs into 8-byte Big-Endian slices (`binary.BigEndian.PutUint64`), forcing disk order to match numerical order. |
| **Secondary Indexing** | Composite keys, inverted indexes, range cursors. | *Challenge:* Prevent key collisions for documents sharing the same value (e.g., `role: "admin"`).<br>*Solution:* Built composite keys `value:id` so entries group contiguously for prefix seeking while remaining unique. |
| **Concurrency in Go** | `sync.RWMutex`, transaction deadlocks, goroutine safety. | Solved concurrency deadlocks by pairing Read-Write mutexes with bbolt transactions and ensuring all mutations use `defer tx.Rollback()`. |
| **Data Fidelity & Numbers** | IEEE-754 64-bit float precision loss. | Go’s default `json.Unmarshal` casts all numbers to `float64`, truncating large 64-bit ints. Learned to configure custom decoders with `dec.UseNumber()` and write robust type coercers. |
| **Query Parsing & AST** | Tokenization, recursive logic, and clause execution. | Built a custom regex lexer and evaluator that parses SQL into projection arrays, boolean filter trees (`AND`/`OR`), and multi-key stable sorters. |
| **Defensive Programming** | Fail-safe database design. | Implemented `ErrNoFilter` to prevent destructive mutations without a filter (stopping accidental `DELETE *` accidents). |
| **Load Testing & Contention** | Concurrency barriers, atomic counters, latency profiling. | Authored a 200-worker concurrent load tester blasting 25,000 requests to benchmark throughput and verify zero transaction failures. |

---

## 3. System Architecture & Transaction Pipeline

```text
                             +-----------------------------------+
                             |         HTTP / Client Layer       |
                             |  (REST API / Web UI via Echo)     |
                             +-----------------+-----------------+
                                               |
                 +-----------------------------+-----------------------------+
                 |                                                           |
   +-------------v-------------+                               +-------------v-------------+
   |   Authentication & RBAC   |                               |      HTMX Web Dashboard   |
   |  - JWT Bearer & Cookie    |                               |  - Glassmorphic UI        |
   |  - SVG Captcha Generator  |                               |  - Live Collection Browser|
   |  - Bcrypt Password Hash   |                               |  - Interactive SQL Console|
   +-------------+-------------+                               +-------------+-------------+
                 |                                                           |
                 +-----------------------------+-----------------------------+
                                               |
                             +-----------------v-----------------+
                             |         API Server Layer          |
                             |   (/api/records, /api/collections)|
                             +-----------------+-----------------+
                                               |
                 +-----------------------------+-----------------------------+
                 |                                                           |
   +-------------v-------------+                               +-------------v-------------+
   |   Schema Enforcement      |                               |   SQL Query Processor     |
   |  - Type & Range Validation|                               |  - Regex Lexer & Parser   |
   |  - JSON-Schema Draft-04   |                               |  - Logical Evaluator      |
   |  - Dry-Run /check Engine  |                               |  - Projection & Sorter    |
   +-------------+-------------+                               +-------------+-------------+
                 |                                                           |
                 +-----------------------------+-----------------------------+
                                               |
                             +-----------------v-----------------+
                             |     Sprig Core Database Engine    |
                             |  - Filter Query Builder (Eq, ...) |
                             |  - Auto Big-Endian Sequence ID    |
                             |  - Secondary Index Manager        |
                             |  - Mutex Concurrency Control      |
                             +-----------------+-----------------+
                                               |
                             +-----------------v-----------------+
                             |      bbolt (Embedded B+Tree)      |
                             |  - ACID Transactions (Mmap disk)  |
                             |  - Bucket Hierarchy (_idx_, docs) |
                             +-----------------+-----------------+
                                               |
                                      [ default.sprig ]
```

---

## 4. Line-by-Line & Component-by-Component Breakdown

### A. `sprig/sprig.go` — The Engine & Lifecycle
- **Lines 28–33 (`Sprig` struct):**
  - `mu sync.RWMutex`: Guards database operations and collection creation against data races.
  - `db *bbolt.DB`: The underlying BoltDB database handle.
  - `Options`: Stores encoding/decoding strategies and filename configurations.
- **Lines 35–54 (`New`):** Implements the **Functional Options Pattern**. Formats the filename with `.sprig` extension and opens via `bbolt.Open(dbname, 0666, nil)`.
- **Lines 76–91 (`CreateCollection`):**
  - Starts write transaction: `tx, err := h.db.Begin(true)`.
  - **Critical Idiom:** `defer tx.Rollback()`. If anything fails before `tx.Commit()`, rollback is automatic.
- **Lines 94–130 (`DropCollection`):** Deletes the primary collection bucket, then scans all buckets with prefix `_idx_<name>_` to delete related indexes, leaving zero orphaned buckets.
- **Lines 137–153 (`ListCollections`):** Reads buckets via `h.db.View()`, filtering out internal buckets starting with `'_'`.

---

### B. `sprig/index.go` — Secondary B-Tree Indexing
- **Lines 13–17 (`sensitiveFields`):**
  ```go
  var sensitiveFields = map[string]bool{"password": true, "secret": true, "token": true}
  ```
  Prevents storing credentials in index B-Trees.
- **Lines 34–37 (Composite Key Format):**
  `key := fmt.Appendf(nil, "%v:%d", value, id)`
  - Encodes `value:id`. Multiple documents with the same value group contiguously without colliding.
- **Lines 63–91 (`lookupByIndex`):**
  - Creates prefix: `fmt.Appendf(nil, "%v:", value)`.
  - Positions cursor with `c.Seek(prefix)` ($O(\log N)$).
  - Iterates with `c.Next()`, collecting document IDs until the prefix stops matching.

---

### C. `sprig/filter.go` — Query Builder & Mutation Safety
- **Lines 136–144 (`Insert`):**
  - `collBucket.NextSequence()` assigns unique uint64 IDs.
  - Formats ID as 8-byte Big-Endian bytes (`binary.BigEndian.PutUint64`) so numerical order strictly mirrors lexicographical disk order.
  - Calls `updateIndexes(tx, f.coll, id, values)`.
- **Lines 158–196 (`InsertMany`):** Bulk inserts up to 100 documents in a single atomic ACID write transaction.
- **Lines 200–236 (`Find`):** Checks for single-field equality filter to execute `tryIndexFind()` ($O(\log N)$) or falls back to `findFiltered()` (streaming cursor scan).
- **Lines 312 & 354 (`Update` & `Delete`):**
  `if len(f.compFilters) == 0 { return ErrNoFilter }`
  Guards against accidental whole-collection data loss.

---

### D. `api/schema.go` — Dynamic Schemas & Dry-Run
- **Lines 88–139 (`SchemaStore`):** Stores schemas in `_schemas` collection partitioned by `owner` and `name`, with version auto-increment.
- **Lines 273–428 (`ValidateDocument`):** Validates types (`string`, `number`, `bool`, `array`, `object`), ranges (`min`, `max`, `minLength`, `maxLength`), `enum`, and regex `pattern`. Applies defaults automatically.
- **Lines 220–297 in `api/api.go` (`HandleCheckCollectionSchema`):**
  - Dry-run validation endpoint.
  - Tests existing records against a proposed schema in memory.
  - Reports count of valid/invalid records and top 10 error strings before committing changes.

---

### E. `api/sql_query.go` — SQL Parsing & Execution Engine
- **Lines 14–15 (Regex Lexer):** Deconstructs queries:
  `SELECT <fields> FROM <coll> [WHERE <expr>] [ORDER BY ...] [LIMIT n] [OFFSET n]`
- **Lines 43–46 (Tenant Isolation):** Enforces `r["_owner"] == owner`.
- **Lines 182–205 (`evalWhereClause`):** Splits on `OR`, then `AND` (DNF precedence). Evaluates `=`, `!=`, `>`, `<`, `>=`, `<=`, `LIKE`, `IN`.
- **Lines 56–86 (`sort.SliceStable`):** Multi-key sorting with mixed ASC/DESC order.

---

### F. `api/auth.go` — Multi-Tenancy & SVG CAPTCHA
- **Lines 81–92 (`captchaSVGDataURI`):** Generates an in-memory 5-character SVG challenge with crossing noise lines and color dots as a base64 Data URI—no external CGo/libpng dependencies.
- **JWT & Password Security:** Hashes passwords with `bcrypt`, issues JWT tokens stored in cookies or headers, and tags all data with `_owner`.

---

### G. `cmd/loadtest/main.go` — High-Concurrency Benchmarking
- Spawns **200 concurrent worker goroutines**.
- Dispatches **25,000 requests** with JSON schema validation.
- Uses `sync/atomic` counters to measure throughput and verify zero transaction failures under saturation.

---

## 5. Minute-by-Minute Presentation Script

- **0:00 – 1:30 | Introduction & The Problem**
  > "Good morning. Today I am presenting **Sprig-DB**, an ultra-lightweight, embedded key-value document database with dynamic schema enforcement, SQL querying, and a glassmorphic dashboard built in Go. We set out to solve a core problem: traditional databases like Postgres and Mongo are too heavy for embedded/CLI/desktop tools, while raw embedded key-value stores like BoltDB only store raw bytes with no schemas, indexes, or queries."
- **1:30 – 3:30 | Architecture & Big-Endian Binary Keys**
  > "Sprig-DB is built on top of an ACID B+Tree engine using memory-mapped I/O. A key systems challenge we solved was numerical key sorting in B-Trees. Since B-Trees sort lexicographically, storing IDs as strings causes '10' to appear before '2'. We resolved this by encoding IDs as 8-byte Big-Endian byte slices, ensuring physical disk order strictly matches numerical sequence order."
- **3:30 – 5:00 | Secondary Indexing & Prefix Seeking**
  > "To avoid full collection scans, we built an automated secondary indexing engine. Each index maps composite keys `value:id` into dedicated index buckets. When an equality filter executes, the engine uses bbolt's cursor seek to jump directly to the prefix in $O(\log N)$ time, streaming matching IDs. Sensitive fields like passwords and tokens are excluded automatically."
- **5:00 – 6:45 | Schema Enforcement & Pre-Flight Dry-Run**
  > "A standout novelty of Sprig-DB is our pre-flight schema dry-run. In existing databases, altering a schema on live data can cause data corruption or unexpected downtime. Sprig-DB provides a `/schema/check` endpoint that validates existing records in memory before committing, showing exactly which document IDs fail and why."
- **6:45 – 8:15 | Dual Query Engine & Multi-Tenancy**
  > "Sprig-DB supports both programmatic NoSQL queries and standard SQL queries via our custom SQL parser, supporting projections, boolean WHERE expressions, multi-field ORDER BY, LIMIT, and OFFSET. Everything is multi-tenant by design, partitioned by tenant `_owner` tags, and protected by JWT authentication and an in-memory SVG CAPTCHA generator."
- **8:15 – 10:00 | Concurrency Benchmarks & Live Demo**
  > "We validated durability using a 200-worker concurrent load tester blasting 25,000 schema-validated requests with zero transaction failures. Now, let me demonstrate the live dashboard."

---

## 6. Top Viva / Interview Defense Q&A

**Q1: Why build on bbolt rather than SQLite?**
> *Answer:* SQLite is relational and table-centric; storing polymorphic JSON documents in SQLite requires awkward JSON functions and circumvents relational indexing. BoltDB is a pure B+Tree key-value store, giving us complete control over our bucket hierarchies, document serialization, composite secondary indexes, and dynamic schema validation.

**Q2: bbolt only allows a single writer transaction at a time. Doesn't that hurt throughput?**
> *Answer:* While writes are serialized, transactions commit directly to memory-mapped pages and disk sequentially, handling thousands of writes per second. Additionally, Sprig-DB provides **Atomic Bulk Ingestion** (`InsertMany`), committing up to 100 documents in a *single* transaction, significantly reducing disk sync overhead.

**Q3: How does the secondary index avoid collisions when multiple documents have the same value?**
> *Answer:* We use composite keys formatted as `value:id` (e.g., `engineer:101`, `engineer:102`). Because document IDs are unique, each composite key is unique, allowing documents with identical values to sort contiguously in the B-Tree for fast prefix range scans.

**Q4: How do you prevent floating-point precision loss with large 64-bit integer IDs?**
> *Answer:* Standard Go `json.Unmarshal` parses numbers into `float64`, losing precision for numbers above $2^{53}$. We configure our decoders with `dec.UseNumber()` and use a type-coercion helper (`toFloat64`) that preserves `int64` and `json.Number` fidelity during comparisons.

**Q5: How does the SQL parser handle operator precedence between AND and OR?**
> *Answer:* In `api/sql_query.go`, clauses are split on `OR` first, and each segment is then split on `AND`. This reflects Disjunctive Normal Form (DNF), ensuring that `AND` binds tighter than `OR` during evaluation.
