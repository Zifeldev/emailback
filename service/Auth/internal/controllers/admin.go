package controllers

import (
	"net/http"
	"strconv"

	"github.com/Zifeldev/emailback/service/Auth/internal/models"
	"github.com/Zifeldev/emailback/service/Auth/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
)

type AdminController struct {
	userRepo repository.UserRepository
	log      *logrus.Entry
}

func NewAdminController(userRepo repository.UserRepository, log *logrus.Entry) *AdminController {
	return &AdminController{
		userRepo: userRepo,
		log:      log,
	}
}

// @Summary Create new user (Admin only)
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body models.CreateUserRequest true "User data"
// @Success 201 {object} models.User
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /admin/users [post]
func (ac *AdminController) CreateUser(c *gin.Context) {
	var req models.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ac.log.WithField("error", err.Error()).Warn("invalid create user request")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate role
	if err := models.ValidateRole(req.Role); err != nil {
		ac.log.WithFields(map[string]interface{}{
			"role":  req.Role,
			"error": err.Error(),
		}).Warn("invalid role")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Hash password
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		ac.log.WithError(err).Error("failed to hash password")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	// Create user with specified role
	user, err := ac.userRepo.CreateWithRole(c.Request.Context(), req.Email, string(passwordHash), req.Role)
	if err != nil {
		if err == repository.ErrUserExists {
			ac.log.WithField("email", req.Email).Warn("user already exists")
			c.JSON(http.StatusConflict, gin.H{"error": "user already exists"})
			return
		}
		ac.log.WithError(err).Error("failed to create user")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	ac.log.WithFields(map[string]interface{}{
		"email": req.Email,
		"role":  req.Role,
	}).Info("user created by admin")

	user.PasswordHash = ""

	c.JSON(http.StatusCreated, user)
}

// @Summary Update user role (Admin only)
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "User ID"
// @Param request body models.UpdateRoleRequest true "New role"
// @Success 200 {object} models.User
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /admin/users/{id}/role [put]
func (ac *AdminController) UpdateUserRole(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		ac.log.WithField("id", c.Param("id")).Warn("invalid user id")
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	var req models.UpdateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ac.log.WithField("error", err.Error()).Warn("invalid update role request")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate role
	if err := models.ValidateRole(req.Role); err != nil {
		ac.log.WithFields(map[string]interface{}{
			"role":  req.Role,
			"error": err.Error(),
		}).Warn("invalid role")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update role
	user, err := ac.userRepo.UpdateRole(c.Request.Context(), userID, req.Role)
	if err != nil {
		if err == repository.ErrUserNotFound {
			ac.log.WithField("user_id", userID).Warn("user not found")
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		ac.log.WithError(err).Error("failed to update user role")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	ac.log.WithFields(map[string]interface{}{
		"user_id":  userID,
		"email":    user.Email,
		"new_role": req.Role,
	}).Info("user role updated by admin")

	// Don't return password hash
	user.PasswordHash = ""

	c.JSON(http.StatusOK, user)
}

// @Summary Delete user (Admin only)
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "User ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /admin/users/{id} [delete]
func (ac *AdminController) DeleteUser(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		ac.log.WithField("id", c.Param("id")).Warn("invalid user id")
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	// Prevent deleting yourself
	currentUserID, exists := c.Get("user_id")
	if exists && currentUserID.(int64) == userID {
		ac.log.WithField("user_id", userID).Warn("admin attempted to delete themselves")
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot delete yourself"})
		return
	}

	// Delete user
	err = ac.userRepo.Delete(c.Request.Context(), userID)
	if err != nil {
		if err == repository.ErrUserNotFound {
			ac.log.WithField("user_id", userID).Warn("user not found for deletion")
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		ac.log.WithError(err).Error("failed to delete user")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	ac.log.WithField("user_id", userID).Info("user deleted by admin")

	c.JSON(http.StatusOK, gin.H{"message": "user deleted successfully"})
}

// @Summary List all users (Admin only)
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param limit query int false "Limit" default(10)
// @Param offset query int false "Offset" default(0)
// @Success 200 {array} models.User
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Router /admin/users [get]
func (ac *AdminController) ListUsers(c *gin.Context) {
	limit := 10
	offset := 0

	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	users, err := ac.userRepo.List(c.Request.Context(), limit, offset)
	if err != nil {
		ac.log.WithError(err).Error("failed to list users")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	ac.log.WithFields(map[string]interface{}{
		"count":  len(users),
		"limit":  limit,
		"offset": offset,
	}).Info("users listed by admin")


	for i := range users {
		users[i].PasswordHash = ""
	}

	c.JSON(http.StatusOK, users)
}
