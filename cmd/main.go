package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/Zifeldev/emailback/docs"
	"github.com/Zifeldev/emailback/internal/config"
	"github.com/Zifeldev/emailback/internal/controllers"
	"github.com/Zifeldev/emailback/internal/db"
	"github.com/Zifeldev/emailback/internal/lang"
	"github.com/Zifeldev/emailback/internal/metrics"
	"github.com/Zifeldev/emailback/internal/middleware"
	"github.com/Zifeldev/emailback/internal/repository"
	"github.com/Zifeldev/emailback/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/pemistahl/lingua-go"
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

	ld := lang.NewDetector(lingua.English, lingua.Russian, lingua.German)

	emailRepo := repository.NewPostgresEmailRepo(pool)

	emailParser := service.NewEnmimeParser(service.Options{
		HTMLToTextLimit: 1 << 20,
		IncludeHTML:     false,
	}, ld)

	r := gin.New()
	p := ginprometheus.NewPrometheus("emailback")
	p.Use(r)
	metrics.RegisterMetrics()
	r.Use(middleware.RecoveryMiddleware(log))
	r.Use(middleware.TraceMiddleware(log))
	r.Use(middleware.LoggerMiddleware(log))

	// Swagger UI
	// docs.SwaggerInfo.BasePath = "/" // set via generated docs if needed
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	pc := controllers.NewParserController(emailParser, emailRepo, baseEntry)
	r.GET("/health", controllers.HealthHandler)
	r.GET("/healthz", controllers.NewDBHealth(pool).Handle)
	r.POST("/parse", pc.ParseAndSave)
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
