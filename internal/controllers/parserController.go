package controllers

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/Zifeldev/emailback/internal/middleware"
	"github.com/Zifeldev/emailback/internal/repository"
	"github.com/Zifeldev/emailback/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// EmailsListResponse defines the list response shape for Swagger
type EmailsListResponse struct {
	Limit  int                      `json:"limit"`
	Offset int                      `json:"offset"`
	Count  int                      `json:"count"`
	Items  []repository.EmailEntity `json:"items"`
}

type ParserController struct {
	parser service.Parser
	repo   repository.EmailRepository
	log    *logrus.Entry
}

func NewParserController(p service.Parser, r repository.EmailRepository, log *logrus.Entry) *ParserController {
	return &ParserController{
		parser: p,
		repo:   r,
		log:    log,
	}
}

func (pc *ParserController) reqLogger(c *gin.Context) *logrus.Entry {
	traceID := c.GetHeader(middleware.HeaderTraceID)
	if traceID == "" {
		traceID = c.GetHeader(middleware.HeaderRequestID)
	}
	return pc.log.WithFields(logrus.Fields{
		"handler":    "ParserController",
		"trace_id":   traceID,
		"remote_ip":  c.ClientIP(),
		"path":       c.Request.URL.Path,
		"client_id":  c.GetHeader("X-Client-ID"),
		"request_id": traceID,
	})
}

// ParseAndSave
// @Summary      Parse and save an email
// @Description  Accepts raw EML (text/plain or message/rfc822), parses it and persists to DB
// @Tags         emails
// @Accept       plain
// @Accept       message/rfc822
// @Produce      json
// @Success      201  {object}  repository.EmailEntity
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /parse [post]
func (pc *ParserController) ParseAndSave(c *gin.Context) {
	log := pc.reqLogger(c)
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 10<<20) // 10 MB limit
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.WithError(err).Warn("bad request body")
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	ent, err := pc.parser.Parse(c.Request.Context(), raw)
	if err != nil {
		log.WithError(err).Error("failed to process and save email")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "processing failed"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	if err := pc.repo.SaveEmail(ctx, ent); err != nil {
		log.WithError(err).
			WithField("message_id", ent.MessageID).
			Error("repo.Save failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save email"})
		return
	}

	saved, err := pc.repo.GetByID(ctx, ent.ID)
	if err != nil {
		log.WithError(err).Warn("saved but failed to re-fetch entity by ID; returning parsed entity")
		c.JSON(http.StatusCreated, ent)
		return
	}

	c.JSON(http.StatusCreated, saved)
}

// GetByID
// @Summary      Get email by ID
// @Tags         emails
// @Produce      json
// @Param        id   path      string  true  "Email ID"
// @Success      200  {object}  repository.EmailEntity
// @Failure      404  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /emails/{id} [get]
func (pc *ParserController) GetByID(c *gin.Context) {
	log := pc.reqLogger(c).WithField("handler", "GetByID")

	id := c.Param("id")
	if id == "" {
		log.Warn("missing id param")
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing id"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	ent, err := pc.repo.GetByID(ctx, id)
	if err != nil {
		if err == repository.ErrEmailNotFound {
			log.WithField("id", id).Warn("email not found")
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
			return
		}
		pc.log.WithError(err).Error("repo.GetByID failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}
	c.JSON(http.StatusOK, ent)
}

// GetAll
// @Summary      List emails
// @Tags         emails
// @Produce      json
// @Param        limit   query   int  false  "Limit"   minimum(1)
// @Param        offset  query   int  false  "Offset"  minimum(0)
// @Success      200  {object}  EmailsListResponse
// @Failure      500  {object}  map[string]string
// @Router       /emails [get]
func (pc *ParserController) GetAll(c *gin.Context) {
	log := pc.reqLogger(c).WithField("handler", "List")

	limit := 50
	offset := 0
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}
	if o := c.Query("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	items, err := pc.repo.GetAll(ctx, limit, offset)
	if err != nil {
		log.WithError(err).Error("repo.GetAll failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"limit":  limit,
		"offset": offset,
		"count":  len(items),
		"items":  items,
	})
}
