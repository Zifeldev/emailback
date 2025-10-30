package controllers

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/Zifeldev/emailback/service/internal/middleware"
	"github.com/Zifeldev/emailback/service/internal/repository"
	"github.com/Zifeldev/emailback/service/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

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

// BatchEmailInput
type BatchEmailInput struct {
	Raw string `json:"raw" example:"From: Alice <alice@example.com>\r\nTo: Bob <bob@example.com>\r\nMessage-ID: <msg-1@example.com>\r\nDate: Wed, 30 Oct 2025 18:00:00 +0000\r\nSubject: Test\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\nHello!"`
}

// BatchItemResult 
type BatchItemResult struct {
	Index      int    `json:"index"`
	Status     string `json:"status"` // ok|error
	EmailID    string `json:"email_id,omitempty"`
	Error      string `json:"error,omitempty"`
	DurationMS int64  `json:"duration_ms,omitempty"` // ms
}

// BatchResponse 
type BatchResponse struct {
	Processed int               `json:"processed"`
	Succeeded int               `json:"succeeded"`
	Failed    int               `json:"failed"`
	Results   []BatchItemResult `json:"results"`
}

// BatchParseAndSave
// @Summary      Batch parse and save emails
// @Description  batch emails parsing.
// @Tags         emails
// @Accept       json
// @Produce      json
// @Param        max_workers   query   int     false  "Максимум параллельных воркеров (1..100)" minimum(1) maximum(100) default(5)
// @Param        item_timeout  query   string  false  "Таймаут на один элемент (напр. 500ms, 2s)" default(500ms)
// @Param        body          body    []BatchEmailInput  true  "Список писем (RFC822 в поле raw)"
// @Success      200  {object}  BatchResponse
// @Failure      400  {object}  map[string]string
// @Router       /parse/batch [post]
func (pc *ParserController) BatchParseAndSave(c *gin.Context) {
	log := pc.reqLogger(c).WithField("handler", "BatchParseAndSave")

	var inputs []BatchEmailInput
	if err := c.ShouldBindJSON(&inputs); err != nil {
		log.WithError(err).Warn("bad batch body")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(inputs) == 0 {
		log.Warn("empty input")
		c.JSON(http.StatusBadRequest, gin.H{"error": "empty input"})
		return
	}

	maxWorkers := 5
	if s := c.Query("max_workers"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v >= 1 && v <= 100 {
			maxWorkers = v
		}
	}
	itemTimeout := 500 * time.Millisecond
	if s := c.Query("item_timeout"); s != "" {
		if d, err := time.ParseDuration(s); err == nil && d > 0 {
			itemTimeout = d
		}
	}

	log = log.WithFields(logrus.Fields{
		"items":        len(inputs),
		"max_workers":  maxWorkers,
		"item_timeout": itemTimeout.String(),
	})
	log.Info("batch started")

	type job struct {
		i   int
		raw string
	}
	jobs := make(chan job, len(inputs))
	results := make(chan BatchItemResult, len(inputs))

	ctx := c.Request.Context()

	worker := func(id int) {
		for j := range jobs {
			start := time.Now()
			ictx, cancel := context.WithTimeout(ctx, itemTimeout)
			ent, err := pc.parser.Parse(ictx, []byte(j.raw)) 
			if err == nil {
				err = pc.repo.SaveEmail(ictx, ent) 
			}
			cancel()

			dur := time.Since(start).Milliseconds()
			if err != nil {
				log.WithFields(logrus.Fields{"idx": j.i, "dur": dur, "err": err.Error()}).Warn("item parse failed")
				results <- BatchItemResult{Index: j.i, Status: "error", Error: err.Error(), DurationMS: dur}
				continue
			}
			results <- BatchItemResult{Index: j.i, Status: "ok", EmailID: ent.ID, DurationMS: dur}
		}
	}

	wc := maxWorkers
	if wc > len(inputs) {
		wc = len(inputs)
	}
	var wg sync.WaitGroup
	wg.Add(wc)
	for i := 0; i < wc; i++ {
		go func(id int) {
			defer wg.Done()
			worker(id)
		}(i + 1)
	}
	for i, in := range inputs {
		jobs <- job{i: i, raw: in.Raw}
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	out := make([]BatchItemResult, len(inputs))
	ok, fail := 0, 0
	for r := range results {
		out[r.Index] = r
		if r.Status == "ok" {
			ok++
		} else {
			fail++
		}
	}

	log.WithFields(logrus.Fields{
		"succeeded": ok,
		"failed":    fail,
		"dur_ms":    time.Since(c.GetTime("start")).Milliseconds(),
	}).Info("batch finished")

	c.JSON(http.StatusOK, BatchResponse{
		Processed: len(inputs),
		Succeeded: ok,
		Failed:    fail,
		Results:   out,
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
