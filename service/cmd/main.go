package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/Zifeldev/emailback/service/docs"
	"github.com/Zifeldev/emailback/service/internal/config"
	"github.com/Zifeldev/emailback/service/internal/controllers"
	"github.com/Zifeldev/emailback/service/internal/db"
	"github.com/Zifeldev/emailback/service/internal/lang"
	"github.com/Zifeldev/emailback/service/internal/metrics"
	"github.com/Zifeldev/emailback/service/internal/middleware"
	"github.com/Zifeldev/emailback/service/internal/repository"
	"github.com/Zifeldev/emailback/service/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/pemistahl/lingua-go"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	ginprometheus "github.com/zsais/go-gin-prometheus"
)

// Package main EmailBack API
//
// @title           EmailBack API
// @version         1.0
// @description     Service for parsing and storing email messages
// @BasePath        /
func main() {
	cfg := config.MustLoad(context.Background())

	log := logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: time.RFC3339Nano,
	})

	baseEntry := logrus.NewEntry(log).WithFields(logrus.Fields{
		"service": "emailback",
	})

	if lvl, err := logrus.ParseLevel(cfg.Logger.Level); err == nil {
		log.SetLevel(lvl)
	}

	pool, err := db.New(context.Background(), cfg.Database)
	if err != nil {
		log.WithError(err).Fatal("failed to connect to database")
	}
	defer pool.Close()
	timeoutPool := &db.TimeoutPool{
		Pool:         pool,
		QueryTimeout: cfg.Database.QueryTimeout,
	}

	ld := lang.NewDetector(lingua.English, lingua.Russian, lingua.German)

	var emailRepo repository.EmailRepository = repository.NewPostgresEmailRepo(timeoutPool)

	var rdb *redis.Client
	if cfg.Redis.Enabled {
		rdb = redis.NewClient(&redis.Options{
			Addr:     cfg.Redis.Addr,
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.DB,
		})
		// Best-effort ping on startup
		pingCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		if err := rdb.Ping(pingCtx).Err(); err != nil {
			baseEntry.WithError(err).Warn("redis ping failed; proceeding without cache")
		} else {
			baseEntry.WithField("addr", cfg.Redis.Addr).Info("redis connected")
			emailRepo = repository.NewCacheEmailRepo(emailRepo, rdb, cfg.Redis.Prefix, cfg.Redis.TTL)
		}
		cancel()
	}

	emailParser := service.NewEnmimeParser(service.Options{
		HTMLToTextLimit: 1 << 20,
		IncludeHTML:     false,
	}, ld)

	baseEntry.WithFields(logrus.Fields{
		"http_addr":        cfg.HTTP.Host,
		"req_timeout":      cfg.HTTP.RequestTimeout.String(),
		"shutdown_timeout": cfg.HTTP.ShutdownTimeout.String(),
		"db_query_timeout": cfg.Database.QueryTimeout.String(),
	}).Info("config loaded")

	r := gin.New()
	p := ginprometheus.NewPrometheus("emailback")
	p.Use(r)
	metrics.RegisterMetrics()
	r.Use(middleware.RecoveryMiddleware(log))
	r.Use(middleware.TraceMiddleware(log))
	r.Use(middleware.LoggerMiddleware(log))

	reqTimeout := cfg.HTTP.RequestTimeout
	if reqTimeout <= 0 {
		reqTimeout = 500 * time.Millisecond
		baseEntry.WithField("effective_req_timeout", reqTimeout.String()).
			Warn("HTTP request timeout was 0; using default")
	}
	r.Use(middleware.TimeoutMiddleware(reqTimeout))

	// Swagger UI
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	pc := controllers.NewParserController(emailParser, emailRepo, baseEntry)
	hc := controllers.NewHealthController(timeoutPool, rdb, baseEntry, time.Now(), "1.0.0")

	r.GET("/health", middleware.TimeoutMiddleware(2*time.Second), hc.Handle)

	r.POST("/parse", pc.ParseAndSave)
	r.POST("/parse/batch", pc.BatchParseAndSave)
	r.GET("/emails/:id", pc.GetByID)
	r.GET("/emails", pc.GetAll)

	r.NoRoute(func(c *gin.Context) {
		c.JSON(404, gin.H{"message": "Not Found"})
	})

	srv := &http.Server{
		Addr:              cfg.HTTP.Host,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	srv.RegisterOnShutdown(func() {
		baseEntry.Info("closing database connection pool")
	})

	go func() {
		log.WithField("addr", cfg.HTTP.Host).Info("starting HTTP server")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithError(err).Fatal("http server failed")
		}
	}()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Info("shutting down gracefully, press Ctrl+C again to force")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer cancel()

	baseEntry.WithFields(logrus.Fields{"signal": sig.String(), "grace_period_sec": cfg.HTTP.ShutdownTimeout}).Info("shutting down")
	if err := srv.Shutdown(shutdownCtx); err != nil {
		baseEntry.WithError(err).Error("shutdown error")
	} else {
		baseEntry.Info("server exited properly")
	}
}
