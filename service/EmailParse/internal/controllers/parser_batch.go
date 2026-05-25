package controllers

import (
	"context"
	"net/http"
	"runtime"
	"sync"

	"github.com/gin-gonic/gin"
)

type BatchEmailInput struct {
	Raw string `json:"raw"`
}

type BatchEmailResult struct {
	ID    string `json:"id,omitempty"`
	Error string `json:"error,omitempty"`
}

type BatchParseResponse struct {
	Count int                `json:"count"`
	Items []BatchEmailResult `json:"items"`
}

func (pc *ParserController) BatchParseAndSave(c *gin.Context) {
	var payload []BatchEmailInput
	if err := c.ShouldBindJSON(&payload); err != nil {
		pc.log.WithError(err).Warn("invalid batch parse request")
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if len(payload) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "empty batch"})
		return
	}

	maxWorkers := parsePositiveIntQuery(c, "max_workers", runtime.NumCPU())
	if maxWorkers > len(payload) {
		maxWorkers = len(payload)
	}

	itemTimeout := parseDurationQuery(c, "item_timeout")
	userID := pc.userIDPtr(c)
	baseCtx := context.WithoutCancel(c.Request.Context())

	type job struct {
		idx int
		raw string
	}
	type result struct {
		idx int
		res BatchEmailResult
	}

	jobs := make(chan job)
	results := make(chan result, len(payload))
	var wg sync.WaitGroup

	worker := func() {
		defer wg.Done()
		for j := range jobs {
			ctx := baseCtx
			var cancel context.CancelFunc
			if itemTimeout > 0 {
				ctx, cancel = context.WithTimeout(baseCtx, itemTimeout)
			}
			res := pc.safeBatchItem(ctx, j.raw, userID)
			if cancel != nil {
				cancel()
			}
			results <- result{idx: j.idx, res: res}
		}
	}

	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go worker()
	}

	for i, item := range payload {
		jobs <- job{idx: i, raw: item.Raw}
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	out := make([]BatchEmailResult, len(payload))
	for res := range results {
		out[res.idx] = res.res
	}

	c.JSON(http.StatusOK, BatchParseResponse{
		Count: len(out),
		Items: out,
	})
}

func (pc *ParserController) handleBatchItem(ctx context.Context, raw string, userID *int64) BatchEmailResult {
	email, err := pc.parseAndSave(ctx, []byte(raw), userID)
	if err != nil {
		return BatchEmailResult{Error: err.Error()}
	}
	return BatchEmailResult{ID: email.ID}
}

func (pc *ParserController) safeBatchItem(ctx context.Context, raw string, userID *int64) (res BatchEmailResult) {
	defer func() {
		if r := recover(); r != nil {
			pc.log.WithField("panic", r).Error("batch parse panic")
			res = BatchEmailResult{Error: "internal error"}
		}
	}()
	return pc.handleBatchItem(ctx, raw, userID)
}
