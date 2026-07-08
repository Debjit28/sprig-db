package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Debjit28/sprig-db/api"
	"github.com/Debjit28/sprig-db/sprig"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func main() {
	logger := sprig.NewLogger(slog.LevelInfo)
	slog.SetDefault(logger)

	// Clean-slate mode is opt-in.
	// Set SPRIG_RESET_DATA=true to delete the DB on startup.
	if os.Getenv("SPRIG_RESET_DATA") == "true" || os.Getenv("SPRIG_RESET_DATA") == "1" {
		if err := os.Remove("default.sprig"); err != nil && !os.IsNotExist(err) {
			slog.Error("failed to reset data file", "error", err)
			os.Exit(1)
		}
	}

	db, err := sprig.New()
	if err != nil {
		slog.Error("failed to initialize database", "error", err)
		os.Exit(1)
	}

	server := api.NewServer(db)
	auth := api.NewAuthHandler(db)
	web := api.NewWebHandler(db)

	e := echo.New()
	e.HideBanner = true
	e.Renderer = api.NewTemplateRenderer("templates/*.html")

	// Global middleware.
	e.Use(middleware.Recover())
	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogStatus: true,
		LogURI:    true,
		LogMethod: true,
		LogError:  true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			slog.Info("request",
				"method", v.Method,
				"uri", v.URI,
				"status", v.Status,
			)
			return nil
		},
	}))

	// Best-effort request logging into the DB (tenant-scoped).
	// This powers the "Logs" UI.
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			err := next(c)

			owner, _ := c.Get("username").(string)
			if owner != "" {
				status := c.Response().Status
				var errMsg string
				if err != nil {
					errMsg = err.Error()
				}
				_, _ = db.Coll("_logs").Insert(sprig.Map{
					"_owner": owner,
					"ts":      time.Now().UnixNano(),
					"method":  req.Method,
					"path":     req.URL.Path,
					"status":  status,
					"error":   errMsg,
				})
			}

			return err
		}
	})

	// Custom error handler.
	e.HTTPErrorHandler = func(err error, c echo.Context) {
		code := http.StatusInternalServerError
		msg := err.Error()
		if he, ok := err.(*echo.HTTPError); ok {
			code = he.Code
			if m, ok := he.Message.(string); ok {
				msg = m
			}
		}
		slog.Error("http error", "code", code, "error", msg)
		c.JSON(code, sprig.Map{"error": msg})
	}

	// Static files.
	e.Static("/static", "static")

	// Web routes (HTML pages).
	e.GET("/", func(c echo.Context) error { return c.Redirect(http.StatusFound, "/login") })
	e.GET("/login", web.HandleLoginPage)

	// Protected web routes (requires login).
	webGroup := e.Group("")
	webGroup.Use(api.CookieOrJWTMiddleware)

	// User portal (accessible to all logged-in users)
	webGroup.GET("/user-portal", web.HandleUserPortal)

	// Tenant-scoped dashboard routes (all logged-in users).
	tenantGroup := webGroup.Group("/dashboard")
	tenantGroup.GET("", web.HandleDashboard)
	tenantGroup.POST("/query", web.HandleDashboardQuery)
	tenantGroup.GET("/settings", web.HandleSettings)
	tenantGroup.GET("/logs", web.HandleLogs)

	// Tenant-scoped collection browsing (all logged-in users).
	collGroup := webGroup.Group("/collections")
	collGroup.GET("/:name", web.HandleCollectionPage)

	// Public auth routes.
	e.GET("/auth/captcha", auth.HandleCaptcha)
	e.POST("/auth/register", auth.HandleRegister)
	e.POST("/auth/login", auth.HandleLogin)
	e.POST("/auth/forgot-password", auth.HandleForgotPassword)
	e.POST("/auth/reset-password", auth.HandleResetPassword)

	// Protected API routes.
	apiGroup := e.Group("/api")
	apiGroup.Use(api.CookieOrJWTAPIMiddleware)
	apiGroup.GET("/collections", server.HandleGetCollections)
	apiGroup.POST("/collections", server.HandleCreateCollection)
	apiGroup.GET("/collections/:collname/schema", server.HandleGetCollectionSchema)
	apiGroup.PUT("/collections/:collname/schema", server.HandlePutCollectionSchema)
	apiGroup.DELETE("/collections/:collname", server.HandleDeleteCollection)
	apiGroup.POST("/records/:collname", server.HandlePostInsert)
	apiGroup.GET("/records/:collname", server.HandleGetQuery)
	apiGroup.PUT("/records/:collname", server.HandlePutUpdate)
	apiGroup.DELETE("/records/:collname", server.HandleDelete)
	apiGroup.POST("/settings/reset", server.HandleResetMyData)

	// Start server in goroutine.
	go func() {
		slog.Info("starting sprig-db server", "port", 7777)
		if err := e.Start(":7777"); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
		}
	}()

	// Graceful shutdown.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := e.Shutdown(ctx); err != nil {
		slog.Error("server shutdown error", "error", err)
	}
	if err := db.Close(); err != nil {
		slog.Error("database close error", "error", err)
	}

	slog.Info("server stopped cleanly")
}
