package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Zifeldev/emailback/service/Auth/internal/models"
	"github.com/Zifeldev/emailback/service/Auth/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) Create(ctx context.Context, email, passwordHash string) (*models.User, error) {
	args := m.Called(ctx, email, passwordHash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepository) CreateWithRole(ctx context.Context, email, passwordHash, role string) (*models.User, error) {
	args := m.Called(ctx, email, passwordHash, role)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepository) GetByID(ctx context.Context, id int64) (*models.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepository) UpdateRole(ctx context.Context, id int64, role string) (*models.User, error) {
	args := m.Called(ctx, id, role)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepository) Delete(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockUserRepository) List(ctx context.Context, limit, offset int) ([]*models.User, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.User), args.Error(1)
}

func setupAdminTest() (*gin.Engine, *MockUserRepository, *AdminController) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	mockRepo := new(MockUserRepository)
	log := logrus.NewEntry(logrus.New())
	controller := NewAdminController(mockRepo, log)

	return r, mockRepo, controller
}

func TestCreateUser_Success(t *testing.T) {
	r, mockRepo, controller := setupAdminTest()

	r.POST("/admin/users", controller.CreateUser)

	mockUser := &models.User{
		ID:        1,
		Email:     "newuser@example.com",
		Role:      models.RoleUser,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	mockRepo.On("CreateWithRole", mock.Anything, "newuser@example.com", mock.Anything, models.RoleUser).
		Return(mockUser, nil)

	reqBody := map[string]string{
		"email":    "newuser@example.com",
		"password": "password123",
		"role":     "user",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/admin/users", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response models.User
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "newuser@example.com", response.Email)
	assert.Equal(t, models.RoleUser, response.Role)

	mockRepo.AssertExpectations(t)
}

func TestCreateUser_InvalidRole(t *testing.T) {
	r, _, controller := setupAdminTest()

	r.POST("/admin/users", controller.CreateUser)

	reqBody := map[string]string{
		"email":    "newuser@example.com",
		"password": "password123",
		"role":     "invalid_role",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/admin/users", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateUser_UserExists(t *testing.T) {
	r, mockRepo, controller := setupAdminTest()

	r.POST("/admin/users", controller.CreateUser)

	mockRepo.On("CreateWithRole", mock.Anything, "existing@example.com", mock.Anything, models.RoleUser).
		Return(nil, repository.ErrUserExists)

	reqBody := map[string]string{
		"email":    "existing@example.com",
		"password": "password123",
		"role":     "user",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/admin/users", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)

	mockRepo.AssertExpectations(t)
}

func TestListUsers_Success(t *testing.T) {
	r, mockRepo, controller := setupAdminTest()

	r.GET("/admin/users", controller.ListUsers)

	mockUsers := []*models.User{
		{
			ID:        1,
			Email:     "user1@example.com",
			Role:      models.RoleUser,
			CreatedAt: time.Now(),
		},
		{
			ID:        2,
			Email:     "admin@example.com",
			Role:      models.RoleAdmin,
			CreatedAt: time.Now(),
		},
	}

	mockRepo.On("List", mock.Anything, 10, 0).
		Return(mockUsers, nil)

	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response []models.User
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Len(t, response, 2)

	mockRepo.AssertExpectations(t)
}

func TestListUsers_WithPagination(t *testing.T) {
	r, mockRepo, controller := setupAdminTest()

	r.GET("/admin/users", controller.ListUsers)

	mockUsers := []*models.User{
		{
			ID:    1,
			Email: "user1@example.com",
			Role:  models.RoleUser,
		},
	}

	mockRepo.On("List", mock.Anything, 10, 20).
		Return(mockUsers, nil)

	req := httptest.NewRequest(http.MethodGet, "/admin/users?limit=10&offset=20", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	mockRepo.AssertExpectations(t)
}

func TestUpdateUserRole_Success(t *testing.T) {
	r, mockRepo, controller := setupAdminTest()

	r.PUT("/admin/users/:id/role", controller.UpdateUserRole)

	mockUser := &models.User{
		ID:    1,
		Email: "user@example.com",
		Role:  models.RoleAdmin,
	}

	mockRepo.On("UpdateRole", mock.Anything, int64(1), models.RoleAdmin).
		Return(mockUser, nil)

	reqBody := map[string]string{
		"role": "admin",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/admin/users/1/role", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	mockRepo.AssertExpectations(t)
}

func TestUpdateUserRole_InvalidRole(t *testing.T) {
	r, _, controller := setupAdminTest()

	r.PUT("/admin/users/:id/role", controller.UpdateUserRole)

	reqBody := map[string]string{
		"role": "invalid_role",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/admin/users/1/role", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateUserRole_UserNotFound(t *testing.T) {
	r, mockRepo, controller := setupAdminTest()

	r.PUT("/admin/users/:id/role", controller.UpdateUserRole)

	mockRepo.On("UpdateRole", mock.Anything, int64(999), models.RoleAdmin).
		Return(nil, repository.ErrUserNotFound)

	reqBody := map[string]string{
		"role": "admin",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/admin/users/999/role", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	mockRepo.AssertExpectations(t)
}

func TestDeleteUser_Success(t *testing.T) {
	r, mockRepo, controller := setupAdminTest()

	r.DELETE("/admin/users/:id", controller.DeleteUser)

	mockRepo.On("Delete", mock.Anything, int64(2)).
		Return(nil)

	req := httptest.NewRequest(http.MethodDelete, "/admin/users/2", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "user deleted successfully", response["message"])

	mockRepo.AssertExpectations(t)
}

func TestDeleteUser_NotFound(t *testing.T) {
	r, mockRepo, controller := setupAdminTest()

	r.DELETE("/admin/users/:id", controller.DeleteUser)

	mockRepo.On("Delete", mock.Anything, int64(999)).
		Return(repository.ErrUserNotFound)

	req := httptest.NewRequest(http.MethodDelete, "/admin/users/999", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	mockRepo.AssertExpectations(t)
}

func TestDeleteUser_InvalidID(t *testing.T) {
	r, _, controller := setupAdminTest()

	r.DELETE("/admin/users/:id", controller.DeleteUser)

	req := httptest.NewRequest(http.MethodDelete, "/admin/users/invalid", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
