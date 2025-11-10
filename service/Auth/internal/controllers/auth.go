package controllers

import (
	"net/http"

	"github.com/Zifeldev/emailback/service/Auth/internal/models"
	"github.com/Zifeldev/emailback/service/Auth/internal/repository"
	"github.com/Zifeldev/emailback/service/Auth/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type AuthController struct {
	authService service.AuthService
	log         *logrus.Entry
}

func NewAuthController(authService service.AuthService, log *logrus.Entry) *AuthController {
	return &AuthController{
		authService: authService,
		log:         log,
	}
}

func (ac *AuthController) Register(c *gin.Context) {
	var req models.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ac.log.WithField("error", err.Error()).Warn("invalid registration request")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tokens, err := ac.authService.Register(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		if err == repository.ErrUserExists {
			ac.log.WithField("email", req.Email).Warn("user already exists")
			c.JSON(http.StatusConflict, gin.H{"error": "user already exists"})
			return
		}
		ac.log.WithError(err).Error("failed to register user")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	// Set access token in cookie
	c.SetCookie("access_token", tokens.AccessToken, 15*60, "/", "", false, true)
	c.SetCookie("refresh_token", tokens.RefreshToken, 24*60*60, "/", "", false, true)

	ac.log.WithField("email", req.Email).Info("user registered successfully")

	c.JSON(http.StatusCreated, gin.H{
		"access_token":  tokens.AccessToken,
		"refresh_token": tokens.RefreshToken,
		"expires_in":    tokens.ExpiresIn,
	})
}

// @Summary Login user
// @Tags auth
// @Accept json
// @Produce json
// @Param request body models.LoginRequest true "Login credentials"
// @Success 200 {object} models.TokenPair
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /auth/login [post]
func (ac *AuthController) Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ac.log.WithField("error", err.Error()).Warn("invalid login request")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tokens, err := ac.authService.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		if err == service.ErrInvalidCredentials {
			ac.log.WithField("email", req.Email).Warn("invalid credentials")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}
		ac.log.WithError(err).Error("failed to login user")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.SetCookie("access_token", tokens.AccessToken, 15*60, "/", "", false, true)
	c.SetCookie("refresh_token", tokens.RefreshToken, 24*60*60, "/", "", false, true)

	ac.log.WithField("email", req.Email).Info("user logged in successfully")

	c.JSON(http.StatusOK, gin.H{
		"access_token":  tokens.AccessToken,
		"refresh_token": tokens.RefreshToken,
		"expires_in":    tokens.ExpiresIn,
	})
}

func (ac *AuthController) Refresh(c *gin.Context) {
	refreshToken, err := c.Cookie("refresh_token")
	if err != nil || refreshToken == "" {
		var req models.RefreshRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			ac.log.WithField("error", err.Error()).Warn("invalid refresh request")
			c.JSON(http.StatusBadRequest, gin.H{"error": "refresh token required in cookie or body"})
			return
		}
		refreshToken = req.RefreshToken
	}

	tokens, err := ac.authService.RefreshTokens(c.Request.Context(), refreshToken)
	if err != nil {
		ac.log.WithError(err).Warn("failed to refresh tokens")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired refresh token"})
		return
	}

	c.SetCookie("access_token", tokens.AccessToken, 15*60, "/", "", false, true)
	c.SetCookie("refresh_token", tokens.RefreshToken, 24*60*60, "/", "", false, true)

	ac.log.Info("tokens refreshed successfully")

	c.JSON(http.StatusOK, gin.H{
		"access_token":  tokens.AccessToken,
		"refresh_token": tokens.RefreshToken,
		"expires_in":    tokens.ExpiresIn,
	})
}

func (ac *AuthController) Logout(c *gin.Context) {
	refreshToken, err := c.Cookie("refresh_token")
	if err != nil || refreshToken == "" {
		var req models.RefreshRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			ac.log.WithField("error", err.Error()).Warn("invalid logout request")
			c.JSON(http.StatusBadRequest, gin.H{"error": "refresh token required in cookie or body"})
			return
		}
		refreshToken = req.RefreshToken
	}

	if err := ac.authService.RevokeToken(c.Request.Context(), refreshToken); err != nil {
		ac.log.WithError(err).Error("failed to revoke token")
	}

	c.SetCookie("access_token", "", -1, "/", "", false, true)
	c.SetCookie("refresh_token", "", -1, "/", "", false, true)

	ac.log.Info("user logged out successfully")

	c.JSON(http.StatusOK, gin.H{"message": "logged out successfully"})
}
