package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Zifeldev/emailback/service/EmailParse/internal/lang"
	"github.com/Zifeldev/emailback/service/EmailParse/internal/middleware"
	"github.com/Zifeldev/emailback/service/EmailParse/internal/repository"
	"github.com/Zifeldev/emailback/service/EmailParse/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

type ParserController struct {
	parser   service.Parser
	repo     repository.EmailRepository
	aiClient *service.Client
	redis    *redis.Client
	log      *logrus.Entry
	detector lang.Detector
}

func NewParserController(parser service.Parser, repo repository.EmailRepository, aiClient *service.Client, redis *redis.Client, log *logrus.Entry, detector lang.Detector) *ParserController {
	return &ParserController{
		parser:   parser,
		repo:     repo,
		aiClient: aiClient,
		redis:    redis,
		log:      log,
		detector: detector,
	}
}

type ParseResponse struct {
	*repository.EmailEntity
	Status string `json:"status"`
}

type EmailsListResponse struct {
	Count  int                       `json:"count"`
	Items  []*repository.EmailEntity `json:"items"`
	Limit  int                       `json:"limit"`
	Offset int                       `json:"offset"`
}

type aiJob struct {
	ID       string `json:"id"`
	Attempts int    `json:"attempts"`
}

type parseStageError struct {
	err error
}

func (e parseStageError) Error() string { return e.err.Error() }
func (e parseStageError) Unwrap() error { return e.err }

type saveStageError struct {
	err error
}

func (e saveStageError) Error() string { return e.err.Error() }
func (e saveStageError) Unwrap() error { return e.err }

// @Summary Parse and save an email
// @Description Accepts raw EML (text/plain or message/rfc822), parses it and persists to DB
// @Tags emails
// @Accept text/plain,message/rfc822
// @Produce json
// @Success 201 {object} repository.EmailEntity
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /parse [post]
func (pc *ParserController) ParseAndSave(c *gin.Context) {
	raw, err := c.GetRawData()
	if err != nil {
		pc.log.WithError(err).Warn("failed to read request body")
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	email, err := pc.parseAndSave(c.Request.Context(), raw, pc.userIDPtr(c))
	if err != nil {
		pc.handleParseError(c, err)
		return
	}

	c.JSON(http.StatusCreated, ParseResponse{EmailEntity: email, Status: "created"})
}

// @Summary Get email by ID
// @Tags emails
// @Produce json
// @Param id path string true "Email ID"
// @Success 200 {object} repository.EmailEntity
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /emails/{id} [get]
func (pc *ParserController) GetByID(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}

	email, err := pc.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrEmailNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "email not found"})
			return
		}
		pc.log.WithError(err).Error("failed to get email by id")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, email)
}

// @Summary List emails
// @Tags emails
// @Produce json
// @Param limit query int false "Limit" minimum(1)
// @Param offset query int false "Offset" minimum(0)
// @Success 200 {object} EmailsListResponse
// @Failure 500 {object} map[string]string
// @Router /emails [get]
func (pc *ParserController) GetAll(c *gin.Context) {
	limit := parsePositiveIntQuery(c, "limit", 100)
	offset := parseNonNegativeIntQuery(c, "offset", 0)

	items, err := pc.repo.GetAll(c.Request.Context(), limit, offset)
	if err != nil {
		pc.log.WithError(err).Error("failed to list emails")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, EmailsListResponse{
		Count:  len(items),
		Items:  items,
		Limit:  limit,
		Offset: offset,
	})
}

// GetMyEmails returns emails for authenticated user.
func (pc *ParserController) GetMyEmails(c *gin.Context) {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	limit := parsePositiveIntQuery(c, "limit", 100)
	offset := parseNonNegativeIntQuery(c, "offset", 0)

	items, err := pc.repo.GetByUserID(c.Request.Context(), strconv.FormatInt(userID, 10), limit, offset)
	if err != nil {
		pc.log.WithError(err).WithField("user_id", userID).Error("failed to list user emails")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, EmailsListResponse{
		Count:  len(items),
		Items:  items,
		Limit:  limit,
		Offset: offset,
	})
}

func (pc *ParserController) parseAndSave(ctx context.Context, raw []byte, userID *int64) (*repository.EmailEntity, error) {
	email, err := pc.parser.Parse(ctx, raw)
	if err != nil {
		return nil, parseStageError{err: err}
	}
	if email == nil {
		return nil, parseStageError{err: errors.New("parser returned empty email")}
	}

	if userID != nil {
		userIDValue := *userID
		email.UserID = &userIDValue
	}

	if err := pc.repo.SaveEmail(ctx, email); err != nil {
		return nil, saveStageError{err: err}
	}

	pc.enqueueAIJob(ctx, email.ID)
	return email, nil
}

func (pc *ParserController) enqueueAIJob(ctx context.Context, emailID string) {
	if pc.aiClient == nil || pc.redis == nil {
		return
	}

	payload, err := json.Marshal(aiJob{ID: emailID, Attempts: 0})
	if err != nil {
		pc.log.WithError(err).Warn("failed to marshal ai job")
		return
	}

	if err := pc.redis.RPush(ctx, "ai:queue", payload).Err(); err != nil {
		pc.log.WithError(err).Warn("failed to enqueue ai job")
	}
}

func (pc *ParserController) handleParseError(c *gin.Context, err error) {
	var perr parseStageError
	var serr saveStageError

	switch {
	case errors.As(err, &perr):
		status := parseErrorStatus(perr.err)
		pc.log.WithError(perr.err).Warn("failed to parse email")
		c.JSON(status, gin.H{"error": "failed to parse email"})
	case errors.As(err, &serr):
		pc.log.WithError(serr.err).Error("failed to save email")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save email"})
	default:
		pc.log.WithError(err).Error("failed to handle email")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}

func (pc *ParserController) userIDPtr(c *gin.Context) *int64 {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		return nil
	}
	return &userID
}

func parseErrorStatus(err error) int {
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "malformed") || strings.Contains(msg, "mime") || strings.Contains(msg, "header") {
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}

func parsePositiveIntQuery(c *gin.Context, key string, def int) int {
	if v := c.Query(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

func parseNonNegativeIntQuery(c *gin.Context, key string, def int) int {
	if v := c.Query(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			return n
		}
	}
	return def
}

func parseDurationQuery(c *gin.Context, key string) time.Duration {
	if v := c.Query(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return 0
}
