<div align="center">
  <h1>🌱 Sprig-DB</h1>
  <p><strong>An ultra-lightweight, embedded document database driven by bbolt.</strong></p>
  
  <p>
    <a href="https://github.com/Debjit28/sprig-db/actions"><img src="https://img.shields.io/github/actions/workflow/status/Debjit28/sprig-db/ci.yml?branch=main" alt="Build Status"></a>
    <a href="https://golang.org/doc/devel/release.html"><img src="https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go" alt="Go Version"></a>
    <a href="https://github.com/Debjit28/sprig-db/blob/main/LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="License"></a>
  </p>
</div>

---

Sprig-DB is a self-contained, ACID-compliant database designed for Go applications. It gives you the speed and reliability of `bbolt` combined with the ease-of-use of a MongoDB-like document store. It ships with a fully functional REST API, JWT authentication, and a built-in HTMX Admin Dashboard.

## ✨ Features

- **Embedded Engine**: Powered by `bbolt` for lightning-fast, zero-configuration file-based persistence.
- **Collections Pattern**: Easily group documents into distinct collections dynamically.
- **REST API Out-of-the-Box**: Embedded HTTP Server (via Echo) exposing fully validated CRUD endpoints.
- **JWT Authentication**: Full built-in security for your API endpoints.
- **HTMX Admin Panel**: Beautiful Glassmorphism dark-mode UI for managing your database directly from your browser.
- **Secondary Indexing**: Automatically indexes documents for rapid querying.
- **Pagination & Optimization**: Safely handle massive datasets with `Limit` and `Offset`.

## 🚀 Quick Start

### 1. Build & Run the API Server
Sprig-DB includes a `Makefile` to streamline the build and run process.

```bash
# Build the binary
make build

# Start the Sprig-DB Server (defaults to port :7777)
make run
```

### 2. Access the Dashboard
Once the server is running, navigate your browser to `http://localhost:7777`. 

*On your first visit, click **"Create one"** on the login screen to register your initial Admin account.*

### 3. Run Tests
You can run the core package tests using simply:
```bash
make test
```

## 🌐 Interacting with the API

Sprig-DB exposes a fully secured HTTP API. Here is how you can interact with it using `curl`.

### Obtain an Auth Token
```bash
curl -X POST http://localhost:7777/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "yourpassword"}'
```

### Insert a Document
```bash
curl -X POST http://localhost:7777/api/users \
  -H "Authorization: Bearer <YOUR_TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"username": "johndoe", "email": "john@example.com", "age": 30}'

# Returns: {"id": 1}
```

### Query Documents
Filter documents by providing query parameters with the format `?{FilterType}.{field}={value}`. Right now, `eq` (equals) is supported.

```bash
curl -X GET "http://localhost:7777/api/users?eq.username=johndoe" \
  -H "Authorization: Bearer <YOUR_TOKEN>"
```

## 🛠️ Embedding Programmatically

Bypass the API layer entirely and embed Sprig directly into your Go binaries.

```go
import "github.com/Debjit28/sprig-db/sprig"

func main() {
    // 1. Initialize DB
    db, err := sprig.New(sprig.WithDBName("production"))
    if err != nil {
        panic(err)
    }
    defer db.Close()
    
    // 2. Insert document
    id, _ := db.Coll("customers").Insert(sprig.Map{"name": "Alice"})
    
    // 3. Query document
    results, _ := db.Coll("customers").Eq(sprig.Map{"name": "Alice"}).Find()
}
```

## 🏗️ Architecture

```text
.
├── api/                # Production web routines, Echo Server, JWT Middleware
├── cmd/                # Entrypoint for the binary build
├── sprig/              # Core Database Package (Collections, bbolt Engine, Indexes)
├── static/             # CSS Design System
├── templates/          # HTML/HTMX admin frontend files
├── Makefile            # Build, test, and formatting automation toolchain
```

## 🤝 Contributing

Contributions are always welcome! Whether it's reporting a bug, discussing improvements, or submitting a Pull Request, your input helps make Sprig-DB better.

1. Fork the Project
2. Create your Feature Branch (`git checkout -b feature/AmazingFeature`)
3. Commit your Changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the Branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

Please ensure you run `make test` and add tests for any new behavior before submitting a PR!

## 📜 License

Distributed under the MIT License. See `LICENSE` for more information.
