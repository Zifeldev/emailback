package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Zifeldev/emailback/service/EmailParse/internal/config"
	"github.com/Zifeldev/emailback/service/EmailParse/internal/db"
	"github.com/Zifeldev/emailback/service/EmailParse/internal/repository"
	"github.com/Zifeldev/emailback/service/EmailParse/internal/service"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

type aiJob struct {
	ID       string `json:"id"`
	Attempts int    `json:"attempts"`
}

var (
	aiJobsProcessed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "ai_jobs_processed_total",
		Help: "Total number of AI jobs processed successfully",
	})
	aiJobsFailed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "ai_jobs_failed_total",
		Help: "Total number of AI jobs that failed",
	})
	aiJobsRetries = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "ai_jobs_retries_total",
		Help: "Total number of AI job retries performed",
	})
	aiJobDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "ai_job_duration_seconds",
		Help:    "Duration of AI job processing in seconds",
		Buckets: prometheus.DefBuckets,
	})
)

func init() {
	prometheus.MustRegister(aiJobsProcessed, aiJobsFailed, aiJobsRetries, aiJobDuration)
}

func main() {
	ctx := context.Background()
	cfg := config.MustLoad(ctx)

	log := logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{TimestampFormat: time.RFC3339Nano})
	entry := logrus.NewEntry(log).WithField("service", "emailback-aiworker")

	pool, err := db.New(ctx, cfg.Database)
	if err != nil {
		entry.WithError(err).Fatal("failed to connect to database")
	}
	defer pool.Close()
	tPool := &db.TimeoutPool{Pool: pool, QueryTimeout: cfg.Database.QueryTimeout}
	repo := repository.NewPostgresEmailRepo(tPool)

	var aiClient *service.Client
	if cfg.AI.Enabled && cfg.AI.HFToken != "" {
		sumModels, err := cfg.AI.ParseSumModels()
		if err != nil {
			entry.WithError(err).Fatal("failed to parse AI models")
		}
		aiClient = service.NewAIClient(cfg.AI.HFToken, sumModels, cfg.AI.ClassificationModel, cfg.AI.Timeout)
		entry.WithField("hf_base", aiClient.Base()).Info("ai client initialized")
	} else {
		entry.Info("ai client disabled or token missing; worker will not process jobs")
	}

	var rdb *redis.Client
	if cfg.Redis.Enabled {
		rdb = redis.NewClient(&redis.Options{Addr: cfg.Redis.Addr, Password: cfg.Redis.Password, DB: cfg.Redis.DB})
		if err := rdb.Ping(ctx).Err(); err != nil {
			entry.WithError(err).Warn("redis ping failed; worker cannot start without redis")
			rdb = nil
		} else {
			entry.WithField("addr", cfg.Redis.Addr).Info("redis connected")
		}
	} else {
		entry.Info("redis disabled; worker requires redis to receive jobs")
	}

	if rdb == nil || aiClient == nil {
		entry.Info("ai worker started in no-op mode (missing redis or ai client)")
		sigch := make(chan os.Signal, 1)
		signal.Notify(sigch, syscall.SIGINT, syscall.SIGTERM)
		<-sigch
		entry.Info("shutting down")
		return
	}

	entry.Info("ai worker started")

	metricsAddr := ":9090"
	if v := os.Getenv("METRICS_ADDR"); v != "" {
		metricsAddr = v
	}
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		entry.WithField("metrics_addr", metricsAddr).Info("starting metrics server")
		if err := http.ListenAndServe(metricsAddr, nil); err != nil {
			entry.WithError(err).Warn("metrics server stopped")
		}
	}()

	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigch
		entry.Info("shutting down")
		rdb.Close()
		pool.Close()
		os.Exit(0)
	}()

	for {
		res, err := rdb.BLPop(ctx, 0, "ai:queue").Result()
		if err != nil {
			entry.WithError(err).Warn("redis BLPop error")
			time.Sleep(1 * time.Second)
			continue
		}
		if len(res) < 2 {
			continue
		}
		payload := res[1]
		var job aiJob
		if err := json.Unmarshal([]byte(payload), &job); err != nil {
			entry.WithError(err).WithField("payload", payload).Warn("invalid job payload")
			continue
		}

		entry := entry.WithField("email_id", job.ID).WithField("attempts", job.Attempts)
		entry.Info("processing ai job")

		emailCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		e, err := repo.GetByID(emailCtx, job.ID)
		cancel()
		if err != nil {
			entry.WithError(err).Warn("failed to fetch email for ai job")
			aiJobsFailed.Inc()
			continue
		}

		maxAttempts := 3
		var summary string
		var usedModel string
		var priorityLabel string
		var priorityScore float64
		var clsModel string

		start := time.Now()
		retryCount := 0

		for attempt := 0; attempt < maxAttempts; attempt++ {
			sumCtx, sumCancel := context.WithTimeout(ctx, 30*time.Second)
			summary, err = aiClient.Summarize(sumCtx, e.Text, e.Language)
			sumCancel()
			if err != nil || summary == "" {
				entry.WithError(err).Warnf("summarization attempt %d failed", attempt+1)
				retryCount++
			} else {
				usedModel = aiClient.GetModelForLanguage(e.Language)
				break
			}
			time.Sleep(time.Duration(1<<attempt) * time.Second)
		}

		if summary == "" {
			err = fmt.Errorf("summarization failed after retries")
			entry.WithError(err).Error("ai processing failed")
			aiJobsFailed.Inc()
			aiJobsRetries.Add(float64(retryCount))
			continue
		}

		for attempt := 0; attempt < maxAttempts; attempt++ {
			priCtx, priCancel := context.WithTimeout(ctx, 15*time.Second)
			lbl, sc, perr := aiClient.DetectPriority(priCtx, e.Text)
			priCancel()
			if perr == nil && lbl != "" {
				priorityLabel = lbl
				priorityScore = sc
				clsModel = aiClient.ClsModel()
				break
			}
			entry.WithError(perr).Warnf("priority detect attempt %d failed", attempt+1)
			retryCount++
			time.Sleep(time.Duration(1<<attempt) * time.Second)
		}

		now := time.Now().UTC()
		aiSumModel := usedModel
		aiCls := clsModel
		updCtx, updCancel := context.WithTimeout(ctx, 5*time.Second)
		if err := repo.UpdateAIFields(updCtx, e.ID, &summary, &aiSumModel, &priorityLabel, &priorityScore, &aiCls, &now); err != nil {
			entry.WithError(err).Error("failed to update ai fields in db")
			aiJobsFailed.Inc()
			updCancel()
			continue
		}
		updCancel()

		aiJobsProcessed.Inc()
		if retryCount > 0 {
			aiJobsRetries.Add(float64(retryCount))
		}
		aiJobDuration.Observe(time.Since(start).Seconds())

		entry.Info("ai job finished")
	}
}
