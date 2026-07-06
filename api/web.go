package api

import (
	"html/template"
	"io"
	"net/http"

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
	result, err := w.db.Coll(name).Limit(50).Find()
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

	data := map[string]any{
		"Name":    name,
		"Result":  result,
		"Keys":    keys,
	}
	return c.Render(http.StatusOK, "collection.html", data)
}
