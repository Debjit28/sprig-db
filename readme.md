# Sprig 🌱

Sprig is an ultra-lightweight, embedded key-value document database driver built on top of **bbolt**. It provides simple abstraction layers for collections, map schemas, and automated document indexing with zero-configuration persistence.

---

## 🏗️ Project Architecture

```text
.
├── api/                # Production web/RPC endpoints and routes
├── cmd/                # Entrypoints for binary builds and CLI execution
├── hopper/             # Internal processing engine and core workflows
├── sprig/              # Core Database Package
│   └── sprig.go        # Database interface, collection engine, and transaction logic
├── Makefile            # Build, test, and formatting automation toolchain
├── go.mod              # Package dependency graph
└── README.md           # Documentation



⚡ Quick Start

go get [github.com/Debjit28/sprig-db/sprig](https://github.com/Debjit28/sprig-db/sprig)



