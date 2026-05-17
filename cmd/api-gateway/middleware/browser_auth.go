package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lgulliver/lodestone/pkg/config"
)

// UIAuthMiddleware authenticates browser requests via a session cookie.
func UIAuthMiddleware(authService AuthServiceInterface, authConfig *config.AuthConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionToken, err := c.Cookie(authConfig.UISessionCookieName)
		if err != nil || sessionToken == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}

		ctx := context.WithValue(c.Request.Context(), contextKeyToken, sessionToken)
		user, err := authService.ValidateToken(ctx, sessionToken)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}

		if requiresCSRFMiddlewareCheck(c.Request.Method) {
			csrfHeader := c.GetHeader("X-CSRF-Token")
			expectedCSRFToken := BuildCSRFSignedToken(sessionToken, authConfig.JWTSecret)
			if csrfHeader == "" || subtle.ConstantTimeCompare([]byte(expectedCSRFToken), []byte(csrfHeader)) != 1 {
				c.JSON(http.StatusForbidden, gin.H{"error": "csrf validation failed"})
				c.Abort()
				return
			}
		}

		c.Set("user", user)
		c.Next()
	}
}

func requiresCSRFMiddlewareCheck(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	default:
		return true
	}
}

// BuildCSRFSignedToken derives a stable CSRF token from the authenticated session token.
func BuildCSRFSignedToken(sessionToken, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(sessionToken))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
