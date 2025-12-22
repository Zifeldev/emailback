package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/sirupsen/logrus"
)

// JWT Claims structure - matches Auth service claims
type JWTClaims struct {
	UserID int64  `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"` // "user", "admin"
	jwt.RegisteredClaims
}

// JWTMiddleware validates JWT tokens and extracts user info
type JWTMiddleware struct {
	accessSecret string
	log          *logrus.Entry
}

// NewJWTMiddleware creates a new JWT middleware
func NewJWTMiddleware(accessSecret string, log *logrus.Entry) *JWTMiddleware {
	return &JWTMiddleware{
		accessSecret: accessSecret,
		log:          log,
	}
}

// Authenticate validates JWT token and adds user info to context
func (m *JWTMiddleware) Authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := m.extractToken(c)
		if err != nil {
			m.log.WithError(err).Warn("authentication failed")
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "invalid or missing authentication token",
			})
			c.Abort()
			return
		}

		claims, err := m.validateToken(token)
		if err != nil {
			m.log.WithError(err).Warn("token validation failed")
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "invalid token",
			})
			c.Abort()
			return
		}

		// Check if token is expired
		if claims.ExpiresAt != nil && claims.ExpiresAt.Time.Before(time.Now()) {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "token expired",
			})
			c.Abort()
			return
		}

		// Add user info to context
		c.Set("user_id", claims.UserID)
		c.Set("email", claims.Email)
		c.Set("user_role", claims.Role)

		m.log.WithFields(logrus.Fields{
			"user_id": claims.UserID,
			"email":   claims.Email,
			"role":    claims.Role,
		}).Debug("user authenticated")

		c.Next()
	}
}

// RequireRole checks if user has required role (for admin endpoints)
func (m *JWTMiddleware) RequireRole(requiredRole string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("user_role")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "authentication required",
			})
			c.Abort()
			return
		}

		userRole, ok := role.(string)
		if !ok || userRole != requiredRole {
			m.log.WithFields(logrus.Fields{
				"user_role":     userRole,
				"required_role": requiredRole,
			}).Warn("insufficient permissions")

			c.JSON(http.StatusForbidden, gin.H{
				"error":   "forbidden",
				"message": fmt.Sprintf("requires %s role", requiredRole),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// extractToken extracts JWT token from Authorization header
func (m *JWTMiddleware) extractToken(c *gin.Context) (string, error) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		return "", errors.New("authorization header missing")
	}

	// Expected format: "Bearer <token>"
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return "", errors.New("invalid authorization header format")
	}

	return parts[1], nil
}

// validateToken validates JWT token and returns claims
func (m *JWTMiddleware) validateToken(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(m.accessSecret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok {
		return nil, errors.New("invalid token claims")
	}

	return claims, nil
}

// GetUserID extracts user ID from context (helper for controllers)
func GetUserID(c *gin.Context) (int64, error) {
	userID, exists := c.Get("user_id")
	if !exists {
		return 0, errors.New("user_id not found in context")
	}

	id, ok := userID.(int64)
	if !ok {
		return 0, errors.New("invalid user_id type")
	}

	return id, nil
}

// GetEmail extracts email from context
func GetEmail(c *gin.Context) (string, error) {
	email, exists := c.Get("email")
	if !exists {
		return "", errors.New("email not found in context")
	}

	e, ok := email.(string)
	if !ok {
		return "", errors.New("invalid email type")
	}

	return e, nil
}

// GetUserRole extracts user role from context
func GetUserRole(c *gin.Context) (string, error) {
	role, exists := c.Get("user_role")
	if !exists {
		return "", errors.New("user_role not found in context")
	}

	r, ok := role.(string)
	if !ok {
		return "", errors.New("invalid user_role type")
	}

	return r, nil
}

// IsAdmin checks if current user is admin
func IsAdmin(c *gin.Context) bool {
	role, err := GetUserRole(c)
	return err == nil && role == "admin"
}
