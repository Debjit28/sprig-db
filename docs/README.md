# Sprig-DB (embedded BaaS) — Docs

Sprig-DB is a **PocketBase-style backend-as-a-service** that runs as a single Go binary and stores data in an embedded **`bbolt`** file (`.sprig`). It gives you collections, schemas, CRUD, auth, and a small admin-style UI — without running Postgres/MySQL/Redis.

## What problem does it solve?

When you want to ship a small product / internal tool / prototype, you usually need:

- **A database** (plus migrations, hosting, backups).
- **An API** (CRUD, filtering, validation).
- **Authentication** and a basic admin UI.
- **Multi-tenant isolation** (each customer sees only their data).

Sprig-DB targets the “I want a backend today” use case by bundling these into a single embedded system.

## How does it solve it?

At a high level:

- **Embedded persistence**: everything is stored in `bbolt` (a single file).
- **Collections + schemas**:
  - Every tenant has their own collection schemas stored in the internal `_schemas` collection.
  - You can define schemas either via the UI (simple) or via JSON Schema (full control).
- **Tenant isolation**:
  - Multi-tenant is implemented with a special `_owner` field on each record.
  - All API reads/writes are scoped by `_owner`, so each authenticated user sees only their own collections and records.
- **Validation**:
  - `POST /api/records/:collection` and `PUT /api/records/:collection` validate the request body against the collection schema.
  - Unknown fields are rejected; required fields and types are enforced.
  - `_owner` is injected by the server and is not accepted from clients.
- **Admin-like UI**:
  - Browse collections, view documents, insert/delete documents, and see request logs.
  - Includes a dashboard “Live Query Console” with SQL-like and NoSQL modes.

## Why would anyone use it?

- **Fastest path to a working backend**: one process, one file, minimal ops.
- **Great for prototypes and demos**: start local, ship the same binary somewhere else.
- **Good for internal tools**: you still get auth + isolation + audit-ish logs.
- **PocketBase-like ergonomics**: collections + a dashboard UI instead of wiring everything by hand.

Where it may not be the right fit (yet):

- Very high write concurrency (bbolt is single-writer).
- Large multi-region deployments.
- Complex transactional workflows across many “collections” (some multi-step flows aren’t wrapped in one cross-bucket transaction yet).

## How can anyone use it?

### Run the server

From the repo root:

```bash
make run
```

The UI is available at `http://localhost:7777/login`.

#### Start from a clean database

By default, `default.sprig` is not deleted on restart. To reset:

```bash
SPRIG_RESET_DATA=true make run
```

### Auth model

- `POST /auth/register` — create a new user
- `POST /auth/login` — username + password, returns JWT and sets `sprig_token` cookie
- The first registered user becomes `is_admin` (admin); others are standard users.
- The frontend uses:
  - a cookie for server-side page protection
  - `localStorage` token for API calls when present

### Create a collection

You have two options:

1. **Dashboard popup** (simple)
   - Visit `/dashboard` → **+ New Collection**
   - Provide a name and a first field (type: `string` / `number` / `bool`)

2. **JSON Schema via HTTP API** (recommended)

```http
PUT /api/collections/:name/schema
Authorization: Bearer <token>
Content-Type: application/json

{
  "json_schema": {
    "type": "object",
    "required": ["field1", "field2"],
    "properties": {
      "field1": { "type": "string" },
      "field2": { "type": "number" },
      "nested": {
        "type": "object",
        "properties": {
          "inner": { "type": "string" }
        }
      },
      "tags": {
        "type": "array",
        "items": { "type": "string" }
      }
    }
  }
}
```

### Insert and browse data

- Insert via UI: open a collection at `/collections/<name>` → **+ Insert Document**
- Insert via API: `POST /api/records/:collection`
- Browse documents: `/collections/<name>` (supports paging from the UI)

## Example: seed an `employee` collection for tenant `zoro`

This repo includes a helper command that creates/updates the `employee` schema and inserts ~25 sample employees for `_owner = "zoro"`.

### Run the populator

```bash
go run ./cmd/populate_employee_zoro
```

Then:

- Register/login as user `zoro` (via UI or `/auth/register`)
- Open `/collections/employee` and page through the seeded documents

> Note: `_owner` is the tenant isolation mechanism. The populator writes `_owner: "zoro"` directly into documents; the `zoro` user can be created via normal auth.

## Logs and settings

- `/dashboard/settings` — tenant-scoped actions (e.g. “Reset My Data”)
- `/dashboard/logs` — recent API request logs for the logged-in tenant (paged in the UI)

## ACID properties (what you get from bbolt)

- `bbolt` provides **ACID** semantics for each write transaction.
- Each `insert/update/delete` is wrapped in a write transaction.
- Some multi-step flows are logically consistent but not yet wrapped in a single cross-bucket transaction.

