package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Debjit28/sprig-db/sprig"
	"github.com/labstack/echo/v4"
)

// setupTestServer creates a fresh Sprig DB, Server, collection, and schema
// for insert tests. The caller must defer db.DropDatabase(dbName).
func setupTestServer(t *testing.T, dbName string) (*Server, *sprig.Sprig) {
	t.Helper()

	db, err := sprig.New(sprig.WithDBName(dbName))
	if err != nil {
		t.Fatalf("failed to create test DB: %v", err)
	}

	srv := NewServer(db)

	// Create the collection and a simple schema.
	if err := db.CreateCollection("items"); err != nil {
		t.Fatalf("create collection: %v", err)
	}
	schema := CollectionSchema{
		Name: "items",
		Fields: map[string]FieldSchema{
			"name": {Type: "string", Required: true},
		},
	}
	if err := srv.schemas.Upsert("testuser", schema); err != nil {
		t.Fatalf("upsert schema: %v", err)
	}
	return srv, db
}

func makeInsertRequest(t *testing.T, srv *Server, body string) *httptest.ResponseRecorder {
	t.Helper()

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/records/items", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("collname")
	c.SetParamValues("items")
	c.Set("username", "testuser")

	if err := srv.HandlePostInsert(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	return rec
}

func TestHandlePostInsert(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
		// If non-empty, response body must contain this substring.
		wantContains string
		// If set, verify the "id" field is present in the response (single insert).
		wantSingleID bool
		// If > 0, verify "ids" array has this many elements (bulk insert).
		wantBulkCount int
	}{
		{
			name:         "single object",
			body:         `{"name": "Alice"}`,
			wantStatus:   http.StatusCreated,
			wantSingleID: true,
		},
		{
			name:          "valid array of 3",
			body:          `[{"name": "A"}, {"name": "B"}, {"name": "C"}]`,
			wantStatus:    http.StatusCreated,
			wantBulkCount: 3,
		},
		{
			name:         "empty array",
			body:         `[]`,
			wantStatus:   http.StatusBadRequest,
			wantContains: "at least one document",
		},
		{
			name:         "mixed invalid – item 1 is number",
			body:         `[{"name": "ok"}, 42, {"name": "also ok"}]`,
			wantStatus:   http.StatusBadRequest,
			wantContains: "item 1",
		},
		{
			name:         "mixed invalid – item 2 is string",
			body:         `[{"name": "ok"}, {"name": "fine"}, "nope"]`,
			wantStatus:   http.StatusBadRequest,
			wantContains: "item 2",
		},
		{
			name:         "mixed invalid – item 0 is null",
			body:         `[null, {"name": "ok"}]`,
			wantStatus:   http.StatusBadRequest,
			wantContains: "item 0 is not a JSON object",
		},
		{
			name:         "mixed invalid – item 1 is array",
			body:         `[{"name": "ok"}, [1,2,3]]`,
			wantStatus:   http.StatusBadRequest,
			wantContains: "item 1",
		},
		{
			name:         "101 items exceeds limit",
			body:         buildNItems(101),
			wantStatus:   http.StatusBadRequest,
			wantContains: "limited to 100",
		},
		{
			name:          "exactly 100 items is fine",
			body:          buildNItems(100),
			wantStatus:    http.StatusCreated,
			wantBulkCount: 100,
		},
		{
			name:         "invalid JSON – broken object",
			body:         `{"name": }`,
			wantStatus:   http.StatusBadRequest,
			wantContains: "invalid JSON at line",
		},
		{
			name:         "invalid JSON – broken array",
			body:         `[{"name": "ok"}, {bad}]`,
			wantStatus:   http.StatusBadRequest,
			wantContains: "invalid JSON at line",
		},
		{
			name:         "invalid JSON – bare string",
			body:         `"hello"`,
			wantStatus:   http.StatusBadRequest,
			wantContains: "must be a JSON object or array",
		},
		{
			name:         "invalid JSON – bare number",
			body:         `123`,
			wantStatus:   http.StatusBadRequest,
			wantContains: "must be a JSON object or array",
		},
		{
			name:         "empty body",
			body:         ``,
			wantStatus:   http.StatusBadRequest,
			wantContains: "cannot be empty",
		},
		{
			name:         "schema validation error in single object",
			body:         `{"bogus": "field"}`,
			wantStatus:   http.StatusBadRequest,
			wantContains: "required field",
		},
		{
			name:         "schema validation error in array item 1",
			body:         `[{"name": "ok"}, {"bogus": "field"}]`,
			wantStatus:   http.StatusBadRequest,
			wantContains: "item 1",
		},
		{
			name:         "multiline invalid JSON gives line info",
			body:         "{\n  \"name\": \n}",
			wantStatus:   http.StatusBadRequest,
			wantContains: "line 3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, db := setupTestServer(t, "test_insert_"+sanitizeDBName(tt.name))
			defer db.DropDatabase("test_insert_" + sanitizeDBName(tt.name))

			rec := makeInsertRequest(t, srv, tt.body)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status: got %d, want %d; body: %s", rec.Code, tt.wantStatus, rec.Body.String())
			}

			if tt.wantContains != "" {
				if !strings.Contains(rec.Body.String(), tt.wantContains) {
					t.Fatalf("body should contain %q; got: %s", tt.wantContains, rec.Body.String())
				}
			}

			if tt.wantSingleID {
				var resp map[string]any
				if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
					t.Fatalf("unmarshal response: %v", err)
				}
				if _, ok := resp["id"]; !ok {
					t.Fatalf("response should have 'id' key; got: %v", resp)
				}
				// Must NOT have "ids" key.
				if _, ok := resp["ids"]; ok {
					t.Fatalf("single-insert response should not have 'ids' key; got: %v", resp)
				}
			}

			if tt.wantBulkCount > 0 {
				var resp map[string]any
				if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
					t.Fatalf("unmarshal response: %v", err)
				}
				idsRaw, ok := resp["ids"]
				if !ok {
					t.Fatalf("response should have 'ids' key; got: %v", resp)
				}
				ids, ok := idsRaw.([]any)
				if !ok {
					t.Fatalf("'ids' should be an array; got: %T", idsRaw)
				}
				if len(ids) != tt.wantBulkCount {
					t.Fatalf("expected %d ids, got %d", tt.wantBulkCount, len(ids))
				}
				// Must NOT have "id" key.
				if _, ok := resp["id"]; ok {
					t.Fatalf("bulk-insert response should not have 'id' key; got: %v", resp)
				}
			}
		})
	}
}

// buildNItems returns a JSON array string with n items, each being {"name": "itemN"}.
func buildNItems(n int) string {
	items := make([]string, n)
	for i := 0; i < n; i++ {
		items[i] = fmt.Sprintf(`{"name": "item%d"}`, i)
	}
	return "[" + strings.Join(items, ",") + "]"
}

// sanitizeDBName makes a test name safe for use as a bbolt filename.
func sanitizeDBName(name string) string {
	r := strings.NewReplacer(" ", "_", "/", "_", "–", "_", "—", "_")
	return r.Replace(strings.ToLower(name))
}
