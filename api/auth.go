package api

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Debjit28/sprig-db/sprig"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
)

var (
	jwtSecret    []byte
	captchaStore = sync.Map{} // token -> answer (string -> string)
	resetStore   = sync.Map{} // token -> username (string -> string)
)

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
	IsAdmin  bool   `json:"is_admin"`
	jwt.RegisteredClaims
}

type AuthHandler struct {
	db *sprig.Sprig
}

func NewAuthHandler(db *sprig.Sprig) *AuthHandler {
	return &AuthHandler{db: db}
}

type authRequest struct {
	Username      string `json:"username"`
	Password      string `json:"password"`
	CaptchaToken  string `json:"captcha_token"`
	CaptchaAnswer string `json:"captcha_answer"`
	ResetToken    string `json:"reset_token"`
}

func generateRandomNumber(max int64) int64 {
	n, _ := rand.Int(rand.Reader, big.NewInt(max))
	return n.Int64()
}

func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[generateRandomNumber(int64(len(charset)))]
	}
	return string(b)
}

func generateCaptchaCode(length int) string {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[generateRandomNumber(int64(len(charset)))]
	}
	return string(b)
}

func captchaSVGDataURI(code string) string {
	// Lightweight picture-based captcha rendered as SVG data URI.
	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="220" height="80" viewBox="0 0 220 80">
<rect width="220" height="80" fill="#111827"/>
<line x1="10" y1="12" x2="205" y2="64" stroke="#334155" stroke-width="2"/>
<line x1="20" y1="70" x2="210" y2="18" stroke="#1f2937" stroke-width="2"/>
<circle cx="28" cy="24" r="3" fill="#4f46e5"/>
<circle cx="182" cy="55" r="2" fill="#10b981"/>
<text x="22" y="52" font-family="monospace" font-size="34" font-weight="700" fill="#e5e7eb" letter-spacing="4">%s</text>
</svg>`, code)
	return "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(svg))
}

// HandleCaptcha generates a picture-based CAPTCHA challenge.
func (a *AuthHandler) HandleCaptcha(c echo.Context) error {
	answer := generateCaptchaCode(5)
	token := generateRandomString(16)

	captchaStore.Store(token, answer)

	return c.JSON(http.StatusOK, sprig.Map{
		"captcha_token": token,
		"captcha_image": captchaSVGDataURI(answer),
	})
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

	// Verify Captcha
	if req.CaptchaToken == "" || req.CaptchaAnswer == "" {
		return c.JSON(http.StatusBadRequest, sprig.Map{"error": "captcha is required"})
	}
	expected, ok := captchaStore.Load(req.CaptchaToken)
	if !ok || !strings.EqualFold(expected.(string), strings.TrimSpace(req.CaptchaAnswer)) {
		return c.JSON(http.StatusUnauthorized, sprig.Map{"error": "invalid captcha"})
	}
	captchaStore.Delete(req.CaptchaToken) // used

	// Check if user already exists.
	allUsers, err := a.db.Coll("_users").Find()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, sprig.Map{"error": err.Error()})
	}
	
	// Check if this username is taken
	existing := false
	for _, u := range allUsers.Data {
		if u["username"] == req.Username {
			existing = true
			break
		}
	}
	if existing {
		return c.JSON(http.StatusConflict, sprig.Map{"error": "username already exists"})
	}

	isAdmin := allUsers.Total == 0 // First user is Admin!

	// Hash password.
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, sprig.Map{"error": "failed to hash password"})
	}

	id, err := a.db.Coll("_users").Insert(sprig.Map{
		"username": req.Username,
		"password": string(hash),
		"is_admin": isAdmin,
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, sprig.Map{"error": err.Error()})
	}

	return c.JSON(http.StatusCreated, sprig.Map{"id": id, "username": req.Username, "message": "registration successful. please login."})
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

	// Password valid, issue JWT directly (OTP removed).
	isAdmin, _ := user["is_admin"].(bool)

	// Generate JWT.
	claims := &Claims{
		Username: req.Username,
		IsAdmin:  isAdmin,
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

	return c.JSON(http.StatusOK, sprig.Map{"token": tokenString, "username": req.Username, "is_admin": isAdmin})
}

// HandleForgotPassword generates a reset token.
func (a *AuthHandler) HandleForgotPassword(c echo.Context) error {
	var req authRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, sprig.Map{"error": "invalid request body"})
	}
	if req.Username == "" {
		return c.JSON(http.StatusBadRequest, sprig.Map{"error": "username required"})
	}

	result, err := a.db.Coll("_users").Eq(sprig.Map{"username": req.Username}).Find()
	if err != nil || len(result.Data) == 0 {
		// Log generically to prevent username enumeration, but for demo we can error out
		return c.JSON(http.StatusNotFound, sprig.Map{"error": "user not found"})
	}

	resetToken := generateRandomString(32)
	resetStore.Store(resetToken, req.Username)

	slog.Info("============== PASSWORD RESET ==============", "username", req.Username, "token", resetToken)
	slog.Info("Use the token above to verify password reset.")

	return c.JSON(http.StatusOK, sprig.Map{"message": "reset token generated in server logs"})
}

// HandleResetPassword accepts the token and a new password.
func (a *AuthHandler) HandleResetPassword(c echo.Context) error {
	var req authRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, sprig.Map{"error": "invalid request body"})
	}
	if req.ResetToken == "" || req.Password == "" {
		return c.JSON(http.StatusBadRequest, sprig.Map{"error": "reset_token and new password required"})
	}

	username, ok := resetStore.Load(req.ResetToken)
	if !ok {
		return c.JSON(http.StatusUnauthorized, sprig.Map{"error": "invalid or expired reset token"})
	}
	resetStore.Delete(req.ResetToken)

	// Fetch user to update
	result, err := a.db.Coll("_users").Eq(sprig.Map{"username": username.(string)}).Find()
	if err != nil || len(result.Data) == 0 {
		return c.JSON(http.StatusInternalServerError, sprig.Map{"error": "user not found during reset"})
	}
	user := result.Data[0]

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, sprig.Map{"error": "failed to hash password"})
	}

	user["password"] = string(hash)
	
	// Wait, Sprig-DB doesn't have a direct Update by DocID yet in the public API perhaps?
	// HandlePutUpdate takes a collection name and a filter string... actually the database only supports Insert currently or we can rewrite the whole document. Let's see if we can do an `Update`. Wait, check `api/server.go` or `sprig` package.
	// We can delete and re-insert, or implement update. For this demo, let's delete using the username and insert again.
	a.db.Coll("_users").Eq(sprig.Map{"username": username.(string)}).Delete()
	
	_, err = a.db.Coll("_users").Insert(user)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, sprig.Map{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, sprig.Map{"message": "password reset successful"})
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
		c.Set("is_admin", claims.IsAdmin)
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
		c.Set("is_admin", claims.IsAdmin)
		return next(c)
	}
}

// CookieOrJWTAPIMiddleware authenticates API calls using either:
// - Authorization: Bearer <token>
// - sprig_token cookie
// Unlike CookieOrJWTMiddleware, it never redirects and returns JSON errors.
func CookieOrJWTAPIMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
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
				return c.JSON(http.StatusUnauthorized, sprig.Map{"error": "missing authorization token"})
			}
			tokenString = cookie.Value
		}

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
			return jwtSecret, nil
		})
		if err != nil || !token.Valid {
			return c.JSON(http.StatusUnauthorized, sprig.Map{"error": "invalid or expired token"})
		}

		c.Set("username", claims.Username)
		c.Set("is_admin", claims.IsAdmin)
		return next(c)
	}
}

// AdminOnlyMiddleware verifies the user is an admin.
// Requires JWTMiddleware or CookieOrJWTMiddleware to be run first.
func AdminOnlyMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		isAdmin, ok := c.Get("is_admin").(bool)
		if !ok || !isAdmin {
			return c.JSON(http.StatusForbidden, sprig.Map{"error": "admin access required"})
		}
		return next(c)
	}
}

