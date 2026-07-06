package api

import (
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Debjit28/sprig-db/sprig"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
)

var jwtSecret []byte

func init() {
	secret := os.Getenv("SPRIG_JWT_SECRET")
	if secret == "" {
		slog.Warn("SPRIG_JWT_SECRET not set, using insecure default — SET THIS IN PRODUCTION")
		secret = "sprig-dev-secret-DO-NOT-USE-IN-PROD"
	}
	jwtSecret = []byte(secret)
}

type Claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

type AuthHandler struct {
	db *sprig.Sprig
}

func NewAuthHandler(db *sprig.Sprig) *AuthHandler {
	return &AuthHandler{db: db}
}

type authRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// HandleRegister creates a new user.
func (a *AuthHandler) HandleRegister(c echo.Context) error {
	var req authRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, sprig.Map{"error": "invalid request body"})
	}
	if req.Username == "" || req.Password == "" {
		return c.JSON(http.StatusBadRequest, sprig.Map{"error": "username and password are required"})
	}

	// Check if user already exists.
	existing, err := a.db.Coll("_users").Eq(sprig.Map{"username": req.Username}).Find()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, sprig.Map{"error": err.Error()})
	}
	if len(existing.Data) > 0 {
		return c.JSON(http.StatusConflict, sprig.Map{"error": "username already exists"})
	}

	// Hash password.
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, sprig.Map{"error": "failed to hash password"})
	}

	id, err := a.db.Coll("_users").Insert(sprig.Map{
		"username": req.Username,
		"password": string(hash),
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, sprig.Map{"error": err.Error()})
	}

	return c.JSON(http.StatusCreated, sprig.Map{"id": id, "username": req.Username})
}

// HandleLogin authenticates a user and returns a JWT token.
func (a *AuthHandler) HandleLogin(c echo.Context) error {
	var req authRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, sprig.Map{"error": "invalid request body"})
	}
	if req.Username == "" || req.Password == "" {
		return c.JSON(http.StatusBadRequest, sprig.Map{"error": "username and password are required"})
	}

	// Find user.
	result, err := a.db.Coll("_users").Eq(sprig.Map{"username": req.Username}).Find()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, sprig.Map{"error": err.Error()})
	}
	if len(result.Data) == 0 {
		return c.JSON(http.StatusUnauthorized, sprig.Map{"error": "invalid credentials"})
	}

	user := result.Data[0]
	storedHash, ok := user["password"].(string)
	if !ok {
		return c.JSON(http.StatusInternalServerError, sprig.Map{"error": "corrupted user data"})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(req.Password)); err != nil {
		return c.JSON(http.StatusUnauthorized, sprig.Map{"error": "invalid credentials"})
	}

	// Generate JWT.
	claims := &Claims{
		Username: req.Username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, sprig.Map{"error": "failed to generate token"})
	}

	// Set cookie for browser-based dashboard access.
	c.SetCookie(&http.Cookie{
		Name:     "sprig_token",
		Value:    tokenString,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400, // 24 hours
	})

	return c.JSON(http.StatusOK, sprig.Map{"token": tokenString, "username": req.Username})
}

// JWTMiddleware validates Bearer tokens on protected API routes.
func JWTMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		authHeader := c.Request().Header.Get("Authorization")
		if authHeader == "" {
			return c.JSON(http.StatusUnauthorized, sprig.Map{"error": "missing authorization header"})
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			return c.JSON(http.StatusUnauthorized, sprig.Map{"error": "invalid authorization format"})
		}

		tokenString := parts[1]
		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
			return jwtSecret, nil
		})
		if err != nil || !token.Valid {
			return c.JSON(http.StatusUnauthorized, sprig.Map{"error": "invalid or expired token"})
		}

		c.Set("username", claims.Username)
		return next(c)
	}
}

// CookieOrJWTMiddleware checks for a Bearer header OR a sprig_token cookie.
// This supports both API clients (header) and the browser dashboard (cookie).
func CookieOrJWTMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		var tokenString string

		// Try Bearer header first.
		if authHeader := c.Request().Header.Get("Authorization"); authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && parts[0] == "Bearer" {
				tokenString = parts[1]
			}
		}

		// Fall back to cookie.
		if tokenString == "" {
			cookie, err := c.Cookie("sprig_token")
			if err != nil || cookie.Value == "" {
				// Redirect to login for browser requests.
				return c.Redirect(http.StatusFound, "/login")
			}
			tokenString = cookie.Value
		}

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
			return jwtSecret, nil
		})
		if err != nil || !token.Valid {
			return c.Redirect(http.StatusFound, "/login")
		}

		c.Set("username", claims.Username)
		return next(c)
	}
}

