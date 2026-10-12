package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
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

func TestSchemaValidation_ReproduceSingleFieldRejection(t *testing.T) {
	crewData, err := os.ReadFile("../testdata/crew.json")
	if err != nil {
		t.Fatalf("failed to read testdata/crew.json: %v", err)
	}

	tests := []struct {
		name         string
		schema       CollectionSchema
		body         string
		wantStatus   int
		wantContains string
	}{
		{
			name: "single field schema with reject rejects crew.json array due to unknown fields",
			schema: CollectionSchema{
				Name: "items",
				Fields: map[string]FieldSchema{
					"name": {Type: "string", Required: true},
				},
				AdditionalFields: "reject",
			},
			body:         string(crewData),
			wantStatus:   http.StatusBadRequest,
			wantContains: `is not defined in schema`,
		},
		{
			name: "single field schema with allow accepts crew.json array",
			schema: CollectionSchema{
				Name: "items",
				Fields: map[string]FieldSchema{
					"name": {Type: "string", Required: true},
				},
				AdditionalFields: "allow",
			},
			body:       string(crewData),
			wantStatus: http.StatusCreated,
		},
		{
			name: "existing stored schema with empty additionalFields treated as allow",
			schema: CollectionSchema{
				Name: "items",
				Fields: map[string]FieldSchema{
					"name": {Type: "string", Required: true},
				},
			},
			body:       string(crewData),
			wantStatus: http.StatusCreated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dbName := "test_reproduce_" + sanitizeDBName(tt.name)
			db, err := sprig.New(sprig.WithDBName(dbName))
			if err != nil {
				t.Fatalf("failed to create db: %v", err)
			}
			defer db.DropDatabase(dbName)

			srv := NewServer(db)
			if err := db.CreateCollection(tt.schema.Name); err != nil {
				t.Fatalf("create collection: %v", err)
			}
			if err := srv.schemas.Upsert("testuser", tt.schema); err != nil {
				t.Fatalf("upsert schema: %v", err)
			}

			rec := makeInsertRequest(t, srv, tt.body)
			if rec.Code != tt.wantStatus {
				t.Fatalf("got status %d, want %d; body: %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.wantContains != "" && !strings.Contains(rec.Body.String(), tt.wantContains) {
				t.Fatalf("expected response to contain %q; got: %s", tt.wantContains, rec.Body.String())
			}
		})
	}
}

func TestInt64SafeNumbers(t *testing.T) {
	dbName := "test_int64_numbers"
	db, err := sprig.New(sprig.WithDBName(dbName))
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	defer db.DropDatabase(dbName)

	srv := NewServer(db)
	minZero := 0.0
	schema := CollectionSchema{
		Name: "items",
		Fields: map[string]FieldSchema{
			"name":   {Type: "string", Required: true},
			"bounty": {Type: "number", Min: &minZero},
		},
		AdditionalFields: "allow",
	}
	if err := db.CreateCollection("items"); err != nil {
		t.Fatalf("create coll: %v", err)
	}
	if err := srv.schemas.Upsert("testuser", schema); err != nil {
		t.Fatalf("upsert schema: %v", err)
	}

	t.Run("insert and retrieve large bounty 5564800000 without truncation", func(t *testing.T) {
		body := `{"name": "Gol D. Roger", "bounty": 5564800000}`
		rec := makeInsertRequest(t, srv, body)
		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
		}

		result, err := db.Coll("items").Eq(sprig.Map{"name": "Gol D. Roger"}).Find()
		if err != nil {
			t.Fatalf("find failed: %v", err)
		}
		if len(result.Data) == 0 {
			t.Fatal("expected 1 document")
		}

		bountyVal := result.Data[0]["bounty"]
		bf, ok := toFloat(bountyVal)
		if !ok {
			t.Fatalf("expected numeric bounty, got %T: %v", bountyVal, bountyVal)
		}
		const expectedBounty = 5564800000.0
		if bf != expectedBounty {
			t.Fatalf("expected bounty %v, got %v", expectedBounty, bf)
		}
		// Confirm it was NOT truncated to int32 (1269832704)
		if int64(bf) != 5564800000 {
			t.Fatalf("bounty truncated: got int64 %d, want 5564800000", int64(bf))
		}
	})

	t.Run("validate with json.Number without truncation", func(t *testing.T) {
		doc := sprig.Map{
			"name":   "Gol D. Roger",
			"bounty": json.Number("5564800000"),
		}
		if err := ValidateDocument(doc, &schema, false); err != nil {
			t.Fatalf("validation failed for json.Number: %v", err)
		}

		// Min check above bounty
		minHigh := 6000000000.0
		schemaHigh := CollectionSchema{
			Name: "items",
			Fields: map[string]FieldSchema{
				"bounty": {Type: "number", Min: &minHigh},
			},
			AdditionalFields: "allow",
		}
		if err := ValidateDocument(doc, &schemaHigh, false); err == nil {
			t.Fatal("expected min violation for 5564800000 < 6000000000")
		}

		// Max check below bounty
		maxLow := 5000000000.0
		schemaLow := CollectionSchema{
			Name: "items",
			Fields: map[string]FieldSchema{
				"bounty": {Type: "number", Max: &maxLow},
			},
			AdditionalFields: "allow",
		}
		if err := ValidateDocument(doc, &schemaLow, false); err == nil {
			t.Fatal("expected max violation for 5564800000 > 5000000000")
		}
	})
}

func TestEditSchema_Versioning_CheckExistingDocs(t *testing.T) {
	dbName := "test_edit_schema_versioning"
	db, err := sprig.New(sprig.WithDBName(dbName))
	if err != nil {
		t.Fatalf("create db: %v", err)
	}
	defer db.DropDatabase(dbName)

	srv := NewServer(db)
	e := echo.New()
	collName := "straw_hat_pirates"

	// 1. Create collection with name-only schema (v1)
	if err := db.CreateCollection(collName); err != nil {
		t.Fatalf("create coll: %v", err)
	}
	initialSchema := CollectionSchema{
		Name: collName,
		Fields: map[string]FieldSchema{
			"name": {Type: "string", Required: true},
		},
		AdditionalFields: "allow",
	}
	if err := srv.schemas.Upsert("testuser", initialSchema); err != nil {
		t.Fatalf("upsert initial schema: %v", err)
	}

	gotSchema, err := srv.schemas.Get("testuser", collName)
	if err != nil || gotSchema.Version != 1 {
		t.Fatalf("expected version 1, got err: %v, version: %v", err, gotSchema.Version)
	}

	// 2. Insert existing document
	doc := sprig.Map{
		"name":   "Monkey D. Luffy",
		"bounty": 3000000000.0,
		"_owner": "testuser",
	}
	docID, err := db.Coll(collName).Insert(doc)
	if err != nil {
		t.Fatalf("insert doc: %v", err)
	}

	// 3. Test Check Existing Documents against a schema requiring "affiliation"
	checkPayload := `{
		"fields": {
			"name": {"type": "string", "required": true},
			"affiliation": {"type": "string", "required": true}
		},
		"additionalFields": "allow"
	}`
	reqCheck := httptest.NewRequest(http.MethodPost, "/api/collections/"+collName+"/schema/check", strings.NewReader(checkPayload))
	reqCheck.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	recCheck := httptest.NewRecorder()
	cCheck := e.NewContext(reqCheck, recCheck)
	cCheck.SetParamNames("collname")
	cCheck.SetParamValues(collName)
	cCheck.Set("username", "testuser")

	if err := srv.HandleCheckCollectionSchema(cCheck); err != nil {
		t.Fatalf("check handler failed: %v", err)
	}
	if recCheck.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recCheck.Code, recCheck.Body.String())
	}

	var checkResp map[string]any
	if err := json.Unmarshal(recCheck.Body.Bytes(), &checkResp); err != nil {
		t.Fatalf("unmarshal check resp: %v", err)
	}
	if checkResp["invalid"] != float64(1) {
		t.Fatalf("expected invalid=1, got: %v", checkResp["invalid"])
	}

	// Verify the existing document was NOT altered or deleted by the check
	found, err := db.Coll(collName).Eq(sprig.Map{"id": docID}).Find()
	if err != nil || len(found.Data) == 0 {
		t.Fatalf("existing document missing after check: %v", err)
	}
	if found.Data[0]["name"] != "Monkey D. Luffy" {
		t.Fatalf("existing document corrupted: %v", found.Data[0])
	}

	// 4. Update the schema via PUT (v2) adding bounty
	putPayload := `{
		"fields": {
			"name": {"type": "string", "required": true},
			"bounty": {"type": "number", "min": 0}
		},
		"additionalFields": "allow"
	}`
	reqPut := httptest.NewRequest(http.MethodPut, "/api/collections/"+collName+"/schema", strings.NewReader(putPayload))
	reqPut.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	recPut := httptest.NewRecorder()
	cPut := e.NewContext(reqPut, recPut)
	cPut.SetParamNames("collname")
	cPut.SetParamValues(collName)
	cPut.Set("username", "testuser")

	if err := srv.HandlePutCollectionSchema(cPut); err != nil {
		t.Fatalf("put schema failed: %v", err)
	}
	if recPut.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recPut.Code, recPut.Body.String())
	}

	// Check updated schema version is 2
	v2Schema, err := srv.schemas.Get("testuser", collName)
	if err != nil || v2Schema.Version != 2 {
		t.Fatalf("expected version 2, got err: %v, version: %v", err, v2Schema.Version)
	}

	// Verify existing document was NOT rewritten or deleted after schema edit
	afterPutFound, err := db.Coll(collName).Eq(sprig.Map{"id": docID}).Find()
	if err != nil || len(afterPutFound.Data) == 0 {
		t.Fatalf("existing document missing after schema edit: %v", err)
	}
	if afterPutFound.Data[0]["name"] != "Monkey D. Luffy" || afterPutFound.Data[0]["bounty"] != 3000000000.0 {
		t.Fatalf("existing document changed after schema edit: %v", afterPutFound.Data[0])
	}

	// 5. Check existing docs with the updated schema -> should now report 0 invalid
	recCheck2 := httptest.NewRecorder()
	reqCheck2 := httptest.NewRequest(http.MethodPost, "/api/collections/"+collName+"/schema/check", nil)
	cCheck2 := e.NewContext(reqCheck2, recCheck2)
	cCheck2.SetParamNames("collname")
	cCheck2.SetParamValues(collName)
	cCheck2.Set("username", "testuser")

	if err := srv.HandleCheckCollectionSchema(cCheck2); err != nil {
		t.Fatalf("check handler failed: %v", err)
	}
	var checkResp2 map[string]any
	if err := json.Unmarshal(recCheck2.Body.Bytes(), &checkResp2); err != nil {
		t.Fatalf("unmarshal check resp 2: %v", err)
	}
	if checkResp2["invalid"] != float64(0) {
		t.Fatalf("expected invalid=0, got: %v", checkResp2["invalid"])
	}
}



