package api

import (
	"html/template"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/Debjit28/sprig-db/sprig"
	"github.com/labstack/echo/v4"
)

// TemplateRenderer wraps Go html/template for Echo.
type TemplateRenderer struct {
	templates *template.Template
}

func NewTemplateRenderer(glob string) *TemplateRenderer {
	return &TemplateRenderer{
		templates: template.Must(template.ParseGlob(glob)),
	}
}

func (t *TemplateRenderer) Render(w io.Writer, name string, data any, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

// WebHandler serves HTMX-powered HTML pages.
type WebHandler struct {
	db *sprig.Sprig
}

func NewWebHandler(db *sprig.Sprig) *WebHandler {
	return &WebHandler{db: db}
}

// HandleLoginPage renders the login/register page.
func (w *WebHandler) HandleLoginPage(c echo.Context) error {
	return c.Render(http.StatusOK, "login.html", nil)
}

// HandleDashboard renders the main dashboard.
func (w *WebHandler) HandleDashboard(c echo.Context) error {
	collections, err := w.db.ListCollections()
	if err != nil {
		collections = []string{}
	}

	type collInfo struct {
		Name  string
		Count int
	}
	var colls []collInfo
	for _, name := range collections {
		result, err := w.db.Coll(name).Find()
		count := 0
		if err == nil {
			count = result.Total
		}
		colls = append(colls, collInfo{Name: name, Count: count})
	}

	data := map[string]any{
		"Collections":    colls,
		"CollectionCount": len(colls),
	}
	return c.Render(http.StatusOK, "dashboard.html", data)
}

// HandleCollectionPage renders a specific collection's documents.
func (w *WebHandler) HandleCollectionPage(c echo.Context) error {
	name := c.Param("name")
	
	page := 1
	if p, err := strconv.Atoi(c.QueryParam("page")); err == nil && p > 0 {
		page = p
	}
	limit := 50
	offset := (page - 1) * limit

	result, err := w.db.Coll(name).Offset(offset).Limit(limit).Find()
	if err != nil {
		result = &sprig.QueryResult{Data: []sprig.Map{}, Total: 0}
	}

	// Collect all unique keys across documents for table headers.
	keySet := map[string]bool{}
	for _, doc := range result.Data {
		for k := range doc {
			keySet[k] = true
		}
	}
	var keys []string
	for k := range keySet {
		keys = append(keys, k)
	}

	hasNext := result.Total > offset+limit
	hasPrev := page > 1

	data := map[string]any{
		"Name":    name,
		"Result":  result,
		"Keys":    keys,
		"Page":    page,
		"HasNext": hasNext,
		"HasPrev": hasPrev,
		"NextPage": page + 1,
		"PrevPage": page - 1,
	}
	return c.Render(http.StatusOK, "collection.html", data)
}

// HandleDashboardQuery handles queries from the Dashboard Console
func (w *WebHandler) HandleDashboardQuery(c echo.Context) error {
	lang := c.FormValue("language")
	var result *sprig.QueryResult
	var err error
	var name string

	if lang == "sql" {
		q := strings.TrimSpace(c.FormValue("sql_query"))
		upper := strings.ToUpper(q)
		const prefix = "SELECT * FROM "
		if !strings.HasPrefix(upper, prefix) {
			return c.Render(http.StatusOK, "query_result.html", map[string]any{"Error": "Only 'SELECT * FROM <table> [WHERE <k> = <v>]' is supported in this proxy demo."})
		}
		if len(q) <= len(prefix) {
			return c.Render(http.StatusOK, "query_result.html", map[string]any{"Error": "Missing table name after SELECT * FROM."})
		}
		q = q[len(prefix):] // Strip prefix using length, preserving original case for table name
		parts := strings.SplitN(strings.ToUpper(q), "WHERE", 2)
		name = strings.TrimSpace(q[:len(parts[0])]) // Use original case for table name

		query := w.db.Coll(name)
		if len(parts) > 1 {
			// Extract WHERE clause from original query (preserving case)
			whereClause := strings.TrimSpace(q[len(parts[0])+5:]) // +5 for "WHERE"
			conds := strings.SplitN(whereClause, "=", 2)
			if len(conds) == 2 {
				k := strings.TrimSpace(conds[0])
				v := strings.TrimSpace(conds[1])
				v = strings.Trim(v, "'\" ")
				query = query.Eq(sprig.Map{k: v})
			}
		}
		result, err = query.Limit(50).Find()
	} else {
		// NoSQL Mode
		name = strings.TrimSpace(c.FormValue("collection"))
		if name == "" {
			return c.Render(http.StatusOK, "query_result.html", map[string]any{"Error": "Collection name is required in NoSQL mode."})
		}
		k := strings.TrimSpace(c.FormValue("filter_key"))
		v := strings.TrimSpace(c.FormValue("filter_val"))
		
		query := w.db.Coll(name)
		if k != "" && v != "" {
			query = query.Eq(sprig.Map{k: v})
		}
		result, err = query.Limit(50).Find()
	}

	if err != nil {
		return c.Render(http.StatusOK, "query_result.html", map[string]any{"Error": err.Error()})
	}
	if result == nil {
		result = &sprig.QueryResult{Data: []sprig.Map{}, Total: 0}
	}

	keySet := map[string]bool{}
	for _, doc := range result.Data {
		for key := range doc {
			keySet[key] = true
		}
	}
	var keys []string
	for key := range keySet {
		keys = append(keys, key)
	}

	data := map[string]any{
		"Result": result,
		"Keys":   keys,
	}
	return c.Render(http.StatusOK, "query_result.html", data)
}
