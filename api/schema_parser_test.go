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

func TestSchemaParser_ValidSchemas(t *testing.T) {
	tests := []struct {
		name       string
		schemaJSON string
		wantFields int
		wantAdd    string
	}{
		{
			name: "full schema with all allowed types and rules",
			schemaJSON: `{
				"fields": {
					"name": {"type": "string", "required": true, "minLength": 1, "maxLength": 100},
					"role": {"type": "string", "enum": ["Captain", "Swordsman"]},
					"bounty": {"type": "number", "min": 0, "max": 10000000000},
					"active": {"type": "boolean", "default": true},
					"tags": {"type": "array", "items": {"type": "string"}},
					"stats": {
						"type": "object",
						"properties": {
							"hp": {"type": "number", "min": 0}
						}
					}
				},
				"additionalFields": "reject"
			}`,
			wantFields: 6,
			wantAdd:    "reject",
		},
		{
			name: "no schema mode with empty fields",
			schemaJSON: `{
				"fields": {},
				"additionalFields": "allow"
			}`,
			wantFields: 0,
			wantAdd:    "allow",
		},
		{
			name: "default additionalFields is allow",
			schemaJSON: `{
				"fields": {
					"name": {"type": "string"}
				}
			}`,
			wantFields: 1,
			wantAdd:    "allow",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, errs := ParseAndValidateSchema("test_coll", []byte(tt.schemaJSON))
			if len(errs) > 0 {
				t.Fatalf("unexpected validation errors: %v", errs)
			}
			if len(schema.Fields) != tt.wantFields {
				t.Errorf("got %d fields, want %d", len(schema.Fields), tt.wantFields)
			}
			if schema.AdditionalFields != tt.wantAdd {
				t.Errorf("got additionalFields %q, want %q", schema.AdditionalFields, tt.wantAdd)
			}
		})
	}
}

func TestSchemaParser_AllErrorsReturnedAtOnce(t *testing.T) {
	badJSON := `{
		"fields": {
			"_id": {"type": "string"},
			"_owner": {"type": "string"},
			"bounty": {"type": "number", "min": "not_a_number"},
			"title": {"type": "invalid_type", "pattern": ".*"},
			"level": {"type": "number", "min": 10, "max": 5}
		},
		"additionalFields": "invalid_choice",
		"unknownTopRule": true
	}`

	schema, errs := ParseAndValidateSchema("bad_coll", []byte(badJSON))
	if schema != nil {
		t.Fatal("expected schema to be nil on errors")
	}
	if len(errs) < 5 {
		t.Fatalf("expected multiple errors returned at once, got %d: %v", len(errs), errs)
	}

	joined := strings.Join(errs, "\n")

	expectedSubstrings := []string{
		"fields.bounty.min must be a number",
		"fields._id: names starting with '_' are reserved",
		"fields._owner: names starting with '_' are reserved",
		`fields.title.type has unsupported type "invalid_type"`,
		"fields.title.pattern is an unsupported rule",
		"fields.level: min cannot be greater than max",
		`additionalFields must be either "allow" or "reject"`,
		"unknownTopRule is an unsupported rule",
	}

	for _, sub := range expectedSubstrings {
		if !strings.Contains(joined, sub) {
			t.Errorf("expected error containing %q, got errors:\n%s", sub, joined)
		}
	}
}

func TestSchemaParser_Limits(t *testing.T) {
	t.Run("exceeds 64 KB", func(t *testing.T) {
		large := map[string]any{
			"fields": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": strings.Repeat("A", 70000),
				},
			},
		}
		b, _ := json.Marshal(large)
		_, errs := ParseAndValidateSchema("oversized", b)
		if len(errs) == 0 || !strings.Contains(strings.Join(errs, "; "), "64 KB") {
			t.Fatalf("expected 64 KB limit error, got: %v", errs)
		}
	})

	t.Run("exceeds 100 fields", func(t *testing.T) {
		fields := map[string]any{}
		for i := 0; i < 105; i++ {
			fields[fmt.Sprintf("field%d", i)] = map[string]any{"type": "string"}
		}
		raw := map[string]any{"fields": fields}
		b, _ := json.Marshal(raw)
		_, errs := ParseAndValidateSchema("too_many_fields", b)
		if len(errs) == 0 || !strings.Contains(strings.Join(errs, "; "), "100 fields") {
			t.Fatalf("expected 100 fields limit error, got: %v", errs)
		}
	})

	t.Run("exceeds nesting depth 5", func(t *testing.T) {
		// Depth 6: level1 (1) -> level2 (2) -> level3 (3) -> level4 (4) -> level5 (5) -> level6 (6)
		deep := map[string]any{
			"fields": map[string]any{
				"l1": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"l2": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"l3": map[string]any{
									"type": "object",
									"properties": map[string]any{
										"l4": map[string]any{
											"type": "object",
											"properties": map[string]any{
												"l5": map[string]any{
													"type": "object",
													"properties": map[string]any{
														"l6": map[string]any{
															"type": "string",
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}
		b, _ := json.Marshal(deep)
		_, errs := ParseAndValidateSchema("too_deep", b)
		if len(errs) == 0 || !strings.Contains(strings.Join(errs, "; "), "nesting depth exceeds maximum limit of 5") {
			t.Fatalf("expected nesting depth error, got: %v", errs)
		}
	})
}

func TestHandleCreateCollection_ValidationErrors(t *testing.T) {
	dbName := "test_create_coll_validation"
	db, err := sprig.New(sprig.WithDBName(dbName))
	if err != nil {
		t.Fatalf("db create: %v", err)
	}
	defer db.DropDatabase(dbName)

	srv := NewServer(db)
	e := echo.New()

	body := `{
		"name": "pirates",
		"fields": {
			"_secret": {"type": "string"},
			"bounty": {"type": "number", "min": "invalid"}
		}
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/collections", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("username", "testuser")

	if err := srv.HandleCreateCollection(c); err != nil {
		t.Fatalf("handler err: %v", err)
	}

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal err: %v", err)
	}

	errsRaw, ok := resp["errors"].([]any)
	if !ok || len(errsRaw) < 2 {
		t.Fatalf("expected multiple errors array in response, got: %v", resp)
	}
}
