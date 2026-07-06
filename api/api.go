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
	db *sprig.Sprig
}

func NewServer(db *sprig.Sprig) *Server {
	return &Server{
		db: db,
	}
}

// HandlePostInsert handles POST /api/:collname — Insert a document.
func (s *Server) HandlePostInsert(c echo.Context) error {
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

	id, err := s.db.Coll(collname).Insert(data)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, sprig.Map{"error": err.Error()})
	}
	return c.JSON(http.StatusCreated, sprig.Map{"id": id})
}

// HandleGetQuery handles GET /api/:collname — Query documents with filters and pagination.
func (s *Server) HandleGetQuery(c echo.Context) error {
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

	err := s.db.Coll(collname).
		Eq(filterMap.Get(sprig.FilterTypeEQ)).
		Delete()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, sprig.Map{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, sprig.Map{"message": "deleted successfully"})
}

// HandleGetCollections handles GET /api — List all collections.
func (s *Server) HandleGetCollections(c echo.Context) error {
	collections, err := s.db.ListCollections()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, sprig.Map{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, sprig.Map{"collections": collections})
}
