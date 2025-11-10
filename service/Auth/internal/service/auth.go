package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	"github.com/Zifeldev/emailback/service/Auth/internal/config"
	"github.com/Zifeldev/emailback/service/Auth/internal/models"
	"github.com/Zifeldev/emailback/service/Auth/internal/repository"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidToken       = errors.New("invalid token")
)

type AuthService interface {
	Register(ctx context.Context, email, password string) (*models.TokenPair, error)
	Login(ctx context.Context, email, password string) (*models.TokenPair, error)
	RefreshTokens(ctx context.Context, refreshToken string) (*models.TokenPair, error)
	RevokeToken(ctx context.Context, refreshToken string) error
	ValidateAccessToken(tokenString string) (*models.AccessTokenClaims, error)
}

type authService struct {
	cfg       *config.JWTConfig
	userRepo  repository.UserRepository
	tokenRepo repository.TokenRepository
}

func NewAuthService(cfg *config.JWTConfig, userRepo repository.UserRepository, tokenRepo repository.TokenRepository) AuthService {
	return &authService{
		cfg:       cfg,
		userRepo:  userRepo,
		tokenRepo: tokenRepo,
	}
}

func (s *authService) Register(ctx context.Context, email, password string) (*models.TokenPair, error) {
	// Hash password
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	// Create user
	user, err := s.userRepo.Create(ctx, email, string(passwordHash))
	if err != nil {
		return nil, err
	}

	// Generate tokens
	return s.generateTokenPair(ctx, user)
}

func (s *authService) Login(ctx context.Context, email, password string) (*models.TokenPair, error) {
	// Get user
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	// Check password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	// Generate tokens
	return s.generateTokenPair(ctx, user)
}

func (s *authService) RefreshTokens(ctx context.Context, refreshToken string) (*models.TokenPair, error) {
	// Validate refresh token
	_, err := s.validateRefreshToken(refreshToken)
	if err != nil {
		return nil, err
	}

	// Check token in database
	storedToken, err := s.tokenRepo.GetRefreshToken(ctx, refreshToken)
	if err != nil {
		return nil, err
	}

	// Get user
	user, err := s.userRepo.GetByID(ctx, storedToken.UserID)
	if err != nil {
		return nil, err
	}

	// Revoke old refresh token
	if err := s.tokenRepo.RevokeRefreshToken(ctx, refreshToken); err != nil {
		return nil, err
	}

	// Generate new tokens
	return s.generateTokenPair(ctx, user)
}

func (s *authService) RevokeToken(ctx context.Context, refreshToken string) error {
	return s.tokenRepo.RevokeRefreshToken(ctx, refreshToken)
}

func (s *authService) ValidateAccessToken(tokenString string) (*models.AccessTokenClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return []byte(s.cfg.AccessSecret), nil
	})

	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, ErrInvalidToken
	}

	userID, ok := claims["user_id"].(float64)
	if !ok {
		return nil, ErrInvalidToken
	}

	email, ok := claims["email"].(string)
	if !ok {
		return nil, ErrInvalidToken
	}

	role, ok := claims["role"].(string)
	if !ok {
		role = models.RoleUser
	}

	return &models.AccessTokenClaims{
		UserID: int64(userID),
		Email:  email,
		Role:   role,
	}, nil
}



func (s *authService) generateTokenPair(ctx context.Context, user *models.User) (*models.TokenPair, error) {
	accessToken, err := s.generateAccessToken(user)
	if err != nil {
		return nil, err
	}


	refreshToken, err := s.generateRefreshToken()
	if err != nil {
		return nil, err
	}

	expiresAt := time.Now().Add(s.cfg.RefreshExpiration)
	_, err = s.tokenRepo.CreateRefreshToken(ctx, user.ID, refreshToken, expiresAt)
	if err != nil {
		return nil, err
	}

	return &models.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(s.cfg.AccessExpiration.Seconds()),
	}, nil
}

func (s *authService) generateAccessToken(user *models.User) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"user_id": user.ID,
		"email":   user.Email,
		"role":    user.Role,
		"iss":     s.cfg.Issuer,
		"iat":     now.Unix(),
		"exp":     now.Add(s.cfg.AccessExpiration).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.cfg.AccessSecret))
}

func (s *authService) generateRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func (s *authService) validateRefreshToken(tokenString string) (*models.RefreshTokenClaims, error) {
	if tokenString == "" {
		return nil, ErrInvalidToken
	}
	return &models.RefreshTokenClaims{}, nil
}
