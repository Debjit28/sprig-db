<div align="center">
  <h1>🌱 Sprig-DB</h1>
  <p><strong>Ultra-lightweight, embedded key-value document database</strong></p>
  <p>Built on top of <b>bbolt</b>, Sprig-DB provides abstraction layers for collections, schemas, and indexing with a zero-configuration persistence model and a built-in REST API.</p>
</div>

---

## ✨ Features

- **Embedded Engine**: Powered by `bbolt`, giving fast and ACID-compliant storage.
- **Enterprise Security**: Built-in JWT Authentication to secure endpoints out-of-the-box.
- **Admin Dashboard**: A stunning, responsive Dark-Mode Glassmorphism dashboard powered by **HTMX**. Data tables feature built-in pagination.
- **Collections Pattern**: Group your documents easily without manual bucket management.
- **REST API Out-of-the-box**: Comes with an embedded API server powered by Echo to easily read/write data.
- **High Performance Benchmark Tested**: Battle-tested load scripts demonstrating scale.

## 🏗️ Architecture

```text
.
├── api/                # Production web endpoints, HTMX handlers, and JWT Auth (Echo Server)
├── cmd/                # Entrypoints for binary builds, main server, and CLI loadtester
├── sprig/              # Core Database Package (Collections, Filters, DB abstraction)
├── templates/          # HTML templates powering the HTMX admin dashboard
├── static/             # Static CSS and styling for the dashboard
├── Makefile            # Build, test, and formatting automation toolchain
├── go.mod              # Package dependencies
└── readme.md           # You are here!
```

## 🚀 Quick Start

### 1. Build & Run the API Server
Sprig-DB includes a Makefile to streamline the build and run process.

```bash
# Build the binary
make build

# Build and run the HTTP API Server (runs on port :7777 by default)
make run
```

### 2. Run Tests & Benchmarks
You can run the core package tests and storage endurance benchmarks using simply:
```bash
make test
```

### 3. Run Throughput Load tester
To test API capability and concurrent user scaling, you can run the massive multithreaded HTTP benchmark script against the running server.
```bash
make loadtest
```

## 🌐 HTTP API Usage

When running the API server (`make run`), Sprig-DB exposes an HTTP API for document storage and queries. The dashboard is accessible at **`http://localhost:7777/dashboard`**.

*Note: The API and Dashboard are secured by JWT Authentication. You must first register and login via `POST /auth/register` and `POST /auth/login` to obtain your Bearer token or cookie.*

### `POST /api/:collection_name` - Insert Document

**Usage**:
```bash
curl -X POST http://localhost:7777/api/users \
  -H "Content-Type: application/json" \
  -d '{"username": "johndoe", "email": "john@example.com", "age": 30}'

# Returns: {"id": 1}
```

### `GET /api/:collection_name` - Query Documents

You can filter documents by providing query parameters with the format `?{FilterType}.{field}={value}`. Right now `eq` (equals) filter type is supported.

**Usage**:
```bash
curl "http://localhost:7777/api/users?eq.username=johndoe"

# Returns list of records matching the condition
# [{"id": 1, "username": "johndoe", "email": "john@example.com", "age": 30}]
```

### `demo.sh` - Auto-populate Collections

You can run the included `demo.sh` script to quickly populate the database with dummy data across multiple collections (`users`, `products`, and `orders`).

**Usage**:
```bash
chmod +x demo.sh
./demo.sh
```

## 🛠️ Embedding Sprig Programmatically

You can also bypass the API layer and embed Sprig directly into your own Go applications as a lightweight store.
```go
import "github.com/Debjit28/sprig-db/sprig"

func main() {
    // Initialize DB
    db, err := sprig.New()
    if err != nil {
        panic(err)
    }
    
    // Insert document
    id, _ := db.Coll("users").Insert(sprig.Map{"name": "Alice"})
    
    // Query document
    results, _ := db.Coll("users").Eq(sprig.Map{"name": "Alice"}).Find()
}
```

## ⚠️ Limitations

As this is a lightweight learning project, there are currently a few limitations:
- **Query Capabilities**: Only simple equality (`eq`) filters are supported. No complex nested query operations (`$gt`, `$lt`, OR/AND chaining).
- **Relational Integrity**: As a document NoSQL store, foreign-key relationships must be enforced at the application layer. (See `examples/relational_embedding/main.go` for how to implement relational joins natively).
- **Transactions**: While boltdb provides ACID properties, Sprig's API doesn't currently easily expose multi-document/cross-collection transactional grouping.

## 🙌 Acknowledgments

This project builds upon great learning resources within the Go community:
- The base structure was inspired by [anthdm/hopper](https://github.com/anthdm/hopper).
- Key underlying database concepts were learned from the [Database Internals PDF](https://github.com/arpitn30/EBooks/blob/master/Database%20Internals.pdf).

## 📜 License

This project is licensed under the MIT License.

### Contributions

This repository is primarily a personal/student learning project.

Please don't open pull requests or contribute code. The goal is to build and maintain everything myself as part of the learning process.

Feel free to:
- ⭐ Star the repository if you find it interesting.
- 🐛 Open an issue if you discover a bug.
- 💡 Share suggestions or ideas in the Issues section.