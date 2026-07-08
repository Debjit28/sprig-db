package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/Debjit28/sprig-db/sprig"
	"github.com/labstack/echo/v4"
)

type Server struct {
	db      *sprig.Sprig
	schemas *SchemaStore
}

func NewServer(db *sprig.Sprig) *Server {
	return &Server{
		db:      db,
		schemas: NewSchemaStore(db),
	}
}

func currentUsername(c echo.Context) (string, error) {
	username, ok := c.Get("username").(string)
	if !ok || strings.TrimSpace(username) == "" {
		return "", fmt.Errorf("unauthorized")
	}
	return username, nil
}

type collectionCreateRequest struct {
	Name string `json:"name"`

	// Preferred (UI) format.
	Fields map[string]FieldSchema `json:"fields,omitempty"`

	// Optional JSON-schema format (subset of draft-04) for easier authoring.
	JSONSchema map[string]any `json:"json_schema,omitempty"`
}

// HandleCreateCollection handles POST /api/collections.
func (s *Server) HandleCreateCollection(c echo.Context) error {
	username, err := currentUsername(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, sprig.Map{"error": err.Error()})
	}

	var req collectionCreateRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, sprig.Map{"error": "invalid request body"})
	}

	var schema *CollectionSchema
	if req.JSONSchema != nil {
		converted, err := ConvertJSONSchemaToCollectionSchema(req.Name, req.JSONSchema)
		if err != nil {
			return c.JSON(http.StatusBadRequest, sprig.Map{"error": err.Error()})
		}
		schema = converted
	} else {
		schema = &CollectionSchema{
			Name:   req.Name,
			Fields: req.Fields,
		}
	}
	if err := schema.Validate(); err != nil {
		return c.JSON(http.StatusBadRequest, sprig.Map{"error": err.Error()})
	}

	if err := s.db.CreateCollection(schema.Name); err != nil {
		return c.JSON(http.StatusInternalServerError, sprig.Map{"error": err.Error()})
	}
	if err := s.schemas.Upsert(username, *schema); err != nil {
		return c.JSON(http.StatusBadRequest, sprig.Map{"error": err.Error()})
	}

	return c.JSON(http.StatusCreated, sprig.Map{
		"message":    "collection created",
		"collection": schema.Name,
	})
}

// HandleGetCollectionSchema handles GET /api/collections/:collname/schema.
func (s *Server) HandleGetCollectionSchema(c echo.Context) error {
	username, err := currentUsername(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, sprig.Map{"error": err.Error()})
	}
	collname := c.Param("collname")
	schema, err := s.schemas.Get(username, collname)
	if err != nil {
		return c.JSON(http.StatusNotFound, sprig.Map{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, schema)
}

// HandlePutCollectionSchema handles PUT /api/collections/:collname/schema.
func (s *Server) HandlePutCollectionSchema(c echo.Context) error {
	username, err := currentUsername(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, sprig.Map{"error": err.Error()})
	}
	collname := c.Param("collname")
	var req collectionCreateRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, sprig.Map{"error": "invalid request body"})
	}

	var schema *CollectionSchema
	if req.JSONSchema != nil {
		converted, err := ConvertJSONSchemaToCollectionSchema(collname, req.JSONSchema)
		if err != nil {
			return c.JSON(http.StatusBadRequest, sprig.Map{"error": err.Error()})
		}
		schema = converted
	} else {
		schema = &CollectionSchema{
			Name:   collname,
			Fields: req.Fields,
		}
	}
	if err := schema.Validate(); err != nil {
		return c.JSON(http.StatusBadRequest, sprig.Map{"error": err.Error()})
	}
	if err := s.schemas.Upsert(username, *schema); err != nil {
		return c.JSON(http.StatusBadRequest, sprig.Map{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, sprig.Map{"message": "schema updated", "collection": collname})
}

// HandleDeleteCollection handles DELETE /api/collections/:collname.
func (s *Server) HandleDeleteCollection(c echo.Context) error {
	username, err := currentUsername(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, sprig.Map{"error": err.Error()})
	}
	collname := c.Param("collname")
	if collname == "" {
		return c.JSON(http.StatusBadRequest, sprig.Map{"error": "collection name is required"})
	}
	if strings.HasPrefix(collname, "_") {
		return c.JSON(http.StatusBadRequest, sprig.Map{"error": "cannot delete reserved collections"})
	}

	// Multi-tenant delete: remove only this user's records, not global bucket.
	_ = s.db.Coll(collname).Eq(sprig.Map{"_owner": username}).Delete()
	_ = s.db.Coll(schemaCollection).Eq(sprig.Map{"name": collname, "owner": username}).Delete()

	return c.JSON(http.StatusOK, sprig.Map{"message": "collection deleted", "collection": collname})
}

// HandlePostInsert handles POST /api/:collname — Insert a document.
func (s *Server) HandlePostInsert(c echo.Context) error {
	username, err := currentUsername(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, sprig.Map{"error": err.Error()})
	}
	collname := c.Param("collname")
	if collname == "" {
		return c.JSON(http.StatusBadRequest, sprig.Map{"error": "collection name is required"})
	}

	var data sprig.Map
	if err := json.NewDecoder(c.Request().Body).Decode(&data); err != nil {
		return c.JSON(http.StatusBadRequest, sprig.Map{"error": "invalid JSON body"})
	}
	if len(data) == 0 {
		return c.JSON(http.StatusBadRequest, sprig.Map{"error": "request body cannot be empty"})
	}
	delete(data, "id")

	schema, err := s.schemas.Get(username, collname)
	if err != nil {
		return c.JSON(http.StatusBadRequest, sprig.Map{"error": "schema does not exist for collection"})
	}
	if err := ValidateDocument(data, schema, false); err != nil {
		return c.JSON(http.StatusBadRequest, sprig.Map{"error": err.Error()})
	}

	data["_owner"] = username
	id, err := s.db.Coll(collname).Insert(data)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, sprig.Map{"error": err.Error()})
	}
	return c.JSON(http.StatusCreated, sprig.Map{"id": id})
}

// HandleGetQuery handles GET /api/:collname — Query documents with filters and pagination.
func (s *Server) HandleGetQuery(c echo.Context) error {
	username, err := currentUsername(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, sprig.Map{"error": err.Error()})
	}
	collname := c.Param("collname")
	if collname == "" {
		return c.JSON(http.StatusBadRequest, sprig.Map{"error": "collection name is required"})
	}

	filterMap := NewFilterMap()

	// Parse limit and offset.
	limit := 0
	offset := 0
	if l := c.QueryParam("limit"); l != "" {
		val, err := strconv.Atoi(l)
		if err != nil {
			return c.JSON(http.StatusBadRequest, sprig.Map{"error": "limit must be an integer"})
		}
		limit = val
	}
	if o := c.QueryParam("offset"); o != "" {
		val, err := strconv.Atoi(o)
		if err != nil {
			return c.JSON(http.StatusBadRequest, sprig.Map{"error": "offset must be an integer"})
		}
		offset = val
	}

	for k, v := range c.QueryParams() {
		// Skip pagination params.
		if k == "limit" || k == "offset" {
			continue
		}
		filterParts := strings.Split(k, ".")
		if len(filterParts) != 2 {
			return c.JSON(http.StatusBadRequest, sprig.Map{"error": fmt.Sprintf("malformed query key: %s", k)})
		}
		if len(v) == 0 || v[0] == "" {
			return c.JSON(http.StatusBadRequest, sprig.Map{"error": fmt.Sprintf("empty value for query key: %s", k)})
		}
		filterMap.Add(filterParts[0], filterParts[1], v[0])
	}
	filterMap.Add(sprig.FilterTypeEQ, "_owner", username)

	query := s.db.Coll(collname).Eq(filterMap.Get(sprig.FilterTypeEQ))
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	result, err := query.Find()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, sprig.Map{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, result)
}

// HandlePutUpdate handles PUT /api/:collname — Update matching documents.
func (s *Server) HandlePutUpdate(c echo.Context) error {
	username, err := currentUsername(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, sprig.Map{"error": err.Error()})
	}
	collname := c.Param("collname")
	if collname == "" {
		return c.JSON(http.StatusBadRequest, sprig.Map{"error": "collection name is required"})
	}

	var data sprig.Map
	if err := json.NewDecoder(c.Request().Body).Decode(&data); err != nil {
		return c.JSON(http.StatusBadRequest, sprig.Map{"error": "invalid JSON body"})
	}
	if len(data) == 0 {
		return c.JSON(http.StatusBadRequest, sprig.Map{"error": "update body cannot be empty"})
	}
	delete(data, "id")
	delete(data, "_owner")

	schema, err := s.schemas.Get(username, collname)
	if err != nil {
		return c.JSON(http.StatusBadRequest, sprig.Map{"error": "schema does not exist for collection"})
	}
	if err := ValidateDocument(data, schema, true); err != nil {
		return c.JSON(http.StatusBadRequest, sprig.Map{"error": err.Error()})
	}

	filterMap := NewFilterMap()
	for k, v := range c.QueryParams() {
		filterParts := strings.Split(k, ".")
		if len(filterParts) != 2 {
			return c.JSON(http.StatusBadRequest, sprig.Map{"error": fmt.Sprintf("malformed query key: %s", k)})
		}
		if len(v) == 0 || v[0] == "" {
			return c.JSON(http.StatusBadRequest, sprig.Map{"error": fmt.Sprintf("empty value for query key: %s", k)})
		}
		filterMap.Add(filterParts[0], filterParts[1], v[0])
	}
	filterMap.Add(sprig.FilterTypeEQ, "_owner", username)

	updated, err := s.db.Coll(collname).
		Eq(filterMap.Get(sprig.FilterTypeEQ)).
		Update(data)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, sprig.Map{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, sprig.Map{"updated": len(updated), "data": updated})
}

// HandleDelete handles DELETE /api/:collname — Delete matching documents.
func (s *Server) HandleDelete(c echo.Context) error {
	username, err := currentUsername(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, sprig.Map{"error": err.Error()})
	}
	collname := c.Param("collname")
	if collname == "" {
		return c.JSON(http.StatusBadRequest, sprig.Map{"error": "collection name is required"})
	}

	filterMap := NewFilterMap()
	for k, v := range c.QueryParams() {
		filterParts := strings.Split(k, ".")
		if len(filterParts) != 2 {
			return c.JSON(http.StatusBadRequest, sprig.Map{"error": fmt.Sprintf("malformed query key: %s", k)})
		}
		if len(v) == 0 || v[0] == "" {
			return c.JSON(http.StatusBadRequest, sprig.Map{"error": fmt.Sprintf("empty value for query key: %s", k)})
		}
		filterMap.Add(filterParts[0], filterParts[1], v[0])
	}
	filterMap.Add(sprig.FilterTypeEQ, "_owner", username)

	err = s.db.Coll(collname).
		Eq(filterMap.Get(sprig.FilterTypeEQ)).
		Delete()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, sprig.Map{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, sprig.Map{"message": "deleted successfully"})
}

// HandleGetCollections handles GET /api — List all collections.
func (s *Server) HandleGetCollections(c echo.Context) error {
	username, err := currentUsername(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, sprig.Map{"error": err.Error()})
	}

	collections, err := s.schemas.ListByOwner(username)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, sprig.Map{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, sprig.Map{"collections": collections})
}

// HandleResetMyData deletes all tenant-scoped data for the logged-in user.
func (s *Server) HandleResetMyData(c echo.Context) error {
	username, err := currentUsername(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, sprig.Map{"error": err.Error()})
	}

	collections, err := s.schemas.ListByOwner(username)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, sprig.Map{"error": err.Error()})
	}

	// Best-effort cleanup of tenant data.
	for _, coll := range collections {
		_ = s.db.Coll(coll).Eq(sprig.Map{"_owner": username}).Delete()
		_ = s.db.Coll(schemaCollection).Eq(sprig.Map{"name": coll, "owner": username}).Delete()
	}
	_ = s.db.Coll("_logs").Eq(sprig.Map{"_owner": username}).Delete()
	_ = s.db.Coll("_settings").Eq(sprig.Map{"_owner": username}).Delete()

	return c.JSON(http.StatusOK, sprig.Map{
		"message":    "tenant data reset successful",
		"collections": len(collections),
	})
}
