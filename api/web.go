package api

import (
	"html/template"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

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

// HandleUserPortal renders the portal for non-admin users.
func (w *WebHandler) HandleUserPortal(c echo.Context) error {
	username, ok := c.Get("username").(string)
	if !ok {
		username = "User"
	}
	return c.Render(http.StatusOK, "user_portal.html", map[string]any{
		"Username": username,
	})
}

// HandleDashboard renders the main admin dashboard.
func (w *WebHandler) HandleDashboard(c echo.Context) error {
	username, ok := c.Get("username").(string)
	if !ok || username == "" {
		return c.Redirect(http.StatusFound, "/login")
	}

	collections, err := NewSchemaStore(w.db).ListByOwner(username)
	if err != nil {
		collections = []string{}
	}

	type collInfo struct {
		Name  string
		Count int
	}
	var colls []collInfo
	for _, name := range collections {
		result, err := w.db.Coll(name).Eq(sprig.Map{"_owner": username}).Find()
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
	username, ok := c.Get("username").(string)
	if !ok || username == "" {
		return c.Redirect(http.StatusFound, "/login")
	}
	
	page := 1
	if p, err := strconv.Atoi(c.QueryParam("page")); err == nil && p > 0 {
		page = p
	}
	limit := 50
	offset := (page - 1) * limit

	result, err := w.db.Coll(name).Eq(sprig.Map{"_owner": username}).Offset(offset).Limit(limit).Find()
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
	username, ok := c.Get("username").(string)
	if !ok || username == "" {
		return c.Render(http.StatusOK, "query_result.html", map[string]any{"Error": "unauthorized"})
	}

	lang := c.FormValue("language")
	var result *sprig.QueryResult
	var err error
	var name string

	if lang == "sql" {
		q := strings.TrimSpace(c.FormValue("sql_query"))
		var selectedKeys []string
		result, selectedKeys, err = ExecuteSQLQuery(w.db, username, q)
		if err != nil {
			return c.Render(http.StatusOK, "query_result.html", map[string]any{"Error": err.Error()})
		}
		if result == nil {
			result = &sprig.QueryResult{Data: []sprig.Map{}, Total: 0}
		}

		keys := selectedKeys
		if len(keys) == 0 {
			keySet := map[string]bool{}
			for _, doc := range result.Data {
				for key := range doc {
					keySet[key] = true
				}
			}
			for key := range keySet {
				keys = append(keys, key)
			}
		}

		data := map[string]any{
			"Result": result,
			"Keys":   keys,
		}
		return c.Render(http.StatusOK, "query_result.html", data)
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
		query = query.Eq(sprig.Map{"_owner": username})
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

// HandleSettings renders a tenant-scoped settings page.
func (w *WebHandler) HandleSettings(c echo.Context) error {
	username, ok := c.Get("username").(string)
	if !ok || username == "" {
		return c.Redirect(http.StatusFound, "/login")
	}
	return c.Render(http.StatusOK, "settings.html", map[string]any{
		"Username": username,
	})
}

// HandleLogs renders the last request logs for the current tenant.
func (w *WebHandler) HandleLogs(c echo.Context) error {
	username, ok := c.Get("username").(string)
	if !ok || username == "" {
		return c.Redirect(http.StatusFound, "/login")
	}

	result, err := w.db.Coll("_logs").Eq(sprig.Map{"_owner": username}).Find()
	if err != nil {
		result = &sprig.QueryResult{Data: []sprig.Map{}, Total: 0}
	}

	// Sort by ts descending (ts stored as float64 after JSON unmarshal).
	type logRow struct {
		m   sprig.Map
		ts  int64
		idx int
	}
	rows := make([]logRow, 0, len(result.Data))
	for i, m := range result.Data {
		ts := int64(0)
		if v, ok := m["ts"].(float64); ok {
			ts = int64(v)
		}
		rows = append(rows, logRow{m: m, ts: ts, idx: i})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].ts > rows[j].ts
	})

	limit := 50
	if len(rows) < limit {
		limit = len(rows)
	}

	type viewLog struct {
		Time   string
		Method string
		Path   string
		Status int
		Error  string
	}
	view := make([]viewLog, 0, limit)
	for i := 0; i < limit; i++ {
		m := rows[i].m
		view = append(view, viewLog{
			Time:   formatUnixNano(m["ts"]),
			Method: strOrEmpty(m["method"]),
			Path:   strOrEmpty(m["path"]),
			Status: int(floatToInt(m["status"])),
			Error:  strOrEmpty(m["error"]),
		})
	}

	// Ensure deterministic empty state.
	if err == nil && view == nil {
		view = []viewLog{}
	}

	return c.Render(http.StatusOK, "logs.html", map[string]any{
		"Username": username,
		"Logs":     view,
	})
}

func formatUnixNano(ts any) string {
	// ts comes back as float64 due to JSON decoding into map[string]any.
	v, ok := ts.(float64)
	if !ok {
		return ""
	}
	t := time.Unix(0, int64(v))
	return t.Format("2006-01-02 15:04:05")
}

func strOrEmpty(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func floatToInt(v any) float64 {
	if f, ok := v.(float64); ok {
		return f
	}
	return 0
}
