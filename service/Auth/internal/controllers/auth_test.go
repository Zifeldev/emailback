package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Zifeldev/emailback/service/Auth/internal/models"
	"github.com/Zifeldev/emailback/service/Auth/internal/repository"
	"github.com/Zifeldev/emailback/service/Auth/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockAuthService struct {
	mock.Mock
}

func (m *MockAuthService) Register(ctx context.Context, email, password string) (*models.TokenPair, error) {
	args := m.Called(ctx, email, password)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.TokenPair), args.Error(1)
}

func (m *MockAuthService) Login(ctx context.Context, email, password string) (*models.TokenPair, error) {
	args := m.Called(ctx, email, password)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.TokenPair), args.Error(1)
}

func (m *MockAuthService) RefreshTokens(ctx context.Context, refreshToken string) (*models.TokenPair, error) {
	args := m.Called(ctx, refreshToken)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.TokenPair), args.Error(1)
}

func (m *MockAuthService) RevokeToken(ctx context.Context, refreshToken string) error {
	args := m.Called(ctx, refreshToken)
	return args.Error(0)
}

func (m *MockAuthService) ValidateAccessToken(token string) (*models.AccessTokenClaims, error) {
	args := m.Called(token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.AccessTokenClaims), args.Error(1)
}

func setupTest() (*gin.Engine, *MockAuthService, *AuthController) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	mockService := new(MockAuthService)
	log := logrus.NewEntry(logrus.New())
	controller := NewAuthController(mockService, log)

	return r, mockService, controller
}

func TestRegister_Success(t *testing.T) {
	r, mockService, controller := setupTest()

	r.POST("/auth/register", controller.Register)

	mockTokens := &models.TokenPair{
		AccessToken:  "access_token_123",
		RefreshToken: "refresh_token_456",
		ExpiresIn:    900,
	}

	mockService.On("Register", mock.Anything, "test@example.com", "password123").
		Return(mockTokens, nil)

	reqBody := map[string]string{
		"email":    "test@example.com",
		"password": "password123",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "access_token_123", response["access_token"])
	assert.Equal(t, "refresh_token_456", response["refresh_token"])
	assert.Equal(t, float64(900), response["expires_in"])

	cookies := w.Result().Cookies()
	assert.Len(t, cookies, 2)

	mockService.AssertExpectations(t)
}

func TestRegister_UserExists(t *testing.T) {
	r, mockService, controller := setupTest()

	r.POST("/auth/register", controller.Register)

	mockService.On("Register", mock.Anything, "existing@example.com", "password123").
		Return(nil, repository.ErrUserExists)

	reqBody := map[string]string{
		"email":    "existing@example.com",
		"password": "password123",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "user already exists", response["error"])

	mockService.AssertExpectations(t)
}

func TestRegister_InvalidRequest(t *testing.T) {
	r, _, controller := setupTest()

	r.POST("/auth/register", controller.Register)

	reqBody := map[string]string{
		"email": "invalid-email",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLogin_Success(t *testing.T) {
	r, mockService, controller := setupTest()

	r.POST("/auth/login", controller.Login)

	mockTokens := &models.TokenPair{
		AccessToken:  "access_token_123",
		RefreshToken: "refresh_token_456",
		ExpiresIn:    900,
	}

	mockService.On("Login", mock.Anything, "test@example.com", "password123").
		Return(mockTokens, nil)

	reqBody := map[string]string{
		"email":    "test@example.com",
		"password": "password123",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "access_token_123", response["access_token"])
	assert.Equal(t, "refresh_token_456", response["refresh_token"])

	mockService.AssertExpectations(t)
}

func TestLogin_InvalidCredentials(t *testing.T) {
	r, mockService, controller := setupTest()

	r.POST("/auth/login", controller.Login)

	mockService.On("Login", mock.Anything, "test@example.com", "wrongpassword").
		Return(nil, service.ErrInvalidCredentials)

	reqBody := map[string]string{
		"email":    "test@example.com",
		"password": "wrongpassword",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	mockService.AssertExpectations(t)
}

func TestRefresh_Success_FromCookie(t *testing.T) {
	r, mockService, controller := setupTest()

	r.POST("/auth/refresh", controller.Refresh)

	mockTokens := &models.TokenPair{
		AccessToken:  "new_access_token",
		RefreshToken: "new_refresh_token",
		ExpiresIn:    900,
	}

	mockService.On("RefreshTokens", mock.Anything, "old_refresh_token").
		Return(mockTokens, nil)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req.AddCookie(&http.Cookie{
		Name:  "refresh_token",
		Value: "old_refresh_token",
	})
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "new_access_token", response["access_token"])

	mockService.AssertExpectations(t)
}

func TestRefresh_Success_FromBody(t *testing.T) {
	r, mockService, controller := setupTest()

	r.POST("/auth/refresh", controller.Refresh)

	mockTokens := &models.TokenPair{
		AccessToken:  "new_access_token",
		RefreshToken: "new_refresh_token",
		ExpiresIn:    900,
	}

	mockService.On("RefreshTokens", mock.Anything, "body_refresh_token").
		Return(mockTokens, nil)

	reqBody := map[string]string{
		"refresh_token": "body_refresh_token",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	mockService.AssertExpectations(t)
}

func TestRefresh_InvalidToken(t *testing.T) {
	r, mockService, controller := setupTest()

	r.POST("/auth/refresh", controller.Refresh)

	mockService.On("RefreshTokens", mock.Anything, "invalid_token").
		Return(nil, service.ErrInvalidToken)

	reqBody := map[string]string{
		"refresh_token": "invalid_token",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	mockService.AssertExpectations(t)
}

func TestLogout_Success_FromCookie(t *testing.T) {
	r, mockService, controller := setupTest()

	r.POST("/auth/logout", controller.Logout)

	mockService.On("RevokeToken", mock.Anything, "refresh_token_to_revoke").
		Return(nil)

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req.AddCookie(&http.Cookie{
		Name:  "refresh_token",
		Value: "refresh_token_to_revoke",
	})
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "logged out successfully", response["message"])

	cookies := w.Result().Cookies()
	assert.True(t, len(cookies) >= 2)

	mockService.AssertExpectations(t)
}

func TestLogout_Success_FromBody(t *testing.T) {
	r, mockService, controller := setupTest()

	r.POST("/auth/logout", controller.Logout)

	mockService.On("RevokeToken", mock.Anything, "body_refresh_token").
		Return(nil)

	reqBody := map[string]string{
		"refresh_token": "body_refresh_token",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	mockService.AssertExpectations(t)
}
