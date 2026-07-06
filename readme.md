<div align="center">
  <h1>🌱 Sprig-DB</h1>
  <p><strong>Ultra-lightweight, embedded key-value document database</strong></p>
  <p>Built on top of <b>bbolt</b>, Sprig-DB provides abstraction layers for collections, schemas, and indexing with a zero-configuration persistence model and a built-in REST API.</p>
</div>

---

## ✨ Features

- **Embedded Engine**: Powered by `bbolt`, giving fast and ACID-compliant storage.
- **Collections Pattern**: Group your documents easily without manual bucket management.
- **REST API Out-of-the-box**: Comes with an embedded API server powered by Echo to easily read/write data.

## 🏗️ Architecture

```text
.
├── api/                # Production web/RPC endpoints and routes (Echo Server)
├── cmd/                # Entrypoints for binary builds and CLI execution
├── sprig/              # Core Database Package (Collections, Filters, DB abstraction)
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

### 2. Run Tests
You can run the core package tests using simply:
```bash
make test
```

## 🌐 HTTP API Usage

When running the API server (`make run`), Sprig-DB exposes an HTTP API for easy document storage and querying out of the box.

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