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
	e.GET("/dashboard", web.HandleDashboard)
	e.GET("/collections/:name", web.HandleCollectionPage)

	// Public auth routes.
	e.POST("/auth/register", auth.HandleRegister)
	e.POST("/auth/login", auth.HandleLogin)

	// Protected API routes.
	apiGroup := e.Group("/api")
	apiGroup.Use(api.JWTMiddleware)
	apiGroup.GET("", server.HandleGetCollections)
	apiGroup.POST("/:collname", server.HandlePostInsert)
	apiGroup.GET("/:collname", server.HandleGetQuery)
	apiGroup.PUT("/:collname", server.HandlePutUpdate)
	apiGroup.DELETE("/:collname", server.HandleDelete)

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
