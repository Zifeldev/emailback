package handler

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/Zifeldev/emailback/internal/repository"
	"github.com/Zifeldev/emailback/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type ParserController struct {
	parser service.Parser
	repo   repository.EmailRepository
	log    *logrus.Logger
}


func NewParserController(p service.Parser, r repository.EmailRepository, log *logrus.Logger) *ParserController {
	return &ParserController{
		parser: p,
		repo:   r,
		log:    log,
	}
}

func (pc *ParserController) ParseAndSave(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 10<<20) // 10 MB limit
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		pc.log.WithError(err).Warn("bad request body")
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	ent, err := pc.parser.Parse(c.Request.Context(), raw)
	if err != nil {
		pc.log.WithError(err).Error("failed to process and save email")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "processing failed"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	if err := pc.repo.SaveEmail(ctx, ent); err != nil {
		pc.log.WithError(err).
			WithField("message_id", ent.MessageID).
			Error("repo.Save failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save email"})
		return
	}

	saved, err := pc.repo.GetByID(ctx, ent.ID)
	if err != nil {
		pc.log.WithError(err).Warn("saved but failed to re-fetch entity by ID; returning parsed entity")
		c.JSON(http.StatusCreated, ent)
		return
	}

	c.JSON(http.StatusCreated, saved)
}

func (pc *ParserController) GetByID(c *gin.Context) {
	log := pc.log.WithField("handler", "GetByID")

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
			log.Warn("email not found")
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
			return
		}
		pc.log.WithError(err).Error("repo.GetByID failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}
	c.JSON(http.StatusOK, ent)
}

func (ec *ParserController) List(c *gin.Context) {
	log := ec.log.WithField("handler", "List")

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

	items, err := ec.repo.GetAll(ctx, limit, offset)
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